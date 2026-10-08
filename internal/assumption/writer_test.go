package assumption

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xxx/kron/internal/model"
)

func strPtr(s string) *string { return &s }
func sevPtr(s model.Severity) *model.Severity {
	x := s
	return &x
}
func statPtr(s model.Status) *model.Status {
	x := s
	return &x
}

func TestWriter_Create(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	if err != nil {
		t.Fatal(err)
	}

	p := CreateParams{
		ID:              "single-region",
		Text:            "服务仅部署在单 region",
		DefaultSeverity: model.SeverityHard,
		CreatedBy:       "@zhangjun005",
	}
	if err := w.Create(context.Background(), p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	path := filepath.Join(dir, ".kron", "assumptions", "single-region.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(data) == 0 {
		t.Error("File is empty")
	}
	if !strings.Contains(string(data), `default_severity: hard`) {
		t.Errorf("File missing default_severity: hard\n%s", string(data))
	}
	if !strings.Contains(string(data), `status: active`) {
		t.Errorf("File missing status: active (default)\n%s", string(data))
	}
	if !strings.Contains(string(data), `updated_at:`) {
		t.Errorf("File missing updated_at\n%s", string(data))
	}
	// updated_at must NOT be the literal "TODO" string
	if strings.Contains(string(data), `updated_at: TODO`) {
		t.Errorf("updated_at must not be literal TODO\n%s", string(data))
	}
	// updated_at must look like RFC3339 (starts with a 4-digit year)
	if !strings.Contains(string(data), `updated_at: "202`) {
		t.Errorf("updated_at must be RFC3339 (year prefix 202x)\n%s", string(data))
	}

	// Duplicate create → ErrAssumptionExists
	if err := w.Create(context.Background(), p); err == nil {
		t.Error("Create duplicate: expected error, got nil")
	}
}

func TestWriter_Create_WithBody(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	if err != nil {
		t.Fatal(err)
	}

	body := "## Why\n\nWe don't have multi-region infrastructure yet.\n"
	p := CreateParams{
		ID:              "with-body",
		Text:            "has body",
		DefaultSeverity: model.SeveritySoft,
		CreatedBy:       "@alice",
		Body:            &body,
	}
	if err := w.Create(context.Background(), p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".kron", "assumptions", "with-body.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "We don't have multi-region infrastructure yet.") {
		t.Errorf("body content not written:\n%s", string(data))
	}
}

func TestWriter_Create_InvalidSeverity(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	if err != nil {
		t.Fatal(err)
	}
	p := CreateParams{
		ID:              "x",
		Text:            "x",
		DefaultSeverity: "invalid",
		CreatedBy:       "@a",
	}
	if err := w.Create(context.Background(), p); err == nil {
		t.Error("Create with invalid severity: expected error, got nil")
	}
}

func TestWriter_Create_EmptyText(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	if err != nil {
		t.Fatal(err)
	}
	p := CreateParams{
		ID:              "x",
		Text:            "",
		DefaultSeverity: model.SeverityHard,
		CreatedBy:       "@a",
	}
	if err := w.Create(context.Background(), p); err == nil {
		t.Error("Create with empty text: expected error, got nil")
	}
}

func TestWriter_Create_EmptyCreatedBy(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	if err != nil {
		t.Fatal(err)
	}
	p := CreateParams{
		ID:              "x",
		Text:            "x",
		DefaultSeverity: model.SeverityHard,
		CreatedBy:       "",
	}
	if err := w.Create(context.Background(), p); err == nil {
		t.Error("Create with empty created_by: expected error, got nil")
	}
}

func TestWriter_Create_InvalidStatus(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	if err != nil {
		t.Fatal(err)
	}
	bad := model.Status("bogus")
	p := CreateParams{
		ID:              "x",
		Text:            "x",
		DefaultSeverity: model.SeverityHard,
		CreatedBy:       "@a",
		Status:          &bad,
	}
	if err := w.Create(context.Background(), p); err == nil {
		t.Error("Create with invalid status: expected error, got nil")
	}
}

func TestWriter_Create_DefaultStatusActive(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	if err != nil {
		t.Fatal(err)
	}
	p := CreateParams{
		ID:              "x",
		Text:            "x",
		DefaultSeverity: model.SeverityHard,
		CreatedBy:       "@a",
		// Status: nil → default active
	}
	if err := w.Create(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".kron", "assumptions", "x.md"))
	if !strings.Contains(string(data), `status: active`) {
		t.Errorf("default status must be active:\n%s", string(data))
	}
}

// =====================================================================
// Update
// =====================================================================

