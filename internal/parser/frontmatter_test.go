package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xxx/kron/internal/model"
)

func TestSplitMarkdown_Empty(t *testing.T) {
	fm, body, err := SplitMarkdown(nil)
	require.NoError(t, err)
	assert.Empty(t, fm)
	assert.Empty(t, body)
}

func TestSplitMarkdown_NoFence(t *testing.T) {
	_, _, err := SplitMarkdown([]byte("just a body, no fence\n"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "opening fence")
}

func TestSplitMarkdown_Unterminated(t *testing.T) {
	_, _, err := SplitMarkdown([]byte("---\ncreated_by: \"@a\"\n"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing closing")
}

func TestSplitMarkdown_HappyPath(t *testing.T) {
	in := "---\ncreated_by: \"@a\"\nupdated_at: 2026-10-01T10:00:00Z\n---\n# Title\n\nBody.\n"
	fm, body, err := SplitMarkdown([]byte(in))
	require.NoError(t, err)
	assert.Equal(t, "created_by: \"@a\"\nupdated_at: 2026-10-01T10:00:00Z\n", string(fm))
	assert.Equal(t, "# Title\n\nBody.\n", body)
}

func TestSplitMarkdown_EmptyBody(t *testing.T) {
	in := "---\ncreated_by: \"@a\"\n---\n"
	fm, body, err := SplitMarkdown([]byte(in))
	require.NoError(t, err)
	assert.Contains(t, string(fm), "created_by")
	assert.Empty(t, body)
}

// Sentinel-format tests (the preferred v1 on-disk form).

func TestSplitMarkdown_Sentinel_HappyPath(t *testing.T) {
	in := "<!-- kron:frontmatter -->\ncreated_by: \"@a\"\nupdated_at: 2026-10-01T10:00:00Z\n<!-- /kron:frontmatter -->\n# Title\n\nBody.\n"
	fm, body, err := SplitMarkdown([]byte(in))
	require.NoError(t, err)
	assert.Equal(t, "created_by: \"@a\"\nupdated_at: 2026-10-01T10:00:00Z\n", string(fm))
	assert.Equal(t, "# Title\n\nBody.\n", body)
}

func TestSplitMarkdown_Sentinel_EmptyBody(t *testing.T) {
	in := "<!-- kron:frontmatter -->\ncreated_by: \"@a\"\n<!-- /kron:frontmatter -->\n"
	fm, body, err := SplitMarkdown([]byte(in))
	require.NoError(t, err)
	assert.Contains(t, string(fm), "created_by")
	assert.Empty(t, body)
}

func TestSplitMarkdown_Sentinel_CRLF(t *testing.T) {
	in := "<!-- kron:frontmatter -->\r\ncreated_by: \"@a\"\r\n<!-- /kron:frontmatter -->\r\n# Title\n"
	fm, body, err := SplitMarkdown([]byte(in))
	require.NoError(t, err)
	assert.Contains(t, string(fm), "created_by")
	assert.Equal(t, "# Title\n", body)
}

func TestSplitMarkdown_Sentinel_Unterminated(t *testing.T) {
	// Opening sentinel without closing → falls through to legacy parser,
	// which reports an error that mentions both the legacy fence and the
	// sentinel form (so users get a single, useful error regardless of
	// which syntax they started writing).
	in := "<!-- kron:frontmatter -->\ncreated_by: \"@a\"\n"
	_, _, err := SplitMarkdown([]byte(in))
	require.Error(t, err)
	// The error should mention the sentinel form so the user is pointed
	// at the right hint, not just the legacy "---" form.
	assert.Contains(t, err.Error(), "kron:frontmatter")
}

func TestSplitMarkdown_BodyWithHorizontalRule(t *testing.T) {
	// Body contains a "---" horizontal rule. With sentinel wrapping it
	// MUST be ignored (this is the whole point of the migration).
	in := "<!-- kron:frontmatter -->\ncreated_by: \"@a\"\n<!-- /kron:frontmatter -->\n# Title\n\n---\n\n## After HR\n"
	fm, body, err := SplitMarkdown([]byte(in))
	require.NoError(t, err)
	assert.Contains(t, string(fm), "created_by")
	assert.Contains(t, body, "---")
	assert.Contains(t, body, "## After HR")
}

func TestSplitMarkdown_Sentinel_MustBeFirstLine(t *testing.T) {
	in := "preamble\n<!-- kron:frontmatter -->\ncreated_by: \"@a\"\n<!-- /kron:frontmatter -->\n"
	// No "---" anywhere → falls through to legacy parser → "opening fence" error.
	_, _, err := SplitMarkdown([]byte(in))
	require.Error(t, err)
}

func TestParseFrontmatter_Required(t *testing.T) {
	raw := []byte("created_by: \"@a\"\nupdated_at: 2026-10-01T10:00:00Z\nstatus: active\n")
	fm, err := ParseFrontmatter(raw)
	require.NoError(t, err)
	assert.Equal(t, "@a", fm.CreatedBy)
	assert.Equal(t, "2026-10-01T10:00:00Z", fm.UpdatedAt.Format(time.RFC3339))
	assert.Equal(t, "active", string(fm.Status))
}

func TestParseFrontmatter_Empty(t *testing.T) {
	_, err := ParseFrontmatter([]byte(""))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestParseFrontmatter_RejectsUnknownField(t *testing.T) {
	raw := []byte("created_by: \"@a\"\nupdated_at: 2026-10-01T10:00:00Z\nmade_up_field: oops\n")
	_, err := ParseFrontmatter(raw)
	require.Error(t, err)
}

func TestParseFrontmatter_RejectsBadStatus(t *testing.T) {
	raw := []byte("created_by: \"@a\"\nupdated_at: 2026-10-01T10:00:00Z\nstatus: drafty\n")
	_, err := ParseFrontmatter(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status")
}

func TestParseFrontmatter_RejectsMissingCreatedBy(t *testing.T) {
	raw := []byte("updated_at: 2026-10-01T10:00:00Z\n")
	_, err := ParseFrontmatter(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "created_by")
}

func TestParseFrontmatter_AcceptsAssumptions(t *testing.T) {
	raw := []byte(`created_by: "@a"
updated_at: 2026-10-01T10:00:00Z
assumptions:
  - id: single-region
    text: single region deployment
    severity: hard
`)
	fm, err := ParseFrontmatter(raw)
	require.NoError(t, err)
	require.Len(t, fm.Assumptions, 1)
	assert.Equal(t, "single-region", fm.Assumptions[0].ID)
	assert.Equal(t, "hard", string(fm.Assumptions[0].Severity))
}

func TestParseFrontmatter_RejectsAssumptionMissingSeverity(t *testing.T) {
	raw := []byte(`created_by: "@a"
updated_at: 2026-10-01T10:00:00Z
assumptions:
  - id: x
    text: y
`)
	_, err := ParseFrontmatter(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "severity")
}

// B-3 (RFC 2026-10-08-assumptions-standalone): assumption IDs are file
// slugs, not intent slugs — they MUST NOT contain "/" because the
// on-disk file is .kron/assumptions/<id>.md (flat directory).

func TestParseFrontmatter_RejectsAssumptionIdWithSlash(t *testing.T) {
	raw := []byte(`created_by: "@a"
updated_at: 2026-10-01T10:00:00Z
assumptions:
  - id: "auth/region"
    severity: hard
    rationale: "this is a long enough rationale string"
`)
	_, err := ParseFrontmatter(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "/")
}

func TestParseFrontmatter_AcceptsAssumptionWithRationale(t *testing.T) {
	raw := []byte(`created_by: "@a"
updated_at: 2026-10-01T10:00:00Z
assumptions:
  - id: single-region
    severity: hard
    rationale: "跨 region 时 token 失效爆炸, 必须保证单 region 部署"
`)
	fm, err := ParseFrontmatter(raw)
	require.NoError(t, err)
	require.Len(t, fm.Assumptions, 1)
	assert.Equal(t, "single-region", fm.Assumptions[0].ID)
	assert.Equal(t, "跨 region 时 token 失效爆炸, 必须保证单 region 部署", fm.Assumptions[0].Rationale)
}

func TestParseFrontmatter_AcceptsLegacyStructForMigration(t *testing.T) {
	// A-path (v1.1) form: full struct with text/severity/expires_at,
	// no rationale. v1.3 accepts this (parse-level), lint surfaces
	// RuleAssumptionRationaleRequired as a Warning.
	raw := []byte(`created_by: "@a"
updated_at: 2026-10-01T10:00:00Z
assumptions:
  - id: single-region
    text: 服务仅部署在单 region
    severity: hard
    expires_at: "2026-12-31T00:00:00Z"
`)
	fm, err := ParseFrontmatter(raw)
	require.NoError(t, err)
	require.Len(t, fm.Assumptions, 1)
	assert.Empty(t, fm.Assumptions[0].Rationale, "empty rationale permitted at parse time (lint surfaces as Warning)")
	assert.Equal(t, "服务仅部署在单 region", fm.Assumptions[0].Text)
	assert.Equal(t, "2026-12-31T00:00:00Z", fm.Assumptions[0].ExpiresAt)
}

func TestParseFrontmatter_RejectsRationaleTooShort(t *testing.T) {
	raw := []byte(`created_by: "@a"
updated_at: 2026-10-01T10:00:00Z
assumptions:
  - id: single-region
    severity: hard
    rationale: "hard"
`)
	_, err := ParseFrontmatter(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rationale")
}

func TestParseFrontmatter_RejectsIdEmpty(t *testing.T) {
	raw := []byte(`created_by: "@a"
updated_at: 2026-10-01T10:00:00Z
assumptions:
  - id: ""
    severity: hard
    rationale: "this is a long enough rationale string"
`)
	_, err := ParseFrontmatter(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "id")
}

func TestValidateSlug_TableDriven(t *testing.T) {
	cases := []struct {
		slug     string
		wantErr  bool
		contains string
	}{
		{slug: "auth/jwt-sliding-window", wantErr: false},
		{slug: "single-region", wantErr: false},
		{slug: "abc_123", wantErr: false},
		{slug: "a/b/c/d", wantErr: false},
		{slug: "", wantErr: true, contains: "empty"},
		{slug: "Auth", wantErr: true, contains: "invalid characters"},
		{slug: "auth/jwt.md", wantErr: true, contains: ".md"},
		{slug: "../etc/passwd", wantErr: true, contains: ".."},
		{slug: "auth\\jwt", wantErr: true, contains: "backslash"},
		{slug: "auth/", wantErr: true, contains: "empty path segment"},
		{slug: "/auth", wantErr: true, contains: "empty path segment"},
		{slug: "auth//jwt", wantErr: true, contains: "empty path segment"},
		{slug: "-leading", wantErr: true, contains: "invalid characters"},
	}
	for _, tc := range cases {
		t.Run("slug="+tc.slug, func(t *testing.T) {
			err := ValidateSlug(tc.slug)
			if tc.wantErr {
				require.Error(t, err)
				if tc.contains != "" {
					assert.True(t, strings.Contains(err.Error(), tc.contains),
						"error %q should contain %q", err.Error(), tc.contains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSerializeMarkdown_RoundTrip(t *testing.T) {
	raw := "---\ncreated_by: \"@a\"\nupdated_at: 2026-10-01T10:00:00Z\nstatus: active\n---\n# Title\n\nBody line.\n"
	fm, body, err := SplitMarkdown([]byte(raw))
	require.NoError(t, err)

	parsed, err := ParseFrontmatter(fm)
	require.NoError(t, err)

	intent := &model.Intent{Frontmatter: parsed, Body: body}
	out, err := SerializeMarkdown(intent)
	require.NoError(t, err)

	// Re-parse the output and ensure it matches the input intent.
	fm2, body2, err := SplitMarkdown(out)
	require.NoError(t, err)
	parsed2, err := ParseFrontmatter(fm2)
	require.NoError(t, err)

	assert.Equal(t, parsed.CreatedBy, parsed2.CreatedBy)
	assert.Equal(t, parsed.UpdatedAt.Format(time.RFC3339), parsed2.UpdatedAt.Format(time.RFC3339))
	assert.Equal(t, string(parsed.Status), string(parsed2.Status))
	assert.Equal(t, body, body2)
}

func TestValidateFrontmatter_HappyPath(t *testing.T) {
	// A well-formed Frontmatter round-trips through ValidateFrontmatter
	// without error.
	fm := &model.Frontmatter{
		CreatedBy: "@a",
		UpdatedAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		Status:    model.StatusActive,
	}
	require.NoError(t, ValidateFrontmatter(fm))
}

func TestValidateFrontmatter_MissingRequired(t *testing.T) {
	// CreatedBy is required; an empty CreatedBy is rejected.
	fm := &model.Frontmatter{
		UpdatedAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
	}
	err := ValidateFrontmatter(fm)
	require.Error(t, err)
	// The marshal→parse round-trip surfaces the same error
	// ParseFrontmatter would have produced (unwrapped string).
	// It is intentionally NOT wrapped with model.ErrFrontmatterInvalid
	// at this layer — the caller (e.g. kron_update) is expected to
	// add the model.ErrFrontmatterInvalid wrap when surfacing to
	// users, matching how the load path already wraps with %w.
	assert.Contains(t, err.Error(), "created_by")
}

func TestValidateFrontmatter_BadStatus(t *testing.T) {
	// Status enum is validated; unknown values are rejected.
	fm := &model.Frontmatter{
		CreatedBy: "@a",
		UpdatedAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		Status:    model.Status("drafty"),
	}
	err := ValidateFrontmatter(fm)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status")
}

func TestValidateFrontmatter_NilPointer(t *testing.T) {
	require.Error(t, ValidateFrontmatter(nil))
}

func TestSlugsForFile_HappyPath(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.go")
	require.NoError(t, os.WriteFile(p, []byte(`package x
// @kron:intent auth/jwt
// @kron:intent auth/refresh
// @kron:intent auth/jwt  // duplicate, dedup
`), 0o644))
	slugs, err := SlugsForFile(p)
	require.NoError(t, err)
	assert.Equal(t, []string{"auth/jwt", "auth/refresh"}, slugs)
}

func TestSlugsForFile_NoAnchors(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.go")
	require.NoError(t, os.WriteFile(p, []byte("package x\n"), 0o644))
	slugs, err := SlugsForFile(p)
	require.NoError(t, err)
	assert.Empty(t, slugs)
}

func TestSlugsForFile_EmptyPath(t *testing.T) {
	_, err := SlugsForFile("")
	require.Error(t, err)
}

func TestSlugsForFile_MissingFile(t *testing.T) {
	_, err := SlugsForFile(filepath.Join(t.TempDir(), "nope"))
	require.Error(t, err)
}
