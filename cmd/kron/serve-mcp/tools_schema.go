package servemcp

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/xxx/kron/internal/lint"
)

// This file holds the typed input / output schemas for the MCP
// tools exposed by kron serve-mcp, plus a single registerTools()
// call site that wires them into the mcp.Server.
//
// Why centralised: the schema is the contract. Putting every
// tool's In/Out next to the registerTools call makes it impossible
// to forget to register a tool, and the SDK reads the struct tags
// to derive JSON-Schema for tools/list automatically (no manual
// schema authoring). See docs/rfc/2026-10-04-mcp-sdk-adoption.md.
//
// Note: In fields use jsonschema:"..." tags because the SDK's
// schema-inference pipeline reads them as field descriptions. Field
// types are read directly from the Go type — string stays string,
// []string becomes a JSON array of strings, optional fields use
// pointer types so SDK marks them as not-required.

// --- kron_init ----------------------------------------------------------

type InitInput struct{} // no parameters

type InitOutput struct {
	OK                 bool   `json:"ok"                       jsonschema:"true if .kron/ exists or was created"`
	IntentsDir         string `json:"intents_dir"               jsonschema:"path to the intents directory (e.g. .kron/intents)"`
	AlreadyInitialised bool   `json:"already_initialised"       jsonschema:"true if .kron/ was already present (idempotent re-init)"`
	// RepoRoot is the absolute on-disk path the server is operating
	// on. Surfaced so the AI caller can include it in user-facing
	// diagnostics when a misconfigured client passes the wrong cwd
	// (the server fails fast in resolveMCPRoot, but the AI sees a
	// JSON-RPC error and may not know which path was rejected).
	// See handlers_init.go for the rationale.
	RepoRoot string `json:"repo_root" jsonschema:"absolute on-disk path of the project root the server is operating on (for AI-side diagnostics)"`
	// RegisteredCurrentProject is the projects.json::current_project
	// after the init+register side effect (RFC
	// 2026-10-14-projects-json-schema-extension.md §2.6). Empty
	// when the side effect was skipped (e.g. a previous init
	// already promoted another project and the user did not
	// intend to switch). Surfaced so an AI caller that just
	// bootstrapped a project via `kron_init` knows whether
	// subsequent MCP calls will resolve MCPRoot to this project
	// (current_project set) or to the previously-active one
	// (unchanged). Equal to RepoRoot on the common case; equal
	// to "" when the registry write was skipped due to a
	// non-fatal warning (see handlers_init.go).
	RegisteredCurrentProject string `json:"registered_current_project,omitempty" jsonschema:"projects.json::current_project after init+register side effect; empty when the registry write was skipped or unchanged"`
}

// --- kron_lint --------------------------------------------------------

type LintInput struct {
	// Reporter selects the output format. "json" (default) returns
	// the structured LintOutput. "text" returns a human-readable
	// summary string. Both formats contain identical data; "text" is
	// intended for interactive use in terminals that cannot parse JSON.
	Reporter string `json:"reporter,omitempty" jsonschema:"output format: json (default) or text"`
}

type LintDiag struct {
	Rule       string `json:"rule"       jsonschema:"rule code, e.g. anchor-dangling / frontmatter-invalid"`
	Where      string `json:"where"      jsonschema:"file:line or slug where the issue is"`
	Severity   string `json:"severity"   jsonschema:"error or warning"`
	Detail     string `json:"detail"     jsonschema:"human-readable description"`
	Suggestion string `json:"suggestion,omitempty" jsonschema:"optional human-readable fix hint; empty when the engine has no confident recommendation (C-5)"`
}

type LintSummary struct {
	Errors   int `json:"errors"   jsonschema:"count of error-severity diagnostics"`
	Warnings int `json:"warnings" jsonschema:"count of warning-severity diagnostics"`
}

type LintOutput struct {
	Passed  bool        `json:"passed"  jsonschema:"true iff no error-severity diagnostics"`
	Errors  []LintDiag  `json:"errors"  jsonschema:"full diagnostic list"`
	Summary LintSummary `json:"summary" jsonschema:"aggregate counters"`
	// Text is populated only when reporter=text. It is the
	// human-readable summary (e.g. "[ok]" or "[error] 2 errors, 1 warning").
	Text string `json:"text,omitempty" jsonschema:"human-readable summary (only when reporter=text)"`
}

// --- kron_list / kron_get --------------------------------------------

type ListInput struct {
	Prefix string `json:"prefix,omitempty" jsonschema:"optional slug prefix filter"`
}

