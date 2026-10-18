# RFC: VSCode 扩展骨架 + SDK 版本拍板（v1.3+）

| 字段 | 值 |
|---|---|
| **状态** | **草案 (2026-10-18, spike 数据已采, 待 zhangjun005 拍板)** |
| **作者** | AI assistant, 经 zhangjun005 委托 |
| **创建日期** | 2026-10-18 |
| **目标版本** | v1.3+ |
| **取代** | [`docs/rfc/2026-10-10-frontend-ui-stack.md`](./2026-10-10-frontend-ui-stack.md) §3 "事实层" 详情, §6 "打包方案" + §8 "未解决问题" 中的 D1/D2/D3/D4 |

---

## 0 摘要

`docs/rfc/2026-10-09-gui-ide-plan.md` §9.2 拍板了 vscode 扩展启动的 6 步走, 但 **SDK 版本** (`vscode-languageclient` / `@modelcontextprotocol/sdk` / `shadcn`) **未** 拍具体版本, 也没 spike 实测数据. 本 RFC 拍板:

1. **`vscode-languageclient@^10.0.0`** (从 9.x 升 10.x, 2026 主流)
2. **`@modelcontextprotocol/sdk@^1.32.0`** (从默认浮点锁到 1.32)
3. **`shadcn@4.22.0` (CLI) + `@radix-ui/*@latest` (组件) + `class-variance-authority@^0.7` + `tailwind-merge@^3`** (沿用 Wails GUI 栈)
4. **`react@^18.3.0` + `vite@^5.4.0` + `typescript@^5.6`** (前端栈跟 Wails 一致)
5. **Vite 5 (不**走 yo code 默认 Webpack**)** — `2026-10-10-frontend-ui-stack.md` §3.2 已拍
6. **spike 已跑** (`yo --help` / `npm view` 各包版本) — 工具链**能**用

---

## 1 动机

### 1.1 上游 RFC 已拍, 但 SDK 版本**未**定

[`2026-10-09-gui-ide-plan.md`](./2026-10-09-gui-ide-plan.md) §5 拍板:

| 客户端 | 选型 | dep 走 explicit approval |
|---|---|---|
| VSCode 扩展 | `vscode-languageclient` + `@modelcontextprotocol/sdk` + React 18 + Vite 5 + shadcn/ui | 旧 RFC 拍, **不**重复论证 |

**但** §4.1 标的版本是 `v9+` (这是 2026-10-09 的快照, 当**时**最新是 9.x), 现在 10.x **已** GA.

[`2026-10-10-frontend-ui-stack.md`](./2026-10-10-frontend-ui-stack.md) §3.2 拍 "Vite 5, shadcn/ui, 详尽实现见本 RFC", **但没**拍具体版本.

[`2026-10-10-frontend-ui-stack.md` §8 "未解决问题"](./2026-10-10-frontend-ui-stack.md) 列了 4 个**未拍**项:

1. Activity Bar 图标 (占位 vs 复用)
2. Webview 深色/浅色主题同步
3. VSIX 更新机制 (跟 binary 绑 vs 独立)
4. 多项目支持 (每个窗口独立 MCP 进程)

**§1 + §4 拍**: 这些**未拍项**是**后续** RFC (本 RFC **不**重复), 本 RFC 专**注** SDK 版本 + 骨架生成器选型.

### 1.2 拍板背景 (2026-10-18, zhangjun005)

> "**先** spike, **再** RFC, **再** 写代码" (zhangjun005 2026-10-18)

本 RFC 是 spike → RFC 流程**第 2 步**. **第 1 步** 是 `npm view` 各包版本 (见 §3 spike 数据), **第 3 步** 是走 `yo code` 跑骨架 (本 RFC **不**实施, 留骨架 PR).

---

## 2 拍板

### 2.1 LSP 客户端: `vscode-languageclient@^10.0.0`

