package servemcp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests cover the 9 handlers added in the v1.1 P1 batch
// (kron_init / add / update / delete / restore / assume_check /
// impact / intent_density / stale). Each test exercises the wire
// envelope end-to-end: JSON-RPC request → tool handler → store
// round-trip → JSON-RPC response. The protocol-layer tests
// (parse errors, notifications, etc.) live in serve_mcp_test.go.

// --- helpers ---------------------------------------------------------

// initRepo creates a temp dir + a one-intent fixture used by the
// handler-specific tests below. Returns the dir.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Run kron_init.
	input := `{"jsonrpc":"2.0","method":"kron_init","id":"i"}` + "\n"
	var out, errOut bytes.Buffer
	require.NoError(t, Run([]string{"-CWD", dir}, strings.NewReader(input), &out, &errOut))
	require.Empty(t, errOut)
	return dir
}

// addIntent seeds <dir>/.kron/intents/<slug>.md with the given
// (symbol + body) pair. Uses the wire protocol end-to-end so the
// test exercises the same code path an AI agent would.
func addIntent(t *testing.T, dir, slug, symbol, body string) {
	t.Helper()
	params := map[string]string{"slug": slug}
	if symbol != "" {
		params["symbol"] = symbol
	}
	if body != "" {
		params["why"] = body
	}
	pjson, _ := json.Marshal(params)
	input := `{"jsonrpc":"2.0","method":"kron_add","params":` + string(pjson) + `,"id":"a"}` + "\n"
	var out, errOut bytes.Buffer
	require.NoError(t, Run([]string{"-CWD", dir}, strings.NewReader(input), &out, &errOut))
	require.Empty(t, errOut, "add stderr: %s", errOut.String())
	resp := decodeSingleResponse(t, out.String())
	assert.Nil(t, resp.Error, "add error: %v", resp.Error)
}

// callJSON runs one JSON-RPC request through Run and returns the
// decoded single-line response.
func callJSON(t *testing.T, dir, request string) singleResponse {
	t.Helper()
	var out, errOut bytes.Buffer
	require.NoError(t, Run([]string{"-CWD", dir}, strings.NewReader(request+"\n"), &out, &errOut))
	require.Empty(t, errOut, "stderr: %s", errOut.String())
	return decodeSingleResponse(t, out.String())
}

// --- kron_init -------------------------------------------------------

func TestInit_HappyPath(t *testing.T) {
	dir := t.TempDir()
	resp := callJSON(t, dir, `{"jsonrpc":"2.0","method":"kron_init","id":1}`)
	require.Nil(t, resp.Error)
	var r struct {
		OK                 bool   `json:"ok"`
		IntentsDir         string `json:"intents_dir"`
		AlreadyInitialised bool   `json:"already_initialised"`
	}
	require.NoError(t, json.Unmarshal(resp.Result, &r))
	assert.True(t, r.OK)
	assert.Equal(t, ".kron/intents", r.IntentsDir)
	assert.False(t, r.AlreadyInitialised)

	// .kron/ tree exists on disk.
	info, err := os.Stat(filepath.Join(dir, ".kron"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestInit_Idempotent(t *testing.T) {
	dir := initRepo(t)
	resp := callJSON(t, dir, `{"jsonrpc":"2.0","method":"kron_init","id":2}`)
	require.Nil(t, resp.Error)
	var r struct {
		OK                 bool `json:"ok"`
		AlreadyInitialised bool `json:"already_initialised"`
	}
	require.NoError(t, json.Unmarshal(resp.Result, &r))
	assert.True(t, r.OK)
	assert.True(t, r.AlreadyInitialised, "second init must report already_initialised=true")
}

// --- kron_add --------------------------------------------------------

func TestAdd_HappyPath(t *testing.T) {
	dir := initRepo(t)
	resp := callJSON(t, dir, `{"jsonrpc":"2.0","method":"kron_add","params":{"slug":"auth/jwt","symbol":"auth.JWT","why":"sliding window"},"id":1}`)
	require.Nil(t, resp.Error)
	var r struct {
		OK   bool   `json:"ok"`
		Path string `json:"path"`
	}
	require.NoError(t, json.Unmarshal(resp.Result, &r))
	assert.True(t, r.OK)
	assert.Equal(t, ".kron/intents/auth/jwt.md", r.Path)
	// File exists on disk.
	_, err := os.Stat(filepath.Join(dir, ".kron/intents/auth/jwt.md"))
	assert.NoError(t, err)
}

func TestAdd_Conflict(t *testing.T) {
	dir := initRepo(t)
	addIntent(t, dir, "auth/jwt", "", "")
	resp := callJSON(t, dir, `{"jsonrpc":"2.0","method":"kron_add","params":{"slug":"auth/jwt"},"id":2}`)
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32005, resp.Error.Code, "kron_add on existing slug returns Conflict")
}

func TestAdd_InvalidSlug(t *testing.T) {
	dir := initRepo(t)
	resp := callJSON(t, dir, `{"jsonrpc":"2.0","method":"kron_add","params":{"slug":"Bad/Uppercase"},"id":1}`)
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32602, resp.Error.Code, "invalid slug returns Invalid params")
}

func TestAdd_NotInitialised(t *testing.T) {
	dir := t.TempDir() // no kron_init
	resp := callJSON(t, dir, `{"jsonrpc":"2.0","method":"kron_add","params":{"slug":"foo"},"id":1}`)
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32006, resp.Error.Code, "add before init returns Unprocessable")
}

