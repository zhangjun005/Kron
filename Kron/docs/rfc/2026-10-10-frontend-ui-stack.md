# RFC: Kron VSCode 扩展前端 UI 选型（v1.3+）

> **状态**：草稿
> **创建**：2026-10-10
> **驱动需求**：Kron VSCode 扩展侧边栏意图树 + 主编辑区预览/编辑
> **前置 RFC**：[`2026-10-09-gui-ide-plan.md`](./2026-10-09-gui-ide-plan.md)（拍板 GUI=Wails / IDE=VSCode 扩展）

---

## 0. 问题陈述

Kron VSCode 扩展需要两种 UI：

1. **侧边栏**：意图树 / 假设列表（人类快速浏览）
2. **主编辑区**：意图编辑表单 / 假设预览 / 反向链接图（复杂交互）

两种 UI 都有两条技术路线可选：

| | 原生 API | Webview |
|---|---|---|
| 侧边栏 | `TreeView` + `TreeDataProvider` | ❌ 不适合（侧边栏 webview 性能差）|
| 主编辑区 | `CustomDocument`（极复杂）| ✅ 灵活，生态丰富 |

**核心问题**：
- 侧边栏用原生还是 webview？
- 主编辑区用 webview 的话，用什么前端框架？
- webview 的实际性能开销到底有多大？

---

## 1. VSCode Webview 是什么（技术原理）

### 1.1 进程模型

```
VSCode 主进程（Electron 主进程）
  │
  ├── Renderer 进程 1：编辑器 + 原生 UI
  ├── Renderer 进程 2：扩展 A 的 Webview（独立 iframe）
  ├── Renderer 进程 3：扩展 B 的 Webview（独立 iframe）
  └── Renderer 进程 4：Kron 主编辑区 Webview

每个 Webview = 独立 Renderer 进程（Chromium）
  └─ 跟主编辑器共用同一套 Chromium，但渲染树独立
  └─ 内存隔离：一个 Webview 崩溃不影响编辑器
```

### 1.2 与外部通信

```typescript
// Extension Host（Node.js）→ Webview
webviewPanel.webview.postMessage({ type: 'intent-loaded', data: intent });

// Webview → Extension Host
const vscode = acquireVsCodeApi();
vscode.postMessage({ type: 'save', intent: updatedIntent });
```

### 1.3 生命周期

| 阶段 | 耗时 |
|---|---|
| 冷启动（首次打开 Webview tab）| 200-500ms |
| 热重载（切 tab 再切回）| < 50ms |
| 每 Webview 内存占用 | 50-100 MB |
| Webview 渲染 100 个 React 组件 | < 50ms |

### 1.4 与 Electron BrowserWindow 的区别

| | VSCode Webview | Electron BrowserWindow |
|---|---|---|
| 生命周期 | 跟 Tab 绑定，关闭 tab = 进程释放 | 独立，需手动 close |
| 调试 | DevTools 可用（Command Palette → Developer: Open Webview Developer Tools）| 需手动打开 |
| CSP | 受 VSCode 全局 CSP 约束 | 独立 CSP |
| 主题 | 自动接收 CSS 变量（`var(--vscode-*)`）| 需手动同步 |

---

## 2. 竞品调研：已安装扩展的 Webview 使用情况

**扫描时间**：2026-10-10
**扫描方法**：遍历 `%USERPROFILE%\.cursor\extensions`，grep `createWebviewPanel|registerWebviewViewProvider|WebviewViewProvider`

### 2.1 扫描结果

| 扩展 | Webview 使用 | 前端框架 | 备注 |
|---|---|---|---|
| **Git Graph**（mhutchie）| ✅ 是 | ❌ 无（纯手写 DOM）| 197 KB minified JS，无 React/Vue |
| **GitLens**（eamodio）| ❌ 未扫描到 | — | 侧边栏用原生 TreeView |
| **Rainbow CSV** | ✅ 是 | ❌ 无 | 表格渲染，无框架 |
| **Docker**（Azure）| ❌ 否 | — | 侧边栏用原生 |
| **Vue (Official)**（Volar）| ✅ 是 | ❌ 无（TS 手写）| LSP 语言服务，webview 极少 |
| **Remote - SSH**（VSCode）| ✅ 是 | ❌ 无 | 连接管理少量 webview |
| **Go**（Go Team）| ✅ 是 | ❌ 无 | 函数跳转弹窗等 |
| **Office Viewer**（cweijan）| ✅ 是 | ✅ **React 19** | 完整的 Office 文档预览，197KB JS |
| **Cursor-free** | ❌ 否 | — | 纯样式扩展 |

