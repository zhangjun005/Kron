package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xxx/kron/internal/model"
)

// =====================================================================
// resolveWritePath — kind-aware shape decisions
// (RFC 2026-10-08-writer-readme-symmetry §3.3)
// =====================================================================

// pathTail returns the last n path segments of abs, joined
// with the OS separator. Used for clean assertions of the form
// "...ends with .kron/intents/auth.md" without absolute path
// noise.
func pathTail(abs string, n int) string {
	segs := splitPath(abs)
	if n > len(segs) {
		n = len(segs)
	}
	return filepath.Join(segs[len(segs)-n:]...)
}

func splitPath(p string) []string {
	// filepath.SplitList isn't what we want; manual split.
	out := []string{}
	rest := p
	for {
		dir, base := filepath.Split(rest)
		if base == "" {
			if dir != "" {
				out = append([]string{filepath.Clean(dir)}, out...)
			}
			break
		}
		out = append([]string{base}, out...)
		rest = filepath.Clean(dir)
		if rest == "" || rest == "." || rest == string(filepath.Separator) {
			if rest != "" {
				out = append([]string{rest}, out...)
			}
			break
		}
	}
	return out
}

func TestResolveWritePath_LeafDefault(t *testing.T) {
	// Empty kind defaults to leaf shape.
	got, err := resolveWritePath(t.TempDir(), "", "auth/jwt")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(".kron", "intents", "auth", "jwt.md"), pathTail(got, 4))
}

func TestResolveWritePath_LeafExplicit(t *testing.T) {
	got, err := resolveWritePath(t.TempDir(), model.IntentKindLeaf, "auth")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(".kron", "intents", "auth.md"), pathTail(got, 3))
}

func TestResolveWritePath_NodeSingleSegment(t *testing.T) {
	got, err := resolveWritePath(t.TempDir(), model.IntentKindNode, "auth")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(".kron", "intents", "auth", "README.md"), pathTail(got, 4))
}

func TestResolveWritePath_NodeDeepNested(t *testing.T) {
	got, err := resolveWritePath(t.TempDir(), model.IntentKindNode, "auth/login")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(".kron", "intents", "auth", "login", "README.md"), pathTail(got, 5))
}

func TestResolveWritePath_TopLevelREADME_AlwaysLeaf(t *testing.T) {
	// Top-level "README" slug must NEVER become README/README.md.
	// Both kinds should resolve to ".kron/intents/README.md".
	for _, kind := range []model.IntentKind{"", model.IntentKindLeaf, model.IntentKindNode} {
		t.Run("kind="+string(kind), func(t *testing.T) {
			got, err := resolveWritePath(t.TempDir(), kind, "README")
			require.NoError(t, err)
			assert.Equal(t, filepath.Join(".kron", "intents", "README.md"), pathTail(got, 3))
		})
	}
}

func TestResolveWritePath_CollisionLeafVsNode(t *testing.T) {
	// Seed the disk: <slug>.md exists, caller asks for node form.
	root := t.TempDir()
	writeFile(t, root, ".kron/intents/auth.md", "# existing leaf")

	_, err := resolveWritePath(root, model.IntentKindNode, "auth")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrSlugCollision)
	assert.Contains(t, err.Error(), "auth")
}

func TestResolveWritePath_CollisionNodeVsLeaf(t *testing.T) {
	// Seed the disk: <slug>/README.md exists, caller asks for leaf.
	root := t.TempDir()
	writeFile(t, root, ".kron/intents/auth/README.md", "# existing node")

	_, err := resolveWritePath(root, model.IntentKindLeaf, "auth")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrSlugCollision)
}

func TestResolveWritePath_EmptySlug(t *testing.T) {
	_, err := resolveWritePath(t.TempDir(), model.IntentKindLeaf, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "slug is empty")
}

func TestResolveWritePath_InvalidKind(t *testing.T) {
	_, err := resolveWritePath(t.TempDir(), model.IntentKind("bogus"), "auth")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid intent kind")
}

// =====================================================================
// Writer.Write — kind routing end-to-end
// =====================================================================

func TestWriter_Write_NodeKind(t *testing.T) {
	// Write a node-form intent; verify the file lands at
	// <slug>/README.md (not <slug>.md).
	dir := t.TempDir()
	w, err := NewWriter(dir)
	require.NoError(t, err)

	intent := &model.Intent{
		Slug: "auth",
		Kind: model.IntentKindNode,
		Frontmatter: model.Frontmatter{
			CreatedBy: "@alice",
			UpdatedAt: mustParseTime("2026-10-08T10:00:00Z"),
		},
		Body: "# auth module\n",
	}
	require.NoError(t, w.Write(context.Background(), intent))

	// Canonical <slug>.md must NOT exist.
	_, err = os.Stat(filepath.Join(dir, ".kron", "intents", "auth.md"))
	assert.True(t, os.IsNotExist(err), "<slug>.md must not exist for node form")

	// Node form <slug>/README.md must exist.
	got, err := os.ReadFile(filepath.Join(dir, ".kron", "intents", "auth", "README.md"))
	require.NoError(t, err)
	assert.Contains(t, string(got), "auth module")
}

func TestWriter_Write_LeafKind_Default(t *testing.T) {
	// Default kind (empty) should write the canonical <slug>.md.
	dir := t.TempDir()
	w, err := NewWriter(dir)
	require.NoError(t, err)

	intent := &model.Intent{
		Slug: "auth/jwt",
		Frontmatter: model.Frontmatter{
			CreatedBy: "@alice",
			UpdatedAt: mustParseTime("2026-10-08T10:00:00Z"),
		},
		Body: "# jwt\n",
	}
	require.NoError(t, w.Write(context.Background(), intent))

	_, err = os.Stat(filepath.Join(dir, ".kron", "intents", "auth", "jwt.md"))
	assert.NoError(t, err)
}

