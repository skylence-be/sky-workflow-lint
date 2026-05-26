package lintfmt

import (
	"path/filepath"
)

// LintFixEnvelope is the JSON wrapper emitted when --fix is set.
// It replaces the bare diagnostics array so fixes can be included in the same response.
type LintFixEnvelope struct {
	Diagnostics          []LintDiagnostic `json:"diagnostics"`
	Fixes                []LintFix        `json:"fixes"`
	SkippedDueToConflict []LintFix        `json:"skipped_due_to_conflict,omitempty"`
}

// LintFix is the JSON representation of a workflow.Fix, kept in the output package
// to avoid an import cycle (cmd -> output, output -> workflow would be circular if
// workflow imports output). The cmd layer converts workflow.Fix -> LintFix.
type LintFix struct {
	Code            string     `json:"code"`
	File            string     `json:"file"`
	DiagnosticIndex int        `json:"diagnostic_index"`
	Title           string     `json:"title"`
	Confidence      string     `json:"confidence"`
	Edits           []LintEdit `json:"edits"`
}

// LintEdit is a single LSP-style text edit (0-based line/character ranges).
type LintEdit struct {
	Range   LintEditRange `json:"range"`
	NewText string        `json:"new_text"`
}

// LintEditRange is the half-open [Start, End) range for a LintEdit.
type LintEditRange struct {
	Start LintPosition `json:"start"`
	End   LintPosition `json:"end"`
}

// LintPosition is a 0-based line and character offset.
type LintPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// LintDiagnostic is a single sky lint finding, shared between text/JSON/SARIF output.
type LintDiagnostic struct {
	File    string `json:"file"`
	Line    int    `json:"line,omitempty"`
	Message string `json:"message"`
	Rule    string `json:"rule,omitempty"`
	Tier    string `json:"tier,omitempty"`
}

// SARIF 2.1.0 minimal struct set for sky lint output.
// Only the fields emitted by LintSARIF are represented.

type sarifLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name  string      `json:"name"`
	Rules []sarifRule `json:"rules,omitempty"`
}

type sarifRule struct {
	ID string `json:"id"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId,omitempty"`
	Level     string          `json:"level"`
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations,omitempty"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysLoc `json:"physicalLocation"`
}

type sarifPhysLoc struct {
	ArtifactLocation sarifArtifact `json:"artifactLocation"`
	Region           *sarifRegion  `json:"region,omitempty"`
}

type sarifArtifact struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine int `json:"startLine"`
}

// LintSARIF converts lint diagnostics to a SARIF 2.1.0 log.
// repoRoot is used to normalise file URIs to repo-relative paths.
// Pass "" to use paths as-is.
func LintSARIF(diags []LintDiagnostic, repoRoot string) sarifLog {
	rulesSeen := map[string]bool{}
	var rules []sarifRule
	results := make([]sarifResult, 0, len(diags))

	for _, d := range diags {
		uri := d.File
		if repoRoot != "" {
			rel, err := filepath.Rel(repoRoot, d.File)
			if err == nil {
				uri = filepath.ToSlash(rel)
			}
		}

		if d.Rule != "" && !rulesSeen[d.Rule] {
			rules = append(rules, sarifRule{ID: d.Rule})
			rulesSeen[d.Rule] = true
		}

		res := sarifResult{
			RuleID:  d.Rule,
			Level:   "error",
			Message: sarifMessage{Text: d.Message},
		}
		loc := sarifLocation{
			PhysicalLocation: sarifPhysLoc{
				ArtifactLocation: sarifArtifact{URI: uri},
			},
		}
		if d.Line > 0 {
			loc.PhysicalLocation.Region = &sarifRegion{StartLine: d.Line}
		}
		res.Locations = []sarifLocation{loc}
		results = append(results, res)
	}

	if results == nil {
		results = []sarifResult{}
	}

	return sarifLog{
		Schema:  "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json",
		Version: "2.1.0",
		Runs: []sarifRun{{
			Tool: sarifTool{
				Driver: sarifDriver{
					Name:  "sky-lint",
					Rules: rules,
				},
			},
			Results: results,
		}},
	}
}
