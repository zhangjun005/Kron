# RFC: 客户端层方案 — Wails GUI（整体预览）+ VSCode 扩展（功能最全）

| 字段 | 值 |
|---|---|
| **状态** | **草案 (2026-10-09, zhangjun005 拍板启动)** |
| **作者** | AI assistant, 经 zhangjun005 委托 |
| **创建日期** | 2026-10-09 |
| **目标版本** | v1.3+ |
| **取代** | [`docs/rfc/archive/2026-10-07-gui-stack.md`](./archive/2026-10-07-gui-stack.md)（SUPERSEDED 2026-10-08, 旧版 Wails 主 + VSCode 副 双路径拍板**沿用**, GUI 边界**收紧**）|

---

## 1 动机 (2026-10-09)

[`docs/rfc/archive/2026-10-07-gui-stack.md`](./archive/2026-10-07-gui-stack.md) 锁定了 Wails v2 主路径 + VSCode 扩展副路径, 但**未**拍板:
- GUI (Wails) 实际**做什么** / **不**做什么（边界模糊）
- VSCode 扩展的功能范围 / UI 分工 / 与 LSP/MCP 的边界
- 图表/关系图渲染库选型（visx 候选）
- 客户端层与协议访问层（CLI / MCP / LSP）的**实际**通讯模式

[`docs/abstractDesign/architecture.md`](../abstractDesign/architecture.md) §〇 铁律 #9 + §〇·五·1 已在架构层把客户端层**枚举**为"VSCode 扩展 / Wails GUI / Cursor / 其他 IDE / Web GUI", 但**职责**和**实现路径**本 RFC 才拍.

### 1.1 拍板背景 (2026-10-09, zhangjun005)

按 `AGENTS.md` §7.1 拍板: GUI = **整体预览** (类 PM 软件), IDE = **功能最全** (LSP+侧边栏+窗口). 本 RFC 把这条**展开**到具体组件分工.

### 1.2 与旧 RFC 的关系

