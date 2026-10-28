import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent, type ReactNode } from 'react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { IntentCanvas } from '@/IntentCanvas'
import { MarkdownPreview } from '@/components/MarkdownPreview'
import { isWebviewInbound, type McpToolName, type TreeNode, type TreeResult, type WebviewOutbound } from '@/lib/mcp-types'

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
  usedBy?: string[]
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
    usedBy: ['auth/jwt-sliding-window', 'auth/token-storage'],
  },
  {
    id: 'csrf-protected',
    text: 'State-changing browser requests include a valid CSRF token.',
    defaultSeverity: 'hard',
    usedBy: ['auth/jwt-sliding-window'],
  },
  {
    id: 'clock-skew-30s',
    text: 'Application hosts stay within thirty seconds of the time source.',
    defaultSeverity: 'soft',
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

function newDraft(slug = ''): IntentDraft {
  return {
    slug,
    kind: 'leaf',
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

function App(): JSX.Element {
  const [intents, setIntents] = useState<Intent[]>(initialIntents)
  const [archivedIntents, setArchivedIntents] = useState<Intent[]>([])
  const [assumptions, setAssumptions] = useState<Assumption[]>(initialAssumptions)
  const [tree, setTree] = useState<TreeResult | null>(null)
  const [treeRelations, setTreeRelations] = useState<Record<string, IntentRelations>>({})
  const [liveIntent, setLiveIntent] = useState<Intent | null>(null)
  const [selectedSlug, setSelectedSlug] = useState<string>(initialIntents[0]?.slug ?? '')
  const [filter, setFilter] = useState('')
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
  const [showArchivePanel, setShowArchivePanel] = useState(false)
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
  const pendingMcpRequests = useRef(new Map<number, PendingMcpRequest>())
  const nextMcpRequestId = useRef(0)
  const detailRequestSequence = useRef(0)
  const treeRelationSequence = useRef(0)

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
  const activeWorkspaceId = selectedNode?.workspaceId ?? liveIntent?.workspaceId
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

  const startCreate = (): void => {
    setDraft(newDraft())
    setDraftParentSlug(null)
    setFormError(null)
    setFormMode('create')
    setPendingDeleteSlug(null)
    setShowAssumptionForm(false)
    setNotice(null)
  }

  const startCreateChild = (parent: TreeNode): void => {
    setSelectedSlug(parent.slug)
    setDraft(newDraft(`${parent.slug}/`))
    setDraftParentSlug(parent.slug)
    setFormError(null)
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
    setFormMode('edit')
    setPendingDeleteSlug(null)
    setShowAssumptionForm(false)
    setNotice(null)
  }

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
    if (draft.title.trim() === '' || draft.summary.trim() === '') {
      setFormError('Title and summary are required.')
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
      summary: draft.summary.trim(),
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
          why: draft.summary.trim(),
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
      setFormError('Use a valid assumption ID and enter its text.')
      return
    }
    if (assumptionDraft.rationale.trim().length < 10) {
      setFormError('Explain this intent’s severity in at least 10 characters.')
      return
    }
    if (assumptions.some((assumption) => assumption.id === assumptionDraft.id)) {
      setFormError('An assumption with this ID already exists.')
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
        setFormError('The selected intent is not associated with a workspace.')
        return
      }
      const link: IntentAssumption = {
        id: assumption.id,
        severity: assumptionDraft.severity,
        rationale: assumptionDraft.rationale.trim(),
        ...(assumptionDraft.expiresAt ? { expiresAt: assumptionDraft.expiresAt } : {}),
      }
      setSaving(true)
      setFormError(null)
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
        setFormError(created ? `The shared assumption was created but not attached. ${message}` : message)
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
    setFormError(null)
    setNotice(`Preview assumption “${assumption.id}” created and attached.`)
  }

  const attachExistingAssumption = async (event: FormEvent<HTMLFormElement>): Promise<void> => {
    event.preventDefault()
    const currentIntent = selectedIntent ?? liveIntent
    if (currentIntent === null || currentIntent === undefined) return
    const shared = assumptions.find((assumption) => assumption.id === selectedAssumptionId)
    if (shared === undefined) {
      setFormError('Choose an existing assumption.')
      return
    }
    if (currentIntent.assumptions.some((item) => item.id === shared.id)) {
      setFormError('This assumption is already linked to the selected intent.')
      return
    }
    if (assumptionDraft.rationale.trim().length < 10) {
      setFormError('Explain this intent’s severity in at least 10 characters.')
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
        setFormError('The selected intent is not associated with a workspace.')
        return
      }
      setSaving(true)
      setFormError(null)
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
        setFormError(requestError instanceof Error ? requestError.message : 'Kron could not attach this assumption.')
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
    setFormError(null)
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

  const archiveIntent = async (): Promise<void> => {
    const archived = selectedIntent ?? liveIntent
    if (archived === null || archived === undefined || pendingDeleteSlug !== archived.slug) return
    if (!canEdit) {
      setSaving(true)
      try {
        await sendMcpRequest('kron_delete', { slug: archived.slug }, archived.workspaceId)
        setArchivedIntents((current) => [archived, ...current])
        setSelectedSlug('')
        setPendingDeleteSlug(null)
        setShowAssumptionForm(false)
        setNotice(`Moved "${archived.slug}" to Kron trash. Restore is available during this Webview session.`)
        setDetailReload((value) => value + 1)
        requestTree(true)
      } catch (requestError: unknown) {
        setDetailError(requestError instanceof Error ? requestError.message : 'Kron could not delete this intent.')
      } finally {
        setSaving(false)
      }
      return
    }
    setArchivedIntents((current) => [archived, ...current])
    setIntents((current) => current.filter((intent) => intent.slug !== archived.slug))
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
        await sendMcpRequest('kron_restore', { slug }, archived.workspaceId)
        setArchivedIntents((current) => current.filter((intent) => intent.slug !== slug))
        setSelectedSlug(slug)
        setNotice(`Restored "${slug}" from Kron trash.`)
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
    setFormError(null)
    if (opening) {
      setAssumptionMode(availableAssumptions.length > 0 ? 'existing' : 'new')
      setSelectedAssumptionId(availableAssumptions[0]?.id ?? '')
    }
  }

  const activeDetail: Intent | null = selectedIntent ?? liveIntent ?? (selectedNode !== undefined
    ? {
        slug: selectedNode.slug,
        kind: selectedNode.is_dir ? 'node' : 'leaf',
        title: selectedNode.title ?? selectedNode.name,
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

  const sharedAssumptions = assumptions.filter((assumption) => activeDetail?.assumptions.some((item) => item.id === assumption.id))
  const availableAssumptions = assumptions.filter((assumption) => !activeDetail?.assumptions.some((item) => item.id === assumption.id))

  const editorContent = formMode === null ? null : (
    <form className="grid items-start gap-4 md:grid-cols-2" onSubmit={saveIntent}>
      <div className="md:col-span-2 flex items-start justify-between gap-3">
        <div>
          <div className="text-base font-semibold">{formMode === 'create' ? 'New intent' : 'Edit intent'}</div>
          <div className="mt-1 text-xs text-muted-foreground">{canEdit ? 'Preview form for the supported intent fields.' : 'Edits are written through the Kron MCP tools.'}</div>
        </div>
        <span className="rounded-full bg-muted px-2 py-1 text-[10px] text-muted-foreground">Editing</span>
      </div>
      <Field label="Intent slug" hint={draftParentSlug === null ? 'Lowercase kebab-case; use / for a group.' : `Child of “${draftParentSlug}” is locked; enter one child name after the slash.`}>
        <Input value={draft.slug} disabled={formMode === 'edit'} placeholder="auth/session-renewal" onChange={(event) => {
          const value = event.target.value
          const prefix = draftParentSlug === null ? '' : `${draftParentSlug}/`
          setDraft({ ...draft, slug: prefix !== '' && !value.startsWith(prefix) ? `${prefix}${value.replace(/^\/+/, '')}` : value })
        }} />
      </Field>
      <Field label="Title">
        <Input value={draft.title} placeholder="Session renewal" onChange={(event) => setDraft({ ...draft, title: event.target.value })} />
      </Field>
      <div className="md:col-span-2">
        <Field label="Summary" hint={formMode === 'create' ? 'Required. Shown in the intent tree and used as the opening paragraph.' : canEdit ? 'A short description shown in the tree and detail header.' : 'Derived from the first paragraph of the Markdown body.'}>
          <textarea className={`${fieldClass()} min-h-20 resize-y`} value={draft.summary} readOnly={!canEdit && formMode !== 'create'} placeholder="What decision does this intent record?" onChange={(event) => setDraft({ ...draft, summary: event.target.value })} />
        </Field>
      </div>
      <div className="md:col-span-2 grid min-h-[24rem] gap-4 lg:grid-cols-2">
        <Field label="Markdown body" hint={canEdit ? 'Edit the decision body; the preview updates as you type.' : 'The Markdown body is saved through Kron MCP.'}>
          <textarea className={`${fieldClass()} min-h-[20rem] resize-y font-mono text-sm leading-6`} value={draft.body} placeholder={'## Why\n\nExplain the decision and its trade-offs.'} onChange={(event) => setDraft({ ...draft, body: event.target.value })} />
        </Field>
        <div className="grid min-h-0 gap-1.5 text-sm">
          <span className="font-medium">Preview</span>
          <div className="min-h-[20rem] overflow-auto rounded-md border border-input bg-muted/20 p-4">
            <MarkdownPreview body={draft.body || 'Nothing to preview yet.'} />
          </div>
        </div>
      </div>
      <details className="md:col-span-2 rounded-md border p-3">
        <summary className="cursor-pointer text-sm font-medium">Advanced fields (optional)</summary>
        <div className="mt-4 grid gap-4 md:grid-cols-2">
          {formMode === 'create' ? (
            <Field label="Intent kind" hint="Leaf is a single intent; node groups child intents.">
              <select className={fieldClass()} value={draft.kind} onChange={(event) => setDraft({ ...draft, kind: event.target.value as Intent['kind'] })}>
                <option value="leaf">leaf</option><option value="node">node</option>
              </select>
            </Field>
          ) : null}
          <Field label="Status">
            <select className={fieldClass()} value={draft.status} onChange={(event) => setDraft({ ...draft, status: event.target.value as Intent['status'] })}>
              <option value="draft">draft</option><option value="active">active</option><option value="superseded">superseded</option>
            </select>
          </Field>
          <Field label="Symbol" hint="Optional code symbol or symbol list.">
            <Input value={draft.symbol} placeholder="auth.RefreshToken" onChange={(event) => setDraft({ ...draft, symbol: event.target.value })} />
          </Field>
          <Field label="Reviewers" hint="Separate handles with commas or new lines.">
            <Input value={draft.reviewersText} placeholder="@reviewer" onChange={(event) => setDraft({ ...draft, reviewersText: event.target.value })} />
          </Field>
          <Field label="References" hint="Intent slugs, one per line.">
            <textarea className={`${fieldClass()} min-h-20 resize-y`} value={draft.referencesText} placeholder="security/session-timeout" onChange={(event) => setDraft({ ...draft, referencesText: event.target.value })} />
          </Field>
          <Field label="Depends on" hint="Required intent slugs, one per line.">
            <textarea className={`${fieldClass()} min-h-20 resize-y`} value={draft.dependsOnText} placeholder="auth/token-storage" onChange={(event) => setDraft({ ...draft, dependsOnText: event.target.value })} />
          </Field>
        </div>
      </details>
      {formError ? <p role="alert" className="md:col-span-2 text-sm text-destructive">{formError}</p> : null}
      <div className="flex justify-end gap-2 md:col-span-2">
        <Button type="button" variant="outline" onClick={() => { setFormMode(null); setDraftParentSlug(null) }}>Cancel</Button>
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
          editorContent={editorContent}
          onCloseDetails={() => { setFormMode(null); setDraftParentSlug(null); setFormError(null) }}
          onClearSelection={() => { setSelectedSlug(''); setFormMode(null); setDraftParentSlug(null); setFormError(null) }}
          onSelect={(node) => { setSelectedSlug(node.slug); setFormMode(null); setShowAssumptionForm(false); setPendingDeleteSlug(null) }}
        />

        <div className="pointer-events-none absolute inset-x-3 top-3 z-20 flex flex-wrap items-center justify-between gap-2">
          <div className="pointer-events-auto flex items-center gap-2 rounded-lg bg-background/85 px-2 py-1 shadow-sm backdrop-blur-sm">
            <div className="grid h-8 w-8 shrink-0 place-items-center rounded-lg bg-primary text-sm font-bold text-primary-foreground">K</div>
            <div className="font-semibold leading-tight">Intent tree</div>
            <Badge variant={canEdit ? 'secondary' : 'default'}>{canEdit ? 'Mock preview' : 'Live tree'}</Badge>
          </div>
          <div className="pointer-events-auto flex flex-wrap items-center justify-end gap-2">
            <Button variant="outline" size="sm" onClick={() => setShowArchivePanel(true)}>
              Archived <Badge variant="secondary" className="ml-2">{archivedIntents.length}</Badge>
            </Button>
            <Button variant="outline" size="sm" onClick={() => requestTree(true)} disabled={loading} aria-label="Refresh intents">
              <span className="mr-2" aria-hidden="true">↻</span>{loading ? 'Loading…' : 'Refresh'}
            </Button>
            <Button size="sm" onClick={startCreate} disabled={!canEdit && activeWorkspaceId === undefined}>
              <span className="mr-2" aria-hidden="true">＋</span>New intent
            </Button>
          </div>
        </div>
        <div className="pointer-events-none absolute left-3 top-[3.75rem] z-20 flex items-center gap-2 rounded-lg bg-background/85 p-1.5 shadow-sm backdrop-blur-sm">
          <span className="px-1 text-xs text-muted-foreground">{canEdit ? intents.length : rows.filter(({ node }) => !node.is_dir).length} intents · hierarchy</span>
          <Input className="pointer-events-auto w-64 max-w-[45vw]" aria-label="Filter intents" placeholder="Filter intents…" value={filter} onChange={(event) => setFilter(event.target.value)} />
        </div>
        {error ? <p role="alert" className="pointer-events-none absolute bottom-10 left-3 z-20 max-w-[min(560px,calc(100%-1.5rem))] rounded-md border border-destructive/40 bg-destructive/90 px-3 py-2 text-sm text-destructive-foreground shadow">{error}</p> : null}

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
                            <div className="truncate font-mono text-[10px] text-muted-foreground">{intent.slug}</div>
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
