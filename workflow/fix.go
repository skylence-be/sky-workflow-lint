package workflow

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/skylence-be/skyerr"
)

// Confidence indicates how safe a fix is to auto-apply.
type Confidence string

const (
	ConfidenceHigh   Confidence = "high"
	ConfidenceMedium Confidence = "medium"
	ConfidenceLow    Confidence = "low"
)

// Position is a 0-based line and character offset (LSP TextEdit convention).
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// EditRange is a half-open [Start, End) range within a text document.
type EditRange struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// TextEdit replaces the text within Range with NewText.
type TextEdit struct {
	Range   EditRange `json:"range"`
	NewText string    `json:"new_text"`
}

// Fix describes a machine-applicable repair for a single diagnostic.
type Fix struct {
	Code            string     `json:"code"`
	File            string     `json:"file"`
	DiagnosticIndex int        `json:"diagnostic_index"`
	Title           string     `json:"title"`
	Confidence      Confidence `json:"confidence"`
	Edits           []TextEdit `json:"edits"`
}

// proposeFix returns a Fix for the given diagnostic, or nil if no fix is available.
func proposeFix(file string, raw []byte, diagIdx int, diag Diagnostic) *Fix {
	switch diag.Code {
	case string(skyerr.ErrTemplateUnsafeSplice):
		return fixWF047(file, raw, diagIdx, diag)
	case string(skyerr.ErrSecretUndeclared):
		return fixWF055(file, raw, diagIdx, diag)
	case string(skyerr.ErrInteractiveStrictConflict):
		return fixWF059(file, raw, diagIdx, diag)
	case string(skyerr.ErrPromptUltraKeywordNoOp):
		return mediumStub(file, diagIdx, diag, "Remove or replace ultra keyword that has no effect under sky run")
	case string(skyerr.ErrLearningsConfig):
		return mediumStub(file, diagIdx, diag, "Fix learnings config (remove conflicting key or correct category name)")
	case string(skyerr.ErrLoopMaxExceeded):
		return mediumStub(file, diagIdx, diag, "Reduce loop.max to the allowed cap")
	case string(skyerr.ErrSafetyClassMissing):
		return mediumStub(file, diagIdx, diag, "Add safety = \"requires_permission\" to the node")
	case string(skyerr.ErrOutputStyleInvalid):
		return mediumStub(file, diagIdx, diag, "Set output_style to \"terse\" or remove it")
	case string(skyerr.ErrIsolationInvalid):
		return mediumStub(file, diagIdx, diag, "Set claude.isolation to \"strict\" or \"loose\" or remove it")
	}
	return nil
}

// ProposeFixes returns a Fix for each diagnostic that has a known repair.
func ProposeFixes(file string, raw []byte, diags []Diagnostic) []Fix {
	var fixes []Fix
	for i, d := range diags {
		if f := proposeFix(file, raw, i, d); f != nil {
			fixes = append(fixes, *f)
		}
	}
	return fixes
}

func mediumStub(file string, diagIdx int, diag Diagnostic, title string) *Fix {
	return &Fix{
		Code:            diag.Code,
		File:            file,
		DiagnosticIndex: diagIdx,
		Title:           title,
		Confidence:      ConfidenceMedium,
		Edits:           []TextEdit{},
	}
}

// bareVarInJSONStringRe matches a bare {{var}} (no filter) inside a JSON string literal.
// Used by fixWF047 to locate the exact placeholder on the flagged line.
var bareVarInJSONStringRe = regexp.MustCompile(`\{\{\s*([\w.\-]+)\s*\}\}`)

// fixWF047 adds a |json filter to bare {{var}} placeholders inside JSON string literals.
// WF-047 diagnostics carry no line number (reported per-node), so we scan the full file
// for the first bare placeholder. When a line number is present we start there.
func fixWF047(file string, raw []byte, diagIdx int, diag Diagnostic) *Fix {
	lines := strings.Split(string(raw), "\n")

	var lineIdx int
	var line string
	var loc []int

	startIdx := 0
	if diag.Line > 0 {
		startIdx = diag.Line - 1
	}
	for i := startIdx; i < len(lines); i++ {
		if l2 := bareVarInJSONStringRe.FindStringIndex(lines[i]); l2 != nil {
			lineIdx = i
			line = lines[i]
			loc = l2
			break
		}
	}
	if loc == nil {
		return nil
	}

	match := line[loc[0]:loc[1]]
	// Extract var name from {{name}} pattern.
	groups := bareVarInJSONStringRe.FindStringSubmatch(match)
	if groups == nil {
		return nil
	}
	varName := groups[1]
	newText := fmt.Sprintf("{{%s|json}}", varName)

	return &Fix{
		Code:            diag.Code,
		File:            file,
		DiagnosticIndex: diagIdx,
		Title:           "Add |json filter to template placeholder inside JSON string",
		Confidence:      ConfidenceHigh,
		Edits: []TextEdit{
			{
				Range: EditRange{
					Start: Position{Line: lineIdx, Character: loc[0]},
					End:   Position{Line: lineIdx, Character: loc[1]},
				},
				NewText: newText,
			},
		},
	}
}

