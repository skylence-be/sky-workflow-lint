package workflow

import "testing"

func TestValidateTemplates(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantHits int
	}{
		{
			name:     "bare var in JSON string literal flagged",
			body:     `{"t": "{{issue.title}}"}`,
			wantHits: 1,
		},
		{
			name:     "json filter unwrapped not flagged",
			body:     `{"t": {{issue.title|json}}}`,
			wantHits: 0,
		},
		{
			name:     "bare var in escaped-quote string flagged",
			body:     `{"t": "he said \"{{x}}\""}`,
			wantHits: 1,
		},
		{
			name:     "multiple bare vars in one body = one issue per node",
			body:     `{"a": "{{x}}", "b": "{{y}}"}`,
			wantHits: 1,
		},
		{
			name:     "plain JSON with no templates",
			body:     `{"t": "hello"}`,
			wantHits: 0,
		},
		{
			name:     "non-JSON body skipped by heuristic",
			body:     `"{{x}}"`,
			wantHits: 0,
		},
		{
			name:     "array body with bare var in string flagged",
			body:     `[{"t": "{{x}}"}]`,
			wantHits: 1,
		},
		{
			name:     "bare var outside string (value position) not flagged",
			body:     `{"n": {{count}}}`,
			wantHits: 0,
		},
		{
			name:     "empty body",
			body:     ``,
			wantHits: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wf := &Workflow{
				Nodes: []Node{{
					ID:   "notify",
					HTTP: &HTTPConfig{URL: "https://example.com", Body: tt.body},
				}},
			}
			issues := ValidateTemplates(wf)
			if len(issues) != tt.wantHits {
				t.Fatalf("got %d issues, want %d: %+v", len(issues), tt.wantHits, issues)
			}
			if tt.wantHits == 1 && issues[0].NodeID != "notify" {
				t.Errorf("issue NodeID = %q, want %q", issues[0].NodeID, "notify")
			}
		})
	}
}

func TestValidateTemplatesSkipsNonHTTPNodes(t *testing.T) {
	wf := &Workflow{
		Nodes: []Node{
			{ID: "a", Prompt: `{"t": "{{x}}"}`},
			{ID: "b", Bash: `echo {"t": "{{x}}"}`},
		},
	}
	if got := ValidateTemplates(wf); len(got) != 0 {
		t.Errorf("got %d issues, want 0: %+v", len(got), got)
	}
}
