// kron is the CLI entry point. All business logic lives in access-layer
// subcommands (cmd/kron/*) and the internal/ packages they orchestrate.
//
// main() owns the top-level "kron <command>" routing. Each access layer
// (cmd/kron/cli, cmd/kron/serve-mcp, future cmd/kron/serve-gui) is a
// peer package that registers its own commands; this file is the only
// place that knows about all of them.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/xxx/kron/cmd/kron/cli"
	"github.com/xxx/kron/cmd/kron/serve-mcp"
)

func main() {
	if len(os.Args) < 2 {
		// No subcommand: route to the CLI's help. (The MCP layer has
		// no interactive help; its protocol is JSON-RPC over stdio.)
		if err := runCLI(os.Args[1:]); err != nil {
			exitFromErr(err)
		}
		return
	}

	var err error
	switch os.Args[1] {
	case "serve-mcp":
		err = servemcp.Run(os.Args[2:], os.Stdout, os.Stderr)
	default:
		err = runCLI(os.Args[1:])
	}
	exitFromErr(err)
}

// runCLI delegates to the CLI access layer. It exists as a thin wrapper
// so main() can route between access layers without cli ever importing
// the MCP layer (or vice versa).
func runCLI(args []string) error {
	// cli.Execute reads os.Args directly; we re-route by swapping
	// os.Args temporarily. This is intentional: cli is designed
	// around cobra's "Execute reads os.Args" pattern, and we do not
	// want to maintain a parallel arg-list API just for testing.
	// The mutation is local to this call frame and never escapes.
	old := os.Args
	defer func() { os.Args = old }()
	os.Args = append([]string{old[0]}, args...)
	return cli.Execute()
}

// exitFromErr applies the access-layer exit-code policy.
//
// `cli.ExitCoder` carries an explicit exit code (used by `kron lint`).
// `servemcp.ErrNotImplemented` is a plain sentinel that this file
// maps to exit 1. Subcommand layers are expected to have already
// written a user-facing message to stderr; this function only
// appends a generic message for unknown errors and never duplicates
// the subcommand's own output.
func exitFromErr(err error) {
	if err == nil {
		return
	}

	var code cli.ExitCoder
	if errors.As(err, &code) {
		os.Exit(code.ExitCode())
	}
	if errors.Is(err, servemcp.ErrNotImplemented) {
		// servemcp.Run already wrote the placeholder message.
		os.Exit(1)
	}

	// Unknown error: surface a generic message and exit 1.
	fmt.Fprintln(os.Stderr, "kron:", err)
	os.Exit(1)
}
