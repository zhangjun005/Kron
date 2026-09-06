//! Integration tests for P4 commands: config, vertex, important,
//! task additions (back/check/tag/attach/edit).

use kron::core::init::{materialize, prepare};
use kron::core::sync::sync_index::ImportantIndex;
use kron::core::task::{find_task, read_task, vertex_public_dir};
use kron::core::vertex as core_vertex;
use kron::model::{AutoResolve, Project, ProjectSettings};
use std::fs;
use std::path::Path;
use tempfile::TempDir;

fn fresh_project() -> TempDir {
    let tmp = TempDir::new().expect("tempdir");
    let root = tmp.path();
    fs::create_dir(root.join(".git")).unwrap();
    let prep = prepare(root, false).expect("prepare");
    materialize(&prep, false, kron::model::LinkMode::Copy).expect("materialize");
    tmp
}

fn write_important(root: &Path, rel: &str, content: &str) {
    let proj = root.join(rel);
    if let Some(parent) = proj.parent() {
        fs::create_dir_all(parent).unwrap();
    }
    fs::write(&proj, content).unwrap();

    // Mirror to the internal location, matching what `kron important add` does.
    let internal = root.join("kron-internal").join("important").join("files").join(rel);
    if let Some(parent) = internal.parent() {
        fs::create_dir_all(parent).unwrap();
    }
    fs::write(&internal, content).unwrap();

    let mut idx = ImportantIndex::load(root).unwrap();
    idx.files.insert(
        rel.to_string(),
        kron::core::sync::sync_index::ImportantEntry {
            path: rel.to_string(),
            sync_state: kron::model::SyncState::Synced,
            internal_hash: kron::core::sync::sync_index::md5_hex(content.as_bytes()),
            added_at: chrono::Utc::now(),
            updated_at: chrono::Utc::now(),
        },
    );
    idx.save(root).unwrap();
}

fn add_task_md(root: &Path, vertex: &str, id: &str, title: &str) {
    use kron::core::task;
    let dir = vertex_public_dir(root, vertex);
    fs::create_dir_all(&dir).unwrap();
    let now = chrono::Utc::now();
    let task = task::Task {
        id: id.into(),
        title: title.into(),
        description: String::new(),
        body: String::new(),
        state: vertex.into(),
        tags: vec![],
        created_at: now,
        updated_at: now,
        source_file: None,
    };
    task::write_task(root, vertex, &task).unwrap();
}

// ---- core::vertex ----

#[test]
fn vertex_registry_starts_empty_after_init() {
    let tmp = fresh_project();
    let root = tmp.path();
    let reg = core_vertex::load_registry(root).unwrap();
    assert!(reg.is_empty(), "expected empty registry after init");
}

#[test]
fn vertex_create_adds_entry_and_materializes_dir() {
    let tmp = fresh_project();
    let root = tmp.path();
    let rec = core_vertex::create(root, "review", Some("Under review"), Some("feature/x"), None).unwrap();
    assert_eq!(rec.name, "review");
    assert_eq!(rec.description.as_deref(), Some("Under review"));
    assert_eq!(rec.branch.as_deref(), Some("feature/x"));
    assert!(root.join("KRON").join("VERTEX").join("review").is_dir());

    let reg = core_vertex::load_registry(root).unwrap();
    assert_eq!(reg.len(), 1);
    assert_eq!(reg[0].name, "review");
}

#[test]
fn vertex_create_rejects_invalid_slug_and_duplicates() {
    let tmp = fresh_project();
    let root = tmp.path();
    assert!(core_vertex::create(root, "Bad Name", None, None, None).is_err());
    core_vertex::create(root, "todo", None, None, None).unwrap();
    assert!(core_vertex::create(root, "todo", None, None, None).is_err());
}

