package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func (d *Daemon) repositoryOperationLoop(ctx context.Context) {
	runtimeSetCh, unsub := d.runtimeSet.Subscribe()
	defer unsub()

	cancels := make(map[string]context.CancelFunc)
	defer func() {
		for _, cancel := range cancels {
			cancel()
		}
	}()

	syncPollers := func() {
		want := make(map[string]struct{})
		for _, rid := range d.allRuntimeIDs() {
			want[rid] = struct{}{}
		}
		for rid, cancel := range cancels {
			if _, ok := want[rid]; !ok {
				cancel()
				delete(cancels, rid)
			}
		}
		for rid := range want {
			if _, ok := cancels[rid]; ok {
				continue
			}
			pctx, pcancel := context.WithCancel(ctx)
			cancels[rid] = pcancel
			go d.runRuntimeRepositoryOperationPoller(pctx, rid)
		}
	}

	syncPollers()
	for {
		select {
		case <-ctx.Done():
			return
		case <-runtimeSetCh:
			syncPollers()
		}
	}
}

func (d *Daemon) runRuntimeRepositoryOperationPoller(ctx context.Context, runtimeID string) {
	interval := d.cfg.PollInterval
	if interval <= 0 {
		interval = DefaultPollInterval
	}

	for {
		if ctx.Err() != nil {
			return
		}

		op, err := d.client.ClaimRepositoryOperation(ctx, runtimeID)
		if err != nil {
			if ctx.Err() == nil {
				if isRuntimeNotFoundError(err) {
					go d.handleRuntimeGone(runtimeID)
					return
				}
				d.logger.Warn("claim repository operation failed", "runtime_id", runtimeID, "error", err)
			}
			if err := sleepWithContext(ctx, interval); err != nil {
				return
			}
			continue
		}

		if op == nil {
			if err := sleepWithContext(ctx, interval); err != nil {
				return
			}
			continue
		}

		if stop := d.runClaimedRepositoryOperation(ctx, runtimeID, op); stop {
			return
		}
	}
}

func (d *Daemon) runClaimedRepositoryOperation(ctx context.Context, runtimeID string, claimed *RepositoryOperation) bool {
	opID := shortID(claimed.ID)
	repoID := shortID(claimed.RepositoryID)
	opLog := d.logger.With("repository_operation", opID, "repository", repoID, "operation_type", claimed.OperationType)
	opLog.Info("repository operation received")

	op, err := d.client.StartRepositoryOperation(ctx, claimed.ID, runtimeID)
	if err != nil {
		if isRuntimeNotFoundError(err) {
			go d.handleRuntimeGone(runtimeID)
			return true
		}
		opLog.Warn("start repository operation failed", "error", err)
		return false
	}
	if op == nil {
		op = claimed
	}

	unlock := d.lockRepositoryOperation(op)
	defer unlock()

	body, failResult, execErr := d.repositoryOperationCompletionBody(op)
	if execErr != nil {
		stop, ok := d.reportRepositoryOperationFailure(ctx, runtimeID, op, execErr.Error(), failResult, opLog)
		if stop {
			return true
		}
		if ok {
			opLog.Info("repository operation failed", "reason", execErr.Error())
		}
		return false
	}

	if stop, ok := d.reportRepositoryOperationSuccess(ctx, runtimeID, op, body, opLog); stop || !ok {
		return stop
	}
	opLog.Info("repository operation completed")
	return false
}

func (d *Daemon) reportRepositoryOperationSuccess(ctx context.Context, runtimeID string, op *RepositoryOperation, body map[string]any, opLog *slog.Logger) (bool, bool) {
	for {
		if _, err := d.client.CompleteRepositoryOperation(ctx, op.ID, runtimeID, body); err != nil {
			if isRuntimeNotFoundError(err) {
				go d.handleRuntimeGone(runtimeID)
				return true, false
			}
			if repositoryOperationCallbackConflict(err) {
				opLog.Warn("complete repository operation callback found terminal operation", "error", err)
				return false, false
			}
			if repositoryOperationCallbackRejected(err) {
				opLog.Warn("complete repository operation callback rejected; reporting operation failure", "error", err)
				stop, _ := d.reportRepositoryOperationFailure(ctx, runtimeID, op, "repository operation completion was rejected by server", map[string]any{
					"status": "failed",
					"reason": "completion_rejected",
				}, opLog)
				return stop, false
			}
			opLog.Warn("complete repository operation callback failed; retrying", "error", err)
			if err := sleepWithContext(ctx, d.repositoryOperationCallbackRetryInterval()); err != nil {
				return true, false
			}
			continue
		}
		return false, true
	}
}

