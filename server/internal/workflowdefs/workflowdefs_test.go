package workflowdefs

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNormalizeSchemaCanonicalizesLegacyFields(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"version": 1,
		"applicability": ["assignment"],
		"source": {
			"mode": "markdown",
			"body_template": "Run {{issue_id}}"
		},
		"steps": [
			{"id": "context", "title": "Context", "order": 1},
			{"id": "verify", "name": "Verify", "depends_on": ["context"]}
		]
	}`)
	normalized, err := NormalizeSchema(raw, "Fallback", "Fallback description")
	if err != nil {
		t.Fatalf("NormalizeSchema returned error: %v", err)
	}

	var schema Schema
	if err := json.Unmarshal(normalized, &schema); err != nil {
		t.Fatalf("unmarshal normalized schema: %v", err)
	}
	if schema.SchemaVersion != 2 {
		t.Fatalf("schema_version = %d, want 2", schema.SchemaVersion)
	}
	if schema.Version != 0 {
		t.Fatalf("legacy version should be omitted, got %d", schema.Version)
	}
	if schema.Source.Format != "markdown" {
		t.Fatalf("source.format = %q, want markdown", schema.Source.Format)
	}
	if schema.Source.Mode != "" {
		t.Fatalf("legacy source.mode should be omitted, got %q", schema.Source.Mode)
	}
	if got := schema.Steps[1].DependsOn; len(got) != 1 || got[0] != "context" {
		t.Fatalf("depends_on = %#v, want [context]", got)
	}
}

func TestNormalizeSchemaValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{
			name:    "unsupported schema version",
			raw:     `{"schema_version": 3, "applicability": ["assignment"]}`,
			wantErr: "unsupported schema_version 3",
		},
		{
			name:    "missing applicability",
			raw:     `{"schema_version": 1, "source": {"format": "markdown"}}`,
			wantErr: "schema applicability is required",
		},
		{
			name:    "duplicate step ids",
			raw:     `{"schema_version": 1, "applicability": ["assignment"], "steps": [{"id": "a", "title": "A"}, {"id": "a", "title": "Again"}]}`,
			wantErr: `duplicate step id "a"`,
		},
		{
			name:    "unknown dependency",
			raw:     `{"schema_version": 1, "applicability": ["assignment"], "steps": [{"id": "a", "title": "A", "depends_on": ["missing"]}]}`,
			wantErr: `step "a" depends on unknown step "missing"`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := NormalizeSchema([]byte(tt.raw), "Fallback", "")
			if err == nil {
				t.Fatalf("NormalizeSchema returned nil error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestNormalizeSchemaV2StepRuntimeFields(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"schema_version": 2,
		"applicability": ["assignment"],
		"steps": [
			{
				"id": "draft",
				"title": "Draft",
				"execution": {"kind": "manual"},
				"artifact": {
					"name": "Design",
					"content_kind": "md",
					"inputs": [{"step_id": "context", "artifact_name": "Context"}]
				},
				"review_required": true,
				"quality_gate": {"prompt": "Check quality"}
			},
			{"id": "context", "title": "Context"}
		]
	}`)
	normalized, err := NormalizeSchema(raw, "Runtime fields", "")
	if err != nil {
		t.Fatalf("NormalizeSchema returned error: %v", err)
	}
	var schema Schema
	if err := json.Unmarshal(normalized, &schema); err != nil {
		t.Fatalf("unmarshal normalized schema: %v", err)
	}
	step := schema.Steps[0]
	if step.Execution == nil || step.Execution.Kind != "manual" {
		t.Fatalf("execution = %#v, want manual", step.Execution)
	}
	if step.Artifact == nil || step.Artifact.ContentKind != "markdown" {
		t.Fatalf("artifact = %#v, want markdown artifact", step.Artifact)
	}
	if step.Review == nil || !step.Review.Required || step.ReviewRequired != nil {
		t.Fatalf("review = %#v review_required = %#v, want canonical required review", step.Review, step.ReviewRequired)
	}
	if step.QualityGate == nil || !step.QualityGate.Enabled || step.QualityGate.ReportMode != "summary" {
		t.Fatalf("quality_gate = %#v, want enabled summary quality gate", step.QualityGate)
	}
}

