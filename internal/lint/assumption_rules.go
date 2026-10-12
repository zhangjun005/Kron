// Package lint: assumption rule engine for v1.3+ (B-3 standalone registry).
//
// runAssumptionRules applies the B-3 + Option-B1 assumption rules:
//
//  1. Per-intent rules (frontmatter.assumptions[] walk):
//     - RuleAssumptionRationaleRequired (v1.3 Warning / v1.5+ Error)
//     - RuleAssumptionRegistryIdMismatch (Error): id not in registry
//     - RuleAssumptionMixedForm (Warning): legacy A-path leftover fields
//     (expires_at on the per-intent side) — kept for v1.3 migration
//     - RuleAssumptionSeverityMismatch (Warning): per-intent severity
//     override of the registry's default_severity
//     - RuleAssumptionRationaleStale (Warning): per-intent inline
//     text diverges from registry text (legacy A-path signal)
//
//  2. Registry-side rules:
//     - RuleAssumptionFileIdMismatch (Error): emitted by RunWith
//     directly via the reader, NOT here
//     - RuleAssumptionOrphan (Warning): active registry entry with
//     zero references (no frontmatter.Assumptions[].id match AND
//     no @kron:assumes <id> marker in any intent body)
//
//  3. Raw-byte rules (no Go model survives the trip):
//     - RuleAssumptionInline (Warning): a "text:" key inside the
//     top-level "assumptions:" block of an intent's frontmatter.
//     Detected by parser.HasInlineAssumptionText because
//     model.Assumption no longer has a Text field — the strict
//     YAML decode silently drops the key, so we re-scan the
//     raw bytes to surface the migration leftover.
//
// Behaviour:
//   - If the assumption registry directory does not exist
//     (ErrAssumptionDirNotFound), we skip the registry-based
//     rules silently. This keeps `kron lint` working on repos
//     that have not migrated to the standalone registry yet.
//   - Per-intent rules (rationale, severity, etc.) still fire
//     even without a registry — they don't depend on the
//     registry for their signal.
//
// root is the repo root; pass it through to (a) read raw
// frontmatter bytes for the RuleAssumptionInline pre-scan and
// (b) walk the @kron:assumes body markers for the orphan
// reverse lookup.
package lint

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/xxx/kron/internal/assumption"
	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/parser"
)

