package servemcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/xxx/kron/internal/lint"
)

// handleLint returns the toolHandler for kron_lint. The handler is
// parameterised by root (the repository root) so the tool reads from
// a fixed location per server process; it does NOT accept a root
// parameter on the wire in v1.1 (the JSON-RPC -CWD flag is the only
// way to set it).
//
// Wire contract: see docs/implementation/mcp.md §2 (kron_lint).
//
//	in:  {}
//	out: { passed: bool, errors: [{rule, where, severity, detail}], summary: {errors, warnings} }
//
// The on-the-wire "errors" key is the array of diagnostics; it is
// named "errors" for consistency with the mcp.md contract even though
// some entries are severity "warning". The "passed" boolean is true
// only when no error-severity diagnostic is present.
func handleLint(root string) toolHandler {
	return func(ctx context.Context, _ json.RawMessage) (any, error) {
		if err := assertCallerMCP(ctx); err != nil {
			return nil, err
		}
		diags, err := lint.Run(ctx, root)
		if err != nil {
			return nil, fmt.Errorf("lint: %w", err)
		}
		return lintResponseFromDiags(diags), nil
	}
}

// lintResponse is the on-the-wire shape for kron_lint's result.
// JSON field order is the same as the mcp.md spec; field tags pin
// the wire names so a rename here does not silently break MCP clients.
type lintResponse struct {
	Passed  bool           `json:"passed"`
	Errors  []lintDiagWire `json:"errors"`
	Summary lintSummary    `json:"summary"`
}

type lintDiagWire struct {
	Rule     string `json:"rule"`
	Where    string `json:"where"`
	Severity string `json:"severity"`
	Detail   string `json:"detail"`
}

type lintSummary struct {
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
}

// lintResponseFromDiags converts the internal/lint.Diag slice to the
// wire shape. Empty severities are normalised to "error" (matches the
// internal/lint semantics of "empty is error").
func lintResponseFromDiags(diags []lint.Diag) lintResponse {
	out := lintResponse{
		Errors: make([]lintDiagWire, 0, len(diags)),
	}
	for _, d := range diags {
		sev := string(d.Severity)
		if sev == "" {
			sev = "error"
		}
		out.Errors = append(out.Errors, lintDiagWire{
			Rule:     string(d.Rule),
			Where:    d.Where,
			Severity: sev,
			Detail:   d.Detail,
		})
		switch sev {
		case "error":
			out.Summary.Errors++
		default:
			out.Summary.Warnings++
		}
	}
	out.Passed = out.Summary.Errors == 0
	return out
}
