package servemcp

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/xxx/kron/internal/parser"
	"github.com/xxx/kron/internal/store"
)

// handleImpact returns the toolHandler for kron_impact.
//
// Wire contract: see docs/implementation/mcp.md §2 (kron_impact).
//
//	in:  { slug: string }
//	out: {
//	  intent: IntentSummary,
//	  incoming_anchors:    [{file_path, line}],
//	  depends_on_intents:  [slug]   // slugs whose Symbol set intersects this intent's
//	}
//
// Incoming anchors are gathered by walking the repo and finding every
// @kron:intent <slug> line that references this intent. "Depends on" is
// inferred from shared symbols: any other intent whose Frontmatter.Symbol
// contains any symbol this intent lists is a candidate.
//
// Returns ErrIntentNotFound (-32004) if slug does not exist.
func handleImpact(root string) toolHandler {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		if err := assertCallerMCP(ctx); err != nil {
			return nil, err
		}
		var args struct {
			Slug string `json:"slug"`
		}
		if len(params) > 0 {
			if err := json.Unmarshal(params, &args); err != nil {
				return nil, fmt.Errorf("kron_impact: invalid params: %w", err)
			}
		}
		if args.Slug == "" {
			return nil, fmt.Errorf("kron_impact requires non-empty slug")
		}
		if err := parser.ValidateSlug(args.Slug); err != nil {
			return nil, err
		}

		r, err := store.NewReader(root)
		if err != nil {
			return nil, fmt.Errorf("kron_impact: open store: %w", err)
		}
		target, err := r.Load(ctx, args.Slug)
		if err != nil {
			return nil, err
		}
		all, err := r.LoadAll(ctx)
		if err != nil {
			return nil, fmt.Errorf("kron_impact: load: %w", err)
		}

		// Build a symbol → slugs reverse index. We only need slugs
		// that overlap with target's symbol set.
		targetSyms := make(map[string]struct{}, len(target.Frontmatter.Symbol))
		for _, s := range target.Frontmatter.Symbol {
			targetSyms[s] = struct{}{}
		}
		var dependsOn []string
		if len(targetSyms) > 0 {
			for _, in := range all {
				if in.Slug == args.Slug {
					continue
				}
				for _, s := range in.Frontmatter.Symbol {
					if _, ok := targetSyms[s]; ok {
						dependsOn = append(dependsOn, in.Slug)
						break
					}
				}
			}
			sort.Strings(dependsOn)
		}

		// Walk the repo and filter to anchors pointing at our slug.
		var incoming []anchorRefWire
		anchors, err := parser.ScanAnchors(root)
		if err != nil {
			return nil, fmt.Errorf("kron_impact: scan anchors: %w", err)
		}
		for _, a := range anchors {
			if a.Slug == args.Slug {
				incoming = append(incoming, anchorRefWire{
					FilePath: a.FilePath,
					Line:     a.LineNumber,
				})
			}
		}
		if incoming == nil {
			incoming = []anchorRefWire{}
		}

		// relativeFilePath trims the root prefix so the wire output is
		// repo-relative. This is what most editors / agents want.
		for i := range incoming {
			if rel, err := filepath.Rel(root, incoming[i].FilePath); err == nil {
				incoming[i].FilePath = rel
			}
		}

		return impactResponse{
			Intent:           intentSummaryFromIntent(target),
			IncomingAnchors:  incoming,
			DependsOnIntents: dependsOn,
		}, nil
	}
}

type impactResponse struct {
	Intent           intentSummaryWire `json:"intent"`
	IncomingAnchors  []anchorRefWire   `json:"incoming_anchors"`
	DependsOnIntents []string          `json:"depends_on_intents"`
}

type anchorRefWire struct {
	FilePath string `json:"file_path"`
	Line     int    `json:"line"`
}
