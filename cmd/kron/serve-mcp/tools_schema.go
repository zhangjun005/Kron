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

type LintInput struct{} // no parameters

type LintDiag struct {
	Rule     string `json:"rule"     jsonschema:"rule code, e.g. A1 / A5 / A6 / D1"`
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
}

type IntentBody struct {
	Slug        string            `json:"slug"                         jsonschema:"kebab-case slug"`
	Frontmatter IntentFrontmatter `json:"frontmatter"                  jsonschema:"frontmatter fields (snake_case wire keys)"`
	Body        string            `json:"body"                         jsonschema:"raw markdown body"`
	SourcePath  string            `json:"source_path"                  jsonschema:"absolute on-disk path"`
	References  []string          `json:"references,omitempty"          jsonschema:"soft links to other intents"`
	DependsOn   []string          `json:"depends_on,omitempty"         jsonschema:"hard dependencies on other intents"`
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
	mcp.AddTool(server, &mcp.Tool{Name: "kron_update", Description: "Patch an existing intent (PATCH semantics; only supplied fields are touched)."}, HandleUpdate)
}
