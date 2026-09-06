//! `kron task` — task management (subcommand group).

use chrono::Utc;
use clap::{Args, Subcommand};
use serde::Serialize;

use crate::commands::Ctx;
use crate::core::task as core_task;
use crate::error::{KronError, Result};

#[derive(Debug, Args)]
pub struct TaskArgs {
    #[command(subcommand)]
    pub action: TaskAction,
}

#[derive(Debug, Subcommand)]
pub enum TaskAction {
    /// List tasks under a vertex.
    List {
        /// Vertex name (e.g. `todo`, `doing`, `done`).
        vertex: String,
        /// Filter by state.
        #[arg(long)]
        state: Option<String>,
        /// Filter by tag (repeatable).
        #[arg(long)]
        tag: Vec<String>,
    },
    /// Show full task details.
    Show {
        /// Task id (e.g. `T1`).
        id: String,
    },
    /// Add a new task.
    Add {
        /// Vertex name.
        vertex: String,
        /// Short title (shown in listings).
        #[arg(long)]
        title: String,
        /// One-line description (stored in tasks.md front-matter).
        #[arg(long)]
        desc: Option<String>,
        /// Multi-line description body (Markdown).
        #[arg(long, short = 'm')]
        message: Option<String>,
        /// Tag (repeatable).
        #[arg(long)]
        tag: Vec<String>,
    },
    /// Update a task's one-line description.
    Describe {
        id: String,
        #[arg(long, short = 'm')]
        message: String,
        /// Open external editor instead of inline message.
        #[arg(long)]
        editor: bool,
    },
    /// Move a task to a new state column.
    Move {
        id: String,
        #[arg(long)]
        to: String,
    },
    /// Mark a task as doing.
    Start { id: String },
    /// Mark a task as done.
    Done { id: String },
    /// Open the task's Markdown body in $EDITOR.
    Edit { id: String },
    /// Delete a task.
    Delete {
        id: String,
        #[arg(long)]
        force: bool,
    },
    /// Send a task one vertex backwards (e.g. doing -> todo, done -> doing).
    Back { id: String },
    /// Toggle a task's `done` field (default state names: 'done' <-> 'todo').
    Check { id: String },
    /// Manage tags on a task.
    Tag {
        id: String,
        #[command(subcommand)]
        action: TagAction,
    },
    /// Attach arbitrary metadata (frontmatter extension) to a task.
    Attach {
        id: String,
        /// `key=value` pairs (e.g. `priority=high`).
        #[arg(required = true)]
        pairs: Vec<String>,
    },
}

/// Tag sub-commands: add, remove, list, clear.
#[derive(Debug, Subcommand)]
pub enum TagAction {
    /// Add a tag (idempotent).
    Add {
        /// Tag name.
        name: String,
    },
    /// Remove a tag (no-op if absent).
    Remove {
        /// Tag name.
        name: String,
    },
    /// List current tags.
    List,
    /// Clear all tags.
    Clear,
}

#[derive(Serialize)]
struct TaskRow {
    id: String,
    title: String,
    state: String,
    vertex: String,
    tags: Vec<String>,
}

fn require_project() -> Result<std::path::PathBuf> {
    let cwd = std::env::current_dir()?;
    if !cwd.join("kron-internal").join("config.json").exists() {
        return Err(KronError::NotAProject(cwd));
    }
    Ok(cwd)
}

pub fn run(ctx: Ctx, args: TaskArgs) -> Result<()> {
    match args.action {
        TaskAction::List { vertex, state, tag } => list_tasks(ctx, &vertex, state.as_deref(), &tag),
        TaskAction::Show { id } => show_task(ctx, &id),
        TaskAction::Add { vertex, title, desc, message, tag } => {
            add_task(ctx, &vertex, &title, desc.as_deref(), message.as_deref(), &tag)
        }
        TaskAction::Describe { id, message, editor } => {
            describe_task(ctx, &id, &message, editor)
        }
        TaskAction::Move { id, to } => move_task_cmd(ctx, &id, &to),
        TaskAction::Start { id } => move_task_cmd(ctx, &id, "doing"),
        TaskAction::Done { id } => move_task_cmd(ctx, &id, "done"),
        TaskAction::Edit { id } => edit_task_cmd(ctx, &id),
        TaskAction::Delete { id, force } => delete_task_cmd(ctx, &id, force),
        TaskAction::Back { id } => back_task_cmd(ctx, &id),
        TaskAction::Check { id } => check_task_cmd(ctx, &id),
        TaskAction::Tag { id, action } => tag_task_cmd(ctx, &id, action),
        TaskAction::Attach { id, pairs } => attach_task_cmd(ctx, &id, &pairs),
    }
}

