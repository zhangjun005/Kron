# Kron 需求文档

> 本文档用于记录 Kron 项目的开发过程、需求分析、架构设计等内容。

---

## 📌 项目概述

**Kron** 是一款专为个人开发者设计的桌面应用工具，核心目标是解决 Windows 文件管理体验差的问题，提供轻量、快速的个人代码项目管理和文件跳转能力。

**核心理念**：做"减法"——简单到无需学习。

**附加价值**：在 AI 时代，KRON/ 目录也是**项目级 AI 可读文档库**——不仅是任务管理，也是让 AI 理解项目全貌的关键。

---

## 🎯 核心设计理念

### 1. 避免过度分类
- 三态状态：Todo / In Progress / Done
- 在三态之上，可选加 Vertex（顶点）反映开发阶段
- **Task 存储在三个 MD 文件中**：`TODO.md`、`IN_PROGRESS.md`、`DONE.md`
- Task 状态改变 = 移动到对应的 MD 文件

### 2. 本地与透明存储
- 数据全保存在本地
- 不依赖云端（云盘同步可选）
- 加载即时
- 可通过 Git 同步备份

### 3. 项目级数据
- 每个代码项目独立的 Kron 数据
- 数据文件位于项目内的 `KRON/` 文件夹

### 4. 双源存储设计
- **项目内存储** + **Kron 内部存储**
- 双源互为备份（任一存在即可同步）
- Kron 内部为主（数据源）
- 项目内为备份副本 + AI 可读文档库

### 5. 不使用快捷键
- 避免快捷键冲突问题
- 所有操作通过界面点击完成

### 6. AI 可读性优先
- `KRON/` 目录结构必须让 AI 易于理解
- Markdown 优先，结构清晰
- 是项目级"AI 友好的文档库"

### 7. Kron 不做 AI 辅助，但通过文件结构让 AI 易读 ⭐⭐⭐⭐⭐

- **Kron 不内置 AI 辅助功能**（不做"AI 生成描述"等烂大街功能）
- AI 工具（Cursor / Claude Code / Copilot）已足够强大
- Kron 通过**精心设计的文件结构**让 AI 自然理解项目：
  - `KRON/README.md`（项目总览）
  - `KRON/VERTEX/*/TODO.md`（待办任务）
  - `KRON/VERTEX/*/IN_PROGRESS.md`（正在做的）
  - `KRON/VERTEX/*/DONE.md`（已完成）
  - `KRON/important/`（重要 idea / 设计文档）
  - `KRON/.kron-context/`（自动生成的项目上下文，详见 2.4）

**核心哲学**：
- ✅ **简洁**：Kron 不引入 AI SDK / API Key 等复杂性
- ✅ **AI 易读**：文件结构让 AI 工具自然理解项目
- ✅ **解耦**：Kron = 数据层，AI 工具 = 智能层
- ✅ **可持续**：通过 AI 易读设计保障项目长期可维护

**详细设计见需求 7（AI 工具联动）**。

---

## 🏗 核心架构：双源存储 + 复原机制

### 双源数据流

```
┌──────────────────────────────────────────────────┐
│  Kron Desktop App（数据管理）                         │
│  <kron_install_dir>/data/projects/<project_hash>/ │
│  ├── config.json         ← 项目配置                  │
│  ├── vertices.json       ← Vertex 绑定关系           │
│  ├── vertex-<id>.json    ← Vertex 详细数据             │
│  ├── _meta.json          ← 全局元数据                 │
│  ├── important/          ← 重要文件完整备份（双源）       │
│  │   └── <files>                                    │
│  ├── vertex-<hash>/      ← Vertex 数据备份              │
│  │   └── notes/                                      │
│  │       └── <files>                                │
│  └── backups/            ← 全局快照备份（仅最新，不保留历史）│
└──────────────────────────────────────────────────┘
                     ↕ 双向同步（任一为最新即可）
┌──────────────────────────────────────────────────┐
│  项目文件夹内（次要 - 备份 + AI 可读）                   │
│  ~/Code/project/KRON/                            │
│  ├── README.md                                        │
│  ├── ARCHITECTURE.md (可选)                           │
│  ├── vertices.json     ← Vertex 绑定关系               │
│  ├── VERTEX/             ← 所有 Vertex                │
│  │   ├── 需求分析/                                   │
│  │   │   ├── TODO.md                                │
│  │   │   ├── IN_PROGRESS.md                         │
│  │   │   ├── DONE.md                                │
│  │   │   └── notes/      ← 该阶段归档的文档            │
│  │   └── 开发/                                       │
│  │       └── ...                                      │
│  └── important/         ← 重要文件夹（双源备份）         │
│      └── <files>                                      │
└──────────────────────────────────────────────────┘
```

> 注：`<kron_install_dir>` 是 Kron 自身的安装绝对路径，例如 `C:\Apps\Kron\`。所有 Kron 内部数据都存放在这里，**不使用 Windows 用户目录**（避免账户变化、系统重装导致路径失效）。

### 酉要文件存储路径

**重要文件不在项目根的固定位置**，而是用户可以指定**与项目根的相对路径**：

```bash
# 添加重要文件（指定相对路径）
kron important add ./docs/api-spec-v1.md
# 复制到 KRON/important/api-spec-v1.md（保持相对路径结构）
# 同步到 Kron 内部

kron important add ./config/prod.json
# 复制到 KRON/important/config/prod.json
# 同步到 Kron 内部
```

**这样设计的优点**：
- 不强制 docs/ 目录
- 用户可以在任何位置使用重要文件功能
- Kron 自动处理路径冲突

---

## ✅ 已确认需求

### 需求 1：快速文件跳转 ⭐ 核心痛点

**背景**：Windows 自带文件管理器体验差

**目标**：实现快速跳转到任意文件夹

**功能**：
- [ ] 模糊搜索文件夹路径
- [ ] 历史记录快速访问
- [ ] 收藏夹 / 常用文件夹置顶
- [ ] **点击直接用文件管理器打开对应文件位置**
- [ ] 在外部编辑器打开：
  - [ ] VSCode
  - [ ] Cursor
  - [ ] Codex
  - [ ] **Typora** ⭐（用户日常使用）
  - [ ] Claude Code 在命令行，无需（自动处理）

**实现方式**（"用外部应用打开"功能）：
- **使用 Windows ShellExecute 调用系统默认应用**（详见 4.7）
- 不需要 Kron 内部维护应用路径
- 不需要 Kron 自动检测已安装应用
- 用户在 Windows 系统设置中管理默认应用即可
- Kron 完全不关心用哪个应用打开

**优势**：
- ✅ Kron 代码极少（只调 ShellExecute）
- ✅ Windows 自动处理"打开方式"菜单
- ✅ 用户切换默认应用时 Kron 自动适配
- ✅ 与 Windows 资源管理器行为一致

---

### 需求 1.1：Task 存储设计（指针）

**核心原则**：task 是 **MD 文件里的结构化内容**，**不是 Kron 内部数据**。

**Task 完整设计请参见 需求 2.3（Task 完整属性设计）**。

包括：
- Task 属性清单（核心、时间、组织、关联、自定义属性）
- Task 描述（description）是重点
- AI 辅助修改/追加描述
- Task 完整存储格式（含 front matter）
- Task 字段管理（自动 vs 用户 vs AI）
- Task ID 格式
- Task 状态转换规则
- Task 历史与变更追踪
- Task 与重要文件夹的关联

**存储位置**（简要）：
```
KRON/VERTEX/<name>/
├── TODO.md          ← Todo 状态的 task 列表
├── IN_PROGRESS.md   ← In Progress 状态的 task 列表
└── DONE.md          ← Done 状态的 task 列表
```

**设计要点**：
- [x] task 的内容**存储在 MD 文件中**（项目内 `KRON/VERTEX/<name>/<STATE>.md`）
- [x] Kron **解析 MD 文件** 展示 task 列表
- [x] Kron **编辑 task 后写回 MD 文件**
- [x] 用户用 Typora / VSCode / Cursor 直接编辑 MD 文件也可以
- [x] Kron 和外部编辑器**双向同步**

**与外部编辑器的协同**：
```
用户在 Typora 编辑 TODO.md（修改某个 task 的描述）
    ↓
Typora 保存文件
    ↓
Kron 文件监听器检测到 TODO.md 变化
    ↓
Kron 重新解析 TODO.md
    ↓
Kron GUI 更新 task 列表
    ↓
双向同步保持一致
```

**详细 Task 字段定义、AI 辅助功能等请参考 2.3 章节。**

---

### 需求 2：个人任务管理（项目级）

#### 2.1 Vertex 概念（极简设计）⭐⭐⭐⭐⭐

**定义**：
- **Vertex**（顶点）：一个开发**阶段**
- 每个 Vertex 是一个独立的开发周期（有明确的开始）
- 阶段的**关系通过 Git 树遍历得到**，不在 Kron 中显式存

**核心设计哲学**：**Kron 不存阶段关系，让 Git 自己说话** ⭐⭐⭐⭐⭐

| 设计 | 说明 |
|------|------|
| Vertex 只存**起点 commit** | 不存终点（终点 = 下一个 Vertex 的起点） |
| **Vertex 关系 = Git 树遍历** | 自动得到阶段顺序，不要硬编码 |
| **不要交叉阶段** | 阶段的顺序是线性的 |
| **允许多个绑定点** | 一个 Vertex 可创建多次，绑定不同 commit |

**Vertex 绑定结构**（极简）：

```json
// KRON/VERTEX/开发/_meta.json
{
  "name": "开发",
  "bind_points": [
    {
      "commit": "abc1234",
      "branch": "feature/login",
      "created_at": "2026-09-04T10:00:00Z"
    }
  ]
}
```

**为什么允许多个绑定点**：

- 一个 Vertex 可创建多次（如"开发"在不同分支多次启动）
- 每个绑定点 = 一个独立的"开发阶段"
- 例如：第一次"开发"绑定 `abc1`，第二次"开发"绑定 `def2`

**Vertex 在 Git 树上的关系（自动推导）**：

```
commit  abc1234  ← "开发" 顶点 1（bind_points[0]）
         |
commit  def5678  ← "重构" 顶点 1（bind_points[0]）
         |
commit  ghi9012  ← "开发" 顶点 2（bind_points[1]）
         |
commit  ...      ← 用户回到"开发"！git 树遍历告诉 Kron 这个顺序
```

**关系推导规则**：
- 从任意 commit 出发，遍历 Git 树（commit graph）
- 找到该 commit 上**距离最近**的 Vertex 绑定点
- 该 Vertex 即为该 commit 所属的阶段

**核心原则**：
- ❌ **不要交叉阶段**（vertex 管理范围是阶段性的）
- ✅ 允许多个绑定点（同一 vertex 不同阶段）
- ✅ 关系由 Git 推导，不在 Kron 中存

**Vertex 管理的工作流**：

```
用户创建 Vertex（CLI：kron vertex create）
    ↓
Kron 读取当前 Git HEAD commit
    ↓
在该 Vertex 的 _meta.json 添加一个 bind_point
    ↓
不需要手动指定 range / 不需要手动指定后续
    ↓
git 树自动告诉 Kron 哪个 commit 属于哪个阶段
```

**Vertex 描述 MD（新增）**：

```bash
# 用户可在 vertex 文件夹下创建一个描述文件
KRON/VERTEX/开发/
├── description.md         ← Vertex 描述（用户编辑）
├── tasks.md               ← 所有 task
└── _meta.json             ← Vertex 绑定点
```

**`description.md` 的用途**：

- 描述这个 Vertex 的**意图**（为什么有这个阶段）
- 比如"重构阶段的目标"、"性能优化的指标"
- **不存储 task 内容**（task 在 tasks.md 中）
- 用户/AI 可自由编辑

**示例**：

```markdown
# Vertex: 开发

## 阶段意图

实现用户登录功能模块，包括邮箱、手机号、OAuth。

## 目标

- 抽象 AuthProvider 接口
- 支持三种登录方式
- 完整的 session 管理

## 备注

参考 `legacy-auth/` 项目。
```

**为什么 Vertex 要有 description.md**：

- ✅ Task 是"做什么"
- ✅ Vertex 描述是"为什么有这个阶段"
- ✅ 两者分离，意图清晰
- ✅ AI 读起来能理解阶段目的

**为什么不要存 Vertex 关系**：

- ❌ 显式存关系 = 双源数据（Kron + Git）
- ❌ 用户可能忘记更新（数据不一致）
- ✅ Git 树是**唯一真理源**
- ✅ Kron 只存 Vertex 的**绑定点**，关系**遍历得到**

**CLI 命令（极简）**：

```bash
kron vertex create <name>     # 创建 Vertex，绑定当前 HEAD commit
kron vertex list              # 列出所有 Vertex
kron vertex delete <name>     # 删除 Vertex（同时清理其下 task）
# 注意：kron vertex bind 已删除！
# 注意：kron vertex range 已删除（关系由 Git 推导）！
```

**手动绑定已删除**：
- 之前 `kron vertex bind` 是手动绑分支
- 现在 Vertex 关系 = Git 树遍历
- 用户只能"创建 + 绑定 commit"，不能手动改关系

---

#### 2.2 Task 直接编辑 MD 设计 ⭐⭐⭐⭐⭐

**核心设计哲学**：**让用户/AI 直接编辑 MD，不要走一堆 CLI** ⭐⭐⭐⭐⭐

**之前设计的问题**：

- ❌ `kron task add "..."` 然后交互式问 vertex/state/description
- ❌ `kron task move <id> <state>` 改变状态
- ❌ `kron task tag add <id> <tag>` 加标签
- ❌ `kron task tag remove <id> <tag>` 删标签
- ❌ 这些命令太多，太麻烦

**新设计（极简）**：

```
用户/AI 直接编辑 tasks.md
    ↓
遵循简单的 MD 格式
    ↓
保存文件
    ↓
Kron 后台守护进程检测文件变化
    ↓
自动解析 + 格式检查
    ↓
格式错误提示用户
    ↓
格式正确则同步到 Kron 内部
    ↓
GUI 自动更新
```

**唯一需要的命令**：`kron task check`（手动触发格式检查）

```bash
kron task check               # 检查所有 tasks.md 格式 + 同步到 Kron 内部
```

**为什么只需这一个命令**：

- ✅ 用户/AI 不用记一堆 task 命令
- ✅ 直接编辑 MD（自然、熟悉）
- ✅ Kron 自动监听 + 自动同步
- ✅ 只有"格式错误"或"想立即同步"时才用 CLI

---

#### 2.2.1 Vertex 文件夹结构

```
KRON/VERTEX/<vertex_name>/
├── description.md         ← Vertex 描述（阶段意图）⭐新增
├── tasks.md               ← 所有 task（不分状态）
└── _meta.json             ← Vertex 绑定点列表
```

**3 个文件，各司其职**：

| 文件 | 内容 | 谁编辑 |
|------|------|--------|
| `description.md` | Vertex 描述（阶段意图） | 用户/AI |
| `tasks.md` | Task 列表 | 用户/AI |
| `_meta.json` | Vertex 绑定点（git commits） | Kron 自动管理 |

---

#### 2.2.2 Task MD 格式规范 ⭐⭐⭐⭐⭐

**格式非常简单**：

```markdown
## [task_001] 实现登录功能

### 背景

用户登录是系统入口...

### 目标

支持邮箱 + 手机号 + OAuth 登录。

### 实现方案

1. 抽象 AuthProvider 接口
2. 实现三种 Provider
3. JWT token 签发与刷新

### 备注

参考 `legacy-auth/` 项目。

tags: #后端 #认证 #feature

---

## [task_002] 设计数据库表结构

### 背景

认证模块依赖基础数据表。

### 目标

用户表、会话表、权限表。

tags: #后端 #数据库
```

**格式规则（极简）**：

| 元素 | 格式 | 示例 |
|------|------|------|
| Task 标题 | `## [task_XXX] 标题` | `## [task_001] 实现登录功能` |
| Task 内容 | 二级标题（### 背景 / 目标 / 方案 / 备注） | `### 背景` |
| Tags | 一行 `tags: #tag1 #tag2` | `tags: #后端 #认证` |
| Task 分隔 | `---` | `---` |

**关键规则**：

- ✅ **task ID 格式**：`task_<数字>`（Kron 自动识别）
- ✅ **标题前缀**：`## [task_XXX]`（Kron 自动识别）
- ✅ **tags 行**：单行 `tags: #...`（Kron 自动解析）
- ✅ **内容自由**：除 tags 行外，用户/AI 完全自由编辑
- ❌ **不强求** 4 个二级标题（用户可写任意内容）

**格式错误处理**：

- Kron 解析时如果检测到格式错误 → 弹出提示
- 错误示例：
  - task ID 格式不对（不是 `task_XXX`）
  - 同一文件有重复 ID
  - tags 格式不对（不是一个独立行）
- 用户修正后保存 → 自动重新检查 → 通过后同步

---

#### 2.2.3 state 字段的处理（极简设计）

**Key 设计**：

| 原则 | 行为 |
|------|------|
| **MD 中不存 state** | task MD 中**不包含** `state: todo/in_progress/done` |
| **状态仅在 GUI 中** | 用户拖拽 task 到不同列 = 状态变化 |
| **状态存在 Kron 内部** | `states/<vertex>.json` 仅 Kron 内部使用 |

**为什么这样设计**：

- ❌ 之前 task MD 中存 `state` → 状态变化要修改 MD
- ✅ 现在 state 只在 GUI 中 → 拖拽 = 状态变化，MD 不动
- ✅ MD 文件极简，AI 读起来清爽
- ✅ 用户不用关心 state 字段（只在 GUI 里操作）

**state 的工作流**：

```
用户拖拽 task_001 从 "Todo" 到 "In Progress"
    ↓
Kron GUI 更新内存中 task_001.state = "in_progress"
    ↓
定时任务（5 分钟） → 写入 Kron 内部 states/<vertex>.json
    ↓
MD 文件（tasks.md）**不动**
```

