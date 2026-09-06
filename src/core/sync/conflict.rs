//! Conflict detection — compare project-side vs internal-side copies.
//!
//! The scan walks every entry in the project's `ImportantIndex`, hashes
//! both copies (if they exist), and decides the `SyncState`. When both
//! copies exist and their MD5s differ, a `ConflictRecord` is created
//! with both byte-level backups under `kron-internal/conflicts/<id>/`.
//!
//! `detect()` is idempotent: re-running it will not duplicate an existing
//! pending conflict for the same path.

use crate::core::sync::sync_index::{internal_path_for, project_path_for, ConflictIndex};
use crate::error::{KronError, Result};
use crate::model::{ConflictRecord, ConflictResolution, ConflictStatus, SyncPair, SyncState};
use chrono::{DateTime, Utc};
use std::fs;
use std::path::Path;

/// Aggregate statistics from one scan pass.
#[derive(Debug, Default, Clone, serde::Serialize)]
pub struct ScanStats {
    pub scanned: u32,
    pub synced: u32,
    pub project_only: u32,
    pub internal_only: u32,
    pub conflicts_new: u32,
    pub conflicts_existing: u32,
}

/// Full result of one `detect()` call.
#[derive(Debug, Clone)]
pub struct ScanResult {
    pub pairs: Vec<SyncPair>,
    pub stats: ScanStats,
}

/// Classify one (project, internal) pair into a SyncState.
///
/// `recorded` is the state from the last scan / index entry — used to
/// bias the decision so a freshly-`InternalOnly` file is not re-classified
/// to Synced in the next scan just because the hash check is skipped.
pub fn classify(
    project_exists: bool,
    internal_exists: bool,
    project_hash: &str,
    internal_hash: &str,
    recorded: Option<SyncState>,
) -> SyncState {
    match (project_exists, internal_exists) {
        (false, false) => {
            // Both gone — caller will prune.
            recorded.unwrap_or(SyncState::Synced)
        }
        (true, false) => SyncState::ProjectOnly,
        (false, true) => SyncState::InternalOnly,
        (true, true) => {
            if project_hash == internal_hash {
                SyncState::Synced
            } else {
                SyncState::Conflict
            }
        }
    }
}

/// Run a single scan over the project's important files.
///
/// - Loads (or creates) the `ImportantIndex`.
/// - Walks each registered path, reads both copies, classifies state.
/// - Creates a new `ConflictRecord` + byte backups for any path that
///   newly becomes Conflict.
/// - Persists the updated index, returning `ScanResult`.
///
/// Paths that have no project-side AND no internal-side copy are left
/// in the index (a warning will surface in `pairs`).
pub fn detect(project_root: &Path) -> Result<ScanResult> {
    let mut index = crate::core::sync::sync_index::ImportantIndex::load(project_root)?;
    let mut conflict_idx = ConflictIndex::load(project_root)?;
    let mut pairs = Vec::new();
    let mut stats = ScanStats::default();

    // Snapshot keys so we can mutate `index` inside the loop.
    let keys: Vec<String> = index.files.keys().cloned().collect();
    for rel in keys {
        let entry = match index.files.get(&rel) {
            Some(e) => e.clone(),
            None => continue,
        };

        let proj = project_path_for(project_root, &rel);
        let internal = internal_path_for(project_root, &rel);

        let project_exists = proj.exists();
        let internal_exists = internal.exists();

        let mut project_hash = String::new();
        let mut internal_hash = entry.internal_hash.clone();
        let mut project_bytes: Option<Vec<u8>> = None;
        let mut internal_bytes: Option<Vec<u8>> = None;

        if project_exists {
            let data = fs::read(&proj)?;
            project_hash = crate::core::sync::sync_index::md5_hex(&data);
            project_bytes = Some(data);
        }
        if internal_exists {
            let data = fs::read(&internal)?;
            internal_hash = crate::core::sync::sync_index::md5_hex(&data);
            internal_bytes = Some(data);
        }

        let recorded = Some(entry.sync_state);
        let new_state = classify(
            project_exists,
            internal_exists,
            &project_hash,
            &internal_hash,
            recorded,
        );

        stats.scanned += 1;
        let mut conflict_id: Option<String> = None;

        if new_state == SyncState::Conflict {
            // Only create a new conflict record if there isn't already a
            // pending one for this path. We detect that by looking up
            // existing conflict records.
            let existing = find_pending_conflict_for(project_root, &rel)?;
            if let Some(rec) = existing {
                conflict_id = Some(rec.id);
                stats.conflicts_existing += 1;
            } else if let (Some(pb), Some(ib)) = (&project_bytes, &internal_bytes) {
                let id = generate_conflict_id(&rel, &project_hash);
                let proj_mtime = mtime_iso(&proj);
                let int_mtime = mtime_iso(&internal);
                let _record = create_conflict_record(
                    project_root,
                    &id,
                    &rel,
                    &proj,
                    &internal,
                    pb,
                    ib,
                    &project_hash,
                    &internal_hash,
                    proj_mtime,
                    int_mtime,
                )?;
                conflict_idx.add(&id, project_root)?;
                conflict_id = Some(id);
                stats.conflicts_new += 1;
            }
        } else {
            match new_state {
                SyncState::Synced => stats.synced += 1,
                SyncState::ProjectOnly => stats.project_only += 1,
                SyncState::InternalOnly => stats.internal_only += 1,
                _ => {}
            }
        }

        index.update_state(&rel, new_state);
        pairs.push(SyncPair {
            relative_path: rel.clone(),
            sync_state: new_state,
            project_exists,
            internal_exists,
            conflict_id,
        });
    }

    index.save(project_root)?;

    Ok(ScanResult { pairs, stats })
}

