package servemcp

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/parser"
	"github.com/xxx/kron/internal/store"
)

// HandleAssumeCheck returns the mcp.ToolHandlerFor binding for
// kron_assume_check.
//
// Wire contract: see docs/implementation/mcp.md §2 (kron_assume_check)
// + 2026-10-03 self-validate decision.
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
// Signal source: SELF-VALIDATE — we read each intent's assumptions[]
// from .kron/intents/*.md and check ExpiresAt < now + VerifiedAt
// presence. We do NOT integrate with any external test signal in
// v1.1+ (deliberate decision 2026-10-03; integration deferred to v1.2+).
//
// Filtering:
//   - file_path omitted: every intent's assumptions are inspected.
//   - file_path provided: only assumptions attached to intents that
//     have an @kron:intent <slug> anchor in that file are inspected.
//
// "hard" severity assumptions that are past ExpiresAt AND unverified
// contribute to summary.expired; soft assumptions with same shape
// contribute to warnings but not expired.
func HandleAssumeCheck(ctx context.Context, _ *mcp.CallToolRequest, in AssumeCheckInput) (
	*mcp.CallToolResult, AssumeCheckOutput, error,
) {
	if err := assertCallerMCP(ctx); err != nil {
		return nil, AssumeCheckOutput{}, err
	}
	root := repoRootFrom(ctx)

	// Build the set of slugs the caller is interested in.
	// ScanAnchors walks a directory recursively, so for a single
	// file_path we walk the parent dir and filter. The skip-list
	// in ScanAnchors matches our access layer's defaults, so this
	// is cheap for source trees <10k files.
	var slugsOfInterest map[string]struct{}
	if in.FilePath != "" {
		// parser.SlugsForFile opens the file directly and returns
		// the set of intent slugs that file anchors (sorted,
		// deduplicated). It replaces the prior "walk the parent
		// directory, filter by FilePath" hack which was both slow
		// (unnecessary walk) and platform-fragile (path-separator
		// comparisons).
		slugs, err := parser.SlugsForFile(in.FilePath)
		if err != nil {
			return nil, AssumeCheckOutput{}, fmt.Errorf("kron_assume_check: scan %s: %w", in.FilePath, err)
		}
		slugsOfInterest = make(map[string]struct{}, len(slugs))
		for _, s := range slugs {
			slugsOfInterest[s] = struct{}{}
		}
	}

	r, err := store.NewReader(root)
	if err != nil {
		return nil, AssumeCheckOutput{}, fmt.Errorf("kron_assume_check: open store: %w", err)
	}
	all, err := r.LoadAll(ctx)
	if err != nil {
		return nil, AssumeCheckOutput{}, fmt.Errorf("kron_assume_check: load: %w", err)
	}

	now := time.Now().UTC()
	var warnings []AssumeWarning
	summary := AssumeCheckSummary{}

	for _, intent := range all {
		if slugsOfInterest != nil {
			if _, ok := slugsOfInterest[intent.Slug]; !ok {
				continue
			}
		}
		for _, a := range intent.Frontmatter.Assumptions {
			w := AssumeWarning{
				IntentSlug:   intent.Slug,
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
					if days < 0 && a.Severity == model.SeverityHard {
						summary.Expired++
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
		warnings = []AssumeWarning{}
	}
	return nil, AssumeCheckOutput{Warnings: warnings, Summary: summary}, nil
}