**直接编辑 MD 时的 state 处理**：

```
用户/AI 修改 tasks.md（添加/删除/修改 task）
    ↓
Kron 后台守护进程监听
    ↓
触发 format check + sync
    ↓
新 task → Kron 内部 state 默认 "todo"
删除 task → Kron 内部 state 清理
修改 task → state 不变（如果 ID 还在）
    ↓
用户在 GUI 中可拖到其他列
```

---

#### 2.2.4 description / tag 直接编辑 MD ⭐⭐⭐⭐⭐

**核心创新**：**description 和 tag 直接编辑 MD，不需要 CLI 命令**。

**description 直接编辑**：

```bash
# ❌ 之前：
# kron task edit task_001 --description "新描述..."
# （要记住命令、参数格式）

# ✅ 现在：
# 用户/AI 直接在 MD 中编辑 task 内容
## [task_001] 实现登录功能

### 背景
新背景内容...

### 目标
新目标内容...
```

**tag 直接编辑**：

```bash
# ❌ 之前：
# kron task tag add task_001 "#后端"
# kron task tag remove task_001 "#认证"
# kron task tag list task_001

# ✅ 现在：
# 用户/AI 直接在 MD 中编辑 tags 行
## [task_001] 实现登录功能
...
tags: #后端 #数据库

# 删除一个 tag → 直接从 tags 行删除
```

**tag 解析规则**：

```markdown
## [task_001] 实现登录功能

[task 内容]

tags: #后端 #认证 #feature
```

**Kron 解析逻辑**：

1. 找到 task 标题 `## [task_001] ...`
2. 在 task 内容中找到 `tags: ` 开头的行
3. 提取 `#xxx` 形式的标签
4. 存储到 Kron 内部 `tags/tasks.json`（不直接存在 MD 中除了 tags 行）

**为什么 tag 行还在 MD 中**：

- ✅ 用户/AI 可读（直接看到 task 有哪些 tag）
- ✅ 可直接编辑（修改 tag = 修改这一行）
- ✅ AI 工具读 MD 自动获得 tag 信息

---

#### 2.2.5 格式检查 + 同步（kron task check）⭐

**核心命令**：

```bash
kron task check              # 检查所有 tasks.md 格式 + 同步到 Kron 内部
kron task check --verbose    # 详细输出（哪个 task 有问题）
kron task fix                # 自动修复常见格式问题（可选）
```

**`kron task check` 工作流**：

```
扫描所有 VERTEX/*/tasks.md
    ↓
对每个 tasks.md：
├─ 解析所有 task
├─ 验证格式（ID 唯一 / 标题前缀 / tags 行格式）
├─ 报告错误（如果有）
└─ 格式正确 → 同步到 Kron 内部
    ↓
扫描所有 VERTEX/*/description.md（格式自由的 MD，无需检查）
    ↓
返回结果（成功/失败 + 错误详情）
```

**自动触发时机**：

| 触发 | 说明 |
|------|------|
| **后台守护进程** | 检测到 tasks.md 变化时自动运行（默认） |
| **手动命令** | `kron task check`（用户主动触发） |
| **保存时** | Typora / VSCode 等编辑后保存即可（自动） |

**自动触发 vs 手动触发的关系**：

- ✅ 后台守护进程自动 check + sync（用户无需手动）
- ✅ `kron task check` 仅在用户**想立即同步**或**确认格式**时使用
- ✅ 自动检查失败时 → 提示用户修正

**自动修复（可选）`kron task fix`**：

- 自动修复常见格式错误
- 例如：`task_1` → `task_001`（补零）
- 例如：缺失 `---` 分隔 → 自动添加
- 用户保留控制权（修复前提示用户）

---

#### 2.2.6 Vertex description.md 格式

**完全自由**（不强制格式）：

```markdown
# Vertex: 开发

## 阶段意图

实现用户登录功能模块。

## 目标

- 抽象 AuthProvider 接口
- 支持三种登录方式
- 完整的 session 管理

## 备注

参考 `legacy-auth/` 项目。
```

**为什么 description.md 完全自由**：

- ✅ 用户/AI 自由表达
- ✅ 无格式约束
- ✅ Kron 不解析（仅同步到 GUI 显示）
- ❌ Kron 不强制任何结构

**Kron 怎么用 description.md**：

- 在 GUI 中显示（让用户看到这个 Vertex 的意图）
- 在 task 上方显示（让用户理解阶段背景）
- 不参与任何逻辑计算

---

#### 2.2.7 状态/Tag 的存储分布 ⭐

**Kron 内部权威源**：

| 字段 | 存储位置 | 说明 |
|------|---------|------|
| `state` | `states/<vertex>.json` | 仅 GUI 维护，5 分钟同步 |
| `tags` | `tags/tasks.json` | 从 MD 中 `tags:` 行解析 |
| `id` / `title` / `description` | MD 文件 | 用户/AI 直接编辑 |

**为什么 tags 还要存 Kron 内部**：

- 用户/AI 在 MD 中编辑 tags 行（自由）
- Kron 解析 tags 行 → 同步到内部 JSON
- Kron GUI 用内部 JSON 显示 tag 过滤/搜索
- MD 中 tags 行**与 Kron 内部 JSON 一致**

---

#### 2.2.8 Task 状态变化的三种方式

**方式 1：GUI 拖拽**（默认）

```
用户拖拽 task → 状态变化 → MD 不动 → 5 分钟同步到内部 JSON
```

**方式 2：直接编辑 MD**（添加/删除 task）

```
用户编辑 tasks.md → 保存 → 后台守护进程检测
    ↓
格式检查通过 → 同步到内部 JSON
    ↓
新 task 默认 state = "todo"
已删除 task → 内部清理
状态栏不变
```

**方式 3：AI 工具编辑 MD**（Cursor / Claude Code）

```
AI 工具修改 tasks.md（添加/修改/删除 task）
    ↓
后台守护进程检测 + 自动格式检查
    ↓
通过 → 同步到内部 + GUI 更新
失败 → 提示用户修正
```

**修改 description 和 tag 不影响 state**：

- 修改 task 内容（背景/目标/方案） → state 不变
- 修改 tags 行 → tag 更新，state 不变
- 删除 task → 内部清理
- 添加 task → 默认 state = "todo"

---

#### 2.2.9 直接编辑 MD 的工作流 ⭐⭐⭐⭐⭐

**用户日常使用**：

```
场景 A：用户想添加 task
└─ 1. 打开 Kron GUI
    2. 在 GUI 中点击"新建"，自动打开 tasks.md 等待用户输入
    3. 用户在 tasks.md 中按格式写 task
    4. 保存后自动同步到 Kron 内部

场景 B：用户想直接用编辑器（不用 GUI）
└─ 1. 用 Typora / VSCode / Cursor 打开 tasks.md
    2. 直接编辑（加 task / 改 description / 改 tags）
    3. 保存
    4. 后台守护进程自动检测 + check + sync
    5. Kron GUI 自动更新

场景 C：AI 工具修改 task
└─ 1. Cursor / Claude Code 修改 tasks.md
    2. 保存
    3. 后台守护进程自动检测
    4. 通过 → 同步；失败 → 提示用户
```

**AI 工具的核心工作流** ⭐⭐⭐⭐⭐：

```
AI 工具（Cursor / Claude Code）读取项目时
    ↓
读 KRON/VERTEX/<name>/tasks.md
    ↓
自然理解 task 内容（AI 友好）
    ↓
用户要求"修改 task_001 的描述"
    ↓
AI 用 Edit 工具直接修改 MD
    ↓
自动触发格式检查
    ↓
通过 → 立即生效
    ↓
AI 工具完全独立操作，不需要 CLI 介入
```

**关键意义**：

- ✅ AI 工具可以**直接编辑 MD**（不依赖 CLI）
- ✅ 用户**不需要学 CLI**（直接编辑 MD）
- ✅ 唯一命令 `kron task check` 仅在用户**想立即同步**时使用
- ✅ 主动权在用户/AI 手中（编辑 MD），不在 Kron 中

---

| 设计 | 说明 |
|------|------|
| **MD 文件不分状态** | 全部 task 在同一个 `tasks.md` 中 |
| **状态只在 GUI 中** | 用户拖拽到哪一列就是什么状态 |
| **不做 TODO/IN_PROGRESS/DONE 三份 MD** | 避免"移动文件"的副作用 |

**为什么这样做**：

- ❌ 之前设计 task 状态存储在 MD 中 → 每次状态变化都要移动 task
- ✅ task 在 GUI 中移动 = 状态变化（**仅视图层**）
- ✅ MD 文件极简（只有 title + description + updated_at）
- ✅ AI 读 task 时只看到核心内容

**Task 存储格式**（极简）：

```markdown
# Vertex: 开发

<!-- kron:vertex_meta
{
  "name": "开发",
  "git_range": {
    "from": "abc1234",
    "to": "HEAD",
    "branch": "feature/login"
  },
  "updated_at": "2026-09-04T20:00:00Z"
}
-->

## [task_001] 实现登录功能

### 背景

用户登录是系统的入口...

### 目标

支持邮箱 + 手机号 + OAuth 登录。

### 实现方案

1. 抽象 AuthProvider 接口
2. ...

### 备注

参考 `legacy-auth/` 项目。

<!-- kron:task_meta
{
  "id": "task_001",
  "updated_at": "2026-09-04T20:00:00Z"
}
-->

---

## [task_002] 设计数据库表结构

### 背景

[...]

<!-- kron:task_meta
{
  "id": "task_002",
  "updated_at": "2026-09-04T18:00:00Z"
}
-->
```

**MD 文件极简设计**：

| 字段 | 是否写入 MD | 说明 |
|------|------------|------|
| `id` | ✅ | 标题 `[task_001]` |
| `title` | ✅ | 标题文字 |
| `description` | ✅ | 二级标题（背景/目标/方案/备注） |
| `updated_at` | ✅ | 元数据注释 |
| `state` | ❌ | 仅 GUI 中跟踪 |
| `tags` | ❌ | 单独存 `.tags.json` |
| `vertex` | ❌ | 由所在文件路径决定 |

**任务移动的实现**：

```
用户在 GUI 中拖拽 task_001
    从 "Todo" 列到 "In Progress" 列
        ↓
Kron GUI 更新内存中的 task state
        ↓
不需要修改 tasks.md 文件！
        ↓
后台守护进程定时（5 分钟）将内存中的 state 写入
`<kron_install_dir>/data/projects/<hash>/states/<vertex>.json`
```

**内存中的 task 状态结构**（Kron 内部）：

```json
// <kron_install_dir>/data/projects/<hash>/states/开发.json
{
  "vertex": "开发",
  "task_001": "in_progress",
  "task_002": "todo",
  "task_003": "done",
  "task_004": "todo",
  "updated_at": "2026-09-04T20:00:00Z"
}
```

**优势**：
- ✅ MD 文件不被状态变化污染
- ✅ 移动 task 不涉及 MD 文件操作
- ✅ AI 读 MD 永远是最干净的版本
- ✅ Kron GUI 内部跟踪状态，**用户拖拽 = 状态变化**

---

#### 2.2.1 Vertex 与 Git 范围的可视化展示 ⭐⭐⭐⭐

**核心设计**：Vertex 绑定的不是单个分支，而是 **Git 范围（range）**。

**Vertex 范围计算规则**：
---

#### 2.2.1 Vertex 与 Git 的关系（自动推导）⭐⭐⭐⭐

**核心设计**：**Vertex 关系由 Git 树遍历得到，不在 Kron 中存**。

**为什么要遍历 Git 树**：

- ❌ 显式存关系 = 双源数据（Kron + Git）
- ❌ 用户容易忘记更新（数据不一致）
- ✅ Git 树是**唯一真理源**
- ✅ Kron 只存 Vertex 绑定点，关系**自动推导**

**Vertex 关系推导规则**：

```
commit 历史（git log --graph）：
                  * (master HEAD)
                  │
                  * abc1234
                  │  ← "开发" Vertex 绑定点 1
                  │
                  * def5678
                  │  ← "重构" Vertex 绑定点 1
                  │
                  * ghi9012
                  │  ← "开发" Vertex 绑定点 2（同一 Vertex 第二次！）
                  │
                  * (latest commit)

阶段关系（自动推导）：
"开发" → "重构" → "开发"
            ↑
            └── 用户回到 "开发"，允许多个绑定点
```

**Git 树遍历算法**：

```
对任意 commit C：
    ↓
找到 C 之前最近的 Vertex 绑定点 P
    ↓
P 所属的 Vertex 即为该 commit 的阶段
    ↓
返回该阶段的元数据（描述、tasks 等）
```

**实现方式**：

- 使用 Git 树的 commit graph（拓扑序）
- 从当前 commit 反向遍历，找到最近的 Vertex bind_point
- Kron 后台守护进程定时更新这个关系（5 分钟）

**Vertex 关系展示**（GUI）：

```
┌── Git Tree (横向) ─────────────────────────────────────────┐
│                                                            │
│  * ── * ── * ── * ── * ── * ── * ── (HEAD)                │
│              │              │                              │
│              │              │                              │
│              ▼              ▼                              │
│         ┌────────┐     ┌────────┐                          │
│         │ 开发   │     │ 重构   │                          │
│         │ (1)    │     │ (1)    │                          │
│         └────────┘     └────────┘                          │
│                  ↑                                         │
│                  │                                         │
│              Git 树自动连接                                │
│                                                            │
└────────────────────────────────────────────────────────────┘
        ↓ ↓ ↓  顶点连接下方 task 列表
        ↓ ↓ ↓

┌── Todo ─────┬── In Progress ──┬── Done ──────────┐
│ task_004    │  task_001       │ task_002         │
│ task_005    │                 │ task_003         │
└─────────────┴─────────────────┴──────────────────┘
```

**关键点**：

- ✅ Vertex 之间的关系**通过 Git 树连接**（可视化）
- ✅ 用户一眼看到每个 commit 属于哪个 Vertex
- ✅ 不需要 Kron 维护任何关系数据
- ✅ 切换 branch 时 Vertex 关系自动更新

---

#### 2.2.2 直接编辑 MD 的工作流 ⭐⭐⭐⭐⭐

**核心设计**：**让用户/AI 直接编辑 MD 文件，不用 CLI 操作 task 内容**。

**用户的全部 task 操作 = 编辑 MD**：

| 操作 | 之前（CLI） | 现在（编辑 MD） |
|------|------------|---------------|
| 新建 task | `kron task add` | 在 tasks.md 中粘贴 task 模板 |
| 修改 description | 无 CLI | 直接编辑 ### 背景 / 目标等 |
| 删除 task | 无 CLI | 直接删除 task 块（`---` 分隔） |
| 添加 tag | `kron task tag add` | 直接编辑 tags 行 |
| 删除 tag | `kron task tag remove` | 直接从 tags 行删除 |
| 移动 task 到另一 vertex | `kron task move` | 直接在另一个 tasks.md 中编辑 |

**唯一 CLI 命令：`kron task check`**

```bash
kron task check              # 检查所有 tasks.md 格式 + 同步到 Kron 内部
kron task check --verbose    # 详细输出（哪个 task 有问题）
kron task fix                # 自动修复常见格式问题（可选）
```

**为什么只需这一个命令**：

- ✅ 用户/AI 不用记一堆 task 命令
- ✅ 直接编辑 MD（自然、熟悉）
- ✅ Kron 自动监听 + 自动同步
- ✅ 只有"格式错误"或"想立即同步"时才用 CLI

**Kron 后台守护进程的工作**：

```
Kron 后台守护进程启动
    ↓
监听 KRON/VERTEX/*/tasks.md 变化（notify crate）
    ↓
检测到 tasks.md 变化：
├─ 解析所有 task
├─ 验证格式（ID 唯一 / 标题前缀 / tags 行格式）
├─ 错误 → 提示用户修正
└─ 正确 → 同步到 Kron 内部
    ↓
GUI 自动更新
```

**用户日常使用**：

```
场景 A：用户想添加 task
└─ 1. 打开 Kron GUI
    2. 在 GUI 中点击"新建"，自动打开 tasks.md 等待用户输入
    3. 用户在 tasks.md 中按格式写 task
    4. 保存后自动同步到 Kron 内部

场景 B：用户想直接用编辑器（不用 GUI）
└─ 1. 用 Typora / VSCode / Cursor 打开 tasks.md
    2. 直接编辑（加 task / 改 description / 改 tags）
    3. 保存
    4. 后台守护进程自动检测 + check + sync
    5. Kron GUI 自动更新

场景 C：AI 工具修改 task
└─ 1. Cursor / Claude Code 修改 tasks.md
    2. 保存
    3. 后台守护进程自动检测
    4. 通过 → 同步；失败 → 提示用户
```

**AI 工具的核心工作流** ⭐⭐⭐⭐⭐：

```
AI 工具（Cursor / Claude Code）读取项目时
    ↓
读 KRON/VERTEX/<name>/tasks.md
    ↓
自然理解 task 内容（AI 友好）
    ↓
用户要求"修改 task_001 的描述"
    ↓
AI 用 Edit 工具直接修改 MD
    ↓
自动触发格式检查
    ↓
通过 → 立即生效
    ↓
AI 工具完全独立操作，不需要 CLI 介入
```

**关键意义**：

- ✅ AI 工具可以**直接编辑 MD**（不依赖 CLI）
- ✅ 用户**不需要学 CLI**（直接编辑 MD）
- ✅ 唯一命令 `kron task check` 仅在用户**想立即同步**时使用
- ✅ 主动权在用户/AI 手中（编辑 MD），不在 Kron 中

---

#### 2.2.2.1 Task 状态转换命令（AI 工具友好）⭐⭐⭐⭐⭐

