// Kron GUI — MCP wire types (work 1.4.c).
//
// These mirror the JSON shapes produced by the Go-side
// `kron serve-mcp` subprocess. The Go definitions live in
// Kron/cmd/kron/serve-mcp/tools_schema.go; whenever a tool is
// added or a field changes there, the matching TS shape here must
// be kept in sync.
//
// The shapes are intentionally narrow: only the fields the React
// side actually reads. Optional fields use `| undefined` so
// `exactOptionalPropertyTypes` (tsconfig) stays happy.

/**
 * kron_tree result (cmd/kron/serve-mcp/handlers_tree.go +
 * tools_schema.go::TreeNode). The wire shape uses `[]any` for
 * children to work around a go-sdk v1.1 jsonschema inference
 * panic on recursive struct pointers — on the TS side we narrow
 * the elements back to TreeNode recursively.
 */
export interface TreeNode {
    /** Full slug of this node (empty string for the synthetic root). */
    slug: string;
    /** Final path segment (empty string for the synthetic root). */
    name: string;
    /** True when this node has children. */
    is_dir: boolean;
    /** First H1 from the intent body; empty for synthetic intermediate dirs. */
    title?: string;
    /** draft / active / superseded; empty for synthetic intermediate dirs. */
    status?: string;
    /** Child nodes; directories sort before leaves at the same level. */
    children?: TreeNode[];
}

/** Result of the kron_tree JSON-RPC call. */
export interface TreeOutput {
    root: TreeNode;
}

// --- kron_get (1.4.c: per-leaf metadata enrichment) ------------------

/** A single assumption attached to an intent. Mirrors AssumeEntry. */
export interface AssumeEntry {
    id: string;
    text?: string;
    severity: string;
    default_severity?: string;
    rationale?: string;
    expires_at?: string;
    verified_at?: string;
    verified_by?: string;
}

/** Frontmatter projection of an intent. */
export interface IntentFrontmatter {
    symbol?: string[];
    created_by: string;
    updated_at: string;
    reviewers?: string[];
    status?: string;
    assumptions?: AssumeEntry[];
    references?: string[];
    depends_on?: string[];
}

/** Full body of an intent returned by kron_get. */
export interface IntentBody {
    slug: string;
    frontmatter: IntentFrontmatter;
    body: string;
    source_path: string;
    references?: string[];
    depends_on?: string[];
    relations?: Record<string, unknown>;
}

export interface GetOutput {
    intent: IntentBody;
}

// --- projects.json registry bridge ---------------------------------------
// The Go bridge reads the portable registry and converts snake_case row
// fields to this camelCase shape for the React UI.

/** One row of the projects.json registry. */
export interface ProjectEntry {
    path: string;
    addedAt?: string;
    lastOpened?: string;
}

/** Return type of ListProjectsLive (mirrors the Go struct). */
export interface ListProjectsLiveResult {
    projects: ProjectEntry[];
    /** Path of the active project, "" if none. */
    current: string;
    /** Absolute path of projects.json (for the diagnostic footer). */
    projectsPath: string;
    /** Human-readable provenance string (for example, the registry path). */
    source: string;
}
