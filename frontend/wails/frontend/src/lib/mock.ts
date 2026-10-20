// Kron GUI — mock data (work 2.x demo).
// Replaces real `kron serve-mcp` answers while the backend is still wiring up.

export type IntentKind = 'adr' | 'rfc' | 'spec' | 'note';
export type IntentStatus = 'draft' | 'review' | 'accepted' | 'stale';

export interface IntentNode {
    id: string;
    title: string;
    kind: IntentKind;
    status: IntentStatus;
    updatedAt: string;
    summary?: string;
    // Work 1.4.e: absolute on-disk path of the .md file. Present
    // for leaves in live mode (populated from kron_get.source_path
    // by real-tree.ts); undefined for synthetic groups (kind='spec'
    // + has children) and for every node in mock mode. The
    // IntentTree component uses this to decide whether clicking the
    // row opens the file in the user's IDE (work 1.4.e "edit-jump-IDE"
    // component, RFC §3.1).
    path?: string;
    children: IntentNode[];
}

export interface KronInfo {
    name: string;
    version: string;
    buildTime: string;
    goVersion: string;
    os: string;
    arch: string;
    description: string;
    rfc: string;
}

export interface TreeStats {
    intents: number;
    groups: number;
    stale: number;
    verified: number;
    draft: number;
    review: number;
    archived: number;
}

const now = new Date();
const isoMinusDays = (d: number): string =>
    new Date(now.getTime() - d * 86_400_000).toISOString();

export const mockStats: TreeStats = {
    intents: 11,
    groups: 4,
    stale: 2,
    verified: 6,
    draft: 3,
    review: 2,
    archived: 2,
};

export const mockIntentTree: IntentNode[] = [
    {
        id: 'g-foundations',
        title: 'Foundations',
        kind: 'spec',
        status: 'accepted',
        updatedAt: isoMinusDays(2),
        summary: 'Language, runtime, and core packaging decisions.',
        children: [
            {
                id: 'i-lang-go',
                title: 'Adopt Go for the core runtime',
                kind: 'adr',
                status: 'accepted',
                updatedAt: isoMinusDays(40),
                summary: 'Single static binary, fast cold start, excellent std lib.',
                children: [],
            },
            {
                id: 'i-license-apache2',
                title: 'License under Apache-2.0',
                kind: 'adr',
                status: 'accepted',
                updatedAt: isoMinusDays(60),
                children: [],
            },
        ],
    },
    {
        id: 'g-ledger',
        title: 'Intent ledger format',
        kind: 'spec',
        status: 'review',
        updatedAt: isoMinusDays(1),
        summary: 'Markdown + frontmatter under ./intents/.',
        children: [
            {
                id: 'i-fm-schema',
                title: 'Frontmatter schema v0.1',
                kind: 'spec',
                status: 'review',
                updatedAt: isoMinusDays(1),
                summary: 'id, kind, status, updated_at, tags[].',
                children: [],
            },
            {
                id: 'i-status-states',
                title: 'Status state machine',
                kind: 'adr',
                status: 'draft',
                updatedAt: isoMinusDays(0),
                summary: 'draft → review → accepted → (stale|superseded).',
                children: [],
            },
        ],
    },
    {
        id: 'g-mcp',
        title: 'MCP integration',
        kind: 'spec',
        status: 'stale',
        updatedAt: isoMinusDays(20),
        summary: 'Expose kron commands as MCP tools over stdio JSON-RPC.',
        children: [
            {
                id: 'i-mcp-stdio',
                title: 'Use stdio JSON-RPC transport',
                kind: 'adr',
                status: 'accepted',
                updatedAt: isoMinusDays(30),
                children: [],
            },
            {
                id: 'i-mcp-toolset',
                title: 'Initial tool set',
                kind: 'rfc',
                status: 'stale',
                updatedAt: isoMinusDays(20),
                summary: 'Needs refresh — list still under discussion.',
                children: [],
            },
        ],
    },
    {
        id: 'g-gui',
        title: 'Desktop GUI',
        kind: 'spec',
        status: 'draft',
        updatedAt: isoMinusDays(0),
        summary: 'Wails + shadcn/ui; left rail + intent tree.',
        children: [
            {
                id: 'i-gui-wails',
                title: 'Pick Wails over Tauri',
                kind: 'adr',
                status: 'draft',
                updatedAt: isoMinusDays(0),
                summary: 'Single Go codebase, simpler build.',
                children: [],
            },
            {
                id: 'i-gui-shadcn',
                title: 'Adopt shadcn/ui for components',
                kind: 'note',
                status: 'draft',
                updatedAt: isoMinusDays(0),
                children: [],
            },
        ],
    },
];

export interface MockProject {
    id: string;
    name: string;
    path: string;
    tree: IntentNode[];
    stats: TreeStats;
    lint: {errors: number; warnings: number};
}

const demoNow = new Date().toISOString();

export const mockProjects: MockProject[] = [
    {
        id: 'kron-core',
        name: 'Kron Core',
        path: 'C:\\Demo\\Kron',
        tree: mockIntentTree,
        stats: mockStats,
        lint: {errors: 2, warnings: 0},
    },
    {
        id: 'web-dashboard',
        name: 'Web Dashboard',
        path: 'C:\\Demo\\WebDashboard',
        tree: [
            {
                id: 'demo-frontend',
                title: 'Frontend',
                kind: 'spec',
                status: 'accepted',
                updatedAt: demoNow,
                children: [
                    {
                        id: 'demo-nav',
                        title: 'Dashboard navigation',
                        kind: 'adr',
                        status: 'accepted',
                        updatedAt: demoNow,
                        summary: 'Keep the primary actions visible on smaller screens.',
                        children: [],
                    },
                    {
                        id: 'demo-accessibility',
                        title: 'Accessibility review',
                        kind: 'note',
                        status: 'review',
                        updatedAt: demoNow,
                        summary: 'Keyboard navigation and contrast checks are in review.',
                        children: [],
                    },
                    {
                        id: 'demo-metrics',
                        title: 'Usage metrics panel',
                        kind: 'rfc',
                        status: 'draft',
                        updatedAt: demoNow,
                        summary: 'Draft scope for the first release.',
                        children: [],
                    },
                ],
            },
        ],
        stats: {intents: 3, groups: 1, stale: 0, verified: 1, draft: 1, review: 1, archived: 0},
        lint: {errors: 1, warnings: 2},
    },
    {
        id: 'empty-sandbox',
        name: 'Empty Sandbox',
        path: 'C:\\Demo\\EmptySandbox',
        tree: [],
        stats: {intents: 0, groups: 0, stale: 0, verified: 0, draft: 0, review: 0, archived: 0},
        lint: {errors: 0, warnings: 0},
    },
];
