// Kron GUI — main page (work 2.x shadcn version).
// Layout per sponsor spec:
//   - Left  ~20% rail  → brand panel with intent statement
//   - Right ~80% panel → intent tree
// Top-of-page intro: brief boot animation, then the real shell.

import {useCallback, useEffect, useState} from 'react';
import {
    ListTree,
    FileCode2,
    GitBranch,
    ShieldCheck,
    CircleAlert,
    History,
    Sparkles,
    Search,
    ChevronRight,
    Folder,
} from 'lucide-react';
import {Button} from '@/components/ui/button';
import {Badge} from '@/components/ui/badge';
import {Input} from '@/components/ui/input';
import {
    Card,
    CardContent,
    CardHeader,
    CardTitle,
    CardDescription,
} from '@/components/ui/card';
import {ScrollArea} from '@/components/ui/scroll-area';
import {Separator} from '@/components/ui/separator';
import {Progress} from '@/components/ui/progress';
import {GetKronInfo, ListMCPTools, ListMCPToolsLive, CallMCPTool, ListProjectsLive, SwitchProject} from '../wailsjs/go/main/App';
import {
    type KronInfo,
    type IntentNode,
    type TreeStats,
    mockStats,
    mockProjects,
} from '@/lib/mock';
import {BootAnimation} from '@/components/boot-animation';
import {IntentCanvas} from '@/components/intent-canvas';
import {kronTreeToIntentNodes} from '@/lib/real-tree';
import {type ListProjectsLiveResult} from '@/lib/mcp-types';

// DataSource: how the MCP tools panel is populated. 'mock' is the
// 1.3-era hard-coded list returned by ListMCPTools(); 'live' asks the
// Go bridge to spawn a real `kron serve-mcp` subprocess and return the
// list of tools it actually registers via the MCP initialize +
// tools/list handshake (work 1.4.b).
type DataSource = 'mock' | 'live';

