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

This is the **work 1.3 / 1.4.b / 1.4.c skeleton** — the Wails build pipeline is wired up, the Go ↔ TS
bridge exposes a live `kron serve-mcp` subprocess (1.4.b), and the React overview page renders the
real intent tree on the `kron_tree` + `kron_get` MCP responses (1.4.c). Later work items add:

- `wails build` cross-platform artefact (work 1.5)
- shadcn/ui full coverage + the rest of the kron_* tools (2.x)

## Running

```bash
# from this directory (frontend/wails/)
wails dev          # live development with HMR + Go bridge
wails build        # production cross-platform binary
```

`wails dev` requires the Wails CLI (Go 1.20+, `go install github.com/wailsapp/wails/v2/cmd/wails@latest`).

### Running without the Wails CLI (manual `go build`)

`wails` is only a convenience wrapper, but a manual `go build` **must** pass the Wails build tags.
Without them the binary compiles fine, then at startup shows:

> Wails applications will not build without the correct build tags.

That message comes from a runtime fallback file in the Wails module
(`internal/app/app_default_windows.go`, build tag `!dev && !production && !bindings && windows`),
so **`go build ./...` and `go vet ./...` passing proves nothing about this** — only launching the
binary does.

```bash
# 1) frontend first — main.go embeds frontend/dist
cd frontend/wails/frontend && npm ci && npm run build && cd ..

# 2) GUI binary (tags desktop + production; -H windowsgui hides the console window)
go build -tags "desktop production" -ldflags "-w -s -H windowsgui" \
  -o build/bin/kron-gui.exe .

# 3) separate CLI binary for live mode (see next section)
cd ../.. && go build -o frontend/wails/build/bin/kron-cli.exe ./cmd/kron
```

Start it with cwd = `frontend/wails/` so the executable probe finds `build/bin/kron-cli.exe`:

```bash
cd frontend/wails && ./build/bin/kron-gui.exe
```

Do **not** use `go run .` — the temporary binary makes the exe-adjacent `projects.json` lookup fail,
so live mode can never find the registry.

### Building the `kron serve-mcp` binary (1.4.b / 1.4.c dev workflow)

The Wails GUI's Go bridge spawns `kron serve-mcp` as a stdio JSON-RPC subprocess. Wails' default
build target is the **Wails GUI** itself (`build/bin/kron.exe`), which is not a CLI binary and
will hang when invoked with `serve-mcp` (it spins up WebView2 instead of processing the
subcommand). To run the live GUI in development you therefore need a separate **CLI binary** at
one of the well-known paths the bridge probes (in order: `kron.exe`, `kron-dev.exe`,
`kron-cli.exe`):

```bash
# from the repo root
go build -o frontend/wails/build/bin/kron-cli.exe ./cmd/kron
```

Then `wails dev` picks up `kron-cli.exe` automatically. Alternatively, set
`$KRON_WAILS_SERVE_MCP_EXE` to an absolute path of any `kron` CLI build to skip the probe.

The CLI binary only needs to be rebuilt when you change the `cmd/kron/serve-mcp/` source tree;
`wails dev` does not pick up `cmd/kron/` changes automatically (it watches `frontend/wails/`).

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
