package workflow

import (
	"fmt"
	"regexp"

	"github.com/skyway-harness-builder/skyerr"
)

// destructivePatterns are the bash input patterns that trigger SKY-WF-063.
// Must stay in sync with the default rules in internal/runner/safety.go.
var destructivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`rm\s+-[rRf]`),
	regexp.MustCompile(`sudo\s`),
	regexp.MustCompile(`curl.*\|\s*(bash|sh)`),
	regexp.MustCompile(`>\s*/etc/`),
}

// ValidateSafetyClass flags any bash node whose command matches a destructive
// pattern but does not carry safety: requires_permission.
func ValidateSafetyClass(wf *Workflow) []SchemaIssue {
	if wf == nil || len(wf.Nodes) == 0 {
		return nil
	}
	var issues []SchemaIssue
	for _, n := range wf.Nodes {
		if n.Bash == "" {
			continue
		}
		if n.Safety == "requires_permission" {
			continue
		}
		for _, pat := range destructivePatterns {
			if pat.MatchString(n.Bash) {
				issues = append(issues, SchemaIssue{
					NodeID:  n.ID,
					Pointer: fmt.Sprintf("/nodes/%s/bash", n.ID),
					Message: fmt.Sprintf("node %q: bash command matches destructive pattern %q — add safety: requires_permission or rewrite the command", n.ID, pat.String()),
					Code:    skyerr.ErrSafetyClassMissing,
				})
				break
			}
		}
	}
	return issues
}
