package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadTaskOutputManifestReadsExplicitFileOnly(t *testing.T) {
	workDir := t.TempDir()
	manifest, err := loadTaskOutputManifest(workDir, false)
	if err != nil {
		t.Fatalf("missing manifest returned error: %v", err)
	}
	if manifest != nil {
		t.Fatalf("missing manifest = %+v, want nil", manifest)
	}

	if err := os.MkdirAll(filepath.Join(workDir, ".multica"), 0o755); err != nil {
		t.Fatalf("create manifest dir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(workDir, TaskOutputManifestRelativePath),
		[]byte(`{"outputs":[{"relative_path":"docs/design.md","kind":"doc","size_bytes":42}]}`),
		0o644,
	); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	manifest, err = loadTaskOutputManifest(workDir, false)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if manifest == nil || len(manifest.Outputs) != 1 {
		t.Fatalf("expected 1 output, got %+v", manifest)
	}
	output := manifest.Outputs[0]
	if output.RelativePath != "docs/design.md" || output.Kind != "doc" {
		t.Fatalf("unexpected output: %+v", output)
	}
	if output.SizeBytes == nil || *output.SizeBytes != 42 {
		t.Fatalf("size_bytes = %v, want 42", output.SizeBytes)
	}
}

func TestLoadTaskOutputManifestRejectsLargeManifest(t *testing.T) {
	workDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workDir, ".multica"), 0o755); err != nil {
		t.Fatalf("create manifest dir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(workDir, TaskOutputManifestRelativePath),
		[]byte(strings.Repeat("x", int(maxTaskOutputManifestBytes)+1)),
		0o644,
	); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	if _, err := loadTaskOutputManifest(workDir, false); err == nil {
		t.Fatal("expected oversized manifest error")
	}
}

func TestLoadStructuredTaskOutputsReadsAllManifests(t *testing.T) {
	workDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workDir, ".multica"), 0o755); err != nil {
		t.Fatalf("create manifest dir: %v", err)
	}
	files := map[string]string{
		TaskChatSummaryManifestRelativePath:    `{"version":1,"title":"Structured title"}`,
		TaskPlanSummaryManifestRelativePath:    `{"version":1,"confirmed_requirements":["Need plan mode"],"rejected_options":[],"consensus_notes":["Use proposals"],"open_questions":[]}`,
		TaskIssueProposalsManifestRelativePath: `{"version":1,"proposals":[{"title":"Follow-ups","items":[{"title":"Create review flow","description":"Review proposed issues","priority":"medium","labels":["chat"]}]}]}`,
		TaskOutputManifestRelativePath:         `{"outputs":[{"relative_path":"docs/chat.md","kind":"doc"}]}`,
	}
	for relativePath, content := range files {
		if err := os.WriteFile(filepath.Join(workDir, relativePath), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", relativePath, err)
		}
	}

	structured := loadStructuredTaskOutputs(workDir, false, nil)
	if structured == nil {
		t.Fatal("structured outputs = nil")
	}
	if structured.ChatSummary == nil || structured.ChatSummary.Title != "Structured title" {
		t.Fatalf("chat summary = %+v", structured.ChatSummary)
	}
	if structured.PlanSummary == nil || len(structured.PlanSummary.ConfirmedRequirements) != 1 {
		t.Fatalf("plan summary = %+v", structured.PlanSummary)
	}
	if structured.IssueProposals == nil || len(structured.IssueProposals.Proposals) != 1 {
		t.Fatalf("issue proposals = %+v", structured.IssueProposals)
	}
	if got := structured.IssueProposals.Proposals[0].Items[0].Labels; len(got) != 1 || got[0] != "chat" {
		t.Fatalf("proposal labels = %v", got)
	}
	if structured.Outputs == nil || len(structured.Outputs.Outputs) != 1 {
		t.Fatalf("outputs = %+v", structured.Outputs)
	}
}

