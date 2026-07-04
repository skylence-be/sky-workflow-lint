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

// writeBashFileFixture writes content as <dir>/<name>.sky and returns the dir and
// the .sky path. Scripts referenced by bash_file are written into the same dir by
// the caller so relative resolution against filepath.Dir(path) works.
func writeBashFileFixture(t *testing.T, name, content string) (dir, path string) {
	t.Helper()
	dir = t.TempDir()
	path = filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return dir, path
}

const bashFileOnlyWorkflow = `⊕meta⊕
name = "bashfile-only"
trigger.manual = true
⊕⊕

§gate§
bash_file = "./gate.sh"
§§
`

const bashFileBothWorkflow = `⊕meta⊕
name = "bashfile-both"
trigger.manual = true
⊕⊕

§run§
bash = "echo hi"
bash_file = "./run.sh"
§§
`

const bashFileMissingWorkflow = `⊕meta⊕
name = "bashfile-missing"
trigger.manual = true
⊕⊕

§gate§
bash_file = "./nonexistent.sh"
§§
`

// ── parser-level behaviour ────────────────────────────────────────────────────

func TestParse_BashFileOnly_OK(t *testing.T) {
	t.Parallel()
	wf, err := Parse(bytes.NewReader([]byte(bashFileOnlyWorkflow)))
	if err != nil {
		t.Fatalf("bash_file-only node should parse, got error: %v", err)
	}
	if len(wf.Nodes) != 1 {
		t.Fatalf("want 1 node, got %d", len(wf.Nodes))
	}
	n := wf.Nodes[0]
	if n.BashFile != "./gate.sh" {
		t.Errorf("BashFile = %q, want ./gate.sh", n.BashFile)
	}
	if n.NodeType() != "bash" {
		t.Errorf("NodeType() = %q, want bash", n.NodeType())
	}
}

func TestParse_BashAndBashFile_Rejected(t *testing.T) {
	t.Parallel()
	_, err := Parse(bytes.NewReader([]byte(bashFileBothWorkflow)))
	if err == nil {
		t.Fatal("setting both bash and bash_file should be rejected")
	}
	var se *skyerr.Error
	if !errors.As(err, &se) {
		t.Fatalf("want *skyerr.Error, got %T: %v", err, err)
	}
	if se.Code != skyerr.ErrNodeAmbiguousAction {
		t.Errorf("code = %q, want %q (SKY-WF-024)", se.Code, skyerr.ErrNodeAmbiguousAction)
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error %q should explain mutual exclusion", err.Error())
	}
}

// ── ValidateBashFile / Lint integration ───────────────────────────────────────

func TestValidateBashFile_MissingFile_SKY_WF_100(t *testing.T) {
	t.Parallel()
	_, path := writeBashFileFixture(t, "missing.sky", bashFileMissingWorkflow)
	diags, err := Lint(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasCode(diags, "SKY-WF-100") {
		t.Errorf("want SKY-WF-100 for missing bash_file, got %+v", diags)
	}
	for _, d := range diags {
		if d.Code == "SKY-WF-100" && !strings.Contains(d.Message, "nonexistent.sh") {
			t.Errorf("SKY-WF-100 message %q should mention the missing file", d.Message)
		}
	}
}

func TestValidateBashFile_ExistingFile_NoBlockingDiags(t *testing.T) {
	t.Parallel()
	dir, path := writeBashFileFixture(t, "valid.sky", bashFileOnlyWorkflow)
	if err := os.WriteFile(filepath.Join(dir, "gate.sh"), []byte("#!/bin/bash\necho ok\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	diags, err := Lint(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, d := range diags {
		if d.Code == "SKY-WF-099" || d.Code == "SKY-WF-100" {
			t.Errorf("unexpected blocking diag: %+v", d)
		}
		if d.Code == "SKY-WF-101" {
			t.Errorf("valid script should not produce SKY-WF-101: %+v", d)
		}
	}
}

func TestValidateBashFile_SyntaxError_SKY_WF_101_Warning(t *testing.T) {
	t.Parallel()
	dir, path := writeBashFileFixture(t, "broken.sky", bashFileOnlyWorkflow)
	// `(((` is an unclosed arithmetic expansion — rejected by mvdan.cc/sh.
	if err := os.WriteFile(filepath.Join(dir, "gate.sh"), []byte("#!/bin/bash\n(((\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	diags, err := Lint(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var found bool
	for _, d := range diags {
		if d.Code == "SKY-WF-101" && strings.Contains(d.Message, "syntax error") {
			found = true
			if d.Severity != "warning" {
				t.Errorf("SKY-WF-101 severity = %q, want warning (non-blocking)", d.Severity)
			}
		}
	}
	if !found {
		t.Errorf("want SKY-WF-101 syntax error diag, got %+v", diags)
	}
}

// TestValidateBashFile_BothSet_SKY_WF_099 exercises the belt-and-suspenders lint
// check on a *Workflow built directly (Parse rejects both-set before lint sees it).
func TestValidateBashFile_BothSet_SKY_WF_099(t *testing.T) {
	t.Parallel()
	wf := &Workflow{Nodes: []Node{{ID: "run", Bash: "echo hi", BashFile: "./run.sh"}}}
	diags := ValidateBashFile(wf, "/tmp/x.sky")
	if len(diags) != 1 {
		t.Fatalf("want 1 diag, got %d: %+v", len(diags), diags)
	}
	if diags[0].Code != "SKY-WF-099" {
		t.Errorf("code = %q, want SKY-WF-099", diags[0].Code)
	}
	if !strings.Contains(diags[0].Message, "run") {
		t.Errorf("message %q should mention node id", diags[0].Message)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func hasCode(diags []Diagnostic, code string) bool {
	for _, d := range diags {
		if d.Code == code {
			return true
		}
	}
	return false
}
