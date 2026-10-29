package servemcp

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/parser"
	"github.com/xxx/kron/internal/relations"
	"github.com/xxx/kron/internal/store"
)

// HandleDelete returns the mcp.ToolHandlerFor binding for kron_delete.
//
// Wire contract: see docs/implementation/2026-10-29-trash-subtree.md.
//
//	in:  { slug: string, confirm_subtree?: bool }
//	out: { ok: bool, trashed_path: string, dependents: [slug], deleted_slugs: [slug] }
//
// Soft-deletes by moving the entry into .kron/.trash/. The on-disk shape is
// preserved: a leaf goes to <trash>/<slug>.md, a node's whole directory (the
// README plus every descendant intent) goes to <trash>/<slug>/. Deleting a
// node therefore removes its subtree from the live tree, and restoring it
// brings the subtree back.
//
// Node deletes require confirm_subtree=true: without it the call fails with an
// error naming the number of intents that would be removed, so programmatic
// callers cannot cascade-delete a module silently.
//
// Dependents is the sorted union of relations.Dependents over every deleted
// slug (excluding the deleted slugs themselves). v1 surfaces this for caller
// awareness (a soft warning) but does NOT block the deletion — see RFC
// 2026-10-03-frontmatter-references §5.3.
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

	// One snapshot drives everything: the target kind (node vs leaf), the
	// subtree slugs, and the dependent union. A failed load only degrades the
	// courtesy fields; the delete itself is the user's intent.
	kind := model.IntentKindLeaf
	deletedSlugs := []string{in.Slug}
	dependents := []string{}
	if all, lerr := LoadAllCached(ctx, root); lerr == nil {
		prefix := in.Slug + "/"
		subtree := map[string]struct{}{in.Slug: {}}
		for _, intent := range all {
			if intent.Slug == in.Slug {
				kind = intent.Kind
			}
			if strings.HasPrefix(intent.Slug, prefix) {
				subtree[intent.Slug] = struct{}{}
			}
		}
		deletedSlugs = deletedSlugs[:0]
		for slug := range subtree {
			deletedSlugs = append(deletedSlugs, slug)
		}
		sort.Strings(deletedSlugs)
		depSet := map[string]struct{}{}
		for slug := range subtree {
			for _, dependent := range relations.Dependents(all, slug) {
				if _, removed := subtree[dependent]; removed {
					continue
				}
				depSet[dependent] = struct{}{}
			}
		}
		for dependent := range depSet {
			dependents = append(dependents, dependent)
		}
		sort.Strings(dependents)
	}
	if kind == model.IntentKindNode && !in.ConfirmSubtree {
		return nil, DeleteOutput{}, fmt.Errorf(
			"kron_delete: %q is a node intent; its subtree has %d intent(s). Pass confirm_subtree: true to move the whole directory to trash",
			in.Slug, len(deletedSlugs))
	}

	w, err := store.NewWriter(root)
	if err != nil {
		return nil, DeleteOutput{}, fmt.Errorf("kron_delete: open store: %w", err)
	}
	if err := w.MoveToTrash(ctx, in.Slug); err != nil {
		return nil, DeleteOutput{}, err
	}
	InvalidateCache(root)

	trashedPath, pathErr := w.RelativeTrashPath(in.Slug)
	if pathErr != nil {
		trashedPath = model.TrashPath(in.Slug)
	}
	return nil, DeleteOutput{
		OK:           true,
		TrashedPath:  trashedPath,
		Dependents:   dependents,
		DeletedSlugs: deletedSlugs,
	}, nil
}
