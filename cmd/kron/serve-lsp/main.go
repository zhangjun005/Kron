// Package lsp is the LSP (Language Server Protocol) access layer of
// Kron. It is the v1.3+ entry point that gives editors (VSCode /
// Cursor / Neovim) hover and definition jumps on
// "// @kron:intent <slug>" anchors.
//
// This file is the stub: it exposes a Run() function mirroring the
// signature of cmd/kron/serve-mcp.Run so cmd/kron/main.go can
// dispatch to it uniformly. The actual LSP protocol dispatch
// (initialize / hover / definition / shutdown / exit) is not yet
// implemented.
//
// Why a stub (not absent) today:
//
//  1. The package compiles in v1.2 so the rest of the access
//     layer matrix stays intact for testing.
//  2. The cmd/kron/{cli,serve-mcp,serve-lsp} 3-of-3 access-layer
//     enum from architecture §〇·五·1 is observable on disk.
//  3. The "kron serve-lsp" argv route is wired through
//     cmd/kron/main.go so editors that try to spawn the binary
//     today fail fast with a "not implemented" message rather
//     than crashing silently or hanging on stdin.
//
// Naming convention: directory is hyphenated (cmd/kron/serve-lsp)
// to match serve-mcp; the Go package is "lsp" (single segment,
// acronym) so the import path is single-segment. This is the
// idiomatic Go trade-off when the directory name is a multi-word
// phrase with a hyphen.
//
// Tech stack (single source of truth: docs/rfc/ — do NOT
// duplicate here, the doc links are the index for future
// implementers):
//
//   - Language:  Go 1.27 (matches the rest of cmd/kron/*)
//   - LSP SDK:   go.lsp.dev/protocol v3.17+  (RFC 2026-10-07 §3)
//   - Version:   LSP 3.17                    (RFC 2026-10-07 §4.1)
//   - Transport: stdio, Content-Length       (RFC 2026-10-07 §4.2)
//     framed JSON-RPC
//   - Capab v1.3: textDocument/hover +       (RFC 2026-10-07 §5)
//     textDocument/definition
//   - Capab v1.4+ (NOT today): completion +  (RFC 2026-10-07 §5)
//     publishDiagnostics
//   - Process:   stdio parent lifetime,      (RFC 2026-10-08 §4.1)
//     no daemon, no socket
//   - Concurr.:  single-instance serial;     (RFC 2026-10-08 §4.2)
//     v1.4+ may parallelise when
//     hover output grows
//   - Client:    each editor spawns its own  (RFC 2026-10-08 §3.2)
//     child process; no Kron-side
//     client SDK
//   - internal:  parser.SlugsForFile +       (RFC 2026-10-08 §4.4)
//     store.Get; NO new internal
//     API needed (the originally
//     proposed AnchorAtPosition
//     is not added)
//
// Process compliance: this package MUST satisfy
// docs/process/new-access-layer.md §4 — the 5 invariants are
// re-stated here so a future implementer does not have to chase
// the doc:
//
//  1. NO import of cmd/kron/cli, cmd/kron/serve-mcp, or any
//     other access-layer package. Sibling access layers are
//     forbidden by architecture §〇 铁律 #2/#3.
//  2. NO import from internal/* into this package's non-stub
//     code (when the real protocol lands, it imports
//     internal/parser + internal/store, not other internals).
//  3. NO context.Context-based caller injection. This layer
//     passes a context down to store/parser as a
//     cancellation/deadline signal only, NOT as an identity
//     carrier. model.WithCaller is NOT called here (architecture
//     §2.3 deprecation, 2026-10-08).
//  4. NO package-level mutable state.
//  5. NO new dependencies beyond what the RFC §3 / §4 SDK
//     lock declares. Adding go.lsp.dev/protocol requires an
//     explicit-approval commit body with the 4-paragraph
//     justification (architecture §〇 铁律 #5 + AGENTS.md §3).
//     This stub uses ONLY stdlib (io, fmt, errors) to demonstrate
//     the zero-dep posture; the real implementation will add
//     go.lsp.dev/protocol as a tracked, approved dependency.
package lsp

import (
	"errors"
	"fmt"
	"io"
)

// Run is the entry point for the LSP access layer, mirroring the
// signature of cmd/kron/serve-mcp.Run so cmd/kron/main.go can
// dispatch with a uniform shape:
//
//	err = lsp.Run(os.Args[2:], os.Stdin, os.Stdout, os.Stderr)
//
// The stub body writes a not-implemented message to stderr and
// returns ErrNotImplemented. Once the protocol is implemented this
// will become the LSP frame loop (read Content-Length framed
// JSON-RPC from in, dispatch to handlers, write responses to out).
//
// args is reserved for future flag parsing (e.g. --log-level).
// The stub currently parses no flags — it always prints the
// not-implemented message and returns.
func Run(args []string, in io.Reader, out, errOut io.Writer) error {
	// in is reserved for the future stdio frame loop. Referencing
	// it here so the signature is honest about its intended use;
	// a linter that flags unused parameters would otherwise
	// suggest removing it.
	_ = in

	// args is reserved for future flag parsing. The stub ignores
	// any args today (no --help, no --version, no --log-level);
	// a future implementation will switch on them.
	_ = args

	_, _ = fmt.Fprintln(errOut, "kron serve-lsp: not yet implemented (v1.3+; see docs/rfc/2026-10-08-lsp-client.md)")
	_, _ = fmt.Fprintln(out) // keep out referenced for future implementation

	return ErrNotImplemented
}

// ErrNotImplemented is the sentinel returned by Run today. It is
// exported so cmd/kron/main.go can match it via errors.Is and map
// it to exit code 1 (same treatment as servemcp.ErrNotImplemented).
// When the real implementation lands this sentinel is removed
// and the function returns nil on graceful shutdown or a different
// error on protocol failure.
var ErrNotImplemented = errors.New("serve-lsp: stub")