type IntentSummary struct {
	Slug      string        `json:"slug"                    jsonschema:"kebab-case slug (the intent's filename minus .md)"`
	Symbol    []string      `json:"symbol,omitempty"        jsonschema:"code symbols this intent anchors to"`
	Status    string        `json:"status,omitempty"        jsonschema:"draft / active / superseded"`
	UpdatedAt string        `json:"updated_at"              jsonschema:"RFC3339 timestamp of last edit"`
	Assumes   []AssumeEntry `json:"assumes,omitempty"       jsonschema:"assumption summaries attached to the frontmatter"`
}

type ListOutput struct {
	Intents []IntentSummary `json:"intents" jsonschema:"all intents matching the prefix filter (empty array if none)"`
}

type GetInput struct {
	Slug string `json:"slug" jsonschema:"slug of the intent to load"`
	// IncludeRelations augments the response with reverse-link and
	// prerequisite views (computed by internal/relations). Off by
	// default to keep the common case small; GUI/IDE detail panels
	// set it true.
	IncludeRelations bool `json:"include_relations,omitempty" jsonschema:"if true, include references / depends_on / incoming_anchors / prerequisites alongside the body"`
}

type IntentBody struct {
	Slug        string            `json:"slug"                         jsonschema:"kebab-case slug"`
	Frontmatter IntentFrontmatter `json:"frontmatter"                  jsonschema:"frontmatter fields (snake_case wire keys)"`
	Body        string            `json:"body"                         jsonschema:"raw markdown body"`
	SourcePath  string            `json:"source_path"                  jsonschema:"absolute on-disk path"`
	References  []string          `json:"references,omitempty"          jsonschema:"soft links to other intents"`
	DependsOn   []string          `json:"depends_on,omitempty"         jsonschema:"hard dependencies on other intents"`

	// Relations is populated only when include_relations=true.
	// Field shape mirrors kron_impact's relevant fields so GUI/IDE
	// detail panels can render directly.
	Relations *GetRelations `json:"relations,omitempty" jsonschema:"reverse-link and prerequisite views (only when include_relations=true)"`
}

// GetRelations is the subset of relations.ReverseLinks + relations.Prerequisites
// output exposed in kron_get's include_relations=true path. Lives in
// tools_schema.go (not internal/relations) because it is a wire
// projection.
type GetRelations struct {
	// References is the soft-link reverse: slugs that list this slug
	// in their Frontmatter.References. Sorted.
	References []string `json:"references,omitempty" jsonschema:"slugs whose References list contains this intent (sorted)"`
	// DependsOnDependents is the hard-link reverse: slugs that list
	// this slug in their Frontmatter.DependsOn. Sorted.
	DependsOnDependents []string `json:"depends_on_dependents,omitempty" jsonschema:"slugs whose depends_on list contains this intent (sorted)"`
	// SymbolInferred is the symbol-based inferred soft dependency:
	// slugs whose Symbol set intersects this intent's Symbol set,
	// excluding explicit dependents. Sorted.
	SymbolInferred []string `json:"symbol_inferred,omitempty" jsonschema:"slugs with overlapping symbols (sorted)"`
	// Prerequisites is the forward view: explicit depends_on ∪
	// symbol-inferred deps. Sorted.
	Prerequisites []string `json:"prerequisites,omitempty" jsonschema:"explicit depends_on ∪ symbol-inferred (sorted)"`
	// Dependents is the union of DependsOnDependents ∪ SymbolInferred,
	// deduplicated and sorted. Added in C-7 to mirror the
	// kron_delete response shape (which already returns
	// dependents in the same way) so a caller that wants the
	// full "who depends on me" view doesn't have to merge two
	// fields client-side. Equal to what kron_delete would
	// surface, except kron_delete is hard-only and this is the
	// full set.
	Dependents []string `json:"dependents,omitempty" jsonschema:"union of depends_on_dependents + symbol_inferred, deduped + sorted (C-7)"`
}

type IntentFrontmatter struct {
	Symbol      []string      `json:"symbol,omitempty"        jsonschema:"code symbols this intent anchors to"`
	CreatedBy   string        `json:"created_by"              jsonschema:"handle of the creator (e.g. @zhangjun005)"`
	UpdatedAt   string        `json:"updated_at"              jsonschema:"RFC3339 timestamp of last edit"`
	Reviewers   []string      `json:"reviewers,omitempty"     jsonschema:"reviewer handles"`
	Status      string        `json:"status,omitempty"        jsonschema:"draft / active / superseded"`
	Assumptions []AssumeEntry `json:"assumptions,omitempty"   jsonschema:"declared assumptions"`
	References  []string      `json:"references,omitempty"    jsonschema:"soft links to other intents"`
	DependsOn   []string      `json:"depends_on,omitempty"    jsonschema:"hard dependencies on other intents"`
}