// secretNameFromMessage extracts the secret name from a WF-055 message like
// "${env:NAME} is not declared in the workflow secrets list".
var secretNameRe = regexp.MustCompile(`\$\{env:(\w+)\}`)

// fixWF055 appends the undeclared secret to the secrets = [...] list in meta,
// or adds the key if it does not exist yet.
func fixWF055(file string, raw []byte, diagIdx int, diag Diagnostic) *Fix {
	m := secretNameRe.FindStringSubmatch(diag.Message)
	if m == nil {
		return nil
	}
	secretName := m[1]

	metaLine, ok := locateSection(raw, symMeta, metaName)
	if !ok {
		return nil
	}
	closeL, ok := locateSectionClose(raw, metaLine, symMeta)
	if !ok {
		return nil
	}

	lines := strings.Split(string(raw), "\n")

	// Look for an existing secrets = [...] line in the meta section.
	for i := metaLine + 1; i < closeL && i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		eqIdx := strings.Index(trimmed, "=")
		if eqIdx < 0 {
			continue
		}
		if strings.TrimSpace(trimmed[:eqIdx]) != "secrets" {
			continue
		}
		// Found an existing secrets = [...] line. Append the new name inside the brackets.
		rawLine := lines[i]
		openBracket := strings.Index(rawLine, "[")
		closeBracket := strings.LastIndex(rawLine, "]")
		if openBracket < 0 || closeBracket < 0 {
			// Multi-line secrets not supported; fall back to inserting a new line.
			break
		}
		inside := strings.TrimSpace(rawLine[openBracket+1 : closeBracket])
		return &Fix{
			Code:            diag.Code,
			File:            file,
			DiagnosticIndex: diagIdx,
			Title:           fmt.Sprintf("Add %q to secrets list in meta section", secretName),
			Confidence:      ConfidenceHigh,
			Edits: []TextEdit{
				{
					Range: EditRange{
						Start: Position{Line: i, Character: openBracket},
						End:   Position{Line: i, Character: closeBracket + 1},
					},
					NewText: inside2slice(inside, secretName),
				},
			},
		}
	}

	// No existing secrets key; insert one before the close marker.
	indent := lineIndent(raw, metaLine+1)
	if indent == "" {
		indent = lineIndent(raw, closeL-1)
	}
	newAssign := fmt.Sprintf("%ssecrets = [%q]", indent, secretName)

	return &Fix{
		Code:            diag.Code,
		File:            file,
		DiagnosticIndex: diagIdx,
		Title:           fmt.Sprintf("Add %q to secrets list in meta section", secretName),
		Confidence:      ConfidenceHigh,
		Edits: []TextEdit{
			{
				Range: EditRange{
					Start: Position{Line: closeL, Character: 0},
					End:   Position{Line: closeL, Character: 0},
				},
				NewText: newAssign + "\n",
			},
		},
	}
}

// inside2slice returns a new `["a", "b", "c"]` literal with secretName appended.
func inside2slice(existing, newName string) string {
	// existing is the trimmed content between [ and ], e.g. `"FOO", "BAR"`.
	if existing == "" {
		return fmt.Sprintf("[%q]", newName)
	}
	return fmt.Sprintf("[%s, %q]", existing, newName)
}

// fixWF059 adds `claude.isolation = "loose"` to the meta section so that
// permissions = "interactive" nodes work correctly.
func fixWF059(file string, raw []byte, diagIdx int, diag Diagnostic) *Fix {
	metaLine, ok := locateSection(raw, symMeta, metaName)
	if !ok {
		return nil
	}
	closeL, ok := locateSectionClose(raw, metaLine, symMeta)
	if !ok {
		return nil
	}

	// If claude.isolation is already set, replace it.
	if assignLine, startChar, endChar, found := locateAssign(raw, metaLine, symMeta, "claude.isolation"); found {
		return &Fix{
			Code:            diag.Code,
			File:            file,
			DiagnosticIndex: diagIdx,
			Title:           "Set claude.isolation = \"loose\" in meta section",
			Confidence:      ConfidenceHigh,
			Edits: []TextEdit{
				{
					Range: EditRange{
						Start: Position{Line: assignLine, Character: startChar},
						End:   Position{Line: assignLine, Character: endChar},
					},
					NewText: `"loose"`,
				},
			},
		}
	}

	// Insert a new claude.isolation line before the close marker.
	indent := lineIndent(raw, metaLine+1)
	if indent == "" {
		indent = lineIndent(raw, closeL-1)
	}
	newAssign := fmt.Sprintf(`%sclaude.isolation = "loose"`, indent)

	return &Fix{
		Code:            diag.Code,
		File:            file,
		DiagnosticIndex: diagIdx,
		Title:           "Add claude.isolation = \"loose\" to meta section",
		Confidence:      ConfidenceHigh,
		Edits: []TextEdit{
			{
				Range: EditRange{
					Start: Position{Line: closeL, Character: 0},
					End:   Position{Line: closeL, Character: 0},
				},
				NewText: newAssign + "\n",
			},
		},
	}
}
