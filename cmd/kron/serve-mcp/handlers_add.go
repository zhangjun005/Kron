package servemcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
//	in:  { slug, symbol?, why?, kind? }
//	out: { ok: bool, path: string }
//
// "symbol" populates Frontmatter.Symbol; "why" is appended to a
// scaffolded body that follows the same shape as CLI `kron add`.
// "kind" selects the on-disk physical form (leaf or node) — see
// RFC 2026-10-08-writer-readme-symmetry §3.5.
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

	// Parse kind via model.IntentKind. Empty string is treated as
	// "leaf" (the default) for backwards compatibility with callers
	// that pre-date RFC 2026-10-08-writer-readme-symmetry.
	kind := model.IntentKind(in.Kind)
	if !kind.Valid() {
		return nil, AddOutput{}, fmt.Errorf("kron_add: invalid kind %q (must be leaf or node)", in.Kind)
	}
	if kind == "" {
		kind = model.IntentKindLeaf
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

	body := parser.IntentBodyTemplate(in.Slug)
	if in.Why != "" {
		body = body + "\n## Why\n\n" + in.Why + "\n"
	}

	intent := &model.Intent{
		Slug:        in.Slug,
		Kind:        kind,
		Frontmatter: fm,
		Body:        body,
	}
	if err := w.Write(ctx, intent); err != nil {
		return nil, AddOutput{}, fmt.Errorf("kron_add: write: %w", err)
	}

	// Echo the path the user actually got. For node form this is
	// <slug>/README.md, not <slug>.md. Writer.Write sets
	// intent.SourcePath to the resolved absolute path; we
	// return a repo-relative string for the JSON contract.
	relPath, err := filepath.Rel(root, intent.SourcePath)
	if err != nil {
		// Fall back to model.IntentPath if rel fails (shouldn't
		// happen because both paths live under root).
		relPath = model.IntentPath(in.Slug)
	}
	return nil, AddOutput{OK: true, Path: filepath.ToSlash(relPath)}, nil
}

// scaffoldedBody delegates to parser.IntentBodyTemplate. The wrapper
// is kept so the handler reads naturally; the actual template lives
// in internal/parser where it is shared with the CLI add path and
// any future access layer. The CLI uses the Chinese variant
// (parser.IntentBodyTemplateCN) to preserve its prior wording.
func scaffoldedBody(slug string) string {
	return parser.IntentBodyTemplate(slug)
}