// AssumeEntry is the wire projection of model.Assumption joined with
// the assumption registry. Used in kron_list / kron_get / kron_impact
// responses.
type AssumeEntry struct {
	ID              string `json:"id"                          jsonschema:"stable assumption identifier"`
	Text            string `json:"text"                        jsonschema:"human-readable claim (preferred from registry; empty if neither present)"`
	Severity        string `json:"severity"                    jsonschema:"hard or soft (per-intent; overrides registry default)"`
	DefaultSeverity string `json:"default_severity,omitempty"  jsonschema:"registry default severity (B-3); empty if registry absent"`
	Rationale       string `json:"rationale,omitempty"         jsonschema:"per-intent explanation of why this severity (B-3)"`
	ExpiresAt       string `json:"expires_at,omitempty"        jsonschema:"optional expiry (YYYY-MM-DD or RFC3339)"`
	VerifiedAt      string `json:"verified_at,omitempty"       jsonschema:"last verification time (empty if never verified)"`
	VerifiedBy      string `json:"verified_by,omitempty"       jsonschema:"handle that last verified"`
}

// AssumptionRef is the minimal shape for referencing an assumption
// in kron_add / kron_update. The caller supplies id + severity +
// rationale; the registry file (text / default_severity) is managed
// by the assumption CRUD tools. G-5: auto-create stub registry
// files for IDs not yet in the registry.
type AssumptionRef struct {
	ID         string `json:"id"         jsonschema:"assumption id (must pass parser.ValidateSlug)"`
	Severity   string `json:"severity"   jsonschema:"hard or soft (per-intent severity; required)"`
	Rationale  string `json:"rationale,omitempty"  jsonschema:"required, ≥10 chars (B-3 rationale-required rule)"`
	ExpiresAt  string `json:"expires_at,omitempty" jsonschema:"optional expiry (YYYY-MM-DD or RFC3339)"`
	VerifiedAt string `json:"verified_at,omitempty" jsonschema:"optional verification date (YYYY-MM-DD or RFC3339)"`
	VerifiedBy string `json:"verified_by,omitempty" jsonschema:"optional @handle of verifier"`
}

type GetOutput struct {
	Intent IntentBody `json:"intent" jsonschema:"the requested intent"`
}

// --- kron_add ---------------------------------------------------------

type AddInput struct {
	Slug   string `json:"slug"             jsonschema:"kebab-case slug (must pass parser.ValidateSlug)"`
	Symbol string `json:"symbol,omitempty" jsonschema:"optional single symbol to populate Frontmatter.Symbol[0]"`
	Why    string `json:"why,omitempty"    jsonschema:"optional scaffold body, appended under ## Why"`
	// Kind selects the on-disk physical form. Empty or "leaf" (default)
	// writes <slug>.md; "node" writes <slug>/README.md (a module-level
	// node intent that can co-exist with child leaf intents). Mirrors
	// the kron add CLI --kind flag — see RFC
	// 2026-10-08-writer-readme-symmetry §3.5.
	Kind string `json:"kind,omitempty" jsonschema:"on-disk form: \"leaf\" (default) or \"node\""`
	// Assumptions populates Frontmatter.Assumptions on the new intent.
	// G-5: for each entry, the corresponding .kron/assumptions/<id>.md
	// file is auto-created (or verified to already exist) so that
	// kron_assume_check / kron_list / kron_get always find a registry
	// entry rather than silently falling back to empty text.
	Assumptions []AssumptionRef `json:"assumptions,omitempty" jsonschema:"optional assumption references to attach (auto-creates registry files for new IDs)"`
}

type AddOutput struct {
	OK   bool   `json:"ok"   jsonschema:"true on success"`
	Path string `json:"path" jsonschema:"relative path to the written intent file (.kron/intents/<slug>.md for leaf; .kron/intents/<slug>/README.md for node)"`
}

// --- kron_update ------------------------------------------------------

