//! Domain-level operations (init, scanning, sync, etc.).
//!
//! Pure logic; CLI/GUI layers in `commands/` call into here.

pub mod init;
pub mod sync;
pub mod task;
pub mod vertex;
