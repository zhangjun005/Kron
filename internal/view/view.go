// Package view provides in-memory view shapes over the raw data
// loaded by store / assumption / parser. Views are computed lazily
// from pre-loaded slices — no I/O, no caching, no side effects.
//
// This package is the destination for "compute a shape the access
// layer (MCP / LSP / IDE / GUI) needs without going back to disk".
// It is intentionally separate from relations (which is purely
// graph-shape computation) and from store (which is purely I/O).
//
// All view types are plain structs with a Build / Sort / Filter
// method family. The rule: callers obtain a snapshot, operate on
// it freely, and discard. The view is never persisted.
package view

import (
	"sort"
	"strings"

	"github.com/xxx/kron/internal/model"
)

// IntentTreeNode is one node in the intent directory tree.
//
// Slug is the on-disk slug (e.g. "auth/jwt" or "auth"). For a
// directory node, IsDir is true and Children holds sub-entries.
// For a leaf node, IsDir is false. A node can be BOTH a directory
// AND carry an Intent pointer (when an intent at "auth" exists
// alongside sub-intents at "auth/jwt") — see BuildIntentTree.
//
// Children is sorted by Sort() (directories first, then
// lexicographic Name). Modifying the slice after Sort invalidates
// the ordering.
type IntentTreeNode struct {
	// Slug is this node's slug, e.g. "auth" (dir) or "auth/jwt" (leaf).
	// For the root, Slug is "".
	Slug string

	// Name is the final path segment, e.g. "auth" or "jwt".
	// For the root, Name is "".
	Name string

	// IsDir is true when this node has children. A node with a
	// single-segment slug AND no children has IsDir == false.
	IsDir bool

	// Intent is the loaded *model.Intent when this node corresponds
	// to an on-disk .md file. May be set on both leaf-only nodes
	// (slug "auth") and directory nodes (slug "auth" + children from
	// "auth/jwt"). nil for purely-synthetic intermediate nodes.
	Intent *model.Intent

	// Children is sorted lexicographically by Name after BuildIntentTree
	// returns.
	Children []*IntentTreeNode
}

// BuildIntentTree builds an in-memory tree from a flat intent slice.
//
// The tree is rooted at a synthetic root node with Slug == "". For
// each unique prefix path implied by the intents' slugs (e.g.
// "auth/jwt" implies the directory "auth"), a directory node is
// created. Leaf nodes carry the *model.Intent pointer.
//
// Edge case: a leaf slug "auth" AND a sub-slug "auth/jwt" coexist.
// The node "auth" is then BOTH a directory (has child "jwt") AND
// has its own Intent pointer (the leaf at "auth"). This preserves
// both pieces of data without dropping either.
//
// Empty input returns a single root with no children (non-nil).
//
// Note: this function does NOT consult the filesystem. The input
// slice is the source of truth; if a file exists on disk but was
// not in intents, it does not appear in the tree. This matches
// the in-memory "no I/O" rule of the relations package.
//
// README.md handling: a file like "auth/README.md" is collapsed by
// store.walkIntentSlugs to slug "auth" (the directory-name shorthand
// — RFC 2026-10-08-intent-tree-api.md §3.1, A1). BuildIntentTree
// therefore receives "auth" (carrying the README's Intent) alongside
// any "auth/<child>" slugs, and the existing "dir + Intent coexist"
// logic (see TestBuildIntentTree_READMEDirNode) places the README on
// the dir node automatically. No view-layer special-casing needed.
func BuildIntentTree(intents []*model.Intent) *IntentTreeNode {
	root := &IntentTreeNode{Slug: "", Name: "", IsDir: true}

	// Index nodes by their full slug so we don't create the same
	// node twice when multiple intents share a prefix. A node
	// here may be purely a directory (created first by a longer
	// slug) and later get its Intent pointer filled in by a
	// shorter slug.
	nodes := map[string]*IntentTreeNode{"": root}

	for _, in := range intents {
		if in == nil {
			continue
		}
		segments := strings.Split(in.Slug, "/")
		currentSlug := ""
		var parent *IntentTreeNode = root
		for i, seg := range segments {
			if i > 0 {
				currentSlug += "/"
			}
			currentSlug += seg

			node, exists := nodes[currentSlug]
			if i == len(segments)-1 {
				// Leaf: this node is exactly `in.Slug`. It might
				// already exist as a directory created by a longer
				// slug; in that case just attach the Intent.
				if !exists {
					node = &IntentTreeNode{
						Slug:     in.Slug,
						Name:     seg,
						IsDir:    false,
						Intent:   in,
						Children: nil,
					}
					nodes[currentSlug] = node
					parent.Children = append(parent.Children, node)
				} else {
					node.Intent = in
				}
			} else {
				// Intermediate directory: ensure a node exists.
				if !exists {
					node = &IntentTreeNode{
						Slug:     currentSlug,
						Name:     seg,
						IsDir:    true,
						Children: nil,
					}
					nodes[currentSlug] = node
					parent.Children = append(parent.Children, node)
				}
				node.IsDir = true
				parent = node
			}
		}
	}

	root.Sort()
	return root
}

