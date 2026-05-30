package workflow

import (
	"testing"

	"github.com/skylence-be/skyerr"
)

func TestValidateSecrets(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		wf        *Workflow
		wantCodes []skyerr.Code
		wantNone  bool
	}{
		{
			name:     "nil workflow",
			wf:       nil,
			wantNone: true,
		},
		{
			name: "WF-055 undeclared in http.url",
			wf: &Workflow{
				Name: "test",
				Nodes: []Node{
					{ID: "n1", HTTP: &HTTPConfig{URL: "https://api.test?k=${env:TOKEN}"}},
				},
			},
			wantCodes: []skyerr.Code{skyerr.ErrSecretUndeclared},
		},
		{
			name: "WF-055 undeclared in http.body",
			wf: &Workflow{
				Name: "test",
				Nodes: []Node{
					{ID: "n1", HTTP: &HTTPConfig{
						URL:  "https://api.test",
						Body: `{"key":"${env:BODY_TOKEN}"}`,
					}},
				},
			},
			wantCodes: []skyerr.Code{skyerr.ErrSecretUndeclared},
		},
		{
			name: "WF-055 undeclared in http.headers",
			wf: &Workflow{
				Name: "test",
				Nodes: []Node{
					{ID: "n1", HTTP: &HTTPConfig{
						URL:     "https://api.test",
						Headers: map[string]string{"Authorization": "Bearer ${env:HDR_TOKEN}"},
					}},
				},
			},
			wantCodes: []skyerr.Code{skyerr.ErrSecretUndeclared},
		},
		{
			name: "WF-055 undeclared in mcp_servers",
			wf: &Workflow{
				Name:       "test",
				MCPServers: map[string]any{"mysvc": map[string]any{"token": "${env:MCP_TOKEN}"}}, //nolint:gosec // test fixture, not a real credential
			},
			wantCodes: []skyerr.Code{skyerr.ErrSecretUndeclared},
		},
		{
			name: "declared variable in http.url — no issue",
			wf: &Workflow{
				Name:    "test",
				Secrets: []string{"DECLARED"},
				Nodes: []Node{
					{ID: "n1", HTTP: &HTTPConfig{URL: "https://api.test?k=${env:DECLARED}"}},
				},
			},
			wantNone: true,
		},
		{
			name: "WF-056 empty name in http.url",
			wf: &Workflow{
				Name: "test",
				Nodes: []Node{
					{ID: "n1", HTTP: &HTTPConfig{URL: "https://api.test?k=${env:}"}},
				},
			},
			wantCodes: []skyerr.Code{skyerr.ErrSecretEmptyName},
		},
		{
			name: "WF-057 out-of-scope in prompt",
			wf: &Workflow{
				Name: "test",
				Nodes: []Node{
					{ID: "n1", Prompt: "do ${env:TOKEN}"},
				},
			},
			wantCodes: []skyerr.Code{skyerr.ErrSecretOutOfScope},
		},
		{
			name: "WF-057 out-of-scope in bash",
			wf: &Workflow{
				Name: "test",
				Nodes: []Node{
					{ID: "n1", Bash: "echo ${env:TOKEN}"},
				},
			},
			wantCodes: []skyerr.Code{skyerr.ErrSecretOutOfScope},
		},
		{
			name: "WF-057 out-of-scope in system_prompt",
			wf: &Workflow{
				Name: "test",
				Nodes: []Node{
					{ID: "n1", Prompt: "hi", SystemPrompt: "system ${env:TOKEN}"},
				},
			},
			wantCodes: []skyerr.Code{skyerr.ErrSecretOutOfScope},
		},
		{
			name: "WF-057 out-of-scope in eval.source",
			wf: &Workflow{
				Name: "test",
				Nodes: []Node{
					{ID: "n1", Eval: &EvalConfig{Source: "${env:TOKEN}", Contains: "x"}},
				},
			},
			wantCodes: []skyerr.Code{skyerr.ErrSecretOutOfScope},
		},
		{
			name: "WF-057 out-of-scope in loop.until.bash",
			wf: &Workflow{
				Name: "test",
				Nodes: []Node{
					{
						ID:   "n1",
						Bash: "echo loop",
						Loop: &LoopConfig{Until: LoopCondition{Bash: "check ${env:TOKEN}"}},
					},
				},
			},
			wantCodes: []skyerr.Code{skyerr.ErrSecretOutOfScope},
		},
		{
			name: "WF-057 out-of-scope in wait.prompt",
			wf: &Workflow{
				Name: "test",
				Nodes: []Node{
					{ID: "n1", Wait: &WaitConfig{Prompt: "approve ${env:TOKEN}"}},
				},
			},
			wantCodes: []skyerr.Code{skyerr.ErrSecretOutOfScope},
		},
		{
			name: "WF-057 out-of-scope in approval.prompt",
			wf: &Workflow{
				Name: "test",
				Nodes: []Node{
					{ID: "n1", Approval: &ApprovalConfig{Prompt: "approve ${env:TOKEN}"}},
				},
			},
			wantCodes: []skyerr.Code{skyerr.ErrSecretOutOfScope},
		},
		{
			name: "WF-057 out-of-scope in step.prompt",
			wf: &Workflow{
				Name:  "test",
				Steps: []Step{{Name: "s1", Prompt: "run ${env:TOKEN}"}},
			},
			wantCodes: []skyerr.Code{skyerr.ErrSecretOutOfScope},
		},
		{
			name: "deduplication — same undeclared name twice in http.url",
			wf: &Workflow{
				Name: "test",
				Nodes: []Node{
					{ID: "n1", HTTP: &HTTPConfig{URL: "https://api.test?a=${env:DUP}&b=${env:DUP}"}},
				},
			},
			wantCodes: []skyerr.Code{skyerr.ErrSecretUndeclared},
		},
		{
			name: "multiple issues — undeclared in http.url AND out-of-scope in prompt",
			wf: &Workflow{
				Name: "test",
				Nodes: []Node{
					{ID: "n1", HTTP: &HTTPConfig{URL: "https://api.test?k=${env:UNDECL}"}},
					{ID: "n2", Prompt: "run ${env:INPROMPT}"},
				},
			},
			wantCodes: []skyerr.Code{skyerr.ErrSecretUndeclared, skyerr.ErrSecretOutOfScope},
		},
		{
			name: "declared name in http.headers value — no issue",
			wf: &Workflow{
				Name:    "test",
				Secrets: []string{"DECLARED_HDR"},
				Nodes: []Node{
					{ID: "n1", HTTP: &HTTPConfig{
						URL:     "https://api.test",
						Headers: map[string]string{"Authorization": "Bearer ${env:DECLARED_HDR}"},
					}},
				},
			},
			wantNone: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			issues := ValidateSecrets(tc.wf)
			if tc.wantNone {
				if len(issues) != 0 {
					t.Errorf("expected no issues, got %d: %v", len(issues), issues)
				}
				return
			}
			if len(issues) != len(tc.wantCodes) {
				t.Fatalf("issue count=%d want %d; issues=%v", len(issues), len(tc.wantCodes), issues)
			}
			// Build a set of actual codes to support unordered matching for multi-issue cases.
			got := make(map[skyerr.Code]int, len(issues))
			for _, iss := range issues {
				got[iss.Code]++
			}
			for _, code := range tc.wantCodes {
				if got[code] == 0 {
					t.Errorf("expected code %q not found in issues: %v", code, issues)
				} else {
					got[code]--
				}
			}
		})
	}
}
