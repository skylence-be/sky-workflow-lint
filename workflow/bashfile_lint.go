package workflow

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// ValidateBashFile runs bash_file-specific lint checks for a parsed workflow.
//
//   - SKY-WF-099: both bash and bash_file set on the same node. NOTE: the parser
//     (validate) already rejects this combination with ErrNodeAmbiguousAction
//     (SKY-WF-024) at Parse time, so a workflow reaching this lint pass never has
//     both set. The check is kept as belt-and-suspenders for callers that build a
//     *Workflow directly without going through Parse.
//   - SKY-WF-100: bash_file references a script that does not exist on disk.
//   - SKY-WF-101 (warning, non-blocking): bash syntax errors detected by the
//     in-process mvdan.cc/sh parser, plus shellcheck findings when shellcheck is
//     installed. shellcheck absence is silent.
//
// wfPath is the path to the .sky file; its directory resolves relative bash_file
// paths. Pass "" when the file location is unknown (relative paths then resolve
// against the current working directory).
func ValidateBashFile(wf *Workflow, wfPath string) []Diagnostic {
	if wf == nil || len(wf.Nodes) == 0 {
		return nil
	}
	wfDir := filepath.Dir(wfPath)
	var diags []Diagnostic

	for i := range wf.Nodes {
		n := &wf.Nodes[i]
		if n.BashFile == "" {
			continue
		}

		// SKY-WF-099: both bash and bash_file on the same node.
		if n.Bash != "" {
			diags = append(diags, Diagnostic{
				Code:    "SKY-WF-099",
				Message: fmt.Sprintf("node %q: bash and bash_file cannot both be set; use one or the other", n.ID),
			})
			continue // node is already broken — skip existence check
		}

		// Resolve the script path relative to the .sky file's directory.
		scriptPath := n.BashFile
		if !filepath.IsAbs(scriptPath) {
			scriptPath = filepath.Join(wfDir, scriptPath)
		}

		// SKY-WF-100: bash_file references a non-existent file.
		if _, err := os.Stat(scriptPath); err != nil {
			diags = append(diags, Diagnostic{
				Code:    "SKY-WF-100",
				Message: fmt.Sprintf("node %q: bash_file %q does not exist", n.ID, n.BashFile),
			})
			continue
		}

		// SKY-WF-101: in-process syntax check.
		if content, err := os.ReadFile(scriptPath); err == nil {
			diags = append(diags, mvdanSyntaxDiags(n.ID, string(content))...)
		}

		// SKY-WF-101: optional shellcheck semantic checks.
		diags = append(diags, shellcheckDiags(n.ID, scriptPath)...)
	}

	return diags
}

// mvdanSyntaxDiags parses script (bash source text) with mvdan.cc/sh and returns
// SKY-WF-101 diagnostics for any parse errors found.
func mvdanSyntaxDiags(nodeID, script string) []Diagnostic {
	parser := syntax.NewParser(syntax.Variant(syntax.LangBash), syntax.RecoverErrors(10))
	_, err := parser.Parse(strings.NewReader(script), nodeID+".sh")
	if err == nil {
		return nil
	}

	var errs []error
	if multi, ok := err.(interface{ Unwrap() []error }); ok {
		errs = multi.Unwrap()
	} else {
		errs = []error{err}
	}

	var diags []Diagnostic
	for _, e := range errs {
		var pe *syntax.ParseError
		if errors.As(e, &pe) {
			diags = append(diags, Diagnostic{
				Code:     "SKY-WF-101",
				Severity: "warning",
				Message:  fmt.Sprintf("node %q: bash syntax error at line %d: %s", nodeID, pe.Pos.Line(), pe.Text),
			})
		} else {
			diags = append(diags, Diagnostic{
				Code:     "SKY-WF-101",
				Severity: "warning",
				Message:  fmt.Sprintf("node %q: bash syntax error: %s", nodeID, e.Error()),
			})
		}
	}
	return diags
}

// shellcheckDiags runs shellcheck on scriptPath and converts findings into
// SKY-WF-101 warning diagnostics. Returns nil when shellcheck is not on PATH —
// its absence is silent (the in-process mvdan parser covers syntax).
func shellcheckDiags(nodeID, scriptPath string) []Diagnostic {
	if _, err := exec.LookPath("shellcheck"); err != nil {
		return nil
	}

	cmd := exec.Command("shellcheck", "--format=gcc", scriptPath) //nolint:gosec // path is a validated .sh file referenced by a .sky workflow
	out, _ := cmd.CombinedOutput()                                // exit 1 = findings; ignore exit error
	if len(bytes.TrimSpace(out)) == 0 {
		return nil
	}

	var diags []Diagnostic
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		diags = append(diags, Diagnostic{
			Code:     "SKY-WF-101",
			Severity: "warning",
			Message:  fmt.Sprintf("node %q: shellcheck: %s", nodeID, line),
		})
	}
	return diags
}
