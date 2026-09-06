//! `kron vertex` — vertex management (P4 implementation).

use clap::{Args, Subcommand};
use serde::Serialize;

use crate::commands::Ctx;
use crate::core::vertex as core_vertex;
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
    /// Bind/unbind a Git branch to a vertex (metadata only in v1).
    Branch {
        name: String,
        /// Provide an empty value to clear the binding.
        #[arg(long)]
        set: Option<String>,
    },
    /// Delete a vertex.
    Delete {
        name: String,
        #[arg(long)]
        force: bool,
        #[arg(long)]
        also_remove_dir: bool,
    },
}

#[derive(Serialize)]
struct VertexRow {
    name: String,
    path: String,
    description: Option<String>,
    branch: Option<String>,
    task_count: u32,
    created_at: String,
    updated_at: String,
}

fn project_root() -> Result<std::path::PathBuf> {
    let cwd = std::env::current_dir()?;
    if !cwd.join("kron-internal").join("config.json").exists() {
        return Err(KronError::NotAProject(cwd));
    }
    Ok(cwd)
}

fn count_tasks_in(project_root: &std::path::Path, vpath: &str) -> u32 {
    let dir = project_root.join(vpath);
    if !dir.is_dir() {
        return 0;
    }
    fs_count_md(&dir)
}

fn fs_count_md(dir: &std::path::Path) -> u32 {
    std::fs::read_dir(dir)
        .map(|rd| {
            rd.filter_map(|e| e.ok())
                .filter(|e| {
                    e.path().is_file()
                        && e.path().extension().map(|x| x == "md").unwrap_or(false)
                })
                .count() as u32
        })
        .unwrap_or(0)
}

pub fn run(ctx: Ctx, args: VertexArgs) -> Result<()> {
    match args.action {
        VertexAction::List => list_cmd(ctx),
        VertexAction::Show { name } => show_cmd(ctx, &name),
        VertexAction::Create { name, message, branch, path } => {
            create_cmd(ctx, &name, message.as_deref(), branch.as_deref(), path.as_deref())
        }
        VertexAction::Describe { name, message, editor } => {
            describe_cmd(ctx, &name, &message, editor)
        }
        VertexAction::Branch { name, set } => branch_cmd(ctx, &name, set.as_deref()),
        VertexAction::Delete { name, force, also_remove_dir } => {
            delete_cmd(ctx, &name, force, also_remove_dir)
        }
    }
}

fn list_cmd(ctx: Ctx) -> Result<()> {
    let root = project_root()?;
    // Auto-register any project-side VERTEX dirs that aren't yet in the registry.
    let mut reg = core_vertex::load_registry(&root)?;
    let public_vertex_root = root.join("KRON").join("VERTEX");
    if public_vertex_root.is_dir() {
        for entry in std::fs::read_dir(&public_vertex_root)? {
            let entry = entry?;
            let name = entry.file_name().to_string_lossy().into_owned();
            if !reg.iter().any(|v| v.name == name) {
                reg.push(core_vertex::VertexRecord {
                    name: name.clone(),
                    path: core_vertex::default_path(&name),
                    description: None,
                    branch: None,
                    created_at: chrono::Utc::now(),
                    updated_at: chrono::Utc::now(),
                });
            }
        }
        core_vertex_save(&root, &reg)?;
    }

    match ctx.mode {
        crate::output::OutputMode::Json => {
            let rows: Vec<VertexRow> = reg
                .iter()
                .map(|v| VertexRow {
                    name: v.name.clone(),
                    path: v.path.clone(),
                    description: v.description.clone(),
                    branch: v.branch.clone(),
                    task_count: count_tasks_in(&root, &v.path),
                    created_at: v.created_at.to_rfc3339(),
                    updated_at: v.updated_at.to_rfc3339(),
                })
                .collect();
            println!("{}", serde_json::to_string_pretty(&serde_json::json!({
                "vertices": rows,
                "total": rows.len(),
            }))?);
        }
        crate::output::OutputMode::Porcelain => {
            for v in &reg {
                let n = count_tasks_in(&root, &v.path);
                println!(
                    "{}\t{}\t{}\t{}\t{}",
                    v.name,
                    v.path,
                    n,
                    v.branch.as_deref().unwrap_or("-"),
                    v.description.as_deref().unwrap_or("-"),
                );
            }
            if reg.is_empty() {
                println!("# (no vertices — run `kron init` first)");
            }
        }
        crate::output::OutputMode::Human => {
            if reg.is_empty() {
                println!("(no vertices — run `kron init` first)");
                return Ok(());
            }
            println!("{:<14}  {:<24}  {:<6}  {}", "NAME", "PATH", "TASKS", "BRANCH / DESCRIPTION");
            println!("{}", "-".repeat(80));
            for v in &reg {
                let n = count_tasks_in(&root, &v.path);
                let branch_or_desc = v
                    .branch
                    .clone()
                    .or_else(|| v.description.clone())
                    .unwrap_or_else(|| "-".into());
                println!(
                    "{:<14}  {:<24}  {:<6}  {}",
                    v.name,
                    v.path,
                    n,
                    branch_or_desc,
                );
            }
        }
    }
    Ok(())
}

