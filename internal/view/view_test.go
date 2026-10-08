package view

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xxx/kron/internal/model"
)

func mkIntent(slug string, body string) *model.Intent {
	return &model.Intent{
		Slug: slug,
		Frontmatter: model.Frontmatter{
			CreatedBy: "@a",
			UpdatedAt: time.Now(),
		},
		Body: body,
	}
}

func TestBuildIntentTree_Empty(t *testing.T) {
	tree := BuildIntentTree(nil)
	require.NotNil(t, tree)
	assert.True(t, tree.IsDir)
	assert.Empty(t, tree.Slug)
	assert.Empty(t, tree.Children)
	assert.Empty(t, tree.Flatten())
}

func TestBuildIntentTree_SingleLeaf(t *testing.T) {
	a := mkIntent("a", "# A")
	tree := BuildIntentTree([]*model.Intent{a})
	require.Len(t, tree.Children, 1)
	assert.False(t, tree.Children[0].IsDir)
	assert.Equal(t, "a", tree.Children[0].Slug)
	assert.Equal(t, "a", tree.Children[0].Name)
	assert.Same(t, a, tree.Children[0].Intent)
}

func TestBuildIntentTree_NestedDirs(t *testing.T) {
	intents := []*model.Intent{
		mkIntent("auth/jwt", ""),
		mkIntent("auth/password", ""),
		mkIntent("storage/format", ""),
	}
	tree := BuildIntentTree(intents)

	// Root has 2 children: auth/, storage/
	require.Len(t, tree.Children, 2)
	assert.Equal(t, "auth", tree.Children[0].Slug)
	assert.Equal(t, "storage", tree.Children[1].Slug)
	assert.True(t, tree.Children[0].IsDir)
	assert.True(t, tree.Children[1].IsDir)

	// auth/ has 2 leaves
	require.Len(t, tree.Children[0].Children, 2)
	assert.Equal(t, "jwt", tree.Children[0].Children[0].Name)
	assert.Equal(t, "password", tree.Children[0].Children[1].Name)

	// storage/ has 1 leaf
	require.Len(t, tree.Children[1].Children, 1)
	assert.Equal(t, "format", tree.Children[1].Children[0].Name)
}

func TestBuildIntentTree_DeepNesting(t *testing.T) {
	// 3 levels: payments/tencent/adapter
	intents := []*model.Intent{
		mkIntent("payments/tencent/adapter", ""),
	}
	tree := BuildIntentTree(intents)

	// root → payments/ → tencent/ → adapter
	require.Len(t, tree.Children, 1)
	payments := tree.Children[0]
	assert.Equal(t, "payments", payments.Slug)
	assert.True(t, payments.IsDir)
	require.Len(t, payments.Children, 1)
	tencent := payments.Children[0]
	assert.Equal(t, "payments/tencent", tencent.Slug)
	assert.True(t, tencent.IsDir)
	require.Len(t, tencent.Children, 1)
	adapter := tencent.Children[0]
	assert.Equal(t, "payments/tencent/adapter", adapter.Slug)
	assert.False(t, adapter.IsDir)
}

