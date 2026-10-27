import vscode = require('vscode')
import zod = require('zod')
import { KronLspClient, DEFAULT_LSP_LANGUAGES } from './lsp/client'
import { KronMcpClient, type ConnectionState, type TreeNode } from './mcp/client'
import { IntentTreeProvider, type WorkspaceMcpConnection } from './views/intentTree'
import { openIntentWorkspace } from './views/intentEditor'
import { KronStatusBar } from './views/statusBar'

const languageListSchema = zod.z.array(zod.z.string().trim().min(1)).min(1)

let output: vscode.OutputChannel | undefined
let statusBar: KronStatusBar | undefined
let mcpConnections: WorkspaceMcpConnection[] = []
let lspConnections: KronLspClient[] = []
let treeProvider: IntentTreeProvider | undefined
let restartPromise: Promise<void> | undefined
const anchorCreateTimers = new Map<string, NodeJS.Timeout>()
const anchorDecorationTimers = new Map<string, NodeJS.Timeout>()
const promptedAnchors = new Set<string>()
let anchorDecoration: vscode.TextEditorDecorationType | undefined

const INTENT_ANCHOR = /^\s*(?:\/\/|#|--)\s*@kron:intent\s+([a-z0-9][a-z0-9_-]*(?:\/[a-z0-9][a-z0-9_-]*)*)\s*$/

function log(message: string): void {
  output?.appendLine(message)
}

function updateMcpStates(): void {
  statusBar?.setMcpStates(mcpConnections.map(({ client }) => client.state))
}

function updateLspStates(): void {
  statusBar?.setLspStates(lspConnections.map((client) => client.state))
}

async function startMcpConnections(): Promise<void> {
  const results = await Promise.allSettled(mcpConnections.map(({ client }) => client.start()))
  for (const result of results) {
    if (result.status === 'rejected') log(`MCP startup failed: ${result.reason instanceof Error ? result.reason.message : String(result.reason)}`)
  }
}

async function startLspConnections(): Promise<void> {
  const results = await Promise.allSettled(lspConnections.map((client) => client.start()))
  for (const result of results) {
    if (result.status === 'rejected') log(`LSP startup failed: ${result.reason instanceof Error ? result.reason.message : String(result.reason)}`)
  }
}

async function startConnections(): Promise<void> {
  await Promise.all([startMcpConnections(), startLspConnections()])
}

async function refreshIntents(showError: boolean): Promise<void> {
  if (treeProvider === undefined) return
  try {
    await treeProvider.refresh()
  } catch (error: unknown) {
    log(`Intent tree refresh failed: ${error instanceof Error ? error.message : String(error)}`)
    if (showError) {
      void vscode.window.showErrorMessage('Kron could not load the intent tree. Check kron.serverPath and the Kron output channel.')
    }
    throw error
  }
}

function treeContainsSlug(node: TreeNode, slug: string): boolean {
  return node.slug === slug || (node.children ?? []).some((child) => treeContainsSlug(child, slug))
}

function scheduleQuickIntentPrompt(document: vscode.TextDocument, line: number): void {
  if (line < 0 || line >= document.lineCount) return
  const key = `${document.uri.toString()}\u0000${line}`
  const previous = anchorCreateTimers.get(key)
  if (previous !== undefined) clearTimeout(previous)
  const timer = setTimeout(() => {
    anchorCreateTimers.delete(key)
    void maybeOfferQuickIntent(document, line)
  }, 900)
  anchorCreateTimers.set(key, timer)
}

function updateIntentAnchorDecorations(document: vscode.TextDocument): void {
  if (anchorDecoration === undefined || document.uri.scheme !== 'file') return
  const ranges: vscode.Range[] = []
  for (let line = 0; line < document.lineCount; line += 1) {
    const text = document.lineAt(line).text
    const match = INTENT_ANCHOR.exec(text)
    const slug = match?.[1]
    if (match === null || slug === undefined) continue
    const start = match[0].lastIndexOf(slug)
    ranges.push(new vscode.Range(line, start, line, start + slug.length))
  }
  for (const editor of vscode.window.visibleTextEditors) {
    if (editor.document.uri.toString() === document.uri.toString()) {
      editor.setDecorations(anchorDecoration, ranges)
    }
  }
}

function scheduleIntentAnchorDecorations(document: vscode.TextDocument): void {
  const key = document.uri.toString()
  const previous = anchorDecorationTimers.get(key)
  if (previous !== undefined) clearTimeout(previous)
  const timer = setTimeout(() => {
    anchorDecorationTimers.delete(key)
    updateIntentAnchorDecorations(document)
  }, 120)
  anchorDecorationTimers.set(key, timer)
}

async function maybeOfferQuickIntent(document: vscode.TextDocument, line: number): Promise<void> {
  if (line < 0 || line >= document.lineCount) return
  const match = INTENT_ANCHOR.exec(document.lineAt(line).text)
  const slug = match?.[1]
  if (slug === undefined) return

  const folder = vscode.workspace.getWorkspaceFolder(document.uri)
  const connection = folder === undefined
    ? undefined
    : mcpConnections.find(({ workspace }) => workspace.uri.toString() === folder.uri.toString())
  if (connection === undefined || connection.client.state !== 'ready') return

  const promptKey = `${document.uri.toString()}\u0000${line}\u0000${slug}`
  if (promptedAnchors.has(promptKey)) return
  promptedAnchors.add(promptKey)

  try {
    const tree = await connection.client.getTree()
    if (treeContainsSlug(tree.root, slug)) return
  } catch (error: unknown) {
    promptedAnchors.delete(promptKey)
    log(`Could not check intent anchor ${slug}: ${error instanceof Error ? error.message : String(error)}`)
    return
  }

  const choice = await vscode.window.showInformationMessage(
    `Create a Kron intent for “${slug}”?`,
    'Create intent',
    'Not now',
  )
  if (choice !== 'Create intent') return

  try {
    await connection.client.callTool('kron_add', { slug })
    try {
      await refreshIntents(false)
    } catch (error: unknown) {
      log(`Intent ${slug} was created, but the tree could not refresh: ${error instanceof Error ? error.message : String(error)}`)
    }
    void vscode.window.showInformationMessage(`Created intent “${slug}”. Its title and required metadata were filled automatically.`)
  } catch (error: unknown) {
    log(`Quick intent creation failed for ${slug}: ${error instanceof Error ? error.message : String(error)}`)
    void vscode.window.showErrorMessage(`Could not create intent “${slug}”. See the Kron output channel for details.`)
  }
}

async function restartConnections(): Promise<void> {
  if (restartPromise !== undefined) return restartPromise
  if (mcpConnections.length === 0) {
    void vscode.window.showWarningMessage('Open a workspace folder before restarting Kron connections.')
    return
  }
  restartPromise = (async () => {
    await Promise.allSettled([
      ...mcpConnections.map(({ client }) => client.stop()),
      ...lspConnections.map((client) => client.stop()),
    ])
    await startConnections()
    const failures: string[] = []
    if (lspConnections.some((client) => client.state !== 'ready')) failures.push('LSP hover/definition')
    try {
      await refreshIntents(false)
    } catch {
      if (!failures.includes('MCP tree')) failures.push('MCP tree')
    }
    if (mcpConnections.some(({ client }) => client.state !== 'ready') && !failures.includes('MCP tree')) {
      failures.push('MCP tree')
    }
    if (failures.length > 0) {
      void vscode.window.showWarningMessage(`Kron ${failures.join(' and ')} could not start. Check kron.serverPath and the Kron output channel.`)
    } else {
      void vscode.window.showInformationMessage('Kron connections restarted.')
    }
  })().finally(() => {
    restartPromise = undefined
  })
  return restartPromise
}

/** Starts independent MCP and LSP clients for the workspaces present at activation. */
export async function activate(context: vscode.ExtensionContext): Promise<void> {
  output = vscode.window.createOutputChannel('Kron')
  statusBar = new KronStatusBar()
  anchorDecoration = vscode.window.createTextEditorDecorationType({
    color: new vscode.ThemeColor('textLink.foreground'),
    textDecoration: 'underline',
  })
  context.subscriptions.push(output, statusBar, anchorDecoration)
  for (const editor of vscode.window.visibleTextEditors) scheduleIntentAnchorDecorations(editor.document)

  const workspaces = vscode.workspace.workspaceFolders ?? []
  const serverPath = vscode.workspace.getConfiguration('kron').get<string>('serverPath', 'kron').trim() || 'kron'
  const configuredLanguages: unknown = vscode.workspace.getConfiguration('kron').get('lspLanguages', DEFAULT_LSP_LANGUAGES)
  const parsedLanguages = languageListSchema.safeParse(configuredLanguages)
  const languages = parsedLanguages.success ? parsedLanguages.data : [...DEFAULT_LSP_LANGUAGES]
  if (!parsedLanguages.success) log('Invalid kron.lspLanguages setting; using the documented default language list.')

  mcpConnections = workspaces.map((workspace) => ({
    workspace,
    client: new KronMcpClient(
      serverPath,
      workspace.uri.fsPath,
      log,
      (_state: ConnectionState) => updateMcpStates(),
    ),
  }))
  lspConnections = workspaces.map((workspace) => new KronLspClient(
    serverPath,
    workspace,
    languages,
    log,
    (_state: ConnectionState) => updateLspStates(),
  ))

  treeProvider = new IntentTreeProvider(mcpConnections)
  context.subscriptions.push(
    vscode.window.createTreeView('kron.intents', { treeDataProvider: treeProvider }),
    treeProvider,
    vscode.commands.registerCommand('kron.refreshIntents', async () => {
      try {
        await refreshIntents(true)
      } catch {
        // The user-facing error was shown by refreshIntents.
      }
    }),
    vscode.commands.registerCommand('kron.openIntentWorkspace', () => {
      if (workspaces.length === 0 || treeProvider === undefined || output === undefined) {
        void vscode.window.showWarningMessage('Open a workspace folder before opening the Kron intent workspace.')
        return
      }
      openIntentWorkspace(context, treeProvider, output)
    }),
    vscode.commands.registerCommand('kron.restartConnections', restartConnections),
    vscode.workspace.onDidChangeTextDocument((event) => {
      if (event.document.uri.scheme !== 'file') return
      scheduleIntentAnchorDecorations(event.document)
      const changedLines = new Set<number>()
      for (const change of event.contentChanges) {
        changedLines.add(change.range.start.line)
        changedLines.add(Math.min(change.range.end.line, event.document.lineCount - 1))
      }
      for (const line of changedLines) scheduleQuickIntentPrompt(event.document, line)
    }),
    vscode.workspace.onDidOpenTextDocument((document) => scheduleIntentAnchorDecorations(document)),
    vscode.window.onDidChangeActiveTextEditor((editor) => {
      if (editor !== undefined) scheduleIntentAnchorDecorations(editor.document)
    }),
    vscode.workspace.onDidCloseTextDocument((document) => {
      const prefix = `${document.uri.toString()}\u0000`
      for (const [key, timer] of anchorCreateTimers) {
        if (key.startsWith(prefix)) {
          clearTimeout(timer)
          anchorCreateTimers.delete(key)
        }
      }
      const decorationTimer = anchorDecorationTimers.get(document.uri.toString())
      if (decorationTimer !== undefined) {
        clearTimeout(decorationTimer)
        anchorDecorationTimers.delete(document.uri.toString())
      }
    }),
  )

  if (workspaces.length === 0) {
    statusBar.setNoWorkspace()
    return
  }

  updateMcpStates()
  updateLspStates()

  void startMcpConnections().then(async () => {
    try {
      await refreshIntents(false)
    } catch {
      void vscode.window.showWarningMessage('Kron could not load the intent tree. Check kron.serverPath and the Kron output channel.')
      return
    }
    if (mcpConnections.some(({ client }) => client.state !== 'ready')) {
      void vscode.window.showWarningMessage('Some Kron workspace trees are unavailable. Check kron.serverPath and the Kron output channel.')
    }
  })
  void startLspConnections().then(() => {
    if (lspConnections.some((client) => client.state !== 'ready')) {
      void vscode.window.showWarningMessage('Kron hover and definition are unavailable. Check kron.serverPath and the Kron output channel.')
    }
  })
}

/** Stops each protocol process independently during Extension Host shutdown. */
export async function deactivate(): Promise<void> {
  for (const timer of anchorCreateTimers.values()) clearTimeout(timer)
  for (const timer of anchorDecorationTimers.values()) clearTimeout(timer)
  anchorCreateTimers.clear()
  anchorDecorationTimers.clear()
  anchorDecoration = undefined
  const shutdowns = [
    ...mcpConnections.map(({ client }) => client.stop()),
    ...lspConnections.map((client) => client.stop()),
  ]
  await Promise.allSettled(shutdowns)
  treeProvider = undefined
  mcpConnections = []
  lspConnections = []
  statusBar = undefined
  output = undefined
}
