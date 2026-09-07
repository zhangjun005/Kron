//! `kron-context` — AI-friendly context generation (P5).
//!
//! Generates `.kron-context/` under `KRON/` with fact-only structured
//! snapshots. Scope is intentionally narrow (v1 only):
//!
//! - `git/recent-commits.md`   — last 100 commits from `git log`
//! - `git/branch-summary.md`   — current vs main branch diff
//! - `code/structure.md`        — project directory tree (depth 3)
//! - `README.md`                — index / scope boundary
//!
//! Philosophy: Kron generates **facts only** — data reproducible from git
//! or the filesystem. No LLM, no API key, no semantic understanding.
//! (see 04b § 3.7 and Q28.)

use std::path::{Path, PathBuf};

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};

use crate::error::{KronError, Result};

pub mod code_structure;
pub mod git_branch;
pub mod git_commits;
pub mod readme;

// ---------------------------------------------------------------------------
// Path constants
// ---------------------------------------------------------------------------

/// `.kron-context/` base directory, relative to `KRON/`.
pub const KRON_CONTEXT_DIR: &str = ".kron-context";

/// Sub-paths under `.kron-context/`.
pub const CTX_GIT_COMMITS: &str = "git/recent-commits.md";
pub const CTX_GIT_BRANCH: &str = "git/branch-summary.md";
pub const CTX_CODE_STRUCTURE: &str = "code/structure.md";
pub const CTX_README: &str = "README.md";

/// Max file size for any context file (1 MB).
pub const MAX_FILE_SIZE: usize = 1 * 1024 * 1024;

/// Default directory tree depth.
pub const DEFAULT_TREE_DEPTH: u32 = 3;

/// Default number of recent commits.
pub const DEFAULT_COMMIT_COUNT: usize = 100;

// ---------------------------------------------------------------------------
// Document registry
// ---------------------------------------------------------------------------

/// Discriminates what kind of fact a document contains.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum DocType {
    GitCommits,
    GitBranch,
    CodeStructure,
    Readme,
}

impl DocType {
    pub fn path(&self) -> &'static str {
        match self {
            DocType::GitCommits => CTX_GIT_COMMITS,
            DocType::GitBranch => CTX_GIT_BRANCH,
            DocType::CodeStructure => CTX_CODE_STRUCTURE,
            DocType::Readme => CTX_README,
        }
    }

    pub fn all() -> &'static [DocType] {
        &[
            DocType::GitCommits,
            DocType::GitBranch,
            DocType::CodeStructure,
            DocType::Readme,
        ]
    }
}

/// Metadata stored in the registry JSON.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DocMeta {
    /// Relative path under `.kron-context/`.
    pub path: String,
    /// Type of content.
    pub doc_type: DocType,
    /// When this file was last generated.
    pub generated_at: DateTime<Utc>,
    /// Human-readable stale reason (null if fresh).
    pub stale_reason: Option<String>,
}

impl DocMeta {
    pub fn is_stale(&self) -> bool {
        self.stale_reason.is_some()
    }
}

/// Document registry — stored in `KRON/.kron-context/.registry.json`.
#[derive(Debug, Default, Serialize, Deserialize)]
pub struct Registry {
    /// Map from relative path to metadata.
    #[serde(rename = "docs", default)]
    pub docs: Vec<DocMeta>,
    /// Last full-regenerate timestamp.
    #[serde(rename = "last_regenerate", default)]
    pub last_regenerate: Option<DateTime<Utc>>,
}

impl Registry {
    /// Load from the registry file, or return an empty registry.
    pub fn load(registry_path: &Path) -> Result<Self> {
        if !registry_path.exists() {
            return Ok(Registry::default());
        }
        let content = std::fs::read_to_string(registry_path)?;
        serde_json::from_str(&content)
            .map_err(|e| KronError::Parse(format!("invalid .registry.json: {e}")))
    }

    /// Save to the registry file.
    pub fn save(&self, registry_path: &Path) -> Result<()> {
        let content = serde_json::to_string_pretty(self)?;
        write_atomic(registry_path, content)
    }

    /// Get metadata for a specific doc type.
    pub fn get(&self, doc_type: DocType) -> Option<&DocMeta> {
        self.docs.iter().find(|d| d.doc_type == doc_type)
    }

