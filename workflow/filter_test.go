package workflow

import (
	"os"
	"path/filepath"
	"testing"
)

// writeWFWithSource creates a .sky workflow under dir/workflows/<filename>.sky
// whose meta block carries a _library_source attribution (as install-time
// injection would produce). An empty source omits the attribution line, so the
// resulting Workflow.LibrarySource is nil.
func writeWFWithSource(t *testing.T, dir, name, wfName, source string) {
	t.Helper()
	wfDir := filepath.Join(dir, "workflows")
	if err := os.MkdirAll(wfDir, 0o750); err != nil {
		t.Fatal(err)
	}
	meta := "⊕meta⊕\nname = \"" + wfName + "\"\n"
	if source != "" {
		meta += "_library_source = \"" + source + "\"\n"
	}
	meta += "⊕⊕\n\n§n1§\nbash = \"echo hi\"\n§§\n"
	if err := os.WriteFile(filepath.Join(wfDir, name+".sky"), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestLoadAllFromRoots_NoOptionReturnsAll is a regression guard: with no
// options the loader admits every workflow regardless of library source,
// exactly as before the filter was added.
func TestLoadAllFromRoots_NoOptionReturnsAll(t *testing.T) {
	user := t.TempDir()
	writeWFWithSource(t, user, "fromlib", "fromlib", "owner/lib")
	writeWFWithSource(t, user, "local", "local", "") // no library source

	wfs, err := LoadAllFromRoots(Roots{User: user})
	if err != nil {
		t.Fatalf("LoadAllFromRoots: %v", err)
	}
	if len(wfs) != 2 {
		t.Fatalf("got %d workflows, want 2 (no filter must return all)", len(wfs))
	}
}

// TestLoadAllFromRoots_FilterOnlyInSet checks that an active source filter
// admits only workflows whose library source is in the allowlist.
func TestLoadAllFromRoots_FilterOnlyInSet(t *testing.T) {
	user := t.TempDir()
	writeWFWithSource(t, user, "a", "a", "owner/allowed")
	writeWFWithSource(t, user, "b", "b", "owner/blocked")
	writeWFWithSource(t, user, "c", "c", "") // no library source

	wfs, err := LoadAllFromRoots(Roots{User: user}, WithAllowedSources("owner/allowed"))
	if err != nil {
		t.Fatalf("LoadAllFromRoots: %v", err)
	}
	if len(wfs) != 1 {
		t.Fatalf("got %d workflows, want 1 in-set only", len(wfs))
	}
	if wfs[0].Name != "a" {
		t.Errorf("admitted %q, want %q", wfs[0].Name, "a")
	}
}

// TestWithAllowedSources_EmptyResolvesNothing documents that an empty (but
// present) allowlist resolves nothing; callers omit the option for no filter.
func TestWithAllowedSources_EmptyResolvesNothing(t *testing.T) {
	user := t.TempDir()
	writeWFWithSource(t, user, "a", "a", "owner/allowed")

	wfs, err := LoadAllFromRoots(Roots{User: user}, WithAllowedSources())
	if err != nil {
		t.Fatalf("LoadAllFromRoots: %v", err)
	}
	if len(wfs) != 0 {
		t.Fatalf("got %d workflows, want 0 for empty allowlist", len(wfs))
	}
}

// TestResolve_FilterInSet resolves an in-set workflow by name.
func TestResolve_FilterInSet(t *testing.T) {
	user := t.TempDir()
	writeWFWithSource(t, user, "deploy", "deploy", "owner/allowed")

	wf, err := Resolve("deploy", Roots{User: user}, WithAllowedSources("owner/allowed"))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if wf.LibrarySource == nil || *wf.LibrarySource != "owner/allowed" {
		t.Errorf("LibrarySource = %v, want owner/allowed", wf.LibrarySource)
	}
}

// TestResolve_FilterExcludesOutOfSet returns not-found when the only matching
// workflow's library source is outside the allowlist.
func TestResolve_FilterExcludesOutOfSet(t *testing.T) {
	user := t.TempDir()
	writeWFWithSource(t, user, "deploy", "deploy", "owner/blocked")

	if _, err := Resolve("deploy", Roots{User: user}, WithAllowedSources("owner/allowed")); err == nil {
		t.Fatal("expected not-found for out-of-set workflow, got nil")
	}
}

// TestResolve_FilterExcludesNilSource returns not-found when the matching
// workflow has no library source at all (hand-authored / repo tier).
func TestResolve_FilterExcludesNilSource(t *testing.T) {
	user := t.TempDir()
	writeWFWithSource(t, user, "deploy", "deploy", "") // no library source

	if _, err := Resolve("deploy", Roots{User: user}, WithAllowedSources("owner/allowed")); err == nil {
		t.Fatal("expected not-found for nil-source workflow under filter, got nil")
	}
}

// TestResolve_FilterSkipsShadowingRepoTier documents that under an active
// filter, a higher-tier workflow whose source is out of set does not shadow a
// lower-tier in-set workflow of the same name: the in-set one resolves.
func TestResolve_FilterSkipsShadowingRepoTier(t *testing.T) {
	repo := t.TempDir()
	user := t.TempDir()
	writeWFWithSource(t, repo, "deploy", "deploy", "") // repo tier, no source
	writeWFWithSource(t, user, "deploy", "deploy", "owner/allowed")

	wf, err := Resolve("deploy", Roots{Repo: repo, User: user}, WithAllowedSources("owner/allowed"))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if wf.Source != SourceUser {
		t.Errorf("Source = %q, want %q (in-set user tier must win under filter)", wf.Source, SourceUser)
	}
}
