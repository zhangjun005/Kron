# 05 GUI 设计（草案 v1）

> 本文档定义 Kron 桌面 GUI 的架构、视图、交互与边界。
> 状态：**草案 v1（2026-09-05）— 待用户拍板后定稿**
> 读者：实现者（前端 + Tauri 后端）
> 前置阅读：[requirements § 2 任务管理 / § 4 重要文件夹 / § 8 双主题 / 🚫 明确不做](./../requirements.md)、[00-总览与架构.md](./00-总览与架构.md)

---

## 0. 设计原则（最高优先级）

### P1：项目 = Kron 管理的最大单位 ⭐⭐⭐⭐⭐

**Kron 不做"Workspace / Group / Project / ..." 的更高层级抽象**。

| 设计决定 | 含义 |
|---------|------|
| ❌ 不做跨项目任务聚合 | v1 不实现 |
| ❌ 不做项目模板 | v1 不实现 |
| ❌ 不做项目分组/标签 | v1 不实现 |
| ✅ 1 项目 = 1 Git 仓库 = 1 个 KRON/ 目录 = 1 个 kron-internal 实例 | 1:1:1:1 |

> **理由**：用户的项目本就独立（不同 Git 仓库、不同语言栈、不同人）。再加抽象 = 噪音。

### P2：Steam 式首页（多项目入口） ⭐⭐⭐⭐⭐

**Kron.exe 启动后第一个画面 = 项目选择**（参考 Steam 多用户启动界面）。

```
┌──────────────────────────────────────────────────────────┐
│  Kron                                          [⚙️] [✕]   │  ← 顶栏
├──────────────────────────────────────────────────────────┤
│                                                          │
│   [项目 A]  [项目 B]  [项目 C]  [+]                       │
│   ┌─────┐  ┌─────┐   ┌─────┐                             │
│   │ A   │  │ B   │   │ C   │                             │
│   │path │  │path │   │path │                             │
│   │3v   │  │2v   │   │5v   │                             │
│   │12t  │  │7t   │   │20t  │                             │
│   └─────┘  └─────┘   └─────┘                             │
│                                                          │
├──────────────────────────────────────────────────────────┤
│  状态栏：[daemon 运行] ·  [3 个项目] ·  [主题: 🌙]        │
└──────────────────────────────────────────────────────────┘
```

**项目卡片字段**：
- 顶部：项目名（大字）
- 中部：项目路径（截断显示 + tooltip 完整路径）
- 下部：vertex 数量 + task 总数 + 最近打开时间
- 角标：冲突数（如有，红色 badge）

**[+] 卡片** = 新建项目（弹原生目录选择对话框 → 选含 Git 仓库的目录）

**点击已存在卡片** = 进入该项目主窗口（V2-V6 视图）

### P3：Vertex 是 Git 子树的标记 ⭐⭐⭐⭐⭐

**Vertex 不存关系，只存绑定点**——关系由 commit graph 自动推导。

| 设计决定 | 含义 |
|---------|------|
| ❌ Vertex 不存"父 vertex" / "子 vertex" | 不在 Kron 内 |
| ❌ 不做 Vertex DAG 可视化 | v1 不实现（v2 考虑） |
| ✅ Vertex = Git commit 上的标记点（bind_point） | Git 是唯一真理源 |
| ✅ 同一 Vertex 可有多个 bind_point（在不同 commit 重新启动） | 同一 Vertex 名可多次创建 |
| ✅ 关系由 commit graph 反推 | daemon 5 分钟算一次，结果缓存 |
| ✅ 用 `git rev-list --ancestry-path --first-parent` | **严格 DAG 语义** |
| ❌ 不用 `git log --pretty=oneline`（array 化） | **拓扑错误风险** |

**⚠️ Git 是 DAG，不是数组**——详见 § 0.5《Git DAG 处理原则》。

**GUI 呈现**：
- 顶点列表 = 树状/时间轴，**不画连线**（连接由 Git 图表达）
- 看板按 vertex 分组，**不做 vertex 之间的关系箭头**
- V5 展示用 `git log --graph --oneline` 同构图（不是线性时间轴）

