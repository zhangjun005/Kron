# RFC: 多项目管理 — kron register + GUI 项目列表

| 字段 | 值 |
|---|---|
| **状态** | **草案 (2026-10-13, zhangjun005 拍板启动)** |
| **作者** | AI assistant, 经 zhangjun005 委托 |
| **创建日期** | 2026-10-13 |
| **目标版本** | v1.3+ |
| **实施时机** | **最后发布时再做** (不阻塞 GUI 骨架开发) |
| **补充** | [`docs/rfc/2026-10-09-gui-ide-plan.md`](./2026-10-09-gui-ide-plan.md) §3.2 (Wails GUI 多项目概览)；[`docs/rfc/2026-10-13-gui-distribution-manual-install.md`](./2026-10-13-gui-distribution-manual-install.md) (分发) |
| **取代** | — (新议题) |

---

## 1 动机

Wails GUI §3.2 拍板了"多项目概览"能力，vsix §4 拍板了"意图树侧边栏"（IDE 单 workspace 场景，不需要多项目管理）。

**未拍板**：GUI 的项目列表**存在哪**？

[`docs/rfc/2026-10-13-gui-distribution-manual-install.md`](./2026-10-13-gui-distribution-manual-install.md) §2.1 拍板了"binary-locator = PATH only"，但没说项目列表。

**不能每次扫盘**（扫描 `.kron/intents` 太慢，用户体验差），需要持久化存储。

---

## 2 拍板

### 2.1 项目列表文件: `projects.json`

kron 和 GUI **共用同一个 JSON 文件**，纯文件 I/O，**零 daemon / 零 socket / 零 HTTP**。

**文件路径 = `Kron.exe` 所在目录**，不写用户 C 盘目录**。

```
kron-v1.3.0-windows-amd64.zip 解压到任意位置（如 C:\Apps\Kron\）
C:\Apps\Kron\
├── Kron\                        (Wails GUI raw)
│   ├── Kron.exe
│   └── ...
├── kron.exe                     (kron CLI)
├── projects.json               ← 项目列表，跟安装包走
└── README.txt
```

| 路径获取方式 | |
|---|---|
| Go | `filepath.Dir(os.Executable())` |
| Windows | `strings.Replace(filepath.Dir(os.Executable()), "\\", "\\\\", -1)` |

**可移植**：用户把整个 Kron 目录拷到 U 盘 / 移动硬盘，`projects.json` 跟着走，不写任何用户目录。

| OS | 路径 |
|---|---|
| Windows | `Kron.exe` 同目录 `projects.json` |
| Linux | `kron` 同目录 `projects.json` |
| macOS | `kron` 同目录 `projects.json` |

### 2.2 JSON 结构

```jsonc
// projects.json
{
  "current_project": "E:\\works\\proj1",    // ← 当前激活项目，可空
  "projects": [
    {
      "path": "E:\\works\\my-project",
      "added_at": "2026-10-13T12:00:00Z",
      "last_opened": "2026-10-13T15:30:00Z"
    },
    {
      "path": "D:\\另一个项目",
      "added_at": "2026-10-11T09:00:00Z",
      "last_opened": "2026-10-12T20:00:00Z"
    }
  ]
}
```

| 字段 | 说明 | 谁写 |
|---|---|---|
| `current_project` | 当前激活项目的绝对路径，可空 | GUI（用户切换项目时写）；`kron register` 成功后自动更新 |
| `projects[].path` | 项目根目录绝对路径（`filepath.Abs`） | `kron register` / `kron init` / GUI |
| `projects[].added_at` | 首次注册时间 (ISO 8601 UTC) | `kron register` / `kron init` |
| `projects[].last_opened` | 最近一次打开时间 (ISO 8601 UTC) | GUI（每次用户点开该项目时更新） |

**去重**：`path` 相同的项目**不**重复写入（`path` 唯一键）。

### 2.3 注册命令: `kron register`

**新** CLI 子命令，需要走 [`docs/process/cli-flag.md`](../../docs/process/cli-flag.md) 流程。

```
$ kron register <path>
$ kron register .          # 当前目录
$ kron register E:\works\my-project

# 自动补全: 无（简单子命令，不复杂）
```

**行为**：
1. 解析 `<path>` → `filepath.Abs` → 验证目录存在
2. 读 `projects.json`（不存在则创建 `{}`）
3. 若 `path` 已存在 → 更新 `added_at`（重新注册）→ 写回
4. 若 `path` 不存在 → 追加 → 写回
5. 更新 `current_project` = `<path>`（自动设为当前激活项目）→ 写回
6. 输出: `Registered: E:\works\my-project`（stdout，1 行）