func TestBuildIntentTree_DirsBeforeLeaves(t *testing.T) {
	// Two cases that would expose a "dirs/leaves interleaved" bug:
	// (a) at the SAME Name level: a directory "auth" (created by
	//     "auth/jwt") and a leaf "zeta" — the directory must come first.
	// (b) Mixed: a leaf "alpha" + a directory "beta" (created by
	//     "beta/child") + a leaf "gamma" — directory "beta" first,
	//     then "alpha", then "gamma" lexicographically.
	t.Run("siblings mixed dir+leaf at root", func(t *testing.T) {
		intents := []*model.Intent{
			mkIntent("alpha", ""),
			mkIntent("beta/child", ""),
			mkIntent("gamma", ""),
		}
		tree := BuildIntentTree(intents)
		require.Len(t, tree.Children, 3)
		// Expect: beta (dir), alpha (leaf), gamma (leaf) — dir first
		// regardless of alphabetic position.
		assert.Equal(t, "beta", tree.Children[0].Slug)
		assert.True(t, tree.Children[0].IsDir)
		assert.Equal(t, "alpha", tree.Children[1].Slug)
		assert.False(t, tree.Children[1].IsDir)
		assert.Equal(t, "gamma", tree.Children[2].Slug)
		assert.False(t, tree.Children[2].IsDir)
	})

	t.Run("same-name dir and leaf coexist", func(t *testing.T) {
		// A top-level leaf "auth" and a directory "auth" (created by
		// "auth/jwt") share the Name "auth". BuildIntentTree's design:
		// the node "auth" is BOTH a directory (has child "jwt") AND
		// has its own Intent pointer (the leaf at "auth"). Both pieces
		// of data are preserved.
		leafAuth := mkIntent("auth", "# top-level")
		leafJwt := mkIntent("auth/jwt", "")
		intents := []*model.Intent{leafAuth, leafJwt}
		tree := BuildIntentTree(intents)
		require.Len(t, tree.Children, 1)
		auth := tree.Children[0]
		assert.Equal(t, "auth", auth.Slug)
		assert.True(t, auth.IsDir, "directory created from 'auth/jwt' prefix")
		assert.Same(t, leafAuth, auth.Intent, "the leaf 'auth' Intent is preserved")
		require.Len(t, auth.Children, 1)
		assert.Equal(t, "jwt", auth.Children[0].Name)
		assert.Same(t, leafJwt, auth.Children[0].Intent)
	})
}

func TestBuildIntentTree_Flatten(t *testing.T) {
	intents := []*model.Intent{
		mkIntent("a", ""),
		mkIntent("auth/jwt", ""),
		mkIntent("auth/password", ""),
		mkIntent("b", ""),
	}
	tree := BuildIntentTree(intents)
	leaves := tree.Flatten()
	// Pre-order, directories first within Sort():
	//   root → [auth/(dir), a, b]   (auth is dir, comes before leaves a, b)
	//   auth/(dir) has no Intent, so skipped by Flatten
	//   auth/ → [jwt, password]   (no Intent on dir, leaves emit)
	//   root → a (leaf)
	//   root → b (leaf)
	// Expected: jwt, password, a, b
	require.Len(t, leaves, 4)
	assert.Equal(t, "auth/jwt", leaves[0].Slug)
	assert.Equal(t, "auth/password", leaves[1].Slug)
	assert.Equal(t, "a", leaves[2].Slug)
	assert.Equal(t, "b", leaves[3].Slug)
}

func TestBuildIntentTree_Flatten_DirWithIntent(t *testing.T) {
	// When a node is BOTH a dir AND has an Intent, Flatten emits it
	// before its children (pre-order).
	intents := []*model.Intent{
		mkIntent("auth", "# module overview"),
		mkIntent("auth/jwt", ""),
	}
	tree := BuildIntentTree(intents)
	leaves := tree.Flatten()
	require.Len(t, leaves, 2)
	assert.Equal(t, "auth", leaves[0].Slug, "dir+Intent node emits before children")
	assert.Equal(t, "auth/jwt", leaves[1].Slug)
}

func TestBuildIntentTree_NilEntriesSkipped(t *testing.T) {
	intents := []*model.Intent{
		mkIntent("a", ""),
		nil, // defensively skip
		mkIntent("b", ""),
	}
	tree := BuildIntentTree(intents)
	leaves := tree.Flatten()
	assert.Len(t, leaves, 2)
}

func TestIntentTitle_FromH1(t *testing.T) {
	in := mkIntent("auth/jwt", "# JWT Sliding Window\n\nbody text")
	assert.Equal(t, "JWT Sliding Window", IntentTitle(in))
}

func TestIntentTitle_StripsLeadingHash(t *testing.T) {
	in := mkIntent("a", "  #   Spaced Title  \nrest")
	assert.Equal(t, "Spaced Title", IntentTitle(in))
}

func TestIntentTitle_FallbackToSlug(t *testing.T) {
	in := mkIntent("auth/jwt-sliding-window", "no heading here")
	assert.Equal(t, "Jwt sliding window", IntentTitle(in))
}

func TestIntentTitle_Nil(t *testing.T) {
	assert.Equal(t, "", IntentTitle(nil))
}

func TestIntentTitle_EmptyBody(t *testing.T) {
	in := mkIntent("single", "")
	assert.Equal(t, "Single", IntentTitle(in))
}
