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

/**
 * Tools the Webview may request through the host bridge. Keep this list, the
 * Webview's `McpToolName` union (frontend/vscode/webview/src/lib/mcp-types.ts)
 * and the zod contracts below in sync; `kron_tree` is not here because the tree
 * has its own request/refresh messages.
 */
export const MCP_TOOL_NAMES = [
  'kron_add', 'kron_assume_check', 'kron_assume_create', 'kron_assume_delete', 'kron_assume_get',
  'kron_assume_list', 'kron_assume_update', 'kron_delete', 'kron_get', 'kron_restore', 'kron_update',
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
  kron_delete: z.object({
    slug: z.string().min(1),
    // Node intents require an explicit acknowledgement that the whole
    // subtree moves to trash.
    confirm_subtree: z.boolean().optional(),
  }).strict(),
  kron_restore: z.object({ slug: z.string().min(1) }).strict(),
  kron_assume_list: z.object({ status: z.enum(['active', 'superseded']).optional() }).strict(),
  kron_assume_create: z.object({
    id: z.string().min(1),
    text: z.string().min(1),
    default_severity: z.enum(['hard', 'soft']),
    body: z.string().optional(),
  }).strict(),
  kron_assume_check: z.object({ file_path: z.string().optional() }).strict(),
  kron_assume_get: z.object({ id: z.string().min(1) }).strict(),
  kron_assume_update: z.object({
    id: z.string().min(1),
    text: z.string().optional(),
    default_severity: z.enum(['hard', 'soft']).optional(),
    body: z.string().optional(),
    status: z.enum(['draft', 'active', 'superseded']).optional(),
    expires_at: z.string().optional(),
    verified_at: z.string().optional(),
    verified_by: z.string().optional(),
    reviewers: stringList.optional(),
  }).strict(),
  kron_assume_delete: z.object({ id: z.string().min(1) }).strict(),
}
const toolOutputSchemas: Record<McpToolName, zod.ZodTypeAny> = {
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
  kron_delete: z.object({
    ok: z.boolean(),
    trashed_path: z.string(),
    dependents: stringList.optional(),
    deleted_slugs: stringList.optional(),
  }),
  kron_restore: z.object({ ok: z.boolean(), path: z.string(), restored_slugs: stringList.optional() }),
  kron_assume_list: z.object({ assumptions: z.array(z.object({
    id: z.string(),
    text: z.string(),
    default_severity: z.string(),
    status: z.string(),
    // Metadata the assumption panel renders. Keep these fields declared: zod
    // object schemas strip unknown keys, so an omission here silently drops the
    // dates before they reach the Webview.
    created_by: z.string().optional(),
    updated_at: z.string().optional(),
    expires_at: z.string().optional(),
    verified_at: z.string().optional(),
    verified_by: z.string().optional(),
    reviewers: stringList.optional(),
  })) }),
  kron_assume_create: z.object({ ok: z.boolean(), path: z.string() }),
  kron_assume_check: z.object({
    warnings: z.array(z.object({
      intent_slug: z.string(), assumption_id: z.string(), severity: z.string(), text: z.string(),
      default_severity: z.string().optional(), rationale: z.string().optional(), expires_at: z.string().optional(),
      days_remaining: z.number().optional(), verified: z.boolean(),
    })),
    summary: z.object({ hard: z.number(), soft: z.number(), expired: z.number() }),
  }),
  kron_assume_get: z.object({ assumption: z.object({
    id: z.string(),
    text: z.string(),
    default_severity: z.string(),
    status: z.string(),
    created_by: z.string().optional(),
    updated_at: z.string().optional(),
    expires_at: z.string().optional(),
    verified_at: z.string().optional(),
    verified_by: z.string().optional(),
    reviewers: stringList.optional(),
    body: z.string().optional(),
    source_path: z.string(),
  }) }),
  kron_assume_update: z.object({ ok: z.boolean(), path: z.string() }),
  kron_assume_delete: z.object({ ok: z.boolean(), trashed_path: z.string(), referencing_intents: stringList.optional() }),
}

/**
 * `kron_impact` is editor-menu only (never requested by the Webview), so it is
 * kept outside MCP_TOOL_NAMES and called through the raw path.
 */
const impactInputSchema = z.object({ slug: z.string().min(1) }).strict()
const impactAnchorSchema = z.object({
  file_path: z.string(),
  line: z.number(),
  kind: z.string().optional(),
})
const impactOutputSchema = z.object({
  intent: z.object({
    slug: z.string(),
    status: z.string().optional(),
    symbol: stringList.optional(),
    updated_at: z.string().optional(),
  }),
  incoming_anchors_code: z.array(impactAnchorSchema).optional(),
  incoming_anchors_markdown: z.array(impactAnchorSchema).optional(),
  references: stringList.optional(),
  prerequisites: stringList.optional(),
})

export type ImpactSummary = zod.infer<typeof impactOutputSchema>

const TOOL_TIMEOUT_MS = 15_000

class McpToolRejected extends Error {}

function withTimeout<T>(promise: Promise<T>, tool: string): Promise<T> {
  let timer: NodeJS.Timeout | undefined
  const deadline = new Promise<never>((_, reject) => {
    timer = setTimeout(() => reject(new Error(`${tool} timed out after ${TOOL_TIMEOUT_MS / 1000} seconds.`)), TOOL_TIMEOUT_MS)
  })
  return Promise.race([promise, deadline]).finally(() => {
    if (timer !== undefined) clearTimeout(timer)
  })
}

/** Owns one kron serve-mcp process for the active Kron project. */
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
    return await this.callToolRaw('kron_tree', {}, treeResultSchema) as TreeResult
  }

  /** Reverse-link view of one intent, used by the editor "Show Impact" action. */
  async getImpact(slug: string): Promise<ImpactSummary> {
    const args = impactInputSchema.parse({ slug }) as Record<string, unknown>
    return await this.callToolRaw('kron_impact', args, impactOutputSchema) as ImpactSummary
  }

  async callTool(tool: McpToolName, args: Record<string, unknown>): Promise<unknown> {
    const input = toolInputSchemas[tool].parse(args) as Record<string, unknown>
    return await this.callToolRaw(tool, input, toolOutputSchemas[tool])
  }

  /**
   * Calls one MCP tool and validates the structured result.
   *
   * Protocol/data errors (tool rejection, missing structured output, invalid
   * shape, timeout) are surfaced as failures of this call only — they do not
   * mark the connection dead. Connection death is reported by the transport
   * hooks (`onerror` / `onclose`) and by `start()`.
   */
  private async callToolRaw(
    tool: string,
    args: Record<string, unknown>,
    outputSchema: zod.ZodTypeAny,
  ): Promise<unknown> {
    if (this.currentState !== 'ready' || this.client === undefined) {
      throw new Error('The Kron MCP connection is not ready.')
    }
    try {
      const result = await withTimeout(this.client.callTool({ name: tool, arguments: args }), tool)
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
      return outputSchema.parse(result.structuredContent)
    } catch (error: unknown) {
      if (error instanceof Error && /unknown tool/i.test(error.message)) {
        this.log(`${tool} is not supported by this Kron CLI: ${error.message}`)
        throw new McpToolRejected(`This Kron CLI does not support ${tool}. Other Kron MCP tools remain available.`)
      }
      this.log(`${tool} failed: ${error instanceof Error ? error.message : String(error)}`)
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
