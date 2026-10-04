package parser

import (
	"fmt"
	"strings"
)

// IntentBodyTemplate returns the default Markdown body for a freshly
// scaffolded intent. The body follows the §三 skeleton from
// docs/abstractDesign/intent-structure.md (Title + one-line summary +
// Why + Trade-offs + Assumptions pointer).
//
// The English template matches the previous serve-mcp
// handlers_add.go:scaffoldedBody; the CLI's
// cmd/kron/cli/add.go:defaultIntentBody used a Chinese variant
// (为什么 / 权衡) which is preserved as IntentBodyTemplateCN for
// back-compat. The default callers should pass IntentBodyTemplate
// (English) to keep wire / on-disk shape consistent across access
// layers; CLI opts in to the Chinese template by passing
// IntentBodyTemplateCN explicitly.
//
// The slug is converted to a title by taking the last path segment
// (everything after the final "/") and replacing dashes with
// spaces. This matches the prior behaviour in both access layers.
func IntentBodyTemplate(slug string) string {
	return intentBody(slug, "## Why", "## Trade-offs",
		"Record the reason for this decision.",
		"What was chosen and what was given up.")
}

// IntentBodyTemplateCN is the Chinese-language variant of
// IntentBodyTemplate. The CLI subcommand kron add used this
// wording (为什么 / 权衡) before the templates were consolidated;
// the English template is now the default. Both call sites use
// the same shape (Title, one-line summary, Why, Trade-offs,
// assumptions pointer) so consumers don't need to special-case
// per access layer.
func IntentBodyTemplateCN(slug string) string {
	return intentBody(slug, "## 为什么（Why）", "## 权衡（Trade-offs）",
		"记录当初为何如此决策。", "选择与放弃的考量。")
}

func intentBody(slug, whyHeading, tradeHeading, whyText, tradeText string) string {
	title := slug
	if i := strings.LastIndex(slug, "/"); i >= 0 {
		title = slug[i+1:]
	}
	title = strings.ReplaceAll(title, "-", " ")

	return fmt.Sprintf("# %s\n\n> One-line summary of this intent's decision.\n\n%s\n\n%s\n\n%s\n\n%s\n\n<!-- 边界假设写在 frontmatter 的 assumptions 字段里，不在正文重复 -->\n",
		title, whyHeading, whyText, tradeHeading, tradeText)
}