**核心问题**：AI 工具怎么操作 task 状态？

之前的设计：**state 仅在 GUI 中维护**（拖拽 = 变化）
问题：**AI 工具没有 GUI**，怎么操作？

**新设计**：**提供优雅的 CLI 命令，让 AI 可以操作** ⭐⭐⭐⭐⭐

**设计原则**：

- ✅ 不需要记 task ID
- ✅ 模糊匹配友好
- ✅ 交互式选择（人类和 AI 都好用）
- ✅ 一行命令可以完成（AI 调用自然）

---

##### 核心命令：kron task move

**4 种调用方式**（从最简单到最完整）：

**方式 1：完全交互式**（最自然）

```bash
$ kron task move

正在加载 task 列表...

  [1] task_001  实现登录功能                    [todo]
  [2] task_002  设计数据库表结构                [todo]
  [3] task_003  实现 JWT 中间件                 [in_progress]
  [4] task_004  添加日志中间件                  [todo]

选择要移动的 task（输入编号或 ID）：2
选择目标状态（todo/in_progress/done）：in_progress

✓ task_002 状态已更新为 in_progress
```

**方式 2：模糊匹配 + 交互式状态**（推荐）

```bash
$ kron task move "登录"

找到 1 个匹配的 task：
  task_001  实现登录功能  [todo]

选择目标状态（todo/in_progress/done）：in_progress

✓ task_001 状态已更新为 in_progress
```

**方式 3：一行完成**（AI 最常用）

```bash
$ kron task move "登录" in_progress
✓ task_001 "实现登录功能" 状态已更新为 in_progress
```

**方式 4：直接指定 task ID**（精确操作）

```bash
$ kron task move task_001 in_progress
✓ task_001 状态已更新为 in_progress
```

---

##### 智能回退命令：kron task back

**核心场景**：误操作回退 / 状态倒退

```bash
# 智能回退到上一个状态
$ kron task back

正在加载 task 列表...

  [1] task_001  实现登录功能                    [in_progress]  ← 上一次 todo
  [2] task_003  实现 JWT 中间件                 [done]         ← 上一次 in_progress

选择要回退的 task：1

✓ task_001 已从 in_progress 回退到 todo
   （记录：2026-09-05 14:30 上次状态为 todo）

# 也支持模糊匹配
$ kron task back "登录"
✓ task_001 已从 in_progress 回退到 todo
```

**回退规则**：

| 当前状态 | 回退到 |
|---------|--------|
| `in_progress` | `todo` |
| `done` | `in_progress`（保留中间过程）|

---

##### 状态捷径命令（更直观）

```bash
# 三个状态捷径命令（让命令更自然）
$ kron task start "登录"          # 等价于 move "登录" in_progress
$ kron task done "登录"           # 等价于 move "登录" done
$ kron task todo "登录"           # 等价于 move "登录" todo

# 这些是命令别名（让 AI 和人类都好记）
```

**为什么需要这些捷径**：

- ✅ `start` 比 `move ... in_progress` 更直观
- ✅ `done` 比 `move ... done` 更自然
- ✅ AI 调用时更友好

---

##### 批量操作（高级）

**按 tag 批量**：

```bash
# 把所有 #后端 的 task 标记为 done
$ kron task move --tag "#后端" done

找到 3 个匹配的 task：
  task_001  实现登录功能        [#后端, #认证]
  task_002  设计数据库表结构    [#后端, #数据库]
  task_003  实现 JWT 中间件     [#后端, #认证]

确认批量操作？(y/N): y
✓ 3 个 task 已移动到 done
```

**根据 git diff 自动推荐**：

```bash
$ kron task move --from-git-diff

当前 git diff 修改的文件：
  src/auth/login.rs
  src/auth/oauth.rs
  src/middleware/jwt.rs

找到可能相关的 task：
  task_001  实现登录功能  [todo]            ← 匹配 login.rs, oauth.rs
  task_003  实现 JWT 中间件  [in_progress]   ← 匹配 jwt.rs

选择要移动的 task：1
选择目标状态：in_progress

✓ task_001 已移动到 in_progress
```

---

##### AI 工具调用示例

**Cursor / Claude Code 调用**：

```python
# Cursor 的 AI 助手可以这样调用
result = subprocess.run(
    ['kron', 'task', 'move', '登录', 'in_progress'],
    capture_output=True, text=True
)
# 输出：✓ task_001 状态已更新为 in_progress
```

**AI 工具的使用流程**：

```
AI 工具读取 KRON/VERTEX/开发/tasks.md
    ↓
自然理解 task 内容
    ↓
用户说"把登录功能标记为进行中"
    ↓
AI 调用：kron task move "登录" in_progress
    ↓
Kron 更新 state
    ↓
AI 工具得到反馈
```

**AI 调用模板**（给 AI 的提示）：

```
你有以下工具可以操作 Kron task 状态：
- kron task move <关键词> <状态>
- kron task back <关键词>
- kron task start/done/todo <关键词>

示例：
- 把"登录"标为进行中：kron task move "登录" in_progress
- 完成"数据库"：kron task done "数据库"
```

---

##### 命令设计哲学 ⭐⭐⭐⭐⭐

**4 个设计原则**：

| 原则 | 实现 |
|------|------|
| **不需要记 ID** | 模糊匹配 + 交互式选择 |
| **AI 友好** | 一行命令完成，输出可读 |
| **人类友好** | 交互式选择器 + 捷径命令 |
| **幂等安全** | 状态转换有记录，可回退 |

**为什么不强制要 task ID**：

- ❌ AI 工具不知道完整 ID
- ❌ 用户也记不住 task_001 这种 ID
- ✅ 模糊匹配 + 关键词搜索 = 直观

**为什么不强制要 state 参数**：

- ❌ `move ... in_progress` 记不住状态名
- ✅ 交互式选择更友好
- ✅ 提供 `start/done/todo` 捷径更自然

---

##### 状态转换的命令 vs 直接编辑 MD

**什么时候用命令**：

- ✅ 改变 task 状态（move / back / start / done / todo）
- ✅ 批量操作
- ✅ AI 工具调用（无需 GUI）

**什么时候直接编辑 MD**：

- ✅ 修改 description
- ✅ 修改 tags
- ✅ 添加新 task
- ✅ 删除 task

**核心原则**：

- **状态变化** → 用命令（state 不在 MD 中）
- **内容编辑** → 直接编辑 MD

---

#### 2.2.3 Vertex description.md（新增）

**位置**：

```
KRON/VERTEX/<vertex_name>/
├── description.md         ← Vertex 描述（阶段意图）⭐新增
├── tasks.md               ← 所有 task
└── _meta.json             ← Vertex 绑定点
```

**为什么要有 description.md**：

- ✅ Task 是"做什么"
- ✅ Vertex 描述是"为什么有这个阶段"
- ✅ 两者分离，意图清晰
- ✅ AI 读起来能理解阶段目的

**格式（完全自由）**：

```markdown
# Vertex: 开发

## 阶段意图

实现用户登录功能模块。

## 目标

- 抽象 AuthProvider 接口
- 支持三种登录方式
- 完整的 session 管理

## 备注

参考 `legacy-auth/` 项目。
```

**Kron 怎么用 description.md**：

- 在 GUI 中显示（让用户看到这个 Vertex 的意图）
- 在 task 上方显示（让用户理解阶段背景）
- 不参与任何逻辑计算
- Kron 不解析（仅同步到 GUI 显示）

---

#### 2.2.4 Vertex 删除的处理

**删除 Vertex 时**：

- 弹窗询问："删除'开发' Vertex 会同时删除其下所有 task，是否继续？"
- 选项：
  - **Vertex + Task 一起删**（默认，最简洁）
  - **取消**

（暂不提供"仅删 Vertex 保留 task"选项，避免孤儿 task）

---

**设计原则**：**简洁 + AI 易读**，保障项目可持续性。

Kron 的所有设计都遵循这两个原则：
- **简洁**：不过度设计，不增加用户负担
- **AI 易读**：大部分 AI 工具只能阅读工作文件夹内的文件，task 内容必须是 AI 可直接读懂的纯文本 MD

##### 2.3.1 Task 属性清单（极致精简）

**写入 MD 文件的字段**：

| 属性 | 类型 | 必填 | 说明 |
|------|------|------|------|
| **id** | string | ✅ | 全局唯一 ID，Kron 自动生成 |
| **title** | string | ✅ | task 标题（一句话说明） |
| **description** | Markdown | ✅ | task 详细描述（详见 2.3.2） |
| **updated_at** | datetime | ✅ | 最后修改时间 |

**Kron 内部维护的字段**（不写入 MD）：

| 属性 | 类型 | 存储位置 | 说明 |
|------|------|---------|------|
| **state** | enum | `states/<vertex>.json` | todo / in_progress / done |
| **tags** | string[] | `tags/tasks.json` | 用户自定义标签 |
| **vertex** | string | 文件路径推导 | 所属 Vertex |
| **created_at** | datetime | Kron 内部 | 创建时间 |
| **started_at** | datetime | Kron 内部 | 进入 In Progress 的时间 |
| **completed_at** | datetime | Kron 内部 | 进入 Done 的时间 |

**有意不设计的属性**：
- ❌ **priority** — 任务重要性用户自己清楚
- ❌ **due_date** — 截止日期由外部项目管理
- ❌ **estimate** — 个人项目不需要
- ❌ **related_files** — AI 通过代码自己发现
- ❌ **related_tasks** — 通过 description 自然表达
- ❌ **git_branch / git_commits** — Git 由 AI 自行获取

**极致精简的好处**：

- ✅ MD 文件**极简**：只有 4 个字段
- ✅ AI 读 MD 时**只看到核心内容**
- ✅ 维护成本**最低**
- ✅ 真正重要的内容写在 description 里

##### 2.3.2 Task 描述（description）是核心 ⭐⭐⭐

**description 是 task 最核心的字段**，因为：

1. **AI 易读**：description 是纯文本，AI 工具直接读
2. **项目知识沉淀**：description 累积成项目文档
3. **决策追溯**：记录"为什么"比"做什么"更重要
4. **可自由修改**：用户在 Kron / Cursor / Claude Code 中自由编辑

**description 格式**（极简）：

```markdown
## 背景

为什么要做这个 task？解决什么问题？

## 目标

完成后达到什么效果？

## 实现方案

具体怎么做？关键步骤、关键决策。

## 备注

其他需要记录的信息。
```

**关键设计**：**description 允许自由修改，不用追加** ⭐

- ❌ 不做"AI 优化追加"（破坏文件可读性）
- ✅ 用户 / AI 工具**直接修改 description**
- ✅ 仅保留**最后一次修改时间**（updated_at）
- ✅ 简洁：保存修改时间即可，不保留历史（历史在 Git 中）

**description 长度**：

- 无硬性限制
- 鼓励写**简洁、有信息量**的内容

##### 2.3.3 tags 是 AI 易读的标签 ⭐

**tags 的设计**：

- **类型**：`string[]`（数组）
- **格式**：用户自由定义（如 `["#后端", "#认证", "#feature"]`）
- **意义**：**让 AI 更易读、更好理解 task 上下文**
- **存储在 MD 中的 tags 行** + Kron 内部 `tags/tasks.json`（同步）

**核心设计**：**tags 直接在 MD 中编辑，不使用 CLI** ⭐

```markdown
## [task_001] 实现登录功能

### 背景
...

tags: #后端 #认证 #feature
```

**操作方式**：

- ❌ 之前：`kron task tag add task_001 "#后端"`
- ✅ 现在：直接在 MD 中编辑 `tags:` 行

**Kron 解析逻辑**：

1. 找到 task 标题 `## [task_001] ...`
2. 在 task 内容中找到 `tags: ` 开头的行
3. 提取 `#xxx` 形式的标签
4. 同步到 Kron 内部 `tags/tasks.json`

**tags 行格式规则**：

- ✅ 单行：`tags: #tag1 #tag2 #tag3`
- ❌ 多行：不允许
- ❌ 其他格式：Kron 解析失败

**AI 如何使用 tags**：

- 通过 tags 快速分类（前端 / 后端 / bug / feature）
- 通过 tags 理解优先级（如果用户用 #P0 / #P1 约定）
- 通过 tags 关联其他 task

##### 2.3.4 Task MD 格式规范（最简版）⭐⭐⭐⭐⭐

**tasks.md 完整示例**（用户/AI 直接编辑）：

```markdown
## [task_001] 实现登录功能

### 背景

用户登录是系统入口，需要支持多种登录方式。

### 目标

支持邮箱 + 手机号 + OAuth 登录。

### 实现方案

1. 抽象 AuthProvider 接口
2. 实现三种 Provider
3. JWT token 签发与刷新

### 备注

参考 `legacy-auth/` 项目。

tags: #后端 #认证 #feature

---

## [task_002] 设计数据库表结构

### 背景

认证模块依赖基础数据表。

### 目标

用户表、会话表、权限表。

tags: #后端 #数据库
```

**格式规则（极简）**：

| 元素 | 格式 | 示例 |
|------|------|------|
| Task 标题 | `## [task_XXX] 标题` | `## [task_001] 实现登录功能` |
| Task 内容 | 二级标题（### 背景 / 目标 / 方案 / 备注） | `### 背景` |
| Tags | 一行 `tags: #tag1 #tag2` | `tags: #后端 #认证` |
| Task 分隔 | `---` | `---` |

**关键规则**：

- ✅ **task ID 格式**：`task_<数字>`（Kron 自动识别）
- ✅ **标题前缀**：`## [task_XXX]`（Kron 自动识别）
- ✅ **tags 行**：单行 `tags: #...`（Kron 自动解析）
- ✅ **内容自由**：除 tags 行外，用户/AI 完全自由编辑
- ❌ **不强求** 4 个二级标题（用户可写任意内容）
- ❌ **不存 state 字段**（状态仅在 GUI 中）
- ❌ **不存 updated_at 等元数据**（Kron 内部维护）

**Vertex 文件夹结构**：

```
KRON/VERTEX/开发/
├── description.md         ← Vertex 描述（自由格式）
├── tasks.md               ← 所有 task（极简格式）
└── _meta.json             ← Vertex 绑定点
```

**字段对照表**：

| 字段 | MD 中 | Kron 内部 |
|------|-------|----------|
| `id` | ✅（标题） | ✅ |
| `title` | ✅ | ✅ |
| `description`（背景/目标/...） | ✅ | ❌ |
| `tags`（从 tags 行解析） | ✅ | ✅ |
| `state` | ❌ | ✅ |
| `updated_at` | ❌ | ✅ |

##### 2.3.5 Task 字段管理（清晰分工）

**Kron 自动管理**（用户无需关心）：
- `id`：自动生成
- `updated_at`：自动记录（每次修改自动更新）
- `state`：仅 GUI 中跟踪，5 分钟同步到内部 JSON
- `created_at` / `started_at` / `completed_at`：自动记录（存 Kron 内部）

**用户在 MD 文件中写**（AI 可直接读）：
- `title` / `description`
- 这两个字段都是**纯文本**，AI 工具直接读 `tasks.md` 即可

**Kron 内部管理（不写入 MD）**：
- `state` / `tags`
- 用户通过 GUI / CLI 操作
- AI 工具**不直接接触**（避免污染 MD）

**完全交给用户决定**：
- description 结构（不强求二级标题）
- tags 格式（用户完全自定义）

##### 2.3.6 Task ID 格式

**格式**：`task_<序号>`（最简洁）

**示例**：
- `task_001`
- `task_002`
- `task_999`

**为什么不用时间戳**：
- 项目级序号即可（每个项目独立编号）
- 更简洁，人类可读
- 时间戳由 Kron 内部记录（不在 MD 中）

##### 2.3.7 Task 状态转换 ⭐

**状态仅存 Kron 内部，不写入 MD**

**合法转换**：

```
todo → in_progress → done
            ↓
            done（直接完成）
            ↓
            todo（重新打开）

done → in_progress（重新开始）
in_progress → todo（暂停）
```

**时间字段自动更新**（存储在 Kron 内部）：

| 转换 | 自动更新 |
|------|---------|
| todo → in_progress | `started_at = now()` |
| in_progress → done | `completed_at = now()` |
| * → todo | 清空 `started_at` 和 `completed_at` |

**用户操作（三种方式）**：

| 方式 | 命令 / 操作 | 是否修改 MD |
|------|------------|------------|
| **GUI 拖拽** | 拖动 task 到不同列 | ❌ 不修改 |
| **CLI** | `kron task move <id> <state>` | ❌ 不修改 |
| **AI 工具** | ❌ AI 不应修改状态（仅改 description） | ❌ 不修改 |

##### 2.3.8 Kron 内部 Task 元数据 ⭐

**为什么不写在 MD 文件中**：

- MD 文件只保留人类/AI 友好的内容
- 时间戳、ID 等元数据放在 Kron 内部
- AI 工具不依赖这些元数据（AI 通过 description 即可理解）

**Kron 内部存储**：

```
<kron_install_dir>/data/projects/<project_hash>/
└── tasks/
    └── <task_id>.json
```

**`<task_id>.json` 内容**：

```json
{
  "id": "task_001",
  "state": "in_progress",
  "vertex": "开发",
  "created_at": "2026-09-04T10:00:00Z",
  "updated_at": "2026-09-04T20:00:00Z",
  "started_at": "2026-09-04T15:00:00Z",
  "completed_at": null,
  "md_file": "KRON/VERTEX/开发/IN_PROGRESS.md"
}
```

**双源同步**：

- MD 文件（项目内）：人类/AI 读取
- JSON 文件（Kron 内部）：Kron 内部管理
- 任意一个被修改 → 同步另一个
- 详见需求 5（双源同步）

##### 2.3.9 不做内置 AI 辅助功能 ⭐⭐⭐

**重要决策**：**Kron 不做内置 AI 辅助功能**（如"AI 生成描述"、"AI 优化描述"等）。

