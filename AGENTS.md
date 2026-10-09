# AGENTS.md — Kron Project Conventions

> Guidance for AI coding agents working in this repository.
> Read this before writing or modifying any code.
>
> **权威源**：`docs/` (按 `docs/abstractDesign/docs-map.md` 角色分类).
> AGENTS.md 是**入口**与**红线**; **不**复制 `docs/` 内容. 两处冲突时, `docs/` 胜.

---

## 1 必须先读

按这个顺序读 — **不**读就写代码是**误导自己**:

1. [`docs/article.md`](docs/article.md) — 项目愿景
2. [`docs/abstractDesign/architecture.md`](docs/abstractDesign/architecture.md) §〇 铁律 + §一 访问层 — 不可妥协
3. [`docs/abstractDesign/intent-structure.md`](docs/abstractDesign/intent-structure.md) — 数据格式真理
4. [`docs/how-it-works.md`](docs/how-it-works.md) — 实景示例 (人类入口)
5. 对应流程文档 (`docs/process/`, 见下表)

---

## 2 实施流程 (plan-first, **不**是 code-first)

**plan-first** — 任何代码改动**先**走 `docs/process/` 对应流程, **不**直接动代码.

| 改动类型 | 流程 |
|---|---|
| 新 CLI flag | [`docs/process/cli-flag.md`](docs/process/cli-flag.md) |
| 新 `internal/<name>/` 包 | [`docs/process/internal-pkg.md`](docs/process/internal-pkg.md) |
| 新 `internal/` API | [`docs/process/new-internal-api.md`](docs/process/new-internal-api.md) |
| 新 lint 规则 | [`docs/process/lint-rule.md`](docs/process/lint-rule.md) |
| 新 MCP 工具 | [`docs/process/mcp-tool.md`](docs/process/mcp-tool.md) |
| 改 frontmatter schema | [`docs/process/migrate.md`](docs/process/migrate.md) |
| 加 CI 检查 | [`docs/process/ci-enforcement.md`](docs/process/ci-enforcement.md) |
| assumptions 数据架构 (B 独立文件, v1.3+) | [`docs/rfc/2026-10-08-assumptions-standalone.md`](docs/rfc/2026-10-08-assumptions-standalone.md)（已落地, **不**需要跑迁移脚本） |
| 新增访问层 (serve-lsp / serve-gui / 新协议) | **先开 RFC** [`docs/rfc/`](docs/rfc/) 拍板; 拍板**后**走对应 `docs/process/*` |

> 流程图见 [`docs/process/README.md`](docs/process/README.md) 文档地图.

---

## 3 红线 (off-limits, **不**走流程**就**不**做**)

任何**未**经显式批准**不**做:

