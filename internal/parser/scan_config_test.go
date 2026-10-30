package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func writeScanFixture(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const anchorLine = "// @kron:intent auth/jwt\n"

func TestScanAnchors_NoConfigScansEverything(t *testing.T) {
	root := t.TempDir()
	writeScanFixture(t, root, "src/a.go", anchorLine)
	writeScanFixture(t, root, "notes.txt", "// @kron:intent notes/readme\n")

	anchors, err := ScanAnchors(root)
	if err != nil {
		t.Fatalf("ScanAnchors: %v", err)
	}
	if len(anchors) != 2 {
		t.Fatalf("expected 2 anchors without config, got %d: %#v", len(anchors), anchors)
	}
}

func TestScanAnchors_ConfigFiltersExtensionsAndGlobs(t *testing.T) {
	root := t.TempDir()
	writeScanFixture(t, root, ".kron/anchor-scan.json", `{
	  "version": 1,
	  "include": ["src/**"],
	  "exclude": ["src/generated/**"],
	  "languages": {"go": {"extensions": [".go"]}}
	}`)
	writeScanFixture(t, root, "src/a.go", anchorLine)
	writeScanFixture(t, root, "src/generated/b.go", anchorLine)
	writeScanFixture(t, root, "src/c.tsx", anchorLine)
	writeScanFixture(t, root, "other/d.go", anchorLine)

	anchors, err := ScanAnchors(root)
	if err != nil {
		t.Fatalf("ScanAnchors: %v", err)
	}
	if len(anchors) != 1 || anchors[0].FilePath != "src/a.go" {
		t.Fatalf("expected only src/a.go, got %#v", anchors)
	}
}

func TestScanAnchors_ConfigOnlyLanguages(t *testing.T) {
	root := t.TempDir()
	writeScanFixture(t, root, ".kron/anchor-scan.json", `{
	  "version": 1,
	  "languages": {"typescriptreact": {"extensions": [".tsx", ".jsx"]}}
	}`)
	writeScanFixture(t, root, "app/a.tsx", anchorLine)
	writeScanFixture(t, root, "app/b.go", anchorLine)

	anchors, err := ScanAnchors(root)
	if err != nil {
		t.Fatalf("ScanAnchors: %v", err)
	}
	if len(anchors) != 1 || anchors[0].FilePath != "app/a.tsx" {
		t.Fatalf("expected only tsx file, got %#v", anchors)
	}
}

func TestScanAnchors_MalformedConfigIsError(t *testing.T) {
	root := t.TempDir()
	writeScanFixture(t, root, ".kron/anchor-scan.json", "{not valid json")

	if _, err := ScanAnchors(root); err == nil {
		t.Fatal("malformed anchor-scan.json must fail the scan")
	}
}

func TestMatchGlob(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"src/**", "src/a/b.go", true},
		{"src/**", "src/a.go", true},
		{"src/**", "other/a.go", false},
		{"**/*.min.ts", "src/a.min.ts", true},
		{"**/*.min.ts", "src/nested/a.min.ts", true},
		{"**/*.min.ts", "src/a.ts", false},
		{"*.go", "a.go", true},
		{"*.go", "a/b.go", false},
		{"src/?.go", "src/a.go", true},
		{"src/?.go", "src/ab.go", false},
	}
	for _, tc := range cases {
		if got := matchGlob(tc.pattern, tc.path); got != tc.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", tc.pattern, tc.path, got, tc.want)
		}
	}
}
