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
└── webview/                  # Vite 5 + React 18 + shadcn/ui
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
        ├── App.tsx           # kron_tree request, filter, refresh, and tree rendering
        ├── index.css         # Tailwind + shadcn HSL vars
        ├── lib/utils.ts      # cn() helper
        ├── lib/mcp-types.ts  # Webview/extension-host message and tree types
        └── components/ui/    # shadcn 5 first-batch (Button/Card/Input/Badge/Separator)
```

## Status (as of this commit)

PR 1 implements RFC 2026-10-18 §4 #1-#4:

- [x] #1 yo code skeleton (F5 launch + tasks + settings + extensions.json)
- [x] #2 Vite 5 webview scaffold (package.json + index.html + vite.config + tsconfig)
- [x] #3 shadcn 5 first-batch components (Button / Card / Input / Badge / Separator)
- [x] #4 config lock to RFC §2.1/§2.2/§2.3 versions + tsconfig strict + components.json

Implementation status against RFC §4:

- [x] #5 `src/extension.ts` + independent MCP and LSP process clients
- [x] #6 `src/views/intentTree.ts` — native Explorer TreeView backed by `kron_tree`
- [x] #7 `src/views/statusBar.ts` — independent connection status and restart command
- [x] #8 `src/views/intentEditor.ts` — WebviewPanel with CSP and validated tree messages
- [x] #9 `webview/src/App.tsx` tree display, intent details, and mock-only intent/assumption forms + `src/lib/mcp-types.ts` — lanyue811
- [ ] #10 Manual F5 smoke — implementation is available; not yet manually accepted in an Extension Host
- [x] #11 `docs/implementation/vscode-extension.md` — implementation guide and host/Webview contract documented
- [ ] D1-D4 (Activity Bar icon / webview theme sync / VSIX update / multi-project) — v.s. PR, RFC §3.4

## How to verify (step-by-step)

```bash
# 0. Preview the UI without VSCode or a Kron binary
cd frontend/vscode/webview
pnpm dev   # Vite serves the mock tree and detail/forms preview

# 1. Build the webview
cd frontend/vscode/webview
pnpm install --frozen-lockfile   # install the pinned webview dependency graph
pnpm build     # → ../dist/webview/index.html + assets/
# Check output:
ls ../dist/webview/

# 2. F5 smoke test — open frontend/vscode as a VS Code workspace and run "Run Kron Extension".
# The pre-launch task builds the Webview and compiles the extension host.
```

The standalone preview uses sample data. Form changes stay in page memory and are never written to project files. Inside VS Code, the host supplies the live tree; detail reads and write actions are not connected yet, so the Webview identifies those details as unavailable and disables edits.

For current implementation state, lifecycle/CSP/LSP integration constraints, and the recommended next steps, see [the VS Code implementation handoff](../../docs/implementation/vscode-extension.md#当前分支交接状态). The skeleton RFC's engineering supplement is in §2.7 and §4.1.

## How to develop

- Extension host code lives under `src/` (TS, Node 16 module) — compiled to `out/` by `tsc -p ./`
- Webview code lives under `webview/src/` (TS, ESNext) — built by Vite to `dist/webview/`
- The two are independent TypeScript projects with separate `node_modules/` (webview uses Vite + React + shadcn; extension host uses `vscode` + `vscode-languageclient` + `@modelcontextprotocol/sdk`).
- See `docs/rfc/2026-10-18-vscode-skeleton-sdk.md` §2.4 for the full directory + build diagram.

## Quick-create intents from code anchors

When a valid `// @kron:intent <slug>` anchor is completed and left unchanged briefly, the extension checks the current workspace. If that intent does not exist, it offers to create it. Confirming creates it through `kron_add`, refreshes the Intent tree, and lets Kron fill creator/time metadata and derive the title from the final slug segment. Optional metadata is not requested.

## Code navigation

Kron intent slugs in valid `@kron:intent` anchors are underlined. Use **Ctrl+left-click** on an anchor to follow its LSP definition in the editor. F12 remains available as VS Code's standard keyboard shortcut.

## Not in scope

- ❌ No direct import of `internal/` (architecture iron rule #1 + #9)
- ❌ No cross-import with `frontend/wails/` (iron rule #9)
- ❌ No chart library preset (`<Chart>` spike deferred, RFC §3.3)
- ❌ No `textDocument/completion` / `publishDiagnostics` in v1.3 (RFC §3.4)
