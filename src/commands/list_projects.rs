//! `kron list` — list all known Kron projects.

use clap::Args;
use serde::Serialize;

use crate::commands::Ctx;
use crate::error::Result;

#[derive(Debug, Args)]
pub struct ListArgs {}

#[derive(Serialize)]
struct ProjectEntry {
    name: String,
    path: String,
    initialized: bool,
}

pub fn run(ctx: Ctx, _args: ListArgs) -> Result<()> {
    let entries: Vec<ProjectEntry> = vec![]; // stub: registry lookup not wired yet

    match ctx.mode {
        crate::output::OutputMode::Json => {
            println!("{}", serde_json::to_string_pretty(&entries)?);
        }
        crate::output::OutputMode::Porcelain => {
            for e in &entries {
                println!("{}\t{}\t{}", e.name, e.path, e.initialized);
            }
            if entries.is_empty() {
                println!("# (no projects registered yet)");
            }
        }
        crate::output::OutputMode::Human => {
            if entries.is_empty() {
                println!("(no projects registered yet)");
            } else {
                println!("{:<20}  {:<50}  INIT", "NAME", "PATH");
                println!("{}", "-".repeat(78));
                for e in &entries {
                    println!("{:<20}  {:<50}  {}", e.name, e.path, e.initialized);
                }
            }
        }
    }
    Ok(())
}
