package servemcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/parser"
	"github.com/xxx/kron/internal/store"
)

// handleDelete returns the toolHandler for kron_delete.
//
// Wire contract: see docs/implementation/mcp.md §2 (kron_delete).
//
//	in:  { slug: string }
//	out: { ok: bool, trashed_path: string }
//
// Soft-deletes by moving .kron/intents/<slug>.md to .kron/.trash/<slug>.md.
// Returns ErrIntentNotFound (-32004) if the slug is not present in
// intents/. Trash contents are NOT considered.
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

		w, err := store.NewWriter(root)
		if err != nil {
			return nil, fmt.Errorf("kron_delete: open store: %w", err)
		}
		if err := w.MoveToTrash(ctx, args.Slug); err != nil {
			return nil, err
		}

		return deleteResponse{OK: true, TrashedPath: model.TrashPath(args.Slug)}, nil
	}
}

type deleteResponse struct {
	OK          bool   `json:"ok"`
	TrashedPath string `json:"trashed_path"`
}
