import clientSdk = require('@modelcontextprotocol/sdk/client/index.js')
import stdio = require('@modelcontextprotocol/sdk/client/stdio.js')
import zod = require('zod')

export type ConnectionState = 'stopped' | 'starting' | 'ready' | 'failed' | 'stopping'

export type TreeNode = {
  slug: string
  name: string
  is_dir: boolean
  workspaceId?: string | undefined
  title?: string | undefined
  status?: string | undefined
  children?: TreeNode[] | undefined
}

export type TreeResult = { root: TreeNode }

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

export const MCP_TOOL_NAMES = [
  'kron_tree', 'kron_get', 'kron_add', 'kron_update', 'kron_delete', 'kron_restore',
  'kron_assume_list', 'kron_assume_create',
] as const
export type McpToolName = typeof MCP_TOOL_NAMES[number]

const z = zod.z
const treeNodeSchema: zod.ZodType<TreeNode> = z.lazy(() => z.object({
  slug: z.string(),
  name: z.string(),
  is_dir: z.boolean(),
  workspaceId: z.string().optional(),
  title: z.string().optional(),
  status: z.string().optional(),
  children: z.array(treeNodeSchema).optional(),
}))
const treeResultSchema = z.object({ root: treeNodeSchema })
const stringList = z.array(z.string())
const assumptionRefSchema = z.object({
  id: z.string().min(1),
  severity: z.enum(['hard', 'soft']),
  rationale: z.string().optional(),
  expires_at: z.string().optional(),
  verified_at: z.string().optional(),
  verified_by: z.string().optional(),
})
const toolInputSchemas: Record<McpToolName, zod.ZodTypeAny> = {
  kron_tree: z.object({}).strict(),
  kron_get: z.object({ slug: z.string().min(1), include_relations: z.boolean().optional() }).strict(),
  kron_add: z.object({
    slug: z.string().min(1),
    symbol: z.string().optional(),
    why: z.string().optional(),
    kind: z.enum(['leaf', 'node']).optional(),
  }).strict(),
  kron_update: z.object({
    slug: z.string().min(1),
    symbol: stringList.optional(),
    status: z.enum(['draft', 'active', 'superseded']).optional(),
    reviewers: stringList.optional(),
    body: z.string().optional(),
    references: stringList.optional(),
    depends_on: stringList.optional(),
    assumptions: z.array(assumptionRefSchema).optional(),
  }).strict(),
  kron_delete: z.object({ slug: z.string().min(1) }).strict(),
  kron_restore: z.object({ slug: z.string().min(1) }).strict(),
  kron_assume_list: z.object({ status: z.enum(['active', 'superseded']).optional() }).strict(),
  kron_assume_create: z.object({
    id: z.string().min(1),
    text: z.string().min(1),
    default_severity: z.enum(['hard', 'soft']),
    body: z.string().optional(),
  }).strict(),
}
const toolOutputSchemas: Record<McpToolName, zod.ZodTypeAny> = {
  kron_tree: treeResultSchema,
  kron_get: z.object({ intent: z.object({
    slug: z.string(),
    frontmatter: z.object({
      symbol: stringList.optional(),
      created_by: z.string().optional(),
      updated_at: z.string().optional(),
      reviewers: stringList.optional(),
      status: z.string().optional(),
      assumptions: z.array(z.object({
        id: z.string(),
        text: z.string().optional(),
        severity: z.string(),
        default_severity: z.string().optional(),
        rationale: z.string().optional(),
        expires_at: z.string().optional(),
      })).optional(),
      references: stringList.optional(),
      depends_on: stringList.optional(),
    }),
    body: z.string(),
    source_path: z.string(),
    references: stringList.optional(),
    depends_on: stringList.optional(),
  }) }),
  kron_add: z.object({ ok: z.boolean(), path: z.string() }),
  kron_update: z.object({ ok: z.boolean(), path: z.string() }),
  kron_delete: z.object({ ok: z.boolean(), trashed_path: z.string(), dependents: stringList.optional() }),
  kron_restore: z.object({ ok: z.boolean(), path: z.string() }),
  kron_assume_list: z.object({ assumptions: z.array(z.object({
    id: z.string(),
    text: z.string(),
    default_severity: z.string(),
    status: z.string(),
  })) }),
  kron_assume_create: z.object({ ok: z.boolean(), path: z.string() }),
}
const TOOL_TIMEOUT_MS = 15_000

class McpToolRejected extends Error {}

