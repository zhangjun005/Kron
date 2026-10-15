package servemcp

// Tests for the coerce middleware (cmd/kron/serve-mcp/coerce.go).
//
// What this file covers (RFC docs/rfc/2026-10-15-mcp-coerce-middleware.md
// §2.3 / §4.2 / §5):
//
//   - The three known Cursor-bridge type-coercion bugs (B1, B2, B3)
//     flip their values to the schema-declared type so the SDK
//     unmarshals cleanly.
//   - Values that are already the schema-declared type are left
//     alone (no-op middleware on raw stdio).
//   - Values that look like genuine errors (non-standard boolean
//     string, fraction that doesn't fit int, etc.) are NOT
//     rewritten, so the user still gets the SDK's original
//     "type X want Y" error.
//   - The middleware is wired into Run so the end-to-end path
//     (raw stdio → middleware → SDK → handler) accepts bridge
//     shapes for kron_get.include_relations, kron_update.reviewers,
//     and kron_stale.days_threshold.
//
// Out of scope: prompt/get arguments. B4 (drift-check days_threshold
// arriving as a number) is documented in
// docs/snapshot/2026-10-15-McpKnownIssues-CursorBridge.md because
// the SDK's GetPromptParams is typed `map[string]string` and the
// receiving middleware cannot see the raw JSON. The fix path runs
// through the SDK itself.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =====================================================================
// Per-type coerce primitives (RFC §2.3 coerceValue policy)
// =====================================================================

// TestCoerceBool_StringVariants covers the B1 case: Cursor's bridge
// strings the bool ("true"/"false"/"1"/"0"). Only the four
// canonical spellings flip; anything else is left alone so the
// user sees a real "type string want boolean" error rather than
// a silent "yes → true" guess.
func TestCoerceBool_StringVariants(t *testing.T) {
	cases := []struct {
		in      any
		want    any
		rewrote bool
	}{
		{"true", true, true},
		{"false", false, true},
		{"1", true, true},
		{"0", false, true},
		{"True", true, true},   // mixed case, common in YAML
		{"FALSE", false, true}, // uppercase, common in env vars
		// Already a bool: no rewrite.
		{true, true, false},
		{false, false, false},
		// Non-canonical strings: must NOT flip.
		{"yes", "yes", false},
		{"no", "no", false},
		{"", "", false},
		{"on", "on", false},
		// Non-string non-bool (e.g. a number 2): leave alone.
		{float64(2), float64(2), false},
	}
	for _, c := range cases {
		got, did := coerceToBool(c.in)
		assert.Equal(t, c.want, got, "input=%v", c.in)
		assert.Equal(t, c.rewrote, did, "input=%v", c.in)
	}
}

// TestCoerceInt_StringAndFloat covers the B3 case: a string number
// ("30", "-5") flips to a JSON number. Floats with zero fractional
// part (30.0) are left alone — they unmarshal fine into an `int`
// field. Floats with a fractional part (30.5) are NOT narrowed to
// an int, because the client clearly meant a fractional number.
func TestCoerceInt_StringAndFloat(t *testing.T) {
	cases := []struct {
		in      any
		want    any
		rewrote bool
	}{
		// B3: string "30" → 30.
		{"30", float64(30), true},
		{"-5", float64(-5), true},
		// Already an int-shaped float: no rewrite.
		{float64(30), float64(30), false},
		{float64(0), float64(0), false},
		// Fractional: must NOT narrow.
		{float64(30.5), float64(30.5), false},
		// Non-numeric strings: leave alone.
		{"abc", "abc", false},
		{"", "", false},
		{"30.5", "30.5", false}, // not an int string; would parse as float in coerceToNumber
	}
	for _, c := range cases {
		got, did := coerceToInt(c.in)
		assert.Equal(t, c.want, got, "input=%v", c.in)
		assert.Equal(t, c.rewrote, did, "input=%v", c.in)
	}
}

