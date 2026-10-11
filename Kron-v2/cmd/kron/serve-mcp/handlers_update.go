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

// HandleUpdate returns the mcp.ToolHandlerFor binding for kron_update.
//
// Wire contract: see docs/implementation/mcp.md §2 (kron_update) +
// 2026-10-03 PATCH-semantics decision.
//
//	in:  {
//	  slug:     string (required),
//	  symbol:   []string? (null/absent = no change; [] = clear;
//	                     non-empty = replace),
//	  status:   *string? (null/absent = no change; pointer = replace with value),
//	  reviewers: []string? (null/absent = no change; [] = clear;
//	                       non-nil = replace),
//	  body:     *string? (null/absent = no change; pointer = replace),
//	  references: []string? (same as reviewers),
//	  depends_on: []string? (same as reviewers),
//	}
//
//	out: { ok: bool, path: string }
//
// PATCH semantics: only the supplied fields are touched. The v1.1
// `patchString` / `patchStringList` custom unmarshalers are gone in
// v1.2; pointer-vs-slice distinction is now expressed in the typed
// In struct (see UpdateInput): nil = absent; non-nil = replace with
// the supplied value (nil slice = clear).
//
// Returns ErrIntentNotFound (→ IsError=true) if slug does not exist.
// Self-reference (A.References contains A or A.DependsOn contains A)
// is rejected with ErrFrontmatterInvalid (→ IsError=true) before write.
func HandleUpdate(ctx context.Context, _ *mcp.CallToolRequest, in UpdateInput) (
	*mcp.CallToolResult, UpdateOutput, error,
) {
	if err := assertCallerMCP(ctx); err != nil {
		return nil, UpdateOutput{}, err
	}
	if in.Slug == "" {
		return nil, UpdateOutput{}, fmt.Errorf("%w: kron_update requires non-empty slug", model.ErrSlugInvalid)
	}
	if err := parser.ValidateSlug(in.Slug); err != nil {
		return nil, UpdateOutput{}, err
	}

	root := repoRootFrom(ctx)
	r, err := store.NewReader(root)
	if err != nil {
		return nil, UpdateOutput{}, fmt.Errorf("kron_update: open store: %w", err)
	}
	intent, err := r.Load(ctx, in.Slug)
	if err != nil {
		return nil, UpdateOutput{}, err
	}

	// Apply patch: each non-nil field replaces the existing value
	// (nil slice = clear).
	if in.Symbol != nil {
		intent.Frontmatter.Symbol = in.Symbol
	}
	if in.Status != nil {
		intent.Frontmatter.Status = model.Status(*in.Status)
	}
	if in.Reviewers != nil {
		intent.Frontmatter.Reviewers = in.Reviewers
	}
	if in.Body != nil {
		intent.Body = *in.Body
	}
	if in.References != nil {
		intent.Frontmatter.References = parser.DedupStrings(in.References)
	}
	if in.DependsOn != nil {
		intent.Frontmatter.DependsOn = parser.DedupStrings(in.DependsOn)
	}

	// Always bump updated_at on any write so downstream observers
	// can detect the change. Match CLI's behaviour of always
	// refreshing the timestamp on Write.
	intent.Frontmatter.UpdatedAt = time.Now().UTC()

	// Re-validate the patched frontmatter BEFORE persisting so a
	// malformed update (e.g. invalid status enum) does not corrupt
	// the on-disk file.
	if err := validateFrontmatterShape(in.Slug, &intent.Frontmatter); err != nil {
		return nil, UpdateOutput{}, fmt.Errorf("%w: %v", model.ErrFrontmatterInvalid, err)
	}

	w, err := store.NewWriter(root)
	if err != nil {
		return nil, UpdateOutput{}, fmt.Errorf("kron_update: open writer: %w", err)
	}
	if err := w.Write(ctx, intent); err != nil {
		return nil, UpdateOutput{}, fmt.Errorf("kron_update: write: %w", err)
	}

	return nil, UpdateOutput{OK: true, Path: model.IntentPath(in.Slug)}, nil
}

// validateFrontmatterShape re-runs parser.ValidateFrontmatter on the
// patched Frontmatter so a malformed update (invalid status enum,
// missing required field, etc.) is rejected BEFORE the on-disk file
// is overwritten. Self-reference (an intent listing its own slug in
// References or DependsOn) is not caught by the YAML round-trip —
// that check needs the owner slug, which we add here.
func validateFrontmatterShape(slug string, fm *model.Frontmatter) error {
	if err := parser.ValidateFrontmatter(fm); err != nil {
		return fmt.Errorf("%w: %v", model.ErrFrontmatterInvalid, err)
	}
	for _, r := range fm.References {
		if r == slug {
			return fmt.Errorf("references contains self (%q)", r)
		}
	}
	for _, d := range fm.DependsOn {
		if d == slug {
			return fmt.Errorf("depends_on contains self (%q)", d)
		}
	}
	return nil
}
