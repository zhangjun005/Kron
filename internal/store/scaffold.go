package store

// EnsureKronScaffold creates the idempotent .kron/ project skeleton:
// intents/, config.toml, templates/intent-body.md and anchor-scan.json.
// Existing files are never overwritten, so re-running init after
// customising the template or scan config is safe.
//
// The AI-facing body template and the anchor scanning config are part of
// the project scaffold (both are consumed by clients), so `kron init` and
// MCP `kron_init` must create them together with the intents directory.

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/parser"
)

// EnsureKronScaffold creates the .kron/ skeleton under root, writing the
// default config, intent body template and anchor-scan.json only when the
// files are absent.
func EnsureKronScaffold(ctx context.Context, root string) error {
	if err := EnsureKronDir(ctx, root); err != nil {
		return err
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("store: resolve root: %w", err)
	}
	configPath := filepath.Join(abs, model.KronDir, model.ConfigFileName)
	if _, err := os.Stat(configPath); errors.Is(err, fs.ErrNotExist) {
		if err := SaveConfig(ctx, abs, &model.Config{IntentsDir: ".kron/intents"}); err != nil {
			return err
		}
	} else if err != nil {
		return fmt.Errorf("store: stat %s: %w", configPath, err)
	}
	if err := writeFileIfMissing(filepath.Join(abs, filepath.FromSlash(parser.IntentBodyTemplatePath)), parser.DefaultIntentBodyTemplate); err != nil {
		return err
	}
	if err := writeFileIfMissing(parser.AnchorScanPath(abs), parser.DefaultAnchorScanJSON); err != nil {
		return err
	}
	return nil
}

// writeFileIfMissing creates path with content when it does not exist. An
// existing file (even empty) is left untouched so user customisations win.
func writeFileIfMissing(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("store: stat %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("store: mkdir %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("store: write %s: %w", path, err)
	}
	return nil
}
