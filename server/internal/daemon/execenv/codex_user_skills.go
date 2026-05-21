package execenv

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

const userCodexSkillsManifestFile = ".multica_user_skills_manifest.json"

type userCodexSkillsManifest struct {
	Version int                                    `json:"version"`
	Skills  map[string]userCodexSkillManifestEntry `json:"skills"`
}

type userCodexSkillManifestEntry struct {
	Source string                     `json:"source"`
	Files  []userCodexSkillFileRecord `json:"files"`
}

type userCodexSkillFileRecord struct {
	Path            string `json:"path"`
	Kind            string `json:"kind"`
	Size            int64  `json:"size,omitempty"`
	Mode            uint32 `json:"mode,omitempty"`
	ModTimeUnixNano int64  `json:"mod_time_unix_nano,omitempty"`
}

type userCodexSkillSyncSource struct {
	name     string
	src      string
	manifest userCodexSkillManifestEntry
}

// seedUserCodexSkills mirrors user-installed skill directories from the shared
// ~/.codex/skills/ into the per-task CODEX_HOME so the codex CLI discovers
// them natively. Codex is the only runtime whose HOME is redirected to a
// per-task directory (via the CODEX_HOME env var), so without this step the
// CLI never sees the user's `~/.codex/skills/` content.
//
// Workspace-assigned skills take precedence on name conflict: any user skill
// whose sanitized name matches a workspace skill's sanitized name is skipped
// here, and writeSkillFiles then writes the workspace version into a clean
// slot.
//
// Unchanged user skills are left in place on env reuse. This preserves the
// final tree while avoiding thousands of repeated file clones for large local
// skill bundles.
//
// Per-skill failures are logged and skipped — a single broken user skill must
// not prevent the task from running. Returning an error is reserved for
// failures that prevent listing the shared skills directory at all.
func seedUserCodexSkills(codexHome string, workspaceSkills []SkillContextForEnv, logger *slog.Logger) error {
	sharedSkillsDir := filepath.Join(resolveSharedCodexHome(), "skills")
	targetSkillsDir := filepath.Join(codexHome, "skills")

	info, err := os.Stat(sharedSkillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			if len(workspaceSkills) == 0 {
				if err := os.RemoveAll(targetSkillsDir); err != nil {
					return fmt.Errorf("clear stale codex skills dir: %w", err)
				}
			}
			if err := writeUserCodexSkillsManifest(codexHome, userCodexSkillsManifest{Version: 1, Skills: map[string]userCodexSkillManifestEntry{}}); err != nil {
				logger.Warn("execenv: codex user-skill manifest write failed", "error", err)
			}
			return nil
		}
		return fmt.Errorf("stat shared skills dir: %w", err)
	}
	if !info.IsDir() {
		if len(workspaceSkills) == 0 {
			if err := os.RemoveAll(targetSkillsDir); err != nil {
				return fmt.Errorf("clear stale codex skills dir: %w", err)
			}
		}
		if err := writeUserCodexSkillsManifest(codexHome, userCodexSkillsManifest{Version: 1, Skills: map[string]userCodexSkillManifestEntry{}}); err != nil {
			logger.Warn("execenv: codex user-skill manifest write failed", "error", err)
		}
		return nil
	}

	reserved := make(map[string]struct{}, len(workspaceSkills))
	for _, s := range workspaceSkills {
		reserved[sanitizeSkillName(s.Name)] = struct{}{}
	}

	entries, err := os.ReadDir(sharedSkillsDir)
	if err != nil {
		return fmt.Errorf("read shared skills dir: %w", err)
	}

	previousManifest := readUserCodexSkillsManifest(codexHome)
	nextManifest := userCodexSkillsManifest{Version: 1, Skills: map[string]userCodexSkillManifestEntry{}}
	sources := make(map[string]userCodexSkillSyncSource, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if name == "" || strings.HasPrefix(name, ".") {
			continue
		}
		if _, claimed := reserved[sanitizeSkillName(name)]; claimed {
			logger.Info("execenv: codex user-skill yields to workspace skill", "name", name)
			continue
		}
		src := filepath.Join(sharedSkillsDir, name)
		// Installers like lark-cli ship each skill as a symlink into a
		// shared ~/.agents/skills/<name>/ directory. Resolve symlinks so we
		// copy the real content into the per-task home.
		resolved, err := filepath.EvalSymlinks(src)
		if err != nil {
			logger.Warn("execenv: codex user-skill resolve failed", "name", name, "error", err)
			continue
		}
		fi, err := os.Stat(resolved)
		if err != nil || !fi.IsDir() {
			continue
		}
		manifest, err := buildUserCodexSkillManifestEntry(resolved)
		if err != nil {
			logger.Warn("execenv: codex user-skill manifest failed", "name", name, "error", err)
			continue
		}
		sources[name] = userCodexSkillSyncSource{name: name, src: resolved, manifest: manifest}
	}

	if len(sources) == 0 && len(workspaceSkills) == 0 {
		if err := os.RemoveAll(targetSkillsDir); err != nil {
			return fmt.Errorf("clear stale codex skills dir: %w", err)
		}
		if err := writeUserCodexSkillsManifest(codexHome, nextManifest); err != nil {
			logger.Warn("execenv: codex user-skill manifest write failed", "error", err)
		}
		return nil
	}

	if err := os.MkdirAll(targetSkillsDir, 0o755); err != nil {
		return fmt.Errorf("create codex skills dir: %w", err)
	}
	if err := removeStaleCodexSkillDirs(targetSkillsDir, sources); err != nil {
		return err
	}

	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		source := sources[name]
		dst := filepath.Join(targetSkillsDir, name)
		nextManifest.Skills[name] = source.manifest
		if previous, ok := previousManifest.Skills[name]; ok &&
			reflect.DeepEqual(previous, source.manifest) &&
			destinationMatchesUserSkillManifest(dst, source.manifest) {
			continue
		}
		if err := os.RemoveAll(dst); err != nil {
			logger.Warn("execenv: codex user-skill clean dst failed", "name", name, "error", err)
			delete(nextManifest.Skills, name)
			continue
		}
		if err := copyDirTree(source.src, dst); err != nil {
			logger.Warn("execenv: codex user-skill copy failed", "name", name, "error", err)
			delete(nextManifest.Skills, name)
			continue
		}
	}
	if err := writeUserCodexSkillsManifest(codexHome, nextManifest); err != nil {
		logger.Warn("execenv: codex user-skill manifest write failed", "error", err)
	}
	return nil
}

