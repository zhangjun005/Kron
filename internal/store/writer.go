// Package store handles file I/O for Kron intent files.
package store

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/parser"
)

// Writer creates, updates, and soft-deletes intent files.
//
// Writer shares path-resolution helpers (IntentPath / TrashPath) with Reader.
// Keep the two structs in sync if more path helpers are added.
type Writer struct {
	// Root is the absolute path to the repository root containing .kron/.
	Root string
}

// NewWriter returns a Writer rooted at root. Same validation as NewReader.
func NewWriter(root string) (*Writer, error) {
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
	return &Writer{Root: abs}, nil
}

// IntentPath returns the absolute path of an intent's .md file.
func (w *Writer) IntentPath(slug string) string {
	return filepath.Join(w.Root, model.IntentPath(slug))
}

// TrashPath returns the absolute path of a trashed intent's .md file.
func (w *Writer) TrashPath(slug string) string {
	return filepath.Join(w.Root, model.TrashPath(slug))
}

// Exists reports whether an intent with the given slug is present
// in .kron/intents/. Trash contents are NOT considered. Convenience
// mirror of Reader.Exists so write-then-verify patterns don't need to
// also construct a Reader.
func (w *Writer) Exists(ctx context.Context, slug string) bool {
	_ = ctx
	_, err := os.Stat(w.IntentPath(slug))
	return err == nil
}

// Write serialises intent to .kron/intents/<slug>.md, creating intermediate
// directories as needed.
//
// If the file already exists it is overwritten in place (atomic via rename
// of a temp file in the same directory). UpdatedAt is NOT auto-bumped;
// the caller is responsible for setting it before calling Write.
//
// The slug MUST be validated (parser.ValidateSlug) before calling; otherwise
// the resulting file path may escape .kron/intents/.
func (w *Writer) Write(ctx context.Context, intent *model.Intent) error {
	_ = ctx
	if intent == nil {
		return fmt.Errorf("store: intent is nil")
	}
	if err := parser.ValidateSlug(intent.Slug); err != nil {
		return fmt.Errorf("%w: %s", model.ErrSlugInvalid, intent.Slug)
	}

	path := w.IntentPath(intent.Slug)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("store: mkdir %s: %w", filepath.Dir(path), err)
	}

	data, err := parser.SerializeMarkdown(intent)
	if err != nil {
		return fmt.Errorf("store: serialize %s: %w", intent.Slug, err)
	}

	if err := writeFileAtomic(path, data, 0o644); err != nil {
		return fmt.Errorf("store: write %s: %w", path, err)
	}
	intent.SourcePath = path
	return nil
}

// MoveToTrash soft-deletes an intent: <root>/.kron/intents/<slug>.md
// is moved to <root>/.kron/.trash/<slug>.md.
//
// Returns model.ErrIntentNotFound (wrapped) if the intent does not exist.
// Returns model.ErrIntentExists (wrapped) if the trashed file already exists
// (so the move cannot clobber a prior deletion); the caller is expected to
// decide whether to surface this to the user or restore and retry.
//
// The original intents directory entry is removed BEFORE the rename so that
// a crash between the two steps leaves either zero or one file with the
// slug — never two, never zero with a phantom path.
func (w *Writer) MoveToTrash(ctx context.Context, slug string) error {
	_ = ctx
	src := w.IntentPath(slug)
	if _, err := os.Stat(src); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%w: %s", model.ErrIntentNotFound, slug)
		}
		return fmt.Errorf("store: stat %s: %w", src, err)
	}

	dst := w.TrashPath(slug)
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("%w: %s already in trash", model.ErrIntentExists, slug)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("store: stat %s: %w", dst, err)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("store: mkdir %s: %w", filepath.Dir(dst), err)
	}

	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("store: move %s → trash: %w", slug, err)
	}
	return nil
}

// RestoreFromTrash is the inverse of MoveToTrash: a trashed intent is moved
// back to .kron/intents/<slug>.md.
//
// Returns model.ErrIntentNotFound (wrapped) if no trashed file exists.
// Returns model.ErrIntentExists (wrapped) if an active intent with the same
// slug already exists (so restore cannot clobber the live version).
func (w *Writer) RestoreFromTrash(ctx context.Context, slug string) error {
	_ = ctx
	src := w.TrashPath(slug)
	if _, err := os.Stat(src); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%w: %s not in trash", model.ErrIntentNotFound, slug)
		}
		return fmt.Errorf("store: stat %s: %w", src, err)
	}

	dst := w.IntentPath(slug)
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("%w: %s already exists in intents", model.ErrIntentExists, slug)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("store: stat %s: %w", dst, err)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("store: mkdir %s: %w", filepath.Dir(dst), err)
	}

	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("store: restore %s: %w", slug, err)
	}
	return nil
}

// writeFileAtomic writes data to path via a temp file + rename so concurrent
// readers never observe a partial file. The temp file lives in the same
// directory as path so the rename is on the same filesystem.
func writeFileAtomic(path string, data []byte, perm fs.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".kron-write-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	// Best-effort cleanup if anything below fails before rename succeeds.
	defer func() {
		_ = os.Remove(tmpName)
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp → %s: %w", path, err)
	}
	return nil
}