### P4：GUI 不"做事"，只"呈现 + 跳转" ⭐⭐⭐⭐⭐

| GUI 做 | GUI 不做 |
|-------|---------|
| ✅ 显示 task / vertex / 重要文件 | ❌ 编辑 MD 内容 |
| ✅ 拖拽改 state（GUI 独有） | ❌ 解析 MD 内容（H3 子节） |
| ✅ 弹 ShellExecute 打开外部编辑器 | ❌ 内嵌编辑器 |
| ✅ 显示冲突状态 + 触发冲突向导 | ❌ 自动 merge |
| ✅ 显示 daemon 状态 | ❌ 控制 daemon（仅托盘菜单） |
| ✅ 调 LLM | ❌ 调 AI SDK |

### P5：鼠标交互 = Windows 资源管理器 1:1 ⭐⭐⭐⭐⭐

| 操作 | 行为 | 实现 |
|------|------|------|
| 左键单击 | 选中（高亮） | Tauri onClick |
| 左键双击 | 打开（ShellExecute） | Tauri onDoubleClick（≤500ms） |
| 右键单击 | Windows 原生菜单 | 系统自动 |
| Ctrl + 单击 | 多选切换 | Kron 前端 |
| Shift + 单击 | 范围选 | Kron 前端 |
| 拖拽 | 系统级拖动 | Tauri 拖拽 API |

### P6：双主题极简 ⭐⭐⭐⭐⭐

- 浅色 / 深色，无第三种
- 主色调中性（不用纯白/纯黑）
- 对比度 ≥ 4.5:1
- 切换：设置窗口 / 托盘菜单

---

## 0.5 Git DAG 处理原则 ⭐⭐⭐⭐⭐

> **Git 是 DAG（有向无环图），不是数组。** 任何涉及 git 历史遍历的设计都必须按 DAG 处理，绝不能用数组思维简化。

### 0.5.1 为什么必须强调

Git 提交历史是**有向无环图**：

- 每个 commit 可能有**多个父**（merge commit）
- 每个 commit 可能有**多个子**（branch point）
- ancestor 关系是**偏序**（partial order），不是全序
- "距离" 在 DAG 上**没有唯一定义**（不同路径长度不同）

如果把 git 当数组处理，会出现以下错误：

| 错误假设 | 实际后果 |
|---------|---------|
| "git log 顺序 = 拓扑顺序" | merge commit 在 log 里只出现一次，但拓扑上可被多次经过 |
| "距离 = commit 数" | merge commit 走不同父得到不同距离，必须指定 `--first-parent` |
| "最近的 bind_point = 时间最近" | bind_point 时间与拓扑距离**无相关性** |
| "vertex A → vertex B = 线性顺序" | 同一 commit 可能同时属于多个 vertex 范围（feature 分支 merge 后） |

### 0.5.2 正确 vs 错误做法对照表

| 场景 | ❌ 错误（array 化） | ✅ 正确（DAG 化） |
|------|-------------------|------------------|
| 找 HEAD 到某 bind_point 的拓扑距离 | `git log --pretty=oneline` 数行 | `git rev-list --ancestry-path HEAD..commit --count` |
| 计算 vertex 范围（commit 集合） | `git log commitA..commitB` | `git rev-list --ancestry-path commitA..commitB --first-parent` |
| 遍历 merge commit | 当成 1 个节点处理 | 必须决定走哪条父链：`--first-parent`（feature 主线）或全部父 |
| `find_nearest_bind_point` 返回值 | `(BindPoint, usize)`（步数） | `(BindPoint, AncestorDist)`（含 first-parent 标记） |
| 列出"最近 commit" 用于 UI | `git log -n N` 顺序列表 | `git log --graph -n N --oneline` 拓扑序扁平化 |
| active_vertex 自动重算 | 在"按时间排序的 bind_points 数组"里二分 | 在"HEAD 到 root 的拓扑路径"上线性扫，必须处理多父 |
| 两个 commit 的共同祖先 | 假设只有一个 ancestor | 用 `git merge-base`（可能有多个，merge-base --octopus） |
| 跨分支的 vertex 范围 | `branchA..branchB` 直接差 | `merge-base(branchA, branchB)..branchB` |