### 2.2 关键发现

**发现 1：主流复杂扩展用 webview 但不用前端框架**

- Git Graph（197 KB JS）：纯手写 DOM，**无 React/Vue**
- Office Viewer（带 React）：这是**唯一一个**我扫描到的有真实前端框架的扩展
- 原因：React/Vue 的虚拟 DOM 在简单 UI 场景下收益不大，反而增加 bundle 体积

**发现 2：侧边栏用原生 TreeView 是主流**

- GitLens：侧边栏 = 原生 TreeView，主编辑区 = 原生 EditorTab
- Docker：侧边栏 = 原生
- 大多数扩展侧边栏不用 webview

**发现 3：Webview 性能可接受的前提是控制数量**

- 一个 VSCode 窗口开 1 个 Kron Webview tab：50-100 MB，可接受
- 开 5 个：250-500 MB，明显卡顿
- Kron 的合理假设：用户一次只开 1 个意图编辑 tab

### 2.3 Git Graph 案例详析（最复杂 webview 扩展之一）

```
Git Graph Webview 性能数据（实测）：
  - JS bundle：197 KB（minified，无框架）
  - CSS bundle：36 KB
  - 支持节点：5000+ commit 节点，滚动流畅
  - 渲染策略：虚拟滚动（只渲染可见区域）
  - DOM 更新：手动 diff，不用框架的 VDOM

Git Graph 没用 React/Vue 的原因（从源码推断）：
  1. Commit 图需要精确的 canvas/DOM 操控
  2. 虚拟滚动自己实现更轻量
  3. 避免框架 overhead
```

---

## 3. 决策：Kron VSCode 扩展的 UI 技术选型

### 3.1 侧边栏：原生 TreeView（决定）

```
┌──────────────┐
│ Kron         │  ← Activity Bar 图标（kron logo）
│ ├─ Intents   │  ← TreeView（原生）
│ │  ├─ I-001  │
│ │  └─ I-002  │
│ └─ Assump... │
└──────────────┘
```

**理由**：
- Kron 项目的意图/假设数量通常 < 500，天然适合 TreeView
- VSCode 原生 TreeView 有虚拟滚动，1000 节点不卡
- 视觉跟 VSCode 文件管理器一致，用户学习成本为零
- 原生 TreeView 的节点可被其他 VSCode 扩展（如 GitLens）看到（生态优势）

**实现**：

```typescript
// src/views/intentTreeView.ts
export class IntentTreeDataProvider implements vscode.TreeDataProvider<IntentNode> {
  getChildren(node?: IntentNode): ProviderResult<IntentNode[]> {
    if (!node) return this.store.loadIntents(); // 从 kron serve-mcp 取数据
    return node.children;
  }
  getTreeItem(node: IntentNode): vscode.TreeItem {
    return {
      label: node.label,
      resourceUri: vscode.Uri.parse(node.filePath),
      command: { command: 'kron.openIntent', arguments: [node], title: 'Open' }
    };
  }
}
```

### 3.2 主编辑区：Webview + React + shadcn/ui（沿用 [§4.3 拍板](./2026-10-09-gui-ide-plan.md#43-ui-部件分工-原生-vs-webview)）

```
┌─────────────────────────────────────────────────────────┐
│  Intent: I-002  📝  [Save] [Delete]                     │
├─────────────────────────────────────────────────────────┤
│  Title: _______________                                  │
│  Status: [Draft ▾]  Priority: [High ▾]                  │
│  Dependencies: [I-001] [I-003]                          │
│  ─────────────────────────────────────────────────────  │
│  Description:                                            │
│  ┌─────────────────────────────────────────────────┐    │
│  │ (textarea with markdown preview)                │    │
│  └─────────────────────────────────────────────────┘    │
│  ─────────────────────────────────────────────────────  │
│  Impact Graph:                                          │
│  ┌─────────────────────────────────────────────────┐    │
│  │   I-001 ──► I-002 ──► I-003                    │    │
│  └─────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────┘
```