func TestClearStaleStructuredTaskOutputManifestsRemovesOnlyTaskOutputs(t *testing.T) {
	workDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workDir, ".multica", "project"), 0o755); err != nil {
		t.Fatalf("create manifest dirs: %v", err)
	}
	files := map[string]string{
		TaskChatSummaryManifestRelativePath:           `{"version":1,"title":"Old chat"}`,
		TaskPlanSummaryManifestRelativePath:           `{"version":1,"confirmed_requirements":["old"]}`,
		TaskIssueProposalsManifestRelativePath:        `{"version":1,"proposals":[{"title":"Old proposal"}]}`,
		TaskOutputManifestRelativePath:                `{"outputs":[{"relative_path":"old.md","kind":"doc"}]}`,
		".multica/project/resources.json":             `{"resources":[{"label":"Keep me"}]}`,
		".multica/project/agent-runtime-context.json": `{"provider":"codex"}`,
	}
	for relativePath, content := range files {
		if err := os.WriteFile(filepath.Join(workDir, relativePath), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", relativePath, err)
		}
	}

	clearStaleStructuredTaskOutputManifests(workDir, false, nil)

	for _, relativePath := range []string{
		TaskChatSummaryManifestRelativePath,
		TaskPlanSummaryManifestRelativePath,
		TaskIssueProposalsManifestRelativePath,
		TaskOutputManifestRelativePath,
	} {
		if _, err := os.Stat(filepath.Join(workDir, relativePath)); !os.IsNotExist(err) {
			t.Fatalf("%s still exists or stat failed with unexpected error: %v", relativePath, err)
		}
	}
	for _, relativePath := range []string{
		".multica/project/resources.json",
		".multica/project/agent-runtime-context.json",
	} {
		if _, err := os.Stat(filepath.Join(workDir, relativePath)); err != nil {
			t.Fatalf("%s should be preserved: %v", relativePath, err)
		}
	}
	if structured := loadStructuredTaskOutputs(workDir, false, nil); structured != nil {
		t.Fatalf("structured outputs after cleanup = %+v, want nil", structured)
	}
}

func TestChatScopedStructuredTaskOutputsIgnoreOtherChatsAndLegacyFiles(t *testing.T) {
	workDir := t.TempDir()
	legacyDir := filepath.Join(workDir, ".multica")
	chatOneDir := filepath.Join(workDir, ".multica", "chats", "chat-one")
	chatTwoDir := filepath.Join(workDir, ".multica", "chats", "chat-two")
	for _, dir := range []string{legacyDir, chatOneDir, chatTwoDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("create manifest dir %s: %v", dir, err)
		}
	}
	files := map[string]string{
		filepath.Join(legacyDir, TaskIssueProposalsManifestFileName):  `{"version":1,"proposals":[{"title":"Legacy stale","items":[{"title":"Wrong","description":"Wrong chat"}]}]}`,
		filepath.Join(chatOneDir, TaskIssueProposalsManifestFileName): `{"version":1,"proposals":[{"title":"Chat one","items":[{"title":"Other","description":"Other chat"}]}]}`,
		filepath.Join(chatTwoDir, TaskIssueProposalsManifestFileName): `{"version":1,"proposals":[{"title":"Chat two","items":[{"title":"Expected","description":"Current chat","labels":["current"]}]}]}`,
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	structured := loadStructuredTaskOutputs(chatTwoDir, true, nil)
	if structured == nil || structured.IssueProposals == nil || len(structured.IssueProposals.Proposals) != 1 {
		t.Fatalf("issue proposals = %+v", structured)
	}
	if got := structured.IssueProposals.Proposals[0].Title; got != "Chat two" {
		t.Fatalf("loaded proposal title = %q, want Chat two", got)
	}

	clearStaleStructuredTaskOutputManifests(chatTwoDir, true, nil)
	if _, err := os.Stat(filepath.Join(chatTwoDir, TaskIssueProposalsManifestFileName)); !os.IsNotExist(err) {
		t.Fatalf("chat two manifest still exists or stat failed with unexpected error: %v", err)
	}
	for _, preserved := range []string{
		filepath.Join(legacyDir, TaskIssueProposalsManifestFileName),
		filepath.Join(chatOneDir, TaskIssueProposalsManifestFileName),
	} {
		if _, err := os.Stat(preserved); err != nil {
			t.Fatalf("%s should be preserved: %v", preserved, err)
		}
	}
}

func TestLoadStructuredTaskOutputsIgnoresInvalidManifestIndividually(t *testing.T) {
	workDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workDir, ".multica"), 0o755); err != nil {
		t.Fatalf("create manifest dir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(workDir, TaskChatSummaryManifestRelativePath),
		[]byte(`{"version":1,"title":"Still valid"}`),
		0o644,
	); err != nil {
		t.Fatalf("write summary manifest: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(workDir, TaskIssueProposalsManifestRelativePath),
		[]byte(`{"version":1,"proposals":[`),
		0o644,
	); err != nil {
		t.Fatalf("write invalid proposals manifest: %v", err)
	}

	structured := loadStructuredTaskOutputs(workDir, false, nil)
	if structured == nil {
		t.Fatal("structured outputs = nil")
	}
	if structured.ChatSummary == nil || structured.ChatSummary.Title != "Still valid" {
		t.Fatalf("chat summary = %+v", structured.ChatSummary)
	}
	if structured.IssueProposals != nil {
		t.Fatalf("invalid issue proposals should be ignored, got %+v", structured.IssueProposals)
	}
}