**原因**：

1. **烂大街的 AI 功能**：所有任务管理工具都在做这些，没有差异化价值
2. **AI 工具已足够强大**：Cursor / Claude Code 等可以直接读 MD 文件并修改
3. **避免过度集成**：Kron 不绑死某个 AI 服务
4. **保持简洁**：不引入 AI SDK / API Key 等复杂性

**Kron 如何支持 AI 工作流**：

```
┌────────────────────────────────────────┐
│  AI 工具如何读懂项目？                  │
├────────────────────────────────────────┤
│                                        │
│  1. AI 读取 KRON/README.md             │
│     → 理解项目总体说明                  │
│                                        │
│  2. AI 读取 KRON/VERTEX/*/IN_PROGRESS.md│
│     → 理解当前在做什么                  │
│                                        │
│  3. AI 读取 KRON/important/            │
│     → 读取重要 idea / 设计文档          │
│                                        │
│  4. AI 读取 KRON/.kron-context/        │
│     → 读取"中间文档"（详见 2.4）        │
│                                        │
│  5. AI 自动完整理解项目                 │
│                                        │
└────────────────────────────────────────┘
```

**设计哲学**：

- Kron 提供**结构化的、易读的 MD 文件**
- AI 工具**自然读取**这些文件
- 用户在 Cursor / Claude Code 中**直接修改 MD 文件**
- Kron 检测到文件变化 → 同步到 Kron 内部
- AI 工具 = 用户的助手（与 Kron 解耦）

**Kron 与 AI 工具的边界**：

| 职责 | Kron | AI 工具 |
|------|------|---------|
| 任务数据存储 | ✅ | ❌ |
| 文件结构 | ✅ | ❌ |
| 状态管理 | ✅ | ❌ |
| AI 生成描述 | ❌ | ✅ |
| AI 优化文本 | ❌ | ✅ |
| AI 补充背景 | ❌ | ✅ |
| AI 修改 MD | ❌ | ✅ |
| 双源同步 | ✅ | ❌ |

**Kron = 数据管理层**，**AI 工具 = 智能处理层**，**两者解耦**。

---

#### 2.4 中间文档（CLI 提供，让 AI 进一步理解项目）⭐⭐⭐⭐

**背景**：

- 大部分 AI 工具只能在工作文件夹内阅读文件
- 但项目代码历史（git log、git diff）、依赖关系、API 文档等中间产物对 AI 很重要
- 这些文档通常很大、很杂，不适合手动维护

**方案**：**Kron CLI 提供命令生成"事实型结构快照"**，放到 `KRON/.kron-context/` 目录，AI 自然读取。

> **⚠️ 范围声明（2026-09-05 修正，与 04b § 3.7 一致）**：
> - Kron 只生成**机器可重现的事实数据**——git log、文件树、依赖清单、分支对比
> - **不做语义理解**——不调 LLM、不生成"项目是干啥的"这种内容
> - **AI 真正理解项目靠三件事**：（1）根 README.md；（2）`KRON/important/` 人类文档；（3）AI 自己读源码
> - `.kron-context/` 是**锦上添花**（省去 AI 反复跑 `git log`），不是"AI 上手银弹"

##### 2.4.1 中间文档目录结构（v1 锁定 4 个文件）

```
KRON/.kron-context/
├── README.md                    ← 中间文档索引（说明每个文件的用途 + 明确范围边界）
├── git/
│   └── recent-commits.md        ← 最近 N 次 commit（事实型，N 默认 100）
└── code/
    └── structure.md             ← 代码目录结构（树状，深度 3，跳过 node_modules/target/.git）
```

**v1 不实现**（避免过度膨胀，详见 04b § 3.7 Q26）：
- ~~`branch-summary.md`~~ → **已实现**（并入 v1 清单）
- ~~`file-history.md`~~（v2）
- ~~`dependencies.md`~~（需多语言解析器，v2）
- ~~`languages.md`~~（tokei 依赖重，v2）
- ~~`public-apis.md`~~（v2）
- ~~`important-files.md`~~（已在 `important/` 内，AI 直接读更准）

**目录在 `.kron-context/` 而非 `important/` 的原因**：

- `important/`：用户主动添加的**重要 idea / 设计文档**（长期保留）
- `.kron-context/`：Kron 自动生成的**事实型快照**（可随时重新生成）

##### 2.4.2 CLI 命令清单

```bash
# 默认：增量更新过期的中间文档
kron context

# 完整重新生成（慎用，耗时）
kron context --regenerate

# 列出所有中间文档及过期状态
kron context --list
```

**自动调用方**：
- Kron 后台守护进程（默认 + 唯一）
- 用户手动
- AI 工具

##### 2.4.3 中间文档详细设计

**`git/recent-commits.md`**：

```markdown
# 最近 100 次 Commit

生成时间：2026-09-04T20:00:00Z

- 2026-09-04 19:30 | abc1234 | feat: 实现登录功能
- 2026-09-04 18:45 | def5678 | fix: 修复 token 刷新 bug
- [...]
```

**AI 如何使用**：
- AI 读到 `git/recent-commits.md` → 立刻理解项目最近做了什么
- 比让 AI 执行 `git log` 更高效（AI 不需要执行命令，直接读文件）

**`code/structure.md`**：

```markdown
# 代码结构

src/
├── auth/
│   ├── login.ts        ← 用户登录
│   ├── session.ts      ← 会话管理
│   └── oauth.ts        ← OAuth 集成
├── api/
│   ├── users.ts        ← 用户 API
│   └── posts.ts        ← 文章 API
└── utils/
    └── logger.ts       ← 日志工具
```

**`code/dependencies.md`**：

```markdown
# 项目依赖

## package.json dependencies

- react: ^18.2.0
- express: ^4.18.0
- jsonwebtoken: ^9.0.0

## package.json devDependencies

- typescript: ^5.0.0
- jest: ^29.0.0
```

**`code/important-files.md`**（与 KRON/important/ 不同）：

```markdown
# 重要文件清单

按修改频率 + 文件重要性排序：

- src/auth/login.ts (修改 23 次)
- src/api/users.ts (修改 18 次)
- README.md (修改 5 次)
- package.json (修改 3 次)
```

##### 2.4.4 自动生成与更新 ⭐⭐⭐⭐

**核心设计**：**由 Kron 后台守护进程自动管理，不依赖 Git Hooks**。

**生成时机**：

| 触发 | 行为 |
|------|------|
| **Kron 后台守护进程**（默认 + 唯一） | 文件变化时自动增量更新 |
| **Kron 后台守护进程**（定时） | 每 5 分钟扫描过期文档 |
| **用户手动** | `kron context` |

**默认行为（无 Git 也能工作）**：

```
Kron 后台进程启动
    ↓
监听文件变化（notify crate）
    ├─ KRON/VERTEX/*/*.md 变化 → 增量更新中间文档
    ├─ 项目根代码变化 → 增量更新结构文档
    └─ 项目根依赖文件变化 → 增量更新依赖文档
    ↓
定时任务（每 5 分钟）
    └─ 检查 .kron-context/ 过期情况
    ↓
用户无感知（后台静默）
```

**自动调用方**：

- ✅ Kron 后台守护进程（默认 + 唯一）
- ✅ 用户手动
- ✅ AI 工具

**注意**：

- ❌ **不再使用 Git Hooks**（Kron 自己监听）
- ❌ 不再生成 `.git/hooks/post-*` 脚本
- ✅ 一个统一机制，简洁清晰

**优点**：
- ✅ **不依赖 Git**（核心：默认靠守护进程）
- ✅ **实时性更好**（文件保存即触发）
- ✅ **无需任何配置**（用户零感知）

##### 2.4.5 中间文档的过期机制

**每个中间文档都有"过期时间"**：

| 文档类型 | 过期时间 |
|---------|---------|
| git/recent-commits.md | 1 小时 |
| code/structure.md | 24 小时 |
| code/dependencies.md | 1 小时 |
| code/important-files.md | 24 小时 |
| api/public-apis.md | 24 小时 |

**过期处理**：
- 过期后下次读取时提示 AI"文档已过期"
- AI 可自行调用 `kron context` 更新（如果 AI 工具有权限执行命令）
- **默认由 Kron 后台守护进程自动更新**（用户无需手动）

**`README.md` 中的说明**：

```markdown
# .kron-context/

Kron 自动生成的项目上下文（由后台守护进程自动维护）。

## 使用方法

- AI 工具可读取本目录所有文件了解项目
- 文件过期时会被守护进程自动刷新（无需用户干预）

## 文档清单

- `git/recent-commits.md` - 最近 100 次 commit（过期：1 小时）
- `code/structure.md` - 代码目录结构（过期：24 小时）
- `code/dependencies.md` - 项目依赖（过期：1 小时）
- `code/important-files.md` - 重要文件清单（过期：24 小时）
- `api/public-apis.md` - 公开 API 摘要（过期：24 小时）

## 手动更新

```bash
kron context              # 增量更新
kron context --regenerate # 完整重新生成
```
```

##### 2.4.6 为什么"中间文档"对 AI 友好

**问题**：AI 工具只能读工作文件夹，但项目信息分散在：

- Git 历史（git log / git diff）
- 代码结构（需要 tree 命令）
- 依赖（package.json / Cargo.toml 等）
- 重要文件（需判断哪些重要）

**解决方案**：把这些信息**预先汇总成 MD 文件**，AI 直接读即可。

**类比**：
- 没有 `.kron-context/`：AI 像一个新员工，要自己摸索项目
- 有 `.kron-context/`：AI 像有 onboarding 文档的新员工，快速上手

**对 AI 工作流的改变**：

```
无 .kron-context/：
AI 启动 → 执行 git log → 执行 tree → 读取 package.json
→ 自己整合信息 → 开始工作（耗时）

有 .kron-context/：
AI 启动 → 读取 KRON/README.md + .kron-context/git/recent-commits.md
+ .kron-context/code/structure.md + .kron-context/code/dependencies.md
→ 立刻理解项目 → 开始工作（高效）
```

##### 2.4.7 与 `KRON/important/` 的关系

**`KRON/important/`**：
- 用户**主动添加**的重要 idea / 设计文档
- 长期保留
- AI 读取后理解项目**关键决策**

**`KRON/.kron-context/`**：
- Kron **自动生成**的项目上下文
- 可重新生成
- AI 读取后理解项目**当前状态**

**协同工作**：

```
AI 完整理解项目 = 
    KRON/README.md          (项目说明)
  + KRON/VERTEX/*/TODO.md   (待办任务)
  + KRON/VERTEX/*/IN_PROGRESS.md (进行中任务)
  + KRON/important/         (重要 idea / 设计)
  + KRON/.kron-context/     (项目当前状态)
```

**AI 完美工作流的文件清单**：

```
项目根/
├── README.md                    ← 项目总体说明
├── src/                         ← 项目源代码
└── KRON/
    ├── README.md                ← KRON 项目说明（AI 第一眼读）
    ├── vertices.json            ← Vertex 定义
    ├── VERTEX/
    │   ├── 开发/
    │   │   ├── TODO.md          ← AI 读：待办任务
    │   │   ├── IN_PROGRESS.md   ← AI 读：进行中任务
    │   │   └── DONE.md          ← AI 读：已完成任务（历史）
    │   └── ...
    ├── important/               ← AI 读：重要 idea / 设计
    │   ├── api-spec-v1.md
    │   └── architecture-decision.md
    └── .kron-context/           ← AI 读：项目当前状态
        ├── git/recent-commits.md
        ├── code/structure.md
        └── code/dependencies.md
```

**AI 启动后只需读取这 ~10 个文件，即可完整理解项目**。

---

#### 2.5 notes/ 的归属追踪

**问题**：用户可能：
- 删除某个 notes/ 下的文件
- 某些 notes/ 文件在其他分支
- 切换分支后 notes/ 内容不一致

**解决方案**：每个 Vertex 下的 `notes/` 需要有 `_meta.json` 记录原始文件位置：

```json
// KRON/VERTEX/开发/notes/_meta.json
{
  "files": [
    {
      "id": "f_001",
      "original_path": "./docs/api-spec-v1.md",    // 项目根的相对路径
      "current_path": "./KRON/VERTEX/开发/notes/api-spec-v1.md",
      "added_at": "2026-09-04T20:00:00Z",
      "backup_path": "D:/Apps/Kron/data/projects/abc/vertex-dev/notes/api-spec-v1.md"
    }
  ]
}
```

**关键设计**：
- `original_path`：原始位置（项目根相对路径）
- `current_path`：当前位置（在 KRON/VERTEX/xxx/notes/ 下）
- `backup_path`：Kron 内部重要文件夹中的备份地址

**功能**：
- 用户删除文件 → Kron 检测到 → 可恢复（从 backup_path 恢复）
- 切换分支导致文件缺失 → Kron 检测到 → 可恢复
- 用户可以查看每个文件的"原始位置"

---

### 需求 3：项目级 AI 可读文档库 ⭐ 重要

#### 3.1 KRON/ 目录结构

```
KRON/
├── README.md                          ← 项目顶层说明（AI 友好）
├── ARCHITECTURE.md                    ← 架构文档（可选）
├── vertices.json                      ← Vertex 绑定关系
│
├── VERTEX/                            ← 所有 Vertex
│   ├── 需求分析/                       ← Vertex 1
│   │   ├── TODO.md
│   │   ├── IN_PROGRESS.md
│   │   ├── DONE.md
│   │   ├── notes/                     ← 该阶段的归档文档
│   │   │   ├── api-spec-v1.md
│   │   │   └── _meta.json             ← 该 Vertex 文件元数据
│   │   └── _meta.json                 ← 该 Vertex 状态元数据（可选）
│   └── 开发/
│       └── ...
│
├── important/                         ← 重要文件夹（双源备份）⭐
│   ├── api-spec-v1.md
│   ├── key-config.json
│   └── config/
│       └── prod.json
│
└── .kron-context/                     ← Kron 自动生成的项目上下文 ⭐⭐⭐
    ├── README.md                      ← 中间文档索引
    ├── git/
    │   ├── recent-commits.md
    │   ├── branch-summary.md
    │   └── file-history.md
    ├── code/
    │   ├── structure.md
    │   ├── dependencies.md
    │   └── important-files.md
    └── api/
        └── public-apis.md
```

**两类上下文文件**：

| 位置 | 内容 | 用途 |
|------|------|------|
| `KRON/important/` | 用户主动添加的**重要 idea / 设计文档** | 让 AI 理解项目**关键决策** |
| `KRON/.kron-context/` | Kron 自动生成的**项目上下文** | 让 AI 理解项目**当前状态** |

详细设计：
- `important/`：见需求 4（重要文件夹）
- `.kron-context/`：见需求 2.4（中间文档）

#### 3.2 文档管理设计简化

**不要 docs/ 强制目录**：
- 用户想记录文档 → 用 VSCode 在项目任何位置创建
- 想被 Kron 管理 → 通过 `kron important add <path>` 添加
- 添加后自动双源备份

**front matter 约定**（可选）：
```markdown
---
title: API 设计文档
status: draft              # temp / draft / final
vertex: 开发                # 归属 vertex
created: 2026-09-04
updated: 2026-09-04
---

# API 设计文档
...
```

**重要**：front matter 不是强制的，Kron 主要通过 `_meta.json` 管理归属。

---

### 需求 4：重要文件夹（双源备份）⭐

#### 4.1 设计目的

**问题场景**：
- 项目中有一些**关键文档/配置文件**很重要
- 例如：API 规范文档、关键配置文件、不能丢的文档
- 这些文档可能不属于任何 Vertex，但需要长期保留
- 需要**双源备份**（项目内 + Kron 内部）以防丢失

#### 4.2 重要文件存储路径

**用户可以指定与项目根的相对路径**：

```bash
# 添加重要文件
kron important add ./docs/api-spec-v1.md
# → 复制到 KRON/important/api-spec-v1.md
# → 备份到 Kron 内部 <kron_install_dir>/data/projects/<hash>/important/api-spec-v1.md

# 添加重要文件夹
kron important add ./config/
# → 复制到 KRON/important/config/
# → 备份到 Kron 内部 <kron_install_dir>/data/projects/<hash>/important/config/
```

**优点**：
- ✅ 不强制 docs/ 目录
- ✅ 用户可以在任何位置使用重要文件功能
- ✅ Kron 自动处理路径冲突

#### 4.2.1 Kron 安装区域设计（软件 + 数据分离）⭐⭐⭐⭐⭐

**核心原则**：**软件本体和数据分开存储** ⭐⭐⭐⭐⭐

**关键决策**（根据用户反馈）：

| 决策 | 说明 |
|------|------|
| ✅ 软件和数据**放在同一父目录** | `D:\Apps\Kron\software\` 和 `D:\Apps\Kron\data\` |
| ✅ data 目录**包含所有项目数据** | `data/<项目名>/KRON/` |
| ✅ 软件目录**保持纯净** | 只包含可执行文件 + 资源 |
| ✅ 默认安装位置 `D:\Apps\Kron\` | 非系统盘 |
| ✅ 分发方式：**下载 zip 解压即用** | 便携，无安装器 |

---

##### Kron 安装目录结构（最终版）⭐

```
D:\Apps\Kron\                           ← Kron 安装根目录（用户可见）
│
├── software\                            ← 软件本体 ⭐ 纯净，只含可执行文件
│   ├── kron.exe                         ← GUI 主程序
│   ├── kron-cli.exe                     ← 命令行工具
│   ├── kron-daemon.exe                  ← 后台守护进程
│   ├── assets/                          ← 静态资源（图标、主题等）
│   └── libs/                            ← 共享库（如需）
│
└── data\                                ← 数据根目录 ⭐ 与软件分离
    │
    ├── global\                          ← Kron 全局配置
    │   ├── config.toml                  ← Kron 全局配置
    │   ├── install_path.lock            ← 安装路径自寻
    │   └── projects.json                ← 已注册项目列表
    │
    ├── projects\                        ← 所有项目数据 ⭐⭐⭐
    │   │
    │   ├── kron-self\                   ← Kron 自身开发项目
    │   │   └── KRON/                    ← 项目内数据（双源之一）
    │   │       ├── README.md
    │   │       ├── VERTEX/
    │   │       │   └── 开发/
    │   │       │       ├── description.md
    │   │       │       ├── tasks.md
    │   │       │       └── _meta.json
    │   │       ├── important/
    │   │       └── .kron-context/
    │   │
    │   └── <其他项目>/
    │       └── KRON/                    ← 项目内数据
    │
    └── backups\                         ← 全局备份
        └── backups\                         ← 全局快照备份（仅最新，不保留历史版本）
            ├── auto-backup-2026-09-05\      ← 每日自动快照（保留最近 7 天）
            └── manual-backup-2026-09-03\    ← 用户手动快照（永久保留）
