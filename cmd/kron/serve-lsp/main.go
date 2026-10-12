// Package lsp is the LSP (Language Server Protocol) access layer of
// Kron. It gives editors (VSCode / Cursor / Neovim) hover and
// definition jumps on "// @kron:intent <slug>" anchors.
//
// Naming convention: directory is hyphenated (cmd/kron/serve-lsp) to
// match serve-mcp; the Go package is "lsp" (single segment, acronym)
// so the import path is single-segment.
//
// Tech stack and process contract (single source of truth:
// docs/implementation/lsp.md and docs/rfc/2026-10-07-lsp-sdk.md /
// 2026-10-08-lsp-client.md):
//
//   - LSP SDK:  go.lsp.dev/protocol (jsonrpc2 + uri come along)
//   - Transport: stdio, Content-Length framed JSON-RPC
//   - Capab v1.3: initialize / hover / definition / shutdown / exit
//   - Process:   stdio parent lifetime, no daemon, no socket
//   - Concurr.:  single-instance serial (jsonrpc2 read loop)
//   - internal:  parser.AnchorsForFile + store.Load only
package lsp

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"

	"github.com/xxx/kron/internal/store"
)

// Run is the entry point for the LSP access layer, mirroring the
// signature of cmd/kron/serve-mcp.Run so cmd/kron/main.go can dispatch
// with a uniform shape:
//
//	err = lsp.Run(os.Args[2:], os.Stdin, os.Stdout, os.Stderr)
//
// It resolves the repo root (optional "-CWD <root>", defaulting to the
// process working directory), builds the store.Reader, and serves the
// LSP protocol over stdio until the parent (editor) closes stdin or a
// genuine transport failure occurs. Logs go to errOut so stdout stays
// pure LSP frames.
func Run(args []string, in io.Reader, out, errOut io.Writer) error {
	root, err := resolveRoot(args)
	if err != nil {
		fmt.Fprintln(errOut, "kron serve-lsp:", err)
		return err
	}

	reader, err := store.NewReader(root)
	if err != nil {
		return fmt.Errorf("kron serve-lsp: %w", err)
	}

	s := &server{reader: reader, anchors: mockAnchorsForFile}

	stream := jsonrpc2.NewHeaderStream(&stdioRWC{in: in, out: out})
	_, conn, _ := protocol.NewServer(context.Background(), s, stream)

	// Block until the connection terminates (parent closes stdin, or an
	// exit notification). conn.Err reports nil for a clean end of stream.
	<-conn.Done()
	return conn.Err()
}

// stdioRWC combines the frame read stream (in) and write stream (out)
// into the single io.ReadWriteCloser that jsonrpc2.NewHeaderStream
// requires. Close is a no-op: the parent owns the underlying OS streams.
type stdioRWC struct {
	in  io.Reader
	out io.Writer
}

func (s *stdioRWC) Read(p []byte) (int, error)  { return s.in.Read(p) }
func (s *stdioRWC) Write(p []byte) (int, error) { return s.out.Write(p) }
func (s *stdioRWC) Close() error                { return nil }

// resolveRoot parses the optional "-CWD <root>" flag pair, matching
// serve-mcp's flag surface so editors spawn both subcommands the same
// way. Absent the flag, the process working directory is the root.
func resolveRoot(args []string) (string, error) {
	if len(args) == 0 {
		return os.Getwd()
	}
	if len(args) == 2 && args[0] == "-CWD" {
		return filepath.Abs(args[1])
	}
	return "", fmt.Errorf("unexpected arguments %v (only -CWD <root> is supported)", args)
}
