package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

const maxTaskOutputManifestBytes int64 = 512 * 1024

func clearStaleStructuredTaskOutputManifests(workDir string, taskLog *slog.Logger) {
	if workDir == "" {
		return
	}
	for _, relativePath := range []string{
		TaskChatSummaryManifestRelativePath,
		TaskIssueProposalsManifestRelativePath,
		TaskOutputManifestRelativePath,
	} {
		if err := os.Remove(filepath.Join(workDir, relativePath)); err != nil && !os.IsNotExist(err) {
			if taskLog != nil {
				taskLog.Warn("stale structured output manifest cleanup failed", "path", relativePath, "error", err)
			}
		}
	}
}

func loadTaskOutputManifest(workDir string) (*TaskOutputManifest, error) {
	var manifest TaskOutputManifest
	ok, err := loadTaskManifestFile(workDir, TaskOutputManifestRelativePath, "output manifest", &manifest)
	if err != nil || !ok {
		return nil, err
	}
	return &manifest, nil
}

func loadStructuredTaskOutputs(workDir string, taskLog *slog.Logger) *StructuredTaskOutputs {
	if workDir == "" {
		return nil
	}

	var structured StructuredTaskOutputs
	loaded := false

	var summary ChatSummaryManifest
	if ok := loadStructuredManifest(workDir, TaskChatSummaryManifestRelativePath, "chat summary manifest", &summary, taskLog); ok {
		structured.ChatSummary = &summary
		loaded = true
	}

	var proposals IssueProposalsManifest
	if ok := loadStructuredManifest(workDir, TaskIssueProposalsManifestRelativePath, "issue proposals manifest", &proposals, taskLog); ok {
		structured.IssueProposals = &proposals
		loaded = true
	}

	var outputs TaskOutputManifest
	if ok := loadStructuredManifest(workDir, TaskOutputManifestRelativePath, "output manifest", &outputs, taskLog); ok {
		structured.Outputs = &outputs
		loaded = true
	}

	if !loaded {
		return nil
	}
	return &structured
}

func loadStructuredManifest(workDir, relativePath, label string, target any, taskLog *slog.Logger) bool {
	ok, err := loadTaskManifestFile(workDir, relativePath, label, target)
	if err != nil {
		if taskLog != nil {
			taskLog.Warn(label+" ignored", "error", err)
		}
		return false
	}
	return ok
}

func loadTaskManifestFile(workDir, relativePath, label string, target any) (bool, error) {
	if workDir == "" {
		return false, nil
	}
	manifestPath := filepath.Join(workDir, relativePath)
	info, err := os.Stat(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat %s: %w", label, err)
	}
	if info.IsDir() {
		return false, fmt.Errorf("%s path is a directory", label)
	}
	if info.Size() > maxTaskOutputManifestBytes {
		return false, fmt.Errorf("%s exceeds %d bytes", label, maxTaskOutputManifestBytes)
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", label, err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		return false, fmt.Errorf("parse %s: %w", label, err)
	}
	return true, nil
}

func (d *Daemon) reportTaskOutputMetadata(ctx context.Context, taskID, workDir string, taskLog *slog.Logger) {
	manifest, err := loadTaskOutputManifest(workDir)
	if err != nil {
		taskLog.Warn("task output manifest ignored", "error", err)
		return
	}
	if manifest == nil {
		return
	}
	if err := d.client.ReportTaskOutputMetadata(ctx, taskID, manifest.Outputs); err != nil {
		taskLog.Warn("task output metadata upload failed", "error", err, "outputs", len(manifest.Outputs))
		return
	}
	taskLog.Info("task output metadata uploaded", "outputs", len(manifest.Outputs))
}
