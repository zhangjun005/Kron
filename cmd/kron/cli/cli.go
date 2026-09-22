// Package cli implements the cobra command tree for kron.
//
// This package is part of the access layer. Like every access layer
// (cmd/kron/cli, cmd/kron/serve-mcp, future cmd/kron/serve-gui), it
// imports cobra and may import internal/* packages, but it MUST NOT
// import any sibling access layer package.
//
// Business logic is delegated to internal/store and internal/parser.
// This file stays thin: parse flags, dispatch, wrap exit codes.
package cli

import "fmt"

// Execute runs the CLI root command and returns any error encountered.
func Execute() error {
	// TODO: wire cobra root command and subcommands:
	//   kron init
	//   kron add <slug>
	//   kron lint
	fmt.Println("kron: not yet implemented")
	return fmt.Errorf("not implemented")
}