func TestImportAIDeskYAMLMapsSupportedFieldsAndWarns(t *testing.T) {
	t.Parallel()

	content := []byte(`
name: Design Review
description: Review a proposed design
workflow_type: AUTOMATION
party_mode_discuss: true
pre_added_agents:
  - helper
step_definitions:
  - id: design
    name: Write design
    default_execution_mode: LOCAL_AGENT
    agent_prompt: Draft the design.
    rules: Keep it concise.
    artifact_name: Design
    artifact_template_content: "# Design"
    review_required: true
    quality_gate_prompt: Check the design.
    quality_report_mode: full
  - id: approve
    name: Approve design
    default_execution_mode: MANUAL
    depends_on_steps: [design]
    input_artifacts:
      - step_id: design
        artifact_name: Design
`)
	result, err := ImportSchema("yaml", content, "", "")
	if err != nil {
		t.Fatalf("ImportSchema returned error: %v", err)
	}
	var schema Schema
	if err := json.Unmarshal(result.Schema, &schema); err != nil {
		t.Fatalf("unmarshal imported schema: %v", err)
	}
	if schema.SchemaVersion != 2 || schema.Name != "Design Review" {
		t.Fatalf("schema = %#v, want v2 Design Review", schema)
	}
	if got := schema.Steps[0].Execution; got == nil || got.Kind != "agent" || got.Prompt != "Draft the design." || got.Rules != "Keep it concise." {
		t.Fatalf("first step execution = %#v", got)
	}
	if schema.Steps[0].Artifact == nil || schema.Steps[0].Artifact.Template == nil || schema.Steps[0].Artifact.Template.Content != "# Design" {
		t.Fatalf("first step artifact = %#v", schema.Steps[0].Artifact)
	}
	if schema.Steps[0].Review == nil || !schema.Steps[0].Review.Required {
		t.Fatalf("review = %#v, want required", schema.Steps[0].Review)
	}
	if schema.Steps[0].QualityGate == nil || schema.Steps[0].QualityGate.ReportMode != "full" {
		t.Fatalf("quality gate = %#v", schema.Steps[0].QualityGate)
	}
	if got := schema.Steps[1].Execution; got == nil || got.Kind != "manual" {
		t.Fatalf("second step execution = %#v, want manual", got)
	}
	joined := strings.Join(result.Warnings, "\n")
	for _, want := range []string{"AUTOMATION", "party_mode_discuss", "pre_added_agents"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("warnings %q missing %q", joined, want)
		}
	}
}

func TestExportSchemaYAMLRoundTripsThroughImport(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"schema_version": 2,
		"name": "Round Trip",
		"applicability": ["assignment"],
		"source": {"format": "markdown", "body_template": "Run"},
		"steps": [{"id": "run", "title": "Run", "execution": {"kind": "agent"}}]
	}`)
	exported, err := ExportSchema(raw, "yaml")
	if err != nil {
		t.Fatalf("ExportSchema returned error: %v", err)
	}
	if exported.Format != "yaml" || !strings.Contains(exported.Content, "schema_version") {
		t.Fatalf("exported = %#v", exported)
	}
	imported, err := ImportSchema(exported.Format, []byte(exported.Content), "", "")
	if err != nil {
		t.Fatalf("ImportSchema(exported) returned error: %v", err)
	}
	var schema Schema
	if err := json.Unmarshal(imported.Schema, &schema); err != nil {
		t.Fatalf("unmarshal imported schema: %v", err)
	}
	if schema.Name != "Round Trip" || len(schema.Steps) != 1 || schema.Steps[0].ID != "run" {
		t.Fatalf("round-tripped schema = %#v", schema)
	}
}

func TestRenderBuildsMarkdownFromStructuredSteps(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"schema_version": 1,
		"name": "Step Workflow",
		"applicability": ["assignment"],
		"source": {"format": "markdown"},
		"steps": [
			{
				"id": "context",
				"name": "Context",
				"body_template": "Read {{issue_id}}.",
				"checklist": ["Open the issue"]
			}
		]
	}`)
	result := Render(raw, RenderContext{IssueID: "MUL-123"})
	for _, want := range []string{
		"## Step Workflow",
		"**Context**",
		"Read MUL-123.",
		"Open the issue",
	} {
		if !strings.Contains(result.Markdown, want) {
			t.Fatalf("rendered markdown missing %q\n---\n%s", want, result.Markdown)
		}
	}
}

func TestSystemSeedsIncludeGraphSteps(t *testing.T) {
	t.Parallel()

	for _, seed := range SystemSeeds() {
		seed := seed
		t.Run(seed.Key, func(t *testing.T) {
			t.Parallel()

			var schema Schema
			if err := json.Unmarshal(seed.Schema, &schema); err != nil {
				t.Fatalf("unmarshal seed schema: %v", err)
			}
			if len(schema.Steps) == 0 {
				t.Fatalf("system seed %q has no graph steps", seed.Key)
			}
			if err := validateSteps(schema.Steps); err != nil {
				t.Fatalf("system seed %q has invalid graph steps: %v", seed.Key, err)
			}
			hasEdge := false
			for _, step := range schema.Steps {
				if len(step.DependsOn) > 0 {
					hasEdge = true
					break
				}
			}
			if !hasEdge {
				t.Fatalf("system seed %q has no dependency edges", seed.Key)
			}
		})
	}
}
