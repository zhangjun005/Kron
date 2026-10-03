package servemcp

import (
	"context"
	"fmt"

	"github.com/xxx/kron/internal/model"
)

// assertCallerMCP is a defensive guard every real (non-stub) tool
// handler should call at entry. The dispatcher's
// callerInjectMiddleware injects the caller identity into ctx, but
// a future refactor that misroutes a request to the wrong access
// layer would fail loud here rather than silently serving a request
// without caller attribution.
//
// The check is an assertion, not authorization: any caller is
// allowed; we only fail if the dispatcher forgot to inject.
//
// Since v1.2 we keep this guard even though the SDK's middleware does
// the injection: it is the cheapest possible insurance against
// regression of the v1.1 dispatcher-bug check (see
// docs/process/mcp-protocol.md §C for the original rationale).
func assertCallerMCP(ctx context.Context) error {
	c := model.CallerFrom(ctx)
	if !model.IsMCPCaller(c) {
		return fmt.Errorf("internal: tool invoked with non-MCP caller %q (dispatcher bug)", c)
	}
	return nil
}