// ---- list ----

fn list_tasks(ctx: Ctx, vertex: &str, state_filter: Option<&str>, tag_filter: &[String]) -> Result<()> {
    let project = require_project()?;
    core_task::validate_vertex_name(vertex)?;

    let dir = core_task::vertex_public_dir(&project, vertex);
    if !dir.exists() {
        return match ctx.mode {
            crate::output::OutputMode::Json => {
                println!("[]");
                Ok(())
            }
            _ => {
                println!("(vertex '{vertex}' has no tasks yet)");
                Ok(())
            }
        };
    }

    let files = core_task::list_task_files(&dir)?;
    let mut rows = Vec::new();
    for f in &files {
        let t = core_task::read_task(f)?;
        if let Some(s) = state_filter {
            if t.state != s {
                continue;
            }
        }
        if !tag_filter.is_empty() {
            let hit = tag_filter.iter().all(|needle| t.tags.iter().any(|t_tag| t_tag == needle));
            if !hit {
                continue;
            }
        }
        rows.push(TaskRow {
            id: t.id,
            title: t.title,
            state: t.state,
            vertex: vertex.to_string(),
            tags: t.tags,
        });
    }

    match ctx.mode {
        crate::output::OutputMode::Json => {
            println!("{}", serde_json::to_string_pretty(&rows)?);
        }
        crate::output::OutputMode::Porcelain => {
            for r in &rows {
                println!("{}\t{}\t{}\t{}\t{}", r.id, r.vertex, r.state, r.title, r.tags.join(","));
            }
            if rows.is_empty() {
                println!("# (empty)");
            }
        }
        crate::output::OutputMode::Human => {
            if rows.is_empty() {
                println!("(no tasks in '{vertex}')");
                return Ok(());
            }
            println!("{:<6}  {:<12}  {}", "ID", "STATE", "TITLE");
            println!("{}", "-".repeat(60));
            for r in &rows {
                println!("{:<6}  {:<12}  {}", r.id, r.state, r.title);
            }
        }
    }
    Ok(())
}

// ---- show ----

fn show_task(ctx: Ctx, id: &str) -> Result<()> {
    let project = require_project()?;
    let (path, _vertex) = core_task::find_task(&project, id)?;
    let t = core_task::read_task(&path)?;

    match ctx.mode {
        crate::output::OutputMode::Json => {
            println!("{}", serde_json::to_string_pretty(&t)?);
        }
        crate::output::OutputMode::Porcelain => {
            println!("{}\t{}\t{}\t{}", t.id, t.state, t.title, t.tags.join(","));
            println!("# description: {}", t.description);
            if !t.body.is_empty() {
                println!("# body: {}", t.body);
            }
        }
        crate::output::OutputMode::Human => {
            println!("Task {} [{}]", t.id, t.state);
            println!("  Title:       {}", t.title);
            println!("  Description: {}", t.description);
            if !t.tags.is_empty() {
                println!("  Tags:        {}", t.tags.join(", "));
            }
            println!("  Created:     {}", t.created_at.to_rfc3339());
            println!("  Updated:     {}", t.updated_at.to_rfc3339());
            println!("  File:        {}", path.display());
            if !t.body.is_empty() {
                println!();
                println!("{}", t.body);
            }
        }
    }
    Ok(())
}

// ---- describe (update description) ----

