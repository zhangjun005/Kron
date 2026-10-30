package servemcp

// Project-level intent body template injection (AI-facing only).
//
// Different teams want different intent-body section shapes (e.g. `## Why` /
// `## How`). The template is a project convention, so it lives at
// .kron/templates/intent-body.md. It is never written into intents by the
// server: its only job is to reach the AI through MCP prompts/metadata —
// initialization instructions, the kron_add/kron_update tool descriptions,
// and the kron-implement prompt.
//
// An absent or empty file disables injection, preserving the previous wire
// shape for projects that do not opt in.

import (
	"os"
	"path/filepath"
	"strings"
)

const intentBodyTemplatePath = ".kron/templates/intent-body.md"

// readIntentBodyTemplate returns the trimmed template body, or "" when the
// file is absent (the common, backward-compatible case).
func readIntentBodyTemplate(root string) string {
	if root == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(intentBodyTemplatePath)))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// intentBodyTemplateSection renders a fenced markdown section that can be
// appended to instructions or prompt text.
func intentBodyTemplateSection(template string) string {
	if template == "" {
		return ""
	}
	return "\n\n## Project intent body template\n\n" +
		"Use this structure for the Markdown body of intents you create or update " +
		"(frontmatter is generated separately):\n\n```markdown\n" + template + "\n```\n"
}

// serverInstructionsFor appends the template section to the static workflow
// instructions. It is used for the MCP initialize result.
func serverInstructionsFor(template string) string {
	return serverInstructions + intentBodyTemplateSection(template)
}

// intentToolDescription appends the template to a tool description so the AI
// sees the required body shape when it decides to call kron_add/kron_update.
func intentToolDescription(base, template string) string {
	if template == "" {
		return base
	}
	return base + "\n\nThis project's intent body template (use it for the `body` field):\n\n```markdown\n" +
		template + "\n```"
}
