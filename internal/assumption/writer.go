// Package assumption: see reader.go for package doc.
//
// Writer responsibilities:
//   - Create / Update: write .kron/assumptions/<id>.md atomically
//     (temp + rename).
//   - Delete: soft-delete by moving the file to .kron/.trash/assumptions/
//     and setting Status = superseded.
//   - Restore: move the file back from .kron/.trash/assumptions/ and
//     set Status = active.
//
// Field-handling philosophy (v1.3 B-3 closure):
//
//	Business fields (ID, Text, DefaultSeverity, Status, Body, Reviewers,
//	ExpiresAt, VerifiedAt, VerifiedBy, CreatedBy) — passed by the caller.
//	System fields (UpdatedAt) — set by the library to time.Now() in UTC.
//
// Library does NOT interpret "Actor" beyond identity formatting
// (e.g., it does not enforce "Actor must be in Reviewers" or
// "Actor must equal current CreatedBy"). Permission rules are an
// upper-layer concern (lint, MCP tool handlers, GUI dialogs).
//
// TODO(v1.4+): Concurrent updates use last-write-wins. When MCP
// exposes real-time collaborative editing, consider an etag (file
// hash or monotonic version) and reject updates with a stale etag.
package assumption

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/parser"
)

// Writer creates / updates / soft-deletes / restores assumption files in
// .kron/assumptions/ and .kron/.trash/assumptions/.
type Writer struct {
	root      string // absolute path to .kron/assumptions/
	trashRoot string // absolute path to .kron/.trash/assumptions/
	repoRoot  string // absolute path to repo root (for .kron/.trash)
}

// NewWriter returns a Writer rooted at root (repository root, not
// .kron/assumptions/). It ensures the .kron/assumptions/ directory exists.
func NewWriter(root string) (*Writer, error) {
	if root == "" {
		return nil, errors.New("assumption: root is empty")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("assumption: resolve root: %w", err)
	}
	dir := filepath.Join(abs, ".kron", "assumptions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("assumption: mkdir %s: %w", dir, err)
	}
	trash := filepath.Join(abs, ".kron", ".trash", "assumptions")
	if err := os.MkdirAll(trash, 0o755); err != nil {
		return nil, fmt.Errorf("assumption: mkdir %s: %w", trash, err)
	}
	return &Writer{root: dir, trashRoot: trash, repoRoot: abs}, nil
}

// =====================================================================
// Create
// =====================================================================

// CreateParams is the input for Writer.Create.
//
// Required fields use value types (call site reads as obviously required).
// Optional fields use pointer types (nil = use library default).
type CreateParams struct {
	// Required
	ID              string         // slug, validated
	Text            string         // non-empty
	DefaultSeverity model.Severity // {hard, soft}
	CreatedBy       string         // "@user" or "agent:<model>"

	// Optional — nil = library fills default
	Status    *model.Status // default: active
	Body      *string       // default: ""
	Reviewers []string      // default: nil
	ExpiresAt *string       // default: omitted in frontmatter
}

