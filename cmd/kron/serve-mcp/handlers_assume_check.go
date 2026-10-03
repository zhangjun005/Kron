package servemcp

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/parser"
	"github.com/xxx/kron/internal/store"
)

// handleAssumeCheck returns the toolHandler for kron_assume_check.
//
// Wire contract: see docs/implementation/mcp.md §2 (kron_assume_check) +
// 2026-10-03 self-validate decision.
//
//	in:  { file_path?: string }
//	out: {
//	  warnings: [{
//	    intent_slug:    string,
//	    assumption_id:  string,
//	    severity:       string,    // "hard" | "soft"
//	    text:           string,
//	    expires_at:     string?,
//	    days_remaining: int? (negative if expired),
//	    verified:       bool,
//	  }],
//	  summary: { hard: int, soft: int, expired: int }
//	}
//
// Signal source: SELF-VALIDATE — we read each intent's assumptions[] from
// .kron/intents/*.md and check ExpiresAt < now + VerifiedAt presence.
// We do NOT integrate with any external test signal in v1.1 (deliberate
// decision 2026-10-03; integration deferred to v1.2+).
//
// Filtering:
//   - file_path omitted: every intent's assumptions are inspected.
//   - file_path provided: only assumptions attached to intents that
//     have an @kron:intent <slug> anchor in that file are inspected.
//     (Slugs are reversed via parser.ScanAnchors; absent slug = empty result.)
//
// "hard" severity assumptions that are past ExpiresAt AND unverified
// contribute to summary.expired; soft assumptions with same shape
// contribute to warnings but not expired.
func handleAssumeCheck(root string) toolHandler {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		if err := assertCallerMCP(ctx); err != nil {
			return nil, err
		}
		var args struct {
			FilePath string `json:"file_path"`
		}
		if len(params) > 0 {
			if err := json.Unmarshal(params, &args); err != nil {
				return nil, fmt.Errorf("kron_assume_check: invalid params: %w", err)
			}
		}

		// Build the set of slugs the caller is interested in.
		// ScanAnchors walks a directory recursively, so for a single
		// file_path we walk the parent dir and filter. The skip-list
		// in ScanAnchors matches our access layer's defaults, so this
		// is cheap for source trees <10k files.
		var slugsOfInterest map[string]struct{}
		if args.FilePath != "" {
			dir := filepath.Dir(args.FilePath)
			anchors, err := parser.ScanAnchors(dir)
			if err != nil {
				return nil, fmt.Errorf("kron_assume_check: scan %s: %w", dir, err)
			}
			abs, _ := filepath.Abs(args.FilePath)
			slugsOfInterest = make(map[string]struct{})
			for _, a := range anchors {
				if a.FilePath == abs || a.FilePath == args.FilePath {
					slugsOfInterest[a.Slug] = struct{}{}
				}
			}
		}

		r, err := store.NewReader(root)
		if err != nil {
			return nil, fmt.Errorf("kron_assume_check: open store: %w", err)
		}
		all, err := r.LoadAll(ctx)
		if err != nil {
			return nil, fmt.Errorf("kron_assume_check: load: %w", err)
		}

		now := time.Now().UTC()
		var warnings []assumeWarningWire
		summary := assumeSummary{}

		for _, in := range all {
			if slugsOfInterest != nil {
				if _, ok := slugsOfInterest[in.Slug]; !ok {
					continue
				}
			}
			for _, a := range in.Frontmatter.Assumptions {
				w := assumeWarningWire{
					IntentSlug:   in.Slug,
					AssumptionID: a.ID,
					Severity:     string(a.Severity),
					Text:         a.Text,
					ExpiresAt:    a.ExpiresAt,
					Verified:     a.VerifiedAt != "",
				}
				if a.ExpiresAt != "" {
					if exp, err := time.Parse(time.RFC3339, a.ExpiresAt); err == nil {
						days := int(exp.Sub(now).Hours() / 24)
						w.DaysRemaining = &days
						if days < 0 {
							if a.Severity == model.SeverityHard {
								summary.Expired++
							}
						}
					}
				}
				warnings = append(warnings, w)
				if a.Severity == model.SeverityHard {
					summary.Hard++
				} else {
					summary.Soft++
				}
			}
		}

		if warnings == nil {
			warnings = []assumeWarningWire{}
		}
		return assumeCheckResponse{Warnings: warnings, Summary: summary}, nil
	}
}

type assumeCheckResponse struct {
	Warnings []assumeWarningWire `json:"warnings"`
	Summary  assumeSummary       `json:"summary"`
}

type assumeWarningWire struct {
	IntentSlug    string `json:"intent_slug"`
	AssumptionID  string `json:"assumption_id"`
	Severity      string `json:"severity"`
	Text          string `json:"text"`
	ExpiresAt     string `json:"expires_at,omitempty"`
	DaysRemaining *int   `json:"days_remaining,omitempty"`
	Verified      bool   `json:"verified"`
}

type assumeSummary struct {
	Hard    int `json:"hard"`
	Soft    int `json:"soft"`
	Expired int `json:"expired"`
}
