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
	"strings"

	"github.com/xxx/kron/internal/parser"
	"github.com/xxx/kron/internal/store"
)

// Run walks the repository rooted at root and returns every diagnostic
// produced by v1's mandatory rule classes. ctx is reserved for future
// cancellation; v1 does not read it.
//
// Errors are returned only for scope-cream failures: a missing root,
// an I/O error walking the source tree, or a catastrophic parse failure
// during anchor scanning. Per-file rule failures (including frontmatter
// load failures) are returned as Diag entries — see LoadAllErrorAsDiag.
//
// root may be absolute; ScanAnchors and store.NewReader both call
// filepath.Abs internally.
//
// Rule classes (May 2026 schema):
//
//	A-class: anchor-dangling (source code → intent)
//	B-class: frontmatter-invalid (intent .md is unparseable)
//	C-class: dangling-reference, dangling-depends-on, depends-on-cycle,
//	         self-reference (intent → intent frontmatter relations)
//
// C-class rules require all intents to be loaded successfully; when
// LoadAll fails, the per-intent C-class checks are skipped (the
// frontmatter-invalid Diag already explains why) but A-class diags
// (which don't need intent data) are still produced.
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

	// B + C-class: load every intent, then run the frontmatter-relation
	// rules on each. A failure to load any single intent (LoadAll is
	// fail-fast) becomes a single repo-wide Diag and the per-intent
	// C-class checks are skipped — but the A-class results above are
	// still meaningful and have already been appended.
	intents, loadErr := r.LoadAll(ctx)
	if loadErr != nil {
		diags = append(diags, Diag{
			Rule:     RuleFrontmatterInvalid,
			Where:    "<root>",
			Detail:   loadErr.Error(),
			Severity: SeverityError,
		})
		return diags, nil
	}

	// C-class: per-intent checks for references / depends_on / cycles.
	// We build a slug set once for the dangling checks; we build a
	// depends_on adjacency map once for the cycle check.
	knownSlugs := make(map[string]struct{}, len(intents))
	for _, in := range intents {
		knownSlugs[in.Slug] = struct{}{}
	}

	// dependsOnAdj[from] = list of to-slugs this intent depends on.
	dependsOnAdj := make(map[string][]string, len(intents))
	for _, in := range intents {
		dependsOnAdj[in.Slug] = in.Frontmatter.DependsOn
	}

	// Cycle detection: for each intent, DFS through depends_onAdj
	// looking for a back-edge to the start node. Emits one Diag per
	// cycle entry point (the first time a node in the cycle is visited
	// as a start). This may report overlapping cycles; v1 prefers
	// clarity over deduplication.
	for _, in := range intents {
		if cycle := detectDependsOnCycle(in.Slug, dependsOnAdj); cycle != nil {
			diags = append(diags, Diag{
				Rule:     RuleDependsOnCycle,
				Where:    in.Slug,
				Detail:   fmt.Sprintf("depends_on cycle: %s", strings.Join(cycle, " -> ")),
				Severity: SeverityError,
			})
		}
	}

	// Per-intent checks: self-reference + dangling reference/depends_on.
	for _, in := range intents {
		for _, ref := range in.Frontmatter.References {
			if ref == in.Slug {
				diags = append(diags, Diag{
					Rule:     RuleSelfReference,
					Where:    in.Slug,
					Detail:   fmt.Sprintf("references contains self (%q)", ref),
					Severity: SeverityError,
				})
				continue
			}
			if _, ok := knownSlugs[ref]; !ok {
				diags = append(diags, Diag{
					Rule:     RuleDanglingReference,
					Where:    in.Slug,
					Detail:   fmt.Sprintf("references non-existent intent %q", ref),
					Severity: SeverityWarning,
				})
			}
		}
		for _, dep := range in.Frontmatter.DependsOn {
			if dep == in.Slug {
				diags = append(diags, Diag{
					Rule:     RuleSelfReference,
					Where:    in.Slug,
					Detail:   fmt.Sprintf("depends_on contains self (%q)", dep),
					Severity: SeverityError,
				})
				continue
			}
			if _, ok := knownSlugs[dep]; !ok {
				diags = append(diags, Diag{
					Rule:     RuleDanglingDependsOn,
					Where:    in.Slug,
					Detail:   fmt.Sprintf("depends_on non-existent intent %q", dep),
					Severity: SeverityError,
				})
			}
		}
	}

	return diags, nil
}

// detectDependsOnCycle returns a path describing the cycle reachable
// from start via the depends_onAdj adjacency map, or nil if no cycle
// is reachable from start. The returned path starts and ends at start
// (e.g. ["A", "B", "C", "A"] for A->B->C->A).
//
// Algorithm: standard iterative DFS with a per-path "on stack" set.
// We use iterative (not recursive) to avoid blowing the goroutine
// stack on pathological input (an intent with thousands of depends_on
// entries pointing at long chains).
func detectDependsOnCycle(start string, adj map[string][]string) []string {
	type frame struct {
		node string
		idx  int // next child index to explore
	}
	path := make([]string, 0, 8)
	onPath := make(map[string]int, 8) // node -> position in path, or absent
	stack := []frame{{node: start}}

	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		if top.idx == 0 {
			// First visit to this node on the current path.
			onPath[top.node] = len(path)
			path = append(path, top.node)
		}
		children, hasChildren := adj[top.node]
		if !hasChildren || top.idx >= len(children) {
			// Done with this node; pop.
			delete(onPath, top.node)
			path = path[:len(path)-1]
			stack = stack[:len(stack)-1]
			continue
		}
		next := children[top.idx]
		top.idx++
		if pos, seen := onPath[next]; seen {
			// Found a cycle: extract the loop portion of the path
			// from `pos` onward, then append `next` to close it.
			cycle := append([]string{}, path[pos:]...)
			cycle = append(cycle, next)
			return cycle
		}
		stack = append(stack, frame{node: next})
	}
	return nil
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