function App() {
    const [booted, setBooted] = useState<boolean>(false);
    const [info, setInfo] = useState<KronInfo | null>(null);
    const [tools, setTools] = useState<string[]>([]);
    const [loadError, setLoadError] = useState<string | null>(null);
    const [filter, setFilter] = useState<string>('');
    const [activeMockProjectId, setActiveMockProjectId] = useState<string>(mockProjects[0]!.id);
    const [tree, setTree] = useState<IntentNode[]>(mockProjects[0]!.tree);
    const [stats, setStats] = useState<TreeStats>(mockStats);

    // 1.4.b: data-source toggle. Default 'mock' preserves the
    // 1.3-era behaviour so the smoke test still works even if no
    // `kron serve-mcp` binary is on disk. Switching to 'live' triggers
    // a one-shot spawn-and-handshake via the Go bridge.
    const [dataSource, setDataSource] = useState<DataSource>('mock');
    const [liveTools, setLiveTools] = useState<string[]>([]);
    const [liveServerName, setLiveServerName] = useState<string>('');
    const [liveServerVersion, setLiveServerVersion] = useState<string>('');
    const [liveSource, setLiveSource] = useState<string>('');
    const [liveError, setLiveError] = useState<string | null>(null);
    const [liveLoading, setLiveLoading] = useState<boolean>(false);
    // 1.4.c: live intent tree state. liveTree holds the
    // kron_tree-derived IntentNode[]; it replaces the mock
    // whenever the user is in 'live' mode AND the fetch has
    // succeeded. liveTreeError surfaces a friendly message in
    // place of the tree on failure. liveTreeLoading controls a
    // subtle skeleton in the main pane (re-using the existing
    // "Connecting…" / "Refreshing…" pattern from liveTools).
    const [liveTree, setLiveTree] = useState<IntentNode[] | null>(null);
    const [liveTreeError, setLiveTreeError] = useState<string | null>(null);
    const [liveTreeLoading, setLiveTreeLoading] = useState<boolean>(false);
    const [noActiveProject, setNoActiveProject] = useState<boolean>(false);
    const [lintStats, setLintStats] = useState<{errors: number; warnings: number} | null>(mockProjects[0]!.lint);
    const [lintError, setLintError] = useState<string | null>(null);
    const [lintLoading, setLintLoading] = useState<boolean>(false);
    const [switchingProject, setSwitchingProject] = useState<string | null>(null);
    const [projectSwitchError, setProjectSwitchError] = useState<string | null>(null);
    const [settingsOpen, setSettingsOpen] = useState<boolean>(false);
    // Selection is owned here so the left-rail rows can drive the canvas.
    const [selectedIntentId, setSelectedIntentId] = useState<string | null>(null);
    const [focusRequest, setFocusRequest] = useState<number>(0);

    // The projects view reads the portable registry through the Go
    // bridge and remains available in both mock and live display modes.
    const [liveProjects, setLiveProjects] = useState<ListProjectsLiveResult | null>(
        null,
    );
    const [liveProjectsError, setLiveProjectsError] = useState<string | null>(null);
    const [liveProjectsLoading, setLiveProjectsLoading] = useState<boolean>(false);
    const activeMockProject = mockProjects.find((project) => project.id === activeMockProjectId) ?? mockProjects[0]!;

    // Live mode must never silently render demo data as if it were real.
    const displayTree: IntentNode[] =
        dataSource === 'live' ? (liveTree ?? []) : tree;
    const showLiveTreeSkeleton: boolean =
        dataSource === 'live' && liveTree === null && liveTreeError === null;
    // Show the real active project's folder name instead of the literal
    // "Active project" placeholder. Falls back to the placeholder when the
    // registry has no current project or holds a bare drive root.
    const liveProjectName: string = (() => {
        const parts = (liveProjects?.current ?? '').split(/[\\/]/).filter(Boolean);
        return parts.length > 0 ? parts[parts.length - 1]! : 'Active project';
    })();

    const refreshLive = useCallback(async (): Promise<void> => {
        setLiveLoading(true);
        setLiveError(null);
        try {
            const projects = await ListProjectsLive();
            setLiveProjects(projects);
            if (projects.current === '') {
                setNoActiveProject(true);
                setLiveTools([]);
                setLiveServerName('');
                setLiveServerVersion('');
                setLiveSource('');
                return;
            }
            const result = await ListMCPToolsLive();
            setLiveTools(result.tools.map((t) => t.name));
            setLiveServerName(result.serverInfo.name);
            setLiveServerVersion(result.serverInfo.version);
            setLiveSource(result.source);
        } catch (err: unknown) {
            const msg = err instanceof Error ? err.message : String(err);
            setLiveError(msg);
            // Keep the previous liveTools on a failed refresh so the
            // UI does not flicker empty. The next successful refresh
            // overwrites them.
        } finally {
            setLiveLoading(false);
        }
    }, []);

    // 1.4.c: refresh the live intent tree by calling kron_tree
    // (one JSON-RPC round-trip) and then kron_get once per leaf
    // (N round-trips, parallelised via Promise.all in
    // kronTreeToIntentNodes). Failures are surfaced via
    // liveTreeError; the main pane falls back to the mock tree
    // when liveTree === null so the page stays usable.
    const refreshLiveTree = useCallback(async (): Promise<void> => {
        setLiveTreeLoading(true);
        setLiveTreeError(null);
        setLintLoading(true);
        setLintError(null);
        try {
            const projects = await ListProjectsLive();
            setLiveProjects(projects);
            if (projects.current === '') {
                setNoActiveProject(true);
                setLiveTree([]);
                setStats(emptyStats());
                setLintStats(null);
                return;
            }
            setNoActiveProject(false);
            const treeText = await CallMCPTool('kron_tree', '{}');
            const treeJson = JSON.parse(treeText) as {
                root: import('@/lib/mcp-types').TreeNode;
            };
            const converted = await kronTreeToIntentNodes(treeJson);
            setLiveTree(converted);
            setStats(statsFromTree(converted));
            try {
                const lintText = await CallMCPTool('kron_lint', '{}');
                const lint = JSON.parse(lintText) as {summary?: {errors?: number; warnings?: number}};
                if (typeof lint.summary?.errors !== 'number' || typeof lint.summary?.warnings !== 'number') {
                    throw new Error('kron_lint returned no numeric summary counters');
                }
                setLintStats({errors: lint.summary.errors, warnings: lint.summary.warnings});
            } catch (err: unknown) {
                setLintStats(null);
                setLintError(err instanceof Error ? err.message : String(err));
            } finally {
                setLintLoading(false);
            }
        } catch (err: unknown) {
            const msg = err instanceof Error ? err.message : String(err);
            setLiveTreeError(msg);
            // Keep the previous liveTree on a failed refresh so the
            // UI does not flicker empty — but never leave the mock
            // counts (11 files / 2 archived) in the left rail while
            // live data is unavailable: that would present demo data
            // as if it described the real project.
            setStats(emptyStats());
            setLintStats(null);
        } finally {
            setLiveTreeLoading(false);
            setLintLoading(false);
        }
    }, []);

    // Refresh the projects.json registry. On failure, keep the previous
    // snapshot visible and surface the read error beside it.
    const refreshLiveProjects = useCallback(async (): Promise<void> => {
        setLiveProjectsLoading(true);
        setLiveProjectsError(null);
        try {
            const result = await ListProjectsLive();
            setLiveProjects(result);
        } catch (err: unknown) {
            const msg = err instanceof Error ? err.message : String(err);
            setLiveProjectsError(msg);
        } finally {
            setLiveProjectsLoading(false);
        }
    }, []);

    const switchLiveProject = useCallback(async (path: string): Promise<void> => {
        setSwitchingProject(path);
        setProjectSwitchError(null);
        setLiveTree(null);
        setLiveTreeError(null);
        try {
            const result = await SwitchProject(path);
            setLiveProjects(result);
            setNoActiveProject(false);
            if (dataSource !== 'live') setDataSource('live');
            else {
                await refreshLive();
                await refreshLiveTree();
            }
        } catch (err: unknown) {
            const message = err instanceof Error ? err.message : String(err);
            setProjectSwitchError(message);
            if (dataSource === 'live') setLiveTreeError(message);
        } finally {
            setSwitchingProject(null);
        }
    }, [dataSource, refreshLive, refreshLiveTree]);

    useEffect(() => {
        let cancelled = false;
        (async (): Promise<void> => {
            try {
                const [kronInfo, kronTools] = await Promise.all([
                    GetKronInfo(),
                    ListMCPTools(),
                ]);
                if (cancelled) return;
                setInfo(kronInfo);
                setTools(kronTools);
            } catch (err: unknown) {
                if (cancelled) return;
                const msg = err instanceof Error ? err.message : String(err);
                setLoadError(msg);
            }
        })();

        return (): void => {
            cancelled = true;
        };
    }, []);

    // 1.4.b: kick off the live handshake whenever the user flips
    // the toggle to 'live'. Switching back to 'mock' does not call
    // the bridge — the subprocess stays alive in the background
    // and is reaped by the Wails OnShutdown hook (Go side).
    //
    // 1.4.c: when dataSource is 'live' we ALSO refresh the intent
    // tree. Both refreshes are independent; a slow kron_tree
    // fetch does not block the tools panel (and vice versa). We
    // do not await them sequentially because the Go bridge
    // serialises tool calls under mcpMu anyway, and parallel
    // React state updates compose into a single render.
    useEffect(() => {
        if (dataSource === 'live') {
            void (async (): Promise<void> => {
                await refreshLiveProjects();
                await refreshLive();
                await refreshLiveTree();
            })();
        } else {
            setTree(activeMockProject.tree);
            setStats(activeMockProject.stats);
            setLintStats(activeMockProject.lint);
            setLintError(null);
        }
    }, [dataSource, activeMockProject, refreshLive, refreshLiveTree, refreshLiveProjects]);

    return (
        <>
            <BootAnimation onDone={(): void => setBooted(true)}/>
            <div
                className={[
                    'kron-shell flex h-screen w-screen overflow-hidden',
                    'bg-background text-foreground',
                    'transition-opacity duration-500',
                    booted ? 'opacity-100' : 'opacity-0 pointer-events-none',
                ].join(' ')}
            >
                <LeftRail
                    info={info}
                    tools={tools}
                    error={loadError}
                    stats={stats}
                    tree={displayTree}
                    dataSource={dataSource}
                    onDataSourceChange={setDataSource}
                    liveTools={liveTools}
                    liveServerName={liveServerName}
                    liveServerVersion={liveServerVersion}
                    liveSource={liveSource}
                    liveError={liveError}
                    liveLoading={liveLoading}
                    onRefreshLive={(): void => {
                        void refreshLive();
                    }}
                    liveProjects={liveProjects}
                    mockProjects={mockProjects}
                    activeMockProjectId={activeMockProjectId}
                    onSelectMockProject={setActiveMockProjectId}
                    liveProjectsError={liveProjectsError}
                    liveProjectsLoading={liveProjectsLoading}
                    onRefreshLiveProjects={(): void => {
                        void refreshLiveProjects();
                    }}
                    onSwitchProject={(path): void => { void switchLiveProject(path); }}
                    switchingProject={switchingProject}
                    projectSwitchError={projectSwitchError}
                    lintStats={lintStats}
                    lintError={lintError}
                    lintLoading={lintLoading}
                    settingsOpen={settingsOpen}
                    onSettingsOpenChange={setSettingsOpen}
                    onSelectIntent={(id): void => {
                        setSelectedIntentId(id);
                        setFocusRequest((n) => n + 1);
                    }}
                />
                <main className="flex-1 flex flex-col min-w-0">
                    <TopBar
                        filter={filter}
                        onFilterChange={setFilter}
                        rootLabel={activeMockProject.name}
                        dataSource={dataSource}
                        liveTreeLoading={liveTreeLoading}
                        liveTreeError={liveTreeError}
                        onRefreshLiveTree={(): void => {
                            void refreshLiveTree();
                        }}
                    />
                    <Separator/>
                    <div className="flex-1 overflow-hidden">
                        {showLiveTreeSkeleton ? (
                            <div className="p-6 text-sm text-muted-foreground">Loading the selected project's intents…</div>
                        ) : noActiveProject && dataSource === 'live' ? (
                            <div className="p-6 text-sm text-muted-foreground">No active Kron project. Run kron init or kron register, refresh Projects, then select a project.</div>
                        ) : liveTreeError !== null && dataSource === 'live' ? (
                            <div className="p-6 text-sm text-destructive">
                                {liveTreeError.includes('cannot find the Kron CLI binary')
                                    ? 'Live mode needs the Kron CLI binary. Build it with `go build -o frontend/wails/build/bin/kron-cli.exe ./cmd/kron` (or put a kron CLI next to the GUI executable), then Retry.'
                                    : 'Could not load the live project tree. Use Retry below after checking the MCP connection.'}
                            </div>
                        ) : dataSource === 'mock' && displayTree.length === 0 ? (
                            <div className="p-6 text-sm text-muted-foreground">This demo project has no intents yet. Switch to another demo project.</div>
                        ) : (
                            <IntentCanvas
                                data={displayTree}
                                projectName={dataSource === 'mock' ? activeMockProject.name : liveProjectName}
                                filter={filter}
                                selectedId={selectedIntentId}
                                onSelectedIdChange={setSelectedIntentId}
                                focusRequest={focusRequest}
                            />
                        )}
                    </div>
                    {/* 1.4.c: live-tree status footer. Shows a
                        subtle hint under the tree when we are
                        loading the live tree, or a destructive
                        error card when the fetch failed. Hidden
                        in mock mode. */}
                    {dataSource === 'live' && liveTreeError !== null && (
                        <div className="px-5 pb-3">
                            <Card className="border-destructive/50 bg-destructive/10">
                                <CardContent className="p-2.5 flex items-start gap-2">
                                    <pre className="text-[10px] text-destructive/90 whitespace-pre-wrap break-all flex-1">
                                        {liveTreeError}
                                    </pre>
                                    <Button
                                        variant="outline"
                                        size="sm"
                                        className="h-6 text-[10px] shrink-0"
                                        onClick={(): void => {
                                            void refreshLiveTree();
                                        }}
                                    >
                                        Retry
                                    </Button>
                                </CardContent>
                            </Card>
                        </div>
                    )}
                    {dataSource === 'live' &&
                        liveTreeError === null &&
                        showLiveTreeSkeleton && (
                            <div className="px-5 pb-3 text-[10px] text-muted-foreground">
                                Loading live intent tree from
                                <code className="font-mono ml-1">
                                    kron serve-mcp
                                </code>
                                …
                            </div>
                        )}
                </main>
            </div>
        </>
    );
}

