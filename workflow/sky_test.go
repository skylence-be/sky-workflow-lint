package workflow

import (
	"fmt"
	"strings"
	"testing"
)

const validWorkflow = `⊕meta⊕
name = "test"
description = "t"
trigger.github.events = ["issues.labeled"]
trigger.github.label = "ready"
⊕⊕

§plan§
command = "plan"
model = "opus"
§§

§implement§
depends_on = ["plan"]
isolation = "worktree"
command = "implement"
§§

∆implement∆
Execute the plan. Run tests. Open a PR when green.
∆∆
`

func TestParse_Valid(t *testing.T) {
	wf, err := Parse(strings.NewReader(validWorkflow))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Name != "test" {
		t.Errorf("name = %q, want %q", wf.Name, "test")
	}
	if wf.Trigger.GitHub == nil || wf.Trigger.GitHub.Label != "ready" {
		t.Errorf("trigger.github.label = %+v, want label=ready", wf.Trigger.GitHub)
	}
	if !wf.IsDAG() {
		t.Fatal("expected DAG workflow")
	}
	if len(wf.Nodes) != 2 {
		t.Fatalf("nodes = %d, want 2", len(wf.Nodes))
	}
	if wf.Nodes[0].ID != "plan" {
		t.Errorf("nodes[0].id = %q, want plan", wf.Nodes[0].ID)
	}
	if wf.Nodes[0].Model != "opus" {
		t.Errorf("nodes[0].model = %q, want opus", wf.Nodes[0].Model)
	}
	if wf.Nodes[1].Isolation != "worktree" {
		t.Errorf("nodes[1].isolation = %q, want worktree", wf.Nodes[1].Isolation)
	}
	if !strings.HasPrefix(wf.Nodes[1].Prompt, "Execute the plan.") {
		t.Errorf("nodes[1].prompt = %q, want to begin with 'Execute the plan.'", wf.Nodes[1].Prompt)
	}
}

func TestParse_RawPopulated(t *testing.T) {
	wf, err := Parse(strings.NewReader(validWorkflow))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Raw != validWorkflow {
		t.Errorf("Raw = %q, want verbatim input", wf.Raw)
	}
}

func TestParse_CommandOnly(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§a§
command = "do-something"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Nodes[0].Command != "do-something" {
		t.Errorf("nodes[0].command = %q, want do-something", wf.Nodes[0].Command)
	}
	if wf.Nodes[0].NodeType() != "command" {
		t.Errorf("node type = %q, want command", wf.Nodes[0].NodeType())
	}
}

func TestParse_BashNode(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§run§
bash = "make test"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Nodes[0].Bash != "make test" {
		t.Errorf("bash = %q, want 'make test'", wf.Nodes[0].Bash)
	}
	if wf.Nodes[0].NodeType() != "bash" {
		t.Errorf("node type = %q, want bash", wf.Nodes[0].NodeType())
	}
}

func TestParse_EmptySectionBody(t *testing.T) {
	// A §-only section with no body and no command/bash/prompt should fail validation.
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§plan§
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for node with no action")
	}
	if !strings.Contains(err.Error(), "prompt, command, bash, bash_file, http, eval, wait, cancel, script, approval, invoke, acquire_lock, release_lock, spawn, council, or review is required") {
		t.Errorf("error = %q, want missing-action message", err.Error())
	}
}

func TestParse_MissingMeta(t *testing.T) {
	input := `§plan§
command = "x"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for missing meta")
	}
	if !strings.Contains(err.Error(), "missing ⊕meta⊕") {
		t.Errorf("error = %q, want missing-meta message", err.Error())
	}
}

func TestParse_MultipleMeta(t *testing.T) {
	input := `⊕meta⊕
name = "a"
trigger.github.events = ["x"]
⊕⊕

⊕meta⊕
name = "b"
trigger.github.events = ["x"]
⊕⊕

§p§
command = "x"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for multiple meta")
	}
	if !strings.Contains(err.Error(), "multiple ⊕meta⊕") {
		t.Errorf("error = %q, want multiple-meta message", err.Error())
	}
}

func TestParse_OrphanPrompt(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§plan§
command = "x"
§§

∆ghost∆
orphan prompt
∆∆
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for orphan ∆")
	}
	if !strings.Contains(err.Error(), "no matching §ghost§") {
		t.Errorf("error = %q, want orphan-∆ message", err.Error())
	}
}

func TestParse_DuplicateStep(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§plan§
command = "x"
§§

§plan§
command = "y"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for duplicate §")
	}
	if !strings.Contains(err.Error(), "duplicate §plan§") {
		t.Errorf("error = %q, want duplicate-§ message", err.Error())
	}
}

func TestParse_DuplicatePrompt(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§plan§
command = "x"
§§

∆plan∆
first
∆∆

∆plan∆
second
∆∆
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for duplicate ∆")
	}
	if !strings.Contains(err.Error(), "duplicate ∆plan∆") {
		t.Errorf("error = %q, want duplicate-∆ message", err.Error())
	}
}

func TestParse_UnclosedSection(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§plan§
command = "x"
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for unclosed §")
	}
	if !strings.Contains(err.Error(), "unclosed section") {
		t.Errorf("error = %q, want unclosed-section message", err.Error())
	}
}

func TestParse_NestedOpener(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
§nested§
⊕⊕
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for nested opener")
	}
	if !strings.Contains(err.Error(), "nested opener") {
		t.Errorf("error = %q, want nested-opener message", err.Error())
	}
}

func TestParse_EmptyName(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§§
body
§§
`
	// §§ on its own line is a close marker without an open — should be "unexpected close".
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for bare §§ with no open")
	}
	if !strings.Contains(err.Error(), "unexpected §§") {
		t.Errorf("error = %q, want unexpected-close message", err.Error())
	}
}

func TestParse_UnknownMetaName(t *testing.T) {
	input := `⊕config⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§plan§
command = "x"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for non-meta ⊕ section")
	}
	if !strings.Contains(err.Error(), "only ⊕meta⊕") {
		t.Errorf("error = %q, want non-meta message", err.Error())
	}
}

func TestParse_ContentOutsideSection(t *testing.T) {
	input := `random text outside any section
⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§p§
command = "x"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for content outside section")
	}
	if !strings.Contains(err.Error(), "unexpected content outside section") {
		t.Errorf("error = %q, want content-outside message", err.Error())
	}
}

func TestParse_MalformedMetaSyntax(t *testing.T) {
	input := `⊕meta⊕
{not valid assignment}
⊕⊕

§p§
command = "x"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for invalid meta syntax")
	}
	if !strings.Contains(err.Error(), "⊕meta⊕") {
		t.Errorf("error = %q, want ⊕meta⊕ message", err.Error())
	}
}

func TestParse_MalformedNodeSyntax(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§p§
{not valid assignment}
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for invalid node syntax")
	}
	if !strings.Contains(err.Error(), "§p§") {
		t.Errorf("error = %q, want §p§ message", err.Error())
	}
}

func TestParse_MissingName(t *testing.T) {
	input := `⊕meta⊕
trigger.github.events = ["a"]
⊕⊕

§p§
command = "x"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("error = %q, want name-required message", err.Error())
	}
}

func TestParse_NoNodes(t *testing.T) {
	input := `⊕meta⊕
name = "empty"
trigger.github.events = ["a"]
⊕⊕
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for empty workflow")
	}
	if !strings.Contains(err.Error(), "at least one step or node") {
		t.Errorf("error = %q, want empty-workflow message", err.Error())
	}
}