// TestCoerceArray_ObjectWithItem covers the B2 case: Cursor's
// bridge wraps an array as `{"item": x}`. We only flip the
// single-key "item" shape — any other object is left for the SDK
// to reject with a useful error.
func TestCoerceArray_ObjectWithItem(t *testing.T) {
	cases := []struct {
		name    string
		in      any
		want    any
		rewrote bool
	}{
		{
			name:    "B2: single-item object → array",
			in:      map[string]any{"item": "@alice"},
			want:    []any{"@alice"},
			rewrote: true,
		},
		{
			name:    "already array: no rewrite",
			in:      []any{"@alice", "@bob"},
			want:    []any{"@alice", "@bob"},
			rewrote: false,
		},
		{
			name:    "object with multiple keys: not the B2 shape",
			in:      map[string]any{"item": "@alice", "extra": 1},
			want:    map[string]any{"item": "@alice", "extra": 1},
			rewrote: false,
		},
		{
			name:    "object without item key: not the B2 shape",
			in:      map[string]any{"foo": "@alice"},
			want:    map[string]any{"foo": "@alice"},
			rewrote: false,
		},
		{
			name:    "JSON-array string: parse and unwrap",
			in:      `["@alice","@bob"]`,
			want:    []any{"@alice", "@bob"},
			rewrote: true,
		},
		{
			name:    "non-JSON string: leave alone",
			in:      "@alice",
			want:    "@alice",
			rewrote: false,
		},
		{
			name:    "string that starts with [ but isn't valid JSON: leave alone",
			in:      "[not json",
			want:    "[not json",
			rewrote: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, did := coerceToArray(c.in)
			assert.Equal(t, c.want, got)
			assert.Equal(t, c.rewrote, did)
		})
	}
}

// =====================================================================
// Per-field coerceArguments (RFC §2.3 end-to-end on the args blob)
// =====================================================================

// TestCoerceArguments_B1UpdateGet is the canonical B1 repro:
// `kron_get` with `include_relations: "true"` (string) is rewritten
// to `true` (bool). Also covers the raw stdio regression: a real
// bool passes through unchanged.
func TestCoerceArguments_B1UpdateGet(t *testing.T) {
	schema := toolPropertySchemas["kron_get"]
	require.NotEmpty(t, schema, "kron_get must be in the schema map")

	// Cursor shape.
	in := json.RawMessage(`{"slug":"auth/jwt","include_relations":"true"}`)
	out, changed := coerceArguments(in, schema)
	require.True(t, changed)
	// The middleware must not change unrelated fields. Round-trip
	// through map to assert the shape rather than a string-compare
	// (which would be brittle to field reordering).
	var got map[string]any
	require.NoError(t, json.Unmarshal(out, &got))
	assert.Equal(t, "auth/jwt", got["slug"])
	assert.Equal(t, true, got["include_relations"])

	// Raw stdio: true stays true, middleware is a no-op.
	in2 := json.RawMessage(`{"slug":"auth/jwt","include_relations":true}`)
	out2, changed2 := coerceArguments(in2, schema)
	assert.False(t, changed2, "raw stdio bool should be a no-op")
	assert.JSONEq(t, `{"slug":"auth/jwt","include_relations":true}`, string(out2))
}

// TestCoerceArguments_B2UpdateReviewers is the canonical B2 repro:
// `kron_update` with `reviewers: {"item": "@dave"}` is rewritten
// to `["@dave"]`.
func TestCoerceArguments_B2UpdateReviewers(t *testing.T) {
	schema := toolPropertySchemas["kron_update"]
	require.NotEmpty(t, schema)

	in := json.RawMessage(`{"slug":"auth/jwt","reviewers":{"item":"@dave"}}`)
	out, changed := coerceArguments(in, schema)
	require.True(t, changed)

	var got map[string]any
	require.NoError(t, json.Unmarshal(out, &got))
	assert.Equal(t, "auth/jwt", got["slug"])
	assert.Equal(t, []any{"@dave"}, got["reviewers"])

	// Raw stdio: array passes through unchanged.
	in2 := json.RawMessage(`{"slug":"auth/jwt","reviewers":["@alice","@bob"]}`)
	out2, changed2 := coerceArguments(in2, schema)
	assert.False(t, changed2)
	assert.JSONEq(t, `{"slug":"auth/jwt","reviewers":["@alice","@bob"]}`, string(out2))
}

// TestCoerceArguments_B3StaleDaysThreshold is the canonical B3
// repro: `kron_stale` with `days_threshold: "30"` is rewritten to
// `30` (number). Note that the in-memory value is `float64(30)`
// because that's how Go's encoding/json represents a number; the
// SDK unmarshals it into the handler's `int` field cleanly.
func TestCoerceArguments_B3StaleDaysThreshold(t *testing.T) {
	schema := toolPropertySchemas["kron_stale"]
	require.NotEmpty(t, schema)

	in := json.RawMessage(`{"days_threshold":"30"}`)
	out, changed := coerceArguments(in, schema)
	require.True(t, changed)

	var got map[string]any
	require.NoError(t, json.Unmarshal(out, &got))
	assert.Equal(t, float64(30), got["days_threshold"])

	// Raw stdio: int passes through unchanged.
	in2 := json.RawMessage(`{"days_threshold":30}`)
	out2, changed2 := coerceArguments(in2, schema)
	assert.False(t, changed2)
	assert.JSONEq(t, `{"days_threshold":30}`, string(out2))
}

