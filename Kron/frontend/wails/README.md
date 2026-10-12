# frontend/wails/ — Kron desktop client (Wails GUI)

Wails v2 GUI client for the [Kron](https://example.invalid/kron) intent ledger. This is the **GUI** half of the
two-client strategy described in [`docs/rfc/2026-10-09-gui-ide-plan.md`](../../docs/rfc/2026-10-09-gui-ide-plan.md)
(the other half is the VSCode extension under `frontend/vscode/`).

## What this client does

Per RFC §3.1, the Wails GUI is a **read-mostly overview** tool:

| Capability | Provided | Notes |
|---|---|---|
| Multi-project overview | yes | cross-project Kron intent aggregates |
| Intent tree preview | yes | rendered via `kron serve-mcp` `kron_tree` |
| Status bar | yes | "last lint: N errors / M warnings" |
| Hover / go-to-def | **no** | IDE responsibility |
| Diagnostics / completion | **no** | IDE responsibility |
| Edit intent files | **no** | GUI jumps to `code <file>`; no embedded editor |
| Relation graph | **no** | IDE responsibility (see §4.3) |

## Architecture (process boundaries)

```
┌────────────────────┐
│  React 18 + Vite 5 │   ← this directory's frontend/ subdir
│  (TypeScript)      │
└─────────┬──────────┘
          │ Wails bridge (in-process Go ↔ TS, generated bindings)
┌─────────▼──────────┐
│  Go (Wails runtime)│   ← this directory's root (app.go, main.go)
│  small bridge only │
└─────────┬──────────┘
          │ stdio JSON-RPC (spawned subprocess)
┌─────────▼──────────┐
│  kron serve-mcp    │   ← Kron Go mainline, cmd/kron/serve-mcp/
└────────────────────┘
```

The GUI **never** imports `internal/*` directly (iron rule #9 + #10 in the RFC). All Kron data flows
through `kron serve-mcp` over stdio JSON-RPC.

## Work status

This is the **work 1.3 skeleton** — the Wails build pipeline is wired up, the Go ↔ TS bridge is verified to
return static metadata, and the React overview page renders. Later work items add:

- `wails dev` live window verification (work 1.4)
- `wails build` cross-platform artefact (work 1.5)
- shadcn/ui + real `kron serve-mcp` plumbing (2.x)

## Running

```bash
# from this directory (frontend/wails/)
wails dev          # live development with HMR + Go bridge
wails build        # production cross-platform binary
```

`wails dev` requires the Wails CLI (Go 1.20+, `go install github.com/wailsapp/wails/v2/cmd/wails@latest`).

## Layout

```
frontend/wails/
├── app.go            # Go bridge surface (KronInfo, ListMCPTools)
├── main.go           # Wails entry point (window options, embed)
├── wails.json        # Wails project config
├── go.mod / go.sum   # Wails Go dependencies
├── build/            # platform build assets (icons, plist, NSIS)
└── frontend/         # React 18 + Vite 5 + TS frontend
    ├── index.html
    ├── package.json
    ├── tsconfig.json
    ├── vite.config.ts
    ├── wailsjs/      # generated Go ↔ TS bindings (do not edit)
    └── src/
        ├── main.tsx
        ├── App.tsx
        ├── App.css
        └── style.css
```

## See also

- [`docs/rfc/2026-10-09-gui-ide-plan.md`](../../docs/rfc/2026-10-09-gui-ide-plan.md) — full RFC
- [`docs/rfc/2026-10-10-frontend-ui-stack.md`](../../docs/rfc/2026-10-10-frontend-ui-stack.md) — UI library decision (pending)
- [`docs/process/internal-pkg.md`](../../docs/process/internal-pkg.md) — `internal/*` immutability doctrine