func TestParse_DuplicateNodeID(t *testing.T) {
	// Two § sections with the same name hit the sky-format duplicate check first.
	// This test instead exercises the validate-path duplicate check by passing
	// a meta with a steps array. But since .sky format only populates Nodes,
	// we rely on § duplicate detection (already covered by TestParse_DuplicateStep).
	t.Skip("duplicate node IDs in .sky are caught by TestParse_DuplicateStep")
}

func TestParse_UnknownDependency(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§p§
command = "x"
depends_on = ["missing"]
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for unknown dependency")
	}
	if !strings.Contains(err.Error(), "unknown node") {
		t.Errorf("error = %q, want unknown-dep message", err.Error())
	}
}

func TestParse_SelfDependency(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§p§
command = "x"
depends_on = ["p"]
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for self-dependency")
	}
	if !strings.Contains(err.Error(), "depends on itself") {
		t.Errorf("error = %q, want self-dep message", err.Error())
	}
}

func TestParse_CycleDetection(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§a§
command = "x"
depends_on = ["b"]
§§

§b§
command = "x"
depends_on = ["a"]
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for cycle")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("error = %q, want cycle message", err.Error())
	}
}

func TestParse_InvalidTriggerRule(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§a§
command = "x"
§§

§b§
command = "x"
depends_on = ["a"]
trigger_rule = "one_succes"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for invalid trigger_rule")
	}
	if !strings.Contains(err.Error(), "invalid trigger_rule") {
		t.Errorf("error = %q, want invalid-trigger-rule message", err.Error())
	}
}

func TestParse_MissingAction(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§p§
model = "opus"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for node with no action")
	}
	if !strings.Contains(err.Error(), "prompt, command, bash, bash_file, http, eval, wait, cancel, script, approval, invoke, acquire_lock, release_lock, spawn, council, or review is required") {
		t.Errorf("error = %q, want missing-action message", err.Error())
	}
}

func TestParse_PromptBodyTrimmed(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§p§
§§

∆p∆

  hello world

∆∆
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Leading/trailing whitespace around the prompt body is trimmed.
	if wf.Nodes[0].Prompt != "hello world" {
		t.Errorf("prompt = %q, want %q", wf.Nodes[0].Prompt, "hello world")
	}
}

func TestParse_MultilinePromptPreserved(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§p§
§§

∆p∆
line one
line two

line four
∆∆
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "line one\nline two\n\nline four"
	if wf.Nodes[0].Prompt != want {
		t.Errorf("prompt = %q, want %q", wf.Nodes[0].Prompt, want)
	}
}

func TestParse_DelimiterInSectionName(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§bad§name§
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for section name containing delimiter")
	}
}

// ── Stress tests ──

func TestParse_StressLargeName(t *testing.T) {
	// 1000-char workflow name
	largeName := strings.Repeat("a", 1000)
	input := `⊕meta⊕
name = "` + largeName + `"
trigger.github.events = ["a"]
⊕⊕

§p§
command = "x"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Name != largeName {
		t.Errorf("name length = %d, want %d", len(wf.Name), len(largeName))
	}
}

func TestParse_StressLargeNodeID(t *testing.T) {
	// 500-char node ID
	largeID := strings.Repeat("x", 500)
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§` + largeID + `§
command = "do-something"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Nodes[0].ID != largeID {
		t.Errorf("node ID length = %d, want %d", len(wf.Nodes[0].ID), len(largeID))
	}
}

func TestParse_StressLargePrompt(t *testing.T) {
	// 100KB prompt body
	largePrompt := strings.Repeat("Lorem ipsum dolor sit amet, consectetur adipiscing elit. ", 1000)
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§p§
§§

∆p∆
` + largePrompt + `
∆∆
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(wf.Nodes[0].Prompt, "Lorem") {
		t.Errorf("prompt not preserved: %s...", wf.Nodes[0].Prompt[:50])
	}
}

func TestParse_StressManyNodes(t *testing.T) {
	// 100 nodes
	input := `⊕meta⊕
name = "many"
trigger.github.events = ["a"]
⊕⊕
`
	for i := 0; i < 100; i++ {
		input += fmt.Sprintf("\n§node%d§\ncommand = \"do%d\"\n§§\n", i, i)
	}

	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wf.Nodes) != 100 {
		t.Errorf("node count = %d, want 100", len(wf.Nodes))
	}
	if wf.Nodes[99].ID != "node99" {
		t.Errorf("last node ID = %q, want node99", wf.Nodes[99].ID)
	}
}

// ── Edge cases ──

func TestParse_BothCommandAndBash(t *testing.T) {
	// Node with both command and bash is ambiguous — validation must reject it.
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§p§
command = "plan"
bash = "make test"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for node with both command and bash")
	}
	if !strings.Contains(err.Error(), "only one of") {
		t.Errorf("error = %q, want 'only one of' message", err.Error())
	}
}

func TestParse_EmptyDependsOn(t *testing.T) {
	// Empty depends_on array should be allowed
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§p§
command = "plan"
depends_on = []
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wf.Nodes[0].DependsOn) != 0 {
		t.Errorf("depends_on = %v, want []", wf.Nodes[0].DependsOn)
	}
}

func TestParse_WhitespaceInNodeID(t *testing.T) {
	// Node ID with spaces
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§my node§
command = "x"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Nodes[0].ID != "my node" {
		t.Errorf("node ID = %q, want 'my node'", wf.Nodes[0].ID)
	}
}

func TestParse_NodeIDWithNumbers(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§step_123_final§
command = "x"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Nodes[0].ID != "step_123_final" {
		t.Errorf("node ID = %q, want step_123_final", wf.Nodes[0].ID)
	}
}

func TestParse_PromptBodyWithCloserMarker(t *testing.T) {
	// Prompt containing ∆∆ as a substring on a line (not as closer)
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§p§
§§

∆p∆
This is a prompt that includes the marker ∆∆ inside
but the marker must be alone on a line to close
∆∆
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(wf.Nodes[0].Prompt, "∆∆ inside") {
		t.Errorf("prompt should contain '∆∆ inside': %q", wf.Nodes[0].Prompt)
	}
}

func TestParse_LongDependencyChain(t *testing.T) {
	// A → B → C → ... → Z (26 nodes in sequence)
	input := `⊕meta⊕
name = "chain"
trigger.github.events = ["a"]
⊕⊕
`
	nodes := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z"}
	var prev string
	for i, node := range nodes {
		if i == 0 {
			input += fmt.Sprintf("\n§%s§\ncommand = %q\n§§\n", node, node)
		} else {
			input += fmt.Sprintf("\n§%s§\ncommand = %q\ndepends_on = [%q]\n§§\n", node, node, prev)
		}
		prev = node
	}

	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wf.Nodes) != 26 {
		t.Errorf("node count = %d, want 26", len(wf.Nodes))
	}
	if wf.Nodes[25].ID != "z" {
		t.Errorf("last node = %q, want z", wf.Nodes[25].ID)
	}
	if len(wf.Nodes[25].DependsOn) != 1 || wf.Nodes[25].DependsOn[0] != "y" {
		t.Errorf("z depends_on = %v, want [y]", wf.Nodes[25].DependsOn)
	}
}

func TestParse_ComplexDependencyDAG(t *testing.T) {
	// Multiple dependencies and complex DAG:
	//     plan
	//    /    \
	//   impl   review
	//    \    /
	//     test
	//      |
	//   create-pr
	input := `⊕meta⊕
name = "complex"
trigger.github.events = ["a"]
⊕⊕

§plan§
command = "analyze"
§§

§impl§
command = "implement"
depends_on = ["plan"]
§§

§review§
command = "review"
depends_on = ["plan"]
§§

§test§
command = "test"
depends_on = ["impl", "review"]
§§

§create-pr§
command = "pr"
depends_on = ["test"]
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wf.Nodes) != 5 {
		t.Errorf("node count = %d, want 5", len(wf.Nodes))
	}
	// Find test node, should have 2 dependencies
	var testNode *Node
	for i := range wf.Nodes {
		if wf.Nodes[i].ID == "test" {
			testNode = &wf.Nodes[i]
			break
		}
	}
	if testNode == nil {
		t.Fatal("test node not found")
		return // unreachable; staticcheck SA5011
	}
	if len(testNode.DependsOn) != 2 {
		t.Errorf("test depends_on count = %d, want 2", len(testNode.DependsOn))
	}
}

