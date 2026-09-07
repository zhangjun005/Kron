//! `kron task` — task management (subcommand group).
//!
//! Anchor #8 brought a major redesign here; the doc at
//! `dev-docs/dev-journal/2026-09-06-cli-semantic-decisions.md` is the
//! canonical reference. Highlights:
//!
//! * `--desc` → `--description`, `-m` / `--message` → `--body`
//! * `task add <vertex>` refuses to create a vertex implicitly (Q6)
//! * `task add` always writes `state: todo` (Q7)
//! * `task list` uses the current-vertex pointer unless `--vertex` overrides
//! * `task delete / show / edit / describe` require explicit `--vertex` when
//!   the target task lives outside the current vertex (Q11)
//! * `task move --to <state>` rejects anything but `todo / doing / done`

use chrono::Utc;
use clap::{Args, Subcommand};
use serde::Serialize;

use crate::commands::Ctx;
use crate::core::task as core_task;
use crate::error::{KronError, Result};
use crate::model::TaskState;
use crate::output::OutputMode;

/// Resolve the vertex that a task command should target.
///
/// Resolution order (matches the decision tree in 2026-09-06-cli-semantic-decisions.md § 2.4):
///
/// 1. Explicit `--vertex` always wins.
/// 2. Otherwise the current-vertex pointer (`kron-internal/state.json`).
/// 3. Otherwise a `Cli` error directing the user to set one or pass `--vertex`.
fn resolve_target_vertex(
    _ctx: &Ctx,
    project_root: &std::path::Path,
    explicit: Option<&str>,
) -> Result<String> {
    if let Some(v) = explicit {
        return Ok(v.to_string());
    }
    if let Some(v) = crate::core::state_pointer::current(project_root)? {
        return Ok(v);
    }
    Err(KronError::Cli(
        "no current vertex set; pass --vertex <name> or run 'kron vertex use <name>' first".into(),
    ))
}

/// Confirm a vertex is both *registered* (in `vertices.json`) and *known*
/// (used by commands that don't need a physical directory but still need
/// a well-defined namespace). Returns the record on success.
fn ensure_vertex_registered(
    project_root: &std::path::Path,
    vertex: &str,
) -> Result<crate::core::vertex::VertexRecord> {
    crate::core::vertex::find_required(project_root, vertex)
}

#[derive(Debug, Args)]
pub struct TaskArgs {
    #[command(subcommand)]
    pub action: TaskAction,
}

