# 05b GUI 技术选型

> 配套文档：定义 05-GUI 设计如何落地的**具体技术栈、库选型、目录结构、IPC 契约实现**。
> 状态：**初稿 v1（2026-09-08）**
> 拍板：用户已认可 Solid.js + Tauri 2 路线，UI 风格对齐 [Yuushya Townscape](https://yuushya.com/townscape/)（清新/写实/方块小镇）
> 前置阅读：[05-GUI设计.md](./05-GUI设计.md)、[00-总览与架构.md](./00-总览与架构.md)

---

## 0. 选型总结表

| # | 维度 | 选择 | 替代方案（不选） |
|---|------|------|------------------|
| T1 | 进程模型 | **Tauri 2 主进程内嵌 tokio 后台**（不走独立 daemon 二进制） | 独立 daemon 子进程 / 纯 CLI + 浏览器 |
| T2 | 前端框架 | **Solid.js 1.9+**（小、快、看板大量节点重排友好） | React / Svelte / Vue |
| T3 | 构建工具 | **Vite 5 + TypeScript strict** | Rsbuild / Next（不适用） |
| T4 | UI 组件 | **shadcn-solid**（headless + 自己写皮肤） | solidcn（含 MCP，太 AI 味）/ Ant Design（太重） |
| T5 | 无障碍原语 | **Kobalte**（Radix-for-Solid） | corvu / 自写 |
| T6 | 样式方案 | **Tailwind CSS 3 + CSS variables**（双主题切换）+ **Unocss** 选其一 | styled-components / Emotion |
| T7 | 拖拽实现 | **HTML5 Drag and Drop API**（看板只有 4 列，需求简单） | @dnd-kit/solid / react-dnd |
| T8 | V5 Git 图渲染 | **纯文本 `git log --graph --oneline` + 自写着色器**（与文档 § 0.5 同构） | cytoscape / mermaid / SVG 自画 |
| T9 | 文件监听 | **`notify` crate + tokio::sync::mpsc**（已有依赖） | 轮询 |
| T10 | Markdown 渲染 | **不渲染**（V6 只列文件名，Kron 不做内容呈现） | react-markdown / markdown-it |
| T11 | Diff 渲染 | **`diff2html` + 自定义 theme 适配双主题** | jsdiff + 自渲染 |
| T12 | IPC | **Tauri 2 原生 `invoke` / `emit`** | 手搓 WebSocket / 内嵌 HTTP |
| T13 | 文件打开 | **`tauri-plugin-shell` + `opener::open`**（平台默认应用） | 手写 ShellExecute |
| T14 | 跨平台 | **Windows 优先 v1**；Tauri 跨平台免费但不承诺 | 一开始就跨平台（成本 ×3） |
| T15 | 主题切换 | **CSS variables + `data-theme` + Tailwind `darkMode: 'class'`** | MUI / ThemeProvider |
| T16 | 状态管理 | **Solid Store + Signals**（原生） | Zustand / Jotai |
| T17 | 路由 | **@solidjs/router**（V0 ↔ V1 切换，Tab 切换） | 不需要（SPA 简单） |
| T18 | 图标 | **Lucide Icons**（line-style，克制） | Material Icons / FontAwesome |
| T19 | 字体 | **系统默认 + 衬线中文（思源宋体 / 方正书宋 fallback）** | Web 字体（离线要求） |
| T20 | 动画 | **CSS transitions 为主**，`@motionone/solid` 仅用于拖拽反馈 | framer-motion（无 Solid 官方版） |

---

## 1. 进程模型（核心架构）

### 1.1 单 Tauri 主进程，tokio 后台内嵌

```
┌──────────────────────────────────────────────────────────────┐
│                   kron.exe（Tauri 主进程）                       │
├──────────────────────────────────────────────────────────────┤
│  ┌─ WebView 主窗口 ──────────────────────────────────────┐   │
│  │  Solid.js 前端（加载自 dist/）                          │   │
│  │  ├─ V0 首页        ─ 项目卡片网格                       │   │
│  │  ├─ V1 主窗口      ─ 5 个 Tab 切换                     │   │
│  │  ├─ V2 看板        ─ 拖拽改 state                       │   │
│  │  ├─ V3 重要文件    ─ 列表 + 详情栏                      │   │
│  │  ├─ V4 冲突向导    ─ 3 选 1 弹窗                       │   │
│  │  ├─ V5 Vertex DAG  ─ git log --graph 文本渲染            │   │
│  │  └─ V6 Context     ─ 只读文件列表                       │   │
│  └─────────────────────────────────────────────────────────┘   │
│                            ▲ invoke / emit                     │
│                            │                                    │
│  ┌─ Tauri Rust Backend（tokio runtime）───────────────────┐   │
│  │  ├─ #[tauri::command] handlers（kron_*）                 │   │
│  │  ├─ notify watcher ──── KRON/.kron-context/              │   │
│  │  │                    └─ KRON/VERTEX/                    │   │
│  │  │                    └─ kron-internal/important/        │   │
│  │  │                    └─ .git/HEAD, .git/refs/heads/*    │   │
│  │  ├─ git-graph 缓存刷新（5 min cron + 触发式）            │   │
│  │  ├─ Tauri EventBus ──── emit('task_file_changed', ...)   │   │
│  │  └─ 调用 kron CLI 子进程（Command::new("kron")）        │   │
│  └─────────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────┘
```

**为什么单进程：**
- 文档原设计是独立 daemon，但 M2 已经删了；GUI 启动时拉起后台监听比独立 daemon 简单
- Tauri 主进程本身就是常驻进程，承担"daemon"角色
- 不需要 IPC-over-IPC（避免 daemon↔GUI 再加一层）
- 关闭主窗口 = 关闭后台监听（用户期望一致）

**违背文档的点（已知）：**
- 文档 § 4 "监听 = daemon / daemon → GUI 推送" 的 `daemon` 字样需更新为 "Rust 后台 tokio task"
- 后续如果要跨进程（比如服务端模式），需要重构成独立 daemon——**但 v1 不做**

---

## 2. 前端栈细节

### 2.1 Solid.js 1.9+

```jsonc
// package.json (核心依赖)
{
  "dependencies": {
    "solid-js": "^1.9.3",
    "@solidjs/router": "^0.15.0",
    "@kobalte/core": "^0.13.0",
    "@solid-primitives/draggable": "^0.1.0",
    "lucide-solid": "^0.460.0",
    "diff2html": "^3.4.51",
    "@motionone/solid": "^10.16.0"
  },
  "devDependencies": {
    "vite": "^5.4.0",
    "vite-plugin-solid": "^2.11.0",
    "tailwindcss": "^3.4.0",
    "typescript": "^5.6.0",
    "@tauri-apps/cli": "^2.0.0"
  }
}
```

**为什么不选 React：**
- 看板场景频繁列表更新，Solid 的 fine-grained reactivity 优于 VDOM diff
- bundle 体积 ~15KB（React+ReactDOM ~45KB）
- API 接近 React，学习曲线低（`createSignal` vs `useState`，`createMemo` vs `useMemo`）

### 2.2 UI 组件方案：shadcn-solid + 手写皮肤

**选用 [`shadcn-solid`](https://github.com/hngngn/shadcn-solid)（hngngn 维护，社区最成熟）。**

不选用 [`solidcn`](https://github.com/solidcn-ui/solidcn) 的原因：
- solidcn 内置 MCP server 让 AI agent 自动装组件——**这是"AI 味"的典型表现**
- 组件来源不明、版本耦合深
- 我们要的是"自己的代码"（与 shadcn 哲学一致）

shadcn-solid 用法：

```bash
# 初始化（一次性）
npx shadcn-solid@latest init

# 按需取组件（不是装包，是复制源码）
npx shadcn-solid@latest add button dialog tooltip
```

**取回来的组件再改皮肤**——这才是关键。组件骨架（无障碍、键盘交互、ARIA）来自 Kobalte，外观完全由我们决定。

### 2.3 不用 Tailwind 工具类堆 UI

Tailwind 是工具集（utility-first），不是设计语言。如果直接用 `bg-blue-500` 这种，会出来"AI 味"。

**正确用法：**
- 用 Tailwind 的 layout/spacing/typography 工具（这些是工具，不是设计）
- **颜色 / 阴影 / 边框 / 字体**全部走 CSS variables，自己定义 token
- 组件样式用 `@apply` 引用 token，**禁止在组件文件里写 `bg-blue-500` `--text-3xl` 这种**

---

## 3. UI 设计语言（对齐 Yuushya Townscape）

> 用户审美参考：[https://yuushya.com/townscape/](https://yuushya.com/townscape/)（方块小镇，清新/写实/市井气息）

### 3.1 Yuushya 风格翻译成 Kron

| Yuushya 视觉 | Kron GUI 对应 |
|------------|-------------|
| **清新色调**（米白、雾蓝、暖黄、青葱绿、咖啡棕） | 主色不用 Tailwind 默认蓝，用**暖中性**色板 |
| **写实光照**（柔和阴影、真实材质） | 卡片用**双层阴影**（near + far），不用 Tailwind 默认 `shadow-md` |
| **市井烟火气**（不规则、有人情味） | **卡片有圆角但不大（6-8px）**，不用 iOS 大圆角 |
| **字体克制**（衬线标题 + 无衬线正文） | 标题用衬线（思源宋体），正文用系统无衬线 |
| **方块化网格** | 用 **8px 网格**，所有间距都是 8 的倍数 |
| **克制配色**（不超过 3 个色） | 单页面**主色 + 1 强调色 + 1 警告色**，其它用灰阶 |
| **安静留白** | section 之间留 24-32px，不用密集布局 |
| **图标 line-style** | Lucide Icons，**stroke-width 1.5**，不用 2.0 |

### 3.2 颜色 token（CSS variables）

```css
/* src/styles/tokens.css — 浅色（默认，类比 Yuushya 米白） */
:root[data-theme="light"] {
  /* —— 背景层 —— */
  --bg-canvas: #f5f1e8;       /* 米白主背景（Yuushya 暖纸感） */
  --bg-surface: #faf6ed;      /* 卡片表面 */
  --bg-surface-elevated: #ffffff;
  --bg-inset: #ece5d3;        /* 凹陷区域（如输入框背景） */

  /* —— 文字 —— */
  --text-primary: #2d2a26;    /* 深咖色，不用纯黑 */
  --text-secondary: #6b6358;
  --text-muted: #a89e8e;
  --text-inverse: #faf6ed;

  /* —— 边框 —— */
  --border-default: #d8cfb9;
  --border-strong: #b8ad94;
  --border-focus: #c47854;    /* 暖红棕，Yuushya 红屋顶色 */

  /* —— 主色（低饱和暖色，不用 Tailwind blue） —— */
  --color-primary: #c47854;   /* 陶土橙，Yuushya 主色 */
  --color-primary-hover: #b06a48;
  --color-primary-bg: #f0d9c8;

  /* —— 强调色 —— */
  --color-accent: #6b8e5a;    /* 青葱绿，市井植物 */
  --color-accent-bg: #e2ead7;

  /* —— 警告/冲突 —— */
  --color-warning: #d4a04b;   /* 暖琥珀 */
  --color-danger: #b85542;    /* 砖红，不用 Tailwind red-500 */
  --color-danger-bg: #f5dad3;

  /* —— 阴影（双层，写实质感） —— */
  --shadow-near: 0 1px 2px rgba(45, 42, 38, 0.06);
  --shadow-far:  0 4px 12px rgba(45, 42, 38, 0.08);

  /* —— 字体 —— */
  --font-serif: "Source Han Serif SC", "Songti SC", "Noto Serif CJK SC", Georgia, serif;
  --font-sans: "Inter", "PingFang SC", "Microsoft YaHei", system-ui, -apple-system, sans-serif;
  --font-mono: "JetBrains Mono", "Cascadia Code", Consolas, monospace;
}

/* 深色（夜间，类比 Yuushya 街灯下的暖黄） */
:root[data-theme="dark"] {
  --bg-canvas: #1f1d1a;
  --bg-surface: #2a2722;
  --bg-surface-elevated: #353128;
  --bg-inset: #15130f;

  --text-primary: #f5f1e8;
  --text-secondary: #c8bfb0;
  --text-muted: #8a8275;
  --text-inverse: #1f1d1a;

  --border-default: #3d3933;
  --border-strong: #5a5448;
  --border-focus: #d4895e;

  --color-primary: #d4895e;
  --color-primary-hover: #e09b6e;
  --color-primary-bg: #3d2d20;

  --color-accent: #8da878;
  --color-accent-bg: #2d3525;

  --color-warning: #e0b667;
  --color-danger: #d97a6a;
  --color-danger-bg: #3d2520;

  --shadow-near: 0 1px 2px rgba(0, 0, 0, 0.2);
  --shadow-far:  0 4px 12px rgba(0, 0, 0, 0.3);
}
```

### 3.3 视觉反例（绝对禁止）

| 禁用 | 原因 |
|------|------|
| `bg-blue-500` / `bg-indigo-600` | Tailwind 默认色板 = AI 味最浓 |
| `shadow-lg`（默认 box-shadow） | 太"塑料"，不像写实质感 |
| `rounded-2xl`（大圆角 16px+） | 太 iOS/Material，不像 Yuushya |
| `bg-gradient-to-r from-purple-500 to-pink-500` | 彩虹渐变 = AI 味之王 |
| `text-3xl font-extrabold` | 标题字号克制（22-28px），不全用粗体 |
| 卡片 hover `scale-105` | 太活泼，Yuushya 是静态美感 |
| 霓虹色边框 / 玻璃拟态 | 与 Yuushya 格格不入 |
| 大量 emoji 作图标 | V0 状态栏的 `[⚙️]` 等保留作为占位 |

### 3.4 V0 首页视觉示例（按 Yuushya 风格重画）

```
┌──────────────────────────────────────────────────────────────┐
│                                                              │
│   Kron                                                       │
│   ────                                                       │
│   项目管理 · Git 之上的任务追踪层                                  │
│                                                              │
│   ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌────┐    │
│   │            │  │            │  │            │  │ +  │    │
│   │  Kron      │  │  Yuushya   │  │  MyNotes   │  │新  │    │
│   │            │  │            │  │            │  │建  │    │
│   │  ~/kron    │  │  ~/yuu     │  │  ~/notes   │  │    │    │
│   │            │  │            │  │            │  └────┘    │
│   │  3 vertex  │  │  5 vertex  │  │  2 vertex  │            │
│   │  12 tasks  │  │  20 tasks  │  │  7 tasks   │            │
│   │            │  │            │  │            │            │
│   └────────────┘  └────────────┘  └────────────┘            │
│                                                              │
│   最近 1 小时前 · 自动同步 · 后台运行                            │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

**关键差异（vs AI 味）：**
- 标题左对齐，不是居中
- 卡片**没有**强投影边框，米白底色 + 暖色细边框
- 数字（"12 tasks"）用衬线斜体，像旧式印刷品
- 间距宽松（24-32px），不像 admin 后台那么挤

---

## 4. Rust 后端（IPC handler 实现）

### 4.1 目录结构

```
src-tauri/src/
├── main.rs                  # Tauri 入口
├── commands/                # 文档 § 3.2 的 kron_* 命令实现
│   ├── mod.rs
│   ├── project.rs           # kron_project_*
│   ├── task.rs              # kron_task_*
│   ├── vertex.rs            # kron_vertex_*
│   ├── important.rs         # kron_important_*
│   ├── conflict.rs          # kron_conflict_*
│   ├── context.rs           # kron_context_*
│   └── open.rs              # kron_open_with_system
├── ipc_types.rs             # 命令参数/返回类型（与前端共享 schema）
├── watcher.rs               # notify crate + tokio task
├── git_graph.rs             # git rev-list --ancestry-path 包装
└── config.rs                # config.json 读写
```

### 4.2 command handler 示例

```rust
// src-tauri/src/commands/task.rs
use tauri::command;
use crate::ipc_types::{TaskDto, AddTaskOpts};

#[tauri::command]
pub async fn kron_task_list(
    project_id: String,
    vertex: String,
    state: Option<String>,
) -> Result<Vec<TaskDto>, String> {
    // 调用 kron CLI 子进程（或直接调 core 库——见 § 6 决策）
    let output = std::process::Command::new("kron")
        .args(["task", "list", &vertex])
        .arg("--json")
        .current_dir(project_root(&project_id)?)
        .output()
        .map_err(|e| e.to_string())?;

    serde_json::from_slice(&output.stdout).map_err(|e| e.to_string())
}

#[tauri::command]
pub async fn kron_task_move(
    project_id: String,
    task_id: String,
    new_state: String,
    window: tauri::Window,
) -> Result<(), String> {
    std::process::Command::new("kron")
        .args(["task", "move", &task_id, "--to", &new_state])
        .current_dir(project_root(&project_id)?)
        .status()
        .map_err(|e| e.to_string())?;

    // 推送给前端（文档 § 3.3 task_state_changed）
    window.emit("task_state_changed",
        serde_json::json!({"task_id": task_id, "new_state": new_state})
    ).ok();

    Ok(())
}
```

### 4.3 文件监听 → emit

```rust
// src-tauri/src/watcher.rs
use notify::{Watcher, RecursiveMode, Event};
use tokio::sync::mpsc;

pub fn spawn_watcher(app_handle: tauri::AppHandle) {
    let (tx, mut rx) = mpsc::channel::<notify::Result<Event>>(100);

    let mut watcher = notify::recommended_watcher(move |res| {
        tx.blocking_send(res).ok();
    }).unwrap();

    watcher.watch(Path::new("KRON"), RecursiveMode::Recursive).unwrap();

    tokio::spawn(async move {
        while let Some(res) = rx.recv().await {
            if let Ok(event) = res {
                // 路径匹配 → emit 对应 event
                if event.paths.iter().any(|p| p.ends_with("tasks.md")) {
                    app_handle.emit("task_file_changed", event.paths).ok();
                }
                if event.paths.iter().any(|p| p.starts_with("kron-internal/important")) {
                    app_handle.emit("important_file_changed", event.paths).ok();
                }
                if event.paths.iter().any(|p| p.starts_with(".git/HEAD")
                    || p.starts_with(".git/refs/heads/"))
                {
                    app_handle.emit("git_ref_changed", ()).ok();
                    // 触发 git graph 重算
                }
            }
        }
    });
}
```

---

## 5. 前端结构

```
frontend/src/
├── App.tsx                  # 路由 + 全局 store
├── main.tsx                 # Solid 入口
├── routes/
│   ├── Home.tsx             # V0 项目首页
│   ├── Project.tsx          # V1 主窗口（Tabs 容器）
│   └── tabs/
│       ├── Kanban.tsx       # V2 看板
│       ├── Important.tsx    # V3 重要文件
│       ├── Conflicts.tsx     # V4 冲突
│       ├── VertexDag.tsx    # V5 Git 图
│       └── Context.tsx      # V6 Context 文件
├── components/
│   ├── ui/                  # shadcn-solid 复制来的组件（手改过皮肤）
│   │   ├── Button.tsx
│   │   ├── Dialog.tsx
│   │   ├── Tooltip.tsx
│   │   └── ...
│   ├── kanban/
│   │   ├── KanbanBoard.tsx  # 4 列容器
│   │   ├── KanbanColumn.tsx # 单列（drop zone）
│   │   └── TaskCard.tsx     # 任务卡片
│   ├── project/
│   │   ├── ProjectCard.tsx  # V0 首页项目卡
│   │   └── Sidebar.tsx      # V1 侧边栏
│   └── common/
│       ├── EmptyState.tsx
│       └── LoadingDots.tsx  # 朴素 3 点，不旋转
├── stores/
│   ├── project.ts           # 当前项目 + 列表
│   ├── task.ts              # 当前 vertex 的任务
│   └── theme.ts             # 主题切换
├── ipc/
│   ├── invoke.ts            # invoke('kron_*') 包装
│   └── events.ts            # listen('*') 包装
├── lib/
│   ├── format.ts            # 时间、大小格式化
│   └── git_log_colorize.tsx # V5 git log --graph 着色
└── styles/
    ├── tokens.css           # § 3.2 CSS variables
    └── global.css           # reset + 基础类型
```

---

## 6. CLI ↔ GUI 集成策略（待定）

**两个方案：**

### 方案 A：GUI 调 `kron` CLI 子进程（薄 GUI）

```rust
// 在 Rust handler 里
Command::new("kron")
    .args(["task", "list", &vertex])
    .arg("--json")
    .output()
```

- **优点**：CLI 是单一真理源，GUI 永远不偏离 CLI 行为
- **缺点**：进程启动开销（每次操作 ~10-30ms）；需要 GUI 安装时附带 `kron.exe`

### 方案 B：GUI 直接调用 `kron` core 库（厚 GUI）

```rust
// 在 Rust handler 里
use kron_core::{TaskRepo, ListOpts};
let tasks = TaskRepo::new(project_root).list(&vertex, opts).await?;
```

- **优点**：零开销；单二进制部署；类型安全
- **缺点**：CLI 和 GUI 两套调用方，可能 drift；core 库需要重构成可嵌入的 lib

**倾向：方案 A（M2 期间先用）**，原因：
- M2 已经稳定了 CLI 的语义
- GUI 初期操作频率低（用户拖拽任务 ~1 秒一次），10ms 进程开销无感
- 后面如果性能真的卡住，再切换到 B

---

## 7. 决策记录

| 决策 ID | 选择 | 备选 | 拍板日期 |
|--------|------|------|---------|
| **GUI-T1** | Tauri 2 单进程 + tokio 后台 | 独立 daemon | 2026-09-08 |
| **GUI-T2** | Solid.js 1.9+ | React / Svelte / Vue | 2026-09-08 |
| **GUI-T3** | Vite 5 + TS strict | Rsbuild | 2026-09-08 |
| **GUI-T4** | shadcn-solid + 手写皮肤 | solidcn / Ant Design / 自写 | 2026-09-08 |
| **GUI-T5** | Kobalte（headless） | corvu / 自写 | 2026-09-08 |
| **GUI-T6** | TailwindCSS + CSS variables（双主题） | styled-components | 2026-09-08 |
| **GUI-T7** | HTML5 Drag and Drop | @dnd-kit | 2026-09-08 |
| **GUI-T8** | 纯文本 git log --graph + 自着色 | cytoscape / SVG | 2026-09-08 |
| **GUI-T9** | notify crate + tokio mpsc | 轮询 | 2026-09-08 |
| **GUI-T10** | 不渲染 Markdown（V6 只列文件） | react-markdown | 2026-09-08 |
| **GUI-T11** | diff2html + 自定义 theme | jsdiff 自渲染 | 2026-09-08 |
| **GUI-T12** | Tauri 原生 invoke/emit | WebSocket | 2026-09-08 |
| **GUI-T13** | tauri-plugin-shell + opener | 自写 ShellExecute | 2026-09-08 |
| **GUI-T14** | Windows 优先 | 一开始跨平台 | 2026-09-08 |
| **GUI-T15** | CSS variables + data-theme | ThemeProvider | 2026-09-08 |
| **GUI-T16** | Solid Store + Signals | Zustand | 2026-09-08 |
| **GUI-T17** | @solidjs/router | 无 | 2026-09-08 |
| **GUI-T18** | Lucide Icons（stroke 1.5） | Material Icons | 2026-09-08 |
| **GUI-T19** | 系统默认 + 衬线中文 | Web 字体 | 2026-09-08 |
| **GUI-T20** | CSS transitions + @motionone/solid 局部 | framer-motion | 2026-09-08 |
| **GUI-VIS-1** | 视觉对齐 Yuushya Townscape（暖中性、双层阴影、衬线标题） | 默认 admin 风格 | 2026-09-08 |

---

## 8. 风险与缓解

| 风险 | 缓解 |
|------|------|
| shadcn-solid 组件不更新/掉档 | 关键组件（Dialog/Tooltip/Dropdown）fork 到我们仓库，自己维护 |
| Kobalte API 变化 | API 稳定已 1 年；锁定 `^0.13.0` 不自动升级 |
| Solid.js 2.0 beta 风险 | 锁 1.9.x；2.0 正式版后再评估迁移 |
| Tauri 2 + Solid HMR 报错 | 用社区验证过的 [tauri-solid-ts-tailwind-vite](https://github.com/AR10Dev/tauri-solid-ts-tailwind-vite) 模板起步 |
| 双主题颜色 tokens 不全 | 先把 `tokens.css` 写完再写组件；颜色统一从 token 取，禁裸值 |
| 衬线中文字体未安装 | fallback 链完整（思源 → 方正 → Noto → Georgia），任何 Windows 都至少有 fallback |
| 拖拽性能（100+ 任务卡） | Solid 的 fine-grained 优势；实测若卡再加虚拟滚动（@solid-primitives/virtual） |
| IPC 类型漂移（前后端 schema 不一致） | 用 `ts-rs` 从 Rust 类型生成 TS 类型，单一来源 |

---

## 9. 后续要做的（不在本选型范围）

| 任务 | 文件 |
|------|------|
| 写 `tokens.css` 双主题颜色定义 | `frontend/src/styles/tokens.css` |
| 创建设计 token 文档（颜色/间距/字号/阴影规范） | `dev-docs/design/05c-GUI设计tokens.md` |
| V0 项目首页 + V1 主窗口骨架 | `frontend/src/routes/` |
| shadcn-solid 初始化 + 取基础组件 | `npx shadcn-solid init` |
| Tauri 工程初始化 | `npm create tauri-app@latest` |
| IPC 类型生成（Rust → TS） | `ts-rs` 配置 |

---

## 10. 参考链接

- [Tauri 2 官方文档](https://v2.tauri.app/)
- [Solid.js 官方文档](https://www.solidjs.com/)
- [shadcn-solid 仓库](https://github.com/hngngn/shadcn-solid)
- [Kobalte 无障碍原语](https://kobalte.dev/)
- [tauri-solid-ts-tailwind-vite 模板](https://github.com/AR10Dev/tauri-solid-ts-tailwind-vite)
- [Lucide Icons](https://lucide.dev/)
- [Yuushya Townscape（用户审美参考）](https://yuushya.com/townscape/)
