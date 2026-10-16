package lint

// Rule is the stable identifier of a single lint check.
//
// Rule names are part of the public contract with consumers (CI,
// editor plugins, MCP clients). New ids may be added; existing ids
// must never be renamed or removed — that would silently change the
// machine-readable lint output consumers depend on.
type Rule string

const (
	// RuleAnchorDangling reports an anchor ("// @kron:intent <slug>")
	// whose slug does not resolve to an existing .kron/intents/<slug>.md.
	// Always severity: error.
	RuleAnchorDangling Rule = "anchor-dangling"

	// RuleMarkdownAnchorDangling reports a markdown anchor (a
	// "// @kron:intent <slug>" line in any .md file OUTSIDE
	// .kron/intents/ and .kron/.trash/, per RFC
	// 2026-10-08-md-anchors.md §2.1) whose slug does not resolve
	// to an existing .kron/intents/<slug>.md. Always severity:
	// error. Kept as a separate rule id from RuleAnchorDangling
	// so CI dashboards / editor plugin filters can distinguish
	// the two surfaces; both share the same wire-level enum
	// (model.ErrAnchorDangling).
	RuleMarkdownAnchorDangling Rule = "markdown-anchor-dangling"

	// RuleFrontmatterInvalid reports that at least one intent's
	// frontmatter could not be parsed or validated by parser.ParseFrontmatter.
	// Always severity: error. Per-file isolation is a Phase 2 follow-up;
	// v1 reports the first failing load as a single repo-wide Diag.
	RuleFrontmatterInvalid Rule = "frontmatter-invalid"

	// RuleLoadAllFailed is the companion warning emitted when a
	// LoadAll failure causes all per-intent checks (C-class
	// relations, S-class staleness, T-class tree shape, B-3
	// assumption rules) to be skipped. Severity: warning (advisory;
	// the underlying error is already reported as
	// RuleFrontmatterInvalid at error severity).
	RuleLoadAllFailed Rule = "load-all-failed"

	// RuleDanglingReference reports that an intent's Frontmatter.References
	// lists a slug that does not resolve to an existing .kron/intents/<slug>.md.
	// Severity: warning (soft link — informational, not load-bearing).
	RuleDanglingReference Rule = "dangling-reference"

	// RuleDanglingDependsOn reports that an intent's Frontmatter.DependsOn
	// lists a slug that does not resolve to an existing .kron/intents/<slug>.md.
	// Severity: error (hard dependency — broken link breaks comprehension).
	RuleDanglingDependsOn Rule = "dangling-depends-on"

	// RuleDependsOnCycle reports a cycle in the depends_on graph
	// (A.depends_on B AND B.depends_on A, or any longer cycle).
	// Severity: error (cycles are nonsensical — neither intent can be
	// "read first").
	RuleDependsOnCycle Rule = "depends-on-cycle"

	// RuleSelfReference reports that an intent lists its own slug in
	// either References or DependsOn. Severity: error.
	RuleSelfReference Rule = "self-reference"

	// RuleStaleSupersededCandidate reports an active intent whose
	// UpdatedAt is older than the configured staleness threshold
	// (default 90 days). It is the lint twin of MCP `kron_stale`'s
	// "superseded_candidates" list — having the same rule fire under
	// `kron lint` means CI catches the staleness without needing a
	// dedicated cron. Severity: warning (the intent is still
	// authoritative; the warning asks a human to decide).
	//
	// The threshold is supplied via RunOpts.StaleDaysThreshold (0 =
	// skip the rule). When RunOpts is zero-valued, default 90 days is
	// used to match MCP `kron_stale`'s default.
	RuleStaleSupersededCandidate Rule = "stale-superseded-candidate"

	// RuleExpiredHardAssumption reports a hard-severity assumption
	// whose ExpiresAt is in the past AND whose VerifiedAt is either
	// absent or earlier than the expiry. Soft-severity assumptions
	// are excluded by spec (only hard assumptions count for staleness
	// alerts). Severity: warning (the code still works; the warning
	// asks a human to re-verify).
	RuleExpiredHardAssumption Rule = "expired-hard-assumption"

	// B-3 (RFC 2026-10-08-assumptions-standalone): assumption-related
	// rules. These read from .kron/assumptions/<id>.md (the
	// "registry") rather than the intent's inline frontmatter.

	// RuleAssumptionRegistryIdMismatch (Error): an intent references
	// an assumption id that does not exist in the registry. Always
	// severity: error.
	RuleAssumptionRegistryIdMismatch Rule = "assumption-registry-id-mismatch"

	// RuleAssumptionOrphan (Warning): an assumption in the registry
	// is not referenced by any intent. Purely informational; an
	// orphan is still valid (a future intent may adopt it).
	RuleAssumptionOrphan Rule = "assumption-orphan"

	// RuleAssumptionMixedForm (Warning): an intent's assumptions[]
	// mixes inline struct fields (text / expires_at) with B-3 fields
	// (rationale). v1.3 migration window permits this at parse
	// time; the rule nudges authors to clean up.
	RuleAssumptionMixedForm Rule = "assumption-mixed-form"

	// RuleAssumptionFileIdMismatch (Error): .kron/assumptions/<id>.md
	// file's frontmatter id field does not match its file name.
	// Always severity: error.
	RuleAssumptionFileIdMismatch Rule = "assumption-file-id-mismatch"

	// RuleAssumptionRationaleRequired (Error): an intent's
	// assumptions[] entry has no rationale field. Hard Error
	// since v1.3 (the A→B migration Warning window closed per
	// RFC 2026-10-12-close-b3-softening §2.1).
	RuleAssumptionRationaleRequired Rule = "assumption-rationale-required"

	// RuleAssumptionRationaleStale (Warning): the rationale of an
	// intent's assumption reference appears to quote a now-changed
	// registry text. Detection is best-effort: a substring of the
	// registry text that was removed in the current version triggers
	// the warning. v1.3 detection scope is intentionally narrow to
	// keep false positives low; future RFCs can extend it.
	RuleAssumptionRationaleStale Rule = "assumption-rationale-stale"

	// RuleAssumptionSeverityMismatch (Warning): an intent's
	// per-intent severity differs from the registry's
	// default_severity. Purely informational; an override is
	// legitimate when the rationale explains why.
	RuleAssumptionSeverityMismatch Rule = "assumption-severity-mismatch"

	// RuleAssumptionInline (Error, permanent; was Warning under
	// Option B1, upgraded to Error per RFC 2026-10-12 §2.2): an
	// intent's assumptions[] block in frontmatter carries a "text:"
	// key. Per B-3 the text lives in the registry
	// (.kron/assumptions/<id>.md) and is the sole source of truth;
	// an inline `text:` is a hard schema error — the file no longer
	// loads cleanly. Detection: parser.HasInlineAssumptionText
	// (raw-bytes pre-scan; the strict YAML decode also rejects
	// unknown fields, so this is the redundancy layer).
	RuleAssumptionInline Rule = "assumption-inline-text"

	// T-class (RFC 2026-10-08-writer-readme-symmetry): tree-shape
	// rules. These read the on-disk intent layout to flag intents
	// that violate the directory-tree contract.

	// RuleIntentOrphanUnderNoNode (Warning): a leaf intent at
	// "<parent>/<slug>" exists but no node intent at
	// "<parent>/README.md" anchors the parent directory. The
	// intent is still valid on its own — the rule is a prompt
	// for the author to either (a) create a parent node intent
	// or (b) move the leaf to a top-level slug. Leaf-only scope
	// (RFC §5): we do NOT chase deeper chains (an orphan whose
	// parent directory is itself orphaned). Severity: warning
	// because the intent is functional; the warning is a
	// documentation-quality nudge.
	RuleIntentOrphanUnderNoNode Rule = "intent-orphan-under-no-node"
)

