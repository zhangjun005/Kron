package servemcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/parser"
	"github.com/xxx/kron/internal/store"
)

// handleAdd returns the toolHandler for kron_add.
//
// Wire contract: see docs/implementation/mcp.md §2 (kron_add).
//
//	in:  { slug, symbol?, why? }
//	out: { ok: bool, path: string }
//
// "symbol" populates Frontmatter.Symbol; "why" is appended to a
// scaffolded body that follows the same shape as CLI `kron add`.
//
// The .kron/ skeleton must already exist; otherwise -32006 (Unprocessable)
// is returned. Reusing an existing slug returns -32005 (Conflict),
// matching the mcp.md contract for ErrIntentExists.
func handleAdd(root string) toolHandler {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		if err := assertCallerMCP(ctx); err != nil {
			return nil, err
		}
		var args struct {
			Slug   string `json:"slug"`
			Symbol string `json:"symbol"`
			Why    string `json:"why"`
		}
		if len(params) > 0 {
			if err := json.Unmarshal(params, &args); err != nil {
				return nil, fmt.Errorf("%w: kron_add: invalid params: %v", model.ErrSlugInvalid, err)
			}
		}
		if args.Slug == "" {
			return nil, fmt.Errorf("%w: kron_add requires non-empty slug", model.ErrSlugInvalid)
		}
		if err := parser.ValidateSlug(args.Slug); err != nil {
			return nil, err
		}

		kronDir := filepath.Join(root, model.KronDir)
		if _, err := os.Stat(kronDir); err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("%w: .kron/ not found; run kron_init first", model.ErrConfigInvalid)
			}
			return nil, fmt.Errorf("kron_add: stat %s: %w", kronDir, err)
		}

		w, err := store.NewWriter(root)
		if err != nil {
			return nil, fmt.Errorf("kron_add: open store: %w", err)
		}
		if w.Exists(ctx, args.Slug) {
			return nil, fmt.Errorf("%w: %s", model.ErrIntentExists, args.Slug)
		}

		handle := detectGitUser()
		if handle == "" {
			handle = "agent:unknown"
		}

		fm := model.Frontmatter{
			CreatedBy: handle,
			UpdatedAt: time.Now().UTC(),
		}
		if args.Symbol != "" {
			fm.Symbol = []string{args.Symbol}
		}

		body := scaffoldedBody(args.Slug)
		if args.Why != "" {
			body = body + "\n## Why\n\n" + args.Why + "\n"
		}

		intent := &model.Intent{
			Slug:        args.Slug,
			Frontmatter: fm,
			Body:        body,
		}
		if err := w.Write(ctx, intent); err != nil {
			return nil, fmt.Errorf("kron_add: write: %w", err)
		}

		return addResponse{OK: true, Path: model.IntentPath(args.Slug)}, nil
	}
}

// scaffoldedBody produces the default Markdown body for a freshly
// scaffolded intent. Mirrors the CLI `kron add` skeleton from
// cmd/kron/cli/add.go:defaultIntentBody, minus the trailing frontmatter
// reminder (which is on the frontmatter side, not the body).
func scaffoldedBody(slug string) string {
	title := slug
	if i := strings.LastIndex(slug, "/"); i >= 0 {
		title = slug[i+1:]
	}
	title = strings.ReplaceAll(title, "-", " ")

	return fmt.Sprintf("# %s\n\n> One-line summary of this intent's decision.\n\n## Why\n\nRecord the reason for this decision.\n\n## Trade-offs\n\nWhat was chosen and what was given up.\n", title)
}

// detectGitUser mirrors cmd/kron/cli/add.go:detectGitUser. Copied here
// because cmd/kron/cli is a sibling access layer and cannot be imported
// (architecture.md §〇 铁律 #3).
func detectGitUser() string {
	cmd := exec.Command("git", "config", "user.name")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	name := strings.TrimSpace(string(out))
	if name == "" {
		return ""
	}
	return "@" + name
}

type addResponse struct {
	OK   bool   `json:"ok"`
	Path string `json:"path"`
}
