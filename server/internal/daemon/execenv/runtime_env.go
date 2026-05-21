package execenv

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const runtimeEnvFileName = "runtime-env.json"

// RuntimeEnvFilePath returns the stable file path used to refresh dynamic
// per-task context for long-lived provider processes.
func RuntimeEnvFilePath(rootDir string) string {
	if rootDir == "" {
		return ""
	}
	return filepath.Join(rootDir, runtimeEnvFileName)
}

// WriteRuntimeEnvFile writes dynamic, non-secret environment values into the
// execution root. Long-lived agent processes receive only a pointer to this
// file so child multica CLI invocations can resolve the current task context.
func WriteRuntimeEnvFile(rootDir string, values map[string]string) (string, error) {
	path := RuntimeEnvFilePath(rootDir)
	if path == "" {
		return "", fmt.Errorf("execenv: runtime env root is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("execenv: create runtime env dir: %w", err)
	}

	filtered := make(map[string]string, len(values))
	for key, value := range values {
		if key == "" || value == "" {
			continue
		}
		filtered[key] = value
	}
	data, err := marshalRuntimeEnv(filtered)
	if err != nil {
		return "", err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), "."+runtimeEnvFileName+".*.tmp")
	if err != nil {
		return "", fmt.Errorf("execenv: create runtime env temp: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("execenv: write runtime env temp: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("execenv: chmod runtime env temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("execenv: close runtime env temp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return "", fmt.Errorf("execenv: replace runtime env file: %w", err)
	}
	cleanup = false
	return path, nil
}

func marshalRuntimeEnv(values map[string]string) ([]byte, error) {
	data, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("execenv: marshal runtime env: %w", err)
	}
	return append(data, '\n'), nil
}
