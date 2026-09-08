//! IPC type definitions shared between Rust handlers and the Solid
//! frontend. Frontend uses `ts-rs` to auto-generate TypeScript types
//! from these (configured separately).
//!
//! Phase 1: minimal types — just enough for the smoke test.

use serde::{Deserialize, Serialize};

/// Project metadata as shown on V0 homepage cards.
///
/// Mirrors `kron_project_list` in 05-GUI设计.md § 3.2.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ProjectMeta {
    pub id: String,
    pub name: String,
    pub path: String,
    pub vertex_count: u32,
    pub task_count: u32,
    pub conflict_count: u32,
    pub last_opened_at: Option<String>,
    pub initialized: bool,
}

/// Response payload for `kron_greet` (smoke-test command).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Greeting {
    pub message: String,
    pub backend: String,
    pub timestamp: String,
}
