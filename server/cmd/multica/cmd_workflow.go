package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/cli"
)

var workflowCmd = &cobra.Command{
	Use:   "workflow",
	Short: "Inspect and control workflow runs",
}

var workflowRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Inspect and control workflow runs",
}

var workflowStepCmd = &cobra.Command{
	Use:   "step",
	Short: "Control workflow step runs",
}

var workflowArtifactCmd = &cobra.Command{
	Use:   "artifact",
	Short: "Persist workflow artifacts",
}

var workflowInputCmd = &cobra.Command{
	Use:   "input",
	Short: "Ask for and answer workflow-scoped human input",
}

var workflowQualityCmd = &cobra.Command{
	Use:   "quality",
	Short: "Report workflow quality gate results",
}

var workflowReviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Approve or reject workflow reviews",
}

var workflowRunListCmd = &cobra.Command{
	Use:   "list",
	Short: "List workflow runs by issue or task",
	RunE:  runWorkflowRunList,
}

var workflowRunGetCmd = &cobra.Command{
	Use:   "get <run-id>",
	Short: "Get a workflow run",
	Args:  exactArgs(1),
	RunE:  runWorkflowRunGet,
}

var workflowRunCancelCmd = &cobra.Command{
	Use:   "cancel <run-id>",
	Short: "Cancel a workflow run's backing task",
	Args:  exactArgs(1),
	RunE:  runWorkflowRunCancel,
}

var workflowRunRerunCmd = &cobra.Command{
	Use:   "rerun <run-id>",
	Short: "Rerun a workflow by creating a fresh task and run",
	Args:  exactArgs(1),
	RunE:  runWorkflowRunRerun,
}

var workflowStepGetCmd = &cobra.Command{
	Use:   "get <step-run-id>",
	Short: "Get a workflow step run",
	Args:  exactArgs(1),
	RunE:  runWorkflowStepGet,
}

var workflowStepStartCmd = &cobra.Command{
	Use:   "start <step-run-id>",
	Short: "Mark a workflow step run as running",
	Args:  exactArgs(1),
	RunE:  runWorkflowStepStart,
}

var workflowStepCompleteCmd = &cobra.Command{
	Use:   "complete <step-run-id>",
	Short: "Complete a workflow step run",
	Args:  exactArgs(1),
	RunE:  runWorkflowStepComplete,
}

var workflowStepManualCompleteCmd = &cobra.Command{
	Use:   "manual-complete <step-run-id>",
	Short: "Complete a manual workflow step run",
	Args:  exactArgs(1),
	RunE:  runWorkflowStepManualComplete,
}

var workflowStepFailCmd = &cobra.Command{
	Use:   "fail <step-run-id>",
	Short: "Fail a workflow step run",
	Args:  exactArgs(1),
	RunE:  runWorkflowStepFail,
}

var workflowStepPauseCmd = &cobra.Command{
	Use:   "pause <step-run-id>",
	Short: "Pause a workflow step run",
	Args:  exactArgs(1),
	RunE:  runWorkflowStepPause,
}

var workflowStepRetryCmd = &cobra.Command{
	Use:   "retry <step-run-id>",
	Short: "Retry a workflow step run inside the same workflow run",
	Args:  exactArgs(1),
	RunE:  runWorkflowStepRetry,
}

var workflowStepSkipCmd = &cobra.Command{
	Use:   "skip <step-run-id>",
	Short: "Skip a workflow step run",
	Args:  exactArgs(1),
	RunE:  runWorkflowStepSkip,
}

var workflowArtifactSaveCmd = &cobra.Command{
	Use:   "save <step-run-id>",
	Short: "Save a versioned workflow artifact",
	Args:  exactArgs(1),
	RunE:  runWorkflowArtifactSave,
}

var workflowArtifactDiffCmd = &cobra.Command{
	Use:   "diff <artifact-id>",
	Short: "Show a diff between two complete workflow artifact versions",
	Args:  exactArgs(1),
	RunE:  runWorkflowArtifactDiff,
}

var workflowInputRequestCmd = &cobra.Command{
	Use:   "request <step-run-id>",
	Short: "Ask for bounded human input and suspend the backing task",
	Args:  exactArgs(1),
	RunE:  runWorkflowInputRequest,
}

