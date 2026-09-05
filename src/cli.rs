//! CLI entrypoint: clap definition + dispatch.

use clap::{Parser, Subcommand};
use tracing_subscriber::EnvFilter;

use crate::commands::{config, conflict, context, daemon, important, init, list_projects, path, status, task, vertex, Ctx};
use crate::error::Result;
use crate::output::OutputMode;

/// kron — Git-native task tracker for AI-assisted development.
#[derive(Debug, Parser)]
#[command(
    name = "kron",
    version,
    about = "Git-native task tracker for AI-assisted development",
    long_about = None,
    propagate_version = true,
)]
pub struct Cli {
    /// Increase log verbosity (repeat for more: -v, -vv, -vvv).
    #[arg(short, long, action = clap::ArgAction::Count, global = true)]
    pub verbose: u8,

    /// Emit machine-readable JSON.
    #[arg(long, global = true)]
    pub json: bool,

    /// Emit machine-readable tab-separated records.
    #[arg(long, global = true)]
    pub porcelain: bool,

    #[command(subcommand)]
    pub command: Command,
}

#[derive(Debug, Subcommand)]
pub enum Command {
    /// Initialize a Kron workspace in the current directory.
    Init(init::InitArgs),
    /// Show project status.
    Status(status::StatusArgs),
    /// List all known Kron projects.
    List(list_projects::ListArgs),
    /// Print an internal Kron path (--kron-root / --important).
    Path(path::PathArgs),
    /// Read or write configuration.
    Config(config::ConfigArgs),
    /// Manage tasks.
    Task(task::TaskArgs),
    /// Manage vertices.
    Vertex(vertex::VertexArgs),
    /// Manage important files.
    Important(important::ImportantArgs),
    /// Manage sync conflicts.
    Conflict(conflict::ConflictArgs),
    /// Control the background daemon.
    Daemon(daemon::DaemonArgs),
    /// Generate AI-friendly context artifacts.
    Context(context::ContextArgs),
}

/// Parse argv and dispatch. Entry point invoked from `main()`.
pub fn run() -> Result<()> {
    let cli = Cli::parse();

    // ---- tracing init (verbose -> RUST_LOG) ----
    if cli.verbose > 0 {
        let default = match cli.verbose {
            1 => "warn",
            2 => "info",
            _ => "debug",
        };
        let _ = tracing_subscriber::fmt()
            .with_env_filter(EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| EnvFilter::new(default)))
            .with_writer(std::io::stderr)
            .try_init();
    }

    let mode = OutputMode::from_flags(cli.json, cli.porcelain);
    let verbose = cli.verbose > 0;
    let ctx = Ctx::new(mode, verbose);

    match cli.command {
        Command::Init(a)    => init::run(ctx, a)?,
        Command::Status(a)  => status::run(ctx, a)?,
        Command::List(a)    => list_projects::run(ctx, a)?,
        Command::Path(a)    => path::run(ctx, a)?,
        Command::Config(a)  => config::run(ctx, a)?,
        Command::Task(a)    => task::run(ctx, a)?,
        Command::Vertex(a)  => vertex::run(ctx, a)?,
        Command::Important(a) => important::run(ctx, a)?,
        Command::Conflict(a) => conflict::run(ctx, a)?,
        Command::Daemon(a)  => daemon::run(ctx, a)?,
        Command::Context(a) => context::run(ctx, a)?,
    }
    Ok(())
}