func (d *Daemon) reportRepositoryOperationFailure(ctx context.Context, runtimeID string, op *RepositoryOperation, errMsg string, result map[string]any, opLog *slog.Logger) (bool, bool) {
	for {
		if _, err := d.client.FailRepositoryOperation(ctx, op.ID, runtimeID, errMsg, result); err != nil {
			if isRuntimeNotFoundError(err) {
				go d.handleRuntimeGone(runtimeID)
				return true, false
			}
			if repositoryOperationCallbackConflict(err) {
				opLog.Warn("fail repository operation callback found terminal operation", "error", err)
				return false, false
			}
			if repositoryOperationCallbackRejected(err) {
				opLog.Warn("fail repository operation callback rejected", "error", err)
				return false, false
			}
			opLog.Warn("fail repository operation callback failed; retrying", "error", err)
			if err := sleepWithContext(ctx, d.repositoryOperationCallbackRetryInterval()); err != nil {
				return true, false
			}
			continue
		}
		return false, true
	}
}

func (d *Daemon) repositoryOperationCallbackRetryInterval() time.Duration {
	if d.cfg.PollInterval > 0 {
		return d.cfg.PollInterval
	}
	return DefaultPollInterval
}

func repositoryOperationCallbackConflict(err error) bool {
	reqErr, ok := err.(*requestError)
	return ok && reqErr.StatusCode == 409
}

func repositoryOperationCallbackRejected(err error) bool {
	reqErr, ok := err.(*requestError)
	return ok && reqErr.StatusCode >= 400 && reqErr.StatusCode < 500
}

func (d *Daemon) repositoryOperationCompletionBody(op *RepositoryOperation) (map[string]any, map[string]any, error) {
	switch op.OperationType {
	case "create_binding":
		return d.createBindingRepositoryOperationBody(op)
	case "init_git":
		return nil, unsupportedRepositoryOperationResult(op.OperationType, "binding_path_unavailable"),
			fmt.Errorf("repository operation init_git is unsupported: existing binding path is not available")
	case "refresh_binding":
		return nil, unsupportedRepositoryOperationResult(op.OperationType, "binding_path_unavailable"),
			fmt.Errorf("repository operation refresh_binding is unsupported: existing binding path is not available")
	case "publish_remote":
		return nil, unsupportedRepositoryOperationResult(op.OperationType, "remote_publish_unsupported"),
			fmt.Errorf("repository operation publish_remote is unsupported")
	default:
		return nil, unsupportedRepositoryOperationResult(op.OperationType, "unsupported_operation"),
			fmt.Errorf("unsupported repository operation type %q", op.OperationType)
	}
}

func (d *Daemon) createBindingRepositoryOperationBody(op *RepositoryOperation) (map[string]any, map[string]any, error) {
	localPath, err := createManagedRepositoryWorkdir(d.cfg.WorkspacesRoot, op.WorkspaceID, op.RepositoryID)
	if err != nil {
		reason := "create_managed_workdir_failed"
		if strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "required") {
			reason = "invalid_managed_workdir"
		}
		return nil, map[string]any{
			"status": "failed",
			"reason": reason,
		}, fmt.Errorf("repository operation create_binding failed: could not create managed workdir")
	}

	binding := map[string]any{
		"machine_label": d.repositoryOperationMachineLabel(),
		"binding_kind":  "daemon_workdir",
		"local_path":    localPath,
		"state":         "ready",
		"metadata":      map[string]any{},
	}
	if targetRuntimeID := stringPtrValue(op.TargetRuntimeID); targetRuntimeID != "" {
		binding["runtime_id"] = targetRuntimeID
	}

	return map[string]any{
		"result": map[string]any{
			"status":       "ready",
			"binding_kind": "daemon_workdir",
		},
		"binding": binding,
	}, nil, nil
}

