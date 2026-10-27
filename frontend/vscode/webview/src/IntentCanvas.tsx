import { useEffect, useMemo, useRef, useState, type MouseEvent } from 'react'
import { CircleCheck, CircleDashed, CircleX, FileText, Folder } from 'lucide-react'
import type { TreeNode } from '@/lib/mcp-types'

type PositionedNode = {
  node: TreeNode
  x: number
  y: number
  isWorkspace: boolean
}

type TreeEdge = { parent: PositionedNode; child: PositionedNode }
type RelationEdge = { from: PositionedNode; to: PositionedNode; kind: 'depends' | 'references' }
type LinkKind = 'depends' | 'references'
type CanvasIntentDetail = {
  slug: string
  title: string
  kind: string
  status: string
  summary: string
  symbol: string
  reviewers: string[]
  body: string
  references: string[]
  dependsOn: string[]
}

type CanvasLayout = {
  nodes: PositionedNode[]
  edges: TreeEdge[]
  width: number
  height: number
}

const COLUMN_WIDTH = 300
const ROW_HEIGHT = 92
const NODE_WIDTH = 236
const NODE_HEIGHT = 64
const PADDING = 40
const CANVAS_TOP_INSET = 104

function buildLayout(root: TreeNode): CanvasLayout {
  const nodes: PositionedNode[] = []
  const edges: TreeEdge[] = []
  const childrenOf = (node: TreeNode): TreeNode[] => node.children ?? []

  const measure = (node: TreeNode): number => {
    const children = childrenOf(node)
    return children.length === 0 ? 1 : children.reduce((sum, child) => sum + measure(child), 0)
  }

  const place = (node: TreeNode, depth: number, start: number, slots: number, isWorkspace = false): PositionedNode => {
    const current: PositionedNode = {
      node,
      x: PADDING + depth * COLUMN_WIDTH,
      y: PADDING + (start + slots / 2) * ROW_HEIGHT - NODE_HEIGHT / 2,
      isWorkspace,
    }
    nodes.push(current)
    let nextStart = start
    for (const child of childrenOf(node)) {
      const childSlots = measure(child)
      const positionedChild = place(child, depth + 1, nextStart, childSlots)
      edges.push({ parent: current, child: positionedChild })
      nextStart += childSlots
    }
    return current
  }

  const totalSlots = measure(root)
  place(root, 0, 0, totalSlots, true)
  const maxDepth = nodes.reduce((max, node) => Math.max(max, Math.round((node.x - PADDING) / COLUMN_WIDTH)), 0)
  return {
    nodes,
    edges,
    width: PADDING * 2 + maxDepth * COLUMN_WIDTH + NODE_WIDTH,
    height: PADDING * 2 + Math.max(totalSlots, 1) * ROW_HEIGHT,
  }
}

function countLeafIntents(node: TreeNode): number {
  const children = node.children ?? []
  if (children.length === 0) return node.is_dir ? 0 : 1
  return children.reduce((sum, child) => sum + countLeafIntents(child), 0)
}

function matchesFilter(node: TreeNode, query: string): boolean {
  if (query === '') return true
  const label = `${node.title ?? ''} ${node.name} ${node.slug}`.toLowerCase()
  return label.includes(query) || (node.children ?? []).some((child) => matchesFilter(child, query))
}

function statusIcon(status: string | undefined): JSX.Element {
  if (status === 'active') return <CircleCheck className="h-3.5 w-3.5 text-emerald-500" />
  if (status === 'superseded') return <CircleX className="h-3.5 w-3.5 text-muted-foreground" />
  return <CircleDashed className="h-3.5 w-3.5 text-muted-foreground" />
}