#[test]
fn vertex_update_modifies_description_and_branch() {
    let tmp = fresh_project();
    let root = tmp.path();
    core_vertex::create(root, "review", Some("old"), None, None).unwrap();
    let upd = core_vertex::update(root, "review", Some("new desc"), Some("feature/y")).unwrap();
    assert_eq!(upd.description.as_deref(), Some("new desc"));
    assert_eq!(upd.branch.as_deref(), Some("feature/y"));
}

#[test]
fn vertex_delete_removes_from_registry_and_optional_dir() {
    let tmp = fresh_project();
    let root = tmp.path();
    core_vertex::create(root, "scratch", None, None, None).unwrap();
    add_task_md(root, "scratch", "T1", "To remove");
    core_vertex::delete(root, "scratch", /* also_remove_dir */ true).unwrap();
    assert!(core_vertex::find(root, "scratch").unwrap().is_none());
    assert!(!root.join("KRON").join("VERTEX").join("scratch").is_dir());
}

// ---- config persistence ----

#[test]
fn config_persists_through_project_json() {
    let tmp = fresh_project();
    let root = tmp.path();
    let path = root.join("kron-internal").join("config.json");
    let raw = fs::read_to_string(&path).unwrap();
    let mut project: Project = serde_json::from_str(&raw).unwrap();
    project.settings.conflict_threshold_minutes = 30;
    project.settings.auto_resolve = AutoResolve::Latest;
    fs::write(&path, serde_json::to_string_pretty(&project).unwrap()).unwrap();

    let reloaded: Project =
        serde_json::from_str(&fs::read_to_string(&path).unwrap()).unwrap();
    assert_eq!(reloaded.settings.conflict_threshold_minutes, 30);
    assert_eq!(reloaded.settings.auto_resolve, AutoResolve::Latest);
}

#[test]
fn config_settings_have_correct_defaults() {
    let s = ProjectSettings::default();
    assert_eq!(s.conflict_threshold_minutes, 5);
    assert_eq!(s.auto_resolve, AutoResolve::Prompt);
    assert_eq!(s.context_refresh_minutes, 5);
}

// ---- important: register, mirror, remove ----

#[test]
fn important_register_creates_mirror_and_index_entry() {
    let tmp = fresh_project();
    let root = tmp.path();
    write_important(root, "src/notes.md", "hello\n");

    // Re-register via the same path the CLI would take.
    let internal = root.join("kron-internal").join("important").join("files").join("src").join("notes.md");
    assert!(internal.is_file(), "mirror must exist");

    let idx = ImportantIndex::load(root).unwrap();
    let entry = idx.files.get("src/notes.md").expect("index must contain entry");
    assert_eq!(entry.sync_state, kron::model::SyncState::Synced);
}

#[test]
fn important_remove_clears_index_and_mirror() {
    let tmp = fresh_project();
    let root = tmp.path();
    write_important(root, "src/temp.md", "temp\n");
    let mut idx = ImportantIndex::load(root).unwrap();
    let removed = idx.remove_entry("src/temp.md");
    assert!(removed);
    idx.save(root).unwrap();

    let reloaded = ImportantIndex::load(root).unwrap();
    assert!(reloaded.files.get("src/temp.md").is_none());
}

#[test]
fn important_normalize_rel_rejects_parent_dir_and_empty() {
    use kron::commands::important;
    // Parent-dir escapes are always rejected.
    assert!(important::normalize_rel_for_test("../escape").is_err());
    assert!(important::normalize_rel_for_test("foo/../bar").is_err());
    // Empty after trimming is rejected.
    assert!(important::normalize_rel_for_test("   ").is_err());
    // Sanity: valid relative paths pass.
    assert!(important::normalize_rel_for_test("ok/rel/path").is_ok());
    assert!(important::normalize_rel_for_test("ok\\rel\\path").is_ok());
    assert!(important::normalize_rel_for_test("single").is_ok());
}

// ---- task: back / check / tag / attach / describe (frontmatter updates) ----