func TestWriter_Write_CollisionOnShape(t *testing.T) {
	// Pre-seed with a node form, try to write a leaf to the same slug.
	dir := t.TempDir()
	writeFile(t, dir, ".kron/intents/auth/README.md", "pre-existing node")

	w, err := NewWriter(dir)
	require.NoError(t, err)

	intent := &model.Intent{
		Slug: "auth",
		Frontmatter: model.Frontmatter{
			CreatedBy: "@bob",
			UpdatedAt: mustParseTime("2026-10-08T10:00:00Z"),
		},
		Body: "# leaf\n",
	}
	err = w.Write(context.Background(), intent)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrSlugCollision)
}

func TestWriter_Exists_NodeAndLeaf(t *testing.T) {
	// Exists must report true for either form (read-side shorthand).
	dir := t.TempDir()
	writeFile(t, dir, ".kron/intents/auth/README.md", "node")
	writeFile(t, dir, ".kron/intents/billing.md", "leaf")
	writeFile(t, dir, ".kron/intents/auth/jwt.md", "leaf in subdir")

	w, err := NewWriter(dir)
	require.NoError(t, err)

	assert.True(t, w.Exists(context.Background(), "auth"), "node-form should be visible")
	assert.True(t, w.Exists(context.Background(), "billing"), "leaf-form should be visible")
	assert.True(t, w.Exists(context.Background(), "auth/jwt"), "leaf under node should be visible")
	assert.False(t, w.Exists(context.Background(), "nope"), "absent must be false")
	assert.False(t, w.Exists(context.Background(), ""), "empty slug must be false")
}

func TestWriter_MoveToTrash_NodeForm(t *testing.T) {
	// MoveToTrash must work regardless of which form the file
	// takes on disk.
	dir := t.TempDir()
	writeFile(t, dir, ".kron/intents/auth/README.md", "node intent")

	w, err := NewWriter(dir)
	require.NoError(t, err)

	require.NoError(t, w.MoveToTrash(context.Background(), "auth"))

	// Source gone (in either form).
	_, err = os.Stat(filepath.Join(dir, ".kron", "intents", "auth", "README.md"))
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(filepath.Join(dir, ".kron", "intents", "auth.md"))
	assert.True(t, os.IsNotExist(err))

	// Trashed file present at canonical .trash/<slug>.md.
	_, err = os.Stat(filepath.Join(dir, ".kron", ".trash", "auth.md"))
	assert.NoError(t, err)
}

func TestWriter_Update_PreservesNodeForm(t *testing.T) {
	// Update on a node-form file must write back to the node
	// form (not silently flip to leaf).
	dir := t.TempDir()
	writeFile(t, dir, ".kron/intents/auth/README.md", "---\ncreated_by: \"@alice\"\nupdated_at: \"2026-10-08T10:00:00Z\"\n---\n\n# old\n")

	w, err := NewWriter(dir)
	require.NoError(t, err)

	patch := UpdatePatch{
		Body:  stringPtr("# new body\n"),
		Actor: "@alice",
	}
	require.NoError(t, w.Update(context.Background(), "auth", patch))

	// Node form still there.
	got, err := os.ReadFile(filepath.Join(dir, ".kron", "intents", "auth", "README.md"))
	require.NoError(t, err)
	assert.Contains(t, string(got), "new body")

	// Leaf form did NOT appear.
	_, err = os.Stat(filepath.Join(dir, ".kron", "intents", "auth.md"))
	assert.True(t, os.IsNotExist(err), "Update must not flip node → leaf")
}

// =====================================================================
// KindFromPath — read-side inference of IntentKind from disk path
// (RFC 2026-10-08-writer-readme-symmetry §3.3 — frontmatter does
// NOT carry Kind, so the file path is the only authoritative
// source at read time; Reader.Load uses this to backfill
// Intent.Kind on the returned struct).
// =====================================================================

func TestKindFromPath_LeafAtTopLevel(t *testing.T) {
	got := KindFromPath(filepath.Join("/root", ".kron", "intents", "auth.md"))
	assert.Equal(t, model.IntentKindLeaf, got)
}

func TestKindFromPath_NodeAtTopLevelDir(t *testing.T) {
	got := KindFromPath(filepath.Join("/root", ".kron", "intents", "auth", "README.md"))
	assert.Equal(t, model.IntentKindNode, got)
}

func TestKindFromPath_NodeDeepNested(t *testing.T) {
	got := KindFromPath(filepath.Join("/root", ".kron", "intents", "a", "b", "c", "README.md"))
	assert.Equal(t, model.IntentKindNode, got)
}

func TestKindFromPath_TopLevelREADMEIsLeaf(t *testing.T) {
	// Top-level README.md (the .kron/intents/README.md file) is
	// leaf-shaped on disk (resolveWritePath forces this to avoid
	// README/README.md recursion), so KindFromPath must agree.
	got := KindFromPath(filepath.Join("/root", ".kron", "intents", "README.md"))
	assert.Equal(t, model.IntentKindLeaf, got)
}

func TestKindFromPath_EmptyPath(t *testing.T) {
	// Defensive default: empty path returns Leaf (the Valid() zero
	// value) rather than panicking.
	got := KindFromPath("")
	assert.Equal(t, model.IntentKindLeaf, got)
}

// =====================================================================
// helpers
// =====================================================================

func stringPtr(s string) *string { return &s }

func mustParseTime(s string) time.Time {
	tt, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return tt
}