### 0.5.3 实现约束（硬性）

- ✅ **必须**用 `git rev-list --ancestry-path` 处理拓扑祖先关系
- ✅ **必须**用 `--first-parent` 处理 feature 分支语义（默认走第一个父）
- ✅ **必须**用 `git merge-base --octopus` 处理多祖先合并
- ✅ **必须**在所有 git 操作前后处理"detached HEAD"状态
- ❌ **绝不**用 `git log --pretty=oneline` 做"距离"计算
- ❌ **绝不**假设 commit 数组有线性顺序
- ❌ **绝不**用 `Vec<Commit>` 表示"git 历史"（仅 UI 展示时可临时扁平化）
- ❌ **绝不**在 Rust 代码里手动模拟 git 拓扑（必须调 `git` CLI 或 libgit2）

### 0.5.4 Kron 中的具体应用

| Kron 行为 | DAG 处理方式 |
|----------|------------|
| **active_vertex 计算** | daemon 监听 HEAD 变化 → `git rev-list --ancestry-path HEAD --first-parent` 得到拓扑路径 → 在路径上找最近 bind_point |
| **vertex 范围计算** | 给定 from/to commit → `git rev-list --ancestry-path from..to --first-parent` |
| **VertexSummary.latest_bind** | ⚠️ 仅按 created_at 时间排序，**不**是拓扑顺序（语义不同）|
| **context/recent-commits.md** | `git log --graph --oneline -n 100`（UI 用，扁平化展示）|
| **GUI V5 Vertex 关系图** | 用 `git log --graph --oneline` 同构渲染，**不画线性时间轴** |
| **拖拽 task → state 变化** | 与 git 拓扑**完全无关**（只与 vertex 名称相关）|
| **重要文件双源同步** | 与 git 拓扑**完全无关**（只与文件路径相关）|

### 0.5.5 性能与缓存

- daemon 每 5 分钟算一次完整 git graph，结果缓存到 `kron-internal/git-graph-cache.json`
- GUI 按需通过 IPC 拉，**不直接调 git CLI**
- 缓存结构：

```json
{
  "computed_at": "2026-09-05T20:00:00Z",
  "head_commit": "abc1234",
  "ancestry_path_first_parent": ["abc1234", "def5678", "ghi9012"],
  "bind_point_resolutions": {
    "abc1234": "开发",
    "def5678": "重构",
    "ghi9012": "开发"
  },
  "active_vertex": "开发"
}
```

- 触发重算：`.git/HEAD` 变化 / `.git/refs/heads/*` 变化（见 § 3.3 `GitRefChanged`）

### 0.5.6 错误检测（实现期 checklist）

实现时如出现以下代码，**立即重审**：

- [ ] 用 `git log` 数 commit 数（应改为 `git rev-list --count`）
- [ ] 在 Rust 里遍历 `Vec<Commit>` 找 ancestor（应改为 `git merge-base`）
- [ ] 按数组顺序处理 commit 历史（应改为拓扑序）
- [ ] 假设 HEAD 有且仅有 1 个父（应检查 merge commit）
- [ ] 用 `last()` / `first()` 找"最近"bind_point（应改为拓扑遍历）

---

## 1. 视图架构（V0-V6，共 7 个视图）

### V0：项目首页（首页，强制显示）

见 P2 图。**所有 Kron.exe 启动都从 V0 开始**。

### V1：项目主窗口（进入项目后）

```
┌──────────────────────────────────────────────────────────────┐
│  [项目 A] 路径/to/A                              [⚙️] [✕]    │  ← 顶栏
├──────────────┬───────────────────────────────────────────────┤
│ 侧边栏        │ 主区（Tab 切换）                              │
│ ────────     │ ─────────                                    │
│ 📋 看板      │ [看板][重要文件][冲突][Vertex 关系][Context]   │
│ 📁 重要文件  │                                              │
│ ⚠️ 冲突 (2)  │                                              │
│ 🌳 Vertex   │                                              │
│ 📖 Context  │                                              │
│ ⚙️ 设置     │                                              │
├──────────────┴───────────────────────────────────────────────┤
│ 状态栏：[daemon ✓] · [vertex: 开发] · [task: 12] · [冲突: 2]  │
└──────────────────────────────────────────────────────────────┘
```

