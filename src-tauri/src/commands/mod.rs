//! IPC command handlers (Rust side of `invoke('kron_*')`).
//!
//! Phase 1 skeleton: only `project::kron_project_list` is wired.
//! Subsequent commands are added per the IPC contract in
//! `dev-docs/design/05-GUI设计.md` § 3.2.

pub mod project;
pub mod watcher;

/// Phase-1 stub: just logs that the watcher would be spawned here.
pub fn spawn_watcher_stub(_app: tauri::AppHandle) {
    tracing::info!("watcher stub: would spawn notify watcher here (see watcher.rs)");
}
