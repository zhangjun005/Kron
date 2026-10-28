import vscode = require('vscode')
import type { KronMcpClient, McpToolName, TreeNode, TreeResult } from '../mcp/client'

export type WorkspaceMcpConnection = {
  workspace: vscode.WorkspaceFolder
  client: KronMcpClient
}

class IntentItem extends vscode.TreeItem {
  constructor(readonly node: TreeNode, id: string) {
    super(node.title || node.name, node.is_dir
      ? vscode.TreeItemCollapsibleState.Collapsed
      : vscode.TreeItemCollapsibleState.None)
    this.id = id
    this.contextValue = node.is_dir ? 'kron.intentFolder' : 'kron.intent'
    if (node.status !== undefined) this.description = node.status
    this.iconPath = new vscode.ThemeIcon(node.is_dir ? 'folder' : 'symbol-misc')
  }
}

/** Native Explorer tree backed by kron_tree over each workspace's MCP process. */
export class IntentTreeProvider implements vscode.TreeDataProvider<IntentItem>, vscode.Disposable {
  private readonly changed = new vscode.EventEmitter<IntentItem | undefined | null | void>()
  readonly onDidChangeTreeData = this.changed.event
  private root: TreeNode | undefined

  private connections: readonly WorkspaceMcpConnection[]

  constructor(connections: readonly WorkspaceMcpConnection[]) {
    this.connections = connections
  }

  setConnections(connections: readonly WorkspaceMcpConnection[]): void {
    this.connections = connections
    this.root = undefined
    this.changed.fire()
  }

  async refresh(): Promise<TreeResult> {
    if (this.connections.length === 0) throw new Error('Open a workspace folder to load Kron intents.')
    const results = await Promise.allSettled(this.connections.map(({ client }) => client.getTree()))
    const roots = results.flatMap((result, index) => {
      if (result.status !== 'fulfilled') return []
      const connection = this.connections[index]
      return connection === undefined ? [] : [{ connection, root: result.value.root }]
    })
    if (roots.length === 0) throw new Error('No workspace returned a Kron intent tree.')
    this.root = this.connections.length === 1
      ? this.withWorkspace(roots[0]!.root, roots[0]!.connection.workspace.uri.toString())
      : {
      slug: 'workspace-roots',
      name: 'Workspaces',
      is_dir: true,
      children: roots.map(({ connection, root }) => ({
          slug: connection.workspace.uri.toString(),
          name: connection.workspace.name,
          title: connection.workspace.name,
          is_dir: true,
          workspaceId: connection.workspace.uri.toString(),
          children: (root.children ?? []).map((child) => this.withWorkspace(child, connection.workspace.uri.toString())),
      })),
    }
    if (this.root === undefined) throw new Error('The Kron intent tree is unavailable.')
    this.changed.fire()
    return { root: this.root }
  }

  callTool(workspaceId: string | undefined, tool: McpToolName, args: Record<string, unknown>): Promise<unknown> {
    const connection = workspaceId === undefined && this.connections.length === 1
      ? this.connections[0]
      : this.connections.find(({ workspace }) => workspace.uri.toString() === workspaceId)
    if (connection === undefined) {
      throw new Error('The selected workspace is not connected to Kron.')
    }
    return connection.client.callTool(tool, args)
  }

  getTreeItem(item: IntentItem): vscode.TreeItem {
    return item
  }

  getChildren(item?: IntentItem): IntentItem[] {
    const node = item?.node ?? this.root
    const parentId = item?.id ?? 'kron-root'
    return (node?.children ?? []).map((child) => new IntentItem(child, `${parentId}/${child.slug}`))
  }

  dispose(): void {
    this.changed.dispose()
  }

  private withWorkspace(node: TreeNode, workspaceId: string): TreeNode {
    return {
      ...node,
      workspaceId,
      ...(node.children === undefined ? {} : { children: node.children.map((child) => this.withWorkspace(child, workspaceId)) }),
    }
  }
}
