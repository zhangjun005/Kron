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

// defaultToolRegistry returns the v1.1+ tool set: 12 tools (3 P0
// read-only + 9 full CRUD + extension tools).
//
// See docs/implementation/mcp.md §1 for the full v1 tool list and
// docs/process/mcp-tool.md §2 for the workflow that adds a new tool.
func defaultToolRegistry(root string) map[string]toolHandler {
	r := make(map[string]toolHandler, 12)

	// P0 — read-only / diagnostic. Implemented in v1.1.
	r["kron_lint"] = handleLint(root)
	r["kron_list"] = handleList(root)
	r["kron_get"] = handleGet(root)

	// Phase 2 (v1.1 P1) — full CRUD + extension tools. Each handler
	// delegates to internal/store and internal/parser; none of them
	// know about the JSON-RPC envelope (that mapping lives in rpc.go).
	r["kron_init"] = handleInit(root)
	r["kron_add"] = handleAdd(root)
	r["kron_update"] = handleUpdate(root)
	r["kron_delete"] = handleDelete(root)
	r["kron_restore"] = handleRestore(root)
	r["kron_assume_check"] = handleAssumeCheck(root)
	r["kron_impact"] = handleImpact(root)
	r["kron_intent_density"] = handleIntentDensity(root)
	r["kron_stale"] = handleStale(root)

	return r
}

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
