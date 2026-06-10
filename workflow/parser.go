package workflow

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/skylence-be/skyerr"
)

func Parse(r io.Reader) (*Workflow, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, skyerr.Wrap(skyerr.ErrSkyFormat, "read workflow source", err)
	}
	wf, err := parseSky(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	wf.Raw = string(data)
	if err := validate(wf); err != nil {
		return nil, err
	}
	return wf, nil
}

// MaxLoopIterations is the hard cap on loop.max. Enforced at parse time and
// clamped at runtime as defense-in-depth.
const MaxLoopIterations = 100

// MaxWorkflowNodes caps the total node count per workflow. The executor spawns a
// goroutine per node at parse time; the cap keeps the model predictable and turns
// accidental fan-outs into an explicit parse error. Authors who need more can
// raise the constant with a code change — there is no runtime knob on purpose.
const MaxWorkflowNodes = 256

func validate(wf *Workflow) error {
	if wf.Name == "" {
		return skyerr.New(skyerr.ErrWorkflowValidation, "workflow: name is required")
	}
	if !workflowNameRe.MatchString(wf.Name) {
		return skyerr.New(skyerr.ErrWorkflowValidation,
			fmt.Sprintf("workflow name %q must match ^[a-zA-Z][a-zA-Z0-9_.-]*$", wf.Name))
	}

	if len(wf.Steps) == 0 && len(wf.Nodes) == 0 {
		return skyerr.New(skyerr.ErrWorkflowEmpty,
			fmt.Sprintf("workflow %q: at least one step or node is required", wf.Name))
	}

	if len(wf.Nodes) > MaxWorkflowNodes {
		return skyerr.New(skyerr.ErrWorkflowTooLarge,
			fmt.Sprintf("workflow %q: %d nodes exceeds max %d", wf.Name, len(wf.Nodes), MaxWorkflowNodes))
	}

	if wf.OutputStyle != "" && wf.OutputStyle != "terse" {
		return skyerr.New(skyerr.ErrOutputStyleInvalid,
			fmt.Sprintf("workflow %q: invalid output_style %q (valid: terse)", wf.Name, wf.OutputStyle))
	}

	if err := ValidateParams(wf); err != nil {
		return err
	}

	if err := validateMCPServers(wf.Name, wf.MCPServers); err != nil {
		return err
	}

	if err := validateClaudeIsolation(wf.Name, wf.Claude.Isolation); err != nil {
		return err
	}

	if wf.UI != "" {
		if filepath.IsAbs(wf.UI) || strings.Contains(wf.UI, "..") {
			return skyerr.New(skyerr.ErrWorkflowValidation,
				fmt.Sprintf("workflow %q: ui must be a relative path inside the repo", wf.Name))
		}
	}

	if t := wf.Trigger.SkyEvent; t != nil {
		if t.Event == "" {
			return skyerr.New(skyerr.ErrWorkflowValidation,
				fmt.Sprintf("workflow %q: trigger.sky_event.event is required", wf.Name))
		}
		if !emitNameRe.MatchString(t.Event) {
			return skyerr.New(skyerr.ErrWorkflowValidation,
				fmt.Sprintf("workflow %q: trigger.sky_event.event %q must match ^[a-z][a-z0-9_.-]*$", wf.Name, t.Event))
		}
	}

	for i, s := range wf.Steps {
		if s.Name == "" {
			return skyerr.New(skyerr.ErrWorkflowValidation,
				fmt.Sprintf("workflow %q: step %d: name is required", wf.Name, i))
		}
		if s.Prompt == "" {
			return skyerr.New(skyerr.ErrWorkflowValidation,
				fmt.Sprintf("workflow %q: step %q: prompt is required", wf.Name, s.Name))
		}
		if s.Emit != nil {
			if err := validateEmit(wf.Name, fmt.Sprintf("step %q", s.Name), s.Emit); err != nil {
				return err
			}
		}
	}

	ids := make(map[string]bool)
	for i, n := range wf.Nodes {
		if n.ID == "" {
			return skyerr.New(skyerr.ErrWorkflowValidation,
				fmt.Sprintf("workflow %q: node %d: id is required", wf.Name, i))
		}
		if ids[n.ID] {
			return skyerr.New(skyerr.ErrDuplicateNodeID,
				fmt.Sprintf("workflow %q: duplicate node id %q", wf.Name, n.ID))
		}
		ids[n.ID] = true

		// Loop wraps a body action — validate body + until, then skip generic kind checks.
		if n.Loop != nil {
			if err := validateLoopNode(wf.Name, n.ID, &n); err != nil {
				return err
			}
		} else {
			// Count mutually exclusive execution kinds (prompt alone is also valid).
			execKinds := 0
			if n.Command != "" {
				execKinds++
			}
			if n.Bash != "" {
				execKinds++
			}
			if n.BashFile != "" {
				execKinds++
			}
			if n.HTTP != nil {
				execKinds++
			}
			if n.Eval != nil {
				execKinds++
			}
			if n.Wait != nil {
				execKinds++
			}
			if n.Cancel != nil {
				execKinds++
			}
			if n.Script != nil {
				execKinds++
			}
			if n.Approval != nil {
				execKinds++
			}
			if n.Invoke != nil {
				execKinds++
			}
			if n.AcquireLock != nil {
				execKinds++
			}
			if n.ReleaseLock != nil {
				execKinds++
			}
			if n.Spawn != nil {
				execKinds++
			}
			if n.Council != nil {
				execKinds++
			}
			if n.Review != nil {
				execKinds++
			}
			nodeType := n.NodeType()
			if execKinds == 0 && n.Prompt == "" {
				return skyerr.New(skyerr.ErrNodeMissingAction,
					fmt.Sprintf("workflow %q: node %q: prompt, command, bash, bash_file, http, eval, wait, cancel, script, approval, invoke, acquire_lock, release_lock, spawn, council, or review is required", wf.Name, n.ID))
			}
			if n.Bash != "" && n.BashFile != "" {
				return skyerr.New(skyerr.ErrNodeAmbiguousAction,
					fmt.Sprintf("workflow %q: node %q: bash and bash_file are mutually exclusive", wf.Name, n.ID))
			}
			if execKinds > 1 {
				return skyerr.New(skyerr.ErrNodeAmbiguousAction,
					fmt.Sprintf("workflow %q: node %q: only one of command, bash, http, eval, wait, cancel, script, approval, invoke, acquire_lock, release_lock, spawn, council, or review may be set", wf.Name, n.ID))
			}
			if n.Invoke != nil && n.Invoke.Target == "" {
				return skyerr.New(skyerr.ErrNodeMissingAction,
					fmt.Sprintf("workflow %q: node %q: invoke.target is required", wf.Name, n.ID))
			}

			// cancel nodes must not carry prompt or other action fields
			if nodeType == "cancel" && n.Prompt != "" {
				return skyerr.New(skyerr.ErrNodeAmbiguousAction,
					fmt.Sprintf("workflow %q: node %q: cancel node must not have a prompt", wf.Name, n.ID))
			}

			if n.HTTP != nil && n.HTTP.URL == "" {
				return skyerr.New(skyerr.ErrHTTPURLRequired,
					fmt.Sprintf("workflow %q: node %q: http.url is required", wf.Name, n.ID))
			}
			if n.Eval != nil {
				if err := validateEvalConfig(wf.Name, n.ID, n.Eval); err != nil {
					return err
				}
			}
			if n.Wait != nil {
				if err := validateWaitConfig(wf.Name, n.ID, n.Wait); err != nil {
					return err
				}
			}
			if n.Approval != nil {
				if err := validateApprovalConfig(wf.Name, n.ID, n.Approval); err != nil {
					return err
				}
			}
			if n.Script != nil {
				if err := validateScriptConfig(wf.Name, n.ID, n.Script); err != nil {
					return err
				}
			}
			if n.AcquireLock != nil {
				if err := validateLockConfig(wf.Name, n.ID, n.AcquireLock); err != nil {
					return err
				}
			}
			if n.ReleaseLock != nil {
				if err := validateLockConfig(wf.Name, n.ID, n.ReleaseLock); err != nil {
					return err
				}
			}
			if n.Spawn != nil {
				if err := validateSpawnConfig(wf.Name, n.ID, n.Spawn); err != nil {
					return err
				}
			}
			if n.Council != nil {
				if err := validateCouncilConfig(wf.Name, n.ID, n.Council); err != nil {
					return err
				}
			}
			if n.Review != nil {
				if err := validateReviewConfig(wf.Name, n.ID, n.Review); err != nil {
					return err
				}
			}
		}

		// Per-node field validation applies to all node types.
		if len(n.AllowedTools) > 0 && len(n.DeniedTools) > 0 {
			return skyerr.New(skyerr.ErrNodeAmbiguousAction,
				fmt.Sprintf("workflow %q: node %q: allowed_tools and denied_tools are mutually exclusive", wf.Name, n.ID))
		}
		if n.Context != "" && n.Context != "fresh" && n.Context != "shared" {
			return skyerr.New(skyerr.ErrWorkflowValidation,
				fmt.Sprintf("workflow %q: node %q: context must be \"fresh\" or \"shared\"", wf.Name, n.ID))
		}
		if n.Context == "fresh" && n.ChainFrom != "" {
			return skyerr.New(skyerr.ErrWorkflowValidation,
				fmt.Sprintf("workflow %q: node %q: context=fresh is incompatible with chain_from", wf.Name, n.ID))
		}
		if n.Thinking != nil {
			if err := validateThinkingConfig(wf.Name, n.ID, n.Thinking); err != nil {
				return err
			}
		}
		if n.Retry != nil {
			if err := validateRetryConfig(wf.Name, n.ID, n.Retry); err != nil {
				return err
			}
		}
		if n.Sandbox != nil {
			if err := validateSandboxConfig(wf.Name, n.ID, n.Sandbox); err != nil {
				return err
			}
		}
		if len(n.Links) > 0 {
			if err := validateLinks(wf.Name, n.ID, n.Links); err != nil {
				return err
			}
		}
		if n.MCPConfig != "" {
			if filepath.IsAbs(n.MCPConfig) || strings.Contains(n.MCPConfig, "..") {
				return skyerr.New(skyerr.ErrWorkflowValidation,
					fmt.Sprintf("workflow %q: node %q: mcp_config must be a relative path inside the repo", wf.Name, n.ID))
			}
		}
		if n.UI != "" {
			if filepath.IsAbs(n.UI) || strings.Contains(n.UI, "..") {
				return skyerr.New(skyerr.ErrWorkflowValidation,
					fmt.Sprintf("workflow %q: node %q: ui must be a relative path inside the repo", wf.Name, n.ID))
			}
		}
		if n.Permissions != "" && n.Permissions != "interactive" {
			return skyerr.New(skyerr.ErrWorkflowValidation,
				fmt.Sprintf("workflow %q: node %q: permissions must be \"interactive\" or omitted", wf.Name, n.ID))
		}
		if n.Permissions == "interactive" && wf.Claude.Isolation != "loose" {
			return skyerr.New(skyerr.ErrInteractiveStrictConflict,
				fmt.Sprintf("workflow %q: node %q: permissions=\"interactive\" requires claude.isolation=\"loose\" in ⊕meta⊕ (--bare suppresses hooks)", wf.Name, n.ID))
		}

		const maxMatcherLen = 1024
		for event, entries := range n.Hooks {
			for j, entry := range entries {
				if len(entry.Matcher) > maxMatcherLen {
					return skyerr.New(skyerr.ErrWorkflowValidation,
						fmt.Sprintf("workflow %q: node %q: hooks[%s][%d].matcher exceeds %d characters", wf.Name, n.ID, event, j, maxMatcherLen))
				}
				if entry.Matcher != "" {
					if _, err := regexp.Compile(entry.Matcher); err != nil {
						return skyerr.New(skyerr.ErrWorkflowValidation,
							fmt.Sprintf("workflow %q: node %q: hooks[%s][%d].matcher is not a valid regex: %v", wf.Name, n.ID, event, j, err))
					}
				}
			}
		}
	}

	nodeMap := make(map[string]*Node, len(wf.Nodes))
	for i := range wf.Nodes {
		nodeMap[wf.Nodes[i].ID] = &wf.Nodes[i]
	}

	adj := make(map[string][]string)
	for _, n := range wf.Nodes {
		for _, dep := range n.DependsOn {
			if dep == n.ID {
				return skyerr.New(skyerr.ErrNodeSelfDependency,
					fmt.Sprintf("workflow %q: node %q depends on itself", wf.Name, n.ID))
			}
			if !ids[dep] {
				return skyerr.New(skyerr.ErrUnknownDependency,
					fmt.Sprintf("workflow %q: node %q depends on unknown node %q", wf.Name, n.ID, dep))
			}
		}
		adj[n.ID] = n.DependsOn

		if n.TriggerRule != "" && !skyerr.ValidTriggerRules[n.TriggerRule] {
			return skyerr.New(skyerr.ErrTriggerRuleInvalid,
				fmt.Sprintf("workflow %q: node %q: invalid trigger_rule %q (valid: all_done, one_success, one_failure)", wf.Name, n.ID, n.TriggerRule))
		}

		if n.Emit != nil {
			if err := validateEmit(wf.Name, fmt.Sprintf("node %q", n.ID), n.Emit); err != nil {
				return err
			}
		}

		if n.ChainFrom != "" {
			// chain_from only valid on prompt/command nodes (the ones that run Claude)
			nt := n.NodeType()
			if nt != "prompt" && nt != "command" {
				return skyerr.New(skyerr.ErrChainFromInvalid,
					fmt.Sprintf("workflow %q: node %q: chain_from only valid on prompt/command nodes, not %q", wf.Name, n.ID, nt))
			}
			// chain_from target must be listed in depends_on
			inDeps := false
			for _, dep := range n.DependsOn {
				if dep == n.ChainFrom {
					inDeps = true
					break
				}
			}
			if !inDeps {
				return skyerr.New(skyerr.ErrChainFromNotInDeps,
					fmt.Sprintf("workflow %q: node %q: chain_from %q must be listed in depends_on", wf.Name, n.ID, n.ChainFrom))
			}
			// chain_from target must itself be a prompt/command node (produces a Claude session)
			depNode := nodeMap[n.ChainFrom]
			if dt := depNode.NodeType(); dt != "prompt" && dt != "command" {
				return skyerr.New(skyerr.ErrChainFromTargetInvalid,
					fmt.Sprintf("workflow %q: node %q: chain_from %q is a %q node — only prompt/command nodes produce sessions", wf.Name, n.ID, n.ChainFrom, dt))
			}
		}

		// approval.on_reject.prompt_node must be a direct depends_on dep and a prompt/command node.
		if n.Approval != nil && n.Approval.OnReject != nil && n.Approval.OnReject.PromptNode != "" {
			pn := n.Approval.OnReject.PromptNode
			inDeps := false
			for _, dep := range n.DependsOn {
				if dep == pn {
					inDeps = true
					break
				}
			}
			if !inDeps {
				return skyerr.New(skyerr.ErrWorkflowValidation,
					fmt.Sprintf("workflow %q: node %q: on_reject.prompt_node %q must be listed in depends_on", wf.Name, n.ID, pn))
			}
			pnNode, exists := nodeMap[pn]
			if !exists {
				return skyerr.New(skyerr.ErrUnknownDependency,
					fmt.Sprintf("workflow %q: node %q: on_reject.prompt_node %q references unknown node", wf.Name, n.ID, pn))
			}
			if dt := pnNode.NodeType(); dt != "prompt" && dt != "command" {
				return skyerr.New(skyerr.ErrWorkflowValidation,
					fmt.Sprintf("workflow %q: node %q: on_reject.prompt_node %q is a %q node — only prompt/command nodes can be re-executed on rejection", wf.Name, n.ID, pn, dt))
			}
		}
	}

	if err := detectCycle(wf.Name, adj); err != nil {
		return err
	}

	return nil
}

