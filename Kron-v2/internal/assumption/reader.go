// Package assumption provides read/write access to the assumption registry:
// .kron/assumptions/<id>.md files.
//
// # Design
//
// Each file stores one assumption's metadata (id, text, default_severity,
// ownership) and optional body text. The intent's frontmatter references
// assumptions by id only; the full assumption data lives here.
//
// B-3: the registry file is the single source of truth for assumption
// metadata; intents reference by id and may override severity per-intent.
// See docs/rfc/2026-10-08-assumptions-standalone.md.
package assumption

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/parser"
)

// Reader reads from the assumption registry.
type Reader struct {
	root string // absolute path to .kron/assumptions/
}

// NewReader returns a Reader rooted at root (the repository root, not
// .kron/assumptions/ itself). It validates that root/.kron/assumptions/
// exists and is accessible.
func NewReader(root string) (*Reader, error) {
	if root == "" {
		return nil, errors.New("assumption: root is empty")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("assumption: resolve root: %w", err)
	}
	dir := filepath.Join(abs, ".kron", "assumptions")
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return nil, ErrAssumptionDirNotFound
		}
		return nil, fmt.Errorf("assumption: stat %s: %w", dir, err)
	}
	return &Reader{root: dir}, nil
}

// Get returns the assumption file for id.
//
// Returns ErrAssumptionNotFound (wrapped) when the file does not
// exist. Returns ErrAssumptionFileIdMismatch (wrapped) when the
// file exists but its frontmatter id does not match the file
// name — the file's parsed content is NOT returned in this case,
// so the caller has to decide whether to surface the file to
// users (typically: re-read it directly if the frontmatter id
// matters). For the bulk-read case where mismatched files must
// still appear in listings, use List instead (List tolerates
// id-mismatch files so lint can flag them).
func (r *Reader) Get(ctx context.Context, id string) (*model.AssumptionFile, error) {
	_ = ctx
	if err := parser.ValidateSlug(id); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrAssumptionNotFound, id)
	}
	path := filepath.Join(r.root, id+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrAssumptionNotFound, id)
		}
		return nil, fmt.Errorf("assumption: read %s: %w", path, err)
	}

	fm, body, err := parser.ParseAssumptionFrontmatter(data)
	if err != nil {
		return nil, fmt.Errorf("assumption: parse %s: %w", path, err)
	}

	// B-3 (RFC 2026-10-08-assumptions-standalone): an id mismatch
	// is a data-quality issue but not a load failure. We return
	// the sentinel error WITHOUT the file so callers that want
	// to fail fast on mismatch (a future MCP "load by id" tool)
	// can do so. Callers that need to enumerate mismatched
	// files (lint) use List, which reads the file directly and
	// surfaces the frontmatter's id alongside the file name.
	if fm.ID != "" && fm.ID != id {
		return nil, fmt.Errorf("%w: frontmatter id %q != file name %q", ErrAssumptionFileIdMismatch, fm.ID, id)
	}

	return &model.AssumptionFile{
		Slug:        id,
		Frontmatter: fm,
		Body:        body,
		SourcePath:  path,
	}, nil
}

// List returns all assumption files in the registry, sorted by id.
//
// List tolerates id-mismatch files (a file whose frontmatter id
// does not match its file name) so lint's RuleAssumptionFileIdMismatch
// can see them. Files that fail to parse are skipped silently;
// their I/O / parse failure is surfaced separately by the
// lint pass that walks the file directly. This is a deliberate
// trade-off: List is a "best-effort enumeration" for callers
// that need to iterate (lint, future MCP list tools), and the
// stricter Get-by-id path surfaces a real error for the
// caller that needs the canonical id lookup.
func (r *Reader) List(ctx context.Context) ([]*model.AssumptionFile, error) {
	_ = ctx
	entries, err := os.ReadDir(r.root)
	if err != nil {
		return nil, fmt.Errorf("assumption: read dir %s: %w", r.root, err)
	}

	var out []*model.AssumptionFile
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
			continue
		}
		id := e.Name()[:len(e.Name())-len(".md")]
		af, err := r.readFile(id)
		if err != nil {
			// Genuine I/O or parse failure — skip. Lint's
			// A-class / frontmatter-invalid surfaces these
			// when it walks the file directly.
			continue
		}
		_ = err // id-mismatch is a warning, not a skip; keep the file
		out = append(out, af)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out, nil
}

// readFile parses the file at <id>.md WITHOUT applying the
// id-mismatch check that Get applies. It is the shared helper
// backing List (which must enumerate mismatched files so lint
// can flag them) and any future "load by path" internal
// helper. Public Get goes through this with the mismatch check
// added on top.
func (r *Reader) readFile(id string) (*model.AssumptionFile, error) {
	if err := parser.ValidateSlug(id); err != nil {
		return nil, err
	}
	path := filepath.Join(r.root, id+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	fm, body, err := parser.ParseAssumptionFrontmatter(data)
	if err != nil {
		return nil, err
	}
	return &model.AssumptionFile{
		Slug:        id,
		Frontmatter: fm,
		Body:        body,
		SourcePath:  path,
	}, nil
}

// Exists reports whether an assumption with the given id exists.
func (r *Reader) Exists(ctx context.Context, id string) bool {
	_ = ctx
	if err := parser.ValidateSlug(id); err != nil {
		return false
	}
	path := filepath.Join(r.root, id+".md")
	_, err := os.Stat(path)
	return err == nil
}
