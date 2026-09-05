//! Integration tests for `kron task move / start / done / describe / delete`.

use chrono::Utc;
use kron::core::task::{
    delete_task, find_task, move_task, read_task, vertex_public_dir, write_task, Task,
};
use kron::model::parse_state;
use std::fs;
use tempfile::TempDir;

fn git_project(tmp: &TempDir) -> std::path::PathBuf {
    let root = tmp.path().to_path_buf();
    fs::create_dir(root.join(".git")).unwrap();
    fs::create_dir_all(root.join("KRON").join("VERTEX")).unwrap();
    fs::create_dir_all(root.join("kron-internal")).unwrap();
    let cfg = r#"{
        "name": "test-project",
        "project_path": "PLACEHOLDER",
        "kron_data_path": "PLACEHOLDER",
        "created_at": "2026-09-05T00:00:00Z",
        "kron_version": "0.1.0",
        "settings": {
            "conflict_threshold_minutes": 5,
            "auto_resolve": "prompt",
            "context_refresh_minutes": 5
        }
    }"#;
    fs::write(root.join("kron-internal").join("config.json"), cfg).unwrap();
    root
}

fn make_task(id: &str, title: &str, state: &str) -> Task {
    Task {
        id: id.into(),
        title: title.into(),
        description: format!("desc for {id}"),
        body: String::new(),
        state: state.into(),
        tags: vec!["smoke".into()],
        created_at: Utc::now(),
        updated_at: Utc::now(),
        source_file: None,
    }
}

#[test]
fn parse_state_lowercases_and_rejects_empty() {
    assert_eq!(parse_state("DOING").unwrap(), "doing");
    assert_eq!(parse_state("todo").unwrap(), "todo");
    assert_eq!(parse_state("Custom-Vertex").unwrap(), "custom-vertex");
    assert!(parse_state("").is_err());
}

#[test]
fn move_task_relocates_file_and_updates_indexes() {
    let tmp = TempDir::new().unwrap();
    let project = git_project(&tmp);

    // Seed: T1 in todo/.
    let t = make_task("T1", "Move me", "todo");
    write_task(&project, "todo", &t).unwrap();
    kron::core::task::append_to_vertex_state(&project, "todo", "T1").unwrap();

    // Move to doing.
    let (new_path, new_vertex) = move_task(&project, "T1", "doing").unwrap();
    assert_eq!(new_vertex, "doing");
    assert!(new_path.ends_with("doing\\T1.md") || new_path.ends_with("doing/T1.md"));
    assert!(new_path.exists());
    assert!(!vertex_public_dir(&project, "todo").join("T1.md").exists());

    // File content reflects new state.
    let moved = read_task(&new_path).unwrap();
    assert_eq!(moved.state, "doing");

    // find_task locates it under the new vertex.
    let (found_path, found_vertex) = find_task(&project, "T1").unwrap();
    assert_eq!(found_vertex, "doing");
    assert_eq!(found_path, new_path);

    // State indexes updated.
    let todo_state = kron::core::task::read_vertex_state(&project, "todo").unwrap();
    let doing_state = kron::core::task::read_vertex_state(&project, "doing").unwrap();
    assert!(todo_state.tasks.is_empty(), "todo index should be empty");
    assert_eq!(doing_state.tasks, vec!["T1"]);
}

#[test]
fn move_task_to_same_vertex_is_noop() {
    let tmp = TempDir::new().unwrap();
    let project = git_project(&tmp);
    write_task(&project, "todo", &make_task("T1", "Same", "todo")).unwrap();

    let (path, vertex) = move_task(&project, "T1", "todo").unwrap();
    assert_eq!(vertex, "todo");
    assert!(path.exists());
    let t = read_task(&path).unwrap();
    assert_eq!(t.state, "todo");
}

#[test]
fn move_task_rejects_invalid_target() {
    let tmp = TempDir::new().unwrap();
    let project = git_project(&tmp);
    write_task(&project, "todo", &make_task("T1", "x", "todo")).unwrap();

    // move_task itself takes a pre-validated target.
    assert!(move_task(&project, "T1", "Has Space").is_err());
    assert!(move_task(&project, "T1", "-leading").is_err());

    // The command path applies parse_state first (lowercasing) before
    // reaching move_task; verify the parse step rejects empty and
    // normalizes case.
    assert!(parse_state("").is_err());
    assert_eq!(parse_state("UPPER").unwrap(), "upper");
    // After normalization, "upper" is still a valid slug:
    assert!(kron::core::task::validate_vertex_name("upper").is_ok());
}

