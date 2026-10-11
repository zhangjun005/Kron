package servemcp

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/xxx/kron/internal/assumption"
	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/parser"
	"github.com/xxx/kron/internal/relations"
	"github.com/xxx/kron/internal/store"
)

// HandleImpact returns the mcp.ToolHandlerFor binding for kron_impact.
//
// Wire contract: see docs/implementation/mcp.md §2 (kron_impact).
//
//	in:  { slug: string }
//	out: {
//	  intent: IntentSummary,
//	  incoming_anchors:    [{file_path, line}],
//	  references:          [slug]   // symmetric soft links (other intents' references to this one)
//	  prerequisites:       [slug]   // depends_on union, with symbol-inferred dependencies
//	                              // filtered out when an explicit depends_on exists for the
//	                              // same symbol (explicit wins)
//	}
//
// Incoming anchors are gathered by walking the repo and finding every
// @kron:intent <slug> line that references this intent.
//
// "References" is the reverse view: which other intents list this
// intent in their Frontmatter.References. Symmetric / soft.
//
// "Prerequisites" is the union of:
//   - this intent's own Frontmatter.DependsOn (hard, explicit)
//   - slugs whose Frontmatter.Symbol set intersects this intent's
//     (symbol-inferred, soft — but only added if NOT already covered
//     by an explicit depends_on for the same symbol)
//
// Returns ErrIntentNotFound (→ IsError=true) if slug does not exist.
func HandleImpact(ctx context.Context, _ *mcp.CallToolRequest, in ImpactInput) (
	*mcp.CallToolResult, ImpactOutput, error,
) {
	if err := assertCallerMCP(ctx); err != nil {
		return nil, ImpactOutput{}, err
	}
	if in.Slug == "" {
		return nil, ImpactOutput{}, fmt.Errorf("kron_impact requires non-empty slug")
	}
	if err := parser.ValidateSlug(in.Slug); err != nil {
		return nil, ImpactOutput{}, err
	}

	root := repoRootFrom(ctx)
	r, err := store.NewReader(root)
	if err != nil {
		return nil, ImpactOutput{}, fmt.Errorf("kron_impact: open store: %w", err)
	}
	target, err := r.Load(ctx, in.Slug)
	if err != nil {
		return nil, ImpactOutput{}, err
	}
	all, err := r.LoadAll(ctx)
	if err != nil {
		return nil, ImpactOutput{}, fmt.Errorf("kron_impact: load: %w", err)
	}

	// All three reverse-graph views (references, depends-on dependents,
	// symbol-inferred) and the prerequisites computation now live in
	// internal/relations. Centralising them here means the impact tool
	// and the delete tool (which surfaces "dependents") share the same
	// graph walks and the same "explicit wins" rule.
	references, _, _ := relations.ReverseLinks(all, in.Slug)
	prerequisites := relations.Prerequisites(all, in.Slug)

	// B-3: read the assumption registry when present so the
	// summary's `assumes[]` field shows registry text /
	// default_severity.
	var ar *assumption.Reader
	if ard, ardErr := assumption.NewReader(root); ardErr == nil {
		ar = ard
	}

	// Walk the repo and filter to anchors pointing at our slug.
	// RFC 2026-10-08-md-anchors.md §2.5: each Anchor carries a Kind
	// (AnchorKindCode / AnchorKindMarkdown) so callers don't have
	// to dispatch on the source file's extension. The two scan
	// streams are concatenated and tagged by parser, not by us.
	// The IsCode / IsMarkdown predicates collapse the empty-string
	// zero value (hand-constructed literals) onto "code" so the
	// wire value is never the empty string.
	var incoming []AnchorRef
	anchors, err := parser.ScanAnchors(root)
	if err != nil {
		return nil, ImpactOutput{}, fmt.Errorf("kron_impact: scan anchors: %w", err)
	}
	markdownAnchors, err := parser.ScanMarkdownAnchors(root)
	if err != nil {
		return nil, ImpactOutput{}, fmt.Errorf("kron_impact: scan markdown anchors: %w", err)
	}
	// `all` is already in use above as the []*model.Intent slice
	// (from r.LoadAll); use a distinct name to avoid shadowing.
	allAnchors := make([]model.Anchor, 0, len(anchors)+len(markdownAnchors))
	allAnchors = append(allAnchors, anchors...)
	allAnchors = append(allAnchors, markdownAnchors...)
	for _, a := range allAnchors {
		if a.Slug != in.Slug {
			continue
		}
		kind := string(model.AnchorKindCode)
		if a.Kind.IsMarkdown() {
			kind = string(model.AnchorKindMarkdown)
		}
		incoming = append(incoming, AnchorRef{
			FilePath: a.FilePath,
			Line:     a.LineNumber,
			Kind:     kind,
		})
	}
	if incoming == nil {
		incoming = []AnchorRef{}
	}

	// relativeFilePath trims the root prefix so the wire output is
	// repo-relative. This is what most editors / agents want.
	for i := range incoming {
		if rel, err := filepath.Rel(root, incoming[i].FilePath); err == nil {
			incoming[i].FilePath = rel
		}
	}

	// Stable order for downstream tests / agents: by (file_path,
	// line). parser.ScanAnchors walks the repo in a directory-order
	// traversal which is usually stable but not guaranteed by the
	// stdlib — sort defensively so kron_impact output never
	// depends on filepath.Walk internals.
	sort.SliceStable(incoming, func(i, j int) bool {
		if incoming[i].FilePath != incoming[j].FilePath {
			return incoming[i].FilePath < incoming[j].FilePath
		}
		return incoming[i].Line < incoming[j].Line
	})

	return nil, ImpactOutput{
		Intent:          intentSummaryFromIntent(target, ar),
		IncomingAnchors: incoming,
		References:      references,
		Prerequisites:   prerequisites,
	}, nil
}
