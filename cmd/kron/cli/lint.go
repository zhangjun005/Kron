package cli

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/xxx/kron/internal/lint"
)

// runLint implements `kron lint`.
//
// It is a thin wrapper around internal/lint.Run: cobra flag parsing,
// repo-root resolution, and output rendering are the only concerns left
// here. Rule definitions, anchor scanning, and frontmatter validation
// all live in internal/lint/ and are shared with the MCP server's
// `kron_lint` tool.
//
// Exit codes match docs/implementation/cli.md §4:
//
//	0 = no errors
//	1 = lint errors
//	2 = internal failure (I/O / parse exception)
func runLint(args []string, out, errOut io.Writer) error {
	fs := flag.NewFlagSet("lint", flag.ContinueOnError)
	fs.SetOutput(errOut)
	fs.Usage = func() {
		fmt.Fprintln(errOut, "Usage: kron lint [-CWD <root>] [-reporter text|json]")
		fs.PrintDefaults()
	}
	cwd := fs.String("CWD", "", "Repository root (default: current working directory)")
	reporter := fs.String("reporter", "text", "Output format: text (default) or json")
	if err := fs.Parse(args); err != nil {
		return errUsage
	}

	root, err := resolveRoot(*cwd)
	if err != nil {
		fmt.Fprintln(errOut, "kron lint:", err)
		return newExitError(exitInternal, err.Error())
	}

	diags, err := lint.Run(context.Background(), root)
	if err != nil {
		fmt.Fprintln(errOut, "kron lint:", err)
		return newExitError(exitInternal, err.Error())
	}

	if *reporter == "json" {
		writeLintJSON(out, diags)
	} else {
		writeLintText(out, diags)
	}

	if lint.HasErrors(diags) {
		return newExitError(exitLint, "")
	}
	return nil
}

func writeLintText(out io.Writer, diags []lint.Diag) {
	for _, d := range diags {
		fmt.Fprintf(out, "[%s] %s: %s -> %s\n", d.Severity, d.Rule, d.Where, d.Detail)
	}
	if len(diags) == 0 {
		fmt.Fprintf(out, "[ok] 0 errors, 0 warnings\n")
		return
	}
	errors, warnings := 0, 0
	for _, d := range diags {
		if d.Severity == lint.SeverityError {
			errors++
		} else {
			warnings++
		}
	}
	fmt.Fprintf(out, "[summary] %d errors, %d warnings\n", errors, warnings)
}

func writeLintJSON(out io.Writer, diags []lint.Diag) {
	// Hand-rolled JSON: each diag is one object on its own line, plus a
	// summary object. Keeping it tiny avoids pulling in encoding/json
	// for what amounts to ~6 fields.
	fmt.Fprintln(out, "{")
	fmt.Fprintf(out, `  "diagnostics": [`)
	for i, d := range diags {
		if i > 0 {
			fmt.Fprint(out, ",")
		}
		fmt.Fprintln(out)
		fmt.Fprintf(out, `    {"rule": %q, "where": %q, "severity": %q, "detail": %q}`,
			string(d.Rule), d.Where, string(d.Severity), d.Detail)
	}
	if len(diags) > 0 {
		fmt.Fprintln(out)
	}
	fmt.Fprintln(out, `  ],`)
	errors, warnings := 0, 0
	for _, d := range diags {
		if d.Severity == lint.SeverityError {
			errors++
		} else {
			warnings++
		}
	}
	fmt.Fprintf(out, `  "summary": {"errors": %d, "warnings": %d}`+"\n", errors, warnings)
	fmt.Fprintln(out, "}")
}

// Exit codes surfaced to the cmd/kron main() layer.
const (
	exitLint     = 1
	exitInternal = 2
)