func TestParse_PromptWithoutMatchingSection(t *testing.T) {
	// ∆prompt∆ with no matching §prompt§
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§plan§
command = "plan"
§§

∆nonexistent∆
This prompt has no matching section
∆∆
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for orphan prompt")
	}
	if !strings.Contains(err.Error(), "no matching §nonexistent§") {
		t.Errorf("error = %q, want orphan message", err.Error())
	}
}

func TestParse_WhitespaceVariations(t *testing.T) {
	// Leading/trailing whitespace on delimiter lines should be trimmed
	input := `  ⊕meta⊕
name = "t"
trigger.github.events = ["a"]
  ⊕⊕

  §plan§
command = "plan"
  §§

  ∆plan∆
do something
  ∆∆
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Name != "t" {
		t.Errorf("name = %q, want t", wf.Name)
	}
	if wf.Nodes[0].ID != "plan" {
		t.Errorf("node ID = %q, want plan", wf.Nodes[0].ID)
	}
	if wf.Nodes[0].Prompt != "do something" {
		t.Errorf("prompt = %q, want 'do something'", wf.Nodes[0].Prompt)
	}
}

func TestParse_SpecialCharsInNodeID(t *testing.T) {
	// Node IDs can contain hyphens, underscores, numbers
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§step-1_a§
command = "x"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Nodes[0].ID != "step-1_a" {
		t.Errorf("node ID = %q, want step-1_a", wf.Nodes[0].ID)
	}
}

func TestParse_NoWhitespaceAfterDelimiter(t *testing.T) {
	// Delimiters require newline after, but section content can be on next line
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§p§
command = "plan"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Name != "t" {
		t.Errorf("name = %q, want t", wf.Name)
	}
	if wf.Nodes[0].ID != "p" {
		t.Errorf("node ID = %q, want p", wf.Nodes[0].ID)
	}
}

// ── assign syntax tests ──

func TestParse_AssignComment(t *testing.T) {
	// Lines starting with # are comments and ignored
	input := `⊕meta⊕
# this is a workflow comment
name = "t"
# another comment
trigger.github.events = ["a"]
⊕⊕

§p§
# node comment
command = "x"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Name != "t" {
		t.Errorf("name = %q, want t", wf.Name)
	}
	if wf.Nodes[0].Command != "x" {
		t.Errorf("command = %q, want x", wf.Nodes[0].Command)
	}
}

func TestParse_AssignNumberValue(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§p§
command = "x"
max_budget_usd = 5.5
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Nodes[0].MaxBudget != 5.5 {
		t.Errorf("max_budget = %v, want 5.5", wf.Nodes[0].MaxBudget)
	}
}

func TestParse_AssignBoolValue(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§p§
loop.until.bash = "make test"
bash = "make fix"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Nodes[0].Loop.Until.Bash != "make test" {
		t.Errorf("loop.until.bash = %q, want %q", wf.Nodes[0].Loop.Until.Bash, "make test")
	}
}

func TestParse_AssignMissingEquals(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§p§
command "missing equals"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for missing =")
	}
	if !strings.Contains(err.Error(), "key = value") {
		t.Errorf("error = %q, want 'key = value' message", err.Error())
	}
}

func TestParse_AssignEmptyValue(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§p§
command =
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for empty value")
	}
	if !strings.Contains(err.Error(), "empty value") {
		t.Errorf("error = %q, want 'empty value' message", err.Error())
	}
}

// ── http node tests ──

func TestParse_HTTPNode_Basic(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§notify§
http.url = "https://hooks.slack.com/x"
http.method = "POST"
http.body = "{{plan}}"
http.expect_status = 200
http.timeout_s = 10
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	n := wf.Nodes[0]
	if n.NodeType() != "http" {
		t.Errorf("node type = %q, want http", n.NodeType())
	}
	if n.HTTP.URL != "https://hooks.slack.com/x" {
		t.Errorf("url = %q", n.HTTP.URL)
	}
	if n.HTTP.Method != "POST" {
		t.Errorf("method = %q, want POST", n.HTTP.Method)
	}
	if n.HTTP.Body != "{{plan}}" {
		t.Errorf("body = %q", n.HTTP.Body)
	}
	if n.HTTP.ExpectStatus != 200 {
		t.Errorf("expect_status = %d, want 200", n.HTTP.ExpectStatus)
	}
	if n.HTTP.TimeoutS != 10 {
		t.Errorf("timeout_s = %d, want 10", n.HTTP.TimeoutS)
	}
}

func TestParse_HTTPNode_MissingURL(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§notify§
http.method = "POST"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for missing http.url")
	}
	if !strings.Contains(err.Error(), "http.url is required") {
		t.Errorf("error = %q, want http.url message", err.Error())
	}
}

func TestParse_HTTPNode_WithHeaders(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§call§
http.url = "https://api.example.com/v1"
http.headers.Authorization = "Bearer token"
http.headers.Content-Type = "application/json"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Nodes[0].HTTP.Headers["Authorization"] != "Bearer token" {
		t.Errorf("Authorization header = %q", wf.Nodes[0].HTTP.Headers["Authorization"])
	}
}

func TestParse_HTTPNode_AmbiguousWithBash(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§n§
http.url = "https://x.com"
bash = "echo hi"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for http + bash")
	}
	if !strings.Contains(err.Error(), "only one of") {
		t.Errorf("error = %q, want 'only one of' message", err.Error())
	}
}

// ── eval node tests ──

func TestParse_EvalNode_Contains(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§analyze§
command = "analyze"
§§

§check§
eval.source = "$analyze.output"
eval.contains = "Implementation Plan"
depends_on = ["analyze"]
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	n := wf.Nodes[1]
	if n.NodeType() != "eval" {
		t.Errorf("node type = %q, want eval", n.NodeType())
	}
	if n.Eval.Source != "$analyze.output" {
		t.Errorf("source = %q", n.Eval.Source)
	}
	if n.Eval.Contains != "Implementation Plan" {
		t.Errorf("contains = %q", n.Eval.Contains)
	}
}

func TestParse_EvalNode_Matches(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§check§
eval.source = "$prior.output"
eval.matches = "^PASS"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Nodes[0].Eval.Matches != "^PASS" {
		t.Errorf("matches = %q", wf.Nodes[0].Eval.Matches)
	}
}

func TestParse_EvalNode_Equals(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§check§
eval.source = "$prior.output"
eval.equals = "ok"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Nodes[0].Eval.Equals != "ok" {
		t.Errorf("equals = %q", wf.Nodes[0].Eval.Equals)
	}
}

