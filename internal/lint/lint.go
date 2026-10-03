// Package lint combines store and parser to validate a repository's
// intent files against v1's two mandatory check classes: every anchor in
// source code must resolve to an existing intent (anchor-dangling), and
// every intent's frontmatter must parse (frontmatter-invalid). It is
// consumed by both CLI (`kron lint`) and MCP server (`kron_lint`).
//
// Run does not write to disk. It performs read-only scans; diagnostics
// are returned as a slice of Diag, never as Go errors (rule failures
// are observations, not exceptions).
package lint

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/xxx/kron/internal/parser"
	"github.com/xxx/kron/internal/store"
)

// Run walks the repository rooted at root and returns every diagnostic
// produced by v1's two mandatory rule classes. ctx is reserved for
// future cancellation; v1 does not read it.
//
// Errors are returned only for scope-cream failures: a missing root,
// an I/O error walking the source tree, or a catastrophic parse failure
// during anchor scanning. Per-file rule failures (including frontmatter
// load failures) are returned as Diag entries — see LoadAllErrorAsDiag.
//
// root may be absolute; ScanAnchors and store.NewReader both call
// filepath.Abs internally.
func Run(ctx context.Context, root string) ([]Diag, error) {
	_ = ctx // reserved for cancellation; no-op in v1

	if root == "" {
		return nil, fmt.Errorf("lint: %w", errEmptyRoot)
	}

	diags := make([]Diag, 0)

	// A-class: scan every source file for anchors; report each one that
	// does not resolve to an existing intent.
	anchors, err := parser.ScanAnchors(root)
	if err != nil {
		return nil, fmt.Errorf("lint: scan anchors: %w", err)
	}
	r, err := store.NewReader(root)
	if err != nil {
		return nil, fmt.Errorf("lint: open store: %w", err)
	}
	for _, a := range anchors {
		if !r.Exists(ctx, a.Slug) {
			diags = append(diags, Diag{
				Rule:     RuleAnchorDangling,
				Where:    relPath(root, a.FilePath) + ":" + itoa(a.LineNumber),
				Detail:   fmt.Sprintf("anchor points to non-existent intent %q", a.Slug),
				Severity: SeverityError,
			})
		}
	}

	// B-class: store.LoadAll is fail-fast on the first bad file, so any
	// failure becomes a single repo-wide Diag rather than a Go error.
	// Phase 2 P2-2 will relax LoadAll to per-file isolation; until then,
	// one Diag per affected repository is the best we can do.
	intents, loadErr := r.LoadAll(ctx)
	if loadErr != nil {
		diags = append(diags, Diag{
			Rule:     RuleFrontmatterInvalid,
			Where:    "<root>",
			Detail:   loadErr.Error(),
			Severity: SeverityError,
		})
		// Anchor diags are still meaningful even if intent loading failed,
		// so we do not return here.
	}
	_ = intents // future: per-intent body checks live here

	return diags, nil
}

// HasErrors reports whether diags contains any diagnostic with error
// severity. Empty severity is treated as error (matches the legacy
// "kron lint" behavior).
func HasErrors(diags []Diag) bool {
	for _, d := range diags {
		if d.Severity == "" || d.Severity == SeverityError {
			return true
		}
	}
	return false
}

// relPath returns path relative to root if possible, else path. Kept
// here (not in cli) because future MCP / IDE access layers will want
// the same repo-relative formatting for their lint output.
func relPath(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil {
		return rel
	}
	return path
}

// itoa is an allocation-free integer formatter for the line-number
// suffix. Inlined here rather than importing strconv for one call site.
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

// errEmptyRoot is returned by Run when root is empty. Internal because
// it is wrapped at the call site; callers see fmt.Errorf with %w.
var errEmptyRoot = errors.New("lint: root path is empty")
