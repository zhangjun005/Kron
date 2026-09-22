// Package model defines the domain types for Kron.
//
// Types here are pure data structures with zero external dependencies.
// All file I/O and parsing logic lives in internal/store and internal/parser.
package model

import "time"

// Status represents the lifecycle state of an Intent.
// It is optional; an intent with no status field does not participate in lifecycle management.
type Status string

const (
	StatusDraft      Status = "draft"
	StatusActive     Status = "active"
	StatusSuperseded Status = "superseded"
)

// Frontmatter holds the YAML metadata at the top of an intent .md file.
// All fields except CreatedBy are optional.
type Frontmatter struct {
	// Symbol lists code symbols (e.g., function or type names) that this intent governs.
	// Format: "pkg.TypeName" or "pkg.funcName".
	Symbol []string `yaml:"symbol,omitempty"`

	// CreatedBy is the author, prefixed with "@" for humans ("@alice") or
	// "agent:<model>" for AI agents ("agent:claude-3-7").
	CreatedBy string `yaml:"created_by"`

	// UpdatedAt is the last modification time in RFC3339 format.
	UpdatedAt time.Time `yaml:"updated_at"`

	// Reviewers lists collaborators who have approved this intent. Optional.
	Reviewers []string `yaml:"reviewers,omitempty"`

	// Status is the lifecycle state. Optional; omit to skip lifecycle management.
	Status Status `yaml:"status,omitempty"`
}

// Intent is one design intent, persisted as .kron/intents/<slug>.md.
type Intent struct {
	// Slug is the relative path under .kron/intents/, without the .md extension.
	// Example: "auth/jwt-sliding-window".
	Slug string

	// Frontmatter is the YAML metadata block at the top of the file.
	Frontmatter Frontmatter

	// Body is the markdown content below the frontmatter separator (---).
	Body string

	// SourcePath is the absolute disk path to this intent's .md file.
	// It is set by the store layer when loading; it is NOT serialized back to disk.
	SourcePath string
}

// Anchor is a parsed // @kron:intent <slug> annotation found in a source file.
type Anchor struct {
	// Slug is the intent path, without the .md extension.
	Slug string

	// FilePath is the path to the source file containing this anchor.
	FilePath string

	// LineNumber is the 1-indexed line where the anchor appears.
	LineNumber int
}
