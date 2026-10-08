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
	"time"

	"github.com/xxx/kron/internal/assumption"
	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/parser"
	"github.com/xxx/kron/internal/store"
)

// StaleDaysDefault is the staleness threshold used by
// RuleStaleSupersededCandidate and RuleExpiredHardAssumption when the
// caller does not supply a value. Matches MCP `kron_stale`'s default
// (90 days) so the two surfaces produce the same supersession view.
const StaleDaysDefault = 90

// RunOptions tunes per-Run behaviour. The zero value is meaningful:
// it applies the default staleness threshold (StaleDaysDefault) and
// runs every rule. Set fields explicitly to opt out of specific rules.
type RunOptions struct {
	// StaleDaysThreshold sets the minimum age (in days) at which an
	// active intent becomes a RuleStaleSupersededCandidate. 0 means
	// "use the package default" (StaleDaysDefault). Negative values
	// disable the staleness rules (S1 + S2) entirely, leaving the
	// other rules active.
	StaleDaysThreshold int

	// MigrationMode controls how RuleAssumptionRationaleRequired is
	// surfaced. During the A→B migration window (v1.3 → v1.5),
	// empty-rationale is a Warning to avoid blocking existing repos.
	// v1.5+ flips this to Error. CLI `kron lint` calls RunWith
	// with MigrationMode = true; CI in a v1.5+ project passes
	// MigrationMode = false.
	MigrationMode bool

	// now is the reference time used by the staleness rules. Tests
	// inject a fixed value; production code leaves it zero, which
	// means "time.Now().UTC()" at call time.
	now time.Time
}

// Now overrides the reference time for staleness rules. Test-only
// helper. Returns a new RunOptions value; the receiver is not modified.
func (o RunOptions) WithNow(t time.Time) RunOptions {
	o.now = t
	return o
}

// resolveStaleDays returns the effective threshold and whether the
// staleness rules are enabled.
func (o RunOptions) resolveStaleDays() (days int, enabled bool) {
	if o.StaleDaysThreshold < 0 {
		return 0, false
	}
	if o.StaleDaysThreshold == 0 {
		return StaleDaysDefault, true
	}
	return o.StaleDaysThreshold, true
}

// resolveNow returns the effective reference time for staleness rules.
func (o RunOptions) resolveNow() time.Time {
	if o.now.IsZero() {
		return time.Now().UTC()
	}
	return o.now
}

// Run walks the repository rooted at root and returns every diagnostic
// produced by v1's mandatory rule classes. ctx is reserved for future
// cancellation; v1 does not read it.
//
// Use RunWith when staleness threshold tuning or test-time now
// injection is needed. Run is the no-options equivalent of
// RunWith(ctx, root, RunOptions{MigrationMode: true}).
//
// MigrationMode defaults to TRUE so that the v1.3 → v1.5 migration
// window remains non-blocking for existing repos. v1.5+ callers
// (CI in upgraded projects) pass MigrationMode = false explicitly.
//
// Errors are returned only for scope-cream failures: a missing root,
// an I/O error walking the source tree, or a catastrophic parse failure
// during anchor scanning. Per-file rule failures (including frontmatter
// load failures) are returned as Diag entries — see LoadAllErrorAsDiag.
//
// root may be absolute; ScanAnchors and store.NewReader both call
// filepath.Abs internally.
//
// Rule classes (May 2026 schema + B-3 assumption rules 2026-10-08):
//
//	A-class: anchor-dangling (source code → intent)
//	B-class: frontmatter-invalid (intent .md is unparseable)
//	C-class: dangling-reference, dangling-depends-on, depends-on-cycle,
//	         self-reference (intent → intent frontmatter relations)
//	S-class: stale-superseded-candidate, expired-hard-assumption
//	         (lifecycle staleness; default 90-day threshold)
//	B-3:     assumption-registry-id-mismatch, assumption-orphan,
//	         assumption-mixed-form, assumption-file-id-mismatch,
//	         assumption-rationale-required, assumption-rationale-stale,
//	         assumption-severity-mismatch
//
// C-class rules require all intents to be loaded successfully; when
// LoadAll fails, the per-intent C-class checks are skipped (the
// frontmatter-invalid Diag already explains why) but A-class diags
// (which don't need intent data) are still produced. S-class rules
// follow the same gating as C-class.
func Run(ctx context.Context, root string) ([]Diag, error) {
	return RunWith(ctx, root, RunOptions{MigrationMode: true})
}

