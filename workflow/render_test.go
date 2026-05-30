package workflow

import (
	"strings"
	"testing"
)

func TestRender_Sequential(t *testing.T) {
	wf := &Workflow{
		Name: "seq",
		Steps: []Step{
			{Name: "greet", Prompt: "Hello {{name}}, issue {{issue.number}}", Model: "opus"},
			{Name: "act", Prompt: "Act on $greet.output now"},
		},
	}

	nodes, err := Render(wf, t.TempDir(), map[string]string{"name": "Jonas", "issue.number": "42"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("len = %d, want 2", len(nodes))
	}

	// First node: vars substituted, no output refs
	if nodes[0].ID != "greet" {
		t.Errorf("id = %q, want greet", nodes[0].ID)
	}
	if !strings.Contains(nodes[0].Prompt, "Jonas") {
		t.Errorf("prompt missing var: %q", nodes[0].Prompt)
	}
	if !strings.Contains(nodes[0].Prompt, "42") {
		t.Errorf("prompt missing issue.number: %q", nodes[0].Prompt)
	}
	if len(nodes[0].Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", nodes[0].Warnings)
	}

	// Second node: $greet.output → placeholder + warning
	if !strings.Contains(nodes[1].Prompt, "<output of greet>") {
		t.Errorf("output ref not replaced: %q", nodes[1].Prompt)
	}
	if len(nodes[1].Warnings) == 0 {
		t.Error("expected warning for unresolvable output ref")
	}
}

func TestRender_DAG_TopoOrder(t *testing.T) {
	// Node B depends on A — A must appear first regardless of declaration order.
	wf := &Workflow{
		Name: "dag",
		Nodes: []Node{
			{ID: "b", DependsOn: []string{"a"}, Prompt: "use $a.output"},
			{ID: "a", Prompt: "step a with {{val}}"},
		},
	}

	nodes, err := Render(wf, t.TempDir(), map[string]string{"val": "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("len = %d, want 2", len(nodes))
	}
	if nodes[0].ID != "a" {
		t.Errorf("first node = %q, want a", nodes[0].ID)
	}
	if nodes[1].ID != "b" {
		t.Errorf("second node = %q, want b", nodes[1].ID)
	}

	// a: var substituted
	if !strings.Contains(nodes[0].Prompt, "hello") {
		t.Errorf("prompt missing var: %q", nodes[0].Prompt)
	}
	// b: output ref → placeholder
	if !strings.Contains(nodes[1].Prompt, "<output of a>") {
		t.Errorf("output ref not replaced: %q", nodes[1].Prompt)
	}
	if len(nodes[1].Warnings) == 0 {
		t.Error("expected warning on node b")
	}
}

func TestRender_DAG_Cycle(t *testing.T) {
	wf := &Workflow{
		Name: "cyclic",
		Nodes: []Node{
			{ID: "a", DependsOn: []string{"b"}},
			{ID: "b", DependsOn: []string{"a"}},
		},
	}
	_, err := Render(wf, t.TempDir(), nil)
	if err == nil {
		t.Fatal("expected cycle error")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("error = %q, want cycle mention", err.Error())
	}
}

func TestRender_BashNode(t *testing.T) {
	// Bash body must be shown verbatim — runtime does not template-expand vars
	// into shell text (that would be command injection). The env channel is
	// surfaced as a comment header so the author sees what $SKY_* resolves to.
	wf := &Workflow{
		Name: "bash",
		Nodes: []Node{
			{ID: "run", Bash: `echo "$SKY_GREETING" && cat $prev.output`},
		},
	}
	nodes, err := Render(wf, t.TempDir(), map[string]string{"greeting": "hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nodes[0].Kind != "bash" {
		t.Errorf("kind = %q, want bash", nodes[0].Kind)
	}
	if !strings.Contains(nodes[0].Bash, "# env: SKY_GREETING=hi") {
		t.Errorf("env header missing: %q", nodes[0].Bash)
	}
	if !strings.Contains(nodes[0].Bash, `echo "$SKY_GREETING"`) {
		t.Errorf("bash body missing: %q", nodes[0].Bash)
	}
	if !strings.Contains(nodes[0].Bash, "<output of prev>") {
		t.Errorf("bash output ref not replaced: %q", nodes[0].Bash)
	}
	if len(nodes[0].Warnings) == 0 {
		t.Error("expected warning for output ref in bash")
	}
}

func TestRender_BashNode_NoExpand(t *testing.T) {
	// Regression: legacy {{var}} syntax in bash must NOT be expanded at preview
	// time — runtime doesn't expand it either. Prevents dry-run from lying
	// about what the shell will actually see.
	wf := &Workflow{
		Name: "bashlegacy",
		Nodes: []Node{
			{ID: "run", Bash: "echo {{issue.title}}"},
		},
	}
	nodes, err := Render(wf, t.TempDir(), map[string]string{"issue.title": "$(touch pwned)"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(nodes[0].Bash, "echo {{issue.title}}") {
		t.Errorf("bash body should show literal {{...}}, got: %q", nodes[0].Bash)
	}
	// Header carries the raw value (safe — it's on a `# env:` comment line),
	// but the bash BODY must not have the injection payload spliced in.
	lines := strings.SplitN(nodes[0].Bash, "\n", 2)
	if len(lines) != 2 {
		t.Fatalf("expected env header + body, got: %q", nodes[0].Bash)
	}
	if !strings.HasPrefix(lines[0], "# env: SKY_ISSUE_TITLE=") {
		t.Errorf("env header format wrong: %q", lines[0])
	}
	if strings.Contains(lines[1], "$(touch pwned)") {
		t.Errorf("injection payload spliced into bash BODY: %q", lines[1])
	}
}

func TestRender_DefaultModel(t *testing.T) {
	wf := &Workflow{
		Name:  "defaults",
		Nodes: []Node{{ID: "x", Prompt: "hello"}},
	}
	nodes, err := Render(wf, t.TempDir(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nodes[0].Model != "sonnet" {
		t.Errorf("model = %q, want sonnet", nodes[0].Model)
	}
}

func TestRender_DuplicateWarningDeduped(t *testing.T) {
	// Same output ref used twice in one prompt — warning should appear once.
	wf := &Workflow{
		Name:  "dedup",
		Nodes: []Node{{ID: "x", Prompt: "$a.output and $a.output again"}},
	}
	nodes, err := Render(wf, t.TempDir(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var count int
	for _, w := range nodes[0].Warnings {
		if strings.Contains(w, "a.output") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("duplicate warning not deduped: warnings = %v", nodes[0].Warnings)
	}
}

func TestRender_MissingDep(t *testing.T) {
	wf := &Workflow{
		Name: "missing",
		Nodes: []Node{
			{ID: "a", DependsOn: []string{"ghost"}, Prompt: "x"},
		},
	}
	_, err := Render(wf, t.TempDir(), nil)
	if err == nil {
		t.Fatal("expected error for missing dep")
	}
	if !strings.Contains(err.Error(), "ghost") || !strings.Contains(err.Error(), "unknown") {
		t.Errorf("error = %q, want mention of unknown ghost", err.Error())
	}
}

func TestRender_DuplicateNodeID(t *testing.T) {
	wf := &Workflow{
		Name: "dup",
		Nodes: []Node{
			{ID: "a", Prompt: "x"},
			{ID: "a", Prompt: "y"},
		},
	}
	_, err := Render(wf, t.TempDir(), nil)
	if err == nil {
		t.Fatal("expected error for duplicate id")
	}
	if !strings.Contains(err.Error(), "duplicate") || !strings.Contains(err.Error(), `"a"`) {
		t.Errorf("error = %q, want duplicate/\"a\" mention", err.Error())
	}
}

func TestRender_LoopBash(t *testing.T) {
	wf := &Workflow{
		Name: "loopbash",
		Nodes: []Node{
			{ID: "lp", Bash: `make "$SKY_TARGET"`, Loop: &LoopConfig{Max: 3, Until: LoopCondition{Bash: "test -f done"}}},
		},
	}
	nodes, err := Render(wf, t.TempDir(), map[string]string{"target": "build"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nodes[0].Kind != "loop" {
		t.Errorf("kind = %q, want loop", nodes[0].Kind)
	}
	if !strings.Contains(nodes[0].Bash, "# env: SKY_TARGET=build") {
		t.Errorf("env header missing: %q", nodes[0].Bash)
	}
	if !strings.Contains(nodes[0].Bash, `make "$SKY_TARGET"`) {
		t.Errorf("bash body missing: %q", nodes[0].Bash)
	}
	if nodes[0].Prompt != "" {
		t.Errorf("loop-bash should not populate Prompt, got %q", nodes[0].Prompt)
	}
}

func TestRender_EvalNode(t *testing.T) {
	wf := &Workflow{
		Name: "evalwf",
		Nodes: []Node{
			{ID: "gen", Prompt: "generate something"},
			{ID: "check", DependsOn: []string{"gen"}, Eval: &EvalConfig{Source: "$gen.output", Contains: "ok"}},
		},
	}
	nodes, err := Render(wf, t.TempDir(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("len = %d, want 2", len(nodes))
	}
	ev := nodes[1]
	if ev.Kind != "eval" {
		t.Errorf("kind = %q, want eval", ev.Kind)
	}
	if ev.Model != "" {
		t.Errorf("model = %q, want empty (eval has no model)", ev.Model)
	}
	if ev.Eval == nil {
		t.Fatal("Eval field nil — assertion config not rendered")
	}
	if ev.Eval.Source != "$gen.output" {
		t.Errorf("eval.source = %q, want $gen.output", ev.Eval.Source)
	}
	if ev.Eval.Contains != "ok" {
		t.Errorf("eval.contains = %q, want ok", ev.Eval.Contains)
	}
}

func TestTopoSort_Linear(t *testing.T) {
	nodes := []Node{
		{ID: "c", DependsOn: []string{"b"}},
		{ID: "a"},
		{ID: "b", DependsOn: []string{"a"}},
	}
	got, err := topoSort(nodes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"a", "b", "c"}
	for i, n := range got {
		if n.ID != want[i] {
			t.Errorf("pos %d = %q, want %q", i, n.ID, want[i])
		}
	}
}
