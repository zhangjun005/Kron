//! Integration tests for the P2 sync engine (conflict detection,
//! daemon PID lock, conflict resolution).

use kron::core::init::{materialize, prepare};
use kron::core::sync::conflict::{self, list_by_status, resolve, ScanResult};
use kron::core::sync::daemon::{self, DaemonStatus};
use kron::core::sync::sync_index::{
    internal_path_for, project_path_for, ImportantIndex,
};
use kron::model::{ConflictResolution, ConflictStatus, SyncState};
use std::fs;
use std::path::Path;
use tempfile::TempDir;

/// Create a project root with a fake `.git` dir, run `init`, and return
/// the tempdir (caller must keep it alive).
fn fresh_project() -> TempDir {
    let tmp = TempDir::new().expect("tempdir");
    let root = tmp.path();
    fs::create_dir(root.join(".git")).unwrap();
    let prep = prepare(root, false).expect("prepare");
    materialize(&prep, false, kron::model::LinkMode::Copy).expect("materialize");
    tmp
}

/// Register `rel` in the important index AND write a starter copy to
/// both the project side and the internal mirror.
fn register_important(root: &Path, rel: &str, content: &str) {
    let proj = project_path_for(root, rel);
    if let Some(parent) = proj.parent() {
        fs::create_dir_all(parent).unwrap();
    }
    fs::write(&proj, content).unwrap();

    let mut idx = ImportantIndex::load(root).unwrap();
    idx.register(root, rel).unwrap();

    // register() also mirrors to internal, but we want to control the
    // internal copy independently for these tests — overwrite.
    let internal = internal_path_for(root, rel);
    if let Some(parent) = internal.parent() {
        fs::create_dir_all(parent).unwrap();
    }
    fs::write(&internal, content).unwrap();
}

fn assert_state(root: &Path, rel: &str, expected: SyncState) {
    let idx = ImportantIndex::load(root).unwrap();
    let entry = idx
        .files
        .get(rel)
        .unwrap_or_else(|| panic!("index missing entry {rel}"));
    assert_eq!(
        entry.sync_state, expected,
        "expected {expected:?} for {rel}, got {:?}",
        entry.sync_state
    );
}

fn assert_scan_stats(result: &ScanResult, scanned: u32, synced: u32) {
    assert_eq!(result.stats.scanned, scanned, "scanned mismatch");
    assert_eq!(result.stats.synced, synced, "synced mismatch");
}

// ---- classify() ----

#[test]
fn classify_both_gone_returns_recorded_or_synced() {
    let s = conflict::classify(false, false, "", "", Some(SyncState::InternalOnly));
    assert_eq!(s, SyncState::InternalOnly);
    let s = conflict::classify(false, false, "", "", None);
    assert_eq!(s, SyncState::Synced);
}

#[test]
fn classify_project_only_when_internal_missing() {
    let s = conflict::classify(true, false, "x", "", None);
    assert_eq!(s, SyncState::ProjectOnly);
}

#[test]
fn classify_internal_only_when_project_missing() {
    let s = conflict::classify(false, true, "", "x", None);
    assert_eq!(s, SyncState::InternalOnly);
}

#[test]
fn classify_synced_when_hashes_match() {
    let s = conflict::classify(true, true, "abc", "abc", None);
    assert_eq!(s, SyncState::Synced);
}

#[test]
fn classify_conflict_when_hashes_differ() {
    let s = conflict::classify(true, true, "abc", "xyz", None);
    assert_eq!(s, SyncState::Conflict);
}

// ---- detect() — happy path ----

#[test]
fn detect_on_empty_index_is_noop() {
    let tmp = fresh_project();
    let root = tmp.path();
    let r = conflict::detect(root).expect("detect");
    assert_eq!(r.stats.scanned, 0);
    assert!(r.pairs.is_empty());
}

#[test]
fn detect_marks_matching_pair_as_synced() {
    let tmp = fresh_project();
    let root = tmp.path();
    register_important(root, "src/main.rs", "fn main() {}\n");
    let r = conflict::detect(root).expect("detect");
    assert_scan_stats(&r, 1, 1);
    assert_state(root, "src/main.rs", SyncState::Synced);
}

// ---- detect() — conflict creation ----

#[test]
fn detect_creates_conflict_when_hashes_differ() {
    let tmp = fresh_project();
    let root = tmp.path();
    register_important(root, "src/main.rs", "version 1\n");

    // Now make the project-side diverge.
    fs::write(project_path_for(root, "src/main.rs"), "version 2 from project\n").unwrap();

    let r = conflict::detect(root).expect("detect");
    assert_eq!(r.stats.scanned, 1);
    assert_eq!(r.stats.conflicts_new, 1);
    assert_eq!(r.stats.synced, 0);

    // The pair should reference the new conflict id.
    let pair = &r.pairs[0];
    assert_eq!(pair.sync_state, SyncState::Conflict);
    let cid = pair.conflict_id.clone().expect("conflict_id missing");

    // Backup files should exist.
    let dir = root.join("kron-internal").join("conflicts").join(&cid);
    assert!(dir.join("project_version").is_file());
    assert!(dir.join("internal_version").is_file());
    assert!(dir.join("_record.json").is_file());

    // The conflict index should include the id.
    let pending = list_by_status(root, "pending").unwrap();
    assert_eq!(pending.len(), 1);
    assert_eq!(pending[0].id, cid);
    assert_eq!(pending[0].status, ConflictStatus::Pending);
    assert_eq!(pending[0].relative_path, "src/main.rs");
}

