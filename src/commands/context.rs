//! `kron context` — AI-friendly context generation (P5).
//!
//! Manages `KRON/.kron-context/` — fact-only structured snapshots for AI tools.
//! All generation is delegated to `core::context::Generator`.
//!
//! **Design anchors** (04b § 3.7 + Q26/Q28):
//! - `.kron-context/` lives under `KRON/`, not project root
//! - v1 generates exactly 4 files: git/recent-commits, git/branch-summary,
//!   code/structure, README
//! - Kron generates **facts only** — no LLM, no API key, no semantic understanding
//! - Daemon is the primary trigger; CLI is the manual / debug entry point

use std::borrow::Cow;
use std::path::PathBuf;

use clap::{Parser, Subcommand};

use crate::commands::Ctx;
use crate::core::context::{DocType, Generator};
use crate::error::{KronError, Result};

// ---------------------------------------------------------------------------
// CLI types
// ---------------------------------------------------------------------------

/// `kron context` — manage AI-friendly context snapshots.
///
/// Default (no flags): incremental generate — only regenerates stale files.
/// Use `--regenerate` for a full rebuild.
#[derive(Debug, Parser)]
pub struct ContextArgs {
    /// Full rebuild: delete and regenerate all context files.
    #[arg(long, short = 'r')]
    pub regenerate: bool,

    /// List all context files with their generation timestamp and stale status.
    #[arg(long, short = 'l')]
    pub list: bool,

    /// Emit machine-readable JSON (for `--list`).
    #[arg(long)]
    pub json: bool,

    /// Delete all context files (they will be rebuilt on next generate).
    #[arg(long, short = 'c')]
    pub clean: bool,

    /// Skip the confirmation prompt for `--clean`.
    #[arg(long)]
    pub force: bool,

    /// Only regenerate files matching this glob pattern (e.g. `git/*`).
    #[arg(long)]
    pub only: Option<String>,

    /// Show a specific context file's content.
    ///
    /// Takes a relative path under `KRON/.kron-context/` (e.g. `git/recent-commits.md`).
    #[command(subcommand)]
    pub action: Option<ContextAction>,
}

#[derive(Debug, Subcommand)]
pub enum ContextAction {
    /// Show a specific context file's content.
    Show {
        /// Relative path under `KRON/.kron-context/` (e.g. `git/recent-commits.md`).
        path: String,

        /// Disable the pager (force stdout output).
        #[arg(long)]
        no_pager: bool,
    },
}

// ---------------------------------------------------------------------------
// Entry point
// ---------------------------------------------------------------------------

pub fn run(ctx: Ctx, args: ContextArgs) -> Result<()> {
    let project_root = crate::commands::require_project_root(&ctx)?;
    let kron_root = project_root.join("KRON");

    // Ensure KRON/ exists so we can write .kron-context/
    if !kron_root.is_dir() {
        std::fs::create_dir_all(&kron_root).map_err(KronError::Io)?;
    }

    // Priority: show > list > clean > regenerate > generate
    if let Some(ContextAction::Show { path, no_pager: _ }) = args.action {
        return run_show(ctx, &project_root, &kron_root, &path);
    }

    if args.list {
        return run_list(ctx, &project_root, &kron_root, args.json);
    }

    if args.clean {
        return run_clean(ctx, &project_root, &kron_root, args.force);
    }

    if args.regenerate {
        return run_regenerate(ctx, &project_root, &kron_root, args.only);
    }

    // Default: incremental generate
    run_generate(ctx, &project_root, &kron_root, false, args.only)
}

// ---------------------------------------------------------------------------
// Subcommand handlers
// ---------------------------------------------------------------------------

/// Incremental generate (default when no flags).
fn run_generate(
    ctx: Ctx,
    project_root: &PathBuf,
    kron_root: &PathBuf,
    force: bool,
    only_pattern: Option<String>,
) -> Result<()> {
    let mut g = Generator::new(project_root.clone(), kron_root.clone())?;
    let docs = g.list_docs();
    let ctx_dir = g.context_dir();

    // Determine which docs to regenerate
    let to_regen: Vec<DocType> = if let Some(ref pat) = only_pattern {
        DocType::all()
            .iter()
            .filter(|dt| glob_match(pat, dt.path()))
            .cloned()
            .collect()
    } else {
        DocType::all().to_vec()
    };

    let count = docs.len();
    let mut regenerated = Vec::new();

    for dt in to_regen {
        let doc_meta = docs.iter().find(|d| d.doc_type == dt);
        let is_stale = doc_meta.map(|d| d.is_stale()).unwrap_or(true);
        let file_exists = ctx_dir.join(dt.path()).exists();

        if !force && !is_stale && file_exists {
            continue; // up to date
        }

        g.generate_one(dt)?;
        regenerated.push(dt.path().to_string());
    }

    let msg: Cow<str> = if regenerated.is_empty() {
        Cow::Borrowed("all files up to date")
    } else {
        Cow::Owned(format!("regenerated {} file(s)", regenerated.len()))
    };
    ctx.json(&serde_json::json!({
        "context_dir": ctx_dir.to_string_lossy(),
        "total": count,
        "regenerated": regenerated,
        "message": msg.as_ref(),
    }))?;

    if regenerated.is_empty() {
        ctx.human("all context files up to date");
    } else {
        ctx.human(&format!("regenerated {} file(s):", regenerated.len()));
        for p in &regenerated {
            ctx.human(&format!("  {}", p));
        }
    }

    Ok(())
}

