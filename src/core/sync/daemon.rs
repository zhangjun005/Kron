//! Daemon PID lock and one-shot scan driver.
//!
//! The M2 implementation is intentionally simple:
//!
//! - A PID file at `kron-internal/.daemon.pid` marks "the daemon is
//!   registered for this project".
//! - `start()` acquires the lock (failing if it's already held by a
//!   live process), runs one full scan, and returns. The PID stays
//!   on disk so subsequent `status()` / `stop()` calls can find it.
//! - `stop()` removes the PID file (no live process to signal in M2).
//! - `status()` returns PID + uptime + last scan timestamp.
//!
//! This is sufficient to demonstrate the M2 CLI contract
//! (`kron daemon start/status/stop`) without pulling in cross-platform
//! process-spawning code. A full background daemon with `notify`-
//! driven watchers arrives in v2.

use crate::core::sync::conflict;
use crate::error::{KronError, Result};
use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use std::fs;
use std::path::{Path, PathBuf};
use std::time::Instant;

/// Path to the daemon PID file, relative to the project root.
pub fn pid_path(project_root: &Path) -> PathBuf {
    project_root.join("kron-internal").join(".daemon.pid")
}

/// Path to the daemon status JSON file.
pub fn status_path(project_root: &Path) -> PathBuf {
    project_root.join("kron-internal").join(".daemon.status.json")
}

/// Persisted daemon status snapshot.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct DaemonStatus {
    pub pid: u32,
    pub started_at: DateTime<Utc>,
    pub last_scan_at: Option<DateTime<Utc>>,
    pub last_scan: Option<ScanSummary>,
}

/// Compact scan summary stored in the daemon status file.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ScanSummary {
    pub scanned: u32,
    pub synced: u32,
    pub conflicts_new: u32,
    pub conflicts_existing: u32,
    pub internal_only: u32,
    pub project_only: u32,
}

impl From<conflict::ScanStats> for ScanSummary {
    fn from(s: conflict::ScanStats) -> Self {
        Self {
            scanned: s.scanned,
            synced: s.synced,
            conflicts_new: s.conflicts_new,
            conflicts_existing: s.conflicts_existing,
            internal_only: s.internal_only,
            project_only: s.project_only,
        }
    }
}

/// Outcome of `start()`.
#[derive(Debug, Clone, Serialize)]
pub struct StartOutcome {
    pub pid: u32,
    pub started_at: DateTime<Utc>,
    pub scan: ScanSummary,
    pub note: &'static str,
}

/// Acquire the lock + write the PID file. Internal helper.
fn write_pid(project_root: &Path, pid: u32) -> Result<()> {
    let path = pid_path(project_root);
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent)?;
    }
    fs::write(&path, pid.to_string())?;
    Ok(())
}

/// Inspect an existing PID file: is the recorded PID still alive?
fn pid_alive(pid: u32) -> bool {
    // We don't actually have a real background process in M2. Treat any
    // recorded PID > 0 as "live enough" so subsequent commands see the
    // daemon as registered. The flag is mostly a courtesy check for
    // when a real daemon is wired up in v2.
    pid > 0
}

/// Return the current daemon status, or `None` if not registered.
pub fn status(project_root: &Path) -> Result<Option<DaemonStatus>> {
    let path = status_path(project_root);
    if !path.exists() {
        return Ok(None);
    }
    let raw = fs::read_to_string(&path)?;
    if raw.trim().is_empty() {
        return Ok(None);
    }
    let st: DaemonStatus = serde_json::from_str(&raw)?;
    Ok(Some(st))
}

fn write_status(project_root: &Path, st: &DaemonStatus) -> Result<()> {
    let path = status_path(project_root);
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent)?;
    }
    let json = serde_json::to_string_pretty(st)?;
    fs::write(&path, json)?;
    Ok(())
}

/// True if a daemon status file exists and records a live PID.
pub fn is_running(project_root: &Path) -> bool {
    match status(project_root) {
        Ok(Some(st)) => pid_alive(st.pid),
        _ => false,
    }
}

/// Start the daemon: write PID, run one scan, persist status.
///
/// Fails with `KronError::Cli("daemon already running...")` if a status
/// file already exists.
pub fn start(project_root: &Path) -> Result<StartOutcome> {
    if let Some(existing) = status(project_root)? {
        return Err(KronError::Cli(format!(
            "daemon already registered (pid {}, started {}) — use `kron daemon stop` first",
            existing.pid, existing.started_at
        )));
    }

    let pid = std::process::id();
    let started_at = Utc::now();
    let _t0 = Instant::now();

    // Run the scan first; if it fails we abort before writing the
    // status file so the caller doesn't think the daemon is healthy.
    let scan = conflict::detect(project_root)?;

    write_pid(project_root, pid)?;

    let st = DaemonStatus {
        pid,
        started_at,
        last_scan_at: Some(Utc::now()),
        last_scan: Some(scan.stats.clone().into()),
    };
    write_status(project_root, &st)?;

    Ok(StartOutcome {
        pid,
        started_at,
        scan: scan.stats.into(),
        note: "M2 demo: a real background process will arrive in v2. The PID marker is registered so `status`/`stop` work.",
    })
}

/// Stop the daemon: remove PID + status files. Idempotent.
pub fn stop(project_root: &Path) -> Result<bool> {
    let pid = pid_path(project_root);
    let st = status_path(project_root);
    let mut removed = false;
    if pid.exists() {
        fs::remove_file(&pid)?;
        removed = true;
    }
    if st.exists() {
        fs::remove_file(&st)?;
        removed = true;
    }
    Ok(removed)
}
