//! `kron vertex` — vertex management (P4 stub).

use clap::{Args, Subcommand};

use crate::commands::Ctx;
use crate::error::{KronError, Result};

#[derive(Debug, Args)]
pub struct VertexArgs {
    #[command(subcommand)]
    pub action: VertexAction,
}

#[derive(Debug, Subcommand)]
pub enum VertexAction {
    /// List all vertices.
    List,
    /// Show vertex details.
    Show { name: String },
    /// Create a new vertex.
    Create {
        name: String,
        #[arg(long, short = 'm')]
        message: Option<String>,
        #[arg(long)]
        branch: Option<String>,
        #[arg(long)]
        path: Option<String>,
    },
    /// Update vertex description.
    Describe {
        name: String,
        #[arg(long, short = 'm')]
        message: String,
        #[arg(long)]
        editor: bool,
    },
    /// Delete a vertex.
    Delete {
        name: String,
        #[arg(long)]
        force: bool,
    },
}

pub fn run(_ctx: Ctx, args: VertexArgs) -> Result<()> {
    let op = match &args.action {
        VertexAction::List => "vertex list",
        VertexAction::Show { .. } => "vertex show",
        VertexAction::Create { .. } => "vertex create",
        VertexAction::Describe { .. } => "vertex describe",
        VertexAction::Delete { .. } => "vertex delete",
    };
    Err(KronError::NotYetImplemented(op))
}
