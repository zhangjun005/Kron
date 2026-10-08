# AGENTS.md — Kron Project Conventions

> Guidance for AI coding agents working in this repository.
> Read this before writing or modifying any code.
>
> **Architecture authority**: [`docs/abstractDesign/architecture.md`](docs/abstractDesign/architecture.md) is the source of truth for architectural decisions. This file is a working-conventions mirror; when they disagree, architecture.md wins.

## What this project is

Kron is a Git-native intent management system for AI-assisted development. It stores design intents as Markdown files inside the repository so AI agents and humans can read/write them directly via `git diff`. See [README.md](README.md) for the full vision.

**Current phase**: Go rewrite (clean slate — Rust/Tauri legacy is in the archive branch). Single binary CLI, no GUI yet.

## Tech stack (locked in)

| Component | Choice | Notes |
|-----------|--------|-------|
| Language (core) | Go 1.27 | See `go-style` skill |
| CLI framework | cobra | `cmd/kron/main.go` is thin wiring only |
| Config format | TOML | `config.toml`, parsed via `BurntSushi/toml` |
| Frontmatter | YAML | `gopkg.in/yaml.v3` |
| Tests | `testing` + `testify/assert` | standard library + assertions |
| MCP SDK | `github.com/modelcontextprotocol/go-sdk` | locked in `go.mod` since v1.2 |
| LSP SDK | `go.lsp.dev/protocol` | **v1.3+ pending RFC** — see [`docs/rfc/2026-10-07-lsp-sdk.md`](docs/rfc/2026-10-07-lsp-sdk.md) |
| GUI (Wails) backend | **Go Wails v2** (WebView2 / WebKit) | **v1.3 main path** — see [`docs/rfc/2026-10-07-gui-stack.md`](docs/rfc/2026-10-07-gui-stack.md) §1.1 |
| VSCode extension (副路径) | TS + VSCode Extension API | **v1.4+ pending** — 复用 `kron serve-mcp` stdio |
| GUI (Wails) frontend | **TypeScript + shadcn/ui** (React) | **v1.3+**; v1 / v1.2 阶段 **不**引入 .ts 代码, 仍只走 Go |
| Future skill | strict (ready-to-load) | 已有 `ts-style` skill 等 v1.3 启用 |

> **v1 / v1.2 阶段**: 仓库**仍只**有 Go 代码 — TS 前端 / Wails 后端 / VSCode 扩展都在 v1.3+ 引入.
> The `.gitignore` already excludes `frontend/` — that's intentional, not stale. (v1.3 启动**时**改: 取消整目录忽略, 改**只**忽略 `frontend/{dist,build/bin,node_modules}/`.)

## Repository layout (Go)

```
kron/
├── cmd/
│   └── kron/
│       ├── main.go        ← CLI entry, thin wiring only
│       ├── cli/           ← cobra commands (kron init / add / lint)
│       ├── serve-mcp/     ← MCP stdio server (v1.2+)
│       ├── serve-lsp/     ← (planned v1.3+) LSP socket server
│       └── serve-gui/     ← (planned v1.3+) Wails HTTP/WebSocket server
├── internal/
│   ├── model/             ← domain types (Intent, Config, Frontmatter, Anchor)
│   ├── store/             ← file I/O, frontmatter parsing, MoveToTrash / RestoreFromTrash
│   ├── parser/            ← markdown / CLI argument / anchor scanning / slug validation
│   ├── relations/         ← reverse-link graph (Prereqs / Dependents / ReverseLinks)
│   ├── lint/              ← anchor-dangling + frontmatter + ref/cycle + staleness
│   └── identity/          ← created_by resolution (GitUser / Handle)
├── frontend/              ← (planned v1.3+) Wails GUI frontend (TS + shadcn/ui) — .gitignore'd in v1 / v1.2
├── docs/
│   ├── article.md
│   ├── business.md
│   ├── requirements.md
│   ├── abstractDesign/
│   │   ├── intent-structure.md
│   │   ├── architecture.md   ← ARCHITECTURE TRUTH
│   │   └── docs-map.md       ← 文档关系图谱
│   ├── how-it-works.md       ← 实景示例（人类入口）
│   ├── implementation/        ← 实施建议（API signatures, tool contracts, flows）
│   │   ├── README.md
│   │   ├── api-surface.md
│   │   ├── cli.md
│   │   ├── domain-model.md
│   │   ├── error-catalog.md
│   │   ├── ide-interaction.md
│   │   ├── mcp.md
│   │   └── testing.md
│   └── process/              ← 实施流程（decision trees, checklists）
│       ├── README.md
│       ├── ci-enforcement.md
│       ├── cli-flag.md
│       ├── internal-pkg.md
│       ├── lint-rule.md
│       └── migrate.md
├── .cursor/
│   ├── skills/            ← agent skills (loaded by Cursor)
│   └── rules/             ← Cursor rule format (.mdc)
├── go.mod
├── go.sum
├── README.md
└── AGENTS.md              ← this file
```

