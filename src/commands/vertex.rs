//! `kron vertex` — vertex management (P4 implementation + Anchor #8 redesign).
//!
//! Anchor #8 changes here:
//! * `-m` / `--message` renamed to `--description` for both `create` and `describe`.
//! * New `vertex use [name]` / `vertex use --unset` subcommand for the
//!   current-vertex pointer (Q10).
//! * `vertex list` marks the current vertex with `*` (in Human and Porcelain
//!   modes).
//!
//! See `dev-docs/dev-journal/2026-09-06-cli-semantic-decisions.md` for
//! the rationale.

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
        #[arg(long)]
        description: Option<String>,
        #[arg(long)]
        branch: Option<String>,
        #[arg(long)]
        path: Option<String>,
    },
    /// Update vertex description.
    Describe {
        name: String,
        #[arg(long)]
        description: String,
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
    /// Set, display, or clear the current-vertex pointer.
    ///
    /// Anchor #8 / Q10: this is the Git-style `HEAD` of Kron — it lets
    /// users run `kron task list` without repeating `--vertex` on every
    /// invocation. Without arguments, prints the current pointer (or
    /// a hint if none is set).
    Use {
        /// Vertex name to point at. Omit to display the current pointer.
        name: Option<String>,
        /// Clear the pointer (after which task commands must specify
        /// `--vertex` explicitly).
        #[arg(long)]
        unset: bool,
    },
    /// Print the current vertex pointer. Equivalent to `vertex use` with
    /// no arguments.
    Current,
}

#[derive(Debug, Serialize)]
struct VertexRow {
    name: String,
    path: String,
    description: Option<String>,
    branch: Option<String>,
    task_count: u32,
    created_at: String,
    updated_at: String,
    current: bool,
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
        VertexAction::Create { name, description, branch, path } => {
            create_cmd(ctx, &name, description.as_deref(), branch.as_deref(), path.as_deref())
        }
        VertexAction::Describe { name, description, editor } => {
            describe_cmd(ctx, &name, &description, editor)
        }
        VertexAction::Branch { name, set } => branch_cmd(ctx, &name, set.as_deref()),
        VertexAction::Delete { name, force, also_remove_dir } => {
            delete_cmd(ctx, &name, force, also_remove_dir)
        }
        VertexAction::Use { name, unset } => use_cmd(ctx, name.as_deref(), unset),
        VertexAction::Current => current_cmd(ctx),
    }
}