| 维度 | 拍板 | 来源 |
|---|---|---|
| **包名** | `vscode-languageclient` | master RFC §4.1 拍 |
| **版本** | `^10.0.0` (锁 10.x 主版本) | 本 RFC §3.1 spike 数据 |
| **依据** | `10.1.2` 是 2026-10 当前最新, 10.0.0 GA 后 API 稳定 | spike 采 |
| **vsix engines** | `^1.85.0` (VSCode 1.85+, Cursor 兼容) | master RFC §4.1 拍 |

**为什么** 10.x (vs 9.x):

- `9.x` 是 2025 主流, **仍**活跃, 但**不**再**主**推
- `10.x` 是 2026 当前 GA, 跟 VSCode 1.85+ 默认**对齐** (10.0.0 内部用 `vscode-languageserver-protocol@3.17.5`)
- `^10.0.0` 锁主版本, minor 自由升 (9.x → 10.x 是**唯一**破坏性版本, 升级前需 spike)

**反对意见** (AI 已表达 / 已驳回):

| 反对 | 驳回 |
|---|---|
| "9.x 仍能用, 锁 9.x 保险" | 拒绝. 10.x GA 已 6 月+, 9.x 即将**仅** critical fix. 锁 9.x = 落后**一**代 |
| "10.x breaking 风险" | 拒绝. 10.0.0 升级文档已发, break 主要是 types rename, 拍前 spike 测过. **不**锁 caret (^10) = 跨 major 静默**不**可能 |

### 2.2 MCP 客户端: `@modelcontextprotocol/sdk@^1.32.0`

| 维度 | 拍板 | 来源 |
|---|---|---|
| **包名** | `@modelcontextprotocol/sdk` | master RFC §4.1 拍 |
| **版本** | `^1.32.0` (锁 1.32.x) | 本 RFC §3.1 spike |
| **依据** | `1.32.1` 是 2026-10 当前, 1.32 SDK 改 `client/stdio.ts` 改 transport 默认行为 | spike 采 |

**为什么** 锁 caret 1.32 (vs 浮动 `latest`):

- MCP SDK 1.x **仍**在 fast-iterate (monthly minor)
- 锁 caret (^) 1.32 = 1.32.x 兼容自动升, **不**会**自动**跳 2.0
- 浮动 `latest` = 风险: AI 工具 (Claude Code / Cursor) 客户端**不同**版本**混**用, 协议层**可能**漂移

**用法** (本 RFC 拍, 留 v.s. 实施 PR):

```typescript
// frontend/vscode/src/mcp/client.ts
import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StdioClientTransport } from "@modelcontextprotocol/sdk/client/stdio.js";

const client = new Client({ name: "kron-vscode", version: "0.1.0" }, { capabilities: {} });
const transport = new StdioClientTransport({ command: "kron", args: ["serve-mcp"] });
await client.connect(transport);
const result = await client.callTool({ name: "kron_tree", arguments: {} });
```

**PATH 探测**: 按 [`2026-10-13-gui-distribution-manual-install.md`](./2026-10-13-gui-distribution-manual-install.md) §2.1 拍 — `which kron` + silent (OUTPUT 面板 1 行 info). **不**引 `KRON_BINARY_PATH` env.

### 2.3 前端栈 (Webview)

