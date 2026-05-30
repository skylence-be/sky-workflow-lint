package workflow

import "fmt"

// lintDocBlocks returns SKY-WF-102 warnings for doc blocks whose StepID
// references a node or step that does not exist in the workflow.
// The warning is non-blocking (severity "warning") — a stale doc reference
// never prevents a workflow from running.
func lintDocBlocks(wf *Workflow) []Diagnostic {
	// build a set of known ids
	known := make(map[string]bool)
	for _, n := range wf.Nodes {
		known[n.ID] = true
	}
	for _, s := range wf.Steps {
		known[s.Name] = true
	}

	var diags []Diagnostic
	for _, d := range wf.Docs {
		if d.StepID == "" {
			continue // loose block, no id to validate
		}
		if !known[d.StepID] {
			diags = append(diags, Diagnostic{
				Code:     "SKY-WF-102",
				Severity: "warning",
				Message:  fmt.Sprintf("doc block references unknown step %q", d.StepID),
				Line:     d.Line,
			})
		}
	}
	return diags
}