var workflowInputAnswerCmd = &cobra.Command{
	Use:   "answer <input-request-id>",
	Short: "Answer a workflow input request and resume the backing task",
	Args:  exactArgs(1),
	RunE:  runWorkflowInputAnswer,
}

var workflowInputCancelCmd = &cobra.Command{
	Use:   "cancel <input-request-id>",
	Short: "Cancel a workflow input request",
	Args:  exactArgs(1),
	RunE:  runWorkflowInputCancel,
}

var workflowQualityReportCmd = &cobra.Command{
	Use:   "report <step-run-id>",
	Short: "Report a workflow quality gate result",
	Args:  exactArgs(1),
	RunE:  runWorkflowQualityReport,
}

var workflowReviewApproveCmd = &cobra.Command{
	Use:   "approve <review-id>",
	Short: "Approve a workflow review",
	Args:  exactArgs(1),
	RunE:  runWorkflowReviewApprove,
}

var workflowReviewRejectCmd = &cobra.Command{
	Use:   "reject <review-id>",
	Short: "Reject a workflow review",
	Args:  exactArgs(1),
	RunE:  runWorkflowReviewReject,
}

func init() {
	workflowCmd.AddCommand(workflowRunCmd)
	workflowCmd.AddCommand(workflowStepCmd)
	workflowCmd.AddCommand(workflowArtifactCmd)
	workflowCmd.AddCommand(workflowInputCmd)
	workflowCmd.AddCommand(workflowQualityCmd)
	workflowCmd.AddCommand(workflowReviewCmd)

	workflowRunCmd.AddCommand(workflowRunListCmd)
	workflowRunCmd.AddCommand(workflowRunGetCmd)
	workflowRunCmd.AddCommand(workflowRunCancelCmd)
	workflowRunCmd.AddCommand(workflowRunRerunCmd)

	workflowStepCmd.AddCommand(workflowStepGetCmd)
	workflowStepCmd.AddCommand(workflowStepStartCmd)
	workflowStepCmd.AddCommand(workflowStepCompleteCmd)
	workflowStepCmd.AddCommand(workflowStepManualCompleteCmd)
	workflowStepCmd.AddCommand(workflowStepFailCmd)
	workflowStepCmd.AddCommand(workflowStepPauseCmd)
	workflowStepCmd.AddCommand(workflowStepRetryCmd)
	workflowStepCmd.AddCommand(workflowStepSkipCmd)

	workflowArtifactCmd.AddCommand(workflowArtifactSaveCmd)
	workflowArtifactCmd.AddCommand(workflowArtifactDiffCmd)
	workflowInputCmd.AddCommand(workflowInputRequestCmd)
	workflowInputCmd.AddCommand(workflowInputAnswerCmd)
	workflowInputCmd.AddCommand(workflowInputCancelCmd)
	workflowQualityCmd.AddCommand(workflowQualityReportCmd)
	workflowReviewCmd.AddCommand(workflowReviewApproveCmd)
	workflowReviewCmd.AddCommand(workflowReviewRejectCmd)

	workflowRunListCmd.Flags().String("issue", "", "Issue ID or key to list workflow runs for")
	workflowRunListCmd.Flags().String("task", "", "Task ID to list workflow runs for")
	workflowRunListCmd.Flags().String("output", "table", "Output format: table or json")
	workflowRunListCmd.Flags().Bool("full-id", false, "Show full UUIDs in table output")
	workflowRunGetCmd.Flags().String("output", "json", "Output format: json or table")
	workflowRunGetCmd.Flags().Bool("full-id", false, "Show full UUIDs in table output")
	workflowRunCancelCmd.Flags().String("output", "json", "Output format: json or table")
	workflowRunRerunCmd.Flags().String("output", "json", "Output format: json or table")

	for _, cmd := range []*cobra.Command{
		workflowStepGetCmd,
		workflowStepStartCmd,
		workflowStepCompleteCmd,
		workflowStepManualCompleteCmd,
		workflowStepFailCmd,
		workflowStepPauseCmd,
		workflowStepRetryCmd,
		workflowStepSkipCmd,
	} {
		cmd.Flags().String("output", "json", "Output format: json or table")
		cmd.Flags().Bool("full-id", false, "Show full UUIDs in table output")
	}
	workflowStepFailCmd.Flags().String("reason", "", "Failure reason")
	workflowStepPauseCmd.Flags().String("reason", "", "Pause reason")

	workflowArtifactSaveCmd.Flags().String("name", "", "Logical artifact name (required)")
	workflowArtifactSaveCmd.Flags().String("file", "-", "Artifact content file path, or - for stdin")
	workflowArtifactSaveCmd.Flags().String("format", "markdown", "Artifact content format: markdown, json, or text")
	workflowArtifactSaveCmd.Flags().String("output", "json", "Output format: json or table")
	workflowArtifactDiffCmd.Flags().Int32("base-version", 0, "Base artifact version (required)")
	workflowArtifactDiffCmd.Flags().Int32("target-version", 0, "Target artifact version (required)")
	workflowArtifactDiffCmd.Flags().String("output", "diff", "Output format: diff or json")

	workflowInputRequestCmd.Flags().String("question-file", "-", "Question content file path, or - for stdin")
	workflowInputRequestCmd.Flags().Int32("max-rounds", 0, "Maximum rounds allowed by this request")
	workflowInputRequestCmd.Flags().String("output", "json", "Output format: json or table")
	workflowInputAnswerCmd.Flags().String("file", "-", "Answer content file path, or - for stdin")
	workflowInputAnswerCmd.Flags().Bool("no-continue", false, "Record the answer without requeueing the backing task")
	workflowInputAnswerCmd.Flags().String("output", "json", "Output format: json or table")
	workflowInputCancelCmd.Flags().String("output", "json", "Output format: json or table")

	workflowQualityReportCmd.Flags().String("artifact", "", "Artifact ID this quality result evaluates")
	workflowQualityReportCmd.Flags().String("status", "", "Quality status: pass, fail, or warning (required)")
	workflowQualityReportCmd.Flags().Bool("blocking", false, "Block the workflow step/run when status is fail")
	workflowQualityReportCmd.Flags().String("file", "", "Report content file path, or - for stdin")
	workflowQualityReportCmd.Flags().String("format", "markdown", "Report content format: markdown, json, or text")
	workflowQualityReportCmd.Flags().String("output", "json", "Output format: json or table")

	workflowReviewApproveCmd.Flags().String("notes", "", "Review notes")
	workflowReviewApproveCmd.Flags().String("output", "json", "Output format: json or table")
	workflowReviewRejectCmd.Flags().String("notes", "", "Review notes")
	workflowReviewRejectCmd.Flags().String("output", "json", "Output format: json or table")
}

