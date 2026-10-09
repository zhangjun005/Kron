package servemcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests cover the 12 v1.1+ handlers (kron_init / add / update /
// delete / restore / assume_check / impact / intent_density / stale /
// lint / list / get) through the in-memory MCP test client defined
// in testkit.go. The test layer is fully end-to-end: it builds a real
// mcp.Server with all 12 tools registered, connects an mcp.Client
// over an in-memory transport, and invokes each tool via
// session.CallTool. Asserts decode either the structured content
// (success) or the IsError flag (tool-level error).
//
// Note: the v1.1 tests used to pipe raw JSON-RPC lines through Run
// (stdio). v1.2 delegates the protocol layer to the official SDK, so
// the test layer uses an in-memory transport + client — the same
// machinery Claude Desktop / Cursor will use to talk to the real
// stdio server in production. The stdio entry point itself
// (Run(args, in, out, errOut)) is exercised by a subprocess test in
// serve_mcp_test.go (TestRun_StdioProtocolSmoke).

// --- helpers ---------------------------------------------------------

// initRepo creates a temp dir and runs kron_init against it via the
// in-memory MCP client. Returns the dir.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	ts := newTestSession(t, dir)
	defer ts.Cleanup()
	res, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "kron_init",
		Arguments: map[string]any{},
	})
	require.NoError(t, err, "kron_init must succeed")
	require.False(t, res.IsError, "kron_init must not be a tool error")
	return dir
}

// addIntent seeds <dir>/.kron/intents/<slug>.md with the given
// (symbol + body) pair via kron_add over the in-memory MCP client.
func addIntent(t *testing.T, dir, slug, symbol, body string) {
	t.Helper()
	ts := newTestSession(t, dir)
	defer ts.Cleanup()
	args := map[string]any{"slug": slug}
	if symbol != "" {
		args["symbol"] = symbol
	}
	if body != "" {
		args["why"] = body
	}
	res, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "kron_add",
		Arguments: args,
	})
	require.NoError(t, err, "kron_add transport must succeed")
	require.False(t, res.IsError, "kron_add must not be a tool error")
}

