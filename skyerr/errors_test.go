package skyerr

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestError_Format(t *testing.T) {
	err := New(ErrWorkflowNotFound, "workflow \"deploy\" not found")
	got := err.Error()
	want := "[SKY-WF-003] workflow \"deploy\" not found"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestError_FormatWithCause(t *testing.T) {
	cause := fmt.Errorf("file not found")
	err := Wrap(ErrDBOpen, "failed to open database", cause)
	got := err.Error()
	want := "[SKY-DB-001] failed to open database: file not found"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestError_Unwrap(t *testing.T) {
	cause := fmt.Errorf("root cause")
	err := Wrap(ErrClaudeStartFailed, "start failed", cause)

	if !errors.Is(err, err) {
		t.Error("errors.Is should match self")
	}

	var skyErr *Error
	if !errors.As(err, &skyErr) {
		t.Error("errors.As should match *Error")
	}
	if skyErr.Code != ErrClaudeStartFailed {
		t.Errorf("code = %q, want %q", skyErr.Code, ErrClaudeStartFailed)
	}
}

func allCodes() []Code {
	return []Code{
		// Workflow
		ErrWorkflowParse, ErrWorkflowValidation, ErrWorkflowNotFound,
		ErrWorkflowLoadFailed, ErrWorkflowCycle, ErrWorkflowEmpty,
		ErrCommandNotFound, ErrCommandParseFailed, ErrCommandEmpty,
		ErrDuplicateNodeID, ErrUnknownDependency, ErrNodeMissingAction,
		ErrNodeSelfDependency, ErrNodeAmbiguousAction, ErrOutputRefInvalid, ErrConditionParse,
		ErrTriggerRuleInvalid,
		ErrOutputStyleInvalid, ErrHTTPURLRequired, ErrEvalSourceRequired, ErrEvalAssertionRequired,
		ErrLoopBodyInvalid, ErrLoopUntilInvalid, ErrWaitChannelInvalid, ErrWaitTimeoutInvalid,
		ErrChainFromInvalid, ErrChainFromNotInDeps, ErrChainFromTargetInvalid,
		ErrWorkflowTooLarge, ErrWorkflowSchema, ErrLoopMaxExceeded,
		ErrTemplateUnsafeSplice, ErrBashTrustUnsafe,
		ErrLoopIdleTimeoutInvalid,
		// Runner
		ErrRunCreateFailed, ErrRunCancelled, ErrRunTimeout,
		ErrStepCreateFailed, ErrStepExecFailed, ErrStepTimeout, ErrStepSkipped,
		ErrDAGNodeFailed, ErrDAGConditionFailed, ErrDAGTriggerSkipped, ErrDAGDeadlock,
		ErrBashExecFailed, ErrBashTimeout, ErrBashExitCode,
		ErrOutputCaptureFailed, ErrOutputSchemaInvalid,
		ErrHTTPExecFailed, ErrHTTPStatusUnexpected,
		ErrEvalAssertFailed,
		ErrLoopMaxReached, ErrLoopIdleTimeout,
		ErrWaitTimeout, ErrWaitNotFound,
		// Webhook
		ErrWebhookInvalidSig, ErrWebhookParseFailed, ErrWebhookMissingEvent,
		ErrWebhookNoMatch, ErrWebhookRateLimit, ErrWebhookPayloadToo,
		ErrWebhookReplay, ErrWebhookUnsupported,
		// Store
		ErrDBOpen, ErrDBSchema, ErrDBQuery, ErrDBNotFound,
		ErrDBMigration, ErrDBConstraint, ErrDBTimeout, ErrDBCorrupt,
		// Claude
		ErrClaudeNotFound, ErrClaudeStartFailed, ErrClaudeStreamParse,
		ErrClaudeExitError, ErrClaudeBudgetExhaust, ErrClaudeTimeout,
		ErrClaudeAuthFailed, ErrClaudeRateLimit, ErrClaudeOverloaded, ErrClaudeContextLimit,
		// Config
		ErrConfigLoad, ErrConfigParse, ErrConfigMissing,
		ErrConfigInvalid, ErrConfigPermission,
		// Clone
		ErrCloneFailed, ErrClonePullFailed, ErrCloneAuthFailed,
		ErrCloneNotFound, ErrCloneDiskFull,
		// Auth
		ErrAuthRequired, ErrAuthInvalidToken, ErrAuthForbidden,
		ErrAuthSessionExpired, ErrAuthIPBlocked,
		// WebSocket
		ErrWSConnectFailed, ErrWSSubscribeDenied, ErrWSMessageTooLarge,
		ErrWSConnectionClosed, ErrWSInvalidMessage, ErrWSRateLimited,
		// API
		ErrAPIBadRequest, ErrAPINotFound, ErrAPIMethodNotAllowed,
		ErrAPIRateLimit, ErrAPIPayloadTooLarge, ErrAPIInternal,
		ErrAPIUnavailable, ErrAPIValidation,
	}
}

func TestAllCodes_Format(t *testing.T) {
	for _, code := range allCodes() {
		s := string(code)
		if !strings.HasPrefix(s, "SKY-") {
			t.Errorf("code %q doesn't start with SKY-", s)
		}
		parts := strings.Split(s, "-")
		if len(parts) != 3 {
			t.Errorf("code %q should have 3 parts (SKY-XX-NNN), got %d", s, len(parts))
		}
	}
}

func TestAllCodes_Unique(t *testing.T) {
	seen := make(map[Code]bool)
	for _, code := range allCodes() {
		if seen[code] {
			t.Errorf("duplicate code: %s", code)
		}
		seen[code] = true
	}
}

func TestAllCodes_Count(t *testing.T) {
	codes := allCodes()
	if len(codes) != 112 {
		t.Errorf("expected 112 error codes, got %d", len(codes))
	}
}
