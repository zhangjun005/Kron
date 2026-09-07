# Kron CLI 命令一览

> 状态：M2 版本（2026-09-07）
> 来源：`src/cli.rs` + `src/commands/*.rs`
> 注意：已移除 daemon 相关命令（M2 决定零后台进程）

---

## 全局选项

| 选项 | 作用 |
|------|------|
| `-v` / `-vv` / `-vvv` | 逐步增加日志级别（warn → info → debug） |
| `--json` | 输出机器可读的 JSON |
| `--porcelain` | 输出机器可友好的制表符分隔记录 |

输出模式优先级：`--json` > `--porcelain` > 默认（Human）。

---

## `kron init`

初始化当前目录为 Kron 项目。

```
kron init [--force] [--mode symlink|copy] [--no-vertex] [--no-git]
```

| 参数 | 作用 |
|------|------|
| `--force` | 强制重新初始化（覆盖已存在的 `kron-internal/`） |
| `--mode symlink\|copy` | 重要文件的链接方式（Windows 默认 copy） |
| `--no-vertex` | 跳过创建 vertex 目录 |
| `--no-git` | 跳过 Git 仓库检测 |

**行为要点：**
- 拒绝嵌套（在已存在的 Kron 项目内执行会报错 `AncestorProject`）
- 不自动创建 vertex 目录；初始化后需手动 `kron vertex create todo/doing/done`
- 创建 `KRON/`、`kron-internal/`、`kron-internal/config.json` 等

---

## `kron status`

查看项目整体状态。

```
kron status [--watch <seconds>]
```

| 参数 | 作用 |
|------|------|
| `--watch` | 每 N 秒刷新一次（暂未实现，会输出提示） |

**输出字段：** 项目路径、初始化状态、vertex 数量、任务数、important 文件数、待处理冲突数。

---

## `kron list`

列出所有已注册的 Kron 项目（stub 状态，目前总是返回空）。

```
kron list
```

---

## `kron path`

输出内部路径供脚本使用。每次只输出一个路径，无装饰。

```
kron path --kron-root | --important
```

| 参数 | 作用 |
|------|------|
| `--kron-root` | 输出 `kron-internal/` 路径 |
| `--important` | 输出 `KRON/important/` 路径 |

> 两个参数必须且只能传一个。

**示例：**
```bash
cd "$(kron path --kron-root)/.."
```

---

## `kron config`

读写项目配置（持久化到 `kron-internal/config.json`）。

```
kron config get <key>
kron config set <key> <value>
kron config list
```

**可读写配置项：**

| key | 说明 | 类型 |
|-----|------|------|
| `name` | 项目名称 | string（只读） |
| `kron_version` | 创建版本 | string（只读） |
| `project_path` | 项目根路径 | path（只读） |
| `kron_data_path` | internal 路径 | path（只读） |
| `created_at` | 创建时间 | rfc3339（只读） |
| `settings.conflict_threshold_minutes` | 冲突判定阈值 | u32，默认 5 |
| `settings.auto_resolve` | 自动解决策略 | `prompt` / `latest` / `manual`，默认 `prompt` |
| `settings.context_refresh_minutes` | context 刷新间隔 | u32，默认 5 |

---

## `kron vertex`

Vertex 管理（Git-style 方向/阶段）。

```
kron vertex list
kron vertex show <name>
kron vertex create <name> [--description <desc>] [--branch <branch>] [--path <path>]
kron vertex describe <name> --description <desc>
kron vertex branch <name> --set <branch>
kron vertex delete <name> [--force] [--also-remove-dir]
kron vertex use [name] [--unset]
kron vertex current
```