// callJSON is the v1.1-era helper retained as a thin shim so the
// rest of the file can keep using `callJSON(t, dir, "<JSON>")` style
// fixtures. Internally it now parses the JSON-RPC envelope, converts
// it to an mcp.CallToolParams, and dispatches via the in-memory
// client. The old raw-bytes-on-stdio semantics are no longer needed
// because the SDK owns the protocol layer.
//
// The returned singleResponse keeps the v1.1 field shape
// (Result/Error/ID) so handlers_relations_test.go (which decodes
// Result.RawMessage back into typed structs) keeps working.
func callJSON(t *testing.T, dir, request string) singleResponse {
	t.Helper()
	var req struct {
		JSONRPC string          `json:"jsonrpc"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
		ID      json.RawMessage `json:"id"`
	}
	require.NoError(t, json.Unmarshal([]byte(request), &req), "test fixture is not valid JSON")

	ts := newTestSession(t, dir)
	defer ts.Cleanup()

	resp := singleResponse{ID: req.ID, JSONRPC: "2.0"}

	var args map[string]any
	if len(req.Params) > 0 {
		require.NoError(t, json.Unmarshal(req.Params, &args), "params must be a JSON object")
	}
	res, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      req.Method,
		Arguments: args,
	})
	if err != nil {
		resp.Error = &struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}{Code: -32603, Message: err.Error()}
		return resp
	}
	if res.IsError {
		var msg string
		if len(res.Content) > 0 {
			if tc, ok := res.Content[0].(*mcp.TextContent); ok {
				msg = tc.Text
			}
		}
		resp.Error = &struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}{Code: -32006, Message: msg}
		return resp
	}
	b, err := json.Marshal(res.StructuredContent)
	require.NoError(t, err, "StructuredContent must marshal cleanly")
	resp.Result = b
	return resp
}

// structuredInto unmarshals CallToolResult.StructuredContent (which
// is `any`) into the supplied typed pointer. It is the v1.2
// equivalent of the v1.1 dance of `json.Unmarshal(resp.Result, &r)`.
func structuredInto(res *mcp.CallToolResult, out any) error {
	b, err := json.Marshal(res.StructuredContent)
	if err != nil {
		return fmt.Errorf("marshal structured content: %w", err)
	}
	return json.Unmarshal(b, out)
}

// --- kron_init -------------------------------------------------------

func TestInit_HappyPath(t *testing.T) {
	dir := t.TempDir()
	ts := newTestSession(t, dir)
	defer ts.Cleanup()
	res, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "kron_init",
		Arguments: map[string]any{},
	})
	require.NoError(t, err)
	require.False(t, res.IsError, "kron_init must succeed")
	var r InitOutput
	require.NoError(t, structuredInto(res, &r))
	assert.True(t, r.OK)
	assert.Equal(t, ".kron/intents", r.IntentsDir)
	assert.False(t, r.AlreadyInitialised)

	info, err := os.Stat(filepath.Join(dir, ".kron"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestInit_Idempotent(t *testing.T) {
	dir := initRepo(t)
	ts := newTestSession(t, dir)
	defer ts.Cleanup()
	res, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "kron_init",
		Arguments: map[string]any{},
	})
	require.NoError(t, err)
	var r InitOutput
	require.NoError(t, structuredInto(res, &r))
	assert.True(t, r.OK)
	assert.True(t, r.AlreadyInitialised, "second init must report already_initialised=true")
}

// --- kron_add --------------------------------------------------------

func TestAdd_HappyPath(t *testing.T) {
	dir := initRepo(t)
	ts := newTestSession(t, dir)
	defer ts.Cleanup()
	res, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "kron_add",
		Arguments: map[string]any{"slug": "auth/jwt", "symbol": "auth.JWT", "why": "sliding window"},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
	var r AddOutput
	require.NoError(t, structuredInto(res, &r))
	assert.True(t, r.OK)
	assert.Equal(t, ".kron/intents/auth/jwt.md", r.Path)
	_, err = os.Stat(filepath.Join(dir, ".kron/intents/auth/jwt.md"))
	assert.NoError(t, err)
}

func TestAdd_Conflict(t *testing.T) {
	dir := initRepo(t)
	addIntent(t, dir, "auth/jwt", "", "")
	ts := newTestSession(t, dir)
	defer ts.Cleanup()
	res, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "kron_add",
		Arguments: map[string]any{"slug": "auth/jwt"},
	})
	require.NoError(t, err)
	assert.True(t, res.IsError, "kron_add on existing slug must fail")
}

func TestAdd_InvalidSlug(t *testing.T) {
	dir := initRepo(t)
	ts := newTestSession(t, dir)
	defer ts.Cleanup()
	res, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "kron_add",
		Arguments: map[string]any{"slug": "Bad/Uppercase"},
	})
	require.NoError(t, err)
	assert.True(t, res.IsError, "invalid slug must fail")
}

// TestAdd_NodeKind covers RFC 2026-10-08-writer-readme-symmetry §3.5:
// the MCP kron_add tool must accept a "kind" field that selects
// the on-disk physical form, and must return the path the file
// actually landed at (not the canonical <slug>.md).
func TestAdd_NodeKind(t *testing.T) {
	dir := initRepo(t)
	ts := newTestSession(t, dir)
	defer ts.Cleanup()
	res, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "kron_add",
		Arguments: map[string]any{"slug": "auth", "kind": "node"},
	})
	require.NoError(t, err)
	require.False(t, res.IsError, "kron_add with kind=node must succeed")
	var r AddOutput
	require.NoError(t, structuredInto(res, &r))
	assert.True(t, r.OK)
	assert.Equal(t, ".kron/intents/auth/README.md", r.Path,
		"kind=node must report the README path, not the canonical <slug>.md")

	// File on disk is the README.
	_, err = os.Stat(filepath.Join(dir, ".kron/intents/auth/README.md"))
	assert.NoError(t, err)
	// Canonical <slug>.md must NOT exist.
	_, err = os.Stat(filepath.Join(dir, ".kron/intents/auth.md"))
	assert.True(t, os.IsNotExist(err), "kind=node must not write <slug>.md")
}

// TestAdd_InvalidKind verifies the schema-side validation:
// unknown kind values fail the call rather than silently falling
// back to leaf.
func TestAdd_InvalidKind(t *testing.T) {
	dir := initRepo(t)
	ts := newTestSession(t, dir)
	defer ts.Cleanup()
	res, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "kron_add",
		Arguments: map[string]any{"slug": "auth", "kind": "tree"},
	})
	require.NoError(t, err)
	assert.True(t, res.IsError, "unknown kind must fail")
}

func TestAdd_NotInitialised(t *testing.T) {
	dir := t.TempDir()
	ts := newTestSession(t, dir)
	defer ts.Cleanup()
	res, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "kron_add",
		Arguments: map[string]any{"slug": "foo"},
	})
	require.NoError(t, err)
	assert.True(t, res.IsError, "add before init must fail")
}

// --- kron_update -----------------------------------------------------

func TestUpdate_BodyOnly(t *testing.T) {
	dir := initRepo(t)
	addIntent(t, dir, "auth/jwt", "", "")
	ts := newTestSession(t, dir)
	defer ts.Cleanup()
	body := "# replaced"
	res, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "kron_update",
		Arguments: map[string]any{"slug": "auth/jwt", "body": &body},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)

	gres, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "kron_get",
		Arguments: map[string]any{"slug": "auth/jwt"},
	})
	require.NoError(t, err)
	var get GetOutput
	require.NoError(t, structuredInto(gres, &get))
	assert.Contains(t, get.Intent.Body, "# replaced")
}

func TestUpdate_StatusPatch(t *testing.T) {
	dir := initRepo(t)
	addIntent(t, dir, "auth/jwt", "", "")
	ts := newTestSession(t, dir)
	defer ts.Cleanup()
	status := "active"
	res, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "kron_update",
		Arguments: map[string]any{"slug": "auth/jwt", "status": &status},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
	gres, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "kron_get",
		Arguments: map[string]any{"slug": "auth/jwt"},
	})
	require.NoError(t, err)
	var get GetOutput
	require.NoError(t, structuredInto(gres, &get))
	assert.Equal(t, "active", get.Intent.Frontmatter.Status)
}

func TestUpdate_NotFound(t *testing.T) {
	dir := initRepo(t)
	ts := newTestSession(t, dir)
	defer ts.Cleanup()
	res, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "kron_update",
		Arguments: map[string]any{"slug": "missing/x", "body": "x"},
	})
	require.NoError(t, err)
	assert.True(t, res.IsError)
}

func TestUpdate_InvalidStatus(t *testing.T) {
	dir := initRepo(t)
	addIntent(t, dir, "auth/jwt", "", "")
	ts := newTestSession(t, dir)
	defer ts.Cleanup()
	banana := "banana"
	res, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "kron_update",
		Arguments: map[string]any{"slug": "auth/jwt", "status": &banana},
	})
	require.NoError(t, err)
	assert.True(t, res.IsError, "invalid status enum must fail")
}

// --- kron_delete + kron_restore --------------------------------------

func TestDeleteThenRestore(t *testing.T) {
	dir := initRepo(t)
	addIntent(t, dir, "auth/jwt", "", "")

	ts := newTestSession(t, dir)
	defer ts.Cleanup()

	d, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "kron_delete",
		Arguments: map[string]any{"slug": "auth/jwt"},
	})
	require.NoError(t, err)
	require.False(t, d.IsError)
	var dr DeleteOutput
	require.NoError(t, structuredInto(d, &dr))
	assert.True(t, dr.OK)
	assert.Contains(t, dr.TrashedPath, ".kron/.trash/auth/jwt.md")

	g, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "kron_get",
		Arguments: map[string]any{"slug": "auth/jwt"},
	})
	require.NoError(t, err)
	assert.True(t, g.IsError, "kron_get on trashed slug must fail")

	r, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "kron_restore",
		Arguments: map[string]any{"slug": "auth/jwt"},
	})
	require.NoError(t, err)
	require.False(t, r.IsError)

	g2, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "kron_get",
		Arguments: map[string]any{"slug": "auth/jwt"},
	})
	require.NoError(t, err)
	assert.False(t, g2.IsError, "kron_get after restore must succeed")
}

func TestDelete_NotFound(t *testing.T) {
	dir := initRepo(t)
	ts := newTestSession(t, dir)
	defer ts.Cleanup()
	res, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "kron_delete",
		Arguments: map[string]any{"slug": "missing/x"},
	})
	require.NoError(t, err)
	assert.True(t, res.IsError)
}

// --- kron_assume_check -----------------------------------------------

func TestAssumeCheck_EmptyStore(t *testing.T) {
	dir := initRepo(t)
	ts := newTestSession(t, dir)
	defer ts.Cleanup()
	res, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "kron_assume_check",
		Arguments: map[string]any{},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
	var r AssumeCheckOutput
	require.NoError(t, structuredInto(res, &r))
	assert.Empty(t, r.Warnings)
}

func TestAssumeCheck_ExpiredHard(t *testing.T) {
	dir := initRepo(t)
	expired := time.Now().AddDate(0, 0, -30).UTC().Format(time.RFC3339)
	yml := `<!-- kron:frontmatter -->
created_by: "@test"
updated_at: 2026-09-01T10:00:00Z
status: active
assumptions:
  - id: single-region
    text: "single region"
    severity: hard
    expires_at: "` + expired + `"
<!-- /kron:frontmatter -->

# test
`
	full := filepath.Join(dir, ".kron/intents/expired.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(yml), 0o644))

	ts := newTestSession(t, dir)
	defer ts.Cleanup()
	res, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "kron_assume_check",
		Arguments: map[string]any{},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
	var r AssumeCheckOutput
	require.NoError(t, structuredInto(res, &r))
	assert.Equal(t, 1, r.Summary.Hard)
	assert.Equal(t, 1, r.Summary.Expired)
	require.Len(t, r.Warnings, 1)
	assert.Equal(t, "hard", r.Warnings[0].Severity)
	require.NotNil(t, r.Warnings[0].DaysRemaining)
	assert.Less(t, *r.Warnings[0].DaysRemaining, 0)
}

// --- kron_assume_check B-3 (RFC 2026-10-08-assumptions-standalone) ---

func TestAssumeCheck_UsesRegistry_PreservesIntentSeverity(t *testing.T) {
	// B-3: assume_check reads the registry for text, but uses the
	// intent's own severity (per-intent override) as the
	// authoritative severity. The registry's default_severity is
	// reported as a separate field.
	dir := initRepo(t)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".kron/assumptions"), 0o755))
	// Registry: default_severity=soft.
	assumptionYAML := `<!-- kron:frontmatter -->
id: single-region
text: REGISTRY TEXT (preferred)
default_severity: soft
created_by: "@a"
updated_at: "2026-09-22T10:00:00Z"
<!-- /kron:frontmatter -->
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".kron/assumptions/single-region.md"), []byte(assumptionYAML), 0o644))
	// Intent: severity=hard (override), rationale present.
	intentYAML := `<!-- kron:frontmatter -->
created_by: "@a"
updated_at: 2026-10-01T10:00:00Z
assumptions:
  - id: single-region
    text: INLINE TEXT (overridden by registry)
    severity: hard
    rationale: "跨 region 时 token 失效爆炸, 必须保证单 region 部署"
<!-- /kron:frontmatter -->

# x
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".kron/intents/uses-single-region.md"), []byte(intentYAML), 0o644))

	ts := newTestSession(t, dir)
	defer ts.Cleanup()
	res, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "kron_assume_check",
		Arguments: map[string]any{},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
	var r AssumeCheckOutput
	require.NoError(t, structuredInto(res, &r))
	require.Len(t, r.Warnings, 1)
	w := r.Warnings[0]
	assert.Equal(t, "hard", w.Severity, "per-intent severity wins")
	assert.Equal(t, "soft", w.DefaultSeverity, "registry default reported separately")
	assert.Equal(t, "REGISTRY TEXT (preferred)", w.Text, "registry text preferred over inline")
	assert.Equal(t, "跨 region 时 token 失效爆炸, 必须保证单 region 部署", w.Rationale)
}

func TestListGet_AssumesToWire_B3(t *testing.T) {
	// B-3: kron_list / kron_get return assumesToWire entries with
	// the registry text + default_severity.
	dir := initRepo(t)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".kron/assumptions"), 0o755))
	assumptionYAML := `<!-- kron:frontmatter -->
