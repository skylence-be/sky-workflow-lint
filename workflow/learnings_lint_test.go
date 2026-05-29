package workflow

import (
	"testing"

	"github.com/skylence-be/skyerr"
)

func TestValidateLearningsConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		wf       *Workflow
		wantCode skyerr.Code
		wantNone bool
	}{
		{name: "nil workflow", wf: nil, wantNone: true},
		{
			name:     "no learnings config",
			wf:       &Workflow{Name: "test", Nodes: []Node{{ID: "n1", Prompt: "do work"}}},
			wantNone: true,
		},
		{
			name:     "valid workflow-level exclude",
			wf:       &Workflow{Name: "test", Learnings: &LearningsConfig{Exclude: []string{"patterns"}}},
			wantNone: true,
		},
		{
			name:     "valid workflow-level only",
			wf:       &Workflow{Name: "test", Learnings: &LearningsConfig{Only: []string{"execution-rules", "patterns"}}},
			wantNone: true,
		},
		{name: "valid max_bytes -1", wf: &Workflow{Name: "t", Learnings: &LearningsConfig{MaxBytes: -1}}, wantNone: true},
		{name: "valid max_bytes 0", wf: &Workflow{Name: "t", Learnings: &LearningsConfig{MaxBytes: 0}}, wantNone: true},
		{name: "valid max_bytes 1", wf: &Workflow{Name: "t", Learnings: &LearningsConfig{MaxBytes: 1}}, wantNone: true},
		{name: "valid max_bytes 1048576", wf: &Workflow{Name: "t", Learnings: &LearningsConfig{MaxBytes: 1048576}}, wantNone: true},
		{
			name:     "all valid categories accepted",
			wf:       &Workflow{Name: "t", Learnings: &LearningsConfig{Exclude: []string{"execution-rules", "patterns", "anti-patterns", "project-notes"}}},
			wantNone: true,
		},
		{
			name: "WF-062 exclude and only both set on workflow",
			wf: &Workflow{
				Name:      "test",
				Learnings: &LearningsConfig{Exclude: []string{"patterns"}, Only: []string{"execution-rules"}},
			},
			wantCode: skyerr.ErrLearningsConfig,
		},
		{
			name:     "WF-062 unknown category in exclude",
			wf:       &Workflow{Name: "test", Learnings: &LearningsConfig{Exclude: []string{"unknown-cat"}}},
			wantCode: skyerr.ErrLearningsConfig,
		},
		{
			name:     "WF-062 unknown category in only",
			wf:       &Workflow{Name: "test", Learnings: &LearningsConfig{Only: []string{"typo-patterns"}}},
			wantCode: skyerr.ErrLearningsConfig,
		},
		{
			name:     "WF-062 max_bytes out of range (negative non-minus-one)",
			wf:       &Workflow{Name: "test", Learnings: &LearningsConfig{MaxBytes: -2}},
			wantCode: skyerr.ErrLearningsConfig,
		},
		{
			name:     "WF-062 max_bytes over 1 MiB",
			wf:       &Workflow{Name: "test", Learnings: &LearningsConfig{MaxBytes: 1048577}},
			wantCode: skyerr.ErrLearningsConfig,
		},
		{
			name: "WF-062 node-level exclude and only both set",
			wf: &Workflow{
				Name: "test",
				Nodes: []Node{{
					ID:        "n1",
					Learnings: &LearningsConfig{Exclude: []string{"patterns"}, Only: []string{"execution-rules"}},
				}},
			},
			wantCode: skyerr.ErrLearningsConfig,
		},
		{
			name: "WF-062 node-level unknown category",
			wf: &Workflow{
				Name:  "test",
				Nodes: []Node{{ID: "n1", Learnings: &LearningsConfig{Only: []string{"not-a-category"}}}},
			},
			wantCode: skyerr.ErrLearningsConfig,
		},
		{
			name: "WF-062 workflow and node both invalid — multiple issues all SKY-WF-062",
			wf: &Workflow{
				Name:      "test",
				Learnings: &LearningsConfig{MaxBytes: -99},
				Nodes: []Node{{
					ID:        "n1",
					Learnings: &LearningsConfig{Exclude: []string{"bad"}, Only: []string{"exec"}},
				}},
			},
			wantCode: skyerr.ErrLearningsConfig,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			issues := ValidateLearningsConfig(tc.wf)
			if tc.wantNone {
				if len(issues) != 0 {
					t.Errorf("expected no issues, got %d: %v", len(issues), issues)
				}
				return
			}
			if len(issues) == 0 {
				t.Fatal("expected at least one issue, got none")
			}
			for _, iss := range issues {
				if iss.Code != tc.wantCode {
					t.Errorf("expected code %q, got %q (message: %s)", tc.wantCode, iss.Code, iss.Message)
				}
			}
		})
	}
}
