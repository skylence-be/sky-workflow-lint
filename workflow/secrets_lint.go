package workflow

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/skyway-harness-builder/skyerr"
)

var secretLintRe = regexp.MustCompile(`\$\{(\w+):([^}]*)\}`)

// ValidateSecrets returns SchemaIssues for secret-reference violations:
//
//   - SKY-WF-055: ${env:NAME} in an allowed location where NAME is not in wf.Secrets
//   - SKY-WF-056: ${env:} with an empty NAME
//   - SKY-WF-057: ${env:NAME} in an unsupported location (prompt/bash/system_prompt/
//     eval/loop.until.bash/wait.prompt/approval.prompt/step.prompt)
//
// Allowed locations: mcp_servers values, http.url, http.body, http.headers values.
func ValidateSecrets(wf *Workflow) []SchemaIssue {
	if wf == nil {
		return nil
	}

	decl := make(map[string]bool, len(wf.Secrets))
	for _, name := range wf.Secrets {
		decl[name] = true
	}

	var issues []SchemaIssue

	// --- In-scope: mcp_servers (scan JSON blob; ${env:X} only appears in string values) ---
	if len(wf.MCPServers) > 0 {
		if b, err := json.Marshal(wf.MCPServers); err == nil {
			issues = append(issues, scanInScope(string(b), "", "/mcp_servers", decl)...)
		}
	}

	// --- DAG nodes ---
	for _, n := range wf.Nodes {
		id := n.ID

		// In-scope: http fields
		if n.HTTP != nil {
			issues = append(issues, scanInScope(n.HTTP.URL, id, "/nodes/"+id+"/http/url", decl)...)
			issues = append(issues, scanInScope(n.HTTP.Body, id, "/nodes/"+id+"/http/body", decl)...)
			for hdr, val := range n.HTTP.Headers {
				issues = append(issues, scanInScope(val, id, "/nodes/"+id+"/http/headers/"+hdr, decl)...)
			}
		}

		// Out-of-scope: prompt, bash, system_prompt, eval, loop, wait, approval
		if n.Prompt != "" {
			issues = append(issues, scanOutOfScope(n.Prompt, id, "/nodes/"+id+"/prompt")...)
		}
		if n.Bash != "" {
			issues = append(issues, scanOutOfScope(n.Bash, id, "/nodes/"+id+"/bash")...)
		}
		if n.SystemPrompt != "" {
			issues = append(issues, scanOutOfScope(n.SystemPrompt, id, "/nodes/"+id+"/system_prompt")...)
		}
		if n.Eval != nil {
			for _, f := range []struct{ v, sub string }{
				{n.Eval.Source, "source"},
				{n.Eval.Contains, "contains"},
				{n.Eval.Matches, "matches"},
				{n.Eval.Equals, "equals"},
			} {
				if f.v != "" {
					issues = append(issues, scanOutOfScope(f.v, id, "/nodes/"+id+"/eval/"+f.sub)...)
				}
			}
		}
		if n.Loop != nil && n.Loop.Until.Bash != "" {
			issues = append(issues, scanOutOfScope(n.Loop.Until.Bash, id, "/nodes/"+id+"/loop/until/bash")...)
		}
		if n.Wait != nil && n.Wait.Prompt != "" {
			issues = append(issues, scanOutOfScope(n.Wait.Prompt, id, "/nodes/"+id+"/wait/prompt")...)
		}
		if n.Approval != nil && n.Approval.Prompt != "" {
			issues = append(issues, scanOutOfScope(n.Approval.Prompt, id, "/nodes/"+id+"/approval/prompt")...)
		}
	}

	// --- Sequential steps ---
	for i, s := range wf.Steps {
		if s.Prompt != "" {
			issues = append(issues, scanOutOfScope(s.Prompt, "", fmt.Sprintf("/steps/%d/prompt", i))...)
		}
	}

	return issues
}

// scanInScope checks a string from an allowed location (mcp_servers, http fields).
// Returns WF-056 for empty names, WF-055 for undeclared names.
func scanInScope(s, nodeID, pointer string, declared map[string]bool) []SchemaIssue {
	if s == "" {
		return nil
	}
	var issues []SchemaIssue
	seen := make(map[string]bool)
	for _, m := range secretLintRe.FindAllStringSubmatch(s, -1) {
		scheme, name := m[1], m[2]
		if scheme != "env" {
			continue
		}
		key := name // deduplicate per location
		if seen[key] {
			continue
		}
		seen[key] = true

		if name == "" {
			issues = append(issues, SchemaIssue{
				NodeID: nodeID, Pointer: pointer,
				Message: `secret reference ${env:} has empty name`,
				Code:    skyerr.ErrSecretEmptyName,
			})
			continue
		}
		if !declared[name] {
			issues = append(issues, SchemaIssue{
				NodeID: nodeID, Pointer: pointer,
				Message: fmt.Sprintf("${env:%s} is not declared in the workflow secrets list", name),
				Code:    skyerr.ErrSecretUndeclared,
			})
		}
	}
	return issues
}

// scanOutOfScope flags any ${env:NAME} ref found in an unsupported location.
func scanOutOfScope(s, nodeID, pointer string) []SchemaIssue {
	if s == "" {
		return nil
	}
	var issues []SchemaIssue
	seen := make(map[string]bool)
	for _, m := range secretLintRe.FindAllStringSubmatch(s, -1) {
		if m[1] != "env" {
			continue
		}
		name := m[2]
		if seen[name] {
			continue
		}
		seen[name] = true
		issues = append(issues, SchemaIssue{
			NodeID: nodeID, Pointer: pointer,
			Message: fmt.Sprintf("${env:%s} is not supported in %s — use mcp_servers or http node headers/url/body", name, pointer),
			Code:    skyerr.ErrSecretOutOfScope,
		})
	}
	return issues
}
