package workflow

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeVarKey(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"issue.number", "ISSUE_NUMBER"},
		{"repo.full_name", "REPO_FULL_NAME"},
		{"pr.title", "PR_TITLE"},
		{"ISSUE.TITLE", "ISSUE_TITLE"},
		{"already_ok", "ALREADY_OK"},
		{"kebab-case", "KEBAB_CASE"},
		{"with spaces", "WITHSPACES"},
		{"bad key!", "BADKEY"},
		{"$(id)", "ID"},
		{"", ""},
		{"1leading", ""},
		{"123", ""},
		{"_under", "_UNDER"},
		{"émoji☃key", "MOJIKEY"},
		{"!!!", ""},
		{"...", "___"},
		{" leading-space", "LEADING_SPACE"},
	}
	for _, tt := range tests {
		if got := SanitizeVarKey(tt.in); got != tt.want {
			t.Errorf("SanitizeVarKey(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestVarsAsEnv_Nil(t *testing.T) {
	if got := VarsAsEnv(nil); got != nil {
		t.Errorf("VarsAsEnv(nil) = %v, want nil", got)
	}
	if got := VarsAsEnv(map[string]string{}); got != nil {
		t.Errorf("VarsAsEnv(empty) = %v, want nil", got)
	}
}

func TestVarsAsEnv_KeySanitization(t *testing.T) {
	// Intentionally malformed keys — whitespace/empty/leading-digit — to
	// exercise SanitizeVarKey's rejection paths.
	in := map[string]string{
		"issue.number":   "42",
		"repo.full_name": "owner/repo",
		"1bad":           "dropped-leading-digit",
		"":               "dropped-empty",
		"pr.title":       "Fix auth",
	}
	in[" leading-space"] = "kept" //nolint:gocritic // exercising sanitize path
	env := VarsAsEnv(in)
	got := map[string]string{}
	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		got[parts[0]] = parts[1]
	}
	cases := map[string]string{
		"SKY_ISSUE_NUMBER":   "42",
		"SKY_REPO_FULL_NAME": "owner/repo",
		"SKY_PR_TITLE":       "Fix auth",
		"SKY_LEADING_SPACE":  "kept",
	}
	for k, want := range cases {
		if got[k] != want {
			t.Errorf("%s: got %q, want %q", k, got[k], want)
		}
	}
	if _, ok := got["SKY_1BAD"]; ok {
		t.Errorf("leading-digit key should be dropped")
	}
	if _, ok := got["SKY_"]; ok {
		t.Errorf("empty-after-sanitize key should be dropped")
	}
}

func TestVarsAsEnv_ValueLengthCap(t *testing.T) {
	huge := strings.Repeat("A", MaxVarEnvLen+1024)
	env := VarsAsEnv(map[string]string{"issue.body": huge})
	if len(env) != 1 {
		t.Fatalf("expected 1 env entry, got %d", len(env))
	}
	val := strings.TrimPrefix(env[0], "SKY_ISSUE_BODY=")
	if len(val) != MaxVarEnvLen {
		t.Errorf("value length = %d, want %d", len(val), MaxVarEnvLen)
	}
}

func TestVarsAsEnv_InjectionPayloadPassesAsLiteral(t *testing.T) {
	// The whole point of the env channel: attacker payload never becomes shell
	// syntax. Pass it into bash via env and assert the subprocess sees it as a
	// literal string, not as something that executed.
	dir := t.TempDir()
	canary := filepath.Join(dir, "pwned")

	vars := map[string]string{
		"issue.title": "$(touch " + canary + ")",
		"issue.body":  "`touch " + canary + "`",
	}
	env := VarsAsEnv(vars)

	script := `printf '%s' "$SKY_ISSUE_TITLE"`
	cmd := exec.CommandContext(context.Background(), "bash", "-c", script)
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v (%s)", err, out)
	}
	if string(out) != vars["issue.title"] {
		t.Errorf("bash saw %q, want %q", out, vars["issue.title"])
	}
	if _, statErr := os.Stat(canary); statErr == nil {
		t.Fatalf("canary file exists: command substitution fired — injection not contained")
	}
}

func TestVarsAsEnv_StripsNULBytes(t *testing.T) {
	// Go's os/exec rejects env values containing NUL. A webhook payload
	// (or upstream node output routed through SanitizeEnvValue) with an
	// embedded NUL would fail the whole bash node at spawn time otherwise.
	env := VarsAsEnv(map[string]string{
		"issue.title": "before\x00after",
		"issue.body":  "\x00leading",
	})
	got := map[string]string{}
	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		got[parts[0]] = parts[1]
	}
	if got["SKY_ISSUE_TITLE"] != "beforeafter" {
		t.Errorf("issue.title = %q, want %q", got["SKY_ISSUE_TITLE"], "beforeafter")
	}
	if got["SKY_ISSUE_BODY"] != "leading" {
		t.Errorf("issue.body = %q, want %q", got["SKY_ISSUE_BODY"], "leading")
	}
	for _, e := range env {
		if strings.ContainsRune(e, 0) {
			t.Errorf("env entry still contains NUL: %q", e)
		}
	}
}

func TestSanitizeEnvValue_CapAndNUL(t *testing.T) {
	// Combined: strip NUL then cap at MaxVarEnvLen.
	v := "\x00" + strings.Repeat("A", MaxVarEnvLen+8) + "\x00"
	got := SanitizeEnvValue(v)
	if strings.ContainsRune(got, 0) {
		t.Error("NUL not stripped")
	}
	if len(got) != MaxVarEnvLen {
		t.Errorf("len = %d, want %d", len(got), MaxVarEnvLen)
	}
}

func TestVarsAsEnv_EmptyKeyAfterSanitize(t *testing.T) {
	env := VarsAsEnv(map[string]string{
		"!!!": "dropped",
		"🎉":   "dropped",
		"":    "dropped",
	})
	if len(env) != 0 {
		t.Errorf("expected empty env, got %v", env)
	}
}