fn describe_task(ctx: Ctx, id: &str, message: &str, editor: bool) -> Result<()> {
    if editor {
        return Err(KronError::NotYetImplemented("task describe --editor"));
    }
    if message.is_empty() {
        return Err(KronError::Cli("--message is required for `task describe`".into()));
    }
    let project = require_project()?;
    let (path, _vertex) = core_task::find_task(&project, id)?;
    let mut task = core_task::read_task(&path)?;

    let (desc, truncated) = core_task::normalize_description(message);
    task.description = desc;
    task.updated_at = Utc::now();
    task.source_file = None;
    core_task::update_task(&path, &task)?;

    match ctx.mode {
        crate::output::OutputMode::Json => {
            let summary = serde_json::json!({
                "id": task.id,
                "description": task.description,
                "updated_at": task.updated_at.to_rfc3339(),
                "truncated": truncated,
            });
            println!("{}", serde_json::to_string_pretty(&summary)?);
        }
        crate::output::OutputMode::Porcelain => {
            println!("{}\t{}", task.id, task.description);
            if truncated {
                println!("# (truncated to 200 chars)");
            }
        }
        crate::output::OutputMode::Human => {
            println!("\u{2713} Task {} description updated", task.id);
            println!("  Description: {}", task.description);
            if truncated {
                println!("\u{26a0}  description was truncated to 200 chars");
            }
        }
    }
    Ok(())
}

// ---- move (also handles start/done as synonyms) ----

fn move_task_cmd(ctx: Ctx, id: &str, to: &str) -> Result<()> {
    let normalized = crate::model::parse_state(to)
        .map_err(|e| KronError::Cli(format!("invalid target state '{to}': {e}")))?;
    crate::core::task::validate_vertex_name(&normalized)
        .map_err(|e| KronError::Cli(format!("invalid target vertex '{normalized}': {e}")))?;

    let project = require_project()?;
    let (new_path, new_vertex) = core_task::move_task(&project, id, &normalized)?;

    match ctx.mode {
        crate::output::OutputMode::Json => {
            let summary = serde_json::json!({
                "id": id,
                "vertex": new_vertex,
                "file": new_path.display().to_string(),
            });
            println!("{}", serde_json::to_string_pretty(&summary)?);
        }
        crate::output::OutputMode::Porcelain => {
            println!("{}\t{}", id, new_vertex);
        }
        crate::output::OutputMode::Human => {
            println!("\u{2713} {} moved to {} ({})", id, new_vertex, new_path.display());
        }
    }
    Ok(())
}

// ---- delete ----

fn delete_task_cmd(ctx: Ctx, id: &str, force: bool) -> Result<()> {
    let project = require_project()?;
    let (path, vertex) = core_task::find_task(&project, id)?;

    if !force {
        // In human mode and TTY we could prompt; for the skeleton we just
        // require --force to avoid accidental deletions.
        return Err(KronError::Cli(format!(
            "delete {id} requires --force (file: {})",
            path.display()
        )));
    }

    core_task::delete_task(&project, id)?;

    match ctx.mode {
        crate::output::OutputMode::Json => {
            let summary = serde_json::json!({
                "id": id,
                "deleted_from": vertex,
                "file": path.display().to_string(),
            });
            println!("{}", serde_json::to_string_pretty(&summary)?);
        }
        crate::output::OutputMode::Porcelain => {
            println!("{}\t{}\t{}", id, vertex, "deleted");
        }
        crate::output::OutputMode::Human => {
            println!("\u{2713} Task {id} deleted (was in '{vertex}')");
        }
    }
    Ok(())
}

// ---- add ----

fn add_task(
    ctx: Ctx,
    vertex: &str,
    title: &str,
    desc: Option<&str>,
    body: Option<&str>,
    tags: &[String],
) -> Result<()> {
    let project = require_project()?;
    core_task::validate_vertex_name(vertex)?;

    // Ensure vertex dir + state file exist.
    let vertex_dir = core_task::vertex_public_dir(&project, vertex);
    std::fs::create_dir_all(&vertex_dir)?;

    let id = core_task::next_task_id(&project, vertex)?;
    let raw_desc = desc.unwrap_or("");
    let (description, _truncated) = core_task::normalize_description(raw_desc);
    let body_str = body.unwrap_or("").to_string();
    let now = Utc::now();

    let task = core_task::Task {
        id: id.clone(),
        title: title.to_string(),
        description,
        body: body_str,
        state: vertex.to_string(),
        tags: tags.to_vec(),
        created_at: now,
        updated_at: now,
        source_file: None,
    };

    let path = core_task::write_task(&project, vertex, &task)?;
    core_task::append_to_vertex_state(&project, vertex, &id)?;

    match ctx.mode {
        crate::output::OutputMode::Json => {
            let summary = serde_json::json!({
                "id": task.id,
                "vertex": vertex,
                "title": task.title,
                "state": task.state,
                "tags": task.tags,
                "file": path.display().to_string(),
            });
            println!("{}", serde_json::to_string_pretty(&summary)?);
        }
        crate::output::OutputMode::Porcelain => {
            println!("{}\t{}\t{}\t{}", task.id, vertex, task.state, task.title);
        }
        crate::output::OutputMode::Human => {
            println!("\u{2713} Task {} created at {}", task.id, path.display());
            println!("  Title:       {}", task.title);
            if !task.description.is_empty() {
                println!("  Description: {}", task.description);
            }
            if !task.tags.is_empty() {
                println!("  Tags:        {}", task.tags.join(", "));
            }
        }
    }
    Ok(())
}

