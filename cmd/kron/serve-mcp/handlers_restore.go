package servemcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/parser"
	"github.com/xxx/kron/internal/store"
)

// handleRestore returns the toolHandler for kron_restore.
//
// Wire contract: see docs/implementation/mcp.md §2 (kron_restore).
//
//	in:  { slug: string }
//	out: { ok: bool, path: string }
//
// Restores a soft-deleted intent by moving it from .kron/.trash/<slug>.md
// back to .kron/intents/<slug>.md. Returns ErrIntentNotFound (-32004)
// if the slug is not in trash; ErrIntentExists (-32005) if an active
// intent with the same slug already exists in intents/.
func handleRestore(root string) toolHandler {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		if err := assertCallerMCP(ctx); err != nil {
			return nil, err
		}
		var args struct {
			Slug string `json:"slug"`
		}
		if len(params) > 0 {
			if err := json.Unmarshal(params, &args); err != nil {
				return nil, fmt.Errorf("kron_restore: invalid params: %w", err)
			}
		}
		if args.Slug == "" {
			return nil, fmt.Errorf("kron_restore requires non-empty slug")
		}
		if err := parser.ValidateSlug(args.Slug); err != nil {
			return nil, err
		}

		w, err := store.NewWriter(root)
		if err != nil {
			return nil, fmt.Errorf("kron_restore: open store: %w", err)
		}
		if err := w.RestoreFromTrash(ctx, args.Slug); err != nil {
			return nil, err
		}

		return restoreResponse{OK: true, Path: model.IntentPath(args.Slug)}, nil
	}
}

type restoreResponse struct {
	OK   bool   `json:"ok"`
	Path string `json:"path"`
}
