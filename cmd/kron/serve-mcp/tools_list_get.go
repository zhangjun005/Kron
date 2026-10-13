package servemcp

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/xxx/kron/internal/assumption"
	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/relations"
	"github.com/xxx/kron/internal/store"
)

// HandleList returns the mcp.ToolHandlerFor binding for kron_list.
//
// Wire contract: see docs/implementation/mcp.md §2 (kron_list).
//
//	in:  { prefix?: string }
//	out: { intents: [{slug, symbol, status, updated_at}] }
//
// The optional "prefix" parameter filters the result to slugs that
// start with the given string. The match is case-sensitive and uses
// the raw slug form (e.g. prefix "auth" matches "auth/jwt" but not
// "Auth/foo"). An empty prefix matches all intents.
func HandleList(ctx context.Context, _ *mcp.CallToolRequest, in ListInput) (
	*mcp.CallToolResult, ListOutput, error,
) {
	if err := assertCallerMCP(ctx); err != nil {
		return nil, ListOutput{}, err
	}
	root := repoRootFrom(ctx)

	all, err := LoadAllCached(ctx, root)
	if err != nil {
		return nil, ListOutput{}, fmt.Errorf("kron_list: load: %w", err)
	}

	// B-3: optional assumption registry reader. Pre-migration
	// repos won't have it; assumesToWire falls back to inline
	// fields.
	var ar *assumption.Reader
	if ard, ardErr := assumption.NewReader(root); ardErr == nil {
		ar = ard
	}

	out := ListOutput{Intents: make([]IntentSummary, 0, len(all))}
	for _, intent := range all {
		if in.Prefix != "" && !strings.HasPrefix(intent.Slug, in.Prefix) {
			continue
		}
		out.Intents = append(out.Intents, intentSummaryFromIntent(intent, ar))
	}
	return nil, out, nil
}

// HandleGet returns the mcp.ToolHandlerFor binding for kron_get.
//
// Wire contract: see docs/implementation/mcp.md §2 (kron_get).
//
//	in:  { slug: string }
//	out: { intent: {slug, frontmatter, body, source_path} }
//
// Returns ErrIntentNotFound (mapped to CallToolResult.IsError=true by
// the SDK) if the slug does not exist. The slug is validated as
// part of the load path (store.Load returns ErrSlugInvalid for
// empty input), so callers do not need to pre-validate.
func HandleGet(ctx context.Context, _ *mcp.CallToolRequest, in GetInput) (
	*mcp.CallToolResult, GetOutput, error,
) {
	if err := assertCallerMCP(ctx); err != nil {
		return nil, GetOutput{}, err
	}
	if in.Slug == "" {
		return nil, GetOutput{}, fmt.Errorf("%w: kron_get requires non-empty slug", model.ErrSlugInvalid)
	}
	root := repoRootFrom(ctx)

	r, err := store.NewReader(root)
	if err != nil {
		return nil, GetOutput{}, fmt.Errorf("kron_get: open store: %w", err)
	}
	intent, err := r.Load(ctx, in.Slug)
	if err != nil {
		return nil, GetOutput{}, err
	}
	var ar *assumption.Reader
	if ard, ardErr := assumption.NewReader(root); ardErr == nil {
		ar = ard
	}
	body := intentFromModel(intent, ar)
	if in.IncludeRelations {
		// Compute reverse-link + prerequisite views from a fresh
		// LoadAll. We use LoadAllCached (singleflight + mtime
		// invalidation) so the views are consistent with the current
		// on-disk state while remaining fast for repeated calls.
		all, loadErr := LoadAllCached(ctx, root)
		if loadErr == nil {
			refs, deps, syms := relations.ReverseLinks(all, in.Slug)
			body.Relations = &GetRelations{
				References:          refs,
				DependsOnDependents: deps,
				SymbolInferred:      syms,
				Prerequisites:       relations.Prerequisites(all, in.Slug),
				// C-7: union of hard + symbol-inferred dependents,
				// deduped + sorted. Mirrors kron_delete's
				// `dependents` field (which is hard-only) but
				// adds the inferred-soft half so GUI/IDE
				// detail panels can render a single "who
				// depends on me" list without client-side
				// merging.
				Dependents: mergeDependents(deps, syms),
			}
		}
	}
	return nil, GetOutput{Intent: body}, nil
}

