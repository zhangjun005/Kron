package assumption

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/xxx/kron/internal/model"
)

// errorsIs is a tiny local alias to keep the test file's import list tight.
func errorsIs(err, target error) bool { return errors.Is(err, target) }

func TestReader_Get(t *testing.T) {
	dir := t.TempDir()
	// Create .kron/assumptions/ subdir.
	sub := filepath.Join(dir, ".kron", "assumptions")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a valid assumption file.
	fixture := `<!-- kron:frontmatter -->
id: single-region
text: 服务仅部署在单 region
default_severity: hard
created_by: "@zhangjun005"
updated_at: "2026-09-22T10:00:00Z"
<!-- /kron:frontmatter -->

# Single Region

> 服务仅部署在单 region，无跨区时钟漂移问题。
`
	if err := os.WriteFile(filepath.Join(sub, "single-region.md"), []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := NewReader(dir)
	if err != nil {
		t.Fatal(err)
	}

	af, err := r.Get(nil, "single-region")
	if err != nil {
		t.Fatalf("Get(single-region): %v", err)
	}
	if af.Slug != "single-region" {
		t.Errorf("Slug = %q, want single-region", af.Slug)
	}
	if af.Frontmatter.ID != "single-region" {
		t.Errorf("Frontmatter.ID = %q, want single-region", af.Frontmatter.ID)
	}
	if af.Frontmatter.DefaultSeverity != model.SeverityHard {
		t.Errorf("DefaultSeverity = %v, want SeverityHard", af.Frontmatter.DefaultSeverity)
	}
	if af.Frontmatter.Text == "" {
		t.Error("Text is empty")
	}
	if af.SourcePath == "" {
		t.Error("SourcePath not set")
	}
}

func TestReader_Get_NotFound(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, ".kron", "assumptions")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	r, err := NewReader(dir)
	if err != nil {
		t.Fatal(err)
	}

	_, err = r.Get(nil, "nonexistent")
	if err == nil {
		t.Fatal("Get(nonexistent): expected error, got nil")
	}
	if !errorsIs(err, ErrAssumptionNotFound) {
		t.Errorf("Get(nonexistent): error = %v, want ErrAssumptionNotFound", err)
	}
}

func TestReader_List(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, ".kron", "assumptions")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"single-region.md": `<!-- kron:frontmatter -->
id: single-region
text: Single region
default_severity: hard
created_by: "@alice"
updated_at: "2026-09-22T10:00:00Z"
<!-- /kron:frontmatter -->`,
		"redis-availability.md": `<!-- kron:frontmatter -->
id: redis-availability
text: Redis 可用性 ≥ 99.9%
default_severity: soft
created_by: "@bob"
updated_at: "2026-09-22T11:00:00Z"
<!-- /kron:frontmatter -->`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(sub, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	r, err := NewReader(dir)
	if err != nil {
		t.Fatal(err)
	}

	list, err := r.List(nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("List count = %d, want 2", len(list))
	}
	// Must be sorted.
	if list[0].Slug != "redis-availability" || list[1].Slug != "single-region" {
		t.Errorf("List not sorted: %v", []string{list[0].Slug, list[1].Slug})
	}
}

func TestReader_List_DirNotFound(t *testing.T) {
	dir := t.TempDir() // no .kron/assumptions/
	_, err := NewReader(dir)
	if err != ErrAssumptionDirNotFound {
		t.Errorf("NewReader: error = %v, want ErrAssumptionDirNotFound", err)
	}
}

func TestReader_Exists(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, ".kron", "assumptions")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "single-region.md"), []byte("<!-- kron:frontmatter -->\nid: single-region\n<!-- /kron:frontmatter -->"), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := NewReader(dir)
	if err != nil {
		t.Fatal(err)
	}

	if !r.Exists(nil, "single-region") {
		t.Error("Exists(single-region): got false, want true")
	}
	if r.Exists(nil, "nonexistent") {
		t.Error("Exists(nonexistent): got true, want false")
	}
}
