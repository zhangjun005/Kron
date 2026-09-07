# 2026-09-06 — 项目根查找与祖先守卫（Anchor #6）

> **事务范围**：补一个核心缺失能力 —— **祖先向上扫描**（Git 风格）。
> 同时承担"不变量"角色：**消除 cwd 严格锚定的脆弱性**，把"找项目根"从 6 处 callsite 的
> 重复样板代码，**收敛为单一 helper**。

---

## 1. 触发原因

用户在打开当前 IDE 编辑 `src/model.rs` 时，处于 `E:\works\Kron\src\`。
此时任何 `kron` 子命令都会**失败**：

```
$ cd E:/works/Kron/src
$ kron task list todo
Error: E:\works\Kron\src is not a Kron project
```

原因：当前所有需要"找项目根"的命令（6 处 callsite）都是：

```rust
let cwd = std::env::current_dir()?;
if !cwd.join("kron-internal").join("config.json").exists() {
    return Err(KronError::NotAProject(cwd));
}
Ok(cwd)
```

—— **只查 cwd 自己，不向上**。

参考 Git 的实现：向上遍历直到找到 `.git` 目录。Kron 应当遵循同一契约。

---

## 2. 设计目标

| 目标 | 说明 |
|------|------|
| **G1** | 在项目任意子目录运行 `kron` 命令都应工作（仿 Git） |
| **G2** | `kron init` 在已存在祖先项目时必须拒绝（仿 Git "nested git" 错误） |
| **G3** | 提供**单一权威 helper**，所有 callsite 共用 |
| **G4** | 行为可在不破坏测试的前提下增量迁移 |
| **G5** | 不破坏当前 60/60 既有测试 |
| **G6** | 行为可解释（`--porcelain` / `--json` 输出能说明选哪个根） |

---

## 3. 影响面盘点

### 3.1 受影响 commands（6 处现状代码）

| 命令 | 文件 | 行号 | 现状 |
|------|------|------|------|
| `task` | `src/commands/task.rs` | 127-131 | `cwd` 严格锚定 |
| `conflict` | `src/commands/conflict.rs` | 108-112 | 同上 |
| `vertex` | `src/commands/vertex.rs` | 69-73 | 同上 |
| `important` | `src/commands/important.rs` | 81-85 | 同上 |
| `daemon` | `src/commands/daemon.rs` | 59-63 | 同上 |
| `config` | `src/commands/config.rs` | 54-58 | 同上 |
| `init` | `src/commands/init.rs` | 43-48 | `AlreadyInitialized(cwd)` 只查 cwd |
| `status` | `src/commands/status.rs` | 50 | 直接用 cwd，无守卫 |
| `path` | `src/commands/path.rs` | 31 | 直接用 cwd，无守卫 |
| `list` | `src/commands/list_projects.rs` | — | stub，vec![] |

**10 个文件受牵连**。

### 3.2 受影响测试

- `tests/init_layout` — 5/5 ✓ 需补 ancestor-nesting 用例
- `tests/cli_p4` — 17/17 ✓ 子目录行为可能因本次改动改变
- 其他测试照旧（它们都 `cd()` 到项目根后跑）

---

## 4. 设计方案

### 4.1 新增 helper 模块

文件：`src/core/project.rs`（新文件）

```rust
//! Locate the project root from an arbitrary starting path.
//!
//! Mirrors Git's ancestor search contract:
//!   1. Walks upward from `start` looking for `kron-internal/config.json`.
//!   2. Stops at the first match; otherwise returns `None` at FS root.
//!   3. `find_ancestor_project` walks further to detect ancestor projects
//!      (used by `kron init` to refuse nested initialization).
//!
//! Centralizing these rules guarantees every subcommand shares one
//! definition of "what is a Kron project, and where is it rooted?".

use std::path::{Path, PathBuf};

/// Marker file identifying a Kron project root.
pub const PROJECT_MARKER: &str = "kron-internal/config.json";

/// Walk upward from `start` to find the nearest ancestor that contains
/// `kron-internal/config.json`. Returns `None` if no project is found
/// before reaching the filesystem root.
///
/// Note: `start` is included in the search; passing a project root is fine.
pub fn find_project_root(start: &Path) -> Option<PathBuf> {
    let mut cur: Option<&Path> = Some(start);
    while let Some(dir) = cur {
        if dir.join(PROJECT_MARKER).is_file() {
            return Some(dir.to_path_buf());
        }
        cur = dir.parent();
    }
    None
}

