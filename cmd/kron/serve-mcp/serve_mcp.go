// Package servemcp is the MCP stdio server entry point for Kron.
//
// This package is a peer of cmd/kron/cli in the access layer. Like
// every access layer, it owns its own command-line surface and its
// own process model (stdin/stdout JSON-RPC) and delegates all
// business logic to internal/store, internal/parser, and internal/lint.
// It MUST NOT import any sibling access layer package (cmd/kron/cli,
// future cmd/kron/serve-gui).
//
// Wire protocol: JSON-RPC 2.0 over stdio (one JSON object per line).
// See docs/implementation/mcp.md and the JSON-RPC 2.0 spec
// (https://www.jsonrpc.org/specification) for the envelope shape.
package servemcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/xxx/kron/internal/model"
)

// ErrNotImplemented is the v1 sentinel kept for backwards compatibility
// with cmd/kron/main.go, which checks errors.Is(err, ErrNotImplemented)
// to map it to exit 1. In v1.1 the MCP server is implemented; this
// sentinel is no longer returned by Run. It is kept exported so external
// tooling (e.g. CI scripts) that grep for "not yet available" continues
// to parse the binary's stderr without changes.
var ErrNotImplemented = errors.New("kron serve-mcp: not yet available (planned for phase 2)")

// Run is the entry point. args[0] is the optional "-CWD <root>" flag;
// absent it defaults to the process's current working directory.
//
// Run reads JSON-RPC 2.0 requests from in (one JSON object per line),
// dispatches each to the registered tool handler, and writes responses
// to out. Notification requests (those without an "id" field) are
// processed but produce no response. Parse errors and protocol-level
// failures are returned as JSON-RPC error responses; Run itself only
// returns a non-nil Go error for I/O or wiring failures that prevent
// the loop from continuing.
//
// The server injects model.CallerMCP into ctx for every dispatched tool,
// satisfying the architecture.md §〇 铁律 #4 requirement that access
// layers identify themselves via context. Per-tool identity is
// "mcp:<client>" where <client> comes from the JSON-RPC "client" hint
// if the client supplied one; otherwise it falls back to plain "mcp".
func Run(args []string, in io.Reader, out, errOut io.Writer) error {
	root, err := resolveMCPRoot(args)
	if err != nil {
		fmt.Fprintln(errOut, "kron serve-mcp:", err)
		return err
	}

	srv := &server{
		root:   root,
		out:    out,
		errOut: errOut,
		tools:  defaultToolRegistry(root),
	}
	return srv.serve(in)
}

// resolveMCPRoot parses the optional "-CWD <root>" flag pair. Keeping
// it stdlib-only (no cobra) is intentional — this layer is
// independently testable and shouldn't drag in CLI argument conventions.
func resolveMCPRoot(args []string) (string, error) {
	if len(args) == 0 {
		return os.Getwd()
	}
	if len(args) == 2 && args[0] == "-CWD" {
		return filepath.Abs(args[1])
	}
	return "", fmt.Errorf("kron serve-mcp: unexpected arguments %v (only -CWD <root> is supported)", args)
}

// withMCPCaller injects the MCP caller identity into ctx. If client is
// non-empty the caller is "mcp:<client>"; otherwise it falls back to
// the bare "mcp" identity. The caller is an assertion, not an authz
// gate; tools MUST NOT use it for permission decisions.
func withMCPCaller(ctx context.Context, client string) context.Context {
	if client == "" {
		return model.WithCaller(ctx, model.CallerMCP)
	}
	return model.WithCaller(ctx, model.CallerMCP+":"+client)
}
