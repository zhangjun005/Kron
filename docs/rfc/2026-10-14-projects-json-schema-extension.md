# RFC: `projects.json` Schema 拍板（精简版）

| 字段 | 值 |
|---|---|
| **状态** | **已拍板 (2026-10-14, zhangjun005 拍板定稿:不预设 `version`/`settings` 子节;精简版 3.5 天;取代 multi-project-registry §2.2;待 PR 实施)** |
| **作者** | AI assistant, 经 zhangjun005 委托 |
| **创建日期** | 2026-10-14 |
| **目标版本** | v1.3+ |
| **实施时机** | 与 [`2026-10-13-multi-project-registry.md`](./2026-10-13-multi-project-registry.md) 同步实施（最后发布时再做）|
| **补充** | [`2026-10-13-multi-project-registry.md` §2.2](./2026-10-13-multi-project-registry.md)（projects.json 基本结构 — **本文拍板的最终版本取代**）|
| **取代** | multi-project-registry.md §2.2 + 撤回本文档之前 16:30 那版的 `version` / `settings` 占位 |

---

## 1 动机

[`2026-10-13-multi-project-registry.md` §2.2](./2026-10-13-multi-project-registry.md) 拍板了 `projects.json` 的**最小** schema。但**没**拍：

1. **CLI / MCP / LSP / GUI 的配置字段**放哪？`config.toml` 架构 §3.5 拍板**只**放 `intents_dir`, 那"其他配置"**放哪**？
2. **schema 将来扩充**时的兼容策略（forward + backward）？

**问题**：是否要在 `projects.json` **预先**加 `version` / `settings` / 子节？

**拍板**：**不**。

理由：
- **CLI 真实需求**：`color` / `default_output_format` 等**已**走 POSIX `NO_COLOR` 标准 + 项目级 `config.toml` + flag 临时覆盖, `projects.json` 插一杠**徒增**概念
- **MCP 真实需求**：`log_to_stderr` / `cache.strategy` 等**都**是**架构铁律固定**或**单一默认**, **不**让用户配置（stdio 隔离 = 铁律 #5）
- **LSP / GUI 真实需求**：SDK 还没引入, GUI 还没实装; **预先**拍字段**会**和实际设计**冲突**（zhangjun005 2026-10-14 拍板）
- **GUI 偏好**（主题 / 默认视图）：**客户端层自治**, 走 GUI 自己的 `localStorage` / `electron-store` 风格, **不**进 `projects.json`

**结论**：`projects.json` **只**放**最小** schema。**任何**新字段需求**单独**开 RFC, **不**预先占位。

---

## 2 拍板

### 2.1 顶层 schema（v1.3 拍板版 — **最终**精简版）

```jsonc
// projects.json — v1.3+ 拍板 schema
{
  "current_project": "E:\\works\\proj1", // 当前激活项目绝对路径, 可空

  "projects": [                          // 已注册项目列表, 必填 (空数组合法)
    {
      "path": "E:\\works\\my-project",   // 绝对路径, 必填
      "added_at": "2026-10-13T12:00:00Z",  // ISO 8601 UTC, 必填
      "last_opened": "2026-10-13T15:30:00Z" // ISO 8601 UTC, 可空 (从未打开)
    }
  ]
}
```

**就这两个字段**。**没有** `version`, **没有** `settings`, **没有** 任何子节。

| 顶层字段 | 必填 | 说明 | 谁写 |
|---|---|---|---|
| `current_project` | ❌ | 绝对路径字符串, 可空 | GUI（用户切换项目时写）；`kron register` 注册成功后自动更新 |
| `projects` | ✅ | 数组, **空数组合法** | `kron register` / `kron init` / GUI |

| `projects[]` 字段 | 必填 | 说明 | 谁写 |
|---|---|---|---|
| `path` | ✅ | 绝对路径 (`filepath.Abs`) | `kron register` / `kron init` |
| `added_at` | ✅ | ISO 8601 UTC | `kron register` / `kron init` |
| `last_opened` | ❌ | ISO 8601 UTC, 可空（从未打开） | GUI（用户点击卡片时更新）|