type UpdateInput struct {
	Slug       string   `json:"slug"                    jsonschema:"target intent slug (required)"`
	Symbol     []string `json:"symbol,omitempty"        jsonschema:"replace Symbol array (null/absent = no change; [] = clear)"`
	Status     *string  `json:"status,omitempty"        jsonschema:"replace status (null/absent = no change)"`
	Reviewers  []string `json:"reviewers,omitempty"     jsonschema:"replace reviewers (null/absent = no change; [] = clear)"`
	Body       *string  `json:"body,omitempty"          jsonschema:"replace body (null/absent = no change)"`
	References []string `json:"references,omitempty"    jsonschema:"replace references (null/absent = no change; [] = clear)"`
	DependsOn  []string `json:"depends_on,omitempty"    jsonschema:"replace depends_on (null/absent = no change; [] = clear)"`
	// Assumptions replaces the entire Assumptions[] on the intent.
	// G-5: for each new ID in the list, the corresponding
	// .kron/assumptions/<id>.md is auto-created. Null/absent = no
	// change to assumptions. Empty slice = clear all assumptions.
	Assumptions []AssumptionRef `json:"assumptions,omitempty" jsonschema:"replace Assumptions[] (null/absent = no change; [] = clear)"`
}

type UpdateOutput struct {
	OK   bool   `json:"ok"   jsonschema:"true on success"`
	Path string `json:"path" jsonschema:"relative path to the patched intent file"`
}

// --- kron_assumption_create -------------------------------------------

// AssumptionCreateInput is the wire shape for kron_assume_create.
type AssumptionCreateInput struct {
	ID              string `json:"id"               jsonschema:"kebab-case slug (must pass parser.ValidateSlug)"`
	Text            string `json:"text"             jsonschema:"non-empty human-readable description"`
	DefaultSeverity string `json:"default_severity" jsonschema:"hard or soft"`
	Body            string `json:"body,omitempty"   jsonschema:"optional free-form markdown body below the frontmatter"`
}

type AssumptionCreateOutput struct {
	OK   bool   `json:"ok"   jsonschema:"true on success"`
	Path string `json:"path" jsonschema:"path to the created assumption file (.kron/assumptions/<id>.md)"`
}

// --- kron_assumption_update -------------------------------------------

type AssumptionUpdateInput struct {
	ID              string   `json:"id"                        jsonschema:"assumption id to update (required)"`
	Text            *string  `json:"text,omitempty"            jsonschema:"replace text (null/absent = no change)"`
	DefaultSeverity *string  `json:"default_severity,omitempty" jsonschema:"replace default_severity (null/absent = no change; hard or soft)"`
	Body            *string  `json:"body,omitempty"            jsonschema:"replace body (null/absent = no change)"`
	Status          *string  `json:"status,omitempty"          jsonschema:"replace status (null/absent = no change; draft, active, or superseded)"`
	ExpiresAt       *string  `json:"expires_at,omitempty"       jsonschema:"replace expires_at (null/absent = no change; YYYY-MM-DD or RFC3339)"`
	VerifiedAt      *string  `json:"verified_at,omitempty"      jsonschema:"replace verified_at (null/absent = no change; YYYY-MM-DD or RFC3339)"`
	VerifiedBy      *string  `json:"verified_by,omitempty"      jsonschema:"replace verified_by (null/absent = no change)"`
	Reviewers       []string `json:"reviewers,omitempty"        jsonschema:"replace reviewers (null/absent = no change; [] = clear)"`
}

type AssumptionUpdateOutput struct {
	OK   bool   `json:"ok"   jsonschema:"true on success"`
	Path string `json:"path" jsonschema:"path to the updated assumption file (.kron/assumptions/<id>.md)"`
}

// --- kron_assumption_delete -------------------------------------------

type AssumptionDeleteInput struct {
	ID string `json:"id" jsonschema:"assumption id to soft-delete (moves to .kron/.trash/assumptions/)"`
}

type AssumptionDeleteOutput struct {
	OK                 bool     `json:"ok"                 jsonschema:"true on success"`
	TrashedPath        string   `json:"trashed_path"        jsonschema:"path the file was moved to"`
	ReferencingIntents []string `json:"referencing_intents" jsonschema:"intents that listed this assumption id in their frontmatter.assumptions[] (sorted; for caller awareness before deletion)"`
}

// --- kron_assumption_list --------------------------------------------

type AssumptionListInput struct {
	Status string `json:"status,omitempty" jsonschema:"optional filter: active (default, non-superseded) or superseded"`
}

type AssumptionListOutput struct {
	Assumptions []AssumptionSummary `json:"assumptions" jsonschema:"assumption files matching the filter (sorted by id)"`
}

// --- kron_assume_get -------------------------------------------------

// AssumptionGetInput is the wire shape for kron_assume_get.
type AssumptionGetInput struct {
	ID string `json:"id" jsonschema:"kebab-case slug (must pass parser.ValidateSlug)"`
}

