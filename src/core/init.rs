//! Workspace initialization.
//!
//! Implements `kron init`: creates the dual-source directory layout,
//! writes `kron-internal/config.json`, `kron-internal/vertices.json`.
//!
//! Layout (per dev-docs/design/00-总览与架构.md § 5):
//!
//! ```text
//! <project_root>/
//! ├── kron-internal/
//! │   ├── config.json
//! │   └── vertices.json
//! └── KRON/
//!     ├── README.md
//!     ├── VERTEX/             (unless --no-vertex)
//!     └── important/
//! ```

use crate::error::{KronError, Result};
use crate::model::LinkMode;
use crate::model::{Project, VertexEntry};
use std::fs;
use std::path::{Path, PathBuf};

/// What `kron init` actually did — used for status reporting.
#[derive(Debug, Clone, serde::Serialize)]
pub struct InitOutcome {
    pub project: Project,
    pub link_mode: LinkMode,
    pub no_vertex: bool,
    pub created: Vec<String>,
    pub skipped: Vec<String>,
}

/// Outcome flags returned by `prepare`.
#[derive(Debug)]
pub struct Prepare {
    pub project_root: PathBuf,
    pub kron_dir: PathBuf,
    pub kron_dir_exists: bool,
    pub is_git: bool,
}

/// Inspect the target directory and decide whether init can proceed.
pub fn prepare(project_root: &Path, no_git: bool) -> Result<Prepare> {
    if !project_root.exists() {
        return Err(KronError::NotFound(project_root.to_path_buf()));
    }
    if !project_root.is_dir() {
        return Err(KronError::NotAProject(project_root.to_path_buf()));
    }

    let kron_dir = project_root.join("kron-internal");
    let kron_dir_exists = kron_dir.exists();

    let is_git = project_root.join(".git").exists();
    if !is_git && !no_git {
        return Err(KronError::NotGitRepo(project_root.to_path_buf()));
    }

    Ok(Prepare {
        project_root: project_root.to_path_buf(),
        kron_dir,
        kron_dir_exists,
        is_git,
    })
}

/// Create the dual-source layout. Caller must check `force` before calling.
pub fn materialize(prep: &Prepare, no_vertex: bool, link_mode: LinkMode) -> Result<InitOutcome> {
    let mode = link_mode.fallback_for_platform();
    let mut created = Vec::new();
    let mut skipped = Vec::new();

    // 1) kron-internal/
    if !prep.kron_dir.exists() {
        fs::create_dir_all(&prep.kron_dir)?;
        created.push("kron-internal/".to_string());
    } else {
        skipped.push("kron-internal/".to_string());
    }

    // 2) kron-internal/config.json
    let project = Project::new(prep.project_root.clone(), mode);
    let config_path = prep.kron_dir.join("config.json");
    let cfg_json = serde_json::to_string_pretty(&project)?;
    write_if_absent(&config_path, &cfg_json, &mut created, &mut skipped)?;

    // 3) kron-internal/vertices.json (empty registry)
    let vertices_path = prep.kron_dir.join("vertices.json");
    let empty_registry: Vec<VertexEntry> = vec![];
    let vr_json = serde_json::to_string_pretty(&empty_registry)?;
    write_if_absent(&vertices_path, &vr_json, &mut created, &mut skipped)?;

    // 4) KRON/  (project-side human/AI readable mirror)
    let kron_public = prep.project_root.join("KRON");
    if !kron_public.exists() {
        fs::create_dir_all(&kron_public)?;
        created.push("KRON/".to_string());
    } else {
        skipped.push("KRON/".to_string());
    }

    // 5) KRON/README.md (friendly entry point for humans/AI)
    let readme = kron_public.join("README.md");
    write_if_absent(
        &readme,
        "# KRON\n\nProject-side mirror of Kron state.\n\n\
         - `VERTEX/<name>/` — one folder per vertex\n\
         - `important/` — important files registry\n\
         - `.kron-context/` — AI-friendly summary (auto-generated)\n",
        &mut created,
        &mut skipped,
    )?;

    // 6) KRON/important/  (and bridge to kron-internal/important/)
    let public_important = kron_public.join("important");
    if !public_important.exists() {
        fs::create_dir_all(&public_important)?;
        created.push("KRON/important/".to_string());
    } else {
        skipped.push("KRON/important/".to_string());
    }
    let internal_important = prep.kron_dir.join("important");
    if !internal_important.exists() {
        fs::create_dir_all(&internal_important)?;
        created.push("kron-internal/important/".to_string());
    } else {
        skipped.push("kron-internal/important/".to_string());
    }

    // 7) KRON/VERTEX/  (unless --no-vertex)
    let public_vertex = kron_public.join("VERTEX");
    if no_vertex {
        skipped.push("KRON/VERTEX/ (--no-vertex)".to_string());
    } else if !public_vertex.exists() {
        fs::create_dir_all(&public_vertex)?;
        created.push("KRON/VERTEX/".to_string());
    } else {
        skipped.push("KRON/VERTEX/".to_string());
    }

    Ok(InitOutcome {
        project,
        link_mode: mode,
        no_vertex,
        created,
        skipped,
    })
}

fn write_if_absent(
    path: &Path,
    contents: &str,
    created: &mut Vec<String>,
    skipped: &mut Vec<String>,
) -> Result<()> {
    if path.exists() {
        skipped.push(format!("{}", path.display()));
        return Ok(());
    }
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent)?;
    }
    fs::write(path, contents)?;
    created.push(format!("{}", path.display()));
    Ok(())
}
