//! `kron important` — important-file management (P4 stub).

use clap::{Args, Subcommand};

use crate::commands::Ctx;
use crate::error::{KronError, Result};

#[derive(Debug, Args)]
pub struct ImportantArgs {
    #[command(subcommand)]
    pub action: ImportantAction,
}

#[derive(Debug, Subcommand)]
pub enum ImportantAction {
    /// List important files.
    List {
        #[arg(long)]
        tag: Vec<String>,
    },
    /// Add a file to important/.
    Add {
        path: String,
        #[arg(long)]
        copy: bool,
        #[arg(long)]
        symlink: bool,
        #[arg(long)]
        tag: Vec<String>,
        #[arg(long, short = 'm')]
        message: Option<String>,
    },
    /// Remove from important/ (kron-internal only).
    Remove {
        path: String,
        #[arg(long)]
        force: bool,
        #[arg(long)]
        also_delete_source: bool,
    },
    /// Force bidirectional sync.
    Sync {
        #[arg(long)]
        dry_run: bool,
        #[arg(long, value_enum)]
        direction: Option<SyncDirection>,
    },
}

#[derive(Debug, Clone, Copy, clap::ValueEnum)]
pub enum SyncDirection {
    #[clap(name = "project->internal")]
    ProjectToInternal,
    #[clap(name = "internal->project")]
    InternalToProject,
    #[clap(name = "both")]
    Both,
}

pub fn run(_ctx: Ctx, args: ImportantArgs) -> Result<()> {
    let op = match &args.action {
        ImportantAction::List { .. } => "important list",
        ImportantAction::Add { .. } => "important add",
        ImportantAction::Remove { .. } => "important remove",
        ImportantAction::Sync { .. } => "important sync",
    };
    Err(KronError::NotYetImplemented(op))
}