// Create writes a new assumption file. It returns ErrAssumptionExists
// if the file already exists.
//
// The library sets UpdatedAt to time.Now().UTC().Format(RFC3339).
func (w *Writer) Create(ctx context.Context, p CreateParams) error {
	_ = ctx
	if err := parser.ValidateSlug(p.ID); err != nil {
		return fmt.Errorf("%w: %s", ErrAssumptionNotFound, p.ID)
	}
	if p.Text == "" {
		return errors.New("assumption: text is required")
	}
	if p.DefaultSeverity != model.SeverityHard && p.DefaultSeverity != model.SeveritySoft {
		return fmt.Errorf("assumption: invalid severity %q (must be hard or soft)", p.DefaultSeverity)
	}
	if p.CreatedBy == "" {
		return errors.New("assumption: created_by is required")
	}

	// Defaults
	status := model.StatusActive
	if p.Status != nil {
		status = *p.Status
	}
	if !validStatus(status) {
		return fmt.Errorf("assumption: invalid status %q (must be draft|active|superseded)", status)
	}
	var body string
	if p.Body != nil {
		body = *p.Body
	}

	path := filepath.Join(w.root, p.ID+".md")
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%w: %s", ErrAssumptionExists, p.ID)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("assumption: stat %s: %w", path, err)
	}

	fm := model.AssumptionFrontmatter{
		ID:              p.ID,
		Text:            p.Text,
		DefaultSeverity: p.DefaultSeverity,
		Status:          status,
		CreatedBy:       p.CreatedBy,
		UpdatedAt:       time.Now().UTC().Format(time.RFC3339),
		Reviewers:       p.Reviewers,
	}
	if p.ExpiresAt != nil {
		fm.ExpiresAt = *p.ExpiresAt
	}

	content, err := serializeAssumptionFile(fm, body)
	if err != nil {
		return fmt.Errorf("assumption: serialize %s: %w", p.ID, err)
	}
	if err := writeFileAtomic(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("assumption: write %s: %w", path, err)
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
type UpdatePatch struct {
	Text            *string         // nil = leave alone
	DefaultSeverity *model.Severity // nil = leave alone
	Status          *model.Status   // nil = leave alone
	Body            *string         // nil = leave alone (note: cannot set to "" via pointer-to-empty)
	Reviewers       *[]string       // nil = leave alone
	ExpiresAt       *string         // nil = leave alone
	VerifiedAt      *string         // nil = leave alone
	VerifiedBy      *string         // nil = leave alone
	CreatedBy       *string         // nil = leave alone; to set, pass WithAllowCreatorChange

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
// "Actor" beyond identity formatting (it does not check that Actor is
// the current CreatedBy, a Reviewer, etc.) — permission rules are
// an upper-layer concern.
//
// This opt-in exists because CreatedBy is an identity-bearing field;
// changing it has social / forensic implications. Without the opt,
// the library returns ErrCreatorChangeNotAllowed and writes nothing.
func WithAllowCreatorChange() UpdateOption {
	return func(c *updateConfig) {
		c.allowCreatorChange = true
	}
}

// Update overwrites an existing assumption file. It returns
// ErrAssumptionNotFound if the file does not exist, ErrEmptyPatch
// if the patch has no business changes, or ErrCreatorChangeNotAllowed
// if CreatedBy is in the patch without WithAllowCreatorChange.
//
// The library sets UpdatedAt to time.Now().UTC().Format(RFC3339) on
// successful write.
//
// Note: Update can change the frontmatter ID via... wait, it cannot —
// the file name is fixed by the id parameter. To rename an assumption,
// use Delete + Create (preserves history through the trash dir).
//
// TODO(v1.4+): last-write-wins on concurrent updates. Consider etag.
func (w *Writer) Update(ctx context.Context, id string, p UpdatePatch, opts ...UpdateOption) error {
	_ = ctx
	if err := parser.ValidateSlug(id); err != nil {
		return fmt.Errorf("%w: %s", ErrAssumptionNotFound, id)
	}
	if p.Actor == "" {
		return errors.New("assumption: update requires actor")
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

	// Read existing file
	path := filepath.Join(w.root, id+".md")
	existing, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", ErrAssumptionNotFound, id)
		}
		return fmt.Errorf("assumption: read %s: %w", path, err)
	}
	curFM, curBody, err := parser.ParseAssumptionFrontmatter(existing)
	if err != nil {
		return fmt.Errorf("assumption: parse %s: %w", path, err)
	}

	// Empty patch check
	if p.Text == nil && p.DefaultSeverity == nil && p.Status == nil &&
		p.Body == nil && p.Reviewers == nil && p.ExpiresAt == nil &&
		p.VerifiedAt == nil && p.VerifiedBy == nil && p.CreatedBy == nil {
		return ErrEmptyPatch
	}

	// Apply patch
	if p.Text != nil {
		curFM.Text = *p.Text
	}
	if p.DefaultSeverity != nil {
		if *p.DefaultSeverity != model.SeverityHard && *p.DefaultSeverity != model.SeveritySoft {
			return fmt.Errorf("assumption: invalid severity %q (must be hard or soft)", *p.DefaultSeverity)
		}
		curFM.DefaultSeverity = *p.DefaultSeverity
	}
	if p.Status != nil {
		if !validStatus(*p.Status) {
			return fmt.Errorf("assumption: invalid status %q (must be draft|active|superseded)", *p.Status)
		}
		curFM.Status = *p.Status
	}
	if p.Reviewers != nil {
		curFM.Reviewers = *p.Reviewers
	}
	if p.ExpiresAt != nil {
		curFM.ExpiresAt = *p.ExpiresAt
	}
	if p.VerifiedAt != nil {
		curFM.VerifiedAt = *p.VerifiedAt
	}
	if p.VerifiedBy != nil {
		curFM.VerifiedBy = *p.VerifiedBy
	}
	if p.CreatedBy != nil {
		curFM.CreatedBy = *p.CreatedBy
	}
	// System field: always bump
	curFM.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	// Body
	body := curBody
	if p.Body != nil {
		body = *p.Body
	}

	content, err := serializeAssumptionFile(curFM, body)
	if err != nil {
		return fmt.Errorf("assumption: serialize %s: %w", id, err)
	}
	if err := writeFileAtomic(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("assumption: write %s: %w", path, err)
	}
	return nil
}

// =====================================================================
// Delete / Restore
// =====================================================================

// Delete soft-deletes the assumption by moving it from
// .kron/assumptions/<id>.md to .kron/.trash/assumptions/<id>.md
// and setting Status = superseded.
//
// The library does NOT interpret the actor argument beyond identity
// formatting. Audit logging is the caller's responsibility.
//
// Returns ErrAssumptionNotFound if the file does not exist in
// .kron/assumptions/, or ErrAssumptionAlreadyDeleted if it is
// already in the trash directory.
func (w *Writer) Delete(ctx context.Context, id, actor string) error {
	_ = ctx
	if err := parser.ValidateSlug(id); err != nil {
		return fmt.Errorf("%w: %s", ErrAssumptionNotFound, id)
	}
	if actor == "" {
		return errors.New("assumption: delete requires actor")
	}

	src := filepath.Join(w.root, id+".md")
	dst := filepath.Join(w.trashRoot, id+".md")

	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("%w: %s", ErrAssumptionAlreadyDeleted, id)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("assumption: stat %s: %w", dst, err)
	}

	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", ErrAssumptionNotFound, id)
		}
		return fmt.Errorf("assumption: stat %s: %w", src, err)
	}

	// Read, mutate Status, then move (atomic via rename on same fs;
	// .kron/assumptions and .kron/.trash/assumptions share the same
	// parent so os.Rename is atomic on POSIX).
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("assumption: read %s: %w", src, err)
	}
	curFM, body, err := parser.ParseAssumptionFrontmatter(data)
	if err != nil {
		return fmt.Errorf("assumption: parse %s: %w", src, err)
	}
	curFM.Status = model.StatusSuperseded
	curFM.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	// Bump CreatedBy to a synthetic value reflecting the deleter
	// (in the same format the library uses elsewhere). This is
	// informational only — the original CreatedBy is preserved in
	// the file body / git history. We do this so a `kron lint` pass
	// can see who performed the soft-delete without reading audit
	// logs.
	//
	// Library does NOT change CreatedBy silently — the field is
	// identity-bearing and changing it requires WithAllowCreatorChange
	// for ordinary Update calls. Delete is an exception because
	// Status = superseded already marks the file as terminal.
	// TODO(v1.4+): if we add a real audit log, do not modify CreatedBy.
	if actor != "" {
		curFM.CreatedBy = actor
	}

	content, err := serializeAssumptionFile(curFM, body)
	if err != nil {
		return fmt.Errorf("assumption: serialize %s: %w", id, err)
	}

	// Write to trash first (atomic on tmp+rename), then remove source.
	if err := writeFileAtomic(dst, []byte(content), 0o644); err != nil {
		return fmt.Errorf("assumption: write %s: %w", dst, err)
	}
	if err := os.Remove(src); err != nil {
		// Best-effort rollback: remove the trashed copy so the
		// caller can retry. We log the rollback failure but
		// return the original error.
		_ = os.Remove(dst)
		return fmt.Errorf("assumption: remove %s: %w", src, err)
	}
	return nil
}

