// Package skyerr provides structured errors with unique codes for Skylence.
//
// Error codes follow the format: SKY-{DOMAIN}-{NUMBER}
//
//	SKY-WF-*    Workflow errors (parse, validate, load, DAG, commands)
//	SKY-RUN-*   Runner errors (execution, DAG, steps, timeouts)
//	SKY-WH-*    Webhook errors (parse, signature, routing, rate limit)
//	SKY-DB-*    Store/database errors
//	SKY-CL-*    Claude CLI errors
//	SKY-CFG-*   Config errors
//	SKY-CLN-*   Clone errors
//	SKY-AUTH-*  Authentication/authorization errors
//	SKY-WS-*    WebSocket errors
//	SKY-API-*   API request errors
package skyerr

import (
	"fmt"
)

// Code is a typed error code string. Using a distinct type prevents
// passing arbitrary strings where an error code is expected.
type Code string

// Error is a structured error with a unique code.
type Error struct {
	Code    Code
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error {
	return e.Cause
}

// New creates a new Error.
func New(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// Wrap creates a new Error wrapping a cause.
func Wrap(code Code, message string, cause error) *Error {
	return &Error{Code: code, Message: message, Cause: cause}
}

// ── Workflow errors (SKY-WF-*) ──

const (
	ErrWorkflowParse             Code = "SKY-WF-001" // Workflow parse failure
	ErrWorkflowValidation        Code = "SKY-WF-002" // Validation (missing name, steps, etc.)
	ErrWorkflowNotFound          Code = "SKY-WF-003" // Workflow name not found
	ErrWorkflowLoadFailed        Code = "SKY-WF-004" // Failed to read workflow dir/file
	ErrWorkflowCycle             Code = "SKY-WF-005" // DAG contains a cycle
	ErrWorkflowEmpty             Code = "SKY-WF-006" // Workflow has no steps or nodes
	ErrSkyFormat                 Code = "SKY-WF-007" // .sky section format error (unclosed, orphan, nested, malformed marker, name collision)
	ErrCommandNotFound           Code = "SKY-WF-010" // Command .md file not found
	ErrCommandParseFailed        Code = "SKY-WF-011" // Command file parse error
	ErrCommandEmpty              Code = "SKY-WF-012" // Command file has no prompt body
	ErrDuplicateNodeID           Code = "SKY-WF-020" // DAG: duplicate node ID
	ErrUnknownDependency         Code = "SKY-WF-021" // DAG: depends_on references unknown node
	ErrNodeMissingAction         Code = "SKY-WF-022" // DAG: node has no prompt/command/bash/http/eval
	ErrNodeSelfDependency        Code = "SKY-WF-023" // DAG: node depends on itself
	ErrNodeAmbiguousAction       Code = "SKY-WF-024" // DAG: node has more than one execution kind set
	ErrOutputRefInvalid          Code = "SKY-WF-030" // Invalid $node.output reference
	ErrConditionParse            Code = "SKY-WF-031" // Failed to parse when condition
	ErrTriggerRuleInvalid        Code = "SKY-WF-032" // Invalid trigger_rule value
	ErrOutputStyleInvalid        Code = "SKY-WF-033" // Invalid output_style value
	ErrHTTPURLRequired           Code = "SKY-WF-034" // http node: url is required
	ErrEvalSourceRequired        Code = "SKY-WF-035" // eval: source is required
	ErrEvalAssertionRequired     Code = "SKY-WF-036" // eval: exactly one assertion required (contains, matches, or equals)
	ErrLoopBodyInvalid           Code = "SKY-WF-037" // loop body cannot be http, eval, or wait
	ErrLoopUntilInvalid          Code = "SKY-WF-038" // loop.until requires exactly one of bash or eval
	ErrWaitChannelInvalid        Code = "SKY-WF-039" // wait.channel is not a valid value
	ErrWaitTimeoutInvalid        Code = "SKY-WF-040" // wait.timeout is not a valid duration
	ErrChainFromInvalid          Code = "SKY-WF-041" // chain_from only valid on prompt/command nodes
	ErrChainFromNotInDeps        Code = "SKY-WF-042" // chain_from not listed in depends_on
	ErrChainFromTargetInvalid    Code = "SKY-WF-043" // chain_from target is not a prompt/command node
	ErrWorkflowTooLarge          Code = "SKY-WF-044" // workflow exceeds max node count
	ErrWorkflowSchema            Code = "SKY-WF-045" // JSON Schema validation failure
	ErrLoopMaxExceeded           Code = "SKY-WF-046" // loop.max exceeds the allowed cap
	ErrTemplateUnsafeSplice      Code = "SKY-WF-047" // bare {{var}} inside a JSON string literal in http.body
	ErrBashTrustUnsafe           Code = "SKY-WF-048" // bash sink (eval/source/bash -c) receives $SKY_OUTPUT_<prompt-node>
	ErrMCPServerInvalid          Code = "SKY-WF-049" // mcp_servers entry has invalid shape (missing command/url/type)
	ErrMCPServerCollision        Code = "SKY-WF-050" // mcp_servers name collides with a managed server
	ErrIsolationInvalid          Code = "SKY-WF-051" // claude.isolation has an unsupported value
	ErrScriptRuntimeInvalid      Code = "SKY-WF-052" // script.runtime is not "bun" or "uv", or deps contain template expansion
	ErrScriptRuntimeMissing      Code = "SKY-WF-053" // script runtime binary not found on PATH
	ErrSandboxPathInvalid        Code = "SKY-WF-054" // sandbox.filesystem.allow path is absolute or traverses outside the repo
	ErrSecretUndeclared          Code = "SKY-WF-055" // ${env:NAME} ref where NAME not in workflow secrets declaration
	ErrSecretEmptyName           Code = "SKY-WF-056" // ${env:} has empty name
	ErrSecretOutOfScope          Code = "SKY-WF-057" // ${env:NAME} used in an unsupported location (prompt/bash/eval/script)
	ErrLoopIdleTimeoutInvalid    Code = "SKY-WF-058" // loop.idle_timeout_ms set on a non-bash body or with a negative value
	ErrInteractiveStrictConflict Code = "SKY-WF-059" // permissions:interactive requires claude.isolation = "loose"; --bare suppresses hooks
	ErrOutputFormatSchemaInvalid Code = "SKY-WF-060" // output_format is not a valid JSON Schema
	ErrPromptUltraKeywordNoOp    Code = "SKY-WF-061" // bare keyword 'ultraplan' or 'ultrareview' in a prompt — silently no-ops under non-interactive 'claude -p'
	ErrLearningsConfig           Code = "SKY-WF-062" // learnings config is invalid (conflicting exclude+only, unknown category, invalid max_bytes)
	ErrSafetyClassMissing        Code = "SKY-WF-063" // bash step uses destructive command without safety: requires_permission
	ErrInvokeDynamicTarget       Code = "SKY-WF-064" // invoke.target contains a template expression; literal workflow name required
	ErrInvokeInsideLoop          Code = "SKY-WF-065" // invoke node inside a loop body; not supported in v1
	ErrInvokeSelfTarget          Code = "SKY-WF-066" // invoke.target resolves to the current workflow (self-invoke not allowed)
	ErrInvokeTargetNotFound      Code = "SKY-WF-067" // invoke.target not found in any workflow tier (repo, workspace, user)
	ErrLockKeyRequired           Code = "SKY-WF-068" // acquire_lock: lock.key is required
	ErrLockTTLInvalid            Code = "SKY-WF-069" // acquire_lock: lock.ttl is not a valid duration
	ErrSpawnWorkersEmpty         Code = "SKY-WF-078" // spawn.workers is empty
	ErrSpawnWorkerIDEmpty        Code = "SKY-WF-079" // spawn worker has an empty id
	ErrSpawnWorkerPromptEmpty    Code = "SKY-WF-080" // spawn worker has an empty prompt
	ErrSpawnMaxWaitInvalid       Code = "SKY-WF-081" // spawn.max_wait is not a valid duration
	ErrSpawnOnIdleInvalid        Code = "SKY-WF-082" // spawn.on_idle must be "any" or "all"
	ErrSpawnBoundaryContradicts  Code = "SKY-WF-083" // spawn.boundary: read_only is contradictory with own or must_not_edit
	ErrSpawnBoundaryGlobInvalid  Code = "SKY-WF-084" // spawn.boundary: glob pattern contains ** (unsupported; use single-segment wildcards)
	ErrCouncilMembersEmpty       Code = "SKY-WF-085" // council.members is empty
	ErrCouncilMemberInvalid      Code = "SKY-WF-086" // council member has an empty id or prompt
	ErrCouncilSynthesisEmpty     Code = "SKY-WF-087" // council.synthesis is empty
	ErrCouncilMaxWaitInvalid     Code = "SKY-WF-088" // council.max_wait is not a valid duration
	ErrCouncilBudgetNegative     Code = "SKY-WF-089" // council.max_budget_usd is negative

	ErrReviewPathsInvalid        Code = "SKY-WF-091" // review.paths contains an empty string entry
	ErrLinksInvalid              Code = "SKY-WF-092" // links entry is empty or contains a path separator
	ErrCheckRunConclusionInvalid Code = "SKY-WF-093" // trigger.github.check_run.conclusion is not a valid GitHub conclusion
	ErrCheckRunMissingEvent      Code = "SKY-WF-094" // trigger.github.check_run set but events lacks "check_run.completed"
	ErrSourceTriggerEventsEmpty  Code = "SKY-WF-095" // sentry or linear trigger has no events listed
)

// ── Runner errors (SKY-RUN-*) ──

const (
	ErrRunCreateFailed       Code = "SKY-RUN-001" // Failed to create run record
	ErrRunCancelled          Code = "SKY-RUN-002" // Run was cancelled by user/system
	ErrRunTimeout            Code = "SKY-RUN-003" // Run exceeded maximum duration
	ErrStepCreateFailed      Code = "SKY-RUN-010" // Failed to create step record
	ErrStepExecFailed        Code = "SKY-RUN-011" // Step execution failed
	ErrStepTimeout           Code = "SKY-RUN-012" // Step exceeded time limit
	ErrStepSkipped           Code = "SKY-RUN-013" // Step skipped (condition false or trigger rule)
	ErrDAGNodeFailed         Code = "SKY-RUN-020" // DAG node execution failed
	ErrDAGConditionFailed    Code = "SKY-RUN-021" // DAG condition evaluation error
	ErrDAGTriggerSkipped     Code = "SKY-RUN-022" // Node skipped due to trigger rule
	ErrDAGDeadlock           Code = "SKY-RUN-023" // DAG has unresolvable dependencies at runtime
	ErrBashExecFailed        Code = "SKY-RUN-030" // Bash node execution failed
	ErrBashTimeout           Code = "SKY-RUN-031" // Bash node exceeded time limit
	ErrBashExitCode          Code = "SKY-RUN-032" // Bash node exited with non-zero code
	ErrOutputCaptureFailed   Code = "SKY-RUN-040" // Failed to capture structured output
	ErrOutputSchemaInvalid   Code = "SKY-RUN-041" // Output doesn't match output_format schema
	ErrHTTPExecFailed        Code = "SKY-RUN-050" // HTTP node request failed
	ErrHTTPStatusUnexpected  Code = "SKY-RUN-051" // HTTP response status not in expected range
	ErrSecretMissingEnv      Code = "SKY-RUN-052" // declared secret env var is not set at run time
	ErrSecretValueTooShort   Code = "SKY-RUN-053" // secret env var value too short for safe redaction (minimum 8 chars)
	ErrEvalAssertFailed      Code = "SKY-RUN-060" // eval assertion failed (halts run)
	ErrLoopMaxReached        Code = "SKY-RUN-070" // loop hit max iterations without condition passing
	ErrLoopIdleTimeout       Code = "SKY-RUN-071" // loop bash body produced no stdout within loop.idle_timeout_ms
	ErrWaitTimeout           Code = "SKY-RUN-080" // wait node timed out before resume signal
	ErrWaitNotFound          Code = "SKY-RUN-081" // resume called but no matching wait node found
	ErrWaitRejected          Code = "SKY-RUN-082" // wait node rejected by approver
	ErrStepBudgetExceeded    Code = "SKY-RUN-090" // node cost exceeded max_budget_usd
	ErrRunBudgetExceeded     Code = "SKY-RUN-091" // workflow run cost exceeded workflow max_budget_usd
	ErrMonthlyBudgetExceeded Code = "SKY-RUN-092" // monthly spend cap reached; run refused
	ErrRunNotInFlight        Code = "SKY-RUN-093" // pause attempted on a run that is not in flight
	ErrRunNotPaused          Code = "SKY-RUN-094" // resume-paused attempted on a run that is not paused
)

// ── Webhook errors (SKY-WH-*) ──

const (
	ErrWebhookInvalidSig   Code = "SKY-WH-001" // Signature verification failed
	ErrWebhookParseFailed  Code = "SKY-WH-002" // Payload parse error
	ErrWebhookMissingEvent Code = "SKY-WH-003" // Missing event type header
	ErrWebhookNoMatch      Code = "SKY-WH-004" // No workflows matched the event
	ErrWebhookRateLimit    Code = "SKY-WH-005" // Too many webhook requests
	ErrWebhookPayloadToo   Code = "SKY-WH-006" // Payload exceeds size limit
	ErrWebhookReplay       Code = "SKY-WH-007" // Request timestamp too old (replay attack)
	ErrWebhookUnsupported  Code = "SKY-WH-008" // Unsupported event type
)

// ── Store errors (SKY-DB-*) ──

const (
	ErrDBOpen       Code = "SKY-DB-001" // Failed to open database
	ErrDBSchema     Code = "SKY-DB-002" // Failed to apply schema
	ErrDBQuery      Code = "SKY-DB-003" // Query execution failed
	ErrDBNotFound   Code = "SKY-DB-004" // Record not found
	ErrDBMigration  Code = "SKY-DB-005" // Migration failed
	ErrDBConstraint Code = "SKY-DB-006" // Constraint violation (unique, FK, etc.)
	ErrDBTimeout    Code = "SKY-DB-007" // Database operation timed out
	ErrDBCorrupt    Code = "SKY-DB-008" // Database file corrupted
	ErrDBAmbiguous  Code = "SKY-DB-009" // Multiple records match; caller must disambiguate
)

// ── Claude CLI errors (SKY-CL-*) ──

const (
	ErrClaudeNotFound      Code = "SKY-CL-001" // claude binary not in PATH
	ErrClaudeStartFailed   Code = "SKY-CL-002" // Failed to start claude process
	ErrClaudeStreamParse   Code = "SKY-CL-003" // Failed to parse stream-json output
	ErrClaudeExitError     Code = "SKY-CL-004" // claude exited with non-zero
	ErrClaudeBudgetExhaust Code = "SKY-CL-005" // Budget limit reached
	ErrClaudeTimeout       Code = "SKY-CL-006" // Claude session timed out
	ErrClaudeAuthFailed    Code = "SKY-CL-007" // Claude authentication failed
	ErrClaudeRateLimit     Code = "SKY-CL-008" // Claude API rate limited
	ErrClaudeOverloaded    Code = "SKY-CL-009" // Claude API overloaded
	ErrClaudeContextLimit  Code = "SKY-CL-010" // Context window exceeded
)

// ── Config errors (SKY-CFG-*) ──

const (
	ErrConfigLoad       Code = "SKY-CFG-001" // Failed to load config file
	ErrConfigParse      Code = "SKY-CFG-002" // Failed to parse config YAML
	ErrConfigMissing    Code = "SKY-CFG-003" // Required config value missing
	ErrConfigInvalid    Code = "SKY-CFG-004" // Config value invalid (wrong type, out of range)
	ErrConfigPermission Code = "SKY-CFG-005" // Cannot read/write config file (permissions)

	// repo-root skylence.json lint codes (SKY-CFG-010+)
	ErrRepoCfgLoad           Code = "SKY-CFG-010" // failed to read skylence.json
	ErrRepoCfgParse          Code = "SKY-CFG-011" // skylence.json is not valid JSON
	ErrRepoCfgScriptEnvRef   Code = "SKY-CFG-013" // ${env:NAME} in scripts (unsupported; use env channel)
	ErrRepoCfgSidecarName    Code = "SKY-CFG-014" // sidecar entry missing name
	ErrRepoCfgSidecarCommand Code = "SKY-CFG-015" // sidecar entry missing command
	ErrRepoCfgPreviewURL     Code = "SKY-CFG-016" // preview block missing url
	ErrRepoCfgSetupFailed    Code = "SKY-CFG-017" // scripts.setup exited non-zero
	ErrRepoCfgArchiveFailed  Code = "SKY-CFG-018" // scripts.archive exited non-zero (non-fatal)
)

// ── Clone errors (SKY-CLN-*) ──

const (
	ErrCloneFailed     Code = "SKY-CLN-001" // git clone failed
	ErrClonePullFailed Code = "SKY-CLN-002" // git pull on existing clone failed
	ErrCloneAuthFailed Code = "SKY-CLN-003" // git authentication failed (bad token, no SSH key)
	ErrCloneNotFound   Code = "SKY-CLN-004" // Repository not found (404)
	ErrCloneDiskFull   Code = "SKY-CLN-005" // Not enough disk space for clone
)

// ── Auth errors (SKY-AUTH-*) ──

const (
	ErrAuthRequired       Code = "SKY-AUTH-001" // Authentication required (no token/session)
	ErrAuthInvalidToken   Code = "SKY-AUTH-002"
	ErrAuthForbidden      Code = "SKY-AUTH-003" // Authenticated but not authorized for this action
	ErrAuthSessionExpired Code = "SKY-AUTH-004" // Session has expired
	ErrAuthIPBlocked      Code = "SKY-AUTH-005" // Request from blocked IP
)

// ── WebSocket errors (SKY-WS-*) ──

const (
	ErrWSConnectFailed    Code = "SKY-WS-001" // WebSocket connection failed
	ErrWSSubscribeDenied  Code = "SKY-WS-002" // Not allowed to subscribe to this run
	ErrWSMessageTooLarge  Code = "SKY-WS-003" // WebSocket message exceeds size limit
	ErrWSConnectionClosed Code = "SKY-WS-004" // WebSocket connection closed unexpectedly
	ErrWSInvalidMessage   Code = "SKY-WS-005" // Malformed WebSocket message
	ErrWSRateLimited      Code = "SKY-WS-006" // Too many WebSocket connections
)

// ── API errors (SKY-API-*) ──

const (
	ErrAPIBadRequest       Code = "SKY-API-001" // Malformed request body
	ErrAPINotFound         Code = "SKY-API-002" // Resource not found
	ErrAPIMethodNotAllowed Code = "SKY-API-003" // HTTP method not allowed
	ErrAPIRateLimit        Code = "SKY-API-004" // API rate limit exceeded
	ErrAPIPayloadTooLarge  Code = "SKY-API-005" // Request body exceeds size limit
	ErrAPIInternal         Code = "SKY-API-006" // Internal server error
	ErrAPIUnavailable      Code = "SKY-API-007" // Service temporarily unavailable
	ErrAPIValidation       Code = "SKY-API-008" // Request validation failed (missing/invalid fields)
)

// ValidTriggerRules are the allowed values for a node's trigger_rule field.
var ValidTriggerRules = map[string]bool{
	"all_done":    true,
	"all_success": true,
	"one_success": true,
	"one_failure": true,
}
