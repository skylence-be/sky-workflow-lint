package workflow

import (
	"fmt"
	"strings"

	"github.com/skylence-be/sky-workflow-lint/skyerr"
)

// ValidLearningCategories is the authoritative list accepted by the loader, linter, and CLI.
var ValidLearningCategories = []string{"execution-rules", "patterns", "anti-patterns", "project-notes"}

// validLearningCategories is the unexported alias used within this package.
var validLearningCategories = ValidLearningCategories

// ValidateLearningsConfig returns SKY-WF-062 issues for any invalid learnings configuration
// on the workflow or its nodes:
//   - exclude and only set simultaneously on the same config block
//   - unknown category names in exclude or only
//   - max_bytes outside the allowed range (-1, 0, or 1..1048576)
func ValidateLearningsConfig(wf *Workflow) []SchemaIssue {
	if wf == nil {
		return nil
	}
	var issues []SchemaIssue

	if wf.Learnings != nil {
		issues = append(issues, checkLearningsConfig("", "/learnings", wf.Learnings)...)
	}

	for _, n := range wf.Nodes {
		if n.Learnings == nil {
			continue
		}
		issues = append(issues, checkLearningsConfig(n.ID, "/nodes/"+n.ID+"/learnings", n.Learnings)...)
	}

	return issues
}

func checkLearningsConfig(nodeID, pointer string, cfg *LearningsConfig) []SchemaIssue {
	var issues []SchemaIssue

	if len(cfg.Exclude) > 0 && len(cfg.Only) > 0 {
		issues = append(issues, SchemaIssue{
			NodeID:  nodeID,
			Pointer: pointer,
			Message: "learnings: exclude and only cannot both be set; use only one filter",
			Code:    skyerr.ErrLearningsConfig,
		})
	}

	for _, cat := range cfg.Exclude {
		if !isValidLearningCategory(cat) {
			issues = append(issues, SchemaIssue{
				NodeID:  nodeID,
				Pointer: pointer + "/exclude",
				Message: fmt.Sprintf("learnings: unknown category %q in exclude; valid: %s", cat, strings.Join(validLearningCategories, ", ")),
				Code:    skyerr.ErrLearningsConfig,
			})
		}
	}

	for _, cat := range cfg.Only {
		if !isValidLearningCategory(cat) {
			issues = append(issues, SchemaIssue{
				NodeID:  nodeID,
				Pointer: pointer + "/only",
				Message: fmt.Sprintf("learnings: unknown category %q in only; valid: %s", cat, strings.Join(validLearningCategories, ", ")),
				Code:    skyerr.ErrLearningsConfig,
			})
		}
	}

	if cfg.MaxBytes != 0 && cfg.MaxBytes != -1 && (cfg.MaxBytes < 1 || cfg.MaxBytes > 1048576) {
		issues = append(issues, SchemaIssue{
			NodeID:  nodeID,
			Pointer: pointer + "/max_bytes",
			Message: fmt.Sprintf("learnings: max_bytes %d is out of range; use -1 (no cap), 0 (default 32 KiB), or 1..1048576", cfg.MaxBytes),
			Code:    skyerr.ErrLearningsConfig,
		})
	}

	return issues
}

func isValidLearningCategory(cat string) bool {
	for _, v := range validLearningCategories {
		if v == cat {
			return true
		}
	}
	return false
}
