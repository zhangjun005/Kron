# AGENTS.md — Kron Project Conventions

> Guidance for AI coding agents working in this repository.
> Read this before writing or modifying any code.

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
│       └── tech-stack.md
├── .cursor/
│   ├── skills/            ← agent skills (loaded by Cursor)
│   └── rules/             ← Cursor rule format (.mdc)
├── go.mod
├── go.sum
├── README.md
└── AGENTS.md              ← this file
```

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
