package servemcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadAllCached_RefreshesForExternalIntentChanges(t *testing.T) {
	dir := initRepo(t)
	intentsDir := filepath.Join(dir, ".kron", "intents")
	authPath := filepath.Join(intentsDir, "auth.md")
	seedIntent(t, dir, filepath.Join(".kron", "intents", "auth.md"), sampleIntentWire)

	first, err := LoadAllCached(context.Background(), dir)
	require.NoError(t, err)
	require.Len(t, first, 1)
	require.Contains(t, first[0].Body, "Body.")

	// A direct edit represents another MCP process or a human editor. Keep
	// the byte length unchanged so the test proves that the file mtime (not
	// just its size) invalidates the cached parsed intent.
	changed := strings.Replace(sampleIntentWire, "Body.", "Fresh", 1)
	require.Len(t, changed, len(sampleIntentWire))
	require.NoError(t, os.WriteFile(authPath, []byte(changed), 0o644))
	changedAt := time.Now().Add(2 * time.Second)
	require.NoError(t, os.Chtimes(authPath, changedAt, changedAt))

	refreshed, err := LoadAllCached(context.Background(), dir)
	require.NoError(t, err)
	require.Len(t, refreshed, 1)
	require.Contains(t, refreshed[0].Body, "Fresh")

	// Additions and deletions are also external filesystem changes; neither
	// operation relies on this process calling InvalidateCache.
	paymentsPath := filepath.Join(intentsDir, "payments.md")
	require.NoError(t, os.WriteFile(paymentsPath, []byte(sampleIntentWire), 0o644))
	withAddition, err := LoadAllCached(context.Background(), dir)
	require.NoError(t, err)
	require.Len(t, withAddition, 2)

	require.NoError(t, os.Remove(paymentsPath))
	withDeletion, err := LoadAllCached(context.Background(), dir)
	require.NoError(t, err)
	require.Len(t, withDeletion, 1)
}
