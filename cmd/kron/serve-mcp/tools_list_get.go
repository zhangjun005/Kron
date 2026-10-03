package servemcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/store"
)

// handleList returns the toolHandler for kron_list.
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
func handleList(root string) toolHandler {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		if err := assertCallerMCP(ctx); err != nil {
			return nil, err
		}
		var args struct {
			Prefix string `json:"prefix"`
		}
		if len(params) > 0 {
			if err := json.Unmarshal(params, &args); err != nil {
				return nil, fmt.Errorf("kron_list: invalid params: %w", err)
			}
		}

		r, err := store.NewReader(root)
		if err != nil {
			return nil, fmt.Errorf("kron_list: open store: %w", err)
		}
		all, err := r.LoadAll(ctx)
		if err != nil {
			return nil, fmt.Errorf("kron_list: load: %w", err)
		}

		out := listResponse{Intents: make([]intentSummaryWire, 0, len(all))}
		for _, in := range all {
			if args.Prefix != "" && !strings.HasPrefix(in.Slug, args.Prefix) {
				continue
			}
			out.Intents = append(out.Intents, intentSummaryFromIntent(in))
		}
		return out, nil
	}
}

// handleGet returns the toolHandler for kron_get.
//
// Wire contract: see docs/implementation/mcp.md §2 (kron_get).
//
//	in:  { slug: string }
//	out: { intent: {slug, frontmatter, body, source_path} }
//
// Returns ErrIntentNotFound (mapped to JSON-RPC -32602) if the slug
// does not exist. The slug is validated as part of the load path
// (store.Load returns ErrSlugInvalid for empty input), so callers do
// not need to pre-validate.
func handleGet(root string) toolHandler {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		if err := assertCallerMCP(ctx); err != nil {
			return nil, err
		}
		var args struct {
			Slug string `json:"slug"`
		}
		if len(params) > 0 {
			if err := json.Unmarshal(params, &args); err != nil {
				return nil, fmt.Errorf("kron_get: invalid params: %w", err)
			}
		}
		if args.Slug == "" {
			return nil, fmt.Errorf("%w: kron_get requires non-empty slug", model.ErrSlugInvalid)
		}

		r, err := store.NewReader(root)
		if err != nil {
			return nil, fmt.Errorf("kron_get: open store: %w", err)
		}
		intent, err := r.Load(ctx, args.Slug)
		if err != nil {
			return nil, err
		}
		return getResponse{Intent: intentFromModel(intent)}, nil
	}
}

// listResponse is the on-the-wire shape for kron_list.
type listResponse struct {
	Intents []intentSummaryWire `json:"intents"`
}

// getResponse is the on-the-wire shape for kron_get.
type getResponse struct {
	Intent intentWire `json:"intent"`
}

// intentSummaryWire is the on-the-wire summary shape (slug + key
// frontmatter fields). Matches docs/implementation/mcp.md §2
// IntentSummary.
type intentSummaryWire struct {
	Slug      string       `json:"slug"`
	Symbol    []string     `json:"symbol"`
	Status    string       `json:"status"`
	UpdatedAt string       `json:"updated_at"`
	Assumes   []assumeWire `json:"assumes,omitempty"`
}

// intentWire is the on-the-wire full-intent shape returned by
// kron_get. The body is the raw markdown string (no body
// transformations); source_path is absolute on disk.
type intentWire struct {
	Slug        string                `json:"slug"`
	Frontmatter intentFrontmatterWire `json:"frontmatter"`
	Body        string                `json:"body"`
	SourcePath  string                `json:"source_path"`
}

// intentFrontmatterWire is the on-the-wire frontmatter shape. We
// keep the same field names as the YAML (snake_case) so a client
// reading one can read the other without renaming.
//
// References and DependsOn are echoed here for kron_get; kron_list's
// summary (intentSummaryWire) intentionally omits them to keep the
// list response small. See docs/implementation/mcp.md §2.
type intentFrontmatterWire struct {
	Symbol      []string     `json:"symbol,omitempty"`
	CreatedBy   string       `json:"created_by"`
	UpdatedAt   string       `json:"updated_at"`
	Reviewers   []string     `json:"reviewers,omitempty"`
	Status      string       `json:"status,omitempty"`
	Assumptions []assumeWire `json:"assumptions,omitempty"`
	References  []string     `json:"references,omitempty"`
	DependsOn   []string     `json:"depends_on,omitempty"`
}

// assumeWire is the on-the-wire assumption shape. Assumes without a
// verified_by/verified_at still serialise those fields as empty
// strings; clients can detect "never verified" via the empty string
// or the absence of a non-zero verified_at.
type assumeWire struct {
	ID         string `json:"id"`
	Text       string `json:"text"`
	Severity   string `json:"severity"`
	ExpiresAt  string `json:"expires_at,omitempty"`
	VerifiedAt string `json:"verified_at,omitempty"`
	VerifiedBy string `json:"verified_by,omitempty"`
}

func intentSummaryFromIntent(in *model.Intent) intentSummaryWire {
	return intentSummaryWire{
		Slug:      in.Slug,
		Symbol:    in.Frontmatter.Symbol,
		Status:    string(in.Frontmatter.Status),
		UpdatedAt: in.Frontmatter.UpdatedAt.Format(time.RFC3339),
		Assumes:   assumesToWire(in.Frontmatter.Assumptions),
	}
}

func intentFromModel(in *model.Intent) intentWire {
	return intentWire{
		Slug: in.Slug,
		Frontmatter: intentFrontmatterWire{
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

func assumesToWire(as []model.Assumption) []assumeWire {
	if len(as) == 0 {
		return nil
	}
	out := make([]assumeWire, 0, len(as))
	for _, a := range as {
		out = append(out, assumeWire{
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
