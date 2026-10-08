// Package relations computes the cross-intent link relationships
// exposed by MCP `kron_impact` (reverse-link view) and `kron_delete`
// (dependents warning). The two views share enough graph traversal
// logic that centralising it in one package eliminates the prior
// "MCP delete computes depends_on dependents; MCP impact computes
// references + depends_on + symbol-inferred" inconsistency.
//
// relations operates on a pre-loaded []*model.Intent slice (typically
// the result of store.Reader.LoadAll). It does no I/O and does not
// resolve on-disk paths. Pass a fully-loaded slice in; get sorted
// string slices out.
//
// All returned slices are sorted lexicographically for stable
// downstream output (lint messages, MCP response, IDE hover).
package relations

import (
	"sort"

	"github.com/xxx/kron/internal/model"
)

// ReverseLinks returns the three reverse views used by kron_impact:
//
//   - References: slugs whose Frontmatter.References list contains
//     target.Slug (soft-link reverse). Excludes target itself.
//   - DependsOnDependents: slugs whose Frontmatter.DependsOn list
//     contains target.Slug (hard-link reverse). Excludes target itself.
//   - SymbolInferred: slugs whose Frontmatter.Symbol set intersects
//     target.Frontmatter.Symbol (inferred soft dependency). Excludes
//     target itself AND any slug already covered by DependsOnDependents
//     (the "explicit wins" rule from impact's behaviour).
//
// All three slices are sorted. Returns empty (non-nil) slices when
// target has no relations. The function does not validate that
// target itself exists in intents; passing a target that is not in
// intents is fine — the returned views just describe the reverse graph
// as if it did.
func ReverseLinks(intents []*model.Intent, targetSlug string) (references, dependsOnDependents, symbolInferred []string) {
	references = []string{}
	dependsOnDependents = []string{}
	symbolInferred = []string{}

	// Build the set of explicit dependents (DependsOnDependents) so
	// we can later exclude them from the symbol-inferred set per the
	// "explicit wins" rule.
	explicitSet := make(map[string]struct{})

	// Find target's symbol set independently of the per-intent loop
	// below (the loop is gated on `in.Slug == targetSlug` continue,
	// which would skip building the set).
	var targetSyms map[string]struct{}
	for _, in := range intents {
		if in.Slug == targetSlug {
			targetSyms = make(map[string]struct{}, len(in.Frontmatter.Symbol))
			for _, s := range in.Frontmatter.Symbol {
				targetSyms[s] = struct{}{}
			}
			break
		}
	}

	for _, in := range intents {
		if in.Slug == targetSlug {
			continue
		}
		for _, ref := range in.Frontmatter.References {
			if ref == targetSlug {
				references = append(references, in.Slug)
				break // one reference is enough; don't list the same slug twice
			}
		}
		for _, dep := range in.Frontmatter.DependsOn {
			if dep == targetSlug {
				dependsOnDependents = append(dependsOnDependents, in.Slug)
				explicitSet[in.Slug] = struct{}{}
				break
			}
		}
	}

	// Symbol-inferred: any other intent whose symbol set intersects
	// target's symbol set, MINUS any slug already in explicitSet.
	if len(targetSyms) > 0 {
		for _, in := range intents {
			if in.Slug == targetSlug {
				continue
			}
			if _, covered := explicitSet[in.Slug]; covered {
				continue
			}
			for _, s := range in.Frontmatter.Symbol {
				if _, ok := targetSyms[s]; ok {
					symbolInferred = append(symbolInferred, in.Slug)
					break
				}
			}
		}
	}

	sort.Strings(references)
	sort.Strings(dependsOnDependents)
	sort.Strings(symbolInferred)
	return references, dependsOnDependents, symbolInferred
}

// Dependents returns the slugs of every intent that lists targetSlug
// in Frontmatter.DependsOn. Excludes target itself. Returned slice is
// sorted.
//
// This is the view surfaced in MCP `kron_delete`'s "dependents" field
// (soft warning, not a block). It is intentionally narrower than
// relations.ReverseLinks: delete only cares about hard-dependency
// dependents because deleting an intent that someone *references*
// (soft) is much less risky than deleting one someone *depends on*
// (hard).
func Dependents(intents []*model.Intent, targetSlug string) []string {
	out := []string{}
	for _, in := range intents {
		if in.Slug == targetSlug {
			continue
		}
		for _, dep := range in.Frontmatter.DependsOn {
			if dep == targetSlug {
				out = append(out, in.Slug)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

// ReferencingIntents returns the sorted slugs of every intent that
// has the given assumptionID in its Frontmatter.Assumptions[].ID
// list. Excludes the target itself (an intent never references
// itself in B-3). The targetSlug is unused but kept in the signature
// for symmetry with the rest of the package — callers can pass the
// intent's own slug or any other anchor.
//
// This is the reverse view that MCP `kron_assumption_delete` (and
// future "what depends on this assumption" features) will need.
// It complements lint.RuleAssumptionOrphan (which flags the FORWARD
// direction: assumptions registered but never used). Lint and this
// function answer different questions, both useful:
//
//	lint.RuleAssumptionOrphan: "this assumption file is unused"
//	ReferencingIntents:         "which intents use this assumption"
//
// Note: this is in-memory only. The relations package is documented
// as "no I/O, no on-disk paths"; loading is the caller's job.
func ReferencingIntents(intents []*model.Intent, assumptionID string) []string {
	out := []string{}
	for _, in := range intents {
		for _, a := range in.Frontmatter.Assumptions {
			if a.ID == assumptionID {
				out = append(out, in.Slug)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

// Prerequisites returns the sorted union of:
//
//   - target.Frontmatter.DependsOn (explicit, hard)
//   - symbol-inferred slugs (any intent whose Symbol set intersects
//     target's Symbol set), MINUS any slug already in the explicit set
//     (explicit wins, matches ReverseLinks's behaviour)
//
// The returned slice is the "prerequisites" view exposed by MCP
// `kron_impact`. It is the same computation as the (explicit ∪
// symbolInferred) union inside ReverseLinks, lifted to its own
// function so callers that only need prereqs (and not the reverse
// views) don't pay for the reverse-link walks.
func Prerequisites(intents []*model.Intent, targetSlug string) []string {
	// Find the target once.
	var target *model.Intent
	for _, in := range intents {
		if in.Slug == targetSlug {
			target = in
			break
		}
	}
	if target == nil {
		return []string{}
	}

	explicitSet := make(map[string]struct{}, len(target.Frontmatter.DependsOn))
	for _, d := range target.Frontmatter.DependsOn {
		explicitSet[d] = struct{}{}
	}

	merged := make(map[string]struct{}, len(explicitSet))
	for d := range explicitSet {
		merged[d] = struct{}{}
	}

	// Symbol-inferred (only when target has at least one symbol).
	targetSyms := make(map[string]struct{}, len(target.Frontmatter.Symbol))
	for _, s := range target.Frontmatter.Symbol {
		targetSyms[s] = struct{}{}
	}
	if len(targetSyms) > 0 {
		for _, in := range intents {
			if in.Slug == targetSlug {
				continue
			}
			if _, covered := explicitSet[in.Slug]; covered {
				continue
			}
			for _, s := range in.Frontmatter.Symbol {
				if _, ok := targetSyms[s]; ok {
					merged[in.Slug] = struct{}{}
					break
				}
			}
		}
	}

	out := make([]string, 0, len(merged))
	for s := range merged {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
