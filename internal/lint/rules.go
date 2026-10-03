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
)

// Severity ranks a Diag's importance. Empty is treated as error by
// severity() and HasErrors for backward compatibility with the legacy
// "kron lint" text output, which always defaulted to "error".
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)