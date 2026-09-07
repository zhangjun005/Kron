//! `kron important` — important-file management (P4 implementation).
//!
//! `important` files live in two places:
//! - Project side: any user-chosen path (typically under `KRON/`).
//! - Internal side: `kron-internal/important/files/<rel_path>`.
//!
//! The internal side is the authoritative copy; the project-side copy
//! is the human/AI-visible one. `kron important add` registers a file
//! and performs the initial mirror; `remove` unregisters; `sync` is
//! a one-shot scan that updates the index state for every entry.

use clap::{Args, Subcommand};
use serde::Serialize;
use std::path::{Path, PathBuf};

use crate::commands::Ctx;
use crate::core::sync::sync_index::{self, ImportantEntry, ImportantIndex};
use crate::error::{KronError, Result};
use crate::model::SyncState;

#[derive(Debug, Args)]
pub struct ImportantArgs {
    #[command(subcommand)]
    pub action: ImportantAction,
}

#[derive(Debug, Subcommand)]
pub enum ImportantAction {
    /// List important files.
    List {
        #[arg(long)]
        tag: Vec<String>,
    },
    /// Register a file as important and mirror it into kron-internal.
    Add {
        /// Path relative to project root.
        path: String,
        #[arg(long)]
        copy: bool,
        #[arg(long)]
        symlink: bool,
        #[arg(long)]
        tag: Vec<String>,
        #[arg(long, short = 'm')]
        message: Option<String>,
    },
    /// Unregister an important file.
    ///
    /// Anchor #8 / Q3: the **default** behaviour is now to delete the
    /// project-side source file (matches user intuition — `remove`
    /// means *remove*). Pass `--keep-source` to opt out of the source
    /// deletion and keep the project-side copy intact (only the
    /// kron-internal mirror is removed).
    Remove {
        /// Path relative to project root.
        path: String,
        #[arg(long)]
        force: bool,
        /// Keep the project-side source file (only remove the
        /// kron-internal mirror). The default is to also remove the
        /// source file.
        #[arg(long)]
        keep_source: bool,
    },
    /// Show details of one important file.
    Show {
        path: String,
    },
    /// Force a one-shot scan to refresh sync_state for every entry.
    Sync {
        #[arg(long)]
        dry_run: bool,
    },
}

#[derive(Serialize)]
struct ImportantRow {
    path: String,
    sync_state: String,
    project_exists: bool,
    internal_exists: bool,
    internal_size: u64,
    project_size: Option<u64>,
    added_at: String,
    updated_at: String,
    conflict_id: Option<String>,
}



/// Public test-only accessor for `normalize_rel`.
pub fn normalize_rel_for_test(path: &str) -> Result<String> {
    normalize_rel(path)
}

fn normalize_rel(path: &str) -> Result<String> {
    let trimmed = path.trim();
    if trimmed.is_empty() {
        return Err(KronError::Cli("path must not be empty".into()));
    }
    // Strip leading ./ and normalize path separators.
    let trimmed = trimmed
        .strip_prefix("./")
        .or_else(|| trimmed.strip_prefix(".\\"))
        .unwrap_or(trimmed);
    let p = Path::new(trimmed);
    if p.is_absolute() {
        return Err(KronError::Cli(
            "path must be relative to project root (no leading '/')".into(),
        ));
    }
    if p.components().any(|c| matches!(c, std::path::Component::ParentDir)) {
        return Err(KronError::Cli("path may not contain '..'".into()));
    }
    Ok(trimmed.replace('\\', "/"))
}

fn file_size(p: &Path) -> Option<u64> {
    std::fs::metadata(p).ok().map(|m| m.len())
}

fn row_for(root: &Path, rel: &str, entry: &ImportantEntry) -> ImportantRow {
    let proj = sync_index::project_path_for(root, rel);
    let internal = sync_index::internal_path_for(root, rel);
    let project_exists = proj.exists();
    let internal_exists = internal.exists();

    let state_str = entry.sync_state.to_string();
    let conflict_id = if entry.sync_state == SyncState::Conflict {
        // Look up an associated conflict id (best-effort).
        crate::core::sync::conflict::list_all(root)
            .ok()
            .and_then(|all| {
                all.into_iter()
                    .find(|r| r.relative_path == rel
                        && matches!(
                            r.status,
                            crate::model::ConflictStatus::Pending
                        ))
                    .map(|r| r.id)
            })
    } else {
        None
    };

    ImportantRow {
        path: rel.to_string(),
        sync_state: state_str,
        project_exists,
        internal_exists,
        internal_size: file_size(&internal).unwrap_or(0),
        project_size: if project_exists { file_size(&proj) } else { None },
        added_at: entry.added_at.to_rfc3339(),
        updated_at: entry.updated_at.to_rfc3339(),
        conflict_id,
    }
}

