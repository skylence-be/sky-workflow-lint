package skyerr_test

import (
	"testing"

	"github.com/skylence-be/sky-workflow-lint/skyerr"
)

func TestLintCodesCoverAllWFCodes(t *testing.T) {
	// Build a set from LintCodes.
	covered := make(map[string]bool, len(skyerr.LintCodes))
	for _, lc := range skyerr.LintCodes {
		covered[lc.Code] = true
	}

	// Every SKY-WF-* constant must appear.
	wfCodes := []string{
		string(skyerr.ErrWorkflowParse),
		string(skyerr.ErrWorkflowValidation),
		string(skyerr.ErrWorkflowNotFound),
		string(skyerr.ErrWorkflowLoadFailed),
		string(skyerr.ErrWorkflowCycle),
		string(skyerr.ErrWorkflowEmpty),
		string(skyerr.ErrSkyFormat),
		string(skyerr.ErrCommandNotFound),
		string(skyerr.ErrCommandParseFailed),
		string(skyerr.ErrCommandEmpty),
		string(skyerr.ErrDuplicateNodeID),
		string(skyerr.ErrUnknownDependency),
		string(skyerr.ErrNodeMissingAction),
		string(skyerr.ErrNodeSelfDependency),
		string(skyerr.ErrNodeAmbiguousAction),
		string(skyerr.ErrOutputRefInvalid),
		string(skyerr.ErrConditionParse),
		string(skyerr.ErrTriggerRuleInvalid),
		string(skyerr.ErrOutputStyleInvalid),
		string(skyerr.ErrHTTPURLRequired),
		string(skyerr.ErrEvalSourceRequired),
		string(skyerr.ErrEvalAssertionRequired),
		string(skyerr.ErrLoopBodyInvalid),
		string(skyerr.ErrLoopUntilInvalid),
		string(skyerr.ErrWaitChannelInvalid),
		string(skyerr.ErrWaitTimeoutInvalid),
		string(skyerr.ErrChainFromInvalid),
		string(skyerr.ErrChainFromNotInDeps),
		string(skyerr.ErrChainFromTargetInvalid),
		string(skyerr.ErrWorkflowTooLarge),
		string(skyerr.ErrWorkflowSchema),
		string(skyerr.ErrLoopMaxExceeded),
		string(skyerr.ErrTemplateUnsafeSplice),
		string(skyerr.ErrBashTrustUnsafe),
		string(skyerr.ErrMCPServerInvalid),
		string(skyerr.ErrMCPServerCollision),
		string(skyerr.ErrIsolationInvalid),
		string(skyerr.ErrScriptRuntimeInvalid),
		string(skyerr.ErrScriptRuntimeMissing),
		string(skyerr.ErrSandboxPathInvalid),
		string(skyerr.ErrSecretUndeclared),
		string(skyerr.ErrSecretEmptyName),
		string(skyerr.ErrSecretOutOfScope),
		string(skyerr.ErrLoopIdleTimeoutInvalid),
		string(skyerr.ErrInteractiveStrictConflict),
		string(skyerr.ErrOutputFormatSchemaInvalid),
		string(skyerr.ErrPromptUltraKeywordNoOp),
		string(skyerr.ErrLearningsConfig),
		string(skyerr.ErrSafetyClassMissing),
		string(skyerr.ErrReviewPathsInvalid),
		string(skyerr.ErrScheduleCronRequired),
		string(skyerr.ErrScheduleCronInvalid),
		string(skyerr.ErrScheduleTimezoneInvalid),
	}

	for _, code := range wfCodes {
		if !covered[code] {
			t.Errorf("LintCodes missing entry for %s", code)
		}
	}
}
