package workflow

import (
	"regexp"
	"strings"
)

// trustSinkRe matches shell "sink" tokens that execute a variable's contents as
// code rather than treating it as a string. Positioned at a statement boundary
// (start of body, after \n ; && || |, or leading whitespace) to avoid matching
// substrings inside other words or inside single-quoted literals that happen to
// contain these tokens.
//
// Narrow set (advisor-vetted):
//   - `eval`         — evaluates arguments as shell code
//   - `source`       — reads and runs a script file/stream
//   - `.`            — POSIX sourcing (same as `source`)
//   - `bash -c`      — runs argument string as bash
//   - `sh -c`        — runs argument string as sh
//
// Deliberately NOT matched: `$(...)`, backticks — too noisy and not specific to
// the "prompt output becomes shell" threat we're narrowing on.
var trustSinkRe = regexp.MustCompile(`(?m)(?:^|[\s;&|])(?:eval|source|\.|bash\s+-c|sh\s+-c)(?:\s|$)`)

// commentRe strips `# …` comments from a line (best-effort: tolerates `#` inside
// quoted strings only by stripping from the first unquoted `#`). We normalise
// to "strip everything after the first `#` on a line" — authors occasionally
// keep `#` in quoted strings in workflow bash, but for the trust scan the
// conservative choice (a token after `#` is ignored, even if quoted) matches
// how `sky lint` already behaves elsewhere and avoids heavy shell parsing.
var commentRe = regexp.MustCompile(`#[^\n]*`)

// ValidateBashTrust flags bash bodies that funnel prompt-node output into a
// shell sink (`eval`, `source`, `bash -c`, `sh -c`, `.`). Prompt-node output is
// authored by Claude at run time and must not be treated as trusted code — it
// must be treated as untrusted data. This lint rule catches the pattern
// statically before the workflow ever runs.
//
// The check is deliberately heuristic:
//   - iterate every node that carries shell text (Bash body or Loop.Until.Bash),
//   - scan for any sink token at a statement boundary,
//   - if a sink is present, scan for any SKY_OUTPUT_<PROMPT_NODE> env reference,
//   - flag the pair. One flag per offending node/body.
//
// Only prompt and command nodes are "untrusted sources" — bash/http/eval outputs
// are author-controlled and already trusted by the bash node that consumes them.
func ValidateBashTrust(wf *Workflow) []SchemaIssue {
	if wf == nil || len(wf.Nodes) == 0 {
		return nil
	}

	promptEnvKeys := collectPromptEnvKeys(wf)
	if len(promptEnvKeys) == 0 {
		return nil
	}

	var issues []SchemaIssue
	for _, n := range wf.Nodes {
		if body := n.Bash; body != "" {
			if hit := scanBashTrust(body, promptEnvKeys); hit != "" {
				issues = append(issues, SchemaIssue{
					NodeID:  n.ID,
					Pointer: "/nodes/" + n.ID + "/bash",
					Message: "bash sink (" + hit + ") receives $" + firstMatchedEnvKey(body, promptEnvKeys) + " from a prompt/command node — treat prompt output as data, not code",
				})
				continue
			}
		}
		if n.Loop != nil && n.Loop.Until.Bash != "" {
			body := n.Loop.Until.Bash
			if hit := scanBashTrust(body, promptEnvKeys); hit != "" {
				issues = append(issues, SchemaIssue{
					NodeID:  n.ID,
					Pointer: "/nodes/" + n.ID + "/loop/until/bash",
					Message: "loop.until.bash sink (" + hit + ") receives $" + firstMatchedEnvKey(body, promptEnvKeys) + " from a prompt/command node — treat prompt output as data, not code",
				})
			}
		}
	}
	return issues
}

// collectPromptEnvKeys builds the set of SKY_OUTPUT_<ID> env-var names that
// correspond to prompt or command nodes in the workflow. These are the
// "untrusted" outputs — authored by Claude at run time.
//
// Mapping mirrors runner/dag.go outputsAsEnv: upper-case, hyphens→underscores.
// This is a forward (id → env-key) mapping so we never have to reason about the
// many-to-one ambiguity of reverse parsing (hyphens and underscores collapse).
func collectPromptEnvKeys(wf *Workflow) map[string]string {
	out := make(map[string]string)
	for i := range wf.Nodes {
		n := &wf.Nodes[i]
		// Loop nodes wrap an inner body — the loop body itself may be
		// prompt/command. We check Prompt and Command directly rather than
		// NodeType(), because NodeType() priorities Loop over Prompt/Command.
		if n.Prompt != "" || n.Command != "" {
			key := "SKY_OUTPUT_" + strings.ToUpper(strings.ReplaceAll(n.ID, "-", "_"))
			out[key] = n.ID
		}
	}
	return out
}

// scanBashTrust returns the matched sink token (without surrounding whitespace)
// if body contains both a sink-at-statement-boundary AND any prompt env key.
// Empty string means no trust violation.
//
// Comments are stripped before scanning so that a `# eval $SKY_OUTPUT_x` line
// does not trigger a flag.
func scanBashTrust(body string, promptEnvKeys map[string]string) string {
	scan := commentRe.ReplaceAllString(body, "")

	sinkMatch := trustSinkRe.FindString(scan)
	if sinkMatch == "" {
		return ""
	}

	for key := range promptEnvKeys {
		if strings.Contains(scan, key) {
			return strings.TrimSpace(sinkMatch)
		}
	}
	return ""
}

// firstMatchedEnvKey returns the first prompt env key that appears in body,
// for use in the diagnostic message. body is assumed pre-scanned (scanBashTrust
// already confirmed at least one match exists).
func firstMatchedEnvKey(body string, promptEnvKeys map[string]string) string {
	scan := commentRe.ReplaceAllString(body, "")
	// Deterministic order: iterate node IDs via the map's values is
	// non-deterministic, so collect-then-sort would add dependencies. For a
	// diagnostic message, stable-enough is: pick the first occurrence in the
	// body text by scanning key-by-key. Map iteration order is fine for the
	// "which key" choice because we pick by earliest position in body.
	earliestIdx := -1
	var earliestKey string
	for key := range promptEnvKeys {
		idx := strings.Index(scan, key)
		if idx >= 0 && (earliestIdx == -1 || idx < earliestIdx) {
			earliestIdx = idx
			earliestKey = key
		}
	}
	return earliestKey
}
