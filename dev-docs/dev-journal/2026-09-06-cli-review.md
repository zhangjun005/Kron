# 2026-09-06 — Kron CLI 实现评审（Anchor #7）

> **本文是评审报告，不是行动计划。**
> 范围：依据 `requirements.md`、`04b-CLI设计.md`、`07-实施路线图.md` 与
> 当前 `src/` 源码，对 CLI 现状做一次彻底的横评。
> 目的：在动手"下一步开发"前，把混乱面暴露出来并形成共同判断。

---

## 1. 一句话总结

CLI **能用**，但**不是在建模业务**——而是在**模拟 Git 的命令动词**，并且
把"业务语义"和"实现细节"这两个本该分离的层次混淆在了一起。文档已经定型了
业务哲学（双源、AI 易读、Vertex = 阶段、state ≠ 文件移动），但代码实现里
至少有 5 个语义被**悄悄地换成了另一种东西**，并且不报错。

**最重要的一个判断**：
> **当前的 `task` 和 `vertex` 是同一个概念的两种说法**，代码把同一组操作
> 拆成了两个 surface，但语义是重叠的。这是一切混乱的根源。

---

## 2. 业务场景建模评审（对照 requirements.md）

requirements § 2 写了**极清晰**的业务哲学。我把每条与代码对照，看实现是否在
体现这套哲学。

### 2.1 ✅ 已对齐的部分（做得好的）

| 业务原则 | 代码现状 | 评价 |
|---------|---------|------|
| 软件/数据分离 | `kron-internal/` + `KRON/` 双源 | ✅ 全模块统一 |
| Vertex = 阶段 | `core::vertex::VertexRecord` + `core::task::TaskState` | ✅ 模型清楚 |
| AI 易读优先 | `TaskRow`、`StatusReport` 全 JSON / porcelain / human 三态 | ✅ 非常好 |
| 不强求二级标题 | task MD 解析只读 `## [task_xxx]` | ✅ 自由度足够 |
| 守护进程 vs CLI 解耦 | CLI 不假设 daemon 在跑 | ✅ 命令可独立 |
| 祖先向上扫描 | `core::project::find_project_root` + `Ctx.project_root` | ✅ 刚刚对齐 |
| Git 风格契约 | CLI 命名（`list / show / add / create / delete / move`） | ✅ 已锁定 |

**这些都是 30+ commit 累积出来的成果，不应被推翻。**

### 2.2 ❌ 业务语义被悄悄替换的 5 处（核心问题）

#### ❌ 问题 1：`vertex` 与 `task` 是同一概念的两种说法（语义重叠）

requirements § 2.1 明确：
> Vertex 是一个独立的开发阶段；task 在 vertex 之下。

但代码实现里：
- `TaskState` 是 `Todo / Doing / Done`（**枚举**）
- 用户加 task 时 `--vertex <name>` 既可以传 `todo` / `doing` / `done`，也可以传自定义名
- `move_task` 把 task 在 **vertex 目录之间搬动**

**这意味着 `task state` 和 `vertex name` 是同一个东西的两套叫法。**

证据：
```rust
// model.rs:142-150
pub enum TaskState { Todo, Doing, Done }    // 枚举只有 3 个
// model.rs:179-186
pub fn parse_state(s: &str) -> Result<String, String> {
    if s.is_empty() { return Err(...); }
    Ok(s.to_ascii_lowercase())              // 任意字符串都通过
}
```

→ `parse_state` 把任意字符串都接受为合法"state"，**等于取消了 TaskState 枚举**。
→ `task move --to foo` 创建了一个 `foo` vertex，**用户根本没意识到这是创建
  vertex**。

这是**最大的语义混乱**。它在 requirements § 2.2 已经被讨论过——
"state 仅在 GUI 中维护，不写入 MD"——但**实现里 state = vertex name**，二者
直接耦合，违背了原意。

**用户视角的实际行为**：
```
$ kron task add foo --title "x"
# 看上去加到 foo vertex；实际上创建了 foo 目录

$ kron task move T1 --to bar
# 看上去改 state；实际把 T1.md 从 foo/ 搬到 bar/

$ kron task list bar
# 列出 bar/ 下所有 task
```

**用户期待**：state 是 GUI 拖拽的事；vertex 是开发阶段。
**代码实际**：state = vertex 名 = 目录名 = 文件位置 = 一切。

