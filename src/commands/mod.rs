//! Command implementations.
//!
//! Each submodule corresponds to one top-level kron subcommand
//! (or a sub-subcommand group like `task list`).

pub mod config;
pub mod conflict;
pub mod context;
pub mod important;
pub mod init;
pub mod list_projects;
pub mod path;
pub mod status;
pub mod task;
pub mod vertex;

use std::path::{Path, PathBuf};

use crate::core;
use crate::error::{KronError, Result};
use crate::output::{self, OutputMode};
use serde::Serialize;

/// Shared per-command context.
///
/// Every command receives this so it can decide whether to emit
/// human/JSON/porcelain output, propagate verbose flags, and resolve
/// the Kron project root without each subcommand re-implementing
/// ancestor search.
#[derive(Debug, Clone)]
pub struct Ctx {
    pub mode: OutputMode,
    pub verbose: bool,
    /// Kron project root, resolved by [`Ctx::resolve`] (which calls
    /// `core::project::find_project_root`). `None` until resolved.
    pub project_root: Option<PathBuf>,
}

impl Ctx {
    pub fn new(mode: OutputMode, verbose: bool) -> Self {
        Self {
            mode,
            verbose,
            project_root: None,
        }
    }

    /// Resolve and cache the project root from the current working
    /// directory. Subsequent calls return the cached value.
    ///
    /// Returns `Err(KronError::NotAProject(cwd))` if no project is found
    /// in the cwd or any ancestor directory.
    pub fn resolve(&mut self) -> Result<&Path> {
        if self.project_root.is_none() {
            let cwd = std::env::current_dir().map_err(KronError::Io)?;
            self.project_root = core::project::find_project_root(&cwd);
        }
        match self.project_root.as_deref() {
            Some(p) => Ok(p),
            None => {
                let cwd = std::env::current_dir().map_err(KronError::Io)?;
                Err(KronError::NotAProject(cwd))
            }
        }
    }
}

/// Helper used by every command body: returns the cached project root or
/// resolves it on the fly. Thin wrapper to keep callsites concise.
///
/// All commands should call this **once** at the top of their handler
/// rather than rolling their own `current_dir()` + marker check.
pub fn require_project_root(ctx: &Ctx) -> Result<PathBuf> {
    if let Some(p) = &ctx.project_root {
        return Ok(p.clone());
    }
    let cwd = std::env::current_dir().map_err(KronError::Io)?;
    core::project::find_project_root(&cwd).ok_or(KronError::NotAProject(cwd))
}

// ============================================================================
// Ctx output helpers — Day 4/5 (Anchor #8) Plan B.
//
// These methods let a command body say `ctx.json(&payload)?` instead of
// `match ctx.mode { OutputMode::Json => { println!(...) } ... }`. The
// helpers DO NOTHING when the mode doesn't match — they're a 4:1 collapse
// for the JSON arm, and 1:1 for porcelain/human. As of Day 5, ALL
// `match ctx.mode` blocks in commands/ have been migrated to these methods
// (38 blocks eliminated across 10 files). See dev-journal/2026-09-07-day-5.
// ============================================================================

impl Ctx {
    /// Emit a value as JSON **only when in `Json` mode**. In `Porcelain`
    /// or `Human` mode, the call is a no-op.
    ///
    /// Use this when the command has a single JSON payload but different
    /// (command-specific) Porcelain/Human renderings. For commands that
    /// want one method across all three modes, use
    /// [`output::emit_record`]/[`output::emit_records`] instead.
    pub fn json<T: Serialize + ?Sized>(&self, value: &T) -> Result<()> {
        if self.mode == OutputMode::Json {
            output::emit_json(value)?;
        }
        Ok(())
    }

    /// Emit a porcelain line **only when in `Porcelain` mode**.
    pub fn porcelain(&self, line: impl std::fmt::Display) {
        if self.mode == OutputMode::Porcelain {
            println!("{line}");
        }
    }

    /// Emit a human line **only when in `Human` mode**.
    pub fn human(&self, line: impl AsRef<str>) {
        if self.mode == OutputMode::Human {
            println!("{}", line.as_ref());
        }
    }

    /// Print a success marker (`✓ ...`) in Human mode only.
    pub fn success(&self, msg: &str) {
        if self.mode == OutputMode::Human {
            println!("\u{2713} {msg}");
        }
    }

    /// Print an informational line in Human mode only.
    pub fn info(&self, msg: &str) {
        if self.mode == OutputMode::Human {
            println!("{msg}");
        }
    }

    /// Print a warning to **stderr regardless of mode**. Use this when
    /// the warning should be visible even in `--json` / `--porcelain`
    /// mode (e.g. orphan pointer after vertex delete).
    pub fn warning(&self, msg: &str) {
        eprintln!("warning: {msg}");
    }
}