| 子命令 | 作用 |
|--------|------|
| `list` | 列出所有 vertex（当前 vertex 标 `*`，孤立目录单独显示） |
| `show` | 显示 vertex 详情（任务数、描述、绑定分支、是否当前） |
| `create` | 创建 vertex（在 `KRON/VERTEX/<name>/` 生成目录） |
| `describe` | 更新 vertex 描述（≤200 字符，超出报错） |
| `branch` | 绑定/解绑 Git 分支（`--set ''` 清除绑定，纯元数据） |
| `delete` | 从注册表删除 vertex（`--also-remove-dir` 同时删除物理目录） |
| `use [name]` | 设置/显示当前 vertex（类比 Git HEAD） |
| `use --unset` | 清除当前 vertex 指针 |
| `current` | 等价于 `vertex use`（不带参数） |

**设计要点：**
- **Vertex 是逻辑分组**，物理目录是 `KRON/VERTEX/<name>/`
- **`todo`/`doing`/`done` 也是 vertex**（状态即目录）
- 删除 vertex **不会**自动清除 `vertex use` 指针；会输出 warning 提醒用户手动 `--unset`
- `list` 会扫描 `KRON/VERTEX/` 找出孤立目录（注册表外的目录），提示用户 `vertex create`

---

## `kron task`

任务管理（状态三件套：`todo → doing → done`）。

```
kron task list [vertex] [--state <state>] [--tag <tag>...]
kron task show <id> [--vertex <name>]
kron task add <vertex> --title <title> [--description <desc>] [--body <md>] [--tag <tag>...]
kron task describe <id> --description <desc> [--vertex <name>] [--editor]
kron task move <id> --to <todo|doing|done>
kron task start <id>
kron task done <id>
kron task back <id>
kron task check <id>
kron task edit <id> [--vertex <name>]
kron task delete <id> [--force] [--vertex <name>]
kron task tag <id> add <name> | remove <name> | list | clear [--vertex <name>]
kron task attach <id> <key=value>... [--vertex <name>]
```

| 子命令 | 作用 |
|--------|------|
| `list [vertex]` | 列出任务（默认当前 vertex；可按 `--state`/`--tag` 过滤） |
| `show <id>` | 显示任务详情（标题、描述、标签、文件路径、body） |
| `add <vertex>` | 创建任务（**必须** registered vertex + existing 目录；默认 state = `todo`） |
| `describe <id>` | 更新单行描述（≤200 字符，超出报错） |
| `move --to <state>` | 移动任务状态（**唯一** 合法跨 vertex 操作） |
| `start` | 等价于 `move --to doing` |
| `done` | 等价于 `move --to done` |
| `back` | 状态后退一步（`done → doing`，`doing → todo`） |
| `check` | 切换 `done ↔ todo` |
| `edit` | 用 `$EDITOR` 打开任务 Markdown 文件（默认 `notepad` / `vi`） |
| `delete` | 删除任务（必须 `--force`） |
| `tag add\|remove\|list\|clear` | 管理任务标签 |
| `attach` | 以 JSON 附加键值对到 body |

**状态规则：**
- `todo`、`doing`、`done` 是**仅有的三种合法状态**（大小写不敏感）
- 新建任务默认 `todo`
- `done.next()` = `None`（终态）
- `back` 不允许在 `todo` 上调用

**Vertex 规则（R2）：**
- 所有操作**严格限制在已存在的 vertex 目录内**
- 不允许隐式创建 vertex
- 跨 vertex 操作必须显式 `--vertex`
- 找不到 `--vertex` 时退而使用 `vertex use` 指针

**`add` 输入要求：**
- 必须传 `--title`
- 必须传 `--description` 或 `--body`（至少一个）
- `--description` ≤200 字符，超出会报错而非截断

---

## `kron important`

重要文件管理（双向同步：项目端 ↔ internal 端）。

```
kron important list [--tag <tag>]
kron important add <path> [--copy] [--symlink]
kron important remove <path> [--force] [--keep-source]
kron important show <path>
kron important sync [--dry-run]
```

