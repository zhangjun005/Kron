// project_registry.go — multi-project registry (v1.3+, RFC 2026-10-14).
//
// This file lives in internal/store because the on-disk schema is
// storage-level concern: a single JSON file, atomic write, Go zero-value
// tolerance. It deliberately does NOT know about CLI / MCP / LSP / GUI
// — the caller decides which fields to set (e.g. CLI sets added_at on
// `kron register`; GUI sets last_opened on user click; etc.).
//
// Wire format (see docs/rfc/2026-10-14-projects-json-schema-extension.md §2.1):
//
//	{
//	  "current_project": "<abs path>",   // optional, ""
//	  "projects": [                      // required, [] is legal
//	    {
//	      "path": "<abs path>",          // required, unique key
//	      "added_at": "<iso8601 utc>",   // required
//	      "last_opened": "<iso8601 utc>" // optional, "" if never
//	    }
//	  ]
//	}
//
// Forward + backward compat: Go's json.Unmarshal ignores unknown
// fields by default, so v1.4+ additions (when added via a new RFC)
// are invisible to v1.3 readers. omitempty + zero-value tolerance
// keep the on-disk file minimal: we never write empty strings / nil
// arrays. See §2.2 of the RFC for the full algorithm.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ProjectRegistry is the in-memory shape of projects.json. The
// json tags are the wire contract; the field set is locked by RFC
// 2026-10-14. omitempty on every field so the marshaler never
// writes zero values (e.g. an empty current_project does NOT show
// up in the on-disk file).
type ProjectRegistry struct {
	CurrentProject string         `json:"current_project,omitempty"`
	Projects       []ProjectEntry `json:"projects"`
}

// ProjectEntry is the per-project row. Path is the unique key; the
// caller (Register / Touch) is responsible for the "don't insert
// duplicates" invariant. added_at is set by Register; last_opened
// is set by the GUI / "open project" path.
type ProjectEntry struct {
	Path       string `json:"path"`
	AddedAt    string `json:"added_at,omitempty"`
	LastOpened string `json:"last_opened,omitempty"`
}

// ProjectsJSONPath returns the absolute path of projects.json that
// lives next to the kron binary (RFC §2.5: os.Executable()'s
// directory). The path is stable across invocations as long as the
// binary is not moved; this is intentional — single-file portable
// distribution is the design (no %APPDATA%, no ~/.config/kron/, no
// env var override for production).
//
// The KRON_PROJECTS_JSON_PATH env var IS honoured as a test escape
// hatch — when set, it overrides the canonical path so unit tests
// can run in an isolated temp directory without polluting the real
// projects.json next to the kron binary. Production invocations MUST
// NOT set this env var; the documented contract is "next to the
// binary".
func ProjectsJSONPath() (string, error) {
	if override := os.Getenv("KRON_PROJECTS_JSON_PATH"); override != "" {
		return override, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("store: locate kron binary for projects.json: %w", err)
	}
	return filepath.Join(filepath.Dir(exe), "projects.json"), nil
}

// ReadProjectsJSON reads projects.json from the canonical path
// (ProjectsJSONPath). The algorithm follows RFC §2.2.1:
//
//  1. File does not exist → return empty registry (no error).
//  2. File exists:
//     a. os.ReadFile the whole file
//     b. json.Unmarshal into ProjectRegistry
//     c. unknown fields ignored (Go default)
//     d. type mismatch → error (we do NOT silently coerce)
//
// The returned registry is always non-nil; a missing file yields
// &ProjectRegistry{Projects: []ProjectEntry{}} (empty Projects so
// the JSON marshaler writes "projects": [], not omits the field
// — Projects is required per RFC §2.1).
func ReadProjectsJSON() (*ProjectRegistry, error) {
	path, err := ProjectsJSONPath()
	if err != nil {
		return nil, err
	}
	return readProjectsJSONAt(path)
}

// readProjectsJSONAt is the path-parameterised core; tests use
// this to point at a t.TempDir() copy.
func readProjectsJSONAt(path string) (*ProjectRegistry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ProjectRegistry{Projects: []ProjectEntry{}}, nil
		}
		return nil, fmt.Errorf("store: read projects.json %s: %w", path, err)
	}
	var reg ProjectRegistry
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("store: parse projects.json %s: %w", path, err)
	}
	// Defensive: if the file legitimately has "projects": null or
	// omits the key, force an empty slice so callers can iterate
	// without nil-checks and the next Write round-trips with
	// "projects": [].
	if reg.Projects == nil {
		reg.Projects = []ProjectEntry{}
	}
	return &reg, nil
}

