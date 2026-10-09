# frontend/ — Kron 客户端层（2026-10-09 修订：客户端层进主仓）

> 2026-10-09 修订：客户端层代码**进** Kron 主仓 `frontend/`（不再独立建仓）。
> 各客户端在 `frontend/` 下各自子目录，不互相 import。
>
> 架构依据：[`docs/abstractDesign/architecture.md`](../docs/abstractDesign/architecture.md)
> §〇 铁律 #9 + §〇·五·1（2026-10-09 修订版）。

## 目录结构

```
frontend/
├── README.md         (本文件)
├── wails/            # Wails GUI (整体预览)
│   └── ...           # wails init + React + Vite + shadcn/ui
└── vscode/           # VSCode 扩展 (功能最全)
    └── ...           # yo code + React + shadcn/ui + vscode-languageclient
```

## 各客户端职责

按 [`docs/rfc/2026-10-09-gui-ide-plan.md`](../docs/rfc/2026-10-09-gui-ide-plan.md) §3/§4：

| 客户端 | 职责 | 调用方式 |
|---|---|---|
| **Wails** (`frontend/wails/`) | 整体预览 (多项目概览 + 意图树预览 + 状态条)；**不**做 hover/def/补全/诊断/关系图 | 进程外 stdio `kron serve-mcp` 拼 JSON (list/get/read 类只读工具) |
| **VSCode 扩展** (`frontend/vscode/`) | 功能最全 (hover/def/侧边栏/详情面板/弹窗/关系图)；编辑器生命周期内所有 kron 能力全聚合 | 进程外 stdio `kron serve-mcp` (业务) + `kron serve-lsp` (编辑器 UI 集成) |

## 关键边界

- **`frontend/wails/` 和 `frontend/vscode/` 不互相 import** — 铁律 #9 约束
- **`frontend/wails/` 不直接 import `internal/`** — 只走 `kron serve-mcp` stdio
- **`frontend/vscode/` 不直接 import `internal/`** — 只走 `kron serve-mcp` + `kron serve-lsp`
- **图表**: VSCode webview 内走 shadcn 原生 `<Chart>` (v0.14+) 或 spike 验证后按需扩展

## `.gitignore` 已覆盖

```
frontend/node_modules/
frontend/dist/
frontend/.vite/
frontend/vscode/out/
frontend/vscode/node_modules/
frontend/wails/node_modules/
frontend/wails/dist/
frontend/wails/.vite/
```

构建产物（node_modules / dist / .vite / VSCode out/）已全部 .gitignore，不会污染 git。

## v1.3 启动时做什么

按 [`docs/rfc/2026-10-09-gui-ide-plan.md`](../docs/rfc/2026-10-09-gui-ide-plan.md) §9：

1. `frontend/wails/` — `wails init` + React + Vite + shadcn/ui；spike 验证调 `kron serve-mcp` list 工具
2. `frontend/vscode/` — `yo code` + React + shadcn/ui + `vscode-languageclient`；spike 验证侧边栏能调 `kron serve-mcp`
3. shadcn 图表组件够不够用 → spike 验证后拍

## 不在 `frontend/` 根目录放代码

各客户端代码**不**放 `frontend/` 根目录，全在 `frontend/wails/` / `frontend/vscode/` 子目录。
