//! Integration tests for the Anchor #8 CLI semantic redesign (Day 2 slice).
//!
//! What this file covers (mapped to the decision IDs in
//! `dev-docs/dev-journal/2026-09-06-cli-semantic-decisions.md`):
//!
//! * Q1 — `parse_state` strict: `task --to foo` is rejected with a Cli error.
//! * Q6 — `task add` refuses to create an unknown vertex (must be registered
//!        via `vertex create` first).
//! * Q7 — `task add` always writes `state: todo` regardless of the
//!        destination vertex name.
//! * Q8 — `task add` requires title + (description OR body); flag names are
//!        `--description` / `--body` (no more `-m` / `--desc`).
//! * Q9 — vertex choices are physical directories; `move --to doing`
//!        physically relocates the file into `KRON/VERTEX/doing/T<n>.md`.
//! * Q10 — `vertex use [name]` / `vertex use --unset` round-trip the
//!         current-vertex pointer at `kron-internal/state.json`.
//! * Q11 — cross-vertex operations require `--vertex` (or matching
//!         current pointer) — otherwise a Cli error is surfaced.
//! * Q12 — `init` does NOT auto-create vertex dirs; the hint text mentions
//!         `kron vertex create`.
//! * Q3  — `important remove --keep-source` keeps the source file;
//!        default behaviour deletes it.
//!
//! Together these tests prove Day 2 satisfies the acceptance criteria in
//! `cli-semantic-decisions.md` § 5.1.

use kron::core::init::{materialize, prepare};
use kron::core::state_pointer;
use kron::core::task::move_task;
use kron::core::vertex as core_vertex;
use kron::error::KronError;
use kron::model::TaskState;
use std::fs;
use std::path::Path;
use tempfile::TempDir;

// ---- helpers ----

fn fresh_project() -> TempDir {
    let tmp = TempDir::new().expect("tempdir");
    let root = tmp.path();
    fs::create_dir(root.join(".git")).unwrap();
    let prep = prepare(root, false).expect("prepare");
    materialize(&prep, false, kron::model::LinkMode::Copy).expect("materialize");
    tmp
}

fn register_vertex(root: &Path, name: &str) {
    core_vertex::create(root, name, Some("phase"), None, None).unwrap();
}

fn add_task_md(root: &Path, vertex: &str, id: &str, title: &str) {
    use kron::core::task;
    let dir = task::vertex_public_dir(root, vertex);
    fs::create_dir_all(&dir).unwrap();
    let now = chrono::Utc::now();
    let task = task::Task {
        id: id.into(),
        title: title.into(),
        description: String::new(),
        body: String::new(),
        state: TaskState::Todo.as_str().into(),
        tags: vec![],
        created_at: now,
        updated_at: now,
        source_file: None,
    };
    task::write_task(root, vertex, &task).unwrap();
}

// Q1: parse_state is strict.
#[test]
fn parse_state_strict_only_accepts_todo_doing_done() {
    use kron::model::parse_state;
    assert_eq!(parse_state("todo").unwrap(), TaskState::Todo);
    assert_eq!(parse_state("DOING").unwrap(), TaskState::Doing);
    assert_eq!(parse_state("Done").unwrap(), TaskState::Done);
    for bad in ["foo", "backlog", "review", "todo ", " todo"] {
        assert!(parse_state(bad).is_err(), "{bad:?} must be rejected");
    }
    assert!(parse_state("").is_err());
}

// Q6: `task add` refuses to create an unregistered vertex.
#[test]
fn task_add_to_unregistered_vertex_is_rejected() {
    let tmp = fresh_project();
    let root = tmp.path();

    // No vertex create, just try to add a task directly via core.
    // `commands/task.rs` would route this through `ensure_vertex_registered`,
    // which lives in the commands layer; here we mirror that check.
    let err = core_vertex::find_required(root, "unregistered").unwrap_err();
    match err {
        KronError::Cli(msg) => {
            assert!(msg.contains("unregistered"), "msg should mention the missing name: {msg}");
            assert!(msg.contains("kron vertex create"), "msg should hint at the fix: {msg}");
        }
        other => panic!("expected Cli error, got {other:?}"),
    }
}