func validateLoopNode(wfName, nodeID string, n *Node) error {
	// Body: exactly one "kind" of action. command and prompt may coexist — the runner
	// concatenates wfCmd.Prompt + node.Prompt when both are set, matching the documented
	// "prompt is also a supplement for command nodes" behaviour. bash is exclusive.
	if n.HTTP != nil || n.Eval != nil || n.Wait != nil || n.Cancel != nil || n.Script != nil || n.Approval != nil || n.AcquireLock != nil || n.ReleaseLock != nil || n.Spawn != nil || n.Council != nil || n.Review != nil {
		return skyerr.New(skyerr.ErrLoopBodyInvalid,
			fmt.Sprintf("workflow %q: node %q: loop body cannot be http, eval, wait, cancel, script, approval, acquire_lock, release_lock, spawn, council, or review", wfName, nodeID))
	}
	hasBash := n.Bash != "" || n.BashFile != ""
	hasClaude := n.Command != "" || n.Prompt != ""
	if !hasBash && !hasClaude {
		return skyerr.New(skyerr.ErrNodeMissingAction,
			fmt.Sprintf("workflow %q: node %q: loop requires a body (command, bash, bash_file, or prompt)", wfName, nodeID))
	}
	if hasBash && hasClaude {
		return skyerr.New(skyerr.ErrNodeAmbiguousAction,
			fmt.Sprintf("workflow %q: node %q: loop body cannot mix bash with command/prompt", wfName, nodeID))
	}

	// Until condition: exactly one of bash or eval
	untilKinds := 0
	if n.Loop.Until.Bash != "" {
		untilKinds++
	}
	if n.Loop.Until.Eval != nil {
		untilKinds++
	}
	if untilKinds != 1 {
		return skyerr.New(skyerr.ErrLoopUntilInvalid,
			fmt.Sprintf("workflow %q: node %q: loop.until requires exactly one of bash or eval", wfName, nodeID))
	}
	if n.Loop.Until.Eval != nil {
		if err := validateEvalConfig(wfName, nodeID+".loop.until", n.Loop.Until.Eval); err != nil {
			return err
		}
	}
	if n.Loop.Max > MaxLoopIterations {
		return skyerr.New(skyerr.ErrLoopMaxExceeded,
			fmt.Sprintf("workflow %q: node %q: loop.max %d exceeds cap of %d",
				wfName, nodeID, n.Loop.Max, MaxLoopIterations))
	}
	// idle_timeout_ms only makes sense for bash bodies. Prompt/command bodies
	// stream model tokens continuously; "idle" is not observable at the process
	// level. Reject the combination so authors fall back to loop.max instead.
	if n.Loop.IdleTimeoutMs != 0 {
		if n.Loop.IdleTimeoutMs < 0 {
			return skyerr.New(skyerr.ErrLoopIdleTimeoutInvalid,
				fmt.Sprintf("workflow %q: node %q: loop.idle_timeout_ms must be > 0", wfName, nodeID))
		}
		if !hasBash {
			return skyerr.New(skyerr.ErrLoopIdleTimeoutInvalid,
				fmt.Sprintf("workflow %q: node %q: loop.idle_timeout_ms only applies to bash bodies; use loop.max for prompt/command bodies", wfName, nodeID))
		}
	}
	return nil
}

