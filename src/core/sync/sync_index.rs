//! Important-file index for a project.
//!
//! Tracks which files are "important" (synced between `KRON/important/`
//! and `kron-internal/important/files/`). Persisted to
//! `kron-internal/important/_index.json`.
//!
//! M2 simplified model: we use a flat JSON file (no per-file metadata
//! beyond path + sync_state + last-known hash). The P2 design in
//! 03-双源同步机制.md § 1.2 calls for a richer schema; the simplified
//! version is enough to demonstrate conflict detection and resolution.

use crate::error::{KronError, Result};
use crate::model::SyncState;
use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use std::collections::BTreeMap;
use std::fs;
use std::path::{Path, PathBuf};

/// Persisted under `kron-internal/important/_index.json`.
#[derive(Debug, Clone, Default, Serialize, Deserialize, PartialEq)]
pub struct ImportantIndex {
    /// Relative-path -> entry.
    #[serde(default)]
    pub files: BTreeMap<String, ImportantEntry>,
}

/// One tracked important file.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ImportantEntry {
    /// Relative path from the project root, e.g. `src/main.rs`.
    pub path: String,
    /// Last known sync state. Updated after each scan.
    #[serde(default = "default_sync_state")]
    pub sync_state: SyncState,
    /// MD5 of the internal copy at last sync (hex).
    #[serde(default)]
    pub internal_hash: String,
    /// When the entry was first added to the index.
    pub added_at: DateTime<Utc>,
    /// When the entry was last updated.
    pub updated_at: DateTime<Utc>,
}

fn default_sync_state() -> SyncState {
    SyncState::Synced
}

impl ImportantIndex {
    /// Load from disk, returning an empty index if the file is missing.
    pub fn load(project_root: &Path) -> Result<Self> {
        let path = index_path(project_root);
        if !path.exists() {
            return Ok(Self::default());
        }
        let raw = fs::read_to_string(&path)?;
        if raw.trim().is_empty() {
            return Ok(Self::default());
        }
        let idx: Self = serde_json::from_str(&raw).unwrap_or_default();
        Ok(idx)
    }

    /// Persist to disk. Creates parent dirs as needed.
    pub fn save(&self, project_root: &Path) -> Result<()> {
        let path = index_path(project_root);
        if let Some(parent) = path.parent() {
            fs::create_dir_all(parent)?;
        }
        let json = serde_json::to_string_pretty(self)?;
        fs::write(&path, json)?;
        Ok(())
    }

    /// Register a path as important. Sets sync_state to Synced after
    /// copying the project-side file into the internal store.
    pub fn register(
        &mut self,
        project_root: &Path,
        rel_path: &str,
    ) -> Result<()> {
        let now = Utc::now();
        let entry = self
            .files
            .entry(rel_path.to_string())
            .or_insert_with(|| ImportantEntry {
                path: rel_path.to_string(),
                sync_state: SyncState::Synced,
                internal_hash: String::new(),
                added_at: now,
                updated_at: now,
            });
        entry.updated_at = now;

        // Mirror the file into the internal store on first registration.
        let proj = project_root.join(rel_path);
        if !proj.exists() {
            return Err(KronError::NotFound(proj));
        }
        let internal = internal_path_for(project_root, rel_path);
        if let Some(parent) = internal.parent() {
            fs::create_dir_all(parent)?;
        }
        let data = fs::read(&proj)?;
        fs::write(&internal, &data)?;
        entry.internal_hash = md5_hex(&data);

        self.save(project_root)?;
        Ok(())
    }

    /// Update the sync_state for a single entry (no-op if missing).
    pub fn update_state(&mut self, rel_path: &str, state: SyncState) {
        if let Some(e) = self.files.get_mut(rel_path) {
            e.sync_state = state;
            e.updated_at = Utc::now();
        }
    }

    /// Remove a path from the index (does not delete files).
    pub fn remove(&mut self, rel_path: &str) -> bool {
        self.files.remove(rel_path).is_some()
    }
}

/// Where on disk the index file lives.
pub fn index_path(project_root: &Path) -> PathBuf {
    project_root
        .join("kron-internal")
        .join("important")
        .join("_index.json")
}

/// Map a relative project path to its mirror inside `kron-internal/important/files/`.
pub fn internal_path_for(project_root: &Path, rel_path: &str) -> PathBuf {
    project_root
        .join("kron-internal")
        .join("important")
        .join("files")
        .join(rel_path)
}

/// Project-side absolute path for a registered file.
pub fn project_path_for(project_root: &Path, rel_path: &str) -> PathBuf {
    project_root.join(rel_path)
}

/// Compute MD5 of a byte slice and return the hex string (lowercase).
pub fn md5_hex(data: &[u8]) -> String {
    let digest = md5::compute(data);
    format!("{:x}", digest)
}

/// Directory holding conflict records.
pub fn conflicts_dir(project_root: &Path) -> PathBuf {
    project_root.join("kron-internal").join("conflicts")
}

/// Directory holding the index of all known conflicts.
pub fn conflicts_index_path(project_root: &Path) -> PathBuf {
    project_root
        .join("kron-internal")
        .join("conflicts")
        .join("_index.json")
}

/// Append-only registry of all known conflict IDs.
#[derive(Debug, Clone, Default, Serialize, Deserialize, PartialEq)]
pub struct ConflictIndex {
    /// Conflict IDs in insertion order.
    #[serde(default)]
    pub ids: Vec<String>,
}

impl ConflictIndex {
    pub fn load(project_root: &Path) -> Result<Self> {
        let path = conflicts_index_path(project_root);
        if !path.exists() {
            return Ok(Self::default());
        }
        let raw = fs::read_to_string(&path)?;
        if raw.trim().is_empty() {
            return Ok(Self::default());
        }
        Ok(serde_json::from_str(&raw).unwrap_or_default())
    }

    pub fn save(&self, project_root: &Path) -> Result<()> {
        let path = conflicts_index_path(project_root);
        if let Some(parent) = path.parent() {
            fs::create_dir_all(parent)?;
        }
        let json = serde_json::to_string_pretty(self)?;
        fs::write(&path, json)?;
        Ok(())
    }

    pub fn add(&mut self, id: &str, project_root: &Path) -> Result<()> {
        if !self.ids.iter().any(|x| x == id) {
            self.ids.push(id.to_string());
            self.save(project_root)?;
        }
        Ok(())
    }
}
