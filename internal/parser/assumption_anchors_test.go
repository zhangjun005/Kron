package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHasInlineAssumptionText(t *testing.T) {
	cases := []struct {
		name      string
		raw       string
		wantCount int
		wantHas   bool
	}{
		{"empty", "", 0, false},
		{"no-text-key", "assumptions:\n  - id: a\n    severity: hard\n    rationale: \"a reasonably long rationale\"\n", 0, false},
		{"one-inline-text", "assumptions:\n  - id: a\n    text: hello world\n    severity: hard\n    rationale: \"a reasonably long rationale\"\n", 1, true},
		{"two-inline-texts", "assumptions:\n  - id: a\n    text: foo\n    severity: hard\n    rationale: \"a reasonably long rationale\"\n  - id: b\n    text: bar\n    severity: soft\n    rationale: \"b reasonably long rationale\"\n", 2, true},
		{"top-level-text-not-in-assumptions", "text: hello\nassumptions:\n  - id: a\n    severity: hard\n", 0, false},
		{"text-in-other-block", "references:\n  - text: oops\nassumptions:\n  - id: a\n    severity: hard\n", 0, false},
		{"comment-line-with-text", "assumptions:\n  # text: commented out\n  - id: a\n    severity: hard\n", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			count, has := HasInlineAssumptionText([]byte(tc.raw))
			assert.Equal(t, tc.wantCount, count)
			assert.Equal(t, tc.wantHas, has)
		})
	}
}

func TestSlugsForAssumption_EmptyInputs(t *testing.T) {
	_, err := SlugsForAssumption("", "id")
	assert.Error(t, err)

	_, err = SlugsForAssumption(".", "")
	assert.Error(t, err)
}

func TestSlugsForAssumption_NoMatches(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".kron/intents"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, ".kron/intents/auth.md"),
		[]byte("<!-- kron:frontmatter -->\ncreated_by: @a\nupdated_at: 2026-10-01T10:00:00Z\n<!-- /kron:frontmatter -->\n\n# Auth\n\nUses some assumption.\n"),
		0o644,
	))
	slugs, err := SlugsForAssumption(dir, "single-region")
	require.NoError(t, err)
	assert.Empty(t, slugs)
}

func TestSlugsForAssumption_BodyMarker(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".kron/intents"), 0o755))
	// Intent with a @kron:assumes marker in body
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, ".kron/intents/auth.md"),
		[]byte("<!-- kron:frontmatter -->\ncreated_by: @a\nupdated_at: 2026-10-01T10:00:00Z\n<!-- /kron:frontmatter -->\n\n# Auth\n\nThis intent depends on <!-- @kron:assumes single-region -->.\n"),
		0o644,
	))
	// Intent without the marker
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, ".kron/intents/payments.md"),
		[]byte("<!-- kron:frontmatter -->\ncreated_by: @a\nupdated_at: 2026-10-01T10:00:00Z\n<!-- /kron:frontmatter -->\n\n# Payments\n"),
		0o644,
	))

	slugs, err := SlugsForAssumption(dir, "single-region")
	require.NoError(t, err)
	assert.Equal(t, []string{"auth"}, slugs)
}

func TestSlugsForAssumption_MultipleIntentsMatch(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".kron/intents"), 0o755))
	for _, slug := range []string{"auth", "payments", "ui"} {
		require.NoError(t, os.WriteFile(
			filepath.Join(dir, ".kron/intents/"+slug+".md"),
			[]byte("<!-- kron:frontmatter -->\ncreated_by: @a\nupdated_at: 2026-10-01T10:00:00Z\n<!-- /kron:frontmatter -->\n\n# "+slug+"\n\n<!-- @kron:assumes single-region -->\n"),
			0o644,
		))
	}
	slugs, err := SlugsForAssumption(dir, "single-region")
	require.NoError(t, err)
	assert.Equal(t, []string{"auth", "payments", "ui"}, slugs)
}

func TestSlugsForAssumption_IgnoresWrongID(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".kron/intents"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, ".kron/intents/auth.md"),
		[]byte("<!-- kron:frontmatter -->\ncreated_by: @a\nupdated_at: 2026-10-01T10:00:00Z\n<!-- /kron:frontmatter -->\n\n# Auth\n\n<!-- @kron:assumes other-assumption -->\n"),
		0o644,
	))
	slugs, err := SlugsForAssumption(dir, "single-region")
	require.NoError(t, err)
	assert.Empty(t, slugs)
}

func TestSlugsForAssumption_IgnoresNonIntentFiles(t *testing.T) {
	// A doc file (outside .kron/intents/) that mentions the
	// marker should NOT be reported as a referencing intent.
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "docs"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".kron/intents"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "docs/notes.md"),
		[]byte("# Notes\n\n<!-- @kron:assumes single-region -->\n"),
		0o644,
	))
	slugs, err := SlugsForAssumption(dir, "single-region")
	require.NoError(t, err)
	assert.Empty(t, slugs)
}

func TestScanAssumptionAnchors(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".kron/intents"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, ".kron/intents/auth.md"),
		[]byte("<!-- kron:frontmatter -->\ncreated_by: @a\nupdated_at: 2026-10-01T10:00:00Z\n<!-- /kron:frontmatter -->\n\n# Auth\n\n<!-- @kron:assumes single-region -->\n<!-- @kron:assumes csrf-protected -->\n"),
		0o644,
	))
	anchors, err := ScanAssumptionAnchors(dir)
	require.NoError(t, err)
	require.Len(t, anchors, 2)
	// Sorted by (FilePath, LineNumber); both lines from the same file.
	slugs := []string{anchors[0].Slug, anchors[1].Slug}
	assert.Contains(t, slugs, "single-region")
	assert.Contains(t, slugs, "csrf-protected")
}

func TestIsValidAssumptionID(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"single-region", true},
		{"csrf-protected", true},
		{"x", true},
		{"auth/jwt", false}, // slashes not allowed
		{"foo.md", false},   // .md suffix not allowed
		{"Foo", false},      // uppercase not allowed
		{"foo_bar", false},  // underscore not allowed (kebab-case only)
		{"foo bar", false},  // space not allowed
		{"foo..bar", false}, // ".." not allowed (path traversal sentinel)
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			assert.Equal(t, tc.want, isValidAssumptionID(tc.in))
		})
	}
}
