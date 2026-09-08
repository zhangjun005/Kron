# Day 6 — GUI 骨架与设计 Token

> 日期：2026-09-08
> 范围：Phase 1 GUI 骨架 + 完整双主题 token 规范

## 今日产出

### 1. 设计文档

- **`dev-docs/design/05b-GUI技术选型.md`** — 20 项技术选型（Tauri 2 + Solid + shadcn-solid + Tailwind + CSS variables）
- **`dev-docs/design/05c-GUI设计tokens.md`** — 完整双主题 token 规范（颜色 / 间距 / 字号 / 阴影 / 圆角 / 动画 / 尺寸）

### 2. 代码骨架

```
src-tauri/                      Tauri 2 Rust 后端（cargo check ✅）
├── Cargo.toml                  tauri 2.0, notify 6.1, kron core lib
├── tauri.conf.json             window 1280x800, light 默认
├── icons/icon.png              占位 32x32（K 字母陶土橙底）
├── icons/icon.ico              占位 32x32 ICO（Windows resource 用）
└── src/
    ├── main.rs                 #![windows_subsystem] → run()
    ├── lib.rs                  Tauri Builder + plugin setup
    ├── ipc_types.rs            ProjectMeta, Greeting
    └── commands/
        ├── mod.rs              模块入口 + watcher stub
        ├── project.rs          kron_project_list, kron_greet（烟测）
        └── watcher.rs          notify watcher stub

frontend/                        Solid + Vite 前端
├── package.json                solid-js 1.9, @solidjs/router, Kobalte, lucide-solid
├── vite.config.ts              Tauri-friendly（port 5173, ignore src-tauri）
├── tsconfig.json               strict + noUncheckedIndexedAccess
├── tailwind.config.js          所有 CSS vars 映射成 Tailwind colors
├── postcss.config.js
├── index.html
└── src/
    ├── main.tsx                mount + initTheme
    ├── App.tsx                 Router
    ├── styles/
    │   ├── tokens.css          双主题（light + dark），与 05c 一一对应
    │   └── global.css          reset + Tailwind base
    ├── stores/theme.ts         light/dark 切换（localStorage + OS）
    ├── ipc/invoke.ts           type-safe invoke 包装
    ├── types/ipc.ts            hand-mirrored from Rust
    └── routes/
        ├── Home.tsx            V0 首页 + IPC 烟测 UI
        ├── Project.tsx         V1 placeholder
        └── Tabs.tsx            V1 tabs placeholder
```

### 3. 验证

- ✅ `cargo check` 通过（0 warnings on `kron-gui`）
- ✅ Rust → Tauri → Solid IPC 烟测命令 `kron_greet` 实现

## 关键决策

| 决策 | 选择 | 理由 |
|------|------|------|
| 进程模型 | Tauri 2 单进程 + tokio 后台 | 文档原 daemon 已删，并入主进程最简 |
| 前端框架 | Solid.js 1.9 | 看板频繁更新，fine-grained 优于 VDOM；bundle ~15KB |
| UI 组件 | shadcn-solid（不用 solidcn） | solidcn 内置 MCP AI agent 自动装组件 = AI 味 |
| 样式 | Tailwind + CSS variables | 颜色从 token 取，禁裸值 |
| 拖拽 | HTML5 DnD | 看板 4 列，简单够用 |
| Git 图 | 纯文本 `git log --graph` + 自着色 | 与 05-GUI § 0.5 完全同构 |
| 类型同步 | 暂时手写 TS mirror | 只有 2 个类型；>10 个再上 ts-rs |

## 视觉语言（对齐 Yuushya Townscape）

- **颜色**：暖中性（陶土橙 `#c47854` 主色 + 青葱绿 `#6b8e5a` 强调）
- **阴影**：双层（near + far），不用 `shadow-lg` 塑料感
- **圆角**：6-8px（小），不像 iOS 大圆角
- **字体**：衬线中文（思源宋体 → 方正 → Noto）+ 系统无衬线正文
- **网格**：8px 网格，section 间距 24-32px
- **图标**：Lucide Icons，stroke 1.5

**反 AI 味清单（05c § 15）：**
- ❌ `bg-blue-500` / `bg-indigo-600`
- ❌ `shadow-lg` 单层塑料阴影
- ❌ `rounded-2xl` 大圆角
- ❌ 渐变 / 彩虹色 / 玻璃拟态
- ❌ `hover:scale-110` 活泼动效

## 下一步（Phase 2）

按 `05b` § 9 backlog：

1. V0 项目列表 + `kron project add`（弹原生目录选择 + 写 registry）
2. V1 主窗口 + 侧边栏 + Tab 切换
3. V2 看板骨架（4 列拖拽）
4. notify watcher 实际实现（替换 stub）
5. ts-rs 自动生成 TS 类型

预计 2-3 天工作量。
