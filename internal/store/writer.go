// Package store handles file I/O for Kron intent files.
//
// It reads and writes .kron/intents/<slug>.md files and moves them
// to/from .kron/.trash/. It does not know whether the caller is CLI,
// MCP, LSP, IDE, or GUI — that identity flows via context.Context.
//
// Errors are wrapped at the point of origin. Sentinel errors come from
// the model package (ErrIntentNotFound, ErrFrontmatterInvalid, ...)
// and from the store package itself (ErrEmptyPatch,
// ErrCreatorChangeNotAllowed — both raised by Writer.Update).
package store

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

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

// IntentPath returns the absolute path of an intent's .md file in
// CANONICAL form (<slug>.md). It does NOT consult the disk to
// apply the README shorthand — for that, callers should use
// Write / Exists / MoveToTrash / RestoreFromTrash which route
// through resolveWritePath / resolveReadPath (the source of
// truth for kind-aware shape).
//
// IntentPath is kept for callers that need a deterministic path
// for a slug regardless of on-disk state (e.g. error messages,
// paths printed back to the user).
func (w *Writer) IntentPath(slug string) string {
	return filepath.Join(w.Root, model.IntentPath(slug))
}

// writePath returns the absolute on-disk path for an intent being
// WRITTEN, applying the kind-aware shape decisions in
// resolveWritePath. Returns ErrSlugCollision if the OTHER form
// is already on disk.
//
// The returned path is what Write / Exists / MoveToTrash /
// RestoreFromTrash operate on, so all four methods agree on
// shape by construction.
func (w *Writer) writePath(kind model.IntentKind, slug string) (string, error) {
	return resolveWritePath(w.Root, kind, slug)
}

// TrashPath returns the absolute path of a trashed intent's .md file.
func (w *Writer) TrashPath(slug string) string {
	return filepath.Join(w.Root, model.TrashPath(slug))
}

// Exists reports whether an intent with the given slug is present
// in .kron/intents/. Trash contents are NOT considered. Convenience
// mirror of Reader.Exists so write-then-verify patterns don't need to
// also construct a Reader.
//
// Like Reader.Exists, applies the README shorthand: a slug
// resolves if EITHER "<slug>.md" OR "<slug>/README.md" is on disk.
// This keeps the existence check consistent with the read path
// (so callers don't falsely flag `// @kron:intent auth` as
// dangling when the module-level README.md is the actual intent).
//
// The implementation is a thin wrapper around resolveReadPath so
// reader and writer share one source of truth for the
// canonical-then-README fallback. Error discrimination (EACCES vs
// ErrNotExist) is intentionally suppressed: a read that can't be
// performed for any reason is "doesn't exist" from the caller's
// perspective.
func (w *Writer) Exists(ctx context.Context, slug string) bool {
	_ = ctx
	if slug == "" {
		return false
	}
	if _, err := resolveReadPath(w.Root, slug); err == nil {
		return true
	}
	return false
}

// Write serialises intent to disk, creating intermediate
// directories as needed. The on-disk shape (canonical <slug>.md
// vs node-shaped <slug>/README.md) is decided by intent.Kind:
//
//	Kind="" or Kind=Leaf → <root>/.kron/intents/<slug>.md
//	Kind=Node            → <root>/.kron/intents/<slug>/README.md
//
// Returns ErrSlugCollision if the OTHER form is already on disk
// (e.g. trying to write a leaf when a node-form README exists at
// the same slug, or vice versa). The two forms cannot share a
// slug — see RFC 2026-10-08-writer-readme-symmetry §3.3.
//
// If the file in the same form already exists it is overwritten
// in place (atomic via rename of a temp file in the same
// directory). UpdatedAt is NOT auto-bumped; the caller is
// responsible for setting it before calling Write.
//
// The slug MUST be validated (parser.ValidateSlug) before calling;
// otherwise the resulting file path may escape .kron/intents/.
func (w *Writer) Write(ctx context.Context, intent *model.Intent) error {
	_ = ctx
	if intent == nil {
		return fmt.Errorf("store: intent is nil")
	}
	if err := parser.ValidateSlug(intent.Slug); err != nil {
		return fmt.Errorf("%w: %s", model.ErrSlugInvalid, intent.Slug)
	}
	return w.withIntentLock(intent.Slug, func() error {
		return w.writeLocked(intent)
	})
}

// writeLocked writes intent after the logical slug lock is held.
func (w *Writer) writeLocked(intent *model.Intent) error {
	path, err := w.writePath(intent.Kind, intent.Slug)
	if err != nil {
		return err
	}
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
	// Backfill Kind from the on-disk path so the caller's
	// in-memory struct matches the file that was actually written.
	// Without this, a caller that calls Write twice with different
	// Kind values would have a stale Kind after the second Write.
	intent.Kind = KindFromPath(path)
	return nil
}