```

---

##### 关键设计：软件与数据分离的好处

| 好处 | 说明 |
|------|------|
| ✅ **软件升级不影响数据** | 升级只需替换 `software/`，数据无变化 |
| ✅ **数据备份/迁移简单** | 只需备份 `data/` 目录 |
| ✅ **多版本并存** | 可同时安装多个 Kron 版本（如 `software-v1/` `software-v2/`）|
| ✅ **软件目录保持纯净** | 删除 `software/` 即可干净卸载 |
| ✅ **数据可视化** | 用户清楚看到数据在哪（`D:\Apps\Kron\data\projects\`）|

---

##### 默认安装位置

**推荐位置**（按优先级）：

| 优先级 | 路径 | 原因 |
|--------|------|------|
| 1 | `D:\Apps\Kron\` | 非系统盘，空间充足 |
| 2 | `C:\Apps\Kron\` | 系统盘但分离目录 |
| 3 | 用户自定义 | 灵活 |

**为什么不使用 Windows 用户目录**：

| 问题 | 后果 |
|------|------|
| 用户账户变化（如更换账户） | 路径失效，需手动迁移 |
| 系统重装 | `C:\Users\<user>\` 数据全部清空 |
| 多账户共享电脑 | 每个账户有独立数据 |
| 路径含中文用户名 | 部分工具兼容性问题 |
| OneDrive 干扰 | 用户目录可能被自动同步到云端 |
| `%AppData%` 等隐藏目录 | 用户不知道数据存在哪 |

---

##### 数据迁移 / 卸载

**迁移数据**：
- 移动整个 `D:\Apps\Kron\` 目录到新位置（如 `E:\Tools\Kron`）
- 无需任何配置改动
- Kron 自动识别新路径

**升级软件**（不影响数据）：

```bash
# 1. 下载新版本 zip
# 2. 解压到临时位置
# 3. 备份当前 software/
mv D:\Apps\Kron\software D:\Apps\Kron\software-old

# 4. 复制新 software/
cp new-software D:\Apps\Kron\software

# 5. 测试
kron status

# 6. 删除旧 software/
rm -rf D:\Apps\Kron\software-old
```

**卸载 Kron**：
- 询问用户是否保留数据
- 默认保留 `data/` 目录
- 用户可手动备份或删除

---

##### 安装路径自寻（已确定）⭐

**核心问题**：用户可能移动整个 Kron 目录到新位置。

**解决方案**：**启动时自动发现真实安装路径**。

**自寻逻辑**：

```
1. 用户启动 kron.exe
    ↓
2. Kron 获取自身进程的可执行文件路径
    ↓
3. 推导父目录：<exe_dir>/../
    ↓
4. 推导 data 目录：<parent>/../data/
    ↓
5. 验证 data/global/install_path.lock
    ↓
6. 比对：
   ├─ 一致 → 正常使用
   └─ 不一致 → 自动更新 install_path.lock
```

**关键设计**：

- ✅ **不依赖注册表**：避免权限问题
- ✅ **不依赖环境变量**：避免被修改
- ✅ **不依赖配置文件**：避免文件丢失就崩溃
- ✅ **可执行文件路径是绝对真相**

**用户场景**：

```
场景 1：用户移动整个 Kron 目录
├─ 旧：D:\Apps\Kron\
└─ 新：E:\Tools\Kron\
处理：Kron 自动适配，无需任何手动操作 ✓

场景 2：用户从 D 盘拷贝到 U 盘
├─ 旧：D:\Apps\Kron\
└─ 新：E:\USB\Kron\
处理：Kron 自动适配 ✓

场景 3：用户只删除 software/，保留 data/
├─ 旧：D:\Apps\Kron\software\（已删）
└─ 新：D:\Apps\Kron\software-v2\（新软件）
处理：Kron 自动找到 data/，无需手动指定 ✓
```

---

##### 项目数据存储结构（核心）

**每个项目的 Kron 数据存储**：

```
D:\Apps\Kron\data\projects\<项目名>\
│
├── kron-internal/                       ← Kron 内部权威数据 ⭐
│   ├── config.json                      ← 项目配置（Vertex 绑定等）
│   ├── states/                          ← task 状态
│   │   ├── 开发.json
│   │   └── 重构.json
│   ├── tags/                            ← task tags
│   │   └── tasks.json
│   ├── important/                       ← 重要文件 Kron 内部备份
│   └── .kron-context/                   ← 中间文档缓存（可选）
│
└── KRON/                                ← 项目内 KRON/（AI 友好）⭐
    ├── README.md                        ← AI 第一眼读
    ├── VERTEX/
    │   └── 开发/
    │       ├── description.md
    │       ├── tasks.md
    │       └── _meta.json
    ├── important/
    │   └── api-spec-v1.md
    └── .kron-context/
        ├── git/recent-commits.md
        └── code/structure.md
```

**关键点**：

- ✅ **`<项目名>` 用人类可读的项目名**（不用 hash）
  - 如 `kron-self`、`auth-service`、`frontend-app`
  - 用户一眼知道是哪个项目
- ✅ **双源数据都在 `data/projects/<项目名>/` 下**
  - `kron-internal/`（Kron 权威源）
  - `KRON/`（AI 友好的项目内双源）
- ✅ **项目内 KRON/ 软链接到项目目录**（详见下方）

---

##### 项目识别机制 ⭐⭐⭐⭐⭐

**核心问题**：Kron 怎么知道"我在哪个项目"？

**解决方案**：**项目目录下的 KRON/ 软链接 / 注册表**

**方案 A：项目内 KRON/ 软链接（推荐）** ⭐

```
D:\code\my-project\                  ← 用户项目目录
├── src/
├── package.json
└── KRON/                            ← 软链接 → D:\Apps\Kron\data\projects\my-project\KRON\
    └── ...                          （实际上是软链接）
```

**初始化流程**：

```
1. 用户在项目目录运行：kron init
    ↓
2. Kron 生成项目名（默认用目录名，如 "my-project"）
    ↓
3. 创建数据目录：
   D:\Apps\Kron\data\projects\my-project\
   ├── kron-internal/
   └── KRON/                          ← 真实目录（不是软链接）
    ↓
4. 在项目目录创建软链接：
   D:\code\my-project\KRON → D:\Apps\Kron\data\projects\my-project\KRON\
    ↓
5. 完成！
   - AI 读 D:\code\my-project\KRON/ 自动看到所有数据
   - Kron 通过软链接识别项目
```

**软链接命令**：

```rust
// Windows
std::os::windows::fs::symlink_dir(
    "D:\\Apps\\Kron\\data\\projects\\my-project\\KRON",
    "D:\\code\\my-project\\KRON"
)?;

// Linux/macOS
std::os::unix::fs::symlink(
    "D:\\Apps\\Kron\\data\\projects\\my-project\\KRON",
    "D:\\code\\my-project\\KRON"
)?;
```

**优势**：

- ✅ 用户在项目目录直接看到 KRON/（不需跳到 data 目录）
- ✅ AI 工具读项目目录自动看到 KRON/
- ✅ 软链接方式，删除项目目录 = 删除 KRON（无遗留）
- ✅ 跨平台一致

**方案 B：全局注册表**

```
D:\Apps\Kron\data\global\projects.json
{
  "projects": [
    {
      "name": "my-project",
      "path": "D:\\code\\my-project",
      "kron_data_path": "D:\\Apps\\Kron\\data\\projects\\my-project"
    }
  ]
}
```

**当前目录检测**：

```
1. 用户在 D:\code\my-project\ 运行 kron status
    ↓
2. Kron 查找当前目录或父目录的 KRON/
    ↓
3. 找到 KRON/ → 通过软链接找到数据目录
    ↓
4. 返回项目信息
```

---

##### 项目名 vs 项目路径

**关键设计**：**项目名 = 人类可读的项目名**（不用 hash）

| 字段 | 值 | 说明 |
|------|------|------|
| **项目名** | `my-project` | 目录名（如 `my-project`） |
| **项目路径** | `D:\code\my-project` | 用户项目目录绝对路径 |
| **Kron 数据路径** | `D:\Apps\Kron\data\projects\my-project` | 自动生成 |

**项目名冲突处理**：

```
场景：用户在不同位置有同名项目
├─ D:\code\my-project\        (项目 A)
└─ D:\work\my-project\        (项目 B)

处理：
├─ 项目 A → D:\Apps\Kron\data\projects\my-project\
├─ 项目 B → D:\Apps\Kron\data\projects\my-project-2\
└─ 或用户自定义：kron init --name "my-project-work"
```

---

##### 多设备同步场景

**设备 A**（开发）：

```
D:\code\my-project\KRON\        (软链接)
    ↓ 真实路径
D:\Apps\Kron\data\projects\my-project\
```

**设备 B**（同一项目）：

```
D:\code\my-project\KRON\        (软链接)
    ↓ 真实路径
D:\Apps\Kron\data\projects\my-project\
```

**同步方式**：通过 Git 推送 KRON/ 目录到远程

```
设备 A：git add KRON/ && git commit && git push
设备 B：git pull
    ↓
项目内 KRON/ 自动同步
    ↓
kron restore --source remote  # 用项目内 KRON/ 覆盖 Kron 内部
```

**核心优势**：

- ✅ KRON/ 是纯文本 MD，易于 Git 同步
- ✅ 软链接方式，设备 B 拿到的是最新数据
- ✅ `kron restore` 一键恢复

---

##### 分发方式：下载 zip

**下载内容**：

```
kron-v0.1.0.zip
├── software/                     ← 软件本体
│   ├── kron.exe
│   ├── kron-cli.exe
│   ├── kron-daemon.exe
│   └── assets/
├── data/                         ← 空目录（首次启动初始化）
│   └── .gitkeep
└── README.md                     ← 安装说明
```

**安装步骤**（用户操作）：

```
1. 下载 kron-v0.1.0.zip
2. 解压到 D:\Apps\Kron\
3. （可选）将 D:\Apps\Kron\software\ 添加到 PATH
4. 运行 kron --version 验证安装
```

**为什么用 zip 而不是安装器**：

- ✅ 无需管理员权限
- ✅ 不会污染注册表
- ✅ 便携（可放到 U 盘）
- ✅ 升级 = 解压新 zip + 替换 software/
- ✅ 卸载 = 删除整个文件夹

---

##### 完整目录示例

```
D:\Apps\Kron\
├── software\                                  ← Kron v0.1.0
│   ├── kron.exe
│   ├── kron-cli.exe
│   ├── kron-daemon.exe
│   └── assets/
│       ├── icons/
│       └── themes/
│
├── data\
│   ├── global\
│   │   ├── config.toml
│   │   ├── install_path.lock
│   │   └── projects.json
│   │
│   ├── projects\
│   │   ├── kron-self\
│   │   │   ├── kron-internal\
│   │   │   │   ├── config.json
│   │   │   │   ├── states/
│   │   │   │   └── tags/
│   │   │   └── KRON/                          ← 软链接到项目目录
│   │   │
│   │   └── my-saas-app\
│   │       ├── kron-internal/
│   │       │   ├── config.json
│   │       │   ├── states/
│   │       │   └── tags/
│   │       └── KRON/                          ← 软链接到项目目录
│   │
│   └── backups\
│       └── auto-backup-2026-09-05\
│
└── README.md                                  ← 安装说明
```

**对应的项目目录**：

```
D:\code\my-saas-app\                           ← 用户项目目录
├── src/
├── package.json
└── KRON → D:\Apps\Kron\data\projects\my-saas-app\KRON\   ← 软链接
    ├── README.md                              ← AI 读
    ├── VERTEX/
    ├── important/
    └── .kron-context/
```

---

#### 4.3 important/ 结构和备份策略 ⭐⭐⭐

##### 重要文件的存储位置

```
D:\code\my-project\KRON\important\        ← 项目内（AI 友好）
    ↓ 软链接或复制
D:\Apps\Kron\data\projects\my-project\kron-internal\important\  ← Kron 内部（权威源）
```

**双源原则**：

- ✅ `KRON/important/`（项目内）— AI 工具直接读取
- ✅ `kron-internal/important/`（Kron 内部）— 备份 + 恢复

##### 备份策略：仅保留最新版 ⭐⭐⭐

**核心原则**：**同名字的只备份最新版，不保留历史版本** ⭐

**备份规则**：

| 场景 | 行为 |
|------|------|
| 首次添加文件 | 备份到 `kron-internal/important/` |
| 同名文件再次添加 | **覆盖**（不保留旧版本） |
| 文件被修改 | **覆盖**（不保留旧版本） |
| 文件被删除 | `kron-internal/` 中同步删除 |

**为什么不保留历史版本**：

- ✅ **简单**：无需管理版本号、时间戳
- ✅ **节省空间**：不会无限增长
- ✅ **Git 已有历史**：用户可用 Git 管理版本历史
  - `git log KRON/important/` 可查看文件修改历史
  - `git diff HEAD~10 KRON/important/api-spec.md` 可对比任意版本

**备份流程**：

```
用户将文件添加到 KRON/important/
    ↓
Kron 后台守护进程检测文件变化
    ↓
提取相对路径 + 文件内容
    ↓
写入 kron-internal/important/<相对路径>
    ↓
覆盖同名文件（如果有）
    ↓
记录操作日志
```

**示例**：

```
场景：用户多次更新 api-spec.md

1. 用户首次添加：KRON/important/api-spec.md
   → 备份到 kron-internal/important/api-spec.md

2. 用户修改后保存：KRON/important/api-spec.md
   → 覆盖 kron-internal/important/api-spec.md（旧版本丢失）

3. 用户想回滚旧版本？
   → 使用 git：git checkout HEAD~1 KRON/important/api-spec.md
```

##### 为什么重要文件不需要版本历史

**Kron 的设计哲学**：

- ❌ **不做 Git**：Kron 不是版本控制系统
- ✅ **利用 Git**：Kron 的备份依赖 Git 的版本历史
- ✅ **简化 Kron**：Kron 只做"备份最新"，不管理历史

**用户的完整回滚方案**：

```
想回滚重要文件到某个版本？
├─ 方案 1：Git
│   └─ git checkout <commit> KRON/important/<file>
│
└─ 方案 2：kron restore（使用项目内 KRON/ 恢复）
    └─ kron restore --source remote
```

**Git 的优势**：

- ✅ 精确到每次提交的修改历史
- ✅ 可对比任意两次提交的差异
- ✅ 可选择性地恢复某个文件
- ✅ 分布式，无需额外存储

##### 重要文件 vs 普通 task 文件的备份差异

| 文件类型 | 备份位置 | 备份策略 | 恢复方式 |
|---------|---------|---------|---------|
| **important/ 文档** | `kron-internal/important/` | 仅最新版 | `kron restore` |
| **VERTEX/*/tasks.md** | `kron-internal/` | 仅最新版 | `kron restore` |
| **VERTEX/*/description.md** | `kron-internal/` | 仅最新版 | `kron restore` |
| **states/*.json** | `kron-internal/states/` | 仅最新版 | 从快照恢复 |
| **tags/*.json** | `kron-internal/tags/` | 仅最新版 | 从快照恢复 |
| **.kron-context/** | ❌ 不备份 | 无 | 可重新生成 |

**为什么 .kron-context/ 不备份**：

- 这是 Kron 自动生成的中间文档
- 可随时重新生成
- 备份无意义

---

#### 4.4 同步策略

**双源双向同步**：
- 用户修改重要文件夹内文件 → Kron 检测到变化 → 同步双源
- 任一存在即可恢复（通过最后修改时间判断最新）

**同步时机**：

| 触发 | 行为 | 延迟 |
|------|------|------|
| **项目内文件变化** | 监听文件变动 → 同步到 Kron 内部 | 实时（< 1s） |
| **Kron 内部文件变化** | Kron 自动写回项目内 | 实时（< 1s） |
| **Kron 启动** | 检查双源差异 → 提示用户 | 启动时 |
| **用户手动触发** | `kron important sync` | 即时 |
| **冲突检测** | 比较修改时间 → 提示用户 | 实时 |

**同步方向判断**：

```
场景 A：仅项目内修改
├─ 项目内 mtime > Kron 内部 mtime
├─ 项目内有文件，Kron 内部没有
└─ 处理：项目内 → Kron 内部（单向同步）

场景 B：仅 Kron 内部修改
├─ Kron 内部 mtime > 项目内 mtime
├─ Kron 内部有文件，项目内没有
└─ 处理：Kron 内部 → 项目内（单向同步）

场景 C：双源都修改（冲突）
├─ 双源 mtime 都在 N 分钟内更新
└─ 处理：提示用户选择保留哪个版本（详见 5.1）

场景 D：双源都有，内容相同
└─ 处理：无需同步

