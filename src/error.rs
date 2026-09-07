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
    #[error(
        "not a Kron project: {0} (no `kron-internal/config.json` found in this directory or any parent)"
    )]
    NotAProject(PathBuf),

    /// `kron init` was attempted but an ancestor directory is already a Kron project.
    /// Mirrors Git's refusal to nest repositories.
    #[error(
        "cannot initialize Kron project at `{attempted}`: \
         an ancestor project already exists at `{ancestor}`"
    )]
    AncestorProject {
        attempted: PathBuf,
        ancestor: PathBuf,
    },

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

    /// JSON serialization/deserialization failure.
    #[error("json error: {0}")]
    Json(#[from] serde_json::Error),

    /// YAML serialization/deserialization failure.
    #[error("yaml error: {0}")]
    Yaml(#[from] serde_yaml::Error),

    /// Catch-all for unexpected internal errors (should be rare).
    #[error("internal error: {0}")]
    Internal(String),

    // ---- Phase 2 (CLI skeleton) additions ----

    /// Workspace is already initialized; user must pass --force to re-init.
    #[error("already initialized at {0} (use --force to re-initialize)")]
    AlreadyInitialized(PathBuf),

    /// Operation requires a Git repository.
    #[error("not a Git repository: {0} (use --no-git to bypass)")]
    NotGitRepo(PathBuf),

    /// CLI-level error (invalid args, conflicting flags, ...).
    #[error("cli: {0}")]
    Cli(String),

    /// CLI-level error with a custom message (not from the enum).
    #[error("{0}")]
    CliCustom(String),

    /// A feature is not yet implemented (placeholder for stub commands).
    #[error("not yet implemented: {0}")]
    NotYetImplemented(&'static str),

    /// A text file could not be parsed into the expected structure.
    #[error("parse error: {0}")]
    Parse(String),
}

/// Convenient Result alias.
pub type Result<T> = std::result::Result<T, KronError>;

impl KronError {
    /// Map a [`KronError`] to a process exit code.
    ///
    /// Anchored to the table in `dev-docs/design/04b-CLI设计.md` § 6. We
    /// haven't wired this into `main()` yet (that's Day 3 of Anchor #8)
    /// but having the function here keeps callers honest while they
    /// switch over.
    pub fn exit_code(&self) -> i32 {
        match self {
            // Cli-style errors: bad arguments, conflicting flags,
            // invalid state strings, invalid task / vertex names.
            Self::Cli(_)
            | Self::CliCustom(_)
            | Self::InvalidFrontmatter { .. }
            | Self::InvalidVertexName(_)
            | Self::InvalidTaskFilename(_)
            | Self::DuplicateTaskId(_) => 2,

            // Workspace not initialised (or attempted init refused
            // because of an ancestor project).
            Self::NotAProject(_)
            | Self::AlreadyInitialized(_)
            | Self::NotGitRepo(_)
            | Self::AncestorProject { .. } => 4,

            // Filesystem / parsing / permission errors.
            Self::PermissionDenied(_) | Self::Parse(_) => 9,
            Self::Io(_) | Self::Json(_) | Self::Yaml(_) => 6,

            // Resource-not-found is a single exit code regardless of
            // which resource — callers can disambiguate via stderr.
            Self::NotFound(_) => 8,

            // Catch-all / unclassified. We deliberately avoid 0 here.
            Self::Internal(_) | Self::NotYetImplemented(_) => 1,
        }
    }
}

#[cfg(test)]
mod exit_code_tests {
    use super::*;

    #[test]
    fn cli_variants_exit_2() {
        assert_eq!(KronError::Cli("x".into()).exit_code(), 2);
        assert_eq!(
            KronError::InvalidFrontmatter {
                file: PathBuf::from("x"),
                reason: "x".into()
            }
            .exit_code(),
            2
        );
        assert_eq!(KronError::InvalidVertexName("x".into()).exit_code(), 2);
        assert_eq!(KronError::InvalidTaskFilename("x".into()).exit_code(), 2);
        assert_eq!(KronError::DuplicateTaskId("x".into()).exit_code(), 2);
    }

    #[test]
    fn not_initialised_variants_exit_4() {
        assert_eq!(KronError::NotAProject(PathBuf::from("x")).exit_code(), 4);
        assert_eq!(KronError::AlreadyInitialized(PathBuf::from("x")).exit_code(), 4);
        assert_eq!(KronError::NotGitRepo(PathBuf::from("x")).exit_code(), 4);
        assert_eq!(
            KronError::AncestorProject {
                attempted: PathBuf::from("x"),
                ancestor: PathBuf::from("y"),
            }
            .exit_code(),
            4
        );
    }

    #[test]
    fn not_found_exits_8() {
        assert_eq!(KronError::NotFound(PathBuf::from("x")).exit_code(), 8);
    }

    #[test]
    fn permission_exits_9_io_exits_6() {
        assert_eq!(KronError::PermissionDenied(PathBuf::from("x")).exit_code(), 9);
    }
}

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
