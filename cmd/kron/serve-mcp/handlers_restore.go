package servemcp

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/parser"
	"github.com/xxx/kron/internal/store"
)

// HandleRestore returns the mcp.ToolHandlerFor binding for kron_restore.
//
// Wire contract: see docs/implementation/2026-10-29-trash-subtree.md.
//
//	in:  { slug: string }
//	out: { ok: bool, path: string, restored_slugs: [slug] }
//
// Restores a soft-deleted entry by moving it back into .kron/intents/. The
// trash shape decides the restored shape: <trash>/<slug>/ (node) comes back as
// <intents>/<slug>/ with its whole subtree, <trash>/<slug>.md (leaf, including
// legacy node-as-file entries) comes back as <intents>/<slug>.md.
//
// Returns ErrIntentNotFound (→ IsError=true) if the slug is not in trash;
// ErrIntentExists (→ IsError=true) if either live shape already exists (restore
// never merges into an existing entry).
func HandleRestore(ctx context.Context, _ *mcp.CallToolRequest, in RestoreInput) (
	*mcp.CallToolResult, RestoreOutput, error,
) {
	if err := assertCallerMCP(ctx); err != nil {
		return nil, RestoreOutput{}, err
	}
	if in.Slug == "" {
		return nil, RestoreOutput{}, fmt.Errorf("kron_restore requires non-empty slug")
	}
	if err := parser.ValidateSlug(in.Slug); err != nil {
		return nil, RestoreOutput{}, err
	}

	root := repoRootFrom(ctx)
	w, err := store.NewWriter(root)
	if err != nil {
		return nil, RestoreOutput{}, fmt.Errorf("kron_restore: open store: %w", err)
	}
	if err := w.RestoreFromTrash(ctx, in.Slug); err != nil {
		return nil, RestoreOutput{}, err
	}
	InvalidateCache(root)

	restoredSlugs := []string{in.Slug}
	if all, lerr := LoadAllCached(ctx, root); lerr == nil {
		prefix := in.Slug + "/"
		restoredSlugs = restoredSlugs[:0]
		for _, intent := range all {
			if intent.Slug == in.Slug || strings.HasPrefix(intent.Slug, prefix) {
				restoredSlugs = append(restoredSlugs, intent.Slug)
			}
		}
		sort.Strings(restoredSlugs)
	}

	path, pathErr := w.RelativeIntentPath(in.Slug)
	if pathErr != nil {
		path = model.IntentPath(in.Slug)
	}
	return nil, RestoreOutput{OK: true, Path: path, RestoredSlugs: restoredSlugs}, nil
}
