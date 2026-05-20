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
	if schema.SchemaVersion != 1 {
		t.Fatalf("schema_version = %d, want 1", schema.SchemaVersion)
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
			raw:     `{"schema_version": 2, "applicability": ["assignment"]}`,
			wantErr: "unsupported schema_version 2",
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

func TestNormalizeDraftSchemaAllowsNonPublishableDraft(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"schema_version": 2,
		"name": "Draft Only",
		"applicability": ["assignment"],
		"steps": [
			{
				"id": "context",
				"title": "Context",
				"required": false,
				"output": {"description": "Context is loaded"},
				"review": {"required": true},
				"quality_gate": {"enabled": true}
			}
		]
	}`)
	normalized, validation, err := NormalizeDraftSchema(raw, "Fallback", "")
	if err != nil {
		t.Fatalf("NormalizeDraftSchema returned error: %v", err)
	}
	if validation.Publishable {
		t.Fatalf("draft validation was publishable, want blocking issue")
	}
	if len(validation.Issues) == 0 || validation.Issues[0].Code != "unsupported_schema_version" {
		t.Fatalf("validation issues = %#v, want unsupported_schema_version", validation.Issues)
	}

	var schema Schema
	if err := json.Unmarshal(normalized, &schema); err != nil {
		t.Fatalf("unmarshal normalized draft: %v", err)
	}
	if schema.Steps[0].Required == nil || *schema.Steps[0].Required {
		t.Fatalf("required = %#v, want explicit false", schema.Steps[0].Required)
	}
	if schema.Steps[0].Output == nil || schema.Steps[0].Output.Description != "Context is loaded" {
		t.Fatalf("output = %#v, want preserved output description", schema.Steps[0].Output)
	}
	if schema.Steps[0].Review == nil || !schema.Steps[0].Review.Required {
		t.Fatalf("review = %#v, want required review", schema.Steps[0].Review)
	}
	if schema.Steps[0].QualityGate == nil || !schema.Steps[0].QualityGate.Enabled {
		t.Fatalf("quality gate = %#v, want enabled", schema.Steps[0].QualityGate)
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
				"output": {"description": "Issue context is loaded"},
				"review": {"required": true},
				"quality_gate": {"enabled": true},
				"checklist": ["Open the issue"]
			}
		]
	}`)
	result := Render(raw, RenderContext{IssueID: "MUL-123"})
	for _, want := range []string{
		"## Step Workflow",
		"**Context**",
		"Read MUL-123.",
		"Done when: Issue context is loaded",
		"Gate: human review required",
		"Gate: quality check required",
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

func TestTrellisSystemSeedInitializesMissingTrellisState(t *testing.T) {
	t.Parallel()

	var trellis Seed
	found := false
	for _, seed := range SystemSeeds() {
		if seed.Key == SystemTrellisTask {
			trellis = seed
			found = true
			break
		}
	}
	if !found {
		t.Fatal("trellis system seed not found")
	}

	var schema Schema
	if err := json.Unmarshal(trellis.Schema, &schema); err != nil {
		t.Fatalf("unmarshal trellis seed schema: %v", err)
	}
	if !strings.Contains(schema.Source.BodyTemplate, "initialize Trellis for that repository first") {
		t.Fatalf("trellis body should initialize missing Trellis state, got:\n%s", schema.Source.BodyTemplate)
	}
	if strings.Contains(schema.Source.BodyTemplate, "fall back to the standard assignment protocol") {
		t.Fatalf("trellis body should not fall back when .trellis is missing:\n%s", schema.Source.BodyTemplate)
	}
	foundSetupGate := false
	for _, step := range schema.Steps {
		if step.ID == "trellis-gate" && strings.Contains(step.Description, "initialize") {
			foundSetupGate = true
			break
		}
	}
	if !foundSetupGate {
		t.Fatalf("trellis seed missing setup gate description: %#v", schema.Steps)
	}
}
