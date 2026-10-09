package servemcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/xxx/kron/internal/lint"
)

// HandleLint returns the mcp.ToolHandlerFor binding for kron_lint.
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
//
// The handler is parameterised by the repo root, set on the ctx by
// callerInjectMiddleware at the start of the request. This matches
// the v1.1 behaviour where root was baked into each handleXxx(root)
// closure; the v1.2 refactor centralises that wiring in one place
// (the middleware) so all 12 handlers can share it.
func HandleLint(ctx context.Context, _ *mcp.CallToolRequest, in LintInput) (
	*mcp.CallToolResult, LintOutput, error,
) {
	if err := assertCallerMCP(ctx); err != nil {
		return nil, LintOutput{}, err
	}
	root := repoRootFrom(ctx)
	diags, err := lint.Run(ctx, root)
	if err != nil {
		return nil, LintOutput{}, fmt.Errorf("lint: %w", err)
	}
	out := lintOutputFromDiags(diags)
	// reporter="text" adds a human-readable summary (CLI parity
	// with `kron lint` text output). Default is json; the
	// structured fields above are the canonical payload.
	if in.Reporter == "text" {
		out.Text = lintTextSummary(diags)
	} else if in.Reporter != "" && in.Reporter != "json" {
		return nil, LintOutput{}, fmt.Errorf("kron_lint: invalid reporter %q (must be json or text)", in.Reporter)
	}
	return nil, out, nil
}

// lintOutputFromDiags converts internal/lint.Diag to the wire LintOutput
// shape. Empty severities are normalised to "error" (matches the
// internal/lint semantics of "empty is error").
func lintOutputFromDiags(diags []lint.Diag) LintOutput {
	out := LintOutput{
		Errors: make([]LintDiag, 0, len(diags)),
	}
	for _, d := range diags {
		sev := string(d.Severity)
		if sev == "" {
			sev = "error"
		}
		out.Errors = append(out.Errors, LintDiag{
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

// lintTextSummary renders diags as a human-readable multi-line
// string mirroring the CLI `kron lint` text output. Used by
// kron_lint when reporter=text. Mirrors the formatting in
// cmd/kron/cli/lint.go writeLintText; kept here rather than
// imported because the CLI's writer is io.Writer-coupled and the
// MCP wire shape is a string field.
func lintTextSummary(diags []lint.Diag) string {
	if len(diags) == 0 {
		return "[ok] 0 errors, 0 warnings"
	}
	errors, warnings := 0, 0
	for _, d := range diags {
		if d.Severity == lint.SeverityError || d.Severity == "" {
			errors++
		} else {
			warnings++
		}
	}
	out := fmt.Sprintf("[summary] %d errors, %d warnings", errors, warnings)
	if errors == 0 && warnings == 0 {
		return "[ok] " + out
	}
	return out
}
