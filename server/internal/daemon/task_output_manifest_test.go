package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadTaskOutputManifestReadsExplicitFileOnly(t *testing.T) {
	workDir := t.TempDir()
	manifest, err := loadTaskOutputManifest(workDir)
	if err != nil {
		t.Fatalf("missing manifest returned error: %v", err)
	}
	if manifest != nil {
		t.Fatalf("missing manifest = %+v, want nil", manifest)
	}

	if err := os.MkdirAll(filepath.Join(workDir, ".multica"), 0o755); err != nil {
		t.Fatalf("create manifest dir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(workDir, TaskOutputManifestRelativePath),
		[]byte(`{"outputs":[{"relative_path":"docs/design.md","kind":"doc","size_bytes":42}]}`),
		0o644,
	); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	manifest, err = loadTaskOutputManifest(workDir)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if manifest == nil || len(manifest.Outputs) != 1 {
		t.Fatalf("expected 1 output, got %+v", manifest)
	}
	output := manifest.Outputs[0]
	if output.RelativePath != "docs/design.md" || output.Kind != "doc" {
		t.Fatalf("unexpected output: %+v", output)
	}
	if output.SizeBytes == nil || *output.SizeBytes != 42 {
		t.Fatalf("size_bytes = %v, want 42", output.SizeBytes)
	}
}

func TestLoadTaskOutputManifestRejectsLargeManifest(t *testing.T) {
	workDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workDir, ".multica"), 0o755); err != nil {
		t.Fatalf("create manifest dir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(workDir, TaskOutputManifestRelativePath),
		[]byte(strings.Repeat("x", int(maxTaskOutputManifestBytes)+1)),
		0o644,
	); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	if _, err := loadTaskOutputManifest(workDir); err == nil {
		t.Fatal("expected oversized manifest error")
	}
}
