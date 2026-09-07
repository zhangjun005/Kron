# Anchor #8 实施修正记录 — 2026-09-07

## 背景

本记录是 [2026-09-06-cli-semantic-decisions.md](../dev-journal/2026-09-06-cli-semantic-decisions.md)（简称"v2 文档"）的增量修正。
Day 2 的 CLI 层改造（由另一 agent 会话完成）在落地时有 4 处与 v2 文档一致但产生实际隔离性问题的实现。用户在 2026-09-07 晨会中提出三条设计原则（"R1/R2/R3"），本记录记录这些原则如何落地。

---

## 设计原则（用户确认）

### R1 — vertex 创建必须绑定当前项目；registry 写操作仅限 `vertex create`

**含义**：vertex 的注册表（`kron-internal/vertices.json`）是单一真理源，**只能通过 `kron vertex create`** 写入。不允许任何其他命令隐式注册、auto-register、或绕过 `core_vertex::create` 直接写文件。

**落地**：
- `commands/vertex.rs` 的 `list_cmd` 不再 auto-register orphan 目录（删除了 20 行 auto-register 逻辑）
- 删除了 `core_vertex_save` helper（绕过 `core_vertex::create` 的旁路）
- orphan 目录现在在 `vertex list` 输出中显示为 `# orphan / orphan` 行（Human 模式），而不是被偷偷写入 registry
- 新测试：`vertex_list_does_not_auto_register_orphan_directory`

### R2 — `kron task` 系列**不创建** vertex，不依赖隐式目录创建

**含义**：`task` 命令组的所有操作**只能在已存在的 vertex 物理目录内进行**。没有任何 `task` 子命令可以创建 `KRON/VERTEX/<name>/` 目录。

**落地**：
- `core/task.rs` 新增 `require_vertex_dir(project_root, vertex)` helper——在读写 task 前验证物理目录存在
- `commands/task.rs` 的 `add_task` 和 `list_tasks` 不再调用 `fs::create_dir_all`
- 改前：`task add` 调用 `create_dir_all`——会在 registry 注册后隐式创建物理目录
- 改后：`require_vertex_dir` 检查目录存在性，不存在则报 `Cli` 错 + hint "run 'kron vertex create ...' first"
- 新测试：`task_add_to_registered_vertex_without_physical_dir_fails`、`task_list_on_vertex_without_physical_dir_fails`

**注意**：`core/task.rs` 里的 `write_task` 和 `move_task` 仍有 `create_dir_all`（这是 core 层操作，与命令层无关）。用户如果**直接调用 `write_task` 或 `move_task`** API，core 层仍会自动创建目录。这是有意的分层边界：core 层 API 不强制目录存在（调用者负责前置校验），命令层强制。

### R3 — vertex 指针切换是 `vertex` 命令组的专属功能，不在非 vertex 命令中暴露

**含义**：指针的**写入**（set/clear）只能通过 `kron vertex use` 系列命令进行。其他命令（包括 `kron vertex delete`）不应隐式修改指针。

**落地**：
- 改前：`vertex delete` 删除当前指向的 vertex 时，**自动清除指针**（4 行隐藏逻辑）
- 改后：删除当前指向的 vertex 后，指针值**保持不变**，但输出 human 警告：
  ```
  ⚠ the current-vertex pointer still points at 'xxx'
    run `kron vertex use --unset` or `kron vertex use <other>` to repair
  ```
- JSON/Porcelain 模式：`pointer_was_orphaned: true` + `hint: "kron vertex use --unset"` 字段
- 新测试：`vertex_delete_leaves_pointer_intact_when_targeting_current`

**关于"隐式"的定义**：读取指针（`state_pointer::current`）是合规的——所有 `task` 子命令通过 `resolve_target_vertex` 读取指针作为 fallback，这不算"切换"。

---

## 与 v2 文档的偏差记录

| 偏差 | v2 文档承诺 | 实际实现 | 偏差原因 |
|------|-----------|---------|---------|
| 孤儿目录处理 | 未提及 | `vertex list` 只读，不 auto-register；orphan 在 human 模式显式显示 | R1 收紧 |
| 无指针时跨 vertex 操作 | v2 § 2.4 隐含"报错" | `task show/delete/...` 在无 current vertex 时**静默允许**跨 vertex | 用户体验改进 |
| `task add` 物理目录 | v2 隐含"task add 不创建 vertex 注册条目" | R2 进一步要求**物理目录也必须已存在** | 用户明确确认 |
| `vertex delete` 清除指针 | 未明确 | R3：不清除，显式警告 | R3 收紧 |
| 创建 vertex 时是否自动 use | v2 § 2.4 未明确 | **不自动 use**（创建后指针不变） | Day 2 显式选择 |

---

## 改动文件清单

| 文件 | 改动类型 | 说明 |
|------|---------|------|
| `src/commands/vertex.rs` | 修改 | 删 auto-register；删 `core_vertex_save`；删隐式 `clear`；加 orphan 警告 |
| `src/commands/task.rs` | 修改 | 删 `create_dir_all`；加 `require_vertex_dir` 调用；更新注释 |
| `src/core/task.rs` | 修改 | 新增 `require_vertex_dir` helper |
| `tests/cli_semantic_v2.rs` | 新增测试 | +4 个测试：R1/R2/R3 各 1 个 + 1 个辅助测试 |

---

## 测试结果

```
cli_semantic_v2: 20 passed (+4 new)
Full suite: 106 tests, 0 failures
```

---

## 下一步（Day 3）

- `src/output.rs` 加 `emit_json_stderr / emit_human_stdout` 等 helper
- `commands/*.rs` 中的 `match ctx.mode` 收敛（38 处 → ≤5 处核心 helper）
- `src/main.rs` exit code 映射（用 `KronError::exit_code()`）
- dev-journal 写 Day 3 Anchor #9 工作记录
