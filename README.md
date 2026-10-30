# Kron

> 基于 Git 的 AI 辅助开发**意图管理**系统。

Kron 把代码设计意图（Design Intent）存为**纯 Markdown 文件，放在你的仓库里**——无需 API 服务、无需守护进程、无需私有格式。AI 编程助手可以直接读写它们。一切差异就是 `git diff`。

> **最新发布：v0.3.0** — Windows 便携包 + VS Code 扩展：[GitHub Releases](https://github.com/zhangjun005/Kron/releases/tag/v0.3.0)

---

## 快速开始

1. 安装 v0.3.0 [Windows 便携包 / VSIX](https://github.com/zhangjun005/Kron/releases/tag/v0.3.0)
2. `kron init -CWD <project>`
3. `kron add auth/login -CWD <project>`
4. 在代码里加 `// @kron:intent auth/login`
5. `kron lint -CWD <project>`

完整安装、VS Code 扩展 / AI Agent MCP 配置与常见问题见 **[快速上手教程](docs/getting-started.md)**。

## 痛点

AI 编程助手在清楚"为什么这么做、权衡了什么、放弃了什么"时表现最好。但现状是：

- 设计决策只存在于会议纪要、Slack 消息流或某人的脑子里；六个月后没人记得当时为什么拒绝方案 B。
- 代码里的 `// FIXME` / `// TODO` 只能描述"要做什么"，无法承载"为什么这样做"。
- 接手者（包括下一个 AI Agent）只能对着代码反推意图，推错了就埋雷。

长期下来就产生了 **Intent Debt**：意图丢失、假设漂移、权衡失效。

## Kron 解决了什么

1. **决策写完即遗忘** — 写在 Notion / Slack / 脑里,半年后没人记得为何拒绝方案 B。  
   → Kron 把决策落为 `.kron/intents/*.md`,跟代码同 PR 同 review,`git diff` 直接看。
2. **假设沉到代码里找不到** — 写代码时的前提(DAU ≤ 10K、Redis 99.9%)只在脑子里。  
   → frontmatter 的 `assumptions[]` 把假设显式化,带 per-intent `severity: hard | soft` 与 `rationale`;共享 `text` 存于 `.kron/assumptions/<id>.md` 独立文件(B-3 架构)。
3. **改代码不知道影响范围** — 改一个 API,下游多少文件多少意图跟着炸。  
   → MCP `kron_impact` 反向查"改这个意图牵连哪些代码 + 哪些下游意图"。
4. **过期假设无人察觉** — "Redis 可用性 ≥ 99.9%" 已经过了一年,没人复审。  
   → MCP `kron_stale` 自动列出过期意图 + 已过 `expires_at` 的假设,CI 可拦门。

第 2、3、4 项是 Kron 跟 `// FIXME`、ADR、Notion、Cursor `codebase.md` 的实质差别——这些替代品都没有 `severity` 字段,也没有被 AI 主动消费的查询接口。

## 意图之间的关系

Kron 用**文件系统 + Markdown + 轻量 frontmatter** 表达关系，不引入数据库：

- **目录 = 层级归属**。`.kron/intents/auth/refresh-token.md` 自动属于 `auth/` 模块；`.kron/intents/auth/README.md` 是该模块的总览。
- **相对链接 = 人类阅读跳转**。正文里的 `[令牌桶](../rate-limit/token-bucket.md)` 在普通编辑器里 `Ctrl+单击` 即可跳转。
- **frontmatter = 机器可消费依赖**。`references` 表示软引用，`depends_on` 表示硬依赖；`kron_impact` / `kron_delete` / `kron lint` 都基于这些字段计算反向影响与依赖完整性。
- **代码锚点 = 代码到意图的反向引用**。源码里写 `// @kron:intent storage/storage-format`，lint / impact / IDE 都能识别。

完整规范见 [`docs/abstractDesign/intent-structure.md`](./docs/abstractDesign/intent-structure.md) §二、§四。

## Kron 是什么

一个把意图沉淀为**仓库内 Markdown 文件**的系统：

```
.kron/
├── config.toml                # Kron 配置（可选）
├── anchor-scan.json           # 锚点扫描：语言 → 扩展名 / 注释语法（可选）
├── templates/
│   └── intent-body.md         # 新 intent 正文模板（注入 AI 提示词）
├── assumptions/               # 共享假设 registry：每条假设一个 .md
└── intents/                   # 目录 = 模块分组
    ├── README.md              # 全部意图的总览
    ├── storage/
    │   ├── README.md          # storage 模块的总览
    │   ├── storage-format.md  # 单条意图 = 单个 .md
    │   └── id-scheme.md
    └── auth/
        ├── README.md
        └── refresh-token.md
```

每个意图文件是一份 Markdown 文档，带一个轻量的 YAML frontmatter：

```markdown
<!-- kron:frontmatter -->
symbol:
  - "store.ParseFrontmatter"
created_by: "@zhangjun005"
updated_at: "2026-09-19T22:30:00Z"
<!-- 可选：边界假设结构化（B-3, 详见 docs/abstractDesign/intent-structure.md §三）
     text 共享在 .kron/assumptions/<id>.md；intent 仅引用 id + 本意图 severity + rationale
assumptions:
  - id: small-repo
    severity: hard
    rationale: "仓库规模在个人/小团队级别, 假设破裂需重构"
-->
<!-- /kron:frontmatter -->

# 存储格式选型

> 选 Markdown + YAML frontmatter，零依赖、零迁移成本。

## 为什么
存储需要同时被人和 AI 编辑。选 Markdown + YAML frontmatter 的理由：
- 人类可直接 `git diff` 阅读，无需学新工具
- AI 可直接 prompt-context 读取，无需解析二进制

## 权衡
- **放弃了**：原生 SQLite 索引查询能力 —— 换来了零依赖、零迁移成本
- **代价**：大规模条目下需要全文搜索，不适合 >1万条 的仓库
- **依赖**：条目命名沿用 [ID 方案](id-scheme.md)，保证排序稳定
```

> 两点说明：
> - `assumptions` 写在 intent frontmatter（`[{id, severity, rationale, ...}]` 引用形式,B-3），共享 `text` 存于 `.kron/assumptions/<id>.md` 独立文件（不在正文重复, 见 [`intent-structure.md` §三](./docs/abstractDesign/intent-structure.md#三意图文件内容模板)）。
> - 末尾的 `[ID 方案](id-scheme.md)` 就是一条意图引用——同一 `storage/` 目录下的横向依赖，普通编辑器 `Ctrl+单击` 可跳转。

代码侧用极轻量锚点反向引用：

```go
// @kron:intent storage/storage-format
func ParseFrontmatter(raw []byte) (map[string]any, string, error) { ... }
```

就这样。**AI 读的文件和你读的一样。**

## Kron 不做什么

- 无后台守护进程、无文件监听、无同步引擎。
- 无双源持久化、无 mtime+hash 状态机冲突处理。
- 无私有格式。Markdown 本身就是 API。
- **不是**任务追踪器。任务只是意图落地时的派生钩子，不是独立实体。
- 无 Electron 应用。桌面 GUI 使用 Wails + WebView2；编辑器侧使用 VS Code 扩展 + LSP，不是浏览器壳。

只要贡献者或 AI 能自信地编辑 Markdown 文件，那文件就是真相。

## 访问层与客户端

Kron 的能力只通过**协议访问层**暴露，客户端不直接依赖内部实现：

```
AI Agent ── MCP (stdio)
编辑器   ── LSP (stdio) + VS Code 扩展
人 / CI  ── CLI
桌面用户 ── Wails GUI ── MCP
```

CLI / MCP / LSP 互不调用；VS Code 扩展与 Wails GUI 通过各自协议访问 Kron，客户端之间也不互相 import（[`docs/abstractDesign/architecture.md`](./docs/abstractDesign/architecture.md) §〇）。

### CLI —— 给人 / CI

| 命令 | 用途 |
|---|---|
| `kron init` | 初始化 `.kron/`（intents、template、anchor-scan 配置） |
| `kron add <slug>` | 脚手架一个新意图文件 |
| `kron register` | 把当前项目登记进 `projects.json`（GUI/多项目用） |
| `kron lint` | 校验锚点 + frontmatter（CI gate） |
| `kron serve-mcp` | 启动 MCP server |
| `kron serve-lsp` | 启动 LSP server |

查询 / 修改类操作主要走 MCP；CLI 刻意保持小而稳定。详情见 [`docs/implementation/cli.md`](./docs/implementation/cli.md)。

### MCP —— 给 AI Agent

AI Agent 通过 stdio JSON-RPC 调用 MCP 工具（当前 **18 个**）：

- **意图生命周期**：`kron_init` / `kron_add` / `kron_get` / `kron_update` / `kron_delete`（软删）/ `kron_restore` / `kron_list` / `kron_tree` / `kron_lint`
- **假设注册表**：`kron_assume_check` / `kron_assume_get` / `kron_assume_list` / `kron_assume_create` / `kron_assume_update` / `kron_assume_delete`
- **影响与健康度**：`kron_impact` / `kron_intent_density` / `kron_stale`

此外提供 `kron://intents/{slug}` resource 和 `kron-implement` / `kron-drift-check` / `kron-review` prompts。项目可在 `.kron/templates/intent-body.md` 定义正文模板，它会被注入 initialize instructions、`kron_add` / `kron_update` 描述和 `kron-implement` prompt，用来约束 AI 写 intent 的结构。详情见 [`docs/mcp/kron-implementation.md`](./docs/mcp/kron-implementation.md)。

### LSP —— 给编辑器

`kron serve-lsp` 提供代码锚点的 hover 与 go-to-definition（`// @kron:intent <slug>` → 对应 intent Markdown）。扫描范围由项目 `.kron/anchor-scan.json` 按语言 / 扩展名 / include-exclude 配置；默认支持 Go、TS/TSX、JS/JSX、Vue、Svelte、Python、Rust、Java、C/C++、C#、Kotlin、Swift、Ruby、PHP、Shell、YAML、Markdown 等。

### VS Code 扩展 —— 编辑器内工作流

从 Releases 安装 `kron-vscode-0.3.0.vsix` 后可用：

- 意图树侧边栏（含 Assumptions / Superseded 视图）、Webview 画布增删改查
- 代码锚点：hover、Ctrl+click 跳转、右键创建/改写锚点、intent → code anchor 跳转
- 项目配置驱动的 anchor 扫描（`.kron/anchor-scan.json`）
- 唯一的完整 IDE 侧能力入口；Cursor 复用同一扩展。

### GUI —— Wails 桌面概览

`Kron-GUI.exe` 是 Wails + WebView2 的**只读**概览：多项目画廊、封面、意图树预览、Settings。写操作留给编辑器与 AI。

## 状态

- **最新发布**：v0.3.0（2026-10-30）— [GitHub Release](https://github.com/zhangjun005/Kron/releases/tag/v0.3.0)
- **分支模型**：`develop` 是默认集成分支；`master` 是发布分支，只接收 release PR / hotfix，发布时打 tag。
- **验证环境**：`kron-mcp-test/` 是可随时重建的 release-shaped 测试环境；产物不入库。
- 各访问层实现进度以 `cmd/` 与 `internal/` 目录为准；设计真理以 `docs/` 为准。

## 参与贡献

格式和 CLI 设计决策在 [`docs/abstractDesign/`](./docs/abstractDesign/)。提 PR 前先读这些——格式是 Kron 赖以生存的东西，改动会直接影响到用户，因为他们拥有这些文件。

## License

MIT.