// TestCoerceArguments_NoOpWhenAlreadyCorrect is the broader
// regression test: every field that arrives in the right type
// must NOT be rewritten. This is the "Cursor eventually fixes
// its bug" case — when the bridge sends correct types, the
// middleware must be a transparent pass-through.
func TestCoerceArguments_NoOpWhenAlreadyCorrect(t *testing.T) {
	cases := []struct {
		tool  string
		input string
	}{
		{"kron_get", `{"slug":"a/b","include_relations":true}`},
		{"kron_update", `{"slug":"a/b","reviewers":["@x"],"depends_on":["c/d"],"references":["e/f"]}`},
		{"kron_stale", `{"days_threshold":90}`},
		{"kron_add", `{"slug":"a/b","kind":"leaf","assumptions":[{"id":"x","severity":"hard","rationale":"long enough rationale"}]}`},
		{"kron_lint", `{"reporter":"text"}`},
		{"kron_list", `{"prefix":"auth/"}`},
	}
	for _, c := range cases {
		t.Run(c.tool, func(t *testing.T) {
			schema := toolPropertySchemas[c.tool]
			out, changed := coerceArguments(json.RawMessage(c.input), schema)
			assert.False(t, changed, "input should be a no-op: %s", c.input)
			assert.JSONEq(t, c.input, string(out), "rewritten output must equal input")
		})
	}
}

// TestCoerceArguments_NoOpForUnrecognisedTypes guards the
// conservative policy: a non-canonical string for a bool field
// ("yes" instead of "true") must NOT be flipped. The SDK will
// surface "type string want boolean" and the user gets a
// useful error rather than a silent wrong-type guess.
func TestCoerceArguments_NoOpForUnrecognisedTypes(t *testing.T) {
	schema := toolPropertySchemas["kron_get"]
	in := json.RawMessage(`{"slug":"a/b","include_relations":"yes"}`)
	out, changed := coerceArguments(in, schema)
	assert.False(t, changed, "non-canonical bool string must not be flipped")
	assert.JSONEq(t, `{"slug":"a/b","include_relations":"yes"}`, string(out))
}

// TestCoerceArguments_EmptyInput covers the "client sent no
// arguments" path: the middleware must be a no-op, not panic.
func TestCoerceArguments_EmptyInput(t *testing.T) {
	schema := toolPropertySchemas["kron_get"]
	out, changed := coerceArguments(nil, schema)
	assert.False(t, changed)
	assert.Nil(t, out)

	out, changed = coerceArguments(json.RawMessage(`{}`), schema)
	assert.False(t, changed)
	assert.JSONEq(t, `{}`, string(out))
}

// TestCoerceArguments_NotAnObject covers the "client sent a
// non-object" path (which is malformed for tools/call): we
// must forward the original bytes so the SDK can surface its
// usual "expected object" error.
func TestCoerceArguments_NotAnObject(t *testing.T) {
	schema := toolPropertySchemas["kron_get"]
	in := json.RawMessage(`"just a string"`)
	out, changed := coerceArguments(in, schema)
	assert.False(t, changed)
	assert.Equal(t, string(in), string(out))
}

// =====================================================================
// Middleware wiring (RFC §2.4)
// =====================================================================

// newTestRequest is a tiny test helper that builds the shape the
// middleware expects to receive from the SDK: a ServerRequest
// wrapping a *mcp.CallToolParamsRaw. The test inspects whether
// the middleware rewrote the Arguments field on that struct.
func newTestRequest(name string, args string) *mcp.ServerRequest[*mcp.CallToolParamsRaw] {
	return &mcp.ServerRequest[*mcp.CallToolParamsRaw]{
		Params: &mcp.CallToolParamsRaw{
			Name:      name,
			Arguments: json.RawMessage(args),
		},
	}
}

// TestCoerceMiddleware_RewritesKnownBridgeShapes is the end-to-end
// middleware test: a request for kron_update with the B2 shape
// `{"slug":"...","reviewers":{"item":"@dave"}}` is rewritten to
// the schema-valid form before the next handler sees it.
func TestCoerceMiddleware_RewritesKnownBridgeShapes(t *testing.T) {
	mw := coerceMiddleware()
	called := false
	next := func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		called = true
		// Inspect the rewritten args. The middleware is supposed
		// to have flipped reviewers in place.
		raw := req.GetParams().(*mcp.CallToolParamsRaw)
		var got map[string]any
		require.NoError(t, json.Unmarshal(raw.Arguments, &got))
		assert.Equal(t, "auth/jwt", got["slug"])
		assert.Equal(t, []any{"@dave"}, got["reviewers"])
		return nil, nil
	}
	wrapped := mw(next)

	req := newTestRequest("kron_update", `{"slug":"auth/jwt","reviewers":{"item":"@dave"}}`)
	_, err := wrapped(context.Background(), "tools/call", req)
	require.NoError(t, err)
	assert.True(t, called, "next handler must have been invoked")
}

