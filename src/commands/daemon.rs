//! `kron daemon` — daemon process control (P2 implementation).

use clap::{Args, Subcommand};
use serde::Serialize;

use crate::commands::Ctx;
use crate::core::sync::daemon as core_daemon;
use crate::error::{KronError, Result};

#[derive(Debug, Args)]
pub struct DaemonArgs {
    #[command(subcommand)]
    pub action: DaemonAction,
}

#[derive(Debug, Subcommand)]
pub enum DaemonAction {
    /// Start the daemon (background by default).
    Start {
        #[arg(long)]
        foreground: bool,
        #[arg(long, default_value = "info")]
        log_level: String,
    },
    /// Stop the daemon.
    Stop {
        #[arg(long, default_value_t = 10)]
        timeout: u64,
        #[arg(long)]
        force: bool,
    },
    /// Show daemon status.
    Status,
    /// Restart (stop + start).
    Restart {
        #[arg(long, default_value_t = 10)]
        timeout: u64,
    },
}

#[derive(Serialize)]
struct DaemonStartSummary {
    pid: u32,
    started_at: String,
    scan: core_daemon::ScanSummary,
    note: &'static str,
}

#[derive(Serialize)]
struct DaemonStatusSummary {
    running: bool,
    pid: Option<u32>,
    started_at: Option<String>,
    last_scan_at: Option<String>,
    last_scan: Option<core_daemon::ScanSummary>,
}

fn require_project() -> Result<std::path::PathBuf> {
    let cwd = std::env::current_dir()?;
    if !cwd.join("kron-internal").join("config.json").exists() {
        return Err(KronError::NotAProject(cwd));
    }
    Ok(cwd)
}

pub fn run(ctx: Ctx, args: DaemonArgs) -> Result<()> {
    match args.action {
        DaemonAction::Start { .. } => start_cmd(ctx),
        DaemonAction::Stop { force, .. } => stop_cmd(ctx, force),
        DaemonAction::Status => status_cmd(ctx),
        DaemonAction::Restart { .. } => restart_cmd(ctx),
    }
}

fn start_cmd(ctx: Ctx) -> Result<()> {
    let project = require_project()?;
    let outcome = core_daemon::start(&project)?;

    match ctx.mode {
        crate::output::OutputMode::Json => {
            let summary = DaemonStartSummary {
                pid: outcome.pid,
                started_at: outcome.started_at.to_rfc3339(),
                scan: outcome.scan.clone(),
                note: outcome.note,
            };
            println!("{}", serde_json::to_string_pretty(&summary)?);
        }
        crate::output::OutputMode::Porcelain => {
            println!(
                "started\t{}\t{}\tscanned={}\tsynced={}\tconflicts_new={}",
                outcome.pid,
                outcome.started_at.to_rfc3339(),
                outcome.scan.scanned,
                outcome.scan.synced,
                outcome.scan.conflicts_new
            );
        }
        crate::output::OutputMode::Human => {
            println!("\u{2713} Daemon started, PID {}", outcome.pid);
            println!("  Started at:  {}", outcome.started_at.to_rfc3339());
            println!();
            println!("  Scan summary:");
            println!("    scanned:           {}", outcome.scan.scanned);
            println!("    synced:            {}", outcome.scan.synced);
            println!("    conflicts new:     {}", outcome.scan.conflicts_new);
            println!("    conflicts existing:{}", outcome.scan.conflicts_existing);
            println!("    internal_only:     {}", outcome.scan.internal_only);
            println!("    project_only:      {}", outcome.scan.project_only);
            println!();
            println!("  \u{2139}  {}", outcome.note);
        }
    }
    Ok(())
}

fn stop_cmd(ctx: Ctx, force: bool) -> Result<()> {
    let project = require_project()?;
    if !force && !core_daemon::is_running(&project) {
        if ctx.mode == crate::output::OutputMode::Human {
            println!("= Daemon was not running (no-op).");
        }
        return Ok(());
    }
    let removed = core_daemon::stop(&project)?;

    match ctx.mode {
        crate::output::OutputMode::Json => {
            println!("{}", serde_json::json!({ "stopped": removed }));
        }
        crate::output::OutputMode::Porcelain => {
            println!("stopped\t{}", if removed { "yes" } else { "no" });
        }
        crate::output::OutputMode::Human => {
            if removed {
                println!("\u{2713} Daemon stopped.");
            } else {
                println!("= Daemon was not running.");
            }
        }
    }
    Ok(())
}

fn status_cmd(ctx: Ctx) -> Result<()> {
    let project = require_project()?;
    let st = core_daemon::status(&project)?;

    let running = st.is_some();
    let summary = DaemonStatusSummary {
        running,
        pid: st.as_ref().map(|s| s.pid),
        started_at: st.as_ref().map(|s| s.started_at.to_rfc3339()),
        last_scan_at: st.as_ref().and_then(|s| s.last_scan_at.map(|t| t.to_rfc3339())),
        last_scan: st.as_ref().and_then(|s| s.last_scan.clone()),
    };

    match ctx.mode {
        crate::output::OutputMode::Json => {
            println!("{}", serde_json::to_string_pretty(&summary)?);
        }
        crate::output::OutputMode::Porcelain => {
            println!(
                "running={}\tpid={}\tstarted_at={}\tlast_scan_at={}",
                summary.running,
                summary.pid.map(|p| p.to_string()).unwrap_or_else(|| "-".into()),
                summary.started_at.as_deref().unwrap_or("-"),
                summary.last_scan_at.as_deref().unwrap_or("-")
            );
        }
        crate::output::OutputMode::Human => {
            if running {
                let s = st.as_ref().unwrap();
                println!("Daemon: running");
                println!("  PID:          {}", s.pid);
                println!("  Started at:   {}", s.started_at.to_rfc3339());
                if let Some(at) = s.last_scan_at {
                    println!("  Last scan:    {}", at.to_rfc3339());
                }
                if let Some(sc) = &s.last_scan {
                    println!("  Scan summary: scanned={} synced={} conflicts_new={} conflicts_existing={}",
                        sc.scanned, sc.synced, sc.conflicts_new, sc.conflicts_existing);
                }
            } else {
                println!("Daemon: stopped");
            }
        }
    }
    Ok(())
}

fn restart_cmd(ctx: Ctx) -> Result<()> {
    let project = require_project()?;
    // Stop (ignore "not running"), then start.
    let _ = core_daemon::stop(&project);
    start_cmd(ctx)
}
