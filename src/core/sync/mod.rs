//! Sync engine: conflict detection, daemon PID lock, single-scan driver.
//!
//! P2 simplified model (matches dev-docs/design/07-实施路线图.md § 4):
//!
//! - **Daemon** is a single-instance "sentinel" identified by a PID file
//!   under `kron-internal/.daemon.pid`. There is no real background
//!   process for the M2 demo; `daemon start` performs one scan and
//!   retains the PID marker so `daemon status` / `daemon stop` work.
//! - **Conflict detection** is a synchronous scan over the important
//!   files (registered via the per-project `ImportantIndex`). The
//!   scanner compares MD5 + mtime and emits `SyncPair`s plus creates
//!   `ConflictRecord`s when both copies differ.
//! - **Conflict resolution** (`UseProject` / `UseInternal` / `Ignore`)
//!   rewrites the chosen copy, deletes the other, removes backups,
//!   and marks the record as resolved.
//!
//! The 5-minute polling loop, `notify` crate integration and full
//! background daemon are deferred to v2 — they add a lot of Windows-
//! specific complexity for limited additional value during M2.

pub mod conflict;
pub mod daemon;
pub mod scan;
pub mod sync_index;