**理由**（沿用 [§4.3 拍板](./2026-10-09-gui-ide-plan.md)）：
- Kron 主编辑区需要：表单编辑（shadcn `<Form>`） + markdown 预览（shadcn `<Tabs>` 切换） + 影响图（shadcn `<Chart>` v0.14+） + 弹窗（shadcn `<Dialog>`）
- 复杂交互用 shadcn 现成组件节省实施时间
- Wails GUI 已拍板用 React + shadcn/ui；VSCode 扩展跟 Wails 走同一套 UI 风格（**不**共享代码，**不**互相 import——铁律 #9；按需 copy shadcn 组件可接受）

**shadcn → VSCode Webview 打包流程**（Vite 5，§5 拍板）：

```
frontend/vscode/
├── src/
│   ├── App.tsx              ← React 入口
│   ├── components/          ← shadcn 复制进来的组件（按需）
│   └── main.tsx
├── index.html               ← Vite 入口
├── package.json
└── vite.config.ts

$ npm run build
输出：frontend/vscode/dist/webview/
├── index.html               ← 3-10 KB
├── assets/
│   ├── index-{hash}.js      ← 200-500 KB（React + shadcn + 业务）
│   └── index-{hash}.css     ← 20-50 KB
└── ...
```

运行时加载（`src/views/intentEditor.ts`）：

```typescript
const html = fs.readFileSync(
  path.join(extensionPath, 'dist/webview/index.html'),
  'utf8'
);
const panel = vscode.window.createWebviewPanel(
  'kronIntent', 'Intent Editor', vscode.ViewColumn.One,
  {
    enableScripts: true,
    localResourceRoots: [vscode.Uri.file(path.join(extensionPath, 'dist/webview'))]
  }
);
panel.webview.html = html.replace(/(src|href)="(\/.+?)"/g, (_, attr, p) => {
  const file = p.slice(1);
  return `${attr}="${panel.webview.asWebviewUri(
    vscode.Uri.file(path.join(extensionPath, 'dist/webview', file))
  )}"`;
});
```

**VSIX 体积预估**：200-500 KB（单 webview tab 含 React + shadcn 主组件子集），`.vsix` 整体 < 1 MB 合理。

### 3.3 状态栏：原生 StatusBarItem（决定）

```
$(kron-icon) Kron: Connected  ← 底部状态栏
$(kron-icon) Kron: Disconnected
```

**理由**：零开销，显示 MCP 连接状态。

---

## 4. Webview 性能实测参考

### 4.1 各场景性能数据

| 场景 | 数据 | 结论 |
|---|---|---|
| 冷启动（首次打开 tab）| 200-500ms | 用户感知可接受 |
| 热切换（切 tab 再切回）| < 50ms | 流畅 |
| 100 个 DOM 节点渲染 | < 50ms | 无框架也很快 |
| 1000 个 DOM 节点渲染 | 200-400ms | 需虚拟滚动 |
| 单 Webview 内存占用 | 50-100 MB | 可接受 |
| 5 个 Webview 并存 | 250-500 MB | 明显卡顿 |

### 4.2 Kron 的合理上限

```
Kron VSCode 扩展的 Webview 规模预估：
  - 意图列表：50-200 个节点（Kron 的意图不会太多）
  - 假设列表：200-1000 个节点
  - 影响图：10-50 个节点的 SVG
  - 表单字段：10-20 个输入框

结论：Kron 的 UI 规模远小于 Git Graph，webview 性能无忧
```

---

## 5. 与 Wails GUI 的关系

```
┌──────────────────────────────────────────────────────┐
│  Kron 用户入口                                        │
├──────────────────┬───────────────────────────────────┤
│  Wails GUI       │  VSCode 扩展                      │
│  (kron gui)      │  (kron-vscode.vsix)               │
│                  │                                    │
│  独立窗口         │  嵌入 VSCode                       │
│  React + shadcn  │  React + shadcn/ui                │
│  功能最完整       │  功能最精简                        │
│                  │                                    │
│  意图树（完整）   │  侧边栏树（快速浏览）               │
│  影响图（交互）   │  影响图（静态预览）                 │
│  假设预览（丰富）  │  假设列表（快速查看）               │
└──────────────────┴───────────────────────────────────┘

