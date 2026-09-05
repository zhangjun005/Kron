//! `kron status` — view project status.

use clap::Args;
use serde::Serialize;

use crate::commands::Ctx;
use crate::error::Result;

#[derive(Debug, Args)]
pub struct StatusArgs {
    /// Auto-refresh every N seconds (Ctrl-C to stop). Default: no refresh.
    #[arg(long)]
    pub watch: Option<u64>,
}

#[derive(Serialize)]
struct StatusReport {
    project: String,
    initialized: bool,
    vertices: u32,
    tasks: u32,
    important_files: u32,
    pending_conflicts: u32,
    daemon: DaemonInfo,
}

#[derive(Serialize)]
struct DaemonInfo {
    running: bool,
    pid: Option<u32>,
}

/// Count `.md` files directly under a directory (one level).
fn count_md(dir: &std::path::Path) -> u32 {
    std::fs::read_dir(dir)
        .map(|rd| {
            rd.filter_map(|e| e.ok())
                .filter(|e| {
                    e.path().is_file()
                        && e.path().extension().map(|x| x == "md").unwrap_or(false)
                })
                .count() as u32
        })
        .unwrap_or(0)
}

pub fn run(ctx: Ctx, args: StatusArgs) -> Result<()> {
    let cwd = std::env::current_dir()?;
    let kron_dir = cwd.join("kron-internal");
    let kron_public = cwd.join("KRON");

    let initialized = kron_dir.join("config.json").exists();

    let vertices: u32 = if kron_public.join("VERTEX").is_dir() {
        std::fs::read_dir(kron_public.join("VERTEX"))
            .map(|rd| rd.filter_map(|e| e.ok()).filter(|e| e.path().is_dir()).count() as u32)
            .unwrap_or(0)
    } else {
        0
    };

    let tasks: u32 = if kron_public.join("VERTEX").is_dir() {
        std::fs::read_dir(kron_public.join("VERTEX"))
            .map(|rd| {
                rd.filter_map(|e| e.ok())
                    .filter(|e| e.path().is_dir())
                    .map(|e| count_md(&e.path()))
                    .sum()
            })
            .unwrap_or(0)
    } else {
        0
    };

    let important_files: u32 = if kron_public.join("important").is_dir() {
        count_md(&kron_public.join("important"))
    } else {
        0
    };

    let report = StatusReport {
        project: cwd.display().to_string(),
        initialized,
        vertices,
        tasks,
        important_files,
        pending_conflicts: 0, // P2 stub
        daemon: DaemonInfo { running: false, pid: None },
    };

    match ctx.mode {
        crate::output::OutputMode::Json => {
            println!("{}", serde_json::to_string_pretty(&report)?);
        }
        crate::output::OutputMode::Porcelain => {
            println!("{}\t{}\t{}\t{}\t{}\t{}\trunning={}\tpid={}",
                report.project,
                report.initialized,
                report.vertices,
                report.tasks,
                report.important_files,
                report.pending_conflicts,
                report.daemon.running,
                report.daemon.pid.map(|p| p.to_string()).unwrap_or_else(|| "-".into()),
            );
        }
        crate::output::OutputMode::Human => {
            println!("Project:       {}", report.project);
            println!("Initialized:   {}", report.initialized);
            println!("Vertices:      {}", report.vertices);
            println!("Tasks:         {}", report.tasks);
            println!("Important:     {}", report.important_files);
            println!("Conflicts:     {} pending", report.pending_conflicts);
            println!("Daemon:        {}",
                if report.daemon.running {
                    format!("running (pid {})", report.daemon.pid.unwrap_or(0))
                } else {
                    "stopped".into()
                });
            if args.watch.is_some() {
                println!("\u{2139}  --watch is a Phase 2 stub; not refreshing.");
            }
        }
    }
    Ok(())
}
