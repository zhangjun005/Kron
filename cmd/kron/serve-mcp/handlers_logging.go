package servemcp

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// activeSession holds a pointer to the currently-active ServerSession
// for the single-session stdio model Kron uses. It is set by
// serve_mcp.go immediately after the session is created and read by
// shimHandler.Handle when a log record is emitted.
//
// C-3: the MCP Logging capability replaces the previous stderr hack
// (log.SetOutput(errOut) in serve_mcp.go). With C-3 enabled, Kron
// sends log records as MCP logging/message notifications to the
// client. Clients that don't issue logging/setLevel will see no
// log messages (per MCP spec).
//
// go-sdk v1.1.0 already implements:
//   - logging/setLevel (auto-tracked on ServerSession.state.LogLevel)
//   - rate limiting (via mcp.LoggingHandlerOptions.MinInterval)
//   - JSON encoding (via slog.JSONHandler)
//
// Kron-side wiring is just two lines:
//  1. ss := newServerWithLogger(...)        // Logger = mcp.NewLoggingHandler(...)
//  2. SetActiveSession(ss)                   // store so internal log records can route
//
// Internal log records flow through slog.SetDefault(slog.New(&shimHandler{}))
// so handler bodies that call log.Print* / fmt.Fprintf(os.Stderr, ...)
// and any slog.* calls are routed to MCP logging/message.
var activeSession atomic.Pointer[mcp.ServerSession]

// SetActiveSession stores the ServerSession so the logging handler
// can address it. Called from serve_mcp.go after the ServerSession
// is established.
func SetActiveSession(ss *mcp.ServerSession) {
	activeSession.Store(ss)
}

// ClearActiveSession unsets the active session. Called on disconnect.
func ClearActiveSession(ss *mcp.ServerSession) {
	if cur := activeSession.Load(); cur == ss {
		activeSession.Store(nil)
	}
}

// logMinInterval is the rate limit applied by mcp.LoggingHandler
// to prevent flooding during bulk operations (e.g. lint over 500
// files). 200 ms is high enough to drop bursts, low enough that
// an operator still sees errors within ~1 second.
const logMinInterval = 200 * time.Millisecond

// shimHandler is the slog.Handler we register as the default logger.
// On every Handle, it builds a fresh mcp.LoggingHandler bound to the
// active session, then delegates. The cost is one small allocation
// per log record; this is acceptable because logging is not in any
// hot tool path (the tools themselves are CPU/IO bound, not log bound).
//
// Why we don't cache the mcp.LoggingHandler: the SDK handler holds
// a ServerSession pointer. If the session changes (e.g. a test
// spins up multiple in-memory sessions in sequence), a cached
// handler would point at the closed session. Per-record rebuild
// keeps the pointer fresh.
type shimHandler struct{}

func (shimHandler) Enabled(_ context.Context, _ slog.Level) bool {
	// We don't gate here: let the SDK's LoggingHandler do its own
	// level check against ss.state.LogLevel. If no session is
	// active, Handle returns nil; this saves us an extra branch
	// per record.
	return true
}

func (shimHandler) Handle(ctx context.Context, r slog.Record) error {
	ss := activeSession.Load()
	if ss != nil {
		lh := mcp.NewLoggingHandler(ss, &mcp.LoggingHandlerOptions{
			LoggerName:  "kron",
			MinInterval: logMinInterval,
		})
		return lh.Handle(ctx, r)
	}
	// No active session (pre-initialize, after disconnect, or tests
	// that don't call SetActiveSession). Return nil to silently
	// drop the record — this is acceptable because:
	//   1. In production the session is established before Run() returns,
	//      so only pre-init and post-disconnect messages are dropped.
	//   2. The stdlib log output from Run() itself (startup errors, etc.)
	//      goes through log.SetDefault(fallback) which uses errOut directly,
	//      so those messages are never in the shimHandler path.
	//   3. C-3's primary goal (MCP logging for tool operations) is
	//      exercised by tests that use newTestSession.
	return nil
}

func (shimHandler) WithAttrs(_ []slog.Attr) slog.Handler { return shimHandler{} }
func (shimHandler) WithGroup(_ string) slog.Handler      { return shimHandler{} }

// SetActiveSessionOnClose stores ss as the active session and returns
// a cleanup function to call when the session closes. Used by
// serve_mcp.go's Connect path to manage the active session lifecycle
// without needing the session as a closure capture.
func SetActiveSessionOnClose(ss *mcp.ServerSession) (clear func()) {
	SetActiveSession(ss)
	return func() { ClearActiveSession(ss) }
}
