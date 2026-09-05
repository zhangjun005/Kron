//! `kron init` — initialize a Kron workspace in the current directory.

use clap::Args;
use serde::Serialize;

use crate::commands::Ctx;
use crate::core::init as core_init;
use crate::error::{KronError, Result};
use crate::model::LinkMode;

/// Arguments for `kron init`.
#[derive(Debug, Args)]
pub struct InitArgs {
    /// Re-initialize even if `kron-internal/` already exists.
    #[arg(long)]
    pub force: bool,

    /// File-link strategy for important/ files.
    #[arg(long, value_enum, default_value_t = LinkMode::Symlink)]
    pub mode: LinkMode,

    /// Skip creating a vertex folder (just register internal state).
    #[arg(long)]
    pub no_vertex: bool,

    /// Skip Git detection (allow init outside a Git repo).
    #[arg(long)]
    pub no_git: bool,
}

/// JSON / porcelain summary of init.
#[derive(Serialize)]
struct InitSummary {
    project: String,
    mode: LinkMode,
    no_vertex: bool,
    git_check_skipped: bool,
    created: Vec<String>,
    skipped: Vec<String>,
}

pub fn run(ctx: Ctx, args: InitArgs) -> Result<()> {
    let cwd = std::env::current_dir().map_err(KronError::Io)?;

    let prep = core_init::prepare(&cwd, args.no_git)?;

    if prep.kron_dir_exists && !args.force {
        return Err(KronError::AlreadyInitialized(cwd));
    }

    let outcome: core_init::InitOutcome =
        core_init::materialize(&prep, args.no_vertex, args.mode)?;

    let summary = InitSummary {
        project: outcome.project.project_path.display().to_string(),
        mode: outcome.link_mode,
        no_vertex: outcome.no_vertex,
        git_check_skipped: args.no_git,
        created: outcome.created.clone(),
        skipped: outcome.skipped.clone(),
    };

    match ctx.mode {
        crate::output::OutputMode::Json => {
            println!("{}", serde_json::to_string_pretty(&summary)?);
        }
        crate::output::OutputMode::Porcelain => {
            println!("{}\t{:?}\t{}\t{}",
                summary.project,
                summary.mode,
                summary.no_vertex,
                summary.git_check_skipped);
            for c in &summary.created {
                println!("+\t{c}");
            }
            for s in &summary.skipped {
                println!("=\t{s}");
            }
        }
        crate::output::OutputMode::Human => {
            if ctx.verbose {
                eprintln!("[debug] mode={:?} no_vertex={} no_git={} force={}",
                    args.mode, args.no_vertex, args.no_git, args.force);
            }
            for c in &outcome.created {
                println!("\u{2713} Created {c}");
            }
            for s in &outcome.skipped {
                println!("= Skipped {s}");
            }
            println!();
            println!("Project:    {}", outcome.project.name);
            println!("Data:       {}", outcome.project.kron_data_path.display());
            println!("Link mode:  {:?}", outcome.link_mode);
            println!("Git repo:   {}", if prep.is_git { "yes" } else { "no (--no-git)" });
        }
    }
    Ok(())
}