// Severity ranks a Diag's importance. Empty is treated as error by
// severity() and HasErrors for backward compatibility with the legacy
// "kron lint" text output, which always defaulted to "error".
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// SuggestionFor returns the human-readable fix hint for a Rule, or ""
// when the engine has no confident recommendation. Wire-up of
// Diag.Suggestion lives in the individual rule functions in
// anchors.go / frontmatter.go / relations.go / staleness.go /
// assumptions.go / tree.go; this is the catalog they read from so all
// suggestions live in one place. Adding a new Rule constant without
// updating this map yields an empty Suggestion (acceptable for
// development; the per-rule site should still call SuggestionFor and
// ignore the empty return).
//
// Format: imperative verb, no trailing punctuation. Keep it short —
// one or two CLI invocations or one sentence of guidance. (C-5,
// docs/mcp/missing-capabilities.md §C-5.)
func SuggestionFor(r Rule) string {
	switch r {
	// C-class anchor / cross-reference rules.
	case RuleAnchorDangling:
		return "Create .kron/intents/<slug>.md or fix the slug in the anchor"
	case RuleMarkdownAnchorDangling:
		return "Create .kron/intents/<slug>.md or fix the slug in the markdown anchor"
	case RuleFrontmatterInvalid:
		return "Open the intent in an editor and repair the YAML; see the parser error"
	case RuleLoadAllFailed:
		return "Run: kron lint and read the frontmatter-invalid diag for the offending file"
	case RuleDanglingReference:
		return "Remove the reference or create .kron/intents/<slug>.md"
	case RuleDanglingDependsOn:
		return "Remove the depends_on entry or create .kron/intents/<slug>.md"
	case RuleDependsOnCycle:
		return "Break the cycle by removing one of the depends_on edges"
	case RuleSelfReference:
		return "Remove the intent's own slug from its references/depends_on"

	// S-class staleness rules.
	case RuleStaleSupersededCandidate:
		return "Run: kron stale to see candidates, then either update or mark superseded"
	case RuleExpiredHardAssumption:
		return "Re-verify the assumption and update VerifiedAt, or remove the reference"

	// B-3 assumption rules.
	case RuleAssumptionRegistryIdMismatch:
		return "Add .kron/assumptions/<id>.md with the referenced id, or fix the reference"
	case RuleAssumptionOrphan:
		return "Reference the assumption from an intent, or delete .kron/assumptions/<id>.md"
	case RuleAssumptionMixedForm:
		return "Move the inline text/rationale into the registry file and keep only the reference"
	case RuleAssumptionFileIdMismatch:
		return "Rename the file to match the frontmatter id (or vice versa)"
	case RuleAssumptionRationaleRequired:
		return "Add a rationale field explaining why the assumption applies to this intent"
	case RuleAssumptionRationaleStale:
		return "Re-read the registry text and update the rationale to match the current version"
	case RuleAssumptionSeverityMismatch:
		return "Either align the per-intent severity with the registry or document the override in rationale"
	case RuleAssumptionInline:
		return "Remove the inline text: key; the registry file is the source of truth"

	// T-class tree-shape rules.
	case RuleIntentOrphanUnderNoNode:
		return "Create a parent README.md node intent at the parent directory, or move the leaf to a top-level slug"
	}
	return ""
}
