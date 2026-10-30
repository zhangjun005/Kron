import path = require('node:path')
import vscode = require('vscode')
import type { ImpactSummary, KronMcpClient } from '../mcp/client'

export type CodeAnchorJumpDependencies = {
  getClient: () => KronMcpClient | undefined
  getProjectFolderPath: () => string | undefined
  log: (message: string) => void
}

type AnchorRow = { filePath: string; line: number; kind: 'code' | 'markdown' }

function toRows(impact: ImpactSummary, includeMarkdown: boolean): AnchorRow[] {
  const rows: AnchorRow[] = (impact.incoming_anchors_code ?? []).map((anchor) => ({
    filePath: anchor.file_path,
    line: anchor.line,
    kind: 'code' as const,
  }))
  if (includeMarkdown) {
    for (const anchor of impact.incoming_anchors_markdown ?? []) {
      rows.push({ filePath: anchor.file_path, line: anchor.line, kind: 'markdown' })
    }
  }
  rows.sort((left, right) => left.filePath === right.filePath
    ? left.line - right.line
    : left.filePath.localeCompare(right.filePath))
  return rows
}

async function openAnchor(projectFolder: string, row: AnchorRow): Promise<void> {
  const target = path.join(projectFolder, ...row.filePath.split('/'))
  const uri = vscode.Uri.file(target)
  const position = new vscode.Position(Math.max(0, row.line - 1), 0)
  const document = await vscode.workspace.openTextDocument(uri)
  const editor = await vscode.window.showTextDocument(document, {
    preview: false,
    selection: new vscode.Range(position, position),
  })
  editor.revealRange(new vscode.Range(position, position), vscode.TextEditorRevealType.InCenterIfOutsideViewport)
}

/**
 * Jump from an intent to its code anchors (reverse of the @kron:intent
 * definition jump). One intent may have N anchors; a single QuickPick selects
 * the target. No caching: kron_impact is called on demand and the intent graph
 * cache inside serve-mcp serves that call.
 */
export async function goToCodeAnchors(
  deps: CodeAnchorJumpDependencies,
  slug: string,
): Promise<void> {
  const client = deps.getClient()
  if (client === undefined || client.state !== 'ready') {
    void vscode.window.showWarningMessage('Kron is not connected. Check the Kron output channel.')
    return
  }
  const projectFolder = deps.getProjectFolderPath()
  if (projectFolder === undefined) {
    void vscode.window.showWarningMessage('Open a workspace folder containing .kron/ first.')
    return
  }
  const includeMarkdown = vscode.workspace.getConfiguration('kron').get<boolean>('anchors.includeMarkdown', false)

  let impact: ImpactSummary
  try {
    impact = await client.getImpact(slug)
  } catch (error: unknown) {
    deps.log(`Code anchor jump failed for ${slug}: ${error instanceof Error ? error.message : String(error)}`)
    void vscode.window.showErrorMessage(`Kron could not load anchors for “${slug}”. See the output channel for details.`)
    return
  }
  const rows = toRows(impact, includeMarkdown)
  if (rows.length === 0) {
    const markdownCount = includeMarkdown ? 0 : (impact.incoming_anchors_markdown ?? []).length
    if (markdownCount > 0) {
      void vscode.window.showInformationMessage(
        `“${slug}” only has ${markdownCount} markdown anchor(s). Enable “Kron: Anchors Include Markdown” to include them.`)
    } else {
      void vscode.window.showInformationMessage(`“${slug}” has no code anchors yet. Add @kron:intent ${slug} above the code it owns.`)
    }
    return
  }
  if (rows.length === 1) {
    const only = rows[0]!
    await openAnchor(projectFolder, only)
    return
  }
  const picked = await vscode.window.showQuickPick(rows.map((row) => ({
    label: `${row.filePath}:${row.line}`,
    description: row.kind,
    row,
  })), {
    title: `Code anchors for “${slug}”`,
    placeHolder: 'Select the code location to open',
    matchOnDescription: true,
  })
  if (picked === undefined) return
  await openAnchor(projectFolder, picked.row)
}
