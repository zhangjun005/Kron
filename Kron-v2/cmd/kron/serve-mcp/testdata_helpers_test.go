package servemcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// sampleIntentWire is a minimal valid intent .md used by the seeded
// integration tests. It uses the sentinel-wrapped frontmatter format
// (see internal/parser/frontmatter.go for the spec).
const sampleIntentWire = `<!-- kron:frontmatter -->
created_by: "@test"
updated_at: 2026-10-03T10:00:00Z
status: active
<!-- /kron:frontmatter -->

# Test intent

Body.
`

// seedIntent writes content to <root>/<rel> with parents created.
// Used by integration tests that need a populated .kron/intents/
// tree to exercise kron_list / kron_get.
func seedIntent(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
}
