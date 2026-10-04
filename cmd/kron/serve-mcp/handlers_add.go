package servemcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/xxx/kron/internal/identity"
	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/parser"
	"github.com/xxx/kron/internal/store"
)

// HandleAdd returns the mcp.ToolHandlerFor binding for kron_add.
//
// Wire contract: see docs/implementation/mcp.md §2 (kron_add).
//
//	in:  { slug, symbol?, why? }
//	out: { ok: bool, path: string }
//
// "symbol" populates Frontmatter.Symbol; "why" is appended to a
// scaffolded body that follows the same shape as CLI `kron add`.
//
// The .kron/ skeleton must already exist; otherwise -32006
// (Unprocessable) is returned. Reusing an existing slug returns
// -32005 (Conflict), matching the mcp.md contract for ErrIntentExists.
func HandleAdd(ctx context.Context, _ *mcp.CallToolRequest, in AddInput) (
	*mcp.CallToolResult, AddOutput, error,
) {
	if err := assertCallerMCP(ctx); err != nil {
		return nil, AddOutput{}, err
	}
	if in.Slug == "" {
		return nil, AddOutput{}, fmt.Errorf("%w: kron_add requires non-empty slug", model.ErrSlugInvalid)
	}
	if err := parser.ValidateSlug(in.Slug); err != nil {
		return nil, AddOutput{}, err
	}

	root := repoRootFrom(ctx)
	kronDir := filepath.Join(root, model.KronDir)
	if _, err := os.Stat(kronDir); err != nil {
		if os.IsNotExist(err) {
			return nil, AddOutput{}, fmt.Errorf("%w: .kron/ not found; run kron_init first", model.ErrConfigInvalid)
		}
		return nil, AddOutput{}, fmt.Errorf("kron_add: stat %s: %w", kronDir, err)
	}

	w, err := store.NewWriter(root)
	if err != nil {
		return nil, AddOutput{}, fmt.Errorf("kron_add: open store: %w", err)
	}
	if w.Exists(ctx, in.Slug) {
		return nil, AddOutput{}, fmt.Errorf("%w: %s", model.ErrIntentExists, in.Slug)
	}

	// Identity is resolved via internal/identity: prefer an explicit
	// client-derived handle, fall back to git config user.name, fall
	// back to "agent:unknown". Identity is an attribute, not an
	// authz gate (architecture.md §2.3).
	handle := identity.Handle("", identity.GitUser())

	fm := model.Frontmatter{
		CreatedBy: handle,
		UpdatedAt: time.Now().UTC(),
	}
	if in.Symbol != "" {
		fm.Symbol = []string{in.Symbol}
	}

	body := scaffoldedBody(in.Slug)
	if in.Why != "" {
		body = body + "\n## Why\n\n" + in.Why + "\n"
	}

	intent := &model.Intent{
		Slug:        in.Slug,
		Frontmatter: fm,
		Body:        body,
	}
	if err := w.Write(ctx, intent); err != nil {
		return nil, AddOutput{}, fmt.Errorf("kron_add: write: %w", err)
	}

	return nil, AddOutput{OK: true, Path: model.IntentPath(in.Slug)}, nil
}

// scaffoldedBody produces the default Markdown body for a freshly
// scaffolded intent. Mirrors the CLI `kron add` skeleton from
// cmd/kron/cli/add.go (which also delegates the body template
// responsibility up the stack when the template becomes shareable).
func scaffoldedBody(slug string) string {
	title := slug
	if i := strings.LastIndex(slug, "/"); i >= 0 {
		title = slug[i+1:]
	}
	title = strings.ReplaceAll(title, "-", " ")

	return fmt.Sprintf("# %s\n\n> One-line summary of this intent's decision.\n\n## Why\n\nRecord the reason for this decision.\n\n## Trade-offs\n\nWhat was chosen and what was given up.\n", title)
}
