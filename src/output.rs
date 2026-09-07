//! Output formatting helpers.
//!
//! Provides unified functions for emitting JSON vs. human-readable output
//! so every command can support `--json` / `--porcelain` consistently.
//!
//! ## Layering
//!
//! * **Low-level** — `emit_json`, `emit_porcelain`, `emit_human` write to
//!   stdout and do no mode-branching. Callers that already know the mode
//!   (e.g. a `match mode { ... }` arm) can use these directly.
//! * **Mode-branching** — `emit_records`, `emit_record` accept a mode and
//!   dispatch to the right low-level helper. They are the **typical
//!   command-layer entry point** when the command has the same payload
//!   shape across all three modes.
//! * **Annotations** — `success`, `info` (human-only) and `warning`
//!   (all-modes-to-stderr) keep the layer consistent: success/info are
//!   silenced in JSON/Porcelain, warnings always surface to stderr.
//!
//! See `dev-docs/dev-journal/2026-09-07-day-3-output.md` for the
//! rationale and migration plan.

use crate::error::Result;
use serde::Serialize;
use std::fmt::Display;

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

// ---- Low-level: no mode branching, write to stdout ----

/// Emit a value as pretty JSON to stdout. Failures surface as `Err`.
pub fn emit_json<T: Serialize + ?Sized>(value: &T) -> Result<()> {
    println!("{}", serde_json::to_string_pretty(value)?);
    Ok(())
}

/// Emit a porcelain-formatted line. The caller composes the line
/// (typically by joining fields with `\t`); this helper just writes it.
pub fn emit_porcelain(line: impl Display) {
    println!("{line}");
}

/// Emit a human-formatted line verbatim.
pub fn emit_human(line: impl AsRef<str>) {
    println!("{}", line.as_ref());
}

// ---- Mid-level: mode-branching helpers ----

/// A typed record for porcelain/JSON output. Implements `Serialize` for
/// JSON mode and `Display` for the porcelain line.
pub trait PorcelainRecord: Serialize {
    /// Tab-separated porcelain fields, in the canonical order.
    /// Implementors compose this with `tab_join`.
    fn porcelain_fields(&self) -> Vec<String>;
}

/// Convenience: join porcelain fields with a tab.
pub fn tab_join(fields: &[String]) -> String {
    fields.join("\t")
}

/// Emit an array of records across all three modes.
///
/// * **Json**: a single pretty JSON array.
/// * **Porcelain**: one tab-separated line per record; if the array is
///   empty, emit a `# (empty)` comment line so scripts can detect it.
/// * **Human**: emit the human message (caller-supplied); when `records`
///   is empty and no human message is supplied, print a default hint.
///
/// # Why a separate human message?
///
/// Records carry structured data; the human-friendly rendering is a
/// caller choice (column widths, ordering, decorative borders). Keeping
/// it as a pre-formatted string avoids forcing every record to expose
/// a `to_human()` method.
pub fn emit_records<T: PorcelainRecord>(
    mode: OutputMode,
    records: &[T],
    human_body: &str,
    empty_human_hint: &str,
) -> Result<()> {
    match mode {
        OutputMode::Json => {
            // Always emit an array — never `null` for empty input,
            // since callers expect `[ ... ]` consistently.
            emit_json(records)?;
        }
        OutputMode::Porcelain => {
            if records.is_empty() {
                println!("# (empty)");
            } else {
                for r in records {
                    println!("{}", tab_join(&r.porcelain_fields()));
                }
            }
        }
        OutputMode::Human => {
            if records.is_empty() {
                println!("{empty_human_hint}");
            } else {
                println!("{human_body}");
            }
        }
    }
    Ok(())
}

/// Emit a single record / summary across all three modes.
///
/// * **Json**: pretty-printed record.
/// * **Porcelain**: one tab-separated line (caller-supplied; use the
///   record's `porcelain_fields()` for consistency).
/// * **Human**: the pre-formatted human message.
pub fn emit_record<T: PorcelainRecord>(
    mode: OutputMode,
    record: &T,
    human_body: &str,
) -> Result<()> {
    match mode {
        OutputMode::Json => emit_json(record),
        OutputMode::Porcelain => {
            println!("{}", tab_join(&record.porcelain_fields()));
            Ok(())
        }
        OutputMode::Human => {
            println!("{human_body}");
            Ok(())
        }
    }
}

// ---- Annotations ----

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

/// Print a warning to **stderr** regardless of mode.
///
/// Unlike `success`/`info`, warnings are intentionally visible in
/// `--json` and `--porcelain` modes (they go to stderr, not stdout,
/// so JSON-on-stdout stays parseable). This is the right hook for
/// "the operation succeeded, but you should know about this" cases
/// such as orphaned-pointer-after-vertex-delete.
pub fn warning(_mode: OutputMode, msg: &str) {
    eprintln!("warning: {msg}");
}

/// Print a warning to stderr in human mode only. Suppressed in JSON/Porcelain.
pub fn info_warning(mode: OutputMode, msg: &str) {
    if mode == OutputMode::Human {
        eprintln!("warning: {msg}");
    }
}

// ---- Backwards-compatible legacy helper ----

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

// ============================================================================
// Inline tests — Day 3 anchor #9 / output-layer helpers
// ============================================================================

#[cfg(test)]
mod tests {
    use super::*;

    #[derive(Serialize, Clone)]
    struct Row {
        id: String,
        state: String,
        title: String,
    }

    impl PorcelainRecord for Row {
        fn porcelain_fields(&self) -> Vec<String> {
            vec![self.id.clone(), self.state.clone(), self.title.clone()]
        }
    }

    fn cap() -> std::io::Cursor<Vec<u8>> {
        std::io::Cursor::new(Vec::new())
    }

    #[test]
    fn emit_json_writes_pretty_to_stdout() {
        // sanity check via the trait bound — we can't easily capture stdout here,
        // but we can confirm the serializer accepts our struct.
        let r = Row {
            id: "T1".into(),
            state: "todo".into(),
            title: "demo".into(),
        };
        let s = serde_json::to_string_pretty(&r).unwrap();
        assert!(s.contains("\"id\": \"T1\"") || s.contains("\"id\":\"T1\""));
        assert!(s.contains("\"state\": \"todo\"") || s.contains("\"state\":\"todo\""));
        assert!(s.contains("\"title\": \"demo\"") || s.contains("\"title\":\"demo\""));
    }

    #[test]
    fn tab_join_joins_with_tab() {
        let parts = vec!["a".into(), "b".into(), "c".into()];
        assert_eq!(tab_join(&parts), "a\tb\tc");
    }

    #[test]
    fn porcelain_fields_order_is_caller_defined() {
        let r = Row {
            id: "T2".into(),
            state: "doing".into(),
            title: "x".into(),
        };
        assert_eq!(
            r.porcelain_fields(),
            vec!["T2".to_string(), "doing".to_string(), "x".to_string()]
        );
    }

    #[test]
    fn output_mode_from_flags_precedence() {
        // porcelain wins over json
        assert_eq!(OutputMode::from_flags(true, true), OutputMode::Porcelain);
        assert_eq!(OutputMode::from_flags(true, false), OutputMode::Json);
        assert_eq!(OutputMode::from_flags(false, true), OutputMode::Porcelain);
        assert_eq!(OutputMode::from_flags(false, false), OutputMode::Human);
    }
}