var validWaitChannels = map[string]bool{"": true, "manual": true, "webhook": true}

func validateWaitConfig(wfName, nodeID string, cfg *WaitConfig) error {
	if !validWaitChannels[cfg.Channel] {
		return skyerr.New(skyerr.ErrWaitChannelInvalid,
			fmt.Sprintf("workflow %q: node %q: wait.channel %q invalid (valid: manual, webhook)", wfName, nodeID, cfg.Channel))
	}
	if cfg.Timeout != "" {
		if _, err := time.ParseDuration(cfg.Timeout); err != nil {
			return skyerr.New(skyerr.ErrWaitTimeoutInvalid,
				fmt.Sprintf("workflow %q: node %q: wait.timeout %q is not a valid duration", wfName, nodeID, cfg.Timeout))
		}
	}
	return nil
}

// validateClaudeIsolation rejects workflow-level claude.isolation values other
// than "", "strict", or "loose". A raw cast in resolveIsolation would otherwise
// let unknown values (e.g. legacy "inherit") silently bypass all isolation
// flags — a loud parse-time error beats silent degradation.
func validateClaudeIsolation(wfName, level string) error {
	switch level {
	case "", "strict", "loose":
		return nil
	default:
		return skyerr.New(skyerr.ErrIsolationInvalid,
			fmt.Sprintf("workflow %q: invalid claude.isolation %q (valid: strict, loose)", wfName, level))
	}
}