func TestParse_EvalNode_MissingSource(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§check§
eval.contains = "foo"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for missing eval.source")
	}
	if !strings.Contains(err.Error(), "eval.source is required") {
		t.Errorf("error = %q, want eval.source message", err.Error())
	}
}

func TestParse_EvalNode_NoAssertion(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§check§
eval.source = "$x.output"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for eval with no assertion")
	}
	if !strings.Contains(err.Error(), "exactly one of contains, matches, or equals") {
		t.Errorf("error = %q, want assertion message", err.Error())
	}
}

func TestParse_EvalNode_MultipleAssertions(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§check§
eval.source = "$x.output"
eval.contains = "foo"
eval.equals = "foo"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for eval with multiple assertions")
	}
	if !strings.Contains(err.Error(), "exactly one of contains, matches, or equals") {
		t.Errorf("error = %q, want assertion message", err.Error())
	}
}

func TestParse_EvalNode_AmbiguousWithCommand(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§n§
eval.source = "$x.output"
eval.contains = "y"
command = "plan"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for eval + command")
	}
	if !strings.Contains(err.Error(), "only one of") {
		t.Errorf("error = %q, want 'only one of' message", err.Error())
	}
}

// ── loop node tests ──

func TestParse_LoopNode_BashBody_BashUntil(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§fix§
loop.until.bash = "make test"
loop.max = 5
bash = "make fix"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	n := wf.Nodes[0]
	if n.NodeType() != "loop" {
		t.Errorf("node type = %q, want loop", n.NodeType())
	}
	if n.Bash != "make fix" {
		t.Errorf("bash body = %q, want 'make fix'", n.Bash)
	}
	if n.Loop.Until.Bash != "make test" {
		t.Errorf("until.bash = %q, want 'make test'", n.Loop.Until.Bash)
	}
	if n.Loop.Max != 5 {
		t.Errorf("max = %d, want 5", n.Loop.Max)
	}
}

func TestParse_LoopNode_CommandBody_EvalUntil(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§fix§
loop.until.eval.source = "$_loop.output"
loop.until.eval.contains = "PASS"
loop.max = 3
command = "fix-failing-test"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	n := wf.Nodes[0]
	if n.NodeType() != "loop" {
		t.Errorf("node type = %q, want loop", n.NodeType())
	}
	if n.Command != "fix-failing-test" {
		t.Errorf("command = %q, want fix-failing-test", n.Command)
	}
	if n.Loop.Until.Eval == nil {
		t.Fatal("until.eval is nil")
	}
	if n.Loop.Until.Eval.Contains != "PASS" {
		t.Errorf("until.eval.contains = %q, want PASS", n.Loop.Until.Eval.Contains)
	}
}

func TestParse_LoopNode_DefaultMax(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§fix§
loop.until.bash = "make test"
bash = "make fix"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Nodes[0].Loop.Max != 0 {
		t.Errorf("max = %d, want 0 (runner defaults to 10)", wf.Nodes[0].Loop.Max)
	}
}

func TestParse_LoopNode_MissingBody(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§fix§
loop.until.bash = "make test"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for loop with no body")
	}
	if !strings.Contains(err.Error(), "loop requires a body") {
		t.Errorf("error = %q, want loop body message", err.Error())
	}
}

func TestParse_LoopNode_MissingUntil(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§fix§
loop = {}
bash = "make fix"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for loop with no until condition")
	}
	if !strings.Contains(err.Error(), "loop.until requires exactly one") {
		t.Errorf("error = %q, want until message", err.Error())
	}
}

func TestParse_LoopNode_HTTPBody_Rejected(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§fix§
loop.until.bash = "make test"
http.url = "https://x.com"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for loop body = http")
	}
	if !strings.Contains(err.Error(), "loop body cannot be http, eval, wait, cancel, script, approval, acquire_lock, release_lock, spawn, council, or review") {
		t.Errorf("error = %q, want http/eval/wait/cancel/approval message", err.Error())
	}
}

func TestParse_LoopNode_AmbiguousBody(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§fix§
loop.until.bash = "make test"
bash = "make fix"
command = "also-fix"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for loop with ambiguous body")
	}
	if !strings.Contains(err.Error(), "loop body") {
		t.Errorf("error = %q, want loop body ambiguous message", err.Error())
	}
}

func TestParse_LoopNode_BashFileBody_BashUntil(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§verify§
loop.until.bash = "make test"
loop.max = 5
bash_file = "./verify.sh"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	n := wf.Nodes[0]
	if n.NodeType() != "loop" {
		t.Errorf("node type = %q, want loop", n.NodeType())
	}
	if n.BashFile != "./verify.sh" {
		t.Errorf("bash_file body = %q, want './verify.sh'", n.BashFile)
	}
	if n.Loop.Until.Bash != "make test" {
		t.Errorf("until.bash = %q, want 'make test'", n.Loop.Until.Bash)
	}
}

func TestParse_LoopNode_BashFileWithPrompt_Rejected(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§verify§
loop.until.bash = "make test"
bash_file = "./verify.sh"
prompt = "also do this"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for loop mixing bash_file with prompt")
	}
	if !strings.Contains(err.Error(), "loop body cannot mix bash with command/prompt") {
		t.Errorf("error = %q, want loop body mix message", err.Error())
	}
}

func TestParse_LoopNode_IdleTimeout_BashBody(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§fix§
loop.until.bash = "make test"
loop.idle_timeout_ms = 60000
bash = "make fix"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := wf.Nodes[0].Loop.IdleTimeoutMs; got != 60000 {
		t.Errorf("idle_timeout_ms = %d, want 60000", got)
	}
}

func TestParse_LoopNode_IdleTimeout_RejectedOnPromptBody(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§fix§
loop.until.bash = "make test"
loop.idle_timeout_ms = 60000
prompt = "fix the failing tests"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for idle_timeout_ms on prompt body")
	}
	if !strings.Contains(err.Error(), "idle_timeout_ms only applies to bash bodies") {
		t.Errorf("error = %q, want bash-only message", err.Error())
	}
}

func TestParse_LoopNode_IdleTimeout_RejectedOnCommandBody(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§fix§
loop.until.eval.source = "$_loop.output"
loop.until.eval.contains = "PASS"
loop.idle_timeout_ms = 5000
command = "fix-failing-test"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for idle_timeout_ms on command body")
	}
	if !strings.Contains(err.Error(), "idle_timeout_ms only applies to bash bodies") {
		t.Errorf("error = %q, want bash-only message", err.Error())
	}
}

// ── wait node tests ──

func TestParse_WaitNode_Basic(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§approve§
wait.prompt = "Approve deployment?"
wait.channel = "manual"
wait.timeout = "24h"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	n := wf.Nodes[0]
	if n.NodeType() != "wait" {
		t.Errorf("node type = %q, want wait", n.NodeType())
	}
	if n.Wait.Prompt != "Approve deployment?" {
		t.Errorf("prompt = %q", n.Wait.Prompt)
	}
	if n.Wait.Channel != "manual" {
		t.Errorf("channel = %q, want manual", n.Wait.Channel)
	}
	if n.Wait.Timeout != "24h" {
		t.Errorf("timeout = %q, want 24h", n.Wait.Timeout)
	}
}

