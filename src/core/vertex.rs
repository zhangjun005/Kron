//! Vertex registry: CRUD over `kron-internal/vertices.json`.
//!
//! A "vertex" is a logical grouping for tasks (e.g. `todo`, `doing`,
//! `done`, `review`, `backlog`). Each vertex lives in two places:
//! - **Project side**: `KRON/VERTEX/<name>/` — holds the MD files
//! - **Internal side**: `kron-internal/vertices.json` — authoritative
//!   metadata registry (name, path, optional description, optional
//!   git branch binding).
//!
//! The registry is the single source of truth: the project-side
//! directories are re-derivable but the description / branch binding
//! only live here.

use crate::core::task::validate_vertex_name;
use crate::error::{KronError, Result};
use crate::model::VertexEntry;
use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use std::fs;
use std::path::{Path, PathBuf};

/// Path to the registry file inside `kron-internal/`.
pub fn registry_path(project_root: &Path) -> PathBuf {
    project_root.join("kron-internal").join("vertices.json")
}

/// Persisted vertex entry — superset of `model::VertexEntry` with P4 fields.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct VertexRecord {
    pub name: String,
    pub path: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub branch: Option<String>,
    #[serde(default = "Utc::now")]
    pub created_at: DateTime<Utc>,
    #[serde(default = "Utc::now")]
    pub updated_at: DateTime<Utc>,
}

impl From<&VertexRecord> for VertexEntry {
    fn from(v: &VertexRecord) -> Self {
        VertexEntry {
            name: v.name.clone(),
            path: v.path.clone(),
        }
    }
}

/// Load the registry; returns an empty vec when the file is missing
/// or empty (this is the fresh-init case).
pub fn load_registry(project_root: &Path) -> Result<Vec<VertexRecord>> {
    let path = registry_path(project_root);
    if !path.exists() {
        return Ok(Vec::new());
    }
    let raw = fs::read_to_string(&path)?;
    if raw.trim().is_empty() {
        return Ok(Vec::new());
    }
    Ok(serde_json::from_str(&raw).unwrap_or_default())
}

fn save_registry(project_root: &Path, records: &[VertexRecord]) -> Result<()> {
    let path = registry_path(project_root);
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent)?;
    }
    let json = serde_json::to_string_pretty(records)?;
    fs::write(&path, json)?;
    Ok(())
}

/// Look up a single vertex by name.
pub fn find(project_root: &Path, name: &str) -> Result<Option<VertexRecord>> {
    let reg = load_registry(project_root)?;
    Ok(reg.into_iter().find(|v| v.name == name))
}

/// Default project-side path for a vertex (matches P1 layout).
pub fn default_path(name: &str) -> String {
    format!("KRON/VERTEX/{name}")
}

/// Create a new vertex. Validates the name and refuses duplicates.
///
/// `branch` is optional metadata (no actual Git hook in v1).
pub fn create(
    project_root: &Path,
    name: &str,
    description: Option<&str>,
    branch: Option<&str>,
    path_override: Option<&str>,
) -> Result<VertexRecord> {
    validate_vertex_name(name)?;
    let mut reg = load_registry(project_root)?;
    if reg.iter().any(|v| v.name == name) {
        return Err(KronError::Cli(format!(
            "vertex '{name}' already exists (use `kron vertex describe` to update)"
        )));
    }
    let now = Utc::now();
    let rec = VertexRecord {
        name: name.to_string(),
        path: path_override
            .map(|s| s.to_string())
            .unwrap_or_else(|| default_path(name)),
        description: description.map(|s| s.to_string()),
        branch: branch.map(|s| s.to_string()),
        created_at: now,
        updated_at: now,
    };
    // Materialize the project-side directory so `task add <vertex>` works
    // immediately after `vertex create`.
    let dir = project_root.join(&rec.path);
    fs::create_dir_all(&dir)?;
    reg.push(rec.clone());
    save_registry(project_root, &reg)?;
    Ok(rec)
}

/// Update the description and/or branch binding of an existing vertex.
pub fn update(
    project_root: &Path,
    name: &str,
    description: Option<&str>,
    branch: Option<&str>,
) -> Result<VertexRecord> {
    let mut reg = load_registry(project_root)?;
    let v = reg
        .iter_mut()
        .find(|v| v.name == name)
        .ok_or_else(|| KronError::NotFound(project_root.join("kron-internal/vertices.json")))?;
    if let Some(d) = description {
        v.description = if d.is_empty() { None } else { Some(d.to_string()) };
    }
    if let Some(b) = branch {
        v.branch = if b.is_empty() { None } else { Some(b.to_string()) };
    }
    v.updated_at = Utc::now();
    let updated = v.clone();
    save_registry(project_root, &reg)?;
    Ok(updated)
}

/// Delete a vertex from the registry. Optionally also removes the
/// project-side directory tree (which may contain task MDs).
pub fn delete(project_root: &Path, name: &str, also_remove_dir: bool) -> Result<()> {
    let mut reg = load_registry(project_root)?;
    let idx = reg
        .iter()
        .position(|v| v.name == name)
        .ok_or_else(|| KronError::Cli(format!("vertex '{name}' not found")))?;
    let rec = reg.remove(idx);
    save_registry(project_root, &reg)?;

    if also_remove_dir {
        let dir = project_root.join(&rec.path);
        if dir.is_dir() {
            fs::remove_dir_all(&dir)?;
        }
        // Best-effort: also remove the state index.
        let state_file = project_root
            .join("kron-internal")
            .join("states")
            .join(format!("{name}.json"));
        if state_file.exists() {
            fs::remove_file(&state_file)?;
        }
    }
    Ok(())
}
