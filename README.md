# Kron

> Git-native task tracker for AI-assisted development — rewrite in progress.

The Rust/Tauri prototype has been **archived** at branch [`archive/rust-v0.1`](../../tree/archive/rust-v0.1) (tag: `v0.1-rust-legacy`).

This branch is a **clean slate** for a Go-based redesign. See the design notes below for the new direction.

---

## Status

🚧 **Under redesign.** Storage format, CLI surface, and integration model are all being reconsidered.

## What stays the same

- Project name (`Kron`) and the core thesis: a Git-native task tracker where data lives as plain Markdown that AI tools can read directly.

## What's changing

| Layer | Old (Rust/Tauri) | New (Go) |
|-------|------------------|----------|
| Language | Rust + Tauri 2 | Go |
| Persistence | Dual-source sync + conflict engine | Single-source Markdown + `.kron/` metadata |
| Background process | Daemon + file watcher | None — manual + API-driven |
| GUI | Tauri 2 + Solid | Tauri 2 (frontend) + Go HTTP API (backend) |

## Why the rewrite

The Rust prototype validated the core idea but accumulated design debt:

- **Over-engineered persistence** (dual-source sync, mtime+hash conflict detection, 5-state machine) for a problem that doesn't exist in sequential human/AI workflows.
- **Daemon + file watcher** infrastructure for a single-user tool that doesn't need background processing.
- **Rust ergonomics** slowed iteration on a course-project-scale codebase.

The Go version targets **clarity over completeness**: keep what the core thesis actually needs, drop what doesn't.

---

See [`dev-docs/`](./dev-docs/) for ongoing design notes (to be written from scratch).
