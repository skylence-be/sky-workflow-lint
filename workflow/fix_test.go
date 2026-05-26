package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeLintFixtureBytes(t *testing.T, content string) (path string, raw []byte) {
	t.Helper()
	path = filepath.Join(t.TempDir(), "test.sky")
	raw = []byte(content)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path, raw
}

// TestFixWF047_BareVarInJSONString verifies that a bare {{var}} inside an http.body
// JSON string literal produces a high-confidence fix adding |json.
func TestFixWF047_BareVarInJSONString(t *testing.T) {
	t.Parallel()
	content := `⊕meta⊕
name = "fix-047"
trigger.manual = true
⊕⊕

§call-api§
http.url = "https://example.com"
http.body = "{\"title\": \"{{issue.title}}\"}"
§§
`
	path, raw := writeLintFixtureBytes(t, content)
	diags, err := Lint(path)
	if err != nil {
		t.Fatalf("lint: %v", err)
	}
	var wf047 *Diagnostic
	for i := range diags {
		if diags[i].Code == "SKY-WF-047" {
			wf047 = &diags[i]
			break
		}
	}
	if wf047 == nil {
		t.Fatalf("expected SKY-WF-047 diagnostic, got %+v", diags)
	}

	fixes := ProposeFixes(path, raw, diags)
	var got *Fix
	for i := range fixes {
		if fixes[i].Code == "SKY-WF-047" {
			got = &fixes[i]
			break
		}
	}
	if got == nil {
		t.Fatal("expected Fix for SKY-WF-047, got none")
	}
	if got.Confidence != ConfidenceHigh {
		t.Errorf("confidence = %q, want %q", got.Confidence, ConfidenceHigh)
	}
	if len(got.Edits) == 0 {
		t.Fatal("expected at least one edit")
	}
	edit := got.Edits[0]
	if edit.NewText != "{{issue.title|json}}" {
		t.Errorf("new_text = %q, want %q", edit.NewText, "{{issue.title|json}}")
	}
}

// TestFixWF055_UndeclaredSecret verifies that a missing secret reference produces a
// high-confidence fix adding the secret name to the meta section.
func TestFixWF055_UndeclaredSecret(t *testing.T) {
	t.Parallel()
	content := `⊕meta⊕
name = "fix-055"
trigger.manual = true
⊕⊕

§call-api§
http.url = "https://api.example.com?key=${env:TOKEN}"
§§
`
	path, raw := writeLintFixtureBytes(t, content)
	diags, err := Lint(path)
	if err != nil {
		t.Fatalf("lint: %v", err)
	}
	var wf055 *Diagnostic
	for i := range diags {
		if diags[i].Code == "SKY-WF-055" {
			wf055 = &diags[i]
			break
		}
	}
	if wf055 == nil {
		t.Fatalf("expected SKY-WF-055 diagnostic, got %+v", diags)
	}

	fixes := ProposeFixes(path, raw, diags)
	var got *Fix
	for i := range fixes {
		if fixes[i].Code == "SKY-WF-055" {
			got = &fixes[i]
			break
		}
	}
	if got == nil {
		t.Fatal("expected Fix for SKY-WF-055, got none")
	}
	if got.Confidence != ConfidenceHigh {
		t.Errorf("confidence = %q, want %q", got.Confidence, ConfidenceHigh)
	}
	if len(got.Edits) == 0 {
		t.Fatal("expected at least one edit")
	}
	// The edit should mention TOKEN somewhere.
	found := false
	for _, e := range got.Edits {
		if contains(e.NewText, "TOKEN") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected edit containing TOKEN, got edits: %+v", got.Edits)
	}
}

// TestFixWF055_AppendToExistingList verifies appending a new secret to an existing list.
func TestFixWF055_AppendToExistingList(t *testing.T) {
	t.Parallel()
	content := `⊕meta⊕
name = "fix-055-append"
trigger.manual = true
secrets = ["EXISTING"]
⊕⊕

§call-api§
http.url = "https://api.example.com?key=${env:NEW_SECRET}"
§§
`
	path, raw := writeLintFixtureBytes(t, content)
	diags, err := Lint(path)
	if err != nil {
		t.Fatalf("lint: %v", err)
	}
	fixes := ProposeFixes(path, raw, diags)
	var got *Fix
	for i := range fixes {
		if fixes[i].Code == "SKY-WF-055" {
			got = &fixes[i]
			break
		}
	}
	if got == nil {
		t.Fatal("expected Fix for SKY-WF-055, got none")
	}
	if got.Confidence != ConfidenceHigh {
		t.Errorf("confidence = %q, want %q", got.Confidence, ConfidenceHigh)
	}
	// The edit should contain both EXISTING and NEW_SECRET.
	found := false
	for _, e := range got.Edits {
		if contains(e.NewText, "EXISTING") && contains(e.NewText, "NEW_SECRET") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected edit containing both EXISTING and NEW_SECRET, got: %+v", got.Edits)
	}
}