#[test]
fn detect_is_idempotent_for_existing_pending_conflict() {
    let tmp = fresh_project();
    let root = tmp.path();
    register_important(root, "src/lib.rs", "alpha\n");

    fs::write(project_path_for(root, "src/lib.rs"), "beta\n").unwrap();
    let r1 = conflict::detect(root).expect("detect 1");
    assert_eq!(r1.stats.conflicts_new, 1);

    // Second scan should NOT create another conflict.
    let r2 = conflict::detect(root).expect("detect 2");
    assert_eq!(r2.stats.scanned, 1);
    assert_eq!(r2.stats.conflicts_new, 0);
    assert_eq!(r2.stats.conflicts_existing, 1);
}

// ---- detect() — single-sided states ----

#[test]
fn detect_marks_project_only_when_internal_missing() {
    let tmp = fresh_project();
    let root = tmp.path();
    register_important(root, "docs/readme.md", "hello\n");
    fs::remove_file(internal_path_for(root, "docs/readme.md")).unwrap();

    let r = conflict::detect(root).expect("detect");
    assert_eq!(r.stats.project_only, 1);
    assert_state(root, "docs/readme.md", SyncState::ProjectOnly);
}

#[test]
fn detect_marks_internal_only_when_project_missing() {
    let tmp = fresh_project();
    let root = tmp.path();
    register_important(root, "src/utils.rs", "fn x() {}\n");
    fs::remove_file(project_path_for(root, "src/utils.rs")).unwrap();

    let r = conflict::detect(root).expect("detect");
    assert_eq!(r.stats.internal_only, 1);
    assert_state(root, "src/utils.rs", SyncState::InternalOnly);
}

// ---- resolve() ----

#[test]
fn resolve_use_project_copies_project_backup_over_both_sides() {
    let tmp = fresh_project();
    let root = tmp.path();
    register_important(root, "src/main.rs", "internal-version\n");

    fs::write(project_path_for(root, "src/main.rs"), "project-version\n").unwrap();
    let r = conflict::detect(root).expect("detect");
    let cid = r.pairs[0].conflict_id.clone().unwrap();

    let updated = resolve(root, &cid, ConflictResolution::UseProject).expect("resolve");
    assert_eq!(updated.status, ConflictStatus::Resolved);
    assert_eq!(updated.resolution, Some(ConflictResolution::UseProject));
    assert!(updated.resolved_at.is_some());

    // Both sides now contain the project backup bytes.
    let proj = fs::read_to_string(project_path_for(root, "src/main.rs")).unwrap();
    let intl = fs::read_to_string(internal_path_for(root, "src/main.rs")).unwrap();
    assert_eq!(proj, "project-version\n");
    assert_eq!(intl, "project-version\n");

    // Index entry should be Synced now.
    assert_state(root, "src/main.rs", SyncState::Synced);

    // No pending conflicts remain.
    let pending = list_by_status(root, "pending").unwrap();
    assert!(pending.is_empty(), "expected no pending conflicts, got {pending:?}");

    // Resolved list has the conflict.
    let resolved = list_by_status(root, "resolved").unwrap();
    assert_eq!(resolved.len(), 1);
    assert_eq!(resolved[0].id, cid);
}

#[test]
fn resolve_use_internal_copies_internal_over_both_sides() {
    let tmp = fresh_project();
    let root = tmp.path();
    register_important(root, "src/main.rs", "internal-version\n");

    fs::write(project_path_for(root, "src/main.rs"), "project-version\n").unwrap();
    let r = conflict::detect(root).expect("detect");
    let cid = r.pairs[0].conflict_id.clone().unwrap();

    let updated = resolve(root, &cid, ConflictResolution::UseInternal).expect("resolve");
    assert_eq!(updated.status, ConflictStatus::Resolved);

    let proj = fs::read_to_string(project_path_for(root, "src/main.rs")).unwrap();
    let intl = fs::read_to_string(internal_path_for(root, "src/main.rs")).unwrap();
    assert_eq!(proj, "internal-version\n");
    assert_eq!(intl, "internal-version\n");
}

