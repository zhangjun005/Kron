package model

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntentKind_Valid(t *testing.T) {
	tests := []struct {
		name string
		give IntentKind
		want bool
	}{
		{name: "leaf is valid", give: IntentKindLeaf, want: true},
		{name: "node is valid", give: IntentKindNode, want: true},
		{name: "empty is valid (treated as leaf)", give: IntentKind(""), want: true},
		{name: "garbage is invalid", give: IntentKind("bogus"), want: false},
		{name: "uppercase is invalid (case-sensitive)", give: IntentKind("LEAF"), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.give.Valid())
		})
	}
}

// TestIntent_KindNotInFrontmatter locks the RFC 2026-10-08-writer-readme-symmetry
// decision: Kind is a write-routing decision and MUST NOT appear in
// frontmatter. If a future change drops the yaml:"-" tag, this test
// fails — a reviewer has to update both the test and the RFC.
func TestIntent_KindNotInFrontmatter(t *testing.T) {
	// The struct tag inspection is the actual contract test: the
	// field declaration in intent.go must be `Kind IntentKind \`yaml:"-"\``.
	// We verify by re-declaring the field shape in a reflect lookup
	// so a rename of the field also fails the test.
	intentType := reflect.TypeOf(Intent{})
	field, ok := intentType.FieldByName("Kind")
	require.True(t, ok, "Intent must have a Kind field (see RFC §3.2)")

	tag := field.Tag.Get("yaml")
	assert.Equal(t, "-", tag, "Intent.Kind must be yaml:\"-\" so it never reaches frontmatter")

	// Sanity: the field type is the right kind.
	assert.Equal(t, reflect.TypeOf(IntentKind("")), field.Type)
}
