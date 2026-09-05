//! Command implementations.
//!
//! Each submodule corresponds to one top-level kron subcommand
//! (or a sub-subcommand group like `task list`).

pub mod config;
pub mod conflict;
pub mod context;
pub mod daemon;
pub mod important;
pub mod init;
pub mod list_projects;
pub mod path;
pub mod status;
pub mod task;
pub mod vertex;

use crate::output::OutputMode;

/// Shared per-command context.
///
/// Every command receives this so it can decide whether to emit
/// human/JSON/porcelain output, propagate verbose flags, etc.
#[derive(Debug, Clone, Copy)]
pub struct Ctx {
    pub mode: OutputMode,
    pub verbose: bool,
}

impl Ctx {
    pub fn new(mode: OutputMode, verbose: bool) -> Self {
        Self { mode, verbose }
    }
}
