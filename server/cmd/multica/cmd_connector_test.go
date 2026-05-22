package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/connectors"
)

func TestRunConnectorGitLabBranchesPostsTaskScopedAction(t *testing.T) {
	resetRuntimeEnvForTest(t)
	var gotBody connectorActionRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/connectors/actions" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-Workspace-ID"); got != "ws-1" {
			t.Fatalf("X-Workspace-ID = %q, want ws-1", got)
		}
		if got := r.Header.Get("X-Agent-ID"); got != "agent-1" {
			t.Fatalf("X-Agent-ID = %q, want agent-1", got)
		}
		if got := r.Header.Get("X-Task-ID"); got != "task-1" {
			t.Fatalf("X-Task-ID = %q, want task-1", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("Authorization = %q, want bearer token", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"provider_id":         connectors.ProviderRingCentralGitLab,
			"capability":          "branch.list",
			"project_resource_id": "resource-1",
			"result":              []map[string]any{},
		})
	}))
	defer srv.Close()

	t.Setenv("MULTICA_AGENT_ID", "agent-1")
	t.Setenv("MULTICA_TASK_ID", "task-1")
	t.Setenv("MULTICA_TOKEN", "test-token")

	cmd := newConnectorTestCommand()
	if err := cmd.Flags().Set("server-url", srv.URL); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("workspace-id", "ws-1"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("search", "release"); err != nil {
		t.Fatal(err)
	}

	output := captureStdout(t, func() error {
		return runConnectorGitLabBranches(cmd, []string{"resource-1"})
	})
	if output == "" {
		t.Fatalf("expected JSON output")
	}
	if gotBody.ProviderID != connectors.ProviderRingCentralGitLab {
		t.Fatalf("provider_id = %q", gotBody.ProviderID)
	}
	if gotBody.Capability != "branch.list" {
		t.Fatalf("capability = %q", gotBody.Capability)
	}
	if gotBody.ProjectResourceID != "resource-1" {
		t.Fatalf("project_resource_id = %q", gotBody.ProjectResourceID)
	}
	if gotBody.Input["search"] != "release" {
		t.Fatalf("search input = %#v", gotBody.Input["search"])
	}
}

func TestPostConnectorActionRequiresDaemonWorkspaceAndTaskIdentity(t *testing.T) {
	resetRuntimeEnvForTest(t)
	t.Setenv(runtimeEnvFileEnv, "")
	t.Setenv("MULTICA_AGENT_ID", "")
	t.Setenv("MULTICA_TASK_ID", "")
	cmd := newConnectorTestCommand()
	if err := cmd.Flags().Set("server-url", "http://example.invalid"); err != nil {
		t.Fatal(err)
	}
	if err := postConnectorAction(cmd, connectorActionRequest{}); err == nil {
		t.Fatalf("expected missing workspace identity error")
	}

	if err := cmd.Flags().Set("workspace-id", "ws-1"); err != nil {
		t.Fatal(err)
	}
	if err := postConnectorAction(cmd, connectorActionRequest{}); err == nil {
		t.Fatalf("expected missing task identity error")
	}
}

func newConnectorTestCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "connector-test"}
	cmd.Flags().String("server-url", "", "")
	cmd.Flags().String("workspace-id", "", "")
	cmd.Flags().String("profile", "", "")
	cmd.Flags().String("search", "", "")
	return cmd
}

func captureStdout(t *testing.T, fn func() error) string {
	t.Helper()
	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = writer
	err = fn()
	_ = writer.Close()
	os.Stdout = oldStdout
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	data, readErr := io.ReadAll(reader)
	if readErr != nil {
		t.Fatalf("read stdout: %v", readErr)
	}
	return string(data)
}
