package cli

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/xxx/kron/internal/store"
)

// runRegister implements `kron register`.
//
// Wire contract (RFC docs/rfc/2026-10-14-projects-json-schema-extension.md
// §2.1 + §2.6 + §4 #2):
//
//   - Default: register the current working directory (or -CWD if
//     given) into the multi-project registry at <kron-bin>/projects.json.
//   - On insert: set added_at = now (UTC) and promote the new
//     entry to current_project (the RFC's "register auto-promotes
//     to current" rule in §2.6).
//   - On existing entry: no-op, exit 0, do not touch added_at or
//     current_project (idempotent — re-registering the same path
//     is not an error).
//
// Output contract: human-readable lines on stdout so a terminal
// user can confirm what happened. JSON output is NOT offered here
// (no --format flag) per docs/process/cli-flag.md §1 — "查询类
// flag 原则上属于 MCP/GUI 的职责; 加到 CLI 里会模糊访问层边界".
// If an AI agent needs structured output it should use the MCP
// tool (forthcoming, not in this PR).
func runRegister(args []string, out, errOut io.Writer) error {
	fs := flag.NewFlagSet("register", flag.ContinueOnError)
	fs.SetOutput(errOut)
	fs.Usage = func() {
		fmt.Fprintln(errOut, "Usage: kron register [-CWD <root>]")
		fs.PrintDefaults()
	}
	cwd := fs.String("CWD", "", "Repository root to register (default: current working directory)")
	if err := fs.Parse(args); err != nil {
		return errUsage
	}

	root, err := resolveRoot(*cwd)
	if err != nil {
		fmt.Fprintln(errOut, "kron register:", err)
		return errUsage
	}

	// Read-modify-write projects.json. We don't expose
	// concurrent-write safety beyond the file-atomic guarantee
	// (RFC §2.4) — single-user local file.
	reg, err := store.ReadProjectsJSON()
	if err != nil {
		fmt.Fprintln(errOut, "kron register:", err)
		return errUsage
	}

	inserted, err := store.RegisterProject(reg, root, time.Now())
	if err != nil {
		fmt.Fprintln(errOut, "kron register:", err)
		return errUsage
	}

	// Auto-promote: per RFC §2.6, `kron register` sets the new
	// (or freshly-registered) entry as current_project. We
	// call SetCurrentProject unconditionally so re-registering
	// the same path also "selects" it — that matches the
	// "switch to this project" intent of a user who ran
	// `kron register` from a new checkout.
	prev, err := store.SetCurrentProject(reg, root)
	if err != nil {
		fmt.Fprintln(errOut, "kron register:", err)
		return errUsage
	}

	if err := store.WriteProjectsJSON(reg); err != nil {
		fmt.Fprintln(errOut, "kron register:", err)
		return errUsage
	}

	if inserted {
		fmt.Fprintf(out, "kron: registered %s\n", root)
	} else {
		fmt.Fprintf(out, "kron: %s is already registered\n", root)
	}
	if prev == "" {
		fmt.Fprintf(out, "kron: current_project = %s\n", root)
	} else if prev != root {
		prevAbs, _ := filepath.Abs(prev)
		fmt.Fprintf(out, "kron: current_project: %s -> %s\n", prevAbs, root)
	}
	return nil
}