#[test]
fn resolve_ignore_keeps_both_sides_and_marks_ignored() {
    let tmp = fresh_project();
    let root = tmp.path();
    register_important(root, "src/main.rs", "internal-version\n");
    fs::write(project_path_for(root, "src/main.rs"), "project-version\n").unwrap();

    let r = conflict::detect(root).expect("detect");
    let cid = r.pairs[0].conflict_id.clone().unwrap();

    let updated = resolve(root, &cid, ConflictResolution::Ignore).expect("resolve");
    assert_eq!(updated.status, ConflictStatus::Ignored);
    assert_eq!(updated.resolution, Some(ConflictResolution::Ignore));

    // Both sides unchanged.
    let proj = fs::read_to_string(project_path_for(root, "src/main.rs")).unwrap();
    let intl = fs::read_to_string(internal_path_for(root, "src/main.rs")).unwrap();
    assert_eq!(proj, "project-version\n");
    assert_eq!(intl, "internal-version\n");

    // Index still Conflict (ignored does not heal the state).
    assert_state(root, "src/main.rs", SyncState::Conflict);
}

#[test]
fn resolve_twice_fails_on_second_call() {
    let tmp = fresh_project();
    let root = tmp.path();
    register_important(root, "src/main.rs", "internal-version\n");
    fs::write(project_path_for(root, "src/main.rs"), "project-version\n").unwrap();
    let r = conflict::detect(root).expect("detect");
    let cid = r.pairs[0].conflict_id.clone().unwrap();

    let _ = resolve(root, &cid, ConflictResolution::UseProject).expect("first resolve");
    let err = resolve(root, &cid, ConflictResolution::UseInternal);
    assert!(err.is_err(), "second resolve must fail");
}

// ---- daemon ----

#[test]
fn daemon_start_registers_pid_and_status() {
    let tmp = fresh_project();
    let root = tmp.path();
    assert!(!daemon::is_running(root));

    let outcome = daemon::start(root).expect("start");
    assert!(outcome.pid > 0);
    assert!(daemon::is_running(root));

    let st = daemon::status(root).expect("status");
    let st: DaemonStatus = st.expect("status not None");
    assert_eq!(st.pid, std::process::id());
    assert!(st.last_scan.is_some());
    assert!(st.last_scan_at.is_some());
}

#[test]
fn daemon_start_twice_fails() {
    let tmp = fresh_project();
    let root = tmp.path();
    let _ = daemon::start(root).expect("start 1");
    let err = daemon::start(root);
    assert!(err.is_err(), "second start must fail");
}

#[test]
fn daemon_stop_removes_marker() {
    let tmp = fresh_project();
    let root = tmp.path();
    let _ = daemon::start(root).expect("start");
    assert!(daemon::is_running(root));
    let removed = daemon::stop(root).expect("stop");
    assert!(removed);
    assert!(!daemon::is_running(root));
}

#[test]
fn daemon_stop_when_not_running_is_idempotent() {
    let tmp = fresh_project();
    let root = tmp.path();
    let removed = daemon::stop(root).expect("stop (first)");
    assert!(!removed);
    let removed = daemon::stop(root).expect("stop (second)");
    assert!(!removed);
}

// ---- end-to-end ----

#[test]
fn full_lifecycle_init_to_resolved_conflict() {
    let tmp = fresh_project();
    let root = tmp.path();

    // 1. Register two important files, both synced.
    register_important(root, "src/main.rs", "v1\n");
    register_important(root, "docs/readme.md", "first\n");

    let r = daemon::start(root).expect("daemon start");
    assert_eq!(r.scan.scanned, 2);
    assert_eq!(r.scan.synced, 2);

    // 2. Diverge one file on the project side.
    fs::write(project_path_for(root, "src/main.rs"), "v2-from-project\n").unwrap();

    let r2 = conflict::detect(root).expect("detect after divergence");
    assert_eq!(r2.stats.conflicts_new, 1);

    let pending = list_by_status(root, "pending").expect("list pending");
    assert_eq!(pending.len(), 1);
    let cid = pending[0].id.clone();

    // 3. Show the conflict — record file is readable, backup files are sane.
    let rec = conflict::load_record(root, &cid).expect("load record");
    let rec = rec.expect("record Some");
    let proj_bytes = fs::read(&rec.project_backup).unwrap();
    let int_bytes = fs::read(&rec.internal_backup).unwrap();
    assert_eq!(proj_bytes, b"v2-from-project\n");
    assert_eq!(int_bytes, b"v1\n");

    // 4. Resolve using the project version.
    let updated = resolve(root, &cid, ConflictResolution::UseProject).expect("resolve");
    assert_eq!(updated.status, ConflictStatus::Resolved);

    // 5. Re-scan finds no pending conflicts.
    let r3 = conflict::detect(root).expect("post-resolve scan");
    assert_eq!(r3.stats.conflicts_new, 0);
    assert_eq!(r3.stats.conflicts_existing, 0);
    let pending = list_by_status(root, "pending").unwrap();
    assert!(pending.is_empty());

    // 6. The daemon status still records the last scan.
    let st = daemon::status(root).expect("status");
    let st = st.expect("status Some");
    assert!(st.last_scan.is_some());
}