**去重**：`path` 唯一的项目**不**重复写入（`path` 唯一键）。

### 2.2 智能读取（Forward + Backward Compat — **仍然**要）

虽然本 RFC **不**引入 `version` 字段, 但**未来**扩字段时**一定**要走"宽容读取"。这里**先**把读取算法拍下来, 未来**新**字段（开 RFC）走 §2.3 流程。

#### 2.2.1 读取算法（v1.3 拍板）

```
ReadProjectsJSON(path):
  1. 文件不存在 → 返回空 schema (projects=[], current_project="")
  2. 文件存在:
     a. os.ReadFile 整个文件
     b. json.Unmarshal 到 ProjectRegistry (Go struct, 字段都是 omitempty)
     c. 字段缺失 → Go 零值兜底 (空字符串 / nil 数组)
     d. **未来**加字段时, 旧 reader 读新文件:
        - 未知字段 → Go json.Unmarshal 默认行为: 忽略 (不报错)
     e. 字段类型不匹配 (e.g. projects 应为数组, 实为 string) → 报错 (解析失败, 不宽容)
```

#### 2.2.2 写入算法（v1.3 拍板）

```
WriteProjectsJSON(path, registry):
  1. Marshal registry (只写已知字段)
  2. 选填字段走 omitempty, 零值**不**写
  3. 写盘走 §2.3 原子写
```

#### 2.2.3 关键不变量

| 不变量 | 说明 |
|---|---|
| **写者只写已知字段** | 永远不写未来字段（即使 v1.4 加了）|
| **选填字段** `omitempty` | 不写零值, 保持文件**最小化** |
| **读者宽容未知字段** | Go `json.Unmarshal` 默认行为, **不**报错 |
| **Go struct 零值兜底** | 业务层**不**做 "字段不存在" 特殊处理 |

### 2.3 扩展流程（**未来**加字段时**必须**走）

**不**在 v1.3 拍板期预先加字段。**未来**要加任何字段（无论 `mcp` / `cli` / `lsp` / `gui` / 其他）时, **必须**走以下流程：

1. **开新 RFC** 拍板新字段, 文件命名 `docs/rfc/YYYY-MM-DD-<slug>.md`
2. RFC 必须包含：
   - 真实需求（不是"以备后用"）
   - 字段 schema（类型 / 必填 / 默认值 / 谁写）
   - 与现有字段的兼容策略（**仍然**走 §2.2.3 不变量）
   - 与 `config.toml` / `localStorage` / 环境变量等**已有**机制的分工
3. **不**在 RFC 中拍"将来 vN.M 还可能加 X" — AGENTS.md §5 拍板**不**双份维护
4. **不**在 RFC 中拍"占位字段" — 占位**等于**猜设计, **会**和实际冲突

### 2.4 原子写

走 `internal/store/writer.go:writeFileAtomic` 既有模式（[`writer.go:478-503`](../../internal/store/writer.go)）：

- 临时文件 `.projects.json.tmp-<rand>` → `f.Sync()` → `os.Rename`
- 同一目录, 同一文件系统, `rename` 原子
- **不**引入新 dep

### 2.5 跨平台路径获取

走 [`2026-10-13-multi-project-registry.md` §2.1](./2026-10-13-multi-project-registry.md) 已拍板的"`os.Executable()` 同目录"：

| OS | 路径 |
|---|---|
| Windows | `filepath.Dir(os.Executable()) + "projects.json"` |
| Linux | 同上 |
| macOS | 同上 |

**不**走 `KRON_PROJECTS_PATH` env。**不**走 `~/.config/kron/`。**不**走 `%APPDATA%`。

