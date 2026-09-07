//! Domain-level operations (init, scanning, sync, etc.).
//!
//! Pure logic; CLI/GUI layers in `commands/` call into here.

pub mod context;
pub mod init;
pub mod project;
pub mod state_pointer;
pub mod sync;
pub mod task;
pub mod vertex;
