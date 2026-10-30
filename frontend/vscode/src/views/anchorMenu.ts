import vscode = require('vscode')
import { findAnchors, isValidAnchorSlug, type AnchorHit } from '../anchors'
import { commentSyntaxFor, isAnchorScanAllowed, type AnchorScanComment, type AnchorScanConfig } from '../anchorConfig'
import type { ImpactSummary, KronMcpClient } from '../mcp/client'

/** Context-key state consumed by the `editor/context` menu `when` clauses. */
type AnchorState = 'none' | 'checking' | 'valid' | 'dangling' | 'unsupported'

type AnchorTarget = {
  readonly document: vscode.TextDocument
  readonly hit: AnchorHit
}

function anchorLineFor(
  syntax: AnchorScanComment | undefined,
  slug: string,
  useBlock: boolean,
): { text: string; known: boolean } {
  if (syntax === undefined) return { text: `// @kron:intent ${slug}`, known: false }
  const block = syntax.block
  if (useBlock && block !== undefined && block.length >= 2) {
    return { text: `${block[0]} @kron:intent ${slug} ${block[1]}`, known: true }
  }
  if (syntax.line !== undefined) return { text: `${syntax.line} @kron:intent ${slug}`, known: true }
  if (block !== undefined && block.length >= 2) {
    return { text: `${block[0]} @kron:intent ${slug} ${block[1]}`, known: true }
  }
  return { text: `// @kron:intent ${slug}`, known: false }
}

/** JSX children cannot take a // comment; use the configured block pair there. */
function shouldUseBlockComment(languageId: string, lineText: string, syntax: AnchorScanComment): boolean {
  if (syntax.block === undefined) return false
  if (languageId === 'typescriptreact' || languageId === 'javascriptreact') {
    return lineText.trimStart().startsWith('<')
  }
  return false
}

const EXISTENCE_CACHE_MS = 5_000
const CONTEXT_REFRESH_DELAY_MS = 200

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function isKronInternalPath(fsPath: string): boolean {
  return fsPath.split(/[\\/]/).includes('.kron')
}

export type AnchorMenuDependencies = {
  /** MCP client of the active Kron project, or undefined when not connected. */
  getClient: () => KronMcpClient | undefined
  /** Slugs available for "change target" / "attach existing" pickers. */
  getIntentSlugs: () => Promise<string[]>
  /** Opens (or focuses) the Kron workspace panel, optionally in edit mode. */
  openIntentWorkspace: (slug: string, edit: boolean) => void
  refreshTree: () => Promise<void>
  /** Project folder used to resolve .kron/anchor-scan.json. */
  getProjectFolderPath: () => string | undefined
  /** Parsed project anchor scan config (undefined = no filtering). */
  getAnchorScanConfig: () => AnchorScanConfig | undefined
  log: (message: string) => void
  showOutput: () => void
}

/**
 * Editor-side anchor menu: keeps the `kron.anchor*` context keys in sync with
 * the cursor and implements the right-click actions for valid / dangling /
 * missing anchors. All file edits go through `WorkspaceEdit`; nothing is
 * written just by opening the context menu.
 */
export class AnchorMenuController implements vscode.Disposable {
  private readonly disposables: vscode.Disposable[] = []
  private readonly existence = new Map<string, { exists: boolean; checkedAt: number }>()
  private target: AnchorTarget | undefined
  private refreshSequence = 0
  private refreshTimer: NodeJS.Timeout | undefined

  constructor(private readonly deps: AnchorMenuDependencies) {
    this.disposables.push(
      vscode.commands.registerCommand('kron.openIntentDetails', () => this.openDetails()),
      vscode.commands.registerCommand('kron.editIntentFromAnchor', () => this.openDetails(true)),
      vscode.commands.registerCommand('kron.openIntentMarkdown', () => this.openMarkdown()),
      vscode.commands.registerCommand('kron.showIntentImpact', () => this.showImpact()),
      vscode.commands.registerCommand('kron.changeAnchorTarget', () => this.changeAnchorTarget()),
      vscode.commands.registerCommand('kron.createIntentForAnchor', () => this.createIntentForAnchor()),
      vscode.commands.registerCommand('kron.createIntentAndAttachAnchor', () => this.createIntentAndAttachAnchor()),
      vscode.commands.registerCommand('kron.attachExistingIntentAnchor', () => this.attachExistingIntentAnchor()),
      vscode.window.onDidChangeActiveTextEditor(() => this.scheduleRefresh()),
      vscode.window.onDidChangeTextEditorSelection(() => this.scheduleRefresh()),
      vscode.workspace.onDidChangeTextDocument((event) => {
        if (event.document === vscode.window.activeTextEditor?.document) this.scheduleRefresh()
      }),
    )
    this.scheduleRefresh()
  }