func createManagedRepositoryWorkdir(workspacesRoot, workspaceID, repositoryID string) (string, error) {
	localPath, err := managedRepositoryWorkdir(workspacesRoot, workspaceID, repositoryID)
	if err != nil {
		return "", err
	}
	root, err := filepath.Abs(workspacesRoot)
	if err != nil {
		return "", fmt.Errorf("resolve workspaces root")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("create workspaces root")
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve workspaces root")
	}
	current := root
	for _, component := range []string{workspaceID, "repositories", repositoryID, "workdir"} {
		current = filepath.Join(current, component)
		if err := ensureManagedRepositoryDirectory(current); err != nil {
			return "", err
		}
	}
	if err := validateResolvedPathWithinRoot(root, current); err != nil {
		return "", err
	}
	return localPath, nil
}

func ensureManagedRepositoryDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.Mkdir(path, 0o755); err != nil {
				return fmt.Errorf("create managed repository directory")
			}
			info, err = os.Lstat(path)
		}
		if err != nil {
			return fmt.Errorf("inspect managed repository directory")
		}
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("managed repository directory contains symlink")
	}
	if !info.IsDir() {
		return fmt.Errorf("managed repository path is not a directory")
	}
	return nil
}

func validateResolvedPathWithinRoot(root, path string) error {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("resolve workspaces root")
	}
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return fmt.Errorf("resolve repository workdir")
	}
	rel, err := filepath.Rel(resolvedRoot, resolvedPath)
	if err != nil {
		return fmt.Errorf("validate repository workdir")
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." || filepath.IsAbs(rel) {
		return fmt.Errorf("repository workdir escapes workspaces root")
	}
	return nil
}

func (d *Daemon) repositoryOperationMachineLabel() string {
	if label := strings.TrimSpace(d.cfg.DeviceName); label != "" {
		return label
	}
	if id := strings.TrimSpace(d.cfg.DaemonID); id != "" {
		return id
	}
	return "Multica daemon"
}

func unsupportedRepositoryOperationResult(operationType, reason string) map[string]any {
	return map[string]any{
		"status":         "unsupported",
		"operation_type": operationType,
		"reason":         reason,
	}
}

func managedRepositoryWorkdir(workspacesRoot, workspaceID, repositoryID string) (string, error) {
	if strings.TrimSpace(workspacesRoot) == "" {
		return "", fmt.Errorf("workspaces root is required")
	}
	if !safeRepositoryPathComponent(workspaceID) || !safeRepositoryPathComponent(repositoryID) {
		return "", fmt.Errorf("invalid repository path component")
	}
	root, err := filepath.Abs(workspacesRoot)
	if err != nil {
		return "", fmt.Errorf("resolve workspaces root")
	}
	path := filepath.Join(root, workspaceID, "repositories", repositoryID, "workdir")
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve repository workdir")
	}
	rel, err := filepath.Rel(root, cleanPath)
	if err != nil {
		return "", fmt.Errorf("validate repository workdir")
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." || filepath.IsAbs(rel) {
		return "", fmt.Errorf("repository workdir escapes workspaces root")
	}
	return cleanPath, nil
}

func safeRepositoryPathComponent(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || value == "." || value == ".." {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}

func (d *Daemon) lockRepositoryOperation(op *RepositoryOperation) func() {
	key := repositoryOperationLockKey(op)
	lockValue, _ := d.repositoryOperationLocks.LoadOrStore(key, &sync.Mutex{})
	mu := lockValue.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

func repositoryOperationLockKey(op *RepositoryOperation) string {
	if op == nil {
		return "operation:"
	}
	if bindingID := stringPtrValue(op.BindingID); bindingID != "" {
		return "binding:" + bindingID
	}
	if repositoryID := strings.TrimSpace(op.RepositoryID); repositoryID != "" {
		return "repository:" + repositoryID
	}
	return "operation:" + strings.TrimSpace(op.ID)
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