// TestFixWF059_InteractiveStrictConflict verifies that a permissions="interactive"
// node without loose isolation produces a high-confidence fix adding claude.isolation.
func TestFixWF059_InteractiveStrictConflict(t *testing.T) {
	t.Parallel()
	content := `⊕meta⊕
name = "fix-059"
trigger.manual = true
⊕⊕

§do-thing§
bash = "echo hi"
permissions = "interactive"
§§
`
	path, raw := writeLintFixtureBytes(t, content)
	diags, err := Lint(path)
	if err != nil {
		t.Fatalf("lint: %v", err)
	}
	var wf059 *Diagnostic
	for i := range diags {
		if diags[i].Code == "SKY-WF-059" {
			wf059 = &diags[i]
			break
		}
	}
	if wf059 == nil {
		t.Fatalf("expected SKY-WF-059 diagnostic, got %+v", diags)
	}

	fixes := ProposeFixes(path, raw, diags)
	var got *Fix
	for i := range fixes {
		if fixes[i].Code == "SKY-WF-059" {
			got = &fixes[i]
			break
		}
	}
	if got == nil {
		t.Fatal("expected Fix for SKY-WF-059, got none")
	}
	if got.Confidence != ConfidenceHigh {
		t.Errorf("confidence = %q, want %q", got.Confidence, ConfidenceHigh)
	}
	if len(got.Edits) == 0 {
		t.Fatal("expected at least one edit")
	}
	found := false
	for _, e := range got.Edits {
		if contains(e.NewText, "loose") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected edit containing 'loose', got: %+v", got.Edits)
	}
}

// TestFix_Idempotency verifies that a clean file produces zero fixes.
func TestFix_Idempotency(t *testing.T) {
	t.Parallel()
	content := `⊕meta⊕
name = "fix-clean"
trigger.manual = true
secrets = ["TOKEN"]
⊕⊕

§call-api§
http.url = "https://api.example.com?key=${env:TOKEN}"
§§
`
	path, raw := writeLintFixtureBytes(t, content)
	diags, err := Lint(path)
	if err != nil {
		t.Fatalf("lint: %v", err)
	}
	if len(diags) != 0 {
		t.Fatalf("expected clean file, got diagnostics: %+v", diags)
	}
	fixes := ProposeFixes(path, raw, diags)
	if len(fixes) != 0 {
		t.Errorf("expected zero fixes for clean file, got %d: %+v", len(fixes), fixes)
	}
}

// TestFix_MediumStubs verifies that medium-confidence stubs are produced with
// no edits and confidence="medium".
func TestFix_MediumStubs(t *testing.T) {
	t.Parallel()
	content := `⊕meta⊕
name = "fix-ultra"
trigger.manual = true
⊕⊕

§plan-step§
prompt = "ultraplan the migration"
§§
`
	path, raw := writeLintFixtureBytes(t, content)
	diags, err := Lint(path)
	if err != nil {
		t.Fatalf("lint: %v", err)
	}
	var wf061 *Diagnostic
	for i := range diags {
		if diags[i].Code == "SKY-WF-061" {
			wf061 = &diags[i]
			break
		}
	}
	if wf061 == nil {
		t.Fatalf("expected SKY-WF-061, got %+v", diags)
	}

	fixes := ProposeFixes(path, raw, diags)
	var got *Fix
	for i := range fixes {
		if fixes[i].Code == "SKY-WF-061" {
			got = &fixes[i]
			break
		}
	}
	if got == nil {
		t.Fatal("expected Fix for SKY-WF-061")
	}
	if got.Confidence != ConfidenceMedium {
		t.Errorf("confidence = %q, want %q", got.Confidence, ConfidenceMedium)
	}
}

// TestFix_ConflictBothProposed verifies that when two WF-055 diagnostics fire
// (two undeclared secrets, no existing secrets list), ProposeFixes proposes a
// separate fix for each. Both fixes target the meta-close line at character 0
// (zero-width insert). The conflict is resolved by applyFixes at the cmd layer;
// here we only verify both are proposed independently with high confidence.
func TestFix_ConflictBothProposed(t *testing.T) {
	t.Parallel()
	content := `⊕meta⊕
name = "fix-conflict"
trigger.manual = true
⊕⊕

§step1§
http.url = "https://api.example.com?key=${env:TOKEN1}"
§§

§step2§
http.url = "https://api.example.com?key=${env:TOKEN2}"
§§
`
	path, raw := writeLintFixtureBytes(t, content)
	diags, err := Lint(path)
	if err != nil {
		t.Fatalf("lint: %v", err)
	}

	count055 := 0
	for _, d := range diags {
		if d.Code == "SKY-WF-055" {
			count055++
		}
	}
	if count055 < 2 {
		t.Fatalf("expected at least 2 SKY-WF-055 diagnostics, got %d: %+v", count055, diags)
	}

	fixes := ProposeFixes(path, raw, diags)

	// Should produce a separate fix for each WF-055 diagnostic.
	fix055count := 0
	for _, f := range fixes {
		if f.Code != "SKY-WF-055" {
			continue
		}
		fix055count++
		if f.Confidence != ConfidenceHigh {
			t.Errorf("fix %d: confidence = %q, want high", fix055count, f.Confidence)
		}
		if len(f.Edits) == 0 {
			t.Errorf("fix %d: expected at least one edit", fix055count)
		}
	}
	if fix055count < 2 {
		t.Errorf("expected at least 2 fixes for SKY-WF-055, got %d", fix055count)
	}

	// Both edits target the same line (meta-close at char 0): this is the conflict
	// that applyFixes must resolve by keeping the first and recording the second as skipped.
	var metaCloseTargets int
	for _, f := range fixes {
		if f.Code != "SKY-WF-055" {
			continue
		}
		for _, e := range f.Edits {
			if e.Range.Start.Character == 0 && e.Range.End.Character == 0 {
				metaCloseTargets++
			}
		}
	}
	if metaCloseTargets < 2 {
		t.Errorf("expected at least 2 zero-width meta-close inserts (conflict candidates), got %d", metaCloseTargets)
	}
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}