理由：单文件可移植 (拷 U 盘 / 移动硬盘整体带走)，符合 GUI 1-zip-装一切 ([`2026-10-13-gui-distribution-manual-install.md` §2.1](./2026-10-13-gui-distribution-manual-install.md))。

### 2.6 GUI 暴露面

GUI 读 / 写 `projects.json` 的**唯一**职责：**项目列表**（[multi-project-registry §2.5](./2026-10-13-multi-project-registry.md)）：

| 场景 | 字段 |
|---|---|
| 启动读全部 | `current_project` + `projects[]` |
| 用户点击卡片 | 写对应项目的 `last_opened` |
| 用户切换项目 | 写 `current_project` + 对应项目的 `last_opened`（一次事务）|

**GUI 偏好**（主题 / 默认视图 / 排序）走**客户端层**自己的 `localStorage` / `electron-store`, **不**进 `projects.json`（zhangjun005 2026-10-14 拍板）。

GUI **不**监听文件（不 poll, 不 inotify）。

---

## 3 不做的事（明确）

| 项 | 状态 | 理由 |
|---|---|---|
| ❌ `version` 字段 | **不**做 | v1.3 拍板期**不**预先占位; 未来扩字段时**不**回写 version (Go `json` 默认忽略未知字段即可) |
| ❌ `settings` 顶层字段 | **不**做 | CLI / MCP / LSP / GUI 各自有**真实**需求时**单独**开 RFC 拍 |
| ❌ `settings.mcp` 子节 | **不**做 | MCP stdio 行为**全是**铁律固定 / 单一默认, **无**真实用户配置需求 |
| ❌ `settings.cli` 子节 | **不**做 | CLI 行为走 `config.toml` (项目级) + POSIX env (跨项目标准) + flag (临时) 三层机制, `projects.json` 插一杠**徒增**概念 |
| ❌ `settings.lsp` 子节 | **不**做 | LSP SDK 仍未引入; 预先拍字段**会**和实际设计**冲突** |
| ❌ `settings.gui` 子节 | **不**做 | GUI 偏好走客户端层 `localStorage`, **不**进 `projects.json` |
| ❌ GUI 偏好 (主题 / 默认视图) 进 `projects.json` | **不**做 | 客户端层自治 |
| ❌ 字段加密 / 敏感配置 (token, secret) | **不**做 | 配置**不**放敏感数据 |
| ❌ 文件锁 / 跨进程并发写处理 | **不**做 | 单用户本地文件, 忽略并发写（极少发生）|
| ❌ `fsnotify` / 文件变化监听 | **不**做 | 零 daemon, 零监听 |
| ❌ `KRON_PROJECTS_PATH` env | **不**做 | 零配置原则 |
| ❌ LSP / Wails SDK 真实接入 | **不**做 (本 RFC) | SDK 仍未引入, 等真实引入**后**开新 RFC |
| ❌ 复杂 schema validation (JSON Schema / protobuf) | **不**做 | Go struct + omitempty + 零值兜底足够 |

---

## 4 实施清单（与 multi-project-registry 同步实施）

| # | 改动 | 仓库位置 | 工作量 |
|---|---|---|---|
| 1 | `internal/store/project_registry.go` 新增 schema struct + `Read()` / `Write()` 函数 (智能读取 §2.2) | `internal/store/project_registry.go` (新建) | 1 天 |
| 2 | `cmd/kron/cli/register.go` + `init.go` 调 `Write()` 写**仅**已知字段 | `cmd/kron/cli/` (修改) | 0.5 天 |
| 3 | `cmd/kron/serve-mcp/serve_mcp.go:resolveMCPRoot` 走 §2.1 精简 schema | `cmd/kron/serve-mcp/serve_mcp.go` (修改) | 0.5 天 |
| 4 | GUI 读 / 写 `projects.json` 的项目列表部分 (Wails Go 侧) | `frontend/wails/` (待 Wails 引入) | 0.5 天 |
| 5 | `internal/store/project_registry_test.go` 覆盖 §2.2 智能读取的所有 case (缺字段 / 未知字段 / 字段类型不匹配) | `internal/store/` (新建) | 1 天 |

