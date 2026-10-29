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
    CircleAlert,
    ShieldCheck,
    History,
    Sparkles,
    Search,
    ChevronRight,
    ChevronDown,
    Folder,
    ImagePlus,
    Settings,
    FolderOpen,
    ArrowRight,
    ArrowLeft,
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
import {AddProject, ChooseProjectCover, GetGUISettings, GetKronInfo, GetProjectCoverData, ListMCPToolsLive, CallMCPTool, ListProjectsLive, OpenProjectFolder, SetGUIProject, SetGUITheme, SwitchProject} from '../wailsjs/go/main/App';
import {main} from '../wailsjs/go/models';
import {type KronInfo, type IntentNode, type TreeStats} from '@/lib/intent-types';
import {BootAnimation} from '@/components/boot-animation';
import {IntentCanvas} from '@/components/intent-canvas';
import {kronTreeToIntentNodes} from '@/lib/real-tree';
import {
    type AssumptionListOutput,
    type AssumptionSummary,
    type ListProjectsLiveResult,
} from '@/lib/mcp-types';

/** Left-rail shape of an assumption (normalised from kron_assume_list). */
interface RailAssumption {
    id: string;
    text: string;
    severity: 'hard' | 'soft';
    expiresAt?: string;
    verifiedAt?: string;
    verifiedBy?: string;
}

type GUITheme = 'system' | 'light' | 'dark';

// The GUI always talks to a real `kron serve-mcp` subprocess (via the Go
// bridge). There is no built-in demo data: the shared integration fixture
// lives at kron-mcp-test/project and is opened like any other project.

