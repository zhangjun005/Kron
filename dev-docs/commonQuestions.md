# Kron Common Questions

> 经过 2026-09-05 8 次问答后定稿的设计原则摘要。
> 用于实现阶段快速对齐 context，避免反复决策。

---

## 1. 核心定位

> Kron = **Git 仓库内的、给 AI 读的任务跟踪器**。
>
> v1 单用户、单机、不做认证。git 是真理源，Kron 是元数据层。
>
> 与同类核心区别：
> - **vs Notion**：不用数据库（用 MD 文件）+ 必须有 Git
> - **vs Jira**：不替代敏捷工作流（轻量看板）
> - **vs Obsidian**：不做知识图谱（不做 backlink/tag/graph view）
> - **vs Linear**：不替代 GitHub Issues（Git 是 issue 真理源）
> - **vs Taskwarrior**：不替代 GTD（不做优先级算法）

---

## 2. 存储四层

```
<git-repo>/KRON/
├── README.md
├── VERTEX/<v>/              # vertex = 目录（slug）
│   ├── README.md
│   ├── tasks.md             # 看板（人写）
│   └── tasks/<id>.md        # task 详情（人/AI 写）
├── IMPORTANT/<encoded>.md   # 重要文件元数据
└── BINDS/<yyyymmdd>-<v>-<hash>.md   # bind 标记

<git-repo>/.kron-internal/   # ❌ 不进 git
├── vertices.json
├── tasks.json
├── important.json
├── sync-state.json
├── git-graph-cache.json
├── binds.json
└── log/
```

**核心约束**：

- ✅ `KRON/`、`.kronignore` 进 git
- ❌ `.kron/`、`.kron-internal/` 不进 git（`.gitignore` 排除）

---

## 3. ID 体系

| 层 | ID 规则 | 唯一性 |
|----|--------|--------|
| project | abs path | 文件系统唯一 |
| vertex | 目录名 slug | 单仓库内 unique |
| task | `t-<yyyy>-<seq>-<title-slug>.md` | **整仓库** unique |
| important | `<path-encoded>.md`（`/` → `--`） | 路径编码 unique |

**ID 不可变性**：

- ✅ project：仓库可迁路径
- ✅ vertex_id：可 git mv（用 `kron vertex rename` 命令）
- ❌ task_id：**永不**改
- ❌ important_id：**永不**改

---

## 4. Frontmatter 锁死 schema

### Task frontmatter

```yaml
---
id: t-2026-0001-oauth-login
title: 实现 OAuth 登录
state: todo | doing | done | dropped
priority: high | medium | low | none
created_at: 2026-09-05T20:00:00Z
updated_at: 2026-09-05T20:00:00Z
description: |-
  多行可选
---
```

### Important frontmatter

```yaml
---
id: src--auth--login.ts.md
original_path: src/auth/login.ts
original_abs_path: E:/works/Kron/src/auth/login.ts
state: tracked | modified | conflict | missing
description: |-
  为什么这是"重要"
last_sync_at: 2026-09-05T20:00:00Z
---
```

**约束**：

- ❌ **不存 vertex 引用**（物理路径是真理）
- ❌ **不存 custom 字段**（daemon warn，不读）
- ❌ **不存 author / tags / due_date / comments**（v2 再加）

---

## 5. daemon 行为契约

| 操作 | 改什么 | 行为 |
|------|--------|------|
| `kron task update-state` | frontmatter 的 `state` 行 | 原地改 MD |
| `kron task edit` | 打开编辑器 | 用户改啥存啥 |
| `updated_at` / `last_sync_at` | 自动维护 | daemon 写 |
| 索引（tasks.json 等）| `kron-internal/` | 不在 MD 文件里 |
| **校验信息** | ❌ 永不写 | 失败只 warn，不污染 MD |

**MD 文件纯洁度**：

- ✅ daemon 改 frontmatter 必要字段
- ❌ daemon 不写 `## 改动记录`（body 100% 用户领地）
- ❌ daemon 不加 hidden 行（不污染 git diff）
- ❌ daemon 不改 body 内容

---

## 6. bind_point 机制

**核心原则**：bind 不在 MD frontmatter，是独立机制。

| 维度 | 内容 |
|------|------|
| 存哪里 | `KRON/BINDS/<yyyymmdd>-<v>-<hash>.md`（人读）|
| 缓存 | `kron-internal/binds.json`（daemon 索引）|
| 触发 | `kron vertex bind` 命令 → 写 git commit |
| 拓扑距离 | `git rev-list --ancestry-path bindA..bindB --first-parent` |

**为什么独立**：task 文件不应被 bind 标记污染——bind 是 git 层面的锚定，不是任务元数据。

---

## 7. 失败处理哲学

| 失败 | 默认行为 |
|------|---------|
| Git CLI 不可用 | 退化为 `NoopGit`，相关功能静默禁用 |
| MD 文件格式错误 | 保留原文 + warning，继续处理其他文件 |
| task ID 冲突 | **拒绝**创建，明确错误 |
| daemon 崩溃 | GUI 显示"daemon offline"，不阻塞读 MD |
| 冲突未解决 | **保留双源**，不自动决策 |
| 磁盘满 | 写失败才报错（不能假装成功） |

**核心原则**：

- **降级 > 错误**：能降级就降级
- **拒绝 > 沉默**：不能降级就明确拒绝
- **绝不妥协**：用户数据完整性（MD 文件不能被静默损坏）

---

## 8. 核心原则 8 条

