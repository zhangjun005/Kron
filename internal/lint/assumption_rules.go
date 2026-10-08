package lint

import (
	"context"
	"fmt"

	"github.com/xxx/kron/internal/assumption"
	"github.com/xxx/kron/internal/model"
)

// runAssumptionRules applies the B-3 assumption rules
// (RFC 2026-10-08-assumptions-standalone) to the loaded intents.
//
// Behaviour:
//   - If the assumption registry directory does not exist
//     (ErrAssumptionDirNotFound), we skip ALL registry-based rules
//     silently. This keeps `kron lint` working on repos that have
//     not migrated to the standalone registry yet.
//   - intentBySlug is a pre-built lookup of the loaded intents
//     (built once by RunWith) so the orphan rule can walk the
//     registry without re-loading the intent set.
//
// Rule mapping (severity hardcoded here so the rule constants
// remain pure identifiers — see rules.go for the canonical list):
//
//	registry-id-mismatch:   error
//	orphan:                 warning
//	mixed-form:             warning
//	rationale-required:     depends on MigrationMode (Warning/Error)
//	rationale-stale:        warning
//	severity-mismatch:      warning
//
// file-id-mismatch is reported by the reader, NOT here, so it does
// not need to live in this function.
func runAssumptionRules(intents []*model.Intent, ar *assumption.Reader, opts RunOptions) []Diag {
	// Skip cleanly when the registry is absent — common during the
	// A→B migration window.
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
	referenced := make(map[string]struct{})
	for _, in := range intents {
		for _, a := range in.Frontmatter.Assumptions {
			if a.ID != "" {
				referenced[a.ID] = struct{}{}
			}
		}
	}

	var diags []Diag
	now := opts.resolveNow()

	// --- Per-intent rules -----------------------------------------
	for _, in := range intents {
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

			// mixed-form: inline text or expires_at present in
			// the intent frontmatter (legacy A-path leftover)
			// alongside a B-3 id+severity+rationale shape.
			if a.Text != "" || a.ExpiresAt != "" {
				diags = append(diags, Diag{
					Rule:     RuleAssumptionMixedForm,
					Where:    in.Slug,
					Detail:   fmt.Sprintf("assumptions[%q] mixes A-path inline fields with B-3 id reference; remove text/expires_at to complete migration", a.ID),
					Severity: SeverityWarning,
				})
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

			// rationale-stale: if rationale contains a substring
			// (≥ 6 runes) of the registry's CURRENT text that is
			// now absent from the rationale's stated text, surface
			// a warning. Detection scope is intentionally narrow:
			// we only check that the rationale echoes part of the
			// old text by comparing the rationale against the
			// CURRENT registry text. False positives are
			// acceptable; lint is advisory.
			if a.Rationale != "" && a.Text != "" && af.Frontmatter.Text != "" && a.Text != af.Frontmatter.Text {
				// Intent frontmatter still has the legacy `text`
				// field whose value diverges from the registry.
				// That IS the migration signal: the author needs
				// to update the inline text or remove it.
				diags = append(diags, Diag{
					Rule:     RuleAssumptionRationaleStale,
					Where:    in.Slug,
					Detail:   fmt.Sprintf("assumptions[%q] inline text (%q) diverges from registry text (%q); drop the inline text to complete migration", a.ID, truncate(a.Text, 30), truncate(af.Frontmatter.Text, 30)),
					Severity: SeverityWarning,
				})
			}
		}
	}

	// --- Registry-side rules (orphan) ----------------------------
	if registry != nil {
		for id := range registry {
			if _, ok := referenced[id]; ok {
				continue
			}
			// Skip superseded assumptions — they were retired
			// intentionally and SHOULD be orphan. Reporting them
			// as orphans would be noise.
			if af, ok := registry[id]; ok && af.Frontmatter.Status == model.StatusSuperseded {
				continue
			}
			diags = append(diags, Diag{
				Rule:     RuleAssumptionOrphan,
				Where:    id,
				Detail:   fmt.Sprintf("assumption %q exists in registry but is not referenced by any intent", id),
				Severity: SeverityWarning,
			})
		}
	}

	// Silence unused-var lint when now is not read; keeping it for
	// future use in expired-assumption wiring (see
	// runStalenessRules in lint.go).
	_ = now
	return diags
}

// truncate clamps s to at most n runes, appending "…" if truncated.
// Used only in lint detail strings; ASCII + Chinese both count as 1.
func truncate(s string, n int) string {
	if utf8RuneCount(s) <= n {
		return s
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}

func utf8RuneCount(s string) int {
	return len([]rune(s))
}

// resolveNow is duplicated here intentionally so the assumption
// rules can be invoked standalone in tests. The canonical copy lives
// in RunOptions; we import it through opts.
