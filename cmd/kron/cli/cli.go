// Package cli implements the cobra command tree for kron.
//
// This package is part of the access layer. Like every access layer
// (cmd/kron/cli, cmd/kron/serve-mcp, future cmd/kron/serve-gui), it
// imports cobra and may import internal/* packages, but it MUST NOT
// import any sibling access layer package.
//
// Business logic is delegated to internal/store and internal/parser.
// This file stays thin: parse flags, dispatch, wrap exit codes.
//
// Status (2026-09-27): stub — Execute() returns "not yet implemented".
// Target commands: kron init / kron add / kron lint / kron serve-mcp.
package cli

import "fmt"

// Execute runs the CLI root command and returns any error encountered.
// Target: wire init / add / lint / serve-mcp.
func Execute() error {
	fmt.Println("kron: not yet implemented")
	return fmt.Errorf("not implemented")
}