  /** Tree changed (write / refresh): cached existence checks are stale. */
  notifyIntentsChanged(): void {
    this.existence.clear()
    this.scheduleRefresh()
  }

  dispose(): void {
    if (this.refreshTimer !== undefined) clearTimeout(this.refreshTimer)
    for (const disposable of this.disposables) disposable.dispose()
  }

  private scheduleRefresh(): void {
    if (this.refreshTimer !== undefined) clearTimeout(this.refreshTimer)
    this.refreshTimer = setTimeout(() => {
      this.refreshTimer = undefined
      void this.refreshContext()
    }, CONTEXT_REFRESH_DELAY_MS)
  }

  private async refreshContext(): Promise<void> {
    const sequence = ++this.refreshSequence
    const editor = vscode.window.activeTextEditor
    if (editor !== undefined && (editor.document.uri.scheme !== 'file' || isKronInternalPath(editor.document.uri.fsPath))) {
      this.target = undefined
      await this.setContext('unsupported', undefined)
      return
    }
    const target = this.locateTarget(editor)
    this.target = target
    const client = this.deps.getClient()
    if (target === undefined) {
      await this.setContext('none', undefined)
      return
    }
    const slug = target.hit.slug
    if (client === undefined || client.state !== 'ready') {
      await this.setContext('none', slug)
      return
    }
    const cached = this.existence.get(slug)
    const now = Date.now()
    if (cached !== undefined && now - cached.checkedAt < EXISTENCE_CACHE_MS) {
      await this.setContext(cached.exists ? 'valid' : 'dangling', slug)
      return
    }
    await this.setContext('checking', slug)
    let exists = false
    try {
      await client.callTool('kron_get', { slug })
      exists = true
    } catch {
      exists = false
    }
    if (sequence !== this.refreshSequence) return
    if (client.state !== 'ready') {
      await this.setContext('none', slug)
      return
    }
    this.existence.set(slug, { exists, checkedAt: Date.now() })
    await this.setContext(exists ? 'valid' : 'dangling', slug)
  }

  private async setContext(state: AnchorState, slug: string | undefined): Promise<void> {
    await vscode.commands.executeCommand('setContext', 'kronAnchorState', state)
    if (slug !== undefined) await vscode.commands.executeCommand('setContext', 'kronAnchorSlug', slug)
  }

  /** Anchor on the cursor line(s), or on the line directly above (anchor convention). */
  private locateTarget(editor: vscode.TextEditor | undefined): AnchorTarget | undefined {
    if (editor === undefined || editor.document.uri.scheme !== 'file') return undefined
    if (isKronInternalPath(editor.document.uri.fsPath)) return undefined
    const projectFolder = this.deps.getProjectFolderPath()
    if (projectFolder !== undefined
      && !isAnchorScanAllowed(projectFolder, editor.document.uri.fsPath, editor.document.languageId, this.deps.getAnchorScanConfig())) {
      return undefined
    }
    const anchors = findAnchors(editor.document.getText(), editor.document.languageId === 'markdown')
    if (anchors.length === 0) return undefined
    const candidates = new Set<number>()
    for (const selection of editor.selections) {
      candidates.add(selection.start.line)
      candidates.add(selection.end.line)
      candidates.add(selection.start.line - 1)
      candidates.add(selection.end.line - 1)
    }
    for (const line of candidates) {
      const hit = anchors.find((anchor) => anchor.line === line)
      if (hit !== undefined) return { document: editor.document, hit }
    }
    return undefined
  }

  private currentTarget(): AnchorTarget | undefined {
    const live = this.locateTarget(vscode.window.activeTextEditor)
    if (live !== undefined) this.target = live
    return live ?? this.target
  }

  private async requireReadyClient(): Promise<KronMcpClient | undefined> {
    const client = this.deps.getClient()
    if (client === undefined || client.state !== 'ready') {
      void vscode.window.showWarningMessage('Kron is not connected. Check kron.serverPath and the Kron output channel.')
      return undefined
    }
    return client
  }

  private async openDetails(edit = false): Promise<void> {
    const target = this.currentTarget()
    if (target === undefined) {
      void vscode.window.showInformationMessage('Place the cursor on a @kron:intent anchor (or the symbol line below it) first.')
      return
    }
    this.deps.openIntentWorkspace(target.hit.slug, edit)
  }

  private async openMarkdown(): Promise<void> {
    const target = this.currentTarget()
    if (target === undefined) return
    const client = await this.requireReadyClient()
    if (client === undefined) return
    try {
      const result = await client.callTool('kron_get', { slug: target.hit.slug })
      const sourcePath = isRecord(result) && isRecord(result.intent) && typeof result.intent.source_path === 'string'
        ? result.intent.source_path
        : undefined
      if (sourcePath === undefined) {
        void vscode.window.showErrorMessage(`Kron did not return a Markdown path for “${target.hit.slug}”.`)
        return
      }
      await vscode.window.showTextDocument(vscode.Uri.file(sourcePath))
    } catch (error: unknown) {
      this.handleError(`Could not open the Markdown for “${target.hit.slug}”.`, error)
    }
  }