#[derive(Debug, Subcommand)]
pub enum TaskAction {
    /// List tasks under a vertex.
    List {
        /// Vertex name (defaults to current vertex if `--vertex` omitted).
        vertex: Option<String>,
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
        /// Vertex the task lives under. Defaults to the current vertex; if
        /// the task lives elsewhere, `--vertex` is required so we don't
        /// silently operate across vertices (Anchor #8 / Q11).
        #[arg(long)]
        vertex: Option<String>,
    },
    /// Add a new task to an already-registered vertex.
    Add {
        /// Vertex name. Must be registered via `kron vertex create <name>`
        /// first; implicit vertex creation is intentionally not allowed
        /// (Anchor #8 / Q6).
        vertex: String,
        /// Short title (shown in listings). Always required.
        #[arg(long)]
        title: String,
        /// One-line description (≤200 chars, stored in front-matter).
        #[arg(long)]
        description: Option<String>,
        /// Multi-line body (Markdown). When omitted the task is description-only.
        #[arg(long)]
        body: Option<String>,
        /// Tag (repeatable).
        #[arg(long)]
        tag: Vec<String>,
    },
    /// Update a task's one-line description.
    Describe {
        id: String,
        /// New one-line description. Must be ≤200 chars; longer input
        /// is rejected with a `Cli` error instead of being silently
        /// truncated (Anchor #8 / Q8).
        #[arg(long)]
        description: String,
        /// Vertex the task lives under. See `task show` for the rules.
        #[arg(long)]
        vertex: Option<String>,
        /// Open external editor instead of inline description.
        #[arg(long)]
        editor: bool,
    },
    /// Move a task to a new state column (`todo | doing | done`).
    ///
    /// Move is the **only** command that may silently cross vertex
    /// boundaries (Anchor #8 / Q11): changing state necessarily means
    /// changing directories when state and directory are kept in
    /// lock-step (decision Q9 — vertex = physical directory).
    Move {
        id: String,
        /// Target state — must be one of `todo`, `doing`, `done`.
        #[arg(long)]
        to: String,
    },
    /// Mark a task as doing.
    Start { id: String },
    /// Mark a task as done.
    Done { id: String },
    /// Open the task's Markdown body in $EDITOR.
    Edit {
        id: String,
        #[arg(long)]
        vertex: Option<String>,
    },
    /// Delete a task.
    Delete {
        id: String,
        #[arg(long)]
        force: bool,
        #[arg(long)]
        vertex: Option<String>,
    },
    /// Send a task one state backwards along the canonical
    /// `todo → doing → done` chain (`done → doing`, `doing → todo`).
    Back { id: String },
    /// Toggle a task's state between `done` and `todo`.
    Check { id: String },
    /// Manage tags on a task.
    Tag {
        id: String,
        #[arg(long)]
        vertex: Option<String>,
        #[command(subcommand)]
        action: TagAction,
    },
    /// Attach arbitrary metadata (frontmatter extension) to a task.
    Attach {
        id: String,
        /// `key=value` pairs (e.g. `priority=high`).
        #[arg(required = true)]
        pairs: Vec<String>,
        #[arg(long)]
        vertex: Option<String>,
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

pub fn run(ctx: Ctx, args: TaskArgs) -> Result<()> {
    match args.action {
        TaskAction::List { vertex, state, tag } => list_tasks(ctx, vertex.as_deref(), state.as_deref(), &tag),
        TaskAction::Show { id, vertex } => show_task(ctx, &id, vertex.as_deref()),
        TaskAction::Add { vertex, title, description, body, tag } => {
            add_task(ctx, &vertex, &title, description.as_deref(), body.as_deref(), &tag)
        }
        TaskAction::Describe { id, description, vertex, editor } => {
            describe_task(ctx, &id, &description, vertex.as_deref(), editor)
        }
        TaskAction::Move { id, to } => move_task_cmd(ctx, &id, &to),
        TaskAction::Start { id } => move_task_cmd(ctx, &id, "doing"),
        TaskAction::Done { id } => move_task_cmd(ctx, &id, "done"),
        TaskAction::Edit { id, vertex } => edit_task_cmd(ctx, &id, vertex.as_deref()),
        TaskAction::Delete { id, force, vertex } => delete_task_cmd(ctx, &id, force, vertex.as_deref()),
        TaskAction::Back { id } => back_task_cmd(ctx, &id),
        TaskAction::Check { id } => check_task_cmd(ctx, &id),
        TaskAction::Tag { id, vertex, action } => tag_task_cmd(ctx, &id, vertex.as_deref(), action),
        TaskAction::Attach { id, pairs, vertex } => attach_task_cmd(ctx, &id, &pairs, vertex.as_deref()),
    }
}

// ---- list ----

fn list_tasks(ctx: Ctx, vertex: Option<&str>, state_filter: Option<&str>, tag_filter: &[String]) -> Result<()> {
    let project = crate::commands::require_project_root(&ctx)?;
    let vertex = resolve_target_vertex(&ctx, &project, vertex)?;
    // 2026-09-07 tightening (R2): task operations are strictly confined
    // to existing vertex directories. We validate both registration
    // (via find) and physical existence before listing.
    let _ = crate::core::vertex::find(&project, &vertex);
    core_task::require_vertex_dir(&project, &vertex)?;

    let dir = core_task::vertex_public_dir(&project, &vertex);
    if !dir.exists() {
        if ctx.mode == OutputMode::Json {
            ctx.json(&serde_json::json!([]))?;
        }
        ctx.human(format!("(vertex '{vertex}' has no tasks yet)"));
        return Ok(());
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
            vertex: vertex.clone(),
            tags: t.tags,
        });
    }