// intentSummaryFromIntent converts an internal Intent to the wire
// summary shape (slug + key frontmatter fields). Used by kron_list
// and kron_impact. Matches docs/implementation/mcp.md §2.
//
// ar is the assumption registry reader; it is consulted for each
// assumption's text + default_severity. Pass nil for pre-migration
// callers (the inline fields are used as-is).
func intentSummaryFromIntent(in *model.Intent, ar *assumption.Reader) IntentSummary {
	return IntentSummary{
		Slug:      in.Slug,
		Symbol:    in.Frontmatter.Symbol,
		Status:    string(in.Frontmatter.Status),
		UpdatedAt: in.Frontmatter.UpdatedAt.Format(time.RFC3339),
		Assumes:   assumesToWire(context.Background(), in.Frontmatter.Assumptions, ar),
	}
}

// mergeDependents computes the union of hard dependents (depends_on)
// and symbol-inferred dependents, deduplicated and sorted. Used by
// C-7 to surface a single "who depends on me" view in kron_get's
// relations block (matching kron_delete's top-level `dependents`
// field, which is hard-only — this includes both forms).
//
// Nil/empty inputs are handled: returns nil if both inputs are empty
// (matching the `omitempty` json tag on GetRelations.Dependents).
func mergeDependents(hard, inferred []string) []string {
	if len(hard) == 0 && len(inferred) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(hard)+len(inferred))
	out := make([]string, 0, len(hard)+len(inferred))
	for _, s := range hard {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	for _, s := range inferred {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func intentFromModel(in *model.Intent, ar *assumption.Reader) IntentBody {
	return IntentBody{
		Slug: in.Slug,
		Frontmatter: IntentFrontmatter{
			Symbol:      in.Frontmatter.Symbol,
			CreatedBy:   in.Frontmatter.CreatedBy,
			UpdatedAt:   in.Frontmatter.UpdatedAt.Format(time.RFC3339),
			Reviewers:   in.Frontmatter.Reviewers,
			Status:      string(in.Frontmatter.Status),
			Assumptions: assumesToWire(context.Background(), in.Frontmatter.Assumptions, ar),
			References:  in.Frontmatter.References,
			DependsOn:   in.Frontmatter.DependsOn,
		},
		Body:       in.Body,
		SourcePath: in.SourcePath,
	}
}

// assumesToWire projects a []model.Assumption to the wire shape,
// reading the assumption registry for the canonical text and
// default_severity.
//
// B-3 (RFC 2026-10-08-assumptions-standalone) wire shape:
//
//	id                string  (from inline)
//	text              string  (registry only; empty if registry absent)
//	severity          string  (per-intent; ground truth)
//	default_severity  string  (registry, informational)
//	rationale         string  (per-intent, B-3)
//	expires_at        string  (inline)
//	verified_at       string  (inline)
//	verified_by       string  (inline)
//
// RFC 2026-10-12-close-b3-softening §2.2: there is no per-intent
// Text field any more. If ar is nil (pre-migration repo) or the
// registry has no entry for a.ID, entry.Text is left empty — callers
// are expected to surface this as a missing-registry-entry error
// (lint RuleAssumptionRegistryIdMismatch handles that path).
func assumesToWire(ctx context.Context, as []model.Assumption, ar *assumption.Reader) []AssumeEntry {
	if len(as) == 0 {
		return nil
	}
	out := make([]AssumeEntry, 0, len(as))
	for _, a := range as {
		entry := AssumeEntry{
			ID:         a.ID,
			Severity:   string(a.Severity),
			Rationale:  a.Rationale,
			ExpiresAt:  a.ExpiresAt,
			VerifiedAt: a.VerifiedAt,
			VerifiedBy: a.VerifiedBy,
		}
		if ar != nil && a.ID != "" {
			if af, err := ar.Get(ctx, a.ID); err == nil && af != nil {
				entry.Text = af.Frontmatter.Text
				entry.DefaultSeverity = string(af.Frontmatter.DefaultSeverity)
			}
		}
		out = append(out, entry)
	}
	return out
}