fn list_cmd(ctx: Ctx) -> Result<()> {
    let root = crate::commands::require_project_root(&ctx)?;
    let current = crate::core::state_pointer::current(&root).ok().flatten();

    // 2026-09-07 tightening (R1): vertex list is strictly read-only.
    //
    // We deliberately do NOT scan `KRON/VERTEX/` and do NOT auto-register
    // any project-side directory that isn't already in vertices.json.
    // Per the user's design rule, the registry is the single source of
    // truth and may only be mutated through `kron vertex create`.
    //
    // If a directory exists on disk that isn't registered, it shows up
    // only as an orphan *directory* — not as a vertex. Operators who
    // want to register it must run `kron vertex create <name>` (which
    // will then create the missing directory).
    let reg = core_vertex::load_registry(&root)?;

    // Orphan directory scan: list project-side directories that have no
    // registry entry, so users notice and can register them explicitly.
    let public_vertex_root = root.join("KRON").join("VERTEX");
    let mut orphan_dirs: Vec<String> = Vec::new();
    if public_vertex_root.is_dir() {
        for entry in std::fs::read_dir(&public_vertex_root)? {
            let entry = entry?;
            let name = entry.file_name().to_string_lossy().into_owned();
            if !reg.iter().any(|v| v.name == name) {
                orphan_dirs.push(name);
            }
        }
        orphan_dirs.sort();
    }

    let rows: Vec<VertexRow> = reg
        .iter()
        .map(|v| VertexRow {
            current: current.as_deref() == Some(v.name.as_str()),
            name: v.name.clone(),
            path: v.path.clone(),
            description: v.description.clone(),
            branch: v.branch.clone(),
            task_count: count_tasks_in(&root, &v.path),
            created_at: v.created_at.to_rfc3339(),
            updated_at: v.updated_at.to_rfc3339(),
        })
        .collect();

    ctx.json(&serde_json::json!({
        "vertices": rows,
        "total": rows.len(),
        "current": current,
        "orphan_dirs": orphan_dirs,
    }))?;

    for v in &reg {
        let n = count_tasks_in(&root, &v.path);
        let marker = if current.as_deref() == Some(v.name.as_str()) {
            "*"
        } else {
            ""
        };
        ctx.porcelain(&format!(
            "{marker}\t{}\t{}\t{}\t{}\t{}",
            v.name,
            v.path,
            n,
            v.branch.as_deref().unwrap_or("-"),
            v.description.as_deref().unwrap_or("-"),
        ));
    }
    for o in &orphan_dirs {
        ctx.porcelain(&format!("# orphan\t{o}\t(unregistered directory; run `kron vertex create {o}` to register)"));
    }
    if reg.is_empty() && orphan_dirs.is_empty() {
        ctx.porcelain("# (no vertices — run `kron init` then `kron vertex create todo/doing/done`)");
    }

    if reg.is_empty() {
        ctx.human("(no vertices — run `kron init` then `kron vertex create todo/doing/done`)");
    } else {
        ctx.human(&format!(
            "{:<3}  {:<14}  {:<24}  {:<6}  {}",
            "", "NAME", "PATH", "TASKS", "BRANCH / DESCRIPTION"
        ));
        ctx.human(&"-".repeat(86));
        for v in &reg {
            let n = count_tasks_in(&root, &v.path);
            let branch_or_desc = v
                .branch
                .clone()
                .or_else(|| v.description.clone())
                .unwrap_or_else(|| "-".into());
            let marker = if current.as_deref() == Some(v.name.as_str()) {
                "*"
            } else {
                ""
            };
            ctx.human(&format!(
                "{:<3}  {:<14}  {:<24}  {:<6}  {}",
                marker,
                v.name,
                v.path,
                n,
                branch_or_desc,
            ));
        }
    }
    if !orphan_dirs.is_empty() {
        ctx.human("");
        ctx.human("Orphan directories (NOT in registry, will NOT be picked up as vertices):");
        for o in &orphan_dirs {
            ctx.human(&format!("  • {o}  (run `kron vertex create {o}` to register)"));
        }
    }
    if let Some(c) = &current {
        ctx.human("");
        ctx.human(&format!("(current vertex: {c} — change with `kron vertex use <name>`)"));
    } else if !reg.is_empty() {
        ctx.human("");
        ctx.human("(no current vertex — set with `kron vertex use <name>`)");
    }
    Ok(())
}

fn show_cmd(ctx: Ctx, name: &str) -> Result<()> {
    let root = crate::commands::require_project_root(&ctx)?;
    let v = core_vertex::find(&root, name)?
        .ok_or_else(|| KronError::Cli(format!("vertex '{name}' not found")))?;

    let task_count = count_tasks_in(&root, &v.path);
    let state_index = root
        .join("kron-internal")
        .join("states")
        .join(format!("{name}.json"));
    let state_exists = state_index.exists();
    let current = crate::core::state_pointer::current(&root).ok().flatten();
    let is_current = current.as_deref() == Some(v.name.as_str());

    ctx.json(&serde_json::json!({
        "name": v.name,
        "path": v.path,
        "description": v.description,
        "branch": v.branch,
        "task_count": task_count,
        "state_index": state_exists,
        "is_current": is_current,
        "created_at": v.created_at.to_rfc3339(),
        "updated_at": v.updated_at.to_rfc3339(),
    }))?;
    ctx.porcelain(&format!(
        "{}\t{}\t{}\t{}\t{}\t{}",
        v.name,
        v.path,
        task_count,
        v.branch.as_deref().unwrap_or("-"),
        v.description.as_deref().unwrap_or("-"),
        if state_exists { "yes" } else { "no" }
    ));
    ctx.human(&format!(
        "Vertex: {}{}",
        v.name,
        if is_current { " (current)" } else { "" }
    ));
    ctx.human(&format!("  Path:        {}", v.path));
    ctx.human(&format!(
        "  Description: {}",
        v.description.as_deref().unwrap_or("-")
    ));
    ctx.human(&format!("  Branch:      {}", v.branch.as_deref().unwrap_or("-")));
    ctx.human(&format!("  Tasks:       {}", task_count));
    ctx.human(&format!(
        "  State idx:   {}",
        if state_exists { "yes" } else { "no" }
    ));
    ctx.human(&format!("  Created:     {}", v.created_at.to_rfc3339()));
    ctx.human(&format!("  Updated:     {}", v.updated_at.to_rfc3339()));
    Ok(())
}