| # | 原则 | 含义 |
|---|------|------|
| 1 | **git 是真理源** | MD 文件 + git log 决定一切 |
| 2 | **Kron = 元数据层** | 不存原文件副本，只引路径 |
| 3 | **降级 > 错误** | git 不可用 → NoopGit |
| 4 | **拒绝 > 沉默** | 不能降级就明确拒绝 |
| 5 | **MD = 完整** | daemon 只改 frontmatter，body 100% 用户领地 |
| 6 | **AI = 另一个用户** | AI 走 CLI，不改索引 json |
| 7 | **CLI 是真理接口** | GUI 是 CLI 的可视层，所有数据从 CLI 拉 |
| 8 | **不存 vertex 引用** | 物理路径 = vertex 归属 |

---

## 9. AI 协作边界

| AI 能 | AI 不能 |
|------|--------|
| 读所有 MD 文件、git log、git ref | ❌ 改 Git 配置 |
| 改 MD 文件（task 创建/更新/状态）| ❌ 改 `.kron-config.toml` |
| 改 `KRON/VERTEX/<v>/tasks.md` 看板 | ❌ 改 `kron-internal/*.json` |
| 触发 `kron` CLI（stdio/IPC）| ❌ 触发 daemon 重启 |
| 用自然语言描述 task（写入 description）| ❌ 改 schema（frontmatter 元数据格式）|
| 标记 task 完成/丢弃 | ❌ 接管 daemon 逻辑（AI 不跑 daemon）|
| 提问"我应该 bind 到哪个 vertex？"| ❌ 自动 bind（需要 AI 写进 PR 描述）|

---

## 10. v1 克制（不做清单）

| ❌ 不做 | ❌ 不做 | ❌ 不做 |
|------|------|------|
| 用户系统 / 认证 / 角色 | 子任务嵌套 | 插件系统 |
| 全文搜索 | 时间追踪（番茄钟/工时）| Web 端 / 移动端 |
| 自定义字段 | 实时协作（多人同时编辑）| 云同步 |
| Vertex DAG 可视化 | 任务依赖图 | 看板自动化 rule |
| 双 AI 协作 | 多端 | 子任务 |

**唯一允许的扩展点**：v2/v3 加 **`.kron/config.toml`**（项目级 + 用户级）。

---

## 11. CLI / GUI 分工

| 维度 | CLI | GUI |
|------|-----|-----|
| **功能边界** | 100% 功能 | 只做最常用的 5-6 个视图 |
| **用户群** | AI + 高级用户 | 普通用户 |
| **作用** | 真理性 + 可脚本化 | **引导 + 可视化** |
| **参数稳定性** | 严格（breaking change 必升 major）| 灵活（可改 UI）|
| **错误码** | 严格 | 友好提示 |
| **数据流** | 拉所有数据 | 调 CLI 拉数据 |
| **变更流** | 改 MD + 改索引 | 调 CLI 改 |

**核心约束**：**GUI 关闭时 CLI 必须仍然能用**——daemon 可独立运行。

---

## 12. 性能预算（P99）

| 操作 | 上限 |
|------|------|
| `kron task list --json` | 200ms |
| `kron task show <id>` | 50ms |
| `kron task create` | 300ms |
| daemon 监听 .git/HEAD → 重算 active_vertex | 5s |
| daemon 完整 git-graph 重算 | 30s（5 min cron）|
| GUI 首次启动（含 cold start）| 3s |
| 看板拖拽 → state 改变 → UI 更新 | 100ms（乐观）+ 5s（cron 确认）|
| 文件监听 → MD 改动 → IPC 推送 | 500ms |
| 冲突检测 | 1s（sync 时）|

**硬规则**：**绝不让用户等超过 1 秒的操作不显示进度条**。

---

## 13. 第三方依赖原则

| 原则 | 含义 |
|------|------|
| **最小依赖** | 能 stdlib 就 stdlib |
| **二进制 ≤ 10MB** | v1 硬约束 |
| **启动 ≤ 1 秒** | v1 硬约束 |

**核心选择**：Tokio + notify + clap + serde + chrono + tracing + thiserror + anyhow + inquire

**绝不引入**：git2（体积 + ffi 痛苦）、任何"框架性"crate、大型 ORM

---

## 14. 关键边界案例

### 14.1 重要文件是否存原文件副本？

**❌ 不存副本**——只引路径，原文件由 git 管。

理由：Kron 是 git 仓库的"元数据层"，不抢 git 的真理。

### 14.2 daemon 改 state 的方式？

**原地改 MD frontmatter**（方案 A）——git diff 看得见状态变化。

### 14.3 important.md 的 `## 改动记录` 谁写？

**❌ daemon 不写**——body 100% 用户/AI 领地。

daemon 写自己的 `kron-internal/sync-state.json`。

### 14.4 ID 编码冲突

- important 用 `path-encoded` 作文件名：`<src>/<auth>/<login>.ts` → `src--auth--login.ts.md`
- 边界：windows 大小写不敏感 → `SRC--AUTH` 和 `src--auth` 视为相同
- 边界：路径含 `--` → 双写为 `----`

---

## 15. 关联文档

| 文档 | 行数 | 关系 |
|------|------|------|
| `requirements.md` | 4189 | 需求源头 |
| `00-总览与架构.md` | 577 | 架构总览 |
| `01-数据模型.md` | 1259 | Rust 数据结构 |
| `02-模块划分.md` | 1237 | crate / 模块划分 |
| `03-双源同步机制.md` | 1720 | sync 状态机 |
| `04-守护进程与文件监听.md` | - | daemon 设计 |
| `04b-CLI设计.md` | 1035 | CLI 接口 |
| `05-GUI设计.md` | 720 | GUI 设计 |
| `06-数据格式规范.md` | - | frontmatter schema |
| `07-实施路线图.md` | 689 | 任务分解 |
| **`commonQuestions.md`** | 本文件 | **设计原则速查** |

---

## 版本

- v1.0（2026-09-05）：经过 8 次问答拍板的设计原则。
