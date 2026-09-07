//! `git/branch-summary.md` generator.
//!
//! If the directory is not a git repo, returns a placeholder (no error).

use std::path::Path;
use std::process::Command;

/// Main branch name heuristics.
const MAIN_CANDIDATES: &[&str] = &["main", "master", "develop", "trunk"];

/// Generate the `git/branch-summary.md` content.
pub fn generate(project_root: &Path) -> String {
    let current_branch = current_branch(project_root).unwrap_or_else(|_| "(unknown)".into());
    let main_branch = find_main_branch(project_root).unwrap_or_else(|_| "main".into());
    let (ahead, behind) = ahead_behind(project_root, &current_branch, &main_branch);

    let now = chrono::Utc::now();
    let header = format!(
        "<!-- kron-generated: {} -->\n<!-- DO NOT EDIT. Run 'kron context --regenerate' to refresh. -->\n",
        now.to_rfc3339()
    );

    let mut lines = vec![
        header,
        "# Branch Summary".to_string(),
        "".to_string(),
        format!("**Current branch**: `{}`", current_branch),
        format!("**Main branch**: `{}`", main_branch),
        format!("**Ahead**: {} commit(s)", ahead.len()),
        format!("**Behind**: {} commit(s)", behind.len()),
        "".to_string(),
    ];

    if ahead.is_empty() {
        lines.push("## Ahead (0)".to_string());
        lines.push("(none)".to_string());
    } else {
        lines.push(format!("## Ahead ({})", ahead.len()));
        for item in &ahead {
            lines.push(format!("- {}", item));
        }
    }
    lines.push("".to_string());

    if behind.is_empty() {
        lines.push("## Behind (0)".to_string());
        lines.push("(none)".to_string());
    } else {
        lines.push(format!("## Behind ({})", behind.len()));
        for item in &behind {
            lines.push(format!("- {}", item));
        }
    }

    lines.push("".to_string());
    lines.join("\n")
}

fn current_branch(project_root: &Path) -> Result<String, ()> {
    let output = Command::new("git")
        .args(["branch", "--show-current"])
        .current_dir(project_root)
        .output()
        .map_err(|_| ())?;

    if !output.status.success() {
        return Err(());
    }

    let branch = String::from_utf8_lossy(&output.stdout).trim().to_string();
    if branch.is_empty() {
        let output = Command::new("git")
            .args(["rev-parse", "--short", "HEAD"])
            .current_dir(project_root)
            .output()
            .map_err(|_| ())?;
        let sha = String::from_utf8_lossy(&output.stdout).trim().to_string();
        Ok(format!("(detached HEAD at {})", sha))
    } else {
        Ok(branch)
    }
}

fn find_main_branch(project_root: &Path) -> Result<String, ()> {
    for candidate in MAIN_CANDIDATES {
        let output = Command::new("git")
            .args(["rev-parse", "--verify", &format!("origin/{candidate}")])
            .current_dir(project_root)
            .output()
            .map_err(|_| ())?;

        if output.status.success() && !output.stdout.is_empty() {
            return Ok(candidate.to_string());
        }
    }

    let output = Command::new("git")
        .args(["branch", "--format=%(refname:short)"])
        .current_dir(project_root)
        .output()
        .map_err(|_| ())?;

    let first = String::from_utf8_lossy(&output.stdout)
        .lines()
        .next()
        .unwrap_or("main")
        .trim()
        .to_string();

    Ok(if first.is_empty() { "main".to_string() } else { first })
}

fn ahead_behind(project_root: &Path, current: &str, main: &str) -> (Vec<String>, Vec<String>) {
    let behind = git_log_summarize(project_root, &format!("{}..{}", current, main));
    let ahead = git_log_summarize(project_root, &format!("{}..{}", main, current));
    (ahead, behind)
}

fn git_log_summarize(project_root: &Path, range: &str) -> Vec<String> {
    let output = match Command::new("git")
        .args(["log", "--format=%h %s", "--max-count=20", range, "--no-show-signature"])
        .current_dir(project_root)
        .output()
    {
        Ok(o) if o.status.success() => o,
        _ => return Vec::new(),
    };

    String::from_utf8_lossy(&output.stdout)
        .lines()
        .map(|l| l.trim().to_string())
        .filter(|l| !l.is_empty())
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_generates_valid_content() {
        let content = generate(Path::new("."));
        assert!(content.contains("# Branch Summary"));
        assert!(content.contains("**Current branch**:"));
    }

    #[test]
    fn test_placeholder_for_non_git() {
        let content = generate(Path::new("/tmp"));
        assert!(content.contains("kron-generated:"));
        assert!(content.contains("# Branch Summary"));
    }
}
