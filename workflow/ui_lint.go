package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveUILocales returns a map of locale → absolute file path for the ui field
// of a node. wfDir is the directory containing the .sky file; ui is the raw
// Node.UI value (relative, no leading slash, no ".." — validated by the parser).
//
// Two forms are supported:
//
//   - Single-file form: ui ends with ".md".
//     Returns {"default": <abs path>}.
//
//   - Locale-set form: ui has no ".md" suffix.
//     Globs for <ui>.<lang>.md files and, if <ui>.md also exists, adds a
//     "default" entry. Note: filepath.Glob with pattern "review.*.md" does not
//     match "review.md" (requires two dots), so the bare .md must be stat-checked
//     separately.
//
// Returns (nil, nil) when no files are found; (nil, err) only on Glob error.
func ResolveUILocales(wfDir, ui string) (map[string]string, error) {
	result := make(map[string]string)

	if strings.HasSuffix(ui, ".md") {
		abs := ui
		if !filepath.IsAbs(ui) {
			abs = filepath.Join(wfDir, ui)
		}
		result["default"] = abs
		return result, nil
	}

	// Locale-set form: glob for <ui>.<lang>.md
	pattern := filepath.Join(wfDir, ui+".*.md")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	base := filepath.Base(ui)
	for _, match := range matches {
		matchBase := filepath.Base(match)
		// strip "<base>." prefix and ".md" suffix to extract locale
		prefix := base + "."
		if !strings.HasPrefix(matchBase, prefix) || !strings.HasSuffix(matchBase, ".md") {
			continue
		}
		locale := matchBase[len(prefix) : len(matchBase)-len(".md")]
		if locale == "" {
			continue
		}
		result[locale] = match
	}

	// Also check for a bare <ui>.md ("default" locale).
	barePath := filepath.Join(wfDir, ui+".md")
	if _, err := os.Stat(barePath); err == nil {
		result["default"] = barePath
	}

	return result, nil
}

// ValidateUI runs ui-specific lint checks for a parsed workflow.
//
//   - SKY-WF-103 (warning, non-blocking): ui references a path that resolves to
//     no markdown files on disk. The warning does not prevent the workflow from
//     running; the dashboard simply renders no node card or workflow card.
//
// wfPath is the path to the .sky file; its directory resolves relative ui paths.
// Pass "" when the file location is unknown (relative paths then resolve against
// the current working directory).
func ValidateUI(wf *Workflow, wfPath string) []Diagnostic {
	if wf == nil {
		return nil
	}
	wfDir := filepath.Dir(wfPath)
	var diags []Diagnostic

	// Workflow-level ui card.
	if wf.UI != "" {
		locales, err := ResolveUILocales(wfDir, wf.UI)
		if err != nil {
			locales = nil
		}
		if len(locales) == 0 {
			diags = append(diags, Diagnostic{
				Code:     "SKY-WF-103",
				Severity: "warning",
				Message:  fmt.Sprintf("workflow ui %q resolves to no markdown files", wf.UI),
			})
		}
	}

	// Per-node ui cards.
	for i := range wf.Nodes {
		n := &wf.Nodes[i]
		if n.UI == "" {
			continue
		}

		locales, err := ResolveUILocales(wfDir, n.UI)
		if err != nil {
			// Glob error — treat as "no files" so the warning still fires.
			locales = nil
		}
		if len(locales) == 0 {
			diags = append(diags, Diagnostic{
				Code:     "SKY-WF-103",
				Severity: "warning",
				Message:  fmt.Sprintf("node %q: ui %q resolves to no markdown files", n.ID, n.UI),
			})
		}
	}

	return diags
}