fn show_cmd(ctx: Ctx, name: &str) -> Result<()> {
    let root = project_root()?;
    let v = core_vertex::find(&root, name)?
        .ok_or_else(|| KronError::Cli(format!("vertex '{name}' not found")))?;

    let task_count = count_tasks_in(&root, &v.path);
    let state_index = root
        .join("kron-internal")
        .join("states")
        .join(format!("{name}.json"));
    let state_exists = state_index.exists();

    match ctx.mode {
        crate::output::OutputMode::Json => {
            println!("{}", serde_json::to_string_pretty(&serde_json::json!({
                "name": v.name,
                "path": v.path,
                "description": v.description,
                "branch": v.branch,
                "task_count": task_count,
                "state_index": state_exists,
                "created_at": v.created_at.to_rfc3339(),
                "updated_at": v.updated_at.to_rfc3339(),
            }))?);
        }
        crate::output::OutputMode::Porcelain => {
            println!(
                "{}\t{}\t{}\t{}\t{}\t{}",
                v.name,
                v.path,
                task_count,
                v.branch.as_deref().unwrap_or("-"),
                v.description.as_deref().unwrap_or("-"),
                if state_exists { "yes" } else { "no" }
            );
        }
        crate::output::OutputMode::Human => {
            println!("Vertex: {}", v.name);
            println!("  Path:        {}", v.path);
            println!("  Description: {}", v.description.as_deref().unwrap_or("-"));
            println!("  Branch:      {}", v.branch.as_deref().unwrap_or("-"));
            println!("  Tasks:       {}", task_count);
            println!("  State idx:   {}", if state_exists { "yes" } else { "no" });
            println!("  Created:     {}", v.created_at.to_rfc3339());
            println!("  Updated:     {}", v.updated_at.to_rfc3339());
        }
    }
    Ok(())
}

fn create_cmd(
    ctx: Ctx,
    name: &str,
    description: Option<&str>,
    branch: Option<&str>,
    path: Option<&str>,
) -> Result<()> {
    let root = project_root()?;
    let rec = core_vertex::create(&root, name, description, branch, path)?;

    match ctx.mode {
        crate::output::OutputMode::Json => {
            println!("{}", serde_json::to_string_pretty(&serde_json::json!({
                "name": rec.name,
                "path": rec.path,
                "description": rec.description,
                "branch": rec.branch,
                "created_at": rec.created_at.to_rfc3339(),
                "updated_at": rec.updated_at.to_rfc3339(),
            }))?);
        }
        crate::output::OutputMode::Porcelain => {
            println!("{}\t{}\t{}", rec.name, rec.path, rec.created_at.to_rfc3339());
        }
        crate::output::OutputMode::Human => {
            println!("\u{2713} Vertex '{}' created at {}", rec.name, rec.path);
            if let Some(b) = &rec.branch {
                println!("  Branch:      {}", b);
            }
            if let Some(d) = &rec.description {
                println!("  Description: {}", d);
            }
        }
    }
    Ok(())
}