// RunWith is the configurable variant of Run.
func RunWith(ctx context.Context, root string, opts RunOptions) ([]Diag, error) {
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

	// B + C-class + S-class: load every intent, then run the
	// frontmatter-relation and staleness rules on each. A failure to
	// load any single intent (LoadAll is fail-fast) becomes a single
	// repo-wide Diag and the per-intent checks are skipped — but the
	// A-class results above are still meaningful and have already been
	// appended.
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

	// S-class: staleness rules. Only the Diag half is consumed by
	// RunWith's return value; the structured StaleReport is
	// recomputed by callers that need it (e.g. MCP kron_stale) via
	// ComputeStaleReport, which takes a pre-loaded intent slice and
	// the same RunOptions. Splitting the two views keeps RunWith's
	// signature stable.
	diags = append(diags, runStalenessRules(intents, opts)...)

	// B-3: assumption registry rules. Skipped cleanly when the
	// registry directory does not exist (pre-migration repos).
	ar, arErr := assumption.NewReader(root)
	var arReader *assumption.Reader
	if arErr == nil {
		arReader = ar
		// file-id-mismatch: walk the registry via the reader
		// (which surfaces each parsed file even when its
		// frontmatter id doesn't match the file name — it
		// returns the file + a wrapped
		// ErrAssumptionFileIdMismatch on the side).
		registry, _ := ar.List(ctx)
		for _, af := range registry {
			if af.Frontmatter.ID != "" && af.Frontmatter.ID != af.Slug {
				diags = append(diags, Diag{
					Rule:     RuleAssumptionFileIdMismatch,
					Where:    af.Slug,
					Detail:   fmt.Sprintf("frontmatter id %q does not match file name %q", af.Frontmatter.ID, af.Slug),
					Severity: SeverityError,
				})
			}
		}
	} else if !errors.Is(arErr, assumption.ErrAssumptionDirNotFound) {
		// Unexpected error (permission, IO) — surface as a Diag so
		// the user sees it instead of getting a silent skip.
		diags = append(diags, Diag{
			Rule:     RuleAssumptionFileIdMismatch,
			Where:    "<root>",
			Detail:   fmt.Sprintf("could not open .kron/assumptions/: %v", arErr),
			Severity: SeverityError,
		})
	}
	diags = append(diags, runAssumptionRules(intents, arReader, opts)...)

	return diags, nil
}

