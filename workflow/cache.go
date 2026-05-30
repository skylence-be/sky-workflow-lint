package workflow

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

// Cache memoises LoadAllFromRoots, invalidating only when the file-mtime
// signature of any tier's workflows/ dir changes. Walking the dirs to compute
// the signature is cheap (one stat per .sky file); parsing is the cost we
// avoid. Safe for concurrent use.
//
// A single Cache may serve many distinct Roots — entries are keyed by the
// joined tier paths.
type Cache struct {
	mu      sync.Mutex
	entries map[string]*cacheEntry
}

type cacheEntry struct {
	sig string
	wfs []*Workflow
}

// NewCache returns an empty cache.
func NewCache() *Cache {
	return &Cache{entries: make(map[string]*cacheEntry)}
}

// LoadAll returns workflows for roots, re-parsing only when any .sky file
// under any tier's workflows/ dir has changed (added, removed, or modified
// in place — detected via path:size:mtime signature).
//
// Returns the same []*Workflow slice across cache hits; callers must not
// mutate the result.
func (c *Cache) LoadAll(roots Roots) ([]*Workflow, error) {
	if c == nil {
		return LoadAllFromRoots(roots)
	}
	key := roots.Repo + "\x00" + roots.Workspace + "\x00" + roots.User
	sig, err := rootsSignature(roots)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if e, ok := c.entries[key]; ok && e.sig == sig {
		return e.wfs, nil
	}

	wfs, err := LoadAllFromRoots(roots)
	if err != nil {
		return nil, err
	}

	c.entries[key] = &cacheEntry{sig: sig, wfs: wfs}
	return wfs, nil
}

// rootsSignature returns a string that changes whenever any .sky file under
// any tier's workflows/ dir is added, removed, or modified. Errors stat'ing
// individual files are folded into the signature (so the caller will re-parse
// and surface the real error).
func rootsSignature(roots Roots) (string, error) {
	var b []byte
	for _, t := range roots.ordered() {
		b = append(b, '\x01')
		if t.path == "" {
			continue
		}
		dir := filepath.Join(t.path, "workflows")
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", err
		}
		for _, e := range entries {
			if e.IsDir() || !isSky(e.Name()) {
				continue
			}
			info, err := e.Info()
			if err != nil {
				// Treat as "changed" — let LoadAllFromRoots surface the real error.
				b = append(b, '\x02')
				b = append(b, e.Name()...)
				continue
			}
			b = append(b, '\x03')
			b = append(b, e.Name()...)
			b = append(b, ':')
			b = strconv.AppendInt(b, info.Size(), 10)
			b = append(b, ':')
			b = strconv.AppendInt(b, info.ModTime().UnixNano(), 10)
		}
	}
	return string(b), nil
}
