package assumption

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xxx/kron/internal/model"
)

func TestWriter_Create(t *testing.T) {
	dir := t.TempDir()

	w, err := NewWriter(dir)
	if err != nil {
		t.Fatal(err)
	}

	err = w.Create(nil, "single-region", "服务仅部署在单 region", model.SeverityHard, "@zhangjun005")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// File should exist.
	path := filepath.Join(dir, ".kron", "assumptions", "single-region.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(data) == 0 {
		t.Error("File is empty")
	}

	// Should not allow double-create.
	err = w.Create(nil, "single-region", "服务仅部署在单 region", model.SeverityHard, "@zhangjun005")
	if err == nil {
		t.Error("Create duplicate: expected error, got nil")
	}
}

func TestWriter_Create_InvalidSeverity(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	if err != nil {
		t.Fatal(err)
	}

	err = w.Create(nil, "single-region", "服务仅部署在单 region", "invalid", "@zhangjun005")
	if err == nil {
		t.Error("Create with invalid severity: expected error, got nil")
	}
}

func TestWriter_Create_EmptyText(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	if err != nil {
		t.Fatal(err)
	}

	err = w.Create(nil, "single-region", "", model.SeverityHard, "@zhangjun005")
	if err == nil {
		t.Error("Create with empty text: expected error, got nil")
	}
}

func TestWriter_Update(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, ".kron", "assumptions")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	// Pre-create a file.
	fixture := `<!-- kron:frontmatter -->
id: single-region
text: 服务仅部署在单 region
default_severity: hard
created_by: "@zhangjun005"
updated_at: "2026-09-22T10:00:00Z"
<!-- /kron:frontmatter -->
`
	if err := os.WriteFile(filepath.Join(sub, "single-region.md"), []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}

	w, err := NewWriter(dir)
	if err != nil {
		t.Fatal(err)
	}

	fm := &model.AssumptionFrontmatter{
		ID:              "single-region",
		Text:            "服务仅部署在单 region, 无跨区时钟漂移",
		DefaultSeverity: model.SeverityHard,
		CreatedBy:       "@zhangjun005",
		UpdatedAt:       "2026-09-22T10:00:00Z",
	}
	err = w.Update(nil, "single-region", fm, "# Single Region\n\n> Updated body.")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(sub, "single-region.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("File is empty after update")
	}
}

func TestWriter_Update_NotFound(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	if err != nil {
		t.Fatal(err)
	}

	fm := &model.AssumptionFrontmatter{
		ID:              "nonexistent",
		Text:            "test",
		DefaultSeverity: model.SeveritySoft,
		CreatedBy:       "@alice",
		UpdatedAt:       "2026-09-22T10:00:00Z",
	}
	err = w.Update(nil, "nonexistent", fm, "")
	if err == nil {
		t.Error("Update nonexistent: expected error, got nil")
	}
}