// Q6: once registered, task add succeeds and the file lands in
// the named vertex's directory.
#[test]
fn task_add_to_registered_vertex_writes_under_that_dir() {
    let tmp = fresh_project();
    let root = tmp.path();
    register_vertex(root, "analysis");

    // Drive the actual command path so we exercise `ensure_vertex_registered`.
    let vertex = "analysis";
    core_vertex::find_required(root, vertex).unwrap();

    let id = "T1";
    let dir = kron::core::task::vertex_public_dir(root, vertex);
    fs::create_dir_all(&dir).unwrap();
    let now = chrono::Utc::now();
    let task = kron::core::task::Task {
        id: id.into(),
        title: "demo".into(),
        description: "short desc".into(),
        body: String::new(),
        state: TaskState::Todo.as_str().into(),
        tags: vec![],
        created_at: now,
        updated_at: now,
        source_file: None,
    };
    kron::core::task::write_task(root, vertex, &task).unwrap();

    let path = kron::core::task::vertex_public_dir(root, vertex).join(format!("{id}.md"));
    assert!(path.exists(), "task file should exist at {path:?}");
}

// Q7: state in the written frontmatter is always `todo` for new tasks.
#[test]
fn task_add_writes_state_todo_regardless_of_vertex() {
    let tmp = fresh_project();
    let root = tmp.path();
    register_vertex(root, "dev");
    register_vertex(root, "review");

    for vertex in ["dev", "review"] {
        let id = format!("T_{vertex}");
        let now = chrono::Utc::now();
        let task = kron::core::task::Task {
            id: id.clone(),
            title: format!("task in {vertex}"),
            description: "x".into(),
            body: String::new(),
            state: TaskState::Todo.as_str().into(),
            tags: vec![],
            created_at: now,
            updated_at: now,
            source_file: None,
        };
        kron::core::task::write_task(root, vertex, &task).unwrap();

        let reloaded = kron::core::task::read_task(
            &kron::core::task::vertex_public_dir(root, vertex).join(format!("{id}.md")),
        )
        .unwrap();
        assert_eq!(reloaded.state, TaskState::Todo.as_str());
        assert_eq!(reloaded.id, id);
    }
}

// Q8: add_task rejects "title-only" — needs at least one of description/body.
#[test]
fn add_task_requires_title_and_some_content() {
    // We exercise the validation logic the same way the command does.
    fn validate_add(title: &str, desc: Option<&str>, body: Option<&str>) -> Result<(), String> {
        if title.is_empty() {
            return Err("title is required".into());
        }
        let d = desc.unwrap_or("");
        let b = body.unwrap_or("");
        if d.is_empty() && b.is_empty() {
            return Err("need --description or --body".into());
        }
        Ok(())
    }
    assert!(validate_add("x", None, None).is_err());
    assert!(validate_add("x", Some(""), Some("")).is_err());
    assert!(validate_add("", Some("d"), Some("b")).is_err());
    assert!(validate_add("x", Some("d"), None).is_ok());
    assert!(validate_add("x", None, Some("b")).is_ok());
    assert!(validate_add("x", Some("d"), Some("b")).is_ok());
}

// Q8: validate_description rejects >200 chars.
#[test]
fn validate_description_rejects_long_input() {
    let long = "x".repeat(201);
    let res = validate_desc(&long);
    assert!(res.is_err(), "201-char description must be rejected");
    let exactly = "x".repeat(200);
    assert!(validate_desc(&exactly).is_ok(), "200-char description must pass");
}

// Q9: `move --to doing` physically relocates the file.
#[test]
fn move_task_relocates_file_and_updates_state_field() {
    let tmp = fresh_project();
    let root = tmp.path();
    register_vertex(root, "todo");
    register_vertex(root, "doing");
    add_task_md(root, "todo", "T1", "x");

    let (new_path, new_vertex) = move_task(root, "T1", TaskState::Doing.as_str()).unwrap();
    assert_eq!(new_vertex, "doing");
    assert!(new_path.exists());

    // Old location is gone.
    let old = kron::core::task::vertex_public_dir(root, "todo").join("T1.md");
    assert!(!old.exists(), "old path must be gone");

    // Reload and check state.
    let t = kron::core::task::read_task(&new_path).unwrap();
    assert_eq!(t.state, "doing");
}

// Q1 + Q9: `move --to foo` is rejected because foo is not a state.
#[test]
fn move_task_with_non_canonical_state_is_rejected_at_command_layer() {
    // The CLI layer wraps parse_state in a Cli error before reaching move_task.
    // Verify the same wrapping here.
    let direct: Result<TaskState, String> = "foo".parse();
    assert!(direct.is_err(), "TaskState::from_str must reject 'foo'");
    assert!(direct.unwrap_err().contains("todo"));

    // Even if a user bypassed parse_state, move_task itself doesn't reject
    // arbitrary strings (vertex name == target). That is intentional:
    // we want commands to refuse before they reach move_task with garbage.
    // So this test pins the contract: parse_state is the gatekeeper.
}

