package workflow

import (
	"strings"
	"testing"
)

// ── foreach node tests: valid shapes ──

func TestParse_ForeachNode_LiteralArray_BashBody(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§fanout§
foreach.items = ["a", "b", "c"]
bash = "echo $ITEM"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	n := wf.Nodes[0]
	if n.NodeType() != "foreach" {
		t.Errorf("node type = %q, want foreach", n.NodeType())
	}
	items, ok := n.Foreach.Items.([]any)
	if !ok || len(items) != 3 {
		t.Fatalf("items = %#v, want 3-element array", n.Foreach.Items)
	}
	if n.Bash != "echo $ITEM" {
		t.Errorf("bash body = %q, want 'echo $ITEM'", n.Bash)
	}
}

func TestParse_ForeachNode_StringItems_PromptBody_MaxConcurrency(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§scan§
bash = "echo hi"
§§

§work§
depends_on = ["scan"]
foreach.items = "$scan.output"
foreach.max_concurrency = 3
§§

∆work∆
process item
∆∆
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	n := wf.Nodes[1]
	if n.NodeType() != "foreach" {
		t.Errorf("node type = %q, want foreach", n.NodeType())
	}
	if n.Foreach.Items != "$scan.output" {
		t.Errorf("items = %#v, want '$scan.output'", n.Foreach.Items)
	}
	if n.Foreach.MaxConcurrency != 3 {
		t.Errorf("max_concurrency = %d, want 3", n.Foreach.MaxConcurrency)
	}
}

func TestParse_ForeachNode_TemplateVarItems(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§fanout§
foreach.items = "{{targets}}"
command = "do-thing"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	n := wf.Nodes[0]
	if n.Foreach.Items != "{{targets}}" {
		t.Errorf("items = %#v, want '{{targets}}'", n.Foreach.Items)
	}
	if n.Command != "do-thing" {
		t.Errorf("command = %q, want 'do-thing'", n.Command)
	}
}

func TestParse_ForeachNode_DefaultMaxConcurrency(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§fanout§
foreach.items = ["a"]
bash = "echo hi"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := wf.Nodes[0].Foreach.MaxConcurrency; got != 0 {
		t.Errorf("max_concurrency = %d, want 0 (runner defaults to 1)", got)
	}
}

// ── foreach node tests: SKY-WF-104 (items invalid) ──

func TestParse_ForeachNode_MissingItems(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§fanout§
foreach = {}
bash = "echo hi"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for foreach with no items")
	}
	if !strings.Contains(err.Error(), "foreach.items is required") {
		t.Errorf("error = %q, want foreach.items required message", err.Error())
	}
}

func TestParse_ForeachNode_EmptyArray(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§fanout§
foreach.items = []
bash = "echo hi"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for foreach with empty items array")
	}
	if !strings.Contains(err.Error(), "foreach.items array must not be empty") {
		t.Errorf("error = %q, want empty array message", err.Error())
	}
}

func TestParse_ForeachNode_NonStringEntry(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§fanout§
foreach.items = ["a", 1]
bash = "echo hi"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for foreach items with non-string entry")
	}
	if !strings.Contains(err.Error(), "foreach.items[1] must be a string") {
		t.Errorf("error = %q, want non-string entry message", err.Error())
	}
}

func TestParse_ForeachNode_EmptyStringItems(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§fanout§
foreach.items = "   "
bash = "echo hi"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for foreach with whitespace-only items string")
	}
	if !strings.Contains(err.Error(), "foreach.items must not be empty") {
		t.Errorf("error = %q, want empty string message", err.Error())
	}
}

func TestParse_ForeachNode_WrongType(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§fanout§
foreach.items = 5
bash = "echo hi"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for foreach.items of wrong type")
	}
	if !strings.Contains(err.Error(), "foreach.items must be an array of strings or a string reference") {
		t.Errorf("error = %q, want wrong-type message", err.Error())
	}
}