    ctx.json(&rows)?;
    for r in &rows {
        ctx.porcelain(format!(
            "{}\t{}\t{}\t{}\t{}",
            r.id,
            r.vertex,
            r.state,
            r.title,
            r.tags.join(",")
        ));
    }
    if rows.is_empty() {
        ctx.porcelain("# (empty)");
    }
    if rows.is_empty() {
        ctx.human(format!("(no tasks in '{vertex}')"));
        return Ok(());
    }
    ctx.human(format!(
        "{:<6}  {:<12}  {}\n{}\n{}",
        "ID",
        "STATE",
        "TITLE",
        "-".repeat(60),
        rows
            .iter()
            .map(|r| format!("{:<6}  {:<12}  {}", r.id, r.state, r.title))
            .collect::<Vec<_>>()
            .join("\n")
    ));
    Ok(())
}

// ---- show ----

fn show_task(ctx: Ctx, id: &str, vertex: Option<&str>) -> Result<()> {
    let project = crate::commands::require_project_root(&ctx)?;
    let target = vertex.map(|s| s.to_string());
    let _ = resolve_target_vertex(&ctx, &project, target.as_deref())?;
    let (path, actual_vertex) = core_task::find_task(&project, id)?;

    // Cross-vertex guard (Q11): if the user gave --vertex it must match.
    // Otherwise the task must live in the current vertex.
    if let Some(v) = target.as_deref() {
        if v != actual_vertex {
            return Err(KronError::Cli(format!(
                "task {id} is in vertex '{actual_vertex}', not '{v}'; \
                 either correct --vertex or omit it (and use the current-vertex pointer)"
            )));
        }
    } else if let Some(current) = crate::core::state_pointer::current(&project)? {
        if current != actual_vertex {
            return Err(KronError::Cli(format!(
                "task {id} is in vertex '{actual_vertex}', not the current '{current}'; \
                 cross-vertex operations require explicit --vertex {actual_vertex}"
            )));
        }
    }
    // If neither --vertex nor current vertex is set, we silently allow it
    // (the user must have known what they were doing).

    let t = core_task::read_task(&path)?;

    ctx.json(&t)?;
    ctx.porcelain(format!(
        "{}\t{}\t{}\t{}",
        t.id,
        t.state,
        t.title,
        t.tags.join(",")
    ));
    ctx.porcelain(format!("# description: {}", t.description));
    if !t.body.is_empty() {
        ctx.porcelain(format!("# body: {}", t.body));
    }
    ctx.human(format!("Task {} [{}]", t.id, t.state));
    ctx.human(format!("  Title:       {}", t.title));
    ctx.human(format!("  Description: {}", t.description));
    if !t.tags.is_empty() {
        ctx.human(format!("  Tags:        {}", t.tags.join(", ")));
    }
    ctx.human(format!("  Created:     {}", t.created_at.to_rfc3339()));
    ctx.human(format!("  Updated:     {}", t.updated_at.to_rfc3339()));
    ctx.human(format!("  File:        {}", path.display()));
    if !t.body.is_empty() {
        ctx.human(String::new());
        ctx.human(t.body.clone());
    }
    Ok(())
}

// ---- describe (update description) ----

fn describe_task(ctx: Ctx, id: &str, description: &str, vertex: Option<&str>, editor: bool) -> Result<()> {
    if editor {
        return Err(KronError::NotYetImplemented("task describe --editor"));
    }
    if description.is_empty() {
        return Err(KronError::Cli("--description is required for `task describe`".into()));
    }
    let project = crate::commands::require_project_root(&ctx)?;
    let target = vertex.map(|s| s.to_string());
    let _ = resolve_target_vertex(&ctx, &project, target.as_deref())?;
    let (path, actual_vertex) = core_task::find_task(&project, id)?;

    // Same cross-vertex rule as `task show`.
    if let Some(v) = target.as_deref() {
        if v != actual_vertex {
            return Err(KronError::Cli(format!(
                "task {id} is in vertex '{actual_vertex}', not '{v}'"
            )));
        }
    } else if let Some(current) = crate::core::state_pointer::current(&project)? {
        if current != actual_vertex {
            return Err(KronError::Cli(format!(
                "task {id} is in vertex '{actual_vertex}', not the current '{current}'; \
                 pass --vertex {actual_vertex} explicitly"
            )));
        }
    }

    // Strong validation: >200 chars is a hard error, not a silent truncation
    // (Anchor #8 / Q8).
    let desc = core_task::validate_description(description)?;

    let mut task = core_task::read_task(&path)?;
    task.description = desc;
    task.updated_at = Utc::now();
    task.source_file = None;
    core_task::update_task(&path, &task)?;

    ctx.json(&serde_json::json!({
        "id": task.id,
        "description": task.description,
        "updated_at": task.updated_at.to_rfc3339(),
    }))?;
    ctx.porcelain(format!("{}\t{}", task.id, task.description));
    ctx.human(format!("\u{2713} Task {} description updated", task.id));
    ctx.human(format!("  Description: {}", task.description));
    Ok(())
}

