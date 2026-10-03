package servemcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/store"
)

// HandleList returns the mcp.ToolHandlerFor binding for kron_list.
//
// Wire contract: see docs/implementation/mcp.md §2 (kron_list).
//
//	in:  { prefix?: string }
//	out: { intents: [{slug, symbol, status, updated_at}] }
//
// The optional "prefix" parameter filters the result to slugs that
// start with the given string. The match is case-sensitive and uses
// the raw slug form (e.g. prefix "auth" matches "auth/jwt" but not
// "Auth/foo"). An empty prefix matches all intents.
func HandleList(ctx context.Context, _ *mcp.CallToolRequest, in ListInput) (
	*mcp.CallToolResult, ListOutput, error,
) {
	if err := assertCallerMCP(ctx); err != nil {
		return nil, ListOutput{}, err
	}
	root := repoRootFrom(ctx)

	r, err := store.NewReader(root)
	if err != nil {
		return nil, ListOutput{}, fmt.Errorf("kron_list: open store: %w", err)
	}
	all, err := r.LoadAll(ctx)
	if err != nil {
		return nil, ListOutput{}, fmt.Errorf("kron_list: load: %w", err)
	}

	out := ListOutput{Intents: make([]IntentSummary, 0, len(all))}
	for _, intent := range all {
		if in.Prefix != "" && !strings.HasPrefix(intent.Slug, in.Prefix) {
			continue
		}
		out.Intents = append(out.Intents, intentSummaryFromIntent(intent))
	}
	return nil, out, nil
}

// HandleGet returns the mcp.ToolHandlerFor binding for kron_get.
//
// Wire contract: see docs/implementation/mcp.md §2 (kron_get).
//
//	in:  { slug: string }
//	out: { intent: {slug, frontmatter, body, source_path} }
//
// Returns ErrIntentNotFound (mapped to CallToolResult.IsError=true by
// the SDK) if the slug does not exist. The slug is validated as
// part of the load path (store.Load returns ErrSlugInvalid for
// empty input), so callers do not need to pre-validate.
func HandleGet(ctx context.Context, _ *mcp.CallToolRequest, in GetInput) (
	*mcp.CallToolResult, GetOutput, error,
) {
	if err := assertCallerMCP(ctx); err != nil {
		return nil, GetOutput{}, err
	}
	if in.Slug == "" {
		return nil, GetOutput{}, fmt.Errorf("%w: kron_get requires non-empty slug", model.ErrSlugInvalid)
	}
	root := repoRootFrom(ctx)

	r, err := store.NewReader(root)
	if err != nil {
		return nil, GetOutput{}, fmt.Errorf("kron_get: open store: %w", err)
	}
	intent, err := r.Load(ctx, in.Slug)
	if err != nil {
		return nil, GetOutput{}, err
	}
	return nil, GetOutput{Intent: intentFromModel(intent)}, nil
}

// intentSummaryFromIntent converts an internal Intent to the wire
// summary shape (slug + key frontmatter fields). Used by kron_list
// and kron_impact. Matches docs/implementation/mcp.md §2.
func intentSummaryFromIntent(in *model.Intent) IntentSummary {
	return IntentSummary{
		Slug:      in.Slug,
		Symbol:    in.Frontmatter.Symbol,
		Status:    string(in.Frontmatter.Status),
		UpdatedAt: in.Frontmatter.UpdatedAt.Format(time.RFC3339),
		Assumes:   assumesToWire(in.Frontmatter.Assumptions),
	}
}

func intentFromModel(in *model.Intent) IntentBody {
	return IntentBody{
		Slug: in.Slug,
		Frontmatter: IntentFrontmatter{
			Symbol:      in.Frontmatter.Symbol,
			CreatedBy:   in.Frontmatter.CreatedBy,
			UpdatedAt:   in.Frontmatter.UpdatedAt.Format(time.RFC3339),
			Reviewers:   in.Frontmatter.Reviewers,
			Status:      string(in.Frontmatter.Status),
			Assumptions: assumesToWire(in.Frontmatter.Assumptions),
			References:  in.Frontmatter.References,
			DependsOn:   in.Frontmatter.DependsOn,
		},
		Body:       in.Body,
		SourcePath: in.SourcePath,
	}
}

func assumesToWire(as []model.Assumption) []AssumeEntry {
	if len(as) == 0 {
		return nil
	}
	out := make([]AssumeEntry, 0, len(as))
	for _, a := range as {
		out = append(out, AssumeEntry{
			ID:         a.ID,
			Text:       a.Text,
			Severity:   string(a.Severity),
			ExpiresAt:  a.ExpiresAt,
			VerifiedAt: a.VerifiedAt,
			VerifiedBy: a.VerifiedBy,
		})
	}
	return out
}
