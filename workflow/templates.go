package workflow

import (
	"regexp"
	"strings"
)

// jsonStringLiteralRe matches a JSON string literal, respecting \" and \\ escapes.
var jsonStringLiteralRe = regexp.MustCompile(`"(?:[^"\\]|\\.)*"`)

// bareVarRe matches {{ name }} — without a `|filter` segment. The negative
// lookahead pattern is emulated by requiring the closer `}}` to follow the
// name directly (modulo whitespace), with no `|`. `\w.-` covers dotted and
// hyphenated var names (e.g. `issue.number`, `repo.full_name`, `repo-name`).
var bareVarRe = regexp.MustCompile(`\{\{\s*[\w.\-]+\s*\}\}`)

// ValidateTemplates flags bare `{{var}}` placeholders that appear inside JSON
// string literals in `http.body`. These are the exact shape that produces
// injection-class bugs when a webhook-controlled value contains `"` or `\`.
//
// Authors should write `{"t": {{v|json}}}` (unwrapped filter), not
// `{"t": "{{v}}"}` (bare var inside a string literal).
//
// One issue is reported per offending node, not per occurrence.
func ValidateTemplates(wf *Workflow) []SchemaIssue {
	var issues []SchemaIssue
	for _, n := range wf.Nodes {
		if n.HTTP == nil {
			continue
		}
		body := strings.TrimSpace(n.HTTP.Body)
		if body == "" {
			continue
		}
		if body[0] != '{' && body[0] != '[' {
			continue
		}
		flagged := false
		for _, sm := range jsonStringLiteralRe.FindAllStringIndex(n.HTTP.Body, -1) {
			if bareVarRe.MatchString(n.HTTP.Body[sm[0]:sm[1]]) {
				flagged = true
				break
			}
		}
		if flagged {
			issues = append(issues, SchemaIssue{
				NodeID:  n.ID,
				Pointer: "/nodes/" + n.ID + "/http/body",
				Message: "bare {{var}} inside JSON string literal in http.body — use {{var|json}} unwrapped: `{\"k\": {{var|json}}}`",
			})
		}
	}
	return issues
}
