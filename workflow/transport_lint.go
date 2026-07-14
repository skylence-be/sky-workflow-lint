package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	codeCurlMissingFail    = "SKY-WF-111"
	codeCurlMissingMaxTime = "SKY-WF-112"
	codeRetryOnSink        = "SKY-WF-113"
	codePreferHTTPNode     = "SKY-WF-114"
)

var (
	sinkHeuristicPattern = regexp.MustCompile(`(?i)(send|post|create|notify|book|deliver|apply)`)
	curlWordPattern      = regexp.MustCompile(`\bcurl\b`)
	curlFailPattern      = regexp.MustCompile(`(?:^|[^-a-zA-Z])(--fail\b|-f\b)`)
	curlMaxTimePattern   = regexp.MustCompile(`--max-time\b`)
)

// ValidateTransportLint applies semantic bash-transport rules from issue #24:
//
//   - SKY-WF-111 (warning): curl without --fail or -f
//   - SKY-WF-112 (warning): curl without --max-time
//   - SKY-WF-113 (warning): retry on a sink-heuristic node
//   - SKY-WF-114 (info): bash body that is essentially a single curl → suggest http node
//
// wfPath resolves relative bash_file paths; pass "" to resolve against cwd.
func ValidateTransportLint(wf *Workflow, wfPath string) []Diagnostic {
	if wf == nil || len(wf.Nodes) == 0 {
		return nil
	}
	wfDir := filepath.Dir(wfPath)
	var diags []Diagnostic

	for i := range wf.Nodes {
		n := &wf.Nodes[i]
		diags = append(diags, lintRetryOnSink(n)...)
		if script, ok := resolveBashScript(n, wfDir); ok {
			diags = append(diags, lintCurlTransport(n.ID, script)...)
			diags = append(diags, lintPreferHTTPNode(n, script)...)
		}
	}
	return diags
}

func lintRetryOnSink(n *Node) []Diagnostic {
	if n.Retry == nil {
		return nil
	}
	if !looksLikeSinkNode(n) {
		return nil
	}
	return []Diagnostic{{
		Code:     codeRetryOnSink,
		Severity: "warning",
		Message: fmt.Sprintf(
			"node %q: retry is set on a sink-like node — retries belong on idempotent reads, not send/post/create/notify/book/deliver/apply operations",
			n.ID,
		),
	}}
}

func looksLikeSinkNode(n *Node) bool {
	candidates := []string{n.ID, n.Bash, n.BashFile, filepath.Base(n.BashFile)}
	for _, s := range candidates {
		if s != "" && sinkHeuristicPattern.MatchString(s) {
			return true
		}
	}
	return false
}

func lintCurlTransport(nodeID, script string) []Diagnostic {
	var diags []Diagnostic
	for _, line := range curlInvocationLines(script) {
		if !curlFailPattern.MatchString(line) {
			diags = append(diags, Diagnostic{
				Code:     codeCurlMissingFail,
				Severity: "warning",
				Message:  fmt.Sprintf("node %q: curl without --fail (or -f): %s", nodeID, truncateCurlLine(line)),
			})
		}
		if !curlMaxTimePattern.MatchString(line) {
			diags = append(diags, Diagnostic{
				Code:     codeCurlMissingMaxTime,
				Severity: "warning",
				Message:  fmt.Sprintf("node %q: curl without --max-time: %s", nodeID, truncateCurlLine(line)),
			})
		}
	}
	return diags
}

func lintPreferHTTPNode(n *Node, script string) []Diagnostic {
	if n.HTTP != nil {
		return nil
	}
	if !isEssentiallySingleCurl(script) {
		return nil
	}
	return []Diagnostic{{
		Code:     codePreferHTTPNode,
		Severity: "info",
		Message: fmt.Sprintf(
			"node %q: bash body is essentially a single curl call — consider an http node (url, method, headers, body) with secrets allowlist and ${env:NAME} refs to avoid shell JSON quoting",
			n.ID,
		),
	}}
}

func resolveBashScript(n *Node, wfDir string) (string, bool) {
	if n.Bash != "" {
		return n.Bash, true
	}
	if n.BashFile == "" {
		return "", false
	}
	scriptPath := n.BashFile
	if !filepath.IsAbs(scriptPath) {
		scriptPath = filepath.Join(wfDir, scriptPath)
	}
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		return "", false
	}
	return string(content), true
}

// curlInvocationLines returns logical lines that invoke curl. Comment-only and
// echo/heredoc lines are skipped; backslash continuations are joined first.
func curlInvocationLines(script string) []string {
	var hits []string
	for _, stmt := range bashStatements(script) {
		if curlWordPattern.MatchString(stmt) {
			hits = append(hits, stmt)
		}
	}
	return hits
}

func bashStatements(script string) []string {
	joined := joinBackslashContinuations(script)
	var stmts []string
	for _, raw := range strings.Split(joined, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Skip obvious non-invocation mentions (conservative).
		if strings.HasPrefix(line, "echo ") || strings.HasPrefix(line, "printf ") {
			continue
		}
		stmts = append(stmts, line)
	}
	return stmts
}

func joinBackslashContinuations(script string) string {
	lines := strings.Split(script, "\n")
	var out []string
	var buf strings.Builder
	for _, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			if buf.Len() > 0 {
				out = append(out, buf.String())
				buf.Reset()
			}
			out = append(out, raw)
			continue
		}
		// Strip inline comments for continuation decisions only.
		code := trimmed
		if idx := strings.Index(code, "#"); idx >= 0 {
			code = strings.TrimSpace(code[:idx])
		}
		if buf.Len() > 0 {
			buf.WriteByte(' ')
		}
		buf.WriteString(strings.TrimSpace(code))
		if strings.HasSuffix(code, `\`) {
			continue
		}
		out = append(out, buf.String())
		buf.Reset()
	}
	if buf.Len() > 0 {
		out = append(out, buf.String())
	}
	return strings.Join(out, "\n")
}

func isEssentiallySingleCurl(script string) bool {
	stmts := bashStatements(script)
	var curlCount int
	var otherExec int
	for _, stmt := range stmts {
		switch {
		case isBenignPreamble(stmt):
			continue
		case curlWordPattern.MatchString(stmt):
			curlCount++
		default:
			otherExec++
		}
	}
	return curlCount == 1 && otherExec == 0
}

func isBenignPreamble(stmt string) bool {
	switch {
	case strings.HasPrefix(stmt, "#!"):
		return true
	case strings.HasPrefix(stmt, "set "):
		return true
	case strings.HasPrefix(stmt, ":"):
		return true
	case strings.Contains(stmt, "=") && !curlWordPattern.MatchString(stmt):
		// Variable assignment without curl on the same line.
		return !strings.Contains(stmt, " $(curl") && !strings.Contains(stmt, "$(curl")
	default:
		return false
	}
}

func truncateCurlLine(line string) string {
	const max = 120
	line = strings.Join(strings.Fields(line), " ")
	if len(line) <= max {
		return line
	}
	return line[:max] + "..."
}

// isNonBlockingSeverity reports whether a diagnostic severity should not flip
// the lint exit code or block tier-dependent checks.
func isNonBlockingSeverity(severity string) bool {
	return severity == "warning" || severity == "info"
}