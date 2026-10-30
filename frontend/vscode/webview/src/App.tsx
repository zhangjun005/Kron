import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent, type ReactNode } from 'react'
import kronLogo from './assets/kronLogo.png'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { IntentCanvas } from '@/IntentCanvas'
import { isWebviewInbound, type McpToolName, type TreeNode, type TreeResult, type WebviewOutbound } from '@/lib/mcp-types'
import { applyWebviewFontScale, webviewFontScaleFromBody } from '@/lib/font-size'

type Severity = 'hard' | 'soft'

type IntentAssumption = {
  id: string
  severity: Severity
  rationale: string
  expiresAt?: string
}

type Intent = {
  slug: string
  kind: 'leaf' | 'node'
  title: string
  createdBy: string
  summary: string
  symbol: string
  status: 'draft' | 'active' | 'superseded'
  reviewers: string[]
  body: string
  references: string[]
  dependsOn: string[]
  assumptions: IntentAssumption[]
  workspaceId?: string
  dependents?: string[]
}

type IntentRelations = {
  references: string[]
  dependsOn: string[]
  kind?: Intent['kind']
  exists?: boolean
}

type Assumption = {
  id: string
  text: string
  defaultSeverity: Severity
  status?: string
  createdBy?: string
  updatedAt?: string
  expiresAt?: string
  verifiedAt?: string
  verifiedBy?: string
  reviewers?: string[]
  usedBy?: string[]
}

type AssumptionUsageEntry = { intentSlug: string; severity: Severity; rationale: string }
type AssumptionUsage = Record<string, AssumptionUsageEntry[]>

type AssumptionPanelDraft = {
  id: string
  text: string
  defaultSeverity: Severity
  status: 'draft' | 'active' | 'superseded'
  /** Value shown in the date input (YYYY-MM-DD). */
  expiresAt: string
  /** Raw frontmatter value, so precision is preserved unless the user edits it. */
  expiresAtRaw: string
  reviewersText: string
}

function emptyAssumptionPanelDraft(): AssumptionPanelDraft {
  return { id: '', text: '', defaultSeverity: 'hard', status: 'active', expiresAt: '', expiresAtRaw: '', reviewersText: '' }
}

type AssumptionCheck = {
  hard: number
  soft: number
  expired: number
  warnings: Array<{ intentSlug: string; assumptionId: string; text: string; severity: string; verified: boolean }>
}

type IntentDraft = Pick<Intent, 'slug' | 'kind' | 'title' | 'symbol' | 'status' | 'body'> & {
  summary: string
  reviewersText: string
  referencesText: string
  dependsOnText: string
}

type BuildNode = Omit<TreeNode, 'children'> & { children: BuildNode[] }

