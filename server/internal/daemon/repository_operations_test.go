package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRepositoryOperationPollerStartsAndCompletesCreateBinding(t *testing.T) {
	root := t.TempDir()
	runtimeID := "runtime-1"
	targetRuntimeID := runtimeID
	op := RepositoryOperation{
		ID:              "operation-1",
		WorkspaceID:     "workspace-1",
		RepositoryID:    "repository-1",
		OperationType:   "create_binding",
		Status:          "running",
		TargetRuntimeID: &targetRuntimeID,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	var calls []string
	var completeBody map[string]any
	claimReturned := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mu.Lock()
		defer mu.Unlock()

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/daemon/repository-operations/claim":
			calls = append(calls, "claim")
			if r.URL.Query().Get("runtime_id") != runtimeID {
				t.Errorf("claim runtime_id = %q, want %q", r.URL.Query().Get("runtime_id"), runtimeID)
			}
			if claimReturned {
				_ = json.NewEncoder(w).Encode(map[string]any{"operation": nil})
				return
			}
			claimReturned = true
			_ = json.NewEncoder(w).Encode(map[string]any{"operation": op})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/start"):
			calls = append(calls, "start")
			if r.URL.Query().Get("runtime_id") != runtimeID {
				t.Errorf("start runtime_id = %q, want %q", r.URL.Query().Get("runtime_id"), runtimeID)
			}
			_ = json.NewEncoder(w).Encode(op)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/complete"):
			calls = append(calls, "complete")
			if err := json.NewDecoder(r.Body).Decode(&completeBody); err != nil {
				t.Errorf("decode complete body: %v", err)
			}
			op.Status = "succeeded"
			_ = json.NewEncoder(w).Encode(op)
			cancel()
		default:
			body, _ := io.ReadAll(r.Body)
			t.Errorf("unexpected request %s %s body=%s", r.Method, r.URL.String(), strings.TrimSpace(string(body)))
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	d := &Daemon{
		cfg: Config{
			WorkspacesRoot: root,
			DaemonID:       "daemon-1",
			DeviceName:     "Test Mac",
			PollInterval:   time.Hour,
		},
		client: NewClient(srv.URL),
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	done := make(chan struct{})
	go func() {
		d.runRuntimeRepositoryOperationPoller(ctx, runtimeID)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("repository operation poller did not stop after completion")
	}

	mu.Lock()
	gotCalls := strings.Join(calls, ",")
	body := completeBody
	mu.Unlock()
	if gotCalls != "claim,start,complete" {
		t.Fatalf("calls = %s, want claim,start,complete", gotCalls)
	}

	binding, ok := body["binding"].(map[string]any)
	if !ok {
		t.Fatalf("complete body missing binding: %+v", body)
	}
	localPath, _ := binding["local_path"].(string)
	if localPath == "" {
		t.Fatalf("complete binding missing local_path: %+v", binding)
	}
	wantPath, err := managedRepositoryWorkdir(root, op.WorkspaceID, op.RepositoryID)
	if err != nil {
		t.Fatalf("managedRepositoryWorkdir: %v", err)
	}
	if localPath != wantPath {
		t.Fatalf("local_path = %q, want %q", localPath, wantPath)
	}
	if info, err := os.Stat(localPath); err != nil || !info.IsDir() {
		t.Fatalf("managed workdir was not created: info=%+v err=%v", info, err)
	}
	if resultText := fmt.Sprint(body["result"]); containsAnyPathFragment(resultText, root, localPath) {
		t.Fatalf("completion result leaked local path: %s", resultText)
	}
}

func TestRepositoryOperationPollerRetriesCompleteCallback(t *testing.T) {
	root := t.TempDir()
	runtimeID := "runtime-1"
	op := RepositoryOperation{
		ID:            "operation-1",
		WorkspaceID:   "workspace-1",
		RepositoryID:  "repository-1",
		OperationType: "create_binding",
		Status:        "running",
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	completeAttempts := 0
	claimReturned := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mu.Lock()
		defer mu.Unlock()

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/daemon/repository-operations/claim":
			if claimReturned {
				_ = json.NewEncoder(w).Encode(map[string]any{"operation": nil})
				return
			}
			claimReturned = true
			_ = json.NewEncoder(w).Encode(map[string]any{"operation": op})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/start"):
			_ = json.NewEncoder(w).Encode(op)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/complete"):
			completeAttempts++
			if completeAttempts == 1 {
				http.Error(w, "temporary complete failure", http.StatusInternalServerError)
				return
			}
			op.Status = "succeeded"
			_ = json.NewEncoder(w).Encode(op)
			cancel()
		default:
			body, _ := io.ReadAll(r.Body)
			t.Errorf("unexpected request %s %s body=%s", r.Method, r.URL.String(), strings.TrimSpace(string(body)))
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	d := &Daemon{
		cfg: Config{
			WorkspacesRoot: root,
			PollInterval:   5 * time.Millisecond,
		},
		client: NewClient(srv.URL),
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	done := make(chan struct{})
	go func() {
		d.runRuntimeRepositoryOperationPoller(ctx, runtimeID)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("repository operation poller did not retry complete callback")
	}

	mu.Lock()
	gotAttempts := completeAttempts
	mu.Unlock()
	if gotAttempts != 2 {
		t.Fatalf("complete attempts = %d, want 2", gotAttempts)
	}
}

func TestRepositoryOperationCreateBindingCreatesDaemonWorkdir(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runtimeID := "runtime-1"
	d := &Daemon{
		cfg: Config{
			WorkspacesRoot: root,
			DaemonID:       "daemon-1",
			DeviceName:     "Test Mac",
		},
	}
	op := &RepositoryOperation{
		ID:              "operation-1",
		WorkspaceID:     "workspace-1",
		RepositoryID:    "repository-1",
		OperationType:   "create_binding",
		TargetRuntimeID: &runtimeID,
	}

	body, failResult, err := d.repositoryOperationCompletionBody(op)
	if err != nil {
		t.Fatalf("repositoryOperationCompletionBody: %v", err)
	}
	if failResult != nil {
		t.Fatalf("unexpected fail result: %+v", failResult)
	}

	wantPath, err := managedRepositoryWorkdir(root, op.WorkspaceID, op.RepositoryID)
	if err != nil {
		t.Fatalf("managedRepositoryWorkdir: %v", err)
	}
	info, err := os.Stat(wantPath)
	if err != nil {
		t.Fatalf("expected managed workdir to exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("managed workdir is not a directory: %s", wantPath)
	}

	binding, ok := body["binding"].(map[string]any)
	if !ok {
		t.Fatalf("completion body missing binding: %+v", body)
	}
	if got := binding["binding_kind"]; got != "daemon_workdir" {
		t.Fatalf("binding_kind = %v, want daemon_workdir", got)
	}
	if got := binding["state"]; got != "ready" {
		t.Fatalf("state = %v, want ready", got)
	}
	if got := binding["machine_label"]; got != "Test Mac" {
		t.Fatalf("machine_label = %v, want Test Mac", got)
	}
	if got := binding["runtime_id"]; got != runtimeID {
		t.Fatalf("runtime_id = %v, want %s", got, runtimeID)
	}
	if got := binding["local_path"]; got != wantPath {
		t.Fatalf("local_path = %v, want daemon-owned path %s", got, wantPath)
	}
	if metadata, ok := binding["metadata"].(map[string]any); !ok || len(metadata) != 0 {
		t.Fatalf("binding metadata = %+v, want empty sanitized object", binding["metadata"])
	}

	result, ok := body["result"].(map[string]any)
	if !ok {
		t.Fatalf("completion body missing result: %+v", body)
	}
	if got := result["binding_kind"]; got != "daemon_workdir" {
		t.Fatalf("result binding_kind = %v, want daemon_workdir", got)
	}
	if fmt.Sprint(result) == "" || containsAnyPathFragment(fmt.Sprint(result), root, wantPath) {
		t.Fatalf("result metadata leaked a local path: %+v", result)
	}
}

func TestRepositoryOperationCreateBindingRejectsSymlinkEscape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "workspace-1"), 0o755); err != nil {
		t.Fatalf("create workspace dir: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "workspace-1", "repositories")); err != nil {
		t.Skipf("symlink unavailable on this platform: %v", err)
	}

	d := &Daemon{cfg: Config{WorkspacesRoot: root}}
	op := &RepositoryOperation{
		ID:            "operation-1",
		WorkspaceID:   "workspace-1",
		RepositoryID:  "repository-1",
		OperationType: "create_binding",
	}

	body, failResult, err := d.repositoryOperationCompletionBody(op)
	if err == nil {
		t.Fatal("expected create_binding to reject symlinked managed workdir")
	}
	if body != nil {
		t.Fatalf("completion body = %+v, want nil", body)
	}
	if got := failResult["reason"]; got != "create_managed_workdir_failed" {
		t.Fatalf("fail reason = %v, want create_managed_workdir_failed", got)
	}
	if _, err := os.Stat(filepath.Join(outside, "repository-1")); !os.IsNotExist(err) {
		t.Fatalf("managed workdir escaped through symlink: err=%v", err)
	}
}

func TestRepositoryOperationLockSerializesSameRepository(t *testing.T) {
	t.Parallel()

	d := &Daemon{}
	first := &RepositoryOperation{ID: "operation-1", RepositoryID: "repository-1"}
	second := &RepositoryOperation{ID: "operation-2", RepositoryID: "repository-1"}

	unlockFirst := d.lockRepositoryOperation(first)
	acquired := make(chan struct{})
	releaseSecond := make(chan struct{})
	go func() {
		unlockSecond := d.lockRepositoryOperation(second)
		close(acquired)
		<-releaseSecond
		unlockSecond()
	}()

	select {
	case <-acquired:
		t.Fatal("second operation acquired the repository lock while first held it")
	case <-time.After(25 * time.Millisecond):
	}

	unlockFirst()
	select {
	case <-acquired:
		close(releaseSecond)
	case <-time.After(time.Second):
		t.Fatal("second operation did not acquire repository lock after first released it")
	}
}

func TestRepositoryOperationUnsupportedOperationsFailExplicitly(t *testing.T) {
	t.Parallel()

	d := &Daemon{cfg: Config{WorkspacesRoot: t.TempDir()}}
	bindingID := "binding-1"
	for _, operationType := range []string{"init_git", "refresh_binding", "publish_remote"} {
		t.Run(operationType, func(t *testing.T) {
			op := &RepositoryOperation{
				ID:            "operation-" + operationType,
				WorkspaceID:   "workspace-1",
				RepositoryID:  "repository-1",
				OperationType: operationType,
				BindingID:     &bindingID,
			}
			body, failResult, err := d.repositoryOperationCompletionBody(op)
			if err == nil {
				t.Fatal("expected unsupported operation error")
			}
			if body != nil {
				t.Fatalf("completion body = %+v, want nil", body)
			}
			if got := failResult["status"]; got != "unsupported" {
				t.Fatalf("fail status = %v, want unsupported", got)
			}
			if got := failResult["operation_type"]; got != operationType {
				t.Fatalf("operation_type = %v, want %s", got, operationType)
			}
			if containsAnyPathFragment(err.Error(), d.cfg.WorkspacesRoot) || containsAnyPathFragment(fmt.Sprint(failResult), d.cfg.WorkspacesRoot) {
				t.Fatalf("unsupported failure leaked a local path: err=%q result=%+v", err.Error(), failResult)
			}
		})
	}
}

func containsAnyPathFragment(value string, fragments ...string) bool {
	for _, fragment := range fragments {
		if fragment != "" && strings.Contains(value, fragment) {
			return true
		}
	}
	return false
}
