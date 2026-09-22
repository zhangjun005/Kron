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
| Future UI | Tauri 2 (Rust) or pure web (TS) | undecided; will document when phase starts |

The `.gitignore` already excludes `frontend/` — that's intentional, not stale.

## Repository layout (Go)

```
kron/
├── cmd/
│   └── kron/              ← CLI entry, thin wiring only
├── internal/
│   ├── model/             ← domain types (Intent, Config)
│   ├── store/             ← file I/O, frontmatter parsing
│   ├── parser/            ← markdown/CLI argument parsing
│   └── cli/               ← cobra command implementations
├── docs/                  ← vision & design abstractions
│   ├── article.md
│   ├── business.md
│   ├── requirements.md
│   └── abstractDesign/
│       ├── intent-structure.md
│       ├── tech-stack.md
│       └── architecture.md   ← ARCHITECTURE TRUTH (mirrored here)
├── .cursor/
│   ├── skills/            ← agent skills (loaded by Cursor)
│   └── rules/             ← Cursor rule format (.mdc)
├── go.mod
├── go.sum
├── README.md
└── AGENTS.md              ← this file
```

> **Phase 2 (not in v1)**: IDE plugin, LSP server, GUI client, hard-delete / GC.

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
- **Commits**: `<scope>: <imperative summary>`, body explains *why* not *what*

## Architecture iron rules

> Source: [`docs/abstractDesign/architecture.md`](docs/abstractDesign/architecture.md) §〇. Full text there.

These constraints come from the architecture doc and are non-negotiable in v1:

1. **Business logic only in `internal/`** — `cmd/kron/` is wiring only.
2. **Core decoupled from access layer** — `internal/store`, `internal/parser`, `internal/model` are pure libraries; they do not know whether the caller is CLI, MCP, LSP, IDE, or GUI.
3. **No cross-access-layer calls** — CLI / MCP / LSP / IDE / GUI five access layers must NEVER import or call each other. Any capability that needs to be shared across access layers must be **sunk into `internal/`** and called independently by each access layer. Code where one access layer invokes another is an architectural violation.
4. **Caller identity flows via `context.Context`** — every store/parser function takes `ctx context.Context` as the first parameter; the access layer injects the caller key (`"cli"` / `"mcp"` / `"lsp"` / `"ide"` / `"gui"`).
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

## Access layers (CLI / MCP / LSP / IDE / GUI)

> Source: [`docs/abstractDesign/architecture.md`](docs/abstractDesign/architecture.md) §一.

Kron exposes the same `.md` data through five entry points:

- **CLI** (human / CI): `kron init` / `kron add` / `kron lint` / `kron serve-mcp` — minimal subset.
- **MCP server** (AI Agent): stdio JSON-RPC, 8 tools covering init / add / list / get / update / delete / restore / lint.
- **LSP server** (Phase 2): editor-side hover + definition.
- **IDE plugin** (Phase 2): hosts LSP / MCP, does **not** directly import `internal/`.
- **GUI** (Phase 2): independent HTTP API boundary (`serve-gui`), not coupled to CLI flags.

CLI explicitly does NOT implement `list` / `get` / `update` / `delete` / `restore` — those live in MCP only. Don't add them as CLI subcommands without consulting architecture.md §一.

## Workflow

### Before writing code

1. Read `docs/article.md` to understand the project vision.
2. Check `docs/requirements.md` for the current phase's requirements.
3. Review the `internal/` package(s) you'll touch to learn existing conventions.
4. Confirm the relevant skill (`go-style` or `ts-style`) is loaded — these auto-invoke, but verify.

### When proposing changes

- Prefer small, reviewable PRs over mega-commits.
- Include tests for any new exported function in the same commit.
- If adding a dependency, justify it in the commit body.
- Run `go vet ./...`, `gofmt -l .`, and `go test ./...` before committing.

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
- Adding `--path` / stdin / external-template flags to existing commands without consulting architecture.md §5.
- Implementing `serve-lsp`, IDE plugin, GUI, hard-delete / GC — these are Phase 2.
- Adding `parent` / `depends_on` to frontmatter — directory + relative links cover it (see intent-structure.md).

## Storage format reminder

`.kron/intents/*.md` files use this frontmatter shape (don't change without migration plan):

```yaml
---
symbol: "auth.RefreshToken"
created_by: "@zhangjun005"
updated_at: "2026-09-22T10:00:00Z"
# optional for multi-collaborator projects:
# reviewers: ["@alice", "@bob"]
---

# Intent title

> One-line summary.

## Why
...

## Trade-offs
...

## Invariants / Assumptions
...
```

Status lifecycle (managed via `status` field when needed):
- `draft` → `active` → `superseded`

Changing this schema breaks every existing user. Coordinate before changing.

## Soft-delete reminder

`kron_delete` (MCP only) does **not** erase files. It moves `.kron/intents/<slug>.md` to `.kron/.trash/<slug>.md`. `kron_restore` moves it back. Hard-delete / GC of `.kron/.trash/` is intentionally **not** in v1 — users can `git rm` it manually. See architecture.md §5.2.1.