// validateMCPServers ensures each mcp_servers entry is an object and carries
// at least one recognised transport field (command for stdio servers, url or
// type for SSE/HTTP servers). Shape beyond that is forwarded to Claude as-is.
func validateMCPServers(wfName string, servers map[string]any) error {
	for name, raw := range servers {
		if name == "" {
			return skyerr.New(skyerr.ErrMCPServerInvalid,
				fmt.Sprintf("workflow %q: mcp_servers entry has empty name", wfName))
		}
		obj, ok := raw.(map[string]any)
		if !ok {
			return skyerr.New(skyerr.ErrMCPServerInvalid,
				fmt.Sprintf("workflow %q: mcp_servers.%s must be an object", wfName, name))
		}
		_, hasCmd := obj["command"]
		_, hasURL := obj["url"]
		_, hasType := obj["type"]
		if !hasCmd && !hasURL && !hasType {
			return skyerr.New(skyerr.ErrMCPServerInvalid,
				fmt.Sprintf("workflow %q: mcp_servers.%s requires at least one of command, url, or type", wfName, name))
		}
	}
	return nil
}

func validateEvalConfig(wfName, nodeID string, cfg *EvalConfig) error {
	if cfg.Source == "" {
		return skyerr.New(skyerr.ErrEvalSourceRequired,
			fmt.Sprintf("workflow %q: node %q: eval.source is required", wfName, nodeID))
	}
	assertions := 0
	if cfg.Contains != "" {
		assertions++
	}
	if cfg.Matches != "" {
		assertions++
	}
	if cfg.Equals != "" {
		assertions++
	}
	if assertions != 1 {
		return skyerr.New(skyerr.ErrEvalAssertionRequired,
			fmt.Sprintf("workflow %q: node %q: eval requires exactly one of contains, matches, or equals", wfName, nodeID))
	}
	return nil
}

