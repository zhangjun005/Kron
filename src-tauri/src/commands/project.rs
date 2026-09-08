//! Project-related IPC commands.
//!
//! Implements `kron_project_list` from
//! `dev-docs/design/05-GUI设计.md` § 3.2 (stubbed — registry not wired
//! yet; returns empty list) and a smoke-test `kron_greet` command used
//! to verify the Rust ↔ Solid IPC pipeline is alive.

#[allow(unused_imports)]
use tauri::command;

use crate::ipc_types::{Greeting, ProjectMeta};

/// `kron_project_list() -> Vec<ProjectMeta>`
///
/// Lists all known Kron projects (V0 homepage data source).
/// Phase 1: returns empty list — registry discovery not yet implemented.
#[tauri::command]
pub async fn kron_project_list() -> Result<Vec<ProjectMeta>, String> {
    tracing::info!("kron_project_list invoked");
    Ok(vec![])
}

/// `kron_greet(name: String) -> Greeting`
///
/// Smoke-test command — proves the IPC pipeline is wired.
#[tauri::command]
pub async fn kron_greet(name: String) -> Result<Greeting, String> {
    tracing::info!("kron_greet invoked with name={name}");
    Ok(Greeting {
        message: format!("Hello from Kron backend, {name}!"),
        backend: "kron-gui Tauri 2".to_string(),
        timestamp: chrono::Utc::now().to_rfc3339(),
    })
}

// Re-export so `lib.rs` can find the `ProjectMeta` type via this module.
#[allow(unused_imports)]
use crate::ipc_types::ProjectMeta as _Reexport;