**主区 5 个 Tab**（V2-V6）：

### V2：任务看板（主视图，最常用）⭐⭐⭐⭐⭐

**布局**：

```
┌─ 看板 / vertex: [开发 ▼]  [过滤: 标签 ▼] [优先级 ▼] [搜索]  [新建] ─┐
│                                                                   │
│  Todo (3)        Doing (1)     Done (5)     Back (0)  Blocked (1)│
│  ┌──────────┐    ┌──────────┐   ┌──────────┐                    │
│  │ task_004 │    │ task_001 │   │ task_002 │                    │
│  │ OAuth    │    │ OAuth    │   │ DB       │                    │
│  │ 高 #auth │    │ 高 #auth │   │ 中 #db   │                    │
│  │ ↳依赖 005│    │ ↳依赖 002│   │          │                    │
│  └──────────┘    └──────────┘   └──────────┘                    │
│                                                                   │
│  [+ 新建 task]   (拖拽改变 state)                                 │
└───────────────────────────────────────────────────────────────────┘
```

**5 列固定**（与 01-数据模型枚举对齐）：Todo / Doing / Done / Back / Blocked

**卡片信息**：
- task ID（task_001）
- title（首行）
- 优先级徽章（高/中/低/紧急）
- 标签徽章（前 3 个 + "+N"）
- 依赖箭头（如有）

**拖拽行为**：
- 拖到 Done / Todo 等 → state 变
- 拖到 Blocked → state 变 + 弹"原因"输入框（可选）
- **拖到 Done 但存在冲突** → 弹冲突向导（O3 选项 C）

**新建 task**：
- 点击列底部 **[+ 新建 task]** → ShellExecute 打开当前 vertex 的 `tasks.md`
- 光标定位到文件末尾 + task 模板

### V3：重要文件浏览器

**布局**（与 requirements § 4.8 完全一致）：

```
┌─────────────────────────────────────────┬──────────────────────────┐
│ 重要文件列表（左 70%）                     │ 详情/操作栏（右 30%）    │
├─────────────────────────────────────────┼──────────────────────────┤
│ 📄 README.md                            │ 文件名：TODO.md          │
│ 📄 TODO.md        ← 选中（高亮）        │ 大小：4.2 KB             │
│ 📄 api-spec-v1.md                       │ 创建：2026-09-04         │
│ 📁 important/                           │ 修改：2026-09-04         │
│                                         │ 双源状态：已同步 ✓       │
│                                         │ 备份位置：<kron_internal>│
│                                         │                          │
│                                         │ ┌─ Kron 内部操作 ────┐  │
│                                         │ │ 🔄 立即同步双源    │  │
│                                         │ │ 📋 复制 Kron 路径  │  │
│                                         │ │ 📂 在文件管理器    │  │
│                                         │ │ 🗑️ 从 Kron 中移除  │  │
│                                         │ │ 📜 查看修改历史    │  │
│                                         │ └────────────────────┘  │
└─────────────────────────────────────────┴──────────────────────────┘
```

**交互**（与 requirements § 4.7 一致）：
- 左键单击 = 选中 → 右侧详情栏更新
- 左键双击 = ShellExecute 打开（用 Windows 默认应用）
- 右键 = Windows 原生菜单（Kron 不实现）

### V4：冲突解决向导

**触发**：
- 用户拖拽 task 时存在 pending 冲突
- daemon 检测到新冲突
- 用户主动打开"冲突"Tab

**3 选 1 对话框**：

```
┌─ 冲突 c-abc123 ──────────────────────────────────────────┐
│ 路径：KRON/important/notes.md                            │
│ 类型：双源都修改（both_modified）                         │
│                                                          │
│ ┌─ 内部（kron-internal）─────────┬─ 项目（KRON/）─────┐ │
│ │ 修改时间：17:55                 │ 修改时间：17:58    │ │
│ │ 大小：1024 B                   │ 大小：1100 B       │ │
│ │ SHA：abc123...                 │ SHA：def456...     │ │
│ │                                │                    │ │
│ │ (预览前 500 字)                │ (预览前 500 字)    │ │
│ │ Lorem ipsum...                 │ Lorem ipsum dolor..│ │
│ └────────────────────────────────┴────────────────────┘ │
│                                                          │
│  [保留内部版本] [保留项目版本] [手动合并并覆盖]  [取消]    │
└──────────────────────────────────────────────────────────┘
```

