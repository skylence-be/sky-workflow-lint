package workflow

import "strings"

// MaxVarEnvLen caps the per-value length of env vars passed to bash nodes —
// both webhook-derived workflow vars (VarsAsEnv) and upstream node outputs
// (runner.outputsAsEnv). Webhook payloads can include attacker-controlled
// titles/bodies, and Claude node outputs can be arbitrarily large; either
// would push the process env over ARG_MAX (E2BIG) on some platforms. 64 KiB
// fits every legitimate use and contains pathological inputs.
const MaxVarEnvLen = 64 * 1024

// VarsAsEnv converts workflow vars (webhook-derived, trigger-time, author-supplied)
// to SKY_<KEY>=<value> env pairs for bash nodes. Keys are sanitized to A-Z0-9_
// (dots/dashes → underscore, upper-cased). Keys that sanitize to empty or start
// with a digit are dropped. Values go through sanitizeEnvValue: NUL stripped
// (os/exec rejects env with embedded NUL), then truncated at MaxVarEnvLen bytes.
//
// This is the ONLY channel by which webhook vars reach a bash subprocess.
// Template expansion (workflow.Expand) must never be applied to bash command
// text — doing so would splice attacker input into shell syntax (issue title
// "$(id)" → arbitrary code execution).
func VarsAsEnv(vars map[string]string) []string {
	if len(vars) == 0 {
		return nil
	}
	env := make([]string, 0, len(vars))
	for k, v := range vars {
		key := SanitizeVarKey(k)
		if key == "" {
			continue
		}
		v = sanitizeEnvValue(v)
		env = append(env, "SKY_"+key+"="+v)
	}
	return env
}

// sanitizeEnvValue makes a string safe to pass as an env value to os/exec.
// Go's syscall layer rejects env entries containing NUL (0x00) — an attacker
// who smuggles a NUL through webhook vars or Claude output would otherwise
// fail the whole subprocess at spawn time. Strip NUL and apply the length cap.
func sanitizeEnvValue(v string) string {
	if strings.IndexByte(v, 0) >= 0 {
		v = strings.ReplaceAll(v, "\x00", "")
	}
	if len(v) > MaxVarEnvLen {
		v = v[:MaxVarEnvLen]
	}
	return v
}

// SanitizeEnvValue is the exported form of sanitizeEnvValue for callers
// outside this package (e.g. runner.outputsAsEnv) that produce env values
// from untrusted sources.
func SanitizeEnvValue(v string) string { return sanitizeEnvValue(v) }

// SanitizeVarKey maps a workflow var name to an env-var-safe token.
// "issue.number" → "ISSUE_NUMBER". Returns "" for unusable inputs (empty after
// sanitize, leading digit). ASCII-only by design — non-ASCII drops silently so
// untrusted key names cannot widen the accepted charset.
func SanitizeVarKey(k string) string {
	if k == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(k))
	for i := 0; i < len(k); i++ {
		c := k[i]
		switch {
		case c >= 'A' && c <= 'Z':
			b.WriteByte(c)
		case c >= 'a' && c <= 'z':
			b.WriteByte(c - 32)
		case c >= '0' && c <= '9':
			b.WriteByte(c)
		case c == '_' || c == '.' || c == '-':
			b.WriteByte('_')
		}
	}
	s := b.String()
	if s == "" {
		return ""
	}
	if s[0] >= '0' && s[0] <= '9' {
		return ""
	}
	return s
}
