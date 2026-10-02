package parser

import (
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
