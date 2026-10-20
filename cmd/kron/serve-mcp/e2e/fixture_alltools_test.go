// Fixture-driven MCP smoke test: drives all 18 registered tools over the real
// stdio JSON-RPC transport against a COPY of the shared integration fixture at
// <repo>/testdata/kron-project. The copy is important: write tools (add /
// update / delete / restore / assume_*) mutate the project, and the committed
// fixture must stay pristine.
//
// Gated by KRON_E2E like the rest of this package:
//
//	KRON_E2E=1 go test -v ./cmd/kron/serve-mcp/e2e/ -run TestE2E_SharedFixtureAllTools
package e2e

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// expectedTools is the registry contract the GUI, CLI and LSP all bind to.
var expectedTools = []string{
	"kron_add",
	"kron_assume_check",
	"kron_assume_create",
	"kron_assume_delete",
	"kron_assume_get",
	"kron_assume_list",
	"kron_assume_update",
	"kron_delete",
	"kron_get",
	"kron_impact",
	"kron_init",
	"kron_intent_density",
	"kron_lint",
	"kron_list",
	"kron_restore",
	"kron_stale",
	"kron_tree",
	"kron_update",
}

// callToolRaw is like session.callTool but tolerates tool-level errors: it
// returns the structured payload, the isError flag and the text fallback so a
// test can assert on failure paths without aborting.
func callToolRaw(t *testing.T, s *session, name string, args map[string]any) (map[string]any, bool, string) {
	t.Helper()
	s.send("tools/call", map[string]any{"name": name, "arguments": args}, 2)
	raw := s.recv()
	var env struct {
		StructuredContent map[string]any `json:"structuredContent"`
		Content           []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	require.NoError(t, json.Unmarshal(raw, &env), "decode tools/call envelope")
	text := ""
	if len(env.Content) > 0 {
		text = env.Content[0].Text
	}
	return env.StructuredContent, env.IsError, text
}

// copyFixture copies the committed fixture into a writable temp dir.
func copyFixture(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	require.NoError(t, err, "copy fixture %s -> %s", src, dst)
}

func TestE2E_SharedFixtureAllTools(t *testing.T) {
	fixture := filepath.Join(repoRoot(t), "testdata", "kron-project")
	if _, err := os.Stat(filepath.Join(fixture, ".kron", "intents")); err != nil {
		t.Skipf("shared fixture missing at %s: %v", fixture, err)
	}
	project := filepath.Join(t.TempDir(), "project")
	copyFixture(t, fixture, project)

	s := newSession(t, project)
	defer s.close()
	s.initialize()

	// ---- tools/list: the registry contract --------------------------
	t.Run("ToolsList", func(t *testing.T) {
		s.send("tools/list", nil, 3)
		raw := s.recv()
		var out struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		}
		require.NoError(t, json.Unmarshal(raw, &out))
		got := make([]string, 0, len(out.Tools))
		for _, tool := range out.Tools {
			got = append(got, tool.Name)
		}
		sort.Strings(got)
		assert.Equal(t, expectedTools, got, "registered MCP tool names")
		t.Logf("tools/list: %d tools", len(got))
	})

	// ---- read tools -------------------------------------------------
	t.Run("KronList", func(t *testing.T) {
		out := s.callTool("kron_list", map[string]any{})
		intents, ok := out["intents"].([]any)
		require.True(t, ok, "intents must be an array: %v", out)
		assert.Len(t, intents, 8, "5 leaf intents + 3 node README intents")
	})

	t.Run("KronTree", func(t *testing.T) {
		out := s.callTool("kron_tree", map[string]any{})
		root, ok := out["root"].(map[string]any)
		require.True(t, ok, "root must be an object: %v", out)
		children, ok := root["children"].([]any)
		require.True(t, ok, "root.children must be an array")
		assert.Len(t, children, 2, "two top-level groups: auth + payments")
	})

	t.Run("KronGet", func(t *testing.T) {
		out := s.callTool("kron_get", map[string]any{"slug": "auth/jwt"})
		intent, ok := out["intent"].(map[string]any)
		require.True(t, ok, "intent must be an object: %v", out)
		fm, ok := intent["frontmatter"].(map[string]any)
		require.True(t, ok, "frontmatter must be an object")
		symbols, ok := fm["symbol"].([]any)
		require.True(t, ok, "frontmatter.symbol must be an array")
		assert.Contains(t, symbols, "auth.SignIn")
		assert.NotEmpty(t, intent["body"], "body rendered from markdown")
	})

	t.Run("KronGetRelations", func(t *testing.T) {
		out := s.callTool("kron_get", map[string]any{
			"slug":              "payments/charge",
			"include_relations": true,
		})
		intent, ok := out["intent"].(map[string]any)
		require.True(t, ok, "intent must be an object: %v", out)
		fm, ok := intent["frontmatter"].(map[string]any)
		require.True(t, ok)
		dependsOn, ok := fm["depends_on"].([]any)
		require.True(t, ok, "frontmatter.depends_on must be an array")
		assert.Contains(t, dependsOn, "auth/jwt")
	})

	t.Run("KronLint", func(t *testing.T) {
		out := s.callTool("kron_lint", map[string]any{})
		summary, ok := out["summary"].(map[string]any)
		require.True(t, ok, "summary must be an object: %v", out)
		assert.EqualValues(t, 1, summary["errors"], "one intentional dangling anchor")
		assert.EqualValues(t, 1, summary["warnings"], "one intentional severity override")
		assert.Equal(t, false, out["passed"])
	})

	t.Run("KronStale", func(t *testing.T) {
		out := s.callTool("kron_stale", map[string]any{})
		_, ok := out["superseded_candidates"].([]any)
		assert.True(t, ok, "superseded_candidates must be an array: %v", out)
		_, ok = out["expired_assumptions"].([]any)
		assert.True(t, ok, "expired_assumptions must be an array")
	})

	t.Run("KronImpact", func(t *testing.T) {
		out := s.callTool("kron_impact", map[string]any{"slug": "auth/jwt"})
		intent, ok := out["intent"].(map[string]any)
		require.True(t, ok, "intent must be an object: %v", out)
		assert.Equal(t, "auth/jwt", intent["slug"])
		codeAnchors, ok := out["incoming_anchors_code"].([]any)
		require.True(t, ok, "incoming_anchors_code must be an array")
		assert.NotEmpty(t, codeAnchors, "src/auth/jwt.go anchors auth/jwt")
		references, ok := out["references"].([]any)
		require.True(t, ok, "references must be an array")
		assert.Contains(t, references, "auth/session", "session lists auth/jwt in references")
	})

	t.Run("KronIntentDensity", func(t *testing.T) {
		out := s.callTool("kron_intent_density", map[string]any{})
		assert.NotEmpty(t, out, "density payload must not be empty")
		t.Logf("density payload keys: %d", len(out))
	})

	// ---- assumption tools ------------------------------------------
	t.Run("KronAssumeList", func(t *testing.T) {
		out := s.callTool("kron_assume_list", map[string]any{})
		list, ok := out["assumptions"].([]any)
		require.True(t, ok, "assumptions must be an array")
		assert.Len(t, list, 2, "single-region + idempotency-key")
	})

	t.Run("KronAssumeGet", func(t *testing.T) {
		out := s.callTool("kron_assume_get", map[string]any{"id": "single-region"})
		detail, ok := out["assumption"].(map[string]any)
		require.True(t, ok, "assumption must be an object: %v", out)
		assert.Equal(t, "single-region", detail["id"])
		assert.NotEmpty(t, detail["text"])
		assert.Equal(t, "hard", detail["default_severity"])
	})

	t.Run("KronAssumeCheck", func(t *testing.T) {
		out := s.callTool("kron_assume_check", map[string]any{"file_path": "src/auth/jwt.go"})
		_, ok := out["summary"].(map[string]any)
		assert.True(t, ok, "summary must be an object: %v", out)
		_, ok = out["warnings"].([]any)
		assert.True(t, ok, "warnings must be an array")
	})

	// ---- write tools (on the temp copy) ----------------------------
	t.Run("KronAddGetUpdateDeleteRestore", func(t *testing.T) {
		out := s.callTool("kron_add", map[string]any{
			"slug":   "smoke-test",
			"symbol": "pkg.Smoke",
			"why":    "exercise the write path",
		})
		assert.Equal(t, true, out["ok"], "kron_add ok")

		got := s.callTool("kron_get", map[string]any{"slug": "smoke-test"})
		intent, ok := got["intent"].(map[string]any)
		require.True(t, ok, "added intent must be readable")
		assert.Equal(t, "smoke-test", intent["slug"])

		updated := s.callTool("kron_update", map[string]any{
			"slug":       "smoke-test",
			"status":     "active",
			"references": []string{"auth/jwt"},
		})
		assert.Equal(t, true, updated["ok"], "kron_update ok")

		deleted := s.callTool("kron_delete", map[string]any{"slug": "smoke-test"})
		assert.Equal(t, true, deleted["ok"], "kron_delete ok")
		assert.NotEmpty(t, deleted["trashed_path"], "delete moves to .trash")

		restored := s.callTool("kron_restore", map[string]any{"slug": "smoke-test"})
		assert.Equal(t, true, restored["ok"], "kron_restore ok")
	})

	t.Run("KronInitIdempotent", func(t *testing.T) {
		out := s.callTool("kron_init", map[string]any{})
		assert.Equal(t, true, out["ok"])
		assert.Equal(t, true, out["already_initialised"], "re-init of an existing project is a no-op")
	})

	t.Run("KronAssumeCRUD", func(t *testing.T) {
		created := s.callTool("kron_assume_create", map[string]any{
			"id":               "smoke-assumption",
			"text":             "smoke test assumption",
			"default_severity": "soft",
		})
		assert.Equal(t, true, created["ok"], "kron_assume_create ok")

		updated := s.callTool("kron_assume_update", map[string]any{
			"id":   "smoke-assumption",
			"text": "smoke test assumption (updated)",
		})
		assert.Equal(t, true, updated["ok"], "kron_assume_update ok")

		got := s.callTool("kron_assume_get", map[string]any{"id": "smoke-assumption"})
		detail, ok := got["assumption"].(map[string]any)
		require.True(t, ok)
		assert.Contains(t, detail["text"], "updated")

		deleted := s.callTool("kron_assume_delete", map[string]any{"id": "smoke-assumption"})
		assert.Equal(t, true, deleted["ok"], "kron_assume_delete ok")
	})

	// ---- error paths ----------------------------------------------
	t.Run("ErrorPaths", func(t *testing.T) {
		_, isErr, text := callToolRaw(t, s, "kron_get", map[string]any{"slug": "does/not/exist"})
		assert.True(t, isErr, "get missing slug must be a tool error")
		assert.NotEmpty(t, text)

		_, isErr, _ = callToolRaw(t, s, "kron_assume_get", map[string]any{"id": "missing-assumption"})
		assert.True(t, isErr, "assume_get missing id must be a tool error")

		_, isErr, _ = callToolRaw(t, s, "kron_add", map[string]any{"slug": "auth/jwt"})
		assert.True(t, isErr, "add duplicate slug must be a tool error")

		_, isErr, _ = callToolRaw(t, s, "kron_delete", map[string]any{"slug": "does/not/exist"})
		assert.True(t, isErr, "delete missing slug must be a tool error")
	})
}
