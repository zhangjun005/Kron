package lint

import (
	"strings"

	"github.com/xxx/kron/internal/model"
)

// runTreeRules applies the T-class tree-shape rules to the loaded
// intents (RFC 2026-10-08-writer-readme-symmetry §5).
//
// Tree-shape contract:
//
//   - A node intent at slug "auth" is physically
//     ".kron/intents/auth/README.md". It is the directory
//     anchor for every child intent at slug "auth/<child>".
//   - A leaf intent at slug "auth/jwt" is physically
//     ".kron/intents/auth/jwt.md". It MUST have its parent
//     directory anchored by a node intent; otherwise a reader
//     that walks the tree has no entry point to display the
//     "auth" module's intent.
//
// Rule mapping (severity hardcoded here so the rule constants
// remain pure identifiers — see rules.go for the canonical list):
//
//	intent-orphan-under-no-node: warning
//
// Leaf-only scope: we report only "leaf with no immediate parent
// node". We do NOT chase multi-level chains (e.g. an orphan at
// "a/b/c" whose parent directory "a/b" is also orphaned). The
// surface is wider but noisy; v1 prefers "fix the obvious one
// first" over "fix every transitive orphan in one shot".
func runTreeRules(intents []*model.Intent) []Diag {
	// First pass: build a set of "node-shaped" slugs (those that
	// have a README.md form on disk). A slug is node-shaped when
	// it appears in the loaded set AND its on-disk path ends in
	// "/README.md" (captured by store.KindFromPath at load time).
	nodeSlugs := make(map[string]struct{}, len(intents))
	for _, in := range intents {
		if in == nil {
			continue
		}
		if in.Kind == model.IntentKindNode {
			nodeSlugs[in.Slug] = struct{}{}
		}
	}

	var out []Diag
	for _, in := range intents {
		if in == nil {
			continue
		}
		// Only leaves can be "orphan under no node" — a node
		// intent at "auth/README.md" is itself the anchor.
		if in.Kind != model.IntentKindLeaf {
			continue
		}
		// Top-level slugs (no "/") have no parent to anchor; skip.
		idx := strings.LastIndex(in.Slug, "/")
		if idx <= 0 {
			continue
		}
		parent := in.Slug[:idx]
		if _, ok := nodeSlugs[parent]; ok {
			continue
		}
		out = append(out, Diag{
			Rule:     RuleIntentOrphanUnderNoNode,
			Where:    in.Slug,
			Detail:   "leaf intent has no parent node intent (no \"" + parent + "/README.md\" found)",
			Severity: SeverityWarning,
		})
	}
	return out
}
