package servemcp

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests cover the 3 v1.3+ prompts (kron-implement /
// kron-drift-check / kron-review) registered via
// registerPrompts. The handlers render user-role messages from
// internal/ data; we assert (a) the contract (one message, role
// user, non-empty text) and (b) the data plumbing (a known
// feature keyword matches the seeded intent; a missing-argument
// error path returns the documented message).
//
// We intentionally do NOT lock the rendered text in a golden
// file — the format is allowed to evolve (e.g. adding a "Sources
// cited" block) without re-recording the test. What we DO lock is
// the structural shape: section names + key data points.

// TestPrompts_AllRegistered confirms the 3 prompts show up in
// prompts/list. Mirrors TestServer_AllToolsRegistered for tools.
// Uses ListPrompts (not GetPrompt) so we don't trip the
// kron-implement "feature required" validation when we just want
// to verify the prompt is registered.
func TestPrompts_AllRegistered(t *testing.T) {
	ts := newTestSession(t, t.TempDir())
	defer ts.Cleanup()

	res, err := ts.Session.ListPrompts(context.Background(), &mcp.ListPromptsParams{})
	require.NoError(t, err, "prompts/list must succeed")
	got := make(map[string]bool, len(res.Prompts))
	for _, p := range res.Prompts {
		got[p.Name] = true
	}
	for _, name := range []string{"kron-implement", "kron-drift-check", "kron-review"} {
		t.Run(name, func(t *testing.T) {
			assert.True(t, got[name], "expected %s in prompts/list, got %v", name, res.Prompts)
		})
	}
}

// TestPrompts_Implement_MissingFeatureArg asserts the only
// required-arg prompt fails loudly when feature is absent.
// Matches the contract in docs/rfc/2026-10-14-mcp-prompts.md §2.1
// ("`feature` is REQUIRED. Empty / missing → error").
func TestPrompts_Implement_MissingFeatureArg(t *testing.T) {
	ts := newTestSession(t, t.TempDir())
	defer ts.Cleanup()

	_, err := ts.Session.GetPrompt(context.Background(), &mcp.GetPromptParams{
		Name:      "kron-implement",
		Arguments: map[string]string{"feature": ""},
	})
	require.Error(t, err, "kron-implement with empty feature must fail")
	assert.Contains(t, err.Error(), "feature", "error must name the missing argument so the AI can self-correct")
}

// TestPrompts_Implement_MatchAndPrereqs seeds two intents where
// one has a depends_on the other, then asks kron-implement for a
// keyword that only matches the dependent intent. The rendered
// message must mention BOTH slugs (the match + the prerequisite)
// so the AI reads the prereq first.
func TestPrompts_Implement_MatchAndPrereqs(t *testing.T) {
	dir := initRepo(t)

	// Seed: "auth-jwt" (prereq) and "auth-jwt-refresh" (depends_on
	// auth-jwt). The keyword "refresh" only matches the second
	// slug, so the AI should be told to read auth-jwt first.
	addIntent(t, dir, "auth-jwt", "", "JWT validation rules")
	addIntent(t, dir, "auth-jwt-refresh", "", "Token refresh strategy\ndepends on auth-jwt")

	// kron_add's wire shape doesn't expose depends_on, so we
	// patch the on-disk frontmatter directly. Production code
	// would use kron_update for this.
	patchIntentDependsOn(t, dir, "auth-jwt-refresh", []string{"auth-jwt"})

	patchIntentAssumptions(t, dir, "auth-jwt-refresh", []map[string]any{{
		"id": "single-region", "severity": "hard", "rationale": "refresh tokens live in one region",
		"expires_at": "2027-01-01",
	}})

	ts := newTestSession(t, dir)
	defer ts.Cleanup()

	res, err := ts.Session.GetPrompt(context.Background(), &mcp.GetPromptParams{
		Name:      "kron-implement",
		Arguments: map[string]string{"feature": "refresh"},
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Len(t, res.Messages, 1, "kron-implement must render exactly one user message")
	msg := res.Messages[0]
	assert.Equal(t, mcp.Role("user"), msg.Role, "prompt template renders as user role")
	text := textContent(t, msg)
	assert.Contains(t, text, "auth-jwt-refresh", "rendered message must list the matched intent slug")
	assert.Contains(t, text, "auth-jwt", "rendered message must list the matched intent's prerequisite")
	assert.Contains(t, text, "## User Request", "rendered message must keep the documented section names")
	assert.Contains(t, text, "## Matched Intents")
	assert.Contains(t, text, "## Action")
	assert.Contains(t, text, "assumptions:", "implement prompt must surface assumption links")
	assert.Contains(t, text, "single-region", "implement prompt must name the linked assumption")
}

// TestPrompts_Implement_NoMatch seeds nothing and asks for a
// keyword with no match. 0-match is NOT an error (handler still
// returns a message), but the message must guide the AI to
// kron_add a new intent.
func TestPrompts_Implement_NoMatch(t *testing.T) {
	dir := initRepo(t)

	ts := newTestSession(t, dir)
	defer ts.Cleanup()

	res, err := ts.Session.GetPrompt(context.Background(), &mcp.GetPromptParams{
		Name:      "kron-implement",
		Arguments: map[string]string{"feature": "absolutely-no-such-feature-keyword-xyzzy"},
	})
	require.NoError(t, err, "0 match is a legal result, not an error")
	require.Len(t, res.Messages, 1)
	text := textContent(t, res.Messages[0])
	assert.Contains(t, text, "no existing intent matches", "0-match message must guide the AI to kron_add")
}

// TestPrompts_DriftCheck_DefaultAndOverride exercises both the
// default-threshold path (no arg) and the explicit-arg path.
// Asserts the rendered threshold appears in the message so the
// AI sees the value it queried with.
func TestPrompts_DriftCheck_DefaultAndOverride(t *testing.T) {
	dir := initRepo(t)
	addIntent(t, dir, "old-intent", "", "old enough to be stale")

	patchIntentAssumptions(t, dir, "old-intent", []map[string]any{{
		"id": "clock-skew-30s", "severity": "hard", "rationale": "clocks must agree for JWT validation",
		"expires_at": "2020-01-01",
	}})

	ts := newTestSession(t, dir)
	defer ts.Cleanup()

	t.Run("default 90 days", func(t *testing.T) {
		res, err := ts.Session.GetPrompt(context.Background(), &mcp.GetPromptParams{
			Name: "kron-drift-check",
		})
		require.NoError(t, err)
		require.Len(t, res.Messages, 1)
		text := textContent(t, res.Messages[0])
		assert.Contains(t, text, "days_threshold=90", "default threshold must be 90")
		assert.Contains(t, text, "old-intent: clock-skew-30s (expired", "drift prompt must keep intent:assumption context")
	})

	t.Run("override 30 days", func(t *testing.T) {
		res, err := ts.Session.GetPrompt(context.Background(), &mcp.GetPromptParams{
			Name:      "kron-drift-check",
			Arguments: map[string]string{"days_threshold": "30"},
		})
		require.NoError(t, err)
		require.Len(t, res.Messages, 1)
		text := textContent(t, res.Messages[0])
		assert.Contains(t, text, "days_threshold=30", "explicit threshold must be reflected in the message")
	})

	t.Run("garbage arg falls back to default", func(t *testing.T) {
		res, err := ts.Session.GetPrompt(context.Background(), &mcp.GetPromptParams{
			Name:      "kron-drift-check",
			Arguments: map[string]string{"days_threshold": "not-a-number"},
		})
		require.NoError(t, err)
		require.Len(t, res.Messages, 1)
		text := textContent(t, res.Messages[0])
		assert.Contains(t, text, "days_threshold=90", "unparseable threshold must fall back to default")
	})
}

// TestPrompts_Review_StatusFilter exercises the default + override
// path for kron-review. Asserts the in-scope intent list is
// correctly filtered by status so the AI sees the right review
// surface.
func TestPrompts_Review_StatusFilter(t *testing.T) {
	dir := initRepo(t)
	addIntentStatus(t, dir, "active-intent", "active", "active")
	addIntentStatus(t, dir, "draft-intent", "draft body", "draft")

	patchIntentReferences(t, dir, "active-intent", []string{"missing-intent"})

	ts := newTestSession(t, dir)
	defer ts.Cleanup()

	t.Run("default status=active", func(t *testing.T) {
		res, err := ts.Session.GetPrompt(context.Background(), &mcp.GetPromptParams{
			Name: "kron-review",
		})
		require.NoError(t, err)
		require.Len(t, res.Messages, 1)
		text := textContent(t, res.Messages[0])
		assert.Contains(t, text, "status=active", "default status must be 'active'")
		assert.Contains(t, text, "active-intent", "active intent must appear in the in-scope list")
		assert.Contains(t, text, "dangling-reference", "review prompt must group lint findings by rule")
		assert.Contains(t, text, "references non-existent intent", "review prompt must include the finding detail")
		assert.Contains(t, text, "[warning]", "review prompt must include the finding severity")
	})

	t.Run("override status=draft shows draft only", func(t *testing.T) {
		res, err := ts.Session.GetPrompt(context.Background(), &mcp.GetPromptParams{
			Name:      "kron-review",
			Arguments: map[string]string{"status": "draft"},
		})
		require.NoError(t, err)
		require.Len(t, res.Messages, 1)
		text := textContent(t, res.Messages[0])
		assert.Contains(t, text, "status=draft", "explicit status must be reflected")
		assert.Contains(t, text, "draft-intent", "draft filter must include draft intents")
		inScope := text
		if idx := strings.Index(text, "### Findings by rule"); idx >= 0 {
			inScope = text[:idx]
		}
		assert.NotContains(t, inScope, "active-intent", "draft filter must exclude active intents from the in-scope list")
	})
}

// --- helpers ---------------------------------------------------------

// textContent extracts the Text field from a PromptMessage whose
// content is a *mcp.TextContent. Tests use this to assert the
// rendered text in one call; any future PromptMessage that uses
// a different content type (Image / Audio / Resource) will fail
// this helper loudly, which is the right signal to update the
// test fixture.
func textContent(t *testing.T, m *mcp.PromptMessage) string {
	t.Helper()
	require.NotNil(t, m, "PromptMessage is nil")
	tc, ok := m.Content.(*mcp.TextContent)
	require.True(t, ok, "PromptMessage.Content must be *mcp.TextContent, got %T", m.Content)
	return tc.Text
}

// addIntentStatus is a test-only helper that adds an intent with a
// specific status. The standard addIntent helper goes through
// kron_add, which does not set a status by default (status is
// optional frontmatter; omitting it means "no lifecycle tag").
// For kron-review we need active / draft / etc. to be visible, so
// we follow up with kron_update to patch the status field. This
// is the same path users would use in production.
func addIntentStatus(t *testing.T, dir, slug, body, status string) {
	t.Helper()
	addIntent(t, dir, slug, "", body)
	ts := newTestSession(t, dir)
	defer ts.Cleanup()
	res, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "kron_update",
		Arguments: map[string]any{"slug": slug, "status": status},
	})
	require.NoError(t, err, "kron_update must succeed")
	require.False(t, res.IsError, "kron_update must not be a tool error: %v", res.Content)
}

// patchIntentDependsOn is a test-only helper that adds a
// depends_on to an intent. kron_add's wire shape doesn't expose
// depends_on (only assumptions + symbol + body), so we follow up
// with kron_update which DOES accept depends_on as a PATCH field.
// This is the same path users would use in production.
func patchIntentDependsOn(t *testing.T, dir, slug string, dependsOn []string) {
	t.Helper()
	ts := newTestSession(t, dir)
	defer ts.Cleanup()
	res, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "kron_update",
		Arguments: map[string]any{"slug": slug, "depends_on": dependsOn},
	})
	require.NoError(t, err, "kron_update transport must succeed")
	require.False(t, res.IsError, "kron_update must not be a tool error: %v", res.Content)
}

// patchIntentAssumptions attaches assumption links to an intent through
// kron_update (the same path users/agents take in production).
func patchIntentAssumptions(t *testing.T, dir, slug string, assumptions []map[string]any) {
	t.Helper()
	ts := newTestSession(t, dir)
	defer ts.Cleanup()
	res, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "kron_update",
		Arguments: map[string]any{"slug": slug, "assumptions": assumptions},
	})
	require.NoError(t, err, "kron_update transport must succeed")
	require.False(t, res.IsError, "kron_update must not be a tool error: %v", res.Content)
}

// patchIntentReferences adds a references list through kron_update so lint
// findings (dangling-reference) become visible to kron-review.
func patchIntentReferences(t *testing.T, dir, slug string, references []string) {
	t.Helper()
	ts := newTestSession(t, dir)
	defer ts.Cleanup()
	res, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "kron_update",
		Arguments: map[string]any{"slug": slug, "references": references},
	})
	require.NoError(t, err, "kron_update transport must succeed")
	require.False(t, res.IsError, "kron_update must not be a tool error: %v", res.Content)
}