场景 E：项目内缺失
├─ Kron 内部有，项目内没有
└─ 处理：从 Kron 内部恢复到项目内（提示用户）
```

**冲突时间阈值**：
- 默认 5 分钟内同时修改 = 冲突
- 用户可在设置中调整（1分钟 / 10分钟 / 30分钟 / 永不视为冲突）

**同步流程图**：

```
文件变化监听（notify crate）
    ↓
检测到文件被修改
    ↓
计算双源 mtime
    ↓
判断场景（A/B/C/D/E）
    ↓
├─ A: 同步项目内 → Kron 内部
├─ B: 同步 Kron 内部 → 项目内
├─ C: 提示用户冲突
├─ D: 无操作
└─ E: 提示用户恢复
    ↓
记录操作日志
```

**复原机制**（见需求 5）

#### 4.5 CLI 命令

```bash
kron important add <path>       # 添加重要文件/文件夹
kron important list             # 列出所有重要文件
```

**详细命令清单见需求 6**。


#### 4.6 GUI 支持

- Kron GUI 中显示重要文件夹列表
- 拖拽添加重要文件
- 可视化双源同步状态
- **不提供 MD 编辑器**（用 Typora / VSCode / Cursor 打开）
- 鼠标交互**完全遵循 Windows 资源管理器规范**（见 4.7）
- **Kron 不实现自定义右键菜单**
- **Kron 内部操作放在右侧详情/操作栏**（选中文件后显示，见 4.8）

---

#### 4.7 鼠标交互机制：完全遵循 Windows 资源管理器 ⭐⭐⭐

**核心设计原则**：**Kron 文件列表的鼠标交互 = Windows 资源管理器**。Kron **不自己写复杂的事件处理**。

##### 设计理念

**用户说**：**完全遵循 Windows 资源管理器的行为**。

**回答**：**完全一致！Kron 文件列表就是 Windows 资源管理器的克隆**。

##### Kron 的极简实现

**Kron 不做的事**（让 Windows 系统处理）：
- ❌ 自定义右键菜单（用 Windows 系统原生菜单）
- ❌ 应用路径检测（用 Windows ShellExecute）
- ❌ 应用列表维护（Windows 自动管理）
- ❌ "打开方式"菜单（Windows 系统提供）
- ❌ 自定义鼠标事件（用 WebView 原生 dblclick、contextmenu）

**Kron 只做的事**（极少代码）：
- ✅ 检测左键双击事件（WebView 原生 dblclick）
- ✅ 调用 ShellExecute 让 Windows 用默认应用打开

##### Windows 资源管理器行为（Kron 1:1 复制）

| 操作 | 行为 | 实现方式 |
|------|------|---------|
| **左键单击** | 选中文件（高亮） | Kron 极简 onClick |
| **左键双击** | 用 Windows 默认应用打开 | Kron onDoubleClick + ShellExecute |
| **右键单击** | 弹 Windows 系统原生菜单 | 系统自带，Kron 不实现 |
| **Ctrl + 单击** | 多选切换 | Kron 内置 |
| **Shift + 单击** | 范围选择 | Kron 内置 |
| **拖拽** | 拖到其他应用 | 系统支持 |
| **鼠标滚轮** | 滚动列表 | 系统支持 |

##### 整体交互设计（完全照搬 Windows 资源管理器）

```
┌─────────────────────────────────────────┐
│  Kron GUI 文件列表                        │
├─────────────────────────────────────────┤
│  📄 README.md                            │
│  📄 TODO.md                              │
│  📄 api-spec-v1.md                       │
│  📁 important/                           │
└─────────────────────────────────────────┘

用户操作（完全遵循 Windows 资源管理器规范）：

1. 鼠标移到文件名上（hover）
   → 显示文件元信息（创建时间、大小）

2. 左键单击文件
   → 选中该文件（高亮显示）
   → 不打开
   → 右侧详情栏更新（见 4.8）

3. 左键双击文件
   → Kron 调用 ShellExecute
   → Windows 用系统默认关联应用（Typora）打开
   → Kron 不关心用哪个应用

4. 右键单击文件
   → Windows 系统弹原生右键菜单（含"打开方式"）
   → Kron 不实现自定义菜单
   → 用户可选择"打开方式" → 子菜单选 Typora / VSCode / Cursor

5. 拖拽文件
   → 拖到其他应用
```

##### Kron 的极简代码实现

**前端代码（极简）**：

```typescript
// 只需要最少的鼠标事件处理
<div
    onClick={() => setSelected(file)}      // 左键单击 = 选中
    onDoubleClick={() => openFile(file)}    // 左键双击 = 打开
>
    {file.name}
</div>

// 打开文件的极简实现
async function openFile(file) {
    // 直接调用 Rust 后端的极简命令
    await invoke("open_with_system", { path: file.path });
}

// 完全不写右键菜单代码
// 右键 = Windows 系统原生菜单（自动）
```

**Rust 后端代码（极简）**：

```rust
// 极简实现：只调用 ShellExecute，让 Windows 处理一切
#[tauri::command]
fn open_with_system(path: String) -> Result<(), String> {
    opener::open(&path).map_err(|e| e.to_string())?;
    // 或使用 Windows API：
    // ShellExecuteW(..., "open", path, ...)
    Ok(())
}

// Kron 不需要这些代码：
// ❌ 应用路径检测
// ❌ 应用列表管理
// ❌ 默认应用配置
// ❌ "打开方式"菜单
```

##### 为什么这样设计

**对比：自己写 vs 系统处理**

| 功能 | 自己写（Kron 实现） | 系统处理（推荐） |
|------|-------------------|----------------|
| 检测应用路径 | Kron 读注册表、扫描固定路径 | Windows 自动管理 |
| 启动应用 | Kron 调 Command::new | Windows ShellExecute |
| "打开方式" | Kron 弹自定义菜单 | Windows 系统原生菜单 |
| 默认应用变更 | 用户在 Kron 内修改 | 用户在 Windows 设置 |
| 代码量 | Kron 实现复杂 | Kron 极简 |

**结论**：**让 Windows 系统处理一切，Kron 只做最少的事**。

##### 关于右键菜单

**Kron 不实现自定义右键菜单**，因为：

1. Windows 系统已经提供了完整的右键菜单
2. Windows 右键菜单包含"打开方式"（含所有已安装应用）
3. 用户习惯系统右键菜单（与 Win 资源管理器一致）
4. Kron 实现自定义菜单 = 重复造轮子

**如果未来需要 Kron 自定义菜单项**（如"Kron 内部操作"）：
- 用 Tauri 的菜单系统
- 追加 Kron 自定义项到 Windows 系统菜单
- 但 v1 不需要

##### 关于左键单击 vs 双击

**为什么 Kron 不用 HTML 默认的单击打开**：

- HTML `<a>` 默认单击打开链接
- Windows 资源管理器默认**单击选中、双击打开**
- 用户使用 Windows，对"双击打开"已习惯
- Kron 遵循 Windows 规范 = 与资源管理器一致

**实现细节**：
- 单击 = onClick = 高亮选中
- 双击 = onDoubleClick = ShellExecute
- onDoubleClick 在 onClick 之后触发
- 两次 onClick 之间的时间差 < 500ms 才算双击

##### 用户在 Windows 中设置默认应用

**Kron 不引导用户设置**，用户在系统中自己设置：

1. 在 Windows 设置中关联：
   - 设置 → 应用 → 默认应用 → 选择 .md → 选 Typora
2. 在文件资源管理器中右键设置：
   - 右键 .md 文件 → 打开方式 → 选 Typora → 勾选"始终使用此应用打开"
3. 设置后，Kron 双击 .md 自动用 Typora 打开

**Kron 完全不需要知道用户用哪个应用**。

##### 这种设计的极致简约

```
Kron 前端代码量：
- 文件列表组件：< 50 行
- 鼠标事件处理：onClick + onDoubleClick（5 行）
- 不需要右键菜单代码

Kron 后端代码量：
- open_with_system 命令：3 行
- 不需要应用管理代码
- 不需要 ShellExecute 之外的任何代码
```

**总结**：**Kron 用最少的代码，让 Windows 做所有复杂的事**。

##### 为什么 Kron 不内置 MD 编辑

- Typora / VSCode / Cursor 等编辑器的体验比 Kron 内置好太多
- Kron 内置编辑器需要造一套"高亮、预览、滚动"等组件，工作量大且体验差
- Kron 专注数据管理，**让专业的事交给专业的工具**

---

#### 4.8 右侧详情/操作栏（Kron 内部操作）⭐ 新增

**问题**：Kron 内部编辑（如编辑 task）放在哪里？

**方案**：**选中文件时，右侧详情栏显示 Kron 操作按钮**。

**这与右键菜单完全不同**：
- ❌ 右键菜单（Windows 原生）= 系统操作（打开、复制、删除等）
- ✅ 右侧详情栏 = Kron 内部操作（编辑 task、双源同步等）

##### 整体布局

```
┌─────────────────────────────────────────┬──────────────────────────┐
│  Kron GUI 文件列表（左 70%）               │  详情/操作栏（右 30%）    │
├─────────────────────────────────────────┼──────────────────────────┤
│  📄 README.md                            │  ┌────────────────────┐  │
│  📄 TODO.md     ← 选中（高亮）           │  │ 📄 TODO.md         │  │
│  📄 api-spec-v1.md                       │  │                    │  │
│  📁 important/                           │  │ 大小：4.2 KB        │  │
│                                          │  │ 创建：2026-09-04   │  │
│                                          │  │ 修改：2026-09-04   │  │
│                                          │  │ 状态：双源已同步 ✓ │  │
│                                          │  └────────────────────┘  │
│                                          │                          │
│                                          │  ┌─ Kron 内部操作 ────┐  │
│                                          │  │                    │  │
│                                          │  │ 📝 编辑 task       │  │
│                                          │  │ 🔄 立即同步双源    │  │
│                                          │  │ 📋 复制 Kron 路径  │  │
│                                          │  │ 📂 在文件管理器    │  │
│                                          │  │ 🗑️ 从 Kron 中移除  │  │
│                                          │  │                    │  │
│                                          │  └────────────────────┘  │
└─────────────────────────────────────────┴──────────────────────────┘
```

##### 右侧详情栏内容

**基础信息区**（始终显示）：
- [ ] 文件名
- [ ] 文件大小
- [ ] 创建时间
- [ ] 修改时间
- [ ] 双源同步状态（已同步 / 有冲突 / 仅单源）
- [ ] 备份位置（Kron 内部路径）

**Kron 操作按钮区**（始终显示）：
- [ ] **编辑 task**（仅 task MD 文件显示，如 TODO.md / IN_PROGRESS.md / DONE.md）
- [ ] **立即同步双源**
- [ ] **复制 Kron 路径**
- [ ] **在文件管理器中显示**
- [ ] **从 Kron 中移除**（如果是 Kron 管理的文件）
- [ ] **查看修改历史**

##### 不同文件类型的操作按钮

**普通 MD 文件**（如 README.md）：
```
┌─ Kron 内部操作 ────┐
│ 📝 查看文件元信息   │
│ 🔄 立即同步双源    │
│ 📋 复制 Kron 路径  │
│ 📂 在文件管理器    │
└────────────────────┘
```

**Task 文件**（TODO.md / IN_PROGRESS.md / DONE.md）：
```
┌─ Kron 内部操作 ────┐
│ 📝 编辑 task 列表   │  ← 核心功能
│ 🔄 立即同步双源    │
│ 📋 复制 Kron 路径  │
│ 📂 在文件管理器    │
│ 📊 查看 task 统计  │
└────────────────────┘
```

**重要文件夹**（important/）：
```
┌─ Kron 内部操作 ────┐
│ 📁 浏览子文件      │
│ 🔄 立即同步双源    │
│ 📋 复制 Kron 路径  │
│ 📂 在文件管理器    │
│ ⚙️ 配置重要文件夹  │
└────────────────────┘
```

##### 为什么用"右侧详情栏"而不是"右键菜单"

**对比**：

| 方案 | 优点 | 缺点 |
|------|------|------|
| **右侧详情栏**（推荐） | ✅ 可见性强（不需要记住快捷键）<br>✅ 与 VSCode、Finder 一致<br>✅ 操作展示更清晰<br>✅ 多按钮更清晰 | ⚠️ 占右侧空间 |
| 自定义右键菜单 | ✅ 不占空间<br>✅ 符合菜单习惯 | ❌ 需要打开菜单才能看到操作<br>❌ 与 Windows 资源管理器右键冲突 |

**结论**：**右侧详情栏** 是 Kron 内部操作的最佳位置。

##### 详情栏的视觉设计

```
┌─────────────────────────────────┐
│ 文件详情                          │
├─────────────────────────────────┤
│ 文件名：TODO.md                  │
│ 类型：Markdown 文档              │
│ 大小：4.2 KB                     │
│ 创建：2026-09-04 18:30           │
│ 修改：2026-09-04 19:15           │
│                                  │
│ ┌─ Kron 状态 ──────────────┐    │
│ │ 双源同步：✓ 已同步       │    │
│ │ Kron 备份：D:\Apps\Kron\ │    │
│ │               data\...   │    │
│ └────────────────────────┘     │
│                                  │
│ ┌─ Kron 操作 ──────────────┐    │
│ │ [📝 编辑 task 列表]      │    │
│ │ [🔄 立即同步]            │    │
│ │ [📋 复制路径]            │    │
│ │ [📂 在文件管理器中显示]  │    │
│ │ [🗑️ 从 Kron 中移除]      │    │
│ └────────────────────────┘     │
└─────────────────────────────────┘
```

##### 详情栏的实现

**前端组件**：

```typescript
// DetailPanel.tsx
function DetailPanel({ selectedFile }: { selectedFile: FileItem | null }) {
    if (!selectedFile) {
        return <div className="detail-empty">未选择文件</div>;
    }

    return (
        <div className="detail-panel">
            {/* 文件基础信息 */}
            <FileInfo file={selectedFile} />

            {/* Kron 状态 */}
            <KronStatus file={selectedFile} />

            {/* Kron 操作按钮 */}
            <KronActions file={selectedFile} />
        </div>
    );
}

function KronActions({ file }: { file: FileItem }) {
    return (
        <div className="kron-actions">
            {/* 编辑 task 按钮（仅 task 文件显示） */}
            {isTaskFile(file) && (
                <button onClick={() => editTask(file)}>
                    📝 编辑 task 列表
                </button>
            )}

            {/* 同步按钮 */}
            <button onClick={() => syncFile(file)}>
                🔄 立即同步双源
            </button>

            {/* 复制路径按钮 */}
            <button onClick={() => copyPath(file)}>
                📋 复制 Kron 路径
            </button>

            {/* 在文件管理器中显示 */}
            <button onClick={() => showInExplorer(file)}>
                📂 在文件管理器中显示
            </button>

            {/* 移除按钮 */}
            {isKronManaged(file) && (
                <button onClick={() => removeFromKron(file)}>
                    🗑️ 从 Kron 中移除
                </button>
            )}
        </div>
    );
}
```

##### 详情栏 vs 右键菜单的职责分工

| 操作类型 | 位置 | 说明 |
|---------|------|------|
| 打开（用 Typora） | 双击 / Windows 右键 | Windows 系统处理 |
| 复制 / 剪切 / 删除 | Windows 右键 | Windows 系统处理 |
| 重命名 | Windows 右键 / F2 | Windows 系统处理 |
| 属性 | Windows 右键 | Windows 系统处理 |
| 在文件管理器中显示 | Kron 详情栏按钮 | Kron 提供（避免 Windows 右键菜单无该选项时找不到） |
| **编辑 task** | **Kron 详情栏按钮** | **Kron 内部操作** |
| **立即同步双源** | **Kron 详情栏按钮** | **Kron 内部操作** |
| **查看 Kron 备份** | **Kron 详情栏按钮** | **Kron 内部操作** |
| **从 Kron 中移除** | **Kron 详情栏按钮** | **Kron 内部操作** |

**原则**：
- **Windows 系统能做的**（打开、复制、删除）→ 用 Windows 右键菜单
- **Kron 内部操作**（编辑 task、同步、查看备份）→ 用 Kron 右侧详情栏

##### 视觉简洁性

详情栏**不干扰文件列表的简洁性**：
- 文件列表区域（左 70%）保持干净
- 详情栏（右 30%）只显示必要信息
- 操作按钮用图标 + 文字

---

### 需求 5：双源同步 + 复原（重要）

#### 5.1 冲突判断

**冲突检测原则**：
- 比较双源文件的最后修改时间（mtime）
- 如果双源都被修改过（时间戳差异在阈值内），则判定为冲突
- 如果只有一个源被修改，则同步到另一个源（无冲突）

**检测算法**：

```
对每个双源文件对（项目内文件, Kron 内部文件）：
    if 仅项目内存在:
        → 复制到 Kron 内部（场景 A）
    elif 仅 Kron 内部存在:
        → 复制到项目内（场景 B）
    elif 双源都存在:
        ├─ 比较 mtime
        ├─ mtime 差异 > 阈值（如 5 分钟）：
        │  ├─ 更新 mtime 较新者 → 较旧者
        │  └─ 无冲突（场景 A）
        └─ mtime 差异 ≤ 阈值：
           └─ 冲突 → 提示用户（场景 C）
```

**冲突场景详细化**：

```
场景 A：单源修改（无冲突）
├─ Kron 内部修改（mtime 更新）→ 项目内未修改
├─ 处理：Kron 内部 → 项目内（单向同步）
└─ 不提示用户

场景 B：双源都修改（冲突）
├─ Kron 内部 20:30:00 修改
├─ 项目内 20:28:00 修改
├─ 时间差 2 分钟 < 5 分钟阈值
├─ 处理：标记为冲突
├─ 备份双方（到 .kron_conflicts/）
└─ 提示用户选择（详见冲突解决策略）

场景 C：项目内缺失
├─ Kron 内部有，项目内没有
├─ 处理：从 Kron 内部恢复到项目内
└─ 提示用户（"是否恢复 X 个文件"）