pub fn run(ctx: Ctx, args: ImportantArgs) -> Result<()> {
    match args.action {
        ImportantAction::List { tag: _ } => list_cmd(ctx),
        ImportantAction::Add { path, copy, symlink, tag: _, message: _ } => {
            add_cmd(ctx, &path, copy, symlink)
        }
        // Anchor #8 / Q3: see the field doc on `Remove.keep_source`.
        ImportantAction::Remove { path, force, keep_source } => {
            remove_cmd(ctx, &path, force, keep_source)
        }
        ImportantAction::Show { path } => show_cmd(ctx, &path),
        ImportantAction::Sync { dry_run } => sync_cmd(ctx, dry_run),
    }
}

fn list_cmd(ctx: Ctx) -> Result<()> {
    let root = crate::commands::require_project_root(&ctx)?;
    let idx = ImportantIndex::load(&root)?;
    let mut entries: Vec<(&String, &ImportantEntry)> = idx.files.iter().collect();
    entries.sort_by(|a, b| a.0.cmp(b.0));

    let rows: Vec<ImportantRow> = entries
        .iter()
        .map(|(rel, e)| row_for(&root, rel, e))
        .collect();
    ctx.json(&serde_json::json!({
        "files": rows,
        "total": rows.len(),
    }))?;
    for (rel, e) in &entries {
        ctx.porcelain(format!("{}\t{}\t{}", rel, e.sync_state, e.updated_at.to_rfc3339()));
    }
    if entries.is_empty() {
        ctx.porcelain("# (no important files — try `kron important add <path>`)");
    }
    if entries.is_empty() {
        ctx.human("(no important files — try `kron important add <path>`)");
        return Ok(());
    }
    ctx.human(format!("{:<40}  {:<14}  {:<8}  {}", "PATH", "STATE", "SIZE", "UPDATED"));
    ctx.human("-".repeat(86));
    for (rel, e) in &entries {
        let internal = sync_index::internal_path_for(&root, rel);
        let sz = file_size(&internal).unwrap_or(0);
        ctx.human(format!(
            "{:<40}  {:<14}  {:<8}  {}",
            rel,
            e.sync_state.to_string(),
            sz,
            e.updated_at.to_rfc3339(),
        ));
    }
    Ok(())
}

fn add_cmd(ctx: Ctx, path: &str, copy: bool, symlink: bool) -> Result<()> {
    let _ = (copy, symlink); // both modes are implemented as plain copy in v1
    let root = crate::commands::require_project_root(&ctx)?;
    let rel = normalize_rel(path)?;
    let proj = sync_index::project_path_for(&root, &rel);
    if !proj.exists() {
        return Err(KronError::NotFound(proj));
    }

    // Ensure the kron-internal important/files directory exists.
    let internal = sync_index::internal_path_for(&root, &rel);
    if let Some(parent) = internal.parent() {
        std::fs::create_dir_all(parent)?;
    }
    let data = std::fs::read(&proj)?;
    std::fs::write(&internal, &data)?;

    let mut idx = ImportantIndex::load(&root)?;
    idx.update_state(&rel, SyncState::Synced);
    // Update internal_hash to the freshly-mirrored bytes.
    if let Some(entry) = idx.files.get_mut(&rel) {
        entry.internal_hash = sync_index::md5_hex(&data);
        entry.updated_at = chrono::Utc::now();
    } else {
        // First-time registration: insert directly.
        let now = chrono::Utc::now();
        idx.files.insert(
            rel.clone(),
            ImportantEntry {
                path: rel.clone(),
                sync_state: SyncState::Synced,
                internal_hash: sync_index::md5_hex(&data),
                added_at: now,
                updated_at: now,
            },
        );
    }
    idx.save(&root)?;

    ctx.json(&serde_json::json!({
        "added": rel,
        "size": data.len(),
        "hash": sync_index::md5_hex(&data),
        "sync_state": "synced",
    }))?;
    ctx.porcelain(format!("{}\tsynced\t{}", rel, data.len()));
    ctx.human(format!("\u{2713} Registered '{}' as important ({} bytes)", rel, data.len()));
    ctx.human(format!("  Mirror:    {}", sync_index::internal_path_for(&root, &rel).display()));
    ctx.human("  Sync:      synced");
    Ok(())
}

