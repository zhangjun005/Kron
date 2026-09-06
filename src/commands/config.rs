//! `kron config` — read/write project configuration.
//!
//! Config is persisted to `kron-internal/config.json`. Keys may be:
//! - Top-level scalars: `name`, `kron_version`
//! - Nested settings: `settings.conflict_threshold_minutes`,
//!   `settings.auto_resolve`, `settings.context_refresh_minutes`
//!
//! All operations are synchronous; no daemon lock needed.

use clap::{Args, Subcommand};
use serde::Serialize;
use std::path::Path;

use crate::commands::Ctx;
use crate::error::{KronError, Result};
use crate::model::Project;

#[derive(Debug, Args)]
pub struct ConfigArgs {
    #[command(subcommand)]
    pub action: ConfigAction,
}

#[derive(Debug, Subcommand)]
pub enum ConfigAction {
    /// Get a single config key.
    Get { key: String },
    /// Set a config key.
    Set { key: String, value: String },
    /// List all config keys.
    List,
}

#[derive(Serialize)]
struct KeyValue {
    key: String,
    value: String,
}

#[derive(Serialize)]
struct ConfigListing {
    project: String,
    settings: ConfigSettingsView,
}

#[derive(Serialize)]
struct ConfigSettingsView {
    conflict_threshold_minutes: u32,
    auto_resolve: String,
    context_refresh_minutes: u32,
}

fn project_path() -> Result<std::path::PathBuf> {
    let cwd = std::env::current_dir()?;
    if !cwd.join("kron-internal").join("config.json").exists() {
        return Err(KronError::NotAProject(cwd));
    }
    Ok(cwd)
}

fn load_project(root: &Path) -> Result<Project> {
    let path = root.join("kron-internal").join("config.json");
    if !path.exists() {
        return Err(KronError::NotFound(path));
    }
    let raw = std::fs::read_to_string(&path)?;
    Ok(serde_json::from_str(&raw)?)
}

fn save_project(root: &Path, project: &Project) -> Result<()> {
    let path = root.join("kron-internal").join("config.json");
    let json = serde_json::to_string_pretty(project)?;
    std::fs::write(&path, json)?;
    Ok(())
}

/// Look up a single config value, formatting it as a string.
fn get_value(project: &Project, key: &str) -> Result<String> {
    match key {
        "name" => Ok(project.name.clone()),
        "kron_version" => Ok(project.kron_version.clone()),
        "project_path" => Ok(project.project_path.display().to_string()),
        "kron_data_path" => Ok(project.kron_data_path.display().to_string()),
        "created_at" => Ok(project.created_at.to_rfc3339()),
        "settings.conflict_threshold_minutes" => {
            Ok(project.settings.conflict_threshold_minutes.to_string())
        }
        "settings.auto_resolve" => Ok(format!("{:?}", project.settings.auto_resolve).to_lowercase()),
        "settings.context_refresh_minutes" => {
            Ok(project.settings.context_refresh_minutes.to_string())
        }
        other => Err(KronError::Cli(format!(
            "unknown config key: {other:?} (try `kron config list`)"
        ))),
    }
}

/// Set a config value, parsing the string to the right type.
fn set_value(project: &mut Project, key: &str, raw: &str) -> Result<String> {
    match key {
        "name" => {
            if raw.is_empty() {
                return Err(KronError::Cli("name must not be empty".into()));
            }
            project.name = raw.to_string();
            Ok(project.name.clone())
        }
        "settings.conflict_threshold_minutes" => {
            let n: u32 = raw.parse().map_err(|_| {
                KronError::Cli(format!("expected integer for {key}, got {raw:?}"))
            })?;
            if n == 0 {
                return Err(KronError::Cli(
                    "conflict_threshold_minutes must be >= 1".into(),
                ));
            }
            project.settings.conflict_threshold_minutes = n;
            Ok(project.settings.conflict_threshold_minutes.to_string())
        }
        "settings.context_refresh_minutes" => {
            let n: u32 = raw.parse().map_err(|_| {
                KronError::Cli(format!("expected integer for {key}, got {raw:?}"))
            })?;
            if n == 0 {
                return Err(KronError::Cli(
                    "context_refresh_minutes must be >= 1".into(),
                ));
            }
            project.settings.context_refresh_minutes = n;
            Ok(project.settings.context_refresh_minutes.to_string())
        }
        "settings.auto_resolve" => {
            let normalized = raw.to_ascii_lowercase();
            use crate::model::AutoResolve;
            let value = match normalized.as_str() {
                "prompt" => AutoResolve::Prompt,
                "latest" => AutoResolve::Latest,
                "manual" => AutoResolve::Manual,
                other => {
                    return Err(KronError::Cli(format!(
                        "auto_resolve must be one of: prompt | latest | manual (got {other:?})"
                    )));
                }
            };
            project.settings.auto_resolve = value;
            Ok(format!("{:?}", value).to_lowercase())
        }
        "kron_version" => {
            return Err(KronError::Cli(
                "kron_version is read-only (set by `kron init`)".into(),
            ));
        }
        "project_path" | "kron_data_path" | "created_at" => {
            return Err(KronError::Cli(format!("{key} is read-only")));
        }
        other => Err(KronError::Cli(format!(
            "unknown config key: {other:?} (try `kron config list`)"
        ))),
    }
}

