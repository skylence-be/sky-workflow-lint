package workflow

import (
	"strings"
	"testing"
)

func TestLoadBuiltinWorkflows_RepoConventionEnforcer_UIDocs(t *testing.T) {
	t.Parallel()
	workflows, err := LoadBuiltinWorkflows()
	if err != nil {
		t.Fatalf("LoadBuiltinWorkflows: %v", err)
	}

	var wf *Workflow
	for _, w := range workflows {
		if w.Name == "repo-convention-enforcer" {
			wf = w
			break
		}
	}
	if wf == nil {
		t.Fatal("repo-convention-enforcer not found in builtin workflows")
	}

	// (a) workflow-level UIDoc has en and nl with non-empty Short and Body.
	for _, locale := range []string{"en", "nl"} {
		doc, ok := wf.UIDoc[locale]
		if !ok {
			t.Fatalf("workflow UIDoc missing locale %q; have %v", locale, wf.UIDoc)
		}
		if doc.Short == "" {
			t.Errorf("workflow UIDoc[%q].Short is empty", locale)
		}
		if doc.Body == "" {
			t.Errorf("workflow UIDoc[%q].Body is empty", locale)
		}
		// (c) frontmatter-leak guard: body must never contain the raw key name.
		if strings.Contains(doc.Body, "short_description") {
			t.Errorf("workflow UIDoc[%q].Body leaks frontmatter: %q", locale, doc.Body)
		}
	}

	// (b) every node has UIDocs with both locales.
	if len(wf.Nodes) == 0 {
		t.Fatal("expected nodes on repo-convention-enforcer")
	}
	for _, n := range wf.Nodes {
		for _, locale := range []string{"en", "nl"} {
			doc, ok := n.UIDocs[locale]
			if !ok {
				t.Errorf("node %q UIDocs missing locale %q; have %v", n.ID, locale, n.UIDocs)
				continue
			}
			if doc.Short == "" {
				t.Errorf("node %q UIDocs[%q].Short is empty", n.ID, locale)
			}
			if doc.Body == "" {
				t.Errorf("node %q UIDocs[%q].Body is empty", n.ID, locale)
			}
			if strings.Contains(doc.Body, "short_description") {
				t.Errorf("node %q UIDocs[%q].Body leaks frontmatter: %q", n.ID, locale, doc.Body)
			}
		}
	}

	// (d) node Docs blocks are populated for every node.
	docsByID := make(map[string]bool)
	for _, d := range wf.Docs {
		docsByID[d.StepID] = true
	}
	for _, n := range wf.Nodes {
		if !docsByID[n.ID] {
			t.Errorf("node %q has no ※ doc block", n.ID)
		}
	}
}

// TestLoadBuiltinWorkflows_AllHaveUIDocsAndDocs guards the other two builtin
// workflows at a lighter level: every builtin workflow must carry both
// locales at the workflow level, per-node UIDocs, and a Docs block per node.
func TestLoadBuiltinWorkflows_AllHaveUIDocsAndDocs(t *testing.T) {
	t.Parallel()
	workflows, err := LoadBuiltinWorkflows()
	if err != nil {
		t.Fatalf("LoadBuiltinWorkflows: %v", err)
	}
	if len(workflows) == 0 {
		t.Fatal("expected at least one builtin workflow")
	}

	for _, wf := range workflows {
		for _, locale := range []string{"en", "nl"} {
			doc, ok := wf.UIDoc[locale]
			if !ok || doc.Short == "" || doc.Body == "" {
				t.Errorf("workflow %q: UIDoc[%q] missing or incomplete", wf.Name, locale)
			}
		}

		docsByID := make(map[string]bool)
		for _, d := range wf.Docs {
			docsByID[d.StepID] = true
		}

		for _, n := range wf.Nodes {
			if !docsByID[n.ID] {
				t.Errorf("workflow %q: node %q has no ※ doc block", wf.Name, n.ID)
			}
			for _, locale := range []string{"en", "nl"} {
				doc, ok := n.UIDocs[locale]
				if !ok || doc.Short == "" || doc.Body == "" {
					t.Errorf("workflow %q: node %q UIDocs[%q] missing or incomplete", wf.Name, n.ID, locale)
				}
			}
		}
	}
}
