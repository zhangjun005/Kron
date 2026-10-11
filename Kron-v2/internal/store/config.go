// Package store handles file I/O for Kron intent files.
package store

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xxx/kron/internal/model"
)

// EnsureKronDir creates the Kron directory structure under root if it does
// not already exist:
//
//	<root>/.kron/
//	<root>/.kron/intents/
//
// It is idempotent: if the directory already exists it returns nil without error.
// Intermediate directories are created with 0o755 permissions.
func EnsureKronDir(ctx context.Context, root string) error {
	_ = ctx // reserved for cancellation; no-op in v1
	abs, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("store: resolve root: %w", err)
	}
	intentsDir := filepath.Join(abs, model.KronDir, model.IntentDir)
	if err := os.MkdirAll(intentsDir, 0o755); err != nil {
		return fmt.Errorf("store: mkdir %s: %w", intentsDir, err)
	}
	return nil
}

// LoadConfig reads .kron/config.toml from root. If the file does not exist
// it returns the default Config (IntentsDir defaults to ".kron/intents")
// without error. If the file exists but is malformed it returns
// model.ErrConfigInvalid wrapped with the parse error.
//
// The parser is intentionally minimal: v1 only supports the
// `intents_dir = "..."` field. This avoids adding a third-party
// TOML dependency for what amounts to a single line of structured
// text. Unknown keys are ignored for forward-compatibility.
func LoadConfig(ctx context.Context, root string) (*model.Config, error) {
	_ = ctx
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("store: resolve root: %w", err)
	}
	path := filepath.Join(abs, model.KronDir, model.ConfigFileName)

	cfg := &model.Config{IntentsDir: ".kron/intents"} // default
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("store: read config: %w", err)
	}
	if err := parseConfigTOML(string(data), cfg); err != nil {
		return nil, fmt.Errorf("%w: %s: %v", model.ErrConfigInvalid, path, err)
	}
	return cfg, nil
}

// SaveConfig writes cfg to .kron/config.toml under root.
//
// The file is created with atomic rename (temp + os.Rename) so readers
// never observe a partial file. Permissions are 0o644.
func SaveConfig(ctx context.Context, root string, cfg *model.Config) error {
	_ = ctx
	abs, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("store: resolve root: %w", err)
	}
	kronDir := filepath.Join(abs, model.KronDir)
	if err := os.MkdirAll(kronDir, 0o755); err != nil {
		return fmt.Errorf("store: mkdir %s: %w", kronDir, err)
	}

	path := filepath.Join(kronDir, model.ConfigFileName)
	data := serializeConfigTOML(cfg)
	if err := writeFileAtomic(path, data, 0o644); err != nil {
		return fmt.Errorf("store: write config: %w", err)
	}
	return nil
}

// parseConfigTOML is a minimal v1-only TOML parser that recognises a
// single string assignment: `intents_dir = "value"`. Lines starting
// with '#' or ';' and blank lines are skipped. Unknown keys are
// silently ignored so older versions reading newer configs do not
// break (forward-compatibility).
func parseConfigTOML(s string, cfg *model.Config) error {
	for i, raw := range strings.Split(s, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			return fmt.Errorf("line %d: expected key = value, got %q", i+1, line)
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		switch key {
		case "intents_dir":
			s, err := unquoteTOMLString(val)
			if err != nil {
				return fmt.Errorf("line %d: %w", i+1, err)
			}
			if s == "" {
				return fmt.Errorf("line %d: intents_dir is empty", i+1)
			}
			cfg.IntentsDir = s
		}
		// Unknown keys: ignored.
	}
	return nil
}

// unquoteTOMLString accepts a double-quoted string with the common
// escape sequences (\", \\, \n, \t). It is intentionally not a full
// TOML string parser.
func unquoteTOMLString(s string) (string, error) {
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return "", fmt.Errorf("expected double-quoted string, got %q", s)
	}
	inner := s[1 : len(s)-1]
	var buf bytes.Buffer
	for i := 0; i < len(inner); i++ {
		c := inner[i]
		if c != '\\' {
			buf.WriteByte(c)
			continue
		}
		if i+1 >= len(inner) {
			return "", fmt.Errorf("trailing backslash")
		}
		i++
		switch inner[i] {
		case '"':
			buf.WriteByte('"')
		case '\\':
			buf.WriteByte('\\')
		case 'n':
			buf.WriteByte('\n')
		case 't':
			buf.WriteByte('\t')
		default:
			return "", fmt.Errorf("unsupported escape \\%c", inner[i])
		}
	}
	return buf.String(), nil
}

func serializeConfigTOML(cfg *model.Config) []byte {
	var buf bytes.Buffer
	buf.WriteString("# Kron configuration. v1: only `intents_dir` is honoured.\n")
	fmt.Fprintf(&buf, "intents_dir = %q\n", cfg.IntentsDir)
	return buf.Bytes()
}