function emptyStats(): TreeStats {
    return {intents: 0, groups: 0, stale: 0, verified: 0, draft: 0, review: 0, archived: 0};
}

function statsFromTree(nodes: IntentNode[]): TreeStats {
    const stats = emptyStats();
    const visit = (items: IntentNode[]): void => {
        for (const node of items) {
            if (node.kind === 'spec') stats.groups += 1;
            else {
                stats.intents += 1;
                if (node.status === 'draft') stats.draft += 1;
                if (node.status === 'review') stats.review += 1;
                if (node.status === 'accepted') stats.verified += 1;
                if (node.status === 'stale') { stats.stale += 1; stats.archived += 1; }
            }
            visit(node.children);
        }
    };
    visit(nodes);
    return stats;
}

function MockProjectsPanel(props: {
    projects: typeof mockProjects;
    activeProjectId: string;
    onSelectProject: (id: string) => void;
}): JSX.Element {
    return (
        <>
            <div className="flex items-center justify-between">
                <span className="text-muted-foreground">Demo projects</span>
                <Badge variant="secondary" className="text-[10px] h-4 px-1.5">{props.projects.length} samples</Badge>
            </div>
            <p className="text-[10px] leading-snug text-muted-foreground">
                Switching here only changes demo data. It does not edit projects.json or connect to MCP.
            </p>
            <ul className="space-y-1">
                {props.projects.map((project) => {
                    const active = project.id === props.activeProjectId;
                    return (
                        <li key={project.id} className={`rounded border p-2 ${active ? 'border-primary/50 bg-primary/10' : 'border-border'}`}>
                            <div className="flex items-center gap-2">
                                <Folder className="h-3.5 w-3.5 shrink-0 text-muted-foreground"/>
                                <span className="min-w-0 flex-1 truncate font-medium">{project.name}</span>
                                <Button size="sm" variant={active ? 'secondary' : 'outline'} className="h-6 px-2 text-[10px]"
                                    disabled={active} onClick={(): void => props.onSelectProject(project.id)}>
                                    {active ? 'Current' : 'Switch'}
                                </Button>
                            </div>
                            <div className="mt-1 pl-5 truncate font-mono text-[9px] text-muted-foreground" title={project.path}>
                                {project.path}
                            </div>
                            <div className="mt-1 pl-5 text-[9px] text-muted-foreground">
                                {project.stats.intents} intents · {project.lint.errors} lint errors · {project.lint.warnings} warnings
                            </div>
                        </li>
                    );
                })}
            </ul>
        </>
    );
}

