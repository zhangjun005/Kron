package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/store"
)

// runInit implements `kron init`.
//
// It creates .kron/intents/ under the current working directory (or
// under -CWD) and writes a default .kron/config.toml. The command is
// idempotent: a second invocation on an already-initialised tree
// reports success without overwriting any existing intent files.
func runInit(args []string, out, errOut io.Writer) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(errOut)
	fs.Usage = func() {
		fmt.Fprintln(errOut, "Usage: kron init [-CWD <root>]")
		fs.PrintDefaults()
	}
	cwd := fs.String("CWD", "", "Repository root (default: current working directory)")
	if err := fs.Parse(args); err != nil {
		return errUsage
	}

	root, err := resolveRoot(*cwd)
	if err != nil {
		fmt.Fprintln(errOut, "kron init:", err)
		return errUsage
	}

	// Detect a re-init. We treat "already initialised" as informational,
	// not an error — same idempotency contract as git init.
	kronDir := filepath.Join(root, model.KronDir)
	if _, err := os.Stat(kronDir); err == nil {
		fmt.Fprintf(out, "kron: %s already exists; nothing to do.\n", kronDir)
		return nil
	}

	if err := store.EnsureKronDir(context.Background(), root); err != nil {
		fmt.Fprintln(errOut, "kron init:", err)
		return err
	}

	cfg := &model.Config{IntentsDir: ".kron/intents"}
	if err := store.SaveConfig(context.Background(), root, cfg); err != nil {
		fmt.Fprintln(errOut, "kron init:", err)
		return err
	}

	fmt.Fprintf(out, "kron: initialised %s\n", kronDir)
	return nil
}

func resolveRoot(cwdFlag string) (string, error) {
	if cwdFlag != "" {
		return filepath.Abs(cwdFlag)
	}
	return os.Getwd()
}