// --- kron_update -----------------------------------------------------

func TestUpdate_BodyOnly(t *testing.T) {
	dir := initRepo(t)
	addIntent(t, dir, "auth/jwt", "", "")
	resp := callJSON(t, dir, `{"jsonrpc":"2.0","method":"kron_update","params":{"slug":"auth/jwt","body":"# replaced"},"id":1}`)
	require.Nil(t, resp.Error)
	// Verify body changed.
	gr := callJSON(t, dir, `{"jsonrpc":"2.0","method":"kron_get","params":{"slug":"auth/jwt"},"id":2}`)
	require.Nil(t, gr.Error)
	var get struct {
		Intent struct {
			Body string `json:"body"`
		} `json:"intent"`
	}
	require.NoError(t, json.Unmarshal(gr.Result, &get))
	assert.Contains(t, get.Intent.Body, "# replaced")
}

func TestUpdate_StatusPatch(t *testing.T) {
	dir := initRepo(t)
	addIntent(t, dir, "auth/jwt", "", "")
	resp := callJSON(t, dir, `{"jsonrpc":"2.0","method":"kron_update","params":{"slug":"auth/jwt","status":"active"},"id":1}`)
	require.Nil(t, resp.Error)
	gr := callJSON(t, dir, `{"jsonrpc":"2.0","method":"kron_get","params":{"slug":"auth/jwt"},"id":2}`)
	require.Nil(t, gr.Error)
	var get struct {
		Intent struct {
			Frontmatter struct {
				Status string `json:"status"`
			} `json:"frontmatter"`
		} `json:"intent"`
	}
	require.NoError(t, json.Unmarshal(gr.Result, &get))
	assert.Equal(t, "active", get.Intent.Frontmatter.Status)
}

func TestUpdate_NotFound(t *testing.T) {
	dir := initRepo(t)
	resp := callJSON(t, dir, `{"jsonrpc":"2.0","method":"kron_update","params":{"slug":"missing/x","body":"x"},"id":1}`)
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32004, resp.Error.Code)
}

func TestUpdate_InvalidStatus(t *testing.T) {
	dir := initRepo(t)
	addIntent(t, dir, "auth/jwt", "", "")
	resp := callJSON(t, dir, `{"jsonrpc":"2.0","method":"kron_update","params":{"slug":"auth/jwt","status":"banana"},"id":1}`)
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32006, resp.Error.Code, "invalid status enum returns Unprocessable")
}

