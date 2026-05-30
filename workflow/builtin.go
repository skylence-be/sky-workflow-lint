package workflow

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed builtin/workflows/*.sky
var builtinWorkflowsFS embed.FS

// LoadBuiltinWorkflows returns the workflows embedded in the binary.
// These are always available as the lowest-priority tier.
func LoadBuiltinWorkflows() ([]*Workflow, error) {
	entries, err := fs.ReadDir(builtinWorkflowsFS, "builtin/workflows")
	if err != nil {
		return nil, fmt.Errorf("read builtin workflows: %w", err)
	}

	var workflows []*Workflow
	for _, e := range entries {
		if e.IsDir() || !isSky(e.Name()) {
			continue
		}
		f, err := builtinWorkflowsFS.Open("builtin/workflows/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("open builtin %s: %w", e.Name(), err)
		}
		wf, parseErr := Parse(f)
		_ = f.Close()
		if parseErr != nil {
			return nil, fmt.Errorf("parse builtin %s: %w", e.Name(), parseErr)
		}
		wf.Source = SourceBuiltin
		workflows = append(workflows, wf)
	}
	return workflows, nil
}
