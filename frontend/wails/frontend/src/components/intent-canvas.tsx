// Intent canvas — the read-only, left-to-right intent tree.
//
// Design constraints (user spec, 2026-10-20):
//   - It is an INTENT tree, not a file explorer: nodes carry the intent
//     title; branch nodes are module groups, leaf nodes are intents.
//   - root → leaf flows left to right.
//   - dot-grid background; the whole canvas can be dragged (pan). No CRUD.
//   - clicking a node opens an info card BELOW the node (not a modal);
//     clicking outside it or its × closes the card.
//
// The component is purely presentational: it never mutates intent data and
// never calls a write bridge. Data still comes from kron_tree / kron_get.

import {useCallback, useEffect, useMemo, useRef, useState} from 'react';
import {
    CircleAlert,
    CircleCheck,
    CircleDashed,
    FileText,
    Folder,
    LocateFixed,
    PencilLine,
    X,
} from 'lucide-react';
import {cn} from '@/lib/utils';
import type {IntentNode, IntentStatus} from '@/lib/mock';

interface IntentCanvasProps {
    data: IntentNode[];
    projectName: string;
    filter: string;
    /** Currently selected node id (controlled by App so the left rail can drive it). */
    selectedId: string | null;
    onSelectedIdChange: (id: string | null) => void;
    /** Increment to ask the canvas to center the selected node (left-rail click). */
    focusRequest: number;
}

interface LaidOutNode {
    node: IntentNode;
    depth: number;
    x: number;
    y: number;
    isBranch: boolean;
}

interface LaidOutEdge {
    from: LaidOutNode;
    to: LaidOutNode;
}

interface Layout {
    nodes: LaidOutNode[];
    edges: LaidOutEdge[];
    width: number;
    height: number;
}

const COLUMN_W = 280;
const ROW_H = 84;
const NODE_W = 212;
const NODE_H = 50;
const PAD = 48;

function layoutTree(root: IntentNode): Layout {
    const nodes: LaidOutNode[] = [];
    const edges: LaidOutEdge[] = [];

    // Classic two-pass tidy layout: measure each subtree in "slots"
    // (a leaf is 1 slot; a branch is the sum of its children), then place
    // children in consecutive slot ranges and center the parent on its own
    // range. This guarantees that siblings never overlap, including the
    // tricky case of a leaf sibling next to a multi-leaf branch.
    const measure = (node: IntentNode): number => {
        if (node.children.length === 0) return 1;
        return node.children.reduce((sum, child) => sum + measure(child), 0);
    };

    const place = (node: IntentNode, depth: number, startSlot: number, slots: number): LaidOutNode => {
        const isBranch = node.children.length > 0;
        const laid: LaidOutNode = {
            node,
            depth,
            x: PAD + depth * COLUMN_W,
            y: PAD + (startSlot + slots / 2) * ROW_H,
            isBranch,
        };
        nodes.push(laid);
        let childStart = startSlot;
        for (const child of node.children) {
            const childSlots = measure(child);
            const childLaid = place(child, depth + 1, childStart, childSlots);
            edges.push({from: laid, to: childLaid});
            childStart += childSlots;
        }
        return laid;
    };

    const totalSlots = measure(root);
    const rootLaid = place(root, 0, 0, totalSlots);
    const maxDepth = nodes.reduce((max, n) => Math.max(max, n.depth), rootLaid.depth);
    return {
        nodes,
        edges,
        width: PAD * 2 + maxDepth * COLUMN_W + NODE_W,
        height: PAD * 2 + Math.max(totalSlots, 1) * ROW_H,
    };
}

const statusDot: Record<IntentStatus, JSX.Element> = {
    draft: <CircleDashed className="h-3 w-3 text-muted-foreground"/>,
    review: <PencilLine className="h-3 w-3 text-amber-400"/>,
    accepted: <CircleCheck className="h-3 w-3 text-emerald-400"/>,
    stale: <CircleAlert className="h-3 w-3 text-destructive"/>,
};

const statusLabel: Record<IntentStatus, string> = {
    draft: 'Draft',
    review: 'Review',
    accepted: 'Accepted',
    stale: 'Stale',
};

