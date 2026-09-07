//! `code/structure.md` generator.
//!
//! Recursively walks the project directory tree and formats it as a tree view,
//! without external dependencies.

use std::fs;
use std::path::Path;

use crate::error::Result;

/// Directories / files to always skip.
const SKIP_NAMES: &[&str] = &[
    "node_modules",
    "target",
    ".git",
    ".svn",
    ".hg",
    "*.pyc",
    "__pycache__",
    ".DS_Store",
    "Thumbs.db",
];

/// Max entries to avoid runaway recursion.
const MAX_ENTRIES: usize = 10_000;

/// One node in the directory tree.
#[derive(Debug)]
pub struct Entry {
    pub rel_path: String,
    pub is_dir: bool,
    pub depth: u32,
}

/// Generate the `code/structure.md` content.
pub fn generate(project_root: &Path, depth: u32) -> Result<String> {
    let now = chrono::Utc::now();
    let header = format!(
        "<!-- kron-generated: {} -->\n<!-- DO NOT EDIT. Run 'kron context --regenerate' to refresh. -->\n",
        now.to_rfc3339()
    );

    let project_name = project_root
        .file_name()
        .and_then(|n| n.to_str())
        .unwrap_or("project");

    let entries = collect_entries(project_root, depth)?;

    let total = entries.len();
    let tree = render_tree(&entries, project_name);

    let mut lines = vec![
        header,
        "# Project Structure".to_string(),
        "".to_string(),
        format!("深度：{}（可配置）", depth),
        "".to_string(),
        "```".to_string(),
    ];
    lines.push(tree);
    lines.push("```".to_string());
    lines.push("".to_string());
    lines.push("(跳过: node_modules/, target/, .git/, 等)".to_string());
    lines.push(format!("(共 {} 个条目)", total));
    lines.push("".to_string());

    Ok(lines.join("\n"))
}

/// Collect all entries up to `max_depth`.
fn collect_entries(root: &Path, max_depth: u32) -> Result<Vec<Entry>> {
    let mut entries = Vec::with_capacity(MAX_ENTRIES);
    let mut count: usize = 0;
    collect_recursive(root, "", 0, max_depth, &mut entries, &mut count)?;
    Ok(entries)
}

fn collect_recursive(
    dir: &Path,
    rel_prefix: &str,
    depth: u32,
    max_depth: u32,
    out: &mut Vec<Entry>,
    count: &mut usize,
) -> Result<()> {
    if *count >= MAX_ENTRIES || depth > max_depth {
        return Ok(());
    }

    let read_dir = match fs::read_dir(dir) {
        Ok(d) => d,
        Err(e) if e.kind() == std::io::ErrorKind::PermissionDenied => return Ok(()),
        Err(e) => return Err(e.into()),
    };

    let mut children: Vec<_> = read_dir
        .filter_map(|e| e.ok())
        .filter_map(|e| {
            let name = e.file_name();
            let name_str = name.to_str()?;
            Some((name_str.to_string(), e.path(), e.path().is_dir()))
        })
        .collect();

    // Sort: directories first, then files, alphabetical within each group
    children.sort_by(|a, b| {
        if a.2 != b.2 {
            b.2.cmp(&a.2) // dirs before files
        } else {
            a.0.to_lowercase().cmp(&b.0.to_lowercase())
        }
    });

    for (name, path, is_dir) in children {
        if should_skip(&name) {
            continue;
        }

        *count += 1;
        if *count > MAX_ENTRIES {
            break;
        }

        let rel_path = if rel_prefix.is_empty() {
            name.clone()
        } else {
            format!("{}/{}", rel_prefix, name)
        };

        out.push(Entry {
            rel_path: rel_path.clone(),
            is_dir,
            depth,
        });

        if is_dir {
            collect_recursive(&path, &rel_path, depth + 1, max_depth, out, count)?;
        }
    }

    Ok(())
}

fn should_skip(name: &str) -> bool {
    for skip in SKIP_NAMES {
        if let Some(suffix) = skip.strip_prefix('*') {
            if name.ends_with(suffix) {
                return true;
            }
        } else if *skip == name {
            return true;
        }
    }
    false
}

impl Entry {
    fn name_only(&self) -> &str {
        self.rel_path
            .rsplit_once('/')
            .map(|(_, n)| n)
            .unwrap_or(&self.rel_path)
    }
}

/// Render entries as a tree string.
fn render_tree(entries: &[Entry], root_name: &str) -> String {
    if entries.is_empty() {
        return format!("{}/\n  (empty)", root_name);
    }

    let n = entries.len();
    let mut lines = vec![format!("{}/", root_name)];

    for (i, entry) in entries.iter().enumerate() {
        let is_last = is_last_at_depth(entries, i, entry.depth);
        let prefix = tree_prefix(entries, i);
        let name = entry.name_only();
        let line = if entry.is_dir {
            format!("{}{}/", prefix, name)
        } else {
            format!("{}{}", prefix, name)
        };
        lines.push(line);
    }

    lines.join("\n")
}

/// Returns true if entry at index `i` is the last entry at its depth level.
fn is_last_at_depth(entries: &[Entry], i: usize, depth: u32) -> bool {
    !entries[i + 1..].iter().any(|e| e.depth == depth)
}

fn tree_prefix(entries: &[Entry], i: usize) -> String {
    let depth = entries[i].depth as usize;
    if depth == 0 {
        return String::new();
    }

    let is_last = !entries[i + 1..].iter().any(|e| e.depth == entries[i].depth);

    let mut result = String::new();
    for d in 1..depth {
        let has_more_at_d = entries[i + 1..]
            .iter()
            .any(|e| e.depth >= d as u32);
        result.push_str(if has_more_at_d { "│   " } else { "    " });
    }

    if is_last {
        result.push_str("└── ");
    } else {
        result.push_str("├── ");
    }

    result
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_collect_entries_includes_src() {
        let entries = collect_entries(Path::new("."), 2).unwrap();
        assert!(!entries.is_empty());
    }

    #[test]
    fn test_generates_valid_content() {
        let content = generate(Path::new("."), 2).unwrap();
        assert!(content.contains("# Project Structure"));
        assert!(content.contains("src/"));
    }

    #[test]
    fn test_render_tree_empty() {
        let result = render_tree(&[], "foo");
        assert!(result.contains("foo/"));
        assert!(result.contains("(empty)"));
    }
}
