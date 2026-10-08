package assumption

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/parser"
)

// Writer creates assumption files in .kron/assumptions/.
type Writer struct {
	root string // absolute path to .kron/assumptions/
}

// NewWriter returns a Writer rooted at root (repository root, not .kron/assumptions/).
// It ensures the .kron/assumptions/ directory exists.
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
	return &Writer{root: dir}, nil
}

// Create writes a new assumption file .kron/assumptions/<id>.md.
// It returns ErrAssumptionExists if the file already exists.
// createdBy is required (format: "@user" or "agent:<model>").
func (w *Writer) Create(ctx context.Context, id, text string, defaultSeverity model.Severity, createdBy string) error {
	_ = ctx
	if err := parser.ValidateSlug(id); err != nil {
		return fmt.Errorf("%w: %s", ErrAssumptionNotFound, id)
	}
	if text == "" {
		return errors.New("assumption: text is required")
	}
	if defaultSeverity != model.SeverityHard && defaultSeverity != model.SeveritySoft {
		return fmt.Errorf("assumption: invalid severity %q (must be hard or soft)", defaultSeverity)
	}
	if createdBy == "" {
		return errors.New("assumption: created_by is required")
	}

	path := filepath.Join(w.root, id+".md")
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%w: %s", ErrAssumptionExists, id)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("assumption: stat %s: %w", path, err)
	}

	fm := model.AssumptionFrontmatter{
		ID:              id,
		Text:            text,
		DefaultSeverity: defaultSeverity,
		CreatedBy:       createdBy,
		UpdatedAt:       "TODO", // caller should set via Update immediately after
	}

	content, err := serializeAssumptionFile(fm, "")
	if err != nil {
		return fmt.Errorf("assumption: serialize %s: %w", id, err)
	}
	if err := writeFileAtomic(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("assumption: write %s: %w", path, err)
	}
	return nil
}

// Update overwrites an existing assumption file. It returns ErrAssumptionNotFound
// if the file does not exist. The frontmatter.ID must match the id parameter.
func (w *Writer) Update(ctx context.Context, id string, fm *model.AssumptionFrontmatter, body string) error {
	_ = ctx
	if err := parser.ValidateSlug(id); err != nil {
		return fmt.Errorf("%w: %s", ErrAssumptionNotFound, id)
	}
	if fm == nil {
		return errors.New("assumption: frontmatter is nil")
	}
	if fm.ID != id {
		return fmt.Errorf("%w: frontmatter id %q != path id %q", ErrAssumptionFileIdMismatch, fm.ID, id)
	}

	path := filepath.Join(w.root, id+".md")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("%w: %s", ErrAssumptionNotFound, id)
	} else if err != nil {
		return fmt.Errorf("assumption: stat %s: %w", path, err)
	}

	content, err := serializeAssumptionFile(*fm, body)
	if err != nil {
		return fmt.Errorf("assumption: serialize %s: %w", id, err)
	}
	if err := writeFileAtomic(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("assumption: write %s: %w", path, err)
	}
	return nil
}

// Exists reports whether an assumption with the given id exists.
func (w *Writer) Exists(ctx context.Context, id string) bool {
	_ = ctx
	path := filepath.Join(w.root, id+".md")
	_, err := os.Stat(path)
	return err == nil
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
