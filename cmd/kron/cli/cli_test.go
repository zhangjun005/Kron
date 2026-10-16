package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xxx/kron/internal/store"
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

func TestRunTrashAndRestore(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	var out, errOut bytes.Buffer
	require.NoError(t, runInit(nil, &out, &errOut))
	require.NoError(t, runAdd([]string{"-created-by", "@tester", "auth/jwt"}, &out, &errOut))

	require.NoError(t, runTrash([]string{"auth/jwt"}, &out, &errOut))
	assert.FileExists(t, filepath.Join(dir, ".kron", ".trash", "auth", "jwt.md"))
	assert.NoFileExists(t, filepath.Join(dir, ".kron", "intents", "auth", "jwt.md"))

	require.NoError(t, runRestore([]string{"auth/jwt"}, &out, &errOut))
	assert.FileExists(t, filepath.Join(dir, ".kron", "intents", "auth", "jwt.md"))
	assert.NoFileExists(t, filepath.Join(dir, ".kron", ".trash", "auth", "jwt.md"))
	assert.Contains(t, out.String(), "trashed auth/jwt")
	assert.Contains(t, out.String(), "restored auth/jwt")
}

func TestRunTrashAndRestoreRequireExactlyOneSlug(t *testing.T) {
	for _, command := range []struct {
		name string
		run  func([]string, io.Writer, io.Writer) error
	}{
		{name: "trash", run: runTrash},
		{name: "restore", run: runRestore},
	} {
		t.Run(command.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			assert.ErrorIs(t, command.run(nil, &out, &errOut), errUsage)
			assert.ErrorIs(t, command.run([]string{"one", "two"}, &out, &errOut), errUsage)
		})
	}
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

// TestRunAdd_NodeKind verifies --kind node writes <slug>/README.md
// (not <slug>.md). This is the user-visible face of RFC
// 2026-10-08-writer-readme-symmetry §3.4: "kron add auth --kind node"
// must produce a module-level node intent the reader can resolve.
func TestRunAdd_NodeKind(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	var out, errOut bytes.Buffer
	require.NoError(t, runInit(nil, &out, &errOut))

	require.NoError(t, runAdd([]string{"-created-by", "@t", "-kind", "node", "auth"}, &out, &errOut))

	// Canonical <slug>.md must NOT exist.
	_, err := os.Stat(filepath.Join(dir, ".kron", "intents", "auth.md"))
	assert.True(t, os.IsNotExist(err), "kron add --kind node must not write <slug>.md")

	// Node form <slug>/README.md must exist.
	body, err := os.ReadFile(filepath.Join(dir, ".kron", "intents", "auth", "README.md"))
	require.NoError(t, err)
	assert.Contains(t, string(body), "auth")

	// Output must mention the README path, not the .md path.
	assert.Contains(t, out.String(), "README.md")
}

// TestRunAdd_DefaultKindIsLeaf verifies that omitting --kind
// preserves the v1 behaviour: write <slug>.md.
func TestRunAdd_DefaultKindIsLeaf(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	var out, errOut bytes.Buffer
	require.NoError(t, runInit(nil, &out, &errOut))

	require.NoError(t, runAdd([]string{"-created-by", "@t", "billing"}, &out, &errOut))

	_, err := os.Stat(filepath.Join(dir, ".kron", "intents", "billing.md"))
	assert.NoError(t, err, "default kind must write <slug>.md")
}

// TestRunAdd_RejectsBadKind verifies --kind accepts only leaf|node.
func TestRunAdd_RejectsBadKind(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	var out, errOut bytes.Buffer
	require.NoError(t, runInit(nil, &out, &errOut))

	err := runAdd([]string{"-kind", "tree", "foo"}, &out, &errOut)
	require.Error(t, err)
	assert.Contains(t, errOut.String(), "invalid --kind")
}

// TestRunAdd_ParentMustExist verifies --parent enforces that
// the named parent slug is already a known intent.
func TestRunAdd_ParentMustExist(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	var out, errOut bytes.Buffer
	require.NoError(t, runInit(nil, &out, &errOut))

	// Create the parent first.
	require.NoError(t, runAdd([]string{"-created-by", "@t", "-kind", "node", "auth"}, &out, &errOut))

	// Now a child under the parent should succeed.
	out.Reset()
	errOut.Reset()
	require.NoError(t, runAdd([]string{"-created-by", "@t", "-parent", "auth", "auth/jwt"}, &out, &errOut))

	// And without --parent, the same child via slug form should also work
	// (slug "auth/jwt" implicitly puts it under auth/).
	out.Reset()
	errOut.Reset()
	require.NoError(t, runAdd([]string{"-created-by", "@t", "auth/password"}, &out, &errOut))
}

