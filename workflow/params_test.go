package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParamsParseAndValidateDefaults(t *testing.T) {
	input := `⊕meta⊕
name = "typed"
params.title.type = "string"
params.title.required = true
params.title.default = "hello"
params.count.type = "number"
params.count.default = 3
params.ok.type = "boolean"
params.ok.default = true
params.priority.type = "enum"
params.priority.enum = ["low", "high"]
params.priority.default = "high"
⊕⊕

§run§
prompt = "{{title}} {{count}} {{ok}} {{priority}}"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(wf.Params) != 4 {
		t.Fatalf("params count = %d, want 4", len(wf.Params))
	}
	if wf.Params["title"].Type != ParamTypeString || !wf.Params["title"].Required {
		t.Fatalf("title param = %+v", wf.Params["title"])
	}
	if diags := LintBytes("typed.sky", []byte(input)); hasCode(diags, string(CodeTemplateUndeclared)) {
		t.Fatalf("declared params should not warn as undeclared: %+v", diags)
	}
}

func TestParamsInvalidDefaultTypedCode(t *testing.T) {
	input := `⊕meta⊕
name = "bad"
params.count.type = "number"
params.count.default = "three"
⊕⊕

§run§
prompt = "hello"
§§
`
	diags := LintBytes("bad.sky", []byte(input))
	if !hasCode(diags, string(CodeParamInvalid)) {
		t.Fatalf("want %s for invalid params default, got %+v", CodeParamInvalid, diags)
	}
}

func TestTemplateParamUndeclaredWarning(t *testing.T) {
	input := `⊕meta⊕
name = "warn"
params.title.type = "string"
⊕⊕

§run§
prompt = "{{title}} {{typo}}"
§§
`
	diags := LintBytes("warn.sky", []byte(input))
	var found bool
	for _, d := range diags {
		if d.Code == string(CodeTemplateUndeclared) {
			found = true
			if d.Severity != "warning" {
				t.Fatalf("severity = %q, want warning", d.Severity)
			}
			if !strings.Contains(d.Message, "typo") {
				t.Fatalf("message = %q, want var name", d.Message)
			}
		}
	}
	if !found {
		t.Fatalf("want %s warning, got %+v", CodeTemplateUndeclared, diags)
	}
}

func TestTemplateParamAllowsTriggerInjectedVars(t *testing.T) {
	input := `⊕meta⊕
name = "gh"
trigger.github.events = ["issues.opened"]
⊕⊕

§run§
prompt = "{{issue.number}} {{repo.full_name}} {{sender.login}}"
§§
`
	diags := LintBytes("gh.sky", []byte(input))
	if hasCode(diags, string(CodeTemplateUndeclared)) {
		t.Fatalf("known github trigger vars should not warn: %+v", diags)
	}
}

func TestTemplateParamAllowsForeachItemVars(t *testing.T) {
	input := `⊕meta⊕
name = "fe"
⊕⊕

§list§
bash = "echo '[\"a\",\"b\"]'"
§§

§work§
depends_on = ["list"]
foreach = {"items": "$list.output", "max_concurrency": 2}
prompt = "process {{item}} ({{item_index}}/{{item_total}})"
§§
`
	diags := LintBytes("fe.sky", []byte(input))
	if hasCode(diags, string(CodeTemplateUndeclared)) {
		t.Fatalf("foreach item vars should not warn as undeclared: %+v", diags)
	}
}

func TestTemplateParamItemVarWarnsWithoutForeach(t *testing.T) {
	input := `⊕meta⊕
name = "noreach"
⊕⊕

§run§
prompt = "process {{item}}"
§§
`
	diags := LintBytes("noreach.sky", []byte(input))
	if !hasCode(diags, string(CodeTemplateUndeclared)) {
		t.Fatalf("item used outside a foreach should still warn: %+v", diags)
	}
}

func TestInvokeParamsCrossCheck(t *testing.T) {
	root := t.TempDir()
	workflowDir := filepath.Join(root, "workflows")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatal(err)
	}
	child := `⊕meta⊕
name = "child"
params.title.type = "string"
params.title.required = true
params.count.type = "number"
params.mode.type = "enum"
params.mode.enum = ["fast", "slow"]
⊕⊕

§run§
prompt = "{{title}}"
§§
`
	parent := `⊕meta⊕
name = "parent"
⊕⊕

§call§
invoke.target = "child"
invoke.vars.count = "not-a-number"
invoke.vars.extra = "x"
invoke.vars.mode = "fast"
§§
`
	if err := os.WriteFile(filepath.Join(workflowDir, "child.sky"), []byte(child), 0o644); err != nil {
		t.Fatal(err)
	}
	parentPath := filepath.Join(workflowDir, "parent.sky")
	if err := os.WriteFile(parentPath, []byte(parent), 0o644); err != nil {
		t.Fatal(err)
	}
	diags, err := LintWithRoots(parentPath, Roots{Repo: root})
	if err != nil {
		t.Fatal(err)
	}
	if !hasCode(diags, string(CodeInvokeParamMissing)) {
		t.Fatalf("want %s for missing required/extra var, got %+v", CodeInvokeParamMissing, diags)
	}
	if !hasCode(diags, string(CodeInvokeParamType)) {
		t.Fatalf("want %s for number mismatch, got %+v", CodeInvokeParamType, diags)
	}
}

func TestManifestIncludesParamsAndLocalLintCodes(t *testing.T) {
	m := Manifest("test")
	if len(m.Params.Types) == 0 || m.Params.NameRegex == "" {
		t.Fatalf("manifest params = %+v", m.Params)
	}
	codes := map[string]bool{}
	for _, c := range m.LintCodes {
		codes[c.Code] = true
	}
	for _, code := range []string{string(CodeParamInvalid), string(CodeTemplateUndeclared), string(CodeInvokeParamMissing), string(CodeInvokeParamType)} {
		if !codes[code] {
			t.Fatalf("manifest missing lint code %s", code)
		}
	}
}
