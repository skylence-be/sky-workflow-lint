package workflow

import (
	"strings"
	"testing"
)

func TestExpand(t *testing.T) {
	tests := []struct {
		name     string
		template string
		vars     map[string]string
		want     string
	}{
		{
			name:     "single var",
			template: "Fix issue #{{issue.number}}",
			vars:     map[string]string{"issue.number": "42"},
			want:     "Fix issue #42",
		},
		{
			name:     "multiple vars",
			template: "{{issue.number}}: {{issue.title}}",
			vars:     map[string]string{"issue.number": "7", "issue.title": "Bug in auth"},
			want:     "7: Bug in auth",
		},
		{
			name:     "no vars",
			template: "Do something",
			vars:     map[string]string{},
			want:     "Do something",
		},
		{
			name:     "missing var unchanged",
			template: "Issue {{issue.number}}",
			vars:     map[string]string{},
			want:     "Issue {{issue.number}}",
		},
		{
			name:     "repeated var",
			template: "{{x}} and {{x}}",
			vars:     map[string]string{"x": "hello"},
			want:     "hello and hello",
		},
		{
			name:     "hyphenated var",
			template: "{{repo-name}}",
			vars:     map[string]string{"repo-name": "skylence"},
			want:     "skylence",
		},
		{
			name:     "json filter escapes quote",
			template: `{"title": {{issue.title|json}}}`,
			vars:     map[string]string{"issue.title": `he said "hi"`},
			want:     `{"title": "he said \"hi\""}`,
		},
		{
			name:     "json filter escapes backslash",
			template: `{{path|json}}`,
			vars:     map[string]string{"path": `C:\tmp`},
			want:     `"C:\\tmp"`,
		},
		{
			name:     "json filter escapes newline",
			template: `{{body|json}}`,
			vars:     map[string]string{"body": "line1\nline2"},
			want:     `"line1\nline2"`,
		},
		{
			name:     "json filter escapes control char",
			template: `{{s|json}}`,
			vars:     map[string]string{"s": "\x01"},
			want:     `"\u0001"`,
		},
		{
			name:     "json filter missing var literal",
			template: `{{missing|json}}`,
			vars:     map[string]string{},
			want:     `{{missing|json}}`,
		},
		{
			name:     "urlencode filter space",
			template: `{{q|urlencode}}`,
			vars:     map[string]string{"q": "hello world"},
			want:     `hello+world`,
		},
		{
			name:     "urlencode filter metachars",
			template: `{{q|urlencode}}`,
			vars:     map[string]string{"q": "a&b=c"},
			want:     `a%26b%3Dc`,
		},
		{
			name:     "urlencode filter slash",
			template: `{{q|urlencode}}`,
			vars:     map[string]string{"q": "a/b"},
			want:     `a%2Fb`,
		},
		{
			name:     "whitespace tolerant",
			template: `{{ issue.title | json }}`,
			vars:     map[string]string{"issue.title": "ok"},
			want:     `"ok"`,
		},
		{
			name:     "unknown filter stays literal",
			template: `{{x|bogus}}`,
			vars:     map[string]string{"x": "value"},
			want:     `{{x|bogus}}`,
		},
		{
			name:     "bare var in json body unchanged behavior",
			template: `{"title": "{{issue.title}}"}`,
			vars:     map[string]string{"issue.title": `he said "hi"`},
			want:     `{"title": "he said "hi""}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Expand(tt.template, tt.vars)
			if got != tt.want {
				t.Errorf("Expand() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSubstituteRunDocPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		text    string
		docPath string
		want    string
	}{
		{
			name:    "basic substitution",
			text:    "Write findings to $run_doc_path",
			docPath: "run/01ABC",
			want:    "Write findings to run/01ABC",
		},
		{
			name:    "empty docPath leaves token literal",
			text:    "Write to $run_doc_path",
			docPath: "",
			want:    "Write to $run_doc_path",
		},
		{
			name:    "word boundary — longer identifier untouched",
			text:    "$run_doc_paths is not $run_doc_path",
			docPath: "run/X",
			want:    "$run_doc_paths is not run/X",
		},
		{
			name:    "co-resolution with node output token — no collision",
			text:    "See $run_doc_path and $analyze.output for details",
			docPath: "run/R1",
			want:    "See run/R1 and $analyze.output for details",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SubstituteRunDocPath(tt.text, tt.docPath)
			if got != tt.want {
				t.Errorf("SubstituteRunDocPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestRunDocPath_OutputRef_CoResolution verifies that applying SubstituteRunDocPath
// before OutputRefRe substitution resolves both tokens without collision.
func TestRunDocPath_OutputRef_CoResolution(t *testing.T) {
	t.Parallel()
	text := "Record to $run_doc_path; prior output: $analyze.output"
	docPath := "run/01RUNID"

	// Step 1: resolve $run_doc_path
	after := SubstituteRunDocPath(text, docPath)

	// Step 2: simulate OutputRefRe substitution (as substituteOutputs does)
	nodeOutput := "analysis result"
	after = OutputRefRe.ReplaceAllStringFunc(after, func(match string) string {
		sub := OutputRefRe.FindStringSubmatch(match)
		if sub[1] == "analyze" && sub[2] == "" {
			return nodeOutput
		}
		return match
	})

	wantRunDoc := docPath
	wantOutput := nodeOutput
	if !strings.Contains(after, wantRunDoc) {
		t.Errorf("expected %q in result %q", wantRunDoc, after)
	}
	if !strings.Contains(after, wantOutput) {
		t.Errorf("expected %q in result %q", wantOutput, after)
	}
}
