//! kron — Git-native task tracker for AI-assisted development.
//!
//! v0.2 — Phase 2 CLI skeleton. See dev-docs/design/04b-CLI设计.md.
//!
//! The binary entry point: parses argv and runs the CLI.

use kron::{cli, error::KronError};

fn main() {
    if let Err(e) = cli::run() {
        eprintln!("error: {e}");
        std::process::exit(exit_code_for(&e));
    }
}

/// Map error variants to process exit codes.
///
/// Conventions follow dev-docs/design/03-双源同步机制.md § 5.8.
fn exit_code_for(err: &KronError) -> i32 {
    match err {
        KronError::AlreadyInitialized(_) => 4,
        KronError::Cli(_) => 2,
        _ => 1,
    }
}
