package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/multica-ai/multica/server/internal/connectors"
)

var connectorCmd = &cobra.Command{
	Use:   "connector",
	Short: "Use task-scoped external connectors",
}

var connectorGitLabCmd = &cobra.Command{
	Use:   "gitlab",
	Short: "Use RingCentral GitLab resources",
}

var connectorGitLabBranchesCmd = &cobra.Command{
	Use:   "branches <project-resource-id>",
	Short: "List GitLab branches for an attached project resource",
	Args:  exactArgs(1),
	RunE:  runConnectorGitLabBranches,
}

var connectorGitLabMRsCmd = &cobra.Command{
	Use:   "merge-requests <project-resource-id>",
	Short: "List GitLab merge requests for an attached project resource",
	Args:  exactArgs(1),
	RunE:  runConnectorGitLabMergeRequests,
}

var connectorGitLabCreateBranchCmd = &cobra.Command{
	Use:   "create-branch <project-resource-id>",
	Short: "Create a semantic task branch through the GitLab API",
	Args:  exactArgs(1),
	RunE:  runConnectorGitLabCreateBranch,
}

var connectorGitLabCommitCmd = &cobra.Command{
	Use:   "commit <project-resource-id>",
	Short: "Create a GitLab commit from JSON file actions",
	Args:  exactArgs(1),
	RunE:  runConnectorGitLabCommit,
}

var connectorGitLabCreateMRCmd = &cobra.Command{
	Use:   "create-mr <project-resource-id>",
	Short: "Create a GitLab merge request from a semantic task branch",
	Args:  exactArgs(1),
	RunE:  runConnectorGitLabCreateMR,
}

var connectorJiraCmd = &cobra.Command{
	Use:   "jira",
	Short: "Use RingCentral Jira resources",
}

var connectorJiraSearchCmd = &cobra.Command{
	Use:   "search <project-resource-id>",
	Short: "Search Jira issues within an attached project resource",
	Args:  exactArgs(1),
	RunE:  runConnectorJiraSearch,
}

var connectorJiraIssueCmd = &cobra.Command{
	Use:   "issue <project-resource-id> [issue-key]",
	Short: "Read a Jira issue from an attached issue or project resource",
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runConnectorJiraIssue,
}

var connectorWikiCmd = &cobra.Command{
	Use:   "wiki",
	Short: "Use RingCentral Wiki resources",
}

var connectorWikiSearchCmd = &cobra.Command{
	Use:   "search <project-resource-id>",
	Short: "Search Wiki pages within an attached space resource",
	Args:  exactArgs(1),
	RunE:  runConnectorWikiSearch,
}

var connectorWikiPageCmd = &cobra.Command{
	Use:   "page <project-resource-id> [page-id]",
	Short: "Read a Wiki page from an attached page or space resource",
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runConnectorWikiPage,
}

func init() {
	connectorGitLabBranchesCmd.Flags().String("search", "", "Branch search filter")
	connectorGitLabMRsCmd.Flags().String("state", "opened", "Merge request state")
	connectorGitLabCreateBranchCmd.Flags().String("branch", "", "New semantic branch name (required)")
	connectorGitLabCreateBranchCmd.Flags().String("ref", "", "Source branch, tag, or SHA")
	connectorGitLabCommitCmd.Flags().String("branch", "", "Target semantic branch (required)")
	connectorGitLabCommitCmd.Flags().String("message", "", "Commit message (required)")
	connectorGitLabCommitCmd.Flags().String("actions-json", "", "GitLab commit actions JSON array (required)")
	connectorGitLabCreateMRCmd.Flags().String("source-branch", "", "Source semantic branch (required)")
	connectorGitLabCreateMRCmd.Flags().String("target-branch", "", "Target branch")
	connectorGitLabCreateMRCmd.Flags().String("title", "", "Merge request title (required)")
	connectorGitLabCreateMRCmd.Flags().String("description", "", "Merge request description")

	connectorJiraSearchCmd.Flags().String("jql", "", "Jira JQL, scoped to the attached project when possible")
	connectorJiraSearchCmd.Flags().Int("max-results", 25, "Maximum issues to return")
	connectorWikiSearchCmd.Flags().String("cql", "", "Confluence CQL, scoped to the attached space when possible")
	connectorWikiSearchCmd.Flags().Int("limit", 25, "Maximum pages to return")

	connectorGitLabCmd.AddCommand(
		connectorGitLabBranchesCmd,
		connectorGitLabMRsCmd,
		connectorGitLabCreateBranchCmd,
		connectorGitLabCommitCmd,
		connectorGitLabCreateMRCmd,
	)
	connectorJiraCmd.AddCommand(connectorJiraSearchCmd, connectorJiraIssueCmd)
	connectorWikiCmd.AddCommand(connectorWikiSearchCmd, connectorWikiPageCmd)
	connectorCmd.AddCommand(connectorGitLabCmd, connectorJiraCmd, connectorWikiCmd)
}

type connectorActionRequest struct {
	ProviderID        string         `json:"provider_id"`
	Capability        string         `json:"capability"`
	ProjectResourceID string         `json:"project_resource_id"`
	Input             map[string]any `json:"input,omitempty"`
}

