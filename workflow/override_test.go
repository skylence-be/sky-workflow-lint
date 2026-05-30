package workflow_test

import (
	"testing"

	"github.com/skylence-be/sky-workflow-lint/workflow"
)

func baseWF() *workflow.Workflow {
	return &workflow.Workflow{
		Name: "test",
		Nodes: []workflow.Node{
			{ID: "a", Prompt: "original prompt", Model: "sonnet"},
			{ID: "b", Effort: "low"},
		},
	}
}

func TestApplyOverrides_EachField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		field string
		value string
		check func(t *testing.T, n workflow.Node)
	}{
		{
			name:  "model",
			field: "model",
			value: "haiku",
			check: func(t *testing.T, n workflow.Node) {
				t.Helper()
				if n.Model != "haiku" {
					t.Errorf("Model = %q, want %q", n.Model, "haiku")
				}
			},
		},
		{
			name:  "prompt",
			field: "prompt",
			value: "new prompt",
			check: func(t *testing.T, n workflow.Node) {
				t.Helper()
				if n.Prompt != "new prompt" {
					t.Errorf("Prompt = %q, want %q", n.Prompt, "new prompt")
				}
			},
		},
		{
			name:  "effort",
			field: "effort",
			value: "high",
			check: func(t *testing.T, n workflow.Node) {
				t.Helper()
				if n.Effort != "high" {
					t.Errorf("Effort = %q, want %q", n.Effort, "high")
				}
			},
		},
		{
			name:  "isolation",
			field: "isolation",
			value: "worktree",
			check: func(t *testing.T, n workflow.Node) {
				t.Helper()
				if n.Isolation != "worktree" {
					t.Errorf("Isolation = %q, want %q", n.Isolation, "worktree")
				}
			},
		},
		{
			name:  "max_budget_usd",
			field: "max_budget_usd",
			value: "1.5",
			check: func(t *testing.T, n workflow.Node) {
				t.Helper()
				if n.MaxBudget != 1.5 {
					t.Errorf("MaxBudget = %v, want 1.5", n.MaxBudget)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			wf := baseWF()
			err := wf.ApplyOverrides([]workflow.Override{{Node: "a", Field: tc.field, Value: tc.value}})
			if err != nil {
				t.Fatalf("ApplyOverrides: %v", err)
			}
			tc.check(t, wf.Nodes[0])
		})
	}
}

func TestApplyOverrides_UnknownNode(t *testing.T) {
	t.Parallel()
	wf := baseWF()
	err := wf.ApplyOverrides([]workflow.Override{{Node: "z", Field: "model", Value: "haiku"}})
	if err == nil {
		t.Fatal("expected error for unknown node, got nil")
	}
}

func TestApplyOverrides_UnknownField(t *testing.T) {
	t.Parallel()
	wf := baseWF()
	err := wf.ApplyOverrides([]workflow.Override{{Node: "a", Field: "nonexistent", Value: "x"}})
	if err == nil {
		t.Fatal("expected error for unknown field, got nil")
	}
}

func TestApplyOverrides_BadFloat(t *testing.T) {
	t.Parallel()
	wf := baseWF()
	err := wf.ApplyOverrides([]workflow.Override{{Node: "a", Field: "max_budget_usd", Value: "notafloat"}})
	if err == nil {
		t.Fatal("expected error for bad float, got nil")
	}
}