func TestWriter_Update_PatchText(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Create(context.Background(), CreateParams{
		ID: "x", Text: "old", DefaultSeverity: model.SeverityHard, CreatedBy: "@a",
	}); err != nil {
		t.Fatal(err)
	}
	patch := UpdatePatch{
		Text:  strPtr("new text"),
		Actor: "@a",
	}
	if err := w.Update(context.Background(), "x", patch); err != nil {
		t.Fatalf("Update: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".kron", "assumptions", "x.md"))
	if !strings.Contains(string(data), "new text") {
		t.Errorf("text not updated:\n%s", string(data))
	}
}

func TestWriter_Update_BumpsUpdatedAt(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Create(context.Background(), CreateParams{
		ID: "x", Text: "x", DefaultSeverity: model.SeverityHard, CreatedBy: "@a",
	}); err != nil {
		t.Fatal(err)
	}
	beforeData, _ := os.ReadFile(filepath.Join(dir, ".kron", "assumptions", "x.md"))
	beforeUpdatedAt := extractUpdatedAt(t, string(beforeData))

	// Sleep 1s to ensure RFC3339 second-resolution timestamp differs.
	time.Sleep(1100 * time.Millisecond)

	patch := UpdatePatch{Text: strPtr("changed"), Actor: "@a"}
	if err := w.Update(context.Background(), "x", patch); err != nil {
		t.Fatal(err)
	}
	afterData, _ := os.ReadFile(filepath.Join(dir, ".kron", "assumptions", "x.md"))
	afterUpdatedAt := extractUpdatedAt(t, string(afterData))
	if afterUpdatedAt == beforeUpdatedAt {
		t.Errorf("updated_at not bumped: %s == %s", beforeUpdatedAt, afterUpdatedAt)
	}
}

func TestWriter_Update_EmptyPatch(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Create(context.Background(), CreateParams{
		ID: "x", Text: "x", DefaultSeverity: model.SeverityHard, CreatedBy: "@a",
	}); err != nil {
		t.Fatal(err)
	}
	patch := UpdatePatch{Actor: "@a"}
	if err := w.Update(context.Background(), "x", patch); err == nil {
		t.Error("empty patch: expected error, got nil")
	}
}

func TestWriter_Update_NotFound(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	if err != nil {
		t.Fatal(err)
	}
	patch := UpdatePatch{Text: strPtr("x"), Actor: "@a"}
	if err := w.Update(context.Background(), "nonexistent", patch); err == nil {
		t.Error("Update nonexistent: expected error, got nil")
	}
}

func TestWriter_Update_CreatorByRejectedWithoutOpt(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Create(context.Background(), CreateParams{
		ID: "x", Text: "x", DefaultSeverity: model.SeverityHard, CreatedBy: "@a",
	}); err != nil {
		t.Fatal(err)
	}
	newAuthor := "@new"
	patch := UpdatePatch{
		CreatedBy: &newAuthor,
		Actor:     "@a",
	}
	err = w.Update(context.Background(), "x", patch)
	if err == nil {
		t.Fatal("Update with CreatedBy without opt: expected error, got nil")
	}
	if !errorsIs(err, ErrCreatorChangeNotAllowed) {
		t.Errorf("Update: error = %v, want ErrCreatorChangeNotAllowed", err)
	}
	// File should be untouched
	data, _ := os.ReadFile(filepath.Join(dir, ".kron", "assumptions", "x.md"))
	if strings.Contains(string(data), "@new") {
		t.Errorf("file should not have been modified on rejected update:\n%s", string(data))
	}
}

func TestWriter_Update_CreatorByAllowedWithOpt(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Create(context.Background(), CreateParams{
		ID: "x", Text: "x", DefaultSeverity: model.SeverityHard, CreatedBy: "@a",
	}); err != nil {
		t.Fatal(err)
	}
	newAuthor := "@new"
	patch := UpdatePatch{
		CreatedBy: &newAuthor,
		Actor:     "@a",
	}
	if err := w.Update(context.Background(), "x", patch, WithAllowCreatorChange()); err != nil {
		t.Fatalf("Update with opt: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".kron", "assumptions", "x.md"))
	// YAML output may use single or double quotes depending on value
	// content; check for the value with either quoting style.
	if !strings.Contains(string(data), `created_by: "@new"`) &&
		!strings.Contains(string(data), `created_by: '@new'`) {
		t.Errorf("created_by not updated:\n%s", string(data))
	}
}

func TestWriter_Update_PatchStatus(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Create(context.Background(), CreateParams{
		ID: "x", Text: "x", DefaultSeverity: model.SeverityHard, CreatedBy: "@a",
	}); err != nil {
		t.Fatal(err)
	}
	patch := UpdatePatch{Status: statPtr(model.StatusDraft), Actor: "@a"}
	if err := w.Update(context.Background(), "x", patch); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".kron", "assumptions", "x.md"))
	if !strings.Contains(string(data), `status: draft`) {
		t.Errorf("status not updated:\n%s", string(data))
	}
}

func TestWriter_Update_PatchSeverity(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Create(context.Background(), CreateParams{
		ID: "x", Text: "x", DefaultSeverity: model.SeverityHard, CreatedBy: "@a",
	}); err != nil {
		t.Fatal(err)
	}
	patch := UpdatePatch{DefaultSeverity: sevPtr(model.SeveritySoft), Actor: "@a"}
	if err := w.Update(context.Background(), "x", patch); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".kron", "assumptions", "x.md"))
	if !strings.Contains(string(data), `default_severity: soft`) {
		t.Errorf("severity not updated:\n%s", string(data))
	}
}