// ---- move (also handles start/done as synonyms) ----

fn move_task_cmd(ctx: Ctx, id: &str, to: &str) -> Result<()> {
    // Anchor #8 / Q1 + Q9: state is now strictly one of todo/doing/done.
    // The destination state IS the target directory name (kept in
    // lock-step with the front-matter field), and the destination
    // directory is created by `core::task::move_task` if it doesn't
    // exist yet.
    let state: TaskState = to.parse().map_err(|e: String| KronError::Cli(format!("invalid target state '{to}': {e}")))?;

    let project = crate::commands::require_project_root(&ctx)?;
    let (new_path, new_vertex) = core_task::move_task(&project, id, state.as_str())?;

    ctx.json(&serde_json::json!({
        "id": id,
        "state": new_vertex,
        "vertex": new_vertex,
        "file": new_path.display().to_string(),
    }))?;
    ctx.porcelain(format!("{}\t{}", id, new_vertex));
    ctx.human(format!("\u{2713} {} moved to {} ({})", id, new_vertex, new_path.display()));
    Ok(())
}

// ---- delete ----

fn delete_task_cmd(ctx: Ctx, id: &str, force: bool, vertex: Option<&str>) -> Result<()> {
    let project = crate::commands::require_project_root(&ctx)?;
    let target = vertex.map(|s| s.to_string());
    let _ = resolve_target_vertex(&ctx, &project, target.as_deref())?;
    let (path, actual_vertex) = core_task::find_task(&project, id)?;

    // Cross-vertex guard (Q11).
    if let Some(v) = target.as_deref() {
        if v != actual_vertex {
            return Err(KronError::Cli(format!(
                "task {id} is in vertex '{actual_vertex}', not '{v}'"
            )));
        }
    } else if let Some(current) = crate::core::state_pointer::current(&project)? {
        if current != actual_vertex {
            return Err(KronError::Cli(format!(
                "task {id} is in vertex '{actual_vertex}', not the current '{current}'; \
                 pass --vertex {actual_vertex} to operate across vertices"
            )));
        }
    }

    if !force {
        // In human mode and TTY we could prompt; for the skeleton we just
        // require --force to avoid accidental deletions.
        return Err(KronError::Cli(format!(
            "delete {id} requires --force (file: {})",
            path.display()
        )));
    }

    core_task::delete_task(&project, id)?;

    ctx.json(&serde_json::json!({
        "id": id,
        "deleted_from": actual_vertex,
        "file": path.display().to_string(),
    }))?;
    ctx.porcelain(format!("{}\t{}\t{}", id, actual_vertex, "deleted"));
    ctx.human(format!("\u{2713} Task {id} deleted (was in '{actual_vertex}')"));
    Ok(())
}

// ---- add ----