function LeftRail(props: {
    info: KronInfo | null;
    tools: string[];
    error: string | null;
    stats: TreeStats;
    tree: IntentNode[];
    dataSource: DataSource;
    onDataSourceChange: (s: DataSource) => void;
    liveTools: string[];
    liveServerName: string;
    liveServerVersion: string;
    liveSource: string;
    liveError: string | null;
    liveLoading: boolean;
    onRefreshLive: () => void;
    liveProjects: ListProjectsLiveResult | null;
    mockProjects: typeof mockProjects;
    activeMockProjectId: string;
    onSelectMockProject: (id: string) => void;
    liveProjectsError: string | null;
    liveProjectsLoading: boolean;
    onRefreshLiveProjects: () => void;
    onSwitchProject: (path: string) => void;
    switchingProject: string | null;
    projectSwitchError: string | null;
    lintStats: {errors: number; warnings: number} | null;
    lintError: string | null;
    lintLoading: boolean;
    settingsOpen: boolean;
    onSettingsOpenChange: (open: boolean) => void;
    /** Select an intent in the canvas (left-rail row click). */
    onSelectIntent: (id: string) => void;
}): JSX.Element {
    const {
        info,
        tools,
        error,
        stats,
        tree,
        dataSource,
        onDataSourceChange,
        liveTools,
        liveServerName,
        liveServerVersion,
        liveSource,
        liveError,
        liveLoading,
        onRefreshLive,
        liveProjects,
        mockProjects: demoProjects,
        activeMockProjectId,
        onSelectMockProject,
        liveProjectsError,
        liveProjectsLoading,
        onRefreshLiveProjects,
        onSwitchProject,
        switchingProject,
        projectSwitchError,
        lintStats,
        lintError,
        lintLoading,
        settingsOpen,
        onSettingsOpenChange,
        onSelectIntent,
    } = props;
    const [activeView, setActiveView] = useState<
        'explorer' | 'projects' | 'git' | 'tools'
    >('explorer');
    const [openGroups, setOpenGroups] = useState<Record<string, boolean>>({
        Files: true,
        Archived: true,
        'MCP Tools': false,
    });

    const toggleGroup = (name: string): void => {
        setOpenGroups((prev) => ({...prev, [name]: !prev[name]}));
    };

    return (
        <aside
            className={[
                'flex h-full overflow-hidden',
                'border-r border-border',
                'bg-card',
            ].join(' ')}
            style={{width: '22%', minWidth: 280, maxWidth: 380}}
        >
            {/* Activity bar — 40px icon rail on the far left, like VS Code / Cursor */}
            <div
                className={[
                    'flex flex-col items-center gap-1 py-3',
                    'border-r border-border bg-background/40',
                ].join(' ')}
                style={{width: 40}}
            >
                <ActivityIcon
                    active={activeView === 'explorer'}
                    label="Explorer"
                    onClick={(): void => setActiveView('explorer')}
                >
                    <ListTree className="h-5 w-5"/>
                </ActivityIcon>
                <ActivityIcon
                    active={activeView === 'projects'}
                    label="Projects"
                    onClick={(): void => setActiveView('projects')}
                >
                    <Folder className="h-5 w-5"/>
                </ActivityIcon>
                <ActivityIcon
                    active={activeView === 'git'}
                    label="Source control"
                    onClick={(): void => setActiveView('git')}
                >
                    <GitBranch className="h-5 w-5"/>
                </ActivityIcon>
                <ActivityIcon
                    active={activeView === 'tools'}
                    label="MCP tools"
                    onClick={(): void => setActiveView('tools')}
                >
                    <Sparkles className="h-5 w-5"/>
                </ActivityIcon>
                <div className="flex-1"/>
                <ActivityIcon
                    active={settingsOpen}
                    label="Settings"
                    onClick={(): void => onSettingsOpenChange(!settingsOpen)}
                >
                    <ShieldCheck className="h-5 w-5"/>
                </ActivityIcon>
            </div>

            {/* Side panel — title bar + tree, VS Code style */}
            <div className="flex-1 flex flex-col min-w-0 overflow-hidden">
                <div
                    className={[
                        'flex items-center justify-between',
                        'h-9 px-4 shrink-0',
                        'text-[11px] uppercase tracking-wider text-muted-foreground',
                    ].join(' ')}
                >
                    <span className="font-semibold">
                        {activeView === 'explorer' && 'Explorer'}
                        {activeView === 'projects' && 'Projects'}
                        {activeView === 'git' && 'Source Control'}
                        {activeView === 'tools' && 'MCP Tools'}
                    </span>
                    <Button
                        variant="ghost"
                        size="sm"
                        className="h-5 w-5 p-0 text-muted-foreground"
                    >
                        <History className="h-3.5 w-3.5"/>
                    </Button>
                </div>
                <Separator/>

                <ScrollArea className="flex-1">
                    {/* 1.4.b: settings popover. Toggling the Settings
                        icon in the activity bar flips settingsOpen;
                        this card slides in at the top of the side
                        panel and exposes the data-source radio. */}
                    {settingsOpen && (
                        <div className="p-3">
                            <Card>
                                <CardHeader className="p-3 pb-2">
                                    <CardTitle className="text-xs">
                                        Settings
                                    </CardTitle>
                                    <CardDescription className="text-[10px]">
                                        Choose how the MCP tools panel is
                                        populated.
                                    </CardDescription>
                                </CardHeader>
                                <CardContent className="p-3 pt-0 space-y-2">
                                    <div
                                        role="radiogroup"
                                        aria-label="Data source"
                                        className="space-y-1.5"
                                    >
                                        <DataSourceRadio
                                            checked={dataSource === 'mock'}
                                            label="Mock data"
                                            description="Hard-coded 13 tool list (1.3-era)."
                                            onSelect={(): void =>
                                                onDataSourceChange('mock')}
                                        />
                                        <DataSourceRadio
                                            checked={dataSource === 'live'}
                                            label="Live (kron serve-mcp)"
                                            description="Spawn the real MCP subprocess and call tools/list."
                                            onSelect={(): void =>
                                                onDataSourceChange('live')}
                                        />
                                    </div>
                                    <Button
                                        variant="outline"
                                        size="sm"
                                        className="w-full h-7 text-[11px]"
                                        onClick={(): void =>
                                            onSettingsOpenChange(false)}
                                    >
                                        Close
                                    </Button>
                                </CardContent>
                            </Card>
                        </div>
                    )}
                    {error !== null && (
                        <div className="p-3">
                            <Card className="border-destructive/50 bg-destructive/10">
                                <CardHeader className="p-3">
                                    <CardTitle className="text-xs text-destructive">
                                        Bridge error
                                    </CardTitle>
                                </CardHeader>
                                <CardContent className="p-3 pt-0">
                                    <pre className="text-[10px] text-destructive/90 whitespace-pre-wrap break-all">
                                        {error}
                                    </pre>
                                </CardContent>
                            </Card>
                        </div>
                    )}

                    {activeView === 'explorer' && (
                        <div className="py-1">
                            <GroupHeader
                                name="Files"
                                count={stats.intents}
                                open={openGroups['Files'] ?? true}
                                onToggle={(): void => toggleGroup('Files')}
                            />
                            {openGroups['Files'] && (
                                <ul className="text-sm">
                                    {tree.map((node) => (
                                        <li
                                            key={node.id}
                                            role="button"
                                            tabIndex={0}
                                            title="Show this intent in the canvas"
                                            onClick={(): void => onSelectIntent(node.id)}
                                            onKeyDown={(event): void => {
                                                if (event.key === 'Enter' || event.key === ' ') {
                                                    event.preventDefault();
                                                    onSelectIntent(node.id);
                                                }
                                            }}
                                            className={[
                                                'group flex items-center gap-1.5',
                                                'pl-6 pr-2 py-1 cursor-pointer',
                                                'hover:bg-accent/60',
                                            ].join(' ')}
                                        >
                                            <ChevronRight className="h-3.5 w-3.5 text-muted-foreground/60"/>
                                            <FileCode2 className="h-3.5 w-3.5 text-muted-foreground"/>
                                            <span className="truncate flex-1">
                                                {node.title}
                                            </span>
                                            <StatusBadge status={node.status}/>
                                        </li>
                                    ))}
                                </ul>
                            )}

                            <GroupHeader
                                name="Archived"
                                count={stats.archived}
                                open={openGroups['Archived'] ?? false}
                                onToggle={(): void => toggleGroup('Archived')}
                            />
                            {openGroups['Archived'] && (
                                <ul className="text-sm">
                                    {tree
                                        .filter((n) => n.status === 'stale')
                                        .map((node) => (
                                            <li
                                                key={node.id}
                                                role="button"
                                                tabIndex={0}
                                                title="Show this intent in the canvas"
                                                onClick={(): void => onSelectIntent(node.id)}
                                                onKeyDown={(event): void => {
                                                    if (event.key === 'Enter' || event.key === ' ') {
                                                        event.preventDefault();
                                                        onSelectIntent(node.id);
                                                    }
                                                }}
                                                className={[
                                                    'group flex items-center gap-1.5',
                                                    'pl-6 pr-2 py-1 cursor-pointer',
                                                    'hover:bg-accent/60',
                                                    'opacity-70',
                                                ].join(' ')}
                                            >
                                                <ChevronRight className="h-3.5 w-3.5 text-muted-foreground/60"/>
                                                <History className="h-3.5 w-3.5 text-muted-foreground"/>
                                                <span className="truncate flex-1">
                                                    {node.title}
                                                </span>
                                                <Badge
                                                    variant="outline"
                                                    className="text-[9px] h-4 px-1"
                                                >
                                                    stale
                                                </Badge>
                                            </li>
                                        ))}
                                </ul>
                            )}
                        </div>
                    )}

                    {/* 1.4.d: Projects view. Renders the live
                        projects.json registry (kron_list_projects)
                        when the user is in 'live' mode, and a
                        short hint pointing to Settings otherwise
                        (mirrors the Tools view's live/mock split).
                        The view is intentionally read-only: there
                        is no "switch" affordance yet (work 1.4.d
                        scope) — the "active" project is just
                        highlighted via Badge. */}
                    {activeView === 'projects' && (
                        <div className="p-4 space-y-3 text-xs">
                            {dataSource === 'mock' ? (
                                <MockProjectsPanel
                                    projects={demoProjects}
                                    activeProjectId={activeMockProjectId}
                                    onSelectProject={onSelectMockProject}
                                />
                            ) : (
                                <>
                                    <div className="flex items-center justify-between">
                                        <span className="text-muted-foreground">
                                            Registry
                                        </span>
                                        <Badge
                                            variant={liveProjectsError !== null
                                                ? 'destructive'
                                                : 'secondary'}
                                            className="text-[10px] h-4 px-1.5"
                                        >
                                            {liveProjectsError !== null
                                                ? 'error'
                                                : liveProjectsLoading
                                                    ? 'loading…'
                                                    : `${liveProjects?.projects.length ?? 0} project${(liveProjects?.projects.length ?? 0) === 1 ? '' : 's'}`}
                                        </Badge>
                                    </div>
                                    {liveProjectsError !== null && (
                                        <Card className="border-destructive/50 bg-destructive/10">
                                            <CardContent className="p-2.5">
                                                <pre className="text-[10px] text-destructive/90 whitespace-pre-wrap break-all">
                                                    {liveProjectsError}
                                                </pre>
                                            </CardContent>
                                        </Card>
                                    )}
                                    {liveProjects !== null && liveProjects.projects.length === 0 && (
                                        <p className="text-muted-foreground leading-snug">
                                            No projects registered yet. Run{' '}
                                            <code className="font-mono">kron init</code>
                                            {' '}in a project directory to add one.
                                        </p>
                                    )}
                                    {liveProjects !== null && liveProjects.projects.length > 0 && (
                                        <ul className="space-y-0.5 font-mono text-[11px]">
                                            {liveProjects.projects.map((p) => {
                                                const isCurrent = liveProjects.current !== ''
                                                    && p.path === liveProjects.current;
                                                return (
                                                    <li
                                                        key={p.path}
                                                        className={[
                                                            'flex items-center gap-1.5 px-1.5 py-1 rounded',
                                                            'hover:bg-accent/60',
                                                            isCurrent
                                                                ? 'bg-primary/10 text-foreground'
                                                                : 'text-muted-foreground',
                                                        ].join(' ')}
                                                        title={p.path}
                                                    >
                                                        <Folder className="h-3.5 w-3.5 shrink-0"/>
                                                        <span className="truncate flex-1">
                                                            {p.path}
                                                        </span>
                                                        <Button size="sm" variant={isCurrent ? 'secondary' : 'outline'}
                                                            className="h-6 px-2 text-[10px] shrink-0"
                                                            disabled={isCurrent || switchingProject !== null}
                                                            onClick={(): void => onSwitchProject(p.path)}>
                                                            {isCurrent ? 'Active' : switchingProject === p.path ? 'Switching…' : 'Switch'}
                                                        </Button>
                                                    </li>
                                                );
                                            })}
                                        </ul>
                                    )}
                                    {liveProjects !== null && liveProjects.projectsPath !== '' && (
                                        <div className="pt-1 text-[10px] text-muted-foreground/70 font-mono break-all">
                                            {liveProjects.projectsPath}
                                        </div>
                                    )}
                                    {projectSwitchError !== null && <p role="alert" className="text-[10px] text-destructive break-words">{projectSwitchError}</p>}
                                    <Button
                                        variant="outline"
                                        size="sm"
                                        disabled={liveProjectsLoading}
                                        onClick={onRefreshLiveProjects}
                                        className="w-full h-7 text-[11px]"
                                    >
                                        {liveProjectsLoading ? 'Refreshing…' : 'Refresh'}
                                    </Button>
                                </>
                            )}
                        </div>
                    )}

                    {activeView === 'git' && (
                        <div className="p-4 space-y-3 text-xs">
                            <div className="flex items-center justify-between">
                                <span className="text-muted-foreground">
                                    Pending intents
                                </span>
                                <Badge variant="secondary">
                                    {stats.draft + stats.review}
                                </Badge>
                            </div>
                            <ul className="space-y-1">
                                <li className="flex items-center gap-2">
                                    <span className="h-1.5 w-1.5 rounded-full bg-amber-400"/>
                                    <span className="truncate">
                                        {stats.review} under review
                                    </span>
                                </li>
                                <li className="flex items-center gap-2">
                                    <span className="h-1.5 w-1.5 rounded-full bg-sky-400"/>
                                    <span className="truncate">
                                        {stats.draft} drafts
                                    </span>
                                </li>
                                <li className="flex items-center gap-2">
                                    <span className="h-1.5 w-1.5 rounded-full bg-emerald-400"/>
                                    <span className="truncate">
                                        {stats.verified} {dataSource === 'live' ? 'active' : 'verified'}
                                    </span>
                                </li>
                            </ul>
                            <Separator />
                            <div className="space-y-1.5">
                                <div className="font-medium">Project lint</div>
                                {dataSource !== 'live' ? <p className="text-muted-foreground">Demo result: {lintStats?.errors ?? 0} errors · {lintStats?.warnings ?? 0} warnings</p>
                                    : lintLoading ? <p className="text-muted-foreground">Checking…</p>
                                    : lintError !== null ? <p className="text-destructive break-words">Lint unavailable: {lintError}</p>
                                    : lintStats !== null ? <p className={lintStats.errors > 0 ? 'text-destructive' : 'text-emerald-400'}>{lintStats.errors} errors · {lintStats.warnings} warnings</p>
                                    : <p className="text-muted-foreground">No active project to lint.</p>}
                            </div>
                        </div>
                    )}

                    {activeView === 'tools' && (
                        <div className="p-4 space-y-3 text-xs">
                            {dataSource === 'live' ? (
                                <>
                                    <div className="flex items-center justify-between">
                                        <span className="text-muted-foreground">
                                            Server
                                        </span>
                                        <Badge
                                            variant={liveError !== null
                                                ? 'destructive'
                                                : 'secondary'}
                                            className="text-[10px] h-4 px-1.5"
                                        >
                                            {liveError !== null
                                                ? 'error'
                                                : liveLoading
                                                    ? 'connecting…'
                                                    : `${liveServerName || 'kron'} ${liveServerVersion}`}
                                        </Badge>
                                    </div>
                                    {liveError !== null && (
                                        <Card className="border-destructive/50 bg-destructive/10">
                                            <CardContent className="p-2.5">
                                                <pre className="text-[10px] text-destructive/90 whitespace-pre-wrap break-all">
                                                    {liveError}
                                                </pre>
                                            </CardContent>
                                        </Card>
                                    )}
                                    <div className="flex items-center justify-between">
                                        <span className="text-muted-foreground">
                                            Tools
                                        </span>
                                        <Badge
                                            variant="secondary"
                                            className="text-[10px] h-4 px-1.5"
                                        >
                                            {liveTools.length}
                                        </Badge>
                                    </div>
                                    <ul className="space-y-0.5 font-mono text-[11px]">
                                        {liveTools.map((name) => (
                                            <li
                                                key={name}
                                                className="flex items-center gap-1.5 px-1.5 py-0.5 rounded hover:bg-accent/60"
                                            >
                                                <CircleAlert className="h-3 w-3 text-muted-foreground"/>
                                                <span className="truncate">
                                                    {name}
                                                </span>
                                            </li>
                                        ))}
                                    </ul>
                                    {liveSource !== '' && (
                                        <div className="pt-1 text-[10px] text-muted-foreground/70 font-mono break-all">
                                            {liveSource}
                                        </div>
                                    )}
                                    <Button
                                        variant="outline"
                                        size="sm"
                                        disabled={liveLoading}
                                        onClick={onRefreshLive}
                                        className="w-full h-7 text-[11px]"
                                    >
                                        {liveLoading ? 'Refreshing…' : 'Refresh'}
                                    </Button>
                                </>
                            ) : (
                                <>
                                    <p className="text-muted-foreground leading-snug">
                                        Mock data — switch to{' '}
                                        <span className="font-semibold">Live</span>{' '}
                                        in Settings to call{' '}
                                        <code className="font-mono">kron serve-mcp</code>
                                        {' '}via the Go bridge.
                                    </p>
                                    <div className="flex items-center justify-between pt-1">
                                        <span className="text-muted-foreground">
                                            Tools
                                        </span>
                                        <Badge
                                            variant="secondary"
                                            className="text-[10px] h-4 px-1.5"
                                        >
                                            {tools.length}
                                        </Badge>
                                    </div>
                                    <ul className="space-y-0.5 font-mono text-[11px]">
                                        {tools.map((name) => (
                                            <li
                                                key={name}
                                                className="flex items-center gap-1.5 px-1.5 py-0.5 rounded hover:bg-accent/60"
                                            >
                                                <CircleAlert className="h-3 w-3 text-muted-foreground"/>
                                                <span className="truncate">
                                                    {name}
                                                </span>
                                            </li>
                                        ))}
                                    </ul>
                                </>
                            )}
                        </div>
                    )}
                </ScrollArea>

                {/* Status footer — current client info, VS Code style */}
                {info !== null && (
                    <div
                        className={[
                            'shrink-0 border-t border-border',
                            'px-3 py-2 text-[10px]',
                            'bg-background/40 text-muted-foreground',
                        ].join(' ')}
                    >
                        <div className="flex items-center gap-2">
                            <span
                                className="h-1.5 w-1.5 rounded-full bg-emerald-400"
                                title="online"
                            />
                            <span className="truncate font-mono">
                                kron {info.version}
                            </span>
                        </div>
                        <div className="font-mono opacity-70 truncate">
                            {info.os}/{info.arch} · go {info.goVersion} · {info.rfc}
                        </div>
                    </div>
                )}
            </div>
        </aside>
    );
}

