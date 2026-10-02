package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xxx/kron/internal/model"
)

func TestEnsureKronDir_CreatesDirs(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, EnsureKronDir(context.Background(), dir))

	// .kron/intents must exist.
	intentsDir := filepath.Join(dir, model.KronDir, model.IntentDir)
	st, err := os.Stat(intentsDir)
	require.NoError(t, err)
	assert.True(t, st.IsDir())
}

func TestEnsureKronDir_Idempotent(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, EnsureKronDir(context.Background(), dir))
	// Second call must also succeed (no error when dir already exists).
	require.NoError(t, EnsureKronDir(context.Background(), dir))
}

func TestLoadConfig_MissingFileReturnsDefault(t *testing.T) {
	dir := t.TempDir()
	cfg, err := LoadConfig(context.Background(), dir)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, ".kron/intents", cfg.IntentsDir)
}

func TestSaveAndLoadConfig_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := &model.Config{IntentsDir: "docs/intents"}

	require.NoError(t, SaveConfig(context.Background(), dir, want))

	got, err := LoadConfig(context.Background(), dir)
	require.NoError(t, err)
	assert.Equal(t, want.IntentsDir, got.IntentsDir)
}

func TestLoadConfig_UnknownKeysIgnored(t *testing.T) {
	dir := t.TempDir()
	kronDir := filepath.Join(dir, model.KronDir)
	require.NoError(t, os.MkdirAll(kronDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(kronDir, model.ConfigFileName),
		[]byte("intents_dir = \"x\"\ndefault_reviewer = \"@alice\"\n"),
		0o644,
	))

	got, err := LoadConfig(context.Background(), dir)
	require.NoError(t, err)
	assert.Equal(t, "x", got.IntentsDir)
}

func TestLoadConfig_MalformedReturnsErrConfigInvalid(t *testing.T) {
	dir := t.TempDir()
	kronDir := filepath.Join(dir, model.KronDir)
	require.NoError(t, os.MkdirAll(kronDir, 0o755))
	// Missing '=', unquoted value.
	require.NoError(t, os.WriteFile(
		filepath.Join(kronDir, model.ConfigFileName),
		[]byte("intents_dir = oops-not-quoted\n"),
		0o644,
	))

	_, err := LoadConfig(context.Background(), dir)
	require.Error(t, err)
	assert.ErrorIs(t, err, model.ErrConfigInvalid)
}

func TestLoadConfig_EmptyValueRejected(t *testing.T) {
	dir := t.TempDir()
	kronDir := filepath.Join(dir, model.KronDir)
	require.NoError(t, os.MkdirAll(kronDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(kronDir, model.ConfigFileName),
		[]byte("intents_dir = \"\"\n"),
		0o644,
	))

	_, err := LoadConfig(context.Background(), dir)
	require.Error(t, err)
	assert.ErrorIs(t, err, model.ErrConfigInvalid)
}

func TestParseConfigTOML_CommentsAndBlankLines(t *testing.T) {
	cfg := &model.Config{}
	src := `# a comment
; another comment

intents_dir = "custom/intents"
`
	require.NoError(t, parseConfigTOML(src, cfg))
	assert.Equal(t, "custom/intents", cfg.IntentsDir)
}

func TestUnquoteTOMLString(t *testing.T) {
	cases := []struct {
		in   string
		want string
		err  bool
	}{
		{`"plain"`, "plain", false},
		{`"with \"quote\""`, `with "quote"`, false},
		{`"with \\back"`, `with \back`, false},
		{`"with \n newline"`, "with \n newline", false},
		{`"with \t tab"`, "with \t tab", false},
		{`unquoted`, "", true},
		{`"`, "", true},
		{`""`, "", false},
		{`"bad \x escape"`, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := unquoteTOMLString(tc.in)
			if tc.err {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