**错误处理**：
- 目录不存在 → `fmt.Fprintln(os.Stderr, "error: directory not found")` → exit 1
- `projects.json` 损坏 → `fmt.Fprintln(os.Stderr, "error: corrupted projects.json")` → exit 1
- 无写入权限 → `fmt.Fprintln(os.Stderr, "error: cannot write projects.json")` → exit 1

### 2.4 `kron init` 后自动注册

`kron init` 执行成功后（目录已有 `.kron/`），自动调用内部 `addProject(absPath)`（`internal/store/` 新增，不走 CLI 调用）。

**不**输出多余信息（用户不需要知道"注册"这件事）。

### 2.5 GUI 读取行为

GUI 启动时读 `projects.json`：
- 文件不存在 → `[]` → 渲染"空状态"（提示用户 `kron init` 或 `kron register`）
- 解析失败 → 渲染错误提示
- 按 `last_opened` 降序排序，最新打开的排第一

GUI 更新 `last_opened`：
- 用户点击项目卡片时，写入当前时间到对应项目的 `last_opened` 字段

GUI **不**监听文件变化（不 poll，不 inotify）。只在**启动时** + **用户操作后**读写文件。

### 2.6 CLI `kron list-projects`（可选 UX）

提供 `kron list-projects` 命令查看已注册项目列表（纯 UX，不影响 GUI）。

```
$ kron list-projects
E:\works\my-project      (last opened: 2026-10-13 15:30)
D:\另一个项目             (last opened: 2026-10-12 20:00)
```

走同一 JSON 文件读取，**不**新增文件，**不**改数据结构。

### 2.7 `current_project` — 当前项目指针

**问题**：`os.Getwd()` 不一定是项目目录（用户在上级目录 / 根目录跑命令）。

**解决**：在 `projects.json` 加顶级字段 `current_project`，作为"当前激活项目"的指针。

```jsonc
// projects.json
{
  "current_project": "E:\\works\\proj1",    // ← 绝对路径，可空
  "projects": [
    { "path": "E:\\works\\proj1", "added_at": "...", "last_opened": "..." },
    { "path": "E:\\works\\proj2", "added_at": "...", "last_opened": "..." }
  ]
}
```

| 字段 | 说明 | 谁写 |
|---|---|---|
| `current_project` | 当前激活项目的绝对路径（`filepath.Abs`），可空 | GUI（用户切换项目时写）；`kron register` 注册成功后自动更新 |

**向后兼容**：`current_project` 可空。空时：
- GUI：默认选 `last_opened` 最大的那个
- MCP server：退到 `os.Getwd()`

**GUI 更新时机**：用户点击项目卡片 → 写入 `current_project` + 更新 `last_opened`（两个字段一次事务性写回）。

### 2.8 MCP server 读取优先级

`kron serve-mcp` 的 `resolveMCPRoot` 按以下优先级决定 repo root：

| 优先级 | 来源 | 说明 |
|---|---|---|
| 1 (最高) | `-CWD <path>` flag | 显式指定，覆盖一切 |
| 2 | `projects.json` 的 `current_project` | GUI 维护的当前激活项目指针 |
| 3 (最低) | `os.Getwd()` | 进程工作目录（stdio 场景默认） |

```go
// cmd/kron/serve-mcp/serve_mcp.go:resolveMCPRoot (伪代码)
func resolveMCPRoot(args []string, projectsJSON string) (string, error) {
    // P1: 显式 -CWD
    if len(args) == 2 && args[0] == "-CWD" {
        return filepath.Abs(args[1])
    }

    // P2: projects.json 的 current_project
    if projectsJSON != "" {
        if data, err := os.ReadFile(projectsJSON); err == nil {
            var cfg struct {
                CurrentProject string `json:"current_project"`
            }
            if json.Unmarshal(data, &cfg) == nil && cfg.CurrentProject != "" {
                if _, err := os.Stat(filepath.Join(cfg.CurrentProject, ".kron")); err == nil {
                    return cfg.CurrentProject, nil
                }
            }
        }
    }

    // P3: 退到 os.Getwd()
    cwd, err := os.Getwd()
    if err != nil {
        return "", fmt.Errorf("getcwd: %w", err)
    }
    return cwd, nil
}
```

**GUI spawn MCP server 时的行为**：
- GUI 始终传 `-CWD <project-root>`（P1 命中，绕开指针逻辑）——这是最可靠的
- CLI `kron` 命令不走 MCP，按 §2.3 / §2.4 行为
- 脚本 / CI 用 `kron register` 注册后，下次 `kron lint` / `kron add` 走 §P3（`os.Getwd()` 正确时）或 §P2（不在项目目录时）