func postConnectorAction(cmd *cobra.Command, req connectorActionRequest) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	if client.WorkspaceID == "" {
		return fmt.Errorf("connector commands require daemon workspace identity: MULTICA_WORKSPACE_ID must be set")
	}
	if client.AgentID == "" || client.TaskID == "" {
		return fmt.Errorf("connector commands require daemon task identity: MULTICA_AGENT_ID and MULTICA_TASK_ID must be set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var result map[string]any
	if err := client.PostJSON(ctx, "/api/connectors/actions", req, &result); err != nil {
		return err
	}
	return cli.PrintJSON(os.Stdout, result)
}

func runConnectorGitLabBranches(cmd *cobra.Command, args []string) error {
	search, _ := cmd.Flags().GetString("search")
	return postConnectorAction(cmd, connectorActionRequest{
		ProviderID:        connectors.ProviderRingCentralGitLab,
		Capability:        "branch.list",
		ProjectResourceID: args[0],
		Input:             map[string]any{"search": search},
	})
}

func runConnectorGitLabMergeRequests(cmd *cobra.Command, args []string) error {
	state, _ := cmd.Flags().GetString("state")
	return postConnectorAction(cmd, connectorActionRequest{
		ProviderID:        connectors.ProviderRingCentralGitLab,
		Capability:        "merge_request.list",
		ProjectResourceID: args[0],
		Input:             map[string]any{"state": state},
	})
}

func runConnectorGitLabCreateBranch(cmd *cobra.Command, args []string) error {
	branch, _ := cmd.Flags().GetString("branch")
	ref, _ := cmd.Flags().GetString("ref")
	return postConnectorAction(cmd, connectorActionRequest{
		ProviderID:        connectors.ProviderRingCentralGitLab,
		Capability:        "branch.create",
		ProjectResourceID: args[0],
		Input:             map[string]any{"branch": branch, "ref": ref},
	})
}

func runConnectorGitLabCommit(cmd *cobra.Command, args []string) error {
	branch, _ := cmd.Flags().GetString("branch")
	message, _ := cmd.Flags().GetString("message")
	actionsRaw, _ := cmd.Flags().GetString("actions-json")
	var actions []map[string]any
	if actionsRaw == "" {
		return fmt.Errorf("--actions-json is required")
	}
	if err := json.Unmarshal([]byte(actionsRaw), &actions); err != nil {
		return fmt.Errorf("parse --actions-json: %w", err)
	}
	return postConnectorAction(cmd, connectorActionRequest{
		ProviderID:        connectors.ProviderRingCentralGitLab,
		Capability:        "commit.create",
		ProjectResourceID: args[0],
		Input: map[string]any{
			"branch":         branch,
			"commit_message": message,
			"actions":        actions,
		},
	})
}

func runConnectorGitLabCreateMR(cmd *cobra.Command, args []string) error {
	sourceBranch, _ := cmd.Flags().GetString("source-branch")
	targetBranch, _ := cmd.Flags().GetString("target-branch")
	title, _ := cmd.Flags().GetString("title")
	description, _ := cmd.Flags().GetString("description")
	return postConnectorAction(cmd, connectorActionRequest{
		ProviderID:        connectors.ProviderRingCentralGitLab,
		Capability:        "merge_request.create",
		ProjectResourceID: args[0],
		Input: map[string]any{
			"source_branch": sourceBranch,
			"target_branch": targetBranch,
			"title":         title,
			"description":   description,
		},
	})
}

func runConnectorJiraSearch(cmd *cobra.Command, args []string) error {
	jql, _ := cmd.Flags().GetString("jql")
	maxResults, _ := cmd.Flags().GetInt("max-results")
	return postConnectorAction(cmd, connectorActionRequest{
		ProviderID:        connectors.ProviderRingCentralJira,
		Capability:        "issue.search",
		ProjectResourceID: args[0],
		Input:             map[string]any{"jql": jql, "max_results": maxResults},
	})
}

func runConnectorJiraIssue(cmd *cobra.Command, args []string) error {
	input := map[string]any{}
	if len(args) > 1 {
		input["issue_key"] = args[1]
	}
	return postConnectorAction(cmd, connectorActionRequest{
		ProviderID:        connectors.ProviderRingCentralJira,
		Capability:        "issue.read",
		ProjectResourceID: args[0],
		Input:             input,
	})
}

func runConnectorWikiSearch(cmd *cobra.Command, args []string) error {
	cql, _ := cmd.Flags().GetString("cql")
	limit, _ := cmd.Flags().GetInt("limit")
	return postConnectorAction(cmd, connectorActionRequest{
		ProviderID:        connectors.ProviderRingCentralWiki,
		Capability:        "page.search",
		ProjectResourceID: args[0],
		Input:             map[string]any{"cql": cql, "limit": limit},
	})
}

func runConnectorWikiPage(cmd *cobra.Command, args []string) error {
	input := map[string]any{}
	if len(args) > 1 {
		input["page_id"] = args[1]
	}
	return postConnectorAction(cmd, connectorActionRequest{
		ProviderID:        connectors.ProviderRingCentralWiki,
		Capability:        "page.read",
		ProjectResourceID: args[0],
		Input:             input,
	})
}
