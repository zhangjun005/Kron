package servemcp

import (
	"context"
	"fmt"
	"sort"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/parser"
	"github.com/xxx/kron/internal/store"
)

// HandleDelete returns the mcp.ToolHandlerFor binding for kron_delete.
//
// Wire contract: see docs/implementation/mcp.md §2 (kron_delete).
//
//	in:  { slug: string }
//	out: { ok: bool, trashed_path: string, dependents: [slug] }
//
// Soft-deletes by moving .kron/intents/<slug>.md to .kron/.trash/<slug>.md.
// Returns ErrIntentNotFound (→ IsError=true) if the slug is not
// present in intents/. Trash contents are NOT considered.
//
// Dependents is the list of intents that list `args.Slug` in their
// Frontmatter.DependsOn at the moment of deletion. v1 surfaces this
// for caller awareness (a soft warning) but does NOT block the
// deletion — see RFC 2026-10-03-frontmatter-references §5.3.
//
// Note: hard-delete / GC of .kron/.trash/ is intentionally out of v1
// (architecture.md §5.2.1).
func HandleDelete(ctx context.Context, _ *mcp.CallToolRequest, in DeleteInput) (
	*mcp.CallToolResult, DeleteOutput, error,
) {
	if err := assertCallerMCP(ctx); err != nil {
		return nil, DeleteOutput{}, err
	}
	if in.Slug == "" {
		return nil, DeleteOutput{}, fmt.Errorf("kron_delete requires non-empty slug")
	}
	if err := parser.ValidateSlug(in.Slug); err != nil {
		return nil, DeleteOutput{}, err
	}

	root := repoRootFrom(ctx)

	// Compute dependents BEFORE moving the file. If LoadAll fails
	// (e.g., another intent is broken) we fall through with an empty
	// dependents list rather than blocking the delete — the delete
	// itself is the user's intent; the dependents list is a courtesy
	// warning.
	dependents := []string{}
	if r, rerr := store.NewReader(root); rerr == nil {
		if all, lerr := r.LoadAll(ctx); lerr == nil {
			for _, intent := range all {
				if intent.Slug == in.Slug {
					continue
				}
				for _, dep := range intent.Frontmatter.DependsOn {
					if dep == in.Slug {
						dependents = append(dependents, intent.Slug)
						break
					}
				}
			}
			sort.Strings(dependents)
		}
	}

	w, err := store.NewWriter(root)
	if err != nil {
		return nil, DeleteOutput{}, fmt.Errorf("kron_delete: open store: %w", err)
	}
	if err := w.MoveToTrash(ctx, in.Slug); err != nil {
		return nil, DeleteOutput{}, err
	}

	return nil, DeleteOutput{
		OK:          true,
		TrashedPath: model.TrashPath(in.Slug),
		Dependents:  dependents,
	}, nil
}