type IntentCanvasProps = {
  root: TreeNode
  filter: string
  selectedSlug: string
  relations: Record<string, { references: string[]; dependsOn: string[]; kind?: 'leaf' | 'node'; exists?: boolean }>
  detail: CanvasIntentDetail | null
  detailLoading: boolean
  onCreateRelation: (source: TreeNode, target: TreeNode, kind: LinkKind) => Promise<void>
  onCreateChild: (parent: TreeNode) => void
  onEditIntent: () => void
  onDeleteIntent: () => void
  onSelect: (node: TreeNode) => void
}

/** Left-to-right, pannable hierarchy canvas, adapted for the VS Code Webview tree contract. */
export function IntentCanvas({ root, filter, selectedSlug, relations, detail, detailLoading, onCreateRelation, onCreateChild, onEditIntent, onDeleteIntent, onSelect }: IntentCanvasProps): JSX.Element {
  const layout = useMemo(() => buildLayout({
    ...root,
    title: root.title ?? root.name,
    children: root.children ?? [],
  }), [root])
  const [pan, setPan] = useState({ x: 0, y: 0 })
  const [viewportSize, setViewportSize] = useState({ width: 0, height: 0 })
  const [panning, setPanning] = useState(false)
  const [inspectedSlug, setInspectedSlug] = useState<string | null>(null)
  const [expandedDetails, setExpandedDetails] = useState(false)
  const [contextMenu, setContextMenu] = useState<{ node: TreeNode; x: number; y: number } | null>(null)
  const [linking, setLinking] = useState<{ source: TreeNode; kind: LinkKind; cursor: { x: number; y: number } } | null>(null)
  const [linkSaving, setLinkSaving] = useState(false)
  const [linkError, setLinkError] = useState<string | null>(null)
  const [linkMessage, setLinkMessage] = useState<string | null>(null)
  const containerRef = useRef<HTMLDivElement | null>(null)
  const viewportRef = useRef<HTMLDivElement | null>(null)
  const dragRef = useRef<{ x: number; y: number; originX: number; originY: number; moved: boolean } | null>(null)
  const query = filter.trim().toLowerCase()
  const intents = countLeafIntents(root)
  const hasChildren = (root.children?.length ?? 0) > 0
  const inspectedNode = inspectedSlug === null ? null : layout.nodes.find(({ node }) => node.slug === inspectedSlug)?.node ?? null
  const fitScale = viewportSize.width > 0 && viewportSize.height > 0
    ? Math.max(0.1, Math.min(1, (viewportSize.width - 32) / layout.width, (viewportSize.height - CANVAS_TOP_INSET - 24) / layout.height))
    : 1
  const fitOffset = {
    x: Math.max(16, (viewportSize.width - layout.width * fitScale) / 2),
    y: CANVAS_TOP_INSET + Math.max(12, (viewportSize.height - CANVAS_TOP_INSET - layout.height * fitScale) / 2),
  }

  useEffect(() => {
    const viewport = viewportRef.current
    if (viewport === null) return
    const observer = new ResizeObserver(([entry]) => {
      if (entry !== undefined) setViewportSize({ width: entry.contentRect.width, height: entry.contentRect.height })
    })
    observer.observe(viewport)
    return () => observer.disconnect()
  }, [])

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent): void => {
      if (event.key === 'Escape') {
        setContextMenu(null)
        setLinking(null)
        setLinkError(null)
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [])
  const relationEdges = useMemo((): RelationEdge[] => {
    const bySlug = new Map(layout.nodes
      .filter(({ isWorkspace, node }) => !isWorkspace && (!node.is_dir || relations[node.slug]?.exists === true))
      .map((positioned) => [positioned.node.slug, positioned]))
    const edges: RelationEdge[] = []
    const seen = new Set<string>()
    for (const from of layout.nodes) {
      if (from.isWorkspace) continue
      const linked = relations[from.node.slug]
      if (linked === undefined) continue
      for (const slug of linked.dependsOn) {
        const to = bySlug.get(slug)
        const key = `depends:${from.node.slug}:${slug}`
        if (to !== undefined && to !== from && !seen.has(key)) {
          seen.add(key)
          edges.push({ from, to, kind: 'depends' })
        }
      }
      for (const slug of linked.references) {
        const to = bySlug.get(slug)
        const key = `references:${from.node.slug}:${slug}`
        if (to !== undefined && to !== from && !seen.has(key)) {
          seen.add(key)
          edges.push({ from, to, kind: 'references' })
        }
      }
    }
    return edges
  }, [layout.nodes, relations])
  const relatedFrames = useMemo(() => {
    const frames = new Map<string, 'depends' | 'references'>()
    const selected = relations[selectedSlug]
    if (selected === undefined) return frames
    selected.dependsOn.forEach((slug) => frames.set(slug, 'depends'))
    selected.references.forEach((slug) => { if (!frames.has(slug)) frames.set(slug, 'references') })
    return frames
  }, [relations, selectedSlug])

  useEffect(() => {
    if (!panning) return
    const onMove = (event: globalThis.MouseEvent): void => {
      const drag = dragRef.current
      if (drag === null) return
      const dx = event.clientX - drag.x
      const dy = event.clientY - drag.y
      if (Math.abs(dx) > 3 || Math.abs(dy) > 3) drag.moved = true
      setPan({ x: drag.originX + dx, y: drag.originY + dy })
    }
    const onUp = (): void => {
      dragRef.current = null
      setPanning(false)
    }
    window.addEventListener('mousemove', onMove)
    window.addEventListener('mouseup', onUp)
    return () => {
      window.removeEventListener('mousemove', onMove)
      window.removeEventListener('mouseup', onUp)
    }
  }, [panning])

  const startPan = (event: MouseEvent<HTMLDivElement>): void => {
    if (linking !== null || event.button !== 0 || (event.target as HTMLElement).closest('[data-intent-node], [data-canvas-overlay]') !== null) return
    dragRef.current = { x: event.clientX, y: event.clientY, originX: pan.x, originY: pan.y, moved: false }
    setPanning(true)
  }

  const updateLinkCursor = (event: MouseEvent<HTMLDivElement>): void => {
    if (linking === null || viewportRef.current === null) return
    const bounds = viewportRef.current.getBoundingClientRect()
    setLinking((current) => current === null ? null : {
      ...current,
      cursor: {
        x: (event.clientX - bounds.left - fitOffset.x - pan.x) / fitScale,
        y: (event.clientY - bounds.top - fitOffset.y - pan.y) / fitScale,
      },
    })
  }

  const beginLink = (kind: LinkKind): void => {
    if (contextMenu === null) return
    const metadata = relations[contextMenu.node.slug]
    if (metadata?.exists !== true && contextMenu.node.is_dir) return
    const source = layout.nodes.find(({ node }) => node.slug === contextMenu.node.slug)
    if (source === undefined) return
    setLinking({ source: contextMenu.node, kind, cursor: { x: source.x + NODE_WIDTH, y: source.y + NODE_HEIGHT / 2 } })
    setLinkError(null)
    setLinkMessage(null)
    setContextMenu(null)
  }

  const chooseLinkTarget = (target: TreeNode): void => {
    if (linking === null || linkSaving) return
    if (relations[target.slug]?.exists !== true) {
      setLinkError('Choose an Intent node, not a structural folder.')
      return
    }
    if (linking.source.slug === target.slug) {
      setLinkError('A node cannot link to itself. Choose another target or press Esc to cancel.')
      return
    }
    setLinkSaving(true)
    setLinkError(null)
    void onCreateRelation(linking.source, target, linking.kind).then(() => {
      setLinkMessage(`Saved ${linking.kind === 'depends' ? 'hard dependency' : 'reference'}: ${linking.source.slug} → ${target.slug}`)
      setLinking(null)
      setLinkError(null)
    }).catch((error: unknown) => {
      setLinkError(error instanceof Error ? error.message : 'Could not save this relation. Try again or press Esc to cancel.')
    }).finally(() => setLinkSaving(false))
  }

  return (
    <div ref={containerRef} className="relative flex min-h-0 w-full flex-1 overflow-hidden" aria-label="Intent relationship canvas">
      <div
        ref={viewportRef}
        className="relative min-w-0 flex-1 overflow-hidden"
        onMouseDown={startPan}
        onMouseMove={updateLinkCursor}
        onDoubleClick={(event) => { if ((event.target as HTMLElement).closest('[data-intent-node], [data-canvas-overlay]') === null) setPan({ x: 0, y: 0 }) }}
        onContextMenu={(event) => { if ((event.target as HTMLElement).closest('[data-intent-node]') === null) { event.preventDefault(); setContextMenu(null); if (linking !== null) setLinking(null) } }}
      >
      <div
        className={`absolute inset-0 touch-none ${linking !== null ? 'cursor-crosshair' : panning ? 'cursor-grabbing' : 'cursor-grab'}`}
        style={{
          backgroundColor: 'hsl(var(--background))',
          backgroundImage: 'radial-gradient(circle, hsl(var(--border) / 0.85) 1px, transparent 1px)',
          backgroundSize: '22px 22px',
          backgroundPosition: `${pan.x}px ${pan.y}px`,
        }}
      >
        <div className="absolute left-0 top-0 origin-top-left" style={{ transform: `translate(${fitOffset.x + pan.x}px, ${fitOffset.y + pan.y}px) scale(${fitScale})`, width: layout.width, height: layout.height }}>
          <svg className="pointer-events-none absolute left-0 top-0" width={layout.width} height={layout.height} aria-hidden="true">
            <defs>
              <marker id="kron-vscode-depends-arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
                <path d="M 0 0 L 10 5 L 0 10 z" fill="hsl(var(--primary))" />
              </marker>
              <marker id="kron-vscode-references-arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
                <path d="M 0 0 L 10 5 L 0 10 z" fill="hsl(var(--muted-foreground))" />
              </marker>
            </defs>
            {layout.edges.map(({ parent, child }) => {
              const x1 = parent.x + NODE_WIDTH
              const y1 = parent.y + NODE_HEIGHT / 2
              const x2 = child.x
              const y2 = child.y + NODE_HEIGHT / 2
              const bend = (x2 - x1) / 2
              return <path key={`${parent.node.slug}->${child.node.slug}`} d={`M ${x1} ${y1} C ${x1 + bend} ${y1}, ${x2 - bend} ${y2}, ${x2} ${y2}`} fill="none" stroke="hsl(var(--border))" strokeWidth="1.5" />
            })}
            {relationEdges.map(({ from, to, kind }) => {
              const isDepends = kind === 'depends'
              const x1 = from.x + NODE_WIDTH
              const y1 = from.y + NODE_HEIGHT / 2 + (isDepends ? -3 : 3)
              const x2 = to.x
              const y2 = to.y + NODE_HEIGHT / 2 + (isDepends ? -3 : 3)
              const bend = (x2 - x1) / 2
              return (
                <path
                  key={`${kind}:${from.node.slug}->${to.node.slug}`}
                  d={`M ${x1} ${y1} C ${x1 + bend} ${y1}, ${x2 - bend} ${y2}, ${x2} ${y2}`}
                  fill="none"
                  stroke={isDepends ? 'hsl(var(--primary))' : 'hsl(var(--muted-foreground))'}
                  strokeWidth={isDepends ? 2 : 1.5}
                  strokeDasharray={isDepends ? undefined : '6 4'}
                  markerEnd={isDepends ? 'url(#kron-vscode-depends-arrow)' : 'url(#kron-vscode-references-arrow)'}
                  opacity={selectedSlug === from.node.slug ? 1 : 0.6}
                />
              )
            })}
            {linking !== null ? (() => {
              const source = layout.nodes.find(({ node }) => node.slug === linking.source.slug)
              if (source === undefined) return null
              const x1 = source.x + NODE_WIDTH
              const y1 = source.y + NODE_HEIGHT / 2
              const x2 = linking.cursor.x
              const y2 = linking.cursor.y
              const bend = (x2 - x1) / 2
              const isDepends = linking.kind === 'depends'
              return <path d={`M ${x1} ${y1} C ${x1 + bend} ${y1}, ${x2 - bend} ${y2}, ${x2} ${y2}`} fill="none" stroke={isDepends ? 'hsl(var(--primary))' : 'hsl(var(--muted-foreground))'} strokeWidth={isDepends ? 2 : 1.5} strokeDasharray={isDepends ? undefined : '6 4'} markerEnd={isDepends ? 'url(#kron-vscode-depends-arrow)' : 'url(#kron-vscode-references-arrow)'} />
            })() : null}
          </svg>

          {layout.nodes.map(({ node, x, y, isWorkspace }) => {
            const isGroup = isWorkspace || node.is_dir
            const selected = !isWorkspace && selectedSlug === node.slug
            const dimmed = !isWorkspace && !matchesFilter(node, query)
            const relationFrame = relatedFrames.get(node.slug)
            const title = node.title ?? node.name
            const card = (
              <>
                {isGroup ? <Folder className="h-4 w-4 shrink-0 text-muted-foreground" /> : <FileText className="h-4 w-4 shrink-0 text-muted-foreground" />}
                <span className="min-w-0 flex-1 text-left">
                  <span className="block truncate text-sm font-medium leading-tight">{title}</span>
                  <span className="block truncate text-[10px] text-muted-foreground">{isWorkspace ? `${intents} intents` : isGroup ? `group · ${countLeafIntents(node)} intents` : node.slug}</span>
                </span>
                {!isGroup ? statusIcon(node.status) : null}
              </>
            )
            const classes = `absolute flex items-center gap-2 rounded-xl border px-3 shadow-sm transition-all ${isGroup ? 'border-border/80 bg-secondary/90 text-secondary-foreground' : 'border-border/70 bg-card text-card-foreground'} ${dimmed ? 'opacity-35' : 'opacity-100'} ${selected ? 'ring-2 ring-primary ring-offset-2 ring-offset-background' : 'hover:border-primary/50 hover:shadow-md'}`
            const styles = {
              left: x,
              top: y,
              width: NODE_WIDTH,
              height: NODE_HEIGHT,
              ...(relationFrame !== undefined && !selected ? {
                outline: `2px ${relationFrame === 'depends' ? 'solid' : 'dashed'} hsl(${relationFrame === 'depends' ? 'var(--primary)' : 'var(--muted-foreground)'})`,
                outlineOffset: '3px',
              } : {}),
            }

            return isWorkspace ? (
              <div key="kron-workspace-root" className={classes} style={styles} aria-label={title}>{card}</div>
            ) : (
              <button
                key={`${node.workspaceId ?? ''}:${node.slug}`}
                type="button"
                data-intent-node="true"
                className={classes}
                style={styles}
                aria-pressed={selected}
                onClick={() => {
                  if (linking !== null) {
                    chooseLinkTarget(node)
                    return
                  }
                  onSelect(node)
                  if (inspectedSlug !== null) { setInspectedSlug(node.slug); setExpandedDetails(false) }
                }}
                onDoubleClick={(event) => {
                  event.stopPropagation()
                  if (linking !== null) return
                  onSelect(node)
                  setInspectedSlug(node.slug)
                  setExpandedDetails(false)
                  setContextMenu(null)
                }}
                onContextMenu={(event) => {
                  event.preventDefault()
                  event.stopPropagation()
                  if (linking !== null) { setLinking(null); setLinkError(null); return }
                  const bounds = containerRef.current?.getBoundingClientRect()
                  onSelect(node)
                  if (bounds !== undefined) setContextMenu({ node, x: event.clientX - bounds.left, y: event.clientY - bounds.top })
                }}
              >
                {card}
              </button>
            )
          })}
        </div>
      </div>
      {!hasChildren ? <div className="pointer-events-none absolute inset-0 grid place-items-center text-sm text-muted-foreground">No intents in this workspace yet.</div> : null}
      {linking !== null ? <div data-canvas-overlay="true" className="absolute bottom-10 left-2 z-20 max-w-[min(480px,calc(100%-1rem))] rounded-md border bg-background/95 px-3 py-2 text-xs shadow-sm">{linkSaving ? 'Saving relation…' : `Choose a target for ${linking.kind === 'depends' ? 'depends_on (solid)' : 'references (dashed)'}. Press Esc to cancel.`}{linkError ? <div role="alert" className="mt-1 text-destructive">{linkError}</div> : null}</div> : null}
      {linking === null && linkMessage !== null ? <div data-canvas-overlay="true" role="status" className="absolute bottom-10 left-2 z-20 max-w-[min(480px,calc(100%-1rem))] rounded-md border bg-background/95 px-3 py-2 text-xs shadow-sm">{linkMessage}<button type="button" className="ml-3 text-muted-foreground hover:text-foreground" onClick={() => setLinkMessage(null)}>×</button></div> : null}
      {contextMenu !== null ? (
        <div data-canvas-overlay="true" className="absolute z-30 grid min-w-56 gap-1 rounded-md border bg-popover p-1 text-popover-foreground shadow-lg" style={{ left: Math.min(contextMenu.x, Math.max(8, (containerRef.current?.clientWidth ?? 600) - 240)), top: Math.min(contextMenu.y, Math.max(8, (containerRef.current?.clientHeight ?? 400) - 100)), backgroundColor: 'hsl(var(--popover))', opacity: 1 }} onMouseDown={(event) => event.stopPropagation()}>
          <div className="px-3 pt-2 text-[10px] text-muted-foreground">Parent: {contextMenu.node.slug}</div>
          <button type="button" className="rounded px-3 py-2 text-left text-xs hover:bg-accent" onClick={() => { onCreateChild(contextMenu.node); setContextMenu(null) }}>Create child intent</button>
          {!contextMenu.node.is_dir || relations[contextMenu.node.slug]?.exists === true ? (
            <>
              {contextMenu.node.is_dir || relations[contextMenu.node.slug]?.exists === true ? <div className="my-0.5 border-t" /> : null}
              <button type="button" className="rounded px-3 py-2 text-left text-xs hover:bg-accent" onClick={() => beginLink('depends')}>Create hard dependency · solid</button>
              <button type="button" className="rounded px-3 py-2 text-left text-xs hover:bg-accent" onClick={() => beginLink('references')}>Create reference · dashed</button>
            </>
          ) : null}
        </div>
      ) : null}
      <div className="pointer-events-none absolute right-2 top-[3.75rem] flex flex-wrap items-center justify-end gap-3 rounded bg-background/90 px-2 py-1 text-[10px] text-muted-foreground">
          <span className="flex items-center gap-1"><svg width="24" height="8" aria-hidden="true"><line x1="1" y1="4" x2="21" y2="4" stroke="hsl(var(--primary))" strokeWidth="2" /></svg>depends_on (hard)</span>
          <span className="flex items-center gap-1"><svg width="24" height="8" aria-hidden="true"><line x1="1" y1="4" x2="21" y2="4" stroke="hsl(var(--muted-foreground))" strokeWidth="1.5" strokeDasharray="6 4" /></svg>references (soft)</span>
      </div>
      {hasChildren && relationEdges.length === 0 ? (
        <div className="pointer-events-none absolute bottom-2 right-2 rounded bg-background/90 px-2 py-1 text-[10px] text-muted-foreground">
          No cross-intent links yet. Add them with Edit intent → depends_on or references.
        </div>
      ) : null}
      <div className="pointer-events-none absolute bottom-2 left-2 rounded bg-background/85 px-2 py-1 text-[10px] text-muted-foreground">
        Drag empty space to pan · double-click a node for details · right-click a node to link
      </div>
      </div>
      {inspectedNode !== null ? (
        <aside data-canvas-overlay="true" className="absolute inset-y-0 right-0 z-40 flex w-[min(380px,42%)] max-w-full flex-col border-l bg-card shadow-2xl" aria-label={`Intent details: ${inspectedNode.title ?? inspectedNode.name}`}>
          <div className="flex items-start justify-between gap-2 border-b p-3">
            <div className="min-w-0">
              <div className="text-[10px] uppercase tracking-wide text-muted-foreground">Intent details</div>
              <div className="mt-1 truncate text-sm font-semibold">{detail?.slug === inspectedSlug ? detail.title : inspectedNode.title ?? inspectedNode.name}</div>
              <div className="truncate font-mono text-[10px] text-muted-foreground">{inspectedNode.slug}</div>
            </div>
            <button type="button" className="rounded px-2 py-1 text-xs text-muted-foreground hover:bg-accent" aria-label="Close intent details" title="Close intent details" onClick={() => { setInspectedSlug(null); setExpandedDetails(false) }}>×</button>
          </div>
          <div className="min-h-0 flex-1 space-y-3 overflow-y-auto p-3 text-xs">
            {detailLoading ? <p role="status" className="text-muted-foreground">Loading intent details…</p> : null}
            {detail?.slug === inspectedSlug ? (
              <>
                <div className="flex gap-1.5"><span className="rounded bg-secondary px-2 py-0.5">{detail.status}</span><span className="rounded bg-secondary px-2 py-0.5">{detail.kind}</span></div>
                <p className="whitespace-pre-wrap leading-5 text-muted-foreground">{detail.summary || 'No summary is available.'}</p>
                {expandedDetails ? (
                  <>
                    <div className="rounded-md bg-muted/50 p-2"><div className="text-[10px] uppercase text-muted-foreground">Symbol</div><div className="mt-1 break-all font-mono">{detail.symbol || 'Not specified'}</div></div>
                    <div><div className="text-[10px] uppercase text-muted-foreground">Reviewers</div><div className="mt-1 break-words">{detail.reviewers.length ? detail.reviewers.join(', ') : 'None'}</div></div>
                    <div><div className="text-[10px] uppercase text-muted-foreground">Decision</div><pre className="mt-1 max-h-56 overflow-auto whitespace-pre-wrap break-words rounded-md bg-muted/50 p-2 font-sans leading-5">{detail.body || 'No decision text.'}</pre></div>
                    <div><div className="text-[10px] uppercase text-muted-foreground">depends_on · solid</div><div className="mt-1 break-words">{detail.dependsOn.join(', ') || 'None'}</div></div>
                    <div><div className="text-[10px] uppercase text-muted-foreground">references · dashed</div><div className="mt-1 break-words">{detail.references.join(', ') || 'None'}</div></div>
                  </>
                ) : null}
                <button type="button" className="text-primary underline-offset-2 hover:underline" onClick={() => setExpandedDetails((value) => !value)}>{expandedDetails ? 'Show summary' : 'Show full details'}</button>
                <div className="flex gap-2 border-t pt-3">
                  <button type="button" className="rounded border px-2.5 py-1.5 hover:bg-accent" onClick={onEditIntent} disabled={detail.kind !== 'leaf'}>Edit</button>
                  <button type="button" className="rounded border border-destructive/50 px-2.5 py-1.5 text-destructive hover:bg-destructive/10" onClick={onDeleteIntent} disabled={detail.kind !== 'leaf'}>Delete</button>
                </div>
              </>
            ) : !detailLoading ? <p className="text-muted-foreground">Details are not available for this node.</p> : null}
          </div>
        </aside>
      ) : null}
    </div>
  )
}
