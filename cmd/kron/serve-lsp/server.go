package lsp

import (
	"context"
	"sync"

	"go.lsp.dev/protocol"

	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/parser"
	"github.com/xxx/kron/internal/store"
)

const (
	serverName    = "kron"
	serverVersion = "v0.3.0"
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
	docMu   sync.RWMutex
	docs    map[string]openDocument

	// logf is the side-channel used to surface non-protocol events
	// (frontmatter corruption, I/O errors, dangling paths) to the
	// editor's Output panel via LSP window/logMessage. It is wired
	// by Run() once the protocol connection's Client handle is
	// available (Run captures the ctx from protocol.NewServer and
	// uses protocol.ClientFromContext to recover the Client).
	//
	// notify is the side-channel for user-facing popups via
	// window/showMessage. Same wiring pattern as logf; semantically
	// distinct (log = output panel, show = popup).
	//
	// exitFn is the process-exit hook used by the Exit notification
	// (LSP §3.5.4 requires exit code 0 after Shutdown; tests inject
	// a recorder so they can assert the call without terminating).
	//
	// Tests inject recorders to assert on emitted messages; the zero
	// values are no-ops, so a test that does not care about logging
	// can build &server{} without wiring these.
	logf   func(ctx context.Context, typ protocol.MessageType, format string, args ...any)
	notify func(ctx context.Context, typ protocol.MessageType, format string, args ...any)
	exitFn func(code int)
}

type openDocument struct {
	text    string
	version int32
}

// anchorFinder extracts the line-level anchors in a single source file.
// It is the injection seam for the internal anchor scanner: Run()
// wires parser.AnchorsForFile (internal/parser/anchors.go) as the
// concrete implementation, so production traffic and tests share the
// same parsing path. Test fixtures that want a different layout
// (e.g. a synthesized dangling anchor) pass a closure into the
// anchorFinder field directly.
//
// The function takes a context.Context first per architecture.md §〇
// 铁律 #3 (every store/parser fn takes ctx first). Run() passes
// context.Background today; v1.4+ cancellation hooks will forward
// the LSP request context.
type anchorFinder func(ctx context.Context, filePath string) ([]model.Anchor, error)

// Initialize reports the server identity and the capabilities Kron
// actually implements. The SDK targets LSP 3.18 (a superset of the 3.17
// the RFC pinned); hover and definition behave identically across both.
//
// The server synchronizes full document contents so queries see unsaved edits.
func (s *server) Initialize(ctx context.Context, params *protocol.InitializeParams) (*protocol.InitializeResult, error) {
	syncKind := protocol.TextDocumentSyncKindFull
	openClose := true
	return &protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			TextDocumentSync:   &protocol.TextDocumentSyncOptions{OpenClose: &openClose, Change: &syncKind},
			HoverProvider:      protocol.Boolean(true),
			DefinitionProvider: protocol.Boolean(true),
		},
		ServerInfo: protocol.ServerInfo{
			Name:    serverName,
			Version: protocol.NewOptional(serverVersion),
		},
	}, nil
}

func (s *server) DidOpen(ctx context.Context, params *protocol.DidOpenTextDocumentParams) error {
	s.docMu.Lock()
	defer s.docMu.Unlock()
	if s.docs == nil {
		s.docs = make(map[string]openDocument)
	}
	s.docs[params.TextDocument.URI.String()] = openDocument{text: params.TextDocument.Text, version: params.TextDocument.Version}
	return nil
}

func (s *server) DidChange(ctx context.Context, params *protocol.DidChangeTextDocumentParams) error {
	if len(params.ContentChanges) == 0 {
		return nil
	}
	whole, ok := params.ContentChanges[len(params.ContentChanges)-1].(*protocol.TextDocumentContentChangeWholeDocument)
	if !ok {
		return nil
	}
	s.docMu.Lock()
	defer s.docMu.Unlock()
	if s.docs == nil {
		s.docs = make(map[string]openDocument)
	}
	key := params.TextDocument.URI.String()
	current, ok := s.docs[key]
	if ok && params.TextDocument.Version < current.version {
		return nil
	}
	s.docs[key] = openDocument{text: whole.Text, version: params.TextDocument.Version}
	return nil
}

func (s *server) DidClose(ctx context.Context, params *protocol.DidCloseTextDocumentParams) error {
	s.docMu.Lock()
	defer s.docMu.Unlock()
	delete(s.docs, params.TextDocument.URI.String())
	return nil
}

// Shutdown acknowledges the client's shutdown request. No teardown is
// required beyond letting the connection close.
func (s *server) Shutdown(ctx context.Context) error {
	return nil
}

// Exit implements the LSP exit notification. Kron runs under the
// editor's lifetime (no daemon), so the exit notification terminates
// the process directly. The exit code is the parameter — Run wires
// s.exitFn = os.Exit, tests inject a recorder to assert on the
// code without terminating the test process.
func (s *server) Exit(ctx context.Context) error {
	if s.exitFn == nil {
		return nil
	}
	s.exitFn(0)
	return nil // unreachable when s.exitFn is os.Exit
}

// NewServer builds a server with the default anchor scanner
// (parser.AnchorsForFile). It is the production entry point used
// by Run(); tests that need to inject a different scanner should
// build &server{reader: r, anchors: fn} directly (the struct is
// in-package, so tests have full field access).
func NewServer(reader *store.Reader) *server {
	return &server{
		reader:  reader,
		anchors: defaultAnchorsForFile(reader.Root),
	}
}

// defaultAnchorsForFile adapts parser.AnchorsForFile to the
// ctx-taking anchorFinder signature AND threads the reader's repo
// root so the returned Anchor.FilePath values are normalised to the
// "repo-relative + forward slash" form (RFC 2026-10-08-path-
// conventions.md §2.1). v1.3 ignores ctx (parser.AnchorsForFile has
// no cancellation hook); v1.4+ may pass it through.
func defaultAnchorsForFile(root string) anchorFinder {
	return func(_ context.Context, filePath string) ([]model.Anchor, error) {
		return parser.AnchorsForFile(root, filePath)
	}
}

// text-document position's line, or ("", false) when the position is
// not on an "@kron:intent <slug>" line (or the file cannot be read).
// LSP Position.Line is 0-indexed; parser line numbers are 1-indexed.
//
// Why AnchorsForFile (not SlugsForFile): SlugsForFile dedups by slug
// and discards LineNumber. LSP hover/definition matches by "cursor
// line = anchor line", so we need every anchor entry preserved
// (parser.AnchorsForFile contract — RFC 2026-10-08-md-anchors §4.4).
func (s *server) anchorSlugAt(ctx context.Context, td *protocol.TextDocumentPositionParams) (string, bool) {
	path := td.TextDocument.URI.FsPath()
	s.docMu.RLock()
	doc, open := s.docs[td.TextDocument.URI.String()]
	s.docMu.RUnlock()
	var anchors []model.Anchor
	var err error
	if open {
		anchors, err = parser.AnchorsFromText(s.reader.Root, path, doc.text)
	} else {
		anchors, err = s.anchors(ctx, path)
	}
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
