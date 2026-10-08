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

// =====================================================================
// Update tests
//
// These mirror assumption.Writer_Update_* in shape. Eight cases:
//   1. basic single-field patch
//   2. multi-field patch
//   3. Body patch (frontmatter unchanged except auto-bump)
//   4. nil-vs-non-nil semantics (zero value via &empty{})
//   5. CreatedBy opt-in (default refuses, WithAllowCreatorChange allows)
//   6. EmptyPatch rejected
//   7. Actor required
//   8. Invalid slug rejected
// =====================================================================

// seedIntentForUpdate writes an intent and returns the Writer + Reader
// so each Update subtest starts from a known shape. The intent's
// UpdatedAt is set 1 hour in the past so tests can detect the
// auto-bump.
func seedIntentForUpdate(t *testing.T) (*Writer, *Reader) {
	t.Helper()
	dir := t.TempDir()
	w, err := NewWriter(dir)
	require.NoError(t, err)

	intent := &model.Intent{
		Slug: "auth-jwt",
		Frontmatter: model.Frontmatter{
			Symbol:     []string{"auth.Token"},
			CreatedBy:  "@alice",
			UpdatedAt:  time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC),
			Status:     model.StatusActive,
			Reviewers:  []string{"@bob"},
			References: []string{"auth/refresh"},
		},
		Body: "original body",
	}
	require.NoError(t, w.Write(context.Background(), intent))

	r, err := NewReader(dir)
	require.NoError(t, err)
	return w, r
}

func TestWriter_Update_SingleField(t *testing.T) {
	w, r := seedIntentForUpdate(t)
	before := time.Now().UTC()

	newStatus := model.StatusSuperseded
	patch := UpdatePatch{
		Status: &newStatus,
		Actor:  "@alice",
	}
	require.NoError(t, w.Update(context.Background(), "auth-jwt", patch))

	got, err := r.Load(context.Background(), "auth-jwt")
	require.NoError(t, err)
	assert.Equal(t, model.StatusSuperseded, got.Frontmatter.Status)
	// Other fields untouched.
	assert.Equal(t, []string{"auth.Token"}, got.Frontmatter.Symbol)
	assert.Equal(t, "@alice", got.Frontmatter.CreatedBy)
	assert.Equal(t, []string{"@bob"}, got.Frontmatter.Reviewers)
	// Auto-bump.
	assert.True(t, got.Frontmatter.UpdatedAt.After(before.Add(-time.Second)),
		"UpdatedAt should be auto-bumped to recent time, got %v", got.Frontmatter.UpdatedAt)
}

func TestWriter_Update_MultipleFields(t *testing.T) {
	w, r := seedIntentForUpdate(t)

	newSymbol := []string{"auth.AccessToken", "auth.RefreshToken"}
	newReviewers := []string{"@carol"}
	newRefs := []string{"auth/refresh", "auth/login"}
	patch := UpdatePatch{
		Symbol:     &newSymbol,
		Reviewers:  &newReviewers,
		References: &newRefs,
		Actor:      "@alice",
	}
	require.NoError(t, w.Update(context.Background(), "auth-jwt", patch))

	got, err := r.Load(context.Background(), "auth-jwt")
	require.NoError(t, err)
	assert.Equal(t, newSymbol, got.Frontmatter.Symbol)
	assert.Equal(t, newReviewers, got.Frontmatter.Reviewers)
	assert.Equal(t, newRefs, got.Frontmatter.References)
	// Untouched.
	assert.Equal(t, model.StatusActive, got.Frontmatter.Status)
	assert.Equal(t, "@alice", got.Frontmatter.CreatedBy)
}

func TestWriter_Update_Body(t *testing.T) {
	w, r := seedIntentForUpdate(t)

	newBody := "completely new body content"
	patch := UpdatePatch{
		Body:  &newBody,
		Actor: "@alice",
	}
	require.NoError(t, w.Update(context.Background(), "auth-jwt", patch))

	got, err := r.Load(context.Background(), "auth-jwt")
	require.NoError(t, err)
	assert.Equal(t, "completely new body content\n", got.Body,
		"SerializeMarkdown always appends a trailing newline")
	// Frontmatter unchanged (other than auto-bump).
	assert.Equal(t, []string{"auth.Token"}, got.Frontmatter.Symbol)
}

func TestWriter_Update_NilVsEmpty(t *testing.T) {
	// nil pointer = leave field alone. Non-nil pointer to empty value
	// = overwrite to empty. Distinguishing the two is the whole point
	// of the pointer-per-field design.
	w, r := seedIntentForUpdate(t)

	emptyReviewers := []string{}
	patch := UpdatePatch{
		Reviewers: &emptyReviewers, // explicit empty: clear reviewers
		Actor:     "@alice",
	}
	require.NoError(t, w.Update(context.Background(), "auth-jwt", patch))

	got, err := r.Load(context.Background(), "auth-jwt")
	require.NoError(t, err)
	assert.Empty(t, got.Frontmatter.Reviewers,
		"non-nil pointer to empty slice should clear the field")
	// Symbol still has its original value.
	assert.Equal(t, []string{"auth.Token"}, got.Frontmatter.Symbol)
}

func TestWriter_Update_CreatedByOptIn(t *testing.T) {
	w, r := seedIntentForUpdate(t)

	// Default: CreatedBy patch is refused.
	newCreatedBy := "@bob"
	patch := UpdatePatch{
		CreatedBy: &newCreatedBy,
		Actor:     "@alice",
	}
	err := w.Update(context.Background(), "auth-jwt", patch)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCreatorChangeNotAllowed)

	// Verify on-disk state is unchanged.
	got, err := r.Load(context.Background(), "auth-jwt")
	require.NoError(t, err)
	assert.Equal(t, "@alice", got.Frontmatter.CreatedBy)

	// With opt-in: allowed.
	require.NoError(t, w.Update(context.Background(), "auth-jwt", patch, WithAllowCreatorChange()))

	got, err = r.Load(context.Background(), "auth-jwt")
	require.NoError(t, err)
	assert.Equal(t, "@bob", got.Frontmatter.CreatedBy)
}

func TestWriter_Update_EmptyPatch(t *testing.T) {
	w, _ := seedIntentForUpdate(t)

	// Every business field nil → ErrEmptyPatch. Library refuses
	// to write a file with only UpdatedAt changed.
	patch := UpdatePatch{Actor: "@alice"}
	err := w.Update(context.Background(), "auth-jwt", patch)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrEmptyPatch)
}

func TestWriter_Update_ActorRequired(t *testing.T) {
	w, _ := seedIntentForUpdate(t)

	newStatus := model.StatusSuperseded
	patch := UpdatePatch{Status: &newStatus} // no Actor
	err := w.Update(context.Background(), "auth-jwt", patch)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "actor")
}

func TestWriter_Update_InvalidSlug(t *testing.T) {
	w, _ := seedIntentForUpdate(t)

	newStatus := model.StatusSuperseded
	patch := UpdatePatch{
		Status: &newStatus,
		Actor:  "@alice",
	}
	err := w.Update(context.Background(), "Auth/JWT", patch)
	require.Error(t, err)
	assert.True(t, errors.Is(err, model.ErrSlugInvalid),
		"expected ErrSlugInvalid, got %v", err)
}

func TestWriter_Update_NotFound(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	require.NoError(t, err)

	newStatus := model.StatusActive
	patch := UpdatePatch{
		Status: &newStatus,
		Actor:  "@alice",
	}
	err = w.Update(context.Background(), "missing", patch)
	require.Error(t, err)
	assert.ErrorIs(t, err, model.ErrIntentNotFound)
}
