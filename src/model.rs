//! Domain models: Project, Settings, AutoResolve.
//!
//! Matches dev-docs/design/01-数据模型.md § 2.

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use std::path::PathBuf;

/// Top-level project metadata persisted to `kron-internal/config.json`.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct Project {
    /// Human-readable name (defaults to the project directory name).
    pub name: String,

    /// Absolute path to the project root.
    pub project_path: PathBuf,

    /// Absolute path to the kron-internal directory (lives inside `project_path`).
    pub kron_data_path: PathBuf,

    /// When the project was first initialized.
    pub created_at: DateTime<Utc>,

    /// Kron version that created this project.
    pub kron_version: String,

    /// User-facing settings (mutable via `kron config`).
    pub settings: ProjectSettings,
}

/// User-tunable project settings.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ProjectSettings {
    /// Minutes of overlap before a sync drift is reported as a conflict.
    #[serde(default = "default_conflict_threshold")]
    pub conflict_threshold_minutes: u32,

    /// Conflict auto-resolve strategy.
    #[serde(default = "default_auto_resolve")]
    pub auto_resolve: AutoResolve,

    /// How often `.kron-context/` is regenerated (minutes).
    #[serde(default = "default_context_refresh_minutes")]
    pub context_refresh_minutes: u32,
}

impl Default for ProjectSettings {
    fn default() -> Self {
        Self {
            conflict_threshold_minutes: default_conflict_threshold(),
            auto_resolve: default_auto_resolve(),
            context_refresh_minutes: default_context_refresh_minutes(),
        }
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum AutoResolve {
    Prompt,
    Latest,
    Manual,
}

fn default_conflict_threshold() -> u32 {
    5
}
fn default_context_refresh_minutes() -> u32 {
    5
}
fn default_auto_resolve() -> AutoResolve {
    AutoResolve::Prompt
}

/// File-link strategy for `KRON/important/` ↔ `kron-internal/important/`.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, clap::ValueEnum)]
#[serde(rename_all = "lowercase")]
pub enum LinkMode {
    Symlink,
    Copy,
}

impl LinkMode {
    /// On Windows we default to `Copy` because symlinks require
    /// either Developer Mode or admin rights. Users who pass
    /// `--mode symlink` explicitly get a best-effort attempt.
    pub fn fallback_for_platform(self) -> Self {
        #[cfg(windows)]
        {
            match self {
                LinkMode::Symlink => LinkMode::Copy,
                LinkMode::Copy => LinkMode::Copy,
            }
        }
        #[cfg(not(windows))]
        {
            self
        }
    }
}

/// One entry in `kron-internal/vertices.json` (vertex registry index).
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct VertexEntry {
    pub name: String,
    pub path: String,
}

impl Project {
    /// Build a Project for a freshly-initialized workspace.
    pub fn new(project_root: PathBuf, _link_mode: LinkMode) -> Self {
        let kron_data_path = project_root.join("kron-internal");
        let name = project_root
            .file_name()
            .map(|s| s.to_string_lossy().into_owned())
            .unwrap_or_else(|| "unnamed".into());
        Self {
            name,
            project_path: project_root,
            kron_data_path,
            created_at: Utc::now(),
            kron_version: env!("CARGO_PKG_VERSION").to_string(),
            settings: ProjectSettings {
                conflict_threshold_minutes: default_conflict_threshold(),
                auto_resolve: default_auto_resolve(),
                context_refresh_minutes: default_context_refresh_minutes(),
            },
        }
    }
}

// ---- Task state ----

/// Lifecycle state of a task. The state also corresponds to which
/// vertex directory the task lives under (simplified M1 model):
/// `KRON/VERTEX/<state>/<id>.md`.
///
/// The default vertex trio is `todo / doing / done`. Users may add
/// more (e.g. `backlog`, `review`).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum TaskState {
    /// Not yet started.
    Todo,
    /// In progress.
    Doing,
    /// Finished.
    Done,
}

impl TaskState {
    /// Canonical lower-case name used as the vertex directory name.
    pub fn as_str(self) -> &'static str {
        match self {
            TaskState::Todo => "todo",
            TaskState::Doing => "doing",
            TaskState::Done => "done",
        }
    }

    /// The default starting state for a freshly-added task.
    pub fn default_for_new() -> Self {
        TaskState::Todo
    }
}

impl Default for TaskState {
    fn default() -> Self {
        TaskState::Todo
    }
}

impl std::fmt::Display for TaskState {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(self.as_str())
    }
}

/// Parse an arbitrary state string into a [`TaskState`].
/// Falls back to the given string being treated as a custom vertex
/// name (i.e. always returns Ok for non-empty slugs).
pub fn parse_state(s: &str) -> Result<String, String> {
    if s.is_empty() {
        return Err("state name must not be empty".into());
    }
    Ok(s.to_ascii_lowercase())
}
