# frontend/vscode/ — Kron VSCode extension

Kron intent ledger IDE integration. See [`docs/rfc/2026-10-09-gui-ide-plan.md`](../../docs/rfc/2026-10-09-gui-ide-plan.md) §4 for the IDE-end design and [`docs/rfc/2026-10-18-vscode-skeleton-sdk.md`](../../docs/rfc/2026-10-18-vscode-skeleton-sdk.md) for the SDK version + skeleton layout this commit implements.

## Layout

```
frontend/vscode/
├── package.json              # extension host SDK (vscode-languageclient ^10.0.0, @modelcontextprotocol/sdk ^1.32.0)
├── tsconfig.json             # extension host TS — strict, noUncheckedIndexedAccess, exactOptionalPropertyTypes
├── .vscodeignore             # VSIX packaging rules (excludes src/, webview/src/, node_modules)
├── .vscode/                  # F5 launch config (yo code default)
│   ├── launch.json
│   ├── tasks.json
│   ├── settings.json
│   └── extensions.json
└── webview/                  # Vite 5 + React 18 + shadcn/ui (PR 1 smoke test)
    ├── package.json
    ├── tsconfig.json
    ├── tsconfig.node.json
    ├── vite.config.ts        # base: './', outDir: ../dist/webview
    ├── tailwind.config.js
    ├── postcss.config.js
    ├── components.json       # shadcn/ui config
    ├── index.html
    └── src/
        ├── main.tsx          # React entry, force dark mode
        ├── App.tsx           # 5 shadcn components smoke test (PR 1)
        ├── index.css         # Tailwind + shadcn HSL vars
        ├── lib/utils.ts      # cn() helper
        └── components/ui/    # shadcn 5 first-batch (Button/Card/Input/Badge/Separator)
```

## Status (as of this commit)

PR 1 implements RFC 2026-10-18 §4 #1-#4:

- [x] #1 yo code skeleton (F5 launch + tasks + settings + extensions.json)
- [x] #2 Vite 5 webview scaffold (package.json + index.html + vite.config + tsconfig)
- [x] #3 shadcn 5 first-batch components (Button / Card / Input / Badge / Separator)
- [x] #4 config lock to RFC §2.1/§2.2/§2.3 versions + tsconfig strict + components.json

Out of scope (separate PRs, lanyue811 domain vs. @HomoFluctus domain per RFC §4 table):

- [ ] #5 `src/extension.ts` + `src/mcp/client.ts` + `src/lsp/client.ts` — @HomoFluctus
- [ ] #6 `src/views/intentTree.ts` — @HomoFluctus
- [ ] #7 `src/views/statusBar.ts` — @HomoFluctus
- [ ] #8 `src/views/intentEditor.ts` (WebviewPanel container) — @HomoFluctus
- [ ] #9 `webview/src/App.tsx` real business (kron_tree MCP call) — lanyue811
- [ ] #10 `.vscode-test/` F5 end-to-end smoke — lanyue811
- [ ] #11 `docs/implementation/vscode-extension.md` — lanyue811
- [ ] D1-D4 (Activity Bar icon / webview theme sync / VSIX update / multi-project) — v.s. PR, RFC §3.4

## How to build (PR 1 smoke)

```bash
cd frontend/vscode/webview
pnpm install   # if not already
pnpm build     # → ../dist/webview/index.html + assets/
```

Vite outputs to `frontend/vscode/dist/webview/` (gitignored). Extension host loads the HTML via `fs.readFileSync` at runtime (see RFC 2026-10-18 §2.4 code sample, to be implemented in #8).

## How to develop

- Extension host code lives under `src/` (TS, Node 16 module) — compiled to `out/` by `tsc -p ./`
- Webview code lives under `webview/src/` (TS, ESNext) — built by Vite to `dist/webview/`
- The two are independent TypeScript projects with separate `node_modules/` (webview uses Vite + React + shadcn; extension host uses `vscode` + `vscode-languageclient` + `@modelcontextprotocol/sdk`).
- See `docs/rfc/2026-10-18-vscode-skeleton-sdk.md` §2.4 for the full directory + build diagram.

## Not in scope

- ❌ No direct import of `internal/` (architecture iron rule #1 + #9)
- ❌ No cross-import with `frontend/wails/` (iron rule #9)
- ❌ No chart library preset (`<Chart>` spike deferred, RFC §3.3)
- ❌ No `textDocument/completion` / `publishDiagnostics` in v1.3 (RFC §3.4)
