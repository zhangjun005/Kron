package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/store"
)

// runInit implements `kron init`.
//
// It creates .kron/intents/ under the current working directory (or
// under -CWD) and writes a default .kron/config.toml. The command is
// idempotent: a second invocation on an already-initialised tree keeps
// the existing .kron/ untouched. Both paths then register the project
// in the multi-project registry and make it current_project (same
// side effect as MCP `kron_init`).
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

	// A pre-existing .kron/ means "already initialised": keep the skeleton
	// untouched (same idempotency contract as git init) but still refresh the
	// multi-project registry below, mirroring MCP `kron_init`. This makes
	// `kron init` a reliable way to select an existing checkout as
	// current_project without remembering `kron register`.
	kronDir := filepath.Join(root, model.KronDir)
	already := false
	if _, err := os.Stat(kronDir); err == nil {
		already = true
	} else if !os.IsNotExist(err) {
		fmt.Fprintln(errOut, "kron init:", err)
		return err
	}

	if !already {
		if err := store.EnsureKronDir(context.Background(), root); err != nil {
			fmt.Fprintln(errOut, "kron init:", err)
			return err
		}
		cfg := &model.Config{IntentsDir: ".kron/intents"}
		if err := store.SaveConfig(context.Background(), root, cfg); err != nil {
			fmt.Fprintln(errOut, "kron init:", err)
			return err
		}
	}

	// RFC docs/rfc/2026-10-14-projects-json-schema-extension.md
	// §2.1 + §2.6: `kron init` also registers the project into
	// the multi-project registry at <kron-bin>/projects.json and
	// auto-promotes it to current_project. The two operations
	// (init + register) are atomic from the user's perspective:
	// "I just kron-init'd a project" should also mean "I just
	// added it to my project list and made it active". Errors
	// here are NOT fatal: the user already has a working .kron/
	// skeleton, so we print a warning and continue with exit 0.
	registered := registerProjectInRegistryCLI(root, errOut)

	if already {
		if registered {
			fmt.Fprintf(out, "kron: %s already exists; current_project = %s\n", kronDir, root)
		} else {
			fmt.Fprintf(out, "kron: %s already exists; project registry update skipped\n", kronDir)
		}
		return nil
	}
	fmt.Fprintf(out, "kron: initialised %s\n", kronDir)
	return nil
}

// registerProjectInRegistryCLI performs the init/register side effect on
// projects.json (read-modify-write). It mirrors serve-mcp's
// registerProjectInRegistry but routes warnings to the CLI's stderr. Returns
// false (after printing a warning) when the update could not be persisted.
func registerProjectInRegistryCLI(root string, errOut io.Writer) bool {
	reg, regErr := store.ReadProjectsJSON()
	if regErr != nil {
		fmt.Fprintf(errOut, "kron init: warning: read projects.json failed: %v\n", regErr)
		return false
	}
	if _, rerr := store.RegisterProject(reg, root, time.Now()); rerr != nil {
		fmt.Fprintf(errOut, "kron init: warning: register project failed: %v\n", rerr)
		return false
	}
	if _, serr := store.SetCurrentProject(reg, root); serr != nil {
		fmt.Fprintf(errOut, "kron init: warning: set current_project failed: %v\n", serr)
		return false
	}
	if werr := store.WriteProjectsJSON(reg); werr != nil {
		fmt.Fprintf(errOut, "kron init: warning: write projects.json failed: %v\n", werr)
		return false
	}
	return true
}

func resolveRoot(cwdFlag string) (string, error) {
	if cwdFlag != "" {
		return filepath.Abs(cwdFlag)
	}
	return os.Getwd()
}
