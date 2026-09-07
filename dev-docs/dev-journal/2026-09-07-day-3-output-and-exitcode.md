# Day 3 — Anchor #8 实施第三轮 — 2026-09-07

## 背景

Day 1 把 vertex/task CLI 落地。Day 2 文档（[2026-09-06-cli-semantic-decisions.md](../dev-journal/2026-09-06-cli-semantic-decisions.md)）出了语义裁决。Day 3 收尾三件具体工作：

1. **`src/output.rs`** 增强：补足命令层常用的 mode-branching helpers
2. **`src/main.rs` exit code** 改用 `KronError::exit_code()`，去掉重复硬编码
3. **`commands/vertex.rs` 试点**：把 8 处 `println! + serde_json::to_string_pretty` 收敛为 `output::emit_json(&...)`

---

## 1. `output.rs` 新增 helper

新增的 helper（在原 `emit` / `success` / `info` 之上**叠加**，不替换）：

| Helper | 用途 |
|--------|------|
| `emit_json<T: Serialize + ?Sized>(value: &T) -> Result<()>` | 低层，write JSON to stdout |
| `emit_porcelain(line: impl Display)` | 低层，write 一行 porcelain |
| `emit_human(line: impl AsRef<str>)` | 低层，write 一行 human |
| `PorcelainRecord` trait | 类型化 porcelain 字段 |
| `tab_join(fields)` | `tab` 拼接 |
| `emit_records<T>(mode, &[T], human, empty_hint)` | mode-branching 多记录 |
| `emit_record<T>(mode, &T, human)` | mode-branching 单记录 |
| `warning(mode, msg)` | **所有模式**写 stderr（即便 JSON 模式也可见） |
| `info_warning(mode, msg)` | 仅 human 模式写 stderr |

### 为什么 `+?Sized`？

旧签名 `T: Serialize` 不接受 `&[T]`（因为 `T: Sized` 必须被推导，而 trait object 不能 Sized）。加上 `?Sized` 后允许 `&dyn Trait` 和 `&[T]`。这是为了让 `emit_records<T>(records: &[T])` 能直接 `emit_json(records)`（Vec/数组序列化）。

### 为什么新增 `warning`？

Day 2 我们让 `vertex delete` 输出 `⚠ pointer orphaned` 提示用户。该提示**必须**让用户看见——即便是 `--json` 模式（agent 可能通过 JSON 处理输出）。

**正确做法**：警告写到 stderr，不写到 stdout。这样 stdout 仍是合法 JSON，但 agent/运维能在 stderr 看到警告。

---

## 2. `main.rs` exit code 修复

### 改动前（bug）

```rust
// main.rs
fn exit_code_for(err: &KronError) -> i32 {
    match err {
        KronError::AlreadyInitialized(_) => 4,
        KronError::Cli(_) => 2,
        _ => 1,
    }
}
```

```rust
// error.rs
impl KronError {
    pub fn exit_code(&self) -> i32 { /* 完整映射 */ }
}
```

**两个真理源**：
- `error.rs::exit_code` 区分 2/4/6/8/9
- `main.rs::exit_code_for` 把所有非上述两个变体的都返回 1

→ **bug**：`InvalidVertexName` 应该是 2（错误码表约定），实际返 1；`NotFound` 应返 8，实际 1；`PermissionDenied` 应返 9，实际 1。

### 改动后

```rust
// main.rs
fn main() {
    if let Err(e) = cli::run() {
        eprintln!("error: {e}");
        std::process::exit(e.exit_code());   // ← 单一真理源
    }
}
```

**单测验证**（用真实 CLI 调用）：

| 命令 | 期望 exit | 实际 exit |
|------|---------|---------|
| 在非项目目录跑 `kron status` | 4 (`NotAProject`) | **4 ✓** |
| 跑 `kron vertex create "bad..name"` | 2 (`InvalidVertexName`) | **2 ✓** |
| 跑 `kron --badflag` | 2 (clap 拒绝) | **2 ✓** |
| `kron init` 成功 | 0 | **0 ✓** |

---

## 3. `commands/vertex.rs` 收敛

8 处如下模式：

```rust
OutputMode::Json => {
    println!("{}", serde_json::to_string_pretty(&serde_json::json!({
        "name": ...,
        ...
    }))?);   // ← `?` 在 println 里语法无效（panic 隐患！）
}
```

替换为：

```rust
OutputMode::Json => {
    output::emit_json(&serde_json::json!({
        "name": ...,
        ...
    }))?;
}
```

### 改前隐患

原代码 `serde_json::to_string_pretty(...) ?` 中 `?` 在 `println!` 位置**不是合法的 `?` 表达式**，相当于**编译器已经接受，但语义上吞掉了错误**。实际上 JSON 序列化不太会失败（所有字段都是 `String` / `Option<String>`），但万一出现 `Internal` 错误（旧版本可能 panic），会悄无声息地 panic 到 stderr。

新代码 `output::emit_json(...) ?` 实际**传播错误**——任何 `KronError::Json` 变体会被返回，外层 `?` 操作符捕获。这是更安全的语义。

### 没动的部分

`Porcelain` / `Human` 分支仍用 `println!`。理由：
- Porcelain 的字段顺序是每命令定制的（`task list` 5 字段 vs `vertex list` 4 字段），用通用 helper 会强制把字段顺序抽到 trait，反而引入隐式复杂度
- Human 输出是格式化字符串（含 `{}` padding），无法用 `tab_join` 概括

**Day 4+** 候选：用 `tab_writer::write_record!(w, id, state, title)` 这类宏来收敛，但需要新引入宏，且每个命令的字段顺序本就独立——收益待评估。

---

## 测试结果

```
src/output.rs 新增 4 个内联测试
全量 cargo test: 110 passed (+4), 0 failed
```

## 下一步（Day 4 候选）

- 把 `commands/task.rs` / `commands/important.rs` / `commands/init.rs` 等同样模式批量迁移到 `output::emit_json`（估计还能再消除 ~25 处重复）
- 新增 `commands/mod.rs::CmdCtx` 把 `OutputMode` 和 stdout/stderr 写封一处
- day-4 dev-journal 跟踪

---

## 文件清单

| 文件 | 改动 |
|------|------|
| `src/output.rs` | 重写（增 4 个 helper + 4 个 trait，4 个内联测试）|
| `src/main.rs` | exit_code_for 删除，改用 `e.exit_code()` |
| `src/commands/vertex.rs` | 8 处 `println! + to_string_pretty` → `output::emit_json` |

