package workflow

import (
	"fmt"
	"strconv"
)

// Override describes a single field mutation to apply to a node before replay.
// Field must be one of: model, prompt, effort, isolation, max_budget_usd.
type Override struct {
	Node  string `json:"node"`
	Field string `json:"field"`
	Value string `json:"value"`
}

// ApplyOverrides mutates wf's nodes in place according to overrides.
// Returns a descriptive error on unknown node ID, unsupported field, or bad float.
func (wf *Workflow) ApplyOverrides(overrides []Override) error {
	for _, o := range overrides {
		node := wf.NodeByID(o.Node)
		if node == nil {
			return fmt.Errorf("override: unknown node %q", o.Node)
		}
		if err := applyOverride(node, o.Field, o.Value); err != nil {
			return fmt.Errorf("override node %q field %q: %w", o.Node, o.Field, err)
		}
	}
	return nil
}

// NodeByID returns a pointer to the node with the given ID, or nil if not found.
func (wf *Workflow) NodeByID(id string) *Node {
	for i := range wf.Nodes {
		if wf.Nodes[i].ID == id {
			return &wf.Nodes[i]
		}
	}
	return nil
}

func applyOverride(n *Node, field, value string) error {
	switch field {
	case "model":
		n.Model = value
	case "prompt":
		n.Prompt = value
	case "effort":
		n.Effort = value
	case "isolation":
		n.Isolation = value
	case "max_budget_usd":
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("max_budget_usd: %w", err)
		}
		n.MaxBudget = f
	default:
		return fmt.Errorf("unsupported field %q (supported: model, prompt, effort, isolation, max_budget_usd)", field)
	}
	return nil
}