pub fn run(ctx: Ctx, args: ConfigArgs) -> Result<()> {
    match args.action {
        ConfigAction::Get { key } => get_cmd(ctx, &key),
        ConfigAction::Set { key, value } => set_cmd(ctx, &key, &value),
        ConfigAction::List => list_cmd(ctx),
    }
}

fn get_cmd(ctx: Ctx, key: &str) -> Result<()> {
    let root = project_path()?;
    let project = load_project(&root)?;
    let value = get_value(&project, key)?;

    match ctx.mode {
        crate::output::OutputMode::Json => {
            let kv = KeyValue { key: key.into(), value };
            println!("{}", serde_json::to_string_pretty(&kv)?);
        }
        crate::output::OutputMode::Porcelain => {
            println!("{}\t{}", key, value);
        }
        crate::output::OutputMode::Human => {
            println!("{}: {}", key, value);
        }
    }
    Ok(())
}

fn set_cmd(ctx: Ctx, key: &str, raw: &str) -> Result<()> {
    let root = project_path()?;
    let mut project = load_project(&root)?;
    let new_value = set_value(&mut project, key, raw)?;
    save_project(&root, &project)?;

    match ctx.mode {
        crate::output::OutputMode::Json => {
            let kv = KeyValue { key: key.into(), value: new_value };
            println!("{}", serde_json::to_string_pretty(&kv)?);
        }
        crate::output::OutputMode::Porcelain => {
            println!("{}\t{}", key, new_value);
        }
        crate::output::OutputMode::Human => {
            println!("\u{2713} {} = {}", key, new_value);
        }
    }
    Ok(())
}

fn list_cmd(ctx: Ctx) -> Result<()> {
    let root = project_path()?;
    let project = load_project(&root)?;
    let view = ConfigListing {
        project: project.name.clone(),
        settings: ConfigSettingsView {
            conflict_threshold_minutes: project.settings.conflict_threshold_minutes,
            auto_resolve: format!("{:?}", project.settings.auto_resolve).to_lowercase(),
            context_refresh_minutes: project.settings.context_refresh_minutes,
        },
    };

    match ctx.mode {
        crate::output::OutputMode::Json => {
            println!("{}", serde_json::to_string_pretty(&view)?);
        }
        crate::output::OutputMode::Porcelain => {
            println!("name\t{}", project.name);
            println!("kron_version\t{}", project.kron_version);
            println!("project_path\t{}", project.project_path.display());
            println!("kron_data_path\t{}", project.kron_data_path.display());
            println!("created_at\t{}", project.created_at.to_rfc3339());
            println!("settings.conflict_threshold_minutes\t{}", project.settings.conflict_threshold_minutes);
            println!("settings.auto_resolve\t{}", format!("{:?}", project.settings.auto_resolve).to_lowercase());
            println!("settings.context_refresh_minutes\t{}", project.settings.context_refresh_minutes);
        }
        crate::output::OutputMode::Human => {
            println!("Project");
            println!("  name:           {}", project.name);
            println!("  kron_version:   {}", project.kron_version);
            println!("  project_path:   {}", project.project_path.display());
            println!("  kron_data_path: {}", project.kron_data_path.display());
            println!("  created_at:     {}", project.created_at.to_rfc3339());
            println!();
            println!("Settings");
            println!("  settings.conflict_threshold_minutes: {}", project.settings.conflict_threshold_minutes);
            println!("  settings.auto_resolve:               {}", format!("{:?}", project.settings.auto_resolve).to_lowercase());
            println!("  settings.context_refresh_minutes:    {}", project.settings.context_refresh_minutes);
        }
    }
    Ok(())
}
