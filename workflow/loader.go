package workflow

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// LoadAll loads all workflows from the repo's .sky/workflows directory.
func LoadAll(repoDir string) ([]*Workflow, error) {
	return loadFromDir(filepath.Join(repoDir, ".sky", "workflows"), SourceRepo)
}

// loadFromDir loads all .sky workflow files from dir, tagging each with source.
// Supports both the directory form (<name>/workflow.sky) and the legacy flat
// form (<name>.sky). Returns (nil, nil) when dir does not exist.
func loadFromDir(dir string, source SourceTier) ([]*Workflow, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read workflows dir %s: %w", dir, err)
	}

	var workflows []*Workflow
	for _, e := range entries {
		var skyPath string
		if e.IsDir() {
			candidate := filepath.Join(dir, e.Name(), "workflow.sky")
			if _, statErr := os.Stat(candidate); statErr == nil {
				skyPath = candidate
			} else {
				continue
			}
		} else if isSky(e.Name()) {
			skyPath = filepath.Join(dir, e.Name())
		} else {
			continue
		}

		f, err := os.Open(skyPath)
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", skyPath, err)
		}
		wf, err := Parse(f)
		if closeErr := f.Close(); closeErr != nil {
			return nil, fmt.Errorf("close %s: %w", skyPath, closeErr)
		}
		if err != nil {
			slog.Warn("loadFromDir: skipping unparseable workflow", "file", skyPath, "err", err)
			continue
		}
		wf.Source = source
		workflows = append(workflows, wf)
	}
	return workflows, nil
}

func isSky(name string) bool {
	return strings.ToLower(filepath.Ext(name)) == ".sky"
}
