import vscode = require('vscode')
import type { TreeNode } from '../mcp/client'
import type { WorkspaceMcpConnection } from './intentTree'

type AssumptionSummary = {
  id: string
  text: string
  default_severity: string
  status?: string
  expires_at?: string
  verified_at?: string
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function parseAssumptions(value: unknown): AssumptionSummary[] {
  if (!isRecord(value) || !Array.isArray(value.assumptions)) return []
  return value.assumptions.flatMap((item): AssumptionSummary[] => {
    if (!isRecord(item) || typeof item.id !== 'string') return []
    return [{
      id: item.id,
      text: typeof item.text === 'string' ? item.text : '',
      default_severity: typeof item.default_severity === 'string' ? item.default_severity : 'hard',
      ...(typeof item.status === 'string' ? { status: item.status } : {}),
      ...(typeof item.expires_at === 'string' ? { expires_at: item.expires_at } : {}),
      ...(typeof item.verified_at === 'string' ? { verified_at: item.verified_at } : {}),
    }]
  })
}

/** Read-only assumption registry section of the dedicated Kron sidebar. */
export class AssumptionTreeProvider implements vscode.TreeDataProvider<vscode.TreeItem>, vscode.Disposable {
  private readonly changed = new vscode.EventEmitter<void>()
  readonly onDidChangeTreeData = this.changed.event
  private assumptions: AssumptionSummary[] = []

  constructor(
    private readonly getConnections: () => readonly WorkspaceMcpConnection[],
    private readonly log: (message: string) => void,
  ) {}

  async refresh(): Promise<void> {
    const connection = this.getConnections()[0]
    if (connection === undefined || connection.client.state !== 'ready') {
      this.assumptions = []
      this.changed.fire()
      return
    }
    try {
      this.assumptions = parseAssumptions(await connection.client.callTool('kron_assume_list', {}))
    } catch (error: unknown) {
      this.log(`Assumption list refresh failed: ${error instanceof Error ? error.message : String(error)}`)
      this.assumptions = []
    }
    this.changed.fire()
  }

  getTreeItem(item: vscode.TreeItem): vscode.TreeItem {
    return item
  }

  getChildren(): vscode.TreeItem[] {
    return this.assumptions.map((assumption) => {
      const item = new vscode.TreeItem(assumption.id, vscode.TreeItemCollapsibleState.None)
      const hard = assumption.default_severity !== 'soft'
      item.description = [assumption.default_severity, assumption.expires_at === undefined ? '' : `expires ${assumption.expires_at}`]
        .filter((part) => part !== '').join(' · ')
      item.iconPath = new vscode.ThemeIcon('symbol-constant', new vscode.ThemeColor(hard ? 'charts.red' : 'charts.blue'))
      item.command = { command: 'kron.openAssumption', title: 'Open assumption file', arguments: [assumption.id] }
      const tooltip = new vscode.MarkdownString()
      tooltip.appendMarkdown(`**${assumption.id}** (${assumption.default_severity})\n\n${assumption.text}\n`)
      if (assumption.expires_at !== undefined) tooltip.appendMarkdown(`\nexpires: ${assumption.expires_at}`)
      item.tooltip = tooltip
      return item
    })
  }

  dispose(): void {
    this.changed.dispose()
  }
}

/** Superseded intents section (GUI "Archived" equivalent; Kron has no trash listing tool). */
export class SupersededTreeProvider implements vscode.TreeDataProvider<vscode.TreeItem>, vscode.Disposable {
  private readonly changed = new vscode.EventEmitter<void>()
  readonly onDidChangeTreeData = this.changed.event
  private items: TreeNode[] = []

  setTree(root: TreeNode): void {
    const found: TreeNode[] = []
    const walk = (node: TreeNode): void => {
      if (node.status === 'superseded') found.push(node)
      for (const child of node.children ?? []) walk(child)
    }
    walk(root)
    found.sort((left, right) => left.slug.localeCompare(right.slug))
    this.items = found
    this.changed.fire()
  }

  getTreeItem(item: vscode.TreeItem): vscode.TreeItem {
    return item
  }

  getChildren(): vscode.TreeItem[] {
    return this.items.map((node) => {
      const item = new vscode.TreeItem(node.title || node.name, vscode.TreeItemCollapsibleState.None)
      item.description = 'superseded'
      item.iconPath = new vscode.ThemeIcon('history', new vscode.ThemeColor('descriptionForeground'))
      item.tooltip = new vscode.MarkdownString(`**${node.slug}**\n\nsuperseded — open the intent card to review it.`)
      item.command = { command: 'kron.openIntentCard', title: 'Open intent card', arguments: [node.slug] }
      return item
    })
  }

  dispose(): void {
    this.changed.dispose()
  }
}
