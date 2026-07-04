package workflow

import (
	"fmt"
	"path/filepath"

	"github.com/skyway-harness-builder/skyerr"
)

// LoadAllFromRoots loads all workflows across all enabled tiers.
// Precedence: Repo > Workspace > User > Builtin — when names collide, the
// higher-tier version shadows lower-tier entries. Non-existent tier
// directories are silently skipped.
func LoadAllFromRoots(roots Roots) ([]*Workflow, error) {
	seen := make(map[string]bool)
	var out []*Workflow
	for _, t := range roots.ordered() {
		if t.path == "" {
			continue
		}
		wfs, err := loadFromDir(filepath.Join(t.path, "workflows"), t.source)
		if err != nil {
			return nil, err
		}
		for _, wf := range wfs {
			if seen[wf.Name] {
				continue
			}
			seen[wf.Name] = true
			out = append(out, wf)
		}
	}

	builtins, err := LoadBuiltinWorkflows()
	if err != nil {
		return nil, err
	}
	for _, wf := range builtins {
		if seen[wf.Name] {
			continue
		}
		seen[wf.Name] = true
		out = append(out, wf)
	}
	return out, nil
}

// Resolve finds the first workflow matching name across tiers (Repo → Workspace → User → Builtin).
// Returns an error if the workflow is not found in any tier.
func Resolve(name string, roots Roots) (*Workflow, error) {
	for _, t := range roots.ordered() {
		if t.path == "" {
			continue
		}
		wfs, err := loadFromDir(filepath.Join(t.path, "workflows"), t.source)
		if err != nil {
			return nil, err
		}
		for _, wf := range wfs {
			if wf.Name == name {
				return wf, nil
			}
		}
	}

	builtins, err := LoadBuiltinWorkflows()
	if err != nil {
		return nil, err
	}
	for _, wf := range builtins {
		if wf.Name == name {
			return wf, nil
		}
	}

	return nil, skyerr.New(skyerr.ErrWorkflowNotFound, fmt.Sprintf("workflow %q not found", name))
}
