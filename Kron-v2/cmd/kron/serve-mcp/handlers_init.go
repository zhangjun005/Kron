package servemcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/store"
)

// HandleInit returns the mcp.ToolHandlerFor binding for kron_init.
//
// Wire contract: see docs/implementation/mcp.md §2 (kron_init).
//
//	in:  {}
//	out: { ok: bool, intents_dir: string, already_initialised: bool }
//
// Semantics (2026-10-03 decision): idempotent — re-running on an
// already-initialised repo returns ok=true with already_initialised=true,
// matching the CLI `kron init` contract (idempotent like `git init`).
// The mcp.md spec says ErrIntentExists for re-init; we deliberately
// diverge to keep CLI ↔ MCP behavior consistent for the common case.
func HandleInit(ctx context.Context, _ *mcp.CallToolRequest, _ InitInput) (
	*mcp.CallToolResult, InitOutput, error,
) {
	if err := assertCallerMCP(ctx); err != nil {
		return nil, InitOutput{}, err
	}
	root := repoRootFrom(ctx)

	kronDir := filepath.Join(root, model.KronDir)
	already := false
	if _, err := os.Stat(kronDir); err == nil {
		already = true
	} else if !os.IsNotExist(err) {
		return nil, InitOutput{}, fmt.Errorf("kron_init: stat %s: %w", kronDir, err)
	}

	if !already {
		if err := store.EnsureKronDir(ctx, root); err != nil {
			return nil, InitOutput{}, fmt.Errorf("kron_init: ensure .kron/: %w", err)
		}
		cfg := &model.Config{IntentsDir: ".kron/intents"}
		if err := store.SaveConfig(ctx, root, cfg); err != nil {
			return nil, InitOutput{}, fmt.Errorf("kron_init: save config: %w", err)
		}
	}

	return nil, InitOutput{
		OK:                 true,
		IntentsDir:         model.KronDir + "/" + model.IntentDir,
		AlreadyInitialised: already,
	}, nil
}