    /// Upsert metadata for a doc type.
    pub fn upsert(&mut self, meta: DocMeta) {
        if let Some(existing) = self.docs.iter_mut().find(|d| d.doc_type == meta.doc_type) {
            *existing = meta;
        } else {
            self.docs.push(meta);
        }
    }
}

// ---------------------------------------------------------------------------
// Context generation driver
// ---------------------------------------------------------------------------

/// One document that can be generated.
pub struct Generator {
    project_root: PathBuf,
    kron_root: PathBuf,
    registry: Registry,
}

impl Generator {
    /// Start a context generation session for the given project root.
    /// `kron_root` is `project_root.join("KRON")`.
    pub fn new(project_root: PathBuf, kron_root: PathBuf) -> Result<Self> {
        let registry_path = kron_root.join(KRON_CONTEXT_DIR).join(".registry.json");
        let registry = Registry::load(&registry_path)?;
        Ok(Self { project_root, kron_root, registry })
    }

    /// Path to `.kron-context/` directory.
    pub fn context_dir(&self) -> PathBuf {
        self.kron_root.join(KRON_CONTEXT_DIR)
    }

    /// Registry (read-only after construction).
    pub fn registry(&self) -> &Registry {
        &self.registry
    }

    /// List all docs with their metadata.
    pub fn list_docs(&self) -> Vec<DocMeta> {
        let ctx_dir = self.context_dir();
        let mut metas: Vec<DocMeta> = self
            .registry
            .docs
            .iter()
            .cloned()
            .map(|mut m| {
                // Re-evaluate staleness by checking actual file state
                let file_path = ctx_dir.join(&m.path);
                if m.stale_reason.is_none() {
                    if !file_path.exists() {
                        m.stale_reason = Some("file missing".to_string());
                    }
                }
                m
            })
            .collect();

        // If registry is empty, synthesise metadata from file existence
        if metas.is_empty() {
            for dt in DocType::all() {
                let path = ctx_dir.join(dt.path());
                let generated_at = read_generated_timestamp(&path).unwrap_or_else(Utc::now);
                metas.push(DocMeta {
                    path: dt.path().to_string(),
                    doc_type: *dt,
                    generated_at,
                    stale_reason: if path.exists() { None } else { Some("file missing".into()) },
                });
            }
        }
        metas
    }

    /// Generate a single document (incremental).
    pub fn generate_one(&mut self, doc_type: DocType) -> Result<DocMeta> {
        match doc_type {
            DocType::GitCommits => self.generate_git_commits(),
            DocType::GitBranch => self.generate_git_branch(),
            DocType::CodeStructure => self.generate_code_structure(),
            DocType::Readme => self.generate_readme(),
        }
    }

    /// Full-regenerate all documents.
    pub fn generate_all(&mut self) -> Result<Vec<DocMeta>> {
        let now = Utc::now();
        self.registry.last_regenerate = Some(now);
        let mut results = Vec::new();
        for dt in DocType::all() {
            let meta = self.generate_one(*dt)?;
            results.push(meta);
        }
        let registry_path = self.context_dir().join(".registry.json");
        self.registry.save(&registry_path)?;
        Ok(results)
    }

    /// Delete the entire context directory.
    pub fn clean(&self) -> Result<()> {
        let ctx_dir = self.context_dir();
        if ctx_dir.exists() {
            std::fs::remove_dir_all(&ctx_dir)
                .map_err(KronError::Io)?;
        }
        Ok(())
    }

    /// Show a file's content, or error if missing.
    pub fn show(&self, rel_path: &str) -> Result<String> {
        let path = self.context_dir().join(rel_path);
        if !path.exists() {
            return Err(KronError::NotFound(self.context_dir().join(rel_path)));
        }
        let meta = read_generated_timestamp(&path);
        let content = std::fs::read_to_string(&path)?;
        // Prepend header comment if missing
        if !content.contains("kron-generated:") {
            let ts = meta.map(|t| t.to_rfc3339()).unwrap_or_else(|| Utc::now().to_rfc3339());
            return Ok(format!(
                "<!-- kron-generated: {ts} -->\n<!-- DO NOT EDIT. Run 'kron context --regenerate' to refresh. -->\n\n{content}"
            ));
        }
        Ok(content)
    }

    // -- private generators --