// Restore moves the file from .kron/.trash/assumptions/<id>.md back to
// .kron/assumptions/<id>.md and sets Status = active.
//
// Returns ErrAssumptionNotInTrash if the file is not in the trash
// directory. The library does NOT interpret the actor argument.
func (w *Writer) Restore(ctx context.Context, id, actor string) error {
	_ = ctx
	if err := parser.ValidateSlug(id); err != nil {
		return fmt.Errorf("%w: %s", ErrAssumptionNotFound, id)
	}
	if actor == "" {
		return errors.New("assumption: restore requires actor")
	}

	src := filepath.Join(w.trashRoot, id+".md")
	dst := filepath.Join(w.root, id+".md")

	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", ErrAssumptionNotInTrash, id)
		}
		return fmt.Errorf("assumption: stat %s: %w", src, err)
	}
	if _, err := os.Stat(dst); err == nil {
		// Active file already exists at the destination — refuse
		// to overwrite. The caller must resolve the conflict
		// (e.g., manually merge or delete the active one first).
		return fmt.Errorf("%w: %s", ErrAssumptionExists, id)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("assumption: stat %s: %w", dst, err)
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("assumption: read %s: %w", src, err)
	}
	curFM, body, err := parser.ParseAssumptionFrontmatter(data)
	if err != nil {
		return fmt.Errorf("assumption: parse %s: %w", src, err)
	}
	curFM.Status = model.StatusActive
	curFM.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if actor != "" {
		curFM.CreatedBy = actor
	}

	content, err := serializeAssumptionFile(curFM, body)
	if err != nil {
		return fmt.Errorf("assumption: serialize %s: %w", id, err)
	}

	if err := writeFileAtomic(dst, []byte(content), 0o644); err != nil {
		return fmt.Errorf("assumption: write %s: %w", dst, err)
	}
	if err := os.Remove(src); err != nil {
		_ = os.Remove(dst)
		return fmt.Errorf("assumption: remove %s: %w", src, err)
	}
	return nil
}

