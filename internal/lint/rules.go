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

	// RuleFrontmatterInvalid reports that at least one intent's
	// frontmatter could not be parsed or validated by parser.ParseFrontmatter.
	// Always severity: error. Per-file isolation is a Phase 2 follow-up;
	// v1 reports the first failing load as a single repo-wide Diag.
	RuleFrontmatterInvalid Rule = "frontmatter-invalid"

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

	// RuleAssumptionRationaleRequired (Error, v1.3+ Warning during
	// migration window): an intent's assumptions[] entry has no
	// rationale field. v1.3 emits Warning to ease the A→B
	// migration; v1.5+ emits Error (hard gate). Severity is
	// applied by the linter based on a MigrationMode flag.
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
)

// Severity ranks a Diag's importance. Empty is treated as error by
// severity() and HasErrors for backward compatibility with the legacy
// "kron lint" text output, which always defaulted to "error".
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)