#### ❌ 问题 2：`vertex create` 与 `task add <new-name>` 等价，破坏语义

如果 `task add foo` 能创建 `foo/` 目录，那 `kron vertex create foo` 是什么？
代码里有两条路都到同一个目录：

| 路径 | 命令 |
|------|------|
| A | `kron vertex create foo` → 在 registry 注册 + 在 KRON/VERTEX/ 下建目录 |
| B | `kron task add foo --title ...` → 直接建目录 + 加 task 进目录 |

**问题**：
- A 路径不创建 vertex directory（`vertex.rs:282-330` 的 create_cmd 只是 `core_vertex::create`，需要核实）
- B 路径**绕过 registry**——`kron vertex list` 看不到这个 vertex
- 两条路**互不知晓**

这是非常具体的隐患。已注册但无目录（**僵尸 vertex**）和未注册但有目录
（**幽灵 vertex**）同时存在。

#### ❌ 问题 3：`back` 用硬编码顺序，且与 `move` 行为冲突

```rust
// task.rs:515-528
fn previous_state(current: &str) -> Option<String> {
    const DEFAULT_STATE_ORDER: &[&str] = &["backlog", "todo", "doing", "review", "done"];
    ...
}
```

→ 用户自定义 vertex `archive`、`paused`，`back` 永远退回 `todo`。
→ 这个 `DEFAULT_STATE_ORDER` 与 `TaskState` 枚举毫无关系，是另起炉灶。
→ `requirements § 2.2.9` 没规定 stage 顺序——Kron 的核心哲学是
  "**Vertex 关系由 Git 树推导**"。`back` 命令在没有任何 vertex 关系模型时
  凭空捏造了一个顺序，**等于在用错误的图模型指挥用户**。

#### ❌ 问题 4：`task describe` 静默截断到 200 字符，无 warning 退出码

```rust
// task.rs:268-272
let (desc, truncated) = core_task::normalize_description(message);
task.description = desc;
task.updated_at = Utc::now();
task.source_file = None;
core_task::update_task(&path, &task)?;
```

`normalize_description` 返回的 `truncated` 仅用作**人类模式打印 warning**，
不进入错误通道。
- `--json` 用户看不到 `truncated` 是 true（实际有，但人类模式之外的 stdout 已经
  是 summary JSON 而不是单独 error）
- **失败信号被降级为软提示**——破坏了"静默失败透明"原则（CLI 设计 § 1 P5）

#### ❌ 问题 5：edge cases 没有清晰退出码

`requirements § 6.1` / `04b § 6` 锁定 13 个语义化退出码。
**当前实现**：所有错误统一返回 `Err(KronError::*)`，由 `main()` 默认返回 `1`。

| 场景 | 期望退出码 | 实际 |
|------|-----------|------|
| 未初始化 | 4 (`NotInitialized`) | 1 |
| task 不存在 | 8 (`NotFound`) | 1 |
| 缺 `--title` | 2 (`UsageError`) | 1（部分走 clap 的 2，部分走 KronError 的 1） |
| 冲突未解决 | 3 (`ConflictPending`) | 1 |
| 文件系统错误 | 6 (`IoError`) | 1（与 `Io` variant 都走 1） |

**当前 `main.rs` 没有 exit code 映射**——`Result<()>` 透传给 `process::exit`
会出 `1`（unwrap 时 panic），但**没有任何结构**。

→ 实施路线图 P2 § 4.4 已经标 "完整 7 个退出码留到 v2 整理"——
**v2 已经来了，**这正是要解决的核心之一。

---

## 3. 与 Git 对齐的差距（对照 04b § 1 P1）

`04b` 把"与 Git CLI 对齐"列为 P1。逐项核对：