func runWorkflowRunList(cmd *cobra.Command, _ []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	if _, err := requireWorkspaceID(cmd); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	issue, _ := cmd.Flags().GetString("issue")
	taskID, _ := cmd.Flags().GetString("task")
	if issue == "" && taskID == "" {
		return fmt.Errorf("--issue or --task is required")
	}
	if issue != "" && taskID != "" {
		return fmt.Errorf("--issue and --task are mutually exclusive")
	}

	values := url.Values{}
	if issue != "" {
		issueRef, err := resolveIssueRef(ctx, client, issue)
		if err != nil {
			return fmt.Errorf("resolve issue: %w", err)
		}
		values.Set("issue_id", issueRef.ID)
	} else {
		values.Set("task_id", taskID)
	}

	var runs []map[string]any
	if err := client.GetJSON(ctx, "/api/workflow-runs?"+values.Encode(), &runs); err != nil {
		return fmt.Errorf("list workflow runs: %w", err)
	}
	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, runs)
	}
	fullID, _ := cmd.Flags().GetBool("full-id")
	printWorkflowRunTable(runs, fullID)
	return nil
}

func runWorkflowRunGet(cmd *cobra.Command, args []string) error {
	var run map[string]any
	if err := workflowGetJSON(cmd, "/api/workflow-runs/"+url.PathEscape(args[0]), &run); err != nil {
		return err
	}
	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, run)
	}
	fullID, _ := cmd.Flags().GetBool("full-id")
	printWorkflowRunTable([]map[string]any{run}, fullID)
	if steps, ok := run["steps"].([]any); ok && len(steps) > 0 {
		fmt.Fprintln(os.Stdout)
		printWorkflowStepTable(anySliceToMaps(steps), fullID)
	}
	return nil
}

