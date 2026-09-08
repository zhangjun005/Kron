//! Kron GUI library entry.
//!
//! Phase 1 skeleton — only exposes `kron_project_list` to verify the
//! Rust → Tauri → Solid IPC pipeline is wired correctly.
//!
//! See `dev-docs/design/05-GUI设计.md` § 3 for the full IPC contract.

pub mod commands;
pub mod ipc_types;

use tracing_subscriber::EnvFilter;
/// Tauri application entry point. Called from `src-tauri/src/main.rs`.
pub fn run() {
    // ---- tracing init ----
    let _ = tracing_subscriber::fmt()
        .with_env_filter(
            EnvFilter::try_from_default_env().unwrap_or_else(|_| EnvFilter::new("info")),
        )
        .with_writer(std::io::stderr)
        .try_init();

    tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_opener::init())
        .setup(|app| {
            // Spawn the file-watcher tokio task (stub for now; see
            // `watcher.rs` for the full implementation).
            commands::spawn_watcher_stub(app.handle().clone());
            Ok(())
        })
        .invoke_handler(tauri::generate_handler![
            commands::project::kron_project_list,
            commands::project::kron_greet,
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
