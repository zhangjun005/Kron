//! Sync engine: conflict detection and resolution.
//!
//! P2 simplified model (matches dev-docs/design/07-实施路线图.md § 4):
//!
//! - **Conflict detection** is a synchronous scan over the important
//!   files (registered via the per-project `ImportantIndex`). The
//!   scanner compares MD5 + mtime and emits `SyncPair`s plus creates
//!   `ConflictRecord`s when both copies differ.
//! - **Conflict resolution** (`UseProject` / `UseInternal` / `Ignore`)
//!   rewrites the chosen copy, deletes the other, removes backups,
//!   and marks the record as resolved.
//!
//! All operations are synchronous. There is no background daemon —
//! users trigger scans explicitly via `kron sync` or `kron conflict detect`.

pub mod conflict;
pub mod scan;
pub mod sync_index;