var validApprovalChannels = map[string]bool{"": true, "manual": true, "webhook": true}

func validateApprovalConfig(wfName, nodeID string, cfg *ApprovalConfig) error {
	if !validApprovalChannels[cfg.Channel] {
		return skyerr.New(skyerr.ErrWaitChannelInvalid,
			fmt.Sprintf("workflow %q: node %q: approval.channel %q invalid (valid: manual, webhook)", wfName, nodeID, cfg.Channel))
	}
	if cfg.Timeout != "" {
		if _, err := time.ParseDuration(cfg.Timeout); err != nil {
			return skyerr.New(skyerr.ErrWaitTimeoutInvalid,
				fmt.Sprintf("workflow %q: node %q: approval.timeout %q is not a valid duration", wfName, nodeID, cfg.Timeout))
		}
	}
	if cfg.OnReject != nil {
		if cfg.OnReject.MaxAttempts < 0 || cfg.OnReject.MaxAttempts > 10 {
			return skyerr.New(skyerr.ErrWorkflowValidation,
				fmt.Sprintf("workflow %q: node %q: approval.on_reject.max_attempts must be 0–10", wfName, nodeID))
		}
	}
	return nil
}

var validScriptRuntimes = map[string]bool{"bun": true, "uv": true}

func validateScriptConfig(wfName, nodeID string, cfg *ScriptConfig) error {
	if !validScriptRuntimes[cfg.Runtime] {
		return skyerr.New(skyerr.ErrScriptRuntimeInvalid,
			fmt.Sprintf("workflow %q: node %q: script.runtime %q invalid (valid: bun, uv)", wfName, nodeID, cfg.Runtime))
	}
	// Deps must be static strings — template expansion into package names is supply-chain injection.
	for _, dep := range cfg.Deps {
		if strings.Contains(dep, "{{") {
			return skyerr.New(skyerr.ErrScriptRuntimeInvalid,
				fmt.Sprintf("workflow %q: node %q: script.deps must not contain template expressions", wfName, nodeID))
		}
	}
	return nil
}

