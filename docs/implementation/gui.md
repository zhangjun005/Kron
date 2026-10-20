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
| `ListMCPTools()` | — | `[]string` | 1.3 skeleton | **硬编码** 13 tool 名 (mock 模式用);**不**调子进程 |
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
- The live explorer and counts come from `kron_tree`; lint counts come from `kron_lint`. They describe the selected project only. If there is no active project, no mock counts or tree are presented as live data.
- The Projects view reports registry loading/read errors and switch errors. The main pane distinguishes loading, no active project, connection/tree errors, and an empty real tree. Refresh retries the live calls.
- `KRON_PROJECTS_JSON_PATH` is an optional local override for development. Production uses the executable directory as the registry location.
- Mock mode has separate frontend-only project fixtures: switching samples changes the displayed tree, counts, and lint summary without reading or writing `projects.json` or starting MCP. It includes an empty project state and expandable groups, and is strictly read-only (no add/reset affordances).

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
| 节点点击 | 在节点**下方**展开信息卡（标题 / slug / 状态 / 更新时间 / 正文描述）；点卡外或右上角 × 关闭 |
| 数据 | 仍然只来自 `kron_tree` + 每叶 `kron_get`；画布不写任何文件、不调写工具 |
| 只读边界 | 移除了 mock 模式的 `Reset` / `New intent` 演示按钮——GUI 不提供 intent 的增删改；编辑在用户 IDE 里做 |
| 过滤 | 顶部 Filter 命中时高亮，未命中的节点淡出（保留结构，不重排） |

> 已知限制：画布没有缩放，宽项目需要拖拽查看；节点很多时不做虚拟化（当前项目规模够用）。