func TestWriter_Update_PatchBody(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Create(context.Background(), CreateParams{
		ID: "x", Text: "x", DefaultSeverity: model.SeverityHard, CreatedBy: "@a",
	}); err != nil {
		t.Fatal(err)
	}
	newBody := "## New body"
	patch := UpdatePatch{Body: &newBody, Actor: "@a"}
	if err := w.Update(context.Background(), "x", patch); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".kron", "assumptions", "x.md"))
	if !strings.Contains(string(data), "## New body") {
		t.Errorf("body not updated:\n%s", string(data))
	}
}

// =====================================================================
// Delete / Restore
// =====================================================================

func TestWriter_Delete_Restore(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Create(context.Background(), CreateParams{
		ID: "x", Text: "x", DefaultSeverity: model.SeverityHard, CreatedBy: "@a",
	}); err != nil {
		t.Fatal(err)
	}

	// Delete moves file to trash
	if err := w.Delete(context.Background(), "x", "@deleter"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// File gone from active dir
	if _, err := os.Stat(filepath.Join(dir, ".kron", "assumptions", "x.md")); !os.IsNotExist(err) {
		t.Errorf("file should be gone from active dir after Delete")
	}
	// File present in trash
	trashData, err := os.ReadFile(filepath.Join(dir, ".kron", ".trash", "assumptions", "x.md"))
	if err != nil {
		t.Fatalf("trashed file missing: %v", err)
	}
	if !strings.Contains(string(trashData), `status: superseded`) {
		t.Errorf("trashed file should have status: superseded:\n%s", string(trashData))
	}

	// Restore brings it back
	if err := w.Restore(context.Background(), "x", "@restorer"); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	activeData, err := os.ReadFile(filepath.Join(dir, ".kron", "assumptions", "x.md"))
	if err != nil {
		t.Fatalf("restored file missing: %v", err)
	}
	if !strings.Contains(string(activeData), `status: active`) {
		t.Errorf("restored file should have status: active:\n%s", string(activeData))
	}
	// Trash should be empty
	if _, err := os.Stat(filepath.Join(dir, ".kron", ".trash", "assumptions", "x.md")); !os.IsNotExist(err) {
		t.Errorf("trash should be empty after Restore")
	}
}

func TestWriter_Delete_NotFound(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Delete(context.Background(), "nope", "@a"); err == nil {
		t.Error("Delete nonexistent: expected error, got nil")
	}
}

func TestWriter_Delete_AlreadyTrashed(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Create(context.Background(), CreateParams{
		ID: "x", Text: "x", DefaultSeverity: model.SeverityHard, CreatedBy: "@a",
	}); err != nil {
		t.Fatal(err)
	}
	if err := w.Delete(context.Background(), "x", "@a"); err != nil {
		t.Fatal(err)
	}
	err = w.Delete(context.Background(), "x", "@a")
	if err == nil {
		t.Error("Delete already trashed: expected error, got nil")
	}
	if !errorsIs(err, ErrAssumptionAlreadyDeleted) {
		t.Errorf("err = %v, want ErrAssumptionAlreadyDeleted", err)
	}
}

func TestWriter_Restore_NotInTrash(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Active file exists, nothing in trash
	if err := w.Create(context.Background(), CreateParams{
		ID: "x", Text: "x", DefaultSeverity: model.SeverityHard, CreatedBy: "@a",
	}); err != nil {
		t.Fatal(err)
	}
	err = w.Restore(context.Background(), "x", "@a")
	if err == nil {
		t.Error("Restore non-trashed: expected error, got nil")
	}
	if !errorsIs(err, ErrAssumptionNotInTrash) {
		t.Errorf("err = %v, want ErrAssumptionNotInTrash", err)
	}
}

// =====================================================================
// helpers
// =====================================================================

// extractUpdatedAt pulls the "updated_at: <value>" line and returns the value.
func extractUpdatedAt(t *testing.T, content string) string {
	t.Helper()
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "updated_at:") {
			return strings.TrimPrefix(line, "updated_at:")
		}
	}
	t.Fatalf("no updated_at in:\n%s", content)
	return ""
}