**总改动 ~3.5 天**. Go 主仓改动约 3 天 (store + cli + serve-mcp), GUI 改动 0.5 天.

**对比撤回版本**: 撤回版 §4 是 5.5 天; 拍板精简版 §4 是 3.5 天. 少 2 天的占位字段实施 (config 命令 + GUI 配置面板).

---

## 5 红线确认

| 红线 | 拍板 |
|---|---|
| 加新 CLI 子命令 (init/add/lint/serve-mcp/serve-lsp 之外) | ❌ **不**加 (撤回版 §4 #4 `kron config` 命令**不**做) |
| 加新 top-level dep (`go.mod` `require` 块) | ❌ 不加 (stdlib `encoding/json` + `os` + `sync/atomic`) |
| 改 `config.toml` schema | ❌ 不改 (架构 §3.5 拍板) |
| 改 frontmatter schema | ❌ 不改 |
| 改 intents/assumptions 数据架构 | ❌ 不改 |
| 改 5 铁律 + 8 import 边界 | ❌ 不改 |
| GUI 进 Kron 主仓 `frontend/` | ⚠️ 仅占位 README (走 [multi-project-registry §1](../rfc/2026-10-13-multi-project-registry.md) — v1.3 骨架期) |
| `internal/` import `cmd/kron/` | ❌ 不让 |
| `cmd/kron/<sub>/` 互调 | ❌ 不让 |

---

## 6 跟现有 RFC 的关系

| 现有 RFC | 关系 |
|---|---|
| [`2026-10-13-multi-project-registry.md`](./2026-10-13-multi-project-registry.md) | **本 RFC 取代其 §2.2**（schema 拍板的最终版）。其他章节（§2.1 路径 / §2.3 register / §2.4 自动注册 / §2.5 GUI 读取 / §2.6 list-projects / §2.7 current_project / §2.8 resolveMCPRoot）**保持** |
| [`2026-10-13-gui-distribution-manual-install.md`](./2026-10-13-gui-distribution-manual-install.md) | **不**改 (分发链路独立) |
| [`2026-10-09-gui-ide-plan.md`](./2026-10-09-gui-ide-plan.md) | **不**改 (GUI 多项目概览是 `projects[]` 渲染, 走本 RFC) |
| [`2026-10-07-lsp-sdk.md`](./2026-10-07-lsp-sdk.md) | **不**改 (LSP SDK 引入**后**才开新 RFC 拍 LSP 配置) |
| [`2026-10-10-frontend-ui-stack.md`](./2026-10-10-frontend-ui-stack.md) | **不**改 (UI 渲染独立) |
| [`docs/abstractDesign/architecture.md` §3.5](../abstractDesign/architecture.md) | **不**改 (`config.toml` 拍板保持) |

---

## 7 跟 `config.toml` 的明确分工

| 场景 | 文件 | 字段 |
|---|---|---|
| 项目内 intents 路径 (单项目) | `.kron/config.toml` (项目级) | `intents_dir` |
| 项目列表 (跨项目) | `projects.json` (Kron 安装目录级) | `projects[]`, `current_project` |
| 跨进程 / 跨客户端配置 (MCP / CLI / LSP / GUI) | **(暂无)** | **未来真实需求出现时单独开 RFC** |
| GUI 用户偏好 (主题 / 默认视图) | GUI 自己的 `localStorage` (客户端层) | (不进 `projects.json`) |

---

## 8 待办（v1.3+ release pipeline 阶段）

> ⚠️ **本节是"最后发布时再做"事项, 不阻塞 GUI 骨架开发.**

- 实施清单 §4 全部 5 项.
- **同步**修订 `multi-project-registry.md` §2.2 → 引用本 RFC §2.1 顶层 schema (**不**重复定义).
