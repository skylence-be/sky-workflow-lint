package workflow

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/skyway-harness-builder/skyerr"
)

var ultraKeywordRe = regexp.MustCompile(`(?i)\b(ultraplan|ultrareview)\b`)

// ValidateUltraKeywords returns a SKY-WF-061 SchemaIssue for each prompt location
// that contains the bare keyword "ultraplan" or "ultrareview". These keywords fire
// only in interactive Claude Code; sky always invokes claude non-interactively via
// -p, so the keywords silently no-op.
func ValidateUltraKeywords(wf *Workflow) []SchemaIssue {
	if wf == nil {
		return nil
	}
	var issues []SchemaIssue
	for _, n := range wf.Nodes {
		id := n.ID
		if n.Script == nil { // script bodies are stored in Node.Prompt — skip to avoid false positives
			if iss, ok := scanUltra(n.Prompt, id, "/nodes/"+id+"/prompt"); ok {
				issues = append(issues, iss)
			}
		}
		if iss, ok := scanUltra(n.SystemPrompt, id, "/nodes/"+id+"/system_prompt"); ok {
			issues = append(issues, iss)
		}
	}
	for i, s := range wf.Steps {
		if iss, ok := scanUltra(s.Prompt, "", fmt.Sprintf("/steps/%d/prompt", i)); ok {
			issues = append(issues, iss)
		}
	}
	return issues
}

func scanUltra(s, nodeID, pointer string) (SchemaIssue, bool) {
	if s == "" {
		return SchemaIssue{}, false
	}
	matches := ultraKeywordRe.FindAllString(s, -1)
	if len(matches) == 0 {
		return SchemaIssue{}, false
	}
	seen := make(map[string]bool)
	var unique []string
	for _, m := range matches {
		lo := strings.ToLower(m)
		if !seen[lo] {
			seen[lo] = true
			unique = append(unique, lo)
		}
	}
	var kw string
	switch len(unique) {
	case 1:
		kw = fmt.Sprintf("'%s'", unique[0])
	default:
		kw = fmt.Sprintf("'%s' and '%s'", unique[0], unique[1])
	}
	return SchemaIssue{
		NodeID:  nodeID,
		Pointer: pointer,
		Message: fmt.Sprintf("bare keyword %s has no effect under 'sky run' (non-interactive 'claude -p'); replace with explicit instructions", kw),
		Code:    skyerr.ErrPromptUltraKeywordNoOp,
	}, true
}