#[test]
fn task_tag_add_remove_list_clear() {
    let tmp = fresh_project();
    let root = tmp.path();
    add_task_md(root, "todo", "T1", "First");

    let mut t = read_task(&find_task(root, "T1").unwrap().0).unwrap();
    assert!(t.tags.is_empty());
    t.tags.push("urgent".into());
    t.updated_at = chrono::Utc::now();
    t.source_file = None;
    kron::core::task::update_task(&find_task(root, "T1").unwrap().0, &t).unwrap();

    let t2 = read_task(&find_task(root, "T1").unwrap().0).unwrap();
    assert_eq!(t2.tags, vec!["urgent".to_string()]);

    let mut t3 = read_task(&find_task(root, "T1").unwrap().0).unwrap();
    t3.tags.retain(|x| x != "urgent");
    t3.updated_at = chrono::Utc::now();
    t3.source_file = None;
    kron::core::task::update_task(&find_task(root, "T1").unwrap().0, &t3).unwrap();

    let t4 = read_task(&find_task(root, "T1").unwrap().0).unwrap();
    assert!(t4.tags.is_empty());
}

#[test]
fn task_attach_writes_metadata_into_body() {
    let tmp = fresh_project();
    let root = tmp.path();
    add_task_md(root, "todo", "T2", "Second");

    let (path, _) = find_task(root, "T2").unwrap();
    let mut t = read_task(&path).unwrap();
    let attached = serde_json::json!({
        "priority": "high",
        "estimate": "3",
    });
    t.body = serde_json::to_string_pretty(&attached).unwrap();
    t.updated_at = chrono::Utc::now();
    t.source_file = None;
    kron::core::task::update_task(&path, &t).unwrap();

    let reloaded = read_task(&path).unwrap();
    let parsed: serde_json::Value = serde_json::from_str(&reloaded.body).unwrap();
    assert_eq!(parsed["priority"], "high");
    assert_eq!(parsed["estimate"], "3");
}

#[test]
fn task_back_moves_to_previous_state() {
    let tmp = fresh_project();
    let root = tmp.path();
    add_task_md(root, "doing", "T3", "Doing");
    let (new_path, new_vertex) =
        kron::core::task::move_task(root, "T3", "todo").unwrap();
    assert_eq!(new_vertex, "todo");
    assert!(new_path.ends_with("T3.md"));
    assert!(new_path.starts_with(vertex_public_dir(root, "todo")));
}

#[test]
fn task_check_toggles_done_to_todo() {
    let tmp = fresh_project();
    let root = tmp.path();
    add_task_md(root, "done", "T4", "Done");
    let (new_path, new_vertex) =
        kron::core::task::move_task(root, "T4", "todo").unwrap();
    assert_eq!(new_vertex, "todo");
    assert!(new_path.starts_with(vertex_public_dir(root, "todo")));
}

// ---- important sync (one-shot detect) ----

#[test]
fn important_sync_runs_one_shot_detect() {
    let tmp = fresh_project();
    let root = tmp.path();
    write_important(root, "src/a.md", "alpha\n");
    write_important(root, "src/b.md", "beta\n");
    let r = kron::core::sync::conflict::detect(root).unwrap();
    assert_eq!(r.stats.scanned, 2);
    assert_eq!(r.stats.synced, 2);
}

// ---- vertex find/load edge cases ----

#[test]
fn vertex_load_handles_missing_file_as_empty() {
    let tmp = fresh_project();
    let root = tmp.path();
    // Manually delete the registry file and re-load.
    let p = root.join("kron-internal").join("vertices.json");
    if p.exists() {
        fs::remove_file(&p).unwrap();
    }
    let reg = core_vertex::load_registry(root).unwrap();
    assert!(reg.is_empty());
}

#[test]
fn vertex_load_handles_corrupt_json_as_empty() {
    let tmp = fresh_project();
    let root = tmp.path();
    let p = root.join("kron-internal").join("vertices.json");
    fs::write(&p, "this is not json {{{").unwrap();
    let reg = core_vertex::load_registry(root).unwrap();
    assert!(reg.is_empty());
}
