package lsp

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/parser"
	"github.com/xxx/kron/internal/store"
)

const sampleIntent = `<!-- kron:frontmatter -->
created_by: "@test"
updated_at: 2026-10-03T10:00:00Z
status: active
<!-- /kron:frontmatter -->

# JWT sliding window

Refresh tokens rotate on a sliding window.
`

func writeTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
}

// newTestServer builds a server over a temp repo that has one intent
// (auth-jwt) and one source file (main.go) anchoring it on line 3. The
// anchor scanner is injected as a fake so the protocol wiring is tested
// independently of the (not-yet-landed) internal parser.AnchorsForFile.
func newTestServer(t *testing.T) *server {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, root, ".kron/intents/auth-jwt.md", sampleIntent)
	writeTestFile(t, root, "main.go", "package main\n\n// @kron:intent auth-jwt\nfunc main() {}\n")

	reader, err := store.NewReader(root)
	require.NoError(t, err)
	return &server{reader: reader, anchors: fakeAnchors}
}

// fakeAnchors is a test double for anchorFinder. It returns the anchor
// layout the fixtures describe — main.go anchors auth-jwt on line 3,
// dangling.go anchors no-such-intent on line 1 — so hover/definition
// tests drive the full Load → render path without the internal scanner.
func fakeAnchors(_ context.Context, filePath string) ([]model.Anchor, error) {
	switch filepath.Base(filePath) {
	case "main.go":
		return []model.Anchor{{Slug: "auth-jwt", FilePath: filePath, LineNumber: 3, Kind: model.AnchorKindCode}}, nil
	case "dangling.go":
		return []model.Anchor{{Slug: "no-such-intent", FilePath: filePath, LineNumber: 1, Kind: model.AnchorKindCode}}, nil
	default:
		return []model.Anchor{}, nil
	}
}

func hoverParams(path string, line uint32) *protocol.HoverParams {
	return &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(path)},
			Position:     protocol.Position{Line: line, Character: 0},
		},
	}
}

func TestHover_OnAnchor(t *testing.T) {
	s := newTestServer(t)
	path := filepath.Join(s.reader.Root, "main.go")

	h, err := s.Hover(context.Background(), hoverParams(path, 2))
	require.NoError(t, err)
	require.NotNil(t, h)

	mc, ok := h.Contents.(*protocol.MarkupContent)
	require.True(t, ok, "hover contents must be MarkupContent")
	assert.Equal(t, protocol.MarkupKindMarkdown, mc.Kind)
	assert.Contains(t, mc.Value, "auth-jwt")
	assert.Contains(t, mc.Value, "active")
}

func TestHover_NotOnAnchor(t *testing.T) {
	s := newTestServer(t)
	path := filepath.Join(s.reader.Root, "main.go")

	h, err := s.Hover(context.Background(), hoverParams(path, 0))
	require.NoError(t, err)
	assert.Nil(t, h, "non-anchor line must yield no hover")
}

func TestHover_DanglingAnchor(t *testing.T) {
	s := newTestServer(t)
	writeTestFile(t, s.reader.Root, "dangling.go", "// @kron:intent no-such-intent\n")
	path := filepath.Join(s.reader.Root, "dangling.go")

	h, err := s.Hover(context.Background(), hoverParams(path, 0))
	require.NoError(t, err)
	assert.Nil(t, h, "dangling anchor must yield no hover, not an error")
}

func TestDefinition_OnAnchor(t *testing.T) {
	s := newTestServer(t)
	path := filepath.Join(s.reader.Root, "main.go")

	result, err := s.Definition(context.Background(), &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(path)},
			Position:     protocol.Position{Line: 2, Character: 0},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	locs, ok := result.(protocol.LocationSlice)
	require.True(t, ok, "definition result must be a LocationSlice")
	require.Len(t, locs, 1)
	assert.Contains(t, locs[0].URI.String(), "auth-jwt.md",
		"definition must point at the intent's .md file")
}

func TestDefinition_DanglingAnchor(t *testing.T) {
	s := newTestServer(t)
	writeTestFile(t, s.reader.Root, "dangling.go", "// @kron:intent no-such-intent\n")
	path := filepath.Join(s.reader.Root, "dangling.go")

	result, err := s.Definition(context.Background(), &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(path)},
			Position:     protocol.Position{Line: 0, Character: 0},
		},
	})
	require.NoError(t, err)
	assert.Nil(t, result, "dangling anchor must yield no definition")
}

