//! `git/recent-commits.md` generator.
//!
//! Runs `git log` and formats the output as a Markdown table.
//! If the directory is not a git repo, returns a placeholder (no error).

use std::path::Path;
use std::process::Command;

use crate::error::Result;

/// Generate the `git/recent-commits.md` content.
pub fn generate(project_root: &Path, count: usize) -> Result<String> {
    let output = match Command::new("git")
        .args([
            "log",
            "--format=%H|%aI|%an|%s",
            &format!("-{}", count),
            "--no-show-signature",
        ])
        .current_dir(project_root)
        .output()
    {
        Ok(o) if o.status.success() => o,
        _ => {
            // Not a git repository — generate a placeholder
            return Ok(placeholder_content(
                "# Recent Commits",
                "*(非 Git 仓库，无 commit 信息)*",
            ));
        }
    };

    let stdout = String::from_utf8_lossy(&output.stdout);
    let now = chrono::Utc::now();
    let header = format!(
        "<!-- kron-generated: {} -->\n<!-- DO NOT EDIT. Run 'kron context --regenerate' to refresh. -->\n",
        now.to_rfc3339()
    );

    let mut lines = vec![
        header,
        "# Recent Commits".to_string(),
        "".to_string(),
        format!("最近 {count} 次 commit，按时间倒序。"),
        "".to_string(),
        "| SHA | Date | Author | Subject |".to_string(),
        "|-----|------|--------|---------|".to_string(),
    ];

    for line in stdout.lines() {
        let line = line.trim();
        if line.is_empty() {
            continue;
        }
        let parts: Vec<&str> = line.splitn(5, '|').collect();
        if parts.len() < 4 {
            continue;
        }
        let sha = &parts[0][..8.min(parts[0].len())];
        let date = parts[1].split('T').next().unwrap_or(parts[1]);
        let author = parts[2];
        let subject = parts[3..].join("|");

        lines.push(format!(
            "| `{}` | {} | {} | {} |",
            sha, date, author, subject
        ));
    }

    if lines.len() <= 6 {
        lines.push("*(空仓库，尚无 commit)*".to_string());
    }

    lines.push("".to_string());
    Ok(lines.join("\n"))
}

/// Build a placeholder document when git is unavailable.
fn placeholder_content(title: &str, body: &str) -> String {
    let now = chrono::Utc::now();
    let header = format!(
        "<!-- kron-generated: {} -->\n<!-- DO NOT EDIT. Run 'kron context --regenerate' to refresh. -->\n",
        now.to_rfc3339()
    );
    format!("{}# {}\n\n{}\n", header, title, body)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_generates_valid_markdown_table() {
        let result = generate(Path::new("."), 5);
        if let Ok(content) = result {
            assert!(content.contains("# Recent Commits"));
            assert!(content.contains("| SHA |"));
        }
    }

    #[test]
    fn test_placeholder_for_non_git_dir() {
        // /tmp should not be a git repo (usually)
        let result = generate(Path::new("/tmp"), 5);
        // Should succeed, not error
        assert!(result.is_ok());
        let content = result.unwrap();
        assert!(content.contains("kron-generated:"));
    }
}