/// Full rebuild.
fn run_regenerate(
    ctx: Ctx,
    project_root: &PathBuf,
    kron_root: &PathBuf,
    only_pattern: Option<String>,
) -> Result<()> {
    let mut g = Generator::new(project_root.clone(), kron_root.clone())?;

    if let Some(ref pat) = only_pattern {
        // Selective full rebuild
        let filtered: Vec<DocType> = DocType::all()
            .iter()
            .filter(|dt| glob_match(pat, dt.path()))
            .cloned()
            .collect();
        let count = filtered.len();
        let mut regenerated = Vec::new();
        for dt in filtered {
            g.generate_one(dt)?;
            regenerated.push(dt.path().to_string());
        }
        ctx.json(&serde_json::json!({
            "regenerated": regenerated,
            "total": count,
        }))?;
        ctx.human(&format!("regenerated {} file(s)", count));
        return Ok(());
    }

    // Full rebuild of all docs
    let results = g.generate_all()?;
    let count = results.len();

    ctx.json(&serde_json::json!({
        "context_dir": g.context_dir().to_string_lossy(),
        "total": count,
        "files": results.iter().map(|r| r.path.clone()).collect::<Vec<_>>(),
    }))?;
    ctx.human(&format!("regenerated {} file(s) in {}", count, g.context_dir().display()));

    Ok(())
}

/// List all context files.
fn run_list(
    ctx: Ctx,
    project_root: &PathBuf,
    kron_root: &PathBuf,
    json_flag: bool,
) -> Result<()> {
    let g = Generator::new(project_root.clone(), kron_root.clone())?;
    let docs = g.list_docs();
    let ctx_dir = g.context_dir();

    if json_flag || ctx.mode == crate::output::OutputMode::Json {
        #[derive(serde::Serialize)]
        struct ListItem {
            path: String,
            generated_at: String,
            stale: bool,
            stale_reason: Option<String>,
        }
        let items: Vec<ListItem> = docs
            .iter()
            .map(|d| ListItem {
                path: d.path.clone(),
                generated_at: d.generated_at.to_rfc3339(),
                stale: d.is_stale(),
                stale_reason: d.stale_reason.clone(),
            })
            .collect();
        ctx.json(&serde_json::json!({
            "context_dir": ctx_dir.to_string_lossy(),
            "files": items,
            "total": items.len(),
        }))?;
        return Ok(());
    }

    // Human / porcelain
    println!("{} — context files", ctx_dir.display());
    println!("{:<45} {:>25} {}", "path", "generated_at", "status");
    println!("{}", "-".repeat(90));

    for doc in &docs {
        let status = if doc.is_stale() {
            format!("STALE — {}", doc.stale_reason.as_deref().unwrap_or("?"))
        } else {
            "fresh".to_string()
        };
        let ts = doc.generated_at.format("%Y-%m-%d %H:%M:%S UTC").to_string();
        ctx.porcelain(format!("{}\t{}\t{}", doc.path, ts, status));
        ctx.human(format!("  {:<42} {:>25} {}", doc.path, ts, status));
    }

    Ok(())
}

/// Show a specific file.
fn run_show(
    ctx: Ctx,
    project_root: &PathBuf,
    kron_root: &PathBuf,
    rel_path: &str,
) -> Result<()> {
    let g = Generator::new(project_root.clone(), kron_root.clone())?;
    // Normalize path separators for Windows
    let rel_path = rel_path.replace('\\', "/");
    let content = g.show(&rel_path)?;

    if ctx.mode == crate::output::OutputMode::Json {
        ctx.json(&serde_json::json!({
            "path": rel_path,
            "content": content,
        }))?;
    } else {
        println!("{content}");
    }

    Ok(())
}

/// Clean all context files.
fn run_clean(
    ctx: Ctx,
    project_root: &PathBuf,
    kron_root: &PathBuf,
    force: bool,
) -> Result<()> {
    let g = Generator::new(project_root.clone(), kron_root.clone())?;
    let ctx_dir = g.context_dir();

    if !ctx_dir.is_dir() {
        ctx.info("nothing to clean — .kron-context/ does not exist");
        return Ok(());
    }

    let count = walk_dir_count(&ctx_dir).unwrap_or(0);

    if !force {
        ctx.human(&format!("this will delete {} file(s) in {}", count, ctx_dir.display()));
        ctx.warning("run `kron context --clean --force` to confirm");
        return Ok(());
    }

    g.clean()?;
    ctx.success(&format!("cleaned {} file(s) from {}", count, ctx_dir.display()));

    Ok(())
}

// ---------------------------------------------------------------------------
// Utilities
// ---------------------------------------------------------------------------

/// Simple glob matching: `*` matches anything up to `/` or end.
fn glob_match(pattern: &str, path: &str) -> bool {
    if pattern == "*" {
        return true;
    }
    for part in pattern.split('/') {
        if part.is_empty() || part == "*" {
            continue;
        }
        if !path.contains(part) {
            return false;
        }
    }
    true
}

/// Count files in a directory recursively.
fn walk_dir_count(dir: &PathBuf) -> std::io::Result<usize> {
    let mut count = 0;
    for entry in std::fs::read_dir(dir)? {
        let entry = entry?;
        if entry.file_type()?.is_dir() {
            count += walk_dir_count(&entry.path())?;
        } else {
            count += 1;
        }
    }
    Ok(count)
}
