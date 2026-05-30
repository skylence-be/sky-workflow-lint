package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestExpandPatterns_doublestar(t *testing.T) {
	// Build a temp tree: root/a/b/c/one.sky  root/a/two.sky  root/a/b/ignore.txt
	tmp := t.TempDir()
	mkfile := func(rel string) {
		full := filepath.Join(tmp, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mkfile("a/b/c/one.sky")
	mkfile("a/two.sky")
	mkfile("a/b/ignore.txt")

	// Run from within tmp so relative paths work.
	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) }) //nolint:errcheck
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	files, err := expandPatterns([]string{"**/*.sky"})
	if err != nil {
		t.Fatalf("expandPatterns: %v", err)
	}

	want := []string{
		filepath.Join("a", "b", "c", "one.sky"),
		filepath.Join("a", "two.sky"),
	}
	slices.Sort(files)
	slices.Sort(want)

	if len(files) != len(want) {
		t.Fatalf("got %v, want %v", files, want)
	}
	for i := range want {
		if files[i] != want[i] {
			t.Errorf("files[%d] = %q, want %q", i, files[i], want[i])
		}
	}
}

func TestExpandPatterns_doublestar_withPrefix(t *testing.T) {
	tmp := t.TempDir()
	mkfile := func(rel string) {
		full := filepath.Join(tmp, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mkfile("meta/audit/workflows/foo/workflow.sky")
	mkfile("meta/authoring/workflows/bar/workflow.sky")
	mkfile("other/workflow.sky") // outside meta — should not match

	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) }) //nolint:errcheck
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	files, err := expandPatterns([]string{"meta/**/*.sky"})
	if err != nil {
		t.Fatalf("expandPatterns: %v", err)
	}

	for _, f := range files {
		if filepath.Dir(filepath.Dir(f)) == "other" {
			t.Errorf("unexpected match outside prefix: %s", f)
		}
	}
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2: %v", len(files), files)
	}
}