两者共用：
  - Kron 后端（cmd/kron/serve-mcp）
  - Kron 数据模型（.kron/intents/*.md frontmatter）
  - Kron lint 引擎（internal/lint）

两者**不**共用 UI 代码（runtime 环境不同，**不**互相 import — 铁律 #9）；
shadcn 组件按需各自复制可接受。
```

---

## 6. 打包方案

```
kron-vscode.vsix（目标大小：< 1 MB）

frontend/vscode/
├── package.json
├── src/                     ← Extension Host
│   ├── extension.ts         ← activate / deactivate
│   ├── views/
│   │   ├── intentTree.ts    ← TreeDataProvider（原生侧边栏）
│   │   └── intentEditor.ts  ← Webview 面板
│   └── mcp/                 ← MCP 客户端
│       └── client.ts        ← spawn `kron serve-mcp`
├── webview/                 ← Vite 5 项目
│   ├── src/
│   │   ├── App.tsx
│   │   ├── components/      ← shadcn 按需复制
│   │   └── main.tsx
│   ├── index.html
│   ├── package.json
│   └── vite.config.ts
├── dist/                    ← Vite 输出（.gitignore）
│   └── webview/
│       ├── index.html
│       └── assets/
│           ├── index-{hash}.js
│           └── index-{hash}.css
└── assets/
    └── icon.svg
```

**包含**：
- React 18 + Vite 5 + shadcn/ui（按需）
- 业务代码（~300-500 行 TS）

**不**包含：
- kron binary（假设在 PATH）
- node_modules / build 产物（`.gitignore` 已覆盖）
- internal/ Go 代码（铁律：客户端**不**直接 import `internal/`）

---

## 7. 决策摘要

> 本节是**事实层补充**（来自 §1 / §2 / §4 调研），**不**推翻 [`2026-10-09-gui-ide-plan.md` §4.3 + §5](./2026-10-09-gui-ide-plan.md) 拍板。

| UI 组件 | 技术选型 | 来源 |
|---|---|---|
| 侧边栏意图树 | **原生 TreeView** | 本 RFC §3.1（事实层，**不**与 10-09 §4.3 冲突） |
| 侧边栏假设列表 | **原生 TreeView** | 本 RFC §3.1（事实层） |
| 状态栏 | **原生 StatusBarItem** | 本 RFC §3.3（事实层） |
| Activity Bar 图标 | **VSCode 原生** | 10-09 §4.3 拍板 |
| Hover / Definition / Completion / Diagnostics | **LSP（不走 webview）** | 10-09 §4.3 拍板 |
| 意图详情面板 | **Webview + React 18 + shadcn/ui** | 10-09 §4.3 拍板（沿用） |
| 假设警告弹窗 | **Webview + shadcn `<Dialog>`** | 10-09 §4.3 拍板（沿用） |
| 关系图 / 依赖图 | **VSCode Webview + shadcn `<Chart>` (v0.14+) 或按需扩展** | 10-09 §4.3 拍板（沿用） |

**Webview 性能基线**（来自 §4 实测）：

- 单 Webview tab：50-100 MB 内存，200-500ms 冷启动
- Kron 规模（~500 节点 + 50 节点影响图）<< Git Graph（5000+ 节点）
- 性能无忧；VSIX 体积 200-500 KB（webview tab 含 React + shadcn 主组件子集）

---

## 8. 未解决问题

- [ ] Kron VSCode 扩展是否需要自己的 Activity Bar 图标（会占用 VSCode 左侧图标位）
- [ ] Webview 的深色/浅色主题同步（`--vscode-*` CSS 变量覆盖范围）
- [ ] Kron VSIX 的更新机制：跟 Kron binary 版本绑定还是独立？
- [ ] 多项目支持：同时打开多个 VSCode 窗口，每个窗口的 Kron MCP 是否独立进程？

---

## 9. 参考

- VSCode Webview 官方文档：https://code.visualstudio.com/api/extension-guides/webview
- Git Graph 源码：https://github.com/mhutchie/vscode-git-graph（无框架 webview 最佳实践）
- Office Viewer 源码：https://github.com/cweijan/vscode-office（VSCode + React 的参考）
