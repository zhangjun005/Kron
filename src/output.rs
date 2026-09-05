//! Output formatting helpers.
//!
//! Provides unified functions for emitting JSON vs. human-readable output
//! so every command can support `--json` / `--porcelain` consistently.

use crate::error::Result;
use serde::Serialize;

/// Output mode derived from CLI flags.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum OutputMode {
    /// Pretty, colored, human-friendly text (the default).
    Human,
    /// Machine-readable, one record per line, no decoration.
    Porcelain,
    /// Structured JSON.
    Json,
}

impl OutputMode {
    /// Resolve the mode from CLI flags. `porcelain` wins over `json`.
    pub fn from_flags(json: bool, porcelain: bool) -> Self {
        if porcelain {
            OutputMode::Porcelain
        } else if json {
            OutputMode::Json
        } else {
            OutputMode::Human
        }
    }
}

/// Render a serializable value according to the chosen mode.
///
/// In `Human` mode the message is printed verbatim (no JSON wrapping);
/// callers should pre-format their human output.
///
/// In `Porcelain` mode, `value` is printed as one record per line
/// (tab-separated fields expected — caller decides structure).
///
/// In `Json` mode, `value` is serialized as a pretty JSON object to stdout.
pub fn emit<T: Serialize>(mode: OutputMode, value: &T, fallback_human: &str) -> Result<()> {
    match mode {
        OutputMode::Json => {
            let s = serde_json::to_string_pretty(value)?;
            println!("{s}");
        }
        OutputMode::Porcelain => {
            // For generic records we just emit the JSON without pretty-printing;
            // callers can override per-command for tighter tab-separated output.
            let s = serde_json::to_string(value)?;
            println!("{s}");
        }
        OutputMode::Human => {
            println!("{fallback_human}");
        }
    }
    Ok(())
}

/// Print a success line (✓ ...) in human mode, no-op in JSON/Porcelain.
pub fn success(mode: OutputMode, msg: &str) {
    if mode == OutputMode::Human {
        println!("\u{2713} {msg}"); // ✓
    }
}

/// Print an informational line; suppressed in JSON/Porcelain modes.
pub fn info(mode: OutputMode, msg: &str) {
    if mode == OutputMode::Human {
        println!("{msg}");
    }
}
