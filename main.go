// sky-workflow-lint lints .sky workflow files and reports diagnostics as text,
// JSON, or SARIF. It is the standalone public analogue of the sky lint command.
//
// Usage:
//
//	sky-workflow-lint [--format text|json|sarif] [--repo-root <path>] <file|glob>...
//
// Exit codes:
//
//	0  no diagnostics found
//	1  one or more diagnostics found
//	2  tool error (unreadable file, bad flag, etc.)
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/skyway-harness-builder/workflow-lint/lintfmt"
	"github.com/skyway-harness-builder/workflow-lint/workflow"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("sky-workflow-lint", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	format := fs.String("format", "text", "output format: text, json, or sarif")
	repoRoot := fs.String("repo-root", "", "repo root for SARIF URI normalisation (optional)")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	switch *format {
	case "text", "json", "sarif":
	default:
		fmt.Fprintf(os.Stderr, "sky-workflow-lint: unknown format %q (want text, json, or sarif)\n", *format)
		return 2
	}

	patterns := fs.Args()
	if len(patterns) == 0 {
		fmt.Fprintln(os.Stderr, "sky-workflow-lint: no files specified")
		fs.Usage()
		return 2
	}

	// Expand globs and collect unique paths.
	files, toolErr := expandPatterns(patterns)
	if toolErr != nil {
		fmt.Fprintf(os.Stderr, "sky-workflow-lint: %v\n", toolErr)
		return 2
	}
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "sky-workflow-lint: no files matched the given patterns")
		return 2
	}

	// Lint all files. Collect all diagnostics before emitting anything so that
	// --format json emits a single flat array (not one array per file).
	var diags []lintfmt.LintDiagnostic
	hasToolErr := false

	for _, f := range files {
		raw, err := workflow.Lint(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "sky-workflow-lint: %s: %v\n", f, err)
			hasToolErr = true
			continue
		}
		for _, d := range raw {
			diags = append(diags, lintfmt.LintDiagnostic{
				File:     d.File,
				Line:     d.Line,
				Message:  d.Message,
				Rule:     d.Code,
				Severity: d.Severity,
			})
		}
	}

	switch *format {
	case "text":
		for _, d := range diags {
			if d.Line > 0 {
				fmt.Printf("%s:%d: [%s] %s\n", d.File, d.Line, d.Rule, d.Message)
			} else {
				fmt.Printf("%s: [%s] %s\n", d.File, d.Rule, d.Message)
			}
		}
	case "json":
		// Always emit a single flat array. This is the primary reason this tool exists
		// as a standalone binary (binary issue #280).
		out := diags
		if out == nil {
			out = []lintfmt.LintDiagnostic{}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			fmt.Fprintf(os.Stderr, "sky-workflow-lint: encode json: %v\n", err)
			return 2
		}
	case "sarif":
		log := lintfmt.LintSARIF(diags, *repoRoot)
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(log); err != nil {
			fmt.Fprintf(os.Stderr, "sky-workflow-lint: encode sarif: %v\n", err)
			return 2
		}
	}

	if hasToolErr {
		return 2
	}
	// Only blocking diagnostics flip the exit code; "warning" severity findings
	// (e.g. SKY-WF-101 bash syntax/shellcheck) are advisory and exit 0.
	for _, d := range diags {
		if d.Severity != "warning" {
			return 1
		}
	}
	return 0
}

// expandPatterns expands each pattern. Patterns containing "**" are handled
// via walkDoublestar; all others use filepath.Glob. Patterns without glob
// metacharacters that don't match an existing file are returned as a tool error.
func expandPatterns(patterns []string) ([]string, error) {
	seen := map[string]bool{}
	var files []string

	for _, p := range patterns {
		var (
			matches []string
			err     error
		)
		if strings.Contains(p, "**") {
			matches, err = walkDoublestar(p)
		} else {
			matches, err = filepath.Glob(p)
		}
		if err != nil {
			return nil, fmt.Errorf("invalid glob %q: %w", p, err)
		}
		if len(matches) == 0 {
			// If the pattern contains no glob metacharacters it was a literal
			// filename; treat as a tool error.
			if !containsGlob(p) {
				return nil, fmt.Errorf("%s: no such file", p)
			}
			// A glob that matches nothing is silently skipped.
			continue
		}
		for _, m := range matches {
			if !seen[m] {
				seen[m] = true
				files = append(files, m)
			}
		}
	}
	return files, nil
}

// walkDoublestar handles glob patterns containing "**" by walking the
// directory tree. It resolves the root as everything before the first "**"
// and matches each regular file's base name against the tail pattern
// (everything after the last "**" separator). This covers the common CI
// patterns: "**/*.sky", "meta/**/*.sky", ".sky/**/*.sky".
func walkDoublestar(pattern string) ([]string, error) {
	// Root: clean path before the first **
	idx := strings.Index(pattern, "**")
	rootRaw := pattern[:idx]
	root := filepath.Clean(rootRaw)
	if root == "" || root == "." {
		root = "."
	}

	// Tail: the file-level pattern after the last **
	lastIdx := strings.LastIndex(pattern, "**")
	tail := pattern[lastIdx+2:]
	tail = strings.TrimPrefix(tail, "/")
	tail = strings.TrimPrefix(tail, string(filepath.Separator))

	var matches []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return nil
		}
		if tail == "" {
			matches = append(matches, path)
			return nil
		}
		ok, merr := filepath.Match(tail, filepath.Base(path))
		if merr != nil {
			return merr
		}
		if ok {
			matches = append(matches, path)
		}
		return nil
	})
	return matches, err
}

func containsGlob(s string) bool {
	for _, c := range s {
		if c == '*' || c == '?' || c == '[' {
			return true
		}
	}
	return false
}