fn describe_cmd(ctx: Ctx, name: &str, message: &str, editor: bool) -> Result<()> {
    if editor {
        return Err(KronError::NotYetImplemented("vertex describe --editor"));
    }
    if message.is_empty() {
        return Err(KronError::Cli(
            "--message is required for `vertex describe`".into(),
        ));
    }
    let root = project_path()?;
    let _ = root; // satisfy linter for now (we use project_path() but use root below)
    let root = project_root()?;
    let updated = core_vertex::update(&root, name, Some(message), None)?;

    match ctx.mode {
        crate::output::OutputMode::Json => {
            println!("{}", serde_json::to_string_pretty(&serde_json::json!({
                "name": updated.name,
                "description": updated.description,
                "updated_at": updated.updated_at.to_rfc3339(),
            }))?);
        }
        crate::output::OutputMode::Porcelain => {
            println!(
                "{}\t{}\t{}",
                updated.name,
                updated.description.as_deref().unwrap_or("-"),
                updated.updated_at.to_rfc3339()
            );
        }
        crate::output::OutputMode::Human => {
            println!(
                "\u{2713} Vertex '{}' description updated",
                updated.name
            );
            println!(
                "  Description: {}",
                updated.description.as_deref().unwrap_or("-")
            );
        }
    }
    Ok(())
}

fn branch_cmd(ctx: Ctx, name: &str, set: Option<&str>) -> Result<()> {
    let root = project_root()?;
    let updated = core_vertex::update(&root, name, None, set)?;

    match ctx.mode {
        crate::output::OutputMode::Json => {
            println!("{}", serde_json::to_string_pretty(&serde_json::json!({
                "name": updated.name,
                "branch": updated.branch,
                "updated_at": updated.updated_at.to_rfc3339(),
            }))?);
        }
        crate::output::OutputMode::Porcelain => {
            println!("{}\t{}", updated.name, updated.branch.as_deref().unwrap_or("-"));
        }
        crate::output::OutputMode::Human => {
            match &updated.branch {
                Some(b) => println!("\u{2713} Vertex '{}' bound to branch '{}'", updated.name, b),
                None => println!("\u{2713} Vertex '{}' branch binding cleared", updated.name),
            }
        }
    }
    Ok(())
}

fn delete_cmd(ctx: Ctx, name: &str, force: bool, also_remove_dir: bool) -> Result<()> {
    let root = project_path()?;
    let _ = root;
    let root = project_root()?;
    if !force && core_vertex::find(&root, name)?.is_some() {
        // Refuse without --force when the vertex has tasks.
        let v = core_vertex::find(&root, name)?.unwrap();
        let n = count_tasks_in(&root, &v.path);
        if n > 0 && !also_remove_dir {
            return Err(KronError::Cli(format!(
                "vertex '{name}' has {n} task(s); use --force --also-remove-dir to delete"
            )));
        }
        if !force {
            return Err(KronError::Cli(format!(
                "delete vertex '{name}' requires --force"
            )));
        }
    }
    core_vertex::delete(&root, name, also_remove_dir)?;

    match ctx.mode {
        crate::output::OutputMode::Json => {
            println!(
                "{}",
                serde_json::to_string_pretty(&serde_json::json!({
                    "deleted": name,
                    "also_removed_dir": also_remove_dir,
                }))?
            );
        }
        crate::output::OutputMode::Porcelain => {
            println!("{}\tdeleted", name);
        }
        crate::output::OutputMode::Human => {
            if also_remove_dir {
                println!(
                    "\u{2713} Vertex '{name}' deleted (and project-side directory removed)"
                );
            } else {
                println!("\u{2713} Vertex '{name}' removed from registry");
            }
        }
    }
    Ok(())
}

/// Internal: persist the registry (used by `list_cmd` to auto-register).
fn core_vertex_save(root: &std::path::Path, records: &[core_vertex::VertexRecord]) -> Result<()> {
    let path = root.join("kron-internal").join("vertices.json");
    std::fs::write(&path, serde_json::to_string_pretty(records)?)?;
    Ok(())
}

/// Kept here to satisfy the unused-helper lint when `project_path` is unused.
#[allow(dead_code)]
fn project_path() -> Result<std::path::PathBuf> {
    project_root()
}
