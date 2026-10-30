# 快速上手 Kron

> 面向第一次使用 Kron 的人：从安装到第一个 intent、代码锚点、VS Code 扩展和 AI Agent 接入。
> 预计 10–15 分钟。

---

## 1. 安装

### 方式 A：Windows 便携包（GUI + CLI）

从 [v0.3.0 Release](https://github.com/zhangjun005/Kron/releases/tag/v0.3.0) 下载：

```
Kron-v0.3.0-windows-amd64.zip
```

解压后目录里应包含：

```
Kron-GUI.exe      Wails 桌面 GUI（只读概览）
kron.exe          CLI + MCP + LSP（一个二进制）
kron-vscode-0.3.0.vsix
README.txt
LICENSE
```

- 双击 `Kron-GUI.exe` 即可启动 GUI（它会在同目录找 `kron.exe` 和 `projects.json`）；
- CLI 可以直接用 `.\kron.exe <command>`，或把所在目录加入 `PATH`；
- GUI 需要 Windows 10/11 + Microsoft Edge WebView2 Runtime（Win 11 通常自带）。

### 方式 B：VS Code 扩展

```powershell
code --install-extension kron-vscode-0.3.0.vsix
```

然后在 VS Code 设置里把 `kron.serverPath` 指到 `kron.exe` 的绝对路径：

```json
{
  "kron.serverPath": "C:\\Tools\\Kron\\kron.exe"
}
```

VS Code 打开项目根目录（包含 `.kron/` 的那一层）后，扩展会自动启动 MCP 和 LSP。

---

## 2. 初始化第一个项目

在项目根目录执行：

```powershell
cd C:\work\my-project
C:\Tools\Kron\kron.exe init -CWD .
```

`init` 是幂等的，会创建/补齐：

```
.kron/
├── config.toml
├── anchor-scan.json          # 锚点扫描配置（可改）
├── templates/
│   └── intent-body.md        # 新 intent 正文模板（注入 AI 提示词）
└── intents/
```

并把项目写进 `kron.exe` 同目录的 `projects.json`（GUI / MCP 会用到）。

> 已有 `.kron/` 的项目直接执行 `kron.exe register -CWD .` 即可重新登记。

---

## 3. 创建第一个 intent

```powershell
C:\Tools\Kron\kron.exe add auth/login -CWD .
```

会生成 `.kron/intents/auth/login.md`。正文骨架来自 `.kron/templates/intent-body.md`：

```markdown
# login

> 一句话摘要

## Why

## How
```

可以直接编辑，也可以在 VS Code 扩展的 Webview 画布里编辑。建议至少写清楚：

- 这个决策解决了什么约束；
- 放弃了哪些替代方案；
- 有哪些边界假设（写在 frontmatter 的 `assumptions`，共享文本放 `.kron/assumptions/<id>.md`）。

---

## 4. 绑定代码锚点

在代码里加一行锚点：

```go
// @kron:intent auth/login
func Login(user, password string) (token string, error) { ... }
```

规则：**任意注释前缀都行**，只要同一行出现 `@kron:intent <slug>`：

- Go/TS/JS/Rust/C：`// @kron:intent auth/login`
- Python/Shell/YAML：`# @kron:intent auth/login`
- JSX：`{/* @kron:intent auth/login */}`
- Vue/Svelte/Markdown：`<!-- @kron:intent auth/login -->`

扫描范围由 `.kron/anchor-scan.json` 控制：语言 → 扩展名、include/exclude、插入用的注释语法。默认已包含主流语言，按需修改即可（删除文件 = 退回全量扫描）。

检查：

```powershell
C:\Tools\Kron\kron.exe lint -CWD .
```

`lint` 会检查锚点是否指向不存在的 intent、frontmatter 是否合法、假设是否需要复核等；退出码非 0 适合作为 CI gate。

---

## 5. VS Code 里怎么用

打开项目根目录后：

- **意图树侧边栏**：查看 intents / assumptions / superseded；
- **Webview 画布**：新建、编辑、删除、建立 references / depends_on；
- **源码锚点**：
  - hover 显示意图摘要；
  - `Ctrl+点击` 跳到对应 intent；
  - 右键可以创建 / 改绑锚点；
- **intent → 代码**：在意图树节点上右键 `Go to Code Anchor(s)…`，或打开意图卡片点 `Code`，多个锚点会弹出 QuickPick（repo 相对路径:行号）；
- 设置 `kron.anchors.includeMarkdown` 可以决定是否把 Markdown 锚点也放进跳转列表。

---

## 6. AI Agent 接入（MCP）

在 Claude Desktop / Cursor 等 MCP 客户端中配置：

```json
{
  "mcpServers": {
    "kron": {
      "command": "C:\\Tools\\Kron\\kron.exe",
      "args": ["serve-mcp", "-CWD", "C:\\work\\my-project"]
    }
  }
}
```

没有 `-CWD` 时，`serve-mcp` 会先读 `kron.exe` 旁 `projects.json` 的 `current_project`，再退回当前目录。因此也可以：

```powershell
C:\Tools\Kron\kron.exe register -CWD C:\work\my-project
```

然后让 MCP 客户端只传 `["serve-mcp"]`。

AI 侧推荐闭环：

1. 先 `kron_list` / `kron_tree` 看现有意图；
2. 实现前读相关 intent（`kron_get`）和假设（`kron_assume_check`）；
3. 写意图用 `kron_add` / `kron_update`，正文遵循项目模板；
4. 改完跑 `kron_lint`，必要时用 `kron_stale` 检查过期假设。

可用的 MCP prompts：`kron-implement`、`kron-drift-check`、`kron-review`。

> `.kron/templates/intent-body.md` 会注入 initialize instructions、`kron_add` / `kron_update` 的描述和 `kron-implement` prompt。修改模板后，重启 MCP 子进程（VS Code Reload Window / 重启 AI 客户端）才会刷新 tool description。

---

## 7. CI 接入

最小 CI 只需要一行：

```yaml
- name: Kron lint
  run: C:\Tools\Kron\kron.exe lint -CWD .
  shell: pwsh
```

`kron lint` 不依赖外部服务、不联网，失败时会输出 file:line + hint。

---

## 8. GUI 概览

`Kron-GUI.exe` 是只读的项目总览：

- 多项目卡片画廊 / 封面 / 主题；
- 意图树预览、状态条；
- Settings 里可以设置当前项目、封面等；
- 写操作请用 VS Code 扩展或 AI（GUI 不直接改 `.kron/`）。

---

## 9. 常见问题

**VS Code 没有 hover / 跳转？**
- 确认打开的是包含 `.kron/` 的项目根目录；
- 确认 `kron.serverPath` 指向有效的 `kron.exe`；
- 检查 `.kron/anchor-scan.json` 是否把当前语言排除掉了；
- 执行 `Kron: Restart Connections` 或 Reload Window。

**MCP 客户端看不到工具？**
- 检查 `command` 路径和 `-CWD`；
- 确认 `kron.exe` 可执行（`kron.exe help` 有输出）；
- 重启 MCP 客户端；工具应为 18 个。

**锚点没有被扫描到？**
- 确认 `.kron/anchor-scan.json` 包含对应扩展名；
- 确认不在 `.git/`、`.kron/`、`node_modules/`、`vendor/` 等跳过目录下；
- 用 `kron lint` 看是否报 `anchor-dangling` 或直接漏扫。

**想自定义正文结构？**
- 编辑 `.kron/templates/intent-body.md`，例如改成 `## Why` / `## How`；
- 重启 MCP 子进程让 AI 看到新模板。

---

## 10. 接下来读什么

| 你是谁 | 读什么 |
|---|---|
| 想理解 Kron 的设计 | [`docs/how-it-works.md`](./how-it-works.md) |
| 想改 Kron 代码 | [`AGENTS.md`](../AGENTS.md) → [`docs/abstractDesign/architecture.md`](./abstractDesign/architecture.md) |
| 想接 AI | [`docs/mcp/kron-implementation.md`](./mcp/kron-implementation.md) |
| 想看当前发布 | [GitHub Releases](https://github.com/zhangjun005/Kron/releases) |