func TestParse_WaitNode_WebhookChannel(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§gate§
wait.channel = "webhook"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Nodes[0].Wait.Channel != "webhook" {
		t.Errorf("channel = %q, want webhook", wf.Nodes[0].Wait.Channel)
	}
}

func TestParse_WaitNode_NoFields(t *testing.T) {
	// Empty wait config is valid — defaults to manual with no timeout
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§gate§
wait = {}
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Nodes[0].NodeType() != "wait" {
		t.Errorf("node type = %q, want wait", wf.Nodes[0].NodeType())
	}
}

func TestParse_WaitNode_InvalidChannel(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§gate§
wait.channel = "slack"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for invalid channel")
	}
	if !strings.Contains(err.Error(), "wait.channel") {
		t.Errorf("error = %q, want channel message", err.Error())
	}
}

func TestParse_WaitNode_InvalidTimeout(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§gate§
wait.timeout = "forever"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for invalid timeout")
	}
	if !strings.Contains(err.Error(), "wait.timeout") {
		t.Errorf("error = %q, want timeout message", err.Error())
	}
}

func TestParse_WaitNode_AmbiguousWithBash(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§gate§
wait = {}
bash = "echo hi"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for wait + bash")
	}
	if !strings.Contains(err.Error(), "only one of") {
		t.Errorf("error = %q, want 'only one of' message", err.Error())
	}
}

func TestParse_OutputStyle_Terse(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
output_style = "terse"
⊕⊕

§a§
bash = "echo hi"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.OutputStyle != "terse" {
		t.Errorf("output_style = %q, want terse", wf.OutputStyle)
	}
}

func TestParse_OutputStyle_Invalid(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
output_style = "verbose"
⊕⊕

§a§
bash = "echo hi"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for invalid output_style")
	}
	if !strings.Contains(err.Error(), "output_style") {
		t.Errorf("error = %q, want output_style message", err.Error())
	}
}

// ── chain_from tests ──

func TestParse_ChainFrom_Valid(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§draft§
command = "write"
§§

§refine§
command = "refine"
depends_on = ["draft"]
chain_from = "draft"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Nodes[1].ChainFrom != "draft" {
		t.Errorf("chain_from = %q, want draft", wf.Nodes[1].ChainFrom)
	}
}

func TestParse_ChainFrom_NotInDependsOn(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§draft§
command = "write"
§§

§refine§
command = "refine"
chain_from = "draft"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error: chain_from not in depends_on")
	}
	if !strings.Contains(err.Error(), "must be listed in depends_on") {
		t.Errorf("error = %q, want depends_on message", err.Error())
	}
}

func TestParse_ChainFrom_OnBashNode(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§draft§
command = "write"
§§

§lint§
bash = "make lint"
depends_on = ["draft"]
chain_from = "draft"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error: chain_from on bash node")
	}
	if !strings.Contains(err.Error(), "only valid on prompt/command nodes") {
		t.Errorf("error = %q, want prompt/command message", err.Error())
	}
}

func TestParse_ChainFrom_TargetIsBashNode(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§setup§
bash = "make setup"
§§

§refine§
command = "refine"
depends_on = ["setup"]
chain_from = "setup"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error: chain_from target is bash node (no session)")
	}
	if !strings.Contains(err.Error(), "only prompt/command nodes produce sessions") {
		t.Errorf("error = %q, want session message", err.Error())
	}
}

func TestValidateLoopNode_MaxExceedsCap(t *testing.T) {
	wf := &Workflow{
		Name: "test",
		Nodes: []Node{
			{
				ID:   "n1",
				Loop: &LoopConfig{Max: 200, Until: LoopCondition{Bash: "true"}},
				Bash: "echo hi",
			},
		},
	}
	if err := validate(wf); err == nil {
		t.Fatal("expected validation error for loop.max > 100, got nil")
	}
}

func TestValidateLoopNode_MaxAtCap(t *testing.T) {
	wf := &Workflow{
		Name: "test",
		Nodes: []Node{
			{
				ID:   "n1",
				Loop: &LoopConfig{Max: 100, Until: LoopCondition{Bash: "true"}},
				Bash: "echo hi",
			},
		},
	}
	if err := validate(wf); err != nil {
		t.Errorf("unexpected error at max=100: %v", err)
	}
}

func TestParse_ClaudeIsolation(t *testing.T) {
	input := `⊕meta⊕
name = "isolated-wf"
trigger.github.events = ["issues.labeled"]
claude.isolation = "loose"
⊕⊕

§step§
prompt = "do something"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if wf.Claude.Isolation != "loose" {
		t.Errorf("claude.isolation = %q, want %q", wf.Claude.Isolation, "loose")
	}
}

func TestParse_MCPServers(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
mcp_servers = {"myserver": {"command": "npx", "args": ["myserver"]}}
⊕⊕

§step§
prompt = "do something"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(wf.MCPServers) != 1 {
		t.Fatalf("mcp_servers count = %d, want 1", len(wf.MCPServers))
	}
	srv, ok := wf.MCPServers["myserver"].(map[string]any)
	if !ok {
		t.Fatalf("mcp_servers.myserver missing or wrong type: %v", wf.MCPServers["myserver"])
	}
	if srv["command"] != "npx" {
		t.Errorf("mcp_servers.myserver.command = %v, want npx", srv["command"])
	}
}

func TestParse_MCPServers_RejectsMissingTransport(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
mcp_servers = {"bad": {"args": ["oops"]}}
⊕⊕

§step§
prompt = "do something"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "command, url, or type") {
		t.Errorf("error = %v, want message mentioning transport fields", err)
	}
}

func TestParse_MCPServers_RejectsNonObject(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
mcp_servers = {"bad": "not-an-object"}
⊕⊕

§step§
prompt = "do something"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "must be an object") {
		t.Errorf("error = %v, want 'must be an object'", err)
	}
}

func TestParse_ClaudeIsolation_RejectsInherit(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
claude.isolation = "inherit"
⊕⊕

§step§
prompt = "do something"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "SKY-WF-051") {
		t.Errorf("error = %v, want SKY-WF-051", err)
	}
}

func TestParse_ClaudeIsolation_AcceptsStrictLoose(t *testing.T) {
	for _, level := range []string{"strict", "loose"} {
		input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
claude.isolation = "` + level + `"
⊕⊕

§step§
prompt = "do something"
§§
`
		if _, err := Parse(strings.NewReader(input)); err != nil {
			t.Errorf("level %q: unexpected error: %v", level, err)
		}
	}
}

func TestParse_NodeRunner_AbsentDefaultsToClaude(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§n§
prompt = "do something"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Nodes[0].Runner != "" {
		t.Errorf("runner = %q, want empty (absent means claude)", wf.Nodes[0].Runner)
	}
}

func TestParse_NodeRunner_AcceptsSkylence(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§n§
prompt = "do something"
runner = "skylence"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Nodes[0].Runner != "skylence" {
		t.Errorf("runner = %q, want skylence", wf.Nodes[0].Runner)
	}
}

func TestParse_NodeRunner_RejectsUnknownValue(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§n§
prompt = "do something"
runner = "gpt5cli"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "SKY-WF-070") {
		t.Errorf("error = %v, want SKY-WF-070", err)
	}
}

func TestParse_DocBlock_Ignored(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

※※
This workflow takes a labeled issue and implements it end to end.
Classify → plan → implement in worktree → test → open PR.
※※

§step§
prompt = "do something"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Name != "t" {
		t.Errorf("name = %q, want t", wf.Name)
	}
	if len(wf.Nodes) != 1 {
		t.Errorf("nodes = %d, want 1", len(wf.Nodes))
	}
}