| 子命令 | 作用 |
|--------|------|
| `list` | 列出所有重要文件及其同步状态 |
| `add <path>` | 注册文件并镜像到 `kron-internal/important/`（`--copy`/`--symlink` 暂未生效） |
| `remove <path>` | 注销文件（**默认同时删除项目端源文件**，需 `--keep-source` 保留） |
| `show <path>` | 显示文件详情（两端是否存在、大小、hash、关联冲突 ID） |
| `sync` | 执行冲突检测扫描（`conflict::detect`），更新 index 状态；`--dry-run` 仅展示将要扫描的文件 |

**冲突状态（5 种）：**

| 状态 | 含义 | 用户操作 |
|------|------|---------|
| `synced` | 两端 hash 一致 | 无 |
| `project_only` | 只有项目端存在 | 无（single-sided） |
| `internal_only` | 只有 internal 端存在 | 无（single-sided） |
| `conflict` | 两端都有但 hash 不同 | 需要 `conflict resolve` |
| `syncing` | 同步进行中（瞬时） | 无 |

---

## `kron conflict`

冲突检测与解决。

```
kron conflict list [--status pending|resolved|ignored] [--since <iso-date>]
kron conflict show <id> [--diff-only]
kron conflict resolve <id> --use project|internal [--reason <text>]
kron conflict ignore <id> [--reason <text>]
```

| 子命令 | 作用 |
|--------|------|
| `list [--status <s>]` | 列出冲突记录（默认 `pending`，可看 `resolved`/`ignored`） |
| `show <id>` | 显示冲突详情（两端版本预览 + unified diff + 可用解决方案） |
| `resolve --use project` | 以项目端版本解决（两端都变成 project 内容） |
| `resolve --use internal` | 以 internal 版本解决（两端都变成 internal 内容） |
| `ignore` | 标记为忽略（保持 conflict 状态，不自动修复） |

**解决后的状态：**
- `UseProject` / `UseInternal` → 两端统一为同一内容 → 索引 `Synced`
- `Ignore` → 两端保持不同 → 索引仍是 `Conflict`

**冲突生命周期：**
```
conflict::detect()
    │
    ├─ hash 相同 ──→ Synced
    ├─ 只有 project ──→ ProjectOnly
    ├─ 只有 internal ──→ InternalOnly
    └─ hash 不同 ──→ Conflict ──→ kron conflict resolve
                          ├─ use_project ──→ Resolved（两端都变 project）
                          ├─ use_internal ──→ Resolved（两端都变 internal）
                          └─ ignore ──→ Ignored（保持 Conflict）
```

---

## `kron context`

AI 友好上下文生成（输出到 `KRON/.kron-context/`）。

```
kron context                                       # 默认：增量生成
kron context --regenerate [--only <glob>]          # 全量重建
kron context --list [--json]                       # 列出文件及其新鲜度
kron context --clean [--force]                     # 清理所有文件
kron context show <path>                           # 显示指定文件
```

| 参数 | 作用 |
|------|------|
| --regenerate / -r | 全量重建（删除所有 context 文件后重新生成） |
| --list / -l | 列出所有 context 文件及其新鲜度（stale / fresh） |
| --clean / -c | 清理所有 context 文件（需 `--force` 确认） |
| --only `<glob>` | 只生成匹配 `git/*` / `code/*` 等的文件 |
| show `<path>` | 显示指定 context 文件内容 |

**生成的文件（v1）：**

| 文件 | 内容 | 过期判定 |
|------|------|---------|
| `git/recent-commits.md` | 最近 N 条 commit | > 60min 或 git diff 有变化 |
| `git/branch-summary.md` | 当前分支信息 | > 60min 或 git diff 有变化 |
| `code/structure.txt` | 项目目录结构 | > 60min 或文件有变化 |
| `README.md` | .kron-context/ 说明文档 | 较少过期 |

**设计原则：**
- **纯事实**：只生成可重现的结构化数据，不做 LLM 推理
- **零 API key**：完全本地，无外部依赖
- **零 daemon**：所有生成都是手动触发（曾经的 daemon 已被移除）

---

## 命令索引速查

