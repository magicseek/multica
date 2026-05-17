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
