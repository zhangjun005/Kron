//! kron — Git-native task tracker for AI-assisted development.
//!
//! Library entry point. CLI binary lives in `src/main.rs`.

pub mod cli;
pub mod commands;
pub mod core;
pub mod error;
pub mod model;
pub mod output;

pub use error::{KronError, Result};
