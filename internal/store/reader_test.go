package store

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xxx/kron/internal/model"
)

// writeFile is a tiny test helper that creates a file with given content
// under dir, creating intermediate directories. It uses 0o644 so on
// Windows (where the test runs) the file is readable by the test process.
func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
}

const sampleIntent = `---
created_by: "@alice"
updated_at: 2026-10-01T10:00:00Z
status: active
---

# Sample

Body line 1.

Body line 2.
`

func TestReader_Load_HappyPath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".kron/intents/auth-jwt.md", sampleIntent)

	r, err := NewReader(dir)
	require.NoError(t, err)

	intent, err := r.Load(context.Background(), "auth-jwt")
	require.NoError(t, err)

	assert.Equal(t, "auth-jwt", intent.Slug)
	assert.Equal(t, "@alice", intent.Frontmatter.CreatedBy)
	assert.Equal(t, model.StatusActive, intent.Frontmatter.Status)
	assert.Equal(t, "2026-10-01T10:00:00Z", intent.Frontmatter.UpdatedAt.Format(time.RFC3339))
	assert.Contains(t, intent.Body, "Body line 1.")
	assert.Equal(t, filepath.Join(dir, ".kron", "intents", "auth-jwt.md"), intent.SourcePath)
}

func TestReader_Load_NotFound(t *testing.T) {
	dir := t.TempDir()
	r, err := NewReader(dir)
	require.NoError(t, err)

	_, err = r.Load(context.Background(), "missing")
	require.Error(t, err)
	assert.ErrorIs(t, err, model.ErrIntentNotFound)
}

func TestReader_Load_InvalidFrontmatter(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".kron/intents/broken.md", "not a frontmatter block\njust body\n")

	r, err := NewReader(dir)
	require.NoError(t, err)

	_, err = r.Load(context.Background(), "broken")
	require.Error(t, err)
	assert.ErrorIs(t, err, model.ErrFrontmatterInvalid)
}

func TestReader_Load_MissingClosingFence(t *testing.T) {
	dir := t.TempDir()
	// Opening fence present, but no closing fence.
	writeFile(t, dir, ".kron/intents/open.md", "---\ncreated_by: \"@bob\"\nupdated_at: 2026-10-01T10:00:00Z\nstatus: active\n")

	r, err := NewReader(dir)
	require.NoError(t, err)

	_, err = r.Load(context.Background(), "open")
	require.Error(t, err)
	assert.ErrorIs(t, err, model.ErrFrontmatterInvalid)
}

func TestReader_Load_NestedSlug(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".kron/intents/auth/jwt.md", sampleIntent)

	r, err := NewReader(dir)
	require.NoError(t, err)

	intent, err := r.Load(context.Background(), "auth/jwt")
	require.NoError(t, err)
	assert.Equal(t, "auth/jwt", intent.Slug)
}

func TestReader_Load_EmptySlug(t *testing.T) {
	dir := t.TempDir()
	r, err := NewReader(dir)
	require.NoError(t, err)

	_, err = r.Load(context.Background(), "")
	require.Error(t, err)
	assert.ErrorIs(t, err, model.ErrSlugInvalid)
}

func TestReader_LoadAll_Empty(t *testing.T) {
	dir := t.TempDir()
	r, err := NewReader(dir)
	require.NoError(t, err)

	intents, err := r.LoadAll(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, intents, "LoadAll must return non-nil empty slice, not nil")
	assert.Len(t, intents, 0)
}

func TestReader_LoadAll_Lexicographic(t *testing.T) {
	dir := t.TempDir()
	// Write in non-sorted order to prove the result is sorted.
	writeFile(t, dir, ".kron/intents/zeta.md", sampleIntent)
	writeFile(t, dir, ".kron/intents/alpha.md", sampleIntent)
	writeFile(t, dir, ".kron/intents/middle.md", sampleIntent)
	// A non-md file (should be skipped):
	writeFile(t, dir, ".kron/intents/README.txt", "ignore me")
	// A subdirectory (should be skipped):
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".kron/intents/sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".kron/intents/sub/inner.md"), []byte(sampleIntent), 0o644))

	r, err := NewReader(dir)
	require.NoError(t, err)

	intents, err := r.LoadAll(context.Background())
	require.NoError(t, err)
	require.Len(t, intents, 3)

	got := make([]string, 0, len(intents))
	for _, in := range intents {
		got = append(got, in.Slug)
	}
	sort.Strings(got) // sanity
	assert.Equal(t, []string{"alpha", "middle", "zeta"}, got)
}

func TestReader_Exists(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".kron/intents/present.md", sampleIntent)

	r, err := NewReader(dir)
	require.NoError(t, err)

	assert.True(t, r.Exists(context.Background(), "present"))
	assert.False(t, r.Exists(context.Background(), "absent"))
}

func TestReader_PathHelpers(t *testing.T) {
	dir := t.TempDir()
	r, err := NewReader(dir)
	require.NoError(t, err)

	assert.Equal(t,
		filepath.Join(dir, ".kron", "intents", "auth", "jwt.md"),
		r.IntentPath("auth/jwt"),
	)
	assert.Equal(t,
		filepath.Join(dir, ".kron", ".trash", "auth", "jwt.md"),
		r.TrashPath("auth/jwt"),
	)
	assert.Equal(t, filepath.Join(dir, ".kron"), r.KronDir())
}

func TestNewReader_Errors(t *testing.T) {
	t.Run("empty root", func(t *testing.T) {
		_, err := NewReader("")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "root path is empty")
	})

	t.Run("nonexistent root", func(t *testing.T) {
		_, err := NewReader(filepath.Join(t.TempDir(), "does-not-exist"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "root not accessible")
	})
}