fn remove_cmd(ctx: Ctx, path: &str, force: bool, keep_source: bool) -> Result<()> {
    let root = crate::commands::require_project_root(&ctx)?;
    let rel = normalize_rel(path)?;

    let mut idx = ImportantIndex::load(&root)?;
    if !idx.files.contains_key(&rel) {
        return Err(KronError::Cli(format!("'{rel}' is not registered as important")));
    }
    if !force {
        return Err(KronError::Cli(format!(
            "remove '{rel}' requires --force"
        )));
    }
    idx.remove_entry(&rel);
    idx.save(&root)?;

    // Anchor #8 / Q3: the **default** now also deletes the project-side
    // source file (matches user intuition). `--keep-source` opts out.
    let also_delete_source = !keep_source;

    // Optionally remove the mirror and/or the project-side source.
    let internal = sync_index::internal_path_for(&root, &rel);
    if internal.exists() {
        let _ = std::fs::remove_file(&internal);
    }
    if also_delete_source {
        let proj = sync_index::project_path_for(&root, &rel);
        if proj.exists() {
            std::fs::remove_file(&proj)?;
        }
    }

    ctx.json(&serde_json::json!({
        "removed": rel,
        "also_deleted_source": also_delete_source,
        "kept_source": keep_source,
    }))?;
    ctx.porcelain(format!("{}\tremoved", rel));
    ctx.human(format!("\u{2713} Unregistered '{rel}' from important"));
    if keep_source {
        ctx.human(format!("  Source file kept: {}", sync_index::project_path_for(&root, &rel).display()));
        ctx.human(format!("  Mirror at {} removed.", internal.display()));
    } else {
        ctx.human(format!("  Mirror at {} removed.", internal.display()));
        let proj = sync_index::project_path_for(&root, &rel);
        if proj.exists() {
            ctx.human(format!("  Source file {} deleted.", proj.display()));
        } else {
            ctx.human("  Source file already missing — mirror removed only.");
        }
    }
    Ok(())
}

fn show_cmd(ctx: Ctx, path: &str) -> Result<()> {
    let root = crate::commands::require_project_root(&ctx)?;
    let rel = normalize_rel(path)?;
    let idx = ImportantIndex::load(&root)?;
    let entry = idx
        .files
        .get(&rel)
        .ok_or_else(|| KronError::Cli(format!("'{rel}' is not registered as important")))?;

    let proj: PathBuf = sync_index::project_path_for(&root, &rel);
    let internal: PathBuf = sync_index::internal_path_for(&root, &rel);
    let proj_exists = proj.exists();
    let internal_exists = internal.exists();

    let row = row_for(&root, &rel, entry);

    ctx.json(&serde_json::json!({
        "path": row.path,
        "sync_state": row.sync_state,
        "project": { "exists": proj_exists, "size": row.project_size, "path": proj.display().to_string() },
        "internal": { "exists": internal_exists, "size": row.internal_size, "path": internal.display().to_string() },
        "added_at": row.added_at,
        "updated_at": row.updated_at,
        "conflict_id": row.conflict_id,
    }))?;
    ctx.porcelain(format!("{}\t{}\t{}\t{}", rel, row.sync_state, row.internal_size, row.updated_at));
    ctx.human(format!("Important file: {}", rel));
    ctx.human(format!("  Sync state:  {}", row.sync_state));
    ctx.human(format!("  Project:     {} ({}, {})",
        if proj_exists { "exists" } else { "missing" },
        row.project_size.map(|s| format!("{s} bytes")).unwrap_or_else(|| "-".into()),
        proj.display()));
    ctx.human(format!("  Internal:    {} ({}, {})",
        if internal_exists { "exists" } else { "missing" },
        row.internal_size,
        internal.display()));
    ctx.human(format!("  Added:       {}", row.added_at));
    ctx.human(format!("  Updated:     {}", row.updated_at));
    if let Some(cid) = &row.conflict_id {
        ctx.human(format!("  Conflict:    {} (run `kron conflict show {}`)", cid, cid));
    }
    Ok(())
}

fn sync_cmd(ctx: Ctx, dry_run: bool) -> Result<()> {
    let root = crate::commands::require_project_root(&ctx)?;
    if dry_run {
        let idx = ImportantIndex::load(&root)?;
        for (rel, e) in &idx.files {
            println!("would_scan\t{}\t{}", rel, e.sync_state);
        }
        return Ok(());
    }
    let result = crate::core::sync::conflict::detect(&root)?;
    let summary = serde_json::json!({
        "scanned": result.stats.scanned,
        "synced": result.stats.synced,
        "conflicts_new": result.stats.conflicts_new,
        "conflicts_existing": result.stats.conflicts_existing,
        "internal_only": result.stats.internal_only,
        "project_only": result.stats.project_only,
    });
    ctx.json(&summary)?;
    ctx.porcelain(format!("scanned={}\tsynced={}\tconflicts_new={}\tinternal_only={}\tproject_only={}",
        result.stats.scanned, result.stats.synced,
        result.stats.conflicts_new, result.stats.internal_only,
        result.stats.project_only));
    ctx.human("Scan complete:");
    ctx.human(format!("  scanned:            {}", result.stats.scanned));
    ctx.human(format!("  synced:             {}", result.stats.synced));
    ctx.human(format!("  conflicts new:      {}", result.stats.conflicts_new));
    ctx.human(format!("  conflicts existing: {}", result.stats.conflicts_existing));
    ctx.human(format!("  internal_only:      {}", result.stats.internal_only));
    ctx.human(format!("  project_only:       {}", result.stats.project_only));
    Ok(())
}