// AssumptionGetOutput wraps the full detail (frontmatter + body + path).
type AssumptionGetOutput struct {
	Assumption AssumptionDetail `json:"assumption" jsonschema:"full assumption (id, text, default_severity, status, body, source_path)"`
}

// AssumptionDetail is the full wire projection of model.AssumptionFile,
// including the raw markdown body and on-disk path. Used by
// kron_assume_get. Plan-2026-10-13-mcp-completeness.md §4 (b): we
// expose a new Detail type (not extend AssumptionSummary) so the
// list response stays minimal.
type AssumptionDetail struct {
	ID              string   `json:"id"                          jsonschema:"kebab-case slug"`
	Text            string   `json:"text"                        jsonschema:"human-readable description"`
	DefaultSeverity string   `json:"default_severity"            jsonschema:"hard or soft"`
	Status          string   `json:"status"                      jsonschema:"draft / active / superseded"`
	CreatedBy       string   `json:"created_by"                  jsonschema:"@handle or agent:<model>"`
	UpdatedAt       string   `json:"updated_at"                  jsonschema:"RFC3339 timestamp"`
	ExpiresAt       string   `json:"expires_at,omitempty"        jsonschema:"optional expiry (YYYY-MM-DD or RFC3339)"`
	VerifiedAt      string   `json:"verified_at,omitempty"       jsonschema:"optional last verification (YYYY-MM-DD or RFC3339)"`
	VerifiedBy      string   `json:"verified_by,omitempty"       jsonschema:"optional last verifier @handle"`
	Reviewers       []string `json:"reviewers,omitempty"         jsonschema:"reviewer handles (sorted)"`
	Body            string   `json:"body,omitempty"              jsonschema:"raw markdown body below the frontmatter (empty if none)"`
	SourcePath      string   `json:"source_path"                 jsonschema:"absolute on-disk path of .kron/assumptions/<id>.md"`
}

// AssumptionSummary is the wire projection of model.AssumptionFile.
// Used in kron_assume_list / kron_assume_check / kron_delete
// (referencing_intents) responses. Stays free of body + source_path
// to keep the list response small; for full detail use kron_assume_get.
type AssumptionSummary struct {
	ID              string   `json:"id"               jsonschema:"assumption id (file name without .md)"`
	Text            string   `json:"text"             jsonschema:"human-readable description"`
	DefaultSeverity string   `json:"default_severity" jsonschema:"hard or soft"`
	Status          string   `json:"status"           jsonschema:"draft or active or superseded"`
	CreatedBy       string   `json:"created_by"       jsonschema:"@handle or agent:<model>"`
	UpdatedAt       string   `json:"updated_at"       jsonschema:"RFC3339 timestamp"`
	ExpiresAt       string   `json:"expires_at,omitempty" jsonschema:"optional expiry (YYYY-MM-DD or RFC3339)"`
	VerifiedAt      string   `json:"verified_at,omitempty" jsonschema:"optional last verification (YYYY-MM-DD or RFC3339)"`
	VerifiedBy      string   `json:"verified_by,omitempty" jsonschema:"optional last verifier @handle"`
	Reviewers       []string `json:"reviewers,omitempty"   jsonschema:"reviewer handles (sorted)"`
}

// --- kron_delete ------------------------------------------------------

type DeleteInput struct {
	Slug           string `json:"slug"                      jsonschema:"slug of the intent to soft-delete"`
	ConfirmSubtree bool   `json:"confirm_subtree,omitempty" jsonschema:"required for node intents: acknowledge that the whole subtree moves to trash"`
}

type DeleteOutput struct {
	OK           bool     `json:"ok"            jsonschema:"true on success"`
	TrashedPath  string   `json:"trashed_path"  jsonschema:"relative path the entry was moved to (file for leaf, directory for node)"`
	Dependents   []string `json:"dependents"    jsonschema:"intents that depend on the deleted entry or its subtree (sorted)"`
	DeletedSlugs []string `json:"deleted_slugs,omitempty" jsonschema:"every intent slug removed by this call (node subtree included)"`
}

// --- kron_restore -----------------------------------------------------

type RestoreInput struct {
	Slug string `json:"slug" jsonschema:"slug to restore from .kron/.trash/"`
}

type RestoreOutput struct {
	OK            bool     `json:"ok"             jsonschema:"true on success"`
	Path          string   `json:"path"           jsonschema:"relative path the entry was restored to (.kron/intents/<slug>.md or <slug>/)"`
	RestoredSlugs []string `json:"restored_slugs,omitempty" jsonschema:"every intent slug brought back by this call (node subtree included)"`
}

