# 2026-09-06 — CLI 语义裁决 v2（Anchor #8 待执行）

> **状态**：可执行。本版本回应用户的 4 个明确决定（A/B 二选 + 三个补充）。
> **范围**：Anchor #8 = 语义裁决 + 标准化（不含 undo，undo 列入 Anchor #9）。
> **来源**：用户对 5 个原始问题 + 4 个新问题的回复（2026-09-06 22:20）。

---

## 1. 全部裁决一览

### 1.1 第一轮（2026-09-06 22:12 已确认）

| # | 裁决 | 含义 |
|---|------|------|
| Q1 | **保留** `TaskState` 枚举（todo/doing/done） | `parse_state` 严格化 |
| Q2 | **做** undo | Anchor #9 |
| Q3 | `important remove` 默认删源文件 | 加 `--keep-source` |
| Q4 | stdout/stderr 分离 v1 必须 | `--json` 走 stderr |
| Q5 | 保留 `--json` / `--porcelain` 两个 flag | 不引入 `--format` |
| Q6 | `task add` 禁止创建 vertex | 必须先 `vertex create` |
| Q7 | `task add` 默认 state=todo | state 恒为 todo |
| Q8 | 统一为 `--description` / `--body` | `add` 时 title + 至少一项必填 |

### 1.2 第二轮（2026-09-06 22:20 新增 / 修正）

| # | 裁决 | 含义 |
|---|------|------|
| **Q9** | **抉择 A** | vertex 即物理目录；state 在 frontmatter；`move` 改 frontmatter + 跨目录搬运 |
| **Q10** | **做 vertex 指针命令** + **task 默认走当前 vertex** | 新增"current vertex"指针能力 |
| **Q11** | **`task` 命令跨 vertex 需显式 `--vertex`** | 不允许隐式跨 vertex（防止误操作） |
| **Q12** | **`kron init` 不自动创建 vertex** | 用户手动 `vertex create todo/doing/done` |
| **Q13** | **breaking change 直接做** | v1 没发布，不保留兼容 |

### 1.3 Anchor 拆分（最终）

| Anchor | 范围 | 工作量 |
|--------|------|--------|
| **#8 语义裁决 + 标准化** | Q1/Q3/Q6/Q7/Q8/Q9/Q10/Q11/Q12/Q13 + stdout/stderr/退出码 | ~3 天 |
| **#9 undo + reflog** | Q2 落地 | ~2 天 |

---

## 2. 新增概念：vertex 指针

### 2.1 为什么需要

- **现状**：用户跑 `kron task list` 必须显式传 vertex，否则报"missing vertex"
- **痛点**：用户 80% 时间都在某一个 vertex 上操作（典型：当前在做"开发"阶段
  的 task），反复输 vertex 名太烦
- **设计目标**：仿 Git 的 `HEAD` / `branch` 指针概念
  - `kron vertex use <name>` —— 设置当前 vertex
  - `kron task list` —— 隐式用当前 vertex
  - `kron task list --vertex <name>` —— 显式指定（覆盖）

### 2.2 存储位置

- 文件：`kron-internal/state.json`（新建，单文件）
- 内容：
  ```json
  {
    "version": 1,
    "current_vertex": "开发",
    "updated_at": "2026-09-06T22:30:00Z"
  }
  ```
- **关键决策**：vertex 指针是**全局**还是**per cwd 子目录**？
  - 选 A：**全局**（推荐）—— 一个项目只有一个"当前 vertex"，所有 cwd 共享
  - 选 B：per cwd（仿 Git 的 per-worktree）—— 太复杂，v1 不做
  - **选 A**

### 2.3 子命令清单

```
kron vertex use <name>      # 设置当前 vertex（等价 `kron vertex checkout <name>`）
kron vertex use             # 显示当前 vertex
kron vertex use --unset     # 清除当前 vertex（之后 task 命令必须显式指定 vertex）
```