> 不在 v1 必需范围：IDE plugin / LSP server / GUI client / hard-delete / GC——目录与代码尚未在仓库出现。

## Coding standards

Detailed rules live as Cursor skills — consult them before writing code:

- **Go**: see [`.cursor/skills/go-style/SKILL.md`](.cursor/skills/go-style/SKILL.md) and [examples.md](.cursor/skills/go-style/examples.md)
- **TypeScript** (when UI phase starts): see [`.cursor/skills/ts-style/SKILL.md`](.cursor/skills/ts-style/SKILL.md)

These are non-negotiable. The rules cover:

- **Type safety**: no `any` / `interface{}` / non-null `!` / `==` in code we own
- **Errors**: always wrap with context, use `errors.Is`/`errors.As`, never discard
- **Naming**: package lowercase, mixed-case acronyms (HTTP, JSON), no `I` prefix on interfaces
- **Project layout**: business logic in `internal/`, `cmd/` is wiring only
- **Documentation**: godoc/JSDoc on every exported identifier, package comment on every package
- **Commits**: `<scope>: <imperative summary>` (≤72 chars), body ≤3 lines explaining *why* not *what*

## Architecture iron rules

> Source: [`docs/abstractDesign/architecture.md`](docs/abstractDesign/architecture.md) §〇. Full text there.

These constraints come from the architecture doc and are non-negotiable in v1:

1. **Business logic only in `internal/`** — `cmd/kron/` is wiring only.
2. **Core decoupled from access layer** — `internal/store`, `internal/parser`, `internal/model` are pure libraries; they do not know whether the caller is CLI, MCP, LSP, IDE, or GUI.
3. **No cross-access-layer calls** — CLI / MCP / LSP three protocol access layers must NEVER import or call each other. Any capability that needs to be shared across access layers must be **sunk into `internal/`** and called independently by each access layer. Code where one access layer invokes another is an architectural violation.
   > **变更 (2026-10-08)**: 协议访问层**枚举**为 CLI / MCP / LSP 三种（**不**含 IDE / GUI）。IDE / GUI 是**客户端层**（外部），只通过协议访问层**之一**调能力，**不**直接 import `internal/`。详见 [`docs/abstractDesign/architecture.md`](docs/abstractDesign/architecture.md) §〇 铁律 #9 + §一。
4. **Caller identity flows via `context.Context`** — every store/parser function takes `ctx context.Context` as the first parameter; the access layer injects the caller key (`"cli"` / `"mcp"` / `"lsp"` / `"ide"` / `"gui"`).
   > **变更 (2026-10-08)**: caller 注入 API **不再推荐**用于新访问层代码。`model.WithCaller` / `CallerFrom` 等函数 **保留**（兼容既有 import + `cmd/kron/serve-mcp` 的 `callerInjectMiddleware`），但**新**访问层代码**不**再调用 `model.WithCaller`；`internal/` **不**依赖 ctx 上的 caller key 做行为分支。原因：`context.Context` 注入身份属性**不便于开发**（调试栈不直观 / 单元测试需额外包装 / 类型安全弱）。详见 [`internal/model/caller.go`](internal/model/caller.go) 头注释。`ctx` 仍保留在 store / parser 函数签名中**用于取消 / 超时传播**。
