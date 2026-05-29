package workflow

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/skylence-be/skyerr"
)

// The .sky format uses three Unicode delimiters, each with a distinct role:
//
//	⊕meta⊕  ... ⊕⊕       workflow-level JSON (name, description, trigger). Required, exactly one.
//	§<id>§  ... §§       per-node JSON config (command, bash, depends_on, model, ...). Becomes a DAG node.
//	∆<id>∆  ... ∆∆       per-node prompt prose. Pairs with a §<id>§ section by matching id.
//	※※      ... ※※       doc/comment block between sections. Content is discarded by the parser.
//
// Every opener and closer must be alone on its line (after trimming whitespace).
const (
	symMeta   = "⊕"
	symStep   = "§"
	symPrompt = "∆"
	symDoc    = "※"
	closeDoc  = symDoc + symDoc
	metaName  = "meta"
)

type skySection struct {
	symbol string
	name   string
	body   string
	line   int // line number where the opener appeared
}

// parseSky reads a .sky file and returns a parsed Workflow (pre-validation).
func parseSky(r io.Reader) (*Workflow, error) {
	sections, err := scanSkySections(r)
	if err != nil {
		return nil, err
	}

	// Require exactly one ⊕meta⊕ section.
	var metaBody string
	metaCount := 0
	for _, s := range sections {
		if s.symbol != symMeta {
			continue
		}
		if s.name != metaName {
			return nil, skyerr.New(skyerr.ErrSkyFormat,
				fmt.Sprintf("line %d: ⊕%s⊕ is not a valid meta section; only ⊕meta⊕ is supported", s.line, s.name))
		}
		metaCount++
		metaBody = s.body
	}
	if metaCount == 0 {
		return nil, skyerr.New(skyerr.ErrSkyFormat, "missing ⊕meta⊕ section")
	}
	if metaCount > 1 {
		return nil, skyerr.New(skyerr.ErrSkyFormat, "multiple ⊕meta⊕ sections; only one allowed")
	}

	metaMap, err := parseAssignBody(metaBody, "⊕meta⊕")
	if err != nil {
		return nil, skyerr.Wrap(skyerr.ErrWorkflowParse, "failed to parse ⊕meta⊕", err)
	}
	metaJSON, err := json.Marshal(metaMap)
	if err != nil {
		return nil, skyerr.Wrap(skyerr.ErrWorkflowParse, "⊕meta⊕ internal marshal", err)
	}
	var wf Workflow
	if err := json.Unmarshal(metaJSON, &wf); err != nil {
		return nil, skyerr.Wrap(skyerr.ErrWorkflowParse, "failed to parse ⊕meta⊕", err)
	}

	// Collect ∆ sections keyed by name.
	prompts := make(map[string]string)
	promptLine := make(map[string]int)
	for _, s := range sections {
		if s.symbol != symPrompt {
			continue
		}
		if _, dup := prompts[s.name]; dup {
			return nil, skyerr.New(skyerr.ErrSkyFormat,
				fmt.Sprintf("line %d: duplicate ∆%s∆ section", s.line, s.name))
		}
		prompts[s.name] = s.body
		promptLine[s.name] = s.line
	}

	// Collect § sections in appearance order; unmarshal each into a Node.
	var nodes []Node
	stepNames := make(map[string]bool)
	for _, s := range sections {
		if s.symbol != symStep {
			continue
		}
		if stepNames[s.name] {
			return nil, skyerr.New(skyerr.ErrSkyFormat,
				fmt.Sprintf("line %d: duplicate §%s§ section", s.line, s.name))
		}
		stepNames[s.name] = true

		var n Node
		if strings.TrimSpace(s.body) != "" {
			nodeMap, err := parseAssignBody(s.body, fmt.Sprintf("§%s§", s.name))
			if err != nil {
				return nil, skyerr.Wrap(skyerr.ErrWorkflowParse,
					fmt.Sprintf("line %d: failed to parse §%s§", s.line, s.name), err)
			}
			nodeJSON, err := json.Marshal(nodeMap)
			if err != nil {
				return nil, skyerr.Wrap(skyerr.ErrWorkflowParse,
					fmt.Sprintf("line %d: §%s§ internal marshal", s.line, s.name), err)
			}
			if err := json.Unmarshal(nodeJSON, &n); err != nil {
				return nil, skyerr.Wrap(skyerr.ErrWorkflowParse,
					fmt.Sprintf("line %d: failed to parse §%s§", s.line, s.name), err)
			}
		}
		n.ID = s.name
		if prompt, ok := prompts[s.name]; ok {
			n.Prompt = strings.TrimSpace(prompt)
		}
		nodes = append(nodes, n)
	}

	// Any ∆ without a matching § is an orphan.
	for name, line := range promptLine {
		if !stepNames[name] {
			return nil, skyerr.New(skyerr.ErrSkyFormat,
				fmt.Sprintf("line %d: ∆%s∆ has no matching §%s§ section", line, name, name))
		}
	}

	if len(nodes) > 0 {
		wf.Nodes = nodes
	}
	return &wf, nil
}

