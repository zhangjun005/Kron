/*
 * src/lib/mcp-types.ts — Kron MCP tool type contracts (webview side).
 *
 * Per RFC 2026-10-18-vscode-skeleton-sdk.md §4 #9, the webview calls
 * the extension host via window.postMessage, and the extension host
 * calls `kron serve-mcp` stdio for the actual tool. This file mirrors
 * the JSON shapes documented in the Kron MCP server.
 *
 * NOTE: the canonical tool list lives in
 *   frontend/wails/frontend/src/lib/mcp-types.ts  (Wails GUI side)
 * We intentionally do NOT share code between the two clients per
 * architecture iron rule #9. Keep this in sync manually when the
 * tool list changes; if it drifts, zod (HomoFluctus's choice in
 * src/views/intentTree.ts) will catch it on the extension-host side.
 */

export type TreeNode = {
    slug: string
    name: string
    is_dir: boolean
    title?: string
    status?: string
    children?: TreeNode[]
}

export type TreeResult = {
    root: TreeNode
}

/** Messages the webview can receive from the extension host. */
export type WebviewInbound =
    | { type: 'kron_tree:result'; result: TreeResult }
    | { type: 'kron_tree:error'; message: string }

/** Messages the webview can send to the extension host. */
export type WebviewOutbound = { type: 'kron_tree:request' } | { type: 'kron_tree:refresh' }
