//! Tauri 2 main entry — minimal skeleton, single command for smoke test.

#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

fn main() {
    kron_gui_lib::run()
}