/// Walk upward from `start` (excluding `start` itself) to detect if any
/// ancestor directory already hosts a Kron project. Used by `init` to
/// reject nested initialization.
///
/// Returns `Some(ancestor_path)` on conflict, `None` otherwise.
pub fn find_ancestor_project(start: &Path) -> Option<PathBuf> {
    let mut cur = start.parent()?;
    loop {
        if cur.join(PROJECT_MARKER).is_file() {
            return Some(cur.to_path_buf());
        }
        match cur.parent() {
            Some(parent) if parent != cur => cur = parent,
            _ => return None,
        }
    }
}

/// Resolve the current Kron project root. Convenience wrapper around
/// `find_project_root` using `std::env::current_dir()`.
///
/// Returns `Err(KronError::NotAProject(cwd))` if no project is found.
pub fn current_project_root() -> Result<PathBuf, crate::error::KronError> {
    let cwd = std::env::current_dir().map_err(crate::error::KronError::Io)?;
    find_project_root(&cwd).ok_or(crate::error::KronError::NotAProject(cwd))
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::fs;

    #[test]
    fn finds_root_at_start() {
        let tmp = tempdir();
        fs::create_dir_all(tmp.join("kron-internal")).unwrap();
        fs::write(tmp.join("kron-internal/config.json"), "{}").unwrap();
        assert_eq!(find_project_root(&tmp), Some(tmp.clone()));
    }

    #[test]
    fn walks_upward() {
        let tmp = tempdir();
        fs::create_dir_all(tmp.join("kron-internal")).unwrap();
        fs::write(tmp.join("kron-internal/config.json"), "{}").unwrap();
        let deep = tmp.join("src").join("commands");
        fs::create_dir_all(&deep).unwrap();
        assert_eq!(find_project_root(&deep), Some(tmp.clone()));
    }

    #[test]
    fn none_when_no_project() {
        let tmp = tempdir();
        let deep = tmp.join("a").join("b");
        fs::create_dir_all(&deep).unwrap();
        assert_eq!(find_project_root(&deep), None);
    }

    #[test]
    fn ancestor_detected_excluding_self() {
        let tmp = tempdir();
        fs::create_dir_all(tmp.join("kron-internal")).unwrap();
        fs::write(tmp.join("kron-internal/config.json"), "{}").unwrap();
        let child = tmp.join("child");
        fs::create_dir_all(&child).unwrap();
        // child has no marker, but its parent does
        assert_eq!(find_ancestor_project(&child), Some(tmp.clone()));
        // self is excluded
        assert_eq!(find_ancestor_project(&tmp), None);
    }
}
```

### 4.2 迁移 callsite（统一改写）

**通用模板**（每个受影响的 commands/* 重复此修改）：

```rust
// OLD:
let cwd = std::env::current_dir()?;
if !cwd.join("kron-internal").join("config.json").exists() {
    return Err(KronError::NotAProject(cwd));
}
Ok(cwd)

// NEW:
let root = core_project::find_project_root(&std::env::current_dir()?)
    .ok_or_else(|| {
        let cwd = std::env::current_dir().unwrap_or_default();
        KronError::NotAProject(cwd)
    })?;
Ok(root)
```

**或者更简洁**：所有 commands 文件直接调 `core_project::current_project_root()?`。

### 4.3 `init` 加 ancestor guard

```rust
// OLD (commands/init.rs:43-48):
let cwd = std::env::current_dir().map_err(KronError::Io)?;
let prep = core_init::prepare(&cwd, args.no_git)?;
if kron_dir.exists() && !args.force {
    return Err(KronError::AlreadyInitialized(cwd));
}

