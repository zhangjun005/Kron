# VSCode / Cursor 扩展开发指南

本文件定义 `frontend/vscode/` 的单个 `.vsix` 的职责、进程模型和人类编辑流程。它适用于
VSCode 与兼容 VSCode 扩展 API 的 Cursor。架构约束见
[`architecture.md`](../abstractDesign/architecture.md)，数据字段见
[`intent-structure.md`](../abstractDesign/intent-structure.md)。

## 职责边界

扩展是人类在代码旁使用 Kron 的完整入口。它同时持有两个独立子进程：

```text
VSCode extension
 ├─ MCP client  ─ stdio ─ kron serve-mcp -CWD <workspace-root>
 │   └─ 树、详情、表单、写入、关系、assumption
 └─ LanguageClient ─ stdio ─ kron serve-lsp -CWD <workspace-root>
     └─ hover、definition、completion、diagnostics
```

M1 已实现 LSP 子进程启动：扩展从 `kron.serverPath` 读取可执行文件路径（默认 `kron`），以
`serve-lsp -CWD <workspace-root>` 启动 stdio server，并通过 `vscode-languageclient` 完成
initialize。Windows 扩展宿主可将设置指向 `kron.exe` 的绝对路径。没有 workspace folder 时显示
提示且不启动进程。M4 已通过官方 `@modelcontextprotocol/sdk` 单独启动
`kron serve-mcp -CWD <workspace-root>`，用 `kron_tree` 提供只读 Explorer 树，并提供刷新命令。
M4 为启动时存在的每个 workspace folder 分别建立 MCP/LSP 子进程，并将多根意图树按 workspace 分组；运行期间动态增删 workspace folder 尚未实现。树节点详情和写操作 UI 仍未实现。
LSP 语言范围由 `kron.lspLanguages` 配置，默认覆盖主要源码语言和 Markdown。

两条连接都由扩展启动、复用和停止，二者不能互相转发请求。扩展也不直接 import Go 的
`internal/`，不手写 MCP 或 LSP 的 JSON-RPC。

| 能力 | 归属 | 通道 |
|---|---|---|
| 意图树、详情、关系、impact、assumption 页面 | 扩展原生 UI / Webview | MCP |
| 新建、修改、删除、恢复 intent 或 assumption | 扩展表单和命令 | MCP |
| 锚点 hover、definition、补全、诊断 | 编辑器语义集成 | LSP |
| Markdown 原文的自由编辑 | 普通 VSCode 文本编辑器 | 文件系统；随后由 MCP 刷新读取 |

TreeView、状态栏、编辑器标题按钮和右键菜单用 VSCode 原生 API；详情、关系图及结构化编辑
表单放在 Webview。不要把 TreeView 重做成 Webview，也不要把 hover 或诊断塞进 Webview。

## 人类编辑闭环

Kron 的数据真相是仓库中的 Markdown。因此界面提供两条等价路径：

1. **结构化快速编辑。** 从树、命令面板或右键打开表单。表单读取 MCP 数据，提交时调用对应
   的 MCP 写工具；写成功后刷新树、详情和当前表单。
2. **原文编辑。** “打开 Markdown”始终可用，使用 `vscode.openTextDocument` 打开实际的
   `.kron/intents/<slug>.md` 或 `.kron/assumptions/<id>.md`。保存后刷新 MCP 视图。

表单只编辑它明确支持的字段。复杂正文、Markdown 链接和未来字段必须保留给原文编辑，不能
用表单的局部数据重写未知内容。

### Intent 表单

“New Intent”可从 Activity Bar、intent 树根节点和编辑器右键进入。创建表单至少收集 slug，
并允许填写 symbol、why、kind 与关联 assumptions；提交调用 `kron_add`。编辑表单加载
`kron_get` 后，以 PATCH 方式调用 `kron_update`，仅发送用户实际改过的字段。可编辑字段包括
symbol、status、reviewers、正文、references、depends_on 和 assumptions。

删除和恢复必须分别调用 `kron_delete` 与 `kron_restore`，由界面显示 MCP 的影响信息；不能以
直接删除 Markdown 文件替代这些操作。

### Assumption 表单

assumption 是可被多个 intent 引用的独立文件。界面应先用 `kron_assume_list` 选择已有条目；
创建时显式调用 `kron_assume_create`，填写 id、文本和默认 severity，再把它通过 `kron_update`
附加到 intent，并要求用户填写该 intent 自己的 severity 与 rationale。

不要为了“快速关联”而依赖 `kron_add`/`kron_update` 的自动创建 stub 行为：人类界面应让用户
在创建时填写 assumption 的真实文本。编辑共享 assumption 前，应显示其被哪些 intent 使用；
删除走 `kron_assume_delete`。当前 MCP 未提供 assumption 恢复工具，因此界面不得展示一个
无法完成恢复的按钮；恢复能力落地后再接入。

### 代码锚点右键菜单

锚点的唯一语法是 `// @kron:intent <slug>`，应紧贴被描述的符号上一行。扩展在光标所在行或
选区附近识别该语法，并按状态提供操作：

| 当前状态 | 菜单项 | 行为 |
|---|---|---|
| 有效锚点 | Open Intent、Edit Intent、Open Markdown、Show Impact | 读取或编辑已存在 slug；不改变锚点 |
| 悬空锚点 | Create Intent `<slug>`、Change Intent Target、Remove Anchor | 创建时预填 slug；修改目标用 slug 选择器；删除只做文本编辑 |
| 无锚点 | Create Intent and Attach Anchor、Attach Existing Intent | 前者先创建 intent，后者只选已有 slug；成功后才插入锚点 |

“Create Intent and Attach Anchor”绝不在点击右键时立即写文件。正确顺序是：打开预填表单 →
用户确认 → `kron_add` 成功 → 用 `WorkspaceEdit` 插入锚点。创建失败时不插入锚点；插入失败时
显示可重试状态，并保留已创建 intent 的链接。注释前缀按当前文档语言选择；无法确定时显示
可编辑的预览，不猜测后写入。

对于已有有效锚点，右键不再提供“再创建一个 intent”。这使一个代码位置始终清楚地指向一个
已知 intent；需要新决策时用户可先编辑目标或从树中创建。

## 刷新和多根工作区

每个 workspace folder 独立解析 Kron 根目录，并启动自己的 MCP/LSP 子进程。多根意图树的 folder
节点使用 workspace URI 作为稳定标识，禁止把 A 项目的 slug 发给 B 项目的进程。workspace folder
在扩展运行中动态增删时，当前版本尚未重新建立对应客户端。
文件保存、MCP 写成功、显式 Refresh 和切回 Webview 时都刷新相关只读数据；不要维护另一份 intent
状态缓存。

## 验收

- 现有锚点可从右键打开对应 intent，悬空锚点不会被误当成协议错误。
- 无锚点位置创建失败时代码不被修改；成功后只有一个正确的锚点插入。
- 表单保存后树、详情和原文一致；原文外部修改后刷新能显示新内容。
- 共享 assumption 的编辑和删除对用户可见其影响范围。
- deactivate 会停止该 workspace 的两个子进程。
