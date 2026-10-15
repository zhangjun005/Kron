package servemcp

// Coerce middleware for MCP tools/call arguments.
//
// Why: Cursor's MCP bridge (and several other AI clients) sometimes
// serialises tool-call arguments with the wrong JSON type. Most
// commonly, a value the server expects as `boolean` / `integer` /
// `array` arrives as a JSON string or object. Examples that have been
// observed end-to-end in r3 (2026-10-15):
//
//   - `include_relations: true`     → "true"     (B1)
//   - `reviewers: ["@alice"]`       → {"item": "@alice"}  (B2)
//   - `days_threshold: 0`           → "0"        (B3)
//
// Strict JSON-Schema validation rejects these with -32602 "type
// mismatch" errors. The schema itself is correct — the spec says
// `boolean`, the server's Go type is `bool`, the only fix is on the
// client side. But the Cursor bug is a long-standing, multi-thread,
// still-open issue (forum threads 145807, 151180, 169687, 164361;
// v3.12.x regression). Waiting for a Cursor fix is not a plan.
//
// Approach (RFC docs/rfc/2026-10-15-mcp-coerce-middleware.md §2):
//
//  1. The schema is NOT changed. Strict types stay strict in the
//     tools/list response so raw stdio / Go SDK / Claude Code
//     clients (which always send the right type) see no surprise.
//  2. A receiving middleware sits in front of every tools/call and
//     inspects the raw JSON-RPC arguments before the SDK unmarshals
//     them into the handler's typed Input struct.
//  3. Coercion is type-directed and conservative: we only flip
//     values that match a small whitelist of "the client obviously
//     meant the right type" patterns (string "true"/"false"/"1"/"0"
//     → bool, string "30" → int, {"item": x} → [x], JSON-array
//     string → array, float 30.0 → int 30). Anything else is left
//     alone, so the SDK will still surface genuine type errors
//     with the usual "type X has type Y" message.
//  4. When the value is already the right type, the middleware is
//     a no-op. So when Cursor eventually fixes its bug, this code
//     becomes a transparent pass-through.
//
// Out of scope (RFC §1.4, §3):
//   - Schema changes (no `anyOf`, no `z.coerce.*` style) — would
//     break strict spec and force raw stdio clients onto the
//     lenient path.
//   - Coercion of the `prompts/get` Arguments field — that field
//     is typed `map[string]string` in the SDK, so the middleware
//     cannot see the raw JSON. B4 is documented in
//     docs/snapshot/2026-10-15-McpKnownIssues-CursorBridge.md
//     and tracked separately until the SDK exposes a
//     `GetPromptParamsRaw` parallel to `CallToolParamsRaw`.

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Prompts/get handling (RFC 2026-10-15-mcp-coerce-middleware.md §2.5 B4):
// the SDK types Arguments as map[string]string (no Raw equivalent), so
// by the time our receiving middleware sees the request, the bridge
// shape has already been un-marshalled or rejected. We do nothing
// here.
//
// We investigated the (b) option ("intercept before SDK unmarshals")
// and confirmed it is unreachable: mcp.Request.GetParams() returns
// the typed *GetPromptParams (not a Raw wrapper), and there is no
// transport-level raw access in the public API. SDK 1.2.x has no
// `GetPromptParamsRaw` parallel to `CallToolParamsRaw`. So a
// receiving middleware cannot see the bridge shape for prompts/get,
// and an indirect workaround (custom transport, JSON-RPC shim) is
// out of scope per RFC §1.4. Status: documented in
// docs/snapshot/2026-10-15-McpKnownIssues-CursorBridge.md.

// coerceMiddleware returns an AddReceivingMiddleware-compatible
// function that pre-processes tools/call arguments. It is the
// implementation of RFC 2026-10-15-mcp-coerce-middleware.md §2.4.
//
// The middleware is read-only on the `Request` value passed to it:
// `*mcp.CallToolParamsRaw` exposes its `Arguments` field as a
// settable json.RawMessage, and we rewrite that field in place
// before forwarding to the next handler in the chain. The rest
// of the request (Name, Meta, session) is untouched.
//
// Returns a no-op middleware for non-tools/call methods, so it
// can safely be applied to the whole server.
func coerceMiddleware() mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			// Only tools/call has raw JSON arguments we can rewrite.
			// initialize / tools/list / prompts/* / resources/* all
			// have typed Params we shouldn't touch.
			if method != "tools/call" {
				return next(ctx, method, req)
			}

			// Type-assert to the raw params struct. The SDK uses
			// generics on the request type, but the receiving
			// middleware API exposes the raw interface, so the
			// assertion is the supported path.
			rawParams, ok := req.GetParams().(*mcp.CallToolParamsRaw)
			if !ok {
				// Should never happen for tools/call, but if
				// a future SDK version changes the type, we
				// forward unchanged rather than panic.
				return next(ctx, method, req)
			}
			if len(rawParams.Arguments) == 0 {
				// No arguments to coerce.
				return next(ctx, method, req)
			}

			// Look up the per-tool property schema. The schema
			// map is the source of truth for "what type does
			// this field actually want?" and is the only thing
			// that tells coercion whether "true" should become
			// bool (B1) or stay a string.
			propSchema, ok := toolPropertySchemas[rawParams.Name]
			if !ok {
				// Unknown tool name — let the SDK reject it.
				return next(ctx, method, req)
			}

			coerced, changed := coerceArguments(rawParams.Arguments, propSchema)
			if changed {
				rawParams.Arguments = coerced
			}
			return next(ctx, method, req)
		}
	}
}

