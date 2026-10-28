# VSCode 扩展验收问题清单

本文记录对 `frontend/vscode/` 当前实现的代码级排查结果。它是问题清单，不代表本轮已经修复。

## P0/P1：Intent 编辑体验

### 1. Edit Intent 弹窗缺少 Markdown 正文渲染

当前编辑窗口的正文区域是普通 `textarea`，详情面板使用 `pre` 展示原文。标题、列表、链接、代码块、引用和分节结构不会按 Markdown 渲染，用户无法在编辑前后看到接近最终文档的阅读效果。

建议：将编辑窗口拆成“编辑 / 预览”两栏或可切换 Tabs；预览必须使用本地打包的 Markdown renderer，禁止远程资源；渲染前保留原始文本，不改变保存语义。

### 2. Edit Intent 弹窗的层级和密度不适合长正文

此前弹窗最大宽度约 `760px`，所有字段和高级字段共用一个滚动表单，正文较长时标题、状态、关系和保存按钮都被挤在同一滚动上下文中。当前已改为画布底部展开的宽屏面板，并支持拖拽上下移动；删除确认仍复用同一面板状态，后续可继续拆分确认视图。

建议：采用宽屏编辑器布局；固定顶部标题/状态和底部操作栏；正文区域独立滚动；删除确认单独成为确认状态，不与表单并列。

### 3. 正文编辑和摘要字段的语义容易重复

`Summary` 与 `Why / body` 同时出现。实时数据路径下 summary 可能由正文首段派生，但编辑器又把两者展示成两个输入区域，存在保存后互相覆盖或用户不知道哪一个是最终真相的风险。

建议：明确 Markdown 正文为真相；摘要只作为派生预览，或明确标记为独立 frontmatter 字段并提供同步规则。

### 4. 当前 Webview 仍保留明显的 Preview/Mock 分支

当 Webview 脱离 VSCode 运行时，表单显示“Mock preview”，写入只保存在页面内存。该模式适合开发预览，但如果被用户误认为真实编辑器，会造成“保存成功但文件未改变”的认知风险。

建议：预览模式使用明显的 Demo banner；正式 VSCode 路径禁用 mock 文案，并在 MCP 不可用时显示明确的只读状态。

## P1：VSCode UI / 可用性

### 5. 缺少字体大小设置（本轮已处理）

当前已增加 `kron.webviewFontSize`（10–24，默认 14），并映射为 Webview 根元素 CSS 变量。后续仍需在不同字体和高 DPI 设置下人工验收。

### 6. Webview 主题没有跟随 VSCode 主题变量（本轮已处理基础同步）

本轮已将正文、背景、链接、引用和代码块等基础颜色接入 VSCode `--vscode-*` 变量；完整 light/dark/high-contrast 仍需人工验收。

建议：优先使用 `--vscode-editor-*`、`--vscode-sideBar-*`、`--vscode-button-*` 等宿主变量，并补充 light/dark/high-contrast 验收。

### 7. Webview 与原生 VSCode UI 的操作入口不完全一致

当前详情、编辑、删除、归档等操作主要集中在 Webview 浮层；Explorer TreeView、编辑器右键和 Webview 之间的选中状态、焦点恢复和错误反馈需要人工验收，尤其是打开多个 workspace folder 时。

### 8. 多根工作区动态变更尚未支持（本轮已处理）

本轮已监听 `onDidChangeWorkspaceFolders`，新增或移除 workspace folder 时会停止旧连接、重建 MCP/LSP 连接并刷新 TreeView。仍需 Extension Host 多根工作区人工验收。

### 9. 协议能力覆盖不完整（本轮补齐 lint 契约）

本轮已把 `kron_lint` 加入客户端工具名、输入/输出 zod 契约和 Webview 类型守卫；关系、资源等更大范围能力仍需按功能设计逐项接入。

### 10. 字段编辑存在类型安全债务

Webview 中仍有多处 `as Intent[...]` 类型断言。它们绕过了 `ts-style` 要求的运行时校验，后续应改为受限 union guard 或 zod schema。

## P2：发布与生命周期

### 11. VSIX 不携带 `kron.exe`

安装 VSIX 后仍需 PATH 或 `kron.serverPath` 配置。需要在首次启动时提供可操作的路径检查、版本显示和错误引导。

### 12. 手工 Extension Host 验收仍是必要门槛

编译和 VSIX 打包通过不等于真实 UI 通过。必须人工验证：编辑/预览、Markdown 长文、字体大小、主题、Ctrl+click/F12、多根工作区、MCP/LSP 子进程关闭和异常恢复。

## 建议优先级

1. Markdown 编辑/预览双栏和长正文布局；
2. VSCode 字体大小设置；
3. VSCode 主题变量同步；
4. 真实 Extension Host 验收；
5. 多根工作区动态生命周期；
6. MCP 工具契约补齐与类型断言清理。

## 讨论中的 UI 重构项

以下内容先作为设计讨论记录，不视为本轮已经拍板或实现。

### A. 新建 / 编辑 Intent 窗口重构

当前首页进入 Webview 后，节点详情已改为与 Wails GUI 类似的节点锚定卡片：单击节点后在节点下方展开，卡片可以相对节点拖拽；卡片内提供 Edit 入口，编辑面板沿同一锚点展开。

- 保留当前意图上下文；
- 支持拖拽调整卡片相对节点的位置；
- 顶部固定 slug、状态、保存/取消；
- 正文 Markdown 预览与编辑分栏或 Tab；
- 删除/归档确认使用独立确认状态，不复用编辑表单容器。

### B. VSCode 侧边栏增强

当前只提供一个较简单的 Kron Intents TreeView。可讨论的增强包括：

- 项目/工作区分组和当前连接状态；
- 搜索、状态过滤、最近打开和刷新状态；
- 选中节点与 Webview、编辑器锚点双向同步；
- 错误、离线和多根工作区状态在侧边栏直接呈现；
- 将“新建 intent / 新建 assumption / 打开项目目录”作为明确的标题栏操作。

### C. Kron 侧边栏容器设置

目前 Kron TreeView 与 VSCode Explorer 共用侧边栏容器，体验类似 Git 插件。可以增加一个设置项，在两种模式间切换：

- `Explorer`：继续挂在资源管理器下；
- `Kron`：使用独立 Activity Bar 容器。

该设置需要同步 `package.json` 的 `viewsContainers` / `views` 声明和激活逻辑，并验证 Cursor 兼容性；在完成 UX 方案前不直接修改 manifest。
