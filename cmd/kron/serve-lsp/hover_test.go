package lsp

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

func TestSummarizeBody_Empty(t *testing.T) {
	assert.Equal(t, "", summarizeBody(""))
	assert.Equal(t, "", summarizeBody("   \n  \n"))
}

func TestSummarizeBody_Short(t *testing.T) {
	assert.Equal(t, "hello world", summarizeBody("hello world"))
}

func TestSummarizeBody_FirstParagraph(t *testing.T) {
	body := "first paragraph\n\nsecond paragraph that should be dropped"
	assert.Equal(t, "first paragraph", summarizeBody(body))
}

func TestSummarizeBody_TruncatesLongUTF8(t *testing.T) {
	// 300 CJK chars, 3 bytes each → 900 bytes, far past maxLen=200.
	// Truncation must land on a rune boundary so the result stays
	// valid UTF-8 (a byte-slice would emit mojibake).
	long := strings.Repeat("界", 300)
	got := summarizeBody(long)

	assert.True(t, utf8.ValidString(got), "truncated body must be valid UTF-8")
	assert.Equal(t, 201, utf8.RuneCountInString(got), "200 chars + ellipsis")
	assert.True(t, strings.HasSuffix(got, "…"), "truncated body must end with ellipsis")
}

func TestSummarizeBody_ASCIIBoundary(t *testing.T) {
	// Exactly maxLen runes: must NOT be truncated (no ellipsis added).
	exact := strings.Repeat("a", 200)
	assert.Equal(t, exact, summarizeBody(exact))

	// One over: must be truncated with ellipsis.
	over := strings.Repeat("a", 201)
	assert.Equal(t, strings.Repeat("a", 200)+"…", summarizeBody(over))
}