#[test]
fn move_task_to_missing_id_returns_error() {
    let tmp = TempDir::new().unwrap();
    let project = git_project(&tmp);
    // No tasks seeded.
    let err = move_task(&project, "T99", "doing").unwrap_err();
    let msg = format!("{err}");
    assert!(msg.contains("not found") || msg.contains("path not found"), "got: {msg}");
}

#[test]
fn delete_task_removes_file_and_index_entry() {
    let tmp = TempDir::new().unwrap();
    let project = git_project(&tmp);
    write_task(&project, "todo", &make_task("T1", "doomed", "todo")).unwrap();
    kron::core::task::append_to_vertex_state(&project, "todo", "T1").unwrap();
    write_task(&project, "todo", &make_task("T2", "keep", "todo")).unwrap();
    kron::core::task::append_to_vertex_state(&project, "todo", "T2").unwrap();

    delete_task(&project, "T1").unwrap();

    // T1 file is gone, T2 untouched.
    assert!(!vertex_public_dir(&project, "todo").join("T1.md").exists());
    assert!(vertex_public_dir(&project, "todo").join("T2.md").exists());
    assert!(find_task(&project, "T1").is_err());

    // Index only has T2.
    let state = kron::core::task::read_vertex_state(&project, "todo").unwrap();
    assert_eq!(state.tasks, vec!["T2"]);
}

#[test]
fn delete_task_missing_id_returns_error() {
    let tmp = TempDir::new().unwrap();
    let project = git_project(&tmp);
    let err = delete_task(&project, "T404").unwrap_err();
    let msg = format!("{err}");
    assert!(msg.contains("not found") || msg.contains("path not found"), "got: {msg}");
}

#[test]
fn full_lifecycle_init_to_done() {
    // Mirrors the demo script in dev-docs/design/07-实施路线图.md § 3.5
    let tmp = TempDir::new().unwrap();
    let project = git_project(&tmp);

    // 1. Add task to todo
    let t = make_task("T1", "M1: 完成双源 CLI", "todo");
    write_task(&project, "todo", &t).unwrap();
    kron::core::task::append_to_vertex_state(&project, "todo", "T1").unwrap();

    // 2. Move to doing (the demo's `kron task move M1 --to doing`)
    let (path_after_move, vertex_after_move) = move_task(&project, "T1", "doing").unwrap();
    assert_eq!(vertex_after_move, "doing");

    // 3. Mark done
    let (path_after_done, vertex_after_done) = move_task(&project, "T1", "done").unwrap();
    assert_eq!(vertex_after_done, "done");

    let final_task = read_task(&path_after_done).unwrap();
    assert_eq!(final_task.id, "T1");
    assert_eq!(final_task.title, "M1: 完成双源 CLI");
    assert_eq!(final_task.state, "done");

    // todo/ and doing/ should be empty.
    assert!(!path_after_move.exists() || vertex_public_dir(&project, "doing").join("T1.md").exists() == false);
    // The done path should exist.
    assert!(path_after_done.exists());

    // Only 'done' index contains T1.
    let done_state = kron::core::task::read_vertex_state(&project, "done").unwrap();
    let todo_state = kron::core::task::read_vertex_state(&project, "todo").unwrap();
    let doing_state = kron::core::task::read_vertex_state(&project, "doing").unwrap();
    assert_eq!(done_state.tasks, vec!["T1"]);
    assert!(todo_state.tasks.is_empty());
    assert!(doing_state.tasks.is_empty());
}

#[test]
fn describe_task_updates_description_only() {
    let tmp = TempDir::new().unwrap();
    let project = git_project(&tmp);
    write_task(&project, "todo", &make_task("T1", "x", "todo")).unwrap();

    let (path, _) = find_task(&project, "T1").unwrap();
    let mut t = read_task(&path).unwrap();
    t.description = "new short".into();
    t.updated_at = Utc::now();
    t.source_file = None;
    kron::core::task::update_task(&path, &t).unwrap();

    let reloaded = read_task(&path).unwrap();
    assert_eq!(reloaded.description, "new short");
    assert_eq!(reloaded.title, "x");
    assert_eq!(reloaded.state, "todo");
    assert_eq!(reloaded.tags, vec!["smoke"]);
}