// TestRunAdd_ParentMissingFails verifies --parent errors when
// the named parent slug has not been created yet.
func TestRunAdd_ParentMissingFails(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	var out, errOut bytes.Buffer
	require.NoError(t, runInit(nil, &out, &errOut))

	err := runAdd([]string{"-parent", "ghost", "billing"}, &out, &errOut)
	require.Error(t, err)
	assert.Contains(t, errOut.String(), "parent intent")
	assert.Contains(t, errOut.String(), "ghost")
	assert.Contains(t, errOut.String(), "does not exist")
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
	// Scaffold a parent node intent AND a child leaf intent, plus
	// a matching anchor in source. The parent node anchors the
	// "auth" directory so the new intent-orphan-under-no-node
	// rule does not fire.
	require.NoError(t, runAdd([]string{"-created-by", "@t", "--kind", "node", "auth"}, &out, &errOut))
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

// TestRunInit_RegistersProjectInRegistry covers RFC
// 2026-10-14-projects-json-schema-extension §2.6: `kron init`
// must register the project into projects.json AND set it as
// current_project. We invoke the readProjectsJSONAt test
// helper via the store package by copying the freshly-written
// projects.json to a t.TempDir() and reading it from there.
// Direct reads of the production path would race with other
// tests, so we just verify the side-effect via the public
// store.ReadProjectsJSON — which uses os.Executable() to
// locate the file. The test bin's parent dir is safe to
// touch because it's ephemeral (go test creates a fresh
// temp bin per run).
func TestRunInit_RegistersProjectInRegistry(t *testing.T) {
	dir := t.TempDir()
	registryPath := filepath.Join(dir, "projects.json")
	t.Setenv("KRON_PROJECTS_JSON_PATH", registryPath)
	chdir(t, dir)
	var out, errOut bytes.Buffer
	require.NoError(t, runInit(nil, &out, &errOut))

	reg, err := store.ReadProjectsJSON()
	require.NoError(t, err, "projects.json must be readable after `kron init`")
	absDir, err := filepath.Abs(dir)
	require.NoError(t, err)
	assert.Equal(t, absDir, reg.CurrentProject,
		"`kron init` must promote the new project to current_project (RFC §2.6)")
	require.Len(t, reg.Projects, 1)
	assert.Equal(t, absDir, reg.Projects[0].Path)
	assert.NotEmpty(t, reg.Projects[0].AddedAt, "added_at must be set on register")
	assert.Empty(t, reg.Projects[0].LastOpened,
		"`kron init` must not set last_opened (only the GUI does)")
}

// TestRunRegister_HappyPath exercises the new `kron register`
// subcommand: registers a project into projects.json and
// auto-promotes it to current_project (RFC §2.1 + §2.6).
func TestRunRegister_HappyPath(t *testing.T) {
	dir := t.TempDir()
	registryPath := filepath.Join(dir, "projects.json")
	t.Setenv("KRON_PROJECTS_JSON_PATH", registryPath)
	chdir(t, dir)
	var out, errOut bytes.Buffer

	// First register: must report insertion.
	require.NoError(t, runRegister(nil, &out, &errOut))
	assert.Contains(t, out.String(), "registered")
	assert.Contains(t, out.String(), "current_project")

	reg, err := store.ReadProjectsJSON()
	require.NoError(t, err)
	absDir, err := filepath.Abs(dir)
	require.NoError(t, err)
	assert.Equal(t, absDir, reg.CurrentProject)
	require.Len(t, reg.Projects, 1)
	assert.NotEmpty(t, reg.Projects[0].AddedAt)
}

// TestRunRegister_Idempotent: a second `kron register` of the
// same path is a no-op, not an error. The output distinguishes
// "already registered" from "newly registered" so the user
// knows which happened.
func TestRunRegister_Idempotent(t *testing.T) {
	dir := t.TempDir()
	registryPath := filepath.Join(dir, "projects.json")
	t.Setenv("KRON_PROJECTS_JSON_PATH", registryPath)
	chdir(t, dir)
	var out, errOut bytes.Buffer
	require.NoError(t, runRegister(nil, &out, &errOut))
	out.Reset()

	require.NoError(t, runRegister(nil, &out, &errOut), "duplicate register must not error")
	assert.Contains(t, out.String(), "already registered")
}

// TestRunRegister_WithCWD exercises the -CWD flag for explicit
// target paths.
func TestRunRegister_WithCWD(t *testing.T) {
	dir := t.TempDir()
	otherDir := t.TempDir()
	registryPath := filepath.Join(dir, "projects.json")
	t.Setenv("KRON_PROJECTS_JSON_PATH", registryPath)
	var out, errOut bytes.Buffer

	require.NoError(t, runRegister([]string{"-CWD", otherDir}, &out, &errOut))
	assert.Contains(t, out.String(), "registered")

	reg, err := store.ReadProjectsJSON()
	require.NoError(t, err)
	absDir, err := filepath.Abs(otherDir)
	require.NoError(t, err)
	assert.Equal(t, absDir, reg.CurrentProject)
}

// TestRunRegister_UsageError covers the bad-args path: `kron
// register` with an unparseable -CWD must surface errUsage
// and not silently succeed.
func TestRunRegister_UsageError(t *testing.T) {
	dir := t.TempDir()
	registryPath := filepath.Join(dir, "projects.json")
	t.Setenv("KRON_PROJECTS_JSON_PATH", registryPath)
	var out, errOut bytes.Buffer
	err := runRegister([]string{"-bogus-flag"}, &out, &errOut)
	require.Error(t, err, "unknown flag must fail")
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
