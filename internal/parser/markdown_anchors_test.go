package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanMarkdownAnchors_BasicAnchor(t *testing.T) {
	dir := t.TempDir()
	md := `// @kron:intent auth/jwt

# Heading
`
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	anchors, err := ScanMarkdownAnchors(dir)
	if err != nil {
		t.Fatalf("ScanMarkdownAnchors: %v", err)
	}
	if len(anchors) != 1 {
		t.Fatalf("anchors count = %d, want 1", len(anchors))
	}
	if anchors[0].Slug != "auth/jwt" {
		t.Errorf("Slug = %q, want auth/jwt", anchors[0].Slug)
	}
	if anchors[0].LineNumber != 1 {
		t.Errorf("LineNumber = %d, want 1", anchors[0].LineNumber)
	}
}

func TestScanMarkdownAnchors_SkipsInsideFence(t *testing.T) {
	dir := t.TempDir()
	md := `# Heading

` + "```" + `
// @kron:intent inside-fence
` + "```" + `

// @kron:intent outside-fence
`
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	anchors, err := ScanMarkdownAnchors(dir)
	if err != nil {
		t.Fatalf("ScanMarkdownAnchors: %v", err)
	}
	if len(anchors) != 1 {
		t.Fatalf("anchors count = %d, want 1 (in-fence must be skipped)", len(anchors))
	}
	if anchors[0].Slug != "outside-fence" {
		t.Errorf("Slug = %q, want outside-fence", anchors[0].Slug)
	}
	if anchors[0].LineNumber != 7 {
		t.Errorf("LineNumber = %d, want 7", anchors[0].LineNumber)
	}
}

func TestScanMarkdownAnchors_TildeFence(t *testing.T) {
	dir := t.TempDir()
	md := `~~~
# @kron:intent inside-tilde-fence
~~~

// @kron:intent outside-tilde
`
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	anchors, err := ScanMarkdownAnchors(dir)
	if err != nil {
		t.Fatalf("ScanMarkdownAnchors: %v", err)
	}
	if len(anchors) != 1 {
		t.Fatalf("anchors count = %d, want 1", len(anchors))
	}
	if anchors[0].Slug != "outside-tilde" {
		t.Errorf("Slug = %q, want outside-tilde", anchors[0].Slug)
	}
}

func TestScanMarkdownAnchors_MalformedSlugSkipped(t *testing.T) {
	dir := t.TempDir()
	md := `// @kron:intent BAD-SLUG
// @kron:intent valid-slug
`
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	anchors, err := ScanMarkdownAnchors(dir)
	if err != nil {
		t.Fatalf("ScanMarkdownAnchors: %v", err)
	}
	if len(anchors) != 1 {
		t.Fatalf("anchors count = %d, want 1 (malformed slug skipped)", len(anchors))
	}
	if anchors[0].Slug != "valid-slug" {
		t.Errorf("Slug = %q, want valid-slug", anchors[0].Slug)
	}
}

func TestScanMarkdownAnchors_MultipleFilesSorted(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "z.md"), []byte("// @kron:intent z-anchor\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte("// @kron:intent a-anchor\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	anchors, err := ScanMarkdownAnchors(dir)
	if err != nil {
		t.Fatalf("ScanMarkdownAnchors: %v", err)
	}
	if len(anchors) != 2 {
		t.Fatalf("anchors count = %d, want 2", len(anchors))
	}
	// Sorted by (file_path, line_number).
	if anchors[0].Slug != "a-anchor" {
		t.Errorf("anchors[0] = %q, want a-anchor", anchors[0].Slug)
	}
	if anchors[1].Slug != "z-anchor" {
		t.Errorf("anchors[1] = %q, want z-anchor", anchors[1].Slug)
	}
}