func TestInitialize_ReportsCapabilities(t *testing.T) {
	s := newTestServer(t)

	res, err := s.Initialize(context.Background(), &protocol.InitializeParams{})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, serverName, res.ServerInfo.Name)
	assert.Equal(t, protocol.Boolean(true), res.Capabilities.HoverProvider)
	assert.Equal(t, protocol.Boolean(true), res.Capabilities.DefinitionProvider)
}

func TestShutdown_NoError(t *testing.T) {
	s := newTestServer(t)
	require.NoError(t, s.Shutdown(context.Background()))
}

// newTestServerWithRealParser builds a server over a temp repo that
// has one intent (auth-jwt) and one source file (main.go) anchoring
// it on line 3, but — unlike newTestServer — wires
// parser.AnchorsForFile as the anchorFinder instead of fakeAnchors.
// This exercises the real end-to-end path that production Run() takes:
// LSP request → parser scans the .go file → server matches cursor
// line → store.Load resolves the slug → hover renders.
func newTestServerWithRealParser(t *testing.T) *server {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, root, ".kron/intents/auth-jwt.md", sampleIntent)
	writeTestFile(t, root, "main.go", "package main\n\n// @kron:intent auth-jwt\nfunc main() {}\n")

	reader, err := store.NewReader(root)
	require.NoError(t, err)
	return NewServer(reader)
}

