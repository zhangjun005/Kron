//! `kron context` — AI-friendly context generation (P5 stub).

use clap::{Args, Subcommand};

use crate::commands::Ctx;
use crate::error::{KronError, Result};

#[derive(Debug, Args)]
pub struct ContextArgs {
    #[command(subcommand)]
    pub action: ContextAction,
}

#[derive(Debug, Subcommand)]
pub enum ContextAction {
    /// Generate (or refresh) `.kron-context/` artifacts.
    Generate {
        /// Force regeneration even if up to date.
        #[arg(long)]
        force: bool,
    },
}

pub fn run(_ctx: Ctx, _args: ContextArgs) -> Result<()> {
    Err(KronError::NotYetImplemented("context generate"))
}