**3 选项含义**：
- **保留内部版本**：project 端 = 内部门容
- **保留项目版本**：internal 端 = project 内容
- **手动合并并覆盖**：ShellExecute 打开外部编辑器 → 用户合并 → 关闭后回到 Kron 触发双源同步

### V5：Vertex 关系展示

**布局**（**与 `git log --graph --oneline` 同构**，**不画线性时间轴**）：

```
┌─ Commit Graph（DAG）────────────────────────────────────────┐
│ 显示范围：从 HEAD 向上 50 个 commit                          │
│ 算法：git log --graph --oneline -n 50 --first-parent        │
│                                                            │
│ * HEAD (main)                                               │
│ |\                                                          │
│ | * a1b2c3d  ← 🏷️ "开发" bind 1  (feature/login)          │
│ | |   实现 OAuth 登录接口                                   │
│ | * e4f5g6h  ← 🏷️ "重构" bind 1                           │
│ |/      拆分 auth 模块                                       │
│ | * i7j8k9l    修复 token 过期                              │
│ * m0n1o2p  ← 🏷️ "开发" bind 2  (feature/api)              │
│ |\                                                          │
│ | * q3r4s5t  ← 🏷️ "开发" bind 3  (fix/auth)               │
│ | |   修复登录态丢失                                         │
│ | * u6v7w8x    补单元测试                                    │
│ |/                                                          │
│ * y9z0a1b   (merge base)                                    │
│ |                                                           │
│ | * c2d3e4f   (hotfix branch, 无 bind)                      │
│ |/                                                           │
│ * g5h6i7j   (init)                                          │
│                                                            │
├────────────────────────────────────────────────────────────┤
│ 鼠标交互：                                                  │
│ - 悬停 commit 行 → 高亮 + 显示完整 hash + 标题              │
│ - 悬停 🏷️ 标签 → 显示 vertex 名 + 该 vertex 的 task 数      │
│ - 点击 🏷️ 标签 → 切换到该 vertex 看板                       │
│ - 点击 commit → 弹"在编辑器打开 KRON/VERTEX/<v>/tasks.md"   │
└────────────────────────────────────────────────────────────┘
```

**关键设计**：

- ✅ **与 git log --graph 完全同构**——不画"线性时间轴"，画"DAG 分支图"
- ✅ 用 ASCII 字符 `* | \ /` 表达拓扑（不要用框图符号）
- ✅ 每个 commit 行后用 `← 🏷️` 标注 vertex bind_point（仅在该 commit 是 bind 时显示）
- ✅ `--first-parent`：只走第一个父，避开 merge commit 的多父语义
- ✅ 鼠标悬停显示 task 数（数据从 `kron-internal/git-graph-cache.json` 拉）

**不画什么**：

- ❌ **不画 vertex 之间的箭头**（连接由 git 图自然表达）
- ❌ **不画线性时间轴**（abc1234 → def5678 → ...）
- ❌ **不画 DAG 节点框图**（避免 UI 复杂度，与 git CLI 风格统一）

**复制 git range**（按钮）：

- 用户选中范围起止两个 commit → 生成 `git rev-list --ancestry-path A..B --first-parent` 列表
- 复制到剪贴板（用户可粘贴给 AI / 同事）
- **范围语义**：从 A 的下一个 commit 到 B（含 B）的拓扑路径（**不是 array**）

**当前 vertex 标记**：

- V5 顶部显示 `当前 vertex：[开发 ▼]`
- 该 vertex 在 DAG 中的所有 bind_point 用**不同颜色徽章**高亮（如绿色 🏷️）
- 其他 vertex 的 bind_point 用灰色 🏷️

### V6：Context 查看器（只读）

