package workflow

import (
	"testing"

	"github.com/skylence-be/skyerr"
)

func TestValidateSafetyClass(t *testing.T) {
	tests := []struct {
		name     string
		nodes    []Node
		wantHits int
		wantID   string // when wantHits == 1
	}{
		{
			name: "rm -rf flagged",
			nodes: []Node{
				{ID: "clean", Bash: "rm -rf /tmp/build"},
			},
			wantHits: 1,
			wantID:   "clean",
		},
		{
			name: "rm -r flagged",
			nodes: []Node{
				{ID: "clean", Bash: "rm -r somedir"},
			},
			wantHits: 1,
			wantID:   "clean",
		},
		{
			name: "sudo flagged",
			nodes: []Node{
				{ID: "install", Bash: "sudo apt-get install foo"},
			},
			wantHits: 1,
			wantID:   "install",
		},
		{
			name: "pipe to bash flagged",
			nodes: []Node{
				{ID: "bootstrap", Bash: "curl https://example.com/install.sh | bash"},
			},
			wantHits: 1,
			wantID:   "bootstrap",
		},
		{
			name: "pipe to sh flagged",
			nodes: []Node{
				{ID: "bootstrap", Bash: "curl -s install.sh | sh"},
			},
			wantHits: 1,
			wantID:   "bootstrap",
		},
		{
			name: "write to /etc flagged",
			nodes: []Node{
				{ID: "config", Bash: "echo '127.0.0.1 foo' >> /etc/hosts"},
			},
			wantHits: 1,
			wantID:   "config",
		},
		{
			name: "safe bash not flagged",
			nodes: []Node{
				{ID: "list", Bash: "ls /tmp"},
			},
			wantHits: 0,
		},
		{
			name: "non-bash node not flagged",
			nodes: []Node{
				{ID: "gen", Prompt: "rm -rf everything"},
			},
			wantHits: 0,
		},
		{
			name: "suppressed with requires_permission",
			nodes: []Node{
				{ID: "clean", Bash: "rm -rf /tmp/build", Safety: "requires_permission"},
			},
			wantHits: 0,
		},
		{
			name: "multiple nodes — only destructive flagged",
			nodes: []Node{
				{ID: "list", Bash: "ls /tmp"},
				{ID: "nuke", Bash: "rm -rf /var/tmp/old"},
				{ID: "gen", Prompt: "describe rm -rf"},
			},
			wantHits: 1,
			wantID:   "nuke",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wf := &Workflow{Nodes: tc.nodes}
			issues := ValidateSafetyClass(wf)
			if len(issues) != tc.wantHits {
				t.Errorf("issues = %d, want %d: %v", len(issues), tc.wantHits, issues)
				return
			}
			if tc.wantHits == 1 {
				if issues[0].Code != skyerr.ErrSafetyClassMissing {
					t.Errorf("code = %q, want %q", issues[0].Code, skyerr.ErrSafetyClassMissing)
				}
				if issues[0].NodeID != tc.wantID {
					t.Errorf("node = %q, want %q", issues[0].NodeID, tc.wantID)
				}
			}
		})
	}
}