func TestParse_DocBlock_MultipleAllowed(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

※※
First doc block.
※※

§step§
prompt = "do something"
§§

※※
Second doc block after a node.
※※
`
	if _, err := Parse(strings.NewReader(input)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParse_DocBlock_Unclosed(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

※※
This block is never closed.
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for unclosed doc block, got nil")
	}
}

func TestParse_DocBlock_WithID(t *testing.T) {
	t.Parallel()
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§classify§
prompt = "do something"
§§

※classify※
This step classifies the PR.
※※
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wf.Docs) != 1 {
		t.Fatalf("Docs = %d, want 1", len(wf.Docs))
	}
	d := wf.Docs[0]
	if d.StepID != "classify" {
		t.Errorf("StepID = %q, want classify", d.StepID)
	}
	if d.Body != "This step classifies the PR." {
		t.Errorf("Body = %q, want %q", d.Body, "This step classifies the PR.")
	}
}

func TestParse_DocBlock_Loose_Captured(t *testing.T) {
	t.Parallel()
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

※※
some note
※※

§step§
prompt = "do something"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wf.Docs) != 1 {
		t.Fatalf("Docs = %d, want 1", len(wf.Docs))
	}
	d := wf.Docs[0]
	if d.StepID != "" {
		t.Errorf("StepID = %q, want empty", d.StepID)
	}
	if d.Body != "some note" {
		t.Errorf("Body = %q, want %q", d.Body, "some note")
	}
}

func TestParse_DocBlock_IDInsidePromptLiteral(t *testing.T) {
	t.Parallel()
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§foo§
prompt = "x"
§§

∆foo∆
※bar※
∆∆
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// ※bar※ inside ∆foo∆ must be literal — no DocBlock emitted
	if len(wf.Docs) != 0 {
		t.Errorf("Docs = %d, want 0 (※bar※ is inside a prompt section)", len(wf.Docs))
	}
	if !strings.Contains(wf.Nodes[0].Prompt, "※bar※") {
		t.Errorf("Prompt = %q, want to contain ※bar※", wf.Nodes[0].Prompt)
	}
}

func TestParse_DocBlock_UnknownID_Warning(t *testing.T) {
	t.Parallel()
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§step§
prompt = "do something"
§§

※bogus※
stale doc
※※
`
	diags := LintBytes("t.sky", []byte(input))
	found := false
	for _, d := range diags {
		if d.Code == "SKY-WF-102" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected SKY-WF-102 warning, got %v", diags)
	}
}

func TestParse_DocBlock_KnownID_Clean(t *testing.T) {
	t.Parallel()
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§classify§
prompt = "classify the input"
§§

※classify※
This step classifies the PR.
※※
`
	diags := LintBytes("t.sky", []byte(input))
	for _, d := range diags {
		if d.Code == "SKY-WF-102" {
			t.Errorf("unexpected SKY-WF-102 for known step: %v", d)
		}
	}
}

func TestParse_DocBlock_ForbiddenCharInID(t *testing.T) {
	t.Parallel()
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§step§
prompt = "do something"
§§

※foo§bar※
should fail
※※
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected parse error for forbidden character in doc block id, got nil")
	}
}

func TestParse_ScriptNode_Valid(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§transform§
script = {"runtime": "bun", "deps": ["zod"], "timeout": 15000}
§§

∆transform∆
import { z } from "zod";
console.log("ok");
∆∆
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	n := wf.Nodes[0]
	if n.Script == nil {
		t.Fatal("expected script config to be set")
	}
	if n.Script.Runtime != "bun" {
		t.Errorf("runtime = %q, want %q", n.Script.Runtime, "bun")
	}
	if len(n.Script.Deps) != 1 || n.Script.Deps[0] != "zod" {
		t.Errorf("deps = %v, want [zod]", n.Script.Deps)
	}
	if n.Script.Timeout != 15000 {
		t.Errorf("timeout = %d, want 15000", n.Script.Timeout)
	}
	if !strings.Contains(n.Prompt, `console.log("ok")`) {
		t.Errorf("prompt = %q, want script body", n.Prompt)
	}
}

func TestParse_ScriptNode_InvalidRuntime(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§transform§
script = {"runtime": "node"}
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for invalid runtime")
	}
	if !strings.Contains(err.Error(), "invalid (valid: bun, uv)") {
		t.Errorf("error = %q, want runtime invalid message", err.Error())
	}
}

func TestParse_ScriptNode_TemplateDep_Rejected(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§transform§
script = {"runtime": "bun", "deps": ["{{package}}"]}
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for template expression in deps")
	}
	if !strings.Contains(err.Error(), "must not contain template expressions") {
		t.Errorf("error = %q, want template expression message", err.Error())
	}
}

func TestParse_CancelNode_Valid(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§abort§
cancel = {"reason": "no eligible tasks"}
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Nodes[0].Cancel == nil {
		t.Fatal("expected cancel config to be set")
	}
	if wf.Nodes[0].Cancel.Reason != "no eligible tasks" {
		t.Errorf("reason = %q, want %q", wf.Nodes[0].Cancel.Reason, "no eligible tasks")
	}
}

func TestParse_CancelNode_WithPrompt_Rejected(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§abort§
cancel = {}
§§

∆abort∆
explain why
∆∆
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for cancel node with prompt")
	}
	if !strings.Contains(err.Error(), "cancel node must not have a prompt") {
		t.Errorf("error = %q, want cancel prompt message", err.Error())
	}
}

func TestParse_CancelNode_WithLoop_Rejected(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§abort§
cancel = {}
loop.until.bash = "true"
bash = "make fix"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for cancel node with loop")
	}
	if !strings.Contains(err.Error(), "loop body cannot be http, eval, wait, cancel, script, approval, acquire_lock, release_lock, spawn, council, or review") {
		t.Errorf("error = %q, want loop body rejection message", err.Error())
	}
}

func TestParse_SandboxNode_Valid(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§fetch§
sandbox.filesystem.allow = ["./src", "./docs"]
§§

∆fetch∆
summarize
∆∆
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	n := wf.Nodes[0]
	if n.Sandbox == nil {
		t.Fatal("expected sandbox config to be set")
	}
	want := []string{"./src", "./docs"}
	if len(n.Sandbox.Filesystem.Allow) != len(want) {
		t.Fatalf("allow = %v, want %v", n.Sandbox.Filesystem.Allow, want)
	}
	for i, p := range want {
		if n.Sandbox.Filesystem.Allow[i] != p {
			t.Errorf("allow[%d] = %q, want %q", i, n.Sandbox.Filesystem.Allow[i], p)
		}
	}
}

func TestParse_SandboxNode_AbsolutePath_Rejected(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§fetch§
sandbox.filesystem.allow = ["/etc/passwd"]
§§

∆fetch∆
summarize
∆∆
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for absolute path in sandbox.filesystem.allow")
	}
	if !strings.Contains(err.Error(), "SKY-WF-054") {
		t.Errorf("error = %q, want SKY-WF-054", err.Error())
	}
}

func TestParse_SandboxNode_TraversalPath_Rejected(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§fetch§
sandbox.filesystem.allow = ["../escape"]
§§

∆fetch∆
summarize
∆∆
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for traversal path in sandbox.filesystem.allow")
	}
	if !strings.Contains(err.Error(), "SKY-WF-054") {
		t.Errorf("error = %q, want SKY-WF-054", err.Error())
	}
}

func TestParse_ContextFresh_Valid(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§step1§
context = "fresh"
§§

∆step1∆
do something
∆∆
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Nodes[0].Context != "fresh" {
		t.Errorf("context = %q, want %q", wf.Nodes[0].Context, "fresh")
	}
}

func TestParse_ContextShared_Valid(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§step1§
context = "shared"
§§

∆step1∆
do something
∆∆
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Nodes[0].Context != "shared" {
		t.Errorf("context = %q, want %q", wf.Nodes[0].Context, "shared")
	}
}

func TestParse_ContextInvalid_Rejected(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§step1§
context = "garbage"
§§

∆step1∆
do something
∆∆
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for invalid context value")
	}
	if !strings.Contains(err.Error(), "context must be") {
		t.Errorf("error = %q, want context validation message", err.Error())
	}
}

// ── emit tests ──

func TestParse_EmitStringForm(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§notify§
bash = "make build"
emit = "build.done"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	n := wf.Nodes[0]
	if n.Emit == nil {
		t.Fatal("expected emit to be set")
	}
	if n.Emit.Name != "build.done" {
		t.Errorf("emit.name = %q, want build.done", n.Emit.Name)
	}
	if len(n.Emit.Payload) != 0 {
		t.Errorf("emit.payload = %v, want empty", n.Emit.Payload)
	}
}

func TestParse_EmitObjectForm(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§notify§
bash = "make build"
emit = {"name": "build.done", "payload": {"pr": "{{pr_number}}", "ref": "{{ref}}"}}
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	n := wf.Nodes[0]
	if n.Emit == nil {
		t.Fatal("expected emit to be set")
	}
	if n.Emit.Name != "build.done" {
		t.Errorf("emit.name = %q, want build.done", n.Emit.Name)
	}
	if n.Emit.Payload["pr"] != "{{pr_number}}" {
		t.Errorf("emit.payload[pr] = %q, want {{pr_number}}", n.Emit.Payload["pr"])
	}
	if n.Emit.Payload["ref"] != "{{ref}}" {
		t.Errorf("emit.payload[ref] = %q, want {{ref}}", n.Emit.Payload["ref"])
	}
}

func TestParse_EmitInvalidName_Rejected(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§notify§
bash = "make build"
emit = "Bad.Name"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for invalid emit name")
	}
	if !strings.Contains(err.Error(), "emit.name") {
		t.Errorf("error = %q, want emit.name message", err.Error())
	}
}

func TestParse_SkyEventTrigger_Valid(t *testing.T) {
	input := `⊕meta⊕
name = "responder"
trigger.sky_event.event = "build.done"
⊕⊕

§handle§
bash = "echo triggered"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Trigger.SkyEvent == nil {
		t.Fatal("expected trigger.sky_event to be set")
	}
	if wf.Trigger.SkyEvent.Event != "build.done" {
		t.Errorf("event = %q, want build.done", wf.Trigger.SkyEvent.Event)
	}
	if wf.Trigger.GitHub != nil {
		t.Error("trigger.github should be nil")
	}
}

