// kron is the CLI entry point. All business logic lives in access-layer
// subcommands (cmd/kron/*) and the internal/ packages they orchestrate.
package main

import (
	"errors"
	"os"

	"github.com/xxx/kron/cmd/kron/cli"
)

func main() {
	err := cli.Execute()
	if err == nil {
		return
	}

	// Subcommands can return typed sentinel errors to influence the
	// process exit code. cli maps these to the codes defined in
	// docs/implementation/cli.md §4.
	var code cli.ExitCoder
	if errors.As(err, &code) {
		os.Exit(code.ExitCode())
	}
	os.Exit(1)
}
