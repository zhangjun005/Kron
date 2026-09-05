//! Integration tests for `kron task add` / `list` / `show`.

use kron::core::task::{read_task, vertex_public_dir, vertex_state_file, normalize_description, validate_vertex_name};
use std::fs;
use tempfile::TempDir;

fn git_project(tmp: &TempDir) -> std::path::PathBuf {
    let root = tmp.path().to_path_buf();
    fs::create_dir(root.join(".git")).unwrap();
    fs::create_dir_all(root.join("KRON").join("VERTEX")).unwrap();
    fs::create_dir_all(root.join("kron-internal")).unwrap();
    // Write a minimal config.json so require_project() would accept it.
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

#[test]
fn validate_vertex_name_accepts_valid_slugs() {
    assert!(validate_vertex_name("todo").is_ok());
    assert!(validate_vertex_name("doing").is_ok());
    assert!(validate_vertex_name("task_42").is_ok());
    assert!(validate_vertex_name("v-1").is_ok());
    assert!(validate_vertex_name("a").is_ok());
    assert!(validate_vertex_name("abc123").is_ok());
}

#[test]
fn validate_vertex_name_rejects_invalid_slugs() {
    assert!(validate_vertex_name("").is_err());
    assert!(validate_vertex_name("Todo").is_err());  // uppercase
    assert!(validate_vertex_name("-leading-dash").is_err()); // starts with -
    assert!(validate_vertex_name("has space").is_err());
    assert!(validate_vertex_name("has/slash").is_err());
    assert!(validate_vertex_name("中文").is_err());
}

#[test]
fn normalize_description_truncates_over_200_chars() {
    let long = "x".repeat(250);
    let (s, truncated) = normalize_description(&long);
    assert_eq!(s.len(), 200);
    assert!(truncated);
    let (s, truncated) = normalize_description("short");
    assert_eq!(s, "short");
    assert!(!truncated);
}

#[test]
fn write_and_read_task_roundtrip() {
    use chrono::Utc;
    use kron::core::task::{write_task, Task};

    let tmp = TempDir::new().unwrap();
    let project = git_project(&tmp);

    let task = Task {
        id: "T1".into(),
        title: "Roundtrip test".into(),
        description: "short desc".into(),
        body: "Long body\nspans\nmultiple lines".into(),
        state: "todo".into(),
        tags: vec!["alpha".into(), "beta".into()],
        created_at: Utc::now(),
        updated_at: Utc::now(),
        source_file: None,
    };

    let path = write_task(&project, "todo", &task).unwrap();
    assert!(path.exists());
    assert_eq!(path.file_name().unwrap(), "T1.md");

    let read_back = read_task(&path).unwrap();
    assert_eq!(read_back.id, "T1");
    assert_eq!(read_back.title, "Roundtrip test");
    assert_eq!(read_back.description, "short desc");
    assert_eq!(read_back.body, "Long body\nspans\nmultiple lines");
    assert_eq!(read_back.state, "todo");
    assert_eq!(read_back.tags, vec!["alpha", "beta"]);
}

#[test]
fn next_task_id_increments_within_vertex() {
    use kron::core::task::{next_task_id, write_task, Task};
    use chrono::Utc;

    let tmp = TempDir::new().unwrap();
    let project = git_project(&tmp);

    // Empty vertex => T1
    assert_eq!(next_task_id(&project, "todo").unwrap(), "T1");

    // Add two tasks => T2, T3
    for (id, title) in [("T1", "first"), ("T2", "second")] {
        let t = Task {
            id: id.into(),
            title: title.into(),
            description: String::new(),
            body: String::new(),
            state: "todo".into(),
            tags: vec![],
            created_at: Utc::now(),
            updated_at: Utc::now(),
            source_file: None,
        };
        write_task(&project, "todo", &t).unwrap();
    }

    assert_eq!(next_task_id(&project, "todo").unwrap(), "T3");
    // Other vertex is independent.
    assert_eq!(next_task_id(&project, "doing").unwrap(), "T1");
}

#[test]
fn append_to_vertex_state_writes_id() {
    use kron::core::task::{append_to_vertex_state, read_vertex_state};

    let tmp = TempDir::new().unwrap();
    let project = git_project(&tmp);

    append_to_vertex_state(&project, "todo", "T1").unwrap();
    append_to_vertex_state(&project, "todo", "T2").unwrap();
    // Idempotent.
    append_to_vertex_state(&project, "todo", "T1").unwrap();

    let state = read_vertex_state(&project, "todo").unwrap();
    assert_eq!(state.tasks, vec!["T1", "T2"]);

    // File lives at kron-internal/states/todo.json
    let expected = vertex_state_file(&project, "todo");
    assert!(expected.exists());
}

#[test]
fn list_task_files_returns_md_in_sorted_order() {
    use kron::core::task::{list_task_files, write_task, Task};
    use chrono::Utc;

    let tmp = TempDir::new().unwrap();
    let project = git_project(&tmp);

    for id in ["T3", "T1", "T2"] {
        let t = Task {
            id: id.into(),
            title: format!("task {id}"),
            description: String::new(),
            body: String::new(),
            state: "todo".into(),
            tags: vec![],
            created_at: Utc::now(),
            updated_at: Utc::now(),
            source_file: None,
        };
        write_task(&project, "todo", &t).unwrap();
    }

    let files = list_task_files(&vertex_public_dir(&project, "todo")).unwrap();
    let names: Vec<String> = files.iter()
        .filter_map(|p| p.file_name()?.to_str().map(String::from))
        .collect();
    assert_eq!(names, vec!["T1.md", "T2.md", "T3.md"]);
}
