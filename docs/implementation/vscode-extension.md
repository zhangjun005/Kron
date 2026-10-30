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
直接删除 Markdown 文件替代这些操作。node 意图的删除是**整棵子树**进 trash：卡片上的 Delete
按钮对 node 也启用，确认框写明"整棵子树 N 个意图一起进 trash"，调用时自动带
`confirm_subtree: true`；删除/恢复后提示本次涉及的意图数量（`deleted_slugs` / `restored_slugs`）。
旧 trash 里的 `<slug>.md` 按 leaf 恢复。细节见
[`2026-10-29-trash-subtree.md`](2026-10-29-trash-subtree.md)。

### Assumption 表单与注册表面板

assumption 是可被多个 intent 引用的独立文件。界面上有两处入口，职责不同：

1. **Intent 卡片内的 Assumptions 区**：负责"这个 intent 用了哪些假设"。
   - 关联已有条目：先用 `kron_assume_list` 选已有条目，再通过 `kron_update` 把
     `{id, severity, rationale}` 附加到该 intent；用户必须填写本 intent 自己的 severity 与 rationale。
   - 创建并关联：显式调用 `kron_assume_create`（填写真实文本与默认 severity），成功后 `kron_update`
     关联。不要依赖 `kron_add`/`kron_update` 的自动 stub 行为。
   - 解除关联只走 `kron_update`（保留共享文件）。
2. **主界面的 Assumptions 面板**（Archived 左侧按钮）：负责"项目里的假设注册表"。
   - 列表来自 `kron_assume_list`，显示 id / 默认 severity / 被引用次数；
   - 详情可编辑共享文本、默认 severity、status、expires_at、reviewers，保存走 `kron_assume_update`；
   - 新建走 `kron_assume_create`；删除走 `kron_assume_delete`（软删除）。
   - "Used by" 来自 `kron_assume_check` 的 per-intent 记录（带 severity / rationale），点击可跳到对应 intent 卡片。
   - 删除前展示仍引用它的 intent 数；因为当前 MCP 没有 assumption 恢复工具，界面不提供"恢复"按钮，
     并提示需要手动解除仍存在的引用。

### 代码锚点识别（已实现，语法以 `internal/parser` 为准）

锚点语法不是扩展私有的：`internal/parser/anchors.go` 是唯一真理源，扩展在
`frontend/vscode/src/anchors.ts` 里镜像它。当前规则：

- 任意注释前缀（`//`、`#`、`--`、`%`、`*` 等）都允许；行内任意位置出现整词
  `@kron:intent` 即可，marker 后必须是空白，取其后第一个 token 作为 slug。
- slug 后允许尾随文字（例如 `// @kron:intent auth/jwt -- 说明`），与 `kron lint` 行为一致。
- Markdown 使用同一语法，但跳过 fenced code block（``` 或 ~~~），避免文档示例被当成锚点。
  为了不刷屏，Markdown 锚点只做下划线 + Ctrl+click 跳转，不自动提示创建 intent。
- `.kron/` 下的文件（intent 自身、`.kron/.trash`）不参与扫描，与 parser 的 skip list 一致。

- **扫描范围由项目 `.kron/anchor-scan.json` 决定**（2026-10-30）：`include`/`exclude` glob +
  `languages`（语言 → `extensions`）；文件缺失时扫描所有非二进制文件（旧行为）。`comment`
  只用于 IDE **插入**锚点（检测只看 `@kron:intent` 标记，不看注释前缀）。`kron init` 会写入
  一份主流语言默认配置。VSIX 直接读取该文件派生 LSP 语言列表/下划线门控/插入注释；删除
  `kron.lspLanguages` 设置。

编辑器行为：

- 匹配到的 slug 显示下划线；`textDocument/hover` 与 definition 由 `serve-lsp` 提供。
- 非 Markdown 文件里，锚点所在行停止编辑 900ms 后，如果该 slug 不在树中，会询问是否创建
  （`kron_add`）；同一位置只询问一次。

### 代码锚点右键菜单（已实现）

光标落在锚点行或符号行上一行时，`editor/context` 菜单按锚点状态提供操作：

| 当前状态 | 菜单项 | 行为 |
|---|---|---|
| 有效锚点 | Open Intent Details / Edit Intent / Open Intent Markdown / Show Intent Impact | 打开工作区面板并选中该 intent（Edit 直接进入编辑态）；打开对应 `.md` 原文；用 `kron_impact` 展示反向引用摘要 |
| 悬空锚点 | Create Intent for Anchor / Change Anchor Target / Remove Anchor | 预填 slug 创建（若改名，锚点同步改为新 slug）；QuickPick 现有 slug 后用 `WorkspaceEdit` 改写锚点；删除整行锚点 |
| 无锚点 | Create Intent and Attach Anchor / Attach Existing Intent Anchor | 先创建（或选已有），成功后才在符号行上方插入锚点 |
| `.kron/` 内文档、非文件文档 | 无菜单 | 不参与锚点菜单 |

- 顺序固定为：`kron_add` 成功 → 刷新树 → `WorkspaceEdit` 插入锚点。创建失败时文件不被修改；
  插入失败提示"已创建但锚点未插入"，并给出应插入的 slug。
- 注释前缀按 `languageId` 选择（`//` / `#` / `--` / `%` / `<!-- -->`）；未知语言弹出可编辑预览，
  不猜测后写入。插入位置是光标所在符号行的上一行，沿用该行缩进。