func runWorkflowRunCancel(cmd *cobra.Command, args []string) error {
	var run map[string]any
	if err := workflowPostJSON(cmd, "/api/workflow-runs/"+url.PathEscape(args[0])+"/cancel", map[string]any{}, &run); err != nil {
		return err
	}
	return workflowPrintResult(cmd, run, nil)
}

func runWorkflowRunRerun(cmd *cobra.Command, args []string) error {
	var task map[string]any
	if err := workflowPostJSON(cmd, "/api/workflow-runs/"+url.PathEscape(args[0])+"/rerun", map[string]any{}, &task); err != nil {
		return err
	}
	return workflowPrintResult(cmd, task, nil)
}

func runWorkflowStepGet(cmd *cobra.Command, args []string) error {
	var step map[string]any
	if err := workflowGetJSON(cmd, "/api/workflow-step-runs/"+url.PathEscape(args[0]), &step); err != nil {
		return err
	}
	return workflowPrintResult(cmd, step, printWorkflowStepResult)
}

func runWorkflowStepStart(cmd *cobra.Command, args []string) error {
	return runWorkflowStepMutation(cmd, args[0], "start", map[string]any{})
}

func runWorkflowStepComplete(cmd *cobra.Command, args []string) error {
	return runWorkflowStepMutation(cmd, args[0], "complete", map[string]any{})
}

func runWorkflowStepManualComplete(cmd *cobra.Command, args []string) error {
	return runWorkflowStepMutation(cmd, args[0], "manual-complete", map[string]any{})
}

func runWorkflowStepFail(cmd *cobra.Command, args []string) error {
	reason, _ := cmd.Flags().GetString("reason")
	return runWorkflowStepMutation(cmd, args[0], "fail", map[string]any{"reason": reason})
}

func runWorkflowStepPause(cmd *cobra.Command, args []string) error {
	reason, _ := cmd.Flags().GetString("reason")
	return runWorkflowStepMutation(cmd, args[0], "pause", map[string]any{"reason": reason})
}

func runWorkflowStepRetry(cmd *cobra.Command, args []string) error {
	return runWorkflowStepMutation(cmd, args[0], "retry", map[string]any{})
}

func runWorkflowStepSkip(cmd *cobra.Command, args []string) error {
	return runWorkflowStepMutation(cmd, args[0], "skip", map[string]any{})
}

func runWorkflowArtifactSave(cmd *cobra.Command, args []string) error {
	name, _ := cmd.Flags().GetString("name")
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("--name is required")
	}
	format, contentText, contentJSON, err := workflowReadContent(cmd)
	if err != nil {
		return err
	}
	body := map[string]any{
		"logical_name": name,
		"content_kind": format,
	}
	if format == "json" {
		body["content_json"] = json.RawMessage(contentJSON)
	} else {
		body["content_text"] = contentText
	}
	var artifact map[string]any
	if err := workflowPostJSON(cmd, "/api/workflow-step-runs/"+url.PathEscape(args[0])+"/artifacts", body, &artifact); err != nil {
		return err
	}
	return workflowPrintResult(cmd, artifact, nil)
}

func runWorkflowArtifactDiff(cmd *cobra.Command, args []string) error {
	baseVersion, _ := cmd.Flags().GetInt32("base-version")
	targetVersion, _ := cmd.Flags().GetInt32("target-version")
	if baseVersion <= 0 {
		return fmt.Errorf("--base-version is required")
	}
	if targetVersion <= 0 {
		return fmt.Errorf("--target-version is required")
	}
	values := url.Values{}
	values.Set("base_version", fmt.Sprint(baseVersion))
	values.Set("target_version", fmt.Sprint(targetVersion))
	var diff map[string]any
	if err := workflowGetJSON(cmd, "/api/workflow-artifacts/"+url.PathEscape(args[0])+"/diff?"+values.Encode(), &diff); err != nil {
		return err
	}
	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, diff)
	}
	fmt.Fprint(os.Stdout, strVal(diff, "unified_diff"))
	return nil
}

