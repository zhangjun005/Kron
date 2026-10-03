package lint

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeFile creates a file at <root>/<rel> with content, creating
// intermediate directories. Mirrors store/reader_test.go's helper but
// is duplicated (tests in internal/lint cannot import test helpers
// from internal/store). Keep the two helpers in sync if the layout
// ever changes.
func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
}

const sampleIntent = `<!-- kron:frontmatter -->
created_by: "@alice"
updated_at: 2026-10-01T10:00:00Z
status: active
<!-- /kron:frontmatter -->

# Sample
`

func TestRun_EmptyRepo(t *testing.T) {
	dir := t.TempDir()
	// No .kron/intents/ at all — anchor scan returns empty, LoadAll
	// returns empty, no diags, no error.
	diags, err := Run(context.Background(), dir)
	require.NoError(t, err)
	assert.Empty(t, diags)
}

func TestRun_AnchorDangling(t *testing.T) {
	dir := t.TempDir()
	// Source file references a slug that does not exist.
	writeFile(t, dir, "internal/auth/refresh.go", `package auth

// @kron:intent auth/refresh-token
func NewRefreshToken() string { return "" }
`)

	diags, err := Run(context.Background(), dir)
	require.NoError(t, err)
	require.Len(t, diags, 1)
	assert.Equal(t, RuleAnchorDangling, diags[0].Rule)
	assert.Equal(t, SeverityError, diags[0].Severity)
	// Where is repo-relative and includes the line number. Compare
	// path components separately to avoid platform-specific separators
	// (Windows filepath.Rel returns backslashes).
	assert.Contains(t, diags[0].Where, "refresh.go")
	assert.Contains(t, diags[0].Where, ":3")
	assert.Contains(t, diags[0].Detail, "auth/refresh-token")
}

func TestRun_AnchorValid(t *testing.T) {
	dir := t.TempDir()
	// The referenced intent exists — no diags expected.
	writeFile(t, dir, "internal/auth/auth.go", "// @kron:intent auth/jwt\npackage auth\n")
	writeFile(t, dir, ".kron/intents/auth/jwt.md", sampleIntent)

	diags, err := Run(context.Background(), dir)
	require.NoError(t, err)
	assert.Empty(t, diags)
}

func TestRun_FrontmatterLoadFailureRepromDiag(t *testing.T) {
	// v1 LoadAll is fail-fast: the first broken intent surfaces as a Go
	// error. Run() converts that error into a single repo-wide Diag so
	// the v1 contract "rule failures are observations, not exceptions"
	// holds uniformly across both rule classes.
	dir := t.TempDir()
	writeFile(t, dir, ".kron/intents/broken.md", "not a frontmatter block\njust body\n")

	diags, err := Run(context.Background(), dir)
	require.NoError(t, err)
	require.Len(t, diags, 1)
	assert.Equal(t, RuleFrontmatterInvalid, diags[0].Rule)
	assert.Equal(t, SeverityError, diags[0].Severity)
	assert.Equal(t, "<root>", diags[0].Where)
	assert.NotEmpty(t, diags[0].Detail)
}

func TestRun_FrontmatterLoadFailureWithValidAnchor(t *testing.T) {
	// A dangling anchor combined with a broken frontmatter file: both
	// diags must appear (anchor scan succeeds, LoadAll fails). Run does
	// NOT short-circuit on LoadAll failure.
	dir := t.TempDir()
	writeFile(t, dir, "x.go", "// @kron:intent missing/slug\npackage x\n")
	writeFile(t, dir, ".kron/intents/broken.md", "not a frontmatter block\n")

	diags, err := Run(context.Background(), dir)
	require.NoError(t, err)
	require.Len(t, diags, 2)

	// Order is deterministic: A-class runs first (anchor), B-class runs
	// second (frontmatter load).
	assert.Equal(t, RuleAnchorDangling, diags[0].Rule)
	assert.Equal(t, RuleFrontmatterInvalid, diags[1].Rule)
}

func TestRun_EmptyRoot(t *testing.T) {
	_, err := Run(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "root path is empty")
}

func TestRun_SkipsKronDir(t *testing.T) {
	// The .kron/intents/*.md files may legitimately include "@kron:intent"
	// inside body examples. Run MUST NOT pick them up via anchor scan.
	dir := t.TempDir()
	writeFile(t, dir, ".kron/intents/sample.md", sampleIntent+
		"\nSee // @kron:intent fake/in-sample for context.\n")
	writeFile(t, dir, "real.go", "// @kron:intent sample\npackage x\n")

	diags, err := Run(context.Background(), dir)
	require.NoError(t, err)
	assert.Empty(t, diags, "anchor in .kron/intents/*.md body must not count")
}

func TestHasErrors(t *testing.T) {
	cases := []struct {
		name  string
		diags []Diag
		want  bool
	}{
		{"empty", []Diag{}, false},
		{"only-warning", []Diag{{Severity: SeverityWarning}}, false},
		{"one-error", []Diag{{Severity: SeverityError}}, true},
		{"mixed", []Diag{{Severity: SeverityWarning}, {Severity: SeverityError}}, true},
		{"empty-severity-is-error", []Diag{{Severity: ""}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, HasErrors(tc.diags))
		})
	}
}

func TestDiagSeverityEmptyIsError(t *testing.T) {
	d := Diag{Severity: ""}
	assert.Equal(t, SeverityError, d.severity())
}

func TestDiagSeverityRoundTrip(t *testing.T) {
	assert.Equal(t, SeverityError, (Diag{Severity: SeverityError}).severity())
	assert.Equal(t, SeverityWarning, (Diag{Severity: SeverityWarning}).severity())
}

func TestRuleConstantsAreStable(t *testing.T) {
	// Guard against accidental rename of the public Rule constants.
	// lint consumers (CI, editor plugins) depend on these string values.
	assert.Equal(t, Rule("anchor-dangling"), RuleAnchorDangling)
	assert.Equal(t, Rule("frontmatter-invalid"), RuleFrontmatterInvalid)
}
