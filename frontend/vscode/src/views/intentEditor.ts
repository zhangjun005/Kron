import crypto = require('node:crypto')
import fs = require('node:fs')
import path = require('node:path')
import vscode = require('vscode')
import type { IntentTreeProvider } from './intentTree'
import { MCP_TOOL_NAMES, type McpToolName } from '../mcp/client'

const REQUEST_TIMEOUT_MS = 15_000
let currentPanel: vscode.WebviewPanel | undefined

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function isTreeRequest(value: unknown): value is { type: 'kron_tree:request' | 'kron_tree:refresh' } {
  return isRecord(value)
    && Object.keys(value).length === 1
    && (value.type === 'kron_tree:request' || value.type === 'kron_tree:refresh')
}

type McpRequest = {
  type: 'kron_mcp:request'
  requestId: number
  tool: McpToolName
  args: Record<string, unknown>
  workspaceId?: string
}

function isMcpRequest(value: unknown): value is McpRequest {
  if (!isRecord(value)
    || !Number.isSafeInteger(value.requestId)
    || typeof value.tool !== 'string'
    || !MCP_TOOL_NAMES.includes(value.tool as McpToolName)
    || !isRecord(value.args)) return false
  const allowedKeys = new Set(['type', 'requestId', 'tool', 'args', 'workspaceId'])
  if (Object.keys(value).some((key) => !allowedKeys.has(key))) return false
  return value.workspaceId === undefined || typeof value.workspaceId === 'string'
}

function timeout<T>(promise: Promise<T>): Promise<T> {
  let timer: NodeJS.Timeout | undefined
  const deadline = new Promise<never>((_, reject) => {
    timer = setTimeout(() => reject(new Error('Kron did not respond in time. Try refreshing the tree.')), REQUEST_TIMEOUT_MS)
  })
  return Promise.race([promise, deadline]).finally(() => {
    if (timer !== undefined) clearTimeout(timer)
  })
}

function renderHtml(context: vscode.ExtensionContext, webview: vscode.Webview, fontSize: number): string {
  const webviewRoot = vscode.Uri.file(path.join(context.extensionPath, 'dist', 'webview'))
  const indexPath = path.join(webviewRoot.fsPath, 'index.html')
  let html: string
  try {
    html = fs.readFileSync(indexPath, 'utf8')
  } catch {
    throw new Error('The Kron Webview has not been built. Run the VSCode extension webview build command.')
  }

  const nonce = crypto.randomBytes(18).toString('base64')
  const policy = [
    "default-src 'none'",
    `img-src ${webview.cspSource} data:`,
    `font-src ${webview.cspSource}`,
    `style-src ${webview.cspSource}`,
    `script-src 'nonce-${nonce}' ${webview.cspSource}`,
  ].join('; ')
  const safeFontSize = Number.isFinite(fontSize) ? Math.min(24, Math.max(10, Math.round(fontSize))) : 14
  html = html.replace(/<head>/i, `<head>\n<meta http-equiv="Content-Security-Policy" content="${policy}">`)
  html = html.replace(/<body\b/i, `<body data-kron-font-size="${safeFontSize}"`)
  html = html.replace(/<script\b([^>]*)>/gi, (_tag, attributes: string) => `<script nonce="${nonce}"${attributes}>`)
  return html.replace(/\b(src|href)="([^"]+)"/g, (match, attribute: string, resource: string) => {
    if (/^(?:https?:|data:|#|vscode-webview:)/i.test(resource)) return match
    const relative = resource.replace(/^\.\//, '')
    const segments = relative.split('/')
    if (relative === '' || segments.some((segment) => segment === '..' || segment === '.')) return match
    return `${attribute}="${webview.asWebviewUri(vscode.Uri.joinPath(webviewRoot, ...segments))}"`
  })
}

async function sendTree(panel: vscode.WebviewPanel, provider: IntentTreeProvider, output: vscode.OutputChannel): Promise<void> {
  try {
    const result = await timeout(provider.refresh())
    await panel.webview.postMessage({ type: 'kron_tree:result', result })
  } catch (error: unknown) {
    output.appendLine(`Webview tree request failed: ${error instanceof Error ? error.message : String(error)}`)
    await panel.webview.postMessage({
      type: 'kron_tree:error',
      message: 'Kron could not load the intent tree. Check that the Kron executable is available, then refresh.',
    })
  }
}

/** Opens the read-only details workspace and owns its host/Webview message boundary. */
export function openIntentWorkspace(
  context: vscode.ExtensionContext,
  provider: IntentTreeProvider,
  output: vscode.OutputChannel,
): void {
  if (currentPanel !== undefined) {
    currentPanel.reveal(vscode.ViewColumn.Beside)
    void sendTree(currentPanel, provider, output)
    return
  }

  const panel = vscode.window.createWebviewPanel(
    'kron.intentWorkspace',
    'Kron Intent Workspace',
    vscode.ViewColumn.Beside,
    {
      enableScripts: true,
      localResourceRoots: [vscode.Uri.file(path.join(context.extensionPath, 'dist', 'webview'))],
    },
  )
  currentPanel = panel
  context.subscriptions.push(panel)

  try {
    const fontSize = vscode.workspace.getConfiguration('kron').get<number>('webviewFontSize', 14)
    panel.webview.html = renderHtml(context, panel.webview, fontSize)
  } catch (error: unknown) {
    output.appendLine(`Webview initialization failed: ${error instanceof Error ? error.message : String(error)}`)
    void vscode.window.showErrorMessage(error instanceof Error ? error.message : 'Unable to open the Kron Webview.')
    panel.dispose()
    currentPanel = undefined
    return
  }

  const messageSubscription = panel.webview.onDidReceiveMessage(async (message: unknown) => {
    if (isMcpRequest(message)) {
      try {
        const result = await timeout(provider.callTool(message.workspaceId, message.tool, message.args))
        await panel.webview.postMessage({ type: 'kron_mcp:response', requestId: message.requestId, tool: message.tool, result })
      } catch (error: unknown) {
        output.appendLine(`Webview ${message.tool} request failed: ${error instanceof Error ? error.message : String(error)}`)
        await panel.webview.postMessage({
          type: 'kron_mcp:error',
          requestId: message.requestId,
          tool: message.tool,
          message: `Kron could not complete ${message.tool}. Check the Kron output channel and try again.`,
        })
      }
      return
    }
    if (!isTreeRequest(message)) {
      output.appendLine('Ignored an invalid message from the Kron Webview.')
      return
    }
    await sendTree(panel, provider, output)
  })
  let disposeSubscription: vscode.Disposable | undefined
  disposeSubscription = panel.onDidDispose(() => {
    messageSubscription.dispose()
    disposeSubscription?.dispose()
    if (currentPanel === panel) currentPanel = undefined
  })
  void sendTree(panel, provider, output)
}