// --- kron_assume_check -----------------------------------------------

type AssumeCheckInput struct {
	FilePath string `json:"file_path,omitempty" jsonschema:"optional source file path; only assumptions of intents anchored to it are inspected"`
}

type AssumeWarning struct {
	IntentSlug      string `json:"intent_slug"             jsonschema:"slug of the intent owning the assumption"`
	AssumptionID    string `json:"assumption_id"           jsonschema:"stable assumption identifier"`
	Severity        string `json:"severity"                 jsonschema:"hard or soft (per-intent; overrides registry default)"`
	DefaultSeverity string `json:"default_severity,omitempty" jsonschema:"registry default severity (B-3); empty if registry absent"`
	Text            string `json:"text"                     jsonschema:"human-readable claim (preferred from registry)"`
	Rationale       string `json:"rationale,omitempty"     jsonschema:"per-intent explanation of why this severity (B-3)"`
	ExpiresAt       string `json:"expires_at,omitempty"     jsonschema:"optional expiry (YYYY-MM-DD or RFC3339)"`
	DaysRemaining   *int   `json:"days_remaining,omitempty" jsonschema:"optional days until expiry (negative if expired)"`
	Verified        bool   `json:"verified"                jsonschema:"true if VerifiedAt is set"`
}

type AssumeCheckSummary struct {
	Hard    int `json:"hard"    jsonschema:"count of hard-severity assumption warnings"`
	Soft    int `json:"soft"    jsonschema:"count of soft-severity assumption warnings"`
	Expired int `json:"expired" jsonschema:"count of expired hard-severity assumptions"`
}

type AssumeCheckOutput struct {
	Warnings []AssumeWarning    `json:"warnings" jsonschema:"list of assumption warnings"`
	Summary  AssumeCheckSummary `json:"summary"  jsonschema:"aggregate counters"`
}

// --- kron_impact ------------------------------------------------------

type ImpactInput struct {
	Slug string `json:"slug" jsonschema:"slug of the intent to inspect"`
}

type AnchorRef struct {
	FilePath string `json:"file_path" jsonschema:"repo-relative path of the source file containing the anchor"`
	Line     int    `json:"line"      jsonschema:"1-based line number of the @kron:intent line"`
	Kind     string `json:"kind"      jsonschema:"anchor surface: 'code' (source file) or 'markdown' (.md outside .kron/intents/); carried by model.Anchor.Kind per RFC 2026-10-08-md-anchors.md §2.5"`
}

type ImpactOutput struct {
	Intent IntentSummary `json:"intent"            jsonschema:"summary of the target intent"`
	// IncomingAnchors is the legacy unified field; new callers should
	// prefer the split fields below. Retained for backward compatibility.
	IncomingAnchors []AnchorRef `json:"incoming_anchors" jsonschema:"DEPRECATED: use incoming_anchors_code or incoming_anchors_markdown; combined list sorted by (file, line)"`
	// IncomingAnchorsCode are the anchors from source files (Kind = "code").
	// C-6: split from the unified incoming_anchors so GUI/IDE can render
	// two tabs without client-side filtering.
	IncomingAnchorsCode     []AnchorRef `json:"incoming_anchors_code" jsonschema:"@kron:intent lines from source files, sorted by (file, line)"`
	IncomingAnchorsMarkdown []AnchorRef `json:"incoming_anchors_markdown" jsonschema:"@kron:intent lines from .md files outside .kron/intents/, sorted by (file, line)"`
	References              []string    `json:"references"        jsonschema:"slugs that list slug in their References (sorted)"`
	Prerequisites           []string    `json:"prerequisites"     jsonschema:"explicit depends_on ∪ symbol-inferred deps (sorted)"`
}

// --- kron_intent_density --------------------------------------------

type IntentDensityInput struct{} // no parameters

type Coverage struct {
	IntentsWithAnchors    int      `json:"intents_with_anchors"   jsonschema:"number of intents with at least one anchor"`
	IntentsWithoutAnchors []string `json:"intents_without_anchors" jsonschema:"slugs with zero anchors (sorted)"`
}

type IntentDensityOutput struct {
	TotalIntents       int      `json:"total_intents"        jsonschema:"count of intents in .kron/intents/"`
	TotalAnchors       int      `json:"total_anchors"        jsonschema:"count of @kron:intent lines in the repo"`
	Coverage           Coverage `json:"coverage"             jsonschema:"anchor coverage summary"`
	FilesWithoutIntent []string `json:"files_without_intent" jsonschema:"repo-relative paths of source files >50 lines with no anchors (sorted)"`
}