**命名抉择**：用 `use` 还是 `checkout`？
- `use`：简洁，意图明确
- `checkout`：与 Git 对齐，但 Git checkout 有歧义（切分支/恢复文件）
- **选 `use`**——更不容易混

### 2.4 task 命令的 vertex 参数语义

| 命令 | 行为 |
|------|------|
| `kron task list` | 用当前 vertex；无当前 vertex 报错 `Cli("no current vertex; pass --vertex <name> or run 'kron vertex use <name>'")` |
| `kron task list --vertex foo` | 显式指定，覆盖当前 vertex |
| `kron task add --title "x"` | 用当前 vertex；无当前 vertex 报错 |
| `kron task add --vertex foo --title "x"` | 显式指定 |
| `kron task move T1 --to doing` | **不查 vertex**（通过 task id 自动定位）；state 必须 todo/doing/done 三选一 |
| `kron task show T1` | 不查 vertex |
| `kron task delete T1` | 不查 vertex |

**关键决策**（Q11）：
- **隐式跨 vertex 不允许**
  - 例：在 `current=todo` 下，`kron task add --title "x" --vertex doing` **允许**（用户显式说明）
  - 例：在 `current=todo` 下，`kron task move T1 --to doing` **允许**（move 改 state 必须跨 vertex；这是 Q11 的**唯一例外**）
  - 例：在 `current=todo` 下，`kron task delete T1`（T1 在 doing 下）**报错**（应先用 `kron vertex use doing` 或显式 `--vertex doing`）
- **但 `move` 是例外**——`move --to doing` 必然把 task 从 todo 移到 doing，**跨 vertex 是语义本身**

### 2.5 vertex 子命令的更新

**新增子命令**：
- `kron vertex use [name]` —— 设置 / 显示当前 vertex
- `kron vertex list` —— 列已注册 vertex（**当前 vertex 用 `*` 标记**）
- `kron vertex current` —— 仅显示当前 vertex（alias for `vertex use` 无参数）

**不变**：
- `create / describe / branch / delete / show` 不动

---

## 3. 全部代码改动清单（按文件）

### 3.1 模型层（`src/`）

#### `src/model.rs`
- **行 ~178-186** `parse_state` → 改为严格枚举：
  ```rust
  pub enum TaskState { Todo, Doing, Done }
  pub fn parse_state(s: &str) -> Result<TaskState, String> {
      match s.to_ascii_lowercase().as_str() {
          "todo" => Ok(TaskState::Todo),
          "doing" => Ok(TaskState::Doing),
          "done" => Ok(TaskState::Done),
          _ => Err(format!("invalid state '{s}', expected todo|doing|done")),
      }
  }
  ```
- **行 ~142-150** `TaskState` 加 `FromStr` 实现（让 clap `value_enum` 配合）

### 3.2 core 层（`src/core/`）

#### `src/core/task.rs` —— 影响 3 处
- **行 ~359-365** `normalize_description`：
  - 不再静默截断，截断时返回 `Err`
  - 改名为 `validate_description(s: &str) -> Result<String>`（≤200 字符）
- **行 ~310-320** `parse_task_md` 的 state 字段：保留 String（frontmatter 兼容），但**写入**时**强制**为 `todo`/`doing`/`done` 三选一（避免历史脏数据）
- **行 ~190-220** `move_task`：
  - 参数改 `to: TaskState` 而非 `&str`
  - 行为：① 改 frontmatter state ② 物理移到对应 vertex 目录
- **行 ~370-400** `add_task`：
  - **删除**自动创建 vertex 目录的代码
  - 校验 vertex 已注册（**调用方负责**，core 不负责）
  - 写文件时 state=todo 强制

#### `src/core/vertex.rs` —— 影响 2 处
- **新增** `pub fn list(project_root: &Path) -> Result<Vec<VertexRecord>>` （或直接 `load_registry`）
- **新增** `pub fn find_required(project_root: &Path, name: &str) -> Result<VertexRecord>` —— 找不到时直接报 `Cli("vertex '{name}' does not exist; run 'kron vertex create {name}' first")`
- **不改 create / update / delete**

