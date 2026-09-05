//! Task storage: per-task `.md` file under each vertex, with a global
//! summary `tasks.json` per vertex under `kron-internal/states/`.
//!
//! Format of each task file (simplified M1):
//!
//! ```markdown
//! ---
//! id: T1
//! title: "First task"
//! state: todo
//! tags: [foo, bar]
//! created_at: 2026-09-05T...
//! updated_at: 2026-09-05T...
//! ---
//!
//! <!-- description -->
//!
//! Short description here (one-line).
//!
//! <!-- /description -->
//!
//! <!-- body -->
//!
//! Optional long Markdown body.
//!
//! <!-- /body -->
//! ```
//!
//! NOTE: this is intentionally simpler than the full schema in
//! dev-docs/design/01-数据模型.md § 5. We use one-file-per-task because the
//! Phase-1 milestone demo (`M1.md`) does. Phase 4 will introduce
//! `tasks.md` per vertex.

use crate::error::{KronError, Result};
use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use std::fs;
use std::path::{Path, PathBuf};

// ---- Public types ----

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct Task {
    pub id: String,
    pub title: String,
    pub description: String,
    pub body: String,
    pub state: String,
    pub tags: Vec<String>,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
    /// Source file (set when read from disk).
    #[serde(skip_serializing_if = "Option::is_none", default)]
    pub source_file: Option<PathBuf>,
}

/// Per-vertex state index. Persisted to `kron-internal/states/<vertex>.json`.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct VertexState {
    pub tasks: Vec<String>, // task IDs
}

/// Locate a vertex directory under `KRON/VERTEX/<name>/`.
pub fn vertex_public_dir(project_root: &Path, vertex: &str) -> PathBuf {
    project_root.join("KRON").join("VERTEX").join(vertex)
}

/// Locate the kron-internal state file for a vertex.
pub fn vertex_state_file(project_root: &Path, vertex: &str) -> PathBuf {
    project_root
        .join("kron-internal")
        .join("states")
        .join(format!("{vertex}.json"))
}

/// List all `.md` task files under a vertex.
pub fn list_task_files(vertex_dir: &Path) -> Result<Vec<PathBuf>> {
    if !vertex_dir.exists() {
        return Err(KronError::NotFound(vertex_dir.to_path_buf()));
    }
    let mut out = Vec::new();
    for entry in fs::read_dir(vertex_dir)? {
        let entry = entry?;
        let p = entry.path();
        if p.is_file() && p.extension().and_then(|x| x.to_str()) == Some("md") {
            out.push(p);
        }
    }
    out.sort();
    Ok(out)
}

/// Read a single task from a `.md` file.
pub fn read_task(path: &Path) -> Result<Task> {
    let raw = fs::read_to_string(path)?;
    parse_task_md(&raw).map(|mut t| {
        t.source_file = Some(path.to_path_buf());
        t
    })
}

/// Write a single task to its `.md` file. Creates parent dirs as needed.
pub fn write_task(project_root: &Path, vertex: &str, task: &Task) -> Result<PathBuf> {
    let dir = vertex_public_dir(project_root, vertex);
    fs::create_dir_all(&dir)?;
    let path = dir.join(format!("{}.md", task.id));
    fs::write(&path, render_task_md(task)?)?;
    Ok(path)
}

/// Read the state index for a vertex (returns empty default if file is missing).
pub fn read_vertex_state(project_root: &Path, vertex: &str) -> Result<VertexState> {
    let p = vertex_state_file(project_root, vertex);
    if !p.exists() {
        return Ok(VertexState::default());
    }
    let raw = fs::read_to_string(&p)?;
    Ok(serde_json::from_str(&raw).unwrap_or_default())
}

/// Append a task ID to the vertex state index.
pub fn append_to_vertex_state(project_root: &Path, vertex: &str, task_id: &str) -> Result<()> {
    let p = vertex_state_file(project_root, vertex);
    if let Some(parent) = p.parent() {
        fs::create_dir_all(parent)?;
    }
    let mut state = read_vertex_state(project_root, vertex)?;
    if !state.tasks.iter().any(|id| id == task_id) {
        state.tasks.push(task_id.to_string());
    }
    fs::write(&p, serde_json::to_string_pretty(&state)?)?;
    Ok(())
}

