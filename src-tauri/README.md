# Kron GUI (Tauri 2 + Solid.js)

Phase 1 skeleton — IPC pipeline smoke test only.

## What's here

```
src-tauri/                     Tauri 2 backend (Rust)
├── Cargo.toml                 deps: tauri 2, notify, kron core lib
├── tauri.conf.json            window 1280x800, light theme default
└── src/
    ├── main.rs                entry: calls kron_gui_lib::run()
    ├── lib.rs                 Tauri builder + plugin setup
    ├── ipc_types.rs           ProjectMeta, Greeting (mirrors 05 § 3.2)
    └── commands/
        ├── mod.rs
        ├── project.rs         kron_project_list, kron_greet (smoke test)
        └── watcher.rs         stub — full notify impl in phase 2

frontend/                       Solid.js + Vite frontend
├── package.json               solid-js 1.9, @solidjs/router, kobalte
├── vite.config.ts             Tauri-friendly (port 5173, ignored src-tauri/)
├── tsconfig.json              strict + noUncheckedIndexedAccess
├── tailwind.config.js         maps all CSS vars to Tailwind colors
├── index.html
└── src/
    ├── main.tsx               mount + initTheme
    ├── App.tsx                @solidjs/router setup
    ├── styles/
    │   ├── tokens.css         DOUBLE THEME — see 05c-GUI设计tokens.md
    │   └── global.css         reset + Tailwind base
    ├── stores/theme.ts        light/dark toggle (localStorage + OS)
    ├── ipc/invoke.ts          type-safe invoke('kron_*') wrappers
    ├── types/ipc.ts           hand-mirrored from Rust (ts-rs in phase 2)
    └── routes/
        ├── Home.tsx           V0 项目首页 + IPC 烟测
        ├── Project.tsx        V1 placeholder
        └── Tabs.tsx           V1 tabs placeholder
```

## Run locally

### Prerequisites

- Rust toolchain (stable)
- Node 20+ + npm
- Tauri 2 prerequisites for Windows: see https://v2.tauri.app/start/prerequisites/

### First time setup

```bash
# Frontend deps (from the frontend/ directory)
cd frontend
npm install

# Backend will compile on first `cargo tauri dev`
```

### Dev mode

From the repo root:

```bash
# Make sure frontend/dist/ exists (Tauri needs it for cargo check)
# It's already created with a .gitkeep; Vite overwrites it on first build.

cd src-tauri
cargo tauri dev
```

This will:
1. Run `npm run dev` in `frontend/` (Vite dev server on port 5173)
2. Compile Rust backend
3. Launch Tauri window pointing at the dev server

### Build (production)

```bash
cd frontend
npm run build       # outputs to frontend/dist/

cd ../src-tauri
cargo tauri build   # produces MSI + NSIS installers
```

Outputs MSI + NSIS installers in `src-tauri/target/release/bundle/`.

## Phase 1 smoke test

When the window opens you should see:

1. **"Kron"** title in serif font (思源宋体 fallback chain)
2. **"切换主题"** button — toggles `data-theme` between `light` and `dark` on `<html>`
3. **"IPC 已连通"** card showing:
   - `消息: Hello from Kron backend, Solid frontend!`
   - `后端: kron-gui Tauri 2`
   - `时间戳: <ISO 8601 from Rust>`
4. **"项目 (0)"** empty state (registry not wired yet)

If you see the greeting card, the Rust ↔ Tauri ↔ Solid IPC pipeline is alive.

## Token discipline

All visual constants live in `frontend/src/styles/tokens.css`, mirroring
`dev-docs/design/05c-GUI设计tokens.md` 1:1. Component CSS uses `var(--*)`
exclusively — **no raw colors, no `bg-blue-500`, no `shadow-lg`**.

To add a token:
1. Add it to `05c-GUI设计tokens.md` first (with both light + dark values)
2. Add it to `tokens.css`
3. Use it via `var(--your-token)`

## Next phases

See `dev-docs/design/05b-GUI技术选型.md` § 9 for the backlog:
- Phase 2: full project list + V1 tabs skeleton
- Phase 3: kanban (V2) with HTML5 Drag and Drop
- Phase 4: important files (V3) + conflict wizard (V4)
- Phase 5: V5 vertex DAG (git log --graph text renderer)
- Phase 6: V6 context viewer