func TestParse_ForeachNode_TooManyLiteralItems(t *testing.T) {
	items := make([]string, 101)
	for i := range items {
		items[i] = `"x"`
	}
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§fanout§
foreach.items = [` + strings.Join(items, ", ") + `]
bash = "echo hi"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for foreach.items exceeding the literal cap")
	}
	if !strings.Contains(err.Error(), "exceeds cap of 100") {
		t.Errorf("error = %q, want cap-exceeded message", err.Error())
	}
}

// ── foreach node tests: SKY-WF-105 (body invalid) ──

func TestParse_ForeachNode_HTTPBody_Rejected(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§fanout§
foreach.items = ["a"]
http.url = "https://x.com"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for foreach body = http")
	}
	if !strings.Contains(err.Error(), "foreach body cannot be http, eval, wait, cancel, approval, invoke, acquire_lock, release_lock, spawn, council, or review") {
		t.Errorf("error = %q, want foreach/http message", err.Error())
	}
}

func TestParse_ForeachNode_LoopBody_Rejected(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§fanout§
foreach.items = ["a"]
loop.until.bash = "true"
bash = "echo hi"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for foreach combined with loop")
	}
	if !strings.Contains(err.Error(), "foreach body cannot be loop") {
		t.Errorf("error = %q, want foreach/loop message", err.Error())
	}
}

func TestParse_ForeachNode_WaitBody_Rejected(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§fanout§
foreach.items = ["a"]
wait.prompt = "approve?"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for foreach body = wait")
	}
	if !strings.Contains(err.Error(), "foreach body cannot be http, eval, wait, cancel, approval, invoke, acquire_lock, release_lock, spawn, council, or review") {
		t.Errorf("error = %q, want foreach/wait message", err.Error())
	}
}

func TestParse_ForeachNode_NoBody_Rejected(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§fanout§
foreach.items = ["a"]
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for foreach with no body")
	}
	if !strings.Contains(err.Error(), "foreach requires a body") {
		t.Errorf("error = %q, want foreach body-required message", err.Error())
	}
}

// ── foreach node tests: SKY-WF-106 (max_concurrency invalid) ──

func TestParse_ForeachNode_ConcurrencyNegative(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§fanout§
foreach.items = ["a"]
foreach.max_concurrency = -1
bash = "echo hi"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for negative max_concurrency")
	}
	if !strings.Contains(err.Error(), "foreach.max_concurrency") {
		t.Errorf("error = %q, want max_concurrency message", err.Error())
	}
}

func TestParse_ForeachNode_ConcurrencyTooHigh(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§fanout§
foreach.items = ["a"]
foreach.max_concurrency = 33
bash = "echo hi"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for max_concurrency exceeding cap")
	}
	if !strings.Contains(err.Error(), "foreach.max_concurrency") {
		t.Errorf("error = %q, want max_concurrency message", err.Error())
	}
}

// ── foreach node tests: SKY-WF-107 (chain_from invalid) ──

func TestParse_ForeachNode_ChainFromOnForeach_Rejected(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§scan§
bash = "echo hi"
§§

§fanout§
depends_on = ["scan"]
chain_from = "scan"
foreach.items = ["a"]
bash = "echo $ITEM"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for chain_from set on a foreach node")
	}
	if !strings.Contains(err.Error(), "chain_from may not be set on a foreach node") {
		t.Errorf("error = %q, want foreach chain_from message", err.Error())
	}
}

func TestParse_ForeachNode_ChainFromTargetsForeach_Rejected(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§fanout§
foreach.items = ["a"]
bash = "echo $ITEM"
§§

§summarize§
depends_on = ["fanout"]
chain_from = "fanout"
command = "summarize"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for chain_from targeting a foreach node")
	}
	if !strings.Contains(err.Error(), "targets a foreach node") {
		t.Errorf("error = %q, want foreach chain_from-target message", err.Error())
	}
}

// ── lint surface ──

func TestLint_ForeachInvalid_YieldsDiagnostic(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§fanout§
foreach = {}
bash = "echo hi"
§§
`
	diags := LintBytes("t.sky", []byte(input))
	found := false
	for _, d := range diags {
		if d.Code == "SKY-WF-104" {
			found = true
		}
	}
	if !found {
		t.Errorf("want code SKY-WF-104 in diagnostics, got %+v", diags)
	}
}