/// Compute the next sequential task ID under a vertex.
/// Format: `T<seq>` where seq is 1-based within the vertex.
pub fn next_task_id(project_root: &Path, vertex: &str) -> Result<String> {
    let dir = vertex_public_dir(project_root, vertex);
    if !dir.exists() {
        return Ok("T1".to_string());
    }
    let mut max: u32 = 0;
    for path in list_task_files(&dir)? {
        if let Some(stem) = path.file_stem().and_then(|s| s.to_str()) {
            if let Some(rest) = stem.strip_prefix('T') {
                if let Ok(n) = rest.parse::<u32>() {
                    if n > max {
                        max = n;
                    }
                }
            }
        }
    }
    Ok(format!("T{}", max + 1))
}

// ---- Markdown rendering ----

fn render_task_md(task: &Task) -> Result<String> {
    // Minimal YAML front matter using serde_yaml.
    #[derive(Serialize)]
    struct Front<'a> {
        id: &'a str,
        title: &'a str,
        state: &'a str,
        tags: &'a Vec<String>,
        created_at: String,
        updated_at: String,
    }
    let fm = Front {
        id: &task.id,
        title: &task.title,
        state: &task.state,
        tags: &task.tags,
        created_at: task.created_at.to_rfc3339(),
        updated_at: task.updated_at.to_rfc3339(),
    };
    let yaml = serde_yaml::to_string(&fm).map_err(KronError::Yaml)?;
    Ok(format!(
        "---\n{yaml}---\n\n<!-- description -->\n{}\n<!-- /description -->\n\n<!-- body -->\n{}\n<!-- /body -->\n",
        task.description, task.body
    ))
}

fn parse_task_md(raw: &str) -> Result<Task> {
    // Split on the closing '---' of front matter.
    let stripped = raw.strip_prefix("---").ok_or_else(|| {
        KronError::InvalidFrontmatter {
            file: PathBuf::from("<inline>"),
            reason: "missing opening '---'".into(),
        }
    })?;
    let after_open = &stripped[1..]; // skip the newline after opening ---
    let close = after_open.find("\n---").ok_or_else(|| KronError::InvalidFrontmatter {
        file: PathBuf::from("<inline>"),
        reason: "missing closing '---'".into(),
    })?;
    let yaml_str = &after_open[..close];
    let rest = &after_open[close + 4..]; // skip '\n---'

    #[derive(Deserialize)]
    struct Front {
        id: String,
        title: String,
        state: String,
        tags: Vec<String>,
        created_at: DateTime<Utc>,
        updated_at: DateTime<Utc>,
    }
    let fm: Front = serde_yaml::from_str(yaml_str).map_err(|e| KronError::InvalidFrontmatter {
        file: PathBuf::from("<inline>"),
        reason: format!("yaml parse: {e}"),
    })?;

    // Extract description and body using sentinel comments.
    let description = extract_block(rest, "description").unwrap_or_default();
    let body = extract_block(rest, "body").unwrap_or_default();

    Ok(Task {
        id: fm.id,
        title: fm.title,
        description,
        body,
        state: fm.state,
        tags: fm.tags,
        created_at: fm.created_at,
        updated_at: fm.updated_at,
        source_file: None,
    })
}

fn extract_block(rest: &str, name: &str) -> Option<String> {
    let open_tag = format!("<!-- {name} -->");
    let close_tag = format!("<!-- /{name} -->");
    let start = rest.find(&open_tag)? + open_tag.len();
    let after = &rest[start..];
    let end = after.find(&close_tag)?;
    Some(after[..end].trim().to_string())
}

// ---- Validation ----

const SLUG_RE: &str = r"^[a-z0-9][a-z0-9_-]*$";

/// Validate that a vertex name is a valid slug.
pub fn validate_vertex_name(name: &str) -> Result<()> {
    // We can't pull in `regex` for one check; do it by hand.
    if name.is_empty() {
        return Err(KronError::InvalidVertexName(name.to_string()));
    }
    let mut chars = name.chars();
    let first = chars.next().unwrap();
    if !(first.is_ascii_lowercase() || first.is_ascii_digit()) {
        return Err(KronError::InvalidVertexName(name.to_string()));
    }
    for c in chars {
        if !(c.is_ascii_lowercase() || c.is_ascii_digit() || c == '_' || c == '-') {
            return Err(KronError::InvalidVertexName(name.to_string()));
        }
    }
    let _ = SLUG_RE; // referenced for documentation
    Ok(())
}

/// Truncate a one-line description to ≤200 chars (warn-level, but we still
/// persist the truncated version — exit code stays 0).
pub fn normalize_description(s: &str) -> (String, bool) {
    let trimmed: String = s.chars().take(200).collect();
    let truncated = trimmed.len() < s.chars().count();
    (trimmed, truncated)
}

/// A no-op token exported for tests that need a SLUG_RE reference.
pub const SLUG_REGEX_DOC: &str = SLUG_RE;