#### `src/core/state_pointer.rs` —— 新文件
- 维护 `kron-internal/state.json` 的读写
- API：
  ```rust
  pub fn load(root: &Path) -> Result<Option<String>>    // 当前 vertex 或 None
  pub fn set(root: &Path, name: &str) -> Result<()>      // 设置
  pub fn clear(root: &Path) -> Result<()>                 // 清除
  ```

### 3.3 commands 层（`src/commands/`）

#### `src/commands/task.rs` —— 大量改动
- **行 ~30-100** `TaskAction` 重构 flag 名：
  | 旧 flag | 新 flag |
  |---------|---------|
  | `--title` | `--title`（保持） |
  | `--desc` | `--description`（单行，≤200 字符） |
  | `-m`/`--message` | `--body`（多行，可从 stdin） |
  - `add` / `describe` / `move` 全部用新 flag
- **行 ~20-25** `List` 加 `--vertex <name>` 参数
- **行 ~30-35** `Show` / `Edit` / `Delete` / `Tag` 加 `--vertex <name>` 参数（用于校验"当前 vertex 不允许跨操作"，且 `move` 是例外）
- **行 ~125** `run()` 把 `vertex` 改为可空，根据"当前 vertex 指针 + --vertex flag"二选一解析
- **行 ~370-400** `add_task`：
  - 删除"自动建 vertex dir"逻辑
  - 校验 vertex 已注册（调 `core::vertex::find_required`）
  - state 恒为 `TaskState::Todo`
- **行 ~308-313** `move_task_cmd`：`to` 改为 `TaskState`
- **行 ~531-550** `check_task_cmd`：不变（已经在两个 state 间切）
- **行 ~263-265** `describe_task`：去掉 200 字符截断的软警告；超长直接报 `Cli` 错误（exit 2）

#### `src/commands/vertex.rs` —— 中等改动
- **行 ~15-50** `VertexAction` 加 `Use` 子命令
- **行 ~30-45** `Create` / `Describe`：flag 重命名 `-m` → `--description`
- **行 ~95-110** `list_cmd`：当前 vertex 加 `*` 标记
- **新增** `use_cmd(name)` 实现 + `current_cmd()`

#### `src/commands/important.rs` —— 少量改动
- **行 ~148-163** `Remove` 子命令的 `--also-delete-source` 改名为 `--keep-source`，**反转默认行为**
- 帮助文本更新

#### `src/commands/init.rs` —— 最小改动
- 不创建 vertex（保持当前行为）
- 在 `--help` 文本里**追加提示**："after init, run `kron vertex create todo/doing/done` to bootstrap state columns"

#### `src/commands/conflict.rs` —— 不改

#### `src/commands/daemon.rs` —— 不改

### 3.4 output 层（`src/output.rs`）—— 重点改造

- **加** `emit_json_stderr(value)` —— JSON 走 stderr
- **加** `emit_human_stdout(msg)` —— 人类模式 stdout
- **加** `emit_porcelain_stdout(line)` —— porcelain stdout
- **加** `error_stderr(structured)` —— 错误走 stderr（人类/JSON 都走）
- **保留** `OutputMode::from_flags` 兼容

**实现模式**：
```rust
// 当前 commands/task.rs 里有 38 处这种 match：
match ctx.mode {
    crate::output::OutputMode::Json => {
        println!("{}", serde_json::to_string_pretty(&summary)?);
    }
    crate::output::OutputMode::Porcelain => {
        println!("{}\t{}", id, vertex);
    }
    crate::output::OutputMode::Human => {
        println!("✓ Task {} created", id);
    }
}

// 改为：
output::emit(ctx.mode, &summary_pretty, &human_msg)?;
```

**目标**：38 处 `match ctx.mode` 收敛到 ≤ 5 处（仅保留真正不同的特殊输出）

### 3.5 main 层（`src/main.rs`）—— 退出码映射