fn add_task(
    ctx: Ctx,
    vertex: &str,
    title: &str,
    description: Option<&str>,
    body: Option<&str>,
    tags: &[String],
) -> Result<()> {
    let project = crate::commands::require_project_root(&ctx)?;
    core_task::validate_vertex_name(vertex)?;

    // Anchor #8 / Q6: refuse to create a vertex implicitly. The vertex
    // MUST be registered (vertices.json) before a task can be added to it.
    ensure_vertex_registered(&project, vertex)?;

    // Anchor #8 / Q8: title + (description OR body) — at least one of the
    // latter two is required so we never persist a task that's just a title.
    let desc_str = description.unwrap_or("");
    let body_str = body.unwrap_or("");
    if desc_str.is_empty() && body_str.is_empty() {
        return Err(KronError::Cli(
            "task requires either --description or --body (in addition to --title)".into(),
        ));
    }

    // Strong validation: >200 chars is a hard error, not silent truncation.
    let description = core_task::validate_description(desc_str)?;

    // Anchor #8 / Q7: state is always `todo` for new tasks. The vertex
    // directory under which we store the file may be anything registered,
    // but the logical state is fixed at creation time.
    let state = TaskState::default_for_new();

    // R2: require physical directory before writing.
    core_task::require_vertex_dir(&project, vertex)?;

    let id = core_task::next_task_id(&project, vertex)?;
    let now = Utc::now();

    let task = core_task::Task {
        id: id.clone(),
        title: title.to_string(),
        description,
        body: body_str.to_string(),
        state: state.as_str().to_string(),
        tags: tags.to_vec(),
        created_at: now,
        updated_at: now,
        source_file: None,
    };

    let path = core_task::write_task(&project, vertex, &task)?;
    core_task::append_to_vertex_state(&project, vertex, &id)?;

    ctx.json(&serde_json::json!({
        "id": task.id,
        "vertex": vertex,
        "state": task.state,
        "title": task.title,
        "tags": task.tags,
        "file": path.display().to_string(),
    }))?;
    ctx.porcelain(format!(
        "{}\t{}\t{}\t{}",
        task.id,
        vertex,
        task.state,
        task.title
    ));
    ctx.human(format!("\u{2713} Task {} created at {}", task.id, path.display()));
    ctx.human(format!("  Title:       {}", task.title));
    if !task.description.is_empty() {
        ctx.human(format!("  Description: {}", task.description));
    }
    if !task.tags.is_empty() {
        ctx.human(format!("  Tags:        {}", task.tags.join(", ")));
    }
    Ok(())
}

// ---- edit (open $EDITOR) ----

fn edit_task_cmd(ctx: Ctx, id: &str, vertex: Option<&str>) -> Result<()> {
    use std::process::Command;
    let project = crate::commands::require_project_root(&ctx)?;
    let target = vertex.map(|s| s.to_string());
    let _ = resolve_target_vertex(&ctx, &project, target.as_deref())?;
    let (path, actual_vertex) = core_task::find_task(&project, id)?;

    if let Some(v) = target.as_deref() {
        if v != actual_vertex {
            return Err(KronError::Cli(format!(
                "task {id} is in vertex '{actual_vertex}', not '{v}'"
            )));
        }
    } else if let Some(current) = crate::core::state_pointer::current(&project)? {
        if current != actual_vertex {
            return Err(KronError::Cli(format!(
                "task {id} is in vertex '{actual_vertex}', not the current '{current}'; \
                 pass --vertex {actual_vertex} to edit across vertices"
            )));
        }
    }

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
    if ctx.mode == OutputMode::Json {
        ctx.json(&serde_json::json!({
            "id": task.id,
            "title": task.title,
            "state": task.state,
            "file": path.display().to_string(),
        }))?;
    }
    ctx.human(format!(
        "\u{2713} Edited task {} (file: {})",
        task.id,
        path.display()
    ));
    Ok(())
}

// ---- back (move one state backwards along the canonical chain) ----

fn back_task_cmd(ctx: Ctx, id: &str) -> Result<()> {
    let project = crate::commands::require_project_root(&ctx)?;
    let (path, current) = core_task::find_task(&project, id)?;

    // Anchor #8: back uses the canonical TaskState chain, not a magic
    // DEFAULT_STATE_ORDER. If `current` isn't a recognised state
    // (e.g. legacy vertex directory names), we report a clear error.
    let current_state = crate::model::parse_state(&current)
        .map_err(|_| KronError::Cli(format!(
            "task {id} has legacy state '{current}' which is not one of todo/doing/done; \
             move it to a valid state first with `kron task move {id} --to todo|doing|done`"
        )))?;
    let previous = current_state.prev_in_chain().ok_or_else(|| {
        KronError::Cli(format!("task {id} is already in 'todo' (terminal state of back chain)"))
    })?;
    let _ = path; // not strictly needed, but ensures we have a valid path
    let (new_path, new_vertex) = core_task::move_task(&project, id, previous.as_str())?;
    ctx.json(&serde_json::json!({
        "id": id,
        "from": current,
        "to": new_vertex,
        "file": new_path.display().to_string(),
    }))?;
    ctx.porcelain(format!("{}\t{}\t{}", id, current, new_vertex));
    ctx.human(format!(
        "\u{2713} {} moved back: {} -> {} ({})",
        id,
        current,
        new_vertex,
        new_path.display()
    ));
    Ok(())
}