```
┌─ .kron-context/ ──────────────────────────────────────┐
│ 总文件：3  ·  过期：0  ·  总大小：12 KB                 │
│                                                       │
│ ✓ git/recent-commits.md   (4.8 KB, 5 秒前生成)        │
│ ✓ git/branch-summary.md   (2.1 KB, 30 秒前生成)       │
│ ✓ code/structure.md       (1.0 KB, 2 分钟前生成)      │
│                                                       │
│ [刷新全部]  [在文件管理器打开]  [查看 README]          │
└───────────────────────────────────────────────────────┘
```

**只读**——Kron 不提供"编辑 context"入口（context 自动生成）。

---

## 2. 关键交互流程（用户故事）

### 2.1 用户首次启动

```
1. 用户双击 kron.exe
   ↓
2. V0：项目首页（空）
   - 中间：[+ 新建项目]（大按钮）
   ↓
3. 用户点击 [+]
   ↓
4. 弹原生目录选择对话框
   ↓
5. 用户选择含 .git 的目录
   ↓
6. 检测：未初始化 → 弹"是否初始化 Kron？"对话框
   - [初始化] → `kron init` 自动跑
   - [取消]
   ↓
7. 初始化完成 → 返回 V0，添加新项目卡片
   ↓
8. 用户点击新项目卡片
   ↓
9. 进入 V1（项目主窗口，默认 Tab = 看板 V2）
```

### 2.2 用户新建 task

```
用户在 V2 看板
   ↓
1. 点击 Todo 列底部 [+ 新建 task]
   ↓
2. ShellExecute 打开 <project>/KRON/VERTEX/<current>/tasks.md
   （用 Windows 默认 .md 应用：Typora / VSCode / Cursor）
   ↓
3. 用户在编辑器里按格式写 task
   ↓
4. 用户保存 tasks.md
   ↓
5. daemon 检测文件变化（≤2 秒）
   ↓
6. daemon 解析 → 更新内存状态
   ↓
7. daemon 通过 IPC 通知 GUI
   ↓
8. GUI V2 看板：列底部插入新卡片（带滑入动画）
```

### 2.3 用户拖拽 task 改变 state

```
用户在 V2 看板
   ↓
1. 鼠标按住 task_001 卡片
   ↓
2. 拖到 Doing 列
   ↓
3. 释放鼠标
   ↓
4. GUI 立即更新：task_001 从 Todo 列移到 Doing 列
   ↓
5. 内存 state 变化
   ↓
6. daemon 5 分钟 cron 写 states/<vertex>.json
   ↓
   （如同时存在冲突 → 弹 V4 向导，不允许 drop）
```

### 2.4 用户编辑 task description

```
用户在 V2 看板
   ↓
1. 左键单击 task_001 卡片（选中）
   ↓
2. 右侧详情栏显示：
   - 标题 / 描述预览（只读）
   - [在编辑器中打开] 按钮
   ↓
3. 用户点击 [在编辑器中打开]
   ↓
4. ShellExecute 打开 tasks.md（光标跳到 task_001 处）
   ↓
5. 用户在 VSCode 修改 description
   ↓
6. 保存 → daemon 检测 → 看板卡片刷新
```

### 2.5 用户遇到冲突

```
daemon 检测到双源不一致
   ↓
1. daemon 写 conflicts/c-abc123.json
   ↓
2. daemon IPC 通知 GUI
   ↓
3. GUI 状态栏：[冲突: 1]（红色）
   ↓
4. 用户切到"冲突"Tab（V4）→ 看冲突列表
   ↓
5. 点击 c-abc123 → 弹冲突向导
   ↓
6. 用户选 [保留项目版本]
   ↓
7. daemon 同步：internal = project 内容
   ↓
8. 冲突标记为 resolved
   ↓
9. GUI 状态栏更新：[冲突: 0]
```

---

## 3. GUI 与 daemon 通信

### 3.1 通信方式

**Tauri IPC**（Rust 端 invoke / event）

| 方向 | 通道 | 用途 |
|------|------|------|
| GUI → daemon | `invoke('daemon_command', args)` | 请求（查询/操作） |
| daemon → GUI | `emit('daemon_event', payload)` | 推送（文件变化/状态变更） |

