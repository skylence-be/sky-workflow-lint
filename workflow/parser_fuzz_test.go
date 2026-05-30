package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FuzzParse ensures Parse never panics on arbitrary input. Any input — valid,
// malformed, or adversarial — must return (workflow, nil) or (nil, error), never crash.
func FuzzParse(f *testing.F) {
	// Seed from real .sky workflow files.
	for _, glob := range []string{
		"../../.sky/workflows/*.sky",
	} {
		matches, _ := filepath.Glob(glob)
		for _, path := range matches {
			data, err := os.ReadFile(path)
			if err == nil {
				f.Add(string(data))
			}
		}
	}

	// Inline seeds: empty, minimal valid, delimiter edge cases, oversized section.
	f.Add("")
	f.Add("⊕ name: hello\n∆\nhello world\n")
	f.Add("⊕ name: x\n§ node: n1\nkind: claude\nmodel: sonnet\n∆\ndo thing\n")
	f.Add("---\nname: yaml-trap\n---\nstep: foo\n")
	f.Add("⊕\n§\n∆\n")
	f.Add(strings.Repeat("a", 1<<20+1)) // exceeds 1 MB scanner cap

	f.Fuzz(func(t *testing.T, s string) {
		_, _ = Parse(strings.NewReader(s))
	})
}
