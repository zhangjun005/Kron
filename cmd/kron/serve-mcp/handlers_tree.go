package servemcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/xxx/kron/internal/store"
	"github.com/xxx/kron/internal/view"
)

// HandleTree returns the mcp.ToolHandlerFor binding for kron_tree.
//
// Wire contract: see docs/implementation/mcp.md §2 (kron_tree).
//
//	in:  {}
//	out: { root: { slug, name, is_dir, title, status, children: [...] } }
//
// Returns the full intent directory tree built from
// internal/view.BuildIntentTree. The tree shape mirrors the on-disk
// layout (slugs with "/" separators become nested directories);
// see view.IntentTreeNode for the edge cases (README.md collapse,
// "dir + Intent" coexistence).
//
// Designed for GUI multi-project overview + IDE sidebar tree views.
// The response is a single tree rooted at {slug:"", name:"",
// is_dir:true}; callers walk children recursively.
//
// Children is typed as []any in tools_schema.go to avoid the
// go-sdk v1.1 jsonschema inference panic on recursive struct
// pointers. The wire shape is identical to a JSON array of
// TreeNode objects; we populate []any directly from the recursive
// handler call.
func HandleTree(ctx context.Context, _ *mcp.CallToolRequest, _ TreeInput) (
	*mcp.CallToolResult, TreeOutput, error,
) {
	if err := assertCallerMCP(ctx); err != nil {
		return nil, TreeOutput{}, err
	}
	root := repoRootFrom(ctx)

	r, err := store.NewReader(root)
	if err != nil {
		return nil, TreeOutput{}, fmt.Errorf("kron_tree: open store: %w", err)
	}
	all, err := r.LoadAll(ctx)
	if err != nil {
		return nil, TreeOutput{}, fmt.Errorf("kron_tree: load: %w", err)
	}

	internalTree := view.BuildIntentTree(all)
	return nil, TreeOutput{Root: treeNodeFromView(internalTree)}, nil
}

// treeNodeFromView projects a *view.IntentTreeNode to the wire
// *TreeNode shape. Title is derived from the intent's first H1
// (view.IntentTitle) so GUI/IDE consumers don't re-parse the body.
//
// Synthetic intermediate dirs (no Intent pointer) return an empty
// Title/Status but still carry Slug/Name/IsDir so the layout is
// preserved. Children is []any (not []*TreeNode) to break the
// schema cycle; the JSON wire shape is identical.
func treeNodeFromView(n *view.IntentTreeNode) *TreeNode {
	if n == nil {
		return nil
	}
	out := &TreeNode{
		Slug:  n.Slug,
		Name:  n.Name,
		IsDir: n.IsDir,
	}
	if n.Intent != nil {
		out.Title = view.IntentTitle(n.Intent)
		out.Status = string(n.Intent.Frontmatter.Status)
	}
	if len(n.Children) > 0 {
		children := make([]any, 0, len(n.Children))
		for _, c := range n.Children {
			children = append(children, treeNodeFromView(c))
		}
		out.Children = children
	}
	return out
}
