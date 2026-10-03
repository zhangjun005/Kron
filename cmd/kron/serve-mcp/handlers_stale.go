package servemcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/store"
)

// handleStale returns the toolHandler for kron_stale.
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
// expired_assumptions: hard-severity assumptions with an ExpiresAt in
// the past and no verified_at (or verified_at before the expiry).
// Soft assumptions are intentionally excluded — the spec only counts
// "硬假设" (hard assumptions) for staleness alerts.
func handleStale(root string) toolHandler {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		if err := assertCallerMCP(ctx); err != nil {
			return nil, err
		}
		var args struct {
			DaysThreshold int `json:"days_threshold"`
		}
		if len(params) > 0 {
			if err := json.Unmarshal(params, &args); err != nil {
				return nil, fmt.Errorf("kron_stale: invalid params: %w", err)
			}
		}
		if args.DaysThreshold <= 0 {
			args.DaysThreshold = 90
		}

		r, err := store.NewReader(root)
		if err != nil {
			return nil, fmt.Errorf("kron_stale: open store: %w", err)
		}
		all, err := r.LoadAll(ctx)
		if err != nil {
			return nil, fmt.Errorf("kron_stale: load: %w", err)
		}

		now := time.Now().UTC()
		cutoff := now.AddDate(0, 0, -args.DaysThreshold)

		var superseded []string
		var expired []expiredAssumptionWire

		for _, in := range all {
			if in.Frontmatter.Status == model.StatusActive &&
				!in.Frontmatter.UpdatedAt.IsZero() &&
				in.Frontmatter.UpdatedAt.Before(cutoff) {
				superseded = append(superseded, in.Slug)
			}

			for _, a := range in.Frontmatter.Assumptions {
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
				expired = append(expired, expiredAssumptionWire{
					Slug:         in.Slug,
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
			expired = []expiredAssumptionWire{}
		}

		return staleResponse{
			SupersededCandidates: superseded,
			ExpiredAssumptions:   expired,
		}, nil
	}
}

type staleResponse struct {
	SupersededCandidates []string                `json:"superseded_candidates"`
	ExpiredAssumptions   []expiredAssumptionWire `json:"expired_assumptions"`
}

type expiredAssumptionWire struct {
	Slug         string `json:"slug"`
	AssumptionID string `json:"assumption_id"`
	ExpiresAt    string `json:"expires_at"`
	DaysOverdue  int    `json:"days_overdue"`
}