场景 D：Kron 内部缺失
├─ 项目内有，Kron 内部没有
├─ 处理：从项目内复制到 Kron 内部
└─ 不提示用户（视为首次添加）

场景 E：双源都缺失
├─ 用户主动删除了双源
└─ 处理：什么都不做（认为是用户意图）
```

**冲突解决策略**（用户选择）：

| 策略 | 行为 | 适用场景 |
|------|------|---------|
| **保留 Kron 内部** | Kron 内部 → 项目内，覆盖项目内 | 信任 Kron 内部 |
| **保留项目内** | 项目内 → Kron 内部，覆盖 Kron 内部 | 信任项目内（如手动修改） |
| **保留最新** | mtime 更新者保留 | 自动决策 |
| **保留两者** | 双源都保留，加后缀区分 | 都不愿丢失 |
| **手工合并** | 打开外部编辑器手工合并 | 两者都重要 |

**冲突时的备份机制**：

```
检测到冲突时：
├─ 创建冲突备份目录：<kron_install_dir>/data/conflicts/<timestamp>/
│   ├── <file_name>.internal    ← Kron 内部版本
│   └── <file_name>.project     ← 项目内版本
├─ 双方版本都备份
└─ 等待用户决策
```

**冲突检测阈值**：
- 默认：**5 分钟**
- 用户可配置：1 / 5 / 10 / 30 分钟 / 永不视为冲突
- 阈值越大，自动同步越多；越小，冲突提示越多

**配置示例**：

```toml
# <kron_install_dir>/data/config.toml
[sync]
conflict_threshold_minutes = 5  # 5 分钟内同时修改视为冲突
auto_resolve = "latest"         # 自动解决策略：latest / prompt / manual
backup_conflicts = true         # 冲突时是否备份
```

#### 5.2 复原机制（核心）

**触发条件**：
- Git 操作（checkout、pull、merge、rebase）后，项目内 `KRON/` 异常
- 用户主动触发 `kron restore`
- Kron 启动时检测到 KRON/ 不一致

**异常检测**：
```
启动或 Git 操作后：
    ├─ 检测项目内 KRON/ 是否存在
    ├─ 检测 KRON/ 结构是否完整（必需文件）
    ├─ 检测双源文件数量是否一致
    └─ 检测 mtime 是否异常（如未来时间、过早时间）
```

**行为流程**：

```
1. 检测异常
    ↓
2. Kron 弹出提示
    "检测到项目内 KRON/ 异常，是否从 Kron 内部恢复？"
    ├─ [恢复]
    ├─ [查看差异]
    ├─ [取消]
    └─ [高级选项]
        ├─ 仅恢复缺失文件
        ├─ 完全覆盖（Kron 内部 → 项目内）
        ├─ 完全覆盖（项目内 → Kron 内部）
        └─ 备份后恢复
    ↓
3. 用户确认后执行恢复
    ↓
4. 记录恢复日志
```

**恢复策略**：

| 策略 | 行为 | 何时用 |
|------|------|--------|
| **智能恢复** | 仅恢复缺失或损坏的文件 | 默认 |
| **完全覆盖** | Kron 内部覆盖整个 KRON/ | 双源都异常 |
| **手动选择** | 列出每个文件，用户决定 | 高级用户 |

**恢复前的自动备份**：
```
恢复前自动创建备份：
├─ 项目内 KRON/ → <kron_install_dir>/data/backups/before_restore_<timestamp>/
└─ 防止恢复出错导致数据丢失
```

#### 5.3 自动复原机制（Kron 监听 > Git Hooks）⭐⭐⭐⭐⭐

**核心设计原则**：**主动权在 Kron 手中，不依赖 Git**。

**为什么不让 Git Hooks 主导**：

- ❌ Git Hooks 需要项目用 Git
- ❌ Git Hooks 需要用户主动 `install`
- ❌ Git Hooks 维护成本高（用户可能误删）
- ❌ Git Hooks 触发时机有限
- ❌ **主动权在 Git 手中** — 我们只能响应 Git 事件

**Kron 的做法**：**自己监听，主动触发**

##### Kron 后台守护进程（默认 + 唯一）

```
Kron 后台守护进程（系统托盘常驻）
    ↓
启动文件监听器（notify crate，跨平台）
    ↓
监听以下内容：
├─ KRON/VERTEX/*/tasks.md（task 内容变化）
├─ KRON/important/（重要文件变化）
├─ .kron-context/（中间文档状态）
├─ .git/HEAD（Git 分支变化，轻量）
├─ .git/refs/heads/*（Git 分支引用变化）
├─ .git/index（Git 状态变化）
    ↓
检测到变化时自动：
├─ checkout / merge / pull → 自动调用 kron restore
├─ commit → 自动调用 kron context
├─ MD 文件变化 → 重新解析 + 更新内部元数据
    ↓
用户零感知（后台静默）
```

**关键技术细节**：

| 技术 | 说明 |
|------|------|
| **notify crate** | Rust 跨平台文件监听库（Windows/macOS/Linux） |
| **监听 .git/HEAD** | 文件内容变化即触发（如 checkout 后 HEAD 改变） |
| **监听 .git/refs/** | 新 commit 推送时自动更新引用 |
| **轻量轮询** | notify 底层用 OS 事件，无需轮询 |

**关键优势**：

- ✅ **主动权在 Kron**（不是 Git）
- ✅ **不依赖 Git Hooks**（不用 Git 也能工作）
- ✅ **实时性好**（事件驱动，非轮询）
- ✅ **无需用户配置**（默认开启）
- ✅ **零 Git Hooks 兼容性问题**

##### Git Hooks：完全删除 ⭐⭐⭐⭐⭐

**重要决策**：**Kron 不再支持 Git Hooks**。

**原因**：

- ❌ Kron 守护进程已**完全覆盖** Git Hooks 的功能
- ❌ 多一种机制 = 多一份维护成本
- ❌ 多一种机制 = 用户认知负担
- ❌ 与 Kron 的"主动权在我"哲学矛盾
- ✅ **Kron 自己监听更好**（一个统一机制）

**删除的 CLI 命令**：

```bash
# 以下命令全部删除：
❌ kron hooks install
❌ kron hooks uninstall
❌ kron hooks status
```

**Git Hooks 完全退役**：

- 不再生成 `.git/hooks/post-*` 脚本
- 不再调用 `kron hooks install`
- 所有 Git 操作由 Kron 守护进程监听响应

**对比表**（最终方案 vs 之前的方案）：

| 维度 | Git Hooks 主导（之前） | Kron 监听（现在） |
|------|---------------------|-----------------|
| 主动权 | Git | **Kron** ⭐ |
| 不依赖 Git | ❌ | ✅ |
| 实时性 | commit 后才触发 | 事件驱动，**更快** |
| 用户配置 | 需要 install | 默认开启 |
| 维护成本 | 高 | 低 |
| 兼容性 | Git 版本问题 | notify crate 跨平台 |

**结论**：**Kron 监听 = 唯一方案**，Git Hooks 完全退役。

#### 5.4 KRON 仓库覆盖本地（多设备开发）⭐ 新增

**问题场景**：
- 用户在设备 A 开发，推送代码到 Git
- 用户在设备 B 克隆仓库（git clone）
- 设备 B 需要 KRON 数据，但项目内的 KRON/ 是设备 A 的版本
- 设备 B 自己的 Kron 内部数据可能是旧的

**解决方案**：

```bash
# 在设备 B 上克隆后，用户主动执行：
kron remote-sync

# 或者：
kron restore --source remote
```

**行为**：
- 用仓库中的 `KRON/` 覆盖本地的 Kron 内部数据
- 或者反过来：用 Kron 内部数据覆盖项目内 `KRON/`

**两种模式**：

| 模式 | 命令 | 行为 |
|------|------|------|
| **本地优先** | `kron restore` | 用 Kron 内部数据覆盖项目内 KRON/ |
| **远程优先** | `kron remote-sync` | 用项目内 KRON/ 覆盖 Kron 内部数据 |

**Git Hook 自动触发**：
- `post-checkout`：自动用项目内 KRON/ 覆盖 Kron 内部（remote-sync）
- `post-merge`：同上
- `post-pull`：同上

#### 5.5 项目改名/移动处理

**场景**：项目改名或移动到新位置

**处理**：
- `KRON/` 文件夹本身还在（项目改名不影响）
- Kron 检测到路径变化 → 提示用户"项目路径已变更，是否更新？"
- 用户确认后，Kron 内部路径记录更新
- 内容中的相对路径无需修改（相对项目根）

---

### 需求 6：命令行工具 `kron-cli`

**设计原则**：**精简、必要、AI 可调用**。

CLI 不是给用户日常用的（用户用 GUI），CLI 主要给：
- 🤖 **AI 工具调用**（Cursor / Claude Code 等）
- 🛠️ **高级用户**（写脚本、批处理）
- ⚙️ **自动化场景**（Git Hooks / 定时任务）

#### 6.1 命令清单（精简版）

**核心命令（约 10 个）**：

```bash
# === 项目初始化与状态 ===
kron init                       # 在当前目录初始化 KRON/
kron status                     # 显示项目状态总览

# === Vertex 管理 ===
kron vertex create <name>       # 创建 Vertex，绑定当前 HEAD commit
kron vertex list                # 列出所有 Vertex
kron vertex delete <name>       # 删除 Vertex（同时清理其下 task）

# === Task 管理 ⭐简化设计 + 优雅交互===

# 直接编辑 MD（用户/AI 的主要操作方式）
# （不需要 CLI，直接打开 tasks.md 编辑即可）

# 状态转换命令（AI 工具友好）
kron task move                       # 交互式：显示所有 task → 选择 → 选择状态
kron task move "登录"                # 模糊匹配 task → 选择状态
kron task move "登录" in_progress    # 一行完成（AI 推荐）
kron task back                       # 交互式：选择 task → 回退到上一状态
kron task back "登录"                # 模糊匹配 → 自动回退

# 状态捷径命令（更直观）
kron task start "登录"               # 等价于 move "登录" in_progress
kron task done "登录"                # 等价于 move "登录" done
kron task todo "登录"                # 等价于 move "登录" todo

# 批量操作（高级）
kron task move --tag "#后端" done    # 所有 #后端 的 task 移到 done
kron task move --from-git-diff       # 根据当前 git diff 推荐 task

# 格式检查 + 同步
kron task check                      # 检查所有 tasks.md 格式 + 同步到 Kron 内部

# 注意：用户/AI 也可以直接编辑 MD（详见 2.2 节）

# === 重要文件 ===
kron important add <path>       # 添加重要文件/文件夹
kron important list             # 列出所有重要文件

# === 中间文档（让 AI 读懂项目）⭐ ===
kron context                    # 更新中间文档（增量，后台守护进程自动调用）

# === 复原（异常时手动触发）===
kron restore                    # 自动检测并恢复

# === Git 集成（Kron 监听，非 Git Hooks）===
kron remote-sync                # 用项目内 KRON/ 覆盖 Kron 内部（多设备同步）
# 注意：Git 操作由 Kron 后台守护进程自动监听（详见 5.3）
# 不再需要 kron hooks install/uninstall
```

**说明**：

- `kron task add` 是交互式命令（不强记参数）
- `kron context` 无参数默认增量更新（无需区分 generate / update）
- `kron restore` 无参数智能恢复（无需 `--scope`）
- `kron hooks` 子命令化（install / uninstall）

#### 6.2 Kron 后台守护进程（Kron 监听 > Git Hooks）⭐⭐⭐⭐⭐

**核心设计**：**Kron 自己监控文件变化，主动权完全在 Kron 手中**。

**设计哲学**：

- ❌ 不依赖 Git Hooks（项目不用 Git 也能工作）
- ❌ 不需要用户手动安装 hooks
- ✅ Kron 守护进程 = 唯一的自动化机制
- ✅ 主动权在 Kron，不在 Git

**Kron 后台守护进程的设计**：

```
┌──────────────────────────────────────┐
│  Kron 后台进程（系统托盘常驻）          │
├──────────────────────────────────────┤
│                                      │
│  1. 启动文件监听器（notify crate）     │
│     ├─ 监听 KRON/VERTEX/*/*.md        │
│     ├─ 监听 KRON/important/          │
│     ├─ 监听 .kron-context/           │
│     └─ 监听项目内 .git/HEAD（轻量）   │
│                                      │
│  2. 文件变化时自动：                   │
│     ├─ 解析 MD 文件，更新 Kron 内部   │
│     ├─ 检查 .kron-context/ 过期情况   │
│     └─ 自动运行增量更新                │
│                                      │
│  3. 定时任务（每 5 分钟）：            │
│     ├─ 检查 .kron-context/ 过期情况   │
│     └─ 自动增量更新                   │
│                                      │
│  4. 用户无感知（后台静默运行）          │
│                                      │
└──────────────────────────────────────┘
```

**关键点**：

- ✅ **完全不依赖 Git**（即使项目不用 Git 也能工作）
- ✅ **实时性更好**（任何文件保存即触发）
- ✅ **无需用户配置**（守护进程默认随 Kron 启动）
- ✅ **省资源**（用 Rust 的 `notify` crate，监听而非轮询）
- ✅ **跨平台**（notify 在 Windows/macOS/Linux 都支持）

**Git Hooks 已被删除**：

- ❌ Kron 不再支持 `kron hooks install/uninstall`
- ❌ 不再生成 `.git/hooks/post-*` 脚本
- ✅ 所有 Git 操作由 Kron 守护进程监听响应
- ✅ 一个统一机制，简洁清晰

**对比表（Kron 监听 vs Git Hooks）**：

| 维度 | Git Hooks（已删除） | Kron 守护进程（唯一） |
|------|-------------------|---------------------|
| 主动权 | Git | **Kron** ⭐ |
| 不依赖 Git | ❌ | ✅ |
| 实时性 | commit 后才触发 | 文件保存即触发 |
| 用户配置 | 需要 install | **默认开启** |
| 维护成本 | 高 | 低 |
| 跨平台 | 依赖 Git | notify crate 跨平台 |

**结论**：**Kron 守护进程 = 唯一方案**。Git Hooks 完全退役。

#### 6.3 中间文档管理 ⭐

**唯一命令**：`kron context`

```bash
# 默认行为：增量更新过期的中间文档
kron context

# 完整重新生成
kron context --regenerate

# 列表 + 状态
kron context --list
```

**由谁触发**：

| 触发源 | 触发方式 |
|--------|---------|
| **Kron 后台守护进程**（默认 + 唯一） | 文件变化时自动调用 |
| **用户手动** | `kron context` |
| **AI 工具** | Cursor / Claude Code 调用 `kron context` |

**中间文档类型**（与之前一致）：

- `git/recent-commits.md` - 最近 100 次 commit
- `git/branch-summary.md` - 当前分支与主分支对比
- `git/file-history.md` - 重要文件修改历史
- `code/structure.md` - 代码目录结构
- `code/dependencies.md` - 项目依赖
- `code/important-files.md` - 重要文件清单
- `api/public-apis.md` - 公开 API 摘要

**详细设计见需求 2.4**。

#### 6.4 复原与同步

```bash
# 智能恢复（自动检测异常）
kron restore

# 跨设备同步（用项目内 KRON/ 覆盖 Kron 内部）
kron remote-sync
```

**详细设计见需求 5**。

**关于 Git 集成**：

- ✅ Kron 后台守护进程自动监听 Git 操作（详见 5.3 / 6.2）
- ❌ 不再需要 `kron hooks install/uninstall`（已删除）
- ❌ 不再生成任何 Git Hooks 脚本

#### 6.5 价值

- ✅ AI 可通过 CLI 修改 KRON 数据（与直接修改 MD 文件等价）
- ✅ 用户在终端中也能管理（高级用户）
- ✅ 自动化脚本支持
- ✅ 复原机制可手动触发
- ✅ **生成 AI 易读的中间文档**（核心价值）
- ✅ **Kron 后台守护进程让一切自动化**（无需 Git Hooks）

---

### 需求 7：AI 工具联动（基于文件结构，非内置 AI）

**核心理念**：**Kron 不做内置 AI 辅助功能**（如"AI 生成描述"）。Kron 通过**精心设计的文件结构**让 AI 自然理解项目。

#### 7.1 Kron 的 AI 易读设计哲学 ⭐⭐⭐⭐⭐

**为什么不做内置 AI 功能**：

- ❌ 所有任务管理工具都在做"AI 生成描述"等烂大街功能
- ❌ AI 工具（Cursor / Claude Code / Copilot）已足够强大
- ❌ 内置 AI 绑死某个服务、引入 API Key 复杂性
- ❌ 真正的价值不在 AI 集成，而在**文件结构**

**Kron 的做法**：

- ✅ 设计**结构化的、易读的 MD 文件**
- ✅ AI 工具**自然读取**这些文件
- ✅ 用户在 AI 工具（Cursor / Claude Code）中**直接修改 MD**
- ✅ Kron 检测到文件变化 → 同步到 Kron 内部
- ✅ **Kron = 数据层，AI 工具 = 智能层，两者解耦**

#### 7.2 与 CC Switch 的联动

**CC Switch** 是管理 AI 编程工具配置的桌面应用

**联动方式**：
- [x] Kron 中提供 CC Switch 的快捷入口（菜单/按钮）
- [x] 双向启动支持：CC Switch 也能跳转到 Kron

**数据互通**：
- **不互通 AI 配置**：AI 配置由 CC Switch 单一管理，Kron 不重复
- **项目列表互通**：CC Switch 中的"项目"与 Kron 的"项目"可互相跳转

#### 7.3 AI 工具读取 KRON/ 的标准路径 ⭐

**目标**：让 AI 编程工具（Claude Code、Cursor、Copilot）能更好地理解 Kron 项目。

**KRON/ 目录的 AI 友好结构**：

```
KRON/
├── README.md                     ← 项目总览（AI 第一眼看到的）
├── vertices.json                 ← Vertex 定义
├── VERTEX/
│   ├── 开发/
│   │   ├── TODO.md               ← 待办任务
│   │   ├── IN_PROGRESS.md        ← 正在做的
│   │   └── DONE.md               ← 已完成（历史）
│   └── ...
├── important/                    ← 重要 idea / 设计文档（双源备份）
│   ├── api-spec-v1.md
│   └── architecture-decision.md
└── .kron-context/                ← 自动生成的项目上下文
    ├── README.md                 ← 中间文档索引
    ├── git/
    │   └── recent-commits.md
    └── code/
        ├── structure.md
        └── dependencies.md