fn create_cmd(
    ctx: Ctx,
    name: &str,
    description: Option<&str>,
    branch: Option<&str>,
    path: Option<&str>,
) -> Result<()> {
    let root = crate::commands::require_project_root(&ctx)?;
    let rec = core_vertex::create(&root, name, description, branch, path)?;

    ctx.json(&serde_json::json!({
        "name": rec.name,
        "path": rec.path,
        "description": rec.description,
        "branch": rec.branch,
        "created_at": rec.created_at.to_rfc3339(),
        "updated_at": rec.updated_at.to_rfc3339(),
    }))?;
    ctx.porcelain(&format!(
        "{}\t{}\t{}",
        rec.name,
        rec.path,
        rec.created_at.to_rfc3339()
    ));
    ctx.human(&format!("✓ Vertex '{}' created at {}", rec.name, rec.path));
    if let Some(b) = &rec.branch {
        ctx.human(&format!("  Branch:      {}", b));
    }
    if let Some(d) = &rec.description {
        ctx.human(&format!("  Description: {}", d));
    }
    Ok(())
}

fn describe_cmd(ctx: Ctx, name: &str, description: &str, editor: bool) -> Result<()> {
    if editor {
        return Err(KronError::NotYetImplemented("vertex describe --editor"));
    }
    if description.is_empty() {
        return Err(KronError::Cli(
            "--description is required for `vertex describe`".into(),
        ));
    }
    let root = crate::commands::require_project_root(&ctx)?;
    let updated = core_vertex::update(&root, name, Some(description), None)?;

    ctx.json(&serde_json::json!({
        "name": updated.name,
        "description": updated.description,
        "updated_at": updated.updated_at.to_rfc3339(),
    }))?;
    ctx.porcelain(&format!(
        "{}\t{}\t{}",
        updated.name,
        updated.description.as_deref().unwrap_or("-"),
        updated.updated_at.to_rfc3339()
    ));
    ctx.human(&format!(
        "✓ Vertex '{}' description updated",
        updated.name
    ));
    ctx.human(&format!(
        "  Description: {}",
        updated.description.as_deref().unwrap_or("-")
    ));
    Ok(())
}

fn branch_cmd(ctx: Ctx, name: &str, set: Option<&str>) -> Result<()> {
    let root = crate::commands::require_project_root(&ctx)?;
    let updated = core_vertex::update(&root, name, None, set)?;

    ctx.json(&serde_json::json!({
        "name": updated.name,
        "branch": updated.branch,
        "updated_at": updated.updated_at.to_rfc3339(),
    }))?;
    ctx.porcelain(&format!(
        "{}\t{}",
        updated.name,
        updated.branch.as_deref().unwrap_or("-")
    ));
    match &updated.branch {
        Some(b) => ctx.human(&format!("✓ Vertex '{}' bound to branch '{}'", updated.name, b)),
        None => ctx.human(&format!("✓ Vertex '{}' branch binding cleared", updated.name)),
    }
    Ok(())
}

