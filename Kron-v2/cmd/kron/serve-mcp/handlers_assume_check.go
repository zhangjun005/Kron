package servemcp

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/xxx/kron/internal/assumption"
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
//	    severity:       string,    // "hard" | "soft"   (per-intent)
//	    default_severity: string?, // from registry, if present
//	    text:           string,    // from registry (preferred) or inline
//	    rationale:      string?,   // from intent frontmatter (B-3)
//	    expires_at:     string?,
//	    days_remaining: int? (negative if expired),
//	    verified:       bool,
//	  }],
//	  summary: { hard: int, soft: int, expired: int }
//	}
//
// B-3 (RFC 2026-10-08-assumptions-standalone): the handler reads the
// assumption registry (.kron/assumptions/*.md) and prefers the
// registry's text/default_severity over the inline fields. The
// per-intent severity (a.Severity) always wins over the registry's
// default_severity — the rationale is what justifies the override.
func HandleAssumeCheck(ctx context.Context, _ *mcp.CallToolRequest, in AssumeCheckInput) (
	*mcp.CallToolResult, AssumeCheckOutput, error,
) {
	if err := assertCallerMCP(ctx); err != nil {
		return nil, AssumeCheckOutput{}, err
	}
	root := repoRootFrom(ctx)

	// Build the set of slugs the caller is interested in.
	var slugsOfInterest map[string]struct{}
	if in.FilePath != "" {
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

	// B-3: read the assumption registry when present. Absence is
	// not a hard error — pre-migration repos have inline-only
	// assumptions; we fall back to those.
	var ar *assumption.Reader
	if ard, ardErr := assumption.NewReader(root); ardErr == nil {
		ar = ard
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
			// Registry (when present) is the source of truth for
			// text + default_severity. Per-intent severity
			// (a.Severity) and expires_at stay on the intent.
			text := a.Text
			defaultSev := ""
			if ar != nil {
				if af, gerr := ar.Get(ctx, a.ID); gerr == nil && af != nil {
					if af.Frontmatter.Text != "" {
						text = af.Frontmatter.Text
					}
					defaultSev = string(af.Frontmatter.DefaultSeverity)
				}
			}
			w := AssumeWarning{
				IntentSlug:      intent.Slug,
				AssumptionID:    a.ID,
				Severity:        string(a.Severity),
				DefaultSeverity: defaultSev,
				Text:            text,
				Rationale:       a.Rationale,
				ExpiresAt:       a.ExpiresAt,
				Verified:        a.VerifiedAt != "",
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
