package workflow

import "testing"

func TestValidateBashTrust(t *testing.T) {
	tests := []struct {
		name     string
		nodes    []Node
		wantHits int
		wantID   string // when wantHits == 1
	}{
		{
			name: "eval on prompt output flagged",
			nodes: []Node{
				{ID: "plan", Prompt: "make a plan"},
				{ID: "run", DependsOn: []string{"plan"}, Bash: `eval "$SKY_OUTPUT_PLAN"`},
			},
			wantHits: 1,
			wantID:   "run",
		},
		{
			name: "source on prompt output flagged",
			nodes: []Node{
				{ID: "plan", Prompt: "make a plan"},
				{ID: "run", DependsOn: []string{"plan"}, Bash: "echo foo\nsource <(echo $SKY_OUTPUT_PLAN)"},
			},
			wantHits: 1,
			wantID:   "run",
		},
		{
			name: "bash -c on command output flagged",
			nodes: []Node{
				{ID: "cmd", Command: "plan"},
				{ID: "run", DependsOn: []string{"cmd"}, Bash: `bash -c "$SKY_OUTPUT_CMD"`},
			},
			wantHits: 1,
			wantID:   "run",
		},
		{
			name: "sh -c on prompt output flagged",
			nodes: []Node{
				{ID: "gen", Prompt: "make code"},
				{ID: "run", DependsOn: []string{"gen"}, Bash: `sh -c "$SKY_OUTPUT_GEN"`},
			},
			wantHits: 1,
			wantID:   "run",
		},
		{
			name: "posix dot-source on prompt output flagged",
			nodes: []Node{
				{ID: "plan", Prompt: "make a plan"},
				{ID: "run", DependsOn: []string{"plan"}, Bash: ". <(printf %s \"$SKY_OUTPUT_PLAN\")"},
			},
			wantHits: 1,
			wantID:   "run",
		},
		{
			name: "hyphenated prompt id maps via underscore",
			nodes: []Node{
				{ID: "fix-plan", Prompt: "make a plan"},
				{ID: "run", DependsOn: []string{"fix-plan"}, Bash: `eval "$SKY_OUTPUT_FIX_PLAN"`},
			},
			wantHits: 1,
			wantID:   "run",
		},
		{
			name: "sink inside loop.until.bash flagged",
			nodes: []Node{
				{ID: "plan", Prompt: "make a plan"},
				{
					ID:        "loop",
					DependsOn: []string{"plan"},
					Bash:      "echo iter",
					Loop: &LoopConfig{
						Until: LoopCondition{Bash: `eval "$SKY_OUTPUT_PLAN"`},
					},
				},
			},
			wantHits: 1,
			wantID:   "loop",
		},
		// ── Clean cases ──
		{
			name: "echo of prompt output — no sink, not flagged",
			nodes: []Node{
				{ID: "plan", Prompt: "make a plan"},
				{ID: "run", DependsOn: []string{"plan"}, Bash: `echo "$SKY_OUTPUT_PLAN" > /tmp/plan.txt`},
			},
			wantHits: 0,
		},
		{
			name: "eval on bash-node output — bash output is trusted, not flagged",
			nodes: []Node{
				{ID: "gen", Bash: "echo 'ls'"},
				{ID: "run", DependsOn: []string{"gen"}, Bash: `eval "$SKY_OUTPUT_GEN"`},
			},
			wantHits: 0,
		},
		{
			name: "eval on http-node output — not a prompt, not flagged",
			nodes: []Node{
				{ID: "api", HTTP: &HTTPConfig{URL: "https://example.com"}},
				{ID: "run", DependsOn: []string{"api"}, Bash: `eval "$SKY_OUTPUT_API"`},
			},
			wantHits: 0,
		},
		{
			name: "sink only, no prompt env ref — not flagged",
			nodes: []Node{
				{ID: "plan", Prompt: "make a plan"},
				{ID: "run", DependsOn: []string{"plan"}, Bash: `eval "$(date)"`},
			},
			wantHits: 0,
		},
		{
			name: "prompt env ref only, no sink — not flagged",
			nodes: []Node{
				{ID: "plan", Prompt: "make a plan"},
				{ID: "run", DependsOn: []string{"plan"}, Bash: `curl -d "$SKY_OUTPUT_PLAN" https://example.com`},
			},
			wantHits: 0,
		},
		{
			name: "sink in comment — stripped, not flagged",
			nodes: []Node{
				{ID: "plan", Prompt: "make a plan"},
				{ID: "run", DependsOn: []string{"plan"}, Bash: "# eval \"$SKY_OUTPUT_PLAN\"  -- just a comment\necho ok"},
			},
			wantHits: 0,
		},
		{
			name: "sink substring inside word — not flagged (boundary required)",
			nodes: []Node{
				{ID: "plan", Prompt: "make a plan"},
				{ID: "run", DependsOn: []string{"plan"}, Bash: `myevalwrapper "$SKY_OUTPUT_PLAN"`},
			},
			wantHits: 0,
		},
		{
			name: "no prompt nodes in workflow — short-circuit, not flagged",
			nodes: []Node{
				{ID: "run", Bash: `eval "$(date)"`},
			},
			wantHits: 0,
		},
		{
			name: "sink after && at statement boundary — flagged",
			nodes: []Node{
				{ID: "plan", Prompt: "make a plan"},
				{ID: "run", DependsOn: []string{"plan"}, Bash: `set -e && eval "$SKY_OUTPUT_PLAN"`},
			},
			wantHits: 1,
			wantID:   "run",
		},
		{
			name: "sink after pipe at statement boundary — flagged",
			nodes: []Node{
				{ID: "plan", Prompt: "make a plan"},
				{ID: "run", DependsOn: []string{"plan"}, Bash: `echo "$SKY_OUTPUT_PLAN" | bash -c cat`},
			},
			wantHits: 1,
			wantID:   "run",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wf := &Workflow{Nodes: tt.nodes}
			issues := ValidateBashTrust(wf)
			if len(issues) != tt.wantHits {
				t.Fatalf("got %d issues, want %d: %+v", len(issues), tt.wantHits, issues)
			}
			if tt.wantHits == 1 && issues[0].NodeID != tt.wantID {
				t.Errorf("issue NodeID = %q, want %q", issues[0].NodeID, tt.wantID)
			}
		})
	}
}

func TestValidateBashTrust_NilAndEmpty(t *testing.T) {
	if got := ValidateBashTrust(nil); got != nil {
		t.Errorf("nil workflow: got %+v, want nil", got)
	}
	if got := ValidateBashTrust(&Workflow{}); got != nil {
		t.Errorf("empty workflow: got %+v, want nil", got)
	}
}
