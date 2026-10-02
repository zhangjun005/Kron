package cli

import (
	"flag"
	"fmt"
	"io"
)

// runServeMCP is the `kron serve-mcp` dispatcher.
//
// The MCP server itself is deferred to phase 2 (see
// docs/process/phase-1-leftovers.md L3, recently re-classified as a
// phase 2 deliverable because architecture §〇·五·5 keeps
// `internal/lint` gated on multiple access-layer consumers and
// the CLI alone does not warrant opening the package).
//
// v1 behaviour: print a clear "not yet available" message and
// return an exit code 1 so CI scripts that mistakenly wire
// serve-mcp see the failure explicitly.
func runServeMCP(args []string, out, errOut io.Writer) error {
	fs := flag.NewFlagSet("serve-mcp", flag.ContinueOnError)
	fs.SetOutput(errOut)
	fs.Usage = func() {
		fmt.Fprintln(errOut, "Usage: kron serve-mcp")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return errUsage
	}

	fmt.Fprintln(errOut, "kron serve-mcp: not yet available (planned for phase 2)")
	return newExitError(1, "serve-mcp not yet available")
}
