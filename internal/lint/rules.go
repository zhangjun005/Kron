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
)

// Severity ranks a Diag's importance. Empty is treated as error by
// severity() and HasErrors for backward compatibility with the legacy
// "kron lint" text output, which always defaulted to "error".
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)