// --- kron_delete + kron_restore --------------------------------------

func TestDeleteThenRestore(t *testing.T) {
	dir := initRepo(t)
	addIntent(t, dir, "auth/jwt", "", "")

	// Delete.
	d := callJSON(t, dir, `{"jsonrpc":"2.0","method":"kron_delete","params":{"slug":"auth/jwt"},"id":1}`)
	require.Nil(t, d.Error)
	var dr struct {
		OK          bool   `json:"ok"`
		TrashedPath string `json:"trashed_path"`
	}
	require.NoError(t, json.Unmarshal(d.Result, &dr))
	assert.True(t, dr.OK)
	assert.Contains(t, dr.TrashedPath, ".kron/.trash/auth/jwt.md")

	// Get now 404s.
	g := callJSON(t, dir, `{"jsonrpc":"2.0","method":"kron_get","params":{"slug":"auth/jwt"},"id":2}`)
	require.NotNil(t, g.Error)
	assert.Equal(t, -32004, g.Error.Code)

	// Restore.
	r := callJSON(t, dir, `{"jsonrpc":"2.0","method":"kron_restore","params":{"slug":"auth/jwt"},"id":3}`)
	require.Nil(t, r.Error)

	// Get succeeds again.
	g2 := callJSON(t, dir, `{"jsonrpc":"2.0","method":"kron_get","params":{"slug":"auth/jwt"},"id":4}`)
	require.Nil(t, g2.Error)
}

func TestDelete_NotFound(t *testing.T) {
	dir := initRepo(t)
	resp := callJSON(t, dir, `{"jsonrpc":"2.0","method":"kron_delete","params":{"slug":"missing/x"},"id":1}`)
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32004, resp.Error.Code)
}

// --- kron_assume_check -----------------------------------------------

func TestAssumeCheck_EmptyStore(t *testing.T) {
	dir := initRepo(t)
	resp := callJSON(t, dir, `{"jsonrpc":"2.0","method":"kron_assume_check","id":1}`)
	require.Nil(t, resp.Error)
	var r struct {
		Warnings []map[string]any `json:"warnings"`
		Summary  struct {
			Hard, Soft, Expired int
		} `json:"summary"`
	}
	require.NoError(t, json.Unmarshal(resp.Result, &r))
	assert.Empty(t, r.Warnings)
}

func TestAssumeCheck_ExpiredHard(t *testing.T) {
	dir := initRepo(t)
	// Write a hand-crafted intent with an expired hard assumption.
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

	resp := callJSON(t, dir, `{"jsonrpc":"2.0","method":"kron_assume_check","id":1}`)
	require.Nil(t, resp.Error)
	var r struct {
		Summary struct {
			Hard, Expired int
		} `json:"summary"`
		Warnings []struct {
			Severity      string `json:"severity"`
			DaysRemaining *int   `json:"days_remaining"`
		} `json:"warnings"`
	}
	require.NoError(t, json.Unmarshal(resp.Result, &r))
	assert.Equal(t, 1, r.Summary.Hard)
	assert.Equal(t, 1, r.Summary.Expired)
	require.Len(t, r.Warnings, 1)
	assert.Equal(t, "hard", r.Warnings[0].Severity)
	require.NotNil(t, r.Warnings[0].DaysRemaining)
	assert.Less(t, *r.Warnings[0].DaysRemaining, 0)
}

// --- kron_impact -----------------------------------------------------

func TestImpact_NotFound(t *testing.T) {
	dir := initRepo(t)
	resp := callJSON(t, dir, `{"jsonrpc":"2.0","method":"kron_impact","params":{"slug":"missing/x"},"id":1}`)
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32004, resp.Error.Code)
}

