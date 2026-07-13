package workflow

import (
	"fmt"
	"path/filepath"

	"github.com/skyway-harness-builder/skyerr"
)

// LoadAllFromRoots loads all workflows across all enabled tiers.
// Precedence: Repo > Workspace > User — when names collide, the higher-tier
// version shadows lower-tier entries. Non-existent tier directories are
// silently skipped. Builtin workflows are the HOST's concern: skyway ships
// them in its embedded library and appends them as the lowest tier.
//
// Passing WithAllowedSources restricts the result to workflows whose library
// source is in the given set. With no options the behavior is unchanged.
func LoadAllFromRoots(roots Roots, opts ...LoadOption) ([]*Workflow, error) {
	o := buildLoadOptions(opts)
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
			if !o.allow(wf) {
				// Not a candidate under the active source filter. A lower
				// tier with the same name may still qualify, so it is not
				// marked seen here.
				continue
			}
			seen[wf.Name] = true
			out = append(out, wf)
		}
	}
	return out, nil
}

// Resolve finds the first workflow matching name across tiers (Repo, then
// Workspace, then User). Returns an error if the workflow is not found in any
// tier. Passing WithAllowedSources skips workflows whose library source is not
// in the given set, so the first in-set match by name is returned.
func Resolve(name string, roots Roots, opts ...LoadOption) (*Workflow, error) {
	o := buildLoadOptions(opts)
	for _, t := range roots.ordered() {
		if t.path == "" {
			continue
		}
		wfs, err := loadFromDir(filepath.Join(t.path, "workflows"), t.source)
		if err != nil {
			return nil, err
		}
		for _, wf := range wfs {
			if wf.Name == name && o.allow(wf) {
				return wf, nil
			}
		}
	}
	return nil, skyerr.New(skyerr.ErrWorkflowNotFound, fmt.Sprintf("workflow %q not found", name))
}
