//! Integration tests for `kron init`.
//!
//! Each test creates a fresh temp directory so the tests can be run
//! in parallel and don't interfere with each other.

use kron::core::init::{materialize, prepare};
use kron::model::LinkMode;
use std::fs;
use tempfile::TempDir;

#[test]
fn init_in_git_repo_creates_full_layout() {
    let tmp = TempDir::new().expect("tempdir");
    let root = tmp.path();

    // fake a .git folder so we pass the is_git check without invoking git
    fs::create_dir(root.join(".git")).unwrap();

    let prep = prepare(root, /* no_git = */ false).expect("prepare");
    assert!(prep.is_git, "should detect .git");

    let outcome = materialize(&prep, /* no_vertex = */ false, LinkMode::Symlink)
        .expect("materialize");

    // -- kron-internal/ --
    assert!(root.join("kron-internal").is_dir());
    assert!(root.join("kron-internal").join("config.json").is_file());
    assert!(root.join("kron-internal").join("vertices.json").is_file());
    assert!(root.join("kron-internal").join("important").is_dir());

    // -- KRON/ (project-side mirror) --
    assert!(root.join("KRON").is_dir());
    assert!(root.join("KRON").join("README.md").is_file());
    assert!(root.join("KRON").join("VERTEX").is_dir());
    assert!(root.join("KRON").join("important").is_dir());

    // -- outcome fields --
    assert_eq!(outcome.link_mode, LinkMode::Copy); // Windows falls back
    assert!(!outcome.no_vertex);
    assert!(outcome.created.iter().any(|c| c.contains("kron-internal/")));
    assert!(outcome.created.iter().any(|c| c.contains("config.json")));
}

#[test]
fn init_without_git_is_rejected_unless_no_git() {
    let tmp = TempDir::new().unwrap();
    let root = tmp.path();

    // no .git folder
    let err = prepare(root, /* no_git = */ false).unwrap_err();
    let msg = format!("{err}");
    assert!(msg.contains("not a Git repository"), "got: {msg}");

    // with --no-git it should succeed
    let prep = prepare(root, /* no_git = */ true).expect("prepare ok with --no-git");
    assert!(!prep.is_git);
    let _ = materialize(&prep, false, LinkMode::Copy).unwrap();
    assert!(root.join("kron-internal").join("config.json").is_file());
}

#[test]
fn init_with_no_vertex_skips_vertex_folder() {
    let tmp = TempDir::new().unwrap();
    let root = tmp.path();
    fs::create_dir(root.join(".git")).unwrap();

    let prep = prepare(root, false).unwrap();
    let outcome = materialize(&prep, /* no_vertex = */ true, LinkMode::Copy).unwrap();

    assert!(!root.join("KRON").join("VERTEX").exists());
    assert!(outcome.skipped.iter().any(|s| s.contains("VERTEX")));
}

#[test]
fn init_is_idempotent_when_kron_internal_already_exists() {
    let tmp = TempDir::new().unwrap();
    let root = tmp.path();
    fs::create_dir(root.join(".git")).unwrap();

    let prep = prepare(root, false).unwrap();
    materialize(&prep, false, LinkMode::Copy).unwrap();

    // Second call should NOT clobber files but still report them as skipped.
    let prep2 = prepare(root, false).unwrap();
    assert!(prep2.kron_dir_exists);

    let outcome2 = materialize(&prep2, false, LinkMode::Copy).unwrap();
    assert!(
        outcome2.created.iter().all(|c| !c.contains("config.json")),
        "config.json should not be re-created, got: {:?}",
        outcome2.created
    );
    assert!(outcome2.skipped.iter().any(|s| s.contains("config.json")));
}

#[test]
fn config_json_roundtrips_through_serde() {
    let tmp = TempDir::new().unwrap();
    let root = tmp.path();
    fs::create_dir(root.join(".git")).unwrap();

    let prep = prepare(root, false).unwrap();
    materialize(&prep, false, LinkMode::Copy).unwrap();

    let raw = fs::read_to_string(root.join("kron-internal").join("config.json")).unwrap();
    let parsed: serde_json::Value = serde_json::from_str(&raw).unwrap();
    // `name` is derived from the directory name (random tempdir name here).
    assert!(parsed["name"].is_string());
    assert!(!parsed["name"].as_str().unwrap().is_empty());
    assert!(parsed["project_path"].is_string());
    assert!(parsed["kron_data_path"].is_string());
    assert!(parsed["created_at"].is_string());
    assert_eq!(parsed["kron_version"], env!("CARGO_PKG_VERSION"));
    assert_eq!(parsed["settings"]["conflict_threshold_minutes"], 5);
}
