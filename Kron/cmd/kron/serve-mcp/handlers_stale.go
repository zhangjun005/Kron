package servemcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/xxx/kron/internal/lint"
	"github.com/xxx/kron/internal/store"
)

// HandleStale returns the mcp.ToolHandlerFor binding for kron_stale.
//
// Wire contract: see docs/implementation/mcp.md §2 (kron_stale).
//
//	in:  { days_threshold?: int (default 90) }
//	out: {
//	  superseded_candidates: [slug],
//	  expired_assumptions:   [{slug, assumption_id, expires_at, days_overdue}]
//	}
//
// superseded_candidates: active intents whose UpdatedAt is older than
// days_threshold days. Same definition as lint.RuleStaleSupersededCandidate.
//
// expired_assumptions: hard-severity assumptions with ExpiresAt in the
// past and no verified_at (or verified_at before the expiry). Same
// definition as lint.RuleExpiredHardAssumption.
//
// Implementation is a thin projection over internal/lint.ComputeStaleReport:
// the staleness rules live in internal/lint so `kron lint` and
// `kron_stale` produce the same view from the same walk.
func HandleStale(ctx context.Context, _ *mcp.CallToolRequest, in StaleInput) (
	*mcp.CallToolResult, StaleOutput, error,
) {
	if err := assertCallerMCP(ctx); err != nil {
		return nil, StaleOutput{}, err
	}
	root := repoRootFrom(ctx)

	threshold := in.DaysThreshold
	if threshold <= 0 {
		threshold = lint.StaleDaysDefault
	}

	r, err := store.NewReader(root)
	if err != nil {
		return nil, StaleOutput{}, fmt.Errorf("kron_stale: open store: %w", err)
	}
	all, err := r.LoadAll(ctx)
	if err != nil {
		return nil, StaleOutput{}, fmt.Errorf("kron_stale: load: %w", err)
	}

	report := lint.ComputeStaleReport(all, lint.RunOptions{
		StaleDaysThreshold: threshold,
	})

	superseded := report.SupersededCandidates
	if superseded == nil {
		superseded = []string{}
	}
	expired := report.ExpiredAssumptions
	if expired == nil {
		expired = []lint.ExpiredAssumption{}
	}

	return nil, StaleOutput{
		SupersededCandidates: superseded,
		ExpiredAssumptions:   expired,
	}, nil
}
