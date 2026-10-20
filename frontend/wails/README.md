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
subcommand). To run the live GUI you therefore need a separate **CLI binary**:

```bash
# from the repo root
go build -o frontend/wails/build/bin/kron-cli.exe ./cmd/kron
```

The bridge searches, in order, for `kron-cli.exe` / `kron-dev.exe` / `kron` / `kron.exe` in:

1. `$KRON_WAILS_SERVE_MCP_EXE` (absolute path override),
2. `<cwd>/build/bin` and `<cwd>`,
3. the GUI executable's own directory and `<exeDir>/build/bin` — this is what makes a
   **double-clicked** GUI work (Windows gives it cwd = exe dir),
4. `PATH`.

Every candidate is validated with a 2-second `serve-mcp` probe, so a GUI binary named `kron.exe`
is skipped rather than deadlocking. `kron-cli.exe` is preferred by name because `wails build`
produces a GUI `kron.exe` too. The resolved path is cached for the life of the app (a failed
resolution is retried later, so you can build the CLI and hit Retry).

Child processes are launched with `CREATE_NO_WINDOW` + `HideWindow`: a double-clicked GUI has no
console, and without those flags Windows would open a console window per child (visible as
"several windows pop up when switching to live"). The long-lived `serve-mcp` child writes its
stderr to `serve-mcp.log` next to the GUI executable — that file is the first place to look when a
handshake fails.

The CLI binary only needs to be rebuilt when you change the `cmd/kron/serve-mcp/` source tree;
`wails dev` does not pick up `cmd/kron/` changes automatically (it watches `frontend/wails/`).

### Live mode: registry + test project

The GUI starts in **mock** mode. Switch to **live** in Settings → Data source. Live mode needs a
`projects.json` registry **next to the GUI binary** (`build/bin/projects.json`;
`$KRON_PROJECTS_JSON_PATH` overrides). Schema is the one `kron register` writes; a UTF-8 BOM is
tolerated (Windows editors love adding one).

`kron-mcp-test/project` in this repository is the fixture used during merge preparation: two
intent groups (`auth` / `payments`), real source anchors, and one deliberately dangling anchor
(`src/broken/orphan.go`) so lint reports `1 error`.

```powershell
# point the GUI-adjacent registry at the fixture
Copy-Item kron-mcp-test/install/projects.json frontend/wails/build/bin/projects.json

# or register it through the CLI (writes next to kron-cli.exe, same directory)
cd kron-mcp-test/project
..\..\frontend\wails\build\bin\kron-cli.exe register
```

### Bridge smoke test (no clicking)

`bridge_e2e_test.go` drives the same `App` methods the React side calls — registry read, MCP
handshake, `kron_tree` / `kron_get` / `kron_lint`, project switch, and the OpenInIDE failure path —
against a real `kron serve-mcp` subprocess:

```powershell
$env:KRON_PROJECTS_JSON_PATH = "E:\works\Kron\frontend\wails\build\bin\projects.json"
go test -tags "desktop production e2e" -run TestAppBridgeE2E -v .
```

It is behind the `e2e` build tag, so `go vet ./...` and `go test ./...` never touch it.

> Note: mainline `internal/store.ReadProjectsJSON` (CLI / MCP side) does **not** strip a UTF-8 BOM,
> unlike the GUI bridge. A registry written by PowerShell 5.1 `Set-Content -Encoding UTF8` is
> readable by the GUI but breaks `kron register` / `kron serve-mcp`.

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
