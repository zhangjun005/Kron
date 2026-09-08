//! File-watcher stub.
//!
//! Phase 1: does nothing. Phase 2 will wrap `notify` crate + tokio mpsc
//! to push Tauri events when `KRON/.kron-context/`, `KRON/VERTEX/`,
//! `kron-internal/important/`, or `.git/HEAD` change. See
//! `dev-docs/design/05-GUI设计.md` § 3.3 for the event payload contract.
