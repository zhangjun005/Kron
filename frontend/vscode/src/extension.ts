import fs = require('node:fs')
import path = require('node:path')
import vscode = require('vscode')
import { findAnchors, type AnchorHit } from './anchors'
import { INTENT_BODY_TEMPLATE_REL_PATH, ANCHOR_SCAN_REL_PATH, commentSyntaxFor, isAnchorScanAllowed, loadAnchorScanConfig, lspLanguages, type AnchorScanConfig } from './anchorConfig'
import { KronLspClient } from './lsp/client'
import { KronMcpClient, type ConnectionState, type TreeNode } from './mcp/client'
import { AnchorMenuController } from './views/anchorMenu'
import { IntentItem, IntentTreeProvider, type WorkspaceMcpConnection } from './views/intentTree'
import { openIntentAt, openIntentWorkspace, postWebviewFontSize } from './views/intentEditor'
import { AssumptionTreeProvider, SupersededTreeProvider } from './views/sidebarSections'
import { KronStatusBar } from './views/statusBar'

/** Canonical project marker: every workspace folder has one `.kron/` root. */
const KRON_DIR = '.kron'

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

let output: vscode.OutputChannel | undefined
let statusBar: KronStatusBar | undefined
let mcpConnections: WorkspaceMcpConnection[] = []
let lspConnections: KronLspClient[] = []
let treeProvider: IntentTreeProvider | undefined
let explorerTreeView: vscode.TreeView<IntentItem> | undefined
let sidebarTreeView: vscode.TreeView<IntentItem> | undefined
let assumptionProvider: AssumptionTreeProvider | undefined
let supersededProvider: SupersededTreeProvider | undefined
let anchorScan: AnchorScanConfig | undefined
let projectFolderPath: string | undefined
let anchorMenu: AnchorMenuController | undefined
let restartPromise: Promise<void> | undefined
const anchorCreateTimers = new Map<string, NodeJS.Timeout>()
const anchorDecorationTimers = new Map<string, NodeJS.Timeout>()
const promptedAnchors = new Set<string>()
let anchorDecoration: vscode.TextEditorDecorationType | undefined

function log(message: string): void {
  output?.appendLine(message)
}

function readSettings(): { serverPath: string } {
  const configuredPath = vscode.workspace.getConfiguration('kron').get<string>('serverPath', 'kron').trim()
  return { serverPath: configuredPath || 'kron' }
}

/**
 * Single-root policy: one window serves the first workspace folder that has a
 * `.kron/` directory. Additional folders are ignored on purpose — the IDE is a
 * single-project client; multi-project overview belongs to the GUI.
 */
function findKronProjectFolder(): vscode.WorkspaceFolder | undefined {
  return (vscode.workspace.workspaceFolders ?? []).find((folder) => {
    try {
      return fs.statSync(path.join(folder.uri.fsPath, KRON_DIR)).isDirectory()
    } catch {
      return false
    }
  })
}

function updateMcpStates(): void {
  statusBar?.setMcpStates(mcpConnections.map(({ client }) => client.state))
}

function updateLspStates(): void {
  statusBar?.setLspStates(lspConnections.map((client) => client.state))
}

async function startConnections(): Promise<void> {
  const results = await Promise.allSettled([
    ...mcpConnections.map(({ client }) => client.start()),
    ...lspConnections.map((client) => client.start()),
  ])
  for (const result of results) {
    if (result.status === 'rejected') log(`Kron connection startup failed: ${result.reason instanceof Error ? result.reason.message : String(result.reason)}`)
  }
}

async function refreshIntents(showError: boolean): Promise<void> {
  if (treeProvider === undefined || mcpConnections.length === 0) return
  try {
    const result = await treeProvider.refresh()
    updateTreeViewDescriptions(result.root)
    supersededProvider?.setTree(result.root)
    void assumptionProvider?.refresh()
    anchorMenu?.notifyIntentsChanged()
  } catch (error: unknown) {
    log(`Intent tree refresh failed: ${error instanceof Error ? error.message : String(error)}`)
    if (showError) {
      void vscode.window.showErrorMessage('Kron could not load the intent tree. Check kron.serverPath and the Kron output channel.')
    }
    throw error
  }
}