const lockTTLMax = time.Hour

func validateLockConfig(wfName, nodeID string, cfg *LockConfig) error {
	if cfg.Key == "" {
		return skyerr.New(skyerr.ErrLockKeyRequired,
			fmt.Sprintf("workflow %q: node %q: lock.key is required", wfName, nodeID))
	}
	if cfg.TTL != "" {
		d, err := time.ParseDuration(cfg.TTL)
		if err != nil || d <= 0 || d > lockTTLMax {
			return skyerr.New(skyerr.ErrLockTTLInvalid,
				fmt.Sprintf("workflow %q: node %q: lock.ttl %q is not a valid duration (must be > 0 and <= 1h)", wfName, nodeID, cfg.TTL))
		}
	}
	return nil
}

var validThinkingModes = map[string]bool{"adaptive": true, "enabled": true, "disabled": true}

func validateThinkingConfig(wfName, nodeID string, cfg *ThinkingConfig) error {
	if !validThinkingModes[cfg.Mode] {
		return skyerr.New(skyerr.ErrWorkflowValidation,
			fmt.Sprintf("workflow %q: node %q: thinking.mode %q invalid (valid: adaptive, enabled, disabled)", wfName, nodeID, cfg.Mode))
	}
	if cfg.Mode == "enabled" && cfg.BudgetTokens <= 0 {
		return skyerr.New(skyerr.ErrWorkflowValidation,
			fmt.Sprintf("workflow %q: node %q: thinking.budget_tokens is required when mode=enabled", wfName, nodeID))
	}
	return nil
}

