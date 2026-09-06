//! `kron conflict` — conflict management (P2 implementation).

use clap::{Args, Subcommand};
use serde::Serialize;

use crate::commands::Ctx;
use crate::core::sync::conflict as core_conflict;
use crate::error::{KronError, Result};
use crate::model::{ConflictRecord, ConflictResolution, ConflictStatus};

#[derive(Debug, Args)]
pub struct ConflictArgs {
    #[command(subcommand)]
    pub action: ConflictAction,
}

#[derive(Debug, Subcommand)]
pub enum ConflictAction {
    /// List conflicts.
    List {
        #[arg(long, default_value = "pending")]
        status: String,
        #[arg(long)]
        since: Option<String>,
    },
    /// Show conflict details.
    Show {
        id: String,
        #[arg(long)]
        diff_only: bool,
    },
    /// Resolve a conflict.
    Resolve {
        id: String,
        #[arg(long, value_enum)]
        r#use: ResolveStrategy,
        #[arg(long)]
        reason: Option<String>,
    },
    /// Mark a conflict as ignorable.
    Ignore {
        id: String,
        #[arg(long)]
        reason: Option<String>,
    },
}

#[derive(Debug, Clone, Copy, clap::ValueEnum)]
pub enum ResolveStrategy {
    Project,
    Internal,
}

impl From<ResolveStrategy> for ConflictResolution {
    fn from(s: ResolveStrategy) -> Self {
        match s {
            ResolveStrategy::Project => ConflictResolution::UseProject,
            ResolveStrategy::Internal => ConflictResolution::UseInternal,
        }
    }
}

#[derive(Serialize)]
struct ConflictListRow {
    id: String,
    file: String,
    status: String,
    detected_at: String,
    project_hash: String,
    internal_hash: String,
}

#[derive(Serialize)]
struct ConflictShow {
    id: String,
    file: String,
    status: String,
    detected_at: String,
    project_version: VersionInfo,
    internal_version: VersionInfo,
    diff: DiffInfo,
    backup_path: String,
    available_resolutions: Vec<String>,
}

#[derive(Serialize)]
struct VersionInfo {
    hash: String,
    size: u64,
    preview: String,
}

#[derive(Serialize)]
struct DiffInfo {
    unified_diff: String,
}

#[derive(Serialize)]
struct ConflictResolveSummary {
    id: String,
    file: String,
    resolution: String,
    sync_state_after: String,
    backup_retained_at: String,
}

fn require_project() -> Result<std::path::PathBuf> {
    let cwd = std::env::current_dir()?;
    if !cwd.join("kron-internal").join("config.json").exists() {
        return Err(KronError::NotAProject(cwd));
    }
    Ok(cwd)
}

