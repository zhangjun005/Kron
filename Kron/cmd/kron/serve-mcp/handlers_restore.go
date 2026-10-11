package servemcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/parser"
	"github.com/xxx/kron/internal/store"
)

// HandleRestore returns the mcp.ToolHandlerFor binding for kron_restore.
//
// Wire contract: see docs/implementation/mcp.md §2 (kron_restore).
//
//	in:  { slug: string }
//	out: { ok: bool, path: string }
//
// Restores a soft-deleted intent by moving it from .kron/.trash/<slug>.md
// back to .kron/intents/<slug>.md. Returns ErrIntentNotFound
// (→ IsError=true) if the slug is not in trash; ErrIntentExists
// (→ IsError=true) if an active intent with the same slug already
// exists in intents/.
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

	return nil, RestoreOutput{OK: true, Path: model.IntentPath(in.Slug)}, nil
}
