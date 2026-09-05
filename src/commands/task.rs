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
            if editor {
                Err(KronError::NotYetImplemented("task describe --editor"))
            } else if message.is_empty() {
                Err(KronError::Cli("--message is required for `task describe`".into()))
            } else {
                let _ = id;
                Err(KronError::NotYetImplemented("task describe"))
            }
        }
        TaskAction::Move { id, to } => {
            let _ = (id, to);
            Err(KronError::NotYetImplemented("task move"))
        }
        TaskAction::Start { id } => {
            let _ = id;
            Err(KronError::NotYetImplemented("task start"))
        }
        TaskAction::Done { id } => {
            let _ = id;
            Err(KronError::NotYetImplemented("task done"))
        }
        TaskAction::Edit { id } => {
            let _ = id;
            Err(KronError::NotYetImplemented("task edit"))
        }
        TaskAction::Delete { id, force } => {
            let _ = (id, force);
            Err(KronError::NotYetImplemented("task delete"))
        }
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
    let vertex_dir = project.join("KRON").join("VERTEX");

    if !vertex_dir.is_dir() {
        return Err(KronError::NotFound(vertex_dir));
    }

    for entry in std::fs::read_dir(&vertex_dir)? {
        let entry = entry?;
        let path = entry.path();
        if !path.is_dir() {
            continue;
        }
        let candidate = path.join(format!("{id}.md"));
        if candidate.exists() {
            let t = core_task::read_task(&candidate)?;
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
                    println!("  File:        {}", candidate.display());
                    if !t.body.is_empty() {
                        println!();
                        println!("{}", t.body);
                    }
                }
            }
            return Ok(());
        }
    }
    Err(KronError::NotFound(project.join("KRON").join("VERTEX").join(format!("{id}.md"))))
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
