// kron is the CLI entry point. All business logic lives in access-layer
// subcommands (cmd/kron/*) and the internal/ packages they orchestrate.
package main

import (
	"os"

	"github.com/xxx/kron/cmd/kron/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
