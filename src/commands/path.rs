//! `kron path` — output internal Kron paths (AI-friendly path discovery).
//!
//! Prints exactly one path per invocation, nothing else. Suitable for
//! `$(kron path --kron-root)`-style shell composition.

use clap::Args;

use crate::commands::Ctx;
use crate::core::project;
use crate::error::{KronError, Result};

#[derive(Debug, Args)]
pub struct PathArgs {
    /// Print the kron-internal root path.
    #[arg(long)]
    pub kron_root: bool,

    /// Print the important/ directory path.
    #[arg(long)]
    pub important: bool,
}

pub fn run(ctx: Ctx, args: PathArgs) -> Result<()> {
    // Exactly one selector required.
    let selector_count = [args.kron_root, args.important].iter().filter(|x| **x).count();
    if selector_count != 1 {
        return Err(KronError::Cli(
            "exactly one of --kron-root / --important must be set".into(),
        ));
    }

    // Resolve project root via ancestor search (Git-style). If no project is
    // found we still return a conventional path so the output is predictable.
    let root = project::find_project_root(&std::env::current_dir().map_err(KronError::Io)?)
        .unwrap_or_else(|| std::env::current_dir().map_err(KronError::Io).unwrap());

    let path = if args.kron_root {
        root.join("kron-internal")
    } else {
        root.join("KRON").join("important")
    };

    if ctx.mode == crate::output::OutputMode::Human {
        // Machine-friendly: print the path alone, no decoration.
    }
    println!("{}", path.display());
    Ok(())
}
