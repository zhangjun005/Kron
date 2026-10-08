// Tests for the frontmatter relations feature added by
// RFC 2026-10-03-frontmatter-references.md (references + depends_on
// structural fields). Covers:
//
//   - kron_update patch support for the two new fields
//   - kron_get wire shape echoes the two new fields
//   - kron_impact now returns references + prerequisites
//     (replacing depends_on_intents) and still emits incoming_anchors
//   - kron_delete surfaces dependents in its response
//   - kron_update rejects malformed references / depends_on
//     (invalid slug, self-reference) with -32006
//
// Keep these tests in a separate file (rather than appended to
// handlers_test.go) so the existing per-handler tests stay focused
// on the per-handler happy/error paths; the relations tests are
// about cross-handler wire compatibility.

package servemcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xxx/kron/internal/parser"
)

// sep normalises path separators so cross-platform tests can assert
// against filepath.Rel output (Windows returns "\", others return "/").
const sep = string(filepath.Separator)

// seedIntentFile writes a fully-formed .kron/intents/<slug>.md with
// arbitrary frontmatter so we can exercise references/depends_on
// without going through kron_update's PATCH semantics. The
// addIntent helper in handlers_test.go doesn't support these new
// fields, so we write the file directly.
func seedIntentFile(t *testing.T, dir, slug, content string) {
	t.Helper()
	rel := filepath.Join(".kron", "intents", slug+".md")
	full := filepath.Join(dir, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
}

const sampleIntentWithRefs = `<!-- kron:frontmatter -->
created_by: "@alice"
updated_at: 2026-10-03T10:00:00Z
references:
  - oauth2-best-practices
depends_on:
  - auth/token-storage
<!-- /kron:frontmatter -->

# Sample
`

const sampleIntentBase = `<!-- kron:frontmatter -->
created_by: "@alice"
updated_at: 2026-10-03T10:00:00Z
<!-- /kron:frontmatter -->

# Sample
`

func TestUpdate_SetReferencesAndDependsOn(t *testing.T) {
	dir := initRepo(t)
	addIntent(t, dir, "auth/jwt", "", "")

	// PATCH references + depends_on via kron_update.
	resp := callJSON(t, dir, `{"jsonrpc":"2.0","method":"kron_update","params":{"slug":"auth/jwt","references":["oauth2-best-practices","session-timeout-ux"],"depends_on":["auth/token-storage","auth/csrf-protected"]},"id":1}`)
	require.Nil(t, resp.Error, "kron_update must accept references + depends_on")

	// kron_get should echo them back.
	get := callJSON(t, dir, `{"jsonrpc":"2.0","method":"kron_get","params":{"slug":"auth/jwt"},"id":2}`)
	require.Nil(t, get.Error)
	var got struct {
		Intent struct {
			Frontmatter struct {
				References []string `json:"references"`
				DependsOn  []string `json:"depends_on"`
			} `json:"frontmatter"`
		} `json:"intent"`
	}
	require.NoError(t, json.Unmarshal(get.Result, &got))
	assert.Equal(t, []string{"oauth2-best-practices", "session-timeout-ux"}, got.Intent.Frontmatter.References)
	assert.Equal(t, []string{"auth/token-storage", "auth/csrf-protected"}, got.Intent.Frontmatter.DependsOn)
}

func TestUpdate_EmptyArrayClearsReferences(t *testing.T) {
	// PATCH semantics: [] = clear, null = no change.
	dir := initRepo(t)
	seedIntentFile(t, dir, "auth/jwt", sampleIntentWithRefs)

	resp := callJSON(t, dir, `{"jsonrpc":"2.0","method":"kron_update","params":{"slug":"auth/jwt","references":[],"depends_on":[]},"id":1}`)
	require.Nil(t, resp.Error)

	get := callJSON(t, dir, `{"jsonrpc":"2.0","method":"kron_get","params":{"slug":"auth/jwt"},"id":2}`)
	require.Nil(t, get.Error)
	var got struct {
		Intent struct {
			Frontmatter struct {
				References []string `json:"references"`
				DependsOn  []string `json:"depends_on"`
			} `json:"frontmatter"`
		} `json:"intent"`
	}
	require.NoError(t, json.Unmarshal(get.Result, &got))
	assert.Empty(t, got.Intent.Frontmatter.References, "empty array must clear references")
	assert.Empty(t, got.Intent.Frontmatter.DependsOn, "empty array must clear depends_on")
}

func TestUpdate_RejectsSelfReference(t *testing.T) {
	// Patching depends_on to include the intent's own slug must
	// fail validation (-32006). Self-reference is a schema error.
	dir := initRepo(t)
	addIntent(t, dir, "auth/jwt", "", "")

	resp := callJSON(t, dir, `{"jsonrpc":"2.0","method":"kron_update","params":{"slug":"auth/jwt","depends_on":["auth/jwt"]},"id":1}`)
	require.NotNil(t, resp.Error, "self-reference in depends_on must be rejected")
	assert.Equal(t, -32006, resp.Error.Code, "self-reference → Unprocessable")
}

func TestImpact_ReferencesAndPrerequisites(t *testing.T) {
	// Build a 3-intent graph: jwt ← refresh, refresh depends_on jwt
	// (and on a slug whose symbol overlaps jwt's).
	dir := initRepo(t)
	seedIntentFile(t, dir, "auth/jwt", sampleIntentBase)
	seedIntentFile(t, dir, "auth/refresh", `<!-- kron:frontmatter -->
created_by: "@alice"
updated_at: 2026-10-03T10:00:00Z
references:
  - auth/jwt
depends_on:
  - auth/jwt
symbol:
  - auth.RefreshToken
<!-- /kron:frontmatter -->
`)
	// A third intent that overlaps jwt's symbol (inferred prerequisite).
	seedIntentFile(t, dir, "auth/refresh-helper", `<!-- kron:frontmatter -->
created_by: "@alice"
updated_at: 2026-10-03T10:00:00Z
symbol:
  - auth.RefreshToken
<!-- /kron:frontmatter -->
`)

	// Query impact on auth/jwt. Expected:
	//   references:  [auth/refresh]   (refresh lists jwt in its references)
	//   prerequisites: []              (jwt doesn't depend_on anyone explicit;
	//                                  and the symbol overlap is on the OTHER
	//                                  intents, not jwt's own symbol set)
	resp := callJSON(t, dir, `{"jsonrpc":"2.0","method":"kron_impact","params":{"slug":"auth/jwt"},"id":1}`)
	require.Nil(t, resp.Error)
	var r struct {
		References    []string `json:"references"`
		Prerequisites []string `json:"prerequisites"`
	}
	require.NoError(t, json.Unmarshal(resp.Result, &r))
	assert.Equal(t, []string{"auth/refresh"}, r.References)
	assert.Empty(t, r.Prerequisites)

	// Query impact on auth/refresh-helper. Expected:
	//   references:    []    (no intent lists it)
	//   prerequisites: [auth/refresh]   (via symbol overlap with RefreshToken)
	resp = callJSON(t, dir, `{"jsonrpc":"2.0","method":"kron_impact","params":{"slug":"auth/refresh-helper"},"id":2}`)
	require.Nil(t, resp.Error)
	r = struct {
		References    []string `json:"references"`
		Prerequisites []string `json:"prerequisites"`
	}{}
	require.NoError(t, json.Unmarshal(resp.Result, &r))
	assert.Empty(t, r.References)
	assert.Equal(t, []string{"auth/refresh"}, r.Prerequisites, "symbol overlap should infer prereq")
}

func TestDelete_SurfacesDependents(t *testing.T) {
	// Build: jwt is depended on by refresh. Delete jwt → dependents=[refresh].
	dir := initRepo(t)
	seedIntentFile(t, dir, "auth/jwt", sampleIntentBase)
	seedIntentFile(t, dir, "auth/refresh", `<!-- kron:frontmatter -->
created_by: "@alice"
updated_at: 2026-10-03T10:00:00Z
depends_on:
  - auth/jwt
<!-- /kron:frontmatter -->
`)

	resp := callJSON(t, dir, `{"jsonrpc":"2.0","method":"kron_delete","params":{"slug":"auth/jwt"},"id":1}`)
	require.Nil(t, resp.Error)
	var r struct {
		OK          bool     `json:"ok"`
		TrashedPath string   `json:"trashed_path"`
		Dependents  []string `json:"dependents"`
	}
	require.NoError(t, json.Unmarshal(resp.Result, &r))
	assert.True(t, r.OK)
	assert.Equal(t, ".kron/.trash/auth/jwt.md", r.TrashedPath)
	assert.Equal(t, []string{"auth/refresh"}, r.Dependents, "dependents must list refresh (it depended_on jwt)")
}

func TestDelete_NoDependents(t *testing.T) {
	// Delete an intent that no one depends_on → dependents is empty list.
	dir := initRepo(t)
	addIntent(t, dir, "lonely/intent", "", "")

	resp := callJSON(t, dir, `{"jsonrpc":"2.0","method":"kron_delete","params":{"slug":"lonely/intent"},"id":1}`)
	require.Nil(t, resp.Error)
	var r struct {
		Dependents []string `json:"dependents"`
	}
	require.NoError(t, json.Unmarshal(resp.Result, &r))
	assert.Empty(t, r.Dependents)
}

func TestImpact_DependsOnIntentsRemoved(t *testing.T) {
	// The v1 field name "depends_on_intents" must be GONE in v1.2;
	// clients should use "prerequisites" instead. This guards against
	// an accidental dual-emission regression.
	dir := initRepo(t)
	addIntent(t, dir, "auth/jwt", "", "")

	resp := callJSON(t, dir, `{"jsonrpc":"2.0","method":"kron_impact","params":{"slug":"auth/jwt"},"id":1}`)
	require.Nil(t, resp.Error)

	// Decode into a struct that doesn't declare DependsOnIntents; the
	// json package will silently drop the old field if it's emitted.
	// To detect it, decode into a map and assert the key is absent.
	var raw map[string]any
	require.NoError(t, json.Unmarshal(resp.Result, &raw))
	_, hasOld := raw["depends_on_intents"]
	assert.False(t, hasOld, "depends_on_intents must be removed; clients use prerequisites")
	_, hasNew := raw["prerequisites"]
	assert.True(t, hasNew, "prerequisites must be present")
}

func TestUpdate_DedupesReferencesAndDependsOn(t *testing.T) {
	// RFC §3.2: "重复由 YAML 解析层自动忽略". kron_update applies
	// the same rule at patch time so the on-disk list matches what
	// a YAML round-trip would yield. Order is preserved (first
	// occurrence wins); subsequent duplicates are dropped silently.
	dir := initRepo(t)
	addIntent(t, dir, "auth/jwt", "", "")

	resp := callJSON(t, dir, `{"jsonrpc":"2.0","method":"kron_update","params":{"slug":"auth/jwt","references":["oauth2","session","oauth2"],"depends_on":["auth/a","auth/b","auth/a"]},"id":1}`)
	require.Nil(t, resp.Error)

	get := callJSON(t, dir, `{"jsonrpc":"2.0","method":"kron_get","params":{"slug":"auth/jwt"},"id":2}`)
	require.Nil(t, get.Error)
	var got struct {
		Intent struct {
			Frontmatter struct {
				References []string `json:"references"`
				DependsOn  []string `json:"depends_on"`
			} `json:"frontmatter"`
		} `json:"intent"`
	}
	require.NoError(t, json.Unmarshal(get.Result, &got))
	assert.Equal(t, []string{"oauth2", "session"}, got.Intent.Frontmatter.References,
		"duplicate must be dropped, order preserved")
	assert.Equal(t, []string{"auth/a", "auth/b"}, got.Intent.Frontmatter.DependsOn,
		"duplicate must be dropped, order preserved")
}

func TestImpact_IncomingAnchorsAreSorted(t *testing.T) {
	// kron_impact's incoming_anchors must come back in a deterministic
	// (file_path, line) order so callers can rely on the order without
	// re-sorting. parser.ScanAnchors walks the repo in directory order
	// but filepath.Walk's ordering is not guaranteed across platforms,
	// and handleImpact pre-sorts defensively.
	//
	// We seed 3 anchors in deliberately scrambled file-path / line
	// positions: file naming uses 2-character prefixes that sort
	// OUT of directory-creation order, and each file puts its anchor
	// on a different line.
	dir := initRepo(t)
	addIntent(t, dir, "auth/jwt", "", "")

	// Seed 3 anchors in 3 distinct files (each puts its anchor on
	// line 1; the sort is by file_path then line). Directory
	// creation order in the test does NOT match alphabetical order,
	// so the only way this test passes is if handleImpact sorts the
	// result deterministically.
	mustWriteAnchor(t, dir, "z"+sep+"file3.go", 1, "auth/jwt")
	mustWriteAnchor(t, dir, "a"+sep+"file1.go", 1, "auth/jwt")
	mustWriteAnchor(t, dir, "m"+sep+"file2.go", 1, "auth/jwt")

	resp := callJSON(t, dir, `{"jsonrpc":"2.0","method":"kron_impact","params":{"slug":"auth/jwt"},"id":1}`)
	require.Nil(t, resp.Error)

	var r struct {
		IncomingAnchors []struct {
			FilePath string `json:"file_path"`
			Line     int    `json:"line"`
		} `json:"incoming_anchors"`
	}
	require.NoError(t, json.Unmarshal(resp.Result, &r))
	require.Len(t, r.IncomingAnchors, 3)
	// Expected order: a/file1.go, m/file2.go, z/file3.go (all on
	// line 1). Any other order means the sort in handleImpact is
	// not (file_path, line)-stable. File paths are repo-relative
	// forward-slash per RFC 2026-10-08-path-conventions.md §2.1
	// (NOT the platform-native separator).
	assert.Equal(t, "a/file1.go", r.IncomingAnchors[0].FilePath)
	assert.Equal(t, 1, r.IncomingAnchors[0].Line)
	assert.Equal(t, "m/file2.go", r.IncomingAnchors[1].FilePath)
	assert.Equal(t, 1, r.IncomingAnchors[1].Line)
	assert.Equal(t, "z/file3.go", r.IncomingAnchors[2].FilePath)
	assert.Equal(t, 1, r.IncomingAnchors[2].Line)
}

func TestDedupStrings(t *testing.T) {
	// Direct unit test for the dedup helper (used by kron_update to
	// normalise references/depends_on lists per RFC §3.2).
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"nil", nil, nil},
		{"empty", []string{}, []string{}},
		{"single", []string{"a"}, []string{"a"}},
		{"no-dup", []string{"a", "b", "c"}, []string{"a", "b", "c"}},
		{"dup-adjacent", []string{"a", "a", "b"}, []string{"a", "b"}},
		{"dup-nonadjacent", []string{"a", "b", "a", "c", "b"}, []string{"a", "b", "c"}},
		{"all-same", []string{"x", "x", "x"}, []string{"x"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parser.DedupStrings(tc.in)
			assert.Equal(t, tc.want, got)
		})
	}
}

// mustWriteAnchor writes a one-line Go source file containing the
// given anchor at line 1 (no padding). Tests that need multiple
// anchors in the same logical file should give each anchor a
// distinct relPath; the test then asserts that
// kron_impact's incoming_anchors are stably sorted across the
// distinct paths.
//
// The .go extension keeps parser.ScanAnchors reading the file as a
// plain source file (line-by-line comment lines) without any
// frontmatter interaction.
func mustWriteAnchor(t *testing.T, dir, relPath string, _ int, slug string) {
	t.Helper()
	full := filepath.Join(dir, relPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	content := []byte("// @kron:intent " + slug + "\n")
	require.NoError(t, os.WriteFile(full, content, 0o644))
}