func TestParse_SkyEventTrigger_EmptyEvent_Rejected(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.sky_event.event = ""
⊕⊕

§handle§
bash = "echo hi"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for empty trigger.sky_event.event")
	}
	if !strings.Contains(err.Error(), "trigger.sky_event.event is required") {
		t.Errorf("error = %q, want required message", err.Error())
	}
}

func TestParse_SkyEventTrigger_InvalidPattern_Rejected(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.sky_event.event = "1bad-name"
⊕⊕

§handle§
bash = "echo hi"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for invalid trigger.sky_event.event pattern")
	}
	if !strings.Contains(err.Error(), "trigger.sky_event.event") {
		t.Errorf("error = %q, want event pattern message", err.Error())
	}
}

func TestParse_ContextFreshWithChainFrom_Rejected(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§step1§
§§

∆step1∆
do something
∆∆

§step2§
context = "fresh"
chain_from = "step1"
§§

∆step2∆
continue
∆∆
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for context=fresh with chain_from")
	}
	if !strings.Contains(err.Error(), "chain_from") {
		t.Errorf("error = %q, want chain_from incompatibility message", err.Error())
	}
}

func TestParse_PermissionsInteractive_RequiresLooseIsolation(t *testing.T) {
	for _, iso := range []string{"", "strict"} {
		metaLine := ""
		if iso != "" {
			metaLine = `claude.isolation = "` + iso + `"`
		}
		input := `⊕meta⊕
name = "t"
trigger.manual = true
` + metaLine + `
⊕⊕

§check§
permissions = "interactive"
§§

∆check∆
run date
∆∆
`
		_, err := Parse(strings.NewReader(input))
		if err == nil {
			t.Fatalf("isolation=%q: expected SKY-WF-059 error, got nil", iso)
		}
		if !strings.Contains(err.Error(), "SKY-WF-059") {
			t.Errorf("isolation=%q: error = %v, want SKY-WF-059", iso, err)
		}
	}
}

