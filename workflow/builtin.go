package workflow

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"
)

//go:embed builtin/workflows/*.sky builtin/workflows/ui/*.md
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
		resolveBuiltinUIDocs(wf)
		workflows = append(workflows, wf)
	}
	return workflows, nil
}

// resolveBuiltinUIDocs populates wf.UIDoc and each node's UIDocs from the
// embedded builtin/workflows/ui directory. Builtin workflows have no on-disk
// directory at runtime, so they can't use ResolveUILocales (which globs a
// real filesystem); file names are matched directly instead:
//
//	<workflow-name>.<locale>.md               -> workflow-level card
//	<workflow-name>--<node-id>.<locale>.md    -> node-level card
//
// A missing file is silently skipped: ui docs are non-critical and a gap
// here never breaks workflow loading.
func resolveBuiltinUIDocs(wf *Workflow) {
	const uiDir = "builtin/workflows/ui"
	entries, err := fs.ReadDir(builtinWorkflowsFS, uiDir)
	if err != nil {
		return
	}

	wfPrefix := wf.Name + "."
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, wfPrefix) || !strings.HasSuffix(name, ".md") {
			continue
		}
		locale := strings.TrimSuffix(strings.TrimPrefix(name, wfPrefix), ".md")
		if locale == "" {
			continue
		}
		raw, err := builtinWorkflowsFS.ReadFile(uiDir + "/" + name)
		if err != nil {
			continue
		}
		if wf.UIDoc == nil {
			wf.UIDoc = make(map[string]UIDoc)
		}
		wf.UIDoc[locale] = parseBuiltinUIFile(raw)
	}

	for i := range wf.Nodes {
		nodePrefix := wf.Name + "--" + wf.Nodes[i].ID + "."
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.HasPrefix(name, nodePrefix) || !strings.HasSuffix(name, ".md") {
				continue
			}
			locale := strings.TrimSuffix(strings.TrimPrefix(name, nodePrefix), ".md")
			if locale == "" {
				continue
			}
			raw, err := builtinWorkflowsFS.ReadFile(uiDir + "/" + name)
			if err != nil {
				continue
			}
			if wf.Nodes[i].UIDocs == nil {
				wf.Nodes[i].UIDocs = make(map[string]UIDoc)
			}
			wf.Nodes[i].UIDocs[locale] = parseBuiltinUIFile(raw)
		}
	}
}

// uiFrontmatter holds the short_description / long_description keys parsed
// from a builtin ui markdown file's YAML frontmatter.
type uiFrontmatter struct {
	Short string
	Long  string
}

// parseBuiltinUIFile splits optional frontmatter (--- ... ---) from a ui
// markdown file. This repo has no YAML dependency, so it implements exactly
// the subset used by builtin ui files: "---" open/close lines alone on their
// line, and "short_description:" / "long_description:" keys with a
// single-line value that may be double-quoted (with \" escapes unescaped).
// Any other content in the frontmatter block is ignored. If there is no
// well-formed frontmatter, Short/Long are empty and Body is the whole file.
func parseBuiltinUIFile(raw []byte) UIDoc {
	const delim = "---"
	text := string(raw)
	lines := strings.Split(text, "\n")

	if len(lines) == 0 || strings.TrimSpace(lines[0]) != delim {
		return UIDoc{Body: strings.TrimRight(text, "\n")}
	}

	closeIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == delim {
			closeIdx = i
			break
		}
	}
	if closeIdx < 0 {
		return UIDoc{Body: strings.TrimRight(text, "\n")}
	}

	fm := parseUIFrontmatterBlock(lines[1:closeIdx])
	body := strings.Join(lines[closeIdx+1:], "\n")
	body = strings.Trim(body, "\n")

	return UIDoc{Short: fm.Short, Long: fm.Long, Body: body}
}

// parseUIFrontmatterBlock parses "short_description:" / "long_description:"
// lines from a frontmatter body. A value may be wrapped in double quotes
// (required when it contains a colon, since unquoted "a: b: c" is not valid
// YAML); quoted values have \" unescaped. Unrecognized lines are ignored.
func parseUIFrontmatterBlock(lines []string) uiFrontmatter {
	var fm uiFrontmatter
	for _, line := range lines {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if len(value) >= 2 && strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
			value = strings.ReplaceAll(value[1:len(value)-1], `\"`, `"`)
		}
		switch key {
		case "short_description":
			fm.Short = value
		case "long_description":
			fm.Long = value
		}
	}
	return fm
}
