package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSplitMarkdown_ToleratesUTF8BOM: Windows editors (and PowerShell 5.1
// `Set-Content -Encoding UTF8`) prepend a UTF-8 BOM to files they save.
// Intent and assumption markdown are user-editable, so both the sentinel
// and the legacy fence splitters must accept a leading BOM.
func TestSplitMarkdown_ToleratesUTF8BOM(t *testing.T) {
	const bom = "\uFEFF"
	testCases := []struct {
		name string
		raw  string
	}{
		{
			name: "sentinel",
			raw: bom + `<!-- kron:frontmatter -->
created_by: "@alice"
updated_at: 2026-10-01T10:00:00Z
<!-- /kron:frontmatter -->

# Sample

Body line.
`,
		},
		{
			name: "legacy fence",
			raw: bom + `---
created_by: "@alice"
updated_at: 2026-10-01T10:00:00Z
---

# Sample

Body line.
`,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fmRaw, body, err := SplitMarkdown([]byte(tc.raw))
			require.NoError(t, err)
			assert.Contains(t, string(fmRaw), "created_by")
			assert.Contains(t, body, "Body line.")
			// The BOM must not leak into the YAML payload.
			assert.NotContains(t, string(fmRaw), "\uFEFF")

			_, err = ParseFrontmatter(fmRaw)
			require.NoError(t, err, "BOM-stripped frontmatter must parse")
		})
	}
}