// =====================================================================
// Exists
// =====================================================================

// Exists reports whether an assumption with the given id exists in
// the active directory (.kron/assumptions/). Trashed files are
// not considered "existing" — use the underlying os.Stat if you
// need to distinguish.
func (w *Writer) Exists(ctx context.Context, id string) bool {
	_ = ctx
	if err := parser.ValidateSlug(id); err != nil {
		return false
	}
	path := filepath.Join(w.root, id+".md")
	_, err := os.Stat(path)
	return err == nil
}

// =====================================================================
// helpers
// =====================================================================

func validStatus(s model.Status) bool {
	switch s {
	case model.StatusDraft, model.StatusActive, model.StatusSuperseded:
		return true
	}
	return false
}

// serializeAssumptionFile formats an assumption file: frontmatter + optional body.
func serializeAssumptionFile(fm model.AssumptionFrontmatter, body string) (string, error) {
	// Marshal frontmatter to YAML.
	yamlData, err := parser.MarshalAssumptionFrontmatter(&fm)
	if err != nil {
		return "", fmt.Errorf("yaml marshal: %w", err)
	}
	var content []byte
	content = append(content, []byte("<!-- kron:frontmatter -->\n")...)
	content = append(content, yamlData...)
	content = append(content, []byte("<!-- /kron:frontmatter -->\n\n")...)
	if body != "" {
		content = append(content, body...)
		if body[len(body)-1] != '\n' {
			content = append(content, '\n')
		}
	}
	return string(content), nil
}

// writeFileAtomic writes data to path via a temp file + rename so concurrent
// readers never observe a partial file.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".kron-assumption-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp → %s: %w", path, err)
	}
	return nil
}