- 当前 `main()` 直接返回 `Result<()>`
- 改为：
  ```rust
  fn main() {
      let result = kron::run();
      let exit_code = match result {
          Ok(()) => 0,
          Err(e) => exit_code_for(&e),
      };
      std::process::exit(exit_code);
  }
  ```
- 新增 `src/error.rs` 的 `KronError::exit_code()` 方法

### 3.6 error 层（`src/error.rs`）

**新增方法**：
```rust
impl KronError {
    pub fn exit_code(&self) -> i32 {
        match self {
            Self::Cli(_) => 2,
            Self::NotFound(_) => 8,
            Self::NotAProject(_) => 4,
            Self::InvalidFrontmatter { .. } => 8,
            Self::InvalidVertexName(_) => 2,
            Self::InvalidTaskFilename(_) => 2,
            Self::DuplicateTaskId(_) => 6,
            Self::AlreadyInitialized(_) => 4,
            Self::NotGitRepo(_) => 4,
            Self::AncestorProject { .. } => 4,
            Self::NotYetImplemented(_) => 1,
            Self::PermissionDenied(_) => 9,
            Self::Io(_) | Self::Json(_) | Self::Yaml(_) => 6,
            Self::Internal(_) => 1,
        }
    }
}
```

---

## 4. 测试覆盖计划

### 4.1 单元测试（src/）

- `model::parse_state` —— todo/doing/done 三合法 + 5 个非法字符串
- `core::task::normalize_description`（改 `validate_description`）—— ≤200 通过、>200 报 Err
- `core::task::move_task` —— todo→doing 物理移动 + frontmatter 更新
- `core::state_pointer` —— load/set/clear/不存在

### 4.2 集成测试（tests/）

**新增 `tests/cli_semantic_v2.rs`**：
- `kron vertex use todo` → `kron task list` 不报错
- `kron task add foo --title x`（无 `vertex create foo`）→ exit ≠ 0 + stderr 含 "vertex 'foo' does not exist"
- `kron task add todo --title x`（vertex 未注册）→ 同样报"does not exist"
- `kron task move T1 --to foo` → exit ≠ 0 + "invalid state"
- `kron task add --title x`（无 current vertex） → exit ≠ 0
- `kron important remove foo.md`（默认）→ 源文件被删
- `kron important remove foo.md --keep-source` → 源文件保留
- `kron task describe T1 --description <201 chars>` → exit ≠ 0
- `kron status --json` 的 stderr 包含 JSON、stdout 为空
- 退出码表全跑一遍

**更新现有测试**：
- `tests/cli_p4.rs` 涉及 `-m` / `--desc` 的全部改成 `--description` / `--body`
- 涉及 `--also-delete-source` 的全部改成 `--keep-source`
- 涉及隐式跨 vertex 的测试要明确传 `--vertex`

### 4.3 既有测试预期影响

- 既有 **60 个测试**预计 **20-30 个** fail 要修
- 主要是 flag 名变化 + 默认行为变化
- **不允许**为通过而删测试，只改输入参数

---

## 5. 验收清单（Anchor #8 完成标准）

### 5.1 业务语义

- [ ] `kron task add todo --title "x"` 必先 `kron vertex create todo`，否则报 `Cli`
- [ ] `kron task add todo --title "x"` 写 frontmatter `state: todo`
- [ ] `kron task move T1 --to doing` 改 frontmatter + 物理移到 `KRON/VERTEX/doing/T1.md`
- [ ] `kron task move T1 --to foo` 报 `Cli("invalid state 'foo', expected todo|doing|done")`
- [ ] `kron task describe T1 --description "x"` 单行 ≤200 字符，超长报 `Cli`
- [ ] `kron task describe T1 --description <201 chars>` exit 2
- [ ] `kron important remove foo.md` 默认删源文件
- [ ] `kron important remove foo.md --keep-source` 保留源文件
- [ ] `kron init` **不**自动创建 vertex；输出含 `hint: kron vertex create todo/doing/done`
- [ ] `kron vertex use <name>` 设置当前 vertex；无参数显示
- [ ] `kron vertex use --unset` 清除当前 vertex
- [ ] `kron vertex list` 当前 vertex 加 `*` 标记
- [ ] `kron task list`（无 --vertex）用当前 vertex；无当前 vertex 报 `Cli`
- [ ] `kron task list --vertex foo` 显式覆盖当前 vertex
- [ ] `kron task delete T1`（T1 在其他 vertex，且无 --vertex）报 `Cli("cross-vertex delete requires --vertex")`

