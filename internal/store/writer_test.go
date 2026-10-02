package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xxx/kron/internal/model"
)

func newTestIntent(slug, createdBy string) *model.Intent {
	return &model.Intent{
		Slug: slug,
		Frontmatter: model.Frontmatter{
			CreatedBy: createdBy,
			UpdatedAt: time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC),
			Status:    model.StatusActive,
		},
		Body: "# Title\n\nBody paragraph.\n",
	}
}

func TestWriter_Write_New(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	require.NoError(t, err)

	intent := newTestIntent("auth-jwt", "@alice")
	require.NoError(t, w.Write(context.Background(), intent))

	// File exists at the expected path.
	assert.True(t, w.Exists(context.Background(), "auth-jwt"))

	// Round-trip: read back via Reader and compare.
	r, err := NewReader(dir)
	require.NoError(t, err)
	got, err := r.Load(context.Background(), "auth-jwt")
	require.NoError(t, err)
	assert.Equal(t, intent.Slug, got.Slug)
	assert.Equal(t, intent.Frontmatter.CreatedBy, got.Frontmatter.CreatedBy)
	assert.Equal(t, intent.Body, got.Body)
}

func TestWriter_Write_Overwrite(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	require.NoError(t, err)

	// Write first version.
	v1 := newTestIntent("auth-jwt", "@alice")
	v1.Body = "first"
	require.NoError(t, w.Write(context.Background(), v1))

	// Overwrite with second version.
	v2 := newTestIntent("auth-jwt", "@bob")
	v2.Body = "second"
	require.NoError(t, w.Write(context.Background(), v2))

	r, err := NewReader(dir)
	require.NoError(t, err)
	got, err := r.Load(context.Background(), "auth-jwt")
	require.NoError(t, err)
	assert.Equal(t, "@bob", got.Frontmatter.CreatedBy)
	// SerializeMarkdown always appends a trailing newline so the file ends
	// in a POSIX-compliant record separator. The test data does not.
	assert.Equal(t, "second\n", got.Body)
}

func TestWriter_Write_InvalidSlug(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	require.NoError(t, err)

	cases := []string{
		"",              // empty
		"Auth/JWT",      // uppercase
		"../etc/passwd", // traversal
		"auth/.md",      // extension
		"auth/",         // trailing slash
		"/auth",         // leading slash
		"auth//jwt",     // empty segment
		"auth\\jwt",     // backslash
	}
	for _, slug := range cases {
		t.Run("slug="+slug, func(t *testing.T) {
			intent := newTestIntent(slug, "@alice")
			err := w.Write(context.Background(), intent)
			require.Error(t, err)
			assert.True(t, errors.Is(err, model.ErrSlugInvalid) || errors.Is(err, model.ErrSlugInvalid),
				"expected ErrSlugInvalid, got %v", err)
		})
	}
}

func TestWriter_Write_NestedSlugCreatesDirs(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	require.NoError(t, err)

	intent := newTestIntent("auth/jwt", "@alice")
	require.NoError(t, w.Write(context.Background(), intent))

	// .kron/intents/auth/ should be created automatically.
	info, err := os.Stat(filepath.Join(dir, ".kron", "intents", "auth"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestWriter_MoveToTrash(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	require.NoError(t, err)

	intent := newTestIntent("auth-jwt", "@alice")
	require.NoError(t, w.Write(context.Background(), intent))

	require.NoError(t, w.MoveToTrash(context.Background(), "auth-jwt"))

	// Source gone, target present.
	_, err = os.Stat(w.IntentPath("auth-jwt"))
	assert.True(t, errors.Is(err, os.ErrNotExist), "intent still at original path")
	assert.FileExists(t, w.TrashPath("auth-jwt"))
}

func TestWriter_MoveToTrash_NotFound(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	require.NoError(t, err)

	err = w.MoveToTrash(context.Background(), "missing")
	require.Error(t, err)
	assert.ErrorIs(t, err, model.ErrIntentNotFound)
}

func TestWriter_MoveToTrash_AlreadyInTrash(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	require.NoError(t, err)

	intent := newTestIntent("auth-jwt", "@alice")
	require.NoError(t, w.Write(context.Background(), intent))
	require.NoError(t, w.MoveToTrash(context.Background(), "auth-jwt"))

	// Second trash on an already-trashed intent reports not-found, because
	// the source path no longer exists. This is the correct behaviour: a
	// second trash attempt is semantically a "find then trash", and find
	// fails. A separate Restore path handles the "intent is in trash" case.
	err = w.MoveToTrash(context.Background(), "auth-jwt")
	require.Error(t, err)
	assert.ErrorIs(t, err, model.ErrIntentNotFound)
}

func TestWriter_RestoreFromTrash(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	require.NoError(t, err)

	intent := newTestIntent("auth-jwt", "@alice")
	require.NoError(t, w.Write(context.Background(), intent))
	require.NoError(t, w.MoveToTrash(context.Background(), "auth-jwt"))
	require.NoError(t, w.RestoreFromTrash(context.Background(), "auth-jwt"))

	// Back at original path, gone from trash.
	assert.FileExists(t, w.IntentPath("auth-jwt"))
	_, err = os.Stat(w.TrashPath("auth-jwt"))
	assert.True(t, errors.Is(err, os.ErrNotExist), "trash file still present")
}

func TestWriter_RestoreFromTrash_NotInTrash(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	require.NoError(t, err)

	err = w.RestoreFromTrash(context.Background(), "missing")
	require.Error(t, err)
	assert.ErrorIs(t, err, model.ErrIntentNotFound)
}

func TestWriter_RestoreFromTrash_AlreadyActive(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	require.NoError(t, err)

	intent := newTestIntent("auth-jwt", "@alice")
	require.NoError(t, w.Write(context.Background(), intent))
	// Manually create the trash file so Restore detects a clash.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".kron", ".trash"), 0o755))
	require.NoError(t, os.WriteFile(w.TrashPath("auth-jwt"), []byte("junk"), 0o644))

	err = w.RestoreFromTrash(context.Background(), "auth-jwt")
	require.Error(t, err)
	assert.ErrorIs(t, err, model.ErrIntentExists)
}

func TestWriter_Write_NilIntent(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	require.NoError(t, err)

	err = w.Write(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "intent is nil")
}
