// Package identity resolves the "who is writing this intent" attribution
// that lands in the frontmatter's created_by field.
//
// It is consumed by every access layer that creates intents (CLI
// `kron add`, MCP `kron_add`, future LSP / IDE / GUI add commands),
// so the "how do we get a handle" decision is centralised here rather
// than duplicated as a private helper in each access layer.
//
// Identity is an attribute, not an authz gate. The returned handle is
// a hint for the frontmatter; it is not validated and does not gate
// any operation (architecture.md §2.3).
package identity

import (
	"os/exec"
	"strings"
)

// DefaultUnknown is the handle written to created_by when no source of
// identity could be resolved (no git, no -created-by flag, no MCP
// client info). Format matches the "@user" / "agent:model" convention
// described in model.Frontmatter.CreatedBy.
const DefaultUnknown = "agent:unknown"

// GitUser returns "@<user.name>" by shelling out to `git config user.name`.
// Returns the empty string when git is missing, the user.name is unset,
// or the value is whitespace-only; callers decide how to fall back
// (typically to DefaultUnknown).
//
// On Windows the `git` binary is resolved via PATH. The command is
// synchronous; identity is a fast op (sub-millisecond when cached) and
// runs at most once per add call. If this ever becomes hot, the
// identity layer is the right place to add a per-process cache.
func GitUser() string {
	cmd := exec.Command("git", "config", "user.name")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	name := strings.TrimSpace(string(out))
	if name == "" {
		return ""
	}
	return "@" + name
}

// Handle returns the first non-empty string among the candidates,
// prefixed with "@" only if it does not already start with one. The
// empty string is treated as "no candidate" and skipped. The final
// fallback is DefaultUnknown so the frontmatter never has an empty
// created_by (which model.Frontmatter requires).
//
// Typical use:
//
//	handle := identity.Handle(identity.GitUser(), explicitFlag, mcpClient)
//
// The "@" prefix is added automatically to entries that lack it; the
// explicit-flag path is allowed to pass "@bob" verbatim without
// doubling the prefix. The "agent:<model>" convention (e.g.
// "agent:claude-3-7") already carries a prefix and is passed through
// unchanged.
func Handle(candidates ...string) string {
	for _, c := range candidates {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if strings.HasPrefix(c, "@") || strings.HasPrefix(c, "agent:") {
			return c
		}
		return "@" + c
	}
	return DefaultUnknown
}