function relativeDate(iso: string): string | null {
    if (iso === '') return null;
    const then = new Date(iso).getTime();
    if (!Number.isFinite(then) || then <= 86_400_000) return null;
    const days = Math.round((Date.now() - then) / 86_400_000);
    if (days <= 0) return 'today';
    if (days === 1) return '1d ago';
    if (days < 30) return `${days}d ago`;
    if (days < 365) return `${Math.round(days / 30)}mo ago`;
    return `${Math.round(days / 365)}y ago`;
}

/** True when the node itself, or any descendant, matches the filter. */
function matchesFilter(node: IntentNode, query: string): boolean {
    if (query === '') return true;
    const haystack = `${node.title} ${node.id} ${node.summary ?? ''}`.toLowerCase();
    if (haystack.includes(query)) return true;
    return node.children.some((child) => matchesFilter(child, query));
}

function countIntents(node: IntentNode): number {
    if (node.children.length === 0) return 1;
    return node.children.reduce((sum, child) => sum + countIntents(child), 0);
}

/** "1 intent" / "3 intents" — the canvas labels groups with this count. */
function intentsLabel(count: number): string {
    return `${count} intent${count === 1 ? '' : 's'}`;
}

/** One row of related-intent chips (references / depends_on). */
function RelationRow(props: {
    label: string;
    slugs: string[];
    onNavigate: (id: string) => void;
}): JSX.Element {
    const {label, slugs, onNavigate} = props;
    return (
        <div className="flex flex-wrap items-center gap-1">
            <span className="text-muted-foreground/70">{label}</span>
            {slugs.map((slug) => (
                <button
                    key={slug}
                    type="button"
                    className="rounded bg-muted px-1.5 py-0.5 font-mono text-[10px] hover:bg-accent hover:text-foreground"
                    title={`Go to ${slug}`}
                    onClick={(event): void => {
                        event.stopPropagation();
                        onNavigate(slug);
                    }}
                >
                    {slug}
                </button>
            ))}
        </div>
    );
}

