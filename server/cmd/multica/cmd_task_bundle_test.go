package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/cobra"
)

func freshTaskBundleCheckpointCmd() *cobra.Command {
	c := &cobra.Command{Use: "checkpoint"}
	c.Flags().String("status", "", "")
	c.Flags().String("result", "", "")
	c.Flags().Bool("result-stdin", false, "")
	c.Flags().String("result-file", "", "")
	c.Flags().String("error", "", "")
	c.Flags().Int32("checkpoint-seq", 0, "")
	c.Flags().String("output", "json", "")
	return c
}

func TestTaskBundleCheckpointPostsDaemonCheckpoint(t *testing.T) {
	var body map[string]any
	var taskHeader string
	var workspaceHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/daemon/tasks/task-1/bundle/checkpoint" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		taskHeader = r.Header.Get("X-Task-ID")
		workspaceHeader = r.Header.Get("X-Workspace-ID")
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"id":     "bundle-1",
			"status": "running",
			"items":  []map[string]any{},
		})
	}))
	defer srv.Close()

	t.Setenv("MULTICA_SERVER_URL", srv.URL)
	t.Setenv("MULTICA_WORKSPACE_ID", "ws-1")
	t.Setenv("MULTICA_TOKEN", "test-token")
	t.Setenv("MULTICA_TASK_ID", "task-1")

	cmd := freshTaskBundleCheckpointCmd()
	_ = cmd.Flags().Set("status", "completed")
	_ = cmd.Flags().Set("result", "finished first issue")
	_ = cmd.Flags().Set("checkpoint-seq", "7")
	if err := runTaskBundleCheckpoint(cmd, []string{"item-1"}); err != nil {
		t.Fatalf("runTaskBundleCheckpoint: %v", err)
	}

	if taskHeader != "task-1" {
		t.Fatalf("X-Task-ID = %q, want task-1", taskHeader)
	}
	if workspaceHeader != "ws-1" {
		t.Fatalf("X-Workspace-ID = %q, want ws-1", workspaceHeader)
	}
	if body["item_id"] != "item-1" || body["status"] != "completed" {
		t.Fatalf("body item/status = %#v", body)
	}
	if got := body["checkpoint_seq"]; got != float64(7) {
		t.Fatalf("checkpoint_seq = %#v, want 7", got)
	}
	result, ok := body["result"].(map[string]any)
	if !ok {
		t.Fatalf("result = %#v, want object", body["result"])
	}
	if result["summary"] != "finished first issue" {
		t.Fatalf("result.summary = %#v", result["summary"])
	}
}

func TestTaskBundleCheckpointRequiresTaskContext(t *testing.T) {
	t.Setenv("MULTICA_SERVER_URL", "http://127.0.0.1:0")
	t.Setenv("MULTICA_WORKSPACE_ID", "ws-1")
	t.Setenv("MULTICA_TOKEN", "test-token")
	t.Setenv("MULTICA_TASK_ID", "")

	cmd := freshTaskBundleCheckpointCmd()
	_ = cmd.Flags().Set("status", "completed")
	err := runTaskBundleCheckpoint(cmd, []string{"item-1"})
	if err == nil {
		t.Fatal("expected missing task context error")
	}
}