function ActivityIcon(props: {
    active: boolean;
    label: string;
    onClick: () => void;
    children: React.ReactNode;
}): JSX.Element {
    return (
        <button
            type="button"
            title={props.label}
            onClick={props.onClick}
            className={[
                'relative h-9 w-9 rounded-md',
                'flex items-center justify-center',
                'transition-colors',
                props.active
                    ? 'text-foreground bg-accent'
                    : 'text-muted-foreground hover:text-foreground hover:bg-accent/60',
            ].join(' ')}
        >
            {props.active && (
                <span
                    className={[
                        'absolute left-0 top-1.5 bottom-1.5 w-0.5',
                        'bg-primary rounded-r',
                    ].join(' ')}
                />
            )}
            {props.children}
        </button>
    );
}

function GroupHeader(props: {
    name: string;
    count: number;
    open: boolean;
    onToggle: () => void;
}): JSX.Element {
    return (
        <button
            type="button"
            onClick={props.onToggle}
            className={[
                'w-full flex items-center gap-1',
                'pl-2 pr-3 py-1',
                'text-[11px] uppercase tracking-wider text-muted-foreground',
                'hover:text-foreground',
            ].join(' ')}
        >
            <ChevronRight
                className={[
                    'h-3 w-3 transition-transform',
                    props.open ? 'rotate-90' : '',
                ].join(' ')}
            />
            <span className="font-semibold">{props.name}</span>
            <Badge variant="secondary" className="ml-auto text-[9px] h-4 px-1">
                {props.count}
            </Badge>
        </button>
    );
}

