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

// Severity indicates the consequence when an assumption is violated.
type Severity string

const (
	SeverityHard Severity = "hard"
	SeveritySoft Severity = "soft"
)

// Assumption is a verifiable precondition that governs the intent's validity.
// Written in frontmatter's assumptions[] field, not duplicated in body.
type Assumption struct {
	// ID is a machine-friendly identifier in kebab-case.
	// Used by lint and MCP tools to reference this assumption.
	ID string `yaml:"id"`

	// Text is a human-readable description of the precondition.
	Text string `yaml:"text"`

	// Severity determines what happens when this assumption is violated.
	// - hard: code logic MUST be changed (e.g., "Redis available ≥ 99.9%" → token revocation fails)
	// - soft: code still works but degrades; impact needs evaluation (e.g., "DAU ≤ 10K" → Redis memory pressure)
	Severity Severity `yaml:"severity"`

	// ExpiresAt is the estimated review deadline for this assumption.
	// It does NOT trigger any automatic behavior; it only serves as a hint
	// for MCP kron_stale and LSP hover. Optional.
	ExpiresAt string `yaml:"expires_at,omitempty"`

	// VerifiedAt is the last manual confirmation date in ISO 8601 format.
	// Optional.
	VerifiedAt string `yaml:"verified_at,omitempty"`

	// VerifiedBy is the handle of the person who last verified this assumption.
	// Format: "@github_handle". Optional.
	VerifiedBy string `yaml:"verified_by,omitempty"`
}

// AssumptionFile is one assumption stored as .kron/assumptions/<id>.md.
// It is the source of truth for an assumption; the intent's frontmatter only
// references it via assumption ID.
type AssumptionFile struct {
	// Slug is the file name without .md extension; e.g., "single-region".
	// It MUST equal Frontmatter.ID; otherwise lint reports
	// assumption-registry-id-mismatch.
	Slug string

	// Frontmatter is the YAML metadata at the top of the file.
	// CreatedBy and UpdatedAt are required; Severity, Text are required;
	// Reviewers, ExpiresAt, VerifiedAt, VerifiedBy are optional.
	Frontmatter AssumptionFrontmatter

	// Body is the markdown content below the frontmatter separator.
	Body string

	// SourcePath is the absolute disk path to this assumption's .md file.
	// Set by the store layer when loading; NOT serialized back to disk.
	SourcePath string
}

// AssumptionFrontmatter is the YAML metadata for an AssumptionFile.
// Same field set as Assumption (id/text/severity) plus ownership metadata.
type AssumptionFrontmatter struct {
	ID         string   `yaml:"id"`         // kebab-case; MUST equal file name without .md
	Text       string   `yaml:"text"`       // human/AI description
	Severity   Severity `yaml:"severity"`   // hard | soft
	CreatedBy  string   `yaml:"created_by"` // "@user" or "agent:<model>"
	UpdatedAt  string   `yaml:"updated_at"` // ISO 8601
	Reviewers  []string `yaml:"reviews,omitempty"`
	ExpiresAt  string   `yaml:"expires_at,omitempty"`
	VerifiedAt string   `yaml:"verified_at,omitempty"`
	VerifiedBy string   `yaml:"verified_by,omitempty"`
}

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

	// Assumptions are verifiable preconditions that govern this intent's validity.
	// Optional; empty means this intent has no explicit assumptions.
	Assumptions []Assumption `yaml:"assumptions,omitempty"`
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
