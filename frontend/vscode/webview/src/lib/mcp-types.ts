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
    | { type: 'kron_font_size'; fontSize: number }
    | { type: 'kron_select'; slug: string; edit: boolean }
    | { type: 'kron_mcp:response'; requestId: number; tool: McpToolName; result: Record<string, unknown> }
    | { type: 'kron_mcp:error'; requestId: number; tool: McpToolName; message: string }

/**
 * Tools the Webview may request. Keep in sync with MCP_TOOL_NAMES in
 * frontend/vscode/src/mcp/client.ts (single source: that file's comment).
 */
export const MCP_TOOL_NAMES = [
    'kron_add', 'kron_assume_check', 'kron_assume_create', 'kron_assume_delete', 'kron_assume_get',
    'kron_assume_list', 'kron_assume_update', 'kron_delete', 'kron_get', 'kron_restore', 'kron_update',
] as const

export type McpToolName = typeof MCP_TOOL_NAMES[number]

/** Messages the webview can send to the extension host. */
export type WebviewOutbound =
    | { type: 'kron_tree:request' }
    | { type: 'kron_tree:refresh' }
    | { type: 'kron_mcp:request'; requestId: number; tool: McpToolName; args: Record<string, unknown>; workspaceId?: string }
    | { type: 'kron_open_intent_code'; slug: string }

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
    if (value.type === 'kron_font_size') {
        return typeof value.fontSize === 'number' && Number.isFinite(value.fontSize)
            && value.fontSize >= 10 && value.fontSize <= 24
    }
    if (value.type === 'kron_select') {
        return typeof value.slug === 'string' && value.slug !== '' && typeof value.edit === 'boolean'
    }
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
    return typeof value === 'string' && (MCP_TOOL_NAMES as readonly string[]).includes(value)
}
