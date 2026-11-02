import { useEffect, useMemo, useRef, useState, type MouseEvent, type ReactNode } from 'react'
import { CircleCheck, CircleDashed, CircleX, FileText, Folder, Lock, Unlock } from 'lucide-react'
import type { TreeNode } from '@/lib/mcp-types'
import { MarkdownPreview } from '@/components/MarkdownPreview'

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
  createdBy: string
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

/**
 * Canvas geometry follows `kron.webviewFontSize` so node boxes grow with the
 * text they contain. Produced from the current scale on every render so a
 * live font-size change does not need a Webview reload.
 */
type CanvasGeometry = {
  columnWidth: number
  rowHeight: number
  nodeWidth: number
  nodeHeight: number
  padding: number
  topInset: number
}

function geometryForScale(scale: number): CanvasGeometry {
  return {
    columnWidth: Math.round(300 * scale),
    rowHeight: Math.round(92 * scale),
    nodeWidth: Math.round(236 * scale),
    nodeHeight: Math.round(64 * scale),
    padding: Math.round(40 * scale),
    topInset: Math.round(104 * scale),
  }
}

function buildLayout(root: TreeNode, geometry: CanvasGeometry): CanvasLayout {
  const { columnWidth, rowHeight, nodeWidth, nodeHeight, padding } = geometry
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
      x: padding + depth * columnWidth,
      y: padding + (start + slots / 2) * rowHeight - nodeHeight / 2,
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
  const maxDepth = nodes.reduce((max, node) => Math.max(max, Math.round((node.x - padding) / columnWidth)), 0)
  return {
    nodes,
    edges,
    width: padding * 2 + maxDepth * columnWidth + nodeWidth,
    height: padding * 2 + Math.max(totalSlots, 1) * rowHeight,
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

type ParamKey = 'created' | 'review' | 'symbol' | 'depends' | 'references'

const PARAM_LABELS: Record<ParamKey, string> = {
  created: 'Created by',
  review: 'Reviewers',
  symbol: 'Symbol',
  depends: 'Depends on',
  references: 'References',
}

function toggleParam(current: ParamKey | null, next: ParamKey): ParamKey | null {
  return current === next ? null : next
}

function paramValue(detail: CanvasIntentDetail, key: ParamKey): string {
  switch (key) {
    case 'created': return detail.createdBy
    case 'review': return detail.reviewers.join(', ')
    case 'symbol': return detail.symbol
    case 'depends': return detail.dependsOn.join(', ')
    case 'references': return detail.references.join(', ')
  }
}

/** Fixed-type field marker: draft is hollow, active green, superseded red. */
function StatusDot({ status }: { status: string }): JSX.Element {
  if (status === 'active') return <span className="h-2 w-2 shrink-0 rounded-full bg-emerald-500" />
  if (status === 'superseded') return <span className="h-2 w-2 shrink-0 rounded-full bg-red-500" />
  return <span className="h-2 w-2 shrink-0 rounded-full border border-muted-foreground/70" />
}

/** Status chip: same height as every other fixed-type chip. */
function StatusChip({ status }: { status: string }): JSX.Element {
  return (
    <span className="flex shrink-0 items-center gap-1 rounded-md border bg-background px-1.5 py-1 text-[0.6875rem] text-muted-foreground" title={`Status: ${status}`}>
      <StatusDot status={status} />
      <span>{status}</span>
    </span>
  )
}

/** Shows the current value without its field name; clicking expands the details. */
function ParamChip({ label, value, expanded, onToggle }: { label: string; value: string; expanded: boolean; onToggle: () => void }): JSX.Element {
  const empty = value.trim() === ''
  return (
    <button
      type="button"
      className={`min-w-0 truncate rounded-md border px-1.5 py-1 text-left text-[0.6875rem] ${expanded ? 'border-primary/50 bg-primary/10 text-foreground' : 'bg-background text-muted-foreground hover:border-primary/40 hover:text-foreground'}`}
      aria-label={label}
      aria-expanded={expanded}
      title={empty ? label : `${label}: ${value}`}
      onClick={onToggle}
    >
      {empty ? '…' : value}
    </button>
  )
}

type IntentCanvasProps = {
  root: TreeNode
  filter: string
  selectedSlug: string
  /** 1 == default `kron.webviewFontSize`; scales canvas geometry with the text. */
  uiScale: number
  /** Incremented by the host / "used by" jumps to reveal the selected node's card. */
  revealToken: number
  relations: Record<string, { references: string[]; dependsOn: string[]; kind?: 'leaf' | 'node'; exists?: boolean }>
  detail: CanvasIntentDetail | null
  details: CanvasIntentDetail[]
  detailLoading: boolean
  onCreateRelation: (source: TreeNode, target: TreeNode, kind: LinkKind) => Promise<void>
  onCreateChild: (parent: TreeNode, kind: 'leaf' | 'node') => void
  onEditIntent: () => void
  onDeleteIntent: () => void
  onOpenCode: () => void
  editorContent: ReactNode | null
  assumptionContent: ReactNode | null
  onRequestCloseWindow: (slug: string) => Promise<boolean>
  onRequestSelectNode: (node: TreeNode) => Promise<boolean>
  onRequestCloseUnlocked: () => Promise<boolean>
  onCloseDetails: () => void
  onClearSelection: () => void
  onSelect: (node: TreeNode) => void
}

/** Left-to-right, pannable hierarchy canvas, adapted for the VS Code Webview tree contract. */
export function IntentCanvas({ root, filter, selectedSlug, uiScale, revealToken, relations, detail, details, detailLoading, onCreateRelation, onCreateChild, onEditIntent, onDeleteIntent, onOpenCode, editorContent, assumptionContent, onRequestCloseWindow, onRequestSelectNode, onRequestCloseUnlocked, onCloseDetails, onClearSelection, onSelect }: IntentCanvasProps): JSX.Element {
  const geometry = useMemo(() => geometryForScale(uiScale), [uiScale])
  const layout = useMemo(() => buildLayout({
    ...root,
    title: root.title ?? root.name,
    children: root.children ?? [],
  }, geometry), [root, geometry])
  const [pan, setPan] = useState({ x: 0, y: 0 })
  const [viewportSize, setViewportSize] = useState({ width: 0, height: 0 })
  const [panning, setPanning] = useState(false)
  const [openWindows, setOpenWindows] = useState<{ slug: string; offset: { x: number; y: number }; locked: boolean }[]>([])
  const [draggingSlug, setDraggingSlug] = useState<string | null>(null)
  const [expandedParam, setExpandedParam] = useState<ParamKey | null>(null)
  const [contextMenu, setContextMenu] = useState<{ node: TreeNode; x: number; y: number } | null>(null)
  const [linking, setLinking] = useState<{ source: TreeNode; kind: LinkKind; cursor: { x: number; y: number } } | null>(null)
  const [linkSaving, setLinkSaving] = useState(false)
  const [linkError, setLinkError] = useState<string | null>(null)
  const [linkMessage, setLinkMessage] = useState<string | null>(null)
  const containerRef = useRef<HTMLDivElement | null>(null)
  const viewportRef = useRef<HTMLDivElement | null>(null)
  const dragRef = useRef<{ x: number; y: number; originX: number; originY: number; moved: boolean } | null>(null)
  const detailDragRef = useRef<{ slug: string; x: number; y: number; originX: number; originY: number } | null>(null)
  const query = filter.trim().toLowerCase()
  const intents = countLeafIntents(root)
  const hasChildren = (root.children?.length ?? 0) > 0
  const fitScale = viewportSize.width > 0 && viewportSize.height > 0
    ? Math.max(0.1, Math.min(1, (viewportSize.width - 32) / layout.width, (viewportSize.height - geometry.topInset - 24) / layout.height))
    : 1
  const fitOffset = {
    x: Math.max(16, (viewportSize.width - layout.width * fitScale) / 2),
    y: geometry.topInset + Math.max(12, (viewportSize.height - geometry.topInset - layout.height * fitScale) / 2),
  }

  const closeUnlockedWindows = async (): Promise<boolean> => {
    const kept = openWindows.filter((window) => window.locked)
    if (kept.length === openWindows.length) return true
    const activeClosed = selectedSlug !== '' && !kept.some((window) => window.slug === selectedSlug)
    if (activeClosed && editorContent !== null) {
      const proceed = await onRequestCloseUnlocked()
      if (!proceed) return false
    }
    setOpenWindows(kept)
    if (activeClosed) {
      onCloseDetails()
      onClearSelection()
    }
    return true
  }

  const openWindow = (slug: string): void => {
    setOpenWindows((current) => current.some((window) => window.slug === slug)
      ? current
      : [...current, { slug, offset: { x: 0, y: 0 }, locked: false }])
  }

  const cardRect = (position: { x: number; y: number }, offset: { x: number; y: number }, width: number): { left: number; top: number } => {
    const left = fitOffset.x + pan.x + position.x * fitScale + offset.x
    const top = fitOffset.y + pan.y + (position.y + geometry.nodeHeight + 12) * fitScale + offset.y
    return {
      left: viewportSize.width > 0 ? Math.max(8, Math.min(left, viewportSize.width - width - 8)) : left,
      top: viewportSize.height > 0 ? Math.max(8, Math.min(top, viewportSize.height - 96)) : top,
    }
  }

  // External jumps (host "Open Intent", assumption "Used by") reveal the card
  // for the already-selected node; ordinary clicking is handled by the node.
  useEffect(() => {
    if (revealToken === 0 || selectedSlug === '') return
    if (!layout.nodes.some(({ node }) => node.slug === selectedSlug)) return
    openWindow(selectedSlug)
  }, [revealToken])

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
        setExpandedParam(null)
        void closeUnlockedWindows()
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [])

  // Context menus close on the first click outside the menu. The node click
  // handler can then open the next node's details in the same event sequence.
  useEffect(() => {
    if (contextMenu === null) return
    const onPointerDown = (event: PointerEvent): void => {
      const target = event.target as HTMLElement | null
      if (target?.closest('[data-canvas-overlay="true"]') !== null) return
      setContextMenu(null)
    }
    document.addEventListener('pointerdown', onPointerDown)
    return () => document.removeEventListener('pointerdown', onPointerDown)
  }, [contextMenu])

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
      const drag = dragRef.current
      dragRef.current = null
      setPanning(false)
      setExpandedParam(null)
      void closeUnlockedWindows()
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
    setLinking({ source: contextMenu.node, kind, cursor: { x: source.x + geometry.nodeWidth, y: source.y + geometry.nodeHeight / 2 } })
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
              const x1 = parent.x + geometry.nodeWidth
              const y1 = parent.y + geometry.nodeHeight / 2
              const x2 = child.x
              const y2 = child.y + geometry.nodeHeight / 2
              const bend = (x2 - x1) / 2
              return <path key={`${parent.node.slug}->${child.node.slug}`} d={`M ${x1} ${y1} C ${x1 + bend} ${y1}, ${x2 - bend} ${y2}, ${x2} ${y2}`} fill="none" stroke="hsl(var(--border))" strokeWidth="1.5" />
            })}
            {relationEdges.map(({ from, to, kind }) => {
              const isDepends = kind === 'depends'
              const x1 = from.x + geometry.nodeWidth
              const y1 = from.y + geometry.nodeHeight / 2 + (isDepends ? -3 : 3)
              const x2 = to.x
              const y2 = to.y + geometry.nodeHeight / 2 + (isDepends ? -3 : 3)
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
              const x1 = source.x + geometry.nodeWidth
              const y1 = source.y + geometry.nodeHeight / 2
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
                  <span className="block truncate text-[0.625rem] text-muted-foreground">{isWorkspace ? `${intents} intents` : isGroup ? `group · ${countLeafIntents(node)} intents` : node.slug}</span>
                </span>
                {!isGroup ? statusIcon(node.status) : null}
              </>
            )
            const classes = `absolute flex items-center gap-2 rounded-xl border px-3 shadow-sm transition-all ${isGroup ? 'border-border/80 bg-secondary/90 text-secondary-foreground' : 'border-border/70 bg-card text-card-foreground'} ${dimmed ? 'opacity-35' : 'opacity-100'} ${selected ? 'ring-2 ring-primary ring-offset-2 ring-offset-background' : 'hover:border-primary/50 hover:shadow-md'}`
            const styles = {
              left: x,
              top: y,
              width: geometry.nodeWidth,
              height: geometry.nodeHeight,
              ...(relationFrame !== undefined && !selected ? {
                outline: `2px ${relationFrame === 'depends' ? 'solid' : 'dashed'} hsl(${relationFrame === 'depends' ? 'var(--primary)' : 'var(--muted-foreground)'})`,
                outlineOffset: '3px',
              } : {}),
            }

            return isWorkspace ? (
              <button
                key={`workspace:${node.workspaceId ?? node.slug}`}
                type="button"
                className={classes}
                style={styles}
                aria-label={`Select workspace ${title}`}
                onClick={() => {
                  void (async () => {
                    const proceed = await closeUnlockedWindows()
                    if (!proceed) return
                    const selected = await onRequestSelectNode(node)
                    if (!selected) return
                    setExpandedParam(null)
                  })()
                }}
              >{card}</button>
            ) : (
              <button
                key={`${node.workspaceId ?? ''}:${node.slug}`}
                type="button"
                data-intent-node="true"
                className={classes}
                style={styles}
                aria-pressed={selected}
                onClick={() => {
                  void (async () => {
                    if (linking !== null) {
                      chooseLinkTarget(node)
                      return
                    }
                    const existing = openWindows.find((window) => window.slug === node.slug)
                    if (existing !== undefined) {
                      if (selectedSlug === node.slug) {
                        if (existing.locked) {
                          setOpenWindows((current) => current.map((window) => window.slug === node.slug ? { ...window, locked: false } : window))
                        } else {
                          if (editorContent !== null) {
                            const proceed = await onRequestCloseWindow(node.slug)
                            if (!proceed) return
                          }
                          setOpenWindows((current) => current.filter((window) => window.slug !== node.slug))
                          setExpandedParam(null)
                          onCloseDetails()
                          onClearSelection()
                        }
                        return
                      }
                      if (editorContent !== null) {
                        const proceed = await onRequestSelectNode(node)
                        if (!proceed) return
                      } else {
                        onSelect(node)
                      }
                      return
                    }
                    if (editorContent !== null) {
                      const proceed = await onRequestSelectNode(node)
                      if (!proceed) return
                    } else {
                      onSelect(node)
                    }
                    openWindow(node.slug)
                    setExpandedParam(null)
                  })()
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
          <div className="px-3 pt-2 text-[0.625rem] text-muted-foreground">{contextMenu.node.is_dir ? `Parent: ${contextMenu.node.slug}` : 'Leaf intents cannot contain children'}</div>
          {contextMenu.node.is_dir ? <>
            <button type="button" className="rounded px-3 py-2 text-left text-xs hover:bg-accent" onClick={() => { onCreateChild(contextMenu.node, 'leaf'); setContextMenu(null) }}>Create child document</button>
            <button type="button" className="rounded px-3 py-2 text-left text-xs hover:bg-accent" onClick={() => { onCreateChild(contextMenu.node, 'node'); setContextMenu(null) }}>Create child folder</button>
          </> : null}
          {!contextMenu.node.is_dir || relations[contextMenu.node.slug]?.exists === true ? (
            <>
              {contextMenu.node.is_dir || relations[contextMenu.node.slug]?.exists === true ? <div className="my-0.5 border-t" /> : null}
              <button type="button" className="rounded px-3 py-2 text-left text-xs hover:bg-accent" onClick={() => beginLink('depends')}>Create hard dependency · solid</button>
              <button type="button" className="rounded px-3 py-2 text-left text-xs hover:bg-accent" onClick={() => beginLink('references')}>Create reference · dashed</button>
            </>
          ) : null}
        </div>
      ) : null}
      <div className="pointer-events-none absolute right-2 top-[3.75rem] flex flex-wrap items-center justify-end gap-3 rounded bg-background/90 px-2 py-1 text-[0.625rem] text-muted-foreground">
          <span className="flex items-center gap-1"><svg width="24" height="8" aria-hidden="true"><line x1="1" y1="4" x2="21" y2="4" stroke="hsl(var(--primary))" strokeWidth="2" /></svg>depends_on (hard)</span>
          <span className="flex items-center gap-1"><svg width="24" height="8" aria-hidden="true"><line x1="1" y1="4" x2="21" y2="4" stroke="hsl(var(--muted-foreground))" strokeWidth="1.5" strokeDasharray="6 4" /></svg>references (soft)</span>
      </div>
      {hasChildren && relationEdges.length === 0 ? (
        <div className="pointer-events-none absolute bottom-2 right-2 rounded bg-background/90 px-2 py-1 text-[0.625rem] text-muted-foreground">
          No cross-intent links yet. Add them with Edit intent → depends_on or references.
        </div>
      ) : null}
      <div className="pointer-events-none absolute bottom-2 left-2 rounded bg-background/85 px-2 py-1 text-[0.625rem] text-muted-foreground">
        Drag empty space to pan · click a node for details · right-click a node to link
      </div>
      </div>
      {openWindows.map((window) => {
        const position = layout.nodes.find(({ node }) => node.slug === window.slug)
        if (position === undefined) return null
        const isActive = window.slug === selectedSlug
        const windowDetail = details.find((item) => item.slug === window.slug)
          ?? (detail?.slug === window.slug ? detail : null)
        const windowEditor = isActive ? editorContent : null
        const windowAssumption = isActive ? assumptionContent : null
        const windowWidth = Math.round((windowEditor !== null ? 760 : 380) * uiScale)
        const rect = cardRect(position, window.offset, windowWidth)
        return (
        <aside
          key={window.slug}
          data-canvas-overlay="true"
          className={`absolute z-40 flex max-h-[78vh] max-w-[calc(100vw-1.5rem)] flex-col overflow-hidden rounded-xl border bg-card shadow-2xl ${draggingSlug === window.slug ? 'cursor-grabbing select-none' : 'cursor-grab'}`}
          style={{ left: rect.left, top: rect.top, width: windowWidth }}
          onMouseDown={(event) => {
            const target = event.target as HTMLElement
            if (target.closest('button, a, input, textarea, select, code') !== null) return
            event.stopPropagation()
            detailDragRef.current = { slug: window.slug, x: event.clientX, y: event.clientY, originX: window.offset.x, originY: window.offset.y }
            setDraggingSlug(window.slug)
          }}
          onMouseMove={(event) => {
            const drag = detailDragRef.current
            if (drag === null) return
            event.stopPropagation()
            setOpenWindows((current) => current.map((item) => item.slug === window.slug
              ? { ...item, offset: { x: drag.originX + event.clientX - drag.x, y: drag.originY + event.clientY - drag.y } }
              : item))
          }}
          onMouseUp={(event) => {
            if (detailDragRef.current === null) return
            event.stopPropagation()
            detailDragRef.current = null
            setDraggingSlug(null)
          }}
          onMouseLeave={() => {
            if (detailDragRef.current !== null) {
              detailDragRef.current = null
              setDraggingSlug(null)
            }
          }}
          aria-label={`Intent details: ${position.node.title ?? position.node.name}`}
        >
          <div className="absolute right-1.5 top-1.5 z-10 flex items-center gap-0.5">
            <button
              type="button"
              className="rounded px-1.5 py-0.5 text-xs text-muted-foreground hover:bg-accent"
              aria-label={window.locked ? 'Unlock intent details' : 'Lock intent details'}
              title={window.locked ? 'Unlock intent details' : 'Lock intent details'}
              onClick={() => setOpenWindows((current) => current.map((item) => item.slug === window.slug ? { ...item, locked: !item.locked } : item))}
            >{window.locked ? <Lock className="h-3 w-3"/> : <Unlock className="h-3 w-3"/>}</button>
            <button
              type="button"
              className="rounded px-1.5 py-0.5 text-xs text-muted-foreground hover:bg-accent"
              aria-label="Close intent details"
              title="Close intent details"
              onClick={() => {
                void (async () => {
                  if (isActive && editorContent !== null) {
                    const proceed = await onRequestCloseWindow(window.slug)
                    if (!proceed) return
                  }
                  setOpenWindows((current) => current.filter((item) => item.slug !== window.slug))
                  if (isActive) {
                    onCloseDetails()
                    onClearSelection()
                  }
                })()
              }}
            >×</button>
          </div>
          <div className="min-h-0 flex-1 space-y-2 overflow-y-auto p-2.5">
            {windowEditor !== null ? windowEditor : null}
            {isActive && detailLoading ? <p role="status" className="text-muted-foreground">Loading intent details…</p> : null}
            {windowEditor === null && windowDetail !== null ? (
              <>
                <p className="pr-5 text-lg font-semibold leading-7">{windowDetail.title}</p>
                <p className="whitespace-pre-wrap text-[0.9375rem] leading-6 text-muted-foreground">{windowDetail.summary || 'No summary is available.'}</p>
                <div className="flex max-w-full flex-wrap items-center gap-1">
                  <StatusChip status={windowDetail.status} />
                  <ParamChip label={PARAM_LABELS.created} value={windowDetail.createdBy} expanded={expandedParam === 'created'} onToggle={() => setExpandedParam(toggleParam(expandedParam, 'created'))} />
                  <ParamChip label={PARAM_LABELS.review} value={windowDetail.reviewers.join(', ')} expanded={expandedParam === 'review'} onToggle={() => setExpandedParam(toggleParam(expandedParam, 'review'))} />
                </div>
                <div className="grid grid-cols-3 gap-1">
                  <ParamChip label={PARAM_LABELS.symbol} value={windowDetail.symbol} expanded={expandedParam === 'symbol'} onToggle={() => setExpandedParam(toggleParam(expandedParam, 'symbol'))} />
                  <ParamChip label={PARAM_LABELS.depends} value={windowDetail.dependsOn.join(', ')} expanded={expandedParam === 'depends'} onToggle={() => setExpandedParam(toggleParam(expandedParam, 'depends'))} />
                  <ParamChip label={PARAM_LABELS.references} value={windowDetail.references.join(', ')} expanded={expandedParam === 'references'} onToggle={() => setExpandedParam(toggleParam(expandedParam, 'references'))} />
                </div>
                {expandedParam !== null ? (
                  <div className="rounded-md bg-muted/50 px-2 py-1.5 text-xs">
                    <div className="text-[0.625rem] uppercase tracking-wide text-muted-foreground">{PARAM_LABELS[expandedParam]}</div>
                    <div className="mt-0.5 whitespace-pre-wrap break-words">{paramValue(windowDetail, expandedParam) || 'Not specified'}</div>
                  </div>
                ) : null}
                {windowAssumption}
                <div className="max-h-40 max-w-full overflow-auto rounded-md bg-muted/40 p-2.5 text-sm leading-6">
                  <MarkdownPreview body={windowDetail.body || 'No body.'} />
                </div>
                {isActive ? (
                  <div className="flex gap-2 border-t pt-2.5">
                    <button
                      type="button"
                      className="rounded border px-2.5 py-1.5 hover:bg-accent"
                      onClick={onOpenCode}
                      disabled={windowDetail.slug === ''}
                      title="Open one of this intent's @kron:intent code anchors"
                    >
                      Code
                    </button>
                    <button type="button" className="rounded border px-2.5 py-1.5 hover:bg-accent" onClick={onEditIntent}>Edit</button>
                    <button
                      type="button"
                      className="rounded border border-destructive/50 px-2.5 py-1.5 text-destructive hover:bg-destructive/10"
                      onClick={onDeleteIntent}
                      disabled={windowDetail.slug === ''}
                      title={windowDetail.kind === 'node' ? 'Moves this node and its whole subtree to trash' : 'Moves this intent to trash'}
                    >
                      Delete
                    </button>
                  </div>
                ) : null}
              </>
            ) : windowEditor === null && !(isActive && detailLoading) ? <p className="text-muted-foreground">Details are not available for this node.</p> : null}
          </div>
        </aside>
        )
      })}
    </div>
  )
}