### 3.2 IPC 命令清单

```rust
// 项目管理
kron_project_list() -> Vec<ProjectMeta>
kron_project_add(path: PathBuf) -> Result<ProjectMeta>
kron_project_remove(project_id: ProjectId) -> Result<()>

// Task 操作
kron_task_list(vertex: String) -> Vec<Task>
kron_task_get(id: TaskId) -> Task
kron_task_add(vertex: String, opts: AddOpts) -> Task   // 打开编辑器（GUI 调 ShellExecute）
kron_task_move(id: TaskId, new_state: State) -> Result<()>  // 拖拽用
kron_task_delete(id: TaskId) -> Result<()>

// 标签
kron_tag_list() -> Vec<Tag>
kron_tag_add(task_id: TaskId, tag: String) -> Result<()>
kron_tag_remove(task_id: TaskId, tag: String) -> Result<()>

// Vertex
kron_vertex_list(project_id: ProjectId) -> Vec<VertexMeta>
kron_vertex_current(project_id: ProjectId) -> Option<String>
kron_vertex_use(project_id: ProjectId, name: String) -> Result<()>  // ⭐ 新增
kron_vertex_create(name: String) -> Vertex  // 绑定当前 HEAD

// 重要文件
kron_important_list(project_id: ProjectId) -> Vec<FileMeta>
kron_important_sync(path: PathBuf) -> Result<()>
kron_important_restore(path: PathBuf) -> Result<()>

// 冲突
kron_conflict_list(project_id: ProjectId) -> Vec<ConflictMeta>
kron_conflict_show(id: ConflictId) -> Conflict
kron_conflict_resolve(id: ConflictId, choice: ResolveChoice) -> Result<()>

// Context
kron_context_list(project_id: ProjectId) -> Vec<ContextFileMeta>
kron_context_refresh(project_id: ProjectId) -> Result<()>

// 文件打开
kron_open_with_system(path: PathBuf) -> Result<()>  // ShellExecute
```

### 3.3 IPC 事件清单（daemon → GUI 推送）

```rust
// 文件变化
task_file_changed(vertex: String, change: FileChange)
vertex_file_changed(vertex: String, change: FileChange)

// 状态变化（拖拽后立即推送，不等 5 分钟 cron）
task_state_changed(task_id: TaskId, new_state: State)

// Git DAG 变化（详见 § 0.5）
// 触发链：daemon 监听 .git/HEAD 或 .git/refs/heads/* 变化
//   → 重算 git-graph-cache.json（用 git rev-list --ancestry-path --first-parent）
//   → 重算 active_vertex
//   → 推送以下事件给 GUI
git_graph_updated(graph: GitGraphSnapshot)        // 完整图快照（含 ancestry_path + bind_point_resolutions）
active_vertex_changed(new_vertex: Option<String>) // active_vertex 切换（GUI 看板自动切换）

// 冲突
conflict_detected(conflict: ConflictMeta)
conflict_resolved(conflict_id: ConflictId)

// Daemon 状态
daemon_status_changed(status: DaemonStatus)
context_stale_changed(file: String, is_stale: bool)
```

**GitGraphSnapshot 结构**（与 § 0.5.5 缓存对应）：

```rust
struct GitGraphSnapshot {
    computed_at: DateTime<Utc>,
    head_commit: String,
    /// HEAD 到 root 的 first-parent 路径（**拓扑序**，不是 array）
    ancestry_path_first_parent: Vec<String>,
    /// commit → vertex 名（拓扑路径上每个 commit 所属的 vertex）
    bind_point_resolutions: HashMap<String, String>,
    /// 当前 active vertex（按拓扑路径上最近的 bind_point 决定）
    active_vertex: Option<String>,
}
```

---

## 4. 与已有架构的边界

