package workflow

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeSky(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

const cacheTestSky = `⊕meta⊕
name = "wf-test"
⊕⊕

§handle§
bash = "echo done"
§§
`

func TestCache_HitsAndMisses(t *testing.T) {
	root := t.TempDir()
	writeSky(t, filepath.Join(root, "workflows"), "wf.sky", cacheTestSky)
	roots := Roots{Repo: root}
	c := NewCache()

	first, err := c.LoadAll(roots)
	if err != nil {
		t.Fatalf("load 1: %v", err)
	}
	second, err := c.LoadAll(roots)
	if err != nil {
		t.Fatalf("load 2: %v", err)
	}
	// Same slice header on cache hit (we return the cached value verbatim).
	if &first[0] != &second[0] {
		t.Errorf("cache miss on second call: got distinct slices")
	}
}

func TestCache_InvalidatesOnMtimeChange(t *testing.T) {
	root := t.TempDir()
	writeSky(t, filepath.Join(root, "workflows"), "wf.sky", cacheTestSky)
	roots := Roots{Repo: root}
	c := NewCache()

	first, err := c.LoadAll(roots)
	if err != nil {
		t.Fatalf("load 1: %v", err)
	}

	// Bump mtime to a clearly different value (advance forward to avoid
	// nanosecond-precision collisions on filesystems that round to seconds).
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(filepath.Join(root, "workflows", "wf.sky"), future, future); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	second, err := c.LoadAll(roots)
	if err != nil {
		t.Fatalf("load 2: %v", err)
	}
	if len(first) > 0 && len(second) > 0 && &first[0] == &second[0] {
		t.Errorf("expected cache invalidation after mtime change, got same slice")
	}
}

func TestCache_NilReceiver(t *testing.T) {
	root := t.TempDir()
	writeSky(t, filepath.Join(root, "workflows"), "wf.sky", cacheTestSky)
	roots := Roots{Repo: root}

	var c *Cache
	wfs, err := c.LoadAll(roots)
	if err != nil {
		t.Fatalf("nil cache LoadAll: %v", err)
	}
	if len(wfs) < 1 {
		t.Errorf("got %d workflows, want at least 1", len(wfs))
	}
}
