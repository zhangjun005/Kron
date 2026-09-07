//! Current-vertex pointer (Git-style `HEAD` for the Kron CLI).
//!
//! Anchor #8 / Q10 introduces a "current vertex" concept so users can
//! run e.g. `kron task list` without repeating `--vertex <name>` on
//! every invocation. The pointer is stored in
//! `kron-internal/state.json` alongside the rest of the per-project
//! internal state. It is a *single global* pointer per workspace —
//! per-cwd pointers (a la Git worktrees) are explicitly out of scope
//! for v1.
//!
//! The pointer is intentionally lightweight: just a name and a
//! timestamp. We do **not** validate the vertex against
//! `vertices.json` on every read because (a) the read path needs to be
//! cheap and (b) validation is the caller's job — `set` is the only
//! operation that mutates, and it should accept whatever the user
//! asked for (the call site is responsible for verifying the vertex
//! exists in `vertices.json` first; this keeps `core::state_pointer`
//! independent of `core::vertex`).
//!
//! Layout:
//!
//! ```text
//! kron-internal/state.json
//! {
//!   "version": 1,
//!   "current_vertex": "开发",
//!   "updated_at": "2026-09-06T22:30:00+08:00"
//! }
//! ```

use crate::error::{KronError, Result};
use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use std::fs;
use std::path::Path;

/// Current schema version. Bump if the on-disk shape ever changes.
const STATE_VERSION: u32 = 1;

/// On-disk representation. Kept private to this module — callers go
/// through the `load` / `set` / `clear` API and never see the raw
/// struct, which keeps us free to evolve the layout.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
struct PointerState {
    version: u32,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    current_vertex: Option<String>,
    #[serde(default = "Utc::now")]
    updated_at: DateTime<Utc>,
}

impl Default for PointerState {
    fn default() -> Self {
        Self {
            version: STATE_VERSION,
            current_vertex: None,
            updated_at: Utc::now(),
        }
    }
}

fn state_path(project_root: &Path) -> std::path::PathBuf {
    project_root.join("kron-internal").join("state.json")
}

fn read_disk(project_root: &Path) -> Result<PointerState> {
    let path = state_path(project_root);
    if !path.exists() {
        // First read on a workspace that hasn't seen the pointer
        // yet. Return the default (no current vertex). We deliberately
        // do *not* write the file here — that's `set`'s job.
        return Ok(PointerState::default());
    }
    let raw = fs::read_to_string(&path)?;
    if raw.trim().is_empty() {
        return Ok(PointerState::default());
    }
    let parsed: PointerState = serde_json::from_str(&raw).map_err(|e| {
        KronError::Internal(format!(
            "failed to parse kron-internal/state.json ({path}): {e}",
            path = path.display()
        ))
    })?;
    if parsed.version != STATE_VERSION {
        return Err(KronError::Internal(format!(
            "kron-internal/state.json has version {}, expected {}; \
             please run 'kron doctor' or reinitialize the workspace",
            parsed.version, STATE_VERSION
        )));
    }
    Ok(parsed)
}

fn write_disk(project_root: &Path, state: &PointerState) -> Result<()> {
    let path = state_path(project_root);
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent)?;
    }
    let json = serde_json::to_string_pretty(state)?;
    fs::write(&path, json)?;
    Ok(())
}

/// Load the current vertex name, if one is set.
///
/// Returns `Ok(None)` when no pointer has been set yet (or the file
/// doesn't exist yet). Callers should treat `None` as "ask the user
/// for a vertex" or surface a `Cli` error suggesting `kron vertex use
/// <name>`.
pub fn current(project_root: &Path) -> Result<Option<String>> {
    Ok(read_disk(project_root)?.current_vertex)
}

/// Load the full pointer state (name + timestamp).
///
/// Mostly useful for `--json` output where the caller wants the
/// timestamp too.
pub fn load_full(project_root: &Path) -> Result<(Option<String>, DateTime<Utc>)> {
    let s = read_disk(project_root)?;
    Ok((s.current_vertex, s.updated_at))
}

/// Set the current vertex pointer.
///
/// The name is stored verbatim — the caller is responsible for
/// verifying the vertex exists in `vertices.json` first
/// (`core::vertex::find_required`). We don't double-validate here so
/// that this module stays decoupled from `core::vertex`.
pub fn set(project_root: &Path, name: &str) -> Result<()> {
    if name.is_empty() {
        return Err(KronError::Cli(
            "vertex name must not be empty (use --unset to clear)".into(),
        ));
    }
    let mut s = read_disk(project_root)?;
    s.current_vertex = Some(name.to_string());
    s.updated_at = Utc::now();
    write_disk(project_root, &s)
}

/// Clear the current vertex pointer.
///
/// After this, [`current`] returns `None` until the next `set`.
pub fn clear(project_root: &Path) -> Result<()> {
    let mut s = read_disk(project_root)?;
    s.current_vertex = None;
    s.updated_at = Utc::now();
    write_disk(project_root, &s)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn tempdir() -> std::path::PathBuf {
        let base = std::env::temp_dir();
        let unique = format!(
            "kron-state-pointer-{}-{}",
            std::process::id(),
            chrono::Utc::now().timestamp_nanos_opt().unwrap_or(0)
        );
        let p = base.join(unique);
        fs::create_dir_all(&p).unwrap();
        p
    }

    #[test]
    fn current_is_none_when_no_file() {
        let root = tempdir();
        assert_eq!(current(&root).unwrap(), None);
    }

    #[test]
    fn set_then_current_round_trips() {
        let root = tempdir();
        set(&root, "开发").unwrap();
        assert_eq!(current(&root).unwrap().as_deref(), Some("开发"));
    }

    #[test]
    fn set_rejects_empty_name() {
        let root = tempdir();
        let err = set(&root, "").unwrap_err();
        assert!(matches!(err, KronError::Cli(_)));
    }

    #[test]
    fn clear_removes_pointer() {
        let root = tempdir();
        set(&root, "doing").unwrap();
        clear(&root).unwrap();
        assert_eq!(current(&root).unwrap(), None);
    }

    #[test]
    fn load_full_returns_timestamp() {
        let root = tempdir();
        set(&root, "done").unwrap();
        let (cur, ts) = load_full(&root).unwrap();
        assert_eq!(cur.as_deref(), Some("done"));
        // Timestamp should be very recent.
        let age = (Utc::now() - ts).num_seconds().abs();
        assert!(age < 5, "timestamp drifted by {age}s");
    }

    #[test]
    fn corrupted_file_surfaces_internal_error() {
        let root = tempdir();
        fs::create_dir_all(root.join("kron-internal")).unwrap();
        fs::write(state_path(&root), "{ not json").unwrap();
        let err = current(&root).unwrap_err();
        assert!(matches!(err, KronError::Internal(_)));
    }

    #[test]
    fn version_mismatch_is_reported() {
        let root = tempdir();
        fs::create_dir_all(root.join("kron-internal")).unwrap();
        fs::write(
            state_path(&root),
            r#"{"version": 999, "current_vertex": "x", "updated_at": "2026-09-06T00:00:00Z"}"#,
        )
        .unwrap();
        let err = current(&root).unwrap_err();
        match err {
            KronError::Internal(msg) => assert!(msg.contains("version")),
            other => panic!("expected Internal, got {other:?}"),
        }
    }
}
