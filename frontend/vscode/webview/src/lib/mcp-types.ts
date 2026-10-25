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
 * tool list changes. `isWebviewInbound` validates host messages in
 * this runtime; the extension host must validate its own inputs too.
 */

export type TreeNode = {
    slug: string
    name: string
    is_dir: boolean
    workspaceId?: string
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
    | { type: 'kron_mcp:response'; requestId: number; tool: McpToolName; result: Record<string, unknown> }
    | { type: 'kron_mcp:error'; requestId: number; tool: McpToolName; message: string }

export type McpToolName =
    | 'kron_get' | 'kron_add' | 'kron_update' | 'kron_delete' | 'kron_restore'
    | 'kron_assume_list' | 'kron_assume_create'

/** Messages the webview can send to the extension host. */
export type WebviewOutbound =
    | { type: 'kron_tree:request' }
    | { type: 'kron_tree:refresh' }
    | { type: 'kron_mcp:request'; requestId: number; tool: McpToolName; args: Record<string, unknown>; workspaceId?: string }

function isRecord(value: unknown): value is Record<string, unknown> {
    return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function isTreeNode(value: unknown): value is TreeNode {
    if (!isRecord(value)) return false

    const children = value.children
    return typeof value.slug === 'string'
        && typeof value.name === 'string'
        && typeof value.is_dir === 'boolean'
        && (value.workspaceId === undefined || typeof value.workspaceId === 'string')
        && (value.title === undefined || typeof value.title === 'string')
        && (value.status === undefined || typeof value.status === 'string')
        && (children === undefined || (Array.isArray(children) && children.every(isTreeNode)))
}

function isTreeResult(value: unknown): value is TreeResult {
    return isRecord(value) && isTreeNode(value.root)
}

/** Validate messages received across the extension-host/Webview boundary. */
export function isWebviewInbound(value: unknown): value is WebviewInbound {
    if (!isRecord(value)) return false

    if (value.type === 'kron_tree:result') return isTreeResult(value.result)
    if (value.type === 'kron_tree:error') return typeof value.message === 'string'
    if (value.type === 'kron_mcp:response') {
        return Number.isSafeInteger(value.requestId)
            && isMcpToolName(value.tool)
            && isRecord(value.result)
    }
    if (value.type === 'kron_mcp:error') {
        return Number.isSafeInteger(value.requestId)
            && isMcpToolName(value.tool)
            && typeof value.message === 'string'
    }
    return false
}

function isMcpToolName(value: unknown): value is McpToolName {
    return value === 'kron_get' || value === 'kron_add' || value === 'kron_update'
        || value === 'kron_delete' || value === 'kron_restore'
        || value === 'kron_assume_list' || value === 'kron_assume_create'
}