5. **Zero new dependencies** — any new top-level dep needs explicit approval (see off-limits below).
6. **Markdown + YAML frontmatter is the data truth** — binaries hold no project data; no state machines, no dual-source sync.
7. **CI lint is the only enforceable gate** — no runtime implicit state; strong constraints are expressed as `kron lint` errors in CI, not runtime defaults.
8. **One-way dependency direction** — `cmd → cli → {parser, store} → model`; no cycles; `model` has zero external imports.

### Import boundary rules (§2.2 of architecture.md, mechanically checkable)

- `cmd/kron/**` may only import `cmd/kron/**` itself + `internal/**` + stdlib + approved deps
- `internal/**` may only import `internal/**` itself + stdlib + approved deps; **never** `cmd/kron/**`
- Any `cmd/kron/<sub>/` may only import itself + `internal/**`; **never** another `cmd/kron/<other>/`

> Example violation to refuse in code review: `cmd/kron/serve-ide/main.go` importing
> `cmd/kron/serve-lsp/...`. If IDE and LSP need shared logic, lift it into `internal/`.
> **(2026-10-08 变更)** `cmd/kron/serve-ide/` 不再存在——IDE 是**客户端层**，不在 Kron 主仓。

## Access layers (CLI / MCP / LSP) + Client layer (IDE / GUI)

> Source: [`docs/abstractDesign/architecture.md`](docs/abstractDesign/architecture.md) §〇 铁律 #9 + §一.
> **(2026-10-08 变更)** 原五访问层 (CLI / MCP / LSP / IDE / GUI) 改为 **3 协议访问层 + 客户端层**: 协议访问层 (CLI / MCP / LSP) 在 Kron 主仓；客户端层 (VSCode 扩展 / Cursor / Wails / Web GUI) **不在**主仓，独立项目。

Kron exposes the same `.md` data through protocol access layers; client-layer UI consumers reach the server via one of the protocol layers:

**协议访问层 (3 个, Kron 主仓)**:

- **CLI** (human / CI): `kron init` / `kron add` / `kron lint` / `kron serve-mcp` / `kron serve-lsp` (v1.3+) — minimal subset.
- **MCP server** (AI Agent): stdio JSON-RPC, 12 tools covering init / add / list / get / update / delete / restore / lint / assume_check / impact / intent_density / stale.
- **LSP server** (editor-side): hover + definition over `// @kron:intent` anchors. **v1.3+** — protocol选型见 [`docs/rfc/2026-10-07-lsp-sdk.md`](docs/rfc/2026-10-07-lsp-sdk.md)（**SUPERSEDED, 已标——待重写**）.

**客户端层 (外部项目, 不在 Kron 主仓)**:

- **VSCode extension / Cursor / 其他 VSCode 兼容 IDE**: 拼 JSON 走 serve-mcp 子进程 + spawn serve-lsp 子进程拿 hover/go-def; 一个项目 = 一个 MCP 实例（与 AI 工具**共用**）.
- **Wails (多项目概览 + 意图树预览)**: 进程内 Go ↔ TS 桥 → 走 serve-mcp (与 VSCode 扩展共用) — **轻**, 不做完整单项目 GUI.
- **Web GUI (TBD)**: 同 Wails 模式 (拼 serve-mcp).

> **关键原则**: 客户端层**不**直接 import `internal/`; 客户端层**不** import Kron 主仓 Go 代码 (VSCode 扩展**不** import `serve-mcp` Go 包, 只通过 stdio JSON-RPC 通信); 客户端层之间 (Wails / VSCode 扩展) 互相**不** import.

CLI explicitly does NOT implement `list` / `get` / `update` / `delete` / `restore` — those live in MCP only. Don't add them as CLI subcommands without consulting architecture.md §一.

## Workflow

### Before writing code

1. Read `docs/article.md` to understand the project vision.
2. Check `docs/requirements.md` for the current phase's requirements.
3. Find the relevant doc:
   - **"How does Kron actually work in a real repo?"** → `docs/how-it-works.md`（实景入口）
   - **"How do all docs relate to each other?"** → `docs/abstractDesign/docs-map.md`
   - **"Why is it designed this way?"** → `docs/abstractDesign/architecture.md`
   - **"What is an intent .md file supposed to look like?"** → `docs/abstractDesign/intent-structure.md`
   - **"What is the function signature / tool contract?"** → `docs/implementation/api-surface.md` or `docs/implementation/mcp.md`
   - **"How do I add a CLI flag / lint rule / internal package?"** → `docs/process/`