// MoveToTrash soft-deletes an intent. The on-disk shape is preserved:
//
//	leaf <intents>/<slug>.md        → <trash>/<slug>.md
//	node <intents>/<slug>/          → <trash>/<slug>/   (whole subtree)
//
// Deleting a node therefore removes its whole subtree from the live tree;
// the trash entry mirrors the directory so RestoreFromTrash can bring the
// subtree back with its shape intact. See
// docs/implementation/2026-10-29-trash-subtree.md.
//
// Returns model.ErrIntentNotFound (wrapped) if the intent does not exist in
// either form. Returns model.ErrIntentExists (wrapped) if either trash shape
// for the slug already exists (so the move cannot clobber a prior deletion).
func (w *Writer) MoveToTrash(ctx context.Context, slug string) error {
	_ = ctx
	return w.withIntentLock(slug, func() error {
		return w.moveToTrashLocked(slug)
	})
}

func (w *Writer) moveToTrashLocked(slug string) error {
	src, err := resolveReadPath(w.Root, slug)
	if err != nil {
		return err
	}

	leafTrash := w.TrashPath(slug)
	dirTrash := filepath.Join(w.Root, model.TrashDirPath(slug))
	for _, existing := range []string{leafTrash, dirTrash} {
		if _, statErr := os.Stat(existing); statErr == nil {
			return fmt.Errorf("%w: %s already in trash", model.ErrIntentExists, slug)
		} else if !errors.Is(statErr, fs.ErrNotExist) {
			return fmt.Errorf("store: stat %s: %w", existing, statErr)
		}
	}

	dst := leafTrash
	if KindFromPath(src) == model.IntentKindNode {
		// A node's README lives inside its directory; move the directory
		// itself so descendants travel with it.
		src = filepath.Dir(src)
		dst = dirTrash
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("store: mkdir %s: %w", filepath.Dir(dst), err)
	}
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("store: move %s → trash: %w", slug, err)
	}
	return nil
}

// RestoreFromTrash is the inverse of MoveToTrash. It detects the trash shape
// (node directory first, then leaf file) and restores the matching live shape:
//
//	<trash>/<slug>/    → <intents>/<slug>/
//	<trash>/<slug>.md  → <intents>/<slug>.md
//
// Legacy entries written by the old node-delete path are leaf files; they are
// restored as leaf intents (no automatic node promotion) and the caller can
// convert them via Write with Kind=Node.
//
// Returns model.ErrIntentNotFound (wrapped) if no trashed entry exists.
// Returns model.ErrIntentExists (wrapped) if either live shape for the slug
// already exists (so restore cannot merge or clobber).
func (w *Writer) RestoreFromTrash(ctx context.Context, slug string) error {
	_ = ctx
	return w.withIntentLock(slug, func() error {
		return w.restoreFromTrashLocked(slug)
	})
}

func (w *Writer) restoreFromTrashLocked(slug string) error {
	leafTrash := w.TrashPath(slug)
	dirTrash := filepath.Join(w.Root, model.TrashDirPath(slug))
	nodeShape := false
	if info, err := os.Stat(dirTrash); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("store: restore %s: %s is not a directory", slug, dirTrash)
		}
		nodeShape = true
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("store: stat %s: %w", dirTrash, err)
	} else if _, leafErr := os.Stat(leafTrash); leafErr != nil {
		if errors.Is(leafErr, fs.ErrNotExist) {
			return fmt.Errorf("%w: %s not in trash", model.ErrIntentNotFound, slug)
		}
		return fmt.Errorf("store: stat %s: %w", leafTrash, leafErr)
	}

	// Refuse to restore onto either live shape; leaf+dir coexistence at the
	// same slug is ambiguous and writers already reject it.
	dirTarget := filepath.Join(w.Root, model.KronDir, model.IntentDir, slug)
	leafTarget := filepath.Join(w.Root, model.KronDir, model.IntentDir, slug+model.IntentExtension)
	for _, dst := range []string{leafTarget, dirTarget} {
		if _, err := os.Stat(dst); err == nil {
			return fmt.Errorf("%w: %s already exists in intents", model.ErrIntentExists, slug)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("store: stat %s: %w", dst, err)
		}
	}

	src, dst := leafTrash, leafTarget
	if nodeShape {
		src, dst = dirTrash, dirTarget
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("store: mkdir %s: %w", filepath.Dir(dst), err)
	}
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("store: restore %s: %w", slug, err)
	}
	return nil
}

