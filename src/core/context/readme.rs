//! `README.md` index generator.
//!
//! Produces the context index with explicit scope boundaries.

/// Generate the `README.md` content.
pub fn generate() -> std::result::Result<String, crate::error::KronError> {
    let now = chrono::Utc::now();
    let header = format!(
        "<!-- kron-generated: {} -->\n<!-- DO NOT EDIT. Run 'kron context --regenerate' to refresh. -->\n",
        now.to_rfc3339()
    );

    let body = r#"# .kron-context/

Kron 自动生成的事实型结构快照（**仅包含可重现的事实，不包含语义理解**）。

## 范围边界

本目录**不包含**：
- 项目是干啥的（看项目根 README.md）
- 架构设计（看 `KRON/important/` 下的文档）
- 代码约定（看 `KRON/important/conventions.md`）
- "为什么这么实现"——没有，请读源码

本目录**包含**：
- 最近 commit 列表（`git log`）
- 当前分支与主分支对比
- 项目目录结构树

## 文件清单

| 文件 | 内容 | 生成时机 |
|------|------|---------|
| `git/recent-commits.md` | 最近 100 次 commit | 新 commit 时 |
| `git/branch-summary.md` | 当前分支 vs main | HEAD 变化时 |
| `code/structure.md` | 目录树（深度 3） | 文件增删时 |
| `README.md` | 本文件 | 手动维护 |

## 重新生成

```bash
kron context              # 增量
kron context --regenerate # 全量
kron context --list       # 看状态
```

## 给 AI 工具的提示

不要假设读 `.kron-context/` 就"理解"了项目。
项目语义信息请读：
  1. 根目录 README.md
  2. `KRON/important/` 下的文档
  3. 必要时直接阅读源码
"#;

    Ok(format!("{}{}", header, body))
}
