package workflow

import (
	"os"
	"path/filepath"
	"testing"
)

func makeCommandDir(t *testing.T, files map[string]string) Roots {
	t.Helper()
	root := t.TempDir()
	cmdDir := filepath.Join(root, "commands")
	if err := os.MkdirAll(cmdDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(cmdDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return Roots{Repo: root}
}

func TestLoadCommandFromRoots_WithFrontmatter(t *testing.T) {
	roots := makeCommandDir(t, map[string]string{
		"deploy.md": "---\ndescription: Deploy the service\nargument-hint: <env>\n---\nRun deployment to $1.",
	})
	cmd, err := LoadCommandFromRoots(roots, "deploy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd.Name != "deploy" {
		t.Errorf("Name = %q, want %q", cmd.Name, "deploy")
	}
	if cmd.Description != "Deploy the service" {
		t.Errorf("Description = %q, want %q", cmd.Description, "Deploy the service")
	}
	if cmd.ArgumentHint != "<env>" {
		t.Errorf("ArgumentHint = %q, want %q", cmd.ArgumentHint, "<env>")
	}
	if cmd.Prompt != "Run deployment to $1." {
		t.Errorf("Prompt = %q, want %q", cmd.Prompt, "Run deployment to $1.")
	}
}

func TestLoadCommandFromRoots_NoFrontmatter(t *testing.T) {
	roots := makeCommandDir(t, map[string]string{
		"simple.md": "Just a plain prompt with no frontmatter.",
	})
	cmd, err := LoadCommandFromRoots(roots, "simple")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd.Description != "" {
		t.Errorf("Description = %q, want empty", cmd.Description)
	}
	if cmd.Prompt != "Just a plain prompt with no frontmatter." {
		t.Errorf("Prompt = %q", cmd.Prompt)
	}
}

func TestLoadCommandFromRoots_UnclosedFrontmatter(t *testing.T) {
	roots := makeCommandDir(t, map[string]string{
		"unclosed.md": "---\ndescription: No closing delimiter\nSome body text",
	})
	cmd, err := LoadCommandFromRoots(roots, "unclosed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No closing --- so falls back to treating entire content as prompt.
	if cmd.Prompt == "" {
		t.Error("want non-empty Prompt for unclosed frontmatter")
	}
}

func TestLoadCommandFromRoots_NotFound(t *testing.T) {
	roots := makeCommandDir(t, nil)
	_, err := LoadCommandFromRoots(roots, "missing")
	if err == nil {
		t.Fatal("want error for missing command, got nil")
	}
}

func TestLoadCommandFromRoots_InvalidName(t *testing.T) {
	roots := Roots{Repo: t.TempDir()}
	_, err := LoadCommandFromRoots(roots, "../../etc/passwd")
	if err == nil {
		t.Fatal("want error for invalid command name, got nil")
	}
}

func TestLoadCommandFromRoots_FallbackToUser(t *testing.T) {
	// Repo tier has no commands; user tier has one.
	repoRoot := t.TempDir()
	userRoot := t.TempDir()
	userCmds := filepath.Join(userRoot, "commands")
	if err := os.MkdirAll(userCmds, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(userCmds, "shared.md"), []byte("User-level prompt."), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	roots := Roots{Repo: repoRoot, User: userRoot}
	cmd, err := LoadCommandFromRoots(roots, "shared")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd.Prompt != "User-level prompt." {
		t.Errorf("Prompt = %q, want %q", cmd.Prompt, "User-level prompt.")
	}
}

func TestLoadCommand_DeprecatedWrapper(t *testing.T) {
	dir := t.TempDir()
	cmdDir := filepath.Join(dir, ".sky", "commands")
	if err := os.MkdirAll(cmdDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "hello.md"), []byte("Hello prompt."), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cmd, err := LoadCommand(dir, "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd.Prompt != "Hello prompt." {
		t.Errorf("Prompt = %q, want %q", cmd.Prompt, "Hello prompt.")
	}
}

func TestParseKV(t *testing.T) {
	cases := []struct {
		line      string
		wantKey   string
		wantValue string
		wantOK    bool
	}{
		{"description: Deploy the app", "description", "Deploy the app", true},
		{"argument-hint: <env>", "argument-hint", "<env>", true},
		{"key: value: with colon", "key", "value: with colon", true},
		{"no colon here", "", "", false},
		{"", "", "", false},
	}
	for _, tc := range cases {
		k, v, ok := parseKV(tc.line)
		if ok != tc.wantOK || k != tc.wantKey || v != tc.wantValue {
			t.Errorf("parseKV(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tc.line, k, v, ok, tc.wantKey, tc.wantValue, tc.wantOK)
		}
	}
}
