package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
		return d.initGitRepositoryOperationBody(op)
	case "refresh_binding":
		return nil, unsupportedRepositoryOperationResult(op.OperationType, "binding_path_unavailable"),
			fmt.Errorf("repository operation refresh_binding is unsupported: existing binding path is not available")
	case "publish_remote":
		return d.publishRemoteRepositoryOperationBody(op)
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

func (d *Daemon) initGitRepositoryOperationBody(op *RepositoryOperation) (map[string]any, map[string]any, error) {
	localPath, err := repositoryOperationBindingPath(op)
	if err != nil {
		return nil, unsupportedRepositoryOperationResult(op.OperationType, "binding_path_unavailable"), err
	}
	if err := validateRepositoryOperationLocalPath(localPath); err != nil {
		return nil, repositoryOperationFailureResult("binding_path_inaccessible"), fmt.Errorf("repository operation init_git failed: binding path is inaccessible")
	}

	if !repositoryOperationHasDotGit(localPath) {
		if err := initLocalGitRepository(localPath); err != nil {
			return nil, repositoryOperationFailureResult("git_init_failed"), fmt.Errorf("repository operation init_git failed: git init failed")
		}
	}

	summary, err := localGitSummary(localPath)
	if err != nil {
		return nil, repositoryOperationFailureResult("git_metadata_failed"), fmt.Errorf("repository operation init_git failed: git metadata unavailable")
	}
	result := summary.sanitizedResult("ready")
	repository := map[string]any{
		"metadata": summary.sanitizedMetadata(),
	}
	if summary.DefaultBranch != "" {
		repository["default_branch"] = summary.DefaultBranch
	}
	return map[string]any{
		"result":     result,
		"repository": repository,
	}, nil, nil
}

