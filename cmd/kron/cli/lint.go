package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/parser"
	"github.com/xxx/kron/internal/store"
)

// lintRule is the id of a single diagnostic produced by runLint.
type lintRule string

const (
	ruleAnchorDangling     lintRule = "anchor-dangling"
	ruleFrontmatterInvalid lintRule = "frontmatter-invalid"
)

type lintDiag struct {
	Rule     lintRule
	Where    string // repo-relative file path, or "<root>" if repo-wide
	Detail   string
	Severity string // "error" | "warning"
}

func (d lintDiag) severity() string {
	if d.Severity == "" {
		return "error"
	}
	return d.Severity
}

// runLint implements `kron lint`.
//
// Two rule classes are active in v1 (see docs/process/lint-rule.md §4):
//
//	A: anchor-dangling — every "// @kron:intent <slug>" in a source file
//	   must resolve to an existing .kron/intents/<slug>.md.
//	B: frontmatter-invalid — every intent's frontmatter must parse and
//	   contain the required fields (created_by, updated_at).
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

	// Two independent scans; both must succeed for an exit-0 result.
	diags, internal := collectLintDiags(root)
	if internal != nil {
		fmt.Fprintln(errOut, "kron lint:", internal)
		return newExitError(exitInternal, internal.Error())
	}

	if *reporter == "json" {
		writeLintJSON(out, diags)
	} else {
		writeLintText(out, diags)
	}

	if hasErrors(diags) {
		return newExitError(exitLint, "")
	}
	return nil
}

func collectLintDiags(root string) ([]lintDiag, error) {
	var diags []lintDiag

	// A-class: scan every source file under root for anchors and resolve
	// each one against the store.
	anchors, err := parser.ScanAnchors(root)
	if err != nil {
		return nil, fmt.Errorf("scan anchors: %w", err)
	}
	r, err := store.NewReader(root)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	for _, a := range anchors {
		if !r.Exists(context.Background(), a.Slug) {
			diags = append(diags, lintDiag{
				Rule:     ruleAnchorDangling,
				Where:    relPath(root, a.FilePath) + ":" + itoa(a.LineNumber),
				Detail:   fmt.Sprintf("anchor points to non-existent intent %q", a.Slug),
				Severity: "error",
			})
		}
	}

	// B-class: walk every intent and ensure its frontmatter parses.
	intents, err := r.LoadAll(context.Background())
	if err != nil {
		return nil, fmt.Errorf("load intents: %w", err)
	}
	for _, in := range intents {
		where := relPath(root, in.SourcePath)
		if err := validateFrontmatterForLint(in); err != nil {
			diags = append(diags, lintDiag{
				Rule:     ruleFrontmatterInvalid,
				Where:    where,
				Detail:   err.Error(),
				Severity: "error",
			})
		}
	}
	return diags, nil
}

// validateFrontmatterForLint re-checks the already-parsed frontmatter
// for v1's required fields. parser.ParseFrontmatter already enforces
// the structure; this is a thin second line of defence in case future
// changes relax parser rules before this code is updated.
func validateFrontmatterForLint(in *model.Intent) error {
	if in.Frontmatter.CreatedBy == "" {
		return fmt.Errorf("frontmatter missing required field: created_by")
	}
	if in.Frontmatter.UpdatedAt.IsZero() {
		return fmt.Errorf("frontmatter missing required field: updated_at")
	}
	return nil
}

func hasErrors(diags []lintDiag) bool {
	for _, d := range diags {
		if d.severity() == "error" {
			return true
		}
	}
	return false
}

func writeLintText(out io.Writer, diags []lintDiag) {
	for _, d := range diags {
		fmt.Fprintf(out, "[%s] %s: %s -> %s\n", d.severity(), d.Rule, d.Where, d.Detail)
	}
	if len(diags) == 0 {
		fmt.Fprintf(out, "[ok] 0 errors, 0 warnings\n")
		return
	}
	errors, warnings := 0, 0
	for _, d := range diags {
		if d.severity() == "error" {
			errors++
		} else {
			warnings++
		}
	}
	fmt.Fprintf(out, "[summary] %d errors, %d warnings\n", errors, warnings)
}

func writeLintJSON(out io.Writer, diags []lintDiag) {
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
			string(d.Rule), d.Where, d.severity(), d.Detail)
	}
	if len(diags) > 0 {
		fmt.Fprintln(out)
	}
	fmt.Fprintln(out, `  ],`)
	errors, warnings := 0, 0
	for _, d := range diags {
		if d.severity() == "error" {
			errors++
		} else {
			warnings++
		}
	}
	fmt.Fprintf(out, `  "summary": {"errors": %d, "warnings": %d}`+"\n", errors, warnings)
	fmt.Fprintln(out, "}")
}

// relPath returns path relative to root if possible, else the original
// path. Used to keep lint output stable across CWD changes.
func relPath(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil {
		return rel
	}
	return path
}

// itoa avoids strconv import in lint.go for a single use.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// Exit codes surfaced to the cmd/kron main() layer.
const (
	exitLint     = 1
	exitInternal = 2
)
