package workflow

import (
	"os"
	"path/filepath"
	"testing"
)

func writeLintFixture(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.sky")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

func TestLint_Valid(t *testing.T) {
	path := writeLintFixture(t, `⊕meta⊕
name = "lint-valid"
trigger.manual = true
⊕⊕

§do-thing§
bash = "echo hello"
§§
`)
	diags, err := Lint(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(diags) != 0 {
		t.Errorf("want no diagnostics, got %+v", diags)
	}
}

func TestLint_ParseFailure(t *testing.T) {
	// No meta section → Parse returns an error.
	path := writeLintFixture(t, `§handle§
bash = "echo done"
§§
`)
	diags, err := Lint(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(diags) == 0 {
		t.Fatal("want diagnostic for missing meta, got none")
	}
	if diags[0].File != path {
		t.Errorf("file = %q, want %q", diags[0].File, path)
	}
	if diags[0].Code == "" {
		t.Errorf("want non-empty code, got empty")
	}
}

func TestLint_SecretUndeclared(t *testing.T) {
	path := writeLintFixture(t, `⊕meta⊕
name = "lint-secret"
trigger.manual = true
⊕⊕

§call-api§
http.url = "https://api.example.com?key=${env:TOKEN}"
§§
`)
	diags, err := Lint(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(diags) == 0 {
		t.Fatal("want undeclared-secret diagnostic, got none")
	}
	found := false
	for _, d := range diags {
		if d.Code == "SKY-WF-055" {
			found = true
		}
	}
	if !found {
		t.Errorf("want code SKY-WF-055 in diagnostics, got %+v", diags)
	}
}

func TestLint_UltraKeyword(t *testing.T) {
	path := writeLintFixture(t, `⊕meta⊕
name = "lint-ultra"
trigger.manual = true
⊕⊕

§plan-step§
prompt = "ultraplan the migration"
§§
`)
	diags, err := Lint(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(diags) == 0 {
		t.Fatal("want ultra-keyword diagnostic, got none")
	}
	found := false
	for _, d := range diags {
		if d.Code == "SKY-WF-061" {
			found = true
		}
	}
	if !found {
		t.Errorf("want code SKY-WF-061 in diagnostics, got %+v", diags)
	}
}

func TestLint_InvokeDynamicTarget(t *testing.T) {
	path := writeLintFixture(t, `⊕meta⊕
name = "invoke-dynamic"
trigger.manual = true
⊕⊕

§call§
invoke.target = "{{wf_name}}"
§§
`)
	diags, err := Lint(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, d := range diags {
		if d.Code == "SKY-WF-064" {
			found = true
		}
	}
	if !found {
		t.Errorf("want code SKY-WF-064 in diagnostics, got %+v", diags)
	}
}

func TestLint_InvokeSelfTarget(t *testing.T) {
	path := writeLintFixture(t, `⊕meta⊕
name = "invoke-self"
trigger.manual = true
⊕⊕

§call§
invoke.target = "invoke-self"
§§
`)
	diags, err := Lint(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, d := range diags {
		if d.Code == "SKY-WF-066" {
			found = true
		}
	}
	if !found {
		t.Errorf("want code SKY-WF-066 in diagnostics, got %+v", diags)
	}
}

func TestLint_InvokeInsideLoop(t *testing.T) {
	path := writeLintFixture(t, `⊕meta⊕
name = "invoke-in-loop"
trigger.manual = true
⊕⊕

§looper§
bash = "echo x"
loop.until.bash = "true"
invoke.target = "other-wf"
§§
`)
	diags, err := Lint(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, d := range diags {
		if d.Code == "SKY-WF-065" {
			found = true
		}
	}
	if !found {
		t.Errorf("want code SKY-WF-065 in diagnostics, got %+v", diags)
	}
}

func TestLint_FileNotFound(t *testing.T) {
	_, err := Lint(filepath.Join(t.TempDir(), "no-such-file.sky"))
	if err == nil {
		t.Fatal("want error for missing file, got nil")
	}
}

func TestLineFromErr(t *testing.T) {
	cases := []struct {
		input   string
		wantN   int
		wantMsg string
	}{
		{"line 5: unexpected token", 5, "unexpected token"},
		{"line 12: ", 12, ""},
		{"no line prefix here", 0, "no line prefix here"},
		{"line abc: bad", 0, "line abc: bad"},
		// "line 99" — no colon after digits → no match
		{"line 99", 0, "line 99"},
		// leading prefix shorter than "line " → no match
		{"lin 3: x", 0, "lin 3: x"},
	}
	for _, tc := range cases {
		n, msg := lineFromErr(tc.input)
		if n != tc.wantN || msg != tc.wantMsg {
			t.Errorf("lineFromErr(%q) = (%d, %q), want (%d, %q)",
				tc.input, n, msg, tc.wantN, tc.wantMsg)
		}
	}
}

func TestLint_OutputFormatInvalidSchema(t *testing.T) {
	path := writeLintFixture(t, `⊕meta⊕
name = "lint-output-format"
trigger.manual = true
⊕⊕

§do-thing§
bash = "echo hello"
output_format = {"type": "not-a-valid-type"}
§§
`)
	diags, err := Lint(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, d := range diags {
		if d.Code == "SKY-WF-060" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("want SKY-WF-060 diagnostic for invalid output_format, got %+v", diags)
	}
}

func TestLint_OutputFormatValidSchema(t *testing.T) {
	path := writeLintFixture(t, `⊕meta⊕
name = "lint-output-format-valid"
trigger.manual = true
⊕⊕

§do-thing§
bash = "echo hello"
output_format = {"type": "object"}
§§
`)
	diags, err := Lint(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, d := range diags {
		if d.Code == "SKY-WF-060" {
			t.Errorf("unexpected SKY-WF-060 for valid output_format: %s", d.Message)
		}
	}
}
