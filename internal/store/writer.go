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
	"strings"
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
func (w *Writer) Exists(ctx context.Context, slug string) bool {
	_ = ctx
	if slug == "" {
		return false
	}
	if _, err := os.Stat(w.IntentPath(slug)); err == nil {
		return true
	}
	// README shorthand (delegate to shared helper for parity with
	// Reader.resolveIntentPath; the Reader also handles the
	// "README" top-level special case).
	if path, err := resolveReadPath(w.Root, slug); err == nil {
		_ = path
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

// MoveToTrash soft-deletes an intent: <root>/.kron/intents/<slug>.md
// is moved to <root>/.kron/.trash/<slug>.md. The source can be
// either physical form (leaf <slug>.md or node <slug>/README.md)
// — the trashed copy is always the canonical form (<slug>.md in
// .trash/), so a subsequent RestoreFromTrash finds it under the
// canonical name. See RFC 2026-10-08-writer-readme-symmetry §3.3.
//
// Returns model.ErrIntentNotFound (wrapped) if the intent does
// not exist in either form. Returns model.ErrIntentExists
// (wrapped) if the trashed file already exists (so the move
// cannot clobber a prior deletion); the caller is expected to
// decide whether to surface this to the user or restore and
// retry.
//
// The original intents directory entry is removed BEFORE the
// rename so that a crash between the two steps leaves either
// zero or one file with the slug — never two, never zero with
// a phantom path.
func (w *Writer) MoveToTrash(ctx context.Context, slug string) error {
	_ = ctx
	src, err := resolveReadPath(w.Root, slug)
	if err != nil {
		return err
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

// RestoreFromTrash is the inverse of MoveToTrash: a trashed
// intent is moved back to .kron/intents/. The trashed file
// always lives at <root>/.kron/.trash/<slug>.md (MoveToTrash
// collapses both physical forms to the canonical leaf name
// in .trash/), so this function restores to the canonical
// <slug>.md location.
//
// IMPORTANT: if the trashed intent was ORIGINALLY a node-form
// README at <slug>/README.md, RestoreFromTrash does NOT
// automatically recreate that shape — it writes <slug>.md.
// To restore as a node intent, the caller should restore and
// then call Write with Kind=Node (overwriting the leaf file
// with the README form). This trade-off is documented in
// RFC 2026-10-08-writer-readme-symmetry §3.3.
//
// Returns model.ErrIntentNotFound (wrapped) if no trashed file
// exists. Returns model.ErrIntentExists (wrapped) if an active
// intent with the same slug already exists (so restore cannot
// clobber the live version).
func (w *Writer) RestoreFromTrash(ctx context.Context, slug string) error {
	_ = ctx
	src := w.TrashPath(slug)
	if _, err := os.Stat(src); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%w: %s not in trash", model.ErrIntentNotFound, slug)
		}
		return fmt.Errorf("store: stat %s: %w", src, err)
	}

	// Determine the restore target shape. If the canonical
	// <slug>.md exists in the live intents dir, we must NOT
	// clobber it (returns ErrIntentExists below). If the
	// <slug>/README.md exists, we restore the leaf form
	// alongside it — collisions on the OTHER form are not
	// possible here because MoveToTrash would have moved
	// either form; we only see the file that was actually
	// moved. So writing to the canonical leaf path is safe.
	dst := filepath.Join(w.Root, model.KronDir, model.IntentDir, slug+model.IntentExtension)
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
// TODO(v1.4+): last-write-wins on concurrent updates. Consider etag.
func (w *Writer) Update(ctx context.Context, slug string, p UpdatePatch, opts ...UpdateOption) error {
	_ = ctx
	if err := parser.ValidateSlug(slug); err != nil {
		return fmt.Errorf("%w: %s", model.ErrSlugInvalid, slug)
	}
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
	// The path either ends in "/README.md" (node) or ".md"
	// (leaf / canonical). The top-level "README.md" (no slash
	// before README) is also node-shaped by convention, so
	// the path's suffix decides.
	var existingKind model.IntentKind
	if strings.HasSuffix(path, string(filepath.Separator)+"README.md") {
		existingKind = model.IntentKindNode
	} else {
		existingKind = model.IntentKindLeaf
	}

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
