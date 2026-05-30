package workflow

import (
	"fmt"
	"sort"
	"strings"
)

// RenderedNode is the dry-run preview of one workflow node.
type RenderedNode struct {
	ID        string      `json:"id"`
	Kind      string      `json:"kind"`
	Model     string      `json:"model,omitempty"`
	Effort    string      `json:"effort,omitempty"`
	Isolation string      `json:"isolation,omitempty"`
	DependsOn []string    `json:"depends_on,omitempty"`
	Prompt    string      `json:"prompt,omitempty"` // prompt/command/loop nodes
	Bash      string      `json:"bash,omitempty"`   // bash nodes
	HTTP      *HTTPConfig `json:"http,omitempty"`   // http nodes
	Eval      *EvalConfig `json:"eval,omitempty"`   // eval nodes
	Warnings  []string    `json:"warnings,omitempty"`
}

// Render produces a dry-run preview for every node in the workflow.
// For DAG workflows, nodes are returned in topological order.
// $node.output references are replaced with <output of {node}> placeholders
// and a warning is added to the node.
func Render(wf *Workflow, repo string, vars map[string]string) ([]RenderedNode, error) {
	if !wf.IsDAG() {
		return renderSequential(wf, vars)
	}
	return renderDAG(wf, repo, vars)
}

func renderSequential(wf *Workflow, vars map[string]string) ([]RenderedNode, error) {
	nodes := make([]RenderedNode, 0, len(wf.Steps))
	for _, step := range wf.Steps {
		model := step.Model
		if model == "" {
			model = "sonnet"
		}
		prompt := Expand(step.Prompt, vars)
		prompt, warns := dryRunSubstituteRefs(prompt)
		nodes = append(nodes, RenderedNode{
			ID:        step.Name,
			Kind:      "prompt",
			Model:     model,
			Effort:    step.Effort,
			Isolation: step.Isolation,
			Prompt:    prompt,
			Warnings:  warns,
		})
	}
	return nodes, nil
}

func renderDAG(wf *Workflow, repo string, vars map[string]string) ([]RenderedNode, error) {
	ordered, err := topoSort(wf.Nodes)
	if err != nil {
		return nil, err
	}
	rendered := make([]RenderedNode, 0, len(ordered))
	for _, node := range ordered {
		rendered = append(rendered, renderNode(node, repo, vars))
	}
	return rendered, nil
}

func renderNode(node *Node, repo string, vars map[string]string) RenderedNode {
	model := node.Model
	if model == "" {
		model = "sonnet"
	}
	rn := RenderedNode{
		ID:        node.ID,
		Kind:      node.NodeType(),
		Model:     model,
		Effort:    node.Effort,
		Isolation: node.Isolation,
		DependsOn: node.DependsOn,
	}

	switch rn.Kind {
	case "bash":
		bash, warns := dryRunSubstituteRefs(node.Bash)
		rn.Bash = prependEnvHeader(bash, vars)
		rn.Warnings = warns

	case "http":
		if node.HTTP != nil {
			h := *node.HTTP
			var allWarns []string
			h.URL, allWarns = appendSubstituted(h.URL, vars, allWarns)
			h.Body, allWarns = appendSubstituted(h.Body, vars, allWarns)
			rn.HTTP = &h
			rn.Warnings = allWarns
		}

	case "eval":
		rn.Eval = node.Eval
		rn.Model = "" // eval nodes don't dispatch to a model

	case "wait":
		if node.Wait != nil && node.Wait.Prompt != "" {
			prompt := Expand(node.Wait.Prompt, vars)
			prompt, warns := dryRunSubstituteRefs(prompt)
			rn.Prompt = prompt
			rn.Warnings = warns
		}

	default: // "prompt", "command", "loop"
		if rn.Kind == "loop" && (node.Bash != "" || node.BashFile != "") {
			// Mirror runLoopBody's bash branch — a loop can be a bash loop
			// (inline bash or a bash_file script reference).
			bash, warns := dryRunSubstituteRefs(node.Bash)
			rn.Bash = prependEnvHeader(bash, vars)
			rn.Warnings = warns
		} else {
			prompt := node.Prompt
			if node.Command != "" {
				cmd, err := LoadCommand(repo, node.Command)
				if err != nil {
					rn.Warnings = append(rn.Warnings, fmt.Sprintf("could not load command %q: %v", node.Command, err))
				} else {
					if node.Prompt != "" {
						prompt = cmd.Prompt + "\n\n" + node.Prompt
					} else {
						prompt = cmd.Prompt
					}
				}
			}
			prompt = Expand(prompt, vars)
			var warns []string
			prompt, warns = dryRunSubstituteRefs(prompt)
			rn.Prompt = prompt
			rn.Warnings = append(rn.Warnings, warns...)
		}
		if rn.Kind == "loop" && node.Loop != nil {
			rn.Warnings = append(rn.Warnings, fmt.Sprintf("loop (max %d): shows first iteration only; subsequent iterations depend on runtime output", node.Loop.Max))
		}
	}

	return rn
}