type TreeLikeNode = { slug: string; children?: TreeLikeNode[] | undefined }

/** Shows intent/folder counts in both sidebar views' titles. */
function updateTreeViewDescriptions(root: TreeNode): void {
  let leaves = 0
  let groups = 0
  const walk = (node: TreeNode): void => {
    if (node.is_dir) groups += 1
    else leaves += 1
    for (const child of node.children ?? []) walk(child)
  }
  walk(root)
  const description = `${leaves} intents · ${groups} folders`
  if (explorerTreeView !== undefined) explorerTreeView.description = description
  if (sidebarTreeView !== undefined) sidebarTreeView.description = description
}

function treeContainsSlug(node: TreeLikeNode, slug: string): boolean {
  if (node.slug === slug) return true
  return (node.children ?? []).some((child) => treeContainsSlug(child, slug))
}

function isKronInternalDocument(document: vscode.TextDocument): boolean {
  if (document.uri.scheme !== 'file') return true
  return document.uri.fsPath.split(/[\\/]/).includes(KRON_DIR)
}

/** Anchors parsed with the same grammar as `internal/parser` (see src/anchors.ts). */
function anchorsInDocument(document: vscode.TextDocument): AnchorHit[] {
  if (isKronInternalDocument(document)) return []
  if (projectFolderPath !== undefined
    && !isAnchorScanAllowed(projectFolderPath, document.uri.fsPath, document.languageId, anchorScan)) return []
  return findAnchors(document.getText(), document.languageId === 'markdown')
}

