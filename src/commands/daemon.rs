//! `kron daemon` — daemon process control (P2 stub).

use clap::{Args, Subcommand};

use crate::commands::Ctx;
use crate::error::{KronError, Result};

#[derive(Debug, Args)]
pub struct DaemonArgs {
    #[command(subcommand)]
    pub action: DaemonAction,
}

#[derive(Debug, Subcommand)]
pub enum DaemonAction {
    /// Start the daemon (background by default).
    Start {
        #[arg(long)]
        foreground: bool,
        #[arg(long, default_value = "info")]
        log_level: String,
    },
    /// Stop the daemon.
    Stop {
        #[arg(long, default_value_t = 10)]
        timeout: u64,
        #[arg(long)]
        force: bool,
    },
    /// Show daemon status.
    Status,
    /// Restart (stop + start).
    Restart {
        #[arg(long, default_value_t = 10)]
        timeout: u64,
    },
}

pub fn run(_ctx: Ctx, args: DaemonArgs) -> Result<()> {
    let op = match &args.action {
        DaemonAction::Start { .. } => "daemon start",
        DaemonAction::Stop { .. } => "daemon stop",
        DaemonAction::Status => "daemon status",
        DaemonAction::Restart { .. } => "daemon restart",
    };
    Err(KronError::NotYetImplemented(op))
}
