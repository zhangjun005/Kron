// Kron GUI — kron_tree → IntentNode adapter (work 1.4.c).
//
// The mock IntentNode shape (src/lib/mock.ts) was designed for the
// 1.3-era demo and carries fields kron_tree does not surface
// (kind, updatedAt, summary). To keep the existing IntentTree
// component (src/components/intent-tree.tsx) untouched, we adapt
// the real wire shape into the mock shape here, fetching kron_get
// for each leaf to fill in updatedAt and a body-derived summary.
//
// Mapping rules (documented; the iron rule is "always round-trip,
// never invent"):
//
//   IntentNode.id       = TreeNode.slug     (or path for the synthetic root)
//   IntentNode.title    = TreeNode.title ?? TreeNode.name ?? slug
//   IntentNode.kind     = "spec" when is_dir (grouping), else "note"
//                          (we have no "adr/rfc/spec/note" signal on
//                          the wire; a richer inference arrives when
//                          frontmatter.kind is added to kron_tree's
//                          projection. See follow-up 1.4.d.)
//   IntentNode.status   = mapMcpStatus(TreeNode.status)
//   IntentNode.updatedAt= from kron_get.frontmatter.updated_at
//                          (ISO 8601 string, as the mock expects)
//   IntentNode.summary  = first non-empty paragraph of kron_get.body
//   IntentNode.children = recursively converted
//
// We do not touch src/lib/mock.ts so the mock fallback path keeps
// working unchanged.

import type {IntentKind, IntentNode, IntentStatus} from '@/lib/mock';
import type {GetOutput, TreeNode as McpTreeNode, TreeOutput} from '@/lib/mcp-types';
import {CallMCPTool} from '../../wailsjs/go/main/App';

/**
 * Convert MCP status string to the mock's status enum.
 *
 * MCP / kron_get frontmatter uses:
 *   "draft" | "active" | "superseded"
 * The mock UI uses:
 *   "draft" | "review" | "accepted" | "stale"
 *
 * Mapping:
 *   "draft"       -> "draft"     (1:1)
 *   "active"      -> "accepted"  (most active intents are accepted
 *                                  in practice; until a "review"
 *                                  state lands on the wire we
 *                                  collapse active→accepted)
 *   "superseded"  -> "stale"     (closest semantic match in the
 *                                  mock palette)
 *   anything else -> "draft"     (defensive default; never throws)
 */
export function mapMcpStatus(raw: string | undefined): IntentStatus {
    switch (raw) {
        case 'draft':
            return 'draft';
        case 'active':
            return 'accepted';
        case 'superseded':
            return 'stale';
        default:
            return 'draft';
    }
}

/**
 * Best-effort summary extraction from the markdown body. Picks the
 * first non-empty, non-heading line and trims it to a single
 * readable snippet. Returns undefined when the body has no such
 * line, so the consumer can choose whether to render a summary row.
 *
 * Why "first line, not first paragraph": the mock summaries are
 * one-liners ("Single static binary, fast cold start, …"). A
 * paragraph slice would surface the long first body paragraph and
 * blow out the row width.
 */
export function summaryFromBody(body: string | undefined): string | undefined {
    if (body === undefined || body === '') return undefined;
    const lines = body.split(/\r?\n/);
    for (const line of lines) {
        const trimmed = line.trim();
        if (trimmed === '') continue;
        // Skip headings, blockquote markers, list bullets, and the
        // typical "## Why" / "## What" stubs that open an intent.
        if (trimmed.startsWith('#')) continue;
        if (trimmed.startsWith('>')) continue;
        if (trimmed.startsWith('-') || trimmed.startsWith('*')) continue;
        if (trimmed.startsWith('```')) continue;
        // Cap at 120 chars to match the mock's typical summary width.
        if (trimmed.length > 120) {
            return trimmed.slice(0, 117) + '…';
        }
        return trimmed;
    }
    return undefined;
}

/**
 * Recursively convert a kron_tree TreeNode into the mock's
 * IntentNode shape, fetching kron_get for each leaf to fill in
 * updatedAt and summary.
 *
 * The synthetic root (slug="", name="", is_dir=true) is unwrapped:
 * its children become the top-level IntentNode[] returned to the
 * caller. This matches the mock's array-of-top-level-groups shape.
 *
 * Per-leaf kron_get calls are bounded by Promise.allSettled so one
 * failing fetch does not poison the whole tree. Leaves that fail
 * to enrich fall back to a synthesised updatedAt (epoch) and no
 * summary; the row still renders, just with a "—" date pill.
 */