// TestCoerceMiddleware_PassesThroughCorrectlyTypedArgs covers the
// "raw stdio client" case: when the args are already the schema
// type, the middleware must not rewrite them. This is the
// "Cursor eventually fixes its bug" regression guard.
func TestCoerceMiddleware_PassesThroughCorrectlyTypedArgs(t *testing.T) {
	mw := coerceMiddleware()
	called := false
	next := func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		called = true
		raw := req.GetParams().(*mcp.CallToolParamsRaw)
		// Args must be byte-for-byte equal to what the client sent.
		assert.Equal(t,
			`{"slug":"auth/jwt","include_relations":true}`,
			strings.TrimSpace(string(raw.Arguments)),
		)
		return nil, nil
	}
	wrapped := mw(next)
	req := newTestRequest("kron_get", `{"slug":"auth/jwt","include_relations":true}`)
	_, err := wrapped(context.Background(), "tools/call", req)
	require.NoError(t, err)
	assert.True(t, called)
}

// TestCoerceMiddleware_IgnoresNonToolsCallMethods makes sure the
// middleware is a transparent pass-through for methods where it
// cannot help (initialize, tools/list, prompts/*, resources/*).
// The SDK's typed Params for those methods don't expose the raw
// JSON, so attempting to coerce would just break things.
func TestCoerceMiddleware_IgnoresNonToolsCallMethods(t *testing.T) {
	mw := coerceMiddleware()
	called := false
	next := func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		called = true
		assert.Equal(t, "tools/list", method, "next handler must see the original method")
		return nil, nil
	}
	wrapped := mw(next)

	// tools/list has Params of a different type; the middleware
	// must not try to coerce it (it would crash on the type
	// assertion). Using a bare Request is enough to verify the
	// method-name short-circuit.
	type fakeRequest struct{ mcp.Request }
	_, err := wrapped(context.Background(), "tools/list", nil)
	require.NoError(t, err)
	assert.True(t, called, "next handler must be invoked for non-tools/call methods")
}

// TestCoerceMiddleware_NoOpForUnknownTool is a safety net: if a
// client invokes a tool name that isn't in toolPropertySchemas
// (theoretical — there are no unknown tools in production), the
// middleware must pass the args through unchanged rather than
// guess. The SDK will reject the unknown tool name with the
// usual "tool not found" error.
func TestCoerceMiddleware_NoOpForUnknownTool(t *testing.T) {
	mw := coerceMiddleware()
	called := false
	next := func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		called = true
		raw := req.GetParams().(*mcp.CallToolParamsRaw)
		assert.Equal(t,
			`{"days_threshold":"30"}`,
			strings.TrimSpace(string(raw.Arguments)),
		)
		return nil, nil
	}
	wrapped := mw(next)
	req := newTestRequest("nonexistent_tool", `{"days_threshold":"30"}`)
	_, err := wrapped(context.Background(), "tools/call", req)
	require.NoError(t, err)
	assert.True(t, called)
}

// =====================================================================
// B4 (RFC §2.5) — prompts/get cannot be coerced at the middleware
// layer because the SDK exposes GetPromptParams (Arguments typed
// map[string]string) with no Raw equivalent. This test pins that
// behaviour: when the receiving middleware is invoked for method
// "prompts/get", it must short-circuit to next() without touching
// the request. The control is end-to-end: the middleware does
// nothing for non-tools/call methods, so the SDK's strict
// map[string]string deserialisation stays in charge.
// =====================================================================

func TestCoerceMiddleware_PromptsGetIsTransparent(t *testing.T) {
	mw := coerceMiddleware()
	called := false
	gotArgs := map[string]string(nil)
	next := func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		called = true
		// Real SDK would have already populated this with the
		// strongly-typed Arguments. We mimic that here.
		if p, ok := req.GetParams().(*mcp.GetPromptParams); ok {
			gotArgs = p.Arguments
		}
		return nil, nil
	}
	wrapped := mw(next)

	// Simulate a *mcp.ServerRequest[*mcp.GetPromptParams] the way
	// the SDK would build it after deserialising a prompts/get
	// request whose arguments were already valid strings.
	req := &mcp.ServerRequest[*mcp.GetPromptParams]{
		Params: &mcp.GetPromptParams{
			Name:      "kron-drift-check",
			Arguments: map[string]string{"days_threshold": "30"},
		},
	}
	_, err := wrapped(context.Background(), "prompts/get", req)
	require.NoError(t, err)
	assert.True(t, called, "next handler must have been invoked")
	assert.Equal(t, "30", gotArgs["days_threshold"], "middleware must not mutate the typed params")
}