func runAssumptionRules(root string, intents []*model.Intent, ar *assumption.Reader, opts RunOptions) []Diag {
	// Registry snapshot. May be nil on pre-migration repos.
	var registry map[string]*model.AssumptionFile
	if ar != nil {
		list, err := ar.List(context.Background())
		if err == nil {
			registry = make(map[string]*model.AssumptionFile, len(list))
			for _, af := range list {
				registry[af.Slug] = af
			}
		}
	}

	// Build intent-side reference set for the orphan rule.
	// "Referenced" means: at least one intent has a matching
	// id in its frontmatter.assumptions[] OR has an
	// @kron:assumes <id> marker in its body. The orphan rule
	// is the conjunction (both must be zero).
	referencedViaFM := make(map[string]struct{})
	for _, in := range intents {
		for _, a := range in.Frontmatter.Assumptions {
			if a.ID != "" {
				referencedViaFM[a.ID] = struct{}{}
			}
		}
	}

	// Pre-load raw frontmatter bytes per intent so we can run
	// HasInlineAssumptionText. We deliberately re-read from
	// disk (not store the bytes on model.Intent) because the
	// model is a pure data struct and adding an io handle would
	// bloat every consumer. The per-file read is small (a
	// single .md < 100 KiB) and idempotent.
	intentRawFM := make(map[string][]byte, len(intents))
	if root != "" {
		for _, in := range intents {
			if in.SourcePath == "" {
				continue
			}
			data, err := os.ReadFile(in.SourcePath)
			if err != nil {
				continue // surface as RuleFrontmatterInvalid elsewhere
			}
			fmRaw, _, splitErr := parser.SplitMarkdown(data)
			if splitErr != nil {
				continue
			}
			intentRawFM[in.Slug] = fmRaw
		}
	}

	var diags []Diag
	_ = opts.resolveNow() // reserved for future use (already dropped)

	// --- Per-intent rules -----------------------------------------
	for _, in := range intents {
		// RuleAssumptionInline: any "text:" key inside the
		// top-level "assumptions:" block. We test the raw
		// frontmatter bytes (not the decoded model) because
		// model.Assumption no longer has a Text field.
		if raw, ok := intentRawFM[in.Slug]; ok {
			if count, has := parser.HasInlineAssumptionText(raw); has {
				diags = append(diags, Diag{
					Rule:     RuleAssumptionInline,
					Where:    in.Slug,
					Detail:   fmt.Sprintf("assumptions[] block carries %d inline text: key(s); per B-3 the assumption text lives in .kron/assumptions/<id>.md and the inline copy should be removed", count),
					Severity: SeverityWarning,
				})
			}
		}

		for _, a := range in.Frontmatter.Assumptions {
			// rationale-required (id is required at parse time, so
			// an empty ID here is a parser-level error and won't
			// reach lint).
			if a.Rationale == "" {
				severity := SeverityWarning
				if !opts.MigrationMode {
					severity = SeverityError
				}
				diags = append(diags, Diag{
					Rule:     RuleAssumptionRationaleRequired,
					Where:    in.Slug,
					Detail:   fmt.Sprintf("assumptions[%q] has no rationale (required ≥ 10 chars; B-3)", a.ID),
					Severity: severity,
				})
			}

			// Skip registry-dependent rules when no registry exists.
			if registry == nil {
				continue
			}

			af, ok := registry[a.ID]
			if !ok {
				diags = append(diags, Diag{
					Rule:     RuleAssumptionRegistryIdMismatch,
					Where:    in.Slug,
					Detail:   fmt.Sprintf("references non-existent assumption %q in .kron/assumptions/", a.ID),
					Severity: SeverityError,
				})
				continue
			}

			// severity-mismatch: per-intent severity differs from
			// registry default_severity. Informational only.
			if a.Severity != "" && af.Frontmatter.DefaultSeverity != "" && a.Severity != af.Frontmatter.DefaultSeverity {
				diags = append(diags, Diag{
					Rule:  RuleAssumptionSeverityMismatch,
					Where: in.Slug,
					Detail: fmt.Sprintf("assumptions[%q] severity %q differs from registry default_severity %q (override is fine if rationale explains why)",
						a.ID, a.Severity, af.Frontmatter.DefaultSeverity),
					Severity: SeverityWarning,
				})
			}

			// rationale-stale: per-intent rationale contains a
			// substring of the registry's CURRENT text that no
			// longer makes sense in the current text. Detection
			// is best-effort and intentionally narrow.
			//
			// B-3 (Option B1): the per-intent side no longer
			// carries a Text field, so the "stale rationale"
			// signal degrades to a substring check against the
			// registry text + the rationale's own wording.
			// False positives are acceptable; lint is advisory.
			if a.Rationale != "" && af.Frontmatter.Text != "" &&
				!strings.Contains(a.Rationale, af.Frontmatter.Text) &&
				hasCommonSubstring(a.Rationale, af.Frontmatter.Text, 6) {
				diags = append(diags, Diag{
					Rule:     RuleAssumptionRationaleStale,
					Where:    in.Slug,
					Detail:   fmt.Sprintf("assumptions[%q] rationale echoes part of the registry text (%q); re-read the registry and refresh the rationale", a.ID, truncate(af.Frontmatter.Text, 30)),
					Severity: SeverityWarning,
				})
			}
		}
	}

	// --- Registry-side rules (orphan) ----------------------------
	//
	// B-3 (Option B1): the orphan rule now walks both surfaces:
	//
	//  1. The frontmatter.Assumptions[].id surface (built above
	//     into referencedViaFM).
	//  2. The @kron:assumes <id> body-marker surface (one parser
	//     call per registry entry; the walk is bounded by the
	//     registry size which is small).
	//
	// An assumption is "referenced" iff it appears in EITHER
	// surface. Skipped status is exempted (same exemption as
	// the pre-refactor rule).
	if registry != nil && root != "" {
		// Walk the body-marker surface once per registry entry.
		// parser.SlugsForAssumption does a full repo walk; calling
		// it N times for N registry entries is O(N * repo size),
		// which is acceptable for the v1 registry size (tens of
		// entries, sub-millisecond on a small repo). A future
		// optimisation can swap to a single ScanAssumptionAnchors
		// call + group-by-id in memory.
		referencedViaMarker := make(map[string]struct{}, len(registry))
		for id := range registry {
			slugs, err := parser.SlugsForAssumption(root, id)
			if err != nil {
				continue // treat as "no marker references" for the
				// orphan rule; lint can always retry next run
			}
			if len(slugs) > 0 {
				referencedViaMarker[id] = struct{}{}
			}
		}
		for id, af := range registry {
			if _, ok := referencedViaFM[id]; ok {
				continue
			}
			if _, ok := referencedViaMarker[id]; ok {
				continue
			}
			// Skip superseded assumptions — they were retired
			// intentionally and SHOULD be orphan. Reporting them
			// as orphans would be noise.
			if af.Frontmatter.Status == model.StatusSuperseded {
				continue
			}
			diags = append(diags, Diag{
				Rule:     RuleAssumptionOrphan,
				Where:    id,
				Detail:   fmt.Sprintf("assumption %q exists in registry but is not referenced by any intent (no frontmatter.assumptions[].id match and no @kron:assumes marker)", id),
				Severity: SeverityWarning,
			})
		}
	}

	return diags
}

// hasCommonSubstring reports whether a and b share a substring of
// at least minRunes consecutive runes. Used by the rationale-stale
// heuristic; a small/false-positive-prone function is fine because
// the rule is advisory.
func hasCommonSubstring(a, b string, minRunes int) bool {
	if len([]rune(a)) < minRunes || len([]rune(b)) < minRunes {
		return false
	}
	// Naive O(|a| * |b|) check. Both strings are rationale
	// text + a registry description, so a few hundred runes
	// at most — fine for an advisory lint rule.
	ra := []rune(a)
	rb := []rune(b)
	for i := 0; i+minRunes <= len(ra); i++ {
		for j := 0; j+minRunes <= len(rb); j++ {
			match := true
			for k := 0; k < minRunes; k++ {
				if ra[i+k] != rb[j+k] {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
	}
	return false
}

// truncate clamps s to at most n runes, appending "…" if truncated.
// Used only in lint detail strings; ASCII + Chinese both count as 1.
func truncate(s string, n int) string {
	if n <= 0 {
		return "…"
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}