func readUserCodexSkillsManifest(codexHome string) userCodexSkillsManifest {
	data, err := os.ReadFile(filepath.Join(codexHome, userCodexSkillsManifestFile))
	if err != nil {
		return userCodexSkillsManifest{Version: 1, Skills: map[string]userCodexSkillManifestEntry{}}
	}
	var manifest userCodexSkillsManifest
	if err := json.Unmarshal(data, &manifest); err != nil || manifest.Version != 1 || manifest.Skills == nil {
		return userCodexSkillsManifest{Version: 1, Skills: map[string]userCodexSkillManifestEntry{}}
	}
	return manifest
}

func writeUserCodexSkillsManifest(codexHome string, manifest userCodexSkillsManifest) error {
	if manifest.Skills == nil {
		manifest.Skills = map[string]userCodexSkillManifestEntry{}
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(codexHome, userCodexSkillsManifestFile), data, 0o644)
}

func buildUserCodexSkillManifestEntry(src string) (userCodexSkillManifestEntry, error) {
	entry := userCodexSkillManifestEntry{Source: src}
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			entry.Files = append(entry.Files, userCodexSkillFileRecord{Path: rel, Kind: "dir"})
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		entry.Files = append(entry.Files, userCodexSkillFileRecord{
			Path:            rel,
			Kind:            "file",
			Size:            info.Size(),
			Mode:            uint32(info.Mode().Perm()),
			ModTimeUnixNano: info.ModTime().UnixNano(),
		})
		return nil
	})
	if err != nil {
		return userCodexSkillManifestEntry{}, err
	}
	sort.Slice(entry.Files, func(i, j int) bool {
		return entry.Files[i].Path < entry.Files[j].Path
	})
	return entry, nil
}

func removeStaleCodexSkillDirs(targetSkillsDir string, sources map[string]userCodexSkillSyncSource) error {
	entries, err := os.ReadDir(targetSkillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read codex skills dir: %w", err)
	}
	for _, entry := range entries {
		if _, keep := sources[entry.Name()]; keep {
			continue
		}
		if err := os.RemoveAll(filepath.Join(targetSkillsDir, entry.Name())); err != nil {
			return fmt.Errorf("remove stale codex skill %s: %w", entry.Name(), err)
		}
	}
	return nil
}

func destinationMatchesUserSkillManifest(dst string, manifest userCodexSkillManifestEntry) bool {
	info, err := os.Stat(dst)
	if err != nil || !info.IsDir() {
		return false
	}
	expected := make(map[string]userCodexSkillFileRecord, len(manifest.Files))
	for _, record := range manifest.Files {
		expected[record.Path] = record
	}
	seen := make(map[string]struct{}, len(manifest.Files))
	err = filepath.WalkDir(dst, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(dst, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		record, ok := expected[rel]
		if !ok {
			return fmt.Errorf("unexpected path %s", rel)
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("unexpected symlink %s", rel)
		}
		switch record.Kind {
		case "dir":
			if !d.IsDir() {
				return fmt.Errorf("expected directory %s", rel)
			}
		case "file":
			if d.IsDir() || !d.Type().IsRegular() {
				return fmt.Errorf("expected regular file %s", rel)
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			if info.Size() != record.Size {
				return fmt.Errorf("size mismatch %s", rel)
			}
		default:
			return fmt.Errorf("unknown manifest kind %s", record.Kind)
		}
		seen[rel] = struct{}{}
		return nil
	})
	if err != nil {
		return false
	}
	return len(seen) == len(expected)
}

// copyDirTree walks src recursively and copies every regular file under it
// to the matching path under dst. Nested symlinks are ignored to keep the
// per-task home self-contained; the caller is expected to resolve the root
// before calling.
func copyDirTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return copyFile(path, target)
	})
}
