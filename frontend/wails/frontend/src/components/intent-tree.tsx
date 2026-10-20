// Kron GUI — intent tree (right pane).
// Recursive group/intent list with collapsible rows, kind/status badges,
// and inline filter highlighting.

import {useMemo, useState} from 'react';
import {
    ChevronDown,
    ChevronRight,
    CircleAlert,
    CircleCheck,
    CircleDashed,
    ExternalLink,
    FileText,
    Folder,
    GitPullRequestArrow,
    PencilLine,
    StickyNote,
} from 'lucide-react';
import {cn} from '@/lib/utils';
import {Badge} from '@/components/ui/badge';
import {Button} from '@/components/ui/button';
import {ScrollArea} from '@/components/ui/scroll-area';
import {Card} from '@/components/ui/card';
import {Separator} from '@/components/ui/separator';
import {OpenInIDE} from '../../wailsjs/go/main/App';
import type {IntentKind, IntentNode, IntentStatus} from '@/lib/mock';

interface IntentTreeProps {
    data: IntentNode[];
    filter: string;
    projectName: string;
    demoMode?: boolean;
}

const kindIcon: Record<IntentKind, React.ReactNode> = {
    adr: <FileText className="h-3.5 w-3.5"/>,
    rfc: <GitPullRequestArrow className="h-3.5 w-3.5"/>,
    spec: <Folder className="h-3.5 w-3.5"/>,
    note: <StickyNote className="h-3.5 w-3.5"/>,
};

const statusBadge: Record<
    IntentStatus,
    {label: string; variant: 'default' | 'secondary' | 'destructive' | 'outline'}
> = {
    draft: {label: 'Draft', variant: 'outline'},
    review: {label: 'Review', variant: 'secondary'},
    accepted: {label: 'Accepted', variant: 'default'},
    stale: {label: 'Stale', variant: 'destructive'},
};

const statusDot: Record<IntentStatus, React.ReactNode> = {
    draft: <CircleDashed className="h-3 w-3 text-muted-foreground"/>,
    review: <PencilLine className="h-3 w-3 text-amber-400"/>,
    accepted: <CircleCheck className="h-3 w-3 text-emerald-400"/>,
    stale: <CircleAlert className="h-3 w-3 text-destructive"/>,
};

export function IntentTree(props: IntentTreeProps): JSX.Element {
    const {data, filter, projectName, demoMode = false} = props;
    const filtered = useMemo(
        () => filterTree(data, filter.trim().toLowerCase()),
        [data, filter]
    );
    const forceOpen = filter.trim() !== '';

    return (
        <ScrollArea className="h-full">
            <div className="p-5 max-w-5xl">
                <Card className="overflow-hidden">
                    <div className="px-4 py-3 flex items-center justify-between">
                        <div className="flex items-center gap-2">
                            <span className="text-sm font-medium">
                                {projectName} intents
                            </span>
                            <Badge variant="secondary" className="text-[10px]">
                                {countNodes(filtered)} items
                            </Badge>
                        </div>
                        <span className="text-[11px] text-muted-foreground">
                            {demoMode ? 'Click folders to expand; click an intent to preview the IDE action' : 'Click a row to expand or open in IDE'}
                        </span>
                    </div>
                    <Separator/>
                    <div className="py-1">
                        {filtered.length === 0 ? (
                            <div className="px-4 py-10 text-center text-sm text-muted-foreground">
                                No intents match “{filter}”.
                            </div>
                        ) : (
                            filtered.map((node) => (
                                <TreeRow key={node.id} node={node} depth={0} demoMode={demoMode} forceOpen={forceOpen}/>
                            ))
                        )}
                    </div>
                </Card>
            </div>
        </ScrollArea>
    );
}

interface TreeRowProps {
    node: IntentNode;
    depth: number;
    demoMode: boolean;
    /** When a filter is active every matching branch must be visible. */
    forceOpen: boolean;
}