func TestParse_PermissionsInteractive_AcceptsLooseIsolation(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.manual = true
claude.isolation = "loose"
⊕⊕

§check§
permissions = "interactive"
§§

∆check∆
run date
∆∆
`
	if _, err := Parse(strings.NewReader(input)); err != nil {
		t.Errorf("permissions=interactive + isolation=loose: unexpected error: %v", err)
	}
}

func TestParse_PermissionsInvalid_RejectsUnknownValue(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.manual = true
claude.isolation = "loose"
⊕⊕

§check§
permissions = "superuser"
§§

∆check∆
run date
∆∆
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for unknown permissions value, got nil")
	}
}

func TestParse_MaxTurns(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§p§
command = "x"
max_turns = 10
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Nodes[0].MaxTurns != 10 {
		t.Errorf("max_turns = %v, want 10", wf.Nodes[0].MaxTurns)
	}
}

func TestParse_MaxTurnsZeroOmitted(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§p§
command = "x"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Nodes[0].MaxTurns != 0 {
		t.Errorf("max_turns = %v, want 0 (zero value)", wf.Nodes[0].MaxTurns)
	}
}

func TestParse_LearningsConfig(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
learnings.exclude = ["patterns", "anti-patterns"]
⊕⊕

§p§
command = "x"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Learnings == nil {
		t.Fatal("wf.Learnings = nil, want non-nil")
	}
	if len(wf.Learnings.Exclude) != 2 {
		t.Fatalf("Exclude = %v, want 2 entries", wf.Learnings.Exclude)
	}
	if wf.Learnings.Exclude[0] != "patterns" || wf.Learnings.Exclude[1] != "anti-patterns" {
		t.Errorf("Exclude = %v, want [patterns anti-patterns]", wf.Learnings.Exclude)
	}
}

func TestParse_Spawn_Valid(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.manual = true
⊕⊕

§investigate§
spawn.workers = [{"id": "worker-a", "prompt": "Investigate auth module."}, {"id": "worker-b", "prompt": "Investigate payments module.", "model": "sonnet"}]
spawn.max_wait = "15m"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	n := wf.Nodes[0]
	if n.Spawn == nil {
		t.Fatal("Spawn = nil, want non-nil")
	}
	if len(n.Spawn.Workers) != 2 {
		t.Fatalf("Workers = %d, want 2", len(n.Spawn.Workers))
	}
	if n.Spawn.Workers[0].ID != "worker-a" {
		t.Errorf("Workers[0].ID = %q, want worker-a", n.Spawn.Workers[0].ID)
	}
	if n.Spawn.MaxWait != "15m" {
		t.Errorf("MaxWait = %q, want 15m", n.Spawn.MaxWait)
	}
	if n.NodeType() != "spawn" {
		t.Errorf("NodeType() = %q, want spawn", n.NodeType())
	}
}

func TestParse_Spawn_EmptyWorkers_SKY_WF_078(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.manual = true
⊕⊕

§investigate§
spawn.workers = []
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected SKY-WF-078 error, got nil")
	}
	if !strings.Contains(err.Error(), "SKY-WF-078") {
		t.Errorf("error = %v, want SKY-WF-078", err)
	}
}

func TestParse_Spawn_InvalidMaxWait_SKY_WF_081(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.manual = true
⊕⊕

§investigate§
spawn.workers = [{"id": "worker-a", "prompt": "do something"}]
spawn.max_wait = "not-a-duration"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected SKY-WF-081 error, got nil")
	}
	if !strings.Contains(err.Error(), "SKY-WF-081") {
		t.Errorf("error = %v, want SKY-WF-081", err)
	}
}

func TestParse_Spawn_InvalidOnIdle_SKY_WF_082(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.manual = true
⊕⊕

§investigate§
spawn.workers = [{"id": "worker-a", "prompt": "do something"}]
spawn.on_idle = "first"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected SKY-WF-082 error, got nil")
	}
	if !strings.Contains(err.Error(), "SKY-WF-082") {
		t.Errorf("error = %v, want SKY-WF-082", err)
	}
}

func TestParse_Spawn_BoundaryReadOnlyWithOwn_SKY_WF_083(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.manual = true
⊕⊕

§investigate§
spawn.workers = [{"id": "worker-a", "prompt": "do something"}]
spawn.boundary = {"read_only": true, "own": ["src/*.go"]}
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected SKY-WF-083 error, got nil")
	}
	if !strings.Contains(err.Error(), "SKY-WF-083") {
		t.Errorf("error = %v, want SKY-WF-083", err)
	}
}

func TestParse_Spawn_BoundaryReadOnlyWithMustNotEdit_SKY_WF_083(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.manual = true
⊕⊕

§investigate§
spawn.workers = [{"id": "worker-a", "prompt": "do something"}]
spawn.boundary = {"read_only": true, "must_not_edit": ["go.mod"]}
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected SKY-WF-083 error, got nil")
	}
	if !strings.Contains(err.Error(), "SKY-WF-083") {
		t.Errorf("error = %v, want SKY-WF-083", err)
	}
}

func TestParse_Spawn_BoundaryDoubleStarGlob_SKY_WF_084(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.manual = true
⊕⊕

§investigate§
spawn.workers = [{"id": "worker-a", "prompt": "do something"}]
spawn.boundary = {"own": ["src/**/*.go"]}
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected SKY-WF-084 error, got nil")
	}
	if !strings.Contains(err.Error(), "SKY-WF-084") {
		t.Errorf("error = %v, want SKY-WF-084", err)
	}
}

func TestParse_Spawn_BoundaryValid_OwnAndMustNotEdit(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.manual = true
⊕⊕

§investigate§
spawn.workers = [{"id": "worker-a", "prompt": "do something"}]
spawn.boundary = {"own": ["src/*.go"], "must_not_edit": ["go.mod"]}
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b := wf.Nodes[0].Spawn.Boundary
	if b == nil {
		t.Fatal("Boundary = nil, want non-nil")
	}
	if len(b.Own) != 1 || b.Own[0] != "src/*.go" {
		t.Errorf("Own = %v, want [src/*.go]", b.Own)
	}
	if len(b.MustNotEdit) != 1 || b.MustNotEdit[0] != "go.mod" {
		t.Errorf("MustNotEdit = %v, want [go.mod]", b.MustNotEdit)
	}
}

func TestParse_Spawn_BoundaryReadOnly_Valid(t *testing.T) {
	input := `⊕meta⊕
name = "t"
trigger.manual = true
⊕⊕

§investigate§
spawn.workers = [{"id": "worker-a", "prompt": "do something"}]
spawn.boundary = {"read_only": true}
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b := wf.Nodes[0].Spawn.Boundary
	if b == nil || !b.ReadOnly {
		t.Fatal("ReadOnly = false, want true")
	}
}

func TestParse_Council_Valid(t *testing.T) {
	t.Parallel()
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§advise§
council.members = [{"id": "sec", "prompt": "Security review."}, {"id": "perf", "prompt": "Performance review."}]
council.synthesis = "Combine the perspectives."
council.max_wait = "10m"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	n := wf.Nodes[0]
	if n.Council == nil {
		t.Fatal("Council = nil, want non-nil")
	}
	if len(n.Council.Members) != 2 {
		t.Fatalf("Members = %d, want 2", len(n.Council.Members))
	}
	if n.Council.Members[0].ID != "sec" {
		t.Errorf("Members[0].ID = %q, want sec", n.Council.Members[0].ID)
	}
	if n.Council.Synthesis != "Combine the perspectives." {
		t.Errorf("Synthesis = %q, want 'Combine the perspectives.'", n.Council.Synthesis)
	}
	if n.NodeType() != "council" {
		t.Errorf("NodeType() = %q, want council", n.NodeType())
	}
}

func TestParse_Council_EmptyMembers_SKY_WF_085(t *testing.T) {
	t.Parallel()
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§advise§
council.members = []
council.synthesis = "Combine."
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for empty members")
	}
	if !strings.Contains(err.Error(), "SKY-WF-085") {
		t.Errorf("error = %q, want SKY-WF-085", err.Error())
	}
}

func TestParse_Council_EmptySynthesis_SKY_WF_087(t *testing.T) {
	t.Parallel()
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§advise§
council.members = [{"id": "m1", "prompt": "Review this."}]
council.synthesis = ""
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for empty synthesis")
	}
	if !strings.Contains(err.Error(), "SKY-WF-087") {
		t.Errorf("error = %q, want SKY-WF-087", err.Error())
	}
}

func TestParse_Council_InvalidMaxWait_SKY_WF_088(t *testing.T) {
	t.Parallel()
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§advise§
council.members = [{"id": "m1", "prompt": "Review this."}]
council.synthesis = "Combine."
council.max_wait = "not-a-duration"
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for invalid max_wait")
	}
	if !strings.Contains(err.Error(), "SKY-WF-088") {
		t.Errorf("error = %q, want SKY-WF-088", err.Error())
	}
}

func TestParse_Council_NegativeBudget_SKY_WF_089(t *testing.T) {
	t.Parallel()
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§advise§
council.members = [{"id": "m1", "prompt": "Review this."}]
council.synthesis = "Combine."
council.max_budget_usd = -1.0
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for negative max_budget_usd")
	}
	if !strings.Contains(err.Error(), "SKY-WF-089") {
		t.Errorf("error = %q, want SKY-WF-089", err.Error())
	}
}

func TestParse_Council_MemberEmptyID_SKY_WF_086(t *testing.T) {
	t.Parallel()
	input := `⊕meta⊕
name = "t"
trigger.github.events = ["a"]
⊕⊕

§advise§
council.members = [{"id": "", "prompt": "Review this."}]
council.synthesis = "Combine."
§§
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for member with empty id")
	}
	if !strings.Contains(err.Error(), "SKY-WF-086") {
		t.Errorf("error = %q, want SKY-WF-086", err.Error())
	}
}
