// Kron GUI — shared domain types for the intent tree / stats / bridge info.
// The GUI is read-only and gets its data from `kron serve-mcp`; these types are
// the projection used by the canvas, the left rail and the info card.

export type IntentKind = 'adr' | 'rfc' | 'spec' | 'note';
export type IntentStatus = 'draft' | 'review' | 'accepted' | 'stale';

/** Per-intent assumption reference (B-3). Shared text lives in .kron/assumptions/. */
export interface IntentAssumption {
    id: string;
    severity: 'hard' | 'soft';
    rationale?: string;
    expiresAt?: string;
    verifiedAt?: string;
    verifiedBy?: string;
}

export interface IntentNode {
    id: string;
    title: string;
    kind: IntentKind;
    status: IntentStatus;
    updatedAt: string;
    summary?: string;
    // Absolute on-disk path of the .md file (from kron_get.source_path).
    // Absent for synthetic group nodes.
    path?: string;
    // Frontmatter projection shown in the canvas info card.
    symbols?: string[];
    reviewers?: string[];
    assumptions?: IntentAssumption[];
    references?: string[];
    dependsOn?: string[];
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