function withTimeout<T>(promise: Promise<T>): Promise<T> {
  let timer: NodeJS.Timeout | undefined
  const deadline = new Promise<never>((_, reject) => {
    timer = setTimeout(() => reject(new Error('kron_tree timed out after 15 seconds.')), TOOL_TIMEOUT_MS)
  })
  return Promise.race([promise, deadline]).finally(() => {
    if (timer !== undefined) clearTimeout(timer)
  })
}

/** Owns one independent kron serve-mcp process for one workspace folder. */
export class KronMcpClient {
  private client: clientSdk.Client | undefined
  private transport: stdio.StdioClientTransport | undefined
  private currentState: ConnectionState = 'stopped'

  constructor(
    private readonly command: string,
    private readonly cwd: string,
    private readonly log: (message: string) => void,
    private readonly onState: (state: ConnectionState) => void,
  ) {}

  get state(): ConnectionState {
    return this.currentState
  }

  async start(): Promise<void> {
    if (this.currentState === 'ready' || this.currentState === 'starting') return
    this.setState('starting')

    const client = new clientSdk.Client({ name: 'kron-vscode', version: '0.0.1' }, { capabilities: {} })
    const transport = new stdio.StdioClientTransport({
      command: this.command,
      args: ['serve-mcp', '-CWD', this.cwd],
    })
    transport.onerror = (error: Error): void => {
      this.log(`MCP transport error: ${error.message}`)
      this.setState('failed')
    }
    transport.onclose = (): void => {
      if (this.currentState !== 'stopping') this.setState('failed')
    }

    this.client = client
    this.transport = transport
    try {
      await client.connect(transport)
      this.setState('ready')
    } catch (error: unknown) {
      this.log(`MCP connection failed: ${error instanceof Error ? error.message : String(error)}`)
      this.setState('failed')
      await this.closeTransport(client, transport)
      this.client = undefined
      this.transport = undefined
      throw error
    }
  }

  async getTree(): Promise<TreeResult> {
    return await this.callTool('kron_tree', {}) as TreeResult
  }

  async callTool(tool: McpToolName, args: Record<string, unknown>): Promise<unknown> {
    if (this.currentState !== 'ready' || this.client === undefined) {
      throw new Error('The Kron MCP connection is not ready.')
    }
    const input = toolInputSchemas[tool].parse(args) as Record<string, unknown>
    try {
      const result = await withTimeout(this.client.callTool({ name: tool, arguments: input }))
      if (result.isError === true) {
        const content = Array.isArray(result.content) ? result.content : []
        const detail = content
          .filter((item): item is { type: 'text'; text: string } => isRecord(item) && item.type === 'text' && typeof item.text === 'string')
          .map((item) => item.text)
          .join('\n')
        this.log(`${tool} rejected: ${detail || 'MCP server returned an error.'}`)
        throw new McpToolRejected(`Kron could not complete ${tool}. Check the entered values and try again.`)
      }
      if (result.structuredContent === undefined) {
        throw new Error(`${tool} did not return a structured result.`)
      }
      return toolOutputSchemas[tool].parse(result.structuredContent)
    } catch (error: unknown) {
      if (error instanceof Error && /unknown tool/i.test(error.message)) {
        this.log(`${tool} is not supported by this Kron CLI: ${error.message}`)
        throw new McpToolRejected(`This Kron CLI does not support ${tool}. Other Kron MCP tools remain available.`)
      }
      this.log(`${tool} failed: ${error instanceof Error ? error.message : String(error)}`)
      if (!(error instanceof McpToolRejected)) this.setState('failed')
      throw error
    }
  }

  async stop(): Promise<void> {
    const client = this.client
    const transport = this.transport
    if (client === undefined && transport === undefined) {
      this.setState('stopped')
      return
    }
    this.setState('stopping')
    try {
      if (client !== undefined) await client.close()
      else if (transport !== undefined) await transport.close()
    } catch (error: unknown) {
      this.log(`MCP shutdown error: ${error instanceof Error ? error.message : String(error)}`)
    } finally {
      this.client = undefined
      this.transport = undefined
      this.setState('stopped')
    }
  }

  private async closeTransport(client: clientSdk.Client, transport: stdio.StdioClientTransport): Promise<void> {
    try {
      await client.close()
    } catch (error: unknown) {
      this.log(`MCP cleanup error: ${error instanceof Error ? error.message : String(error)}`)
      try {
        await transport.close()
      } catch {
        // The child may already have exited while the connection was failing.
      }
    }
  }

  private setState(state: ConnectionState): void {
    this.currentState = state
    this.onState(state)
  }
}