type PendingMcpRequest = {
  resolve: (value: unknown) => void
  reject: (error: Error) => void
  timer: ReturnType<typeof setTimeout>
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function stringArray(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string') : []
}

function summaryFromBody(body: string): string {
  for (const line of body.split(/\r?\n/)) {
    const value = line.trim()
    if (value === '' || value.startsWith('#') || value.startsWith('>') || value.startsWith('-') || value.startsWith('*')) continue
    return value
  }
  return ''
}

function headingFromBody(body: string, fallback: string): string {
  const heading = body.split(/\r?\n/).find((line) => /^#\s+/.test(line.trim()))
  return heading?.trim().replace(/^#\s+/, '').trim() || fallback
}

function intentFromMcp(value: unknown, node: TreeNode): Intent {
  if (!isRecord(value) || !isRecord(value.intent)) {
    throw new Error('The Kron server returned invalid intent details.')
  }
  const wire = value.intent
  const frontmatter = wire.frontmatter
  if (!isRecord(frontmatter)) throw new Error('The Kron server returned invalid intent frontmatter.')
  const body = typeof wire.body === 'string' ? wire.body : ''
  const sourcePath = typeof wire.source_path === 'string' ? wire.source_path.replace(/\\/g, '/') : ''
  const intentsRoot = '/.kron/intents/'
  const intentsRootIndex = sourcePath.toLowerCase().lastIndexOf(intentsRoot)
  const relativeIntentPath = intentsRootIndex >= 0 ? sourcePath.slice(intentsRootIndex + intentsRoot.length) : ''
  const kind: Intent['kind'] = relativeIntentPath !== ''
    ? (relativeIntentPath.toLowerCase().endsWith('/readme.md') ? 'node' : 'leaf')
    : node.is_dir ? 'node' : 'leaf'
  const rawStatus = frontmatter.status
  const status: Intent['status'] = rawStatus === 'active' || rawStatus === 'superseded' ? rawStatus : 'draft'
  const assumptions = Array.isArray(frontmatter.assumptions)
    ? frontmatter.assumptions.flatMap((entry): IntentAssumption[] => {
        if (!isRecord(entry) || typeof entry.id !== 'string') return []
        const severity: Severity = entry.severity === 'soft' ? 'soft' : 'hard'
        return [{
          id: entry.id,
          severity,
          rationale: typeof entry.rationale === 'string' ? entry.rationale : '',
          ...(typeof entry.expires_at === 'string' && entry.expires_at !== '' ? { expiresAt: entry.expires_at } : {}),
        }]
      })
    : []
  const relations = isRecord(wire.relations) ? wire.relations : undefined
  return {
    slug: typeof wire.slug === 'string' ? wire.slug : node.slug,
    kind,
    title: headingFromBody(body, node.title ?? node.name),
    createdBy: typeof frontmatter.created_by === 'string' ? frontmatter.created_by : 'Unknown',
    summary: summaryFromBody(body),
    symbol: stringArray(frontmatter.symbol).join(', '),
    status,
    reviewers: stringArray(frontmatter.reviewers),
    body,
    references: stringArray(wire.references ?? frontmatter.references),
    dependsOn: stringArray(wire.depends_on ?? frontmatter.depends_on),
    assumptions,
    ...(node.workspaceId === undefined ? {} : { workspaceId: node.workspaceId }),
    ...(relations === undefined ? {} : { dependents: stringArray(relations.dependents) }),
  }
}

function createIntentBody(title: string, summary: string, body: string): string {
  return [`# ${title.trim()}`, summary.trim(), body.trim()].filter(Boolean).join('\n\n')
}

function replaceIntentHeading(body: string, title: string): string {
  const lines = body.split(/\r?\n/)
  const firstContent = lines.findIndex((line) => line.trim() !== '')
  if (firstContent < 0) return `# ${title.trim()}`
  if (/^#\s+/.test(lines[firstContent]!.trim())) {
    lines[firstContent] = `# ${title.trim()}`
    return lines.join('\n')
  }
  lines.splice(firstContent, 0, `# ${title.trim()}`, '')
  return lines.join('\n')
}

function assumptionsToMcp(items: IntentAssumption[]): Array<Record<string, unknown>> {
  return items.map((item) => ({
    id: item.id,
    severity: item.severity,
    rationale: item.rationale,
    ...(item.expiresAt === undefined ? {} : { expires_at: item.expiresAt }),
  }))
}

const initialIntents: Intent[] = [
  {
    slug: 'auth/jwt-sliding-window',
    kind: 'leaf',
    title: 'JWT sliding window renewal',
    createdBy: '@zhangjun005',
    summary: 'Keep refresh tokens short lived while preserving a smooth session for active users.',
    symbol: 'auth.RefreshToken',
    status: 'active',
    reviewers: ['@zhangjun005', '@lanyue811'],
    body: '## Why\n\nRotate the refresh token after each active session renewal. Keep access tokens short lived so a leaked token has a limited window.\n\n## Trade-offs\n\nThe client must persist the newest refresh token after every successful renewal.',
    references: ['auth/token-storage', 'security/session-timeout'],
    dependsOn: ['auth/token-storage'],
    assumptions: [
      {
        id: 'single-region',
        severity: 'hard',
        rationale: 'Refresh tokens are stored in one region, so cross-region reads cannot race renewal.',
        expiresAt: '2026-12-01',
      },
      {
        id: 'csrf-protected',
        severity: 'hard',
        rationale: 'The renewal endpoint checks the CSRF token before rotating credentials.',
      },
    ],
  },
  {
    slug: 'auth/token-storage',
    kind: 'leaf',
    title: 'Refresh token storage',
    createdBy: '@zhangjun005',
    summary: 'Store refresh tokens as hashes and support revocation by session.',
    symbol: 'auth.TokenStore',
    status: 'active',
    reviewers: ['@zhangjun005'],
    body: '## Why\n\nPersist only a one-way hash of each refresh token. Keep the session identifier available for revocation and audit.',
    references: ['security/session-timeout'],
    dependsOn: [],
    assumptions: [
      {
        id: 'single-region',
        severity: 'soft',
        rationale: 'A single region is preferred for now; replication lag only delays revocation visibility.',
      },
    ],
  },
  {
    slug: 'ui/console-errors',
    kind: 'leaf',
    title: 'Readable console errors',
    createdBy: 'agent:preview',
    summary: 'Show actionable errors without exposing internal stack traces to end users.',
    symbol: 'ui.ErrorNotice',
    status: 'draft',
    reviewers: [],
    body: '## Why\n\nUsers need a clear next step when a request fails. Keep diagnostic details in the application log.',
    references: [],
    dependsOn: [],
    assumptions: [],
  },
]

const initialAssumptions: Assumption[] = [
  {
    id: 'single-region',
    text: 'Production traffic is served from one region.',
    defaultSeverity: 'hard',
    status: 'active',
    createdBy: '@zhangjun005',
    updatedAt: '2026-10-01T08:00:00Z',
    expiresAt: '2027-01-01',
    verifiedAt: '2026-10-01',
    verifiedBy: '@zhangjun005',
    usedBy: ['auth/jwt-sliding-window', 'auth/token-storage'],
  },
  {
    id: 'csrf-protected',
    text: 'State-changing browser requests include a valid CSRF token.',
    defaultSeverity: 'hard',
    status: 'active',
    createdBy: '@zhangjun005',
    updatedAt: '2026-10-01T08:00:00Z',
    verifiedAt: '2026-10-01',
    verifiedBy: '@lanyue811',
    usedBy: ['auth/jwt-sliding-window'],
  },
  {
    id: 'clock-skew-30s',
    text: 'Application hosts stay within thirty seconds of the time source.',
    defaultSeverity: 'soft',
    status: 'active',
    usedBy: [],
  },
]

declare global {
  interface Window {
    acquireVsCodeApi?: () => {
      postMessage: (message: WebviewOutbound) => void
      getState: () => unknown
      setState: (state: unknown) => void
    }
  }
}

const vscodeApi = typeof window.acquireVsCodeApi === 'function'
  ? window.acquireVsCodeApi()
  : null

type PersistedWebviewState = { selectedSlug?: string; filter?: string }

/** Selection/filter survive the panel being hidden (the Webview context is recreated). */
function readPersistedState(): PersistedWebviewState {
  if (vscodeApi === null) return {}
  const raw = vscodeApi.getState()
  if (!isRecord(raw)) return {}
  return {
    ...(typeof raw.selectedSlug === 'string' ? { selectedSlug: raw.selectedSlug } : {}),
    ...(typeof raw.filter === 'string' ? { filter: raw.filter } : {}),
  }
}

const persistedState = readPersistedState()

function makeTree(intents: Intent[]): TreeNode {
  const root: BuildNode = { slug: 'workspace', name: 'workspace', is_dir: true, children: [] }

  for (const intent of intents) {
    const segments = intent.slug.split('/')
    let parent = root
    let path = ''

    segments.forEach((segment, index) => {
      path = path === '' ? segment : `${path}/${segment}`
      const isLastSegment = index === segments.length - 1
      const isLeaf = isLastSegment && intent.kind === 'leaf'
      let child: BuildNode | undefined = parent.children.find((item) => item.name === segment)

      if (child === undefined) {
        child = {
          slug: path,
          name: segment,
          is_dir: !isLeaf,
          children: [],
          ...(isLastSegment ? { title: intent.title, status: intent.status } : {}),
        }
        parent.children.push(child)
      } else if (isLastSegment) {
        child.title = intent.title
        child.status = intent.status
        child.is_dir = !isLeaf
      }

      parent = child
    })
  }

  return root
}

function flattenTree(node: TreeNode, depth = 0): Array<{ node: TreeNode; depth: number }> {
  const entries = [{ node, depth }]
  for (const child of node.children ?? []) {
    entries.push(...flattenTree(child, depth + 1))
  }
  return entries
}

function splitLines(value: string): string[] {
  return value.split(/[\n,]/).map((item) => item.trim()).filter(Boolean)
}

function isWorkspaceRoot(node: TreeNode): boolean {
  return node.is_dir && node.workspaceId !== undefined && node.slug === node.workspaceId
}

function newDraft(slug = '', kind: Intent['kind'] = 'leaf'): IntentDraft {
  return {
    slug,
    kind,
    title: '',
    summary: '',
    symbol: '',
    status: 'draft',
    reviewersText: '',
    body: '',
    referencesText: '',
    dependsOnText: '',
  }
}

function draftFromIntent(intent: Intent): IntentDraft {
  return {
    slug: intent.slug,
    kind: intent.kind,
    title: intent.title,
    summary: intent.summary,
    symbol: intent.symbol,
    status: intent.status,
    reviewersText: intent.reviewers.join(', '),
    body: intent.body,
    referencesText: intent.references.join('\n'),
    dependsOnText: intent.dependsOn.join('\n'),
  }
}

function fieldClass(): string {
  return 'w-full rounded-md border border-input bg-background px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring'
}

function Field({ label, hint, children }: { label: string; hint?: string; children: ReactNode }): JSX.Element {
  return (
    <label className="grid gap-1.5 text-sm">
      <span className="font-medium">{label}</span>
      {children}
      {hint ? <span className="text-xs text-muted-foreground">{hint}</span> : null}
    </label>
  )
}

type EditParam = 'status' | 'created' | 'review' | 'symbol' | 'depends' | 'references'

function toggleEditParam(current: EditParam | null, next: EditParam): EditParam | null {
  return current === next ? null : next
}

function paramChipClass(active: boolean): string {
  return `min-w-0 truncate rounded-md border px-1.5 py-1 text-left text-[0.6875rem] ${active ? 'border-primary/50 bg-primary/10 text-foreground' : 'bg-background text-muted-foreground hover:border-primary/40 hover:text-foreground'}`
}

/** Fixed-type field marker: draft is hollow, active green, superseded red. */
function StatusDot({ status }: { status: string }): JSX.Element {
  if (status === 'active') return <span className="h-2 w-2 shrink-0 rounded-full bg-emerald-500" />
  if (status === 'superseded') return <span className="h-2 w-2 shrink-0 rounded-full bg-red-500" />
  return <span className="h-2 w-2 shrink-0 rounded-full border border-muted-foreground/70" />
}

/** Shows the current value without its field name; clicking expands the editor. */
function EditChip({ label, value, active, onToggle }: { label: string; value: string; active: boolean; onToggle: () => void }): JSX.Element {
  const empty = value.trim() === ''
  return (
    <button
      type="button"
      className={paramChipClass(active)}
      aria-label={label}
      aria-expanded={active}
      title={empty ? label : `${label}: ${value}`}
      onClick={onToggle}
    >
      {empty ? '…' : value}
    </button>
  )
}

function App(): JSX.Element {
  const [intents, setIntents] = useState<Intent[]>(initialIntents)
  const [archivedIntents, setArchivedIntents] = useState<Intent[]>([])
  const [assumptions, setAssumptions] = useState<Assumption[]>(initialAssumptions)
  const [tree, setTree] = useState<TreeResult | null>(null)
  const [treeRelations, setTreeRelations] = useState<Record<string, IntentRelations>>({})
  const [liveIntent, setLiveIntent] = useState<Intent | null>(null)
  const [selectedSlug, setSelectedSlug] = useState<string>(persistedState.selectedSlug ?? initialIntents[0]?.slug ?? '')
  const [revealToken, setRevealToken] = useState(0)
  const [filter, setFilter] = useState<string>(persistedState.filter ?? '')
  const [uiScale, setUiScale] = useState<number>(() => webviewFontScaleFromBody())
  const [loading, setLoading] = useState(false)
  const [detailLoading, setDetailLoading] = useState(false)
  const [detailReload, setDetailReload] = useState(0)
  const [detailError, setDetailError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [formMode, setFormMode] = useState<'create' | 'edit' | null>(null)
  const [draft, setDraft] = useState<IntentDraft>(newDraft())
  const [draftParentSlug, setDraftParentSlug] = useState<string | null>(null)
  const [formError, setFormError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const [pendingDeleteSlug, setPendingDeleteSlug] = useState<string | null>(null)
  const [pendingEditSlug, setPendingEditSlug] = useState<string | null>(null)
  const [showArchivePanel, setShowArchivePanel] = useState(false)
  const [showAssumptionPanel, setShowAssumptionPanel] = useState(false)
  const [assumptionPanelId, setAssumptionPanelId] = useState<string | null>(null)
  const [assumptionPanelCreating, setAssumptionPanelCreating] = useState(false)
  const [assumptionPanelDraft, setAssumptionPanelDraft] = useState<AssumptionPanelDraft>(emptyAssumptionPanelDraft)
  const [assumptionPanelError, setAssumptionPanelError] = useState<string | null>(null)
  const [assumptionPanelSaving, setAssumptionPanelSaving] = useState(false)
  const [assumptionPanelPendingDelete, setAssumptionPanelPendingDelete] = useState<string | null>(null)
  const [assumptionUsage, setAssumptionUsage] = useState<AssumptionUsage>({})
  const [showAssumptionForm, setShowAssumptionForm] = useState(false)
  const [assumptionMode, setAssumptionMode] = useState<'existing' | 'new'>('existing')
  const [selectedAssumptionId, setSelectedAssumptionId] = useState('')
  const [assumptionDraft, setAssumptionDraft] = useState({
    id: '',
    text: '',
    defaultSeverity: 'hard' as Severity,
    severity: 'hard' as Severity,
    rationale: '',
    expiresAt: '',
  })
  const [assumptionCheck, setAssumptionCheck] = useState<AssumptionCheck | null>(null)
  const [assumptionCheckError, setAssumptionCheckError] = useState<string | null>(null)
  const [assumptionFormError, setAssumptionFormError] = useState<string | null>(null)
  const [editParam, setEditParam] = useState<EditParam | null>(null)
  const pendingMcpRequests = useRef(new Map<number, PendingMcpRequest>())
  const nextMcpRequestId = useRef(0)
  const detailRequestSequence = useRef(0)
  const treeRelationSequence = useRef(0)

  useEffect(() => {
    if (notice === null) return
    const timer = setTimeout(() => setNotice(null), 5000)
    return () => clearTimeout(timer)
  }, [notice])

  useEffect(() => {
    vscodeApi?.setState({ selectedSlug, filter })
  }, [selectedSlug, filter])

  const sendMcpRequest = useCallback((tool: McpToolName, args: Record<string, unknown>, workspaceId?: string): Promise<unknown> => {
    if (vscodeApi === null) return Promise.reject(new Error('MCP requests are only available inside VS Code.'))
    const requestId = ++nextMcpRequestId.current
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        pendingMcpRequests.current.delete(requestId)
        reject(new Error(`Kron ${tool} request timed out. Refresh and try again.`))
      }, 20_000)
      pendingMcpRequests.current.set(requestId, { resolve, reject, timer })
      const message: WebviewOutbound = {
        type: 'kron_mcp:request',
        requestId,
        tool,
        args,
        ...(workspaceId === undefined ? {} : { workspaceId }),
      }
      try {
        vscodeApi.postMessage(message)
      } catch (error: unknown) {
        clearTimeout(timer)
        pendingMcpRequests.current.delete(requestId)
        reject(error instanceof Error ? error : new Error(String(error)))
      }
    })
  }, [])

  const canEdit = vscodeApi === null
  const mockTree = useMemo(() => makeTree(intents), [intents])
  const emptyLiveTree = useMemo<TreeNode>(() => ({ slug: 'workspace', name: 'workspace', is_dir: true, children: [] }), [])
  const activeTree = tree?.root ?? (canEdit ? mockTree : emptyLiveTree)
  const rows = useMemo(() => flattenTree(activeTree).slice(1), [activeTree])
  const selectedNode = rows.find(({ node }) => node.slug === selectedSlug)?.node
  const selectedIntent = canEdit ? intents.find((intent) => intent.slug === selectedSlug) : undefined
  const activeWorkspaceId = selectedNode?.workspaceId ?? liveIntent?.workspaceId ?? activeTree.workspaceId
  const selectedParent = selectedNode?.is_dir && !isWorkspaceRoot(selectedNode) ? selectedNode : undefined
  const canCreateUnderSelection = selectedNode === undefined || selectedNode.is_dir
  const canCreateIntent = canCreateUnderSelection && (canEdit || activeWorkspaceId !== undefined)
  const createTargetHighlighted = canCreateIntent && formMode === 'create'
  const selectedDependents = selectedIntent === undefined
    ? liveIntent?.dependents ?? []
    : intents.filter((intent) => intent.slug !== selectedIntent.slug && intent.dependsOn.includes(selectedIntent.slug)).map((intent) => intent.slug)

  const selectRelatedIntent = (slug: string): void => {
    if (!rows.some(({ node }) => node.slug === slug)) {
      setNotice(`“${slug}” is referenced here but is not present in the current intent tree.`)
      return
    }
    setSelectedSlug(slug)
    setFormMode(null)
    setShowAssumptionForm(false)
    setPendingDeleteSlug(null)
    setNotice(null)
    setAssumptionCheckError(null)
  }

  const requestTree = useCallback((refresh = false): void => {
    if (vscodeApi === null) {
      setTree(null)
      setNotice('Preview refreshed. Changes are stored only in this page.')
      return
    }

    setLoading(true)
    setError(null)
    vscodeApi.postMessage({ type: refresh ? 'kron_tree:refresh' : 'kron_tree:request' })
  }, [])

  useEffect(() => {
    if (vscodeApi === null) return

    const handler = (event: MessageEvent<unknown>): void => {
      const message = event.data
      if (!isWebviewInbound(message)) {
        if (typeof message === 'object' && message !== null && 'type' in message
          && typeof message.type === 'string') {
          if (message.type.startsWith('kron_tree:')) {
            setLoading(false)
            setError('The extension host returned an invalid Kron tree response. Refresh and try again.')
          } else if (message.type.startsWith('kron_mcp:')) {
            setDetailError('The extension host returned an invalid response. Refresh and try again.')
          }
        }
        return
      }

      if (message.type === 'kron_font_size') {
        const scale = message.fontSize / 14
        applyWebviewFontScale(scale)
        setUiScale(scale)
        return
      }

      if (message.type === 'kron_select') {
        setSelectedSlug(message.slug)
        setRevealToken((value) => value + 1)
        setFormMode(null)
        setShowAssumptionForm(false)
        setPendingDeleteSlug(null)
        setPendingEditSlug(message.edit ? message.slug : null)
        return
      }

      if (message.type === 'kron_tree:result') {
        setTree(message.result)
        setTreeRelations({})
        setLoading(false)
        setError(null)
        const entries = flattenTree(message.result.root)
        const intentNodes = entries.map(({ node }) => node).filter((node) => node.slug !== 'workspace'
          && (!node.is_dir || node.title !== undefined || node.status !== undefined))
        const relationSequence = ++treeRelationSequence.current
        void Promise.allSettled(intentNodes.map(async (node) => {
          const result = await sendMcpRequest('kron_get', { slug: node.slug, include_relations: true }, node.workspaceId)
          const intent = intentFromMcp(result, node)
          return [node.slug, { references: intent.references, dependsOn: intent.dependsOn, kind: intent.kind, exists: true }] as const
        })).then((settled) => {
          if (relationSequence !== treeRelationSequence.current) return
          const next: Record<string, IntentRelations> = {}
          for (const item of settled) {
            if (item.status === 'fulfilled') next[item.value[0]] = item.value[1]
          }
          setTreeRelations(next)
        })
        setSelectedSlug((current) => entries.some(({ node }) => node.slug === current)
          ? current
          : entries.find(({ node }) => !node.is_dir)?.node.slug ?? entries[0]?.node.slug ?? '')
      } else if (message.type === 'kron_tree:error') {
        setError(message.message)
        setLoading(false)
      } else if (message.type === 'kron_mcp:response' || message.type === 'kron_mcp:error') {
        const pending = pendingMcpRequests.current.get(message.requestId)
        if (pending === undefined) return
        clearTimeout(pending.timer)
        pendingMcpRequests.current.delete(message.requestId)
        if (message.type === 'kron_mcp:response') pending.resolve(message.result)
        else pending.reject(new Error(message.message))
      }
    }

    window.addEventListener('message', handler)
    requestTree()
    return () => {
      window.removeEventListener('message', handler)
      for (const pending of pendingMcpRequests.current.values()) {
        clearTimeout(pending.timer)
        pending.reject(new Error('The Kron Webview was closed.'))
      }
      pendingMcpRequests.current.clear()
    }
  }, [requestTree, sendMcpRequest])

  useEffect(() => {
    const selectedNodeIsIntent = selectedNode !== undefined
      && (!selectedNode.is_dir || treeRelations[selectedNode.slug]?.exists === true)
    if (vscodeApi === null || selectedNode === undefined || !selectedNodeIsIntent) {
      setLiveIntent(null)
      setDetailLoading(false)
      setDetailError(null)
      return
    }
    const sequence = ++detailRequestSequence.current
    setLiveIntent(null)
    setDetailLoading(true)
    setDetailError(null)
    void sendMcpRequest('kron_get', { slug: selectedNode.slug, include_relations: true }, selectedNode.workspaceId)
      .then((result) => {
        if (sequence !== detailRequestSequence.current) return
        const intent = intentFromMcp(result, selectedNode)
        setLiveIntent(intent)
        setAssumptions((current) => {
          const next = [...current]
          for (const linked of intent.assumptions) {
            const existingIndex = next.findIndex((item) => item.id === linked.id)
            const fromWire = isRecord(result) && isRecord(result.intent) && isRecord(result.intent.frontmatter)
              && Array.isArray(result.intent.frontmatter.assumptions)
              ? result.intent.frontmatter.assumptions.find((entry) => isRecord(entry) && entry.id === linked.id)
              : undefined
            const shared: Assumption = {
              id: linked.id,
              text: isRecord(fromWire) && typeof fromWire.text === 'string' ? fromWire.text : '',
              defaultSeverity: isRecord(fromWire) && fromWire.default_severity === 'soft' ? 'soft' : linked.severity,
            }
            if (existingIndex < 0) next.push(shared)
            else next[existingIndex] = { ...next[existingIndex], ...shared }
          }
          return next
        })
      })
      .catch((requestError: unknown) => {
        if (sequence === detailRequestSequence.current) {
          setDetailError(requestError instanceof Error ? requestError.message : 'Unable to load intent details.')
        }
      })
      .finally(() => {
        if (sequence === detailRequestSequence.current) setDetailLoading(false)
      })
    return () => {
      detailRequestSequence.current++
    }
  }, [selectedSlug, selectedNode?.is_dir, selectedNode?.workspaceId, treeRelations[selectedNode?.slug ?? '']?.exists, detailReload, sendMcpRequest])

  useEffect(() => {
    if (vscodeApi === null) return
    let active = true
    void sendMcpRequest('kron_assume_list', {}, selectedNode?.workspaceId).then((result) => {
      if (!active || !isRecord(result) || !Array.isArray(result.assumptions)) return
      const liveAssumptions = result.assumptions.flatMap((item): Assumption[] => {
        if (!isRecord(item) || typeof item.id !== 'string') return []
        return [{
          id: item.id,
          text: typeof item.text === 'string' ? item.text : '',
          defaultSeverity: item.default_severity === 'soft' ? 'soft' : 'hard',
        }]
      })
      setAssumptions(liveAssumptions)
    }).catch(() => {
      // The intent tree and details remain usable when the optional registry is unavailable.
    })
    return () => { active = false }
  }, [sendMcpRequest, selectedNode?.workspaceId])

  const startCreate = (kind: Intent['kind'] = 'leaf'): void => {
    const parentSlug = selectedParent?.slug
    setDraft(newDraft(parentSlug === undefined ? '' : `${parentSlug}/`, kind))
    setDraftParentSlug(parentSlug ?? null)
    setFormError(null)
    setEditParam(null)
    setFormMode('create')
    setPendingDeleteSlug(null)
    setShowAssumptionForm(false)
    setNotice(null)
  }

  const startCreateChild = (parent: TreeNode, kind: Intent['kind'] = 'leaf'): void => {
    setSelectedSlug(parent.slug)
    setDraft(newDraft(`${parent.slug}/`, kind))
    setDraftParentSlug(parent.slug)
    setFormError(null)
    setEditParam(null)
    setFormMode('create')
    setPendingDeleteSlug(null)
    setShowAssumptionForm(false)
    setNotice(null)
  }

  const startEdit = (): void => {
    const editableIntent = selectedIntent ?? liveIntent
    if (editableIntent === null || editableIntent === undefined) return
    setDraft(draftFromIntent(editableIntent))
    setDraftParentSlug(null)
    setFormError(null)
    setEditParam(null)
    setFormMode('edit')
    setPendingDeleteSlug(null)
    setShowAssumptionForm(false)
    setNotice(null)
  }

  // Host asked for "Edit Intent" from the editor anchor menu: enter edit mode
  // once the selected intent's details have loaded.
  useEffect(() => {
    if (pendingEditSlug === null) return
    if (liveIntent === null || liveIntent.slug !== pendingEditSlug) return
    setPendingEditSlug(null)
    startEdit()
  }, [pendingEditSlug, liveIntent])

  const createGraphRelation = async (source: TreeNode, target: TreeNode, kind: 'depends' | 'references'): Promise<void> => {
    const relationField = kind === 'depends' ? 'dependsOn' : 'references'
    if (canEdit) {
      const sourceIntent = intents.find((intent) => intent.slug === source.slug)
      if (sourceIntent === undefined) throw new Error('The source intent is no longer in the tree.')
      const current = sourceIntent[relationField]
      if (current.includes(target.slug)) throw new Error(`“${target.slug}” is already listed as ${kind === 'depends' ? 'a prerequisite' : 'a reference'}.`)
      setIntents((items) => items.map((intent) => intent.slug === source.slug
        ? { ...intent, [relationField]: [...intent[relationField], target.slug] }
        : intent))
      setTreeRelations((currentRelations) => ({
        ...currentRelations,
        [source.slug]: {
          references: relationField === 'references' ? [...current, target.slug] : currentRelations[source.slug]?.references ?? sourceIntent.references,
          dependsOn: relationField === 'dependsOn' ? [...current, target.slug] : currentRelations[source.slug]?.dependsOn ?? sourceIntent.dependsOn,
          kind: sourceIntent.kind,
          exists: true,
        },
      }))
      setNotice(`Added ${kind === 'depends' ? 'hard dependency' : 'reference'} from “${source.slug}” to “${target.slug}”.`)
      return
    }

    setSaving(true)
    try {
      // Read the source immediately before patching so duplicate detection uses
      // the canonical stored relations. kron_update has PATCH semantics; send
      // only the changed relation field and leave the body/other metadata intact.
      const result = await sendMcpRequest('kron_get', { slug: source.slug, include_relations: true }, source.workspaceId)
      const currentIntent = intentFromMcp(result, source)
      const current = currentIntent[relationField]
      if (current.includes(target.slug)) throw new Error(`“${target.slug}” is already listed as ${kind === 'depends' ? 'a prerequisite' : 'a reference'}.`)
      const next = [...current, target.slug]
      await sendMcpRequest('kron_update', {
        slug: source.slug,
        [kind === 'depends' ? 'depends_on' : 'references']: next,
      }, source.workspaceId)
      setNotice(`Saved ${kind === 'depends' ? 'hard dependency' : 'reference'} from “${source.slug}” to “${target.slug}”.`)
      if (selectedSlug === source.slug) setDetailReload((value) => value + 1)
      requestTree(true)
    } finally {
      setSaving(false)
    }
  }

  const saveIntent = async (event: FormEvent<HTMLFormElement>): Promise<void> => {
    event.preventDefault()
    if (formMode === null) return
    const slugPattern = /^[a-z0-9]+(?:-[a-z0-9]+)*(?:\/[a-z0-9]+(?:-[a-z0-9]+)*)*$/
    if (!slugPattern.test(draft.slug)) {
      setFormError('Use lowercase kebab-case segments, with optional / grouping.')
      return
    }
    if (draftParentSlug !== null) {
      const childSlug = draft.slug.slice(`${draftParentSlug}/`.length)
      if (!draft.slug.startsWith(`${draftParentSlug}/`) || !/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(childSlug)) {
        setFormError(`This child must have exactly one new slug segment under “${draftParentSlug}/”.`)
        return
      }
    }
    const summary = draft.summary.trim() || summaryFromBody(draft.body) || draft.title.trim()
    if (draft.title.trim() === '') {
      setFormError('Title is required.')
      return
    }
    if (formMode === 'create' && (canEdit
      ? intents.some((intent) => intent.slug === draft.slug)
      : rows.some(({ node }) => node.slug === draft.slug && (!node.is_dir || node.title !== undefined || node.status !== undefined)))) {
      setFormError('An intent with this slug already exists.')
      return
    }

    const previous = selectedIntent ?? liveIntent ?? undefined
    const next: Intent = {
      slug: draft.slug,
      kind: formMode === 'create' ? draft.kind : (previous?.kind ?? 'leaf'),
      title: draft.title.trim(),
      createdBy: previous?.createdBy ?? 'agent:preview',
      summary,
      symbol: draft.symbol.trim(),
      status: draft.status,
      reviewers: splitLines(draft.reviewersText),
      body: draft.body,
      references: splitLines(draft.referencesText),
      dependsOn: splitLines(draft.dependsOnText),
      assumptions: formMode === 'edit' && previous !== undefined ? previous.assumptions : [],
      ...(activeWorkspaceId === undefined ? {} : { workspaceId: activeWorkspaceId }),
    }

    if (canEdit) {
      setIntents((current) => formMode === 'edit'
        ? current.map((intent) => intent.slug === selectedSlug ? next : intent)
        : [...current, next])
      setSelectedSlug(next.slug)
      setFormMode(null)
      setDraftParentSlug(null)
      setFormError(null)
      setNotice(`Preview ${formMode === 'create' ? 'created' : 'updated'} “${next.title}”. No workspace files were written.`)
      return
    }

    if (activeWorkspaceId === undefined) {
      setFormError('Select a workspace in the intent tree before creating an intent.')
      return
    }
    const symbolList = splitLines(draft.symbol)
    const body = formMode === 'create'
      ? createIntentBody(draft.title, draft.summary, draft.body)
      : replaceIntentHeading(draft.body, draft.title)
    setSaving(true)
    setFormError(null)
    let created = false
    try {
      if (formMode === 'create') {
        await sendMcpRequest('kron_add', {
          slug: draft.slug,
          kind: draft.kind,
          ...(symbolList[0] === undefined ? {} : { symbol: symbolList[0] }),
          why: summary,
        }, activeWorkspaceId)
        created = true
      }
      await sendMcpRequest('kron_update', {
        slug: draft.slug,
        symbol: symbolList,
        status: draft.status,
        reviewers: splitLines(draft.reviewersText),
        body,
        references: splitLines(draft.referencesText),
        depends_on: splitLines(draft.dependsOnText),
      }, activeWorkspaceId)
      setSelectedSlug(draft.slug)
      setFormMode(null)
      setFormError(null)
      setNotice(`Saved “${draft.slug}” to the workspace.`)
      setDraftParentSlug(null)
      setDetailReload((value) => value + 1)
      requestTree(true)
    } catch (requestError: unknown) {
      const message = requestError instanceof Error ? requestError.message : 'Kron could not save this intent.'
      setFormError(created
        ? `The intent was created, but its remaining fields were not saved. ${message} Refresh the tree, then edit the new intent.`
        : message)
      if (created) {
        setFormMode(null)
        setSelectedSlug(draft.slug)
        requestTree(true)
      }
    } finally {
      setSaving(false)
    }
  }

  const attachAssumption = async (event: FormEvent<HTMLFormElement>): Promise<void> => {
    event.preventDefault()
    const currentIntent = selectedIntent ?? liveIntent
    if (currentIntent === null || currentIntent === undefined) return
    if (!/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(assumptionDraft.id) || assumptionDraft.text.trim() === '') {
      setAssumptionFormError('Use a valid assumption ID and enter its text.')
      return
    }
    if (assumptionDraft.rationale.trim().length < 10) {
      setAssumptionFormError('Explain this intent’s severity in at least 10 characters.')
      return
    }
    if (assumptions.some((assumption) => assumption.id === assumptionDraft.id)) {
      setAssumptionFormError('An assumption with this ID already exists. Pick it under “Use existing”.')
      return
    }

    const assumption: Assumption = {
      id: assumptionDraft.id,
      text: assumptionDraft.text.trim(),
      defaultSeverity: assumptionDraft.defaultSeverity,
      ...(canEdit ? { usedBy: [currentIntent.slug] } : {}),
    }
    if (!canEdit) {
      if (currentIntent.workspaceId === undefined) {
        setAssumptionFormError('The selected intent is not associated with a workspace.')
        return
      }
      const link: IntentAssumption = {
        id: assumption.id,
        severity: assumptionDraft.severity,
        rationale: assumptionDraft.rationale.trim(),
        ...(assumptionDraft.expiresAt ? { expiresAt: assumptionDraft.expiresAt } : {}),
      }
      setSaving(true)
      setAssumptionFormError(null)
      let created = false
      try {
        await sendMcpRequest('kron_assume_create', {
          id: assumption.id,
          text: assumption.text,
          default_severity: assumption.defaultSeverity,
        }, currentIntent.workspaceId)
        created = true
        await sendMcpRequest('kron_update', {
          slug: currentIntent.slug,
          assumptions: assumptionsToMcp([...currentIntent.assumptions, link]),
        }, currentIntent.workspaceId)
        setAssumptions((current) => [...current.filter((item) => item.id !== assumption.id), assumption])
        setAssumptionDraft({ id: '', text: '', defaultSeverity: 'hard', severity: 'hard', rationale: '', expiresAt: '' })
        setShowAssumptionForm(false)
        setNotice(`Created and attached assumption “${assumption.id}”.`)
        setDetailReload((value) => value + 1)
      } catch (requestError: unknown) {
        const message = requestError instanceof Error ? requestError.message : 'Kron could not save this assumption.'
        setAssumptionFormError(created ? `The shared assumption was created but not attached. ${message}` : message)
      } finally {
        setSaving(false)
      }
      return
    }
    setAssumptions((current) => [...current, assumption])
    setIntents((current) => current.map((intent) => intent.slug === selectedSlug
      ? {
          ...intent,
          assumptions: [...intent.assumptions, {
            id: assumption.id,
            severity: assumptionDraft.severity,
            rationale: assumptionDraft.rationale.trim(),
            ...(assumptionDraft.expiresAt ? { expiresAt: assumptionDraft.expiresAt } : {}),
          }],
        }
      : intent))
    setAssumptionDraft({ id: '', text: '', defaultSeverity: 'hard', severity: 'hard', rationale: '', expiresAt: '' })
    setShowAssumptionForm(false)
    setAssumptionFormError(null)
    setNotice(`Preview assumption “${assumption.id}” created and attached.`)
  }

  const attachExistingAssumption = async (event: FormEvent<HTMLFormElement>): Promise<void> => {
    event.preventDefault()
    const currentIntent = selectedIntent ?? liveIntent
    if (currentIntent === null || currentIntent === undefined) return
    const shared = assumptions.find((assumption) => assumption.id === selectedAssumptionId)
    if (shared === undefined) {
      setAssumptionFormError('Choose an existing assumption.')
      return
    }
    if (currentIntent.assumptions.some((item) => item.id === shared.id)) {
      setAssumptionFormError('This assumption is already linked to the selected intent.')
      return
    }
    if (assumptionDraft.rationale.trim().length < 10) {
      setAssumptionFormError('Explain this intent’s severity in at least 10 characters.')
      return
    }

    const link: IntentAssumption = {
      id: shared.id,
      severity: assumptionDraft.severity,
      rationale: assumptionDraft.rationale.trim(),
      ...(assumptionDraft.expiresAt ? { expiresAt: assumptionDraft.expiresAt } : {}),
    }
    if (!canEdit) {
      if (currentIntent.workspaceId === undefined) {
        setAssumptionFormError('The selected intent is not associated with a workspace.')
        return
      }
      setSaving(true)
      setAssumptionFormError(null)
      try {
        await sendMcpRequest('kron_update', {
          slug: currentIntent.slug,
          assumptions: assumptionsToMcp([...currentIntent.assumptions, link]),
        }, currentIntent.workspaceId)
        setAssumptionDraft({ id: '', text: '', defaultSeverity: 'hard', severity: 'hard', rationale: '', expiresAt: '' })
        setShowAssumptionForm(false)
        setNotice(`Attached assumption “${shared.id}”.`)
        setDetailReload((value) => value + 1)
      } catch (requestError: unknown) {
        setAssumptionFormError(requestError instanceof Error ? requestError.message : 'Kron could not attach this assumption.')
      } finally {
        setSaving(false)
      }
      return
    }

    setIntents((current) => current.map((intent) => intent.slug === selectedSlug
      ? {
          ...intent,
          assumptions: [...intent.assumptions, link],
        }
      : intent))
    setAssumptions((current) => current.map((assumption) => assumption.id === shared.id
      ? { ...assumption, usedBy: [...(assumption.usedBy ?? []), selectedSlug] }
      : assumption))
    setAssumptionDraft({ id: '', text: '', defaultSeverity: 'hard', severity: 'hard', rationale: '', expiresAt: '' })
    setShowAssumptionForm(false)
    setAssumptionFormError(null)
    setNotice(`Preview assumption “${shared.id}” attached to this intent.`)
  }

  const detachAssumption = async (id: string): Promise<void> => {
    const currentIntent = selectedIntent ?? liveIntent
    if (currentIntent === null || currentIntent === undefined) return
    if (!canEdit) {
      if (currentIntent.workspaceId === undefined) return
      setSaving(true)
      try {
        await sendMcpRequest('kron_update', {
          slug: currentIntent.slug,
          assumptions: assumptionsToMcp(currentIntent.assumptions.filter((item) => item.id !== id)),
        }, currentIntent.workspaceId)
        setNotice(`Unlinked assumption “${id}”. The shared assumption file was kept.`)
        setDetailReload((value) => value + 1)
      } catch (requestError: unknown) {
        setDetailError(requestError instanceof Error ? requestError.message : 'Kron could not unlink this assumption.')
      } finally {
        setSaving(false)
      }
      return
    }
    setIntents((current) => current.map((intent) => intent.slug === selectedSlug
      ? { ...intent, assumptions: intent.assumptions.filter((item) => item.id !== id) }
      : intent))
    setAssumptions((current) => current.map((assumption) => assumption.id === id
      ? { ...assumption, usedBy: (assumption.usedBy ?? []).filter((slug) => slug !== selectedSlug) }
      : assumption))
    setNotice(`Preview association to “${id}” removed. The shared assumption was kept.`)
  }

  const checkAssumptions = async (): Promise<void> => {
    const currentIntent = selectedIntent ?? liveIntent
    if (currentIntent?.workspaceId === undefined) {
      setAssumptionCheckError('Select a live workspace intent before checking assumptions.')
      return
    }
    setSaving(true)
    setAssumptionCheckError(null)
    try {
      const result = await sendMcpRequest('kron_assume_check', { file_path: '' }, currentIntent.workspaceId)
      if (!isRecord(result) || !isRecord(result.summary) || !Array.isArray(result.warnings)) throw new Error('Kron returned an invalid assumption check result.')
      const summary = result.summary
      const warnings = result.warnings.flatMap((item): AssumptionCheck['warnings'] => {
        if (!isRecord(item) || typeof item.intent_slug !== 'string' || typeof item.assumption_id !== 'string' || typeof item.text !== 'string' || typeof item.severity !== 'string') return []
        return [{ intentSlug: item.intent_slug, assumptionId: item.assumption_id, text: item.text, severity: item.severity, verified: item.verified === true }]
      })
      setAssumptionCheck({
        hard: typeof summary.hard === 'number' ? summary.hard : 0,
        soft: typeof summary.soft === 'number' ? summary.soft : 0,
        expired: typeof summary.expired === 'number' ? summary.expired : 0,
        warnings,
      })
      setNotice('Assumption check completed.')
    } catch (requestError: unknown) {
      setAssumptionCheckError(requestError instanceof Error ? requestError.message : 'Kron could not check assumptions.')
    } finally {
      setSaving(false)
    }
  }

  const archiveIntent = async (): Promise<void> => {
    const archived = selectedIntent ?? liveIntent
    if (archived === null || archived === undefined || pendingDeleteSlug !== archived.slug) return
    if (!canEdit) {
      setSaving(true)
      try {
        const result = await sendMcpRequest('kron_delete', {
          slug: archived.slug,
          // Node commits require an explicit acknowledgement of the cascade.
          ...(archived.kind === 'node' ? { confirm_subtree: true } : {}),
        }, archived.workspaceId)
        const deletedSlugs = isRecord(result) && Array.isArray(result.deleted_slugs)
          ? result.deleted_slugs.filter((slug): slug is string => typeof slug === 'string')
          : [archived.slug]
        setArchivedIntents((current) => [archived, ...current])
        setSelectedSlug('')
        setPendingDeleteSlug(null)
        setShowAssumptionForm(false)
        setNotice(deletedSlugs.length > 1
          ? `Moved "${archived.slug}" and ${deletedSlugs.length - 1} intent(s) in its subtree to Kron trash. Restore is available during this Webview session.`
          : `Moved "${archived.slug}" to Kron trash. Restore is available during this Webview session.`)
        setDetailReload((value) => value + 1)
        requestTree(true)
      } catch (requestError: unknown) {
        setDetailError(requestError instanceof Error ? requestError.message : 'Kron could not delete this intent.')
      } finally {
        setSaving(false)
      }
      return
    }
    // Mock preview: drop the node's whole subtree from the in-memory tree.
    const subtreePrefix = `${archived.slug}/`
    setArchivedIntents((current) => [archived, ...current])
    setIntents((current) => current.filter((intent) => intent.slug !== archived.slug && !intent.slug.startsWith(subtreePrefix)))
    setAssumptions((current) => current.map((assumption) => ({
      ...assumption,
      usedBy: (assumption.usedBy ?? []).filter((slug) => slug !== archived.slug),
    })))
    const nextSlug = intents.find((intent) => intent.slug !== archived.slug)?.slug ?? ''
    setSelectedSlug(nextSlug)
    setPendingDeleteSlug(null)
    setShowAssumptionForm(false)
    setNotice(`Preview archived "${archived.slug}". It can be restored from the Archived intents list.`)
  }

  const restoreIntent = async (slug: string): Promise<void> => {
    const archived = archivedIntents.find((intent) => intent.slug === slug)
    if (archived === undefined) return
    if (canEdit ? intents.some((intent) => intent.slug === slug) : rows.some(({ node }) => node.slug === slug)) {
      setNotice(`Cannot restore "${slug}" while an active intent with the same slug exists.`)
      return
    }
    if (!canEdit) {
      setSaving(true)
      try {
        const result = await sendMcpRequest('kron_restore', { slug }, archived.workspaceId)
        const restoredSlugs = isRecord(result) && Array.isArray(result.restored_slugs)
          ? result.restored_slugs.filter((entry): entry is string => typeof entry === 'string')
          : [slug]
        setArchivedIntents((current) => current.filter((intent) => intent.slug !== slug))
        setSelectedSlug(slug)
        setNotice(restoredSlugs.length > 1
          ? `Restored "${slug}" and ${restoredSlugs.length - 1} intent(s) in its subtree from Kron trash.`
          : `Restored "${slug}" from Kron trash.`)
        setDetailReload((value) => value + 1)
        setShowArchivePanel(false)
        requestTree(true)
      } catch (requestError: unknown) {
        setDetailError(requestError instanceof Error ? requestError.message : 'Kron could not restore this intent.')
      } finally {
        setSaving(false)
      }
      return
    }
    setIntents((current) => [...current, archived])
    setArchivedIntents((current) => current.filter((intent) => intent.slug !== slug))
    const linkedIds = new Set(archived.assumptions.map((item) => item.id))
    setAssumptions((current) => current.map((assumption) => linkedIds.has(assumption.id)
      ? { ...assumption, usedBy: [...new Set([...(assumption.usedBy ?? []), slug])] }
      : assumption))
    setSelectedSlug(slug)
    setNotice(`Preview restored "${slug}". No workspace files were changed.`)
    setShowArchivePanel(false)
  }

  const toggleAssumptionForm = (): void => {
    const opening = !showAssumptionForm
    setShowAssumptionForm(opening)
    setAssumptionFormError(null)
    if (opening) {
      setAssumptionMode(availableAssumptions.length > 0 ? 'existing' : 'new')
      setSelectedAssumptionId(availableAssumptions[0]?.id ?? '')
    }
  }

  // ---------------------------------------------------------------------
  // Assumption registry panel (global "Assumptions" button)
  // ---------------------------------------------------------------------

  const assumptionPanelWorkspaceId = activeWorkspaceId

  const selectPanelAssumption = (assumption: Assumption): void => {
    setAssumptionPanelId(assumption.id)
    setAssumptionPanelCreating(false)
    setAssumptionPanelPendingDelete(null)
    setAssumptionPanelError(null)
    const expiresAtRaw = assumption.expiresAt ?? ''
    setAssumptionPanelDraft({
      id: assumption.id,
      text: assumption.text,
      defaultSeverity: assumption.defaultSeverity,
      status: assumption.status === 'draft' || assumption.status === 'superseded' ? assumption.status : 'active',
      // <input type="date"> only accepts YYYY-MM-DD; keep the raw value for saving.
      expiresAt: expiresAtRaw.slice(0, 10),
      expiresAtRaw,
      reviewersText: (assumption.reviewers ?? []).join(', '),
    })
  }

  const startPanelCreate = (): void => {
    setAssumptionPanelCreating(true)
    setAssumptionPanelId(null)
    setAssumptionPanelPendingDelete(null)
    setAssumptionPanelError(null)
    setAssumptionPanelDraft(emptyAssumptionPanelDraft())
  }

  const openAssumptionPanel = (): void => {
    setShowAssumptionPanel(true)
    setAssumptionPanelError(null)
    setAssumptionPanelPendingDelete(null)
    setAssumptionPanelCreating(false)
  }

  const refreshAssumptionRegistry = useCallback(async (workspaceId: string | undefined): Promise<Assumption[]> => {
    if (vscodeApi === null) {
      setAssumptionUsage({})
      return []
    }
    const listResult = await sendMcpRequest('kron_assume_list', {}, workspaceId)
    if (!isRecord(listResult) || !Array.isArray(listResult.assumptions)) {
      throw new Error('Kron returned an invalid assumption list.')
    }
    const next: Assumption[] = listResult.assumptions.flatMap((item): Assumption[] => {
      if (!isRecord(item) || typeof item.id !== 'string') return []
      return [{
        id: item.id,
        text: typeof item.text === 'string' ? item.text : '',
        defaultSeverity: item.default_severity === 'soft' ? 'soft' : 'hard',
        ...(typeof item.status === 'string' ? { status: item.status } : {}),
        ...(typeof item.created_by === 'string' && item.created_by !== '' ? { createdBy: item.created_by } : {}),
        ...(typeof item.updated_at === 'string' && item.updated_at !== '' ? { updatedAt: item.updated_at } : {}),
        ...(typeof item.expires_at === 'string' && item.expires_at !== '' ? { expiresAt: item.expires_at } : {}),
        ...(typeof item.verified_at === 'string' && item.verified_at !== '' ? { verifiedAt: item.verified_at } : {}),
        ...(typeof item.verified_by === 'string' && item.verified_by !== '' ? { verifiedBy: item.verified_by } : {}),
        ...(Array.isArray(item.reviewers) ? { reviewers: stringArray(item.reviewers) } : {}),
      }]
    })
    const usage: AssumptionUsage = {}
    try {
      const checkResult = await sendMcpRequest('kron_assume_check', { file_path: '' }, workspaceId)
      if (isRecord(checkResult) && Array.isArray(checkResult.warnings)) {
        for (const warning of checkResult.warnings) {
          if (!isRecord(warning) || typeof warning.intent_slug !== 'string'
            || typeof warning.assumption_id !== 'string' || typeof warning.severity !== 'string') continue
          const entry: AssumptionUsageEntry = {
            intentSlug: warning.intent_slug,
            severity: warning.severity === 'soft' ? 'soft' : 'hard',
            rationale: typeof warning.rationale === 'string' ? warning.rationale : '',
          }
          usage[warning.assumption_id] = [...(usage[warning.assumption_id] ?? []), entry]
        }
      }
    } catch {
      // Usage is best-effort; the registry list is still useful without it.
    }
    setAssumptions(next)
    setAssumptionUsage(usage)
    setAssumptionPanelId((current) => current !== null && next.some((item) => item.id === current) ? current : null)
    return next
  }, [sendMcpRequest])

  const createPanelAssumption = async (event: FormEvent<HTMLFormElement>): Promise<void> => {
    event.preventDefault()
    const id = assumptionPanelDraft.id.trim()
    const text = assumptionPanelDraft.text.trim()
    if (!/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(id)) {
      setAssumptionPanelError('Use a lowercase kebab-case assumption ID.')
      return
    }
    if (text === '') {
      setAssumptionPanelError('Enter the assumption text.')
      return
    }
    if (assumptions.some((item) => item.id === id)) {
      setAssumptionPanelError('An assumption with this ID already exists.')
      return
    }
    setAssumptionPanelSaving(true)
    setAssumptionPanelError(null)
    try {
      if (!canEdit) {
        if (assumptionPanelWorkspaceId === undefined) throw new Error('Select a project before editing assumptions.')
        await sendMcpRequest('kron_assume_create', {
          id,
          text,
          default_severity: assumptionPanelDraft.defaultSeverity,
        }, assumptionPanelWorkspaceId)
        const next = await refreshAssumptionRegistry(assumptionPanelWorkspaceId)
        const created = next.find((item) => item.id === id)
        setAssumptionPanelCreating(false)
        if (created !== undefined) selectPanelAssumption(created)
      } else {
        setAssumptions((current) => [...current, { id, text, defaultSeverity: assumptionPanelDraft.defaultSeverity, status: 'active' }])
        setAssumptionPanelCreating(false)
        setAssumptionPanelId(id)
      }
      setNotice(`Created assumption “${id}”.`)
    } catch (error: unknown) {
      setAssumptionPanelError(error instanceof Error ? error.message : 'Kron could not create this assumption.')
    } finally {
      setAssumptionPanelSaving(false)
    }
  }

  const savePanelAssumption = async (event: FormEvent<HTMLFormElement>): Promise<void> => {
    event.preventDefault()
    const id = assumptionPanelId
    if (id === null) return
    const text = assumptionPanelDraft.text.trim()
    if (text === '') {
      setAssumptionPanelError('Enter the assumption text.')
      return
    }
    // Keep the original timestamp precision unless the user changed the date.
    const rawExpires = assumptionPanelDraft.expiresAtRaw
    const expiresToSend = assumptionPanelDraft.expiresAt.trim() === rawExpires.slice(0, 10).trim()
      ? rawExpires
      : assumptionPanelDraft.expiresAt.trim()
    setAssumptionPanelSaving(true)
    setAssumptionPanelError(null)
    try {
      if (!canEdit) {
        if (assumptionPanelWorkspaceId === undefined) throw new Error('Select a project before editing assumptions.')
        await sendMcpRequest('kron_assume_update', {
          id,
          text,
          default_severity: assumptionPanelDraft.defaultSeverity,
          status: assumptionPanelDraft.status,
          expires_at: expiresToSend,
          reviewers: splitLines(assumptionPanelDraft.reviewersText),
        }, assumptionPanelWorkspaceId)
        const next = await refreshAssumptionRegistry(assumptionPanelWorkspaceId)
        const updated = next.find((item) => item.id === id)
        if (updated !== undefined) selectPanelAssumption(updated)
      } else {
        setAssumptions((current) => current.map((item) => item.id === id
          ? {
              ...item,
              text,
              defaultSeverity: assumptionPanelDraft.defaultSeverity,
              status: assumptionPanelDraft.status,
              ...(expiresToSend === '' ? {} : { expiresAt: expiresToSend }),
            }
          : item))
      }
      setNotice(`Updated assumption “${id}”.`)
    } catch (error: unknown) {
      setAssumptionPanelError(error instanceof Error ? error.message : 'Kron could not save this assumption.')
    } finally {
      setAssumptionPanelSaving(false)
    }
  }

  const deletePanelAssumption = async (): Promise<void> => {
    const id = assumptionPanelPendingDelete
    if (id === null) return
    setAssumptionPanelSaving(true)
    setAssumptionPanelError(null)
    try {
      if (!canEdit) {
        if (assumptionPanelWorkspaceId === undefined) throw new Error('Select a project before editing assumptions.')
        await sendMcpRequest('kron_assume_delete', { id }, assumptionPanelWorkspaceId)
        await refreshAssumptionRegistry(assumptionPanelWorkspaceId)
      } else {
        setAssumptions((current) => current.filter((item) => item.id !== id))
      }
      setAssumptionPanelPendingDelete(null)
      setAssumptionPanelId(null)
      setNotice(`Moved assumption “${id}” to the Kron trash. Intents that still reference it need to be detached.`)
    } catch (error: unknown) {
      setAssumptionPanelError(error instanceof Error ? error.message : 'Kron could not delete this assumption.')
    } finally {
      setAssumptionPanelSaving(false)
    }
  }

  useEffect(() => {
    if (!showAssumptionPanel) return
    if (canEdit) {
      // Standalone preview: derive usage from the in-memory intents.
      const usage: AssumptionUsage = {}
      for (const intent of intents) {
        for (const link of intent.assumptions) {
          usage[link.id] = [...(usage[link.id] ?? []), { intentSlug: intent.slug, severity: link.severity, rationale: link.rationale }]
        }
      }
      setAssumptionUsage(usage)
      return
    }
    if (assumptionPanelWorkspaceId === undefined) {
      setAssumptionPanelError('Select a project in the intent tree before opening the assumption registry.')
      return
    }
    void refreshAssumptionRegistry(assumptionPanelWorkspaceId).catch((error: unknown) => {
      setAssumptionPanelError(error instanceof Error ? error.message : 'Kron could not load the assumption registry.')
    })
  }, [showAssumptionPanel, canEdit, assumptionPanelWorkspaceId, intents, refreshAssumptionRegistry])

  const activeDetail: Intent | null = selectedIntent ?? liveIntent ?? (selectedNode !== undefined
    ? {
        slug: selectedNode.slug,
        kind: selectedNode.is_dir ? 'node' : 'leaf',
        title: selectedNode.title ?? selectedNode.name,
        createdBy: '',
        summary: 'Select an intent to load its details.',
        symbol: '',
        status: (selectedNode.status === 'draft' || selectedNode.status === 'superseded' ? selectedNode.status : 'active'),
        reviewers: [],
        body: '',
        references: [],
        dependsOn: [],
        assumptions: [],
      }
    : null)

  const assumptionTarget = selectedIntent ?? liveIntent
  const linkedAssumptions = assumptionTarget?.assumptions ?? []

  // Node deletes cascade to the whole subtree; the confirm dialog must say so.
  const deleteSubtreeCount = activeDetail !== null && activeDetail.kind === 'node'
    ? intents.filter((intent) => intent.slug === activeDetail.slug || intent.slug.startsWith(`${activeDetail.slug}/`)).length
    : 1

  /**
   * The intent's own `assumptions[]` is the source of truth for what is linked;
   * the registry only fills in the shared text. A failed/empty registry load
   * must not hide links that the intent actually has.
   */
  const sharedAssumptions = linkedAssumptions.map((link) => {
    const registry = assumptions.find((item) => item.id === link.id)
    return {
      id: link.id,
      severity: link.severity,
      rationale: link.rationale,
      text: registry?.text ?? '',
      defaultSeverity: registry?.defaultSeverity ?? link.severity,
    }
  })
  const availableAssumptions = assumptions.filter((assumption) => !linkedAssumptions.some((item) => item.id === assumption.id))

  const assumptionContent = assumptionTarget === null ? null : (
    <section className="rounded-md border">
      <div className="flex items-center justify-between gap-2 border-b px-2 py-1">
        <span className="flex items-center gap-1.5 text-[0.6875rem] font-medium">
          Assumptions
          {sharedAssumptions.length > 0 ? <span className="rounded-full bg-muted px-1.5 text-[0.625rem] text-muted-foreground">{sharedAssumptions.length}</span> : null}
        </span>
        <div className="flex items-center gap-1">
          {!canEdit ? <Button type="button" size="sm" variant="ghost" className="h-6 px-2 text-[0.6875rem]" title="Check every assumption in this workspace" onClick={() => void checkAssumptions()} disabled={saving}>Check</Button> : null}
          <Button type="button" size="sm" variant="outline" className="h-6 px-2 text-[0.6875rem]" onClick={toggleAssumptionForm} disabled={saving}>{showAssumptionForm ? 'Close' : 'Add'}</Button>
        </div>
      </div>
      {sharedAssumptions.length > 0 ? (
        <div className="divide-y">
          {sharedAssumptions.map((assumption) => (
            <details key={assumption.id}>
              <summary className="flex cursor-pointer list-none items-center gap-1.5 px-2 py-1 text-[0.6875rem] hover:bg-accent/50" title={assumption.text}>
                <span className={`h-1.5 w-1.5 shrink-0 rounded-full ${assumption.severity === 'soft' ? 'bg-muted-foreground' : 'bg-destructive'}`} />
                <span className="truncate font-mono">{assumption.id}</span>
                <span className="ml-auto shrink-0 text-muted-foreground">{assumption.severity}</span>
              </summary>
              <div className="space-y-1.5 px-2 pb-2 text-xs">
                <p className="text-muted-foreground">{assumption.text || 'Shared text not found in the assumption registry.'}</p>
                {assumption.rationale ? <p className="text-[0.6875rem] text-muted-foreground">Why: {assumption.rationale}</p> : null}
                <Button type="button" size="sm" variant="ghost" className="h-6 px-2 text-[0.6875rem]" onClick={() => void detachAssumption(assumption.id)} disabled={saving}>Remove</Button>
              </div>
            </details>
          ))}
        </div>
      ) : <div className="px-2 py-1.5 text-[0.6875rem] text-muted-foreground">No assumptions linked.</div>}
      {assumptionCheck !== null ? (
        <div className="border-t px-2 py-1.5 text-[0.6875rem]">
          <span className="text-muted-foreground">workspace: {assumptionCheck.hard} hard · {assumptionCheck.soft} soft · {assumptionCheck.expired} expired</span>
          {assumptionCheck.warnings.length > 0 ? <div className="mt-1 space-y-0.5">{assumptionCheck.warnings.slice(0, 5).map((warning) => <div key={`${warning.intentSlug}:${warning.assumptionId}`} className="text-muted-foreground">{warning.intentSlug} · {warning.assumptionId}: {warning.text}{warning.verified ? ' (verified)' : ''}</div>)}</div> : null}
        </div>
      ) : null}
      {assumptionCheckError !== null ? <div role="alert" className="border-t px-2 py-1.5 text-[0.6875rem] text-destructive">{assumptionCheckError}</div> : null}
      {assumptionFormError !== null ? <div role="alert" className="border-t px-2 py-1.5 text-[0.6875rem] text-destructive">{assumptionFormError}</div> : null}
      {showAssumptionForm ? (
        <div className="border-t p-2">
          {availableAssumptions.length > 0 ? (
            <div className="mb-3 flex gap-1 rounded-md bg-muted p-1">
              <Button type="button" size="sm" variant={assumptionMode === 'existing' ? 'secondary' : 'ghost'} onClick={() => setAssumptionMode('existing')}>Use existing</Button>
              <Button type="button" size="sm" variant={assumptionMode === 'new' ? 'secondary' : 'ghost'} onClick={() => setAssumptionMode('new')}>Create new</Button>
            </div>
          ) : null}
          {assumptionMode === 'existing' && availableAssumptions.length > 0 ? (
            <form className="space-y-3" onSubmit={attachExistingAssumption}>
              <Field label="Assumption">
                <select className={fieldClass()} value={selectedAssumptionId} onChange={(event) => setSelectedAssumptionId(event.target.value)}>
                  {availableAssumptions.map((assumption) => <option key={assumption.id} value={assumption.id}>{assumption.id} — {assumption.text}</option>)}
                </select>
              </Field>
              <Field label="Severity">
                <select className={fieldClass()} value={assumptionDraft.severity} onChange={(event) => setAssumptionDraft({ ...assumptionDraft, severity: event.target.value === 'soft' ? 'soft' : 'hard' })}>
                  <option value="hard">hard</option><option value="soft">soft</option>
                </select>
              </Field>
              <Field label="Rationale" hint="At least 10 characters. Why does this intent use this severity?">
                <textarea className={`${fieldClass()} min-h-16 resize-y`} value={assumptionDraft.rationale} onChange={(event) => setAssumptionDraft({ ...assumptionDraft, rationale: event.target.value })} />
              </Field>
              <Button type="submit" size="sm" disabled={saving}>Attach assumption</Button>
            </form>
          ) : (
            <form className="space-y-3" onSubmit={attachAssumption}>
              <Field label="Assumption ID" hint="Lowercase kebab-case.">
                <Input value={assumptionDraft.id} placeholder="single-region" onChange={(event) => setAssumptionDraft({ ...assumptionDraft, id: event.target.value })} />
              </Field>
              <Field label="Claim">
                <textarea className={`${fieldClass()} min-h-16 resize-y`} value={assumptionDraft.text} placeholder="Production traffic is served from one region." onChange={(event) => setAssumptionDraft({ ...assumptionDraft, text: event.target.value })} />
              </Field>
              <div className="grid gap-3 md:grid-cols-2">
                <Field label="Default severity">
                  <select className={fieldClass()} value={assumptionDraft.defaultSeverity} onChange={(event) => setAssumptionDraft({ ...assumptionDraft, defaultSeverity: event.target.value === 'soft' ? 'soft' : 'hard' })}>
                    <option value="hard">hard</option><option value="soft">soft</option>
                  </select>
                </Field>
                <Field label="Intent severity">
                  <select className={fieldClass()} value={assumptionDraft.severity} onChange={(event) => setAssumptionDraft({ ...assumptionDraft, severity: event.target.value === 'soft' ? 'soft' : 'hard' })}>
                    <option value="hard">hard</option><option value="soft">soft</option>
                  </select>
                </Field>
              </div>
              <Field label="Rationale" hint="At least 10 characters.">
                <textarea className={`${fieldClass()} min-h-16 resize-y`} value={assumptionDraft.rationale} onChange={(event) => setAssumptionDraft({ ...assumptionDraft, rationale: event.target.value })} />
              </Field>
              <Field label="Expires at" hint="Optional RFC3339 date or leave blank.">
                <Input type="date" value={assumptionDraft.expiresAt} onChange={(event) => setAssumptionDraft({ ...assumptionDraft, expiresAt: event.target.value })} />
              </Field>
              <Button type="submit" size="sm" disabled={saving}>Create and attach</Button>
            </form>
          )}
        </div>
      ) : null}
    </section>
  )

  const editorContent = formMode === null ? null : (
    <form className="grid gap-3" onSubmit={saveIntent}>
      <div className="flex items-start justify-between gap-2 pr-5">
        <span className="text-[0.6875rem] uppercase tracking-wide text-muted-foreground">
          {formMode === 'create' ? (draft.kind === 'node' ? 'New intent folder' : 'New intent document') : 'Edit intent'}
        </span>
        <span className="truncate font-mono text-[0.625rem] text-muted-foreground">{draft.slug || '…'}</span>
      </div>
      {formMode === 'create' ? (
        <Field label="Intent slug" hint={draftParentSlug === null ? 'Lowercase kebab-case; use / for a group.' : `Child of “${draftParentSlug}” is locked; enter one child name after the slash.`}>
          <Input value={draft.slug} placeholder="auth/session-renewal" onChange={(event) => {
            const value = event.target.value
            const prefix = draftParentSlug === null ? '' : `${draftParentSlug}/`
            setDraft({ ...draft, slug: prefix !== '' && !value.startsWith(prefix) ? `${prefix}${value.replace(/^\/+/, '')}` : value })
          }} />
        </Field>
      ) : null}
      <Field label="Title">
        <Input className="h-10 text-lg font-semibold" value={draft.title} placeholder="Session renewal" onChange={(event) => setDraft({ ...draft, title: event.target.value })} />
      </Field>
      {formMode === 'edit' ? (
        <>
          <Field label="Summary" hint={canEdit ? 'Shown in the tree and detail header.' : 'Derived from the first paragraph of the Markdown body.'}>
            <textarea className={`${fieldClass()} min-h-14 resize-y`} value={draft.summary} readOnly={!canEdit} placeholder="What decision does this intent record?" onChange={(event) => setDraft({ ...draft, summary: event.target.value })} />
          </Field>
          <div className="flex max-w-full flex-wrap items-center gap-1">
            <button
              type="button"
              className={`flex shrink-0 items-center gap-1 ${paramChipClass(editParam === 'status')}`}
              aria-label="Status"
              aria-expanded={editParam === 'status'}
              title={`Status: ${draft.status}`}
              onClick={() => setEditParam(toggleEditParam(editParam, 'status'))}
            >
              <StatusDot status={draft.status} />
              <span>{draft.status}</span>
            </button>
            <EditChip label="Created by" value={(selectedIntent ?? liveIntent)?.createdBy ?? ''} active={editParam === 'created'} onToggle={() => setEditParam(toggleEditParam(editParam, 'created'))} />
            <EditChip label="Reviewers" value={splitLines(draft.reviewersText)[0] ?? ''} active={editParam === 'review'} onToggle={() => setEditParam(toggleEditParam(editParam, 'review'))} />
          </div>
          <div className="grid grid-cols-3 gap-1">
            <EditChip label="Symbol" value={draft.symbol} active={editParam === 'symbol'} onToggle={() => setEditParam(toggleEditParam(editParam, 'symbol'))} />
            <EditChip label="Depends on" value={splitLines(draft.dependsOnText)[0] ?? ''} active={editParam === 'depends'} onToggle={() => setEditParam(toggleEditParam(editParam, 'depends'))} />
            <EditChip label="References" value={splitLines(draft.referencesText)[0] ?? ''} active={editParam === 'references'} onToggle={() => setEditParam(toggleEditParam(editParam, 'references'))} />
          </div>
          {editParam !== null ? (
            <div className="rounded-md border bg-muted/20 p-2.5">
              {editParam === 'status' ? (
                <Field label="Status">
                  <select className={fieldClass()} value={draft.status} onChange={(event) => setDraft({ ...draft, status: event.target.value === 'active' || event.target.value === 'superseded' ? event.target.value : 'draft' })}>
                    <option value="draft">draft</option><option value="active">active</option><option value="superseded">superseded</option>
                  </select>
                </Field>
              ) : editParam === 'created' ? (
                <p className="text-xs text-muted-foreground">
                  {(selectedIntent ?? liveIntent)?.createdBy || 'Not specified'} — filled by kron_add from the MCP identity or git config; not editable.
                </p>
              ) : editParam === 'review' ? (
                <Field label="Reviewers" hint="Separate handles with commas or new lines.">
                  <Input value={draft.reviewersText} placeholder="@reviewer" onChange={(event) => setDraft({ ...draft, reviewersText: event.target.value })} />
                </Field>
              ) : editParam === 'symbol' ? (
                <Field label="Symbol" hint="Optional code symbol or symbol list.">
                  <Input value={draft.symbol} placeholder="auth.RefreshToken" onChange={(event) => setDraft({ ...draft, symbol: event.target.value })} />
                </Field>
              ) : editParam === 'depends' ? (
                <Field label="Depends on" hint="Required intent slugs, one per line.">
                  <textarea className={`${fieldClass()} min-h-16 resize-y`} value={draft.dependsOnText} placeholder="auth/token-storage" onChange={(event) => setDraft({ ...draft, dependsOnText: event.target.value })} />
                </Field>
              ) : (
                <Field label="References" hint="Intent slugs, one per line.">
                  <textarea className={`${fieldClass()} min-h-16 resize-y`} value={draft.referencesText} placeholder="security/session-timeout" onChange={(event) => setDraft({ ...draft, referencesText: event.target.value })} />
                </Field>
              )}
            </div>
          ) : null}
        </>
      ) : (
        <p className="text-[0.6875rem] text-muted-foreground">The summary is taken from the first paragraph of the body after you save.</p>
      )}
      <Field label="Markdown body">
        <textarea className={`${fieldClass()} h-48 resize-y font-mono text-sm leading-6`} value={draft.body} placeholder={'## Why\n\nExplain the decision and its trade-offs.'} onChange={(event) => setDraft({ ...draft, body: event.target.value })} />
      </Field>
      {formError ? <p role="alert" className="text-sm text-destructive">{formError}</p> : null}
      <div className="flex justify-end gap-2">
        <Button type="button" variant="outline" onClick={() => { setFormMode(null); setDraftParentSlug(null); setEditParam(null) }}>Cancel</Button>
        <Button type="submit" disabled={saving}>{saving ? 'Saving…' : canEdit ? 'Save preview' : 'Save intent'}</Button>
      </div>
    </form>
  )

  return (
    <main className="h-full w-full overflow-hidden bg-background text-foreground">
      <div className="relative flex h-full min-h-0 w-full flex-col overflow-hidden">
        <IntentCanvas
          root={activeTree}
          filter={filter}
          selectedSlug={selectedSlug}
          uiScale={uiScale}
          revealToken={revealToken}
          relations={treeRelations}
          detail={canEdit ? selectedIntent ?? null : liveIntent}
          detailLoading={detailLoading}
          onCreateRelation={createGraphRelation}
          onCreateChild={startCreateChild}
          onEditIntent={startEdit}
          onDeleteIntent={() => {
            if (activeDetail !== null) {
              setPendingDeleteSlug(activeDetail.slug)
              setFormMode(null)
              setFormError(null)
            }
          }}
          onOpenCode={() => {
            const target = selectedIntent ?? liveIntent
            if (target === null || target === undefined) return
            if (vscodeApi === null) {
              setNotice('Code-anchor jump is available inside VS Code.')
              return
            }
            const message: WebviewOutbound = { type: 'kron_open_intent_code', slug: target.slug }
            vscodeApi.postMessage(message)
          }}
          editorContent={editorContent}
          assumptionContent={assumptionContent}
          onCloseDetails={() => { setFormMode(null); setDraftParentSlug(null); setFormError(null); setAssumptionFormError(null) }}
          onClearSelection={() => { setSelectedSlug(''); setFormMode(null); setDraftParentSlug(null); setFormError(null); setAssumptionCheck(null); setAssumptionCheckError(null); setAssumptionFormError(null) }}
          onSelect={(node) => { setSelectedSlug(node.slug); setFormMode(null); setShowAssumptionForm(false); setPendingDeleteSlug(null); setAssumptionCheck(null); setAssumptionCheckError(null); setAssumptionFormError(null) }}
        />

        <div className="pointer-events-none absolute inset-x-3 top-3 z-20 flex flex-wrap items-center justify-between gap-2">
          <div className="pointer-events-auto flex items-center gap-2 rounded-lg bg-background/85 px-2 py-1 shadow-sm backdrop-blur-sm">
            <img src={kronLogo} alt="Kron" className="h-8 w-8 shrink-0 object-contain" />
          </div>
          <div className="pointer-events-auto flex flex-wrap items-center justify-end gap-2">
            <Button variant="outline" size="sm" onClick={openAssumptionPanel}>
              Assumptions <Badge variant="secondary" className="ml-2">{assumptions.length}</Badge>
            </Button>
            <Button variant="outline" size="sm" onClick={() => setShowArchivePanel(true)}>
              Archived <Badge variant="secondary" className="ml-2">{archivedIntents.length}</Badge>
            </Button>
            <Button variant="outline" size="sm" onClick={() => requestTree(true)} disabled={loading} aria-label="Refresh intents">
              <span className="mr-2" aria-hidden="true">↻</span>{loading ? 'Loading…' : 'Refresh'}
            </Button>
            <div className={`flex items-center gap-1 rounded-md border p-1 ${createTargetHighlighted ? 'border-primary/40 bg-primary/10' : 'border-border/60 bg-muted/30'}`}>
              <Button size="sm" variant={createTargetHighlighted ? 'secondary' : 'ghost'} onClick={() => startCreate('leaf')} disabled={!canCreateIntent} title={canCreateIntent ? 'Create an intent document' : 'Select a workspace or folder node first'}>
                <span className="mr-1" aria-hidden="true">＋</span>Document
              </Button>
              <Button size="sm" variant={createTargetHighlighted ? 'secondary' : 'ghost'} onClick={() => startCreate('node')} disabled={!canCreateIntent} title={canCreateIntent ? 'Create an intent folder' : 'Select a workspace or folder node first'}>
                <span className="mr-1" aria-hidden="true">＋</span>Folder
              </Button>
            </div>
          </div>
        </div>
        <div className="pointer-events-none absolute left-3 top-[3.75rem] z-20 flex items-center gap-2 rounded-lg bg-background/85 p-1.5 shadow-sm backdrop-blur-sm">
          <span className="px-1 text-xs text-muted-foreground">{canEdit ? intents.length : rows.filter(({ node }) => !node.is_dir).length} intents · hierarchy</span>
          <Input className="pointer-events-auto w-64 max-w-[45vw]" aria-label="Filter intents" placeholder="Filter intents…" value={filter} onChange={(event) => setFilter(event.target.value)} />
        </div>
        {error ? <p role="alert" className="pointer-events-none absolute bottom-10 left-3 z-20 max-w-[min(560px,calc(100%-1.5rem))] rounded-md border border-destructive/40 bg-destructive/90 px-3 py-2 text-sm text-destructive-foreground shadow">{error}</p> : null}
        <div className="pointer-events-none absolute bottom-10 right-3 z-20 flex max-w-[min(460px,calc(100%-1.5rem))] flex-col items-end gap-2">
          {detailError !== null ? <p role="alert" className="rounded-md border border-destructive/40 bg-destructive/90 px-3 py-2 text-sm text-destructive-foreground shadow">{detailError}</p> : null}
          {notice !== null ? <p role="status" className="rounded-md border border-border bg-muted/90 px-3 py-2 text-sm shadow">{notice}</p> : null}
        </div>

            <aside className="fixed inset-0 z-50 overflow-y-auto bg-background p-4 md:p-8" hidden={!showAssumptionPanel} aria-label="Assumption registry">
              <Card className="mx-auto w-full max-w-5xl shadow-none">
                <CardHeader className="p-4">
                  <div className="flex items-center justify-between gap-2">
                    <div className="flex items-center gap-3">
                      <CardTitle className="text-lg">Assumptions</CardTitle>
                      <Badge variant="secondary">{assumptions.length}</Badge>
                    </div>
                    <div className="flex items-center gap-2">
                      <Button variant="outline" size="sm" onClick={startPanelCreate} disabled={assumptionPanelSaving}>New assumption</Button>
                      <Button variant="outline" size="sm" onClick={() => setShowAssumptionPanel(false)}>Close</Button>
                    </div>
                  </div>
                  <CardDescription>
                    Shared assumptions live in <code>.kron/assumptions/&lt;id&gt;.md</code>. Intents reference them from their frontmatter;
                    deleting here soft-deletes the file and leaves existing references for you to detach.
                  </CardDescription>
                </CardHeader>
                <CardContent className="grid gap-3 p-3 pt-0 md:grid-cols-[minmax(0,1fr)_minmax(0,1.4fr)]">
                  <div className="max-h-[62vh] space-y-1 overflow-y-auto rounded-md border p-2">
                    {assumptions.length === 0
                      ? <p className="px-1 py-2 text-xs text-muted-foreground">No assumptions in this project yet.</p>
                      : assumptions.map((assumption) => (
                        <button
                          key={assumption.id}
                          type="button"
                          className={`flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-xs ${assumptionPanelId === assumption.id ? 'bg-accent text-accent-foreground' : 'hover:bg-accent/50'}`}
                          title={[
                            assumption.text,
                            assumption.expiresAt !== undefined ? `expires ${assumption.expiresAt}` : 'no expiry',
                            assumption.verifiedAt !== undefined ? `verified ${assumption.verifiedAt}` : 'never verified',
                          ].join('\n')}
                          onClick={() => selectPanelAssumption(assumption)}
                        >
                          <span className="truncate font-mono">{assumption.id}</span>
                          {assumption.expiresAt !== undefined
                            ? <span className="shrink-0 text-[0.625rem] text-muted-foreground">{assumption.expiresAt.slice(0, 10)}</span>
                            : null}
                          <span className="ml-auto shrink-0 text-muted-foreground">{assumption.defaultSeverity}</span>
                          {(assumptionUsage[assumption.id]?.length ?? 0) > 0
                            ? <span className="shrink-0 rounded-full bg-muted px-1.5 text-[0.625rem] text-muted-foreground">{assumptionUsage[assumption.id]?.length}</span>
                            : null}
                        </button>
                      ))}
                  </div>
                  <div className="space-y-3">
                    {assumptionPanelError !== null ? <p role="alert" className="rounded-md border border-destructive/40 bg-destructive/10 px-2 py-1.5 text-xs text-destructive">{assumptionPanelError}</p> : null}
                    {assumptionPanelCreating ? (
                      <form className="space-y-3" onSubmit={createPanelAssumption}>
                        <div className="text-sm font-medium">New assumption</div>
                        <Field label="Assumption ID" hint="Lowercase kebab-case; becomes the file name.">
                          <Input value={assumptionPanelDraft.id} placeholder="single-region" onChange={(event) => setAssumptionPanelDraft({ ...assumptionPanelDraft, id: event.target.value })} />
                        </Field>
                        <Field label="Shared text" hint="What must stay true for intents that use it?">
                          <textarea className={`${fieldClass()} min-h-20 resize-y`} value={assumptionPanelDraft.text} placeholder="Production traffic is served from a single region." onChange={(event) => setAssumptionPanelDraft({ ...assumptionPanelDraft, text: event.target.value })} />
                        </Field>
                        <Field label="Default severity">
                          <select className={fieldClass()} value={assumptionPanelDraft.defaultSeverity} onChange={(event) => setAssumptionPanelDraft({ ...assumptionPanelDraft, defaultSeverity: event.target.value === 'soft' ? 'soft' : 'hard' })}>
                            <option value="hard">hard</option><option value="soft">soft</option>
                          </select>
                        </Field>
                        <div className="flex justify-end gap-2">
                          <Button type="button" variant="outline" size="sm" onClick={() => { setAssumptionPanelCreating(false); setAssumptionPanelError(null) }}>Cancel</Button>
                          <Button type="submit" size="sm" disabled={assumptionPanelSaving}>{assumptionPanelSaving ? 'Creating…' : 'Create assumption'}</Button>
                        </div>
                      </form>
                    ) : assumptionPanelId === null ? (
                      <p className="px-1 py-2 text-xs text-muted-foreground">Select an assumption on the left, or create a new one.</p>
                    ) : (
                      <form className="space-y-3" onSubmit={savePanelAssumption}>
                        <div className="flex items-center justify-between gap-2">
                          <span className="truncate font-mono text-xs">{assumptionPanelId}</span>
                          <Badge variant="secondary">{assumptionPanelDraft.status}</Badge>
                        </div>
                        <Field label="Shared text">
                          <textarea className={`${fieldClass()} min-h-24 resize-y`} value={assumptionPanelDraft.text} onChange={(event) => setAssumptionPanelDraft({ ...assumptionPanelDraft, text: event.target.value })} />
                        </Field>
                        <div className="grid gap-2 md:grid-cols-2">
                          <Field label="Default severity">
                            <select className={fieldClass()} value={assumptionPanelDraft.defaultSeverity} onChange={(event) => setAssumptionPanelDraft({ ...assumptionPanelDraft, defaultSeverity: event.target.value === 'soft' ? 'soft' : 'hard' })}>
                              <option value="hard">hard</option><option value="soft">soft</option>
                            </select>
                          </Field>
                          <Field label="Status">
                            <select className={fieldClass()} value={assumptionPanelDraft.status} onChange={(event) => setAssumptionPanelDraft({ ...assumptionPanelDraft, status: event.target.value === 'draft' || event.target.value === 'superseded' ? event.target.value : 'active' })}>
                              <option value="active">active</option><option value="draft">draft</option><option value="superseded">superseded</option>
                            </select>
                          </Field>
                        </div>
                        <div className="grid gap-2 md:grid-cols-2">
                          <Field label="Expires at" hint="Optional date; clearing it removes the expiry.">
                            <Input type="date" value={assumptionPanelDraft.expiresAt} onChange={(event) => setAssumptionPanelDraft({ ...assumptionPanelDraft, expiresAt: event.target.value })} />
                          </Field>
                          <Field label="Reviewers" hint="Comma or newline separated.">
                            <Input value={assumptionPanelDraft.reviewersText} placeholder="@reviewer" onChange={(event) => setAssumptionPanelDraft({ ...assumptionPanelDraft, reviewersText: event.target.value })} />
                          </Field>
                        </div>
                        {(() => {
                          const stored = assumptions.find((item) => item.id === assumptionPanelId)
                          if (stored === undefined) return null
                          const rows: Array<[string, string]> = []
                          if (stored.createdBy !== undefined) rows.push(['Created by', stored.createdBy])
                          if (stored.updatedAt !== undefined) rows.push(['Updated at', stored.updatedAt])
                          if (stored.expiresAt !== undefined) rows.push(['Expires at (stored)', stored.expiresAt])
                          if (stored.verifiedAt !== undefined) rows.push(['Verified at', stored.verifiedAt])
                          if (stored.verifiedBy !== undefined) rows.push(['Verified by', stored.verifiedBy])
                          if (rows.length === 0) return null
                          return (
                            <div className="space-y-0.5 rounded-md bg-muted/40 px-2 py-1.5 text-[0.6875rem] text-muted-foreground">
                              {rows.map(([label, value]) => (
                                <div key={label} className="flex gap-2">
                                  <span className="w-32 shrink-0">{label}</span>
                                  <span className="truncate font-mono">{value}</span>
                                </div>
                              ))}
                            </div>
                          )
                        })()}
                        <Field label="Used by">
                          {(assumptionUsage[assumptionPanelId]?.length ?? 0) === 0 ? (
                            <p className="text-xs text-muted-foreground">Not referenced by any intent.</p>
                          ) : (
                            <div className="flex flex-wrap gap-1">
                              {(assumptionUsage[assumptionPanelId] ?? []).map((entry) => (
                                <button
                                  key={entry.intentSlug}
                                  type="button"
                                  className="rounded-md border bg-background px-2 py-1 text-left text-[0.6875rem] hover:border-primary/40"
                                  title={`${entry.severity}: ${entry.rationale}`}
                                  onClick={() => {
                                    setSelectedSlug(entry.intentSlug)
                                    setRevealToken((value) => value + 1)
                                    setShowAssumptionPanel(false)
                                    setFormMode(null)
                                    setShowAssumptionForm(false)
                                  }}
                                >
                                  <span className="font-mono">{entry.intentSlug}</span>
                                  <span className="ml-1 text-muted-foreground">{entry.severity}</span>
                                </button>
                              ))}
                            </div>
                          )}
                        </Field>
                        {assumptionPanelPendingDelete === assumptionPanelId ? (
                          <div className="space-y-2 rounded-md border border-destructive/40 bg-destructive/10 p-2">
                            <p className="text-xs text-destructive">
                              Move “{assumptionPanelId}” to the Kron trash? {(assumptionUsage[assumptionPanelId]?.length ?? 0) > 0
                                ? `${assumptionUsage[assumptionPanelId]?.length} intent(s) still reference it; their links stay until you detach them.`
                                : 'Nothing references it.'}
                            </p>
                            <div className="flex justify-end gap-2">
                              <Button type="button" variant="outline" size="sm" onClick={() => setAssumptionPanelPendingDelete(null)}>Cancel</Button>
                              <Button type="button" variant="destructive" size="sm" onClick={() => void deletePanelAssumption()} disabled={assumptionPanelSaving}>Delete assumption</Button>
                            </div>
                          </div>
                        ) : (
                          <div className="flex justify-between gap-2 border-t pt-2">
                            <Button type="button" variant="outline" size="sm" onClick={() => setAssumptionPanelPendingDelete(assumptionPanelId)} disabled={assumptionPanelSaving}>Delete</Button>
                            <Button type="submit" size="sm" disabled={assumptionPanelSaving}>{assumptionPanelSaving ? 'Saving…' : 'Save assumption'}</Button>
                          </div>
                        )}
                      </form>
                    )}
                  </div>
                </CardContent>
              </Card>
            </aside>

            <aside className="fixed inset-0 z-50 overflow-y-auto bg-background p-4 md:p-8" hidden={!showArchivePanel} aria-label="Archived intents">
              <Card className="mx-auto w-full max-w-3xl shadow-none">
                <CardHeader className="p-4">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3">
                      <CardTitle className="text-lg">Archived intents</CardTitle>
                      <Badge variant="secondary">{archivedIntents.length}</Badge>
                    </div>
                    <Button variant="outline" size="sm" onClick={() => setShowArchivePanel(false)}>Exit</Button>
                  </div>
                  <CardDescription>{canEdit ? 'Preview items can be restored in this session.' : 'Deleted intents can be restored during this Webview session.'}</CardDescription>
                </CardHeader>
                {archivedIntents.length > 0 ? (
                  <CardContent className="space-y-2 p-3 pt-0">
                    {archivedIntents.map((intent) => {
                      const slugConflict = rows.some(({ node }) => node.slug === intent.slug)
                      return (
                        <div key={intent.slug} className="flex items-center gap-2 rounded-md border p-2">
                          <div className="min-w-0 flex-1">
                            <div className="truncate text-sm font-medium">{intent.title}</div>
                            <div className="truncate font-mono text-[0.625rem] text-muted-foreground">{intent.slug}</div>
                          </div>
                          <Button size="sm" variant="outline" disabled={slugConflict || saving} onClick={() => void restoreIntent(intent.slug)}>Restore</Button>
                        </div>
                      )
                    })}
                  </CardContent>
                ) : <CardContent className="p-4 pt-0 text-xs text-muted-foreground">Nothing archived.</CardContent>}
              </Card>
            </aside>

            {pendingDeleteSlug !== null ? <section className="fixed inset-0 z-[60] grid place-items-center p-4">
              <button
                type="button"
                className="absolute inset-0 bg-background/40"
                aria-label="Close intent panel"
                onClick={() => {
                  setFormMode(null)
                  setPendingDeleteSlug(null)
                  setDraftParentSlug(null)
                  setShowAssumptionForm(false)
                  setFormError(null)
                }}
              />
              <div
                role="dialog"
                aria-modal="true"
                aria-label="Delete intent"
                className="relative z-[1] flex max-h-[78vh] w-[min(560px,calc(100vw-1.5rem))] flex-col gap-4 overflow-y-auto rounded-xl border bg-background p-4 shadow-2xl md:p-6"
                onKeyDown={(event) => {
                  if (event.key === 'Escape') {
                    setFormMode(null)
                    setPendingDeleteSlug(null)
                    setDraftParentSlug(null)
                    setShowAssumptionForm(false)
                    setFormError(null)
                  }
                }}
              >
              <div className="flex justify-end">
                <Button variant="outline" size="sm" onClick={() => setPendingDeleteSlug(null)}>Close</Button>
              </div>
              {canEdit ? (
                <div className="rounded-md border border-amber-500/30 bg-amber-500/5 px-3 py-2 text-xs text-muted-foreground">
                  Mock preview: edits stay in this page session and are not written to Kron files or sent to MCP.
                </div>
              ) : null}
              {error ? <div role="alert" className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">{error}</div> : null}
              {detailError ? <div role="alert" className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">{detailError}</div> : null}
              {detailLoading ? <div role="status" className="rounded-md border px-3 py-2 text-sm text-muted-foreground">Loading intent details…</div> : null}
              {notice ? <div role="status" className="rounded-md border border-border bg-muted/40 px-3 py-2 text-sm">{notice}</div> : null}
              {pendingDeleteSlug === (selectedIntent ?? liveIntent)?.slug ? (
                <Card className="border-destructive/40">
                  <CardContent className="space-y-3 p-4">
                    <div>
                      <div className="font-medium">{canEdit ? `Archive “${(selectedIntent ?? liveIntent)?.slug}”?` : `Move “${(selectedIntent ?? liveIntent)?.slug}” to trash?`}</div>
                      <p className="mt-1 text-sm text-muted-foreground">{canEdit ? 'This preview moves the intent to Archived intents. It does not remove the file.' : 'Kron soft-deletes the intent. You can restore it from this panel during the current Webview session.'}</p>
                      {(selectedIntent ?? liveIntent)?.kind === 'node' ? (
                        <p className="mt-2 text-sm text-amber-600">
                          This node is a folder: its whole subtree ({deleteSubtreeCount} intent(s)) moves to trash together.
                        </p>
                      ) : null}
                      {selectedDependents.length > 0 ? <p className="mt-2 text-sm text-amber-600">Referenced as a prerequisite by: {selectedDependents.join(', ')}. The dependency will remain visible.</p> : null}
                    </div>
                    <div className="flex justify-end gap-2">
                      <Button size="sm" variant="outline" onClick={() => setPendingDeleteSlug(null)}>Cancel</Button>
                      <Button size="sm" variant="destructive" onClick={() => void archiveIntent()} disabled={saving}>{canEdit ? 'Archive intent' : 'Move to trash'}</Button>
                    </div>
                  </CardContent>
                </Card>
              ) : null}

              </div>
            </section> : null}
      </div>
    </main>
  )
}

export default App
