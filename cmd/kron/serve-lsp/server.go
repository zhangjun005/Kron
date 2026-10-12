package lsp

import (
	"context"
	"os"

	"go.lsp.dev/protocol"

	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/store"
)

const (
	serverName    = "kron"
	serverVersion = "v1.3.0"
)

// server implements the LSP Server interface by embedding
// protocol.UnimplementedServer, so only the v1.3 capabilities
// (initialize / hover / definition / shutdown / exit) are overridden.
// Every other request returns "not implemented" and every other
// notification is ignored, per UnimplementedServer's contract.
type server struct {
	protocol.UnimplementedServer
	reader  *store.Reader
	anchors anchorFinder
}

// anchorFinder extracts the line-level anchors in a single source file.
// It is the injection seam for the internal anchor scanner: v1.3 wires
// mockAnchorsForFile (see mock.go) until parser.AnchorsForFile lands on
// master, at which point Run() switches to the real implementation.
type anchorFinder func(filePath string) ([]model.Anchor, error)

// Initialize reports the server identity and the capabilities Kron
// actually implements. The SDK targets LSP 3.18 (a superset of the 3.17
// the RFC pinned); hover and definition behave identically across both.
func (s *server) Initialize(ctx context.Context, params *protocol.InitializeParams) (*protocol.InitializeResult, error) {
	return &protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			HoverProvider:      protocol.Boolean(true),
			DefinitionProvider: protocol.Boolean(true),
		},
		ServerInfo: protocol.ServerInfo{
			Name:    serverName,
			Version: protocol.NewOptional(serverVersion),
		},
	}, nil
}

// Shutdown acknowledges the client's shutdown request. No teardown is
// required beyond letting the connection close.
func (s *server) Shutdown(ctx context.Context) error {
	return nil
}

// Exit implements the LSP exit notification. Kron runs under the
// editor's lifetime (no daemon), so the exit notification terminates
// the process directly.
func (s *server) Exit(ctx context.Context) error {
	os.Exit(0)
	return nil // unreachable; os.Exit never returns
}

// anchorSlugAt returns the intent slug anchored on the given
// text-document position's line, or ("", false) when the position is
// not on an "@kron:intent <slug>" line (or the file cannot be read).
// LSP Position.Line is 0-indexed; parser line numbers are 1-indexed.
func (s *server) anchorSlugAt(td *protocol.TextDocumentPositionParams) (string, bool) {
	anchors, err := s.anchors(td.TextDocument.URI.FsPath())
	if err != nil {
		return "", false
	}
	line := int(td.Position.Line) + 1
	for _, a := range anchors {
		if a.LineNumber == line {
			return a.Slug, true
		}
	}
	return "", false
}
