package model

import (
	"context"
	"errors"
	"fmt"
)

// Known caller identities stored under CallerKey.
//
// Access layers (cmd/kron/cli, cmd/kron/serve-mcp, future LSP / IDE / GUI)
// inject exactly one of these values (or a derivative, e.g. "mcp:claude-3.7")
// via WithCaller before delegating to internal packages.
//
// internal/* packages read the value back via CallerFrom. The set of callers
// is finite and protocol-agnostic, so it is enumerated here in the model layer
// rather than duplicated in every package that needs to interpret it.
const (
	CallerCLI = "cli"
	CallerMCP = "mcp"
	CallerLSP = "lsp"
	CallerIDE = "ide"
	CallerGUI = "gui"
)

// callerKeyType prevents collisions with context keys defined in other packages.
// The zero value is never used; only the exported CallerKey below carries it.
type callerKeyType struct{ name string }

// CallerKey is the context.Context value key under which the caller identity
// is stored. Use WithCaller to inject and CallerFrom to read.
//
// The value stored under this key is always a string from set
// {CallerCLI, CallerMCP, CallerLSP, CallerIDE, CallerGUI} or a derivative
// like "mcp:claude-3.7". internal/ packages MUST NOT inspect the value
// shape beyond string-typed equality.
var CallerKey = callerKeyType{"caller"}

// WithCaller returns a derived context that carries the given caller identity.
//
// Call this at the entry point of every access layer, BEFORE delegating to
// internal packages, so internal / lint / store / parser can report results
// back attributed to the caller.
//
//	// In cmd/kron/cli:
//	ctx = model.WithCaller(cmd.Context(), model.CallerCLI)
//
//	// In cmd/kron/serve-mcp:
//	ctx = model.WithCaller(ctx, "mcp:"+req.ClientInfo.Name)
//
// Empty caller is allowed (used by direct test invocations) and CallerFrom will
// return "" for such contexts.
func WithCaller(ctx context.Context, caller string) context.Context {
	return context.WithValue(ctx, CallerKey, caller)
}

// CallerFrom returns the caller identity stored on ctx, or "" if none was
// injected. Callers should treat "" as "direct test / unknown origin" rather
// than as an error.
//
// The caller identity is intended for:
//   - tagging lint output / log entries with the access layer that triggered them
//   - future audit logs (v1 does not write audit logs, but the interface is reserved)
//
// The caller identity is NOT intended for authorization / permission gates.
// See docs/abstractDesign/architecture.md §2.3.
func CallerFrom(ctx context.Context) string {
	v, _ := ctx.Value(CallerKey).(string)
	return v
}

// CallerKnown reports whether s is one of the five base caller identities
// (CLI / MCP / LSP / IDE / GUI). It returns false for the empty string and
// for derivative forms like "mcp:claude-3.7"; callers that need to match
// derivatives should consult IsMCPCaller (or pattern match on the prefix
// themselves).
//
// This is a pure string-set check. It is intended for diagnostic purposes
// ("did the caller identity get injected at all?") and for early-fail
// assertions in access-layer entry points, not for authorization.
func CallerKnown(s string) bool {
	switch s {
	case CallerCLI, CallerMCP, CallerLSP, CallerIDE, CallerGUI:
		return true
	}
	return false
}

// IsMCPCaller reports whether s represents an MCP-origin caller. This
// includes both the base "mcp" identity and any derivative form starting
// with "mcp:" (e.g. "mcp:claude-3.7", "mcp:cursor", even "mcp:" with an
// empty agent suffix — the MCP server may not always know the agent
// name). The check is string-prefix based; it does not parse or validate
// the suffix.
//
// Use this in the MCP access layer when matching the caller identity
// against the protocol; use CallerKnown when matching against the
// abstract layer set.
func IsMCPCaller(s string) bool {
	if s == CallerMCP {
		return true
	}
	const prefix = CallerMCP + ":"
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// RequireKnownCaller returns an error if ctx's caller identity is empty
// or is not one of the five known base callers. Access layers should
// call this at their entry point, immediately after WithCaller, so a
// missing or malformed injection fails fast with a clear message
// rather than propagating an empty caller to internal packages.
//
// This is an assertion (catches "I forgot to inject"), not an
// authorization check. The five base callers are all equivalent in
// terms of privilege under v1's caller model.
func RequireKnownCaller(ctx context.Context) error {
	c := CallerFrom(ctx)
	if c == "" {
		return errors.New("caller identity not injected on context; access layer must call model.WithCaller before delegating to internal packages")
	}
	if !CallerKnown(c) {
		return fmt.Errorf("caller identity %q is not one of %s/%s/%s/%s/%s (or an mcp: derivative)", c, CallerCLI, CallerMCP, CallerLSP, CallerIDE, CallerGUI)
	}
	return nil
}
