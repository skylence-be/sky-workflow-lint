package workflow

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/skyway-harness-builder/skyerr"
)

// ── ResolveUILocales ──────────────────────────────────────────────────────────

func TestResolveUILocales_SingleFileMdForm(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "review.md"), []byte("# Review"), 0o600); err != nil {
		t.Fatal(err)
	}

	locales, err := ResolveUILocales(dir, "review.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(locales) != 1 {
		t.Fatalf("want 1 locale, got %d: %v", len(locales), locales)
	}
	got, ok := locales["default"]
	if !ok {
		t.Fatal("want locale \"default\", not found")
	}
	if !strings.HasSuffix(got, "review.md") {
		t.Errorf("default path = %q, want suffix review.md", got)
	}
}

func TestResolveUILocales_ExtensionlessLocaleSet(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Write two locale files.
	for _, name := range []string{"card.en.md", "card.nl.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("# "+name), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	locales, err := ResolveUILocales(dir, "card")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(locales) != 2 {
		t.Fatalf("want 2 locales, got %d: %v", len(locales), locales)
	}
	for _, lang := range []string{"en", "nl"} {
		p, ok := locales[lang]
		if !ok {
			t.Errorf("want locale %q in result, not found; locales = %v", lang, locales)
			continue
		}
		want := "card." + lang + ".md"
		if !strings.HasSuffix(p, want) {
			t.Errorf("locale %q path = %q, want suffix %q", lang, p, want)
		}
	}
}

func TestResolveUILocales_ExtensionlessBareDefaultLocale(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Bare <ui>.md should appear as "default" in addition to locale files.
	for _, name := range []string{"doc.md", "doc.fr.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("# "+name), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	locales, err := ResolveUILocales(dir, "doc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := locales["default"]; !ok {
		t.Errorf("want \"default\" locale for bare doc.md; got locales = %v", locales)
	}
	if _, ok := locales["fr"]; !ok {
		t.Errorf("want \"fr\" locale for doc.fr.md; got locales = %v", locales)
	}
}

func TestResolveUILocales_NoFiles_EmptyMap(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// No files written — extensionless form resolves to nothing.
	locales, err := ResolveUILocales(dir, "ghost")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(locales) != 0 {
		t.Errorf("want empty map, got %v", locales)
	}
}

// ── ValidateUI ────────────────────────────────────────────────────────────────

func TestValidateUI_MissingFiles_SKY_WF_103_Warning(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// wfPath within the temp dir; no ui markdown files written.
	wfPath := filepath.Join(dir, "workflow.sky")

	wf := &Workflow{
		Nodes: []Node{
			{ID: "review", UI: "ui/card"},
		},
	}
	diags := ValidateUI(wf, wfPath)
	if !hasCode(diags, "SKY-WF-103") {
		t.Fatalf("want SKY-WF-103 when ui resolves to no files, got %+v", diags)
	}
	for _, d := range diags {
		if d.Code != "SKY-WF-103" {
			continue
		}
		if d.Severity != "warning" {
			t.Errorf("SKY-WF-103 severity = %q, want \"warning\"", d.Severity)
		}
		if !strings.Contains(d.Message, "review") {
			t.Errorf("message %q should mention node id \"review\"", d.Message)
		}
		if !strings.Contains(d.Message, "ui/card") {
			t.Errorf("message %q should mention ui path \"ui/card\"", d.Message)
		}
	}
}

func TestValidateUI_ExistingFile_NoDiag(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	wfPath := filepath.Join(dir, "workflow.sky")
	if err := os.WriteFile(filepath.Join(dir, "card.md"), []byte("# Card"), 0o600); err != nil {
		t.Fatal(err)
	}

	wf := &Workflow{
		Nodes: []Node{
			{ID: "review", UI: "card.md"},
		},
	}
	diags := ValidateUI(wf, wfPath)
	if hasCode(diags, "SKY-WF-103") {
		t.Errorf("existing ui file should not produce SKY-WF-103, got %+v", diags)
	}
}

func TestValidateUI_NilWorkflow_NodiagNoPanic(t *testing.T) {
	t.Parallel()
	diags := ValidateUI(nil, "/tmp/x.sky")
	if diags != nil {
		t.Errorf("nil workflow: want nil diags, got %+v", diags)
	}
}

func TestValidateUI_NoNodes_NilResult(t *testing.T) {
	t.Parallel()
	diags := ValidateUI(&Workflow{}, "/tmp/x.sky")
	if diags != nil {
		t.Errorf("empty nodes: want nil diags, got %+v", diags)
	}
}

// ── ValidateUI: workflow-level ui ────────────────────────────────────────────

func TestValidateUI_WorkflowUI_MissingFiles_SKY_WF_103(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	wfPath := filepath.Join(dir, "workflow.sky")

	wf := &Workflow{
		UI: "ui/card", // no files on disk
	}
	diags := ValidateUI(wf, wfPath)
	if !hasCode(diags, "SKY-WF-103") {
		t.Fatalf("want SKY-WF-103 when workflow ui resolves to no files, got %+v", diags)
	}
	for _, d := range diags {
		if d.Code != "SKY-WF-103" {
			continue
		}
		if d.Severity != "warning" {
			t.Errorf("SKY-WF-103 severity = %q, want \"warning\"", d.Severity)
		}
		if !strings.Contains(d.Message, "ui/card") {
			t.Errorf("message %q should mention ui path \"ui/card\"", d.Message)
		}
	}
}

func TestValidateUI_WorkflowUI_ExistingFile_NoDiag(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	wfPath := filepath.Join(dir, "workflow.sky")
	if err := os.WriteFile(filepath.Join(dir, "overview.md"), []byte("# Overview"), 0o600); err != nil {
		t.Fatal(err)
	}

	wf := &Workflow{
		UI: "overview.md",
	}
	diags := ValidateUI(wf, wfPath)
	if hasCode(diags, "SKY-WF-103") {
		t.Errorf("existing workflow ui file should not produce SKY-WF-103, got %+v", diags)
	}
}

