package workflow

import (
	"os"
	"path/filepath"
	"testing"
)

// writeWF creates a minimal .sky workflow file under dir/workflows/<filename>.sky.
func writeWF(t *testing.T, dir, name, wfName string) {
	t.Helper()
	wfDir := filepath.Join(dir, "workflows")
	if err := os.MkdirAll(wfDir, 0o750); err != nil {
		t.Fatal(err)
	}
	content := "⊕meta⊕\nname = \"" + wfName + "\"\n⊕⊕\n\n§n1§\nbash = \"echo hi\"\n§§\n"
	if err := os.WriteFile(filepath.Join(wfDir, name+".sky"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolve_RepoWins(t *testing.T) {
	repo := t.TempDir()
	workspace := t.TempDir()
	user := t.TempDir()

	writeWF(t, repo, "deploy", "deploy")
	writeWF(t, workspace, "deploy", "deploy")
	writeWF(t, user, "deploy", "deploy")

	roots := Roots{Repo: repo, Workspace: workspace, User: user}
	wf, err := Resolve("deploy", roots)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if wf.Source != SourceRepo {
		t.Errorf("Source = %q, want %q", wf.Source, SourceRepo)
	}
}

func TestResolve_WorkspaceFallback(t *testing.T) {
	workspace := t.TempDir()
	user := t.TempDir()
	writeWF(t, workspace, "triage", "triage")
	writeWF(t, user, "triage", "triage")

	roots := Roots{Repo: "", Workspace: workspace, User: user}
	wf, err := Resolve("triage", roots)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if wf.Source != SourceWorkspace {
		t.Errorf("Source = %q, want %q", wf.Source, SourceWorkspace)
	}
}

func TestResolve_UserFallback(t *testing.T) {
	user := t.TempDir()
	writeWF(t, user, "review", "review")

	roots := Roots{Repo: "", Workspace: "", User: user}
	wf, err := Resolve("review", roots)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if wf.Source != SourceUser {
		t.Errorf("Source = %q, want %q", wf.Source, SourceUser)
	}
}

func TestResolve_NotFound(t *testing.T) {
	roots := Roots{Repo: t.TempDir(), Workspace: "", User: ""}
	if _, err := Resolve("nonexistent", roots); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestResolve_MissingDirsSkipped(t *testing.T) {
	user := t.TempDir()
	writeWF(t, user, "ci", "ci")

	roots := Roots{
		Repo:      "/does/not/exist",
		Workspace: "/also/missing",
		User:      user,
	}
	wf, err := Resolve("ci", roots)
	if err != nil {
		t.Fatalf("Resolve with missing tiers: %v", err)
	}
	if wf.Source != SourceUser {
		t.Errorf("Source = %q, want %q", wf.Source, SourceUser)
	}
}

func TestResolve_SkipsUnparseableWorkflow(t *testing.T) {
	repo := t.TempDir()

	writeWF(t, repo, "good", "good")

	brokenContent := "§handle§\nbash = \"echo done\"\n§§\n"
	if err := os.WriteFile(filepath.Join(repo, "workflows", "broken.sky"), []byte(brokenContent), 0o644); err != nil {
		t.Fatal(err)
	}

	wf, err := Resolve("good", Roots{Repo: repo})
	if err != nil {
		t.Fatalf("Resolve: %v (broken.sky blocked resolution of good.sky)", err)
	}
	if wf.Name != "good" {
		t.Errorf("Name = %q, want %q", wf.Name, "good")
	}
	if wf.Source != SourceRepo {
		t.Errorf("Source = %q, want %q", wf.Source, SourceRepo)
	}
}

func TestLoadAllFromRoots_Precedence(t *testing.T) {
	repo := t.TempDir()
	workspace := t.TempDir()
	user := t.TempDir()

	// "deploy" in all three tiers — repo must win
	writeWF(t, repo, "deploy", "deploy")
	writeWF(t, workspace, "deploy", "deploy")
	writeWF(t, user, "deploy", "deploy")
	// "triage" only in workspace and user — workspace must win
	writeWF(t, workspace, "triage", "triage")
	writeWF(t, user, "triage", "triage")
	// "review" only in user
	writeWF(t, user, "review", "review")

	roots := Roots{Repo: repo, Workspace: workspace, User: user}
	wfs, err := LoadAllFromRoots(roots)
	if err != nil {
		t.Fatalf("LoadAllFromRoots: %v", err)
	}

	byName := make(map[string]*Workflow)
	for _, wf := range wfs {
		byName[wf.Name] = wf
	}

	if len(byName) < 3 {
		t.Fatalf("got %d distinct workflows, want at least 3", len(byName))
	}
	if byName["deploy"].Source != SourceRepo {
		t.Errorf("deploy source = %q, want repo", byName["deploy"].Source)
	}
	if byName["triage"].Source != SourceWorkspace {
		t.Errorf("triage source = %q, want workspace", byName["triage"].Source)
	}
	if byName["review"].Source != SourceUser {
		t.Errorf("review source = %q, want user", byName["review"].Source)
	}
}