// --- kron_tree -------------------------------------------------------

// TreeInput is empty in v1; future fields may include depth limit /
// status filter / with_anchors.
type TreeInput struct{}

// TreeNode is one entry in the kron_tree response. Mirrors
// internal/view.IntentTreeNode's projection to the wire: when a
// node has no Intent (purely-synthetic intermediate directory),
// Slug/Name/IsDir still describe the grouping but Title/Status are
// empty. Directories sort before leaves at the same level.
//
// Children is typed as []any (pre-serialised to a slice at handler
// time) rather than []*TreeNode because the official go-sdk v1.1
// jsonschema inference panics on recursive struct pointers ("cycle
// detected"). The wire-level shape is identical to a JSON array of
// TreeNode objects; consumers decode with their language's normal
// JSON-array-of-objects API. This is a SDK-induced wire-shape
// quirk; if a future SDK version fixes the cycle detection we can
// revert to []*TreeNode.
type TreeNode struct {
	Slug     string `json:"slug"                 jsonschema:"full slug of this node (empty for root)"`
	Name     string `json:"name"                 jsonschema:"final path segment (empty for root)"`
	IsDir    bool   `json:"is_dir"               jsonschema:"true when this node has children"`
	Title    string `json:"title,omitempty"      jsonschema:"first H1 from intent body, empty for synthetic intermediate dirs"`
	Status   string `json:"status,omitempty"     jsonschema:"draft / active / superseded, empty for synthetic intermediate dirs"`
	Children []any  `json:"children,omitempty"   jsonschema:"child nodes (directories first, then lexicographic name) — []any to break schema cycle on go-sdk v1.1"`
}

type TreeOutput struct {
	Root *TreeNode `json:"root" jsonschema:"root of the intent directory tree"`
}

// --- kron_stale ------------------------------------------------------

type StaleInput struct {
	DaysThreshold int `json:"days_threshold,omitempty" jsonschema:"minimum age in days for an active intent to be a superseded candidate (default 90)"`
}

type ExpiredAssumption = lint.ExpiredAssumption

type StaleOutput struct {
	SupersededCandidates []string                 `json:"superseded_candidates" jsonschema:"active intents older than days_threshold"`
	ExpiredAssumptions   []lint.ExpiredAssumption `json:"expired_assumptions"   jsonschema:"unverified hard assumptions past expiry"`
}

// --- registerTools ----------------------------------------------------

// registerTools wires every kron_* tool into server via the SDK's
// typed AddTool helper. The SDK reads the In/Out struct fields and
// jsonschema tags to derive tools/list schemas automatically, so
// we never specify InputSchema/OutputSchema by hand.
//
// Handler functions are defined alongside each tool's wire struct
// (in handlers_*.go) for locality: when a schema field is added, the
// matching handler signature change is right next to it.
//
// Tools are registered in this stable, alphabetical-by-same-name
// order so that:
//
//   - tools/list response is byte-identical across process starts,
//     letting prompt caches (and human readers) rely on order.
//   - Diff reviews of this file show one line per added tool.
//
// If you add a tool, append it to this slice AND its handler to
// handlers_*.go with consistent naming.
// toolAnnotations returns MCP safety hints for one tool. Kron tools only
// touch the local .kron/ tree, so openWorldHint is always false. The
// destructive flag matters for client confirmation UX: `kron_delete` /
// `kron_assume_delete` remove files, while add/update/restore are additive.
func toolAnnotations(readOnly, destructive, idempotent bool) *mcp.ToolAnnotations {
	openWorld := false
	return &mcp.ToolAnnotations{
		ReadOnlyHint:    readOnly,
		DestructiveHint: &destructive,
		IdempotentHint:  idempotent,
		OpenWorldHint:   &openWorld,
	}
}

