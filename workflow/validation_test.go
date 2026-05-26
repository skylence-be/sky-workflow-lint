package workflow

import (
	"strings"
	"testing"
)

// validateThinkingConfig — parser.go:437

func TestValidateThinkingConfig(t *testing.T) {
	cases := []struct {
		name    string
		cfg     ThinkingConfig
		wantErr string
	}{
		{name: "adaptive ok", cfg: ThinkingConfig{Mode: "adaptive"}},
		{name: "disabled ok", cfg: ThinkingConfig{Mode: "disabled"}},
		{name: "enabled with budget ok", cfg: ThinkingConfig{Mode: "enabled", BudgetTokens: 1000}},
		{name: "enabled no budget", cfg: ThinkingConfig{Mode: "enabled", BudgetTokens: 0}, wantErr: "budget_tokens is required"},
		{name: "invalid mode", cfg: ThinkingConfig{Mode: "quantum"}, wantErr: "thinking.mode"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateThinkingConfig("wf", "n", &tc.cfg)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tc.wantErr)
			}
		})
	}
}

// validateRetryConfig — parser.go:451 (range 1–5, delay 1000–60000 or 0)

func TestValidateRetryConfig(t *testing.T) {
	cases := []struct {
		name    string
		cfg     RetryConfig
		wantErr string
	}{
		{name: "max_attempts=1 ok", cfg: RetryConfig{MaxAttempts: 1}},
		{name: "max_attempts=5 ok", cfg: RetryConfig{MaxAttempts: 5}},
		{name: "max_attempts=0 reject", cfg: RetryConfig{MaxAttempts: 0}, wantErr: "max_attempts"},
		{name: "max_attempts=6 reject", cfg: RetryConfig{MaxAttempts: 6}, wantErr: "max_attempts"},
		{name: "delay_ms=0 ok (omitted)", cfg: RetryConfig{MaxAttempts: 1, DelayMS: 0}},
		{name: "delay_ms=1000 ok", cfg: RetryConfig{MaxAttempts: 1, DelayMS: 1000}},
		{name: "delay_ms=60000 ok", cfg: RetryConfig{MaxAttempts: 1, DelayMS: 60000}},
		{name: "delay_ms=999 reject", cfg: RetryConfig{MaxAttempts: 1, DelayMS: 999}, wantErr: "delay_ms"},
		{name: "delay_ms=60001 reject", cfg: RetryConfig{MaxAttempts: 1, DelayMS: 60001}, wantErr: "delay_ms"},
		{name: "on_error empty ok", cfg: RetryConfig{MaxAttempts: 1, OnError: ""}},
		{name: "on_error=transient ok", cfg: RetryConfig{MaxAttempts: 1, OnError: "transient"}},
		{name: "on_error=all ok", cfg: RetryConfig{MaxAttempts: 1, OnError: "all"}},
		{name: "on_error invalid", cfg: RetryConfig{MaxAttempts: 1, OnError: "garbage"}, wantErr: "on_error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateRetryConfig("wf", "n", &tc.cfg)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tc.wantErr)
			}
		})
	}
}

// validateApprovalConfig — parser.go:394