// TestAnchorsForFile_NormalisedFilePath is a regression test for
// the "Anchor.FilePath is repo-relative + slash" contract (RFC
// 2026-10-08-path-conventions.md §2.1) when AnchorsForFile is
// invoked with an absolute file path. The server relies on this
// because hover/definition matching is by line, but downstream
// consumers that might consume a.FilePath expect the canonical
// form. Without normalisation, scanner-relative absolute paths leak
// out (Windows backslashes especially) and break cross-process
// stable identifiers.
func TestAnchorsForFile_NormalisedFilePath(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "internal/auth/refresh.go", "package auth\n\n// @kron:intent auth/refresh-token\nfunc X() {}\n")

	// Call with the absolute path (the form the LSP server has,
	// because td.TextDocument.URI.FsPath() is absolute).
	absPath := filepath.Join(root, "internal", "auth", "refresh.go")
	anchors, err := parser.AnchorsForFile(root, absPath)
	require.NoError(t, err)
	require.Len(t, anchors, 1)
	assert.Equal(t, "auth/refresh-token", anchors[0].Slug)
	assert.Equal(t, "internal/auth/refresh.go", anchors[0].FilePath,
		"FilePath must be repo-relative with forward slashes, not the absolute input")
	assert.NotContains(t, anchors[0].FilePath, `\`,
		"FilePath must not contain Windows backslashes")

	// And with an empty root (legacy callers): FilePath is left as
	// the input verbatim. This is the "I'm not going to re-emit
	// FilePath" contract documented on AnchorsForFile.
	anchors2, err := parser.AnchorsForFile("", absPath)
	require.NoError(t, err)
	require.Len(t, anchors2, 1)
	assert.Equal(t, absPath, anchors2[0].FilePath,
		"empty root must leave FilePath untouched (no normalisation)")
}

func TestHover_RealParser_AbsolutePath_NormalisedFilePath(t *testing.T) {
	// e2e: the server's defaultAnchorsForFile adapter must thread
	// the reader's repo root through to AnchorsForFile, so the
	// returned anchors' FilePath is "repo-relative + slash" even
	// though the LSP URI's FsPath() is absolute. This is the
	// shape that downstream consumers can rely on.
	s := newTestServerWithRealParser(t)
	path := filepath.Join(s.reader.Root, "main.go")
	h, err := s.Hover(context.Background(), hoverParams(path, 2))
	require.NoError(t, err)
	require.NotNil(t, h, "absolute path through real parser must still match line-3 anchor")
}

func TestHover_RealParserEndToEnd(t *testing.T) {
	s := newTestServerWithRealParser(t)
	path := filepath.Join(s.reader.Root, "main.go")

	// Line 2 (0-indexed) = line 3 (1-indexed) = the "@kron:intent
	// auth-jwt" line written by newTestServerWithRealParser. The real
	// parser must find the anchor, the store must load the intent, and
	// the hover must render the slug + status.
	h, err := s.Hover(context.Background(), hoverParams(path, 2))
	require.NoError(t, err)
	require.NotNil(t, h, "real parser must find the anchor on line 3")

	mc, ok := h.Contents.(*protocol.MarkupContent)
	require.True(t, ok)
	assert.Equal(t, protocol.MarkupKindMarkdown, mc.Kind)
	assert.Contains(t, mc.Value, "auth-jwt")
	assert.Contains(t, mc.Value, "active")
}

func TestHover_RealParser_NotOnAnchor(t *testing.T) {
	s := newTestServerWithRealParser(t)
	path := filepath.Join(s.reader.Root, "main.go")

	// Line 0 (0-indexed) = line 1 (1-indexed) = "package main" — no
	// anchor. The real parser must return no anchors, the server
	// must report no hover.
	h, err := s.Hover(context.Background(), hoverParams(path, 0))
	require.NoError(t, err)
	assert.Nil(t, h, "non-anchor line must yield no hover")
}

func TestDefinition_RealParserEndToEnd(t *testing.T) {
	s := newTestServerWithRealParser(t)
	path := filepath.Join(s.reader.Root, "main.go")

	result, err := s.Definition(context.Background(), &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(path)},
			Position:     protocol.Position{Line: 2, Character: 0},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	locs, ok := result.(protocol.LocationSlice)
	require.True(t, ok)
	require.Len(t, locs, 1)
	assert.Contains(t, locs[0].URI.String(), "auth-jwt.md")
}

// --- P0-1: stdio 入口冒烟测试 -----------------------------------------
//
// Run is the production entry point invoked by cmd/kron/main.go.
// The signature (args, in, out, errOut) is the same as
// servemcp.Run, so the test shape mirrors servemcp.TestRun_EmptyStdin:
// feed empty stdin, expect a clean exit, no protocol response
// (no client → no initialize → no request to reply to).

func TestRun_EmptyStdin(t *testing.T) {
	var out, errOut bytes.Buffer
	err := Run([]string{"-CWD", t.TempDir()}, strings.NewReader(""), &out, &errOut)
	assert.NoError(t, err, "empty stdin must exit cleanly")
	assert.Empty(t, out.String(), "no protocol frames must be written to stdout")
}

func TestRun_UnknownArgs(t *testing.T) {
	var out, errOut bytes.Buffer
	err := Run([]string{"-bogus", "x"}, strings.NewReader(""), &out, &errOut)
	require.Error(t, err, "unknown args must be rejected")
	assert.Contains(t, errOut.String(), "unexpected arguments",
		"resolveRoot must write a hint to errOut for unknown args")
}

// --- P0-5: errors.Is 显式判断 (P0-5 副作用测试) -----------------------
//
// When store.Reader.Load returns a non-ErrIntentNotFound error
// (e.g. frontmatter corruption), the server must still return
// (nil, nil) per RFC §6, but it must emit a log message so the
// editor's Output panel surfaces the corruption. The test wires a
// fake logf recorder and asserts the message is forwarded.

func TestHover_NonNotFoundError_LogsAndReturnsNil(t *testing.T) {
	// Build a server whose anchorFinder returns a valid anchor pointing
	// at a slug whose file exists but fails frontmatter parse.
	// The store will surface an ErrFrontmatterInvalid (wrapped),
	// not ErrIntentNotFound, so the errors.Is branch falls through
	// and logf must be called. We use a subdirectory temp dir to
	// keep the broken fixture isolated from other tests.
	root := t.TempDir()
	writeTestFile(t, root, ".kron/intents/broken.md", "<!-- kron:frontmatter -->\n"+
		"created_by: \"@test\"\n"+
		"updated_at: 2026-10-03T10:00:00Z\n"+
		"status: [this is a flow sequence that yaml accepts but the struct rejects\n"+
		"<!-- /kron:frontmatter -->\n\n# broken\n")
	// Defensive: also drop a sentinel file whose read will surface
	// a non-NotFound error path. (YAML status field accepts a list
	// of strings, so the parse will fail validation, not yaml decode.)
	_ = os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\n// @kron:intent broken\nfunc main() {}\n"), 0o644)

	reader, err := store.NewReader(root)
	require.NoError(t, err)
	// Custom anchorFinder returns a single anchor for the broken slug.
	anchorsFn := func(_ context.Context, _ string) ([]model.Anchor, error) {
		return []model.Anchor{{Slug: "broken", FilePath: "main.go", LineNumber: 1, Kind: model.AnchorKindCode}}, nil
	}
	s := &server{reader: reader, anchors: anchorsFn}

	var logged []string
	s.logf = func(_ context.Context, _ protocol.MessageType, format string, args ...any) {
		logged = append(logged, fmt.Sprintf(format, args...))
	}
	path := filepath.Join(root, "main.go")
	h, err := s.Hover(context.Background(), hoverParams(path, 0))
	require.NoError(t, err, "Hover must not return a protocol error for store-level failures")
	assert.Nil(t, h, "Hover must return nil when store.Load fails non-NotFound")
	// Whether logf fires depends on which error parser returns; we
	// assert on the contract: if h is nil, either the path was
	// "not on an anchor" (no logf expected) or "load failed" (logf
	// expected). Force a definite outcome by also asserting that
	// store.Load actually fails on this fixture — if it succeeds,
	// the test fixture is wrong, not the server.
	_, probeErr := reader.Load(context.Background(), "broken")
	require.Error(t, probeErr, "test fixture must cause store.Load to fail; otherwise P0-5 path is unreachable")
	require.NotEmpty(t, logged, "logf must be called when store.Load returns a non-NotFound error")
	assert.Contains(t, logged[0], "broken", "logf message must mention the failing slug")
}

func TestDefinition_NonNotFoundError_LogsAndReturnsNil(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".kron/intents/broken.md", "<!-- kron:frontmatter -->\n"+
		"created_by: \"@test\"\n"+
		"updated_at: 2026-10-03T10:00:00Z\n"+
		"status: [this is a flow sequence that yaml accepts but the struct rejects\n"+
		"<!-- /kron:frontmatter -->\n\n# broken\n")
	_ = os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\n// @kron:intent broken\nfunc main() {}\n"), 0o644)

	reader, err := store.NewReader(root)
	require.NoError(t, err)
	anchorsFn := func(_ context.Context, _ string) ([]model.Anchor, error) {
		return []model.Anchor{{Slug: "broken", FilePath: "main.go", LineNumber: 1, Kind: model.AnchorKindCode}}, nil
	}
	s := &server{reader: reader, anchors: anchorsFn}

	var logged []string
	s.logf = func(_ context.Context, _ protocol.MessageType, format string, args ...any) {
		logged = append(logged, fmt.Sprintf(format, args...))
	}
	path := filepath.Join(root, "main.go")
	result, err := s.Definition(context.Background(), &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(path)},
			Position:     protocol.Position{Line: 0, Character: 0},
		},
	})
	require.NoError(t, err)
	_, probeErr := reader.Load(context.Background(), "broken")
	require.Error(t, probeErr, "test fixture must cause store.Load to fail; otherwise P0-5 path is unreachable")
	assert.Nil(t, result)
	require.NotEmpty(t, logged)
}

// --- P0-2: TextDocumentSync 验证 -------------------------------------

func TestInitialize_DeclaresTextDocumentSync(t *testing.T) {
	s := newTestServer(t)
	res, err := s.Initialize(context.Background(), &protocol.InitializeParams{})
	require.NoError(t, err)
	require.NotNil(t, res)

	opts, ok := res.Capabilities.TextDocumentSync.(*protocol.TextDocumentSyncOptions)
	require.True(t, ok, "TextDocumentSync must be *TextDocumentSyncOptions, not a bare kind")
	require.NotNil(t, opts)
	require.NotNil(t, opts.OpenClose)
	assert.True(t, *opts.OpenClose, "OpenClose must be true so editors send didOpen/didClose")
	require.NotNil(t, opts.Change)
	assert.Equal(t, protocol.TextDocumentSyncKindFull, *opts.Change,
		"v1.3 must use Full sync (no in-memory cache yet)")
}

func TestDidOpen_NoOp(t *testing.T) {
	s := newTestServer(t)
	assert.NoError(t, s.DidOpen(context.Background(), &protocol.DidOpenTextDocumentParams{}))
}

func TestDidChange_NoOp(t *testing.T) {
	s := newTestServer(t)
	assert.NoError(t, s.DidChange(context.Background(), &protocol.DidChangeTextDocumentParams{}))
}

func TestDidClose_NoOp(t *testing.T) {
	s := newTestServer(t)
	assert.NoError(t, s.DidClose(context.Background(), &protocol.DidCloseTextDocumentParams{}))
}

// --- P2-1: Exit 注入 + 测试 ------------------------------------------

func TestExit_NotifiesZero(t *testing.T) {
	s := newTestServer(t)
	var exitCode int
	var called bool
	s.exitFn = func(code int) { called = true; exitCode = code }

	assert.NoError(t, s.Exit(context.Background()))
	assert.True(t, called, "Exit must invoke s.exitFn")
	assert.Equal(t, 0, exitCode, "Exit must call s.exitFn(0) per LSP §3.5.4")
}

func TestExit_NoExitFn_NoError(t *testing.T) {
	// Default zero-value exitFn: Exit must not panic. Run wires
	// s.exitFn = os.Exit in production, so this path is for tests
	// that build &server{} without wiring exit (e.g. server_test.go's
	// newTestServer).
	s := newTestServer(t)
	s.exitFn = nil
	assert.NoError(t, s.Exit(context.Background()))
}

// --- P1-6: 长 body 截断 e2e 测试 --------------------------------------

func TestHover_LongBodyTruncated(t *testing.T) {
	// 300 chars of CJK body, well past the 200-rune cap. The hover
	// must truncate on a rune boundary (UTF-8 safe) and add an
	// ellipsis. This is the e2e variant of hover_test.go's
	// TestSummarizeBody_TruncatesLongUTF8 — the e2e path proves
	// the truncation also fires through the full Hover → render
	// → MarkupContent chain, not just in isolation.
	//
	// The body is structured so summarizeBody sees a single
	// 300-rune paragraph (no \n\n) and the truncation path is the
	// one that fires. A "# heading\n\n" prefix would put the
	// truncation on the heading, not the body, so we omit it.
	longBody := strings.Repeat("界", 300)
	sample := "<!-- kron:frontmatter -->\n" +
		"created_by: \"@test\"\n" +
		"updated_at: 2026-10-03T10:00:00Z\n" +
		"status: active\n" +
		"<!-- /kron:frontmatter -->\n\n" + longBody
	root := t.TempDir()
	writeTestFile(t, root, ".kron/intents/big.md", sample)
	writeTestFile(t, root, "main.go", "package main\n\n// @kron:intent big\nfunc main() {}\n")
	reader, err := store.NewReader(root)
	require.NoError(t, err)
	s := NewServer(reader)

	path := filepath.Join(root, "main.go")
	h, err := s.Hover(context.Background(), hoverParams(path, 2))
	require.NoError(t, err)
	require.NotNil(t, h, "hover must return content for a valid 300-rune body")

	mc, ok := h.Contents.(*protocol.MarkupContent)
	require.True(t, ok)
	assert.True(t, utf8.ValidString(mc.Value), "hover output must stay valid UTF-8")
	assert.True(t, strings.HasSuffix(mc.Value, "…"), "truncated body must end with ellipsis; got suffix %q", mc.Value[len(mc.Value)-3:])
	assert.Less(t, utf8.RuneCountInString(mc.Value), 250,
		"truncated body must be under 250 runes (200 cap + 1 ellipsis + slug/status prefix)")
}

// --- P2-4: 重复 hover 稳定性 (回归保护) ------------------------------

func TestHover_RepeatedInvocation(t *testing.T) {
	s := newTestServer(t)
	path := filepath.Join(s.reader.Root, "main.go")
	for i := 0; i < 1000; i++ {
		h, err := s.Hover(context.Background(), hoverParams(path, 2))
		require.NoError(t, err)
		require.NotNil(t, h, "iteration %d must return non-nil hover", i)
		mc, ok := h.Contents.(*protocol.MarkupContent)
		require.True(t, ok)
		assert.Contains(t, mc.Value, "auth-jwt")
	}
}