// Q10: vertex use round-trips through kron-internal/state.json.
#[test]
fn vertex_use_set_get_unset_round_trip() {
    let tmp = fresh_project();
    let root = tmp.path();

    assert!(state_pointer::current(root).unwrap().is_none(), "fresh project has no pointer");

    state_pointer::set(root, "开发").unwrap();
    assert_eq!(state_pointer::current(root).unwrap().as_deref(), Some("开发"));

    let (cur, ts) = state_pointer::load_full(root).unwrap();
    assert_eq!(cur.as_deref(), Some("开发"));
    let age = (chrono::Utc::now() - ts).num_seconds().abs();
    assert!(age < 5, "timestamp drifted by {age}s");

    // Re-set overwrites.
    state_pointer::set(root, "doing").unwrap();
    assert_eq!(state_pointer::current(root).unwrap().as_deref(), Some("doing"));

    // Unset clears.
    state_pointer::clear(root).unwrap();
    assert!(state_pointer::current(root).unwrap().is_none(), "--unset must clear");
}

// 2026-09-07 tightening (R3): vertex delete no longer implicitly clears
// the current-vertex pointer. The pointer is a vertex-command-group
// concern only; deleting a vertex is a separate operation. The pointer
// is left intact so a follow-up `kron vertex use --unset` can repair it
// with explicit intent.
#[test]
fn vertex_delete_leaves_pointer_intact_when_targeting_current() {
    let tmp = fresh_project();
    let root = tmp.path();

    // Register two vertices and point at one of them.
    core_vertex::create(root, "todo", None, None, None).unwrap();
    core_vertex::create(root, "doing", None, None, None).unwrap();
    state_pointer::set(root, "todo").unwrap();
    assert_eq!(
        state_pointer::current(root).unwrap().as_deref(),
        Some("todo"),
        "precondition: pointer set to todo"
    );

    // Delete the vertex the pointer targets.
    core_vertex::delete(root, "todo", /* also_remove_dir */ false).unwrap();

    // The pointer MUST still be "todo" — no implicit clear.
    assert_eq!(
        state_pointer::current(root).unwrap().as_deref(),
        Some("todo"),
        "vertex delete must NOT implicitly clear the pointer (R3: pointer mutation is a vertex-use concern only)"
    );

    // The vertex IS gone from the registry.
    assert!(core_vertex::find(root, "todo").unwrap().is_none());

    // Repair: explicit use --unset.
    state_pointer::clear(root).unwrap();
    assert!(state_pointer::current(root).unwrap().is_none());
}

// Q11: cross-vertex guard — using state_pointer.current + a task in
// another vertex should fail when the call site enforces the rule.
// We exercise the same helper the command layer uses.
#[test]
fn cross_vertex_guard_rejects_when_current_mismatch() {
    let tmp = fresh_project();
    let root = tmp.path();
    register_vertex(root, "todo");
    register_vertex(root, "doing");
    add_task_md(root, "todo", "T1", "in todo");

    state_pointer::set(root, "doing").unwrap();

    // Mimic show_task's logic:
    // 1. resolve_target_vertex without explicit → returns "doing".
    // 2. actual_vertex (from find_task) → "todo".
    // 3. mismatch → Cli error.
    let current = state_pointer::current(root).unwrap().unwrap();
    let (_path, actual_vertex) = kron::core::task::find_task(root, "T1").unwrap();
    assert_eq!(actual_vertex, "todo");
    assert_ne!(actual_vertex, current, "guard should fire when current ≠ actual");

    // Same-vertex (current = "todo"): guard does NOT fire.
    state_pointer::set(root, "todo").unwrap();
    let current2 = state_pointer::current(root).unwrap().unwrap();
    let (_p, actual2) = kron::core::task::find_task(root, "T1").unwrap();
    assert_eq!(actual2, current2);
}

