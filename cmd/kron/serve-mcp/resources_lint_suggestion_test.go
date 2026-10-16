package servemcp

// C-1 (resources/list + resources/read against kron://intents/{slug})
// and C-5 (LintDiag.suggestion populated end-to-end) wire-level
// tests. Both run against the in-memory testSession (testkit.go)
// which mirrors the production server's registration path.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResources_TemplateAdvertised (C-1): after a fresh
// newTestSession, the client can list resource templates and finds
// the kron://intents/{slug} entry. Verifies the template is
// registered AND the URITemplate field is the published string.
func TestResources_TemplateAdvertised(t *testing.T) {
	ts := newTestSession(t, t.TempDir())
	defer ts.Cleanup()

	res, err := ts.Session.ListResourceTemplates(context.Background(), nil)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.NotEmpty(t, res.ResourceTemplates, "expected at least one resource template to be advertised")

	var found bool
	for _, tmpl := range res.ResourceTemplates {
		if tmpl.URITemplate == "kron://intents/{+slug}" {
			found = true
			assert.Equal(t, "kron-intent", tmpl.Name)
			assert.Equal(t, "text/markdown", tmpl.MIMEType)
			break
		}
	}
	assert.True(t, found, "kron://intents/{slug} template not found in %+v", res.ResourceTemplates)
}

// TestResources_ReadHappyPath (C-1): reading kron://intents/<slug>
// for a slug that exists on disk returns the re-serialised markdown
// (frontmatter + body). Verifies the per-read handler resolves the
// slug via store.Reader and round-trips through
// parser.SerializeMarkdown.
func TestResources_ReadHappyPath(t *testing.T) {
	root := t.TempDir()
	seedIntent(t, root, ".kron/intents/auth/jwt.md", sampleIntentWire)

	ts := newTestSession(t, root)
	defer ts.Cleanup()

	res, err := ts.Session.ReadResource(context.Background(), &mcp.ReadResourceParams{
		URI: "kron://intents/auth/jwt",
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Len(t, res.Contents, 1)

	body := res.Contents[0].Text
	assert.Equal(t, "kron://intents/auth/jwt", res.Contents[0].URI)
	assert.Equal(t, "text/markdown", res.Contents[0].MIMEType)
	// The body must include the frontmatter sentinel + a key from
	// the seeded file. We don't pin the exact bytes (yaml.Marshal
	// is stable but the field order is implementation-defined);
	// pinning the sentinel and a unique seed value is enough to
	// prove "loaded and re-emitted, not just an empty document".
	assert.Contains(t, body, "<!-- kron:frontmatter -->")
	assert.Contains(t, body, "<!-- /kron:frontmatter -->")
	assert.Contains(t, body, "@test", "expected seeded created_by to round-trip through marshal")
}

// TestResources_ReadNotFound (C-1): reading a slug that doesn't
// exist on disk returns an MCP "not found" error (NOT a wrapped
// "store: load" error). Verifies the handler maps
// model.ErrIntentNotFound to mcp.ResourceNotFoundError so the wire
// contract is clean.
func TestResources_ReadNotFound(t *testing.T) {
	ts := newTestSession(t, t.TempDir())
	defer ts.Cleanup()

	_, err := ts.Session.ReadResource(context.Background(), &mcp.ReadResourceParams{
		URI: "kron://intents/does-not-exist",
	})
	require.Error(t, err, "expected a not-found error for missing slug")
	// The SDK surfaces ResourceNotFoundError as a JSON-RPC error
	// with a specific code; asserting on the wrapped message keeps
	// the test resilient to message wording changes while still
	// proving we hit the not-found path.
	assert.True(t,
		strings.Contains(err.Error(), "not found") ||
			strings.Contains(err.Error(), "not-found"),
		"expected not-found error, got %v", err)
}

// TestResources_ReadMalformedURI (C-1): reading a URI that doesn't
// match the kron://intents/ prefix returns not-found. Guards the
// "URI must be one we registered" branch of the handler.
func TestResources_ReadMalformedURI(t *testing.T) {
	ts := newTestSession(t, t.TempDir())
	defer ts.Cleanup()

	_, err := ts.Session.ReadResource(context.Background(), &mcp.ReadResourceParams{
		URI: "file:///etc/passwd",
	})
	require.Error(t, err)
}

// TestLint_SuggestionPropagates (C-5): the kron_lint tool result
// carries a `suggestion` field on every diag whose rule has a
// non-empty SuggestionFor entry. We seed a single dangling anchor
// (A-class) so the diag is well-formed, and assert the wire-level
// `suggestion` matches the catalog text.
func TestLint_SuggestionPropagates(t *testing.T) {
	root := t.TempDir()
	// Dangling code anchor — the simplest A-class trigger.
	writeFileAt(t, root, "main.go", "// @kron:intent auth/never-existed\n")

	ts := newTestSession(t, root)
	defer ts.Cleanup()

	res, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "kron_lint",
		Arguments: map[string]any{},
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.False(t, res.IsError, "kron_lint should not fail at the tool level")

	// Decode the structured output: the tool returns LintOutput.
	// structuredInto (handlers_test.go) handles the
	// map[string]interface{} representation the SDK returns.
	var out LintOutput
	require.NoError(t, structuredInto(res, &out))
	require.NotEmpty(t, out.Errors)

	var saw bool
	for _, d := range out.Errors {
		if d.Rule == "anchor-dangling" {
			saw = true
			assert.NotEmpty(t, d.Suggestion, "anchor-dangling must surface a suggestion")
			assert.Contains(t, d.Suggestion, ".kron/intents/<slug>.md")
		}
	}
	assert.True(t, saw, "expected an anchor-dangling diag, got %+v", out.Errors)
}

// writeFileAt writes content to <root>/<rel>, creating parent dirs.
// Mirrors the shape of seedIntent (testdata_helpers_test.go) but is
// used here for non-intent files (e.g. main.go) where seedIntent's
// intent-shaped content is not appropriate.
func writeFileAt(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
}
