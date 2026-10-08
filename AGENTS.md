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