func runWorkflowInputRequest(cmd *cobra.Command, args []string) error {
	filePath, _ := cmd.Flags().GetString("question-file")
	if strings.TrimSpace(filePath) == "" {
		return fmt.Errorf("--question-file is required")
	}
	data, err := readWorkflowFile(filePath)
	if err != nil {
		return err
	}
	maxRounds, _ := cmd.Flags().GetInt32("max-rounds")
	body := map[string]any{
		"question_text": string(data),
	}
	if maxRounds > 0 {
		body["max_rounds"] = maxRounds
	}
	var request map[string]any
	if err := workflowPostJSON(cmd, "/api/workflow-step-runs/"+url.PathEscape(args[0])+"/input-requests", body, &request); err != nil {
		return err
	}
	return workflowPrintResult(cmd, request, nil)
}

func runWorkflowInputAnswer(cmd *cobra.Command, args []string) error {
	filePath, _ := cmd.Flags().GetString("file")
	if strings.TrimSpace(filePath) == "" {
		return fmt.Errorf("--file is required")
	}
	data, err := readWorkflowFile(filePath)
	if err != nil {
		return err
	}
	noContinue, _ := cmd.Flags().GetBool("no-continue")
	var request map[string]any
	if err := workflowPostJSON(cmd, "/api/workflow-input-requests/"+url.PathEscape(args[0])+"/answer", map[string]any{
		"answer_text": string(data),
		"continue":    !noContinue,
	}, &request); err != nil {
		return err
	}
	return workflowPrintResult(cmd, request, nil)
}

func runWorkflowInputCancel(cmd *cobra.Command, args []string) error {
	var request map[string]any
	if err := workflowPostJSON(cmd, "/api/workflow-input-requests/"+url.PathEscape(args[0])+"/cancel", map[string]any{}, &request); err != nil {
		return err
	}
	return workflowPrintResult(cmd, request, nil)
}

func runWorkflowQualityReport(cmd *cobra.Command, args []string) error {
	status, _ := cmd.Flags().GetString("status")
	if strings.TrimSpace(status) == "" {
		return fmt.Errorf("--status is required")
	}
	format, contentText, contentJSON, err := workflowReadOptionalContent(cmd)
	if err != nil {
		return err
	}
	artifactID, _ := cmd.Flags().GetString("artifact")
	blocking, _ := cmd.Flags().GetBool("blocking")
	body := map[string]any{
		"artifact_id": artifactID,
		"status":      status,
		"blocking":    blocking,
	}
	if format == "json" && len(contentJSON) > 0 {
		body["report_json"] = json.RawMessage(contentJSON)
	} else if contentText != "" {
		body["report_text"] = contentText
	}
	var result map[string]any
	if err := workflowPostJSON(cmd, "/api/workflow-step-runs/"+url.PathEscape(args[0])+"/quality-gates", body, &result); err != nil {
		return err
	}
	return workflowPrintResult(cmd, result, nil)
}

func runWorkflowReviewApprove(cmd *cobra.Command, args []string) error {
	return runWorkflowReviewDecision(cmd, args[0], "approve")
}

func runWorkflowReviewReject(cmd *cobra.Command, args []string) error {
	return runWorkflowReviewDecision(cmd, args[0], "reject")
}

func runWorkflowStepMutation(cmd *cobra.Command, stepID, action string, body map[string]any) error {
	var step map[string]any
	if err := workflowPostJSON(cmd, "/api/workflow-step-runs/"+url.PathEscape(stepID)+"/"+action, body, &step); err != nil {
		return err
	}
	return workflowPrintResult(cmd, step, printWorkflowStepResult)
}

func runWorkflowReviewDecision(cmd *cobra.Command, reviewID, action string) error {
	notes, _ := cmd.Flags().GetString("notes")
	var review map[string]any
	if err := workflowPostJSON(cmd, "/api/workflow-reviews/"+url.PathEscape(reviewID)+"/"+action, map[string]any{"notes": notes}, &review); err != nil {
		return err
	}
	return workflowPrintResult(cmd, review, nil)
}