| 关注点 | 谁负责 |
|--------|--------|
| 监听文件变化 | **daemon**（notify crate） |
| MD 解析/校验 | **daemon** |
| 双源同步 | **daemon** |
| 冲突检测 | **daemon** |
| task state 持久化 | **daemon**（5 分钟 cron 写 states/<vertex>.json）|
| Git 树遍历 | **daemon**（libgit2） |
| Active vertex 计算 | **daemon**（每次 git ref 变化重算）|
| 渲染看板 | **GUI 前端** |
| 拖拽交互 | **GUI 前端** |
| 双主题切换 | **GUI 前端**（Zustand 状态）|
| ShellExecute | **GUI Rust 端**（opener::open）|
| 守护进程启停 | **GUI 托盘菜单**（不通过主窗口）|

---

## 5. 待补全的设计点（你拍板后填）

| ID | 项 | 需要决定 |
|----|---|---------|
| **D1** | active_vertex 持久化字段位置 | `kron-internal/config.json` 加 `active_vertex: String \| null`？ |
| **D2** | `kron vertex use <name>` 命令 | 与 `kron vertex create` 区别？ |
| **D3** | daemon → GUI 推送用轮询还是 IPC | 倾向 IPC（Tauri event） |
| **D4** | V0 首页"最近项目"排序 | 按 mtime 还是显式置顶？ |
| **D5** | 看板拖拽到 Blocked 列是否必填"原因" | 必填 / 可选 / 不填 |
| **D6** | 项目卡片右键菜单 | 全用 Windows 原生？还是 Kron 加几项？ |

---

## 6. 已知约束（不实现）

| ❌ | 原因 |
|----|------|
| 内置 MD 编辑器 | requirements § 🚫 L3970 |
| task CLI 子命令（已改为保留） | 拍板 X3：保留，参考 Git |
| 快捷键 | requirements § 🚫 L3978 |
| 自定义右键菜单 | requirements § 4.7 |
| Vertex DAG 可视化 | 拍板 P3：v1 只画时间轴 |
| Vertex 手动 bind | requirements § 🚫 L3994 |
| AI 辅助（任何形式） | requirements § 🚫 L3975 |
| 多用户协作 | requirements § 🚫 L3979 |
| 云端同步 | requirements § 🚫 L3980 |
| 插件系统 | requirements § 🚫 L3981 |
| Git 可视化操作 | requirements § 🚫 L3983 |
| Linux/macOS 支持 | requirements 锁 Windows；架构层已解耦（v1 不实现） |

---

## 7. 后续：补 active_vertex 与 `kron vertex use`

基于你的提议，建议**补 2 处**：

### 7.1 `kron-internal/config.json` 加 `active_vertex`

```json
{
  "active_vertex": "开发",       // ⭐ 新增
  ...
}
```

### 7.2 新增 `kron vertex use <name>` CLI 命令

```
kron vertex use <name>    # 切换当前 vertex（写到 config.active_vertex）
kron vertex use --clear   # 清除（= null，由 daemon 按 git HEAD 自动推断）
```

**用途**：
- 拖拽看板时默认显示 active vertex
- `kron task add` 不带参数时知道加到哪
- daemon 监听 git HEAD 变化时，**自动重算 active_vertex**（按 commit 找最近 bind point）

---

## 8. 视图速查表

| 视图 | ID | 触发 | 主要交互 | 是否核心 |
|------|----|------|---------|---------|
| V0 项目首页 | startup | Kron.exe 启动 | 点卡片 | ✅ 必做 |
| V1 项目主窗口 | project | 点项目卡片 | 切 Tab | ✅ 必做 |
| V2 任务看板 | kanban | 主窗口默认 Tab | 拖拽 | ⭐ 核心 |
| V3 重要文件 | important | 侧边栏 | 双击 + 右侧详情 | ✅ 必做 |
| V4 冲突向导 | conflict | 冲突 Tab 或拖拽触发 | 3 选 1 | ✅ 必做 |
| V5 Vertex 关系 | vertex | 侧边栏 | 时间轴 | ✅ 必做 |
| V6 Context 查看 | context | 侧边栏 | 只读 | ⭐ 重要 |

---

**本文档维护规则**：
- P1-P6 核心原则不变 → 文档结构稳定
- V0-V6 视图变更 → 必须更新 § 1 + § 8
- IPC 命令/事件变更 → 必须更新 § 3
- 新增 § 🚫 排除项 → 必须更新 § 6
