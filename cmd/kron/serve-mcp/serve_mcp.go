// Package servemcp is the MCP stdio server entry point for Kron.
//
// This package is a peer of cmd/kron/cli in the access layer. Like
// every access layer, it owns its own command-line surface and its
// own process model (stdin/stdout JSON-RPC) and delegates all
// business logic to internal/store and internal/parser. It MUST
// NOT import any sibling access layer package (cmd/kron/cli,
// future cmd/kron/serve-gui).
//
// The MCP server itself is a phase 2 deliverable. The v1 binary
// must still respond to `kron serve-mcp` so CI scripts that
// mistakenly wire it see a clear failure, not a silent no-op.
package servemcp

import (
	"errors"
	"fmt"
	"io"
)

// ErrNotImplemented is the v1 sentinel: the MCP JSON-RPC server has
// not been written yet. main() maps it to exit code 1.
var ErrNotImplemented = errors.New("kron serve-mcp: not yet available (planned for phase 2)")

// Run is the entry point. It writes a clear "not yet available"
// message to errOut and returns ErrNotImplemented so the calling
// layer (cmd/kron/main.go) can decide the exit code.
func Run(args []string, out, errOut io.Writer) error {
	_ = args
	_ = out
	fmt.Fprintln(errOut, "kron serve-mcp: not yet available (planned for phase 2)")
	return ErrNotImplemented
}