| 命令 | 层级 | 一句话说明 |
|------|------|-----------|
| `kron init` | 一级 | 项目初始化 |
| `kron status` | 一级 | 项目状态总览 |
| `kron list` | 一级 | 列出项目（stub） |
| `kron path` | 一级 | 输出内部路径（脚本用） |
| `kron config` | 一级 | 配置读写 |
| `kron vertex` | 一级 | Vertex 管理（含 `use`/`current`） |
| `kron task` | 一级 | 任务 CRUD + 状态机 |
| `kron important` | 一级 | 重要文件管理 + 同步 |
| `kron conflict` | 一级 | 冲突检测与解决 |
| `kron context` | 一级 | AI 上下文生成 |

---

## 已移除（M2 不再提供）

| 命令 | 移除原因 |
|------|---------|
| `kron daemon start/stop/status` | 改为显式命令触发；M2 demo 用 PID 文件模拟，已删除 |

---

## 用户使用流程示例

下面展示一个真实用户从零开始使用 kron 的完整流程。

### 场景：开发者小明准备用 kron 管理一个新项目

#### 1. 项目初始化

```bash
$ cd ~/projects/myapp
$ ls
.git/

$ kron init
✓ Created KRON/
✓ Created kron-internal/
✓ Created kron-internal/config.json
...
hint: run `kron vertex create todo/doing/done` to bootstrap state columns,
      then `kron vertex use todo` to set the current vertex
```

#### 2. 创建三个状态 vertex

```bash
$ kron vertex create todo --description "待开始的任务"
✓ Vertex 'todo' created at KRON/VERTEX/todo/

$ kron vertex create doing --description "进行中"
✓ Vertex 'doing' created at KRON/VERTEX/doing/

$ kron vertex create done --description "已完成"
✓ Vertex 'done' created at KRON/VERTEX/done/
```

#### 3. 设置当前 vertex 并添加任务

```bash
$ kron vertex use todo
✓ Current vertex set to 'todo'

$ kron task add todo --title "实现登录功能" --description "支持邮箱密码登录" --tag backend
✓ Task T1 created at KRON/VERTEX/todo/T1.md
  Title:       实现登录功能
  Description: 支持邮箱密码登录
  Tags:        backend

$ kron task add todo --title "设计数据模型" --description "User 表 + Session 表"
✓ Task T2 created at KRON/VERTEX/todo/T2.md

$ kron task add doing --title "搭建 CI" --description "GitHub Actions 跑测试"
✓ Task T3 created at KRON/VERTEX/doing/T3.md
```

#### 4. 查看任务列表

```bash
$ kron task list              # 默认当前 vertex = todo
ID       STATE           TITLE
------------------------------------------------------------
T1       todo            实现登录功能
T2       todo            设计数据模型

$ kron task list doing       # 显示 doing 目录下的任务
ID       STATE           TITLE
------------------------------------------------------------
T3       doing            搭建 CI

$ kron task list --state todo --tag backend
ID       STATE           TITLE
------------------------------------------------------------
T1       todo            实现登录功能
```

#### 5. 任务状态流转

```bash
$ kron task start T1
✓ T1 moved to doing (KRON/VERTEX/doing/T1.md)

$ kron task done T1
✓ T1 moved to done (KRON/VERTEX/done/T1.md)

# 也可以显式指定目标状态
$ kron task move T2 --to doing
✓ T2 moved to doing (KRON/VERTEX/doing/T2.md)

# 后退一步
$ kron task back T1          # done → doing
✓ T1 moved back: done -> doing

# 切换 done/todo
$ kron task check T3         # doing → done
✓ T3 toggled: doing -> done
```

#### 6. 标记重要文件（双向同步）

