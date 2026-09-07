//! kron — Git-native task tracker for AI-assisted development.
//!
//! v0.2 — Phase 2 CLI skeleton. See dev-docs/design/04b-CLI设计.md.
//!
//! The binary entry point: parses argv and runs the CLI.

use kron::cli;

fn main() {
    if let Err(e) = cli::run() {
        eprintln!("error: {e}");
        // Day 3 (Anchor #8): all error → exit code mapping now lives
        // in `KronError::exit_code()`. Two sources of truth (the old
        // local `exit_code_for` shadowing the canonical mapping in
        // `error.rs`) were a correctness bug — the local version
        // returned `1` for everything not in `{Cli, AlreadyInitialized}`
        // while the canonical mapping distinguishes PermissionDenied (9),
        // Io (6), NotFound (8), etc. Single source of truth.
        std::process::exit(e.exit_code());
    }
}