function updateIntentAnchorDecorations(document: vscode.TextDocument): void {
  if (anchorDecoration === undefined) return
  const ranges = anchorsInDocument(document).map((anchor) =>
    new vscode.Range(anchor.line, anchor.character, anchor.line, anchor.character + anchor.slug.length))
  for (const editor of vscode.window.visibleTextEditors) {
    if (editor.document.uri.toString() === document.uri.toString()) editor.setDecorations(anchorDecoration, ranges)
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

function scheduleQuickIntentPrompt(document: vscode.TextDocument, line: number): void {
  const key = `${document.uri.toString()}\u0000${line}`
  const previous = anchorCreateTimers.get(key)
  if (previous !== undefined) clearTimeout(previous)
  const timer = setTimeout(() => {
    anchorCreateTimers.delete(key)
    void maybeOfferQuickIntent(document, line)
  }, 900)
  anchorCreateTimers.set(key, timer)
}

async function maybeOfferQuickIntent(document: vscode.TextDocument, line: number): Promise<void> {
  // Markdown anchors are documentation links: underline + go-to-definition only,
  // never an auto-create prompt (docs would spam it).
  if (document.languageId === 'markdown') return
  if (projectFolderPath !== undefined
    && !isAnchorScanAllowed(projectFolderPath, document.uri.fsPath, document.languageId, anchorScan)) return
  const anchor = anchorsInDocument(document).find((hit) => hit.line === line)
  if (anchor === undefined) return
  const slug = anchor.slug

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

/**
 * Rebuilds the single Kron project connection from the current settings.
 * Settings are re-read here (not captured at activation) so changing
 * `kron.serverPath` / `kron.lspLanguages` and running "Restart Connections"
 * takes effect without reloading the window.
 */
async function rebuildConnections(): Promise<void> {
  await Promise.allSettled([
    ...mcpConnections.map(({ client }) => client.stop()),
    ...lspConnections.map((client) => client.stop()),
  ])

  const settings = readSettings()
  const folder = findKronProjectFolder()
  projectFolderPath = folder?.uri.fsPath
  anchorScan = folder === undefined ? undefined : loadAnchorScanConfig(folder.uri.fsPath)
  void vscode.commands.executeCommand('setContext', 'kronHasProject', folder !== undefined)
  mcpConnections = folder === undefined
    ? []
    : [{
        workspace: folder,
        client: new KronMcpClient(settings.serverPath, folder.uri.fsPath, log, (_state: ConnectionState) => updateMcpStates()),
      }]
  lspConnections = folder === undefined
    ? []
    : [new KronLspClient(settings.serverPath, folder, lspLanguages(anchorScan), log, (_state: ConnectionState) => updateLspStates())]
  treeProvider?.setConnections(mcpConnections)
  updateMcpStates()
  updateLspStates()

  if (folder === undefined) {
    statusBar?.setNoProject()
    return
  }

  statusBar?.setProject(folder.name)
  await startConnections()
  try {
    await refreshIntents(false)
  } catch (error: unknown) {
    log(`Workspace tree refresh failed: ${error instanceof Error ? error.message : String(error)}`)
  }

  const failures: string[] = []
  if (mcpConnections.some(({ client }) => client.state !== 'ready')) failures.push('MCP tree')
  if (lspConnections.some((client) => client.state !== 'ready')) failures.push('LSP hover/definition')
  if (failures.length > 0) {
    void vscode.window.showWarningMessage(
      `Kron ${failures.join(' and ')} did not start for “${folder.name}”. Check kron.serverPath and the Kron output channel.`,
    )
  }
}

async function restartConnections(): Promise<void> {
  if (restartPromise !== undefined) return restartPromise
  restartPromise = rebuildConnections()
    .then(() => {
      if (mcpConnections.length === 0) {
        void vscode.window.showWarningMessage('No workspace folder with a .kron directory is open.')
        return
      }
      void vscode.window.showInformationMessage('Kron connections restarted.')
    })
    .catch((error: unknown) => {
      log(`Kron restart failed: ${error instanceof Error ? error.message : String(error)}`)
      void vscode.window.showErrorMessage('Kron could not restart its connections. Check the Kron output channel.')
    })
    .finally(() => {
      restartPromise = undefined
    })
  return restartPromise
}

/** Starts the single Kron project connection for the active window. */
export async function activate(context: vscode.ExtensionContext): Promise<void> {
  output = vscode.window.createOutputChannel('Kron')
  statusBar = new KronStatusBar()
  anchorDecoration = vscode.window.createTextEditorDecorationType({
    color: new vscode.ThemeColor('textLink.foreground'),
    textDecoration: 'underline',
  })
  context.subscriptions.push(output, statusBar, anchorDecoration)
  for (const editor of vscode.window.visibleTextEditors) scheduleIntentAnchorDecorations(editor.document)

  treeProvider = new IntentTreeProvider(mcpConnections)
  assumptionProvider = new AssumptionTreeProvider(() => mcpConnections, log)
  supersededProvider = new SupersededTreeProvider()

  const collectIntentSlugs = async (): Promise<string[]> => {
    if (treeProvider === undefined) return []
    const result = await treeProvider.refresh()
    const slugs = new Set<string>()
    const walk = (node: TreeNode): void => {
      if (!node.is_dir || node.title !== undefined || node.status !== undefined) slugs.add(node.slug)
      for (const child of node.children ?? []) walk(child)
    }
    walk(result.root)
    return [...slugs].sort()
  }

  anchorMenu = new AnchorMenuController({
    getClient: () => mcpConnections[0]?.client,
    getIntentSlugs: collectIntentSlugs,
    openIntentWorkspace: (slug, edit) => {
      const provider = treeProvider
      const channel = output
      if (provider === undefined || channel === undefined) return
      void openIntentAt(context, provider, channel, slug, edit)
    },
    refreshTree: () => refreshIntents(true),
    getProjectFolderPath: () => projectFolderPath,
    getAnchorScanConfig: () => anchorScan,
    log,
    showOutput: () => output?.show(true),
  })
  context.subscriptions.push(anchorMenu)

  explorerTreeView = vscode.window.createTreeView('kron.intents', { treeDataProvider: treeProvider, showCollapseAll: true })
  sidebarTreeView = vscode.window.createTreeView('kron.intentsSidebar', { treeDataProvider: treeProvider, showCollapseAll: true })
  const assumptionView = vscode.window.createTreeView('kron.assumptions', { treeDataProvider: assumptionProvider })
  const supersededView = vscode.window.createTreeView('kron.superseded', { treeDataProvider: supersededProvider })
  context.subscriptions.push(
    explorerTreeView,
    sidebarTreeView,
    assumptionView,
    supersededView,
    treeProvider,
    vscode.commands.registerCommand('kron.openIntentCard', (slug: unknown) => {
      if (typeof slug !== 'string' || slug === '' || treeProvider === undefined || output === undefined) return
      void openIntentAt(context, treeProvider, output, slug, false)
    }),
    vscode.commands.registerCommand('kron.openAssumption', async (id: unknown) => {
      if (typeof id !== 'string' || id === '') return
      const client = mcpConnections[0]?.client
      if (client === undefined || client.state !== 'ready') {
        void vscode.window.showWarningMessage('Kron is not connected.')
        return
      }
      try {
        const result = await client.callTool('kron_assume_get', { id })
        const sourcePath = isRecord(result) && isRecord(result.assumption)
          && typeof result.assumption.source_path === 'string' ? result.assumption.source_path : undefined
        if (sourcePath === undefined) {
          void vscode.window.showErrorMessage(`Kron did not return a file for assumption "${id}".`)
          return
        }
        await vscode.window.showTextDocument(vscode.Uri.file(sourcePath))
      } catch (error: unknown) {
        log(`Open assumption failed: ${error instanceof Error ? error.message : String(error)}`)
        void vscode.window.showErrorMessage(`Kron could not open assumption "${id}". See the Kron output channel for details.`)
      }
    }),
    vscode.commands.registerCommand('kron.refreshIntents', async () => {
      try {
        await refreshIntents(true)
      } catch {
        // The user-facing error was shown by refreshIntents.
      }
    }),
    vscode.commands.registerCommand('kron.openIntentWorkspace', () => {
      if (treeProvider === undefined || output === undefined) return
      if (mcpConnections.length === 0) {
        void vscode.window.showWarningMessage('Open a workspace folder that contains a .kron directory before opening the Kron intent workspace.')
        return
      }
      openIntentWorkspace(context, treeProvider, output)
    }),
    vscode.commands.registerCommand('kron.restartConnections', restartConnections),
    vscode.workspace.onDidChangeWorkspaceFolders(() => {
      void rebuildConnections()
    }),
    vscode.workspace.onDidSaveTextDocument((document) => {
      if (projectFolderPath === undefined) return
      const rel = path.relative(projectFolderPath, document.uri.fsPath).split(path.sep).join('/')
      if (rel === ANCHOR_SCAN_REL_PATH || rel === INTENT_BODY_TEMPLATE_REL_PATH) {
        log(`Project Kron config changed (${rel}); restarting connections.`)
        void rebuildConnections()
      }
    }),
    vscode.workspace.onDidChangeConfiguration((event) => {
      if (event.affectsConfiguration('kron.serverPath') || event.affectsConfiguration('kron.lspLanguages')) {
        void rebuildConnections()
      }
      if (event.affectsConfiguration('kron.webviewFontSize')) {
        postWebviewFontSize()
      }
    }),
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

  void rebuildConnections()
}

/** Stops the protocol process during Extension Host shutdown. */
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
  assumptionProvider = undefined
  supersededProvider = undefined
  anchorMenu = undefined
  mcpConnections = []
  lspConnections = []
  statusBar = undefined
  output = undefined
}
