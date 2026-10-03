package servemcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/parser"
	"github.com/xxx/kron/internal/store"
)

// handleDelete returns the toolHandler for kron_delete.
//
// Wire contract: see docs/implementation/mcp.md §2 (kron_delete).
//
//	in:  { slug: string }
//	out: { ok: bool, trashed_path: string, dependents: [slug] }
//
// Soft-deletes by moving .kron/intents/<slug>.md to .kron/.trash/<slug>.md.
// Returns ErrIntentNotFound (-32004) if the slug is not present in
// intents/. Trash contents are NOT considered.
//
// Dependents is the list of intents that list `args.Slug` in their
// Frontmatter.DependsOn at the moment of deletion. v1 surfaces this
// for caller awareness (a soft warning) but does NOT block the
// deletion — see RFC 2026-10-03-frontmatter-references §5.3.
//
// Note: hard-delete / GC of .kron/.trash/ is intentionally out of v1
// (architecture.md §5.2.1).
func handleDelete(root string) toolHandler {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		if err := assertCallerMCP(ctx); err != nil {
			return nil, err
		}
		var args struct {
			Slug string `json:"slug"`
		}
		if len(params) > 0 {
			if err := json.Unmarshal(params, &args); err != nil {
				return nil, fmt.Errorf("kron_delete: invalid params: %w", err)
			}
		}
		if args.Slug == "" {
			return nil, fmt.Errorf("kron_delete requires non-empty slug")
		}
		if err := parser.ValidateSlug(args.Slug); err != nil {
			return nil, err
		}

		// Compute dependents BEFORE moving the file. We do this by
		// scanning the store for any intent whose Frontmatter.DependsOn
		// includes args.Slug. If LoadAll fails (e.g., another intent
		// is broken) we fall through with an empty dependents list
		// rather than blocking the delete — the delete itself is the
		// user's intent; the dependents list is a courtesy warning.
		// Compute dependents BEFORE moving the file. We do this by
		// scanning the store for any intent whose Frontmatter.DependsOn
		// includes args.Slug. If LoadAll fails (e.g., another intent
		// is broken) we fall through with an empty dependents list
		// rather than blocking the delete — the delete itself is the
		// user's intent; the dependents list is a courtesy warning.
		dependents := []string{}
		if r, rerr := store.NewReader(root); rerr == nil {
			if all, lerr := r.LoadAll(ctx); lerr == nil {
				for _, in := range all {
					if in.Slug == args.Slug {
						continue
					}
					for _, dep := range in.Frontmatter.DependsOn {
						if dep == args.Slug {
							dependents = append(dependents, in.Slug)
							break
						}
					}
				}
				sort.Strings(dependents)
			}
		}

		w, err := store.NewWriter(root)
		if err != nil {
			return nil, fmt.Errorf("kron_delete: open store: %w", err)
		}
		if err := w.MoveToTrash(ctx, args.Slug); err != nil {
			return nil, err
		}

		return deleteResponse{
			OK:          true,
			TrashedPath: model.TrashPath(args.Slug),
			Dependents:  dependents,
		}, nil
	}
}

type deleteResponse struct {
	OK          bool     `json:"ok"`
	TrashedPath string   `json:"trashed_path"`
	Dependents  []string `json:"dependents"`
}