var validRetryOnError = map[string]bool{"": true, "transient": true, "all": true}

func validateRetryConfig(wfName, nodeID string, cfg *RetryConfig) error {
	if cfg.MaxAttempts < 1 || cfg.MaxAttempts > 5 {
		return skyerr.New(skyerr.ErrWorkflowValidation,
			fmt.Sprintf("workflow %q: node %q: retry.max_attempts must be 1–5", wfName, nodeID))
	}
	if cfg.DelayMS != 0 && (cfg.DelayMS < 1000 || cfg.DelayMS > 60000) {
		return skyerr.New(skyerr.ErrWorkflowValidation,
			fmt.Sprintf("workflow %q: node %q: retry.delay_ms must be 1000–60000", wfName, nodeID))
	}
	if !validRetryOnError[cfg.OnError] {
		return skyerr.New(skyerr.ErrWorkflowValidation,
			fmt.Sprintf("workflow %q: node %q: retry.on_error %q invalid (valid: transient, all)", wfName, nodeID, cfg.OnError))
	}
	return nil
}

func validateSandboxConfig(wfName, nodeID string, cfg *SandboxConfig) error {
	for _, p := range cfg.Filesystem.Allow {
		if p == "" || filepath.IsAbs(p) || strings.Contains(p, "..") {
			return skyerr.New(skyerr.ErrSandboxPathInvalid,
				fmt.Sprintf("workflow %q: node %q: sandbox.filesystem.allow %q must be a relative path inside the repo", wfName, nodeID, p))
		}
	}
	return nil
}

// detectCycle uses DFS to find cycles in the DAG.
func detectCycle(wfName string, adj map[string][]string) error {
	const (
		white = 0 // unvisited
		gray  = 1 // in current path
		black = 2 // fully explored
	)
	color := make(map[string]int)

	var dfs func(node string) error
	dfs = func(node string) error {
		color[node] = gray
		for _, dep := range adj[node] {
			switch color[dep] {
			case gray:
				return skyerr.New(skyerr.ErrWorkflowCycle,
					fmt.Sprintf("workflow %q: cycle detected involving node %q", wfName, dep))
			case white:
				if err := dfs(dep); err != nil {
					return err
				}
			}
		}
		color[node] = black
		return nil
	}

	for node := range adj {
		if color[node] == white {
			if err := dfs(node); err != nil {
				return err
			}
		}
	}
	return nil
}

// emitNameRe validates sky event names: lowercase letter, then letters/digits/underscores/dots/hyphens.
var emitNameRe = regexp.MustCompile(`^[a-z][a-z0-9_.-]*$`)

// workflowNameRe constrains workflow.Name to characters safe for delimited
// joins (e.g. comma-joined ancestor chain in sky-event dispatch). Allowing
// commas would break the cycle guard at internal/runner/emit.go.
var workflowNameRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_.-]*$`)

func validateLinks(wfName, nodeID string, links []string) error {
	for i, name := range links {
		if name == "" || strings.ContainsAny(name, `/\`) {
			return skyerr.New(skyerr.ErrLinksInvalid,
				fmt.Sprintf("workflow %q: node %q: links[%d] %q must be a codebase name (non-empty, no path separators)", wfName, nodeID, i, name))
		}
	}
	return nil
}

func validateEmit(wfName, location string, emit *EmitSpec) error {
	if emit.Name == "" {
		return skyerr.New(skyerr.ErrWorkflowValidation,
			fmt.Sprintf("workflow %q: %s: emit.name is required", wfName, location))
	}
	if !emitNameRe.MatchString(emit.Name) {
		return skyerr.New(skyerr.ErrWorkflowValidation,
			fmt.Sprintf("workflow %q: %s: emit.name %q must match ^[a-z][a-z0-9_.-]*$", wfName, location, emit.Name))
	}
	return nil
}