// NEW:
let cwd = std::env::current_dir().map_err(KronError::Io)?;
if let Some(ancestor) = core_project::find_ancestor_project(&cwd) {
    return Err(KronError::AncestorProject {
        attempted: cwd,
        ancestor,
    });
}
if kron_dir.exists() && !args.force {
    return Err(KronError::AlreadyInitialized(cwd));
}
```

### 4.4 错误模型扩展

`src/error.rs` 新增一个变体：

```rust
/// Initialization refused because an ancestor directory is already
/// a Kron project (mirrors Git's "fatal: cannot nest" semantic).
#[error(
    "cannot initialize Kron project at `{attempted}`: \
     an ancestor project already exists at `{ancestor}`"
)]
AncestorProject {
    attempted: PathBuf,
    ancestor: PathBuf,
},
```

### 4.5 `Ctx` 升级：把 `project_root` 收归 Ctx（顺手做）

### 现状（问题）

```rust
// commands/mod.rs
pub struct Ctx {
    pub mode: OutputMode,
    pub verbose: bool,
}
```

`Ctx` 没有 `project_root`。每个命令自取 `require_project()?`，**全项目 32 处重复**。

### 新结构

```rust
// commands/mod.rs
pub struct Ctx {
    pub mode: OutputMode,
    pub verbose: bool,
    /// Resolved Kron project root, found by `core::project::find_project_root`.
    /// `None` means "not yet resolved — call `Ctx::resolve()` or use
    /// `commands::*::require_project_root(&ctx)?"`."
    pub project_root: Option<PathBuf>,
}

impl Ctx {
    pub fn new(mode: OutputMode, verbose: bool) -> Self {
        Self { mode, verbose, project_root: None }
    }

    /// Resolve and cache the project root from the current working directory.
    pub fn resolve(&mut self) -> Result<&Path> {
        if self.project_root.is_none() {
            let cwd = std::env::current_dir()?;
            self.project_root = core::project::find_project_root(&cwd);
        }
        self.project_root
            .as_deref()
            .ok_or_else(|| /* NotAProject */)
    }
}

