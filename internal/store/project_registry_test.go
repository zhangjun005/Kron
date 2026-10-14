package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixed time for deterministic RFC3339 outputs.
var fixedNow = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

// TestProjectsJSON_RoundTripMissingFile: per RFC §2.2.1, a missing
// projects.json must return an empty registry (no error), and the
// empty registry must marshal to a file that round-trips on read.
func TestProjectsJSON_RoundTripMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "projects.json")

	reg, err := readProjectsJSONAt(path)
	require.NoError(t, err, "missing file must not error")
	require.NotNil(t, reg)
	assert.Empty(t, reg.CurrentProject, "missing file yields empty current_project")
	assert.Empty(t, reg.Projects, "missing file yields empty projects slice")

	// Write the empty registry and re-read.
	require.NoError(t, writeProjectsJSONAt(path, reg))
	reg2, err := readProjectsJSONAt(path)
	require.NoError(t, err)
	assert.Equal(t, "", reg2.CurrentProject)
	assert.Empty(t, reg2.Projects)
}

// TestProjectsJSON_ForwardCompatIgnoresUnknownFields: v1.4+ may add
// new fields; v1.3 readers must not break. Simulate by hand-writing
// a file with an extra "settings" key.
func TestProjectsJSON_ForwardCompatIgnoresUnknownFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "projects.json")
	// Hand-crafted: a v1.4 hypothetical "settings.mcp" sub-object
	// that v1.3 doesn't know about.
	body := `{
		"current_project": "",
		"projects": [
			{"path": "C:/a", "added_at": "2026-10-14T00:00:00Z"}
		],
		"settings": {"mcp": {"log_level": "debug"}}
	}
	`
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))

	reg, err := readProjectsJSONAt(path)
	require.NoError(t, err, "unknown fields must be silently ignored (Go default)")
	require.Len(t, reg.Projects, 1)
	assert.Equal(t, "C:/a", reg.Projects[0].Path)
}

// TestProjectsJSON_TypeMismatchIsError: per RFC §2.2.1e, type
// mismatches (e.g. projects is a string instead of an array) are
// NOT tolerated. We must surface the parse error so the user
// sees a clear "your projects.json is corrupted" message instead
// of silently dropping their data.
func TestProjectsJSON_TypeMismatchIsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "projects.json")
	body := `{"projects": "this should be an array, not a string"}`
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))

	_, err := readProjectsJSONAt(path)
	require.Error(t, err, "type mismatch must fail parse, not be coerced")
	assert.Contains(t, err.Error(), "projects.json", "error must name the file")
}

// TestProjectsJSON_OmitemptyProducesMinimalFile: a registry with
// an empty current_project and a single fresh project must marshal
// to a file that does NOT contain "current_project" or
// "last_opened" — those are omitempty per RFC §2.1. This is the
// "writer never writes zero values" invariant (RFC §2.2.3).
func TestProjectsJSON_OmitemptyProducesMinimalFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "projects.json")
	reg := &ProjectRegistry{
		CurrentProject: "",
		Projects: []ProjectEntry{
			{Path: "C:/a", AddedAt: "2026-10-14T00:00:00Z"},
		},
	}
	require.NoError(t, writeProjectsJSONAt(path, reg))

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	asString := string(raw)
	assert.NotContains(t, asString, "current_project",
		"empty current_project must NOT appear in the file (omitempty)")
	assert.NotContains(t, asString, "last_opened",
		"missing last_opened must NOT appear in the file (omitempty)")
	assert.Contains(t, asString, "C:/a", "real data must round-trip")
}