```bash
$ kron important add README.md
✓ Registered 'README.md' as important (1234 bytes)
  Mirror:    kron-internal/important/files/README.md
  Sync:      synced

$ kron important list
PATH                                     STATE           SIZE      UPDATED
------------------------------------------------------------------------------------------
README.md                                synced          1234      2026-09-07T18:30:00Z

# 同事改了 README.md，本地没变，状态仍 synced
# 如果两端都改了 hash 不一致，下一次 sync 会创建冲突

$ kron important sync
Scan complete:
  scanned:            1
  synced:             1
  conflicts new:      0
```

#### 7. 处理冲突

```bash
# 假设有人同时改了 README.md 的两端
$ kron important sync
Scan complete:
  scanned:            1
  conflicts new:      1

$ kron conflict list
ID                                  FILE                           STATUS      DETECTED
----------------------------------------------------------------------------------------------------
2026-09-07-abc123                   README.md                      pending     2026-09-07T18:35:00Z

$ kron conflict show 2026-09-07-abc123
Conflict ID:    2026-09-07-abc123
File:           README.md
Detected:       2026-09-07T18:35:00Z
Status:         pending
Backup at:      kron-internal/conflicts/2026-09-07-abc123

Project version (856 bytes, MD5 aaa):
----------------------------------------
...

Internal version (1024 bytes, MD5 bbb):
----------------------------------------
...

Unified diff:
----------------------------------------
--- a/README.md
+++ b/README.md
-Old line
+New line

Resolutions: use --use project|internal | ignore

# 选择保留项目端的版本
$ kron conflict resolve 2026-09-07-abc123 --use project
✓ Resolved: README.md now matches (use_project)
```

#### 8. 生成 AI 上下文

```bash
# 增量生成（只重新生成过期的）
$ kron context
all context files up to date

# 全量重建（提交重大修改后）
$ kron context --regenerate
regenerated 4 file(s) in ~/projects/myapp/KRON/.kron-context

# 看看生成的文件
$ kron context --list
~/projects/myapp/KRON/.kron-context — context files
path                                          generated_at            status
------------------------------------------------------------------------------------------
  README.md                                    2026-09-07 18:40:00 UTC  fresh
  git/branch-summary.md                        2026-09-07 18:40:00 UTC  fresh
  git/recent-commits.md                        2026-09-07 18:40:00 UTC  fresh
  code/structure.txt                           2026-09-07 18:40:00 UTC  fresh

# 给 AI 工具喂上下文
$ kron path --kron-root
~/projects/myapp/kron-internal

$ cat "$(kron path --kron-root)/../KRON/.kron-context/README.md"
```

#### 9. 项目状态总览

```bash
$ kron status
Project:       /home/xiaoming/projects/myapp
Initialized:   true
Vertices:      3
Tasks:         4
Important:     1
Conflicts:     0 pending
```

#### 10. 调整配置

```bash
$ kron config get settings.context_refresh_minutes
5

$ kron config set settings.auto_resolve latest
✓ settings.auto_resolve = latest

$ kron config list
name:           myapp
created_at:     2026-09-07T18:00:00+08:00
conflict_threshold_minutes:    5
auto_resolve:                  latest
context_refresh_minutes:       5
```

---

## 与 Git 的对照

| Git | Kron | 说明 |
|-----|------|------|
| `git init` | `kron init` | 初始化仓库 / 项目 |
| `git status` | `kron status` | 状态总览 |
| `git log` | `kron context git/recent-commits.md` | 历史 |
| `git branch` | `kron vertex list` | 分支 / 方向 |
| `git checkout <branch>` | `kron vertex use <name>` | 切换当前 |
| `git HEAD` | `kron vertex current` | 当前指针 |
| (无) | `kron task list/move` | 任务状态机 |
| `git add <important>` | `kron important add <path>` | 标记重要文件 |
| `git diff` | `kron conflict show <id>` | 查看差异 |
| (无) | `kron context` | 生成 AI 上下文 |

**核心理念：** Kron 不与 Git 竞争，而是补足 Git 缺失的任务追踪与 AI 协作层。所有数据要么在 Git 仓库内（`KRON/`），要么明确标记 Git 忽略（`kron-internal/`）。
