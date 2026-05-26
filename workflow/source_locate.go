package workflow

import (
	"bytes"
	"strings"
)

// locateSection finds the line index (0-based) of the opener for a section with the
// given symbol and name (e.g. symMeta/"meta" or symStep/"fetch"). Returns -1 if not found.
// It does a simple line scan (no parser invocation) so it works on raw bytes.
func locateSection(raw []byte, symbol, name string) (startLine int, ok bool) {
	target := symbol + name + symbol
	lines := bytes.Split(raw, []byte("\n"))
	for i, line := range lines {
		if strings.TrimSpace(string(line)) == target {
			return i, true
		}
	}
	return -1, false
}

// locateAssign finds the first occurrence of `key = ...` (or `key=...`) within the
// section that starts at sectionOpenerLine and ends at its paired close marker
// (the first line consisting solely of symbol+symbol after the opener).
//
// Returns the 0-based line and the character offsets of the full value token
// (from the character after '=' and any whitespace, to end of line). If the key
// is not found, ok is false.
func locateAssign(raw []byte, sectionOpenerLine int, symbol, key string) (line, startChar, endChar int, ok bool) {
	closer := symbol + symbol
	lines := bytes.Split(raw, []byte("\n"))
	for i := sectionOpenerLine + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(string(lines[i]))
		if trimmed == closer {
			break
		}
		eqIdx := strings.Index(trimmed, "=")
		if eqIdx < 0 {
			continue
		}
		k := strings.TrimSpace(trimmed[:eqIdx])
		if k != key {
			continue
		}
		// Find the position of '=' in the original (untrimmed) line.
		rawLine := string(lines[i])
		rawEq := strings.Index(rawLine, "=")
		if rawEq < 0 {
			continue
		}
		// Value starts after '=' and any whitespace.
		valStart := rawEq + 1
		for valStart < len(rawLine) && (rawLine[valStart] == ' ' || rawLine[valStart] == '\t') {
			valStart++
		}
		return i, valStart, len(rawLine), true
	}
	return -1, -1, -1, false
}

// locateSectionClose finds the line index of the close marker for the section
// opened at sectionOpenerLine (e.g. "⊕⊕" or "§§" depending on symbol).
func locateSectionClose(raw []byte, sectionOpenerLine int, symbol string) (closeLine int, ok bool) {
	closer := symbol + symbol
	lines := bytes.Split(raw, []byte("\n"))
	for i := sectionOpenerLine + 1; i < len(lines); i++ {
		if strings.TrimSpace(string(lines[i])) == closer {
			return i, true
		}
	}
	return -1, false
}

// lineIndent returns the leading whitespace of the line at the given 0-based index.
func lineIndent(raw []byte, lineIdx int) string {
	lines := bytes.Split(raw, []byte("\n"))
	if lineIdx < 0 || lineIdx >= len(lines) {
		return ""
	}
	line := string(lines[lineIdx])
	return line[:len(line)-len(strings.TrimLeft(line, " \t"))]
}