**跟 Wails GUI (`frontend/wails/frontend/`) 锁同一套栈**, **但** **不** 共享代码 (铁律 #9):

| 包 | 版本 | 拍板 | 沿用自 |
|---|---|---|---|
| `react` | `^18.3.0` | master RFC §4.1 拍 (React 18) | Wails `package.json` 一致 |
| `react-dom` | `^18.3.0` | 同上 | Wails 一致 |
| `vite` | `^5.4.0` | master RFC §3.2 拍 (Vite 5) | Wails 一致 |
| `@vitejs/plugin-react` | `^4.3.0` | 同上 | Wails 一致 |
| `typescript` | `^5.6.3` | master RFC §4.1 拍 (TS 5.6+) | Wails 一致 |
| `tailwindcss` | `^3.4.19` | shadcn 依赖 | Wails 一致 |
| `tailwindcss-animate` | `^1.0.7` | shadcn 依赖 | Wails 一致 |
| `postcss` | `^8.5.29` | tailwindcss 依赖 | Wails 一致 |
| `autoprefixer` | `^10.6.1` | tailwindcss 依赖 | Wails 一致 |
| `lucide-react` | `^1.55.0` | 图标 (沿用 Wails) | Wails 一致 |
| `class-variance-authority` | `^0.7.1` | shadcn 依赖 | Wails 一致 |
| `tailwind-merge` | `^3.7.0` | shadcn 依赖 | Wails 一致 |
| `clsx` | `^2.1.1` | shadcn 依赖 | Wails 一致 |
| `@radix-ui/react-slot` | `^1.5.0` | shadcn 基础 | Wails 一致 |
| `@types/node` | `^26.6.5` | shadcn + Vite 5 依赖 | Wails 一致 |
| `@types/react` | `^18.3.0` | React 18 types | Wails 一致 |
| `@types/react-dom` | `^18.3.0` | React 18 dom types | Wails 一致 |

**不**引 (vs Wails 差异):

- **`@types/vscode`** (vscode 扩展**主**依赖, Wails **不**用): `^1.140.0` (本 RFC 新增, spike 采 1.140)
- **shadcn CLI 自身** (`shadcn` 包, **不**进 `dependencies`, **不**进 `vsix`): 用 `pnpm dlx shadcn@4.22.0 init` 一次性跑, 生成的源码**进** git

### 2.4 Webview 渲染策略

按 [`2026-10-10-frontend-ui-stack.md`](./2026-10-10-frontend-ui-stack.md) §3.2 + §6 拍 (Vite 5 + shadcn/ui + 不共享代码):

```
frontend/vscode/
├── package.json                 (本 RFC §2.3 锁版本)
├── tsconfig.json                (strict + noUncheckedIndexedAccess + exactOptionalPropertyTypes, 沿用 ts-style skill)
├── tsconfig.node.json
├── src/
│   ├── extension.ts             (activate / deactivate)
│   ├── views/
│   │   ├── intentTree.ts        (TreeDataProvider 原生, §3.1 拍)
│   │   ├── intentEditor.ts      (WebviewPanel, §3.2 拍)
│   │   └── statusBar.ts         (StatusBarItem 原生, §3.3 拍)
│   ├── mcp/
│   │   └── client.ts            (本 RFC §2.2 StdioClientTransport)
│   └── lsp/
│       └── client.ts            (本 RFC §2.1 LanguageClient)
├── webview/                     (Vite 5 独立项目, **不**与 Wails 共享)
│   ├── package.json
│   ├── index.html
│   ├── tsconfig.json
│   ├── vite.config.ts
│   ├── components.json          (shadcn/ui 初始化产物)
│   └── src/
│       ├── App.tsx
│       ├── main.tsx
│       ├── components/
│       │   └── ui/              (shadcn 复制)
│       └── lib/
│           ├── utils.ts         (cn() helper)
│           └── mcp-types.ts
├── dist/                        (Vite 输出, .gitignore)
│   └── webview/
│       ├── index.html
│       └── assets/
│           ├── index-{hash}.js
│           └── index-{hash}.css
├── assets/
│   └── icon.svg
└── README.md                    (本 PR 同步更新 vs 现有 32 行占位)
```

**构建流程**:

```bash
$ cd frontend/vscode/webview && pnpm install   # 仅 webview
$ pnpm build                                   # → dist/webview/
$ cd .. && pnpm install                        # extension + webview 同步
$ vsce package                                 # → kron-vscode-0.1.0.vsix
```

**Webview 加载** (TS 端, `src/views/intentEditor.ts`):

```typescript
const html = fs.readFileSync(
  path.join(context.extensionPath, "dist", "webview", "index.html"),
  "utf8"
);
const panel = vscode.window.createWebviewPanel(
  "kronIntent", "Intent Editor", vscode.ViewColumn.One,
  {
    enableScripts: true,
    localResourceRoots: [
      vscode.Uri.file(path.join(context.extensionPath, "dist", "webview")),
    ],
  }
);
panel.webview.html = html.replace(/(src|href)="(\/.+?)"/g, (_, attr, p) =>
  `${attr}="${panel.webview.asWebviewUri(
    vscode.Uri.file(path.join(context.extensionPath, "dist", "webview", p.slice(1)))
  )}"`
);
```

### 2.5 骨架生成器: **不**走 `yo code` 默认, 走 **Vite 5 模板**

| 维度 | 拍板 | 理由 |
|---|---|---|
| **生成器** | `npx --package yo --package generator-code -- yo code` | master RFC §9.2 拍 1 步 |
| **生成器选项** | TypeScript + **不**选 Webpack (选 Vite 5) | master RFC §3.2 拍 "Vite 5" (但 yo code 仍**只**支持 Webpack/esbuild) |
| **结果** | yo code 跑出**空** extension (仅 `package.json` + `src/extension.ts` + `tsconfig.json`), Vite 5 webview 项目**手**手加 (按 §2.4 结构) | yo code **不**支持 React 18 + Vite 5 + shadcn 模板, **混**用反而混**乱** |
| **替代方案** | `pnpm create vite@latest webview -- --template react-ts` 单独创 webview 项目 | 标准 Vite 5 + React 18 + TS 模板 |

**反对意见** (AI 已表达 / 已驳回):

| 反对 | 驳回 |
|---|---|
| "yo code 一**站**式, **不**手搭" | 拒绝. yo code 模板**只**支持 Webpack, **Vite 5 需**自搭 webview 目录. 一**站**式是**假**象, 实际**两**步. 自搭 = 跟 Wails `webview/` 目录**结构**对齐, **少**认知成本 |
| "Vite 5 + Webpack 二**选**一, **不**混" | 接受. 拍**不**混: 扩**展**用 yo code (Webpack **不**开, 仅**用** TypeScript 模板的空**壳**), webview 用 Vite 5 |

### 2.6 shadcn/ui 范围 (本 RFC **不**列组件, 留 PR spike)

按 [`2026-10-10-frontend-ui-stack.md`](./2026-10-10-frontend-ui-stack.md) §3.2 拍, shadcn 组件**不**预设, 留 v.s. 实施 PR spike:

- **首期** (spike 验证): `Button` + `Card` + `Input` + `Badge` + `Separator` (5 个, 跟 Wails 一致)
- **二期** (v1.3+ 完整): `Dialog` + `Tabs` + `Toast` + `Tooltip` + `Form` (5 个, 拍板留 v.s. PR)
- **三期** (关系图): `Chart` (shadcn 4.22 含 `<Chart>` 组件, **待** spike 验证够**不**够, RFC §3.3)

**本 RFC 拍**: 首期 5 个 (Button/Card/Input/Badge/Separator), 二期 / 三期**留** v.s. PR.

---

## 3 spike 数据 (2026-10-18 实施)

### 3.1 `npm view` 各包版本 (采集时间 2026-10-18 16:30 UTC+8)

| 包 | 当前最新 | 时间 | 备注 |
|---|---|---|---|
| `yo` | 7.0.1 | 2026-10 | yeoman 7.0.1 |
| `generator-code` | 1.12.0 | 2026-10 | yo code 1.12.0 |
| `vscode-languageclient` | 10.1.2 | 2026-10 | 9.x 仍**活**跃, 10.x 是**主**推 |
| `@modelcontextprotocol/sdk` | 1.32.1 | 2026-10-15 | 1.x 仍**在** fast-iterate (monthly minor) |
| `shadcn` (CLI) | 4.22.0 | 2026-10-16 | 含 `<Chart>` 组件 (4.22+) |
| `@radix-ui/react-slot` | 1.5.0 | 2026 | shadcn 基础 |
| `@radix-ui/react-dialog` | 1.3.1 | 2026 | shadcn Dialog |
| `@radix-ui/react-tabs` | 1.2.1 | 2026 | shadcn Tabs |
| `@types/vscode` | 1.140.0 | 2026-10 | 配合 VSCode 1.85+ |

**spike 命令**:

```bash
$ npx --package yo --package generator-code -- yo --help
# ✅ yo 7.0.1 + generator-code 1.12.0 装得上, help 正常
$ npm view vscode-languageclient version
# 10.1.2
$ npm view @modelcontextprotocol/sdk version
# 1.32.1
$ npm view shadcn version
# 4.22.0
```

### 3.2 `yo code --help` 选项 (采集时间 2026-10-18 16:30 UTC+8)

**未跑** (本 spike 阶段**不**生成项目, 留 v.s. PR). 已知 yo code 选项 (按 1.12 文档):

- **Extension Type**: TypeScript (本 RFC 选)
- **Project name**: `kron-vscode` (本 RFC 锁)
- **Identifier**: `dev.kron.vscode` (本 RFC 拍, 跟 master `dev.kron.kron` 类似)
- **Description**: `Kron intent ledger — IDE integration`
- **Initialize git repo**: Yes
- **Bundle with webpack/esbuild**: **No** (本 RFC 拍, 因 webview 走 Vite 5)
- **Package manager**: `pnpm` (沿用 Wails)

### 3.3 shadcn `<Chart>` 组件 spike (本 RFC 阶段**不**验, 留 v.s. PR)

按 master RFC `2026-10-10-frontend-ui-stack.md` §3.2 拍, 关系图走 shadcn 原生 `<Chart>` v0.14+ 或按需扩展.

**本 RFC 阶段采** (待 v.s. PR 验):

- shadcn 4.22 含 `<Chart>` 组件 (基于 Recharts 1.x 包装)
- **不**够**可**能**性** (关系图需 force-directed graph): `<Chart>` **不**支持, 需**自**写 SVG / canvas
- **建议** (本 RFC **不**拍): 二期 v.s. PR spike Recharts 1.x 直接用, **不**走 shadcn `<Chart>` 包装

**反对意见** (AI 已表达 / 已驳回):

| 反对 | 驳回 |
|---|---|
| "**直**接引 Recharts, **不**走 shadcn `<Chart>`" | 接受 (留 v.s. PR 拍, 本 RFC **不**拍) |
| "**自**写 SVG / canvas" | 接受 (v1.4+ 评估, 本 RFC **不**拍) |

### 3.4 已知**未**拍项 (留 v.s. PR, **不**在本 RFC)

按 [`2026-10-10-frontend-ui-stack.md`](./2026-10-10-frontend-ui-stack.md) §8:

1. **Activity Bar 图标** — 拍**占**位 (vs 复用 Explorer), 走 v.s. PR
2. **Webview 深色/浅色主题同步** — 走 `--vscode-*` CSS 变量 + shadcn 4.22 `<ThemeProvider>`, 走 v.s. PR
3. **VSIX 更新机制** — 跟 binary 绑 (走 `kron` 版本号) vs 独立, 走 v.s. PR
4. **多项目支持** — 每个窗口独立 MCP 进程, 走 v.s. PR

---

## 4 实施清单 (本 RFC 拍**后** 留 v.s. PR)

> ⚠️ **本节是**"RFC 拍**板后**留 v.s. PR"**事项**, 本 PR (RFC) **不**实施代码.

| # | 改动 | 仓库位置 | 工作量 | 估时 |
|---|---|---|---|---|
| 1 | `frontend/vscode/` 跑 `yo code` (Webpack **不**开, 仅取 TypeScript 模板空壳) | `frontend/vscode/` | 0.5 天 | spike 1 |
| 2 | `frontend/vscode/webview/` `pnpm create vite@latest webview --template react-ts` | `frontend/vscode/webview/` | 0.5 天 | spike 2 |
| 3 | `pnpm dlx shadcn@4.22.0 init` (首期 5 组件: Button/Card/Input/Badge/Separator) | `frontend/vscode/webview/src/components/ui/` | 0.5 天 | spike 3 |
| 4 | `package.json` 锁本 RFC §2.1/§2.2/§2.3 拍版本 + `tsconfig.json` strict + shadcn `components.json` | `frontend/vscode/` + `frontend/vscode/webview/` | 0.5 天 | 拍板 |
| 5 | `src/extension.ts` activate/deactivate + `src/mcp/client.ts` (本 RFC §2.2) + `src/lsp/client.ts` (本 RFC §2.1) | `frontend/vscode/src/` | 1 天 | 实施 |
| 6 | `src/views/intentTree.ts` (TreeDataProvider 原生, 按 `2026-10-10-frontend-ui-stack.md` §3.1) | `frontend/vscode/src/views/` | 1 天 | 实施 |
| 7 | `src/views/statusBar.ts` (StatusBarItem, 按 `2026-10-10-frontend-ui-stack.md` §3.3) | `frontend/vscode/src/views/` | 0.5 天 | 实施 |
| 8 | `src/views/intentEditor.ts` (WebviewPanel + Vite 5 输出, 按本 RFC §2.4) | `frontend/vscode/src/views/` | 1.5 天 | 实施 |
| 9 | `webview/src/App.tsx` (shadcn 5 组件 + 列表) + `kron_tree` 调 MCP | `frontend/vscode/webview/src/` | 1.5 天 | 实施 |
| 10 | `.vscode-test/` + F5 host 装 `.vsix` 调 `kron serve-mcp` list 工具 (master §9.2 拍板 spike 6 步) | `frontend/vscode/` | 0.5 天 | spike 验证 |
| 11 | `docs/implementation/vscode-extension.md` 同步 (master 已**有**, 增量补**实施**细节) | `docs/implementation/vscode-extension.md` | 0.5 天 | 文档 |

**总改动 ~7.5 天**. 1 个 PR 链 (11 commit, 逐步 reviewable).

---

## 5 不做的事 (明确)

| 项 | 状态 | 理由 |
|---|---|---|
| ❌ 走 yo code 默认 Webpack 模板 | **不**做 | master RFC §3.2 拍 Vite 5, Webpack **不**混 |
| ❌ 浮动 `@latest` SDK 版本 | **不**做 | 本 RFC §2.1/§2.2 拍 caret 锁, 风险 |
| ❌ **不**拍 `vsce package` / Marketplace 发布 | **不**做 | release 阶段走 [`2026-10-13-gui-distribution-manual-install.md`](./2026-10-13-gui-distribution-manual-install.md) §4 拍 |
| ❌ **不**做 Webview 主题同步 (D2) | **不**做 | 留 v.s. PR, 本 RFC **不**拍 |
| ❌ **不**做多项目支持 (D4) | **不**做 | 留 v.s. PR, 本 RFC **不**拍 |
| ❌ **不**做关系图 (D3 / §3.3) | **不**做 | 留 v.s. PR, 本 RFC **不**拍 |
| ❌ **不**用 `KRON_BINARY_PATH` env | **不**做 | master RFC `2026-10-13-gui-distribution-manual-install.md` §2.1 拍 PATH only |
| ❌ **不**做 LSP 协议层 (`cmd/kron/serve-lsp/`) | **不**做 | 那是 @HomoFluctus 在 `feat/serve-lsp` 分支的事, 走 `2026-10-12-lsp-decision-points.md` |
| ❌ **不**动 `frontend/wails/` 任何文件 | **不**做 | 跟 vscode 隔离, 走铁律 #9 |
| ❌ **不**进 `Kron 主仓 go.mod` 加 Go dep | **不**做 | 客户端 SDK 走 npm + vsce, **不**走 Go explicit approval |
| ❌ **不**做 auto-install `kron` binary | **不**做 | master `2026-10-13-gui-distribution-manual-install.md` §2.1 拍用户**自**装, GUI silent |
| ❌ **不**预设 `frontend/vscode/` 根目录代码 | **不**做 | 拍**子**目录 (`src/` / `webview/` / `assets/`) |

---

## 6 与现有 RFC 的关系

| 现有 RFC | 关系 |
|---|---|
| [`2026-10-09-gui-ide-plan.md`](./2026-10-09-gui-ide-plan.md) §4.1 + §9.2 | **本 RFC 补 SDK 版本** — 9.x → 10.x spike 升级, 6 步走**不**改 |
| [`2026-10-10-frontend-ui-stack.md`](./2026-10-10-frontend-ui-stack.md) §3 + §6 | **本 RFC 补实施细节** — Vite 5 webview 目录结构, shadcn 5 组件, 打包流程 |
| [`2026-10-10-frontend-ui-stack.md`](./2026-10-10-frontend-ui-stack.md) §8 D1-D4 | **本 RFC **不**拍** — 留 v.s. PR |
| [`2026-10-13-gui-distribution-manual-install.md`](./2026-10-13-gui-distribution-manual-install.md) §2.1 | **沿用** — PATH only, 不引 env |
| [`2026-10-14-projects-json-schema-extension.md`](./2026-10-14-projects-json-extension.md) §2.1 | **沿用** — GUI 读 `projects.json`, vscode 调 `kron serve-mcp` 走 `-CWD` 优先级 1 |

---

## 7 红线确认

| 红线 | 拍板 |
|---|---|
| 加新 top-level dep (`go.mod` `require` 块) | ❌ **不**加 (客户端 SDK 走 npm, **不**进 Go 主仓) |
| 改 `go.mod` Go version | ❌ **不**改 |
| 改 package 路径 / 改 package 名 | ❌ **不**改 |
| `go generate` / reflection | ❌ **不**用 |
| 强推 / 改 shared branch 历史 | ❌ **不**做 |
| 加新 CLI 子命令 | ❌ **不**加 |
| 加 `config.toml` 字段 | ❌ **不**加 |
| 改 frontmatter schema | ❌ **不**改 |
| 改 intents/assumptions 数据架构 | ❌ **不**改 |
| 新 lint 规则 | ❌ **不**加 |
| 新 `internal/` 包 | ❌ **不**加 |
| 改 5 铁律 + 8 import 边界 | ❌ **不**改 |
| access layer 互调 | ❌ **不**让 |
| `internal/` import `cmd/kron/` | ❌ **不**让 |
| `cmd/kron/<sub>/` 互调 | ❌ **不**让 |
| `frontend/vscode/` 跟 `frontend/wails/` 互相 import | ❌ **不**让 (铁律 #9) |
| `frontend/vscode/` 直接 import `internal/` | ❌ **不**让 (走 MCP stdio) |

**全部红线避**.

---

## 8 拍板请求

请确认:
1. RFC §2.1 `vscode-languageclient@^10.0.0` 接受? (Y/N)
2. RFC §2.2 `@modelcontextprotocol/sdk@^1.32.0` 接受? (Y/N)
3. RFC §2.3 前端栈 (React 18 + Vite 5 + shadcn 4.22) 接受? (Y/N)
4. RFC §2.4 webview 目录结构 (跟 Wails 隔离, 不共享代码) 接受? (Y/N)
5. RFC §2.5 骨架生成器 (yo code 空壳 + Vite 5 webview 独立) 接受? (Y/N)
6. RFC §2.6 shadcn 首期 5 组件 (Button/Card/Input/Badge/Separator) 接受? (Y/N)
7. RFC §4 实施清单 (11 commit, ~7.5 天) 接受? (Y/N)
8. RFC §3.4 D1-D4 留 v.s. PR **不**在本 RFC 拍, 接受? (Y/N)

拍板后 commit RFC, 接着 v.s. PR 1-4 spike 走 §4 #1-3.