function StatusBadge(props: {status: string}): JSX.Element {
    const tone =
        props.status === 'accepted'
            ? 'border-emerald-500/40 text-emerald-400'
            : props.status === 'review'
                ? 'border-amber-500/40 text-amber-400'
                : props.status === 'stale'
                    ? 'border-destructive/40 text-destructive'
                    : 'border-sky-500/40 text-sky-400';
    return (
        <span
            className={[
                'text-[9px] px-1.5 h-4 rounded',
                'border bg-transparent inline-flex items-center',
                tone,
            ].join(' ')}
        >
            {props.status}
        </span>
    );
}

function StatChip(props: {
    icon: React.ReactNode;
    label: string;
    value: number;
    tone?: 'default' | 'destructive' | 'success';
}): JSX.Element {
    const toneClass =
        props.tone === 'destructive'
            ? 'border-destructive/40 text-destructive'
            : props.tone === 'success'
                ? 'border-emerald-500/40 text-emerald-400'
                : 'border-border text-foreground';
    return (
        <div
            className={[
                'rounded-md border px-2.5 py-1.5',
                'flex items-center gap-2',
                toneClass,
            ].join(' ')}
        >
            <span className="text-muted-foreground">{props.icon}</span>
            <span className="text-xs">{props.label}</span>
            <span className="ml-auto text-sm font-semibold tabular-nums">
                {props.value}
            </span>
        </div>
    );
}

