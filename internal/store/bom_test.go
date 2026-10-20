package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The users of Kron are on Windows, where editors and PowerShell's
// `Set-Content -Encoding UTF8` prepend a UTF-8 BOM. Every user-editable
// data file must tolerate it; these tests pin each reader.

func TestLoadConfig_ToleratesUTF8BOM(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".kron"), 0o755))
	raw := append([]byte{0xEF, 0xBB, 0xBF}, []byte("intents_dir = \".kron/custom\"\n")...)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".kron", "config.toml"), raw, 0o644))

	cfg, err := LoadConfig(context.Background(), dir)
	require.NoError(t, err)
	assert.Equal(t, ".kron/custom", cfg.IntentsDir,
		"a BOM'd config must still apply intents_dir, not silently fall back to the default")
}

func TestReadProjectsJSONAt_ToleratesUTF8BOM(t *testing.T) {
	path := filepath.Join(t.TempDir(), "projects.json")
	raw := append(
		[]byte{0xEF, 0xBB, 0xBF},
		[]byte(`{"current_project":"","projects":[{"path":"E:\\works\\proj"}]}`)...,
	)
	require.NoError(t, os.WriteFile(path, raw, 0o644))

	reg, err := readProjectsJSONAt(path)
	require.NoError(t, err)
	require.Len(t, reg.Projects, 1)
	assert.Equal(t, `E:\works\proj`, reg.Projects[0].Path)
}

func TestReader_Load_ToleratesUTF8BOMIntent(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".kron/intents/auth-jwt.md", "\uFEFF"+sampleIntent)

	r, err := NewReader(dir)
	require.NoError(t, err)

	intent, err := r.Load(context.Background(), "auth-jwt")
	require.NoError(t, err)
	assert.Equal(t, "auth-jwt", intent.Slug)
	assert.Equal(t, "@alice", intent.Frontmatter.CreatedBy)
	assert.Contains(t, intent.Body, "Body line 1.")

	// LoadAll must see it too: a single BOM'd file used to fail the whole
	// snapshot and (via lint) skip all per-intent rules.
	all, err := r.LoadAll(context.Background())
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.Equal(t, "auth-jwt", all[0].Slug)
}