// ---- edit (open $EDITOR) ----

fn edit_task_cmd(ctx: Ctx, id: &str) -> Result<()> {
    use std::process::Command;
    let project = require_project()?;
    let (path, _vertex) = core_task::find_task(&project, id)?;

    let editor = std::env::var("EDITOR")
        .or_else(|_| std::env::var("VISUAL"))
        .unwrap_or_else(|_| {
            if cfg!(windows) {
                "notepad".to_string()
            } else {
                "vi".to_string()
            }
        });

    let status = Command::new(&editor)
        .arg(&path)
        .status()
        .map_err(|e| KronError::Cli(format!("failed to launch editor {editor:?}: {e}")))?;

    if !status.success() {
        return Err(KronError::Cli(format!(
            "editor exited with non-zero status: {status}"
        )));
    }

    // Re-read the task to confirm parseability after edit.
    let task = core_task::read_task(&path)?;
    match ctx.mode {
        crate::output::OutputMode::Json => {
            println!("{}", serde_json::to_string_pretty(&serde_json::json!({
                "id": task.id,
                "title": task.title,
                "state": task.state,
                "file": path.display().to_string(),
            }))?);
        }
        _ => {
            println!("\u{2713} Edited task {} (file: {})", task.id, path.display());
        }
    }
    Ok(())
}

// ---- back (move one vertex backwards) ----

fn back_task_cmd(ctx: Ctx, id: &str) -> Result<()> {
    let project = require_project()?;
    let (path, current) = core_task::find_task(&project, id)?;
    let task = core_task::read_task(&path)?;
    let previous = previous_state(&current).ok_or_else(|| {
        KronError::Cli(format!("vertex '{current}' has no previous state to go back to"))
    })?;
    let _ = task; // not strictly needed, but ensures we have a valid task
    let (new_path, new_vertex) = core_task::move_task(&project, id, &previous)?;

    match ctx.mode {
        crate::output::OutputMode::Json => {
            println!("{}", serde_json::to_string_pretty(&serde_json::json!({
                "id": id,
                "from": current,
                "to": new_vertex,
                "file": new_path.display().to_string(),
            }))?);
        }
        crate::output::OutputMode::Porcelain => {
            println!("{}\t{}\t{}", id, current, new_vertex);
        }
        crate::output::OutputMode::Human => {
            println!("\u{2713} {} moved back: {} -> {} ({})", id, current, new_vertex, new_path.display());
        }
    }
    Ok(())
}

/// Default ordering of state vertices. Users can extend this by
/// editing the `DEFAULT_STATE_ORDER` constant; custom vertices without
/// an entry fall back to `todo` as their predecessor.
fn previous_state(current: &str) -> Option<String> {
    const DEFAULT_STATE_ORDER: &[&str] = &["backlog", "todo", "doing", "review", "done"];
    let pos = DEFAULT_STATE_ORDER.iter().position(|s| *s == current)?;
    if pos == 0 {
        None
    } else {
        Some(DEFAULT_STATE_ORDER[pos - 1].to_string())
    }
}

// ---- check (toggle done <-> todo) ----

fn check_task_cmd(ctx: Ctx, id: &str) -> Result<()> {
    let project = require_project()?;
    let (_path, current) = core_task::find_task(&project, id)?;

    let target = if current == "done" { "todo" } else { "done" };
    let (new_path, new_vertex) = core_task::move_task(&project, id, target)?;

    match ctx.mode {
        crate::output::OutputMode::Json => {
            println!("{}", serde_json::to_string_pretty(&serde_json::json!({
                "id": id,
                "from": current,
                "to": new_vertex,
                "file": new_path.display().to_string(),
            }))?);
        }
        crate::output::OutputMode::Porcelain => {
            println!("{}\t{}\t{}", id, current, new_vertex);
        }
        crate::output::OutputMode::Human => {
            println!("\u{2713} {} toggled: {} -> {} ({})", id, current, new_vertex, new_path.display());
        }
    }
    Ok(())
}