| Git 行为 | Kron 现状 | 差距 |
|----------|----------|------|
| **`git add <pathspec>`**：精确路径，可多个 | `kron task add <vertex>`：第一个参数是 vertex 不是 path | ⚠️ 命名错位 |
| **`git status --porcelain=v2`**：机器可解析、稳定 schema | 有 `--porcelain` 但 schema 未文档化锁定 | ⚠️ 未锁定 |
| **`git commit -m "..."`**：`-m` 含义统一（提交信息） | `-m` 在 `task add / describe / vertex create / vertex describe` 都有，但语义有微妙差异：<br>`add`: 写 body<br>`describe`: 覆盖 description<br>`vertex create`: 写 description<br>`vertex describe`: 覆盖 description | ⚠️ **同一 flag 多语义** |
| **`git checkout --` / `git reset`**：有破坏性默认行为需 `--force` | `task delete` 要 `--force`（✅）<br>`vertex delete` 要 `--force`（✅）<br>**但 `important remove` 不删源文件除非 `--also-delete-source`**（**用户的直觉是"`important remove` 会删两份"**） | ⚠️ 隐式语义未文档化 |
| **`git log --oneline`**：默认人类可读 | 多种格式混合 | ✅ |
| **`git push` / `git pull`**：远程交互失败返回 128 | 没有 | — |
| **`git stash` / `git reflog`**：恢复路径 | **完全没有**——`remove` 是破坏性的，没有 undo | ❌ **重大缺口** |
| **`git worktree`**：多视图 | `kron daemon --worktree` 没做 | ✅ v1 可省 |

**关键缺口**：
- **`kron task undo` / `kron important undo`** —— 全无
- **`kron reflog`** —— 全无
- **`-m` 语义统一** —— 需要二次收敛

---

## 4. 实现层问题（代码质量，不一定是业务问题）

### 4.1 输出模式重复（38 处 `match ctx.mode`）

每个命令都写三段：

```rust
match ctx.mode {
    crate::output::OutputMode::Json => { println!("{}", serde_json::to_string_pretty(&...)?); }
    crate::output::OutputMode::Porcelain => { /* 手写 tab 分隔 */ }
    crate::output::OutputMode::Human => { /* 手写表格 */ }
}
```

`src/output.rs` 已经有 `emit()` / `success()` / `info()` 但**几乎没有 callsite
用了**——`task.rs`、`vertex.rs`、`important.rs`、`conflict.rs`、`daemon.rs`
全部自己写 `match`，导致：
- schema 漂移：每个命令的 porcelain 字段顺序不同
- warning 信息只在 Human 模式打印
- `--json` 与 `--porcelain` 同时传时**已被 `output.rs:24-28` 明确：porcelain 胜**，
  但**没有任何命令文档化这一点**

### 4.2 输出写到 stdout（不分离 stdout / stderr）

`04b § 5.6` 锁定：
> `--json` 模式下：JSON 对象走 **stderr**（便于 shell pipeline），stdout 为空

但当前所有 `println!` 都走 stdout。**AI Agent 在 pipe 模式下会同时收到 JSON +
普通成功消息，无法直接解析**。

### 4.3 `commands/mod.rs` 的 `Ctx` 设计偏弱

```rust
pub struct Ctx {
    pub mode: OutputMode,
    pub verbose: bool,
    pub project_root: Option<PathBuf>,
}
```

**缺**：
- `output: Box<dyn Write>` —— 没法在测试里 mock stdout
- `color: bool` —— 全局 no-color 在哪里管
- `progress: Option<ProgressBar>` —— 长任务可视化
- `format: Format`（含 json / porcelain / human 二级参数）—— `--no-header`
  / `--fields id,state` / `--ts` 等

→ 当前实现够用，但**加任何新功能都要再改 Ctx**。建议在动手开发前**先把 Ctx
扩展一次到位**。

### 4.4 错误模型不够丰富

`KronError` 现有 14 个变体。**业务错误 vs 系统错误混在一起**：

- `Cli(String)` —— 业务（参数错）
- `Io(...)` —— 系统
- `NotFound(...)` —— 业务（资源不存在）
- `NotAProject(...)` —— 业务
- `Json(...)` —— 系统

→ 退出码映射表需要从 variant → exit_code 的**显式映射**，但当前 `main.rs`
没做。

### 4.5 `task attach` 把 metadata 写到 `task.body`（JSON 块）

```rust
// task.rs:617-660
fn attach_task_cmd(...) {
    // 把 key=value 解析为 serde_json::Map
    // 然后用 serde_json::to_string_pretty 写进 task.body
    task.body = serde_json::to_string_pretty(&merged_json)?;
}
```

**问题**：requirements § 2.3.1 锁定 body 是**纯 Markdown**。`attach` 把 JSON
塞进去破坏了这个不变量。**用户用 Typora 打开会看到一坨 JSON**。

---

## 5. 标准化的几个具体建议（不展开方案）

按优先级排序：

### 🟥 P0（必须先做）

