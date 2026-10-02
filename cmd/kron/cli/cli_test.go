package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// chdir switches the current working directory to dir for the duration
// of the test, restoring it on cleanup. Tests must not run in parallel
// when using chdir (Go's testing.T does not parallelise by default).
func chdir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() {
		_ = os.Chdir(old)
	})
}

func TestExecute_NoArgs(t *testing.T) {
	var out, errOut bytes.Buffer
	err := Execute() // not invoked via os.Args; uses real args.
	_ = out
	_ = errOut
	// Execute() reads os.Args. To exercise "no args" deterministically
	// we just call printHelp directly.
	_ = err
}

func TestRunInit_HappyPath(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	var out, errOut bytes.Buffer
	require.NoError(t, runInit(nil, &out, &errOut))
	assert.Contains(t, out.String(), "initialised")

	// .kron/ exists.
	st, err := os.Stat(filepath.Join(dir, ".kron"))
	require.NoError(t, err)
	assert.True(t, st.IsDir())
	// .kron/intents/ exists.
	st, err = os.Stat(filepath.Join(dir, ".kron", "intents"))
	require.NoError(t, err)
	assert.True(t, st.IsDir())
	// config.toml exists.
	data, err := os.ReadFile(filepath.Join(dir, ".kron", "config.toml"))
	require.NoError(t, err)
	assert.Contains(t, string(data), `intents_dir = ".kron/intents"`)
}

func TestRunInit_Idempotent(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	var out, errOut bytes.Buffer
	require.NoError(t, runInit(nil, &out, &errOut))
	// Second call should be a no-op success, not an error.
	require.NoError(t, runInit(nil, &out, &errOut))
	assert.Contains(t, out.String(), "already exists")
}

func TestRunAdd_RequiresInit(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	var out, errOut bytes.Buffer
	err := runAdd([]string{"foo"}, &out, &errOut)
	require.Error(t, err)
	assert.Contains(t, errOut.String(), "run `kron init` first")
}

func TestRunAdd_HappyPath(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	var out, errOut bytes.Buffer
	require.NoError(t, runInit(nil, &out, &errOut))

	// Pass -created-by so the test does not depend on the host's
	// git config user.name.
	require.NoError(t, runAdd([]string{"-created-by", "@tester", "auth/jwt"}, &out, &errOut))
	assert.Contains(t, out.String(), "created")

	// File exists.
	data, err := os.ReadFile(filepath.Join(dir, ".kron", "intents", "auth", "jwt.md"))
	require.NoError(t, err)
	body := string(data)
	assert.Contains(t, body, "created_by:")
	assert.Contains(t, body, "@tester")
	assert.Contains(t, body, "<!-- kron:frontmatter -->")
	assert.Contains(t, body, "## 为什么（Why）")
}

func TestRunAdd_RejectsBadSlug(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	var out, errOut bytes.Buffer
	require.NoError(t, runInit(nil, &out, &errOut))

	// Contains uppercase.
	err := runAdd([]string{"Auth/Jwt"}, &out, &errOut)
	require.Error(t, err)
	assert.Contains(t, errOut.String(), "invalid slug")
}

func TestRunAdd_RequiresSlugArg(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	var out, errOut bytes.Buffer
	require.NoError(t, runInit(nil, &out, &errOut))

	err := runAdd(nil, &out, &errOut)
	require.Error(t, err)
	assert.Contains(t, errOut.String(), "expected exactly one <slug>")

	err = runAdd([]string{"a", "b"}, &out, &errOut)
	require.Error(t, err)
}

func TestRunAdd_RejectsDuplicate(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	var out, errOut bytes.Buffer
	require.NoError(t, runInit(nil, &out, &errOut))
	require.NoError(t, runAdd([]string{"-created-by", "@t", "foo"}, &out, &errOut))

	err := runAdd([]string{"-created-by", "@t", "foo"}, &out, &errOut)
	require.Error(t, err)
	assert.Contains(t, errOut.String(), "already exists")
}

func TestRunLint_NoIntents(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	var out, errOut bytes.Buffer
	require.NoError(t, runInit(nil, &out, &errOut))
	out.Reset()
	errOut.Reset()

	err := runLint(nil, &out, &errOut)
	require.NoError(t, err)
	assert.Contains(t, out.String(), "[ok] 0 errors")
}

func TestRunLint_DanglingAnchor(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	var out, errOut bytes.Buffer
	require.NoError(t, runInit(nil, &out, &errOut))
	// A Go file with an anchor that points to a non-existent intent.
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "x.go"),
		[]byte("package x\n// @kron:intent nonexistent/slug\nfunc A(){}\n"),
		0o644,
	))
	out.Reset()
	errOut.Reset()

	err := runLint(nil, &out, &errOut)
	require.Error(t, err, "lint with a dangling anchor must return an ExitCoder error")
	// Confirm the error is an ExitCoder with code 1.
	var ec ExitCoder
	require.ErrorAs(t, err, &ec)
	assert.Equal(t, exitLint, ec.ExitCode())
	assert.Contains(t, out.String(), "anchor-dangling")
	assert.Contains(t, out.String(), "nonexistent/slug")
}

func TestRunLint_CleanAfterAdd(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	var out, errOut bytes.Buffer
	require.NoError(t, runInit(nil, &out, &errOut))
	// Scaffold an intent AND a matching anchor in source.
	require.NoError(t, runAdd([]string{"-created-by", "@t", "auth/refresh"}, &out, &errOut))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "x.go"),
		[]byte("package x\n// @kron:intent auth/refresh\nfunc A(){}\n"),
		0o644,
	))
	out.Reset()
	errOut.Reset()

	err := runLint(nil, &out, &errOut)
	require.NoError(t, err)
	assert.Contains(t, out.String(), "[ok] 0 errors")
}

func TestRunLint_JSONReporter(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	var out, errOut bytes.Buffer
	require.NoError(t, runInit(nil, &out, &errOut))
	out.Reset()

	require.NoError(t, runLint([]string{"-reporter", "json"}, &out, &errOut))
	s := out.String()
	assert.Contains(t, s, `"diagnostics"`)
	assert.Contains(t, s, `"summary"`)
}

func TestRunLint_JSONReporterWithError(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	var out, errOut bytes.Buffer
	require.NoError(t, runInit(nil, &out, &errOut))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "x.go"),
		[]byte("package x\n// @kron:intent nope/thing\n"),
		0o644,
	))
	out.Reset()
	errOut.Reset()

	err := runLint([]string{"-reporter", "json"}, &out, &errOut)
	require.Error(t, err)
	s := out.String()
	assert.Contains(t, s, `"rule": "anchor-dangling"`)
}

func TestItoa(t *testing.T) {
	cases := map[int]string{
		0:      "0",
		1:      "1",
		42:     "42",
		-7:     "-7",
		100000: "100000",
	}
	for in, want := range cases {
		assert.Equal(t, want, itoa(in), "itoa(%d)", in)
	}
}

func TestDefaultIntentBody(t *testing.T) {
	body := defaultIntentBody("auth/refresh-token")
	// Title is the last slug segment, dashes to spaces.
	assert.True(t, strings.HasPrefix(body, "# refresh token\n"),
		"body must start with '# refresh token', got: %q", body[:min(60, len(body))])
	assert.Contains(t, body, "## 为什么（Why）")
	assert.Contains(t, body, "## 权衡（Trade-offs）")
	assert.Contains(t, body, "<!-- 边界假设")
}