// RelativeIntentPath resolves the live on-disk path for slug (applying the
// node README fallback) and returns it repo-relative in slash form.
func (w *Writer) RelativeIntentPath(slug string) (string, error) {
	abs, err := resolveReadPath(w.Root, slug)
	if err != nil {
		return "", err
	}
	rel, relErr := filepath.Rel(w.Root, abs)
	if relErr != nil {
		return filepath.ToSlash(abs), nil
	}
	return filepath.ToSlash(rel), nil
}

// RelativeTrashPath resolves the soft-deleted on-disk path for slug (node
// directory first, then legacy leaf file) and returns it repo-relative.
func (w *Writer) RelativeTrashPath(slug string) (string, error) {
	for _, rel := range []string{model.TrashDirPath(slug), model.TrashPath(slug)} {
		if _, err := os.Stat(filepath.Join(w.Root, rel)); err == nil {
			return rel, nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("store: stat %s: %w", rel, err)
		}
	}
	return "", fmt.Errorf("%w: %s not in trash", model.ErrIntentNotFound, slug)
}

// =====================================================================
// Update
// =====================================================================

// UpdatePatch is the input for Writer.Update.
//
// Every business field is a pointer: nil = leave alone, non-nil = overwrite
// (even to zero value). Required field Actor is a value (the library
// stamps UpdatedAt from time.Now() regardless).
//
// Empty patch (all *T fields nil) returns ErrEmptyPatch — the library
// refuses to write a file with only UpdatedAt changed.
//
// Changing CreatedBy requires the WithAllowCreatorChange opt-in.
// CreatedBy is identity-bearing; mutating it has social / forensic
// implications and is a separate gate from the patch itself.
//
// Mirrors assumption.UpdatePatch. Field set matches model.Frontmatter
// (every frontmatter field is patchable) plus Body (the markdown below
// the frontmatter separator). Assumptions is patched as a whole slice
// — partial per-assumption updates are not supported at this layer;
// use assumption.Writer for assumption-specific edits.
type UpdatePatch struct {
	// Frontmatter fields (all optional pointers)
	Symbol      *[]string           // nil = leave alone; non-nil = overwrite
	CreatedBy   *string             // nil = leave alone; non-nil = overwrite (gated by WithAllowCreatorChange)
	Reviewers   *[]string           // nil = leave alone
	Status      *model.Status       // nil = leave alone
	Assumptions *[]model.Assumption // nil = leave alone; whole-slice replacement
	References  *[]string           // nil = leave alone
	DependsOn   *[]string           // nil = leave alone

	// Body (markdown below the frontmatter separator)
	Body *string // nil = leave alone; non-nil = overwrite

	// Required
	Actor string // "@user" or "agent:<model>" — library records who triggered the update
}

// updateConfig holds the parsed UpdateOption values.
type updateConfig struct {
	allowCreatorChange bool
}

// UpdateOption configures a single Update call.
type UpdateOption func(*updateConfig)

// WithAllowCreatorChange permits Update to change CreatedBy.
//
// The CALLER is responsible for any confirmation UI / audit log before
// calling Update with this opt-in. The library does NOT interpret
// "Actor" beyond identity formatting (it does not check that Actor
// is the current CreatedBy, a Reviewer, etc.) — permission rules
// are an upper-layer concern.
//
// Mirrors assumption.WithAllowCreatorChange.
func WithAllowCreatorChange() UpdateOption {
	return func(c *updateConfig) {
		c.allowCreatorChange = true
	}
}

// Update overwrites an existing intent's frontmatter fields on disk,
// keeping the slug (file name) fixed. It returns ErrIntentNotFound if
// the file does not exist, ErrEmptyPatch if the patch has no business
// changes, or ErrCreatorChangeNotAllowed if CreatedBy is in the patch
// without WithAllowCreatorChange.
//
// The library sets UpdatedAt to time.Now().UTC() on a successful write.
// The previous UpdatedAt is overwritten regardless of the patch contents
// (this is the point of an Update: a record was changed).
//
// Note: Update cannot rename an intent. To rename, use MoveToTrash +
// Write (preserves history through the trash dir).
//
// Mutations to the same slug are serialized across Writer instances and
// processes using a stable OS-locked sidecar file.
func (w *Writer) Update(ctx context.Context, slug string, p UpdatePatch, opts ...UpdateOption) error {
	if err := parser.ValidateSlug(slug); err != nil {
		return fmt.Errorf("%w: %s", model.ErrSlugInvalid, slug)
	}
	return w.withIntentLock(slug, func() error {
		return w.updateLocked(ctx, slug, p, opts...)
	})
}