// ---- check (toggle done <-> todo) ----

fn check_task_cmd(ctx: Ctx, id: &str) -> Result<()> {
    let project = crate::commands::require_project_root(&ctx)?;
    let (_path, current) = core_task::find_task(&project, id)?;

    let current_state = crate::model::parse_state(&current).map_err(|_| {
        KronError::Cli(format!(
            "task {id} has legacy state '{current}' which is not one of todo/doing/done; \
             use `kron task move {id} --to todo|doing|done` first"
        ))
    })?;
    // Toggle along the canonical chain: done → todo, anything else → done.
    let target = match current_state {
        TaskState::Done => TaskState::Todo,
        _ => TaskState::Done,
    };
    let (new_path, new_vertex) = core_task::move_task(&project, id, target.as_str())?;
    ctx.json(&serde_json::json!({
        "id": id,
        "from": current,
        "to": new_vertex,
        "file": new_path.display().to_string(),
    }))?;
    ctx.porcelain(format!("{}\t{}\t{}", id, current, new_vertex));
    ctx.human(format!(
        "\u{2713} {} toggled: {} -> {} ({})",
        id,
        current,
        new_vertex,
        new_path.display()
    ));
    Ok(())
}

// ---- tag (add/remove/list/clear) ----

fn tag_task_cmd(ctx: Ctx, id: &str, vertex: Option<&str>, action: TagAction) -> Result<()> {
    let project = crate::commands::require_project_root(&ctx)?;
    let target = vertex.map(|s| s.to_string());
    let _ = resolve_target_vertex(&ctx, &project, target.as_deref())?;
    let (path, actual_vertex) = core_task::find_task(&project, id)?;
    if let Some(v) = target.as_deref() {
        if v != actual_vertex {
            return Err(KronError::Cli(format!(
                "task {id} is in vertex '{actual_vertex}', not '{v}'"
            )));
        }
    } else if let Some(current) = crate::core::state_pointer::current(&project)? {
        if current != actual_vertex {
            return Err(KronError::Cli(format!(
                "task {id} is in vertex '{actual_vertex}', not the current '{current}'; \
                 pass --vertex {actual_vertex} to operate across vertices"
            )));
        }
    }

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

    ctx.json(&summary)?;
    ctx.porcelain(format!("{}\t{}", task.id, task.tags.join(",")));
    if task.tags.is_empty() {
        ctx.human("(no tags)");
    } else {
        ctx.human(format!("{}: {}", task.id, task.tags.join(", ")));
    }
    Ok(())
}

// ---- attach (key=value metadata) ----

fn attach_task_cmd(ctx: Ctx, id: &str, pairs: &[String], vertex: Option<&str>) -> Result<()> {
    let project = crate::commands::require_project_root(&ctx)?;
    let target = vertex.map(|s| s.to_string());
    let _ = resolve_target_vertex(&ctx, &project, target.as_deref())?;
    let (path, actual_vertex) = core_task::find_task(&project, id)?;
    if let Some(v) = target.as_deref() {
        if v != actual_vertex {
            return Err(KronError::Cli(format!(
                "task {id} is in vertex '{actual_vertex}', not '{v}'"
            )));
        }
    } else if let Some(current) = crate::core::state_pointer::current(&project)? {
        if current != actual_vertex {
            return Err(KronError::Cli(format!(
                "task {id} is in vertex '{actual_vertex}', not the current '{current}'; \
                 pass --vertex {actual_vertex} to operate across vertices"
            )));
        }
    }

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

    ctx.json(&serde_json::json!({
        "id": task.id,
        "attached": merged_json,
        "body_size": task.body.len(),
    }))?;
    ctx.porcelain(format!(
        "{}\t{}\t{}",
        task.id,
        merged_json,
        task.body.len()
    ));
    ctx.human(format!("\u{2713} Attached metadata to task {}", task.id));
    ctx.human(format!("  Body now: {}", serde_json::to_string_pretty(&merged_json)?));
    Ok(())
}
