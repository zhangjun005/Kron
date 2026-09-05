//! `kron conflict` — conflict management (P2 stub).

use clap::{Args, Subcommand};

use crate::commands::Ctx;
use crate::error::{KronError, Result};

#[derive(Debug, Args)]
pub struct ConflictArgs {
    #[command(subcommand)]
    pub action: ConflictAction,
}

#[derive(Debug, Subcommand)]
pub enum ConflictAction {
    /// List conflicts.
    List {
        #[arg(long, default_value = "pending")]
        status: String,
        #[arg(long)]
        since: Option<String>,
    },
    /// Show conflict details.
    Show {
        id: String,
        #[arg(long)]
        diff_only: bool,
    },
    /// Resolve a conflict.
    Resolve {
        id: String,
        #[arg(long, value_enum)]
        r#use: ResolveStrategy,
        #[arg(long)]
        reason: Option<String>,
    },
    /// Mark a conflict as ignorable.
    Ignore {
        id: String,
        #[arg(long)]
        reason: Option<String>,
    },
}

#[derive(Debug, Clone, Copy, clap::ValueEnum)]
pub enum ResolveStrategy {
    Project,
    Internal,
    Both,
    Prompt,
}

pub fn run(_ctx: Ctx, args: ConflictArgs) -> Result<()> {
    let op = match &args.action {
        ConflictAction::List { .. } => "conflict list",
        ConflictAction::Show { .. } => "conflict show",
        ConflictAction::Resolve { .. } => "conflict resolve",
        ConflictAction::Ignore { .. } => "conflict ignore",
    };
    Err(KronError::NotYetImplemented(op))
}
