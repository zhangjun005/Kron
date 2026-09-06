//! Convenience wrapper around `conflict::detect()`.
//!
//! Kept as a separate module so the public surface is one name
//! (`core::sync::scan::run`) and tests can pin the wrapper behaviour
//! independently of the underlying detection logic.

use crate::core::sync::conflict::{self, ScanResult};
use crate::error::Result;
use std::path::Path;

/// Run one sync scan over the project's important files.
pub fn run(project_root: &Path) -> Result<ScanResult> {
    conflict::detect(project_root)
}
