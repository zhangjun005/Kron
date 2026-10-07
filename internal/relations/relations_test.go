package relations

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xxx/kron/internal/model"
)

func mkIntent(slug string, syms, refs, deps []string) *model.Intent {
	return &model.Intent{
		Slug: slug,
		Frontmatter: model.Frontmatter{
			CreatedBy: "@a",
			UpdatedAt: time.Now(),
			Symbol:    syms,
			References: refs,
			DependsOn:  deps,
		},
	}
}

func TestReverseLinks_Empty(t *testing.T) {
	refs, deps, sym := ReverseLinks(nil, "x")
	assert.Equal(t, []string{}, refs)
	assert.Equal(t, []string{}, deps)
	assert.Equal(t, []string{}, sym)
}

func TestReverseLinks_SelfExcluded(t *testing.T) {
	// An intent that references itself in Frontmatter.References does
	// not count as a reverse-link from itself.
	a := mkIntent("a", nil, []string{"a"}, []string{"a"})
	refs, deps, sym := ReverseLinks([]*model.Intent{a}, "a")
	assert.Empty(t, refs)
	assert.Empty(t, deps)
	assert.Empty(t, sym)
}

func TestReverseLinks_References(t *testing.T) {
	a := mkIntent("a", nil, []string{"b"}, nil)
	b := mkIntent("b", nil, nil, nil)
	c := mkIntent("c", nil, []string{"b", "b", "b"}, nil) // duplicate refs dedup'd to one hit
	refs, _, _ := ReverseLinks([]*model.Intent{a, b, c}, "b")
	assert.Equal(t, []string{"a", "c"}, refs, "slugs whose References list contains b")
}

func TestReverseLinks_DependsOn(t *testing.T) {
	a := mkIntent("a", nil, nil, []string{"b"})
	b := mkIntent("b", nil, nil, nil)
	_, deps, _ := ReverseLinks([]*model.Intent{a, b}, "b")
	assert.Equal(t, []string{"a"}, deps)
}

func TestReverseLinks_SymbolInferred_ExplicitWins(t *testing.T) {
	// Reverse-links view: which OTHER intents list target in their
	// DependsOn. The "explicit wins" rule means a slug already in
	// dependsOnDependents must not be repeated in symbolInferred.
	//
	// Concretely:
	//   - a.DependsOn = [target]  →  a is a hard-link reverse (in deps)
	//   - a also shares symbol "X" with target
	//   - b shares symbol "X" with target but does NOT depend_on target
	// Expected: deps=[a], sym=[b] (a wins via explicit, b contributes via symbol)
	target := mkIntent("target", []string{"X"}, nil, nil)
	a := mkIntent("a", []string{"X"}, nil, []string{"target"})
	b := mkIntent("b", []string{"X"}, nil, nil)
	_, deps, sym := ReverseLinks([]*model.Intent{target, a, b}, "target")
	assert.Equal(t, []string{"a"}, deps)
	assert.Equal(t, []string{"b"}, sym, "a is already in explicit deps, must not appear in symbol-inferred")
}

func TestReverseLinks_TargetNotInInput(t *testing.T) {
	// Passing a target not in intents is allowed; returns empty for
	// symbol-inferred (no target to read symbols from) and reverse
	// walks proceed normally.
	a := mkIntent("a", nil, []string{"ghost"}, nil)
	refs, _, sym := ReverseLinks([]*model.Intent{a}, "ghost")
	assert.Equal(t, []string{"a"}, refs)
	assert.Empty(t, sym)
}

func TestDependents_HardOnly(t *testing.T) {
	// Dependents ignores references; only DependsOn is checked.
	a := mkIntent("a", nil, []string{"b"}, nil)
	c := mkIntent("c", nil, nil, []string{"b"})
	b := mkIntent("b", nil, nil, nil)
	got := Dependents([]*model.Intent{a, b, c}, "b")
	assert.Equal(t, []string{"c"}, got)
}

func TestDependents_Sorted(t *testing.T) {
	// Insertion in non-sorted order; output must be sorted.
	c := mkIntent("c", nil, nil, []string{"x"})
	a := mkIntent("a", nil, nil, []string{"x"})
	b := mkIntent("b", nil, nil, []string{"x"})
	got := Dependents([]*model.Intent{c, a, b}, "x")
	assert.Equal(t, []string{"a", "b", "c"}, got)
}

func TestPrerequisites_ExplicitOnly(t *testing.T) {
	target := mkIntent("target", nil, nil, []string{"b", "a"})
	b := mkIntent("b", nil, nil, nil)
	a := mkIntent("a", nil, nil, nil)
	got := Prerequisites([]*model.Intent{target, a, b}, "target")
	assert.Equal(t, []string{"a", "b"}, got)
}

func TestPrerequisites_ExplicitPlusSymbolInferred(t *testing.T) {
	target := mkIntent("target", []string{"S"}, []string{}, []string{"explicit"})
	explicit := mkIntent("explicit", nil, nil, nil)
	inferred := mkIntent("inferred", []string{"S"}, nil, nil)
	other := mkIntent("other", []string{"T"}, nil, nil)
	got := Prerequisites([]*model.Intent{target, explicit, inferred, other}, "target")
	assert.Equal(t, []string{"explicit", "inferred"}, got)
}

func TestPrerequisites_TargetNotFound(t *testing.T) {
	got := Prerequisites([]*model.Intent{mkIntent("a", nil, nil, nil)}, "ghost")
	require.NotNil(t, got)
	assert.Empty(t, got)
}

func TestPrerequisites_NoSymbolsNoInferred(t *testing.T) {
	target := mkIntent("target", nil, nil, []string{"b"})
	b := mkIntent("b", nil, nil, nil)
	sameSym := mkIntent("same-sym", []string{"X"}, nil, nil) // not relevant; target has no symbols
	got := Prerequisites([]*model.Intent{target, b, sameSym}, "target")
	assert.Equal(t, []string{"b"}, got)
}