// ── parser: workflow-level ui path safety (validate) ─────────────────────────

func TestParse_WorkflowUI_DotDotEscape_Rejected(t *testing.T) {
	t.Parallel()
	src := `⊕meta⊕
name = "wf-ui-escape"
trigger.manual = true
ui = "../escape.md"
⊕⊕

§n§
prompt = "do something"
§§
`
	_, err := Parse(bytes.NewReader([]byte(src)))
	if err == nil {
		t.Fatal("workflow ui=\"../escape.md\" should be rejected by validate()")
	}
	var se *skyerr.Error
	if !errors.As(err, &se) {
		t.Fatalf("want *skyerr.Error, got %T: %v", err, err)
	}
	if se.Code != skyerr.ErrWorkflowValidation {
		t.Errorf("code = %q, want ErrWorkflowValidation", se.Code)
	}
	if !strings.Contains(err.Error(), "ui must be a relative path") {
		t.Errorf("error %q should explain that ui must be relative", err.Error())
	}
}

func TestParse_WorkflowUI_AbsolutePath_Rejected(t *testing.T) {
	t.Parallel()
	src := `⊕meta⊕
name = "wf-ui-abs"
trigger.manual = true
ui = "/etc/passwd"
⊕⊕

§n§
prompt = "do something"
§§
`
	_, err := Parse(bytes.NewReader([]byte(src)))
	if err == nil {
		t.Fatal("absolute workflow ui path should be rejected by validate()")
	}
	if !strings.Contains(err.Error(), "ui must be a relative path") {
		t.Errorf("error %q should explain that ui must be relative", err.Error())
	}
}

func TestParse_WorkflowUI_ValidRelativePath_BindsToWorkflowUI(t *testing.T) {
	t.Parallel()
	src := `⊕meta⊕
name = "wf-ui-valid"
trigger.manual = true
ui = "docs/overview.md"
⊕⊕

§n§
prompt = "do something"
§§
`
	wf, err := Parse(bytes.NewReader([]byte(src)))
	if err != nil {
		t.Fatalf("valid relative workflow ui should parse, got error: %v", err)
	}
	if wf.UI != "docs/overview.md" {
		t.Errorf("Workflow.UI = %q, want \"docs/overview.md\"", wf.UI)
	}
}

// ── Lint integration (SKY-WF-103 via LintBytes) ───────────────────────────────

const uiMissingWorkflow = `⊕meta⊕
name = "ui-missing"
trigger.manual = true
⊕⊕

§review§
prompt = "review the code"
ui = "ui/nonexistent"
§§
`

func TestLint_UIMissingFiles_SKY_WF_103(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "workflow.sky")
	if err := os.WriteFile(path, []byte(uiMissingWorkflow), 0o600); err != nil {
		t.Fatal(err)
	}
	diags, err := Lint(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasCode(diags, "SKY-WF-103") {
		t.Errorf("want SKY-WF-103 for missing ui files, got %+v", diags)
	}
	for _, d := range diags {
		if d.Code == "SKY-WF-103" && d.Severity != "warning" {
			t.Errorf("SKY-WF-103 must be non-blocking (severity=\"warning\"), got %q", d.Severity)
		}
	}
}

// ── parser: ui path safety (validate) ────────────────────────────────────────

func TestParse_UI_DotDotEscape_Rejected(t *testing.T) {
	t.Parallel()
	src := `⊕meta⊕
name = "ui-escape"
trigger.manual = true
⊕⊕

§n§
prompt = "do something"
ui = "../escape.md"
§§
`
	_, err := Parse(bytes.NewReader([]byte(src)))
	if err == nil {
		t.Fatal("ui=\"../escape.md\" should be rejected by validate()")
	}
	var se *skyerr.Error
	if !errors.As(err, &se) {
		t.Fatalf("want *skyerr.Error, got %T: %v", err, err)
	}
	if se.Code != skyerr.ErrWorkflowValidation {
		t.Errorf("code = %q, want ErrWorkflowValidation", se.Code)
	}
	if !strings.Contains(err.Error(), "ui must be a relative path") {
		t.Errorf("error %q should explain that ui must be relative", err.Error())
	}
}

func TestParse_UI_AbsolutePath_Rejected(t *testing.T) {
	t.Parallel()
	src := `⊕meta⊕
name = "ui-abs"
trigger.manual = true
⊕⊕

§n§
prompt = "do something"
ui = "/etc/passwd"
§§
`
	_, err := Parse(bytes.NewReader([]byte(src)))
	if err == nil {
		t.Fatal("absolute ui path should be rejected by validate()")
	}
	if !strings.Contains(err.Error(), "ui must be a relative path") {
		t.Errorf("error %q should explain that ui must be relative", err.Error())
	}
}

func TestParse_UI_ValidRelativePath_BindsToNodeUI(t *testing.T) {
	t.Parallel()
	src := `⊕meta⊕
name = "ui-valid"
trigger.manual = true
⊕⊕

§n§
prompt = "do something"
ui = "docs/card.md"
§§
`
	wf, err := Parse(bytes.NewReader([]byte(src)))
	if err != nil {
		t.Fatalf("valid relative ui should parse, got error: %v", err)
	}
	if len(wf.Nodes) != 1 {
		t.Fatalf("want 1 node, got %d", len(wf.Nodes))
	}
	if wf.Nodes[0].UI != "docs/card.md" {
		t.Errorf("Node.UI = %q, want \"docs/card.md\"", wf.Nodes[0].UI)
	}
}