4. Review the `internal/` package(s) you'll touch to learn existing conventions.
5. Confirm the relevant skill (`go-style` or `ts-style`) is loaded.

### When proposing changes

- Prefer small, reviewable PRs over mega-commits.
- Include tests for any new exported function in the same commit.
- If adding a dependency, justify it in the commit body.
- Run `go vet ./...`, `gofmt -l .`, and `go test ./...` before committing.
- For changes that have a process doc (new CLI flag, new lint rule, new `internal/` package, frontmatter schema change), follow the corresponding `docs/process/*.md` checklist before opening a PR.

### When in doubt

- Default to the more conservative / explicit choice (e.g., concrete types over `any`).
- Ask the human rather than guess. Especially for: dependency choices, schema changes, public CLI flag surface.

## Things that are off-limits without explicit ask

- Adding new top-level dependencies (anything in `require` block of `go.mod`).
- Modifying `go.mod`'s Go version directive.
- Renaming or moving packages.
- Generating code via reflection or `go generate` — these need a // why comment and human review.
- Force-pushing, rewriting history on shared branches.
- Adding CLI subcommands beyond `init` / `add` / `lint` / `serve-mcp` — see architecture.md §1.1.
- Adding config fields beyond `intents_dir` — see architecture.md §3.5 (zero-config).
- Adding `--path` / stdin / external-template flags to existing commands without consulting `docs/process/cli-flag.md`.
- Adding **`serve-lsp`** / **`serve-gui`** subcommand / new access-layer package — 必走 [`docs/process/new-access-layer.md`](docs/process/new-access-layer.md)（v1.3+ 解锁, **不**在 v1 必需范围）.
- Implementing **IDE plugin** / **hard-delete / GC** — 仍不在 v1 必需范围.
- Adding new lint rules without following `docs/process/lint-rule.md`.
- Modifying frontmatter schema without following `docs/process/migrate.md`.
- Adding new `internal/` packages without following `docs/process/internal-pkg.md`.
- Adding new top-level dependencies to `go.mod` `require` block (Wails / `go.lsp.dev/protocol` / shadcn 都要走 explicit approval).

## Storage format reminder

`.kron/intents/*.md` files use this frontmatter shape (don't change without migration plan):

```markdown
<!-- kron:frontmatter -->
symbol: "auth.RefreshToken"
created_by: "@zhangjun005"
updated_at: "2026-09-22T10:00:00Z"
# optional for multi-collaborator projects:
# reviewers: ["@alice", "@bob"]
# optional: structured assumptions (see docs/abstractDesign/intent-structure.md)
# assumptions:
#   - id: single-region
#     text: 服务仅部署在单 region，无跨区时钟漂移问题
#     severity: hard
#     expires_at: "2026-12-31"
# optional: structural relations (v1.2+; see docs/rfc/2026-10-03-frontmatter-references.md)
# references:                          # soft links (see-also / recommended reading)
#   - oauth2-best-practices
# depends_on:                          # hard dependencies (must-read-before)
#   - auth/token-storage
<!-- /kron:frontmatter -->

# Intent title

> One-line summary.

## Why
...

## Trade-offs
...
```

> **边界假设写在 frontmatter 的 `assumptions` 字段里**（见 [`docs/abstractDesign/intent-structure.md`](docs/abstractDesign/intent-structure.md) §三），不在正文重复。
> **意图间引用/依赖写在 `references` / `depends_on` 字段里**（v1.2+），不在正文 markdown 链接——结构化字段是机器可消费的（`kron_lint` 校验、`kron_impact` 报告、`kron_delete` 提示 dependents）。markdown 链接仍可用于人类阅读。

Status lifecycle (managed via `status` field when needed):
- `draft` → `active` → `superseded`

Changing this schema breaks every existing user. Coordinate before changing.

## Soft-delete reminder

`kron_delete` (MCP only) does **not** erase files. It moves `.kron/intents/<slug>.md` to `.kron/.trash/<slug>.md`. `kron_restore` moves it back. Hard-delete / GC of `.kron/.trash/` is intentionally **not** in v1 — users can `git rm` it manually. See architecture.md §5.2.1.
