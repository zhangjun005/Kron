package servemcp

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/xxx/kron/internal/lint"
)

// This file holds the typed input / output schemas for the 12 MCP
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
	Rule     string `json:"rule"     jsonschema:"rule code, e.g. anchor-dangling / frontmatter-invalid"`
	Where    string `json:"where"    jsonschema:"file:line or slug where the issue is"`
	Severity string `json:"severity" jsonschema:"error or warning"`
	Detail   string `json:"detail"   jsonschema:"human-readable description"`
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

type AssumeEntry struct {
	ID              string `json:"id"                          jsonschema:"stable assumption identifier"`
	Text            string `json:"text"                        jsonschema:"human-readable claim (preferred from registry; empty if neither present)"`
	Severity        string `json:"severity"                    jsonschema:"hard or soft (per-intent; overrides registry default)"`
	DefaultSeverity string `json:"default_severity,omitempty"  jsonschema:"registry default severity (B-3); empty if registry absent"`
	Rationale       string `json:"rationale,omitempty"         jsonschema:"per-intent explanation of why this severity (B-3)"`
	ExpiresAt       string `json:"expires_at,omitempty"        jsonschema:"optional RFC3339 expiry"`
	VerifiedAt      string `json:"verified_at,omitempty"       jsonschema:"last verification time (empty if never verified)"`
	VerifiedBy      string `json:"verified_by,omitempty"       jsonschema:"handle that last verified"`
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
}

type UpdateOutput struct {
	OK   bool   `json:"ok"   jsonschema:"true on success"`
	Path string `json:"path" jsonschema:"relative path to the patched intent file"`
}

// --- kron_delete ------------------------------------------------------

type DeleteInput struct {
	Slug string `json:"slug" jsonschema:"slug of the intent to soft-delete"`
}

type DeleteOutput struct {
	OK          bool     `json:"ok"           jsonschema:"true on success"`
	TrashedPath string   `json:"trashed_path" jsonschema:"relative path the file was moved to"`
	Dependents  []string `json:"dependents"   jsonschema:"intents that listed slug in depends_on (sorted, for caller awareness)"`
}

// --- kron_restore -----------------------------------------------------

type RestoreInput struct {
	Slug string `json:"slug" jsonschema:"slug to restore from .kron/.trash/"`
}

type RestoreOutput struct {
	OK   bool   `json:"ok"   jsonschema:"true on success"`
	Path string `json:"path" jsonschema:"relative path the file was moved to (.kron/intents/<slug>.md)"`
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
	ExpiresAt       string `json:"expires_at,omitempty"     jsonschema:"optional RFC3339 expiry"`
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
	Intent          IntentSummary `json:"intent"            jsonschema:"summary of the target intent"`
	IncomingAnchors []AnchorRef   `json:"incoming_anchors"  jsonschema:"@kron:intent lines pointing at slug, sorted by (file, line)"`
	References      []string      `json:"references"        jsonschema:"slugs that list slug in their References (sorted)"`
	Prerequisites   []string      `json:"prerequisites"     jsonschema:"explicit depends_on ∪ symbol-inferred deps (sorted)"`
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
func registerTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{Name: "kron_add", Description: "Create a new intent file at .kron/intents/<slug>.md."}, HandleAdd)
	mcp.AddTool(server, &mcp.Tool{Name: "kron_assume_check", Description: "Inspect every intent's assumptions[] and surface expired / unverified hard assumptions."}, HandleAssumeCheck)
	mcp.AddTool(server, &mcp.Tool{Name: "kron_delete", Description: "Soft-delete an intent (move to .kron/.trash/)."}, HandleDelete)
	mcp.AddTool(server, &mcp.Tool{Name: "kron_get", Description: "Load a single intent by slug (full frontmatter + body)."}, HandleGet)
	mcp.AddTool(server, &mcp.Tool{Name: "kron_impact", Description: "Reverse-link view: who depends on, references, and anchors an intent."}, HandleImpact)
	mcp.AddTool(server, &mcp.Tool{Name: "kron_init", Description: "Create the .kron/ skeleton (idempotent)."}, HandleInit)
	mcp.AddTool(server, &mcp.Tool{Name: "kron_intent_density", Description: "Cross-an intent-coverage: how many intents have anchors, which source files lack anchors."}, HandleIntentDensity)
	mcp.AddTool(server, &mcp.Tool{Name: "kron_lint", Description: "Run lint over .kron/intents/ (frontmatter, dangling refs, depends_on cycles)."}, HandleLint)
	mcp.AddTool(server, &mcp.Tool{Name: "kron_list", Description: "List every intent (with optional prefix filter)."}, HandleList)
	mcp.AddTool(server, &mcp.Tool{Name: "kron_restore", Description: "Restore a soft-deleted intent from .kron/.trash/."}, HandleRestore)
	mcp.AddTool(server, &mcp.Tool{Name: "kron_stale", Description: "Find active intents older than days_threshold and expired hard assumptions."}, HandleStale)
	mcp.AddTool(server, &mcp.Tool{Name: "kron_tree", Description: "Return the full intent directory tree (root + nested children) for GUI/IDE tree views."}, HandleTree)
	mcp.AddTool(server, &mcp.Tool{Name: "kron_update", Description: "Patch an existing intent (PATCH semantics; only supplied fields are touched)."}, HandleUpdate)
}
