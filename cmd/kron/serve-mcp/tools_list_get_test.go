package servemcp

import (
	"reflect"
	"testing"
)

// TestMergeDependents exercises the C-7 helper that unions
// hard and symbol-inferred dependents. Edge cases:
//   - both nil → nil (so `omitempty` keeps the field out of the wire)
//   - one empty → returns the other
//   - overlap → deduped (single occurrence)
//   - unsorted inputs → sorted output
//   - duplicates within a single input → deduped
func TestMergeDependents(t *testing.T) {
	cases := []struct {
		name     string
		hard     []string
		inferred []string
		want     []string
	}{
		{
			name: "both empty",
			want: nil,
		},
		{
			name: "hard only",
			hard: []string{"b", "a"},
			want: []string{"a", "b"},
		},
		{
			name:     "inferred only",
			inferred: []string{"c", "a"},
			want:     []string{"a", "c"},
		},
		{
			name:     "no overlap, unsorted",
			hard:     []string{"z", "a"},
			inferred: []string{"m"},
			want:     []string{"a", "m", "z"},
		},
		{
			name:     "full overlap",
			hard:     []string{"a", "b"},
			inferred: []string{"a", "b"},
			want:     []string{"a", "b"},
		},
		{
			name:     "partial overlap",
			hard:     []string{"a", "c"},
			inferred: []string{"b", "c"},
			want:     []string{"a", "b", "c"},
		},
		{
			name:     "duplicates within hard",
			hard:     []string{"a", "a", "b"},
			inferred: []string{"c"},
			want:     []string{"a", "b", "c"},
		},
		{
			name:     "duplicates within inferred",
			hard:     []string{"a"},
			inferred: []string{"b", "b", "c"},
			want:     []string{"a", "b", "c"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := mergeDependents(c.hard, c.inferred)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("mergeDependents(%v, %v) = %v, want %v",
					c.hard, c.inferred, got, c.want)
			}
		})
	}
}