func TestValidateApprovalConfig(t *testing.T) {
	cases := []struct {
		name    string
		cfg     ApprovalConfig
		wantErr string
	}{
		{name: "channel empty ok", cfg: ApprovalConfig{Channel: ""}},
		{name: "channel manual ok", cfg: ApprovalConfig{Channel: "manual"}},
		{name: "channel webhook ok", cfg: ApprovalConfig{Channel: "webhook"}},
		{name: "channel slack reject", cfg: ApprovalConfig{Channel: "slack"}, wantErr: "channel"},
		{name: "timeout empty ok", cfg: ApprovalConfig{Timeout: ""}},
		{name: "timeout 30s ok", cfg: ApprovalConfig{Timeout: "30s"}},
		{name: "timeout invalid", cfg: ApprovalConfig{Timeout: "not-a-duration"}, wantErr: "timeout"},
		{name: "on_reject max_attempts=0 ok", cfg: ApprovalConfig{OnReject: &RejectOptions{MaxAttempts: 0}}},
		{name: "on_reject max_attempts=10 ok", cfg: ApprovalConfig{OnReject: &RejectOptions{MaxAttempts: 10}}},
		{name: "on_reject max_attempts=11 reject", cfg: ApprovalConfig{OnReject: &RejectOptions{MaxAttempts: 11}}, wantErr: "max_attempts"},
		{name: "on_reject max_attempts=-1 reject", cfg: ApprovalConfig{OnReject: &RejectOptions{MaxAttempts: -1}}, wantErr: "max_attempts"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateApprovalConfig("wf", "n", &tc.cfg)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tc.wantErr)
			}
		})
	}
}

// validateSandboxConfig — parser.go:467

// validateEmit — parser.go:517

func TestValidateEmit(t *testing.T) {
	cases := []struct {
		name    string
		emit    EmitSpec
		wantErr string
	}{
		{name: "simple name ok", emit: EmitSpec{Name: "review.posted"}},
		{name: "underscores ok", emit: EmitSpec{Name: "a_b_c"}},
		{name: "digits ok", emit: EmitSpec{Name: "a1b2"}},
		{name: "dots ok", emit: EmitSpec{Name: "team.deployed.prod"}},
		{name: "hyphens ok", emit: EmitSpec{Name: "pr-merged"}},
		{name: "with payload ok", emit: EmitSpec{Name: "pr.ready", Payload: map[string]string{"pr": "123"}}},
		{name: "empty name reject", emit: EmitSpec{Name: ""}, wantErr: "emit.name is required"},
		{name: "starts with digit reject", emit: EmitSpec{Name: "1event"}, wantErr: "emit.name"},
		{name: "uppercase reject", emit: EmitSpec{Name: "ReviewPosted"}, wantErr: "emit.name"},
		{name: "space reject", emit: EmitSpec{Name: "bad name"}, wantErr: "emit.name"},
		{name: "starts with dot reject", emit: EmitSpec{Name: ".bad"}, wantErr: "emit.name"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateEmit("wf", "node \"n\"", &tc.emit)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestValidate_WorkflowName(t *testing.T) {
	cases := []struct {
		name    string
		wfName  string
		wantErr string
	}{
		{name: "simple ok", wfName: "wf-a"},
		{name: "underscores ok", wfName: "build_release"},
		{name: "uppercase ok", wfName: "DeployProd"},
		{name: "digits and dots ok", wfName: "team.v2"},
		{name: "comma reject", wfName: "wf,evil", wantErr: "must match"},
		{name: "space reject", wfName: "wf one", wantErr: "must match"},
		{name: "starts with digit reject", wfName: "1wf", wantErr: "must match"},
		{name: "empty name reject", wfName: "", wantErr: "name is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wf := &Workflow{
				Name:  tc.wfName,
				Steps: []Step{{Name: "s", Prompt: "p"}},
			}
			err := validate(wf)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestValidateSandboxConfig(t *testing.T) {
	cases := []struct {
		name    string
		allow   []string
		wantErr string
	}{
		{name: "relative path ok", allow: []string{"./src"}},
		{name: "multiple relative ok", allow: []string{"./src", "internal/pkg"}},
		{name: "empty allow ok", allow: []string{}},
		{name: "absolute path reject", allow: []string{"/etc"}, wantErr: "relative path"},
		{name: "dotdot escape reject", allow: []string{"../sibling"}, wantErr: "relative path"},
		{name: "empty string reject", allow: []string{""}, wantErr: "relative path"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &SandboxConfig{Filesystem: FilesystemConfig{Allow: tc.allow}}
			err := validateSandboxConfig("wf", "n", cfg)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tc.wantErr)
			}
		})
	}
}