  private async showImpact(): Promise<void> {
    const target = this.currentTarget()
    if (target === undefined) return
    const client = await this.requireReadyClient()
    if (client === undefined) return
    try {
      const impact: ImpactSummary = await client.getImpact(target.hit.slug)
      const code = impact.incoming_anchors_code?.length ?? 0
      const markdown = impact.incoming_anchors_markdown?.length ?? 0
      const references = impact.references ?? []
      const prerequisites = impact.prerequisites ?? []
      const details = [
        `intent: ${impact.intent.slug} (${impact.intent.status ?? 'unknown status'})`,
        `code anchors (${code}):`,
        ...(impact.incoming_anchors_code ?? []).map((anchor) => `  ${anchor.file_path}:${anchor.line}`),
        `markdown anchors (${markdown}):`,
        ...(impact.incoming_anchors_markdown ?? []).map((anchor) => `  ${anchor.file_path}:${anchor.line}`),
        `dependent intents (${references.length}): ${references.join(', ') || 'none'}`,
        `prerequisites (${prerequisites.length}): ${prerequisites.join(', ') || 'none'}`,
      ]
      for (const line of details) this.deps.log(line)
      const choice = await vscode.window.showInformationMessage(
        `Kron impact for “${impact.intent.slug}”: ${code} code anchor(s), ${markdown} markdown anchor(s), `
        + `${references.length} dependent intent(s), ${prerequisites.length} prerequisite(s).`,
        'Open Output',
      )
      if (choice === 'Open Output') this.deps.showOutput()
    } catch (error: unknown) {
      this.handleError(`Could not show impact for “${target.hit.slug}”.`, error)
    }
  }

  private async changeAnchorTarget(): Promise<void> {
    const target = this.currentTarget()
    if (target === undefined) return
    const slugs = (await this.deps.getIntentSlugs()).filter((slug) => slug !== target.hit.slug)
    if (slugs.length === 0) {
      void vscode.window.showInformationMessage('No other intents exist yet, so there is nothing to point this anchor at.')
      return
    }
    const picked = await vscode.window.showQuickPick(slugs, {
      title: 'Change the intent this anchor points to',
      placeHolder: target.hit.slug,
    })
    if (picked === undefined) return
    const line = target.hit.line
    const range = new vscode.Range(line, target.hit.character, line, target.hit.character + target.hit.slug.length)
    const edit = new vscode.WorkspaceEdit()
    edit.replace(target.document.uri, range, picked)
    const applied = await vscode.workspace.applyEdit(edit)
    if (!applied) {
      void vscode.window.showErrorMessage('Kron could not update the anchor. Edit the line manually.')
      return
    }
    this.existence.clear()
    this.scheduleRefresh()
  }

  private async createIntentForAnchor(): Promise<void> {
    const target = this.currentTarget()
    if (target === undefined) return
    const client = await this.requireReadyClient()
    if (client === undefined) return
    const slug = await vscode.window.showInputBox({
      prompt: 'Slug for the new intent',
      value: target.hit.slug,
      validateInput: (value) => isValidAnchorSlug(value.trim()) ? undefined : 'Use lowercase kebab-case segments, e.g. auth/session.',
    })
    if (slug === undefined) return
    const trimmed = slug.trim()
    try {
      await client.callTool('kron_add', { slug: trimmed })
    } catch (error: unknown) {
      this.handleError(`Could not create intent “${trimmed}”.`, error)
      return
    }
    this.existence.clear()
    try {
      await this.deps.refreshTree()
    } catch (error: unknown) {
      this.deps.log(`Intent ${trimmed} was created, but the tree could not refresh: ${error instanceof Error ? error.message : String(error)}`)
    }
    if (trimmed !== target.hit.slug) {
      // The user renamed the slug while creating: keep the anchor pointing at it.
      const line = target.hit.line
      const range = new vscode.Range(line, target.hit.character, line, target.hit.character + target.hit.slug.length)
      const edit = new vscode.WorkspaceEdit()
      edit.replace(target.document.uri, range, trimmed)
      const applied = await vscode.workspace.applyEdit(edit)
      if (!applied) {
        void vscode.window.showWarningMessage(`Created intent “${trimmed}”, but the anchor still points at “${target.hit.slug}”. Update the line manually.`)
        return
      }
    }
    void vscode.window.showInformationMessage(`Created intent “${trimmed}”. The anchor now points at it.`)
    this.scheduleRefresh()
  }

