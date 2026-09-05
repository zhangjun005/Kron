//! `kron config` — read/write configuration (P4 stub).

use clap::{Args, Subcommand};

use crate::commands::Ctx;
use crate::error::{KronError, Result};

#[derive(Debug, Args)]
pub struct ConfigArgs {
    #[command(subcommand)]
    pub action: ConfigAction,
}

#[derive(Debug, Subcommand)]
pub enum ConfigAction {
    /// Get a single config key.
    Get { key: String },
    /// Set a config key.
    Set { key: String, value: String },
    /// List all config keys.
    List,
}

pub fn run(_ctx: Ctx, args: ConfigArgs) -> Result<()> {
    let op = match args.action {
        ConfigAction::Get { .. } => "config get",
        ConfigAction::Set { .. } => "config set",
        ConfigAction::List => "config list",
    };
    Err(KronError::NotYetImplemented(op))
}