// toolPropertySchemas maps each tool name to the per-argument schema
// (name → expected JSON type). It is hand-maintained alongside
// tools_schema.go and must be updated whenever a tool's Input type
// adds, removes, or retypes a field.
//
// Only the EXPECTED TYPE is recorded here. The actual shape
// (required vs optional, item type, etc.) is enforced by the SDK
// after the middleware runs. The middleware's job is narrow: fix
// the small set of "obviously wrong but obviously meant right"
// cases that AI-client bridges produce.
//
// Map value is a flat map of argument name → expected JSON type
// (one of "boolean", "integer", "number", "string", "array",
// "object"). This is the smallest set of types the coerce logic
// understands; any field NOT in the per-tool map is left alone.
var toolPropertySchemas = map[string]map[string]string{
	"kron_add": {
		"slug":        "string",
		"symbol":      "string",
		"why":         "string",
		"kind":        "string",
		"assumptions": "array",
	},
	"kron_update": {
		"slug":        "string",
		"symbol":      "array",
		"status":      "string",
		"reviewers":   "array",
		"body":        "string",
		"references":  "array",
		"depends_on":  "array",
		"assumptions": "array",
	},
	"kron_get": {
		"slug":              "string",
		"include_relations": "boolean",
	},
	"kron_list": {
		"prefix": "string",
	},
	"kron_delete": {
		"slug": "string",
	},
	"kron_restore": {
		"slug": "string",
	},
	"kron_init": {},
	"kron_lint": {
		"reporter": "string",
	},
	"kron_stale": {
		"days_threshold": "integer",
	},
	"kron_impact": {
		"slug": "string",
	},
	"kron_intent_density": {},
	"kron_tree":           {},
	"kron_assume_create": {
		"id":               "string",
		"text":             "string",
		"default_severity": "string",
		"body":             "string",
	},
	"kron_assume_get": {
		"id": "string",
	},
	"kron_assume_list": {
		"status": "string",
	},
	"kron_assume_update": {
		"id":               "string",
		"text":             "string",
		"default_severity": "string",
		"body":             "string",
		"status":           "string",
		"expires_at":       "string",
		"verified_at":      "string",
		"verified_by":      "string",
		"reviewers":        "array",
	},
	"kron_assume_delete": {
		"id": "string",
	},
	"kron_assume_check": {
		"file_path": "string",
	},
}

// coerceArguments reads args (the raw JSON object the client sent),
// rewrites any values that the schema says should be a different
// JSON type, and returns the rewritten bytes. The second return
// value is true iff at least one value was changed; callers use it
// to decide whether to overwrite the request field.
//
// Implementation note: we do the rewrite on a Go map[string]any
// rather than a streaming JSON token walker. The arguments object
// is small (always < 10 fields) and we need to look up "expected
// type" by name, so a map is the natural shape. A streaming
// implementation would be more allocation-efficient but harder to
// reason about, and arguments are a tiny fraction of overall
// request cost.
//
// We intentionally do NOT validate the rewritten JSON here. That
// is the SDK's job on the next leg of the dispatch chain. If our
// rewrite produces a value the schema rejects, the user gets the
// same -32602 error they would have without the middleware — the
// middleware is a best-effort shim, not a validator.
func coerceArguments(args json.RawMessage, schema map[string]string) (json.RawMessage, bool) {
	if len(args) == 0 || len(schema) == 0 {
		return args, false
	}
	// Round-trip through map[string]any. We accept the cost
	// because (a) args is always a JSON object in tools/call,
	// (b) the volume is small, and (c) it lets us treat every
	// field uniformly without hand-rolling a JSON walker.
	var m map[string]any
	if err := json.Unmarshal(args, &m); err != nil {
		// Not an object — let the SDK surface a clear error.
		return args, false
	}
	changed := false
	for name, expected := range schema {
		v, present := m[name]
		if !present {
			continue
		}
		coerced, did := coerceValue(v, expected)
		if did {
			m[name] = coerced
			changed = true
		}
	}
	if !changed {
		return args, false
	}
	out, err := json.Marshal(m)
	if err != nil {
		// Should not happen — we just unmarshalled into a
		// map[string]any and Go's JSON encoder can always
		// re-emit it. Fall back to the original bytes so
		// the SDK surfaces its usual error.
		return args, false
	}
	return out, true
}

