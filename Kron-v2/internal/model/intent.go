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

// IntentKind is the on-disk physical form of an intent.
//
// It is a write-time routing decision, NOT a frontmatter field. The
// frontmatter schema is unchanged by this type — only Go callers
// see Kind. The on-disk truth is the file path (resolveWritePath
// in internal/store/paths.go maps Kind → file layout).
//
// Why not in frontmatter:
//  1. The README/leaf distinction is implicit from the file path
//     ("<slug>/README.md" vs "<slug>.md") and exposing it as
//     frontmatter would require every author to remember to set
//     it consistently. Storing the kind on the file path is
//     self-describing.
//  2. Re-rooting a node (moving auth/README.md to a different
//     directory) would not require editing frontmatter — the kind
//     travels with the slug.
//
// Values:
//   - IntentKindLeaf: <root>/.kron/intents/<slug>.md           (default)
//   - IntentKindNode: <root>/.kron/intents/<slug>/README.md   (module-level node intent)
type IntentKind string

const (
	// IntentKindLeaf is the default. File on disk: <slug>.md.
	IntentKindLeaf IntentKind = "leaf"

	// IntentKindNode is a directory-level intent. File on disk:
	// <slug>/README.md. A node intent can coexist with child
	// intents at "<slug>/<child>" — see internal/view.
	IntentKindNode IntentKind = "node"
)

// Valid reports whether k is a recognised IntentKind value.
// Empty string is treated as Leaf (the default) so callers that
// leave Kind unset continue to work without migration.
func (k IntentKind) Valid() bool {
	switch k {
	case "", IntentKindLeaf, IntentKindNode:
		return true
	}
	return false
}