id: redis
text: REGISTRY-REDIS-TEXT
default_severity: soft
created_by: "@a"
updated_at: "2026-09-22T10:00:00Z"
<!-- /kron:frontmatter -->
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".kron/assumptions/redis.md"), []byte(assumptionYAML), 0o644))
	intentYAML := `<!-- kron:frontmatter -->
created_by: "@a"
updated_at: 2026-10-01T10:00:00Z
assumptions:
  - id: redis
    text: INLINE TEXT
    severity: hard
    rationale: "token revocation must remain fast under load"
<!-- /kron:frontmatter -->

# uses-redis
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".kron/intents/uses-redis.md"), []byte(intentYAML), 0o644))

	ts := newTestSession(t, dir)
	defer ts.Cleanup()
	res, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "kron_get",
		Arguments: map[string]any{"slug": "uses-redis"},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
	var r GetOutput
	require.NoError(t, structuredInto(res, &r))
	require.Len(t, r.Intent.Frontmatter.Assumptions, 1)
	a := r.Intent.Frontmatter.Assumptions[0]
	assert.Equal(t, "redis", a.ID)
	assert.Equal(t, "REGISTRY-REDIS-TEXT", a.Text, "wire prefers registry text")
	assert.Equal(t, "hard", a.Severity)
	assert.Equal(t, "soft", a.DefaultSeverity, "wire reports default_severity")
	assert.Equal(t, "token revocation must remain fast under load", a.Rationale)
}

// --- kron_impact -----------------------------------------------------

func TestImpact_NotFound(t *testing.T) {
	dir := initRepo(t)
	ts := newTestSession(t, dir)
	defer ts.Cleanup()
	res, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "kron_impact",
		Arguments: map[string]any{"slug": "missing/x"},
	})
	require.NoError(t, err)
	assert.True(t, res.IsError)
}

func TestImpact_HappyPath(t *testing.T) {
	dir := initRepo(t)
	addIntent(t, dir, "auth/jwt", "auth.JWT", "")
	src := filepath.Join(dir, "auth.go")
	require.NoError(t, os.WriteFile(src, []byte("package auth\n// @kron:intent auth/jwt\n"), 0o644))

	ts := newTestSession(t, dir)
	defer ts.Cleanup()
	res, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "kron_impact",
		Arguments: map[string]any{"slug": "auth/jwt"},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
	var r ImpactOutput
	require.NoError(t, structuredInto(res, &r))
	assert.Equal(t, "auth/jwt", r.Intent.Slug)
	require.Len(t, r.IncomingAnchors, 1)
	assert.Equal(t, "auth.go", r.IncomingAnchors[0].FilePath)
	// RFC 2026-10-08-md-anchors.md §2.5: kind is carried through
	// transparently from model.Anchor.Kind to the wire. Code
	// anchors come back as "code".
	assert.Equal(t, "code", r.IncomingAnchors[0].Kind)
}

// TestImpact_MarkdownAnchorKind locks the v1.2 kind dispatch:
// a markdown anchor on disk surfaces as kind="markdown" on the
// wire (not as kind="code" and not as a separate envelope).
func TestImpact_MarkdownAnchorKind(t *testing.T) {
	dir := initRepo(t)
	addIntent(t, dir, "auth/jwt", "", "")
	// Drop a .md file with an anchor pointing at our slug.
	md := "// @kron:intent auth/jwt\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte(md), 0o644))

	ts := newTestSession(t, dir)
	defer ts.Cleanup()
	res, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "kron_impact",
		Arguments: map[string]any{"slug": "auth/jwt"},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
	var r ImpactOutput
	require.NoError(t, structuredInto(res, &r))
	require.Len(t, r.IncomingAnchors, 1)
	assert.Equal(t, "README.md", r.IncomingAnchors[0].FilePath)
	assert.Equal(t, "markdown", r.IncomingAnchors[0].Kind)
}

// --- kron_intent_density --------------------------------------------

func TestIntentDensity_EmptyRepo(t *testing.T) {
	dir := initRepo(t)
	ts := newTestSession(t, dir)
	defer ts.Cleanup()
	res, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "kron_intent_density",
		Arguments: map[string]any{},
	})
	require.NoError(t, err)
	var r IntentDensityOutput
	require.NoError(t, structuredInto(res, &r))
	assert.Equal(t, 0, r.TotalIntents)
	assert.Equal(t, 0, r.TotalAnchors)
}

// TestIntentDensity_CountsMarkdownAnchors locks RFC
// 2026-10-08-md-anchors.md §2.5: the TotalAnchors and
// IntentsWithAnchors aggregates must include markdown anchors
// in addition to code anchors. Pre-v1.2 the handler only called
// ScanAnchors, so a doc-only anchor would not move the
// with-anchor count and the intent would falsely appear in
// IntentsWithoutAnchors.
func TestIntentDensity_CountsMarkdownAnchors(t *testing.T) {
	dir := initRepo(t)
	addIntent(t, dir, "auth/jwt", "", "")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"),
		[]byte("// @kron:intent auth/jwt\n"), 0o644))

	ts := newTestSession(t, dir)
	defer ts.Cleanup()
	res, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "kron_intent_density",
		Arguments: map[string]any{},
	})
	require.NoError(t, err)
	var r IntentDensityOutput
	require.NoError(t, structuredInto(res, &r))
	assert.Equal(t, 1, r.TotalIntents)
	assert.Equal(t, 1, r.TotalAnchors, "markdown anchor must count")
	assert.Equal(t, 1, r.Coverage.IntentsWithAnchors)
	assert.Empty(t, r.Coverage.IntentsWithoutAnchors,
		"auth/jwt must NOT be in without-anchors when the only anchor is markdown")
}

// --- kron_stale ------------------------------------------------------

func TestStale_EmptyStore(t *testing.T) {
	dir := initRepo(t)
	ts := newTestSession(t, dir)
	defer ts.Cleanup()
	res, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "kron_stale",
		Arguments: map[string]any{"days_threshold": 30},
	})
	require.NoError(t, err)
	var r StaleOutput
	require.NoError(t, structuredInto(res, &r))
	assert.Empty(t, r.SupersededCandidates)
	assert.Empty(t, r.ExpiredAssumptions)
}

func TestStale_DefaultThreshold(t *testing.T) {
	dir := initRepo(t)
	ts := newTestSession(t, dir)
	defer ts.Cleanup()
	res, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "kron_stale",
		Arguments: map[string]any{},
	})
	require.NoError(t, err)
	assert.False(t, res.IsError, "default threshold should be 90 and not fail")
}

// --- AllToolsRegistered ---------------------------------------------

func TestServer_AllToolsRegistered(t *testing.T) {
	dir := initRepo(t)
	// Map of tool name → minimal valid arguments that pass the SDK's
	// JSON-schema input validator (type: string, required). Tools with
	// no required params get nil (empty args). Tools with optional params
	// only also get nil.
	toolArgs := map[string]map[string]any{
		"kron_lint":           nil,
		"kron_list":           nil,
		"kron_init":           nil,
		"kron_intent_density": nil,
		"kron_stale":          nil,
		// Required-param tools: provide minimal valid values.
		"kron_get":          {"slug": "does-not-exist"},
		"kron_add":          {"slug": "smoke-test-add"},
		"kron_update":       {"slug": "smoke-test-ux", "body": "x"},
		"kron_delete":       {"slug": "smoke-test-delete"},
		"kron_restore":      {"slug": "smoke-test-delete"},
		"kron_assume_check": {"file_path": ""}, // empty is valid; SDK schema allows ""
		"kron_impact":       {"slug": "does-not-exist"},
	}
	for _, name := range []string{
		"kron_lint", "kron_list", "kron_get",
		"kron_init", "kron_add", "kron_update", "kron_delete", "kron_restore",
		"kron_assume_check", "kron_impact", "kron_intent_density", "kron_stale",
	} {
		t.Run(name, func(t *testing.T) {
			ts := newTestSession(t, dir)
			defer ts.Cleanup()
			_, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
				Name:      name,
				Arguments: toolArgs[name],
			})
			// Registration failure surfaces as a transport error (SDK
			// cannot call an unregistered method). Other errors
			// (invalid params, not found) mean the tool IS registered;
			// we're just testing registration, not handler correctness.
			require.NoError(t, err, "%s must be registered", name)
		})
	}
}