// prependEnvHeader shows the SKY_<KEY>=<value> env pairs a bash node will see at
// runtime. Runtime never substitutes vars into bash text — vars reach the shell
// only through the environment. The preview reflects that: bash body is shown
// verbatim, env channel is shown as a comment header so the author can see
// what references like $SKY_ISSUE_NUMBER will resolve to.
func prependEnvHeader(bash string, vars map[string]string) string {
	env := VarsAsEnv(vars)
	if len(env) == 0 {
		return bash
	}
	sort.Strings(env)
	var b strings.Builder
	for _, pair := range env {
		b.WriteString("# env: ")
		b.WriteString(pair)
		b.WriteByte('\n')
	}
	b.WriteString(bash)
	return b.String()
}

// appendSubstituted expands vars and replaces output refs in s, appending any warnings to warns.
func appendSubstituted(s string, vars map[string]string, warns []string) (out string, allWarns []string) {
	s = Expand(s, vars)
	var w []string
	s, w = dryRunSubstituteRefs(s)
	return s, append(warns, w...)
}

// dryRunSubstituteRefs replaces $node.output... references with <output of {node}>
// placeholders and returns any warnings generated.
func dryRunSubstituteRefs(text string) (out string, warnings []string) {
	seen := map[string]bool{}
	out = OutputRefRe.ReplaceAllStringFunc(text, func(match string) string {
		sub := OutputRefRe.FindStringSubmatch(match)
		if sub == nil {
			return match
		}
		nodeID := sub[1]
		warn := fmt.Sprintf("depends on %s.output; not available in dry-run", nodeID)
		if !seen[warn] {
			warnings = append(warnings, warn)
			seen[warn] = true
		}
		return fmt.Sprintf("<output of %s>", nodeID)
	})
	return out, warnings
}

// topoSort returns nodes in topological order using Kahn's algorithm.
// Returns an error if the graph contains a cycle or a depends_on reference to an unknown node.
func topoSort(nodes []Node) ([]*Node, error) {
	index := make(map[string]*Node, len(nodes))
	for i := range nodes {
		id := nodes[i].ID
		if _, dup := index[id]; dup {
			return nil, fmt.Errorf("duplicate node id %q", id)
		}
		index[id] = &nodes[i]
	}

	// Surface missing deps explicitly — otherwise they leave indegree > 0
	// and the caller sees a spurious "cycle" error.
	for i := range nodes {
		for _, dep := range nodes[i].DependsOn {
			if _, ok := index[dep]; !ok {
				return nil, fmt.Errorf("node %q depends on unknown node %q", nodes[i].ID, dep)
			}
		}
	}

	// dependents[A] = list of node IDs that declare A in their depends_on
	dependents := make(map[string][]string, len(nodes))
	indegree := make(map[string]int, len(nodes))
	for i := range nodes {
		id := nodes[i].ID
		if _, ok := indegree[id]; !ok {
			indegree[id] = 0
		}
		for _, dep := range nodes[i].DependsOn {
			dependents[dep] = append(dependents[dep], id)
			indegree[id]++
		}
	}

	var queue []*Node
	for i := range nodes {
		if indegree[nodes[i].ID] == 0 {
			queue = append(queue, &nodes[i])
		}
	}

	result := make([]*Node, 0, len(nodes))
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		result = append(result, n)
		for _, depID := range dependents[n.ID] {
			indegree[depID]--
			if indegree[depID] == 0 {
				queue = append(queue, index[depID])
			}
		}
	}

	if len(result) != len(nodes) {
		return nil, fmt.Errorf("workflow has a dependency cycle")
	}
	return result, nil
}
