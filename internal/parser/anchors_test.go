package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xxx/kron/internal/model"
)

func writeSourceFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
}

func TestParseAnchorLine_CommentPrefixes(t *testing.T) {
	cases := []struct {
		name string
		line string
		want string
		ok   bool
	}{
		// Common comment prefixes.
		{"go-double-slash", "// @kron:intent auth/jwt", "auth/jwt", true},
		{"python-hash", "# @kron:intent single-region", "single-region", true},
		{"sql-dashdash", "-- @kron:intent auth/jwt", "auth/jwt", true},
		{"erlang-percent", "% @kron:intent erl-cache", "erl-cache", true},

		// Tabs / extra spaces.
		{"extra-spaces", "  //   @kron:intent   auth/jwt   trailing", "auth/jwt", true},
		{"tab-spaces", "\t#\t@kron:intent\tauth/jwt", "auth/jwt", true},

		// Trailing comment on same line.
		{"trailing", "// @kron:intent auth/jwt - this is the JWT module", "auth/jwt", true},

		// Whole-token separation means "@kron:intent" preceded by
		// a non-identifier byte (whitespace, punctuation, etc.) and
		// followed by whitespace WILL match, even if the surrounding
		// context is a string literal. v1 accepts this as a known
		// false-positive: the comment-position heuristic is a future
		// refinement. Slug validation (next test cases) is the
		// v1 safeguard against obvious typos.
		{"marker-in-string", "const url = \"see @kron:intent auth/jwt for details\"", "auth/jwt", true},
		// Adjacent identifier characters block the match.
		{"marker-in-identifier", "// see@kron:intent auth/jwt (no whitespace)", "", false},

		// Slug validation enforced.
		{"invalid-slug-uppercase", "// @kron:intent Auth/Jwt", "", false},
		{"invalid-slug-trailing-dot", "// @kron:intent auth/.", "", false},

		// No marker at all.
		{"unrelated-line", "// regular comment without anchor", "", false},
		{"blank-line", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseAnchorLine(tc.line)
			assert.Equal(t, tc.ok, ok)
			if tc.ok {
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestScanAnchors_Empty(t *testing.T) {
	dir := t.TempDir()
	got, err := ScanAnchors(dir)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestScanAnchors_MultiFile(t *testing.T) {
	dir := t.TempDir()
	writeSourceFile(t, dir, "internal/auth/refresh.go", `package auth

// @kron:intent auth/refresh-token
func NewRefreshToken() string { return "" }
`)
	writeSourceFile(t, dir, "internal/rate/limit.py", `# @kron:intent rate-limit/token-bucket
def allow(): return True
`)
	// File with no anchors: must not appear in result.
	writeSourceFile(t, dir, "internal/unrelated.go", `package x
func Y() {}
`)

	anchors, err := ScanAnchors(dir)
	require.NoError(t, err)
	require.Len(t, anchors, 2)

	// Deterministic sort: (FilePath, LineNumber). The Go file path
	// (refresh.go) sorts before the Python file (limit.py) on case
	// ('r' < 'l' is false on case-insensitive sort, but on
	// byte-wise sort, lowercase 'l' (0x6C) < lowercase 'r' (0x72),
	// so limit.py comes first; we just check both anchors exist).
	assert.Equal(t, "auth/refresh-token", anchors[0].Slug)
	assert.Equal(t, 3, anchors[0].LineNumber)
	assert.Equal(t, "rate-limit/token-bucket", anchors[1].Slug)
	assert.Equal(t, 1, anchors[1].LineNumber)
}

func TestScanAnchors_SkipsKronDir(t *testing.T) {
	dir := t.TempDir()
	// The .kron/intents/*.md files may contain @kron:intent text in
	// body examples; ScanAnchors MUST NOT pick them up.
	writeSourceFile(t, dir, ".kron/intents/auth-jwt.md", `<!-- kron:frontmatter -->
created_by: "@a"
updated_at: 2026-10-01T10:00:00Z
<!-- /kron:frontmatter -->

# Sample

See // @kron:intent auth/jwt for the real anchor.
`)
	// And a real source file with one anchor.
	writeSourceFile(t, dir, "real.go", "// @kron:intent auth/jwt\npackage x\n")

	anchors, err := ScanAnchors(dir)
	require.NoError(t, err)
	require.Len(t, anchors, 1)
	assert.Equal(t, "auth/jwt", anchors[0].Slug)
	assert.Contains(t, anchors[0].FilePath, "real.go")
}

func TestScanAnchors_SkipsBinaryFiles(t *testing.T) {
	dir := t.TempDir()
	// A binary blob with a marker substring must NOT be scanned.
	// Sniff heuristic: a NUL byte anywhere in the first 512 bytes.
	writeSourceFile(t, dir, "blob.bin", string([]byte{
		'P', 'K', 0x00, 0x03, 0x04, // ZIP-like header with a NUL byte
		'@', 'k', 'r', 'o', 'n', ':', 'i', 'n', 't', 'e', 'n', 't', ' ',
		'a', 'u', 't', 'h', '/', 'j', 'w', 't', '\n',
	}))

	anchors, err := ScanAnchors(dir)
	require.NoError(t, err)
	assert.Empty(t, anchors)
}

func TestScanAnchors_MultipleAnchorsPerFile(t *testing.T) {
	dir := t.TempDir()
	content := `package x

// @kron:intent auth/refresh
func A() {}

// @kron:intent auth/rotate
func B() {}

// unrelated comment
func C() {}

// @kron:intent auth/single-region
func D() {}
`
	writeSourceFile(t, dir, "auth.go", content)

	anchors, err := ScanAnchors(dir)
	require.NoError(t, err)
	require.Len(t, anchors, 3)
	assert.Equal(t, []string{"auth/refresh", "auth/rotate", "auth/single-region"},
		[]string{anchors[0].Slug, anchors[1].Slug, anchors[2].Slug})
	assert.Equal(t, []int{3, 6, 12}, []int{anchors[0].LineNumber, anchors[1].LineNumber, anchors[2].LineNumber})
}

func TestScanAnchors_TrailingComment(t *testing.T) {
	dir := t.TempDir()
	writeSourceFile(t, dir, "x.go", "// @kron:intent auth/jwt - the JWT module\n")

	anchors, err := ScanAnchors(dir)
	require.NoError(t, err)
	require.Len(t, anchors, 1)
	assert.Equal(t, "auth/jwt", anchors[0].Slug)
}

// Sanity: the Anchor struct is unchanged; this test guards against
// accidental field renames (lint relies on these names).
func TestAnchorStructFields(t *testing.T) {
	a := model.Anchor{Slug: "x", FilePath: "/p", LineNumber: 7}
	assert.Equal(t, "x", a.Slug)
	assert.Equal(t, "/p", a.FilePath)
	assert.Equal(t, 7, a.LineNumber)
}
