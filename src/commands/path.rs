//! `kron path` — output internal Kron paths (AI-friendly path discovery).
//!
//! Prints exactly one path per invocation, nothing else. Suitable for
//! `$(kron path --kron-root)`-style shell composition.

use clap::Args;

use crate::commands::Ctx;
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

    let cwd = std::env::current_dir().map_err(KronError::Io)?;

    // Conventional locations per dev-docs/design/00-总览与架构.md § 5
    // and 04b-CLI设计.md § 3.1.
    // Real resolution (does the project exist?) lands with Phase 1 milestone.
    let path = if args.kron_root {
        cwd.join("kron-internal")
    } else {
        cwd.join("KRON").join("important")
    };

    if ctx.mode == crate::output::OutputMode::Human {
        // Machine-friendly: print the path alone, no decoration.
    }
    println!("{}", path.display());
    Ok(())
}
