// Package cli implements Kron's CLI access layer.
//
// This package is part of the access layer. It accepts user input via
// flags/args and delegates all business logic to internal/store and
// internal/parser. It MUST NOT import any sibling access layer package
// (cmd/kron/serve-mcp, future cmd/kron/serve-gui).
//
// CLI subcommands: kron init / kron add / kron lint.
// Query operations (list / get / update / delete / restore) and the
// MCP stdio server belong to other access layers and are intentionally
// not exposed as CLI subcommands — see cmd/kron/serve-mcp.
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// errUsage is returned when the user invokes kron incorrectly
// (no args, unknown subcommand, missing required args). It is mapped
// to exit code 1 at the main() layer.
var errUsage = errors.New("usage error")

// ExitCoder is implemented by errors that carry a specific process
// exit code. main() inspects Execute()'s return value via errors.As
// and exits with code.ExitCode() if matched. Subcommands that need
// non-1 exit codes (currently only `lint`) wrap their sentinel
// values in an exitCodeError.
type ExitCoder interface {
	error
	ExitCode() int
}

type exitCodeError struct {
	code int
	desc string
}

func (e *exitCodeError) Error() string { return e.desc }
func (e *exitCodeError) ExitCode() int { return e.code }

// newExitError returns an ExitCoder that will cause main() to exit
// with the given code. The description is what gets printed to
// stderr (when non-empty) before exit.
func newExitError(code int, desc string) error {
	return &exitCodeError{code: code, desc: desc}
}

// Execute parses os.Args[2:] and dispatches to the appropriate CLI
// subcommand. It does NOT own the top-level "kron <command>" routing;
// cmd/kron/main.go does that, so that sibling access layers
// (cmd/kron/serve-mcp, ...) can register their own top-level commands
// without cli having to know about them.
func Execute() error {
	out := os.Stdout
	if len(os.Args) < 2 {
		printHelp(out)
		return errUsage
	}

	switch os.Args[1] {
	case "init":
		return runInit(os.Args[2:], out, os.Stderr)
	case "add":
		return runAdd(os.Args[2:], out, os.Stderr)
	case "lint":
		return runLint(os.Args[2:], out, os.Stderr)
	case "help", "-h", "--help":
		printHelp(out)
		return nil
	default:
		fmt.Fprintf(os.Stderr, "kron: unknown subcommand %q\n", os.Args[1])
		printHelp(os.Stderr)
		return errUsage
	}
}

func printHelp(w io.Writer) {
	fmt.Fprintln(w, "Kron — Git-native intent management for AI-assisted development")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage: kron <command> [arguments]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  init         Create .kron/ skeleton in the current repository")
	fmt.Fprintln(w, "  add <slug>   Scaffold a new intent file at .kron/intents/<slug>.md")
	fmt.Fprintln(w, "  lint         Scan anchors and frontmatter; exit 0 on clean, 1 on errors")
	fmt.Fprintln(w, "  serve-mcp    Start the MCP stdio server (separate access layer; see cmd/kron/serve-mcp)")
	fmt.Fprintln(w, "  serve-lsp    Start the LSP stdio server (v1.3+ stub; see cmd/kron/serve-lsp)")
	fmt.Fprintln(w, "  help         Show this message")
}