| 旧 RFC 拍板 | 状态 |
|---|---|
| Wails v2 (主) + VSCode 扩展 (副) | ✅ 沿用 |
| 客户端层**不**进 Kron Go 主仓 | ✅ 沿用 (architecture §〇 铁律 #9) |
| GUI 边界"模糊" / VSCode 范围"待定" | ❌ **本 RFC §3 + §4 拍** |

---

## 2 整体架构 (一图)

```
┌─────────────────────────────────────────────────────────────┐
│ 客户端层 (client layer) — 不进 Kron Go 主仓                │
│                                                              │
│  ┌─[ GUI: Wails v2 ]──────────────────────────┐            │
│  │  整体预览 (类 PM 软件)                      │            │
│  │  多项目概览 + 意图树预览 + 状态条           │            │
│  │  调: kron serve-mcp (拼 JSON, list/get)     │            │
│  │  ✗ 不做 hover/def/completion/diagnostics     │            │
│  │  ✗ 不做项目管理以外的深度编辑                │            │
│  └────────────────────────────────────────────┘            │
│                                                              │
│  ┌─[ IDE: VSCode 扩展 .vsix ]─────────────────┐            │
│  │  功能最全                                   │            │
│  │  hover/def (LSP) + 侧边栏 (TreeView)        │            │
│  │  + 详情面板 (Webview) + 弹窗 (Webview)      │            │
│  │  + 跳转/补全/diagnostics (LSP)              │            │
│  │  + 关系图渲染 (visx 候选, webview 内)       │            │
│  │  调: kron serve-mcp (业务) + serve-lsp (UI) │            │
│  └────────────────────────────────────────────┘            │
└──────────────┬─────────────────────┬────────────────────────┘
               │ stdio JSON-RPC     │ stdio JSON-RPC
               │ (Content-Length)   │ (LSP frame)
       ┌───────▼──────┐      ┌──────▼───────┐
       │  cmd/kron/   │      │  cmd/kron/   │
       │  serve-mcp/  │      │  serve-lsp/  │  ← 3 协议访问层
       └──────┬───────┘      └──────┬───────┘     (Kron Go 主仓)
              │                     │
              └──────────┬──────────┘
                         ▼
              ┌──────────────────────┐
              │ internal/ (8 包)     │
              │ model / store /      │
              │ parser / relations /  │
              │ lint / view /        │
              │ assumption / identity│
              └──────────────────────┘
                         ▲
                         │
              ┌──────────┴───────┐
              │  cmd/kron/cli/   │  ← 第 3 协议访问层
              └──────────────────┘
```

**关键边界**（铁律 #9 强约束）:

- 客户端层**不**直接 import `internal/`
- 客户端层**不**互相 import（VSCode 扩展**不** import Wails, 反之亦然）
- 3 协议访问层 (CLI / MCP / LSP) **互不**调用（铁律 #2）

---

## 3 GUI 端 — Wails v2 (整体预览 only)

> 完整拍板见 [`docs/rfc/archive/2026-10-07-gui-stack.md` §2](../docs/rfc/archive/2026-10-07-gui-stack.md)（Wails v2 + Go ↔ TS 桥 + 单二进制产物，本 RFC 沿用）。

### 3.1 职责范围

| 部件 | 状态 | 备注 |
|---|---|---|
| 多项目概览 | ✅ | "我有 3 个 repo 用了 Kron, 意图总数 42, 假设 8" |
| 意图树预览 | ✅ | 按 `internal/view.BuildIntentTree` 渲染 |
| 状态条 / 概览 | ✅ | "最后 lint: 2 errors / 0 warnings" |
| Hover 悬浮 | ❌ | IDE 职责 |
| Definition 跳转 | ❌ | IDE 职责 |
| 弹窗 / 补全 / 诊断 | ❌ | IDE 职责 |
| 编辑意图文件 | ⚠️ 半支持 | 调 `code <file>` 跳到 IDE 编辑, **不**自带编辑器 |
| 关系图 (visx 等) | ❌ | IDE 职责 (见 §4.4) |

### 3.2 调的能力 (后端)

**唯一**: `kron serve-mcp` 子进程, 拼 JSON 调用 `kron_list` / `kron_get` / `kron_impact` / `kron_density` 等**只读类**工具.

**不**调 `kron serve-lsp`（hover/def 是 IDE 职责, GUI 用不到）.

### 3.3 仓库位置

**独立仓** `github.com/xxx/kron-wails/` (或类似), **不**进 Kron Go 主仓. 进程内 Go ↔ TS 桥 (Wails 自身) + 进程外 Go ↔ Kron 主仓 (stdio 子进程).

---

## 4 IDE 端 — VSCode 扩展 (功能最全)

### 4.1 形态

| 项 | 选 | 备注 |
|---|---|---|
| 分发 | `.vsix` (单包, 多 contribution) | 见 §4.2 |
| 主仓位置 | 独立仓 `github.com/xxx/kron-ide/` (或类似) | **不**进 Kron Go 主仓 |
| 跨编辑器 | Cursor 复用 (内核 = VSCode 1.85+) | Neovim / Helix 各**自**写 |
| 宿主 API | VSCode Extension API (TS) | **不**用 wrapper (如 yo code) |
| LSP 客户端 | `vscode-languageclient` v9+ (TS) | 事实标准, **不**手写 LSP 协议 |
| MCP 客户端 | `@modelcontextprotocol/sdk` (TS) | 官方, **不**手写 JSON-RPC |

### 4.2 1 个 .vsix 装下全部 UI 部件

VSCode 扩展的 `package.json` `contributes` 字段**同时**声明 LSP / 侧边栏 / 窗口, 3 部件共享 1 个 extension entry:

```jsonc
// extensions/vscode-kron/package.json (示意, 实施时按 RFC + 拍板)
{
  "name": "kron-ide",
  "engines": { "vscode": "^1.85.0" },
  "main": "./out/extension.js",
  "activationEvents": [
    "workspaceContains:.kron/intents"
  ],
  "contributes": {
    "configuration": { "title": "Kron", "properties": { ... } },
    "viewsContainers": { "activitybar": [{ "id": "kron-sidebar", ... }] },
    "views": {
      "kron-sidebar": [
        { "type": "tree",   "id": "kronIntentTree",   "name": "Intent Tree" },
        { "type": "webview","id": "kronIntentDetail", "name": "Intent Detail" }
      ]
    },
    "commands": [
      { "command": "kron.openIntent",   "title": "Open Intent" },
      { "command": "kron.refreshTree",  "title": "Refresh Intent Tree" },
      { "command": "kron.showImpact",   "title": "Show Impact (Graph)" },
      { "command": "kron.assumeCheck",  "title": "Run Assume Check" }
    ]
  }
}
```

### 4.3 UI 部件分工 (原生 vs Webview)

| 部件 | 选 | 理由 |
|---|---|---|
| Activity Bar 图标 | VSCode 原生 | 必须, 不复杂 |
| 意图树 (侧边栏) | **VSCode TreeView** | 跟 VSCode 文件树同风格, 用户熟悉 |
| 状态条 (底部) | **VSCode StatusBarItem** | 必须, 简单 |
| Editor title 按钮 | **VSCode 原生 `editor/title` menu** | 跳转类操作 |
| Hover 悬浮 | **LSP (不走 webview)** | LSP 协议原生 |
| Diagnostics 红线 | **LSP `publishDiagnostics`** | LSP 协议原生 |
| Completion 补全 | **LSP** | LSP 协议原生 |
| Definition 跳转 | **LSP** | LSP 协议原生 |
| **意图详情面板** (侧边栏 webview) | **Webview + shadcn/ui** | "独立面板", 适合 shadcn Card/Tabs/Badge |
| **假设警告弹窗** | **Webview + shadcn Dialog** | 复杂交互, shadcn 强项 |
| **关系图 / 依赖图** (visx 候选) | **Webview + visx** | 关系图是"独立可视化", 适合 visx; 见 §4.4 |

**规律**:

- **想"跟 VSCode 紧耦合"** → VSCode 原生 API
- **想"独立面板 + 复杂交互"** → Webview + shadcn/ui / visx

### 4.4 关系图渲染 (visx 候选)

**候选**: [visx](https://airbnb.io/visx) — Airbnb 出品的 low-level React 可视化库, "d3 + React, the modular way".

**适配场景**（Kron 内部已有 / 将有的关系图）:

| 关系图 | 数据来源 | 渲染场景 |
|---|---|---|
| `kron_impact` 反向依赖图 | `internal/relations.ReverseLinks` | 改某 intent 前看"谁依赖我" |
| `kron_density` 假设密度 | `internal/relations` (B-3) | 项目"假设膨胀"健康度 |
| 假设 → 意图 引用图 | `internal/assumption.Reader.List + Intent.Frontmatter.Assumptions` | 假设漂移追踪 (B-3) |
| Frontmatter 字段引用图 | `internal/store` | 数据 lineage (v1.4+ 候选) |

**为什么 visx**:

- ✅ **Modular** — 按需 import `@visx/network` / `@visx/tree` / `@visx/scale` / `@visx/shape`, bundle 体积可控
- ✅ **React 友好** — 与 shadcn/ui / React 18 同生态
- ✅ **TypeScript 原生** — 强类型
- ✅ **低层级** — 不强制图表样式, Kron 自己设计"意图图"视觉
- ✅ **VSCode webview 适配** — 纯 React, 在 webview iframe 内跑通 (CSP 友好, 走 Vite production build)

**visx 选型论证待补**（按 `AGENTS.md` §3 "新 top-level dep 走 explicit approval"）:

| 维度 | 待写 | 何时 |
|---|---|---|
| 竞品对比 (visx vs Recharts vs d3 + React vs ECharts vs Nivo) | TODO | 实施前补 |
| 包大小 (按需剪枝后) | TODO | spike 验证后填 |
| 维护活跃度 | TODO | 实施前查最新 |
| API 稳定性 | TODO | 实施前查 |

**不**在本 RFC 拍 visx 选型 — 本 RFC 只**记录**"visx 是候选" + "关系图用 webview + 任意 React 可视化库". 具体 dep 锁定走 v1.3 启动时的子 RFC.

### 4.5 调的能力 (后端)

| 后端 | 用法 | 触发 |
|---|---|---|
| `kron serve-mcp` (子进程) | 拼 JSON 调 12+ 工具 | 侧边栏 / 详情面板 / 弹窗 / 命令面板 |
| `kron serve-lsp` (子进程) | hover / definition / completion / diagnostics | 编辑器自动 (LSP client 接管) |

**2 个子进程同生同死**: VSCode 扩展 activate 时 spawn, deactivate 时 kill. **不**互相通讯 (architecture §〇 铁律 #2).

---

## 5 客户端 SDK 与 dep 选型约束

| 客户端 | 选型 | dep 走 explicit approval |
|---|---|---|
| Wails GUI | Wails v2 + Go ↔ TS 桥 (TS 端 React + shadcn/ui + **不**用 visx) | 旧 RFC 拍, **不**重复论证 |
| VSCode 扩展 | `vscode-languageclient` + `@modelcontextprotocol/sdk` + React 18 + Vite 5 + shadcn/ui + **visx (关系图, 待选型论证)** | 4 段论证在 v1.3 启动时补 |
| LSP server (Kron Go 主仓) | `go.lsp.dev/protocol v3.17+` ([`docs/rfc/2026-10-07-lsp-sdk.md` §3](./2026-10-07-lsp-sdk.md) 已拍) | 旧 RFC 拍, **不**重复 |

**AGENTS.md §3 红线重申**: 任何**新** top-level dep 进 Kron Go 主仓 (`go.mod`) 走 explicit approval, VSCode 扩展**不**进 Go 主仓, 其 dep 走 npm + vsce 流程, **不**走 Go explicit approval 路径.

---

## 6 实施位置 (再次强调, 客户端层**不**进主仓)

| 客户端 | 仓库 | 与 Kron Go 主仓关系 |
|---|---|---|
| Wails GUI | 独立仓 (e.g. `github.com/xxx/kron-wails/`) | **不**进主仓; 进程内 Wails 桥 + 进程外 stdio `kron serve-mcp` |
| VSCode 扩展 | 独立仓 (e.g. `github.com/xxx/kron-ide/`) | **不**进主仓; stdio `kron serve-mcp` + `kron serve-lsp` |
| Cursor 客户端 | 复用 VSCode 扩展 | 直接装同一个 `.vsix` |
| Neovim / Helix | 各编辑器生态**自**写 | **不**下沉 SDK |

Kron Go 主仓**不**建 `extensions/` 目录, **不**建 `kron-wails/` 目录. `frontend/` 当前是 README 占位 (commit `5864c1a`).

---

## 7 关键决策 (本 RFC 拍板后**不可**回退的)

1. **GUI = 整体预览 only**: Wails 端**不**做 hover / def / 补全 / 诊断 / 关系图 (那是 IDE 职责). GUI 调 serve-mcp 走 list/get 类只读工具.
2. **IDE = 功能最全**: VSCode 扩展装 1 个 .vsix 同时承担 LSP 客户端 + 侧边栏 + 窗口 + 弹窗 + 关系图. IDE 调 serve-mcp 业务 + serve-lsp UI.
3. **关系图渲染 (visx 候选)**: 在 VSCode 扩展 webview 内, **不**在 Wails GUI. visx dep 锁定走 v1.3 启动时的子 RFC + 4 段论证.
4. **客户端层不进主仓**: Wails / VSCode 扩展 / Cursor / Neovim / Helix 全部走独立仓或编辑器生态, Kron Go 主仓**不**存.
5. **shadcn/ui 范围**: Webview (Card / Tabs / Badge / Dialog) + 详情面板 + 弹窗. VSCode 原生 UI 部件 (TreeView / StatusBar / Activity Bar / Editor title menu) **不**用 shadcn.

---

## 8 反对意见 (AI 已表达 / 已驳回)

| 反对 | 驳回理由 |
|---|---|
| "GUI 也应该做关系图 (visx)" | 拒绝. 关系图需要"快速定位 + 跳转", GUI 是"概览", **不**做"跳转". 跳转 = IDE 职责 (LSP definition). |
| "VSCode 扩展和 Wails GUI 应该共享 TS 组件" | 拒绝. 客户端层**不**互相 import (architecture §0 铁律 #9). 各自包, 各自重写 (shadcn 按需 copy **可**接受, 但**不**共享包). |
| "visx 选型应该现在就拍" | 拒绝. 本 RFC 只**记录**"visx 是候选". dep 锁定需要 4 段论证 (竞品 / 包大小 / 维护 / API), **不**在客户端层方案 RFC 里拍. 走子 RFC. |
| "GUI 应该用 LSP 提供 hover (像文档型 GUI)" | 拒绝. GUI 职责 = 整体预览, hover 是 IDE 职责. 越界 = 违反 `AGENTS.md` §7.1 拍板的"GUI 不做深度功能". |
| "LSP 客户端应该手写 JSON-RPC, 不引依赖" | 拒绝. `vscode-languageclient` 是事实标准, 手写 = 重复造轮子 + 协议兼容风险. |

---

## 9 实施路径 (拍板后**不**立刻开工, 按 process 走)

按 [`docs/process/new-access-layer.md`](../docs/process/new-access-layer.md) §3 7 个交付物. 客户端层**不**进 Go 主仓, 所以 §3 中"骨架放 `cmd/kron/serve-<layer>/`"这步**不适用**; 改为"独立仓 + 拍板 SDK 选型 + 写 RFC".

### 9.1 Wails GUI

- [ ] 独立仓 `kron-wails/`
- [ ] v1.3 启动时开子 RFC: Wails SDK 版本锁定 + shadcn/ui 范围
- [ ] spike 验证: Wails 起窗口 + 调 `kron serve-mcp` list 工具

### 9.2 VSCode 扩展

- [ ] 独立仓 `kron-ide/`
- [ ] v1.3 启动时开子 RFC: `vscode-languageclient` 版本 + MCP SDK 版本 + **visx 选型 4 段论证** + shadcn/ui 范围
- [ ] spike 验证: 最小 `.vsix` 装在 VSCode F5 host, 侧边栏能调 `kron serve-mcp` list 工具

### 9.3 不做的事

- ❌ **不**把 Wails / VSCode 扩展代码进 Kron Go 主仓
- ❌ **不**在 `frontend/` 加任何 Wails / shadcn / visx 实施代码 (本目录是 README 占位)
- ❌ **不**写跨客户端 SDK 仓 (各客户端各自包, 各自打包)
- ❌ **不**拍 visx 选型 (留子 RFC)

---

## 10 与旧 RFC 的关系

| 旧 RFC `2026-10-07-gui-stack.md` 章节 | 状态 | 说明 |
|---|---|---|
| §1 动机 / §1.1 双路径 | ✅ 沿用 (Wails 主 + VSCode 副) | 但**当前**拍板是 Wails overview-only (本 RFC §3) |
| §1.2 与 architecture.md 的关系 | ✅ 沿用 | |
| §1.3 4 件事 | ⚠️ 重新拆分 | 本 RFC §3 + §4 分别拍 GUI / IDE |
| §2 候选评估 — Wails | ✅ 沿用 (Wails v2 锁定) | |
| §3 候选评估 — VSCode | ✅ 沿用 (VSCode 扩展 锁定) | |
| §4 Wails ↔ Go 桥 vs VSCode 扩展 ↔ Go 通信 | ⚠️ 收紧 | 旧版"双路径"松散; 本 RFC §3.2 / §4.5 明确 "GUI 只调 serve-mcp / IDE 调 serve-mcp + serve-lsp" |
| §5 路径实施先后 (v1.3 / v1.4+) | ⚠️ 重新拍 | 本 RFC 不**排**先后, 按"独立仓 + 子 RFC"走 |
| §6-§9 实施细节 | ❌ **作废** | 走 v1.3 子 RFC 重写 |
| §10 顺修表 (architecture / mcp.md 行号) | ❌ **作废** | architecture 已 2026-10-08 改完, mcp.md 已 2026-10-09 归档 |

> **状态修订**: 本 RFC 拍板后, 旧 RFC `2026-10-07-gui-stack.md` 头部 `状态` 字段**保留** SUPERSEDED (旧 RFC **整体**被本文档 §7 拍板后**部分**沿用 / 部分**重新**拍).
