//! Kron error types and Result alias.
//!
//! All fallible operations in kron return `Result<T, KronError>`.
//! Errors are user-facing: every variant's Display must be a clean message
//! that can be shown in CLI / GUI / AI agent context.

use std::path::PathBuf;
use thiserror::Error;

/// Top-level error type for kron.
#[derive(Debug, Error)]
pub enum KronError {
    /// A required directory or file does not exist.
    #[error("path not found: {0}")]
    NotFound(PathBuf),

    /// Tried to operate on a path that is not inside a Kron project.
    #[error("not a Kron project: {0} (missing KRON/ directory)")]
    NotAProject(PathBuf),

    /// Frontmatter missing required fields, or field has invalid value.
    #[error("invalid frontmatter in {file}: {reason}")]
    InvalidFrontmatter { file: PathBuf, reason: String },

    /// Vertex directory name violates the slug rules.
    #[error("invalid vertex name: {0:?} (expected slug: [a-z0-9-_])")]
    InvalidVertexName(String),

    /// Task filename does not match the expected pattern.
    #[error("invalid task filename: {0:?} (expected t-<yyyy>-<seq>-<slug>.md)")]
    InvalidTaskFilename(String),

    /// Two tasks claim the same ID (data corruption or migration mistake).
    #[error("duplicate task id: {0}")]
    DuplicateTaskId(String),

    /// Permission denied reading or writing a file.
    #[error("permission denied: {0}")]
    PermissionDenied(PathBuf),

    /// I/O error from the standard library.
    #[error("io error: {0}")]
    Io(#[from] std::io::Error),

    /// Catch-all for unexpected internal errors (should be rare).
    #[error("internal error: {0}")]
    Internal(String),
}

/// Convenient Result alias.
pub type Result<T> = std::result::Result<T, KronError>;

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn error_display_does_not_leak_internal_paths_to_user_messages() {
        // Display strings must be human-readable, not panic-inducing.
        let err = KronError::NotAProject(PathBuf::from("/tmp/foo"));
        let s = format!("{err}");
        assert!(s.contains("not a Kron project"));
        assert!(s.contains("/tmp/foo"));
    }

    #[test]
    fn error_implements_std_error_trait() {
        fn assert_error<E: std::error::Error>() {}
        assert_error::<KronError>();
    }
}
