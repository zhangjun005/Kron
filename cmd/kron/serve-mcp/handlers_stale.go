package servemcp

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/xxx/kron/internal/model"
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
// superseded_candidates: intents whose status is "active" but whose
// updated_at is older than days_threshold days (i.e. they look stale
// and may be worth superseding).
//
// expired_assumptions: hard-severity assumptions with an ExpiresAt
// in the past and no verified_at (or verified_at before the expiry).
// Soft assumptions are intentionally excluded — the spec only counts
// "硬假设" (hard assumptions) for staleness alerts.
func HandleStale(ctx context.Context, _ *mcp.CallToolRequest, in StaleInput) (
	*mcp.CallToolResult, StaleOutput, error,
) {
	if err := assertCallerMCP(ctx); err != nil {
		return nil, StaleOutput{}, err
	}
	root := repoRootFrom(ctx)

	if in.DaysThreshold <= 0 {
		in.DaysThreshold = 90
	}

	r, err := store.NewReader(root)
	if err != nil {
		return nil, StaleOutput{}, fmt.Errorf("kron_stale: open store: %w", err)
	}
	all, err := r.LoadAll(ctx)
	if err != nil {
		return nil, StaleOutput{}, fmt.Errorf("kron_stale: load: %w", err)
	}

	now := time.Now().UTC()
	cutoff := now.AddDate(0, 0, -in.DaysThreshold)

	var superseded []string
	var expired []ExpiredAssumption

	for _, intent := range all {
		if intent.Frontmatter.Status == model.StatusActive &&
			!intent.Frontmatter.UpdatedAt.IsZero() &&
			intent.Frontmatter.UpdatedAt.Before(cutoff) {
			superseded = append(superseded, intent.Slug)
		}

		for _, a := range intent.Frontmatter.Assumptions {
			if a.Severity != model.SeverityHard || a.ExpiresAt == "" {
				continue
			}
			exp, err := time.Parse(time.RFC3339, a.ExpiresAt)
			if err != nil || !exp.Before(now) {
				continue
			}
			// Verified after expiry? Then not stale.
			if a.VerifiedAt != "" {
				if v, err := time.Parse(time.RFC3339, a.VerifiedAt); err == nil && v.After(exp) {
					continue
				}
			}
			daysOverdue := int(now.Sub(exp).Hours() / 24)
			expired = append(expired, ExpiredAssumption{
				Slug:         intent.Slug,
				AssumptionID: a.ID,
				ExpiresAt:    a.ExpiresAt,
				DaysOverdue:  daysOverdue,
			})
		}
	}

	if superseded == nil {
		superseded = []string{}
	}
	if expired == nil {
		expired = []ExpiredAssumption{}
	}

	return nil, StaleOutput{
		SupersededCandidates: superseded,
		ExpiredAssumptions:   expired,
	}, nil
}
