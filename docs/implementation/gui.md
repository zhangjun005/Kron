# Wails GUI — 接口面文档 (work 1.4.d 多项目概览起步)

> 状态: **v1.4.d 草案, 2026-10-18**
> 架构依据: [`../abstractDesign/architecture.md` §〇 铁律 #9 + §〇·五·1](../abstractDesign/architecture.md) + [`../rfc/2026-10-09-gui-ide-plan.md`](../rfc/2026-10-09-gui-ide-plan.md) §3
> 实现位置: `frontend/wails/`

---

## 1 范围

Wails 桌面 GUI 的"接口面"——**只**描述 GUI 与后端能力之间的边界、Go ↔ TS 桥方法、MCP tool 选型、**不**列 UI 细节。

UI 细节归 `frontend/wails/frontend/src/components/` 内的组件文件 + `App.tsx`。本文档是它们背后的契约。

---

## 2 边界铁律 (重申)

| 约束 | 落点 |
|---|---|
| **不**直接 import 主仓 `internal/` | `frontend/wails/go.mod` module = `kron-wails` (独立 module), 无 `replace` 指向主仓 |
| **不** import 主仓 `cmd/kron/` Go 代码 | 同上; GUI ↔ 主仓**只能**通过 stdio JSON-RPC (即 `kron serve-mcp` 子进程) |
| **不**与 `frontend/vscode/` 互相 import | 铁律 #9 (客户端层互不依赖) |
| 进程外 stdio JSON-RPC 是唯一通路 | `frontend/wails/app.go` 的 `handshakeLocked` 走 LSP/MCP 三消息握手; `CallMCPTool` 走 `tools/call` |

---

## 3 Go ↔ TS 桥方法 (App 暴露)

Wails 把 `App` 结构体的**导出方法**作为 Go ↔ TS 桥。每个方法从 TS 端是 `Promise<T>`, Go 端是普通方法。

| 方法 | 入参 | 返回 | 阶段 | 备注 |
|---|---|---|---|---|
| `GetKronInfo()` | — | `KronInfo` | 1.3 skeleton | 静态元数据; 不调子进程 |
| `ListMCPToolsLive()` | — | `ListMCPToolsLiveResult` | 1.4.b | spawn `kron serve-mcp` + 三消息握手 + 缓存 serverInfo / tools |
| `CallMCPTool(name, argsJSON)` | tool 名 + JSON 字符串 | `string` (首个 content block 的 text) | 1.4.c | 通用 `tools/call`; 复用 1.4.b 的子进程 |
| **`ListProjectsLive()`** | — | `ListProjectsLiveResult` | **1.4.d (本 RFC 范围)** | 走 `kron_list_projects` MCP tool, 复用现有子进程 |

### 3.1 `ListProjectsLive()` 设计 (1.4.d)

> **已实现，但与本节描述不同（2026-10-19 合并时标注）**：`kron_list_projects` MCP tool **不存在**，
> 主仓从未注册过它。实际实现是 Go 端**直读** `projects.json`（路径：`KRON_PROJECTS_JSON_PATH` >
> 可执行文件同目录），项目切换时原子写回并重启 `serve-mcp`。以本文末尾
> 「Wails follow-up: live dashboard and project switching」为准；本节保留为 1.4.d 设计原稿。

**目的**: GUI 顶部 / 侧栏显示"已注册项目"列表, 让用户看到机器上所有 Kron 项目 (类似 VSCode "Recent" / "Open Folder" 体验)。

**调用链**:
```
TS: ListProjectsLive()
  → Go: ListProjectsLive(ctx)
    → CallMCPTool("kron_list_projects", "{}")  (复用 1.4.c 桥)
      → JSON-RPC tools/call
        → MCP server: HandleListProjects
          → store.ReadProjectsJSON()  (主仓 internal/store)
```

**返回类型**:
```go
type ListProjectsLiveResult struct {
    Projects     []ProjectEntry `json:"projects"`
    Current      string         `json:"current"`     // path of current_project, "" if none
    ProjectsPath string         `json:"projectsPath"` // for diagnostic display
    Source       string         `json:"source"`      // human-readable provenance
}

type ProjectEntry struct {
    Path       string `json:"path"`
    AddedAt    string `json:"addedAt,omitempty"`
    LastOpened string `json:"lastOpened,omitempty"`
}
```

**错误**: 任何 stdio / 解析失败 → `(ListProjectsLiveResult{}, error)`; Go 端不吞错误, TS 端在 `LiveProjectsError` state 显示.

**复用**:
- 复用 1.4.c 的 `CallMCPTool` 桥——**不**开新的握手 / 子进程逻辑
- 复用 1.4.b 的 `mcpInitialized` 缓存——首次 `ListProjectsLive` 触发握手, 后续 `CallMCPTool` 复用

**不做什么 (本轮)**:
- ❌ **不**为每个项目并发调 `kron_lint` / `kron_tree` 聚合健康度 (RFC §3.1 列举的"42 个意图 / 8 个假设 / 2 errors"留给后续 1.4.e+)
- ❌ **不**支持项目"切换"(GUI 没 spawn 子进程的工作目录切换能力; 切项目 = 重启 GUI / 重启 serve-mcp, 超本轮范围)
- ❌ **不**改写主仓 `internal/store` 的 `ProjectRegistry` schema

---

## 4 MCP tool 选型 (GUI 调的工具清单)

| MCP tool | 用途 | 阶段 |
|---|---|---|
| `kron_tree` | 单项目意图目录树 | 1.4.c 已用 |
| `kron_get` | 意图详情 (按需) | 1.4.c 预留 |
| `kron_lint` | 锚点 + frontmatter 校验 | 1.4.d 已用（lint 计数） |
| `kron_impact` / `kron_intent_density` / `kron_stale` / `kron_assume_check` | AI 主动消费 | 后续 |
| ~~`kron_list_projects`~~ | ~~多项目注册表~~ | **未实现**：主仓无此 tool；实际直读 `projects.json`（见 §3.1 顶部标注与末尾 follow-up） |

> 本轮**不**改主仓 internal/store / serve-mcp Go 代码。`projects.json` 是 GUI 的**项目清单**（决定打开哪些项目），
> 不属 intent 数据；intent / assumption 仍然只经 `serve-mcp`。

---

## 5 TS 端调用规约

| 约束 | 落点 |
|---|---|
| 每次 `ListProjectsLive()` 调用都包在 `try/catch` | TS `App.tsx` 已有 `refreshLive` 模板, 复制其错误处理形态 |
| 并发安全: `ListProjectsLive` 与 `ListMCPToolsLive` / `CallMCPTool` **共用一个** Go 子进程, mcpMu 串行化 | Go 端已保证, TS 端无需关心 |
| 渲染态: `liveProjects` / `liveProjectsError` / `liveProjectsLoading` 三态 | 复用 `liveTools*` 的 state 模式 |
| 失败不抖动: 失败时保留旧 `liveProjects`, 只在 `liveProjectsError` 中提示 | 同 `liveTools` |

---

## 6 测试策略 (Wails 模块独立)

| 层 | 工具 | 范围 |
|---|---|---|
| Go (`frontend/wails/`) | `go test ./...` | `app.go` 现有函数 + 1.4.d 新增 `ListProjectsLive` |
| TS (`frontend/wails/frontend/`) | `npm run lint` + `tsc --noEmit` | App.tsx + 新组件 |
| 手测 | `wails dev` 启 GUI, 切到 "Projects" 视图, 确认列表渲染 | work 1.4.d 验收 |

> Go 侧单元测试**不**测 MCP 协议细节 (那是主仓 `cmd/kron/serve-mcp/` 的责任); GUI Go 端只测"子进程解析失败时返回合理错误"。

---

## Wails follow-up: live dashboard and project switching

This section supersedes the 1.4.d deferrals above for the Wails GUI only.

- `ListProjectsLive()` reads the RFC 2026-10-13 `projects.json` beside the GUI executable. This is the documented GUI exception for the registry file; it does not import `internal/` or call a tool that the MCP server does not register.
- When `current_project` is absent or invalid, the GUI displays the most recently opened registered project as the default without writing the registry. A user click on **Switch** transactionally updates `current_project` and `last_opened`, then restarts `serve-mcp` with `-CWD <project-root>`.
- The live explorer and counts come from `kron_tree`; lint counts come from `kron_lint`. They describe the selected project only. If there is no active project, the main pane shows the no-active-project state instead of any placeholder data.
- The Projects view reports registry loading/read errors and switch errors. The main pane distinguishes loading, no active project, connection/tree errors, and an empty real tree. Refresh retries the live calls.
- `KRON_PROJECTS_JSON_PATH` is an optional local override for development. Production uses the executable directory as the registry location.
- There is no mock/demo data source any more: the GUI always reads `projects.json` and calls `kron serve-mcp`. The shared integration fixture at `testdata/kron-project/` is opened like any other registered project.

## 7 与 RFC §9.1 交付物对应

| RFC §9.1 checklist | 本文档对应 | 状态 |
|---|---|---|
| `frontend/wails/` Wails 项目骨架 | `wails.json` + `app.go` + `frontend/src/` 已就位 | ✅ 1.3 / 1.4.b / 1.4.c 已实做 |
| 子 RFC: Wails SDK 版本锁定 + shadcn/ui 范围 | Wails v2.16.0 (`frontend/wails/go.mod`) + shadcn 组件 (`frontend/src/components/ui/`) | ✅ 1.3 拍板 |
| spike 验证: Wails 起窗口 + 调 `kron serve-mcp` list 工具 | 1.4.b `ListMCPToolsLive` 跑通 | ✅ 1.4.b 实做 |
| **1.4.d 多项目概览起步** | 本文档 §3.1 + `app.go` `ListProjectsLive` | 🆕 **本轮交付** |

## 8 Explorer 意图画布（2026-10-20）

Explorer 主面板不再渲染文件浏览器式的列表树，而是**左→右的意图树画布**，实现文件
`frontend/wails/frontend/src/components/intent-canvas.tsx`（原 `intent-tree.tsx` 已删除）。

| 行为 | 说明 |
|---|---|
| 布局 | root → 分组目录 → 意图叶子，**从左到右**；分组节点显示目录名与意图数，叶子显示意图标题与 slug |
| 背景 | 点阵（radial-gradient），随画布平移 |
| 平移 | 拖拽空白处移动整张画布；`Reset view` 复位。不做缩放（未拍板） |
| 节点点击 | 在节点**下方**展开信息卡：标题 / slug / 状态 / 更新时间 / 正文描述 / **符号、Reviewers、Assumptions（severity + rationale + expires_at）、references / depends_on**；点卡外或右上角 × 关闭；relations 芯片可点击跳到目标节点 |
| 信息卡拖拽 | 按住卡内任意非交互区域（按钮 / 链接 / `code` 除外）即可拖动，位置相对节点锚点偏移并随画布平移；切换选中节点时回到默认锚点。拖拽不会触发画布平移，也不会触发「点卡外关闭」 |
| 数据 | 仍然只来自 `kron_tree` + 每叶 `kron_get`；画布不写任何文件、不调写工具 |
| 只读边界 | GUI 不提供 intent 的增删改（无 `Reset` / `New intent` 之类入口）；编辑在用户 IDE 里做 |
| 过滤 | 顶部 Filter 命中时高亮，未命中的节点淡出（保留结构，不重排） |
| 跨意图关系 | `references`（软）画**虚线细箭头**、`depends_on`（硬）画**实线粗箭头**；两者都是节点间的横向关系，不是父子层级边；端点不在当前树中的关系不画；画布左下角有图例 |
| 关系高亮 | 选中节点时，它指向的节点按字段加框：`depends_on` → **实线框**，`references` → **虚线框**（与箭头轻重一致；两边都有的取 `depends_on`）。选中节点自身仍用 `ring` 高亮 |
| 左侧栏 | `FILES` 是**完整递归树**（文件夹可展开/折叠到叶子，叶子点击选中并居中画布）；`ARCHIVED` 汇总 stale 叶子；`ASSUMPTIONS` 是独立的假设区 |
| 假设展示 | 数据来自 `kron_assume_list`；行内显示 id + `hard/soft` 徽章，展开后显示 registry 文本、`expires_at` / `verified_at` / `verified_by`，以及 **referenced by** 的意图芯片（从树的 per-intent `assumptions[]` 反推），点击芯片跳到画布节点 |

### 8.1 子进程调度（弹窗 / 探测）

- GUI 与 CLI 是两个二进制；bridge 为**每个 GUI 实例**只解析一次 CLI 路径（结果缓存，解析失败不缓存，方便补上二进制后重试）。
- 所有子进程（CLI 探测探针 + 长期 `serve-mcp`）都以 `CREATE_NO_WINDOW` + `HideWindow` 启动。否则双击启动（GUI 无控制台）时 Windows 会为每个子进程新建控制台窗口——表现为「切 Live 弹出好几个窗口」。
- `serve-mcp` 的 stderr 写入 GUI 可执行文件旁的 `serve-mcp.log`（windowsgui 子系统没有可用 stderr），握手失败可直接查日志。

> 已知限制：画布没有缩放，宽项目需要拖拽查看；节点很多时不做虚拟化（当前项目规模够用）。

### 8.2 测试夹具（live 模式）

`testdata/kron-project/`（**已入库**，供 GUI / MCP / LSP / CLI 各模块共用）由 [`frontend/wails/scripts/seed-test-project.ps1`](../../frontend/wails/scripts/seed-test-project.ps1)
生成，覆盖完整 frontmatter schema：`symbol` / `status` / `reviewers` / `assumptions`（B-3 registry 文本与 per-intent severity）/ `references` / `depends_on`，三层树、源码与 Markdown 锚点，外加**有意设计**的 1 error + 1 warning 供错误/警告渲染测试。

### 8.3 发布态模拟环境（`kron-mcp-test/`，2026-10-20 约定）

**约定**：以后所有「真实发布态」集成测试都在 gitignored 的 `kron-mcp-test/` 进行；环境由[`frontend/wails/scripts/seed-release-env.ps1`](../../frontend/wails/scripts/seed-release-env.ps1) 从 HEAD 重建（可随时删掉重来）。

布局是**一体化**的——GUI、CLI/MCP、registry、被测项目放在一起，直接双击运行，不模拟 `%LOCALAPPDATA%\Programs\kron` / PATH-only 的用户安装：

```
kron-mcp-test/
├── app/                 ← 启动这里
│   ├── Kron-GUI.exe     ← GUI（raw Wails build）
│   ├── kron.exe         ← CLI + serve-mcp + serve-lsp
│   ├── projects.json    ← 由 `kron register` 写在 exe 旁
│   └── serve-mcp.log
└── project/             ← testdata/kron-project 的副本（含 .kron/）
```

关键点：

- **`projects.json` 必须和两个 exe 在一起**：GUI 桥读**自己 exe 目录**下的 registry；`kron serve-mcp` 不带 `-CWD` 时走 [`internal/store.ProjectsJSONPath`](../../internal/store/project_registry.go)（同样读 exe 旁的 json），再回退 `os.Getwd()`。
- 因此 GUI 与 CLI 必须在**同一目录**（否则两份 registry 各说各话）；Windows 大小写不敏感，`Kron.exe` 与 `kron.exe` 不能共存，故 GUI 命名 `Kron-GUI.exe`、CLI 保留发布名 `kron.exe`。
- **验证脚本**：[`verify-mcp-registry.ps1`](../../frontend/wails/scripts/verify-mcp-registry.ps1) 用 [`mcp-probe`](../../frontend/wails/scripts/mcp-probe/main.go)（Go 手写 stdio 客户端）在「cwd 为无关目录、无 `-CWD`」条件下调 `kron_list`，要求从 exe 旁 registry 解析出 8 条 intent；并把 registry 移走做负向对照（必须失败）。
- 已验证：GUI 双击（cwd=`app`）→ Live 树正常，spawn 的是 `app\kron.exe serve-mcp -CWD project`；MCP 无 `-CWD` 仍能引用 json；`kron lint` 与 fixture 一致（1 error + 1 warning）。
