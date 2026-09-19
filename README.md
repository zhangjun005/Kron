# Kron

> Git-native task tracker for AI-assisted development.

Kron turns your Git repository's task data into **plain Markdown files** that AI coding agents (Cursor, Claude Code, Aider) can read and write directly — no API server, no daemon, no proprietary format. You talk to it; your AI talks to it; the diff is just `git diff`.

---

## The problem

AI coding agents work best when they have a clear sense of what needs to be done. Today, you write that down in chat: paste TODOs, copy task text, re-explain context every session. It's friction.

Task trackers (Jira, Linear, GitHub Projects) live on the network — invisible to the agent. You can't `cat tasks.md`, you can't commit them, you can't diff them against last week.

## What Kron is

A task tracker that stores everything as **Markdown files inside your repo**:

```
.kron/
├── tasks/                     # one .md per task
│   ├── 2026-09-19-001-init-go-mod.md
│   ├── 2026-09-19-002-storage-schema.md
│   └── ...
├── projects.md                # project registry (optional)
└── config.toml                # kron config (optional)
```

Each task file is a Markdown document with a small YAML frontmatter block:

```markdown
---
id: 2026-09-19-001
status: open
priority: high
labels: [storage, schema]
created: 2026-09-19T22:30:00Z
---

# Initialize Go module + directory skeleton

## Why
- [design doc](./dev-docs/design/01-storage-format.md) needs a real Go project to live in
- decision: Go (clarity over completeness for a course-scale codebase)

## Done when
- [ ] `go mod init github.com/zhangjun005/kron`
- [ ] directory layout decided: `cmd/kron/`, `internal/model/`, `internal/store/`
```

That's it. **The agent reads the same file you read.**

## What Kron does NOT do

- No background daemon. No file watcher. No sync engine.
- No dual-source persistence. No conflict resolution mtime+hash state machine.
- No proprietary format. The Markdown is the API.
- No electron app. (A Tauri GUI may come later, layered on top — never required.)

If a contributor or an AI can edit a Markdown file with confidence, that file is the truth.

## Status

🚧 **Go rewrite in progress.** The Rust/Tauri prototype is archived at branch [`archive/rust-v0.1`](../../tree/archive/rust-v0.1) (tag: `v0.1-rust-legacy`) for reference — it validated the thesis but accumulated design debt (see commit history).

## Roadmap (rough)

| Phase | What | State |
|-------|------|-------|
| 0 | Design: storage format & CLI surface | ⏳ drafting |
| 1 | Go skeleton: `go mod init` + directory layout | ⏳ next |
| 2 | Core CLI: `kron add`, `kron ls`, `kron done`, `kron show` | ⏳ |
| 3 | Storage layer: read/write task .md files | ⏳ |
| 4 | AI integration: `--for-ai` mode (concatenate tasks as one block) | ⏳ |
| 5 | (Optional) Tauri GUI over HTTP API | ⏳ |

## CLI preview (target)

```bash
# initialize .kron/ in current git repo
kron init

# add a new task — opens $EDITOR if interactive, else reads stdin
kron add

# list open tasks (Markdown table, default)
kron ls

# show full content of one task
kron show 2026-09-19-001

# mark a task done
kron done 2026-09-19-001

# emit everything as a single concatenated block — paste into Cursor chat
kron ls --for-ai
```

All of the above has a `--json` mode for scripting, and a `--for-ai` mode that emits a single human-readable block.

## Contributing

Decisions about format and CLI shape live in [`dev-docs/design/`](./dev-docs/design/). Read those before proposing changes — the format is what makes Kron worth using, and changing it costs users nothing because they own the files.

## License

TBD.