export function IntentCanvas({data, projectName, filter, selectedId, onSelectedIdChange, focusRequest}: IntentCanvasProps): JSX.Element {
    const [pan, setPan] = useState<{x: number; y: number}>({x: 24, y: 16});
    const [panning, setPanning] = useState<boolean>(false);
    const containerRef = useRef<HTMLDivElement | null>(null);
    const dragRef = useRef<{startX: number; startY: number; originX: number; originY: number; moved: boolean} | null>(null);
    const panelRef = useRef<HTMLDivElement | null>(null);

    const query = filter.trim().toLowerCase();

    const layout = useMemo(() => {
        const root: IntentNode = {
            id: '__kron_root__',
            title: projectName,
            kind: 'spec',
            status: 'accepted',
            updatedAt: '',
            children: data,
        };
        return layoutTree(root);
    }, [data, projectName]);

    const selected = useMemo(
        () => layout.nodes.find((n) => n.node.id === selectedId) ?? null,
        [layout.nodes, selectedId],
    );

    // Cross-intent relations are NOT hierarchy edges: references are soft links
    // (dashed arrow), depends_on are hard prerequisites (solid arrow). Only the
    // endpoints that exist in the current tree are drawn.
    const relationEdges = useMemo(() => {
        const byId = new Map(layout.nodes.map((laid) => [laid.node.id, laid]));
        const out: Array<{from: LaidOutNode; to: LaidOutNode; kind: 'depends' | 'references'}> = [];
        for (const laid of layout.nodes) {
            if (laid.isBranch) continue;
            for (const slug of laid.node.dependsOn ?? []) {
                const target = byId.get(slug);
                if (target !== undefined && target.node.id !== laid.node.id) {
                    out.push({from: laid, to: target, kind: 'depends'});
                }
            }
            for (const slug of laid.node.references ?? []) {
                const target = byId.get(slug);
                if (target !== undefined && target.node.id !== laid.node.id) {
                    out.push({from: laid, to: target, kind: 'references'});
                }
            }
        }
        return out;
    }, [layout.nodes]);

    // Center the selected node only when the click came from outside the canvas
    // (left-rail row). Clicking a node inside the canvas must not move the view.
    const focusState = useRef<{selectedId: string | null; nodes: LaidOutNode[]}>({
        selectedId: null,
        nodes: [],
    });
    focusState.current = {selectedId, nodes: layout.nodes};
    useEffect(() => {
        const {selectedId: id, nodes} = focusState.current;
        if (id === null) return;
        const container = containerRef.current;
        const node = nodes.find((n) => n.node.id === id);
        if (container === null || node === undefined) return;
        const rect = container.getBoundingClientRect();
        setPan({
            x: rect.width / 2 - (node.x + NODE_W / 2),
            y: rect.height / 2 - (node.y + NODE_H / 2),
        });
    }, [focusRequest]);

    const resetView = useCallback((): void => {
        setPan({x: 24, y: 16});
        onSelectedIdChange(null);
    }, [onSelectedIdChange]);

    // Close the info card on outside click / Escape.
    useEffect(() => {
        if (selectedId === null) return;
        const onPointerDown = (event: PointerEvent): void => {
            const target = event.target as HTMLElement | null;
            if (panelRef.current !== null && target !== null && panelRef.current.contains(target)) return;
            if (target !== null && target.closest('[data-intent-node="true"]') !== null) return;
            onSelectedIdChange(null);
        };
        const onKeyDown = (event: KeyboardEvent): void => {
            if (event.key === 'Escape') onSelectedIdChange(null);
        };
        document.addEventListener('pointerdown', onPointerDown);
        document.addEventListener('keydown', onKeyDown);
        return () => {
            document.removeEventListener('pointerdown', onPointerDown);
            document.removeEventListener('keydown', onKeyDown);
        };
    }, [selectedId]);

    // Pan with plain mouse events: Wails' WebView2 does not deliver
    // PointerEvents for injected/assisted input in every host, and mouse
    // events cover the desktop use case without pointer-capture subtleties.
    useEffect(() => {
        if (!panning) return;
        const onMove = (event: MouseEvent): void => {
            const drag = dragRef.current;
            if (drag === null) return;
            const dx = event.clientX - drag.startX;
            const dy = event.clientY - drag.startY;
            if (Math.abs(dx) > 3 || Math.abs(dy) > 3) drag.moved = true;
            setPan({x: drag.originX + dx, y: drag.originY + dy});
        };
        const onUp = (): void => {
            const drag = dragRef.current;
            dragRef.current = null;
            setPanning(false);
            // A plain click on empty canvas dismisses the info card.
            if (drag !== null && !drag.moved) onSelectedIdChange(null);
        };
        window.addEventListener('mousemove', onMove);
        window.addEventListener('mouseup', onUp);
        return () => {
            window.removeEventListener('mousemove', onMove);
            window.removeEventListener('mouseup', onUp);
        };
    }, [panning]);

    const handleMouseDown = (event: React.MouseEvent<HTMLDivElement>): void => {
        const target = event.target as HTMLElement;
        if (target.closest('[data-intent-node="true"]') !== null) return;
        if (target.closest('[data-intent-panel="true"]') !== null) return;
        dragRef.current = {
            startX: event.clientX,
            startY: event.clientY,
            originX: pan.x,
            originY: pan.y,
            moved: false,
        };
        setPanning(true);
    };

    return (
        <div ref={containerRef} className="relative h-full w-full overflow-hidden">
            {/* Dot-grid background; the grid moves with the canvas so panning
                feels like moving a board rather than scrolling a list. */}
            <div
                className={cn('absolute inset-0 touch-none', panning ? 'cursor-grabbing' : 'cursor-grab')}
                style={{
                    backgroundColor: 'hsl(var(--background))',
                    backgroundImage:
                        'radial-gradient(circle, hsl(var(--border) / 0.85) 1px, transparent 1px)',
                    backgroundSize: '22px 22px',
                    backgroundPosition: `${pan.x}px ${pan.y}px`,
                }}
                onMouseDown={handleMouseDown}
            >
                <div
                    className="absolute left-0 top-0"
                    style={{
                        transform: `translate(${pan.x}px, ${pan.y}px)`,
                        width: layout.width,
                        height: layout.height,
                    }}
                >
                    {/* Edges */}
                    <svg
                        className="pointer-events-none absolute left-0 top-0"
                        width={layout.width + 280}
                        height={layout.height + 280}
                        aria-hidden="true"
                    >
                        <defs>
                            <marker
                                id="kron-arrow-depends"
                                viewBox="0 0 10 10"
                                refX="9"
                                refY="5"
                                markerWidth="6"
                                markerHeight="6"
                                orient="auto-start-reverse"
                            >
                                <path d="M 0 0 L 10 5 L 0 10 z" fill="hsl(var(--primary))"/>
                            </marker>
                            <marker
                                id="kron-arrow-references"
                                viewBox="0 0 10 10"
                                refX="9"
                                refY="5"
                                markerWidth="6"
                                markerHeight="6"
                                orient="auto-start-reverse"
                            >
                                <path
                                    d="M 0 0 L 10 5 L 0 10"
                                    fill="none"
                                    stroke="hsl(var(--muted-foreground))"
                                    strokeWidth={1.5}
                                />
                            </marker>
                        </defs>

                        {/* Structural parent → child edges. */}
                        {layout.edges.map((edge) => {
                            const x1 = edge.from.x + NODE_W;
                            const y1 = edge.from.y + NODE_H / 2;
                            const x2 = edge.to.x;
                            const y2 = edge.to.y + NODE_H / 2;
                            const mid = (x2 - x1) / 2;
                            return (
                                <path
                                    key={`${edge.from.node.id}->${edge.to.node.id}`}
                                    d={`M ${x1} ${y1} C ${x1 + mid} ${y1}, ${x2 - mid} ${y2}, ${x2} ${y2}`}
                                    fill="none"
                                    stroke="hsl(var(--border))"
                                    strokeWidth={1.5}
                                />
                            );
                        })}

                        {/* Cross-intent relations. These are NOT hierarchy edges:
                            references (soft) = dashed thin arrow,
                            depends_on (hard) = solid thicker arrow. */}
                        {relationEdges.map((edge) => {
                            const isDepends = edge.kind === 'depends';
                            const x1 = edge.from.x + NODE_W / 2;
                            const y1 = edge.from.y + NODE_H;
                            const x2 = edge.to.x + NODE_W / 2;
                            const y2 = edge.to.y;
                            // Same-column edges (the common case for siblings at
                            // one depth) must bow sideways, otherwise the curve
                            // runs straight through the nodes stacked between
                            // the endpoints.
                            const horizontallySeparated =
                                Math.abs(x2 - x1) >= Math.abs(y2 - y1);
                            const bowAmount = horizontallySeparated ? 80 : 150;
                            const bow = (isDepends ? 1 : -1) * bowAmount;
                            const controlX = horizontallySeparated
                                ? (x1 + x2) / 2
                                : (x1 + x2) / 2 + bow;
                            const controlY = horizontallySeparated
                                ? (y1 + y2) / 2 + bow
                                : (y1 + y2) / 2;
                            const active =
                                selectedId === edge.from.node.id || selectedId === edge.to.node.id;
                            return (
                                <path
                                    key={`rel-${edge.kind}-${edge.from.node.id}->${edge.to.node.id}`}
                                    d={`M ${x1} ${y1} Q ${controlX} ${controlY} ${x2} ${y2}`}
                                    fill="none"
                                    stroke={
                                        isDepends
                                            ? 'hsl(var(--primary))'
                                            : 'hsl(var(--muted-foreground))'
                                    }
                                    strokeWidth={isDepends ? 2 : 1.5}
                                    strokeDasharray={isDepends ? undefined : '6 4'}
                                    markerEnd={
                                        isDepends
                                            ? 'url(#kron-arrow-depends)'
                                            : 'url(#kron-arrow-references)'
                                    }
                                    opacity={active ? 1 : 0.55}
                                />
                            );
                        })}
                    </svg>

                    {/* Nodes */}
                    {layout.nodes.map((laid) => {
                        const dimmed = !matchesFilter(laid.node, query);
                        const isSelected = laid.node.id === selectedId;
                        return (
                            <div
                                key={laid.node.id}
                                data-intent-node="true"
                                data-testid={`intent-node-${laid.node.id}`}
                                role="button"
                                tabIndex={0}
                                aria-label={laid.node.title}
                                className={cn(
                                    'absolute flex select-none items-center gap-2 rounded-xl border px-3 shadow-sm',
                                    'transition-all duration-150',
                                    laid.isBranch
                                        ? 'border-border/80 bg-secondary/80 text-secondary-foreground'
                                        : 'border-border/70 bg-card text-card-foreground',
                                    dimmed ? 'opacity-35' : 'opacity-100',
                                    isSelected
                                        ? 'ring-2 ring-primary ring-offset-2 ring-offset-background'
                                        : 'hover:border-primary/50 hover:shadow-md',
                                )}
                                style={{left: laid.x, top: laid.y, width: NODE_W, height: NODE_H}}
                                onClick={(event): void => {
                                    event.stopPropagation();
                                    onSelectedIdChange(isSelected ? null : laid.node.id);
                                }}
                                onKeyDown={(event): void => {
                                    if (event.key === 'Enter' || event.key === ' ') {
                                        event.preventDefault();
                                        onSelectedIdChange(isSelected ? null : laid.node.id);
                                    }
                                }}
                            >
                                {laid.isBranch ? (
                                    <Folder className="h-4 w-4 shrink-0 text-muted-foreground"/>
                                ) : (
                                    <FileText className="h-4 w-4 shrink-0 text-muted-foreground"/>
                                )}
                                <div className="min-w-0 flex-1">
                                    <div className="truncate text-sm font-medium leading-tight">
                                        {laid.node.title}
                                    </div>
                                    {laid.isBranch ? (
                                        <div className="text-[10px] text-muted-foreground">
                                            group · {intentsLabel(countIntents(laid.node))}
                                        </div>
                                    ) : (
                                        <div className="truncate text-[10px] text-muted-foreground">
                                            {laid.node.id}
                                        </div>
                                    )}
                                </div>
                                {laid.isBranch ? null : (
                                    <span title={statusLabel[laid.node.status]}>
                                        {statusDot[laid.node.status]}
                                    </span>
                                )}
                            </div>
                        );
                    })}

                    {/* Info card: expands below the selected node. */}
                    {selected !== null && (
                        <div
                            ref={panelRef}
                            data-intent-panel="true"
                            role="dialog"
                            aria-label={`${selected.node.title} details`}
                            className="absolute z-20 w-[360px] max-h-[70vh] overflow-y-auto rounded-xl border border-border bg-popover p-3.5 text-popover-foreground shadow-xl"
                            style={{left: selected.x, top: selected.y + NODE_H + 12}}
                        >
                            <div className="flex items-start gap-2">
                                <div className="min-w-0 flex-1">
                                    <div className="truncate text-sm font-semibold">{selected.node.title}</div>
                                    <div className="mt-0.5 truncate text-[10px] text-muted-foreground">
                                        {selected.isBranch
                                            ? `${intentsLabel(countIntents(selected.node))} in this group`
                                            : selected.node.id}
                                    </div>
                                </div>
                                <button
                                    type="button"
                                    aria-label="Close details"
                                    className="rounded-md p-1 text-muted-foreground hover:bg-accent hover:text-foreground"
                                    onClick={(): void => onSelectedIdChange(null)}
                                >
                                    <X className="h-3.5 w-3.5"/>
                                </button>
                            </div>

                            {selected.isBranch ? (
                                <p className="mt-2.5 text-xs leading-relaxed text-muted-foreground">
                                    Module group. Its children are the intents filed under this directory.
                                </p>
                            ) : (
                                <>
                                    <div className="mt-2.5 flex items-center gap-2 text-[11px] text-muted-foreground">
                                        <span className="flex items-center gap-1">
                                            {statusDot[selected.node.status]}
                                            {statusLabel[selected.node.status]}
                                        </span>
                                        <span>·</span>
                                        <span>{selected.node.kind}</span>
                                        {relativeDate(selected.node.updatedAt) !== null && (
                                            <>
                                                <span>·</span>
                                                <span>{relativeDate(selected.node.updatedAt)}</span>
                                            </>
                                        )}
                                    </div>
                                    <p className="mt-2 whitespace-pre-wrap break-words text-xs leading-relaxed">
                                        {selected.node.summary ?? 'No description in the body yet.'}
                                    </p>

                                    {selected.node.symbols !== undefined && selected.node.symbols.length > 0 && (
                                        <div className="mt-2.5">
                                            <div className="text-[10px] uppercase tracking-wide text-muted-foreground/70">
                                                Symbols
                                            </div>
                                            <div className="mt-1 flex flex-wrap gap-1">
                                                {selected.node.symbols.map((symbol) => (
                                                    <code
                                                        key={symbol}
                                                        className="rounded bg-muted px-1.5 py-0.5 font-mono text-[10px]"
                                                    >
                                                        {symbol}
                                                    </code>
                                                ))}
                                            </div>
                                        </div>
                                    )}

                                    {selected.node.reviewers !== undefined && selected.node.reviewers.length > 0 && (
                                        <div className="mt-2 text-[11px]">
                                            <span className="text-muted-foreground/70">Reviewed by </span>
                                            {selected.node.reviewers.join(', ')}
                                        </div>
                                    )}

                                    {selected.node.assumptions !== undefined && selected.node.assumptions.length > 0 && (
                                        <div className="mt-2.5">
                                            <div className="text-[10px] uppercase tracking-wide text-muted-foreground/70">
                                                Assumptions
                                            </div>
                                            <ul className="mt-1 space-y-1">
                                                {selected.node.assumptions.map((assumption) => (
                                                    <li
                                                        key={assumption.id}
                                                        className="rounded-md border border-border/60 px-2 py-1"
                                                    >
                                                        <div className="flex items-center gap-1.5 text-[11px]">
                                                            <code className="font-mono">{assumption.id}</code>
                                                            <span
                                                                className={cn(
                                                                    'rounded px-1 text-[9px]',
                                                                    assumption.severity === 'hard'
                                                                        ? 'bg-destructive/15 text-destructive'
                                                                        : 'bg-amber-400/15 text-amber-300',
                                                                )}
                                                            >
                                                                {assumption.severity}
                                                            </span>
                                                            {assumption.expiresAt !== undefined && (
                                                                <span className="text-muted-foreground/70">
                                                                    expires {assumption.expiresAt}
                                                                </span>
                                                            )}
                                                        </div>
                                                        {assumption.rationale !== undefined && (
                                                            <p className="mt-0.5 text-[10px] leading-snug text-muted-foreground">
                                                                {assumption.rationale}
                                                            </p>
                                                        )}
                                                    </li>
                                                ))}
                                            </ul>
                                        </div>
                                    )}

                                    {(selected.node.references !== undefined || selected.node.dependsOn !== undefined) && (
                                        <div className="mt-2.5 space-y-1 text-[10px]">
                                            {selected.node.references !== undefined && selected.node.references.length > 0 && (
                                                <RelationRow
                                                    label="references"
                                                    slugs={selected.node.references}
                                                    onNavigate={onSelectedIdChange}
                                                />
                                            )}
                                            {selected.node.dependsOn !== undefined && selected.node.dependsOn.length > 0 && (
                                                <RelationRow
                                                    label="depends_on"
                                                    slugs={selected.node.dependsOn}
                                                    onNavigate={onSelectedIdChange}
                                                />
                                            )}
                                        </div>
                                    )}
                                </>
                            )}

                            <p className="mt-2.5 text-[10px] text-muted-foreground/70">
                                Read-only preview. Edit the Markdown file in your IDE.
                            </p>
                        </div>
                    )}
                </div>
            </div>

            {/* Canvas controls */}
            <div className="pointer-events-none absolute inset-x-0 bottom-0 flex items-end justify-between p-3">
                <div className="pointer-events-auto flex flex-col gap-1 rounded-md bg-background/80 px-2 py-1 text-[10px] text-muted-foreground">
                    <span>Drag empty space to pan · click a node for details</span>
                    <span className="flex flex-wrap items-center gap-3">
                        <span className="flex items-center gap-1">
                            <svg width="26" height="8" aria-hidden="true">
                                <line
                                    x1="1" y1="4" x2="22" y2="4"
                                    stroke="hsl(var(--primary))"
                                    strokeWidth={2}
                                    markerEnd="url(#kron-arrow-depends)"
                                />
                            </svg>
                            depends_on (hard)
                        </span>
                        <span className="flex items-center gap-1">
                            <svg width="26" height="8" aria-hidden="true">
                                <line
                                    x1="1" y1="4" x2="22" y2="4"
                                    stroke="hsl(var(--muted-foreground))"
                                    strokeWidth={1.5}
                                    strokeDasharray="6 4"
                                    markerEnd="url(#kron-arrow-references)"
                                />
                            </svg>
                            references (soft)
                        </span>
                    </span>
                </div>
                <button
                    type="button"
                    onClick={resetView}
                    className="pointer-events-auto flex items-center gap-1 rounded-md border border-border bg-background/80 px-2 py-1 text-[10px] text-muted-foreground hover:text-foreground"
                    aria-label="Reset view"
                >
                    <LocateFixed className="h-3 w-3"/>
                    Reset view
                </button>
            </div>
        </div>
    );
}