func TestImpact_HappyPath(t *testing.T) {
	dir := initRepo(t)
	addIntent(t, dir, "auth/jwt", "auth.JWT", "")
	// Anchor one source file to it.
	src := filepath.Join(dir, "auth.go")
	require.NoError(t, os.WriteFile(src, []byte("package auth\n// @kron:intent auth/jwt\n"), 0o644))

	resp := callJSON(t, dir, `{"jsonrpc":"2.0","method":"kron_impact","params":{"slug":"auth/jwt"},"id":1}`)
	require.Nil(t, resp.Error)
	var r struct {
		Intent           map[string]any   `json:"intent"`
		IncomingAnchors  []map[string]any `json:"incoming_anchors"`
		DependsOnIntents []string         `json:"depends_on_intents"`
	}
	require.NoError(t, json.Unmarshal(resp.Result, &r))
	assert.Equal(t, "auth/jwt", r.Intent["slug"])
	require.Len(t, r.IncomingAnchors, 1)
	assert.Equal(t, "auth.go", r.IncomingAnchors[0]["file_path"])
}

// --- kron_intent_density --------------------------------------------

func TestIntentDensity_EmptyRepo(t *testing.T) {
	dir := initRepo(t)
	resp := callJSON(t, dir, `{"jsonrpc":"2.0","method":"kron_intent_density","id":1}`)
	require.Nil(t, resp.Error)
	var r struct {
		TotalIntents int `json:"total_intents"`
		TotalAnchors int `json:"total_anchors"`
		Coverage     struct {
			IntentsWithAnchors    int      `json:"intents_with_anchors"`
			IntentsWithoutAnchors []string `json:"intents_without_anchors"`
		} `json:"coverage"`
		FilesWithoutIntent []string `json:"files_without_intent"`
	}
	require.NoError(t, json.Unmarshal(resp.Result, &r))
	assert.Equal(t, 0, r.TotalIntents)
	assert.Equal(t, 0, r.TotalAnchors)
}

// --- kron_stale ------------------------------------------------------

func TestStale_EmptyStore(t *testing.T) {
	dir := initRepo(t)
	resp := callJSON(t, dir, `{"jsonrpc":"2.0","method":"kron_stale","params":{"days_threshold":30},"id":1}`)
	require.Nil(t, resp.Error)
	var r struct {
		SupersededCandidates []string `json:"superseded_candidates"`
		ExpiredAssumptions   []any    `json:"expired_assumptions"`
	}
	require.NoError(t, json.Unmarshal(resp.Result, &r))
	assert.Empty(t, r.SupersededCandidates)
	assert.Empty(t, r.ExpiredAssumptions)
}

func TestStale_DefaultThreshold(t *testing.T) {
	dir := initRepo(t)
	// days_threshold omitted → defaults to 90.
	resp := callJSON(t, dir, `{"jsonrpc":"2.0","method":"kron_stale","id":1}`)
	require.Nil(t, resp.Error)
}

// --- AllToolsRegistered ---------------------------------------------

// TestServer_AllToolsRegistered is a smoke test that every named
// tool returns a well-formed response (not a method-not-found error).
// The detailed per-tool behaviour is covered by the per-handler
// tests above; this is the cheap "the registry actually has them
// all" check.
func TestServer_AllToolsRegistered(t *testing.T) {
	dir := initRepo(t)
	tools := []string{
		"kron_lint", "kron_list", "kron_get",
		"kron_init", "kron_add", "kron_update", "kron_delete", "kron_restore",
		"kron_assume_check", "kron_impact", "kron_intent_density", "kron_stale",
	}
	for _, name := range tools {
		t.Run(name, func(t *testing.T) {
			resp := callJSON(t, dir, `{"jsonrpc":"2.0","method":"`+name+`","id":"x"}`)
			// All 12 tools must be registered. A method-not-found would
			// come through as -32601; we want neither that nor a parse
			// error. The body may be empty (e.g. kron_list on empty repo)
			// or an error envelope for bad params; either is acceptable
			// as long as it's not a "method not found".
			if resp.Error != nil {
				assert.NotEqual(t, -32601, resp.Error.Code, "%s must be registered", name)
			}
		})
	}
}