---

## 3 不做的事 (明确)

| 项 | 状态 | 理由 |
|---|---|---|
| ❌ MCP 层管项目列表 | **不**做 | MCP 是 stateless stdio server，有状态化违反架构 §3.6 假设；GUI 自管更干净 |
| ❌ GUI daemon / socket / HTTP server | **不**做 | 零 daemon，零监听端口，纯文件 I/O；GUI 启动读文件，关窗口退进程 |
| ❌ 文件 sentinel + poll 轮询 | **不**做 | GUI 不轮询，只在用户操作时读写 |
| ❌ `KRON_PROJECTS_PATH` 环境变量 | **不**引 | 零配置原则；路径硬编码（各 OS 标准位置） |
| ❌ vsix 多项目概览 | **不**做 | IDE 单 workspace 场景，vsix 不需要多项目管理；只做单项目意图树（§4） |
| ❌ 文件锁 / 竞态处理 | **不**做 | 单用户本地文件，Windows/macOS 文件系统锁保护；忽略并发写（极少发生） |
| ❌ `kron register` 新增 Go dep | **不**做 | 用 `encoding/json` + `os` 纯 stdlib |

---

## 4 实施清单 (最后发布时再做)

> ⚠️ **本节是"最后发布时再做"事项, 不阻塞 GUI 骨架开发.**

| # | 改动 | 仓库位置 | 工作量 |
|---|---|---|---|
| 1 | `internal/store/` 新增 `AddProject(path)` / `ListProjects()` / `RemoveProject(path)` / `UpdateLastOpened(path)` 函数 | `internal/store/project_registry.go` (新建) | 1 天 |
| 2 | `cmd/kron/cli/` 新增 `register` 子命令 + `list-projects` 子命令 | `cmd/kron/cli/register.go` (新建) | 0.5 天 |
| 3 | `kron init` 自动调 `AddProject()` + 设置 `current_project` | `cmd/kron/cli/init.go` (修改) | 0.5 天 |
| 4 | GUI 读/写 `projects.json` (Wails Go 侧 `os.ReadFile` / `os.WriteFile`) | `frontend/wails/` | 0.5 天 |
| 5 | `serve-mcp` 的 `resolveMCPRoot` 加 3 级 fallback（-CWD → projects.json → os.Getwd()） | `cmd/kron/serve-mcp/serve_mcp.go` (修改) | 0.5 天 |
| 6 | 修订 `docs/process/cli-flag.md` — `kron register` 是新 CLI 子命令（不是 flag 变更） | `docs/process/` | 0.5 天 |
| 7 | 修订 `docs/rfc/2026-10-09-gui-ide-plan.md` §3.2 — 加"项目列表存储"引用 | `docs/rfc/` | 0.5 天 |

**总改动 ~4 天. Go 主仓改动约 2.5 天 (store + cli + serve-mcp), GUI 改动 0.5 天.**

---

## 5 红线确认

| 红线 | 拍板 |
|---|---|
| 加新 CLI 子命令 (init/add/lint/serve-mcp/serve-lsp 之外) | ✅ `register` / `list-projects` — 走 cli-flag.md 流程 |
| 加新 top-level dep (`go.mod` `require` 块) | ❌ 不加 (std lib only) |
| MCP 层有状态化 | ❌ 不做 |
| GUI 起 daemon / socket / HTTP | ❌ 不做 |
| `internal/` import `cmd/kron/` | ❌ 不让 |
| 改 frontmatter schema | ❌ 不改 |
| 改 intents/assumptions 数据架构 | ❌ 不改 |
| 改 5 铁律 + 8 import 边界 | ❌ 不改 |

---

## 6 跟现有 RFC 的关系

| 现有 RFC | 关系 |
|---|---|
| [`2026-10-09-gui-ide-plan.md`](./2026-10-09-gui-ide-plan.md) §3.2 (Wails GUI 多项目概览) | **补充** — "项目列表存在哪" |
| [`2026-10-13-gui-distribution-manual-install.md`](./2026-10-13-gui-distribution-manual-install.md) §2.1 (binary-locator = PATH only) | **不**改 — 独立议题 |
| [`2026-10-10-frontend-ui-stack.md`](./2026-10-10-frontend-ui-stack.md) (UI 技术栈) | **不**改 — UI 渲染独立于数据存储 |

---

## 7 待办 (v1.3+ release pipeline 阶段)

> ⚠️ **本节是"最后发布时再做"事项.**
>
> 实施清单 §4 全部 6 项.