// WriteProjectsJSON writes reg to the canonical path atomically
// (RFC §2.4: temp + os.Rename, same directory). omitempty on
// every field means the file stays minimal across writes — the
// marshaler never writes zero values, so a project that's never
// been opened has no "last_opened" key on disk at all.
func WriteProjectsJSON(reg *ProjectRegistry) error {
	path, err := ProjectsJSONPath()
	if err != nil {
		return err
	}
	return writeProjectsJSONAt(path, reg)
}

// writeProjectsJSONAt is the path-parameterised core; tests use
// this to round-trip through a t.TempDir() copy.
func writeProjectsJSONAt(path string, reg *ProjectRegistry) error {
	if reg == nil {
		return fmt.Errorf("store: write projects.json: registry is nil")
	}
	if reg.Projects == nil {
		// Marshal an empty array, not "null" — RFC §2.1
		// requires the projects key.
		reg.Projects = []ProjectEntry{}
	}
	// Pretty-print with 2-space indent so the file is human-
	// readable in editors / git diffs. The atomic writer
	// already handles temp+rename.
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return fmt.Errorf("store: marshal projects.json: %w", err)
	}
	data = append(data, '\n') // POSIX-friendly trailing newline
	if err := writeFileAtomic(path, data, 0o644); err != nil {
		return fmt.Errorf("store: write projects.json %s: %w", path, err)
	}
	return nil
}

// RegisterProject adds path to the registry if it isn't already
// present (path is the unique key per RFC §2.1). On insert,
// added_at is set to now (UTC, RFC 3339). On existing entry, the
// entry is left untouched (idempotent). The returned bool reports
// whether the entry was newly inserted (true) or already existed
// (false); callers that need to know "did the registry change?"
// use it to decide whether to print a confirmation message.
//
// `now` is injected so tests can pin the timestamp; production
// callers pass time.Now().UTC().
//
// RegisterProject does NOT touch CurrentProject. That's a
// separate call (SetCurrentProject) because the GUI's "switch
// project" flow is conceptually different from the CLI's
// "register a new project" flow (see RFC §2.6).
func RegisterProject(reg *ProjectRegistry, path string, now time.Time) (inserted bool, err error) {
	if reg == nil {
		return false, fmt.Errorf("store: RegisterProject: registry is nil")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return false, fmt.Errorf("store: RegisterProject: resolve path: %w", err)
	}
	for _, e := range reg.Projects {
		if e.Path == abs {
			return false, nil // already present — no-op
		}
	}
	reg.Projects = append(reg.Projects, ProjectEntry{
		Path:    abs,
		AddedAt: now.UTC().Format(time.RFC3339),
	})
	return true, nil
}

// SetCurrentProject updates CurrentProject. Empty string is
// allowed (clears the active project); absolute-path enforcement
// is the caller's responsibility (we don't want to silently
// rewrite the value the caller chose). Returns the previous
// current value so the caller can log "switched X → Y" if it
// wants to.
func SetCurrentProject(reg *ProjectRegistry, path string) (previous string, err error) {
	if reg == nil {
		return "", fmt.Errorf("store: SetCurrentProject: registry is nil")
	}
	abs := path
	if path != "" {
		var aerr error
		abs, aerr = filepath.Abs(path)
		if aerr != nil {
			return "", fmt.Errorf("store: SetCurrentProject: resolve path: %w", aerr)
		}
	}
	prev := reg.CurrentProject
	reg.CurrentProject = abs
	return prev, nil
}

// TouchLastOpened sets the LastOpened timestamp on the entry
// whose Path matches `path` (matched after filepath.Abs so
// "C:/foo" and "C:\foo" both hit the same Windows entry). No-op
// if the path is not in the registry. Returns true if the entry
// was found and updated.
//
// `now` is injected for testability; production callers pass
// time.Now().UTC().
func TouchLastOpened(reg *ProjectRegistry, path string, now time.Time) (bool, error) {
	if reg == nil {
		return false, fmt.Errorf("store: TouchLastOpened: registry is nil")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return false, fmt.Errorf("store: TouchLastOpened: resolve path: %w", err)
	}
	for i := range reg.Projects {
		if reg.Projects[i].Path == abs {
			reg.Projects[i].LastOpened = now.UTC().Format(time.RFC3339)
			return true, nil
		}
	}
	return false, nil
}