// Assumption is a verifiable precondition that governs the intent's validity.
// Written in frontmatter's assumptions[] field, not duplicated in body.
//
// B-3 fields (v1.3): ID (required) + Severity (required, per-intent) + Rationale
// (required, ≥ 10 chars) + Text (optional, read from assumption registry) + expires/verified.
type Assumption struct {
	// ID is a machine-friendly identifier in kebab-case.
	// Used by lint and MCP tools to reference this assumption.
	ID string `yaml:"id"`

	// Text is a human-readable description of the precondition.
	// Optional; if empty, callers should read from the assumption registry via ID.
	Text string `yaml:"text,omitempty"`

	// Severity determines what happens when this assumption is violated.
	// - hard: code logic MUST be changed (e.g., "Redis available ≥ 99.9%" → token revocation fails)
	// - soft: code still works but degrades; impact needs evaluation (e.g., "DAU ≤ 10K" → Redis memory pressure)
	Severity Severity `yaml:"severity"`

	// Rationale explains why THIS intent uses this severity level.
	// Required, ≥ 10 characters. Rationale is per-intent: the same assumption
	// (e.g., "single-region") can have different severity in different intents,
	// with each intent explaining its own rationale.
	// Lint: RuleAssumptionRationaleRequired (Error, v1.3); v1.5 hard Error.
	Rationale string `yaml:"rationale"`

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
// Same field set as Assumption (id/text/severity/rationale) plus ownership metadata.
//
// B-3: Severity → DefaultSeverity (the assumption's default severity;
// individual intents may override via their Frontmatter.Assumptions[].Severity).
type AssumptionFrontmatter struct {
	ID              string   `yaml:"id"`               // kebab-case; MUST equal file name without .md
	Text            string   `yaml:"text"`             // human/AI description
	DefaultSeverity Severity `yaml:"default_severity"` // hard | soft — suggestion only; intents override
	Status          Status   `yaml:"status,omitempty"` // draft | active | superseded (B-3: per-assumption lifecycle)
	CreatedBy       string   `yaml:"created_by"`       // "@user" or "agent:<model>"
	UpdatedAt       string   `yaml:"updated_at"`       // ISO 8601
	Reviewers       []string `yaml:"reviewers,omitempty"`
	ExpiresAt       string   `yaml:"expires_at,omitempty"`
	VerifiedAt      string   `yaml:"verified_at,omitempty"`
	VerifiedBy      string   `yaml:"verified_by,omitempty"`
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

	// References are soft links to other intents (see-also / inspiration /
	// recommended reading order). Symmetric in practice — direction is
	// purely a presentation choice. Deletion of a referenced target does
	// NOT fail this intent; lint reports a "dangling-reference" warning.
	//
	// Slug form only (no .md extension); example: "auth/jwt".
	// Optional.
	//
	// See docs/rfc/2026-10-03-frontmatter-references.md for motivation.
	References []string `yaml:"references,omitempty"`

	// DependsOn lists hard dependencies — intents that must be read
	// before this one can be correctly interpreted. Asymmetric: A
	// depends_on B does NOT imply B depends_on A.
	//
	// Deletion of a depended-on target: lint surfaces a
	// "dangling-depends-on" error, and kron_delete returns the list of
	// dependents in its response (soft warning, not a hard block in v1).
	//
	// Slug form only; must not include the intent's own slug. Optional.
	DependsOn []string `yaml:"depends_on,omitempty"`
}

// Intent is one design intent, persisted as .kron/intents/<slug>.md.
type Intent struct {
	// Slug is the relative path under .kron/intents/, without the .md extension.
	// Example: "auth/jwt-sliding-window".
	Slug string

	// Kind is the on-disk physical form (leaf vs node). It is a
	// write-time routing decision and is NOT serialized to
	// frontmatter (yaml:"-") — see IntentKind for why. On read,
	// Kind is left at its zero value (Leaf) because the loader
	// uses the file path to determine shape; callers that need
	// the distinction should look at SourcePath or use
	// store.Reader's tree helpers.
	Kind IntentKind `yaml:"-"`

	// Frontmatter is the YAML metadata block at the top of the file.
	Frontmatter Frontmatter

	// Body is the markdown content below the frontmatter separator (---).
	Body string

	// SourcePath is the absolute disk path to this intent's .md file.
	// It is set by the store layer when loading; it is NOT serialized back to disk.
	SourcePath string
}

// AnchorKind classifies the source surface an anchor was parsed from.
//
// It is a v1.2+ addition (RFC 2026-10-08-md-anchors.md §2.5) so
// callers that need a per-anchor kind (kron_impact, kron_intent_density,
// future lint rules) can read it off model.Anchor instead of
// hand-coding the source surface at every call site.
//
// Two values, one binary: every anchor is either in source code
// (".go", ".ts", ".py", ...) or in a markdown file OUTSIDE the
// intent directory (any .md). There is no third category in v1.
//
// The zero value is the empty string, NOT AnchorKindCode, because
// AnchorKind is a string-typed alias and string zero is "". The
// parser layer always sets the field explicitly
// (ScanAnchors → AnchorKindCode; ScanMarkdownAnchors →
// AnchorKindMarkdown); hand-constructed literals that omit Kind
// get "" which downstream consumers treat as "unknown / default to
// code". The empty string is NOT a sentinel error value — it is
// the legitimate pre-v1.2 value for code anchors that the parser
// has just upgraded.
//
// Use Kind.IsSet / IsCode / IsMarkdown to dispatch safely.
type AnchorKind string

const (
	// AnchorKindCode: the anchor was parsed from a source-code file
	// (any non-.md file walked by parser.ScanAnchors). The historical
	// default.
	AnchorKindCode AnchorKind = "code"

	// AnchorKindMarkdown: the anchor was parsed from a .md file
	// walked by parser.ScanMarkdownAnchors (RFC
	// 2026-10-08-md-anchors.md §1.1 — repo-wide .md, excluding
	// .kron/intents/ and .kron/.trash/).
	AnchorKindMarkdown AnchorKind = "markdown"
)

// IsSet reports whether k carries a parser-assigned value.
// Hand-constructed literals (zero value "") return false; parser
// output always returns true. Use this to gate "do I need to guess?"
// logic in long-lived data structures.
func (k AnchorKind) IsSet() bool {
	return k == AnchorKindCode || k == AnchorKindMarkdown
}

// IsCode reports whether k identifies a code-surface anchor. The
// empty string is treated as Code (backward-compatible default
// for pre-v1.2 callers that didn't set the field).
func (k AnchorKind) IsCode() bool {
	return k == "" || k == AnchorKindCode
}

// IsMarkdown reports whether k identifies a markdown-surface anchor.
// Only the explicit AnchorKindMarkdown value returns true; empty
// returns false (use IsCode for the "default" branch).
func (k AnchorKind) IsMarkdown() bool {
	return k == AnchorKindMarkdown
}

// Anchor is a parsed // @kron:intent <slug> annotation found in a source file.
type Anchor struct {
	// Slug is the intent path, without the .md extension.
	Slug string

	// FilePath is the path to the source file containing this anchor,
	// expressed as a REPO-RELATIVE identifier with forward-slash
	// separators (filepath.ToSlash of filepath.Rel(root, abs)).
	//
	// It is an identifier, not an IO path. Callers that need to open
	// the file must join it against their own root. The format is
	// locked by RFC 2026-10-08-path-conventions.md §2.1 so anchors
	// remain stable across processes and platforms (no Windows
	// backslashes, no leading slash). Scanner implementations (see
	// internal/parser.ScanAnchors / ScanMarkdownAnchors) emit this
	// canonical form; tools that synthesize Anchor values directly
	// (e.g. tests) must follow the same rule.
	FilePath string

	// LineNumber is the 1-indexed line where the anchor appears.
	LineNumber int

	// Kind classifies the source surface. Set by the parser
	// (ScanAnchors emits Code; ScanMarkdownAnchors emits Markdown);
	// left at its zero value (Code) by hand-constructed literals.
	// Callers that need a per-anchor kind (kron_impact,
	// kron_intent_density, lint) should read this field instead of
	// dispatching on the path or the call site. The dispatch
	// pattern (call ScanAnchors / ScanMarkdownAnchors separately and
	// hardcode the kind) was the v1.1 implementation; v1.2 collapses
	// both code paths onto the Kind field so a single sorted scan
	// produces a kind-tagged result.
	Kind AnchorKind
}
