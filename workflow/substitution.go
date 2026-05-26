package workflow

import (
	"encoding/json"
	"net/url"
	"regexp"
)

// runDocPathRe matches the literal token $run_doc_path with a word boundary so
// it cannot be swallowed by OutputRefRe (which requires ".output" after the id)
// and does not match longer identifiers like $run_doc_paths.
var runDocPathRe = regexp.MustCompile(`\$run_doc_path\b`)

// OutputRefRe matches $node-id.output and $node-id.output.field.subfield.
// Group 1 is the node ID; group 2 is the optional dotted field path (including leading dot) or empty.
var OutputRefRe = regexp.MustCompile(`\$([\w-]+)\.output((?:\.[\w-]+)*)`)

// placeholderRe matches {{name}} and {{name|filter}}, with optional whitespace
// around name and filter. Group 1 is the var name, group 2 is the filter name
// (empty if no filter).
var placeholderRe = regexp.MustCompile(`\{\{\s*([\w.\-]+)(?:\s*\|\s*(\w+))?\s*\}\}`)

// SubstituteRunDocPath replaces $run_doc_path in text with docPath.
// When docPath is empty the text is returned unchanged.
// Resolved independently of OutputRefRe — $run_doc_path never matches that
// pattern because OutputRefRe requires ".output" after the identifier.
func SubstituteRunDocPath(text, docPath string) string {
	if docPath == "" {
		return text
	}
	return runDocPathRe.ReplaceAllLiteralString(text, docPath)
}

// Expand replaces {{key}} and {{key|filter}} placeholders in the template.
// {{key|json}} emits a complete quoted JSON value — author unwrapped:
// `{"t": {{v|json}}}`. Missing var or unknown filter leaves the placeholder
// literal (matches pre-filter missing-var semantics).
func Expand(template string, vars map[string]string) string {
	return placeholderRe.ReplaceAllStringFunc(template, func(match string) string {
		groups := placeholderRe.FindStringSubmatch(match)
		name, filter := groups[1], groups[2]
		value, ok := vars[name]
		if !ok {
			return match
		}
		switch filter {
		case "":
			return value
		case "json":
			encoded, err := json.Marshal(value)
			if err != nil {
				return match
			}
			return string(encoded)
		case "urlencode":
			return url.QueryEscape(value)
		default:
			return match
		}
	})
}
