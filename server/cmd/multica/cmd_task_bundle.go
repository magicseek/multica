package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/cli"
)

var taskBundleCmd = &cobra.Command{
	Use:   "task-bundle",
	Short: "Work with request-efficient task bundles",
}

var taskBundleCheckpointCmd = &cobra.Command{
	Use:   "checkpoint <item-id>",
	Short: "Record progress for the current task bundle item",
	Args:  exactArgs(1),
	RunE:  runTaskBundleCheckpoint,
}

func init() {
	taskBundleCmd.AddCommand(taskBundleCheckpointCmd)

	taskBundleCheckpointCmd.Flags().String("status", "", "Item status: completed, failed, blocked, input_needed, or cancelled (required)")
	taskBundleCheckpointCmd.Flags().String("result", "", "Result metadata or summary text")
	taskBundleCheckpointCmd.Flags().Bool("result-stdin", false, "Read result metadata or summary text from stdin")
	taskBundleCheckpointCmd.Flags().String("result-file", "", "Read result metadata or summary text from a file")
	taskBundleCheckpointCmd.Flags().String("error", "", "Error or blocking reason")
	taskBundleCheckpointCmd.Flags().Int32("checkpoint-seq", 0, "Transcript sequence number to mark as the item boundary")
	taskBundleCheckpointCmd.Flags().String("output", "json", "Output format: json or table")
}

func runTaskBundleCheckpoint(cmd *cobra.Command, args []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	if strings.TrimSpace(client.TaskID) == "" {
		return fmt.Errorf("task bundle checkpoint must run inside a Multica task with MULTICA_TASK_ID set")
	}

	status, _ := cmd.Flags().GetString("status")
	status = strings.TrimSpace(status)
	if !validTaskBundleCheckpointStatus(status) {
		return fmt.Errorf("invalid --status %q; expected completed, failed, blocked, input_needed, or cancelled", status)
	}

	result, hasResult, err := resolveTextFlag(cmd, "result")
	if err != nil {
		return err
	}
	errText, _ := cmd.Flags().GetString("error")
	body := map[string]any{
		"item_id": args[0],
		"status":  status,
	}
	if hasResult {
		body["result"] = map[string]any{"summary": result}
	}
	if errText != "" {
		body["error"] = errText
	}
	if cmd.Flags().Changed("checkpoint-seq") {
		seq, _ := cmd.Flags().GetInt32("checkpoint-seq")
		body["checkpoint_seq"] = seq
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var resp map[string]any
	path := "/api/daemon/tasks/" + client.TaskID + "/bundle/checkpoint"
	if err := client.PostJSON(ctx, path, body, &resp); err != nil {
		return fmt.Errorf("checkpoint task bundle item: %w", err)
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "table" {
		fmt.Fprintf(os.Stdout, "Checkpointed item %s as %s\n", args[0], status)
		return nil
	}
	return cli.PrintJSON(os.Stdout, resp)
}

func validTaskBundleCheckpointStatus(status string) bool {
	switch status {
	case "completed", "failed", "blocked", "input_needed", "cancelled":
		return true
	default:
		return false
	}
}
