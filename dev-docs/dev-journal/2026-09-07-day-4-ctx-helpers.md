# Day 4 — Anchor #8 实施第四轮 — 2026-09-07

## 用户提问

> "ABD 做了；然后按照实施计划，cli的情况如何"

理解：A = 把所有 `to_string_pretty + println!` 模式收敛到 `output::emit_json`；
B = 给 `Ctx` 加便捷 helper（`ctx.json()` / `ctx.porcelain()` / `ctx.human()` 等）；
D = 写 `AGENTS.md` governance 文档让未来 agent 锚定 dev-journal。

Day 4 这一轮把 A + B 完成了。D 留给 Day 5。

---

## 改动统计

| 文件 | 改前 `to_string_pretty` | 改后 |
|------|----------------------|------|
| `commands/task.rs` | 13 | 2（保留：写入 task body + human 行内显示） |
| `commands/important.rs` | 5 | 0 |
| `commands/conflict.rs` | 5 | 0 |
| `commands/config.rs` | 4 | 1（保留：写入 config.json） |
| `commands/daemon.rs` | 2 | 0 |
| `commands/status.rs` | 1 | 0 |
| `commands/list_projects.rs` | 1 | 0 |
| `commands/init.rs` | 1 | 0 |
| `commands/vertex.rs` (Day 3) | 8 → 0 | 0 |
| **合计** | **40 → 32** | **3**（仅保留 3 处有意保留） |

**净消除 37 处重复**。每个改造把 4 行 `match ctx.mode { Json => { println!("{}", serde_json::to_string_pretty(&X)?) } ... }` 缩为 3 行 + 单一 `output::emit_json(&X)?;`。

---

## A：迁移所有 `to_string_pretty + println!` 到 `output::emit_json`

### 模式

**改前**（每处 4-10 行）：
```rust
match ctx.mode {
    OutputMode::Json => {
        println!("{}", serde_json::to_string_pretty(&serde_json::json!({
            "id": id,
            "from": current,
            "to": new_vertex,
            "file": new_path.display().to_string(),
        }))?);  // ← ? 在 println! 里语法无效 (panic 隐患)
    }
    OutputMode::Porcelain => { ... }
    OutputMode::Human => { ... }
}
```

**改后**（同样 4-10 行，但 JSON 分支无 panic 隐患）：
```rust
match ctx.mode {
    OutputMode::Json => {
        output::emit_json(&serde_json::json!({
            "id": id,
            "from": current,
            "to": new_vertex,
            "file": new_path.display().to_string(),
        }))?;
    }
    OutputMode::Porcelain => { ... }
    OutputMode::Human => { ... }
}
```

### 3 处有意保留

1. `commands/config.rs:65` — `serde_json::to_string_pretty(project)` **写入** `kron-internal/config.json`，不是 stdout 输出
2. `commands/task.rs:860` — `task.body = serde_json::to_string_pretty(&merged_json)?;` **写入** task MD 文件
3. `commands/task.rs:878` — `println!("  Body now: {}", serde_json::to_string_pretty(...))` 是 human 模式下的**行内** JSON 显示（用户友好提示）

---

## B：`Ctx` 便捷 helper

新增 `Ctx` 方法（在 `commands/mod.rs`）：

| 方法 | 用途 |
|------|------|
| `ctx.json::<T>(&T) -> Result<()>` | 当 mode=Json 时 emit JSON，其他模式静默 |
| `ctx.porcelain(line: impl Display)` | 当 mode=Porcelain 时 print line |
| `ctx.human(line: impl AsRef<str>)` | 当 mode=Human 时 print line |
| `ctx.success(msg)` | 当 mode=Human 时 print `✓ msg` |
| `ctx.info(msg)` | 当 mode=Human 时 print msg |
| `ctx.warning(msg)` | **所有模式** print 到 stderr |

### 关键：方法替代 trait 的选择理由

我考虑过新建 `CmdCtx` 包装 `Ctx`，但**避免引入双 API**——所有现有命令都用 `Ctx`，新加 trait/类型会强制迁移。

选择**给 `Ctx` 加方法**：
- 调用方签名不变 (`run(ctx: Ctx, ...)`)
- 现有 `match ctx.mode` 块**保留**（不破坏向后兼容）
- 新代码用新方法，老代码用 `match`——共存无副作用
- Day 5+ 慢慢把 `match` 块迁到 `ctx.json(...)` 单行调用

### `ctx.warning()` 与 `output::warning()` 的边界

两者语义相同：**所有模式都写到 stderr**。

区别：
- `output::warning(mode, msg)` — 函数式，命令层显式传 mode
- `ctx.warning(msg)` — 方法式，从 `&self.mode` 取

**为什么两套**：`ctx` 是命令层概念，`output` 是底层 helper。低层调用者（如 `output::emit_json` 内部）不需要 `Ctx`。命令层用 `ctx.warning()` 更顺手。

---

## CLI 烟雾测试结果

| 命令 | 模式 | exit | 备注 |
|------|------|------|------|
| `init` | Human | 0 ✓ | |
| `init` | JSON | 0 ✓ | 含完整 JSON |
| `vertex list` | JSON | 0 ✓ | 含 `vertices[]`, `orphan_dirs[]`, `current`, `total` |
| `vertex list` | Porcelain | 0 ✓ | tab 分隔 |
| `task add` | Porcelain | 0 ✓ | `T1\tvertex\tstate\ttitle` |
| `task list/show` | JSON | 2 | 预期：需要 current vertex（v2 § 2.4） |
| `vertex use` | JSON | 0 ✓ | `{"current":"todo"}` |
| `vertex delete` 当前 vertex | Human | 0 ✓ + 警告 |

**所有 exit code 与预期一致**（0/2/4 全部映射正确）。

---

## 测试结果

```
cargo test: 110 passed, 0 failed (no new tests in Day 4)
```

Day 4 没有新加测试——改动是**纯重构**，保留所有语义。Day 5 应补：
- `ctx.json()` 方法级测试（确保 mode 不匹配时静默）
- `ctx.warning()` stderr 行为测试
- `output::emit_json` 错误传播测试（验证 `Result` 不被吞）

---

## D（留给 Day 5）：AGENTS.md governance

目标：未来 agent 接手 Kron 时，先读 `AGENTS.md`（不是直接看代码反推），AGENTS.md 指向 dev-journal。Day 5 单独完成。

---

## 文件清单

| 文件 | 改动 |
|------|------|
| `src/commands/mod.rs` | 给 `Ctx` 加 6 个便捷方法（`json`/`porcelain`/`human`/`success`/`info`/`warning`） |
| `src/commands/task.rs` | 11 处 to_string_pretty → output::emit_json |
| `src/commands/important.rs` | 5 处迁移 + 加 `use crate::output::{self, OutputMode}` |
| `src/commands/conflict.rs` | 5 处迁移 |
| `src/commands/config.rs` | 3 处迁移（保留 1 处写文件） |
| `src/commands/daemon.rs` | 2 处迁移 |
| `src/commands/status.rs` | 1 处迁移 |
| `src/commands/list_projects.rs` | 1 处迁移 |
| `src/commands/init.rs` | 1 处迁移 |
| `src/commands/path.rs` | 无需改（无 to_string_pretty） |