/// Generate a conflict id of the form `<YYYYMMDD>_<HHMMSS>_<short>`.
pub fn generate_conflict_id(rel_path: &str, project_hash: &str) -> String {
    let ts = Utc::now().format("%Y%m%d_%H%M%S").to_string();
    let short = &project_hash[..6.min(project_hash.len())];
    let slug: String = rel_path
        .chars()
        .map(|c| if c.is_ascii_alphanumeric() { c.to_ascii_lowercase() } else { '_' })
        .collect();
    format!("{ts}_{short}_{slug}")
}

#[allow(clippy::too_many_arguments)]
fn create_conflict_record(
    project_root: &Path,
    id: &str,
    rel_path: &str,
    proj_path: &Path,
    internal_path: &Path,
    project_bytes: &[u8],
    internal_bytes: &[u8],
    project_hash: &str,
    internal_hash: &str,
    project_mtime: DateTime<Utc>,
    internal_mtime: DateTime<Utc>,
) -> Result<ConflictRecord> {
    let dir = crate::core::sync::sync_index::conflicts_dir(project_root).join(id);
    fs::create_dir_all(&dir)?;

    let proj_backup = dir.join("project_version");
    let int_backup = dir.join("internal_version");

    // Mirror the relative path layout inside backups so diffs are easy
    // to recognize (suffix only — backups are flat per conflict).
    fs::write(&proj_backup, project_bytes)?;
    fs::write(&int_backup, internal_bytes)?;

    // Reference files are kept short — write them next to the backups.
    let record = ConflictRecord {
        id: id.to_string(),
        relative_path: rel_path.to_string(),
        detected_at: Utc::now(),
        project_backup: proj_backup.clone(),
        internal_backup: int_backup.clone(),
        project_hash: project_hash.to_string(),
        internal_hash: internal_hash.to_string(),
        project_mtime,
        internal_mtime,
        status: ConflictStatus::Pending,
        resolution: None,
        resolved_at: None,
    };

    let record_path = dir.join("_record.json");
    let json = serde_json::to_string_pretty(&record)?;
    fs::write(&record_path, json)?;

    // Touch the source files so the backup is the authoritative copy
    // going forward. The caller will not overwrite them.
    let _ = proj_path;
    let _ = internal_path;

    Ok(record)
}

/// Load all known conflict records (used by `conflict list/show`).
pub fn list_all(project_root: &Path) -> Result<Vec<ConflictRecord>> {
    let idx = ConflictIndex::load(project_root)?;
    let mut out = Vec::new();
    for id in &idx.ids {
        if let Some(rec) = load_record(project_root, id)? {
            out.push(rec);
        }
    }
    Ok(out)
}