// Sort recursively orders children lexicographically by Name.
// Directories sort before leaves at the same Name level so the
// tree reads "auth/, auth/jwt, auth/jwt-sliding-window" rather
// than interleaving. The root node is sorted last (children first).
//
// Stable across calls: idempotent.
func (n *IntentTreeNode) Sort() {
	if n == nil {
		return
	}
	// Sort children: directories first, then by Name.
	sort.SliceStable(n.Children, func(i, j int) bool {
		if n.Children[i].IsDir != n.Children[j].IsDir {
			return n.Children[i].IsDir // dirs first
		}
		return n.Children[i].Name < n.Children[j].Name
	})
	for _, c := range n.Children {
		c.Sort()
	}
}

// Flatten returns every node in pre-order (parents before children,
// siblings in Sort() order). Includes both leaf-only and
// dir-with-Intent nodes; purely-synthetic intermediate dirs
// (no Intent, no children) are not emitted.
//
// A node that is BOTH a directory AND has its own Intent
// (e.g. "auth" + "auth/jwt" both exist) is emitted at its position
// in pre-order, then its children follow.
//
// Useful for callers that need a deterministic tree ordering
// (e.g. lint walks, MCP list responses) without re-implementing
// the tree walk.
//
// Returns an empty slice (not nil) when the tree is empty.
func (n *IntentTreeNode) Flatten() []*IntentTreeNode {
	out := []*IntentTreeNode{}
	n.flattenInto(&out)
	return out
}

func (n *IntentTreeNode) flattenInto(out *[]*IntentTreeNode) {
	if n == nil {
		return
	}
	// Emit this node if it has an Intent. Purely-synthetic
	// intermediates (no Intent, only a grouping role) are skipped.
	if n.Intent != nil {
		*out = append(*out, n)
	}
	for _, c := range n.Children {
		c.flattenInto(out)
	}
}

// IntentTitle extracts a human-readable title from an intent's body.
//
// Strategy (in order):
//  1. First H1 line in body ("# Title")
//  2. Fallback: TitleCase(slug's last segment) with dashes → spaces
//
// Empty body returns "". Nil intent returns "".
//
// This is a view-layer concern: Body parsing is intentionally
// lightweight (no full markdown AST). The "first H1" rule is
// good enough for intent display; LSP hover and IDE panels can
// re-render the full body if they want.
func IntentTitle(in *model.Intent) string {
	if in == nil {
		return ""
	}
	for _, line := range strings.Split(in.Body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		}
	}
	// Fallback: title-case the last slug segment.
	seg := in.Slug
	if idx := strings.LastIndex(seg, "/"); idx >= 0 {
		seg = seg[idx+1:]
	}
	seg = strings.ReplaceAll(seg, "-", " ")
	if seg == "" {
		return in.Slug
	}
	// Title-case: uppercase first letter, keep rest as-is.
	return strings.ToUpper(seg[:1]) + seg[1:]
}
