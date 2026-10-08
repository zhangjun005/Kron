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

// Get returns the assumption file for id. If the file does not exist,
// it returns ErrAssumptionNotFound.
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

	// Verify file name matches frontmatter id.
	if fm.ID != id {
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
		af, err := r.Get(ctx, id)
		if err != nil {
			// Skip files that fail to parse; lint will surface the error.
			continue
		}
		out = append(out, af)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out, nil
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
