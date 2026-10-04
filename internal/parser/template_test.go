package parser

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIntentBodyTemplate_TitleFromSlug(t *testing.T) {
	cases := []struct {
		slug     string
		wantLine string
	}{
		{"auth/jwt-sliding-window", "# jwt sliding window"},
		{"single-region", "# single region"},
		{"a/b/c/d", "# d"},
		{"plain", "# plain"},
	}
	for _, tc := range cases {
		t.Run(tc.slug, func(t *testing.T) {
			got := IntentBodyTemplate(tc.slug)
			assert.True(t, strings.HasPrefix(got, tc.wantLine),
				"expected title line %q at start of:\n%s", tc.wantLine, got)
		})
	}
}

func TestIntentBodyTemplate_HasSkeleton(t *testing.T) {
	body := IntentBodyTemplate("auth/jwt")
	// Required sections per docs/abstractDesign/intent-structure.md §三.
	assert.Contains(t, body, "## Why")
	assert.Contains(t, body, "## Trade-offs")
	// Assumptions pointer (the inline HTML comment that points
	// readers at the frontmatter's assumptions[] field).
	assert.Contains(t, body, "assumptions")
}

func TestIntentBodyTemplateCN_HasChineseSections(t *testing.T) {
	body := IntentBodyTemplateCN("auth/jwt")
	assert.Contains(t, body, "## 为什么（Why）")
	assert.Contains(t, body, "## 权衡（Trade-offs）")
}

func TestIntentBodyTemplate_EmptySlug(t *testing.T) {
	// Edge case: empty slug still produces a valid body with "# "
	// as the title. Callers are expected to validate the slug
	// before calling (parser.ValidateSlug), so this is just
	// "doesn't crash" defensive coverage.
	got := IntentBodyTemplate("")
	assert.Contains(t, got, "# ")
}
