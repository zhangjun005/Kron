package lsp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/xxx/kron/internal/model"
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
func fakeAnchors(filePath string) ([]model.Anchor, error) {
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