// TestProjectsJSON_ProjectsAlwaysPresent: the `projects` key is
// required (RFC §2.1). Even an empty registry must marshal with
// "projects": [] not omit the key. This guarantees downstream
// readers always see an array, not a missing/null field.
func TestProjectsJSON_ProjectsAlwaysPresent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "projects.json")
	reg := &ProjectRegistry{Projects: nil} // explicit nil
	require.NoError(t, writeProjectsJSONAt(path, reg))

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"projects": []`,
		"projects key must always be present in the file (RFC §2.1)")

	// Re-read: the empty projects slice round-trips.
	reg2, err := readProjectsJSONAt(path)
	require.NoError(t, err)
	assert.NotNil(t, reg2.Projects, "empty projects on disk must read as empty slice, not nil")
}

// TestRegisterProject_InsertsNew: first call to RegisterProject
// inserts the entry with the given timestamp. Path is absolute'd
// before storage.
func TestRegisterProject_InsertsNew(t *testing.T) {
	reg := &ProjectRegistry{Projects: []ProjectEntry{}}
	inserted, err := RegisterProject(reg, ".", fixedNow)
	require.NoError(t, err)
	assert.True(t, inserted, "first register must return inserted=true")

	require.Len(t, reg.Projects, 1)
	entry := reg.Projects[0]
	assert.True(t, filepath.IsAbs(entry.Path), "stored path must be absolute")
	assert.Equal(t, "2026-10-14T12:00:00Z", entry.AddedAt, "added_at must be RFC 3339 UTC")
	assert.Empty(t, entry.LastOpened, "fresh register must not set last_opened")
}

// TestRegisterProject_Dedup: a second register of the same path
// is a no-op (path is the unique key per RFC §2.1). inserted
// returns false and the existing entry is unchanged.
func TestRegisterProject_Dedup(t *testing.T) {
	dir := t.TempDir()
	abs, err := filepath.Abs(dir)
	require.NoError(t, err)
	reg := &ProjectRegistry{Projects: []ProjectEntry{
		{Path: abs, AddedAt: "2026-10-13T00:00:00Z"},
	}}
	inserted, err := RegisterProject(reg, abs, fixedNow)
	require.NoError(t, err)
	assert.False(t, inserted, "duplicate register must return inserted=false")
	require.Len(t, reg.Projects, 1, "duplicate must not append")
	assert.Equal(t, "2026-10-13T00:00:00Z", reg.Projects[0].AddedAt,
		"existing added_at must not be overwritten on dedup")
}

// TestRegisterProject_DedupAfterAbs: "C:/a" and
// "C:\a" (Windows) or "/a/." and "/a" (POSIX) must both
// resolve to the same absolute path and trigger dedup. We
// can't easily synthesise the Windows case from a Linux runner
// (filepath.Abs on Linux won't change "C:/a"), so we use a
// POSIX-equivalent: relative path that normalises to an existing
// absolute entry.
func TestRegisterProject_DedupAfterAbs(t *testing.T) {
	dir := t.TempDir()
	abs, err := filepath.Abs(dir)
	require.NoError(t, err)

	reg := &ProjectRegistry{Projects: []ProjectEntry{
		{Path: abs, AddedAt: "2026-10-13T00:00:00Z"},
	}}
	// Register again with a different spelling (trailing slash
	// on POSIX) — filepath.Abs normalises it to the same key.
	inserted, err := RegisterProject(reg, abs+"/", fixedNow)
	require.NoError(t, err)
	assert.False(t, inserted, "filepath.Abs must normalise before dedup")
}

// TestSetCurrentProject_RecordsPrev: SetCurrentProject returns
// the previous value so the GUI can log "switched X → Y".
func TestSetCurrentProject_RecordsPrev(t *testing.T) {
	oldDir := t.TempDir()
	newDir := t.TempDir()
	oldAbs, err := filepath.Abs(oldDir)
	require.NoError(t, err)
	newAbs, err := filepath.Abs(newDir)
	require.NoError(t, err)

	reg := &ProjectRegistry{CurrentProject: oldAbs}
	prev, err := SetCurrentProject(reg, newAbs)
	require.NoError(t, err)
	assert.Equal(t, oldAbs, prev, "previous current_project must be returned")
	assert.Equal(t, newAbs, reg.CurrentProject)
}

// TestSetCurrentProject_EmptyClears: empty string is allowed
// (clears the active project) — covers the GUI's "no project
// selected" state.
func TestSetCurrentProject_EmptyClears(t *testing.T) {
	dir := t.TempDir()
	abs, err := filepath.Abs(dir)
	require.NoError(t, err)
	reg := &ProjectRegistry{CurrentProject: abs}
	prev, err := SetCurrentProject(reg, "")
	require.NoError(t, err)
	assert.Equal(t, abs, prev)
	assert.Equal(t, "", reg.CurrentProject, "empty arg must clear current_project")
}

// TestTouchLastOpened_UpdatesEntry: TouchLastOpened sets the
// timestamp on the matching entry. Returns true.
func TestTouchLastOpened_UpdatesEntry(t *testing.T) {
	dir := t.TempDir()
	abs, err := filepath.Abs(dir)
	require.NoError(t, err)
	reg := &ProjectRegistry{Projects: []ProjectEntry{
		{Path: abs, AddedAt: "2026-10-13T00:00:00Z"},
	}}
	ok, err := TouchLastOpened(reg, abs, fixedNow)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "2026-10-14T12:00:00Z", reg.Projects[0].LastOpened)
}

// TestTouchLastOpened_MissingEntryNoOp: touching a path that's
// not in the registry is a no-op (we don't auto-register; the
// GUI's flow is: switch project → register if new → touch).
func TestTouchLastOpened_MissingEntryNoOp(t *testing.T) {
	dir := t.TempDir()
	abs, err := filepath.Abs(dir)
	require.NoError(t, err)
	other := filepath.Join(abs, "subdir-that-doesnt-exist")
	reg := &ProjectRegistry{Projects: []ProjectEntry{
		{Path: abs, AddedAt: "2026-10-13T00:00:00Z"},
	}}
	ok, err := TouchLastOpened(reg, other, fixedNow)
	require.NoError(t, err)
	assert.False(t, ok, "touching unknown path must return false")
	assert.Empty(t, reg.Projects[0].LastOpened, "untouched entry stays untouched")
}

// TestProjectsJSON_FullCycleRegisterAndTouch: the full lifecycle
// from RFC §2.1: register a project, set it current, touch it.
// The file on disk matches what we expect after a round-trip
// read.
func TestProjectsJSON_FullCycleRegisterAndTouch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "projects.json")
	projectPath, err := filepath.Abs(t.TempDir())
	require.NoError(t, err)

	// Step 1: empty registry
	reg, err := readProjectsJSONAt(path)
	require.NoError(t, err)

	// Step 2: register a new project
	inserted, err := RegisterProject(reg, projectPath, fixedNow)
	require.NoError(t, err)
	assert.True(t, inserted)

	// Step 3: set it as the current project (simulates `kron register`
	// auto-promoting to current per RFC §2.6)
	_, err = SetCurrentProject(reg, projectPath)
	require.NoError(t, err)

	// Step 4: touch (simulates GUI user clicking the project card)
	ok, err := TouchLastOpened(reg, projectPath, fixedNow.Add(time.Hour))
	require.NoError(t, err)
	assert.True(t, ok)

	// Step 5: persist
	require.NoError(t, writeProjectsJSONAt(path, reg))

	// Step 6: round-trip
	reg2, err := readProjectsJSONAt(path)
	require.NoError(t, err)
	assert.Equal(t, projectPath, reg2.CurrentProject)
	require.Len(t, reg2.Projects, 1)
	assert.Equal(t, "2026-10-14T12:00:00Z", reg2.Projects[0].AddedAt)
	assert.Equal(t, "2026-10-14T13:00:00Z", reg2.Projects[0].LastOpened)
}

// TestProjectsJSON_AtomicWriteLeavesNoTempFile: the
// writeFileAtomic helper already cleans up on success; verify
// the post-write directory contains only projects.json, no
// stray .kron-write-*.tmp file.
func TestProjectsJSON_AtomicWriteLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "projects.json")
	require.NoError(t, writeProjectsJSONAt(path, &ProjectRegistry{Projects: []ProjectEntry{}}))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		assert.False(t, strings.HasPrefix(e.Name(), ".kron-write-"),
			"atomic write must not leave a temp file: %s", e.Name())
	}
}

// TestProjectsJSON_DefensiveRejectsNilReceiver: every public
// function must not panic on a nil registry (callers in
// production can hit this if they pass a freshly-zeroed struct).
// We surface a clear error instead of dereferencing nil.
func TestProjectsJSON_DefensiveRejectsNilReceiver(t *testing.T) {
	_, err := RegisterProject(nil, "/a", fixedNow)
	assert.Error(t, err)
	_, err = SetCurrentProject(nil, "/a")
	assert.Error(t, err)
	_, err = TouchLastOpened(nil, "/a", fixedNow)
	assert.Error(t, err)
	err = WriteProjectsJSON(nil)
	assert.Error(t, err)
}

// TestProjectsJSON_EmptyProjectsAfterNil: per the algorithm in
// readProjectsJSONAt, a "projects": null in the file must
// surface as an empty (non-nil) slice after read. Verified via
// round-trip.
func TestProjectsJSON_EmptyProjectsAfterNil(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "projects.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"projects": null}`), 0o644))

	reg, err := readProjectsJSONAt(path)
	require.NoError(t, err)
	assert.NotNil(t, reg.Projects, "null projects must read as empty slice")
	assert.Empty(t, reg.Projects)
}

// TestProjectsJSON_IndentationIsHumanReadable: per the comment
// in writeProjectsJSONAt, we use 2-space indent + trailing
// newline. Verify so future contributors don't "optimise" it
// into single-line JSON.
func TestProjectsJSON_IndentationIsHumanReadable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "projects.json")
	reg := &ProjectRegistry{Projects: []ProjectEntry{
		{Path: "C:/a", AddedAt: "2026-10-14T00:00:00Z"},
	}}
	require.NoError(t, writeProjectsJSONAt(path, reg))

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	// 2-space indent, trailing newline, valid JSON.
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(raw, &parsed))
	assert.True(t, strings.Contains(string(raw), "\n  "),
		"file must be multi-line (2-space indent) for human readability")
	assert.True(t, strings.HasSuffix(string(raw), "\n"),
		"file must end with a trailing newline (POSIX-friendly)")
}