function Row(props: {k: string; v: string; mono?: boolean}): JSX.Element {
    return (
        <div className="flex items-baseline gap-2">
            <dt className="text-muted-foreground shrink-0 w-14">{props.k}</dt>
            <dd
                className={[
                    'truncate',
                    props.mono === true
                        ? 'font-mono text-[11px]'
                        : 'text-foreground',
                ].join(' ')}
                title={props.v}
            >
                {props.v}
            </dd>
        </div>
    );
}

function TopBar(props: {
    filter: string;
    onFilterChange: (v: string) => void;
    rootLabel: string;
    dataSource: DataSource;
    liveTreeLoading: boolean;
    liveTreeError: string | null;
    onRefreshLiveTree: () => void;
}): JSX.Element {
    const {
        filter,
        onFilterChange,
        rootLabel,
        dataSource,
        liveTreeLoading,
        liveTreeError,
        onRefreshLiveTree,
    } = props;
    const isLive = dataSource === 'live';
    return (
        <div className="flex items-center gap-3 px-5 h-14 shrink-0">
            <div className="flex items-center gap-2 text-sm text-muted-foreground min-w-0">
                <History className="h-4 w-4 shrink-0"/>
                <span className="truncate">Intent tree</span>
                <ChevronRight className="h-3.5 w-3.5 shrink-0"/>
                <span className="text-foreground font-medium truncate">
                    {isLive ? 'kron serve-mcp' : rootLabel}
                </span>
            </div>

            <div className="flex-1 max-w-md ml-4 relative">
                <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground"/>
                <Input
                    value={filter}
                    onChange={(e): void => onFilterChange(e.target.value)}
                    placeholder="Filter intents…"
                    className="pl-8 h-8 text-xs"
                />
            </div>

            <div className="ml-auto flex items-center gap-2">
                {/* The GUI is deliberately read-only: no intent CRUD here.
                    Editing happens in the user's IDE on the Markdown files;
                    the GUI only previews what serve-mcp returns. */}
                {isLive && (
                    <Button
                        variant="secondary"
                        size="sm"
                        disabled={liveTreeLoading}
                        onClick={onRefreshLiveTree}
                        title={
                            liveTreeError !== null
                                ? 'Retry the kron_tree fetch'
                                : 'Re-fetch kron_tree from the live server'
                        }
                    >
                        {liveTreeLoading
                            ? 'Refreshing…'
                            : liveTreeError !== null
                                ? 'Retry'
                                : 'Refresh tree'}
                    </Button>
                )}
            </div>
        </div>
    );
}

function DataSourceRadio(props: {
    checked: boolean;
    label: string;
    description: string;
    onSelect: () => void;
}): JSX.Element {
    const {checked, label, description, onSelect} = props;
    return (
        <button
            type="button"
            role="radio"
            aria-checked={checked}
            onClick={onSelect}
            className={[
                'w-full text-left flex items-start gap-2',
                'px-2 py-1.5 rounded border',
                'transition-colors',
                checked
                    ? 'border-primary bg-primary/10 text-foreground'
                    : 'border-border hover:bg-accent/60 text-muted-foreground',
            ].join(' ')}
        >
            <span
                className={[
                    'mt-0.5 h-3 w-3 rounded-full border-2 shrink-0',
                    checked
                        ? 'border-primary bg-primary'
                        : 'border-muted-foreground/40',
                ].join(' ')}
                aria-hidden="true"
            />
            <span className="flex-1 min-w-0">
                <span className="block text-[11px] font-semibold">
                    {label}
                </span>
                <span className="block text-[10px] text-muted-foreground leading-snug">
                    {description}
                </span>
            </span>
        </button>
    );
}

export default App;