// ---- tag (add/remove/list/clear) ----

fn tag_task_cmd(ctx: Ctx, id: &str, action: TagAction) -> Result<()> {
    let project = require_project()?;
    let (path, _vertex) = core_task::find_task(&project, id)?;
    let mut task = core_task::read_task(&path)?;

    let summary = match action {
        TagAction::Add { name } => {
            if name.is_empty() {
                return Err(KronError::Cli("tag name must not be empty".into()));
            }
            if !task.tags.iter().any(|t| t == &name) {
                task.tags.push(name.clone());
            }
            task.updated_at = Utc::now();
            task.source_file = None;
            core_task::update_task(&path, &task)?;
            serde_json::json!({"id": task.id, "tags": task.tags})
        }
        TagAction::Remove { name } => {
            let before = task.tags.len();
            task.tags.retain(|t| t != &name);
            if task.tags.len() != before {
                task.updated_at = Utc::now();
                task.source_file = None;
                core_task::update_task(&path, &task)?;
            }
            serde_json::json!({"id": task.id, "tags": task.tags})
        }
        TagAction::List => {
            serde_json::json!({"id": task.id, "tags": task.tags})
        }
        TagAction::Clear => {
            task.tags.clear();
            task.updated_at = Utc::now();
            task.source_file = None;
            core_task::update_task(&path, &task)?;
            serde_json::json!({"id": task.id, "tags": task.tags})
        }
    };

    match ctx.mode {
        crate::output::OutputMode::Json => {
            println!("{}", serde_json::to_string_pretty(&summary)?);
        }
        crate::output::OutputMode::Porcelain => {
            println!("{}\t{}", task.id, task.tags.join(","));
        }
        crate::output::OutputMode::Human => {
            if task.tags.is_empty() {
                println!("(no tags)");
            } else {
                println!("{}: {}", task.id, task.tags.join(", "));
            }
        }
    }
    Ok(())
}

// ---- attach (key=value metadata) ----

fn attach_task_cmd(ctx: Ctx, id: &str, pairs: &[String]) -> Result<()> {
    let project = require_project()?;
    let (path, _vertex) = core_task::find_task(&project, id)?;
    let mut task = core_task::read_task(&path)?;

    // Parse pairs. Allow `=` in the value (only first `=` splits).
    let mut attached = serde_json::Map::new();
    for p in pairs {
        let (k, v) = p.split_once('=').ok_or_else(|| {
            KronError::Cli(format!("expected key=value, got {p:?}"))
        })?;
        let key = k.trim().to_string();
        if key.is_empty() {
            return Err(KronError::Cli(format!("empty key in pair {p:?}")));
        }
        attached.insert(key, serde_json::Value::String(v.to_string()));
    }

    // Merge into body as a fenced JSON block at the top of the body.
    let merged_json = match serde_json::from_str::<serde_json::Value>(&task.body) {
        Ok(existing) if existing.is_object() => {
            let mut obj = existing;
            if let Some(map) = obj.as_object_mut() {
                for (k, v) in attached {
                    map.insert(k, v);
                }
            }
            obj
        }
        _ => serde_json::Value::Object(attached),
    };
    task.body = serde_json::to_string_pretty(&merged_json)?;
    task.updated_at = Utc::now();
    task.source_file = None;
    core_task::update_task(&path, &task)?;

    match ctx.mode {
        crate::output::OutputMode::Json => {
            println!("{}", serde_json::to_string_pretty(&serde_json::json!({
                "id": task.id,
                "attached": merged_json,
                "body_size": task.body.len(),
            }))?);
        }
        crate::output::OutputMode::Porcelain => {
            println!("{}\t{}\t{}", task.id, merged_json, task.body.len());
        }
        crate::output::OutputMode::Human => {
            println!("\u{2713} Attached metadata to task {}", task.id);
            println!("  Body now: {}", serde_json::to_string_pretty(&merged_json)?);
        }
    }
    Ok(())
}