    fn generate_git_commits(&mut self) -> Result<DocMeta> {
        let ctx_dir = self.context_dir();
        let out_path = ctx_dir.join(CTX_GIT_COMMITS);
        let content = git_commits::generate(&self.project_root, DEFAULT_COMMIT_COUNT)?;
        self.write_doc(&out_path, &content)?;
        let meta = DocMeta {
            path: CTX_GIT_COMMITS.to_string(),
            doc_type: DocType::GitCommits,
            generated_at: Utc::now(),
            stale_reason: None,
        };
        self.registry.upsert(meta.clone());
        Ok(meta)
    }

    fn generate_git_branch(&mut self) -> Result<DocMeta> {
        let ctx_dir = self.context_dir();
        let out_path = ctx_dir.join(CTX_GIT_BRANCH);
        let content = git_branch::generate(&self.project_root);
        self.write_doc(&out_path, &content)?;
        let meta = DocMeta {
            path: CTX_GIT_BRANCH.to_string(),
            doc_type: DocType::GitBranch,
            generated_at: Utc::now(),
            stale_reason: None,
        };
        self.registry.upsert(meta.clone());
        Ok(meta)
    }

    fn generate_code_structure(&mut self) -> Result<DocMeta> {
        let ctx_dir = self.context_dir();
        let out_path = ctx_dir.join(CTX_CODE_STRUCTURE);
        let content = code_structure::generate(&self.project_root, DEFAULT_TREE_DEPTH)?;
        self.write_doc(&out_path, &content)?;
        let meta = DocMeta {
            path: CTX_CODE_STRUCTURE.to_string(),
            doc_type: DocType::CodeStructure,
            generated_at: Utc::now(),
            stale_reason: None,
        };
        self.registry.upsert(meta.clone());
        Ok(meta)
    }

    fn generate_readme(&mut self) -> Result<DocMeta> {
        let ctx_dir = self.context_dir();
        let out_path = ctx_dir.join(CTX_README);
        let content = readme::generate()?;
        self.write_doc(&out_path, &content)?;
        let meta = DocMeta {
            path: CTX_README.to_string(),
            doc_type: DocType::Readme,
            generated_at: Utc::now(),
            stale_reason: None,
        };
        self.registry.upsert(meta.clone());
        Ok(meta)
    }

    fn write_doc(&self, path: &Path, content: &str) -> Result<()> {
        // Ensure parent dir
        if let Some(parent) = path.parent() {
            std::fs::create_dir_all(parent).map_err(KronError::Io)?;
        }
        // Truncate if too large
        let truncated = if content.len() > MAX_FILE_SIZE {
            format!("{}...\n[TRUNCATED: exceeded 1 MB limit]", &content[..MAX_FILE_SIZE])
        } else {
            content.to_string()
        };
        // Atomic write: write to .tmp then rename
        let tmp_path = path.with_extension("tmp");
        std::fs::write(&tmp_path, truncated.as_bytes()).map_err(KronError::Io)?;
        std::fs::rename(&tmp_path, path).map_err(KronError::Io)?;
        Ok(())
    }
}

// ---------------------------------------------------------------------------
// Utilities
// ---------------------------------------------------------------------------

/// Read the `kron-generated:` timestamp from a file's first 3 lines.
pub fn read_generated_timestamp(path: &Path) -> Option<DateTime<Utc>> {
    let content = std::fs::read_to_string(path).ok()?;
    for line in content.lines().take(3) {
        if let Some(rest) = line.strip_prefix("<!-- kron-generated:") {
            if let Some(ts_str) = rest.trim().split('>').next() {
                let ts_str = ts_str.trim().trim_end_matches(" -->").trim();
                if let Ok(dt) = DateTime::parse_from_rfc3339(ts_str) {
                    return Some(dt.with_timezone(&Utc));
                }
            }
        }
    }
    None
}

/// Render the standard file header comment.
pub fn file_header() -> String {
    let ts = Utc::now().to_rfc3339();
    format!(
        "<!-- kron-generated: {ts} -->\n<!-- DO NOT EDIT. Run 'kron context --regenerate' to refresh. -->\n"
    )
}

/// Atomic write: write content to a temp file then rename to the target.
pub fn write_atomic(path: &Path, content: String) -> Result<()> {
    let tmp_path = path.with_extension("tmp");
    std::fs::write(&tmp_path, content.as_bytes()).map_err(KronError::Io)?;
    std::fs::rename(&tmp_path, path).map_err(KronError::Io)?;
    Ok(())
}