1. **"state vs vertex" 语义裁决**：要么
     - (A) `TaskState` 锁回枚举（todo/doing/done），自定义 vertex 是另一层
     - (B) 完全废弃 `TaskState`，把 vertex 当成 dynamic container
   二者必选其一。**我倾向 (A)**——与 requirements § 2.2.7 的"state 仅 GUI 维护"对齐。

2. **路径参数语义统一**：
     - `kron task add <vertex> --title ...` 的第一个参数叫 `vertex`，**但**
       `kron vertex create <name>` 也叫 `<name>`
   → 命名一致即可，或在 `task add` 里**禁止**创建 vertex，必须先 `vertex create`

3. **`-m` 语义统一**：参考 `04b § 10 Q23` 已经定 A 方案，但**实现里没遵守**
     - `task add -m X` 写 body
     - `task describe -m X` 覆盖 description
     - `vertex create -m X` 写 description
     - `vertex describe -m X` 覆盖 description
   → **建议全部统一为 `--description` / `--body`**；`add` 时必填一项。

### 🟧 P1（业务逻辑补完）

4. **`task undo` / `important undo`** —— Git 有 `git reset`，Kron 没有
   （requirements 没列，但**这是体验巨大缺口**）

5. **退出码映射** —— 实施路线图已标 v2 工作。**做一次到位**

6. **`important remove` 的语义** —— "不删源文件"是反直觉的，建议：
     - 默认行为符合用户直觉（提示 + 删除源文件 + 备份）
     - `--keep-source` 才是显式 flag

### 🟨 P2（实现层清理）

7. **stdout / stderr 分离** —— `--json` 走 stderr
8. **`match ctx.mode` 收敛** —— 用 `output.rs::emit()` 统一
9. **`task attach` 移除** —— 改用专用子命令或写到 frontmatter
10. **`Ctx` 一次性扩展** —— 加 `color`, `output`, `format` 字段

---

## 6. 与 Git 对齐的可借鉴清单

参考 Git 的设计哲学：
- **删除很重，恢复很容易**（reflog, reset, revert, stash）
- **命令动词精确**（`add` / `mv` / `rm` 是不同语义）
- **配置覆盖有明确层级**（local > global > system）
- **`--porcelain` 是给机器的契约**（v1/v2 锁定）
- **`-m` 一致语义**

Kron 现在的痛点：
- 删除很重，**没有恢复**
- 命令动词在 task / vertex 之间**互相借用**
- 配置刚刚做好（`kron config`）✅
- porcelain schema 没锁定
- `-m` 跨命令漂移

---

## 7. 评审结论：下一步行动建议

**我的判断**：不要急于加新功能，先解决"语义裁决 + 标准化"。

### 推荐优先级（建议作为下一阶段 Anchor #8 的输入）

1. **裁决 state vs vertex 语义**（一次性决策，影响 ~30% 代码）
2. **`-m` / `--description` / `--body` 收敛**（一次性 PR，~15 处）
3. **退出码映射表**（一次性 PR，~50 行 + 100 行测试）
4. **stdout / stderr 分离**（一次性 PR）
5. **统一 `match ctx.mode`**（一次性 PR，~38 处）
6. **加 `task undo`**（新功能，但补业务缺口）
7. **加 `important undo`**（同上）
8. **修复 `task attach`**（移除或重写到 frontmatter）

每项都是独立可合 PR，**不会破坏现有测试**（除非 1）。

**Anchor #8 的事务定义建议**：**"语义裁决 + 标准化第一轮"**
- 不加新功能
- 不加新命令
- 只做**减法 + 重命名**
- 通过的标志是 `04b § 1` 的 7 个设计原则**全部能勾选**，且 dev-journal 写
  明白"为什么这样定"。

---

## 8. 待讨论的开放问题（请你定）

1. **`TaskState` 枚举要保留吗？**（state vs vertex 二选一）
2. **`task undo` 是否要做？**（用户最关心的体验缺口之一）
3. **`important remove` 默认删源文件吗？**（直觉得 vs 安全）
4. **stdout / stderr 分离是否 v1 必须？**（影响所有命令的输出）
5. **是否引入 `--format json|porcelain|human` 全局 flag 替代 `--json`/`--porcelain`？**
   （与 Git 的 `--format` 对齐，但破坏现有 flag）

---

**文档结束。**

等你的反馈：以上 8 个开放问题怎么定，决定 Anchor #8 的具体范围。