func workflowGetJSON(cmd *cobra.Command, path string, out any) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := client.GetJSON(ctx, path, out); err != nil {
		return fmt.Errorf("workflow request: %w", err)
	}
	return nil
}

func workflowPostJSON(cmd *cobra.Command, path string, body any, out any) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := client.PostJSON(ctx, path, body, out); err != nil {
		return fmt.Errorf("workflow request: %w", err)
	}
	return nil
}

func workflowPrintResult(cmd *cobra.Command, result map[string]any, table func(map[string]any, bool)) error {
	output, _ := cmd.Flags().GetString("output")
	if output == "json" || table == nil {
		return cli.PrintJSON(os.Stdout, result)
	}
	fullID, _ := cmd.Flags().GetBool("full-id")
	table(result, fullID)
	return nil
}

func workflowReadContent(cmd *cobra.Command) (string, string, []byte, error) {
	filePath, _ := cmd.Flags().GetString("file")
	if strings.TrimSpace(filePath) == "" {
		return "", "", nil, fmt.Errorf("--file is required")
	}
	data, err := readWorkflowFile(filePath)
	if err != nil {
		return "", "", nil, err
	}
	format, err := workflowContentFormat(cmd)
	if err != nil {
		return "", "", nil, err
	}
	if format == "json" && !json.Valid(data) {
		return "", "", nil, fmt.Errorf("JSON content is invalid")
	}
	return format, string(data), data, nil
}

func workflowReadOptionalContent(cmd *cobra.Command) (string, string, []byte, error) {
	filePath, _ := cmd.Flags().GetString("file")
	format, err := workflowContentFormat(cmd)
	if err != nil {
		return "", "", nil, err
	}
	if strings.TrimSpace(filePath) == "" {
		return format, "", nil, nil
	}
	data, err := readWorkflowFile(filePath)
	if err != nil {
		return "", "", nil, err
	}
	if format == "json" && !json.Valid(data) {
		return "", "", nil, fmt.Errorf("JSON content is invalid")
	}
	return format, string(data), data, nil
}

func workflowContentFormat(cmd *cobra.Command) (string, error) {
	format, _ := cmd.Flags().GetString("format")
	switch strings.TrimSpace(strings.ToLower(format)) {
	case "markdown", "json", "text":
		return strings.TrimSpace(strings.ToLower(format)), nil
	default:
		return "", fmt.Errorf("--format must be markdown, json, or text")
	}
}

func readWorkflowFile(path string) ([]byte, error) {
	if path == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("read stdin: %w", err)
		}
		return data, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	return data, nil
}

func printWorkflowRunTable(runs []map[string]any, fullID bool) {
	headers := []string{"ID", "STATUS", "TRIGGER", "TASK", "CREATED", "COMPLETED"}
	rows := make([][]string, 0, len(runs))
	for _, run := range runs {
		rows = append(rows, []string{
			displayID(strVal(run, "id"), fullID),
			strVal(run, "status"),
			strVal(run, "trigger_type"),
			displayID(strVal(run, "agent_task_queue_id"), fullID),
			shortTime(strVal(run, "created_at")),
			shortTime(strVal(run, "completed_at")),
		})
	}
	cli.PrintTable(os.Stdout, headers, rows)
}

func printWorkflowStepResult(step map[string]any, fullID bool) {
	printWorkflowStepTable([]map[string]any{step}, fullID)
}

func printWorkflowStepTable(steps []map[string]any, fullID bool) {
	headers := []string{"ID", "STEP", "STATUS", "KIND", "ATTEMPT", "TITLE"}
	rows := make([][]string, 0, len(steps))
	for _, step := range steps {
		rows = append(rows, []string{
			displayID(strVal(step, "id"), fullID),
			strVal(step, "step_definition_id"),
			strVal(step, "status"),
			strVal(step, "execution_kind"),
			strVal(step, "attempt"),
			strVal(step, "title"),
		})
	}
	cli.PrintTable(os.Stdout, headers, rows)
}

func anySliceToMaps(values []any) []map[string]any {
	out := make([]map[string]any, 0, len(values))
	for _, value := range values {
		if m, ok := value.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func shortTime(value string) string {
	if len(value) >= 16 {
		return value[:16]
	}
	return value
}
