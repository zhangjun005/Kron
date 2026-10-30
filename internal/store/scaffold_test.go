package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/xxx/kron/internal/parser"
)

func TestEnsureKronScaffold_CreatesDefaults(t *testing.T) {
	root := t.TempDir()
	if err := EnsureKronScaffold(context.Background(), root); err != nil {
		t.Fatalf("EnsureKronScaffold: %v", err)
	}
	for _, rel := range []string{
		".kron/config.toml",
		parser.IntentBodyTemplatePath,
		".kron/anchor-scan.json",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Errorf("expected %s: %v", rel, err)
		}
	}
}

func TestEnsureKronScaffold_DoesNotOverwriteCustomisations(t *testing.T) {
	root := t.TempDir()
	if err := EnsureKronScaffold(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	templatePath := filepath.Join(root, filepath.FromSlash(parser.IntentBodyTemplatePath))
	if err := os.WriteFile(templatePath, []byte("## Custom\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureKronScaffold(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "## Custom\n" {
		t.Fatalf("custom template overwritten: %q", string(data))
	}
}