fn delete_cmd(ctx: Ctx, name: &str, force: bool, also_remove_dir: bool) -> Result<()> {
    let root = crate::commands::require_project_root(&ctx)?;
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

    // Anchor #8 / 2026-09-07 tightening: vertex delete no longer
    // implicitly clears the current-vertex pointer. Per the user's
    // design rule, the pointer is a `vertex`-command-group concern
    // only — deleting a vertex is not a vertex-use mutation.
    //
    // If the user just deleted the vertex they were pointing at, we
    // surface a clear warning so they know to run
    // `kron vertex use --unset` (or pick a new vertex). The pointer
    // value is left intact; subsequent task commands will then fail
    // with a clear Cli error that the pointer targets a non-existent
    // vertex, rather than silently auto-clearing.
    let pointer_was_orphaned = matches!(
        crate::core::state_pointer::current(&root),
        Ok(Some(ref cur)) if cur == name
    );

    ctx.json(&serde_json::json!({
        "deleted": name,
        "also_removed_dir": also_remove_dir,
        "pointer_was_orphaned": pointer_was_orphaned,
        "hint": if pointer_was_orphaned {
            Some("kron vertex use --unset")
        } else {
            None
        },
    }))?;
    ctx.porcelain(&format!(
        "{}\tdeleted\t{}",
        name,
        if pointer_was_orphaned { "pointer_orphaned" } else { "ok" }
    ));
    if also_remove_dir {
        ctx.human(&format!("✓ Vertex '{name}' deleted (and project-side directory removed)"));
    } else {
        ctx.human(&format!("✓ Vertex '{name}' removed from registry"));
    }
    if pointer_was_orphaned {
        ctx.human("");
        ctx.human(&format!("  ⚠ the current-vertex pointer still points at '{name}'"));
        ctx.human(&format!(
            "    run `kron vertex use --unset` or `kron vertex use <other>` to repair"
        ));
    }
    Ok(())
}

// ---- Use / Current (Anchor #8 / Q10) ----

fn use_cmd(ctx: Ctx, name: Option<&str>, unset: bool) -> Result<()> {
    let root = crate::commands::require_project_root(&ctx)?;

    // Mutual exclusion: --unset and a name cannot coexist.
    if unset && name.is_some() {
        return Err(KronError::Cli(
            "`vertex use` cannot combine a name with --unset".into(),
        ));
    }

    if unset {
        crate::core::state_pointer::clear(&root)?;
        ctx.json(&serde_json::json!({
            "current": serde_json::Value::Null,
            "cleared": true,
        }))?;
        ctx.porcelain("# (current vertex cleared)");
        ctx.human("✓ Current vertex pointer cleared");
        ctx.human("  (task commands must now pass --vertex explicitly)");
        return Ok(());
    }

    // No argument and no --unset → display only.
    if name.is_none() {
        let current = crate::core::state_pointer::current(&root)?;
        ctx.json(&serde_json::json!({
            "current": current,
        }))?;
        match &current {
            Some(c) => ctx.porcelain(&c.to_string()),
            None => ctx.porcelain("# (no current vertex)"),
        }
        match &current {
            Some(c) => ctx.human(&format!("Current vertex: {c}")),
            None => {
                ctx.human("No current vertex set.");
                ctx.human("  hint: `kron vertex use <name>` to set one");
            }
        }
        return Ok(());
    }

    // Name provided: validate it's registered, then set the pointer.
    let vertex = name.unwrap();
    core_vertex::find_required(&root, vertex)?;
    crate::core::state_pointer::set(&root, vertex)?;

    ctx.json(&serde_json::json!({
        "current": vertex,
        "set": true,
    }))?;
    ctx.porcelain(&format!("{vertex}\tset"));
    ctx.human(&format!("✓ Current vertex set to '{vertex}'"));
    ctx.human("  (subsequent `kron task …` commands will use this vertex unless --vertex overrides)");
    Ok(())
}

fn current_cmd(ctx: Ctx) -> Result<()> {
    use_cmd(ctx, None, false)
}