### 5.2 输出与退出码

- [ ] `--json` 模式：stdout 为空，stderr 包含结构化 JSON
- [ ] `--porcelain` 模式：stdout 走 tab 分隔记录，stderr 走错误
- [ ] 人类模式：stdout 走人类消息，stderr 走错误
- [ ] `Cli(_)` → exit 2
- [ ] `NotFound(_)` → exit 8
- [ ] `NotAProject(_)` → exit 4
- [ ] `Io(_)` → exit 6
- [ ] `task add <未注册>` → exit 2（Cli 变体）
- [ ] `task move <id> --to <非法 state>` → exit 2
- [ ] `kron status --json | jq` 能直接解析（stdout 空、stderr 一行 JSON）

### 5.3 既有测试

- [ ] `cargo test --workspace` 全绿
- [ ] **不删任何既有测试**（仅修改输入参数适配新 flag）
- [ ] 新增测试 ≥ 15 个

---

## 6. 执行顺序（建议）

### Day 1：模型 + 核心层

```
1. model.rs —— parse_state 严格化 + TaskState::FromStr
2. error.rs —— 加 exit_code()
3. core/task.rs —— validate_description 改名 + 强校验
4. core/vertex.rs —— 加 find_required
5. core/state_pointer.rs —— 新文件
```

### Day 2：commands 层

```
6. commands/task.rs —— flag 重命名 + 校验 vertex + state=todo
7. commands/vertex.rs —— 加 use 子命令 + flag 重命名
8. commands/important.rs —— remove 默认反转 + flag 重命名
9. commands/init.rs —— 加 hint 帮助文本
```

### Day 3：output + main + 退出码

```
10. output.rs —— emit_json_stderr / emit_human_stdout 等
11. commands/*.rs —— 38 处 match ctx.mode 收敛
12. main.rs —— exit code 映射
13. tests/ —— 新增 cli_semantic_v2.rs + 更新既有测试
```

### Day 4：打磨

```
14. 全 cargo test 跑通
15. 手工 smoke test（5 个核心场景）
16. dev-journal 写 Anchor #8 工作记录
```

---

## 7. 风险与决策记录

### 7.1 已锁定的关键决策

| 决策 | 选择 | 理由 |
|------|------|------|
| vertex = 物理目录 | 抉择 A | 与现状兼容；按阶段分组清晰 |
| vertex 指针位置 | 全局 `kron-internal/state.json` | 一个项目一个"当前 vertex" |
| undo | Anchor #9（不在本期） | 工作量 ~2 天，单独 anchor 更聚焦 |
| breaking change | 直接破坏旧 flag | v1 未发布，无用户依赖 |
| `--keep-source` 语义 | 反义于旧 `--also-delete-source` | 直觉优先 |

### 7.2 待二次确认的小事

- `kron vertex use` 还是 `kron vertex checkout`？（**倾向 use**——checkout 多义）
- `state.json` 与 `vertices.json` 是否合并？（**倾向不合并**——state 是用户行为，vertices 是数据）
- 跨 vertex 操作（除 move 外）禁止 / 允许？（**倾向禁止**——Q11）

---

## 8. 关联

- **上游**：`2026-09-06-cli-review.md`（评审报告）
- **下游**：Anchor #9（undo + reflog）
- **关联锚点**：#6（祖先搜索）、#7（CLI 评审）、#8（本文件）、#9（undo）

---

**文档结束。**

确认无误后我开始动手 Day 1。