/// Helper used by every command: returns `ctx.project_root.clone()` or
/// resolves it on the fly. Thin wrapper to keep callsites concise.
pub fn require_project_root(ctx: &Ctx) -> Result<PathBuf> {
    ctx.project_root.clone().ok_or_else(|| /* NotAProject */)
}
```

### 迁移

- 6 个 callsite 的 `require_project() / project_root() / project_path()` 函数
  **整体删除**
- 32 个 callsite 改为：
  - 头部 1 次 `require_project_root(&ctx)?`
  - 函数体直接用 `ctx.project_root.as_ref().unwrap()`（或封装 helper）
- `init` / `path` / `status` / `list_projects` 不用 `Ctx.project_root`（它们本身就是
  定义 project_root 的命令；`init` 调用 `find_ancestor_project`，`path`/`status` 直接
  走 cwd 或 `find_project_root`）

### 影响面

| 变化 | 行数 |
|------|------|
| 新增 `Ctx::resolve` + `require_project_root` helper | +20 |
| 删除 6 个文件内的 `require_project()` / `project_root()` / `project_path()` 函数 | -42 |
| 替换 32 个 callsite 为 `require_project_root(&ctx)?` | 净 ±0 |
| 改 `commands::run` 入口调一次 `ctx.resolve()` | +3 |
| **净影响** | **-20 行** + 32 处分散 → 1 处集中 |

---

## 4.6 错误信息修正

`KronError::NotAProject` 当前消息说"missing KRON/ directory"，但代码检查的是
`kron-internal/config.json`。修正为：

```rust
#[error("not a Kron project: {0} (no `kron-internal/config.json` found in this directory or any parent)")]
NotAProject(PathBuf),
```

---

## 5. 与既有架构的一致性自检

| 维度 | 既有架构 | 本次改动 | 一致？ |
|------|---------|---------|--------|
| **核心层位置** | `src/core/{init,task,vertex,important,sync,daemon,conflict}.rs` | 新增 `src/core/project.rs` | ✅ 一致（核心层扩展） |
| **commands 调用模式** | `commands/*.rs` 调 `core::*` | 同上 | ✅ |
| **错误传播** | `KronError` enum + `Result<T>` | 新增 `AncestorProject` 变体 | ✅ |
| **配置驱动** | `kron-internal/config.json` | `PROJECT_MARKER = "kron-internal/config.json"` 复用 | ✅ |
| **测试组织** | `tests/` 集成测试 + `src/**` 单测 | 新 helper 带 unit test，集成测补 2 个 | ✅ |
| **JSON/Porcelain 输出** | 全命令支持 | `--json` 输出含 `selected_root` / `ancestor` 字段 | ✅ |
| **平台兼容** | Windows 路径用 `Path` API | 复用 PathBuf API，跨平台 | ✅ |

**结论**：与既有架构完全契合，未引入新分层或新依赖。

---

## 6. 潜在架构风险（自检）

### 6.1 风险：循环符号链接导致死循环

`Path::parent()` 在符号链接上**不**解析。Unix 上有 mount point 概念。
**现状**：`pop()` 在 root 会返回 None，循环终止 → ✅ 安全。
**补强**：future work 可加 symlink visit 计数。

### 6.2 风险：把别人的 `kron-internal/config.json` 误识别

如果用户在 `~/.config/some-tool/kron-internal/` 这种**非项目**目录里恰好有同名文件，
向上扫描会把它当项目。

**防御**：
- 必须 `kron-internal/config.json` 是**有效 JSON** 且包含 `schema_version` 字段 → 加 JSON 校验
- 或：除 marker 外还要校验同级有 `kron-internal/vertices.json`（双文件 guard）

**建议**：本次先加 JSON 解析校验（最简单）：

```rust
fn is_valid_marker(dir: &Path) -> bool {
    let p = dir.join(PROJECT_MARKER);
    if !p.is_file() { return false; }
    std::fs::read_to_string(&p)
        .ok()
        .and_then(|s| serde_json::from_str::<serde_json::Value>(&s).ok())
        .map(|v| v.get("schema_version").is_some())
        .unwrap_or(false)
}
```

### 6.3 风险：`init --force` 与 ancestor guard 冲突

`--force` 标志允许重初始化 cwd 自身——但**祖先重初始化**应该不允许。
**决策**：ancestor guard 不被 `--force` 绕过。

### 6.4 风险：Windows 长路径

Windows `MAX_PATH` 260 字符限制。`pop()` 不会破，但显示错误信息可能截断。
**对策**：错误信息用 `display()` 不带前缀 `\\?\`（够用）；长路径 PR 留给未来。

### 6.5 风险：现有测试假设 cwd = 项目根

`tests/cli_p4.rs` / `tests/init_layout.rs` 用 `tempfile::tempdir()` + 直接调 cargo
binary，**不是从外部 cd 进去**——CWD 由测试 runner 控制。
**对策**：跑全测，**任何 fail 都必须现场评估**。**不允许**修改既有测试以让它们通过。

### 6.6 风险：相对路径 / 软链接差异

`std::env::current_dir()` 在 Windows 上**总是绝对路径**（返回 `UNC` 或 `drive:\\`），
无歧义。Unix 上可能是相对路径 → 用 `std::path::absolute()`（Rust 1.79+ 有此 API）。

### 6.7 风险：性能

每命令向上扫描 N 层。在 monorepo 极深时（30+ 层）理论上 O(N) 文件存在性检查。
**对策**：常用 shell 启动开销里可忽略；不需要 cache。如果真成为问题，未来加
`KronProject` 环境变量 bypass（仿 git 的 `GIT_DIR`）。

---

## 7. 验收准则

- [ ] `cd project/src && kron task list todo` 返回项目任务清单（不再报 NotAProject）
- [ ] `cd project-root && kron init`（已有祖先项目）报错 `AncestorProject` 退出码非 0
- [ ] `cd project-root && kron task list todo` 行为不变（向后兼容）
- [ ] `kron status` 在 `cd project/src` 时仍输出正确项目状态
- [ ] `cargo test` 60/60 既有测试 + 至少 4 新 helper unit test 全绿
- [ ] `--json` 输出在两种路径下都可机读
- [ ] `kron help` 文本无变化（公共 CLI 不变）

---

## 8. 不在本次事务范围

- `kron list` 的真实全局项目注册表（`~/.kron/projects.json`）——独立事务
- `kron init --mode symlink` 在 Windows 上的实际效果——已知 stub
- `kron daemon` 真后台进程——已知 stub

---

## 9. 关联索引

- 触发问题：用户提出"在不同项目间应该做了隔离吧" + "向上扫描就行"
- 涉及 issue：暂无 GitHub issue（本地 dev）
- 设计参考：Git 的 `setup.c` 中 `is_git_directory()` / `find_git_dir()` 行为
- 上次同类事务：Anchor #5（config 子命令重构，详见 commonQuestions.md）
