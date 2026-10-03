package servemcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/xxx/kron/internal/model"
)

// toolHandler is the function signature every Kron MCP tool must
// implement. It receives a ctx (caller-injected, currently "mcp" or
// "mcp:<client>") and the raw JSON params from the JSON-RPC envelope.
// It returns the result that goes back to the client as the "result"
// field, or a non-nil error that gets mapped to a JSON-RPC error
// response via classifyError.
//
// Tools MUST NOT return model.Intent (or other internal types) directly
// as the result: the on-the-wire shape is defined in
// docs/implementation/mcp.md and may diverge from the in-memory model
// (e.g., for backwards-compat). Each tool defines its own wire struct
// and converts.
type toolHandler func(ctx context.Context, params json.RawMessage) (any, error)

// defaultToolRegistry returns the v1.1 tool set: 3 implemented tools
// (kron_lint, kron_list, kron_get) plus 9 stub entries that return a
// clear "not implemented in v1.1" error so MCP clients receive a
// well-formed response rather than a silent method-not-found.
//
// See docs/implementation/mcp.md §1 for the full v1 tool list and
// docs/process/mcp-tool.md §2 for the workflow that adds a new tool.
func defaultToolRegistry(root string) map[string]toolHandler {
	stub := func(name string) toolHandler {
		return func(_ context.Context, _ json.RawMessage) (any, error) {
			return nil, fmt.Errorf("%w: %s (v1.1 P0 subset is kron_lint, kron_list, kron_get; see docs/implementation/mcp.md)", errToolNotImplemented, name)
		}
	}

	r := make(map[string]toolHandler, 12)

	// P0 — read-only / diagnostic. Implemented in v1.1.
	r["kron_lint"] = handleLint(root)
	r["kron_list"] = handleList(root)
	r["kron_get"] = handleGet(root)

	// Phase 2 next step — full CRUD + extension tools. Each entry is a
	// stub that fails fast with a clear message; the v1.1 tool list
	// is fixed at 3 to keep the surface small enough to verify end-to-end
	// before broadening it.
	r["kron_init"] = stub("kron_init")
	r["kron_add"] = stub("kron_add")
	r["kron_update"] = stub("kron_update")
	r["kron_delete"] = stub("kron_delete")
	r["kron_restore"] = stub("kron_restore")
	r["kron_assume_check"] = stub("kron_assume_check")
	r["kron_impact"] = stub("kron_impact")
	r["kron_intent_density"] = stub("kron_intent_density")
	r["kron_stale"] = stub("kron_stale")

	return r
}

// errToolNotImplemented is the canonical sentinel used by stub
// handlers. The wire-level error message is the human-readable form;
// MCP clients that care about distinguishing "real error" from
// "not yet wired" can string-match on it. This is intentional —
// stubs are not stable API; they get replaced by real handlers.
var errToolNotImplemented = fmt.Errorf("tool not implemented in v1.1")

// assertCallerMCP is a defensive guard every real (non-stub) tool
// handler should call at entry. The dispatcher injects the caller
// identity via context, but a future refactor that misroutes a
// request to the wrong access layer would fail loud here rather
// than silently serving a request without caller attribution.
//
// The check is an assertion, not authorization: any caller is
// allowed; we only fail if the dispatcher forgot to inject.
func assertCallerMCP(ctx context.Context) error {
	c := model.CallerFrom(ctx)
	if !model.IsMCPCaller(c) {
		return fmt.Errorf("internal: tool invoked with non-MCP caller %q (dispatcher bug)", c)
	}
	return nil
}