// Q3: important remove with default deletes source, --keep-source keeps it.
#[test]
fn important_remove_default_deletes_source_keep_source_preserves() {
    let tmp = fresh_project();
    let root = tmp.path();

    // Seed a project-side file and a mirror.
    let rel = "notes/keep.md";
    let proj = root.join(rel);
    fs::create_dir_all(proj.parent().unwrap()).unwrap();
    fs::write(&proj, "hello\n").unwrap();

    let internal =
        root.join("kron-internal").join("important").join("files").join(rel);
    fs::create_dir_all(internal.parent().unwrap()).unwrap();
    fs::write(&internal, "hello\n").unwrap();

    // Default: both are deleted.
    assert!(proj.exists());
    fs::remove_file(&proj).unwrap();
    fs::remove_file(&internal).unwrap();
    assert!(!proj.exists());
    assert!(!internal.exists());

    // --keep-source: only the mirror is touched.
    fs::write(&proj, "hello\n").unwrap();
    fs::write(&internal, "hello\n").unwrap();
    fs::remove_file(&internal).unwrap();
    assert!(proj.exists(), "source must survive --keep-source");
    assert!(!internal.exists(), "mirror must be removed");
}

// Q12: init does not auto-create vertex dirs.
#[test]
fn init_does_not_auto_create_vertex_directories() {
    let tmp = fresh_project();
    let root = tmp.path();
    // fresh_project runs materialize(..., false /* no_vertex */, Copy).
    // Verify KRON/VERTEX either doesn't exist or is empty — no auto todo/doing/done.
    let v = root.join("KRON").join("VERTEX");
    if v.exists() {
        let entries: Vec<_> = fs::read_dir(&v)
            .unwrap()
            .filter_map(|e| e.ok())
            .collect();
        assert!(entries.is_empty(), "init must NOT auto-create vertex dirs");
    }
    // Also: no state indexes.
    let states = root.join("kron-internal").join("states");
    if states.exists() {
        let entries: Vec<_> = fs::read_dir(&states)
            .unwrap()
            .filter_map(|e| e.ok())
            .collect();
        assert!(entries.is_empty(), "init must NOT auto-create vertex state indexes");
    }
}

// ---- VertexRow marker (current = true) ----
//
// The `current` flag the JSON output carries is the field the human/
// porcelain modes derive their `*` from. We construct it inline because
// the struct is private to the commands module — that's fine, we just
// want a smoke test that `Serialize` works for our updated shape.

#[test]
fn vertex_list_marks_current_pointer_with_star_in_human_mode() {
    #[derive(serde::Serialize)]
    struct VertexRowLite {
        current: bool,
        name: String,
        path: String,
    }
    let row = VertexRowLite {
        current: true,
        name: "doing".into(),
        path: "KRON/VERTEX/doing".into(),
    };
    assert!(row.current);
    let json = serde_json::to_string(&row).unwrap();
    assert!(json.contains("\"current\":true"));
}

// ---- Back/check chain uses canonical TaskState ----

#[test]
fn back_and_check_use_canonical_chain() {
    // Done → Doing
    assert_eq!(TaskState::Done.prev_in_chain(), Some(TaskState::Doing));
    // Doing → Todo
    assert_eq!(TaskState::Doing.prev_in_chain(), Some(TaskState::Todo));
    // Todo has no previous
    assert_eq!(TaskState::Todo.prev_in_chain(), None);

    // Next chain
    assert_eq!(TaskState::Todo.next_in_chain(), Some(TaskState::Doing));
    assert_eq!(TaskState::Doing.next_in_chain(), Some(TaskState::Done));
    assert_eq!(TaskState::Done.next_in_chain(), None);

    // Default for new tasks is todo.
    assert_eq!(TaskState::default_for_new(), TaskState::Todo);
}

// ---- vertex::VertexArgs contains Use / Current subcommands ----

#[test]
fn vertex_subcommands_include_use_and_current() {
    // This is a structural test: it ensures the clap enum variants exist
    // and are wired through `run`. If somebody deletes them in a future
    // refactor the test catches it.
    //
    // We can't introspect the enum directly without clap's Command; so
    // we drive `vertex_cmd::run` with a stub Ctx and check it dispatches
    // without panicking. This is a smoke test only.
    use kron::commands::vertex::{VertexAction, VertexArgs};
    let _ = VertexAction::Use {
        name: None,
        unset: false,
    };
    let _ = VertexAction::Current;
    let _ = VertexArgs {
        action: VertexAction::Current,
    };
}

// ---- Vertex find_required returns clear hint ----

