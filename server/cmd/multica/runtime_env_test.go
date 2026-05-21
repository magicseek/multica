package main

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func resetRuntimeEnvForTest(t *testing.T) {
	t.Helper()
	runtimeEnvOnce = sync.Once{}
	runtimeEnvValues = nil
}

func TestRuntimeEnvValueFallsBackToFile(t *testing.T) {
	resetRuntimeEnvForTest(t)
	path := filepath.Join(t.TempDir(), "runtime-env.json")
	if err := os.WriteFile(path, []byte(`{"MULTICA_TASK_ID":"task-from-file"}`), 0o600); err != nil {
		t.Fatalf("write runtime env: %v", err)
	}
	t.Setenv(runtimeEnvFileEnv, path)
	t.Setenv("MULTICA_TASK_ID", "")

	if got := runtimeEnvValue("MULTICA_TASK_ID"); got != "task-from-file" {
		t.Fatalf("runtimeEnvValue() = %q, want task-from-file", got)
	}
}

func TestRuntimeEnvValuePrefersProcessEnv(t *testing.T) {
	resetRuntimeEnvForTest(t)
	path := filepath.Join(t.TempDir(), "runtime-env.json")
	if err := os.WriteFile(path, []byte(`{"MULTICA_TASK_ID":"task-from-file"}`), 0o600); err != nil {
		t.Fatalf("write runtime env: %v", err)
	}
	t.Setenv(runtimeEnvFileEnv, path)
	t.Setenv("MULTICA_TASK_ID", "task-from-env")

	if got := runtimeEnvValue("MULTICA_TASK_ID"); got != "task-from-env" {
		t.Fatalf("runtimeEnvValue() = %q, want task-from-env", got)
	}
}
