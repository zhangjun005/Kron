<!-- kron-generated: 2026-09-07T09:29:17.434850100+00:00 -->
<!-- DO NOT EDIT. Run 'kron context --regenerate' to refresh. -->

# Project Structure

深度：3（可配置）

```
Kron/
dev-docs/
├── design/
│   ├── 00-总览与架构.md
│   ├── 01-数据模型.md
│   ├── 02-模块划分.md
│   ├── 03-双源同步机制.md
│   ├── 04-守护进程与文件监听.md
│   ├── 04b-CLI设计.md
│   ├── 05-GUI设计.md
│   ├── 06-数据格式规范.md
│   ├── 07-实施路线图.md
├── dev-journal/
│   ├── 2026-09-06-ancestor-search.md
│   ├── 2026-09-06-cli-review.md
│   ├── 2026-09-06-cli-semantic-decisions.md
│   ├── 2026-09-06-work-log.md
│   ├── 2026-09-07-anchor-8-tighten.md
│   ├── 2026-09-07-day-3-output-and-exitcode.md
│   ├── 2026-09-07-day-4-ctx-helpers.md
├── commonQuestions.md
├── requirements.md
KRON/
├── .kron-context/
│   ├── git/
│   │   ├── branch-summary.md
│   │   ├── recent-commits.md
├── important/
├── VERTEX/
├── README.md
kron-internal/
├── important/
├── config.json
├── vertices.json
src/
├── commands/
│   ├── config.rs
│   ├── conflict.rs
│   ├── context.rs
│   ├── daemon.rs
│   ├── important.rs
│   ├── init.rs
│   ├── list_projects.rs
│   ├── mod.rs
│   ├── path.rs
│   ├── status.rs
│   ├── task.rs
│   ├── vertex.rs
├── core/
│   ├── context/
│   │   ├── code_structure.rs
│   │   ├── git_branch.rs
│   │   ├── git_commits.rs
│   │   ├── mod.rs
│   │   ├── readme.rs
│   ├── sync/
│   │   ├── conflict.rs
│   │   ├── daemon.rs
│   │   ├── mod.rs
│   │   ├── scan.rs
│   │   ├── sync_index.rs
│   ├── init.rs
│   ├── mod.rs
│   ├── project.rs
│   ├── state_pointer.rs
│   ├── task.rs
│   ├── vertex.rs
├── cli.rs
├── error.rs
├── lib.rs
├── main.rs
├── model.rs
├── output.rs
testProject/
├── KRON/
│   ├── important/
│   │   ├── notes.md
│   ├── VERTEX/
│   │   ├── doing/
│   │   ├── done/
│   │   ├── review/
│   │   ├── todo/
│   ├── README.md
├── kron-internal/
│   ├── conflicts/
│   │   ├── 20260906_035007_be8a9f_kron_important_notes_md/
│   │   ├── demo_conflict/
│   │   ├── _index.json
│   ├── important/
│   │   ├── files/
│   │   ├── _index.json
│   ├── states/
│   │   ├── doing.json
│   │   ├── done.json
│   │   ├── todo.json
│   ├── .daemon.pid
│   ├── .daemon.status.json
│   ├── config.json
│   ├── vertices.json
├── src/
│   ├── demo.md
tests/
├── cli_p4.rs
├── cli_semantic_v2.rs
├── init_layout.rs
├── sync_engine.rs
├── task_crud.rs
├── task_transitions.rs
.gitignore
build.log
Cargo.lock
Cargo.toml
gnu-install.log
LICENSE
plan.md
README.md
```

(跳过: node_modules/, target/, .git/, 等)
(共 115 个条目)
