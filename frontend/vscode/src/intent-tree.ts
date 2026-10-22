import client = require('@modelcontextprotocol/sdk/client/index.js')
import stdio = require('@modelcontextprotocol/sdk/client/stdio.js')
import vscode = require('vscode')
import zod = require('zod')

const z = zod.z

const TreeNodeSchema: zod.ZodType<TreeNode> = z.lazy(() => z.object({
  slug: z.string(),
  name: z.string(),
  is_dir: z.boolean(),
  title: z.string().optional(),
  status: z.string().optional(),
  children: z.array(TreeNodeSchema).optional(),
}))

const TreeResultSchema = z.object({ root: TreeNodeSchema })

type TreeNode = {
  slug: string
  name: string
  is_dir: boolean
  title?: string | undefined
  status?: string | undefined
  children?: TreeNode[] | undefined
}

class IntentItem extends vscode.TreeItem {
  constructor(readonly node: TreeNode) {
    super(node.title || node.name, node.is_dir ? vscode.TreeItemCollapsibleState.Collapsed : vscode.TreeItemCollapsibleState.None)
    if (node.status !== undefined) this.description = node.status
    this.id = node.slug
  }
}

class IntentTreeProvider implements vscode.TreeDataProvider<IntentItem>, vscode.Disposable {
  private readonly changed = new vscode.EventEmitter<IntentItem | undefined | null | void>()
  readonly onDidChangeTreeData = this.changed.event
  private root: TreeNode | undefined
  private readonly ready: Promise<void>
  private readonly mcpClients: client.Client[]
  private readonly workspaces: readonly vscode.WorkspaceFolder[]

  constructor(command: string, workspaces: readonly vscode.WorkspaceFolder[]) {
    this.workspaces = workspaces
    this.mcpClients = workspaces.map((workspace) => new client.Client({ name: `kron-vscode-${workspace.uri.toString()}`, version: '0.0.1' }))
    this.ready = Promise.all(this.mcpClients.map(async (mcpClient, index) => {
      const workspace = workspaces[index]
      if (workspace === undefined) throw new Error('workspace/client pairing is inconsistent')
      const transport = new stdio.StdioClientTransport({ command, args: ['serve-mcp', '-CWD', workspace.uri.fsPath] })
      await mcpClient.connect(transport)
    })).then(() => undefined)
  }

  async refresh(): Promise<void> {
    await this.ready
    const roots = await Promise.all(this.mcpClients.map(async (mcpClient) => {
      const result = await mcpClient.callTool({ name: 'kron_tree', arguments: {} })
      if (result.isError === true || result.structuredContent === undefined) {
        throw new Error('kron_tree returned no structured result')
      }
      return TreeResultSchema.parse(result.structuredContent).root
    }))
    if (this.workspaces.length === 1) {
      this.root = roots[0]
    } else {
      this.root = {
        slug: 'workspace-roots',
        name: 'Workspaces',
        is_dir: true,
        children: roots.map((root, index) => {
          const workspace = this.workspaces[index]
          if (workspace === undefined) throw new Error('workspace/tree pairing is inconsistent')
          return {
            slug: workspace.uri.toString(),
            name: workspace.name,
            title: workspace.name,
            is_dir: true,
            children: root.children ?? [],
          }
        }),
      }
    }
    this.changed.fire()
  }

  getTreeItem(item: IntentItem): vscode.TreeItem {
    return item
  }

  getChildren(item?: IntentItem): IntentItem[] {
    const node = item?.node ?? this.root
    if (node === undefined) return []
    return (node.children ?? []).map((child) => new IntentItem(child))
  }

  dispose(): void {
    this.changed.dispose()
    for (const mcpClient of this.mcpClients) void mcpClient.close()
  }
}

export = { IntentTreeProvider }
