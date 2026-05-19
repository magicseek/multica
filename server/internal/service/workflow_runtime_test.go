package service

import (
	"encoding/json"
	"testing"
)

func TestWorkflowInitialStepRunsFromSnapshot(t *testing.T) {
	raw := marshalWorkflowRuntimeTestSnapshot(t, `{
		"schema_version": 1,
		"steps": [
			{"id":"context","title":"Load context"},
			{"id":"manual-review","title":"Review artifact","depends_on":["context"],"execution":{"kind":"manual"},"required":false},
			{"id":"external-check","title":"External check","execution":{"kind":"external"},"artifact":{"inputs":["context:summary"]}}
		]
	}`)

	steps, err := workflowInitialStepRunsFromSnapshot(raw)
	if err != nil {
		t.Fatalf("workflowInitialStepRunsFromSnapshot: %v", err)
	}
	if len(steps) != 3 {
		t.Fatalf("expected 3 step runs, got %d", len(steps))
	}

	if got := steps[0].Status; got != workflowStepStatusReady {
		t.Fatalf("first step status = %q, want %q", got, workflowStepStatusReady)
	}
	if got := steps[0].ExecutionKind; got != workflowStepExecutionAgent {
		t.Fatalf("first step execution kind = %q, want %q", got, workflowStepExecutionAgent)
	}
	if !steps[0].Required {
		t.Fatalf("first step should default to required")
	}

	if got := steps[1].Status; got != workflowStepStatusPending {
		t.Fatalf("manual step with dependency status = %q, want %q", got, workflowStepStatusPending)
	}
	if got := steps[1].ExecutionKind; got != workflowStepExecutionManual {
		t.Fatalf("manual step execution kind = %q, want %q", got, workflowStepExecutionManual)
	}
	if steps[1].Required {
		t.Fatalf("manual step should preserve required=false")
	}
	if got := string(steps[1].DependsOnStepIDs); got != `["context"]` {
		t.Fatalf("manual step dependencies = %s", got)
	}

	if got := steps[2].Status; got != workflowStepStatusWaitingExternal {
		t.Fatalf("external step status = %q, want %q", got, workflowStepStatusWaitingExternal)
	}
	if got := string(steps[2].ArtifactInputs); got != `["context:summary"]` {
		t.Fatalf("external step artifact inputs = %s", got)
	}
}

func TestWorkflowInitialStepRunsFromSnapshotRejectsMissingStepID(t *testing.T) {
	raw := marshalWorkflowRuntimeTestSnapshot(t, `{
		"schema_version": 1,
		"steps": [{"title":"Missing id"}]
	}`)

	if _, err := workflowInitialStepRunsFromSnapshot(raw); err == nil {
		t.Fatalf("expected missing step id to fail")
	}
}

func marshalWorkflowRuntimeTestSnapshot(t *testing.T, schema string) []byte {
	t.Helper()
	raw, err := json.Marshal(WorkflowSnapshot{
		SchemaVersion:    1,
		TriggerType:      "assignment",
		DefinitionID:     "definition-id",
		RevisionID:       "revision-id",
		RevisionNumber:   1,
		WorkflowName:     "Test workflow",
		Origin:           "user",
		Schema:           json.RawMessage(schema),
		RenderedMarkdown: "## Test\n",
	})
	if err != nil {
		t.Fatalf("marshal workflow snapshot: %v", err)
	}
	return raw
}
