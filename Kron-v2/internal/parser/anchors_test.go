package parser

import (
	"os"
	"path/filepath"
	"strings"
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
	// Kind is a v1.2+ field. Its zero value is the empty string
	// (AnchorKind is a string alias); IsCode treats "" as code
	// (the historical default) so hand-constructed literals that
	// omit Kind behave the same as parser-produced code anchors.
	// See RFC 2026-10-08-md-anchors.md §2.5.
	assert.Equal(t, model.AnchorKind(""), a.Kind, "zero value of AnchorKind must be the empty string")
	assert.True(t, a.Kind.IsCode(), "zero-value Kind must behave as code (IsCode)")
	assert.False(t, a.Kind.IsMarkdown(), "zero-value Kind must NOT be markdown")
	assert.False(t, a.Kind.IsSet(), "zero-value Kind must not be IsSet (parser didn't run)")
}

// TestScanAnchors_KindCode locks the v1.2 contract (RFC
// 2026-10-08-md-anchors.md §2.5): every anchor returned by
// ScanAnchors must report Kind=code. We use IsCode() rather than
// ==-compare so a future addition of the explicit AnchorKindCode
// constant is not a breaking change. This is the mirror image of
// TestScanMarkdownAnchors_KindMarkdown.
func TestScanAnchors_KindCode(t *testing.T) {
	dir := t.TempDir()
	writeSourceFile(t, dir, "x.go", "// @kron:intent auth/jwt\n// @kron:intent auth/refresh\n")
	anchors, err := ScanAnchors(dir)
	require.NoError(t, err)
	require.Len(t, anchors, 2)
	for i, a := range anchors {
		assert.True(t, a.Kind.IsCode(), "anchors[%d].Kind = %q must be code", i, a.Kind)
		assert.False(t, a.Kind.IsMarkdown(), "anchors[%d].Kind = %q must NOT be markdown", i, a.Kind)
	}
}

// TestScanAnchors_FilePathIsRepoRelative locks down the
// "FilePath is repo-relative, forward-slash" contract from
// RFC 2026-10-08-path-conventions.md §2.1. Callers rely on this
// format to (a) serialize anchors into JSON-RPC payloads without
// leaking absolute OS paths and (b) join FilePath against their
// own root to locate the file again. Both invariants are checked.
func TestScanAnchors_FilePathIsRepoRelative(t *testing.T) {
	dir := t.TempDir()
	writeSourceFile(t, dir, "internal/auth/refresh.go", "// @kron:intent auth/jwt\n")
	writeSourceFile(t, dir, "internal/auth/password.go", "// @kron:intent auth/password\n")
	// Top-level file.
	writeSourceFile(t, dir, "root.go", "// @kron:intent root-intent\n")

	anchors, err := ScanAnchors(dir)
	require.NoError(t, err)
	require.Len(t, anchors, 3)

	for _, a := range anchors {
		// (a) No leading slash (relative).
		assert.False(t, strings.HasPrefix(a.FilePath, "/"),
			"FilePath must not be absolute: %q", a.FilePath)
		// (b) No backslash (forward-slash only — Windows safe).
		assert.NotContains(t, a.FilePath, "\\",
			"FilePath must use forward slashes: %q", a.FilePath)
	}

	// Specific values: anchors[0] is the lexicographically first by
	// (FilePath, LineNumber) — "password.go" sorts before
	// "refresh.go" (p < r).
	assert.Equal(t, "internal/auth/password.go", anchors[0].FilePath)
	assert.Equal(t, "internal/auth/refresh.go", anchors[1].FilePath)
	assert.Equal(t, "root.go", anchors[2].FilePath)
}

// TestScanAnchors_FilePath_StableWhenDirIsRelative: even when the
// caller passes a relative dir, the emitted FilePath is still
// repo-relative (not "./auth/foo.md"). filepath.Abs is taken
// internally so the Rel() baseline is the resolved absolute root.
func TestScanAnchors_FilePath_StableWhenDirIsRelative(t *testing.T) {
	root := t.TempDir()
	writeSourceFile(t, root, "a.go", "// @kron:intent single-region\n")

	// Chdir so passing "." is meaningful.
	oldCwd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(root))
	t.Cleanup(func() { _ = os.Chdir(oldCwd) })

	anchors, err := ScanAnchors(".")
	require.NoError(t, err)
	require.Len(t, anchors, 1)
	assert.Equal(t, "a.go", anchors[1-1].FilePath,
		"FilePath must be relative to the resolved walk root, not to the caller's CWD string")
}

func TestScanMarkdownAnchors_FilePathIsRepoRelative(t *testing.T) {
	dir := t.TempDir()
	writeSourceFile(t, dir, "docs/spec.md", "<!-- @kron:intent auth/jwt -->\n")
	writeSourceFile(t, dir, "docs/notes.md", "<!-- @kron:intent auth/password -->\n")

	anchors, err := ScanMarkdownAnchors(dir)
	require.NoError(t, err)
	require.Len(t, anchors, 2)

	for _, a := range anchors {
		assert.False(t, strings.HasPrefix(a.FilePath, "/"),
			"FilePath must not be absolute: %q", a.FilePath)
		assert.NotContains(t, a.FilePath, "\\",
			"FilePath must use forward slashes: %q", a.FilePath)
	}
	assert.Equal(t, "docs/notes.md", anchors[0].FilePath)
	assert.Equal(t, "docs/spec.md", anchors[1].FilePath)
}
