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

// sampleIntent uses the v1 sentinel-wrapped frontmatter format.
// A separate fixture (`sampleLegacyIntent`) exercises the legacy "---"
// fence format so backward compatibility is covered by TestReader_Load_LegacyFence.
const sampleIntent = `<!-- kron:frontmatter -->
created_by: "@alice"
updated_at: 2026-10-01T10:00:00Z
status: active
<!-- /kron:frontmatter -->

# Sample

Body line 1.

Body line 2.
`

// sampleLegacyIntent is a fixture using the legacy "---" fence format.
// It must continue to parse without migration: see TestReader_Load_LegacyFence.
const sampleLegacyIntent = `---
created_by: "@alice"
updated_at: 2026-10-01T10:00:00Z
status: active
---

# Sample

Body line.
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
	// Legacy fence: opening fence present, but no closing fence.
	writeFile(t, dir, ".kron/intents/open.md", "---\ncreated_by: \"@bob\"\nupdated_at: 2026-10-01T10:00:00Z\nstatus: active\n")

	r, err := NewReader(dir)
	require.NoError(t, err)

	_, err = r.Load(context.Background(), "open")
	require.Error(t, err)
	assert.ErrorIs(t, err, model.ErrFrontmatterInvalid)
}

func TestReader_Load_MissingClosingSentinel(t *testing.T) {
	dir := t.TempDir()
	// Sentinel opening tag, but no closing tag — must be rejected.
	writeFile(t, dir, ".kron/intents/open.md", "<!-- kron:frontmatter -->\ncreated_by: \"@bob\"\nupdated_at: 2026-10-01T10:00:00Z\nstatus: active\n")

	r, err := NewReader(dir)
	require.NoError(t, err)

	_, err = r.Load(context.Background(), "open")
	require.Error(t, err)
	assert.ErrorIs(t, err, model.ErrFrontmatterInvalid)
}

func TestReader_Load_LegacyFence(t *testing.T) {
	// Backward compatibility: a file written with the old "---" fence must
	// continue to parse identically to a file written with the new sentinel
	// format. This is the migration window guarantee.
	dir := t.TempDir()
	writeFile(t, dir, ".kron/intents/legacy-jwt.md", sampleLegacyIntent)

	r, err := NewReader(dir)
	require.NoError(t, err)

	intent, err := r.Load(context.Background(), "legacy-jwt")
	require.NoError(t, err)
	assert.Equal(t, "legacy-jwt", intent.Slug)
	assert.Equal(t, "@alice", intent.Frontmatter.CreatedBy)
	assert.Equal(t, model.StatusActive, intent.Frontmatter.Status)
	assert.Equal(t, "2026-10-01T10:00:00Z", intent.Frontmatter.UpdatedAt.Format(time.RFC3339))
	assert.Contains(t, intent.Body, "Body line.")
}

func TestReader_Load_Sentinel_BodyWithHorizontalRule(t *testing.T) {
	// Body contains "---" horizontal rule + setext underline. With sentinel
	// wrapping it must NOT be confused for a closing fence.
	content := "<!-- kron:frontmatter -->\ncreated_by: \"@alice\"\nupdated_at: 2026-10-01T10:00:00Z\nstatus: active\n<!-- /kron:frontmatter -->\n# Sample\n\n---\n\n## After HR\n"
	dir := t.TempDir()
	writeFile(t, dir, ".kron/intents/hr-jwt.md", content)

	r, err := NewReader(dir)
	require.NoError(t, err)

	intent, err := r.Load(context.Background(), "hr-jwt")
	require.NoError(t, err)
	assert.Contains(t, intent.Body, "---")
	assert.Contains(t, intent.Body, "## After HR")
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

func TestReader_LoadAll_RecursiveSubdirs(t *testing.T) {
	// Added 2026-10-03 alongside the RFC 2026-10-03-frontmatter-references
	// landing. LoadAll now walks subdirectories because intent slugs
	// may contain "/" (e.g. "auth/jwt-sliding-window" → auth/jwt-sliding-window.md
	// under .kron/intents/). A subdirectory whose name is not a valid
	// slug prefix is pruned (its files are still walked; see below).
	dir := t.TempDir()
	writeFile(t, dir, ".kron/intents/auth/jwt.md", sampleIntent)
	writeFile(t, dir, ".kron/intents/auth/refresh.md", sampleIntent)
	writeFile(t, dir, ".kron/intents/payments/tencent-adapter.md", sampleIntent)
	// Top-level file with no slash, still works.
	writeFile(t, dir, ".kron/intents/single-region-deployment.md", sampleIntent)

	r, err := NewReader(dir)
	require.NoError(t, err)

	intents, err := r.LoadAll(context.Background())
	require.NoError(t, err)
	require.Len(t, intents, 4)

	got := make([]string, 0, len(intents))
	for _, in := range intents {
		got = append(got, in.Slug)
	}
	sort.Strings(got)
	assert.Equal(t, []string{
		"auth/jwt",
		"auth/refresh",
		"payments/tencent-adapter",
		"single-region-deployment",
	}, got)
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
