package servemcp

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/store"
)

// HandleInit returns the mcp.ToolHandlerFor binding for kron_init.
//
// Wire contract: see docs/implementation/mcp.md §2 (kron_init).
//
//	in:  {}
//	out: { ok, intents_dir, already_initialised, repo_root,
//	       registered_current_project }
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
//
// Why registered_current_project is in the response: `kron_init`
// has a side effect on the multi-project registry (RFC
// 2026-10-14-projects-json-schema-extension.md §2.6 — same as
// CLI `kron init`, mirror semantics, not a new tool). Surfacing
// the post-write value lets the AI caller know whether subsequent
// `kron serve-mcp` invocations on the same machine will resolve
// MCPRoot to this project (current_project promoted to `root`)
// or to whichever project was previously active. The empty string
// signals "registry write was skipped or unchanged" so the AI
// can decide whether to follow up with `kron_register`.
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

	// RFC docs/rfc/2026-10-14-projects-json-schema-extension.md
	// §2.1 + §2.6: `kron init` also registers the project into
	// the multi-project registry at <kron-bin>/projects.json and
	// auto-promotes it to current_project. The two operations
	// (init + register) are atomic from the user's perspective:
	// "I just kron-init'd a project" should also mean "I just
	// added it to my project list and made it active". This
	// mirrors the CLI `kron init` contract (see
	// cmd/kron/cli/init.go).
	//
	// Errors here are NOT fatal: the user already has a working
	// .kron/ skeleton, so we log a warning and continue with
	// ok=true. The CLI path prints the warning to stderr; the
	// MCP path uses slog so the warning surfaces through the
	// MCP Logging capability (set up in serve_mcp.go via
	// shimHandler → SDK session.Log). Tests that drive the
	// session via newTestSession capture the log line; bare
	// production runs may drop it if no client has called
	// logging/setLevel, which is acceptable for a "side
	// effect" warning.
	//
	// We do NOT return an error from this branch; the function
	// falls through with registeredCurrentProject = "" when
	// the registry write was skipped. The AI caller can see
	// the empty value in the response and decide whether to
	// follow up with `kron_register`.
	registeredCurrent := registerProjectInRegistry(root)

	return nil, InitOutput{
		OK:                       true,
		IntentsDir:               model.KronDir + "/" + model.IntentDir,
		AlreadyInitialised:       already,
		RepoRoot:                 root,
		RegisteredCurrentProject: registeredCurrent,
	}, nil
}

// registerProjectInRegistry performs the init+register side effect
// mandated by RFC 2026-10-14-projects-json-schema-extension.md §2.6.
// Returns the projects.json::current_project after the side effect,
// or "" if the write was skipped due to a non-fatal error.
//
// The function is a deliberate duplicate of the CLI path
// (cmd/kron/cli/init.go) — the CLI uses fmt.Fprintf(errOut, ...)
// for warnings, the MCP path uses slog so the message surfaces
// through the MCP Logging capability. The two paths are too
// thin (a single read-modify-write on projects.json) to justify
// a shared helper that would have to thread both an io.Writer
// and a *slog.Logger through every call site.
func registerProjectInRegistry(root string) string {
	reg, regErr := store.ReadProjectsJSON()
	if regErr != nil {
		slog.Warn("kron_init: read projects.json failed", "err", regErr, "root", root)
		return ""
	}
	if _, rerr := store.RegisterProject(reg, root, time.Now()); rerr != nil {
		slog.Warn("kron_init: register project failed", "err", rerr, "root", root)
		return ""
	}
	prev, serr := store.SetCurrentProject(reg, root)
	if serr != nil {
		slog.Warn("kron_init: set current_project failed", "err", serr, "root", root)
		return ""
	}
	if werr := store.WriteProjectsJSON(reg); werr != nil {
		slog.Warn("kron_init: write projects.json failed", "err", werr, "root", root)
		return ""
	}
	_ = prev // logged at debug level if needed; not surfaced to the AI caller
	return root
}