- **加**新 top-level dep (`go.mod` `require` 块)
- **改** `go.mod` Go version directive
- **改** package 路径 / 改 package 名
- `go generate` / reflection — 需 `// why` 注释 + 人类 review
- 强推 (force-push) / 改 shared branch 历史
- **加**新 CLI 子命令 (架构 §1.1 只允许 `init` / `add` / `lint` / `serve-mcp`; `serve-lsp` / `serve-gui` v1.3+ 走 RFC)
- **加** `config.toml` 字段 (零配置, 架构 §3.5)
- 改 **frontmatter schema** (走 `migrate.md`)
- 改 **intents/ assumptions 数据架构** (走 `migrate.md` + RFC)
- 新 **lint 规则** (走 `lint-rule.md`)
- 新 **`internal/` 包** (走 `internal-pkg.md`)
- **不**改 5 铁律 + 8 import 边界 (架构 §〇 + §二.2; 违反 = 架构违规, 拒绝 review)
- **不**让 access layer (CLI / MCP / LSP) 互调 (架构 §〇 铁律 #2)
- **不**让 `internal/` import `cmd/kron/` (架构 §二.2)
- **不**让 `cmd/kron/<sub>/` import 另一个 `cmd/kron/<other>/` (架构 §二.2)

---

## 4 提交前必跑

```
go vet ./...
gofmt -l .
go test ./...
```

PR 含: 测试 + `docs/` 同步 (真理改 → `abstractDesign/`; 流程改 → `process/`; 实施改 → `implementation/`).

---

## 5 当不确定

- **不**猜 — **问**人
- **不**双份维护 — `docs/` 是真理, AGENTS.md **不**复制
- **不**在 AGENTS.md 加**新**流程 / **新**红线 — 直接走 `docs/process/` 开 PR 改
- **人命 > 文档实施计划** — 任何 "可以以后做" / "建议 v1.5 评估" / "RFC 拍板后再说" 的实施提案, **先**问人**是否**真的**要**纳入; 默认**不**主动加, 默认**不**主动提. 文档实施计划是**辅助**, **不**是任务清单. 人类说"以后再考虑" = 删, 不是留 todo.
- **慎给阶段性实施提案** — 任何"v1.3+ 拍板 / v1.4+ 评估 / v1.5 收口"等分阶段实施提案**显著拉高**实施成本: (a) 制造跟踪债 (b) 给后续 PR 强加"该实现 X" 的隐性承诺 (c) 与"plan-first, **不**是 stage-first" 流程纪律冲突. 拍板时**只**说"现在做不做" + "做哪个", **不**说"分 3 阶段" / "先骨架后填充". 例外: 架构 §7 已有 "不做的事" 表, 那**不**算提案, **不**重写.

---

## 6 删掉的段 (避免双份误导)

下面段**曾**在 AGENTS.md, **不**再重复 — 查 `docs/`:

| AGENTS.md 旧段 | 查 |
|---|---|
| Storage format reminder (frontmatter 例子) | [`intent-structure.md`](docs/abstractDesign/intent-structure.md) |
| Tech stack table (MCP / LSP / Wails SDK 选型) | [`docs/rfc/*-sdk.md`](docs/rfc/) (各 RFC 拍板) |
| Architecture iron rules #1-8 摘要 | [`architecture.md`](docs/abstractDesign/architecture.md) §〇 |
| Things off-limits 14 条摘要 | [`architecture.md`](docs/abstractDesign/architecture.md) §〇 + [`docs/process/*.md`](docs/process/) |
| Access layers (CLI / MCP / LSP) 枚举 | [`architecture.md`](docs/abstractDesign/architecture.md) §一 + [`2026-10-08-lsp-client.md`](docs/rfc/2026-10-08-lsp-client.md) |
| Coding standards (no any / errors wrap / ...) | `go-style` / `ts-style` skill (`.cursor/skills/`) |
| Workflow (Before writing code 5 步) | `docs/process/README.md` 文档地图 |

> **2026-10-08 改**: 旧版"Adding `serve-lsp` 必走 `docs/process/new-access-layer.md`" 引用 — **该文件不存在**; 新指令是"**先开 RFC** [`docs/r/`](docs/rfc/) 拍板, 拍板后走对应 `docs/process/*`". 见本文档 §2 表格最后一行.

---

## 7 客户端层 (Client Layer) 分工 (2026-10-09)

按 [`architecture.md` §〇·五·1 + §0 铁律 #9](../docs/abstractDesign/architecture.md), 客户端层 (VSCode 扩展 / Wails GUI / Cursor / 其他 IDE / Web) **不**进 Kron 主仓, **只**通过协议访问层 (CLI / MCP / LSP) 调能力. 客户端层**之间**也**不**互相 import.

### 7.1 职责分工 (人 2026-10-09 拍板)

| 客户端 | 形态 | 职责范围 | 调的能力 |
|---|---|---|---|
| **GUI (Wails 桌面 / Web)** | 整体项目管理软件 | **只**做**整体预览**: 多项目概览 + 意图树预览 + 状态条; **不**做编辑/搜索/跳转/补全等**深度功能** | `serve-mcp` 拼 JSON (走 list / get / read 类工具) |
| **IDE (VSCode 扩展 / Cursor 兼容层)** | 编辑器侧 UI | **功能最全**: hover / definition / 侧边栏 / 弹窗 / 跳转 / 补全 / 错误诊断; 编辑器生命周期内的所有 kron 能力**全**在 IDE 这层聚合 | `serve-mcp` 拼 JSON (业务能力) + `serve-lsp` 子进程 (编辑器 UI 集成) |

**关键边界**:

- GUI **不**做 hover / 跳转 / 实时诊断 (那是 IDE 职责)
- IDE **不**做项目管理 (那是 GUI 职责)
- 同一个项目可以**同时**有 GUI 实例 (Wails) + IDE 实例 (VSCode) + AI 工具实例 (Claude MCP), **各自**调**各自**需要的协议访问层子进程, **互不**通讯, 互不锁

### 7.2 实施位置 (客户端层**进**主仓 `frontend/`)

| 客户端 | 仓库 | 与 Kron Go 主仓关系 |
|---|---|---|
| VSCode 扩展 | `frontend/extensions/vscode/` 或类似 (Kron 主仓) | **进**主仓 `frontend/`；通过 stdio 子进程调 `kron serve-mcp` + `kron serve-lsp` |
| Wails GUI | `frontend/wails/` 或类似 (Kron 主仓) | **进**主仓 `frontend/`；进程内 Go ↔ TS 桥，拼 JSON 调 `kron serve-mcp` |
| Cursor 客户端 | 复用 VSCode 扩展 | Cursor **不**需要单独扩展；复用同一套代码 |
| Neovim / Helix | 各编辑器生态**自**写 | **不**下沉到 Kron 仓库；各自实现 LSP client (lspconfig / 内置 LSP) |

### 7.3 客户端 SDK 选型 (拍板约束)

- **VSCode 扩展**: 用 `vscode-languageclient` (TypeScript, 事实标准) 处理 LSP; MCP 客户端**用 SDK** (`@modelcontextprotocol/sdk` 或 VSCode 1.85+ 内置 MCP 客户端) **不**手写 JSON-RPC. UI 渲染走 VSCode Webview API + React + shadcn/ui.
- **Wails GUI**: Wails v2 + Go ↔ TS 桥 (React + shadcn/ui); 拼 JSON 调 `kron serve-mcp`.
- **图表**: VSCode webview 内走 shadcn 原生 `<Chart>` (v0.14+) 或 spike 验证后按需扩展.
- **LSP server SDK 锁定**: `go.lsp.dev/protocol v3.17+` (RFC `2026-10-07-lsp-sdk.md` §3). 加 Go dep 走 explicit approval (AGENTS.md §3 红线).

### 7.4 客户端层代码约束

- ⚠️ **不**在 `frontend/` 根目录放代码（各客户端在 `frontend/` 下各自子目录，如 `frontend/vscode/` / `frontend/wails/`）

具体实施路径走 [`docs/process/new-access-layer.md`](../process/new-access-layer.md) §3 + 客户端 RFC 拍板。