pub fn run(ctx: Ctx, args: ConflictArgs) -> Result<()> {
    match args.action {
        ConflictAction::List { status, since: _ } => list_cmd(ctx, &status),
        ConflictAction::Show { id, diff_only } => show_cmd(ctx, &id, diff_only),
        ConflictAction::Resolve { id, r#use, reason: _ } => {
            resolve_cmd(ctx, &id, r#use.into())
        }
        ConflictAction::Ignore { id, reason: _ } => ignore_cmd(ctx, &id),
    }
}

fn list_cmd(ctx: Ctx, status: &str) -> Result<()> {
    let project = require_project()?;
    let records = core_conflict::list_by_status(&project, status)?;

    match ctx.mode {
        crate::output::OutputMode::Json => {
            let rows: Vec<ConflictListRow> = records
                .iter()
                .map(|r| ConflictListRow {
                    id: r.id.clone(),
                    file: r.relative_path.clone(),
                    status: r.status.to_string(),
                    detected_at: r.detected_at.to_rfc3339(),
                    project_hash: r.project_hash.clone(),
                    internal_hash: r.internal_hash.clone(),
                })
                .collect();
            println!("{}", serde_json::to_string_pretty(&serde_json::json!({
                "conflicts": rows,
                "total": rows.len(),
                "filter": status,
            }))?);
        }
        crate::output::OutputMode::Porcelain => {
            for r in &records {
                println!(
                    "{}\t{}\t{}\t{}\t{}\t{}",
                    r.id,
                    r.relative_path,
                    r.status,
                    r.detected_at.to_rfc3339(),
                    r.project_hash,
                    r.internal_hash,
                );
            }
            if records.is_empty() {
                println!("# (no {status} conflicts)");
            }
        }
        crate::output::OutputMode::Human => {
            if records.is_empty() {
                println!("(no {status} conflicts)");
                return Ok(());
            }
            println!("{:<32}  {:<30}  {:<10}  {}", "ID", "FILE", "STATUS", "DETECTED");
            println!("{}", "-".repeat(96));
            for r in &records {
                println!(
                    "{:<32}  {:<30}  {:<10}  {}",
                    r.id,
                    truncate(&r.relative_path, 30),
                    r.status,
                    r.detected_at.to_rfc3339(),
                );
            }
        }
    }
    Ok(())
}

fn show_cmd(ctx: Ctx, id: &str, diff_only: bool) -> Result<()> {
    let project = require_project()?;
    let rec = core_conflict::load_record(&project, id)?
        .ok_or_else(|| KronError::NotFound(project.join("kron-internal").join("conflicts").join(id)))?;

    let proj_bytes = std::fs::read(&rec.project_backup)?;
    let int_bytes = std::fs::read(&rec.internal_backup)?;

    let unified_diff = build_unified_diff(
        &rec.relative_path,
        &proj_bytes,
        &rec.relative_path,
        &int_bytes,
    );

    let backup_path = rec
        .project_backup
        .parent()
        .map(|p| p.display().to_string())
        .unwrap_or_default();

    if diff_only {
        if ctx.mode != crate::output::OutputMode::Human {
            // Even for diff_only we emit JSON if requested, so AI agents
            // can pipe it without parsing human text.
            println!("{}", serde_json::to_string_pretty(&serde_json::json!({
                "id": rec.id,
                "file": rec.relative_path,
                "diff": unified_diff,
            }))?);
        } else {
            println!("{unified_diff}");
        }
        return Ok(());
    }

    let proj_preview = String::from_utf8_lossy(&proj_bytes).to_string();
    let int_preview = String::from_utf8_lossy(&int_bytes).to_string();

    let show = ConflictShow {
        id: rec.id.clone(),
        file: rec.relative_path.clone(),
        status: rec.status.to_string(),
        detected_at: rec.detected_at.to_rfc3339(),
        project_version: VersionInfo {
            hash: rec.project_hash.clone(),
            size: proj_bytes.len() as u64,
            preview: truncate(&proj_preview, 4000),
        },
        internal_version: VersionInfo {
            hash: rec.internal_hash.clone(),
            size: int_bytes.len() as u64,
            preview: truncate(&int_preview, 4000),
        },
        diff: DiffInfo { unified_diff },
        backup_path: backup_path.clone(),
        available_resolutions: vec![
            "use_project".into(),
            "use_internal".into(),
            "ignore".into(),
        ],
    };

    match ctx.mode {
        crate::output::OutputMode::Json => {
            println!("{}", serde_json::to_string_pretty(&show)?);
        }
        crate::output::OutputMode::Porcelain => {
            println!(
                "{}\t{}\t{}\tproject={}B\tinternal={}B\tbackup={}",
                show.id, show.file, show.status,
                show.project_version.size, show.internal_version.size, backup_path
            );
        }
        crate::output::OutputMode::Human => {
            println!("Conflict ID:    {}", show.id);
            println!("File:           {}", show.file);
            println!("Detected:       {}", show.detected_at);
            println!("Status:         {}", show.status);
            println!("Backup at:      {}", show.backup_path);
            println!();
            println!("Project version ({} bytes, MD5 {}):", show.project_version.size, show.project_version.hash);
            println!("{}", "-".repeat(72));
            println!("{}", truncate(&show.project_version.preview, 2000));
            println!();
            println!("Internal version ({} bytes, MD5 {}):", show.internal_version.size, show.internal_version.hash);
            println!("{}", "-".repeat(72));
            println!("{}", truncate(&show.internal_version.preview, 2000));
            println!();
            println!("Unified diff:");
            println!("{}", "-".repeat(72));
            println!("{}", show.diff.unified_diff);
            println!();
            println!("Resolutions: use --use project|internal | ignore");
        }
    }
    Ok(())
}

fn resolve_cmd(ctx: Ctx, id: &str, decision: ConflictResolution) -> Result<()> {
    let project = require_project()?;
    let updated = core_conflict::resolve(&project, id, decision)?;

    let summary = ConflictResolveSummary {
        id: updated.id.clone(),
        file: updated.relative_path.clone(),
        resolution: updated
            .resolution
            .map(|r| r.to_string())
            .unwrap_or_default(),
        sync_state_after: if updated.status == ConflictStatus::Ignored {
            "conflict".to_string()
        } else {
            "synced".to_string()
        },
        backup_retained_at: updated
            .project_backup
            .parent()
            .map(|p| p.display().to_string())
            .unwrap_or_default(),
    };

    match ctx.mode {
        crate::output::OutputMode::Json => {
            println!("{}", serde_json::to_string_pretty(&summary)?);
        }
        crate::output::OutputMode::Porcelain => {
            println!(
                "{}\t{}\t{}\t{}\t{}",
                summary.id,
                summary.file,
                summary.resolution,
                summary.sync_state_after,
                summary.backup_retained_at
            );
        }
        crate::output::OutputMode::Human => {
            println!(
                "\u{2713} Resolved: {} now matches ({})",
                summary.file, summary.resolution
            );
            println!("  Backup retained at: {}", summary.backup_retained_at);
        }
    }
    Ok(())
}

fn ignore_cmd(ctx: Ctx, id: &str) -> Result<()> {
    let project = require_project()?;
    let updated = core_conflict::resolve(&project, id, ConflictResolution::Ignore)?;

    let summary = ConflictResolveSummary {
        id: updated.id.clone(),
        file: updated.relative_path.clone(),
        resolution: "ignore".into(),
        sync_state_after: "conflict".into(),
        backup_retained_at: updated
            .project_backup
            .parent()
            .map(|p| p.display().to_string())
            .unwrap_or_default(),
    };

    match ctx.mode {
        crate::output::OutputMode::Json => {
            println!("{}", serde_json::to_string_pretty(&summary)?);
        }
        crate::output::OutputMode::Porcelain => {
            println!(
                "{}\t{}\tignored\tconflict\t{}",
                summary.id, summary.file, summary.backup_retained_at
            );
        }
        crate::output::OutputMode::Human => {
            println!("\u{26a0} Conflict {} marked as Ignored.", summary.id);
            println!("  File remains in conflict state until you resolve it.");
        }
    }
    Ok(())
}

fn truncate(s: &str, max: usize) -> String {
    if s.chars().count() <= max {
        s.to_string()
    } else {
        let head: String = s.chars().take(max).collect();
        format!("{head}\n... ({} more chars)", s.chars().count() - max)
    }
}

/// Build a tiny unified-diff-style string between two byte buffers.
///
/// We do not pull in the `similar` crate to keep the binary lean; for
/// small text files a line-by-line comparison is more than good enough.
/// Binary files (non-UTF8) get a placeholder diff.
fn build_unified_diff(
    proj_name: &str,
    proj_bytes: &[u8],
    int_name: &str,
    int_bytes: &[u8],
) -> String {
    let proj_text = match std::str::from_utf8(proj_bytes) {
        Ok(s) => s,
        Err(_) => return format!(
            "Binary file diff not supported.\n  project: {} bytes\n  internal: {} bytes\n",
            proj_bytes.len(),
            int_bytes.len()
        ),
    };
    let int_text = match std::str::from_utf8(int_bytes) {
        Ok(s) => s,
        Err(_) => return format!(
            "Binary file diff not supported.\n  project: {} bytes\n  internal: {} bytes\n",
            proj_bytes.len(),
            int_bytes.len()
        ),
    };
    let proj_lines: Vec<&str> = proj_text.split_inclusive('\n').collect();
    let int_lines: Vec<&str> = int_text.split_inclusive('\n').collect();
    let mut out = String::new();
    out.push_str(&format!("--- a/{proj_name}\n"));
    out.push_str(&format!("+++ b/{int_name}\n"));
    // For each differing line, emit - and + (very simple LCS-free).
    let max = proj_lines.len().max(int_lines.len());
    for i in 0..max {
        let p = proj_lines.get(i).copied();
        let q = int_lines.get(i).copied();
        match (p, q) {
            (Some(a), Some(b)) if a == b => {
                out.push_str(&format!(" {a}"));
            }
            (Some(a), Some(b)) => {
                out.push_str(&format!("-{a}"));
                out.push_str(&format!("+{b}"));
            }
            (Some(a), None) => out.push_str(&format!("-{a}")),
            (None, Some(b)) => out.push_str(&format!("+{b}")),
            (None, None) => {}
        }
    }
    out
}

// ---- helpers used by the resolver summary ----

#[allow(dead_code)]
fn fmt_record(r: &ConflictRecord) -> String {
    format!(
        "{} {} {} {}",
        r.id,
        r.relative_path,
        r.status,
        r.detected_at.to_rfc3339()
    )
}