```

**AI 工具读取 KRON/ 的最佳实践**：

```
AI 编程工具（如 Claude Code）启动：
    ↓
1. 读取 KRON/README.md（项目总览）
    ↓
2. 读取 KRON/VERTEX/*/IN_PROGRESS.md（当前任务）
    ↓
3. 读取 KRON/VERTEX/*/TODO.md（待办任务）
    ↓
4. 读取 KRON/important/（重要 idea / 设计文档）
    ↓
5. 读取 KRON/.kron-context/（项目当前状态）
    ↓
AI 完整理解项目（无需执行任何命令）
```

**AI 工具修改 task 的工作流**：

```
用户在 Cursor / Claude Code 中说：
"帮我把 task_001 的描述补充一下，重点说明用了什么设计模式"
    ↓
AI 工具读取 IN_PROGRESS.md，找到 task_001
    ↓
AI 工具修改 task_001 的 description
    ↓
保存文件
    ↓
Kron 文件监听器检测到变化
    ↓
Kron 重新解析 task_001
    ↓
Kron 内部元数据更新（created_at / updated_at 等）
    ↓
双向同步完成
```

**关键点**：

- AI 工具**直接修改 MD 文件**（与人类用户一样）
- Kron **不介入 AI 操作**
- AI 工具 = 用户的"超级编辑器"，与 Typora / VSCode 地位相同

#### 7.4 Kron 提供给 AI 的两类上下文 ⭐⭐⭐

**Kron 提供两类文件供 AI 读取**：

| 文件位置 | 内容 | 用途 |
|---------|------|------|
| `KRON/important/` | 用户主动添加的**重要 idea / 设计文档** | 让 AI 理解项目**关键决策** |
| `KRON/.kron-context/` | Kron 自动生成的**项目上下文** | 让 AI 理解项目**当前状态** |

**详细设计**：
- `KRON/important/`：见需求 4（重要文件夹）
- `KRON/.kron-context/`：见需求 2.4（中间文档）

**AI 工作流的完整文件清单**：

```
AI 启动 → 读取以下文件即可完整理解项目：

1. KRON/README.md                 ← 项目说明
2. KRON/VERTEX/*/IN_PROGRESS.md   ← 当前任务
3. KRON/VERTEX/*/TODO.md          ← 待办任务
4. KRON/important/                ← 重要 idea
5. KRON/.kron-context/            ← 项目当前状态
6. 项目根 README.md               ← 项目主说明
```

**约 6 类文件，AI 即可获得项目的完整视图**。

#### 7.5 Kron 与 AI 工具的边界

| 职责 | Kron | AI 工具（Cursor / Claude Code 等） |
|------|------|--------------------------------|
| 任务数据存储 | ✅ | ❌ |
| 文件结构 | ✅ | ❌ |
| 状态管理 | ✅ | ❌ |
| 双源同步 | ✅ | ❌ |
| 文件监听 | ✅ | ❌ |
| **生成描述** | ❌ | ✅ |
| **优化文本** | ❌ | ✅ |
| **补充背景** | ❌ | ✅ |
| **修改 MD** | ❌ | ✅ |
| **回答问题** | ❌ | ✅ |
| **代码生成** | ❌ | ✅ |

**Kron 严格不涉足 AI 智能处理**，保持工具的本质。

---

### 需求 8：双主题支持

- [ ] 浅色模式（白色背景）和深色模式（深灰色背景）
- **颜色原则**：
  - 所有 UI 颜色必须在两种背景下都可清晰识别
  - 避免纯白/纯黑色文本
  - 主色调使用中性色

---

## 🚫 明确不做的事情

- ❌ Git `--skip-worktree`（容易出问题，用复原机制代替）
- ❌ **Git Hooks**（Kron 自己监听，主动权在 Kron 而非 Git）
- ❌ **Kron 不做普通 MD 文件编辑**（用 Typora / VSCode / Cursor / 其他编辑器）
- ❌ **Kron 编辑 task 后写回 MD 文件**（task 是 MD 文件里的结构化内容，Kron 可读可写）
- ❌ 复杂的任务嵌套（Epic/Story/Subtask）
- ❌ **Kron 不做内置 AI 辅助功能**（不做"AI 生成描述"、"AI 优化描述"等烂大街功能）
- ❌ **不绑定特定 AI 服务**（不引入 AI SDK / API Key，保持工具本质）
- ❌ 快捷键（避免冲突）
- ❌ 多用户协作
- ❌ 云端同步（v1 阶段）
- ❌ 插件系统（v1 阶段）
- ❌ 内置终端
- ❌ Git 可视化操作（v1 阶段）
- ❌ 复杂的学习成本
- ❌ "无感知记录"功能（暂搁置）
- ❌ VSCode 扩展（v1 阶段；非主要开发对象）
- ❌ **重要文件夹容量限制**（不限制）
- ❌ **Kron 内置版本历史**（依赖 Git 做版本管理）
- ❌ **重要文件保留历史版本**（仅备份最新版）
- ❌ **强制 docs/ 目录**（任何位置都可以）
- ❌ **过度询问用户偏好**（如"哪些文件加入重要文件夹"由用户决定，不询问）
- ❌ **task state 字段**（状态仅在 GUI 中，不写入 MD）
- ❌ **tags 写入 MD**（tags 单独存 Kron 内部 JSON）
- ❌ **task CLI 命令（add/list/move/edit/tag）**（用户/AI 直接编辑 MD）
- ❌ **强制 Vertex 范围绑定**（Vertex 关系 = Git 树遍历）
- ❌ **手动绑定 Vertex 与分支**（Vertex 只能创建，不能手动 bind）
- ❌ **Kron 安装向导**（用 zip 解压即用，无安装器）
- ❌ **注册表 / 系统级安装**（便携式，无系统侵入）
- ❌ **Windows 用户目录存储**（使用 Kron 安装绝对路径）

---

## 🏗 技术架构

- **桌面框架**: Tauri 2.x (Rust + WebView)
- **前端**: React 18 + TypeScript
- **样式**: Tailwind CSS (双主题支持)
- **状态管理**: Zustand
- **构建**: Vite
- **后台守护进程**：
  - Rust 端使用 `notify` crate 监听文件变化
  - 默认随 Kron 启动（在系统托盘常驻）
  - 不依赖 Git（详见 6.2 节）
- **Git 集成**：
  - Rust 端调用 `git` CLI 读取分支信息（可选）
  - **不依赖 Git Hooks**（Kron 后台守护进程监听 .git/HEAD + .git/refs/）
  - Git Hooks 已被删除（Kron 自己监听，主动权在 Kron）
- **数据存储**：
  - Kron 内部：JSON + 文件
  - 项目内：Markdown 文件（KRON/）
- **复原机制**：后台守护进程（默认 + 唯一） + CLI 手动触发

---

## 📁 目录结构约定（最终版）

### Kron 自身开发

- **`./dev-docs/`** - Kron 自身开发过程的 Markdown 文档
  - `requirements.md` - 需求文档（当前文档）

### Kron 安装根目录（用户可见）⭐

```
D:\Apps\Kron\                       ← 安装根目录（推荐位置）
│
├── software\                        ← 软件本体（升级只需替换此目录）
│   ├── kron.exe                     ← GUI 主程序
│   ├── kron-cli.exe                 ← 命令行工具
│   ├── kron-daemon.exe              ← 后台守护进程
│   └── assets/                      ← 静态资源
│
└── data\                            ← 数据根目录（备份只需备份此目录）
    │
    ├── global\                      ← Kron 全局配置
    │   ├── config.toml
    │   ├── install_path.lock        ← 安装路径自寻
    │   └── projects.json            ← 已注册项目列表
    │
    ├── projects\                    ← 所有项目数据（核心）⭐
    │   └── <项目名>/                ← 用人类可读的项目名
    │       ├── kron-internal/       ← Kron 内部权威数据
    │       │   ├── config.json
    │       │   ├── states/          ← task 状态
    │       │   ├── tags/            ← task tags
    │       │   └── important/       ← 重要文件 Kron 内部备份
    │       │
    │       └── KRON/                ← 项目内数据（AI 友好）
    │           ├── README.md
    │           ├── VERTEX/<name>/
    │           │   ├── description.md
    │           │   ├── tasks.md
    │           │   └── _meta.json
    │           ├── important/
    │           └── .kron-context/
    │
    └── backups\                     ← 全局自动备份
```

### 项目目录（用户代码项目）

```
D:\code\my-project\                 ← 用户代码项目
├── src/
├── package.json
│
└── KRON\                            ← 软链接 → D:\Apps\Kron\data\projects\my-project\KRON\
    └── ...                          （AI 工具读项目目录时自动看到所有 Kron 数据）
```

**关键点**：

- ✅ 项目目录的 KRON/ 是软链接（不是真实目录）
- ✅ 用户不需要在两个地方管理数据
- ✅ AI 工具读项目目录自然看到 KRON/
- ✅ 删除项目目录 = 自动删除 KRON（软链接跟随）

### 项目内的 KRON/（AI 友好视图）

```
KRON/
├── README.md                        ← AI 第一眼读的项目说明
├── VERTEX/<name>/
│   ├── description.md               ← 阶段意图
│   ├── tasks.md                     ← 所有 task
│   └── _meta.json                   ← Vertex 绑定点
├── important/                       ← 用户主动添加的重要文件
│   └── api-spec-v1.md
└── .kron-context/                   ← Kron 自动生成的中间文档
    ├── git/recent-commits.md
    ├── code/structure.md
    └── code/dependencies.md
```

### 设计原则总结

| 原则 | 实现 |
|------|------|
| **软件与数据分离** | `software/` + `data/` 在同一父目录 |
| **项目名人类可读** | `<项目名>/` 而非 `<hash>/` |
| **软链接统一视图** | 项目目录 KRON/ = 软链接到 data |
| **AI 友好** | 项目内 KRON/ 是纯文本 MD |
| **数据本地化** | 不使用 Windows 用户目录 |

**详细设计说明见需求 4.2.1 节**。

---

## 📊 设计决策记录

| 日期 | 决策 | 原因 |
|------|------|------|
| 2026-09-04 | 使用 `KRON/` 文件夹而非 `KRON.md` 单文件 | 支持多 Vertex 存储、文档 |
| 2026-09-04 | Vertex 命名 "Vertex"（顶点） | 几何概念 |
| 2026-09-04 | 双源存储（Kron 内部 + 项目内） | 解决备份需求 |
| 2026-09-04 | 不用 `--skip-worktree` | 容易出问题 |
| 2026-09-04 | 用**复原机制**代替 `--skip-worktree` | 更可靠 |
| 2026-09-04 | 通过修改时间检测冲突 | 简单可靠 |
| 2026-09-04 | **Kron 不做 MD 文件编辑** | 用 Typora / VSCode 编辑体验更好 |
| 2026-09-04 | **Kron 解析 MD 文件显示 task** | task 内容是 AI 可读的项目文档 |
| 2026-09-04 | **支持 Typora 作为外部编辑器** | 用户日常使用 Typora |
| 2026-09-04 | **Git Hooks 自动复原** | 自动化，无需用户干预 |
| 2026-09-04 | **KRON 仓库覆盖本地** | 解决多设备开发 |
| 2026-09-04 | **不限制重要文件夹容量** | 灵活性优先 |
| 2026-09-04 | **不强制 docs/ 目录** | 用户可指定任意相对路径 |
| 2026-09-04 | **notes/ 文件元数据 _meta.json** | 记录原始路径与备份地址 |
| 2026-09-04 | **Kron 数据使用绝对安装路径** | 不使用 Windows 用户目录，避免账户/重装问题 |
| 2026-09-04 | **Task 存储在 MD 文件中**（TODO.md / IN_PROGRESS.md / DONE.md） | AI 可读、Kron 解析、Typora 可编辑 |
| 2026-09-04 | **左键双击打开，左键单击选中** | 与 Windows 资源管理器行为一致 |
| 2026-09-04 | **直接用 Windows 原生右键菜单** | 不实现自定义右键菜单，让系统处理"打开方式" |
| 2026-09-04 | **使用 ShellExecute 打开文件** | 调用系统默认关联应用，无需 Kron 维护应用列表 |
| 2026-09-04 | **直接使用 Windows 鼠标按键语义**（极简设计） | Kron 不写复杂事件处理，让 Windows 系统处理一切 |
| 2026-09-04 | **Kron 完全遵循 Windows 资源管理器行为** | 1:1 复制，单击选中、双击打开、右键系统菜单 |
| 2026-09-04 | **Kron 内部操作放在右侧详情/操作栏** | 选中时显示按钮，不污染右键菜单 |
| 2026-09-04 | **Task 属性精简**：只保留 id/title/description/state/vertex/tags | 简洁 + AI 易读，避免过度设计 |
| 2026-09-04 | **tags 是 string，不是数组** | 用户自由定义，AI 易读自然 |
| 2026-09-04 | **不做 priority/due_date/estimate** | 个人项目不需要这些企业级字段 |
| 2026-09-04 | **Kron 不做内置 AI 辅助功能** | 烂大街功能，避免绑定 AI 服务 |
| 2026-09-04 | **通过文件结构让 AI 自然理解项目** | Kron = 数据层，AI 工具 = 智能层 |
| 2026-09-04 | **重要 idea 放 KRON/important/** | AI 可在工作文件夹直接读取 |
| 2026-09-04 | **CLI 提供 .kron-context/ 中间文档** | 让 AI 无需执行命令即可读懂项目 |
| 2026-09-04 | **Kron 设计哲学：简洁 + AI 易读 + 可持续** | 通过文件结构保障项目长期可维护 |
| 2026-09-04 | **Vertex 创建时绑定当前 Git 分支** | task 不再单独绑定分支，简化设计 |
| 2026-09-04 | **Vertex 与分支绑定不随分支切换自动改** | 绑定关系是用户意图，保持稳定 |
| 2026-09-04 | **Vertex 关系由 Git 树遍历，不存 Kron** | 双源数据不一致风险，Git 是唯一真理 |
| 2026-09-04 | **Vertex 允许多个绑定点** | 同一 Vertex 可在不同 commit 重新启动 |
| 2026-09-04 | **Vertex 描述独立 description.md** | 阶段意图（为什么）vs task 内容（做什么）分离 |
| 2026-09-04 | **Vertex 范围 = Git range（描边可视化）** | 状态信息仅 GUI 维护，不写入 MD |
| 2026-09-04 | **task 不再单独存 state 字段** | 状态仅在 GUI 中（拖拽 = 状态变化） |
| 2026-09-04 | **tags 单独存 Kron 内部 JSON** | 不污染 MD 文件 |
| 2026-09-04 | **description 允许自由修改，不追加** | 仅保留 updated_at，简洁 |
| 2026-09-04 | **Kron 后台守护进程监听文件变化** | 主动权在 Kron，不在 Git |
| 2026-09-04 | **Git Hooks 完全删除** | Kron 自己监听 = 唯一机制 |
| 2026-09-04 | **Task 直接编辑 MD，不需要 CLI** | 主动权在用户/AI 而非 Kron |
| 2026-09-04 | **唯一 task CLI 命令：kron task check** | 格式检查 + 同步，非内容编辑 |
| 2026-09-04 | **Task 状态转换仍需 CLI（AI 工具友好）** | AI 没有 GUI，但需要操作状态 |
| 2026-09-04 | **kron task move / back / start / done / todo** | 不需要记 ID，模糊匹配 + 交互式选择 |
| 2026-09-04 | **CLI 命令精简到 ~10 个** | 子命令组织，移除冗余命令 |
| 2026-09-04 | **软件本体与数据目录分离** | `software/` + `data/` 在同一父目录 |
| 2026-09-04 | **数据目录在 Kron 安装绝对路径** | 不使用 Windows 用户目录 |
| 2026-09-04 | **默认安装位置：D:\Apps\Kron\** | 非系统盘，便携 |
| 2026-09-04 | **分发方式：zip 解压即用** | 无安装器，无注册表 |
| 2026-09-04 | **项目数据目录用人类可读项目名** | `<项目名>/` 而非 `<hash>/` |
| 2026-09-04 | **项目目录 KRON/ 是软链接** | 软链接到 data/projects/<name>/KRON/ |
| 2026-09-04 | **kron init 自动创建软链接** | 用户无感知 |
| 2026-09-05 | **备份仅保留最新版，不保留历史版本** | 简单 + Git 已有历史 |
| 2026-09-05 | **重要文件回滚依赖 Git** | 不做版本管理，利用 Git 的版本历史 |

---

## 🔄 待讨论问题

> 以下问题需要用户进一步明确或讨论

1. [ ] Kron 内部数据是否加密？
2. [ ] 云盘自动备份 UI？
3. [ ] Vertex 删除时是否需要"保留 task"选项（当前默认 Vertex + Task 一起删）
4. [ ] Kron 后台监控频率（默认 5 分钟扫描过期文档，是否需要更频繁？）
5. [ ] Kron 是否在用户首次启动时引导设置默认 MD 应用？
6. [ ] ShellExecute 失败时的 fallback 策略
7. [ ] Vertex 默认值是否预设？（需求分析问题，UI/布局不在此阶段讨论）