  private async createIntentAndAttachAnchor(): Promise<void> {
    const editor = vscode.window.activeTextEditor
    if (editor === undefined || editor.document.uri.scheme !== 'file' || isKronInternalPath(editor.document.uri.fsPath)) {
      void vscode.window.showWarningMessage('Open a source or Markdown file before creating an intent anchor.')
      return
    }
    const client = await this.requireReadyClient()
    if (client === undefined) return
    const selected = editor.document.getText(editor.selection).trim()
    const suggested = isValidAnchorSlug(selected) ? selected : ''
    const slug = await vscode.window.showInputBox({
      prompt: 'Slug for the new intent',
      value: suggested,
      validateInput: (value) => isValidAnchorSlug(value.trim()) ? undefined : 'Use lowercase kebab-case segments, e.g. auth/session.',
    })
    if (slug === undefined) return
    const trimmed = slug.trim()

    // 1) create first; a failed create must not touch the document
    try {
      await client.callTool('kron_add', { slug: trimmed })
    } catch (error: unknown) {
      this.handleError(`Could not create intent “${trimmed}”. Nothing was inserted into the file.`, error)
      return
    }
    this.existence.clear()
    try {
      await this.deps.refreshTree()
    } catch (error: unknown) {
      this.deps.log(`Intent ${trimmed} was created, but the tree could not refresh: ${error instanceof Error ? error.message : String(error)}`)
    }

    // 2) only after the intent exists, insert the anchor above the symbol line
    const inserted = await this.insertAnchor(trimmed, editor, editor.selection.start.line)
    if (!inserted) {
      void vscode.window.showWarningMessage(
        `Created intent “${trimmed}”, but the anchor could not be inserted. Add the @kron:intent ${trimmed} line above the symbol manually.`,
      )
      return
    }
    void vscode.window.showInformationMessage(`Created intent “${trimmed}” and attached the anchor.`)
    this.scheduleRefresh()
  }

  private async attachExistingIntentAnchor(): Promise<void> {
    const editor = vscode.window.activeTextEditor
    if (editor === undefined || editor.document.uri.scheme !== 'file' || isKronInternalPath(editor.document.uri.fsPath)) {
      void vscode.window.showWarningMessage('Open a source or Markdown file before attaching an intent anchor.')
      return
    }
    const slugs = await this.deps.getIntentSlugs()
    if (slugs.length === 0) {
      void vscode.window.showInformationMessage('No intents exist yet. Create one first.')
      return
    }
    const picked = await vscode.window.showQuickPick(slugs, { title: 'Attach an existing intent to this location' })
    if (picked === undefined) return
    const inserted = await this.insertAnchor(picked, editor, editor.selection.start.line)
    if (!inserted) return
    this.scheduleRefresh()
  }

  /** Inserts `comment @kron:intent <slug>` above `symbolLine`; returns success. */
  private async insertAnchor(slug: string, editor: vscode.TextEditor, symbolLine: number): Promise<boolean> {
    const document = editor.document
    const syntax = commentSyntaxFor(document.languageId, this.deps.getAnchorScanConfig())
    const symbolLineText = document.lineAt(symbolLine).text
    const useBlock = syntax === undefined ? false : shouldUseBlockComment(document.languageId, symbolLineText, syntax)
    const candidate = anchorLineFor(syntax, slug, useBlock)
    let anchorText = candidate.text
    if (!candidate.known) {
      // Unknown comment syntax: show an editable preview instead of guessing.
      const edited = await vscode.window.showInputBox({
        prompt: `Comment syntax for "${document.languageId}" is unknown — edit the anchor line before inserting`,
        value: candidate.text,
        validateInput: (value) => findAnchors(value, false).some((hit) => hit.slug === slug)
          ? undefined
          : 'The line must contain @kron:intent followed by the slug.',
      })
      if (edited === undefined) return false
      anchorText = edited.trim()
    }
    const indent = document.lineAt(symbolLine).text.match(/^[ \t]*/)?.[0] ?? ''
    const eol = document.eol === vscode.EndOfLine.CRLF ? '\r\n' : '\n'
    const edit = new vscode.WorkspaceEdit()
    edit.insert(document.uri, new vscode.Position(symbolLine, 0), `${indent}${anchorText}${eol}`)
    const applied = await vscode.workspace.applyEdit(edit)
    if (!applied) {
      void vscode.window.showErrorMessage('Kron could not insert the anchor. Add it manually above the symbol.')
      return false
    }
    return true
  }

  private handleError(prefix: string, error: unknown): void {
    const message = error instanceof Error ? error.message : String(error)
    this.deps.log(`${prefix} ${message}`)
    void vscode.window.showErrorMessage(prefix)
  }
}