func (w *Writer) updateLocked(ctx context.Context, slug string, p UpdatePatch, opts ...UpdateOption) error {
	_ = ctx
	if p.Actor == "" {
		return errors.New("store: update requires actor")
	}

	// Apply options
	cfg := updateConfig{}
	for _, o := range opts {
		o(&cfg)
	}

	// Creator change gate
	if p.CreatedBy != nil && !cfg.allowCreatorChange {
		return ErrCreatorChangeNotAllowed
	}

	// Empty patch check (mirrors assumption.Update empty-patch semantics)
	if p.Symbol == nil && p.CreatedBy == nil && p.Reviewers == nil &&
		p.Status == nil && p.Assumptions == nil && p.References == nil &&
		p.DependsOn == nil && p.Body == nil {
		return ErrEmptyPatch
	}

	// Read existing file. Use resolveReadPath to accept both
	// physical forms (leaf and node) — Update does not change
	// the kind, it just patches fields in place. The resolved
	// path's suffix tells us which form to write back to.
	path, err := resolveReadPath(w.Root, slug)
	if err != nil {
		return err
	}
	existing, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%w: %s", model.ErrIntentNotFound, slug)
		}
		return fmt.Errorf("store: read %s: %w", path, err)
	}
	fmRaw, curBody, err := parser.SplitMarkdown(existing)
	if err != nil {
		return fmt.Errorf("store: split %s: %w", slug, err)
	}
	curFM, err := parser.ParseFrontmatter(fmRaw)
	if err != nil {
		return fmt.Errorf("store: parse %s: %w", slug, err)
	}

	// Determine the on-disk kind from the resolved path. Update
	// must write back to the SAME form it read from — otherwise
	// we silently flip a node intent to a leaf or vice versa.
	// Use store.KindFromPath (the canonical helper) rather than a
	// hand-rolled string check: the helper handles all the
	// edge cases (top-level "README.md" is leaf-shaped on disk,
	// any deeper "*/README.md" is node-shaped) in one place.
	existingKind := KindFromPath(path)

	// Apply patch
	if p.Symbol != nil {
		curFM.Symbol = *p.Symbol
	}
	if p.CreatedBy != nil {
		curFM.CreatedBy = *p.CreatedBy
	}
	if p.Reviewers != nil {
		curFM.Reviewers = *p.Reviewers
	}
	if p.Status != nil {
		if !validStatus(*p.Status) {
			return fmt.Errorf("store: invalid status %q (must be draft|active|superseded)", *p.Status)
		}
		curFM.Status = *p.Status
	}
	if p.Assumptions != nil {
		curFM.Assumptions = *p.Assumptions
	}
	if p.References != nil {
		curFM.References = *p.References
	}
	if p.DependsOn != nil {
		curFM.DependsOn = *p.DependsOn
	}
	// System field: always bump
	curFM.UpdatedAt = time.Now().UTC()

	// Body
	body := curBody
	if p.Body != nil {
		body = *p.Body
	}

	intent := &model.Intent{
		Slug:        slug,
		Kind:        existingKind,
		Frontmatter: curFM,
		Body:        body,
	}
	// Re-validate the patched shape BEFORE writing so a malformed
	// patch (e.g. empty CreatedBy) does not reach disk. The
	// round-trip cost is small for a single file; the trade-off
	// is "fail loudly in Update" vs "next Load blows up with a
	// confusing parse error".
	if err := parser.ValidateFrontmatter(&intent.Frontmatter); err != nil {
		return fmt.Errorf("store: patch invalid: %w", err)
	}
	content, err := parser.SerializeMarkdown(intent)
	if err != nil {
		return fmt.Errorf("store: serialize %s: %w", slug, err)
	}
	// Write back to the SAME form we read from. Use writePath
	// (the kind-aware path resolver) so the file lands in the
	// shape the user expects — read README.md, write README.md.
	writePath, err := w.writePath(existingKind, slug)
	if err != nil {
		return fmt.Errorf("store: resolve write path for update: %w", err)
	}
	if err := writeFileAtomic(writePath, content, 0o644); err != nil {
		return fmt.Errorf("store: write %s: %w", writePath, err)
	}
	return nil
}

// validStatus mirrors assumption.validStatus. Both packages share
// the same Status enum from model; duplicating the check here keeps
// the packages independent (assumption does not import store, store
// does not import assumption).
func validStatus(s model.Status) bool {
	switch s {
	case model.StatusDraft, model.StatusActive, model.StatusSuperseded:
		return true
	}
	return false
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
