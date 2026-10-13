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
//	out: { ok, intents_dir, already_initialised, repo_root }
//
// Semantics (2026-10-03 decision): idempotent — re-running on an
// already-initialised repo returns ok=true with already_initialised=true,
// matching the CLI `kron init` contract (idempotent like `git init`).
// The mcp.md spec says ErrIntentExists for re-init; we deliberately
// diverge to keep CLI ↔ MCP behavior consistent for the common case.
//
// Why repo_root is in the response: the 12 kron_* tools do not take
// a path field (server-side ctx drives all I/O), so the AI caller
// never sees which directory the server is operating on. If a
// client misconfigures cwd / -CWD the server fails fast in
// resolveMCPRoot, but the AI sees a JSON-RPC error and may not
// know *which* path was rejected. Surfacing the absolute repo
// root on the first tool call (kron_init is always called first)
// gives the AI a concrete reference to include in user-facing
// error messages ("kron is operating on /foo/bar, but you said
// /baz/qux — re-configure your client"), turning silent misconfig
// into self-debugging. The cost is one string per init call
// (sub-millisecond, not a hot path).
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
		RepoRoot:           root,
	}, nil
}