function TreeRow({node, depth, demoMode, forceOpen}: TreeRowProps): JSX.Element {
    const [open, setOpen] = useState<boolean>(true);
    // Work 1.4.e: per-row IDE-open error state. Cleared on next
    // successful open. Rendered as a tiny destructive line under the
    // row so the failure is visible without a toast. Empty string
    // means "no error" (kept as a string rather than `string | null`
    // so useState's initialiser is symmetric and the conditional
    // render stays terse).
    const [openErr, setOpenErr] = useState<string>('');
    const [demoMessage, setDemoMessage] = useState<string>('');
    const hasChildren = node.children.length > 0;
    const isGroup = node.kind === 'spec' && hasChildren;
    // While filtering, always expand so a match nested under a group that
    // the user collapsed earlier is still visible (otherwise the filter
    // reports matches the user cannot see).
    const expanded = open || forceOpen;
    // Directories have no updated_at on the wire; real-tree.ts uses the
    // epoch as a sentinel, which would render as "57y ago". Treat it as
    // "no date" instead of showing a bogus one.
    const updatedMs = new Date(node.updatedAt).getTime();
    const hasUpdatedAt = Number.isFinite(updatedMs) && updatedMs > 86_400_000;
    const status = statusBadge[node.status];
    const indent = 12 + depth * 18;

    // Work 1.4.e: leaves that carry an absolute .md path are
    // clickable targets for the "open in IDE" bridge. Directories
    // (isGroup) keep their expand/collapse behaviour. Leaves WITHOUT
    // a path (mock-mode demo data, or a kron_get that did not
    // surface source_path) stay non-clickable for IDE opens but
    // still get a subtle "live only" hint in the title attribute.
    const nodePath: string | undefined = node.path;
    const canOpenInIDE: boolean = !isGroup && nodePath !== undefined && nodePath !== '';

    // Work 1.4.e: single click handler that branches on the row
    // shape. We keep the row itself as the click target (no
    // separate button) because RFC §3.1 specifies "调 code <file>
    // 跳到 IDE 编辑" as the primary affordance — adding a separate
    // icon button would dilute that. The chevron + ExternalLink
    // indicators hint at the click outcome.
    const handleClick = (): void => {
        if (demoMode && !isGroup) {
            setDemoMessage('Demo: Live mode would open this intent in your IDE.');
            return;
        }
        if (canOpenInIDE) {
            // OpenInIDE is fire-and-forget: we optimistically clear
            // the previous error and surface a new one if the bridge
            // returns ok=false. We deliberately do NOT preventDefault
            // / stopPropagation because the row is a plain div.
            setOpenErr('');
            void (async (): Promise<void> => {
                try {
                    const result = await OpenInIDE(nodePath!);
                    if (!result.ok) {
                        setOpenErr(result.err || 'failed to open in IDE');
                    }
                } catch (err: unknown) {
                    const msg = err instanceof Error ? err.message : String(err);
                    setOpenErr(msg);
                }
            })();
            return;
        }
        if (hasChildren) {
            setOpen(!open);
        }
    };

    return (
        <div>
            <div
                className={cn(
                    'flex items-center gap-2 px-3 py-2',
                    'hover:bg-accent/50 transition-colors',
                    'cursor-pointer select-none',
                )}
                style={{paddingLeft: indent}}
                onClick={handleClick}
                data-testid={`intent-row-${node.id}`}
            >
                <span className="w-4 h-4 flex items-center justify-center text-muted-foreground shrink-0">
                    {hasChildren ? (
                        expanded ? (
                            <ChevronDown className="h-3.5 w-3.5"/>
                        ) : (
                            <ChevronRight className="h-3.5 w-3.5"/>
                        )
                    ) : null}
                </span>

                <span className="text-muted-foreground shrink-0">
                    {kindIcon[node.kind]}
                </span>

                <span
                    className={cn(
                        'truncate',
                        isGroup ? 'text-sm font-medium' : 'text-sm',
                    )}
                    title={
                        canOpenInIDE
                            ? `Open ${nodePath} in your IDE`
                            : demoMode && !isGroup
                                ? 'Demo: Live mode would open this intent in your IDE'
                            : !isGroup && !canOpenInIDE
                                ? 'Open in IDE available in live mode (switch in Settings)'
                                : node.title
                    }
                >
                    {node.title}
                </span>

                {node.summary !== undefined && !isGroup && (
                    <span className="hidden lg:inline text-xs text-muted-foreground truncate max-w-[40ch]">
                        {node.summary}
                    </span>
                )}

                <span className="ml-auto flex items-center gap-2 shrink-0">
                    {/* Work 1.4.e: ExternalLink affordance for leaves
                        that have a path. Rendered after the status
                        badge so it does not steal the click target
                        on the row itself. The icon is purely
                        decorative — clicking it does nothing extra;
                        the whole row is the click target. */}
                    {canOpenInIDE && (
                        <ExternalLink
                            className="h-3 w-3 text-muted-foreground/70"
                            aria-label="Open in IDE"
                        />
                    )}
                    <span className="hidden md:flex items-center gap-1 text-[11px] text-muted-foreground">
                        {statusDot[node.status]}
                        {hasUpdatedAt ? relativeDate(node.updatedAt) : null}
                    </span>
                    <Badge variant={status.variant} className="text-[10px]">
                        {status.label}
                    </Badge>
                </span>
            </div>

            {/* Work 1.4.e: surface IDE-open failures as a tiny
                destructive line under the row. Hidden by default
                (openErr === ''). Same indent as the row body so it
                visually belongs to the same node. Cleared by the
                next successful click. */}
            {openErr !== '' && (
                <div
                    className="pb-1.5 text-[10px] text-destructive"
                    style={{paddingLeft: indent + 24}}
                    role="alert"
                >
                    {openErr}
                </div>
            )}

            {demoMessage !== '' && (
                <div className="pb-1.5 text-[10px] text-muted-foreground" style={{paddingLeft: indent + 24}}>
                    {demoMessage}
                </div>
            )}

            {hasChildren && expanded && (
                <div>
                    {node.children.map((child) => (
                        <TreeRow key={child.id} node={child} depth={depth + 1} demoMode={demoMode} forceOpen={forceOpen}/>
                    ))}
                </div>
            )}
        </div>
    );
}

function filterTree(nodes: IntentNode[], q: string): IntentNode[] {
    if (q === '') return nodes;
    const out: IntentNode[] = [];
    for (const n of nodes) {
        const kids = filterTree(n.children, q);
        const hit =
            n.title.toLowerCase().includes(q) ||
            (n.summary?.toLowerCase().includes(q) ?? false);
        if (hit || kids.length > 0) {
            out.push({...n, children: kids});
        }
    }
    return out;
}

function countNodes(nodes: IntentNode[]): number {
    let n = 0;
    for (const node of nodes) {
        n += 1 + countNodes(node.children);
    }
    return n;
}

function relativeDate(iso: string): string {
    const then = new Date(iso).getTime();
    const now = Date.now();
    const days = Math.round((now - then) / 86_400_000);
    if (days <= 0) return 'today';
    if (days === 1) return '1d ago';
    if (days < 30) return `${days}d ago`;
    if (days < 365) return `${Math.round(days / 30)}mo ago`;
    return `${Math.round(days / 365)}y ago`;
}