export async function kronTreeToIntentNodes(
    output: TreeOutput,
): Promise<IntentNode[]> {
    const root = output.root;
    const rootChildren = root.children ?? [];

    // Parallelise kron_get for every leaf under the root so the
    // wall-clock cost is one round-trip, not N. We collect slugs
    // up-front (depth-first, leaves only) and fire all calls in
    // parallel; failures degrade gracefully via summaryFromBody
    // returning undefined.
    const leaves = collectLeaves(rootChildren);
    const enrichmentEntries = await Promise.all(
        leaves.map(async (slug): Promise<[string, GetOutput | undefined]> => {
            try {
                const text = await CallMCPTool('kron_get', JSON.stringify({slug}));
                const parsed = JSON.parse(text) as GetOutput;
                return [slug, parsed];
            } catch (err) {
                // Per the design note: a single failed enrichment
                // must not poison the tree. The fallback row uses
                // an "epoch" updatedAt so the relativeDate helper
                // shows a very-old date — easy to spot visually as
                // a degraded entry.
                console.warn(`kron_get(${slug}) failed:`, err);
                return [slug, undefined];
            }
        }),
    );
    const enrichmentMap = new Map<string, GetOutput | undefined>(
        enrichmentEntries,
    );

    return rootChildren.map((node) =>
        convertNode(node, enrichmentMap, ''),
    );
}

/**
 * Recursive converter. depth is the slug-prefix accumulator so
 * nested children of <slug>/<child> get the right kron_get key
 * (e.g. "g-foo/i-bar"). For the synthetic root depth is "" and
 * we use the node's own slug as-is.
 */
function convertNode(
    node: McpTreeNode,
    enrichment: Map<string, GetOutput | undefined>,
    parentSlug: string,
): IntentNode {
    // The on-disk slug for this node (full path, with parent
    // prefix when nested). When parentSlug is non-empty we
    // concatenate; otherwise the node's own slug is the full
    // path. Empty slug (synthetic root) is filtered out upstream.
    const fullSlug =
        parentSlug === '' ? node.slug : `${parentSlug}/${node.slug || node.name}`;

    const kind: IntentKind = node.is_dir ? 'spec' : 'note';
    const status: IntentStatus = mapMcpStatus(node.status);
    const title: string = node.title?.trim() || node.name || node.slug;

    // Leaf enrichment: pull updatedAt + summary from kron_get.
    // We only enrich leaves (is_dir === false); directories get
    // updatedAt = epoch (they don't carry one on the wire today
    // either — see RFC TODO for kron_tree group metadata).
    const enriched = node.is_dir ? undefined : enrichment.get(fullSlug);
    const updatedAt: string =
        enriched?.intent.frontmatter.updated_at ?? new Date(0).toISOString();
    const summary: string | undefined = node.is_dir
        ? undefined
        : summaryFromBody(enriched?.intent.body);

    const children: IntentNode[] = (node.children ?? []).map((child) =>
        convertNode(child, enrichment, fullSlug),
    );

    return {
        id: fullSlug,
        title,
        kind,
        status,
        updatedAt,
        // exactOptionalPropertyTypes is on in tsconfig — optional
        // properties cannot be assigned `undefined` explicitly.
        // When summaryFromBody returns undefined we omit the key
        // via conditional spread so the resulting object matches
        // the IntentNode interface exactly.
        ...(summary !== undefined ? {summary} : {}),
        children,
    };
}

/** Collect every leaf slug under the given root-level nodes. */
function collectLeaves(nodes: McpTreeNode[]): string[] {
    const out: string[] = [];
    const stack: Array<{node: McpTreeNode; parent: string}> = nodes.map(
        (n) => ({node: n, parent: ''}),
    );
    while (stack.length > 0) {
        const {node, parent} = stack.pop()!;
        const fullSlug =
            parent === '' ? node.slug : `${parent}/${node.slug || node.name}`;
        if (node.is_dir) {
            for (const child of node.children ?? []) {
                stack.push({node: child, parent: fullSlug});
            }
        } else {
            out.push(fullSlug);
        }
    }
    return out;
}
