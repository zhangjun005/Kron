package servemcp

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gopkg.in/yaml.v3"

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
		intent.Frontmatter.References = dedupStrings(in.References)
	}
	if in.DependsOn != nil {
		intent.Frontmatter.DependsOn = dedupStrings(in.DependsOn)
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

// dedupStrings returns a slice with duplicate entries removed,
// preserving the order of first appearance. nil / empty inputs
// pass through unchanged.
//
// This mirrors RFC 2026-10-03-frontmatter-references §3.2: "重复由
// YAML 解析层自动忽略". We apply the same rule to the patch layer so
// `kron_update {"references": ["a","b","a"]}` doesn't write back a
// list that disagrees with what YAML round-trip would yield.
//
// Reference equality is exact-string; we deliberately do not trim or
// normalise whitespace because parser.ValidateSlug already rejects
// entries that wouldn't round-trip cleanly through `ValidateSlug`.
func dedupStrings(in []string) []string {
	if len(in) < 2 {
		return in
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// validateFrontmatterShape round-trips fm through parser's YAML
// validator to enforce the same schema rules that parser.ParseFrontmatter
// applies on load. By doing it after the patch we catch enum violations
// (status, assumption severity) before they reach the on-disk file.
func validateFrontmatterShape(slug string, fm *model.Frontmatter) error {
	yml, err := yaml.Marshal(fm)
	if err != nil {
		return fmt.Errorf("yaml marshal: %w", err)
	}
	if _, err := parser.ParseFrontmatter(yml); err != nil {
		return err
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
