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

func prepareStructuredTaskOutputDir(rootDir string, scoped bool, taskLog *slog.Logger) {
	if rootDir == "" {
		return
	}
	dir := filepath.Join(rootDir, ".multica")
	if scoped {
		dir = rootDir
	}
	if err := os.MkdirAll(dir, 0o755); err != nil && taskLog != nil {
		taskLog.Warn("structured output directory prepare failed", "dir", dir, "error", err)
	}
}

func clearStaleStructuredTaskOutputManifests(rootDir string, scoped bool, taskLog *slog.Logger) {
	if rootDir == "" {
		return
	}
	for _, relativePath := range []string{
		structuredManifestRelativePath(scoped, TaskChatSummaryManifestFileName),
		structuredManifestRelativePath(scoped, TaskPlanSummaryManifestFileName),
		structuredManifestRelativePath(scoped, TaskIssueProposalsManifestFileName),
		structuredManifestRelativePath(scoped, TaskOutputManifestFileName),
	} {
		if err := os.Remove(filepath.Join(rootDir, relativePath)); err != nil && !os.IsNotExist(err) {
			if taskLog != nil {
				taskLog.Warn("stale structured output manifest cleanup failed", "path", relativePath, "error", err)
			}
		}
	}
}

func loadTaskOutputManifest(rootDir string, scoped bool) (*TaskOutputManifest, error) {
	var manifest TaskOutputManifest
	ok, err := loadTaskManifestFile(rootDir, structuredManifestRelativePath(scoped, TaskOutputManifestFileName), "output manifest", &manifest)
	if err != nil || !ok {
		return nil, err
	}
	return &manifest, nil
}

func loadStructuredTaskOutputs(rootDir string, scoped bool, taskLog *slog.Logger) *StructuredTaskOutputs {
	if rootDir == "" {
		return nil
	}

	var structured StructuredTaskOutputs
	loaded := false

	var summary ChatSummaryManifest
	if ok := loadStructuredManifest(rootDir, structuredManifestRelativePath(scoped, TaskChatSummaryManifestFileName), "chat summary manifest", &summary, taskLog); ok {
		structured.ChatSummary = &summary
		loaded = true
	}

	var planSummary PlanSummaryManifest
	if ok := loadStructuredManifest(rootDir, structuredManifestRelativePath(scoped, TaskPlanSummaryManifestFileName), "plan summary manifest", &planSummary, taskLog); ok {
		structured.PlanSummary = &planSummary
		loaded = true
	}

	var proposals IssueProposalsManifest
	if ok := loadStructuredManifest(rootDir, structuredManifestRelativePath(scoped, TaskIssueProposalsManifestFileName), "issue proposals manifest", &proposals, taskLog); ok {
		structured.IssueProposals = &proposals
		loaded = true
	}

	var outputs TaskOutputManifest
	if ok := loadStructuredManifest(rootDir, structuredManifestRelativePath(scoped, TaskOutputManifestFileName), "output manifest", &outputs, taskLog); ok {
		structured.Outputs = &outputs
		loaded = true
	}

	if !loaded {
		return nil
	}
	return &structured
}

func structuredManifestRelativePath(scoped bool, fileName string) string {
	if scoped {
		return fileName
	}
	switch fileName {
	case TaskChatSummaryManifestFileName:
		return TaskChatSummaryManifestRelativePath
	case TaskPlanSummaryManifestFileName:
		return TaskPlanSummaryManifestRelativePath
	case TaskIssueProposalsManifestFileName:
		return TaskIssueProposalsManifestRelativePath
	case TaskOutputManifestFileName:
		return TaskOutputManifestRelativePath
	default:
		return filepath.Join(".multica", fileName)
	}
}

func structuredOutputRoot(result TaskResult) (string, bool) {
	if result.StructuredOutputDir != "" {
		return result.StructuredOutputDir, result.StructuredOutputDir != result.WorkDir
	}
	return result.WorkDir, false
}

func loadStructuredManifest(rootDir, relativePath, label string, target any, taskLog *slog.Logger) bool {
	ok, err := loadTaskManifestFile(rootDir, relativePath, label, target)
	if err != nil {
		if taskLog != nil {
			taskLog.Warn(label+" ignored", "error", err)
		}
		return false
	}
	return ok
}

func loadTaskManifestFile(rootDir, relativePath, label string, target any) (bool, error) {
	if rootDir == "" {
		return false, nil
	}
	manifestPath := filepath.Join(rootDir, relativePath)
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

func (d *Daemon) reportTaskOutputMetadata(ctx context.Context, taskID, rootDir string, scoped bool, taskLog *slog.Logger) {
	manifest, err := loadTaskOutputManifest(rootDir, scoped)
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