// coerceValue is the per-field policy. The (expected) type comes
// from toolPropertySchemas; the actual Go value is what the JSON
// parser produced for the client's input. Returns (coerced, true)
// when a rewrite is applied, or (v, false) when the value is
// already the right type or we don't recognise the pattern.
//
// Conservative policy: we only flip values that match a small,
// well-known set of "the client obviously meant the schema's
// declared type" patterns. In particular:
//
//   - bool expects bool, got string → only flip "true"/"false"
//     /"1"/"0" (and their lowercase variants). Anything else
//     ("yes", "on", "enabled") is left alone so the SDK can
//     surface a useful error.
//   - integer/number expects number, got string → only flip
//     strings that parse cleanly as a decimal number. Reject
//     "1e2", "0x10", empty strings, etc., so genuine errors
//     still produce genuine error messages.
//   - integer expects int, got float → only flip floats with
//     zero fractional part. 30.0 → 30 yes, 30.5 → no (the
//     client clearly meant a fractional number).
//   - array expects array, got object with single "item" key
//     → wrap as [item] (B2 — Cursor's MCP bridge shape).
//   - array expects array, got string starting with "[" →
//     parse as JSON array (some bridges stringify arrays).
//
// Anything outside these patterns returns (v, false) and the
// SDK gets the original value. If the original is also wrong,
// the SDK rejects with the same -32602 it would have produced
// without the middleware.
func coerceValue(v any, expected string) (any, bool) {
	switch expected {
	case "boolean":
		return coerceToBool(v)
	case "integer":
		return coerceToInt(v)
	case "number":
		return coerceToNumber(v)
	case "array":
		return coerceToArray(v)
	}
	return v, false
}

// coerceToBool maps the bridge-shaped string / number / object
// values back to a JSON bool. Returns (v, false) when the value
// already is a bool (the common case for raw stdio clients) or
// when we don't recognise the pattern.
func coerceToBool(v any) (any, bool) {
	switch x := v.(type) {
	case bool:
		return v, false
	case string:
		switch x {
		case "true", "True", "TRUE", "1":
			return true, true
		case "false", "False", "FALSE", "0":
			return false, true
		}
	case float64:
		// JSON numbers are always float64 in Go. 0/1 are the
		// only integer values we trust as bool — anything
		// else is left to the SDK to reject.
		switch x {
		case 0:
			return false, true
		case 1:
			return true, true
		}
	}
	return v, false
}

// coerceToInt maps bridge-shaped string values to a JSON integer.
// Note: the rewrite stores the int as a float64 (Go's JSON
// number type) so the SDK's json.Unmarshal into a typed `int`
// field succeeds. Storing a Go `int` in the map[string]any
// would re-marshal to a JSON number with a fractional .0 in
// some clients, which is harmless but ugly. Storing the float
// directly is the conventional shape.
func coerceToInt(v any) (any, bool) {
	switch x := v.(type) {
	case float64:
		// Already a number. Only "looks like an integer"
		// (zero fractional part) is safe to narrow. 30.5
		// is left alone so the SDK surfaces "type number
		// want integer" — the client clearly meant a
		// fractional number, not a boolean-looking int.
		if x == float64(int64(x)) {
			return x, false // no rewrite needed; SDK will unmarshal fine
		}
		return v, false
	case string:
		n, err := strconv.ParseInt(x, 10, 64)
		if err != nil {
			return v, false
		}
		return float64(n), true // store as JSON number
	}
	return v, false
}

// coerceToNumber maps bridge-shaped string values to a JSON
// number. More permissive than coerceToInt (accepts fractions),
// but still rejects anything that doesn't parse cleanly so
// genuine errors still surface.
func coerceToNumber(v any) (any, bool) {
	if _, ok := v.(float64); ok {
		return v, false
	}
	if s, ok := v.(string); ok {
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return v, false
		}
		return f, true
	}
	return v, false
}

// coerceToArray handles the two known bridge shapes for arrays:
//
//  1. Object with a single "item" key (B2): {"item": "@alice"}.
//     We deliberately only act on the "item" key shape (the one
//     Cursor's bridge uses); arbitrary object shapes are left to
//     the SDK to reject with a useful error.
//
//  2. String that looks like a JSON array (some bridges stringify
//     arrays before transport). We try to parse the inside and
//     return it as []any. A parse failure leaves the string
//     alone so the user gets the usual "type string want array"
//     error.
//
// Plain JSON arrays (the raw stdio shape) are no-ops.
func coerceToArray(v any) (any, bool) {
	switch x := v.(type) {
	case []any:
		return v, false
	case map[string]any:
		// B2 shape: {"item": x}. We require exactly one key
		// named "item" so we don't accidentally reshape
		// genuine objects the client meant to send.
		if len(x) != 1 {
			return v, false
		}
		item, ok := x["item"]
		if !ok {
			return v, false
		}
		return []any{item}, true
	case string:
		// Some bridges stringify arrays: "[1,2,3]" arrives
		// as a string. Only attempt to parse strings that
		// look like a JSON array (start with "["); everything
		// else is left to fail in the usual way.
		if len(x) < 2 || x[0] != '[' {
			return v, false
		}
		var arr []any
		if err := json.Unmarshal([]byte(x), &arr); err != nil {
			return v, false
		}
		return arr, true
	}
	return v, false
}