// scanSkySections walks the input line by line and returns all sections in file order.
func scanSkySections(r io.Reader) ([]skySection, error) {
	scanner := bufio.NewScanner(r)
	// Allow long prompt bodies; the default 64KB line limit is too tight for some prompts.
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	var (
		sections  []skySection
		curSymbol string
		curName   string
		curLine   int
		curBody   strings.Builder
		lineNum   int
		inDoc     bool
		docLine   int
	)

	const (
		closeMeta   = symMeta + symMeta
		closeStep   = symStep + symStep
		closePrompt = symPrompt + symPrompt
	)

	flush := func() {
		sections = append(sections, skySection{
			symbol: curSymbol, name: curName, body: curBody.String(), line: curLine,
		})
		curSymbol, curName, curLine = "", "", 0
		curBody.Reset()
	}

	for scanner.Scan() {
		lineNum++
		raw := scanner.Text()
		trimmed := strings.TrimSpace(raw)

		// Doc block: discard all content until the closing ※※.
		if inDoc {
			if trimmed == closeDoc {
				inDoc = false
			}
			continue
		}

		// Close markers take priority — they must match the currently open symbol.
		switch trimmed {
		case closeMeta:
			if curSymbol != symMeta {
				return nil, skyerr.New(skyerr.ErrSkyFormat,
					fmt.Sprintf("line %d: unexpected ⊕⊕ close", lineNum))
			}
			flush()
			continue
		case closeStep:
			if curSymbol != symStep {
				return nil, skyerr.New(skyerr.ErrSkyFormat,
					fmt.Sprintf("line %d: unexpected §§ close", lineNum))
			}
			flush()
			continue
		case closePrompt:
			if curSymbol != symPrompt {
				return nil, skyerr.New(skyerr.ErrSkyFormat,
					fmt.Sprintf("line %d: unexpected ∆∆ close", lineNum))
			}
			flush()
			continue
		}

		if curSymbol == "" {
			// Between sections: expect an opener, a doc block, or a blank line.
			if trimmed == closeDoc {
				inDoc = true
				docLine = lineNum
				continue
			}
			if sym, name, ok := tryOpener(trimmed); ok {
				if name == "" {
					return nil, skyerr.New(skyerr.ErrSkyFormat,
						fmt.Sprintf("line %d: empty section name", lineNum))
				}
				if containsForbidden(name) {
					return nil, skyerr.New(skyerr.ErrSkyFormat,
						fmt.Sprintf("line %d: section name %q contains forbidden character", lineNum, name))
				}
				curSymbol = sym
				curName = name
				curLine = lineNum
				continue
			}
			if trimmed == "" {
				continue
			}
			if startsWithDelimiter(trimmed) {
				return nil, skyerr.New(skyerr.ErrSkyFormat,
					fmt.Sprintf("line %d: malformed marker %q", lineNum, trimmed))
			}
			return nil, skyerr.New(skyerr.ErrSkyFormat,
				fmt.Sprintf("line %d: unexpected content outside section: %q", lineNum, trimmed))
		}

		// Inside a section — a line that looks like a full opener is a nesting error.
		if _, _, ok := tryOpener(trimmed); ok {
			return nil, skyerr.New(skyerr.ErrSkyFormat,
				fmt.Sprintf("line %d: nested opener inside %s%s%s (opened at line %d)",
					lineNum, curSymbol, curName, curSymbol, curLine))
		}

		// Body line — preserve original (untrimmed) content and newlines.
		if curBody.Len() > 0 {
			curBody.WriteByte('\n')
		}
		curBody.WriteString(raw)
	}
	if err := scanner.Err(); err != nil {
		return nil, skyerr.Wrap(skyerr.ErrSkyFormat, "scan error", err)
	}
	if inDoc {
		return nil, skyerr.New(skyerr.ErrSkyFormat,
			fmt.Sprintf("unclosed ※※ doc block opened at line %d", docLine))
	}
	if curSymbol != "" {
		return nil, skyerr.New(skyerr.ErrSkyFormat,
			fmt.Sprintf("unclosed section %s%s%s opened at line %d", curSymbol, curName, curSymbol, curLine))
	}

	return sections, nil
}

// tryOpener returns (symbol, name, true) if line is `<sym><name><sym>` for any of the three symbols.
// Returns (_, _, false) if line is blank, a bare close marker, or not an opener.
func tryOpener(line string) (symbol, name string, ok bool) {
	for _, sym := range []string{symMeta, symStep, symPrompt} {
		if line == sym+sym {
			// Bare close marker, not an opener.
			continue
		}
		if !strings.HasPrefix(line, sym) || !strings.HasSuffix(line, sym) {
			continue
		}
		if len(line) < 2*len(sym) {
			continue
		}
		inner := line[len(sym) : len(line)-len(sym)]
		return sym, strings.TrimSpace(inner), true
	}
	return "", "", false
}

// startsWithDelimiter reports whether the line begins with any of the four delimiter symbols.
func startsWithDelimiter(line string) bool {
	return strings.HasPrefix(line, symMeta) ||
		strings.HasPrefix(line, symStep) ||
		strings.HasPrefix(line, symPrompt) ||
		strings.HasPrefix(line, symDoc)
}

// containsForbidden reports whether the name contains any of the four delimiter symbols.
func containsForbidden(name string) bool {
	return strings.ContainsAny(name, symMeta+symStep+symPrompt+symDoc)
}