// ComputeStaleReport walks intents and returns the structured view of
// every RuleStaleSupersededCandidate and RuleExpiredHardAssumption
// match. It is the data-shape twin of runStalenessRules: the same
// walk, the same opts, but a StaleReport instead of []Diag.
//
// Callers that need structured stale data (MCP kron_stale, future
// IDE status bar) call this directly with the result of
// store.Reader.LoadAll. The Diag half is what `kron lint` text/JSON
// output renders; this half is what machine consumers consume.
//
// When opts disables staleness rules the returned StaleReport has
// ThresholdDays == 0 and empty slices.
func ComputeStaleReport(intents []*model.Intent, opts RunOptions) StaleReport {
	days, enabled := opts.resolveStaleDays()
	if !enabled {
		return StaleReport{}
	}
	now := opts.resolveNow()
	cutoff := now.AddDate(0, 0, -days)

	report := StaleReport{ThresholdDays: days}
	for _, in := range intents {
		if in.Frontmatter.Status == model.StatusActive &&
			!in.Frontmatter.UpdatedAt.IsZero() &&
			in.Frontmatter.UpdatedAt.Before(cutoff) {
			report.SupersededCandidates = append(report.SupersededCandidates, in.Slug)
		}
		for _, a := range in.Frontmatter.Assumptions {
			if a.Severity != model.SeverityHard || a.ExpiresAt == "" {
				continue
			}
			exp, perr := time.Parse(time.RFC3339, a.ExpiresAt)
			if perr != nil || !exp.Before(now) {
				continue
			}
			if a.VerifiedAt != "" {
				if v, verr := time.Parse(time.RFC3339, a.VerifiedAt); verr == nil && v.After(exp) {
					continue
				}
			}
			daysOverdue := int(now.Sub(exp).Hours() / 24)
			report.ExpiredAssumptions = append(report.ExpiredAssumptions, ExpiredAssumption{
				Slug:         in.Slug,
				AssumptionID: a.ID,
				ExpiresAt:    a.ExpiresAt,
				DaysOverdue:  daysOverdue,
			})
		}
	}
	return report
}

// runStalenessRules is the Diag-shaped twin of ComputeStaleReport.
// It applies the same S-class rules but emits a []Diag suitable for
// the `kron lint` text/JSON output. Kept separate so the two views
// can evolve independently (e.g. adding new fields to StaleReport
// without changing the lint Diag schema).
//
// B-3 (RFC 2026-10-08-assumptions-standalone): S2 (expired hard
// assumption) now reads from the assumption registry when present,
// falling back to inline frontmatter for pre-migration repos. This
// keeps `kron lint` meaningful during the A→B migration window.
func runStalenessRules(intents []*model.Intent, opts RunOptions) []Diag {
	days, enabled := opts.resolveStaleDays()
	if !enabled {
		return nil
	}
	now := opts.resolveNow()
	cutoff := now.AddDate(0, 0, -days)

	var out []Diag
	for _, in := range intents {
		// S1: active intent older than the threshold.
		if in.Frontmatter.Status == model.StatusActive &&
			!in.Frontmatter.UpdatedAt.IsZero() &&
			in.Frontmatter.UpdatedAt.Before(cutoff) {
			daysOld := int(now.Sub(in.Frontmatter.UpdatedAt).Hours() / 24)
			out = append(out, Diag{
				Rule:     RuleStaleSupersededCandidate,
				Where:    in.Slug,
				Detail:   fmt.Sprintf("active intent has not been updated in %d days (threshold %d)", daysOld, days),
				Severity: SeverityWarning,
			})
		}

		// S2: hard-severity assumption past expiry without a
		// post-expiry verification. B-3: in migration window, the
		// assumption metadata is still inline (registry may not
		// exist yet); use the inline shape. When the registry
		// exists, runAssumptionRules covers the registry-aware
		// variant. Here we keep the inline check so repos mid-
		// migration still get the warning.
		for _, a := range in.Frontmatter.Assumptions {
			if a.Severity != model.SeverityHard || a.ExpiresAt == "" {
				continue
			}
			exp, perr := time.Parse(time.RFC3339, a.ExpiresAt)
			if perr != nil || !exp.Before(now) {
				continue
			}
			if a.VerifiedAt != "" {
				if v, verr := time.Parse(time.RFC3339, a.VerifiedAt); verr == nil && v.After(exp) {
					continue
				}
			}
			daysOverdue := int(now.Sub(exp).Hours() / 24)
			out = append(out, Diag{
				Rule:     RuleExpiredHardAssumption,
				Where:    in.Slug,
				Detail:   fmt.Sprintf("hard assumption %q expired %d days ago (expired_at %s)", a.ID, daysOverdue, a.ExpiresAt),
				Severity: SeverityWarning,
			})
		}
	}
	return out
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
