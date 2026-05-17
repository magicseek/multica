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

func loadTaskOutputManifest(workDir string) (*TaskOutputManifest, error) {
	if workDir == "" {
		return nil, nil
	}
	manifestPath := filepath.Join(workDir, TaskOutputManifestRelativePath)
	info, err := os.Stat(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat output manifest: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("output manifest path is a directory")
	}
	if info.Size() > maxTaskOutputManifestBytes {
		return nil, fmt.Errorf("output manifest exceeds %d bytes", maxTaskOutputManifestBytes)
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read output manifest: %w", err)
	}
	var manifest TaskOutputManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse output manifest: %w", err)
	}
	return &manifest, nil
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
