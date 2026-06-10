package workflow_test

import (
	"encoding/json"
	"testing"

	"github.com/skylence-be/sky-workflow-lint/workflow"
)

func TestManifest_Shape(t *testing.T) {
	m := workflow.Manifest("test")

	if m.SchemaVersion != "1" {
		t.Errorf("schema_version = %q, want %q", m.SchemaVersion, "1")
	}
	if m.SkyVersion != "test" {
		t.Errorf("sky_version = %q, want %q", m.SkyVersion, "test")
	}
	if len(m.Format.Delimiters) != 4 {
		t.Errorf("delimiters count = %d, want 4", len(m.Format.Delimiters))
	}
	if len(m.Nodes.Kinds) == 0 {
		t.Error("nodes.kinds is empty")
	}
	if len(m.LintCodes) == 0 {
		t.Error("lint_codes is empty")
	}
	if len(m.Params.Types) == 0 {
		t.Error("params.types is empty")
	}
}

func TestManifest_JSON(t *testing.T) {
	m := workflow.Manifest("v1.0.0")
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var check map[string]any
	if err := json.Unmarshal(b, &check); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	for _, key := range []string{"schema_version", "sky_version", "format", "nodes", "triggers", "when_grammar", "params", "templates", "lint_codes"} {
		if _, ok := check[key]; !ok {
			t.Errorf("manifest JSON missing key %q", key)
		}
	}
}