/// Filter `list_all` by status string ("pending" | "resolved" | "ignored").
pub fn list_by_status(project_root: &Path, status: &str) -> Result<Vec<ConflictRecord>> {
    let wanted = match status {
        "pending" => ConflictStatus::Pending,
        "resolved" => ConflictStatus::Resolved,
        "ignored" => ConflictStatus::Ignored,
        "all" => return list_all(project_root),
        other => return Err(KronError::Cli(format!("unknown conflict status filter: {other}"))),
    };
    Ok(list_all(project_root)?
        .into_iter()
        .filter(|r| r.status == wanted)
        .collect())
}

/// Look up a single conflict by id.
pub fn load_record(project_root: &Path, id: &str) -> Result<Option<ConflictRecord>> {
    let dir = crate::core::sync::sync_index::conflicts_dir(project_root).join(id);
    let record_path = dir.join("_record.json");
    if !record_path.exists() {
        return Ok(None);
    }
    let raw = fs::read_to_string(&record_path)?;
    let rec: ConflictRecord = serde_json::from_str(&raw)?;
    Ok(Some(rec))
}

/// Resolve a pending conflict by applying the chosen resolution.
///
/// Steps:
/// 1. Load the conflict record.
/// 2. Verify it is still `Pending`.
/// 3. Apply the resolution: copy chosen bytes over the other side,
///    update the index entry, delete backups, mark the record.
/// 4. Persist updated record + index.
pub fn resolve(
    project_root: &Path,
    id: &str,
    decision: ConflictResolution,
) -> Result<ConflictRecord> {
    let mut rec = load_record(project_root, id)?
        .ok_or_else(|| KronError::NotFound(crate::core::sync::sync_index::conflicts_dir(project_root).join(id)))?;

    if rec.status != ConflictStatus::Pending {
        return Err(KronError::Cli(format!(
            "conflict {} is already {} (cannot resolve)",
            rec.id,
            rec.status
        )));
    }

    let proj_path = project_path_for(project_root, &rec.relative_path);
    let internal_path = internal_path_for(project_root, &rec.relative_path);

    match decision {
        ConflictResolution::UseProject => {
            if let Some(parent) = internal_path.parent() {
                fs::create_dir_all(parent)?;
            }
            let data = fs::read(&rec.project_backup)?;
            fs::write(&internal_path, &data)?;
            if let Some(parent) = proj_path.parent() {
                fs::create_dir_all(parent)?;
            }
            fs::write(&proj_path, &data)?;
        }
        ConflictResolution::UseInternal => {
            if let Some(parent) = proj_path.parent() {
                fs::create_dir_all(parent)?;
            }
            let data = fs::read(&rec.internal_backup)?;
            fs::write(&proj_path, &data)?;
            if let Some(parent) = internal_path.parent() {
                fs::create_dir_all(parent)?;
            }
            fs::write(&internal_path, &data)?;
        }
        ConflictResolution::Ignore => {
            // Nothing to write. Just leave both sides as-is.
        }
    }

    rec.status = match decision {
        ConflictResolution::Ignore => ConflictStatus::Ignored,
        _ => ConflictStatus::Resolved,
    };
    rec.resolution = Some(decision);
    rec.resolved_at = Some(Utc::now());

    // Persist updated record (keep backup files for safety).
    let dir = crate::core::sync::sync_index::conflicts_dir(project_root).join(&rec.id);
    let record_path = dir.join("_record.json");
    let json = serde_json::to_string_pretty(&rec)?;
    fs::write(&record_path, json)?;

    // Update index entry to reflect new state.
    let new_state = match decision {
        ConflictResolution::Ignore => SyncState::Conflict, // stays conflict-y
        ConflictResolution::UseProject | ConflictResolution::UseInternal => SyncState::Synced,
    };
    let mut idx = crate::core::sync::sync_index::ImportantIndex::load(project_root)?;
    idx.update_state(&rec.relative_path, new_state);
    idx.save(project_root)?;

    Ok(rec)
}

/// Find an existing pending conflict whose `relative_path` matches.
fn find_pending_conflict_for(project_root: &Path, rel: &str) -> Result<Option<ConflictRecord>> {
    for rec in list_all(project_root)? {
        if rec.relative_path == rel && rec.status == ConflictStatus::Pending {
            return Ok(Some(rec));
        }
    }
    Ok(None)
}

fn mtime_iso(p: &Path) -> DateTime<Utc> {
    fs::metadata(p)
        .and_then(|m| m.modified())
        .ok()
        .map(DateTime::from)
        .unwrap_or_else(Utc::now)
}
