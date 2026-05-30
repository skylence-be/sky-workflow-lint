package workflow

import (
	"fmt"
	"strings"
	"time"

	"github.com/skylence-be/skyerr"
)

func validateSpawnConfig(wfName, nodeID string, cfg *SpawnConfig) error {
	if len(cfg.Workers) == 0 {
		return skyerr.New(skyerr.ErrSpawnWorkersEmpty,
			fmt.Sprintf("workflow %q: node %q: spawn.workers is empty", wfName, nodeID))
	}
	for i, w := range cfg.Workers {
		if w.ID == "" {
			return skyerr.New(skyerr.ErrSpawnWorkerIDEmpty,
				fmt.Sprintf("workflow %q: node %q: spawn.workers[%d].id is empty", wfName, nodeID, i))
		}
		if w.Prompt == "" {
			return skyerr.New(skyerr.ErrSpawnWorkerPromptEmpty,
				fmt.Sprintf("workflow %q: node %q: spawn.workers[%d] (id=%q) has empty prompt", wfName, nodeID, i, w.ID))
		}
	}
	if cfg.MaxWait != "" {
		if _, err := time.ParseDuration(cfg.MaxWait); err != nil {
			return skyerr.New(skyerr.ErrSpawnMaxWaitInvalid,
				fmt.Sprintf("workflow %q: node %q: spawn.max_wait %q is not a valid duration", wfName, nodeID, cfg.MaxWait))
		}
	}
	if cfg.OnIdle != "" && cfg.OnIdle != "any" && cfg.OnIdle != "all" {
		return skyerr.New(skyerr.ErrSpawnOnIdleInvalid,
			fmt.Sprintf("workflow %q: node %q: spawn.on_idle %q must be \"any\" or \"all\"", wfName, nodeID, cfg.OnIdle))
	}
	if cfg.Boundary != nil {
		if err := validateBoundaryConfig(wfName, nodeID, cfg.Boundary); err != nil {
			return err
		}
	}
	return nil
}

func validateBoundaryConfig(wfName, nodeID string, b *BoundaryConfig) error {
	if b.ReadOnly && (len(b.Own) > 0 || len(b.MustNotEdit) > 0) {
		return skyerr.New(skyerr.ErrSpawnBoundaryContradicts,
			fmt.Sprintf("workflow %q: node %q: spawn.boundary.read_only is contradictory with own or must_not_edit", wfName, nodeID))
	}
	for _, g := range append(b.Own, b.MustNotEdit...) {
		if strings.Contains(g, "**") {
			return skyerr.New(skyerr.ErrSpawnBoundaryGlobInvalid,
				fmt.Sprintf("workflow %q: node %q: spawn.boundary glob %q contains ** (unsupported; use single-segment wildcards)", wfName, nodeID, g))
		}
	}
	return nil
}

func validateReviewConfig(wfName, nodeID string, cfg *ReviewConfig) error {
	for i, p := range cfg.Paths {
		if p == "" {
			return skyerr.New(skyerr.ErrReviewPathsInvalid,
				fmt.Sprintf("workflow %q: node %q: review.paths[%d] is empty", wfName, nodeID, i))
		}
	}
	return nil
}

func validateCouncilConfig(wfName, nodeID string, cfg *CouncilConfig) error {
	if len(cfg.Members) == 0 {
		return skyerr.New(skyerr.ErrCouncilMembersEmpty,
			fmt.Sprintf("workflow %q: node %q: council.members is empty", wfName, nodeID))
	}
	for i, m := range cfg.Members {
		if m.ID == "" {
			return skyerr.New(skyerr.ErrCouncilMemberInvalid,
				fmt.Sprintf("workflow %q: node %q: council.members[%d].id is empty", wfName, nodeID, i))
		}
		if m.Prompt == "" {
			return skyerr.New(skyerr.ErrCouncilMemberInvalid,
				fmt.Sprintf("workflow %q: node %q: council.members[%d] (id=%q) has empty prompt", wfName, nodeID, i, m.ID))
		}
	}
	if cfg.Synthesis == "" {
		return skyerr.New(skyerr.ErrCouncilSynthesisEmpty,
			fmt.Sprintf("workflow %q: node %q: council.synthesis is empty", wfName, nodeID))
	}
	if cfg.MaxWait != "" {
		if _, err := time.ParseDuration(cfg.MaxWait); err != nil {
			return skyerr.New(skyerr.ErrCouncilMaxWaitInvalid,
				fmt.Sprintf("workflow %q: node %q: council.max_wait %q is not a valid duration", wfName, nodeID, cfg.MaxWait))
		}
	}
	if cfg.MaxBudgetUSD < 0 {
		return skyerr.New(skyerr.ErrCouncilBudgetNegative,
			fmt.Sprintf("workflow %q: node %q: council.max_budget_usd must be >= 0 (0 = disabled)", wfName, nodeID))
	}
	return nil
}