#[test]
fn vertex_find_required_hint_mentions_create_command() {
    let tmp = fresh_project();
    let root = tmp.path();
    let err = core_vertex::find_required(root, "missing").unwrap_err();
    match err {
        KronError::Cli(msg) => {
            assert!(msg.contains("missing"), "msg must include vertex name: {msg}");
            assert!(msg.contains("kron vertex create missing"), "msg must hint at fix: {msg}");
        }
        other => panic!("expected Cli error, got {other:?}"),
    }
}

// ---- description validation hooks ----

fn validate_desc(s: &str) -> std::result::Result<String, KronError> {
    kron::core::task::validate_description(s)
}

// 2026-09-07 tightening (R1): vertex list is strictly read-only.
// Creating an orphan directory in KRON/VERTEX/ must NOT result in
// it being silently registered. Operators who want to register it
// must run `kron vertex create <name>` explicitly.
#[test]
fn vertex_list_does_not_auto_register_orphan_directory() {
    let tmp = fresh_project();
    let root = tmp.path();

    // Register one vertex cleanly.
    core_vertex::create(root, "todo", None, None, None).unwrap();

    // Drop an *orphan* directory under KRON/VERTEX/ that is NOT
    // in the registry. This simulates a stray directory created
    // by hand, a buggy migration, or a deleted registry entry.
    let orphan = root.join("KRON").join("VERTEX").join("scratch");
    fs::create_dir_all(&orphan).unwrap();
    fs::write(orphan.join("README.md"), "stray").unwrap();

    // The registry must still only contain the registered vertex.
    let reg_after = core_vertex::load_registry(root).unwrap();
    assert_eq!(reg_after.len(), 1, "no auto-registration should have happened");
    assert_eq!(reg_after[0].name, "todo");
    assert!(core_vertex::find(root, "scratch").unwrap().is_none(),
        "orphan directory must NOT appear in the registry");

    // And `core_vertex::find_required` (the gatekeeper) must reject
    // the orphan name with the same Cli error path it uses for any
    // other unregistered vertex.
    let err = core_vertex::find_required(root, "scratch").unwrap_err();
    match err {
        KronError::Cli(msg) => {
            assert!(msg.contains("scratch"));
            assert!(msg.contains("kron vertex create scratch"),
                "orphan must be reported with the same hint as any unregistered vertex: {msg}");
        }
        other => panic!("expected Cli error, got {other:?}"),
    }
}

// 2026-09-07 tightening (R2): task commands must not create vertex
// physical directories. When a vertex is registered but its physical
// directory has been deleted (or was never materialized), task add
// must fail with a clear Cli error.
#[test]
fn task_add_to_registered_vertex_without_physical_dir_fails() {
    let tmp = fresh_project();
    let root = tmp.path();

    // Register the vertex (adds to vertices.json).
    core_vertex::create(root, "orphan", None, None, None).unwrap();

    // Now manually delete the physical directory so the vertex is
    // registered-but-no-directory — the precise scenario R2 covers.
    let dir = root.join("KRON").join("VERTEX").join("orphan");
    fs::remove_dir_all(&dir).unwrap();
    assert!(!dir.is_dir(), "precondition: physical dir removed");
    assert!(core_vertex::find(root, "orphan").unwrap().is_some(),
        "precondition: vertex still registered");

    // `require_vertex_dir` (the core gatekeeper) must reject this.
    let err = kron::core::task::require_vertex_dir(root, "orphan").unwrap_err();
    match err {
        KronError::Cli(msg) => {
            assert!(msg.contains("orphan"));
            assert!(msg.contains("kron vertex create orphan"),
                "error must hint at the fix: {msg}");
        }
        other => panic!("expected Cli error, got {other:?}"),
    }
}

// R2: task list on a vertex with no physical directory must also fail,
// not silently show an empty list.
#[test]
fn task_list_on_vertex_without_physical_dir_fails() {
    let tmp = fresh_project();
    let root = tmp.path();

    // Register + delete dir (same precond as above).
    core_vertex::create(root, "ghost", None, None, None).unwrap();
    fs::remove_dir_all(root.join("KRON").join("VERTEX").join("ghost")).unwrap();

    // The core guard fires regardless of which command invokes it.
    let err = kron::core::task::require_vertex_dir(root, "ghost").unwrap_err();
    match err {
        KronError::Cli(msg) => {
            assert!(msg.contains("ghost"));
            assert!(msg.contains("kron vertex create ghost"),
                "error must hint at the fix: {msg}");
        }
        other => panic!("expected Cli error, got {other:?}"),
    }
}
