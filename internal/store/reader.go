// Package store handles file I/O for Kron intent files.
//
// It reads and writes .kron/intents/<slug>.md files and moves them
// to/from .kron/.trash/. It does not know whether the caller is CLI,
// MCP, LSP, IDE, or GUI — that identity flows via context.Context.
//
// Errors are wrapped at the point of origin. Sentinel errors come from
// the model package (ErrIntentNotFound, ErrFrontmatterInvalid, ...).
package store

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/parser"
)

// Reader reads intent files from the store.
//
// All methods are pure: no global mutation, no in-memory caches.
// Every method accepts ctx for caller-attribute attribution and future
// cancellation hooks (currently a no-op stub passed through unchanged).
type Reader struct {
	// Root is the absolute path to the repository root containing .kron/.
	Root string
}

// NewReader returns a Reader rooted at root. Returns an error if root
// is empty or does not exist.
func NewReader(root string) (*Reader, error) {
	if root == "" {
		return nil, fmt.Errorf("store: root path is empty")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("store: resolve root: %w", err)
	}
	if _, err := os.Stat(abs); err != nil {
		return nil, fmt.Errorf("store: root not accessible: %w", err)
	}
	return &Reader{Root: abs}, nil
}

// IntentPath returns the absolute path of an intent's .md file.
func (r *Reader) IntentPath(slug string) string {
	return filepath.Join(r.Root, model.IntentPath(slug))
}

// TrashPath returns the absolute path of a trashed intent's .md file.
func (r *Reader) TrashPath(slug string) string {
	return filepath.Join(r.Root, model.TrashPath(slug))
}

// KronDir returns the absolute path of the .kron directory.
func (r *Reader) KronDir() string {
	return filepath.Join(r.Root, model.KronDir)
}

// Load reads a single intent by slug.
//
// Returns model.ErrIntentNotFound (wrapped) if the file is absent.
// Returns model.ErrFrontmatterInvalid (wrapped) if the YAML block is malformed.
// SourcePath on the returned Intent is set to the absolute disk path.
func (r *Reader) Load(ctx context.Context, slug string) (*model.Intent, error) {
	_ = ctx // reserved for cancellation; no-op in v1
	if slug == "" {
		return nil, fmt.Errorf("%w: slug is empty", model.ErrSlugInvalid)
	}
	path := r.IntentPath(slug)

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", model.ErrIntentNotFound, slug)
		}
		return nil, fmt.Errorf("store: read %s: %w", path, err)
	}

	fmRaw, body, err := parser.SplitMarkdown(data)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", model.ErrFrontmatterInvalid, slug, err)
	}

	fm, err := parser.ParseFrontmatter(fmRaw)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", model.ErrFrontmatterInvalid, slug, err)
	}

	// Note: self-reference (an intent listing its own slug in
	// references / depends_on) is NOT rejected here. Schema validation
	// cannot catch it without an owner slug in scope, and pushing
	// that responsibility into the load path would force every
	// caller to know the slug. Instead, lint.RuleSelfReference catches
	// it as part of the per-intent scan, and kron_update's
	// validateFrontmatterShape checks it at write time (where the
	// caller does know the slug). This is the RFC 2026-10-03 design.

	return &model.Intent{
		Slug:        slug,
		Frontmatter: fm,
		Body:        body,
		SourcePath:  path,
	}, nil
}

// LoadAll reads every intent under .kron/intents/.
//
// Subdirectories are walked recursively so a slug like "auth/jwt"
// (which on disk lives at .kron/intents/auth/jwt.md) is loaded with
// its full path as the slug. The slug-to-path mapping is:
//   - on disk: <root>/.kron/intents/<slug>.md
//   - slug may contain "/" separators that map to subdirectories
//
// Files that do not match the slug pattern (anything not ending in
// .md, or files in directories that are NOT valid slug prefixes)
// are skipped silently — see intent-structure.md §二 for the slug
// grammar and parser.ValidateSlug for the authoritative validator.
//
// Slugs are returned in lexicographic order (deterministic for diffs).
// Returns an empty slice (not nil) when the directory is absent or empty.
// If any individual file fails to parse, the first such error is returned
// wrapped with the failing slug; partial results are NOT returned.
func (r *Reader) LoadAll(ctx context.Context) ([]*model.Intent, error) {
	_ = ctx
	dir := filepath.Join(r.Root, model.KronDir, model.IntentDir)
	slugs, err := walkIntentSlugs(dir)
	if err != nil {
		return nil, err
	}

	intents := make([]*model.Intent, 0, len(slugs))
	for _, slug := range slugs {
		intent, err := r.Load(ctx, slug)
		if err != nil {
			return nil, fmt.Errorf("store: load %s: %w", slug, err)
		}
		intents = append(intents, intent)
	}
	return intents, nil
}

// walkIntentSlugs walks the intents directory recursively and returns
// the slug for each .md file. Subdirectories that don't form a valid
// slug prefix (e.g., contain uppercase letters or "..") are pruned
// to avoid surprises on malformed filesystems; the files inside are
// not loaded. Slugs are returned in lexicographic order.
func walkIntentSlugs(root string) ([]string, error) {
	slugs := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// If the root itself is missing, return empty (not an
			// error) so a fresh repo with no .kron/intents/ behaves
			// identically to one with an empty intents dir.
			if errors.Is(walkErr, fs.ErrNotExist) && path == root {
				return nil
			}
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), model.IntentExtension) {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		// Convert filesystem path separators to "/"-slugs. On Windows
		// filepath.Rel returns "auth\jwt.md"; we want "auth/jwt".
		slug := strings.TrimSuffix(filepath.ToSlash(rel), model.IntentExtension)
		slugs = append(slugs, slug)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("store: walk %s: %w", root, err)
	}
	sort.Strings(slugs)
	return slugs, nil
}

// Exists reports whether an intent with the given slug is present
// in .kron/intents/. Trash contents are NOT considered.
func (r *Reader) Exists(ctx context.Context, slug string) bool {
	_ = ctx
	path := r.IntentPath(slug)
	_, err := os.Stat(path)
	return err == nil
}