function App() {
    const [booted, setBooted] = useState<boolean>(false);
    const [info, setInfo] = useState<KronInfo | null>(null);
    const [loadError, setLoadError] = useState<string | null>(null);
    const [filter, setFilter] = useState<string>('');
    const [stats, setStats] = useState<TreeStats>(emptyStats());
    const [liveTools, setLiveTools] = useState<string[]>([]);
    const [liveServerName, setLiveServerName] = useState<string>('');
    const [liveServerVersion, setLiveServerVersion] = useState<string>('');
    const [liveSource, setLiveSource] = useState<string>('');
    const [liveError, setLiveError] = useState<string | null>(null);
    const [liveLoading, setLiveLoading] = useState<boolean>(false);
    // 1.4.c: live intent tree state. liveTree holds the
    // kron_tree-derived IntentNode[];
    // whenever the user is in 'live' mode AND the fetch has
    // succeeded. liveTreeError surfaces a friendly message in
    // place of the tree on failure. liveTreeLoading controls a
    // subtle skeleton in the main pane (re-using the existing
    // "Connecting…" / "Refreshing…" pattern from liveTools).
    const [liveTree, setLiveTree] = useState<IntentNode[] | null>(null);
    const [liveTreeError, setLiveTreeError] = useState<string | null>(null);
    const [liveTreeLoading, setLiveTreeLoading] = useState<boolean>(false);
    const [noActiveProject, setNoActiveProject] = useState<boolean>(false);
    const [lintStats, setLintStats] = useState<{errors: number; warnings: number} | null>(null);
    const [lintError, setLintError] = useState<string | null>(null);
    const [lintLoading, setLintLoading] = useState<boolean>(false);
    // B-3 assumption registry (separate from the intent tree), filled from
    // kron_assume_list.
    const [liveAssumptions, setLiveAssumptions] = useState<AssumptionSummary[]>([]);
    const [assumptionsError, setAssumptionsError] = useState<string | null>(null);
    const [switchingProject, setSwitchingProject] = useState<string | null>(null);
    const [projectSwitchError, setProjectSwitchError] = useState<string | null>(null);
    const [settingsOpen, setSettingsOpen] = useState<boolean>(false);
    const [theme, setTheme] = useState<GUITheme>('system');
    const [settingsError, setSettingsError] = useState<string | null>(null);
    const [settingsSaving, setSettingsSaving] = useState<boolean>(false);
    const [guiSettings, setGuiSettings] = useState<main.GUISettings | null>(null);
    const [workspaceEntered, setWorkspaceEntered] = useState<boolean>(false);
    const [addingProject, setAddingProject] = useState<boolean>(false);
    // Selection is owned here so the left-rail rows can drive the canvas.
    const [selectedIntentId, setSelectedIntentId] = useState<string | null>(null);
    const [focusRequest, setFocusRequest] = useState<number>(0);

    // The projects view reads the portable registry through the Go
    // bridge and is shown in the Projects view.
    const [liveProjects, setLiveProjects] = useState<ListProjectsLiveResult | null>(
        null,
    );
    const [liveProjectsError, setLiveProjectsError] = useState<string | null>(null);
    const [liveProjectsLoading, setLiveProjectsLoading] = useState<boolean>(false);

    useEffect(() => {
        let cancelled = false;
        void GetGUISettings().then((settings: main.GUISettings) => {
            if (cancelled) return;
            const nextTheme: GUITheme = settings.theme === 'light' || settings.theme === 'dark'
                ? settings.theme
                : 'system';
            setTheme(nextTheme);
            setGuiSettings(settings);
        }).catch((err: unknown) => {
            if (!cancelled) setSettingsError(err instanceof Error ? err.message : String(err));
        });
        return (): void => { cancelled = true; };
    }, []);

    useEffect(() => {
        const root = document.documentElement;
        const media = window.matchMedia('(prefers-color-scheme: dark)');
        const apply = (): void => {
            const isDark = theme === 'dark' || (theme === 'system' && media.matches);
            root.classList.toggle('dark', isDark);
            root.style.colorScheme = isDark ? 'dark' : 'light';
        };
        apply();
        if (theme !== 'system') return;
        media.addEventListener('change', apply);
        return (): void => media.removeEventListener('change', apply);
    }, [theme]);

    const changeTheme = useCallback((nextTheme: GUITheme): void => {
        setSettingsSaving(true);
        setSettingsError(null);
        void SetGUITheme(nextTheme).then(() => {
            setTheme(nextTheme);
        }).catch((err: unknown) => {
            setSettingsError(err instanceof Error ? err.message : String(err));
        }).finally(() => setSettingsSaving(false));
    }, []);

    const saveProjectSettings = useCallback((projectPath: string, displayName: string): void => {
        setSettingsSaving(true);
        setSettingsError(null);
        void SetGUIProject(projectPath, displayName).then((settings) => {
            setGuiSettings(settings);
        }).catch((err: unknown) => {
            setSettingsError(err instanceof Error ? err.message : String(err));
        }).finally(() => setSettingsSaving(false));
    }, []);

    const chooseProjectCover = useCallback((projectPath: string): void => {
        setSettingsSaving(true);
        setSettingsError(null);
        void ChooseProjectCover(projectPath).then((settings) => {
            setGuiSettings(settings);
        }).catch((err: unknown) => {
            setSettingsError(err instanceof Error ? err.message : String(err));
        }).finally(() => setSettingsSaving(false));
    }, []);

    const displayTree: IntentNode[] = liveTree ?? [];
    const showLiveTreeSkeleton: boolean = liveTree === null && liveTreeError === null;
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
    // liveTreeError; the main pane renders an empty canvas
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

            // Assumption registry listing. Failure is non-fatal: the tree and
            // lint counts stay usable, the rail surfaces the error instead.
            try {
                const assumptionsText = await CallMCPTool('kron_assume_list', '{}');
                const parsed = JSON.parse(assumptionsText) as AssumptionListOutput;
                setLiveAssumptions(Array.isArray(parsed.assumptions) ? parsed.assumptions : []);
                setAssumptionsError(null);
            } catch (err: unknown) {
                setLiveAssumptions([]);
                setAssumptionsError(err instanceof Error ? err.message : String(err));
            }
        } catch (err: unknown) {
            const msg = err instanceof Error ? err.message : String(err);
            setLiveTreeError(msg);
            // Keep the previous liveTree on a failed refresh so the
            // UI does not flicker empty — but never leave placeholder
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
            await refreshLive();
            await refreshLiveTree();
        } catch (err: unknown) {
            const message = err instanceof Error ? err.message : String(err);
            setProjectSwitchError(message);
            setLiveTreeError(message);
        } finally {
            setSwitchingProject(null);
        }
    }, [refreshLive, refreshLiveTree]);

    const openProject = useCallback((path: string): void => {
        void switchLiveProject(path).then(() => setWorkspaceEntered(true));
    }, [switchLiveProject]);

    const addProject = useCallback((): void => {
        setAddingProject(true);
        setSettingsError(null);
        void AddProject().then(async (projects) => {
            setLiveProjects(projects);
            const settings = await GetGUISettings();
            setGuiSettings(settings);
        }).catch((err: unknown) => {
            setSettingsError(err instanceof Error ? err.message : String(err));
        }).finally(() => setAddingProject(false));
    }, []);

    useEffect(() => {
        let cancelled = false;
        (async (): Promise<void> => {
            try {
                const kronInfo = await GetKronInfo();
                if (cancelled) return;
                setInfo(kronInfo);
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

    // On mount: read the projects registry, then hand-shake with `kron
    // serve-mcp` and pull the tool list + intent tree. There is no offline
    // demo path any more — the integration fixture is opened as a project.
    useEffect(() => {
        void (async (): Promise<void> => {
            await refreshLiveProjects();
            await refreshLive();
            await refreshLiveTree();
        })();
    }, [refreshLive, refreshLiveTree, refreshLiveProjects]);

    const railAssumptions: RailAssumption[] = liveAssumptions.map((item) => ({
        id: item.id,
        text: item.text,
        severity: item.default_severity === 'soft' ? 'soft' : 'hard',
        ...(item.expires_at !== undefined ? {expiresAt: item.expires_at} : {}),
        ...(item.verified_at !== undefined ? {verifiedAt: item.verified_at} : {}),
        ...(item.verified_by !== undefined ? {verifiedBy: item.verified_by} : {}),
    }));

    if (settingsOpen) {
        return (
            <>
                {!booted && (
                    <BootAnimation onDone={(): void => setBooted(true)}/>
                )}
                {booted && (
                    <SettingsPage
                        theme={theme}
                        settings={guiSettings}
                        projects={liveProjects}
                        settingsError={settingsError}
                        settingsSaving={settingsSaving}
                        onThemeChange={changeTheme}
                        onSaveProject={saveProjectSettings}
                        onChooseCover={chooseProjectCover}
                        onClose={(): void => setSettingsOpen(false)}
                    />
                )}
            </>
        );
    }

    if (!workspaceEntered) {
        return (
            <>
                {!booted && (
                    <BootAnimation onDone={(): void => setBooted(true)}/>
                )}
                {booted && (
                    <ProjectHome
                        projects={liveProjects}
                        settings={guiSettings}
                        onOpenProject={openProject}
                        addingProject={addingProject}
                        addProject={addProject}
                    />
                )}
            </>
        );
    }

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
                    error={loadError}
                    stats={stats}
                    tree={displayTree}
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
                    currentProjectPath={liveProjects?.current ?? ''}
                    onOpenProjectFolder={(): void => {
                        if (liveProjects?.current === undefined || liveProjects.current === '') return;
                        void OpenProjectFolder(liveProjects.current).catch((err: unknown) => {
                            setLiveError(err instanceof Error ? err.message : String(err));
                        });
                    }}
                    theme={theme}
                    settingsError={settingsError}
                    settingsSaving={settingsSaving}
                    onThemeChange={changeTheme}
                    assumptions={railAssumptions}
                    assumptionsError={assumptionsError}
                    onSelectIntent={(id): void => {
                        setSelectedIntentId(id);
                        setFocusRequest((n) => n + 1);
                    }}
                />
                <main className="flex-1 flex flex-col min-w-0">
                    <TopBar
                        filter={filter}
                        onFilterChange={setFilter}
                        rootLabel={liveProjectName}
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
                        ) : noActiveProject ? (
                            <div className="p-6 text-sm text-muted-foreground">No active Kron project. Run kron init or kron register, refresh Projects, then select a project.</div>
                        ) : liveTreeError !== null ? (
                            <div className="p-6 text-sm text-destructive">
                                {liveTreeError.includes('cannot find the Kron CLI binary')
                                    ? 'Live mode needs the Kron CLI binary. Build it with `go build -o frontend/wails/build/bin/kron-cli.exe ./cmd/kron` (or put a kron CLI next to the GUI executable), then Retry.'
                                    : 'Could not load the live project tree. Use Retry below after checking the MCP connection.'}
                            </div>
                        ) : displayTree.length === 0 ? (
                            <div className="p-6 text-sm text-muted-foreground">This project has no intents yet.</div>
                        ) : (
                            <IntentCanvas
                                data={displayTree}
                                projectName={liveProjectName}
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
                        any more. */}
                    {liveTreeError !== null && (
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
                    {liveTreeError === null &&
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

function ProjectHome(props: {
    projects: ListProjectsLiveResult | null;
    settings: main.GUISettings | null;
    onOpenProject: (path: string) => void;
    addingProject: boolean;
    addProject: () => void;
}): JSX.Element {
    const [covers, setCovers] = useState<Record<string, string>>({});

    useEffect(() => {
        let cancelled = false;
        const paths = props.projects?.projects.map((project) => project.path) ?? [];
        void Promise.all(paths.map(async (path) => {
            try {
                const data = await GetProjectCoverData(path);
                return data === '' ? null : {path, data};
            } catch {
                return null;
            }
        })).then((results) => {
            if (cancelled) return;
            const next: Record<string, string> = {};
            for (const result of results) {
                if (result !== null) next[result.path] = result.data;
            }
            setCovers(next);
        });
        return (): void => { cancelled = true; };
    }, [props.projects, props.settings]);

    return (
        <main className="flex min-h-screen w-screen items-center justify-center overflow-hidden bg-background text-foreground">
            <div className="w-full max-w-7xl px-8">
                <div className="flex gap-6 overflow-x-auto pb-5 pt-5 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
                    {(props.projects?.projects ?? []).map((project) => {
                            const metadata = props.settings?.projects?.[project.path];
                            const displayName = metadata?.displayName?.trim() || project.path.split(/[\\/]/).filter(Boolean).pop() || project.path;
                            const cover = covers[project.path];
                            const isCurrent = project.path === props.projects?.current;
                            return (
                                <button
                                    key={project.path}
                                    type="button"
                                    onClick={(): void => props.onOpenProject(project.path)}
                                    className="group w-56 shrink-0 text-left"
                                >
                                    <div className={["relative aspect-square overflow-hidden rounded-lg bg-card shadow-sm transition-all duration-200 group-hover:-translate-y-1 group-hover:ring-1 group-hover:ring-primary/40", isCurrent ? 'ring-1 ring-primary/50' : ''].join(' ')}>
                                        <img src={cover || '/kronLogo.png'} alt="" className="h-full w-full object-cover"/>
                                        <div className="absolute inset-x-0 bottom-0 flex items-center justify-between bg-gradient-to-t from-black/80 to-transparent px-4 pb-4 pt-12 text-white">
                                            <span className="text-xs font-medium">{isCurrent ? 'Current project' : 'Open project'}</span>
                                            <ArrowRight className="h-4 w-4 opacity-0 transition-opacity group-hover:opacity-100"/>
                                        </div>
                                    </div>
                                    <div className="mt-3 truncate text-sm font-semibold">{displayName}</div>
                                    <div className="mt-1 truncate font-mono text-[11px] text-muted-foreground" title={project.path}>{project.path}</div>
                                </button>
                            );
                        })}
                    <button type="button" onClick={props.addProject} disabled={props.addingProject} className="group w-56 shrink-0 text-left disabled:cursor-wait">
                        <div className="flex aspect-square items-center justify-center border border-dashed border-border bg-card transition-all duration-200 group-hover:-translate-y-1 group-hover:border-primary group-hover:bg-accent">
                            <span className="text-7xl font-extralight text-muted-foreground transition-colors group-hover:text-foreground">{props.addingProject ? '…' : '+'}</span>
                        </div>
                        <div className="mt-3 text-sm font-semibold">Add project</div>
                        <div className="mt-1 text-[11px] text-muted-foreground">Choose a folder</div>
                    </button>
                </div>
            </div>
        </main>
    );
}

function SettingsPage(props: {
    theme: GUITheme;
    settings: main.GUISettings | null;
    projects: ListProjectsLiveResult | null;
    settingsError: string | null;
    settingsSaving: boolean;
    onThemeChange: (theme: GUITheme) => void;
    onSaveProject: (path: string, name: string) => void;
    onChooseCover: (path: string) => void;
    onClose: () => void;
}): JSX.Element {
    const projectPaths = new Set([
        ...Object.keys(props.settings?.projects ?? {}),
        ...(props.projects?.projects.map((project) => project.path) ?? []),
    ]);
    return (
        <main className="min-h-screen w-screen overflow-auto bg-background text-foreground">
            <div className="mx-auto max-w-4xl px-8 py-8">
                <div className="flex items-center gap-3">
                    <Button variant="ghost" size="sm" onClick={props.onClose}><ArrowLeft className="mr-2 h-4 w-4"/> Back</Button>
                    <h1 className="text-2xl font-semibold">Settings</h1>
                </div>
                <Separator className="my-6"/>
                <Card>
                    <CardHeader><CardTitle className="text-base">Appearance</CardTitle><CardDescription>Choose how Kron should render its desktop interface.</CardDescription></CardHeader>
                    <CardContent>
                        <label className="flex max-w-sm items-center justify-between gap-4 text-sm">
                            <span>Theme</span>
                            <select value={props.theme} disabled={props.settingsSaving} onChange={(event): void => {
                                const value = event.target.value;
                                if (value === 'system' || value === 'light' || value === 'dark') props.onThemeChange(value);
                            }} className="h-9 rounded-md border border-input bg-background px-3 text-sm">
                                <option value="system">System</option><option value="light">Light</option><option value="dark">Dark</option>
                            </select>
                        </label>
                    </CardContent>
                </Card>
                <Card className="mt-6">
                    <CardHeader><CardTitle className="text-base">Project cards</CardTitle><CardDescription>Customize the name and cover shown on the project home screen.</CardDescription></CardHeader>
                    <CardContent className="space-y-4">
                        {projectPaths.size === 0 ? <p className="text-sm text-muted-foreground">Project cards will appear here after projects are registered.</p> : Array.from(projectPaths).map((path) => (
                            <ProjectSettingRow key={path} path={path} metadata={props.settings?.projects?.[path] ?? {displayName: '', coverImage: ''}} disabled={props.settingsSaving} onSave={props.onSaveProject} onChooseCover={props.onChooseCover}/>
                        ))}
                    </CardContent>
                </Card>
                {props.settingsError !== null && <p role="alert" className="mt-4 text-sm text-destructive">{props.settingsError}</p>}
            </div>
        </main>
    );
}

function ProjectSettingRow(props: {
    path: string;
    metadata: main.GUIProjectSettings;
    disabled: boolean;
    onSave: (path: string, name: string) => void;
    onChooseCover: (path: string) => void;
}): JSX.Element {
    const fallback = props.path.split(/[\\/]/).filter(Boolean).pop() || props.path;
    const [name, setName] = useState<string>(props.metadata.displayName || fallback);
    const [cover, setCover] = useState<string>('');
    useEffect(() => {
        let cancelled = false;
        void GetProjectCoverData(props.path).then((data) => {
            if (!cancelled) setCover(data);
        }).catch(() => {
            if (!cancelled) setCover('');
        });
        return (): void => { cancelled = true; };
    }, [props.path, props.metadata.coverImage]);
    return (
        <div className="flex flex-wrap items-center gap-3 rounded-lg border border-border p-3">
            <img src={cover === '' ? '/kronLogo.png' : cover} alt="Project cover preview" className="h-14 w-14 shrink-0 rounded-md border border-border object-cover"/>
            <div className="min-w-0 flex-1"><div className="truncate text-sm font-medium">{fallback}</div><div className="truncate font-mono text-[11px] text-muted-foreground" title={props.path}>{props.path}</div></div>
            <Input value={name} disabled={props.disabled} onChange={(event): void => setName(event.target.value)} className="w-48" aria-label={`Display name for ${fallback}`}/>
            <Button variant="outline" size="sm" disabled={props.disabled} onClick={(): void => props.onChooseCover(props.path)}><ImagePlus className="mr-2 h-4 w-4"/> Cover</Button>
            <Button size="sm" disabled={props.disabled} onClick={(): void => props.onSave(props.path, name)}>Save</Button>
        </div>
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

function LeftRail(props: {
    info: KronInfo | null;
    error: string | null;
    stats: TreeStats;
    tree: IntentNode[];
    liveTools: string[];
    liveServerName: string;
    liveServerVersion: string;
    liveSource: string;
    liveError: string | null;
    liveLoading: boolean;
    onRefreshLive: () => void;
    liveProjects: ListProjectsLiveResult | null;
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
    currentProjectPath: string;
    onOpenProjectFolder: () => void;
    theme: GUITheme;
    settingsError: string | null;
    settingsSaving: boolean;
    onThemeChange: (theme: GUITheme) => void;
    /** B-3 assumption registry entries (from kron_assume_list). */
    assumptions: RailAssumption[];
    assumptionsError: string | null;
    /** Select an intent in the canvas (left-rail row click). */
    onSelectIntent: (id: string) => void;
}): JSX.Element {
    const {
        info,
        error,
        stats,
        tree,
        liveTools,
        liveServerName,
        liveServerVersion,
        liveSource,
        liveError,
        liveLoading,
        onRefreshLive,
        liveProjects,
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
        currentProjectPath,
        onOpenProjectFolder,
        theme,
        settingsError,
        settingsSaving,
        onThemeChange,
        assumptions,
        assumptionsError,
        onSelectIntent,
    } = props;
    const [activeView, setActiveView] = useState<
        'explorer' | 'projects' | 'git' | 'tools'
    >('explorer');
    const [openGroups, setOpenGroups] = useState<Record<string, boolean>>({
        Files: true,
        Archived: true,
        Assumptions: true,
        'MCP Tools': false,
    });

    // Which intents declare each assumption (derived from the tree, so it
    // always matches what the canvas shows).
    const referencedBy = new Map<string, string[]>();
    const walkAssumptions = (nodes: IntentNode[]): void => {
        for (const node of nodes) {
            for (const assumption of node.assumptions ?? []) {
                const list = referencedBy.get(assumption.id) ?? [];
                list.push(node.id);
                referencedBy.set(assumption.id, list);
            }
            walkAssumptions(node.children);
        }
    };
    walkAssumptions(tree);

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
                    <Settings className="h-5 w-5"/>
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
                    <div className="flex items-center gap-1">
                        <Button
                            variant="ghost"
                            size="sm"
                            className="h-5 w-5 p-0 text-muted-foreground"
                        >
                            <History className="h-3.5 w-3.5"/>
                        </Button>
                        {activeView === 'explorer' && currentProjectPath !== '' ? (
                            <Button
                                variant="ghost"
                                size="sm"
                                className="h-5 w-5 p-0 text-muted-foreground"
                                title="Open project folder"
                                aria-label="Open project folder"
                                onClick={onOpenProjectFolder}
                            >
                                <FolderOpen className="h-3.5 w-3.5"/>
                            </Button>
                        ) : null}
                    </div>
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
                                        GUI preferences are stored in <code className="font-mono">guiSettings.json</code>.
                                    </CardDescription>
                                </CardHeader>
                                <CardContent className="p-3 pt-0 space-y-1.5">
                                    <div className="flex items-center justify-between gap-2">
                                        <span className="text-muted-foreground">Server</span>
                                        <span className="truncate font-mono">
                                            {liveServerName === ''
                                                ? 'not connected'
                                                : `${liveServerName} ${liveServerVersion}`}
                                        </span>
                                    </div>
                                    <div className="flex items-center justify-between gap-2">
                                        <span className="text-muted-foreground">Tools</span>
                                        <span className="font-mono">{liveTools.length}</span>
                                    </div>
                                    <div className="flex items-center justify-between gap-2">
                                        <span className="text-muted-foreground">CLI</span>
                                        <span className="truncate font-mono" title={liveSource}>{liveSource === '' ? 'unresolved' : liveSource}</span>
                                    </div>
                                    <div className="flex items-center justify-between gap-2">
                                        <span className="text-muted-foreground">Registry</span>
                                        <span className="truncate font-mono" title={liveProjects?.projectsPath ?? ''}>{liveProjects?.projectsPath ?? 'not loaded'}</span>
                                    </div>
                                    <label className="flex items-center justify-between gap-2 pt-1">
                                        <span className="text-muted-foreground">Theme</span>
                                        <select
                                            value={theme}
                                            disabled={settingsSaving}
                                            onChange={(event): void => {
                                                const value = event.target.value;
                                                if (value === 'system' || value === 'light' || value === 'dark') onThemeChange(value);
                                            }}
                                            className="h-7 rounded-md border border-input bg-background px-2 text-[11px] text-foreground"
                                        >
                                            <option value="system">System</option>
                                            <option value="light">Light</option>
                                            <option value="dark">Dark</option>
                                        </select>
                                    </label>
                                    {settingsError !== null && <p role="alert" className="text-[10px] text-destructive break-words">{settingsError}</p>}
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
                                        <RailTreeRow
                                            key={node.id}
                                            node={node}
                                            depth={0}
                                            onSelectIntent={onSelectIntent}
                                        />
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
                                    {collectArchivedLeaves(tree).map((node) => (
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

                            <GroupHeader
                                name="Assumptions"
                                count={assumptions.length}
                                open={openGroups['Assumptions'] ?? true}
                                onToggle={(): void => toggleGroup('Assumptions')}
                            />
                            {openGroups['Assumptions'] && (
                                <div className="pb-2">
                                    {assumptionsError !== null && (
                                        <p className="px-3 py-1 text-[10px] leading-snug text-destructive break-words">
                                            {assumptionsError}
                                        </p>
                                    )}
                                    {assumptionsError === null && assumptions.length === 0 && (
                                        <p className="px-3 py-1 text-[11px] text-muted-foreground">
                                            No assumptions registered.
                                        </p>
                                    )}
                                    <ul className="text-sm">
                                        {assumptions.map((assumption) => (
                                            <AssumptionRailRow
                                                key={assumption.id}
                                                assumption={assumption}
                                                intents={referencedBy.get(assumption.id) ?? []}
                                                onSelectIntent={onSelectIntent}
                                            />
                                        ))}
                                    </ul>
                                </div>
                            )}
                        </div>
                    )}

                    {/* 1.4.d: Projects view. Renders the live
                        projects.json registry (kron_list_projects)
                        when the user is in 'live' mode, and a
                        short hint pointing to Settings otherwise
                        (mirrors the Tools view).
                        The view is intentionally read-only: there
                        is no "switch" affordance yet (work 1.4.d
                        scope) — the "active" project is just
                        highlighted via Badge. */}
                    {activeView === 'projects' && (
                        <div className="p-4 space-y-3 text-xs">
                            {(
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
                                        {stats.verified} active
                                    </span>
                                </li>
                            </ul>
                            <Separator />
                            <div className="space-y-1.5">
                                <div className="font-medium">Project lint</div>
                                {lintLoading ? <p className="text-muted-foreground">Checking…</p>
                                    : lintError !== null ? <p className="text-destructive break-words">Lint unavailable: {lintError}</p>
                                    : lintStats !== null ? <p className={lintStats.errors > 0 ? 'text-destructive' : 'text-emerald-400'}>{lintStats.errors} errors · {lintStats.warnings} warnings</p>
                                    : <p className="text-muted-foreground">No active project to lint.</p>}
                            </div>
                        </div>
                    )}

                    {activeView === 'tools' && (
                        <div className="p-4 space-y-3 text-xs">
                            {(
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

// Recursive left-rail tree row: folders expand/collapse, leaves select the
// matching canvas node. Previously only the top-level folders were rendered,
// so the intents inside them were unreachable from the rail.
function RailTreeRow(props: {
    node: IntentNode;
    depth: number;
    onSelectIntent: (id: string) => void;
}): JSX.Element {
    const {node, depth, onSelectIntent} = props;
    const hasChildren = node.children.length > 0;
    const [open, setOpen] = useState<boolean>(depth === 0);
    const activate = (): void => {
        if (hasChildren) {
            setOpen((value) => !value);
            return;
        }
        onSelectIntent(node.id);
    };
    return (
        <li>
            <div
                role="button"
                tabIndex={0}
                title={hasChildren ? 'Expand or collapse' : 'Show this intent in the canvas'}
                onClick={activate}
                onKeyDown={(event): void => {
                    if (event.key === 'Enter' || event.key === ' ') {
                        event.preventDefault();
                        activate();
                    }
                }}
                className={[
                    'group flex items-center gap-1.5',
                    'pr-2 py-1 cursor-pointer',
                    'hover:bg-accent/60',
                ].join(' ')}
                style={{paddingLeft: 8 + depth * 14}}
            >
                <span className="flex w-3.5 shrink-0 items-center justify-center text-muted-foreground/60">
                    {hasChildren ? (
                        open ? (
                            <ChevronDown className="h-3.5 w-3.5"/>
                        ) : (
                            <ChevronRight className="h-3.5 w-3.5"/>
                        )
                    ) : null}
                </span>
                {hasChildren ? (
                    <Folder className="h-3.5 w-3.5 shrink-0 text-muted-foreground"/>
                ) : (
                    <FileCode2 className="h-3.5 w-3.5 shrink-0 text-muted-foreground"/>
                )}
                <span className="truncate flex-1">{node.title}</span>
                <StatusBadge status={node.status}/>
            </div>
            {hasChildren && open && (
                <ul>
                    {node.children.map((child) => (
                        <RailTreeRow
                            key={child.id}
                            node={child}
                            depth={depth + 1}
                            onSelectIntent={onSelectIntent}
                        />
                    ))}
                </ul>
            )}
        </li>
    );
}

/** Leaves that are stale/superseded anywhere in the tree. */
function collectArchivedLeaves(nodes: IntentNode[]): IntentNode[] {
    const out: IntentNode[] = [];
    const walk = (list: IntentNode[]): void => {
        for (const node of list) {
            if (node.children.length === 0) {
                if (node.status === 'stale') out.push(node);
                continue;
            }
            walk(node.children);
        }
    };
    walk(nodes);
    return out;
}

// One assumption in the rail. Clicking expands its registry text, verification
// metadata and the intents that reference it (with jump-to-canvas chips).
function AssumptionRailRow(props: {
    assumption: RailAssumption;
    intents: string[];
    onSelectIntent: (id: string) => void;
}): JSX.Element {
    const {assumption, intents, onSelectIntent} = props;
    const [open, setOpen] = useState<boolean>(false);
    return (
        <li>
            <div
                role="button"
                tabIndex={0}
                title="Show assumption details"
                onClick={(): void => setOpen((value) => !value)}
                onKeyDown={(event): void => {
                    if (event.key === 'Enter' || event.key === ' ') {
                        event.preventDefault();
                        setOpen((value) => !value);
                    }
                }}
                className={[
                    'group flex items-center gap-1.5',
                    'pl-6 pr-2 py-1 cursor-pointer',
                    'hover:bg-accent/60',
                ].join(' ')}
            >
                <span className="flex w-3.5 shrink-0 items-center justify-center text-muted-foreground/60">
                    {open ? (
                        <ChevronDown className="h-3.5 w-3.5"/>
                    ) : (
                        <ChevronRight className="h-3.5 w-3.5"/>
                    )}
                </span>
                <ShieldCheck className="h-3.5 w-3.5 shrink-0 text-muted-foreground"/>
                <span className="truncate flex-1 font-mono text-[11px]">{assumption.id}</span>
                <Badge
                    variant={assumption.severity === 'hard' ? 'destructive' : 'secondary'}
                    className="text-[9px] h-4 px-1"
                >
                    {assumption.severity}
                </Badge>
            </div>
            {open && (
                <div className="space-y-1 px-6 pb-2 text-[11px] leading-snug text-muted-foreground">
                    <p>{assumption.text}</p>
                    {(assumption.expiresAt !== undefined || assumption.verifiedAt !== undefined) && (
                        <p className="text-[10px] text-muted-foreground/80">
                            {assumption.expiresAt !== undefined ? `expires ${assumption.expiresAt}` : 'no expiry'}
                            {assumption.verifiedAt !== undefined
                                ? ` · verified ${assumption.verifiedAt}${assumption.verifiedBy !== undefined ? ` by ${assumption.verifiedBy}` : ''}`
                                : ' · never verified'}
                        </p>
                    )}
                    {intents.length > 0 ? (
                        <div className="flex flex-wrap items-center gap-1">
                            <span className="text-muted-foreground/70">referenced by</span>
                            {intents.map((id) => (
                                <button
                                    key={id}
                                    type="button"
                                    className="rounded bg-muted px-1.5 py-0.5 font-mono text-[10px] hover:bg-accent hover:text-foreground"
                                    onClick={(event): void => {
                                        event.stopPropagation();
                                        onSelectIntent(id);
                                    }}
                                >
                                    {id}
                                </button>
                            ))}
                        </div>
                    ) : (
                        <p className="text-[10px] text-muted-foreground/60">referenced by no intent yet</p>
                    )}
                </div>
            )}
        </li>
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
    liveTreeLoading: boolean;
    liveTreeError: string | null;
    onRefreshLiveTree: () => void;
}): JSX.Element {
    const {
        filter,
        onFilterChange,
        rootLabel,
        liveTreeLoading,
        liveTreeError,
        onRefreshLiveTree,
    } = props;
    return (
        <div className="flex items-center gap-3 px-5 h-14 shrink-0">
            <div className="flex items-center gap-2 text-sm text-muted-foreground min-w-0">
                <History className="h-4 w-4 shrink-0"/>
                <span className="truncate">Intent tree</span>
                <ChevronRight className="h-3.5 w-3.5 shrink-0"/>
                <span className="text-foreground font-medium truncate">
                    {rootLabel}
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
            </div>
        </div>
    );
}

export default App;
