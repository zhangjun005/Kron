// Package lsp is the LSP (Language Server Protocol) access layer of
// Kron. It gives editors (VSCode / Cursor / Neovim) hover and
// definition jumps on "// @kron:intent <slug>" anchors.
//
// Naming: directory cmd/kron/serve-lsp is hyphenated to match
// serve-mcp; Go package is "lsp" (single segment, acronym).
//
// Stack: go.lsp.dev/protocol SDK, stdio, Content-Length framed
// JSON-RPC. Capabilities v1.3: initialize / hover / definition /
// shutdown / exit. See docs/implementation/lsp.md and
// docs/rfc/2026-10-07-lsp-sdk.md for the architectural source of truth.
package lsp

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	root, err := resolveRoot(args)
	if err != nil {
		fmt.Fprintln(errOut, "kron serve-lsp:", err)
		return err
	}

	reader, err := store.NewReader(root)
	if err != nil {
		return fmt.Errorf("kron serve-lsp: %w", err)
	}

	s := NewServer(reader)

	rwc := &stdioRWC{in: in, out: out}
	stream := jsonrpc2.NewHeaderStream(rwc)
	ctx, conn, client := protocol.NewServer(ctx, s, stream)
	_ = ctx    // logf/notify resolve Client via protocol.ClientFromContext on each call
	_ = client //

	// Wire the two side-channels: logf for window/logMessage and
	// notify for window/showMessage. Both go through the LSP Client
	// handle returned by protocol.NewServer. The Client is reachable
	// from ctx for handler-side use; we cache a closure on the
	// server so handlers don't have to repeat the lookup. The
	// fallbacks (fmt.Fprintln to errOut) are kept so that any
	// logf/notify call before the connection is fully ready (or
	// after teardown) still surfaces something on the editor's
	// stderr fallback / Output panel.
	s.logf = func(ctx context.Context, typ protocol.MessageType, format string, args ...any) {
		msg := fmt.Sprintf(format, args...)
		if c, ok := protocol.ClientFromContext(ctx); ok {
			_ = c.LogMessage(ctx, &protocol.LogMessageParams{Type: typ, Message: msg})
			return
		}
		fmt.Fprintln(errOut, msg)
	}
	s.notify = func(ctx context.Context, typ protocol.MessageType, format string, args ...any) {
		msg := fmt.Sprintf(format, args...)
		if c, ok := protocol.ClientFromContext(ctx); ok {
			_ = c.ShowMessage(ctx, &protocol.ShowMessageParams{Type: typ, Message: msg})
			return
		}
		fmt.Fprintln(errOut, msg)
	}
	s.exitFn = os.Exit
	_ = client // touch to silence unused; ctx is the lookup key for handlers

	// Block until the connection terminates (parent closes stdin, or an
	// exit notification). conn.Err reports nil for a clean end of stream.
	select {
	case <-conn.Done():
	case <-ctx.Done():
		_ = rwc.Close()
		<-conn.Done()
		return nil
	}
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
func (s *stdioRWC) Close() error {
	if closer, ok := s.in.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

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