// registerTools installs the 18 kron_* tools with their wire metadata.
//
// Every tool carries a human-readable Title plus MCP ToolAnnotations so
// clients can render safe confirmations (read-only vs destructive) and
// agents can reason about repeatability.
func registerTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "kron_add", Title: "Create intent",
		Description: "Create a new intent file at .kron/intents/<slug>.md.",
		Annotations: toolAnnotations(false, false, false),
	}, HandleAdd)
	mcp.AddTool(server, &mcp.Tool{
		Name: "kron_assume_check", Title: "Check assumptions",
		Description: "Inspect every intent's assumptions[] and surface expired / unverified hard assumptions.",
		Annotations: toolAnnotations(true, false, true),
	}, HandleAssumeCheck)
	mcp.AddTool(server, &mcp.Tool{
		Name: "kron_assume_create", Title: "Create assumption",
		Description: "Create a new assumption registry file at .kron/assumptions/<id>.md.",
		Annotations: toolAnnotations(false, false, false),
	}, HandleAssumptionCreate)
	mcp.AddTool(server, &mcp.Tool{
		Name: "kron_assume_delete", Title: "Delete assumption",
		Description: "Soft-delete an assumption (move to .kron/.trash/assumptions/).",
		Annotations: toolAnnotations(false, true, true),
	}, HandleAssumptionDelete)
	mcp.AddTool(server, &mcp.Tool{
		Name: "kron_assume_get", Title: "Get assumption",
		Description: "Load a single assumption by id (full frontmatter + body + source path).",
		Annotations: toolAnnotations(true, false, true),
	}, HandleAssumptionGet)
	mcp.AddTool(server, &mcp.Tool{
		Name: "kron_assume_list", Title: "List assumptions",
		Description: "List every assumption in the registry, optionally filtered by status.",
		Annotations: toolAnnotations(true, false, true),
	}, HandleAssumptionList)
	mcp.AddTool(server, &mcp.Tool{
		Name: "kron_assume_update", Title: "Update assumption",
		Description: "Patch an existing assumption registry file (PATCH semantics).",
		Annotations: toolAnnotations(false, false, true),
	}, HandleAssumptionUpdate)
	mcp.AddTool(server, &mcp.Tool{
		Name: "kron_delete", Title: "Delete intent",
		Description: "Soft-delete an intent (move to .kron/.trash/). Node intents require confirm_subtree.",
		Annotations: toolAnnotations(false, true, true),
	}, HandleDelete)
	mcp.AddTool(server, &mcp.Tool{
		Name: "kron_get", Title: "Get intent",
		Description: "Load a single intent by slug (full frontmatter + body).",
		Annotations: toolAnnotations(true, false, true),
	}, HandleGet)
	mcp.AddTool(server, &mcp.Tool{
		Name: "kron_impact", Title: "Intent impact",
		Description: "Reverse-link view: who depends on, references, and anchors an intent.",
		Annotations: toolAnnotations(true, false, true),
	}, HandleImpact)
	mcp.AddTool(server, &mcp.Tool{
		Name: "kron_init", Title: "Initialize Kron project",
		Description: "Create the .kron/ skeleton (idempotent) and register the project as current.",
		Annotations: toolAnnotations(false, false, true),
	}, HandleInit)
	mcp.AddTool(server, &mcp.Tool{
		Name: "kron_intent_density", Title: "Intent coverage density",
		Description: "Cross-an intent-coverage: how many intents have anchors, which source files lack anchors.",
		Annotations: toolAnnotations(true, false, true),
	}, HandleIntentDensity)
	mcp.AddTool(server, &mcp.Tool{
		Name: "kron_lint", Title: "Lint intents",
		Description: "Run lint over .kron/intents/ (frontmatter, dangling refs, depends_on cycles, assumption rules).",
		Annotations: toolAnnotations(true, false, true),
	}, HandleLint)
	mcp.AddTool(server, &mcp.Tool{
		Name: "kron_list", Title: "List intents",
		Description: "List every intent (with optional prefix filter), including assumptions and link fields.",
		Annotations: toolAnnotations(true, false, true),
	}, HandleList)
	mcp.AddTool(server, &mcp.Tool{
		Name: "kron_restore", Title: "Restore intent",
		Description: "Restore a soft-deleted intent from .kron/.trash/ (node subtrees restore as a whole).",
		Annotations: toolAnnotations(false, false, true),
	}, HandleRestore)
	mcp.AddTool(server, &mcp.Tool{
		Name: "kron_stale", Title: "Find stale intents",
		Description: "Find active intents older than days_threshold and expired hard assumptions.",
		Annotations: toolAnnotations(true, false, true),
	}, HandleStale)
	mcp.AddTool(server, &mcp.Tool{
		Name: "kron_tree", Title: "Intent tree",
		Description: "Return the full intent directory tree (root + nested children) for GUI/IDE tree views.",
		Annotations: toolAnnotations(true, false, true),
	}, HandleTree)
	mcp.AddTool(server, &mcp.Tool{
		Name: "kron_update", Title: "Update intent",
		Description: "Patch an existing intent (PATCH semantics; only supplied fields are touched).",
		Annotations: toolAnnotations(false, false, true),
	}, HandleUpdate)
}
