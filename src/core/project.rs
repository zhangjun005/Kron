//! Locate the Kron project root from an arbitrary starting path.
//!
//! Mirrors Git's ancestor-search contract:
//!
//! * `find_project_root(start)` walks upward until it finds a directory
//!   containing [`PROJECT_MARKER`]. Returns `None` if no project is found
//!   before reaching the filesystem root.
//! * `find_ancestor_project(start)` walks upward *excluding* `start`, used
//!   by `kron init` to refuse nested initialization.
//! * `current_project_root()` is a convenience wrapper around
//!   `find_project_root` using `std::env::current_dir()`.
//!
//! Centralizing these rules guarantees every subcommand shares a single
//! definition of "what is a Kron project, and where is it rooted?".

use std::path::{Path, PathBuf};

/// Marker file that identifies a directory as a Kron project root.
///
/// Stored at `<project>/kron-internal/config.json` by `kron init`.
pub const PROJECT_MARKER: &str = "kron-internal/config.json";

/// Walk upward from `start` to find the nearest ancestor (including `start`
/// itself) that contains [`PROJECT_MARKER`].
///
/// Returns `None` if no project is found before the filesystem root.
///
/// This is the Kron analogue of Git's `discover_git_directory`.
pub fn find_project_root(start: &Path) -> Option<PathBuf> {
    let mut cur: Option<&Path> = Some(start);
    while let Some(dir) = cur {
        if is_project_root(dir) {
            return Some(dir.to_path_buf());
        }
        cur = dir.parent();
    }
    None
}

/// Walk upward from `start` (excluding `start` itself) to detect if any
/// ancestor directory already hosts a Kron project. Used by `kron init`
/// to reject nested initialization.
///
/// Returns `Some(ancestor_path)` on conflict, `None` otherwise.
pub fn find_ancestor_project(start: &Path) -> Option<PathBuf> {
    let mut cur = start.parent()?;
    loop {
        if is_project_root(cur) {
            return Some(cur.to_path_buf());
        }
        match cur.parent() {
            Some(parent) if parent != cur => cur = parent,
            _ => return None,
        }
    }
}

/// Resolve the current Kron project root from the process's working
/// directory.
///
/// Returns `Err(KronError::NotAProject(cwd))` if no project is found.
pub fn current_project_root() -> Result<PathBuf, crate::error::KronError> {
    let cwd = std::env::current_dir().map_err(crate::error::KronError::Io)?;
    find_project_root(&cwd).ok_or(crate::error::KronError::NotAProject(cwd))
}

/// Strict check: directory contains a syntactically valid marker file.
///
/// A bare `kron-internal/config.json` file is NOT enough — it must also be
/// parseable JSON. This prevents false positives from unrelated files
/// that happen to live under a `kron-internal/` path by coincidence.
fn is_project_root(dir: &Path) -> bool {
    let marker = dir.join(PROJECT_MARKER);
    if !marker.is_file() {
        return false;
    }
    match std::fs::read_to_string(&marker) {
        Ok(contents) => serde_json::from_str::<serde_json::Value>(&contents).is_ok(),
        Err(_) => false,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::fs;

    /// Build a unique temporary directory under the OS temp dir.
    fn tempdir() -> PathBuf {
        let base = std::env::temp_dir();
        let unique = format!(
            "kron-test-{}-{}",
            std::process::id(),
            std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .map(|d| d.as_nanos())
                .unwrap_or(0)
        );
        let p = base.join(unique);
        fs::create_dir_all(&p).unwrap();
        p
    }

    /// Drop a valid project marker at `dir` so it counts as a project root.
    fn mark_project(dir: &Path) {
        fs::create_dir_all(dir.join("kron-internal")).unwrap();
        // Minimal valid JSON object — the strictness check requires parseable JSON.
        fs::write(dir.join("kron-internal/config.json"), "{}").unwrap();
    }

    #[test]
    fn finds_root_at_start() {
        let tmp = tempdir();
        mark_project(&tmp);
        assert_eq!(find_project_root(&tmp), Some(tmp.clone()));
    }

    #[test]
    fn walks_upward() {
        let tmp = tempdir();
        mark_project(&tmp);
        let deep = tmp.join("src").join("commands");
        fs::create_dir_all(&deep).unwrap();
        assert_eq!(find_project_root(&deep), Some(tmp.clone()));
    }

    #[test]
    fn none_when_no_project() {
        let tmp = tempdir();
        let deep = tmp.join("a").join("b");
        fs::create_dir_all(&deep).unwrap();
        assert_eq!(find_project_root(&deep), None);
    }

    #[test]
    fn rejects_invalid_marker() {
        // A file exists but is not valid JSON — must NOT be considered a project.
        let tmp = tempdir();
        fs::create_dir_all(tmp.join("kron-internal")).unwrap();
        fs::write(tmp.join("kron-internal/config.json"), "not json {").unwrap();
        assert_eq!(find_project_root(&tmp), None);
    }

    #[test]
    fn ancestor_detected_excluding_self() {
        let tmp = tempdir();
        mark_project(&tmp);
        let child = tmp.join("child");
        fs::create_dir_all(&child).unwrap();

        // child has no marker of its own, but its parent does.
        assert_eq!(find_ancestor_project(&child), Some(tmp.clone()));

        // self is excluded — find_ancestor_project on the project root itself
        // must not report the root as its own ancestor.
        assert_eq!(find_ancestor_project(&tmp), None);
    }
}