func (d *Daemon) publishRemoteRepositoryOperationBody(op *RepositoryOperation) (map[string]any, map[string]any, error) {
	localPath, err := repositoryOperationBindingPath(op)
	if err != nil {
		return nil, unsupportedRepositoryOperationResult(op.OperationType, "binding_path_unavailable"), err
	}
	if err := validateRepositoryOperationLocalPath(localPath); err != nil {
		return nil, repositoryOperationFailureResult("binding_path_inaccessible"), fmt.Errorf("repository operation publish_remote failed: binding path is inaccessible")
	}
	remoteURL, err := repositoryOperationRequestRemoteURL(op)
	if err != nil {
		return nil, repositoryOperationFailureResult("remote_url_required"), err
	}
	if !repositoryOperationGitHasCommits(localPath) {
		return nil, map[string]any{
			"status":      "failed",
			"reason":      "no_commits_to_push",
			"recoverable": true,
		}, fmt.Errorf("repository operation publish_remote failed: no commits to push")
	}
	summary, err := localGitSummary(localPath)
	if err != nil || summary.DefaultBranch == "" {
		return nil, repositoryOperationFailureResult("git_metadata_failed"), fmt.Errorf("repository operation publish_remote failed: git metadata unavailable")
	}
	if err := setRepositoryOperationOrigin(localPath, remoteURL); err != nil {
		return nil, repositoryOperationFailureResult("git_remote_failed"), fmt.Errorf("repository operation publish_remote failed: git remote update failed")
	}
	if _, err := runRepositoryOperationGit(localPath, "push", "-u", "origin", summary.DefaultBranch); err != nil {
		return nil, repositoryOperationFailureResult("git_push_failed"), fmt.Errorf("repository operation publish_remote failed: git push failed")
	}
	summary, err = localGitSummary(localPath)
	if err != nil {
		return nil, repositoryOperationFailureResult("git_metadata_failed"), fmt.Errorf("repository operation publish_remote failed: git metadata unavailable")
	}
	result := summary.sanitizedResult("published")
	result["remote_url"] = remoteURL
	repository := map[string]any{
		"remote_url": remoteURL,
		"metadata":   summary.sanitizedMetadata(),
	}
	if summary.DefaultBranch != "" {
		repository["default_branch"] = summary.DefaultBranch
	}
	return map[string]any{
		"result":     result,
		"repository": repository,
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

func repositoryOperationFailureResult(reason string) map[string]any {
	return map[string]any{
		"status": "failed",
		"reason": reason,
	}
}

func repositoryOperationBindingPath(op *RepositoryOperation) (string, error) {
	if op == nil || op.Binding == nil || strings.TrimSpace(op.Binding.LocalPath) == "" {
		return "", fmt.Errorf("repository operation %s is unsupported: existing binding path is not available", stringPtrValue(operationTypePtr(op)))
	}
	if op.BindingID != nil && strings.TrimSpace(*op.BindingID) != "" && strings.TrimSpace(op.Binding.ID) != strings.TrimSpace(*op.BindingID) {
		return "", fmt.Errorf("repository operation %s is unsupported: binding payload does not match target binding", stringPtrValue(operationTypePtr(op)))
	}
	if strings.TrimSpace(op.Binding.State) != "ready" {
		return "", fmt.Errorf("repository operation %s is unsupported: binding is not ready", stringPtrValue(operationTypePtr(op)))
	}
	return strings.TrimSpace(op.Binding.LocalPath), nil
}

func operationTypePtr(op *RepositoryOperation) *string {
	if op == nil {
		return nil
	}
	return &op.OperationType
}

func validateRepositoryOperationLocalPath(localPath string) error {
	info, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("binding path is inaccessible")
	}
	if !info.IsDir() {
		return fmt.Errorf("binding path is not a directory")
	}
	return nil
}

func repositoryOperationHasDotGit(localPath string) bool {
	_, err := os.Stat(filepath.Join(localPath, ".git"))
	return err == nil
}

func initLocalGitRepository(localPath string) error {
	out, err := runRepositoryOperationGit(localPath, "init", "-b", "main")
	if err == nil {
		return nil
	}
	if !strings.Contains(strings.ToLower(string(out)), "unknown switch") && !strings.Contains(strings.ToLower(string(out)), "unknown option") {
		return err
	}
	if _, err := runRepositoryOperationGit(localPath, "init"); err != nil {
		return err
	}
	_, err = runRepositoryOperationGit(localPath, "symbolic-ref", "HEAD", "refs/heads/main")
	return err
}

type repositoryOperationGitSummary struct {
	DefaultBranch         string
	HeadCommit            string
	HasCommits            bool
	HasUncommittedChanges bool
	UntrackedCount        int
}

func localGitSummary(localPath string) (repositoryOperationGitSummary, error) {
	branch, err := repositoryOperationGitBranch(localPath)
	if err != nil {
		return repositoryOperationGitSummary{}, err
	}
	head, hasCommits := repositoryOperationGitHead(localPath)
	statusOut, err := runRepositoryOperationGit(localPath, "status", "--porcelain=v1", "--untracked-files=normal")
	if err != nil {
		return repositoryOperationGitSummary{}, err
	}
	statusLines := nonEmptyLines(string(statusOut))
	return repositoryOperationGitSummary{
		DefaultBranch:         branch,
		HeadCommit:            head,
		HasCommits:            hasCommits,
		HasUncommittedChanges: len(statusLines) > 0,
		UntrackedCount:        countUntrackedStatusLines(statusLines),
	}, nil
}

func (s repositoryOperationGitSummary) sanitizedResult(status string) map[string]any {
	result := map[string]any{
		"status":                  status,
		"has_commits":             s.HasCommits,
		"has_uncommitted_changes": s.HasUncommittedChanges,
		"untracked_count":         s.UntrackedCount,
	}
	if s.DefaultBranch != "" {
		result["default_branch"] = s.DefaultBranch
	}
	if s.HeadCommit != "" {
		result["head_commit"] = s.HeadCommit
	}
	return result
}

func (s repositoryOperationGitSummary) sanitizedMetadata() map[string]any {
	metadata := map[string]any{
		"has_commits":             s.HasCommits,
		"has_uncommitted_changes": s.HasUncommittedChanges,
		"untracked_count":         s.UntrackedCount,
	}
	if s.HeadCommit != "" {
		metadata["head_commit"] = s.HeadCommit
	}
	return metadata
}

func repositoryOperationGitBranch(localPath string) (string, error) {
	out, err := runRepositoryOperationGit(localPath, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return "", fmt.Errorf("git branch unavailable")
	}
	return strings.TrimSpace(string(out)), nil
}

func repositoryOperationGitHead(localPath string) (string, bool) {
	out, err := runRepositoryOperationGit(localPath, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

func repositoryOperationGitHasCommits(localPath string) bool {
	_, hasCommits := repositoryOperationGitHead(localPath)
	return hasCommits
}

func nonEmptyLines(value string) []string {
	raw := strings.Split(value, "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func countUntrackedStatusLines(lines []string) int {
	count := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "?? ") {
			count++
		}
	}
	return count
}

func repositoryOperationRequestRemoteURL(op *RepositoryOperation) (string, error) {
	var req struct {
		RemoteURL string `json:"remote_url"`
	}
	if op == nil || len(op.Request) == 0 {
		return "", fmt.Errorf("repository operation publish_remote failed: request.remote_url is required")
	}
	if err := json.Unmarshal(op.Request, &req); err != nil {
		return "", fmt.Errorf("repository operation publish_remote failed: request is invalid")
	}
	remoteURL := strings.TrimSpace(req.RemoteURL)
	if remoteURL == "" {
		return "", fmt.Errorf("repository operation publish_remote failed: request.remote_url is required")
	}
	return remoteURL, nil
}

func setRepositoryOperationOrigin(localPath, remoteURL string) error {
	if _, err := runRepositoryOperationGit(localPath, "remote", "get-url", "origin"); err == nil {
		_, err = runRepositoryOperationGit(localPath, "remote", "set-url", "origin", remoteURL)
		return err
	}
	_, err := runRepositoryOperationGit(localPath, "remote", "add", "origin", remoteURL)
	return err
}

func runRepositoryOperationGit(localPath string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", localPath}, args...)...)
	cmd.Env = repositoryOperationGitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		name := "git"
		if len(args) > 0 {
			name += " " + args[0]
		}
		return out, fmt.Errorf("%s failed", name)
	}
	return out, nil
}

func repositoryOperationGitEnv() []string {
	base := os.Environ()
	existing := 0
	for _, e := range base {
		if strings.HasPrefix(e, "GIT_CONFIG_COUNT=") {
			if n, err := strconv.Atoi(strings.TrimPrefix(e, "GIT_CONFIG_COUNT=")); err == nil {
				existing = n
			}
		}
	}
	idx := strconv.Itoa(existing)
	return append(base,
		"GIT_TERMINAL_PROMPT=0",
		"GIT_CONFIG_COUNT="+strconv.Itoa(existing+1),
		"GIT_CONFIG_KEY_"+idx+"=safe.directory",
		"GIT_CONFIG_VALUE_"+idx+"=*",
	)
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
