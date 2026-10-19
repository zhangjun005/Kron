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
  private readonly mcpClient: client.Client
  private readonly ready: Promise<void>

  constructor(command: string, cwd: string) {
    this.mcpClient = new client.Client({ name: 'kron-vscode', version: '0.0.1' })
    const transport = new stdio.StdioClientTransport({ command, args: ['serve-mcp', '-CWD', cwd] })
    this.ready = this.mcpClient.connect(transport)
  }

  async refresh(): Promise<void> {
    await this.ready
    const result = await this.mcpClient.callTool({ name: 'kron_tree', arguments: {} })
    if (result.isError === true || result.structuredContent === undefined) {
      throw new Error('kron_tree returned no structured result')
    }
    this.root = TreeResultSchema.parse(result.structuredContent).root
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
    void this.mcpClient.close()
  }
}

export = { IntentTreeProvider }
