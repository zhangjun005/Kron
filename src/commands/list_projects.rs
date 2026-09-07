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

    ctx.json(&entries)?;
    for e in &entries {
        ctx.porcelain(format!("{}\t{}\t{}", e.name, e.path, e.initialized));
    }
    if entries.is_empty() {
        ctx.porcelain("# (no projects registered yet)");
    }
    if entries.is_empty() {
        ctx.human("(no projects registered yet)");
    } else {
        ctx.human(format!("{:<20}  {:<50}  INIT", "NAME", "PATH"));
        ctx.human("-".repeat(78));
        for e in &entries {
            ctx.human(format!("{:<20}  {:<50}  {}", e.name, e.path, e.initialized));
        }
    }
    Ok(())
}