- 有效/悬空判定用一次 `kron_get`（5 秒缓存），树刷新或写操作后缓存失效；判断期间菜单隐藏。
- 菜单通过 `kronAnchorState` 上下文键控制（`none` / `checking` / `valid` / `dangling` / `unsupported`），
  扩展激活时和光标移动后更新。

## 侧边栏与设置

意图树有两个可独立开关的落点：

| 设置 | 默认 | 落点 |
|---|---|---|
| `kron.sidebar.independent` | `true` | 活动栏独立 Kron 图标下的侧边栏 |
| `kron.sidebar.explorer` | `false` | Explorer 侧边栏里的 "Kron Intents" 视图 |

两者可以都开、都关，或只开一个；视图显隐由 `config.kron.sidebar.*` 的 `when` 条件控制，改设置即时生效。
独立侧边栏（对齐 GUI 左栏的信息密度）包含三个只读视图：

- **Intents**：意图树，点击叶子打开 Webview 卡片；标题显示 `N intents · M folders`；
- **Assumptions**：`kron_assume_list` 的注册表条目（id · 默认 severity · 过期日期），点击打开对应的
  `.kron/assumptions/<id>.md`；
- **Superseded**：状态为 `superseded` 的意图（GUI 的 "Archived" 等价物——Kron 目前没有 trash 列举工具），
  点击打开对应意图卡片。

所有 Kron 视图的标题栏都有 Refresh；两个意图树视图额外有 "Open Kron workspace"。状态颜色：
active 绿、draft 黄、superseded 灰；tooltip 显示 slug、leaf/folder、status、标题。没有 `.kron/` 项目时
各视图用 `viewsWelcome` 显示提示和入口。

没有 `.kron/` 的工作区不会在启动时激活扩展（`workspaceContains` 不匹配），只有用户点开 Kron 视图
（`onView:*`）才加载扩展代码；此时不 spawn MCP/LSP 子进程、不注册文件监听，开销仅为一次小 JS 加载。

## 刷新与项目范围

**单项目策略**：一个窗口只服务第一个包含 `.kron/` 目录的 workspace folder；额外的 folder
不建立连接、也不出现在意图树里（多项目概览属于 GUI 职责，见 architecture §〇·五·1）。
`kron.serverPath` 在每次重建连接时重新读取，修改后执行
"Kron: Restart Connections" 即生效，无需重载窗口；`kron.webviewFontSize` 变更会实时推送到
已打开的 Webview。

刷新入口：显式 Refresh、MCP 写成功、workspace folder 变化、配置变化（serverPath、`.kron/anchor-scan.json`、`.kron/templates/intent-body.md` 保存事件），
以及 Webview 重新打开。**尚未实现**：编辑器保存（`onDidSaveTextDocument`）与外部文件变更
（FileSystemWatcher）触发的刷新——磁盘被 git 或其他工具改动后需要手动 Refresh。

## 验收

- 锚点按 parser 语义识别：尾随文字、任意注释前缀、Markdown 裸行都能下划线；
  Markdown 代码块里的示例不产生下划线，`.kron/` 内文档不参与扫描。
- 右键菜单按状态出现：有效锚点有 4 项、悬空锚点有 3 项、无锚点有 2 项；
  创建失败不改文件，插入失败有明确提示；`.kron/` 内文档不出现菜单。
- 悬空锚点不会被误当成协议错误；Ctrl+click 跳到对应 intent 文件。
- 表单保存后树、详情和原文一致；共享 assumption 的编辑和删除对用户可见其影响范围。
- 修改 `kron.serverPath` 或 `kron.lspLanguages` 后执行 Restart Connections 生效；
  `kron.webviewFontSize` 变更立即作用于已打开的 Webview。
- deactivate 会停止该 workspace 的两个子进程。
