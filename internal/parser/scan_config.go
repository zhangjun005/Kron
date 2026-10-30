package parser

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/xxx/kron/internal/model"
)

// AnchorScanFileName is the project-level anchor scanning configuration
// file (relative to the repo root). Absent file = scan every non-binary
// file that survives DefaultSkipDirs, matching the pre-config behaviour.
const AnchorScanFileName = "anchor-scan.json"

// AnchorScanComment describes how to insert an anchor comment for a
// language. Detection never uses this: the scanner only looks for the
// literal "@kron:intent" marker, so comment syntax is client-side
// metadata for IDE insertion.
type AnchorScanComment struct {
	Line  string   `json:"line,omitempty"`
	Block []string `json:"block,omitempty"`
}

// AnchorScanLanguage is one language entry in anchor-scan.json.
type AnchorScanLanguage struct {
	Extensions []string           `json:"extensions"`
	Comment    *AnchorScanComment `json:"comment,omitempty"`
}

// AnchorScanConfig is the on-disk shape of .kron/anchor-scan.json.
//
// Semantics:
//   - include (optional): file must match at least one glob when non-empty;
//   - exclude (optional): file is skipped when it matches any glob;
//   - languages (optional): when non-empty, only files whose extension is
//     listed under one of the languages are scanned.
//
// Globs are repo-relative, forward-slash, and support *, ?, and **.
type AnchorScanConfig struct {
	Version   int                           `json:"version"`
	Include   []string                      `json:"include,omitempty"`
	Exclude   []string                      `json:"exclude,omitempty"`
	Languages map[string]AnchorScanLanguage `json:"languages,omitempty"`
}

// AnchorScanPath returns the absolute path of the scan config for root.
func AnchorScanPath(root string) string {
	return filepath.Join(root, model.KronDir, AnchorScanFileName)
}

// LoadAnchorScanConfig reads .kron/anchor-scan.json. A missing file returns
// (nil, nil): callers treat nil as "no filtering". A malformed file returns an
// error so lint surfaces it instead of silently scanning more/less than the
// project asked for.
func LoadAnchorScanConfig(root string) (*AnchorScanConfig, error) {
	data, err := os.ReadFile(AnchorScanPath(root))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var cfg AnchorScanConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	return &cfg, nil
}

// allowsFile reports whether relPath (forward-slash, repo-relative) should be
// scanned under this configuration.
func (c *AnchorScanConfig) allowsFile(relPath string) bool {
	if c == nil {
		return true
	}
	rel := strings.TrimPrefix(filepath.ToSlash(relPath), "./")
	if len(c.Include) > 0 {
		matched := false
		for _, pattern := range c.Include {
			if matchGlob(pattern, rel) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	for _, pattern := range c.Exclude {
		if matchGlob(pattern, rel) {
			return false
		}
	}
	if len(c.Languages) == 0 {
		return true
	}
	lower := strings.ToLower(rel)
	for _, language := range c.Languages {
		for _, ext := range language.Extensions {
			if ext != "" && strings.HasSuffix(lower, strings.ToLower(ext)) {
				return true
			}
		}
	}
	return false
}

// matchGlob matches a repo-relative slash path against a small glob dialect
// supporting *, ?, and ** (with **/ also matching zero directories).
func matchGlob(pattern, path string) bool {
	if pattern == "" {
		return false
	}
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				i++
				if i+1 < len(pattern) && pattern[i+1] == '/' {
					i++
					b.WriteString("(?:.*/)?")
				} else {
					b.WriteString(".*")
				}
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(pattern[i])))
		}
	}
	b.WriteString("$")
	return regexp.MustCompile(b.String()).MatchString(path)
}

// DefaultAnchorScanJSON is written by `kron init` when the project has no
// config. It lists mainstream languages so users can trim/extend the file
// instead of starting from an empty object. Detection itself does not need
// the comment prefixes; they are consumed by IDE clients for anchor insertion.
const DefaultAnchorScanJSON = `{
  "version": 1,
  "languages": {
    "go":              { "extensions": [".go"], "comment": { "line": "//", "block": ["/*", "*/"] } },
    "typescript":      { "extensions": [".ts", ".mts", ".cts"], "comment": { "line": "//", "block": ["/*", "*/"] } },
    "typescriptreact": { "extensions": [".tsx"], "comment": { "line": "//", "block": ["{/*", "*/}"] } },
    "javascript":      { "extensions": [".js", ".mjs", ".cjs"], "comment": { "line": "//", "block": ["/*", "*/"] } },
    "javascriptreact": { "extensions": [".jsx"], "comment": { "line": "//", "block": ["{/*", "*/}"] } },
    "vue":             { "extensions": [".vue"], "comment": { "line": "//", "block": ["<!--", "-->"] } },
    "svelte":          { "extensions": [".svelte"], "comment": { "line": "//", "block": ["<!--", "-->"] } },
    "astro":           { "extensions": [".astro"], "comment": { "line": "//", "block": ["<!--", "-->"] } },
    "python":          { "extensions": [".py"], "comment": { "line": "#" } },
    "rust":            { "extensions": [".rs"], "comment": { "line": "//", "block": ["/*", "*/"] } },
    "java":            { "extensions": [".java"], "comment": { "line": "//", "block": ["/*", "*/"] } },
    "c":               { "extensions": [".c", ".h"], "comment": { "line": "//", "block": ["/*", "*/"] } },
    "cpp":             { "extensions": [".cc", ".cpp", ".cxx", ".hpp", ".hh"], "comment": { "line": "//", "block": ["/*", "*/"] } },
    "csharp":          { "extensions": [".cs"], "comment": { "line": "//", "block": ["/*", "*/"] } },
    "kotlin":          { "extensions": [".kt", ".kts"], "comment": { "line": "//", "block": ["/*", "*/"] } },
    "swift":           { "extensions": [".swift"], "comment": { "line": "//", "block": ["/*", "*/"] } },
    "ruby":            { "extensions": [".rb"], "comment": { "line": "#" } },
    "php":             { "extensions": [".php"], "comment": { "line": "//", "block": ["/*", "*/"] } },
    "shellscript":     { "extensions": [".sh", ".bash"], "comment": { "line": "#" } },
    "yaml":            { "extensions": [".yml", ".yaml"], "comment": { "line": "#" } },
    "markdown":        { "extensions": [".md"], "comment": { "block": ["<!--", "-->"] } }
  }
}
`

// DefaultIntentBodyTemplate is written by `kron init` as the project body
// template. serve-mcp injects it into AI-facing prompts/tool descriptions.
const DefaultIntentBodyTemplate = `# {{title}}

> One-line summary of this intent's decision.

## Why

Describe the constraint, the alternatives considered, and why this choice wins.

## How

Describe the implementation shape and the trade-offs it accepts.
`
