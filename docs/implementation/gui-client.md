# Wails GUI 开发边界

本文件给 `frontend/wails/` 的开发者说明产品职责和协议边界。架构约束以
[`architecture.md`](../abstractDesign/architecture.md) 为准；intent 与 assumption 的
数据格式以 [`intent-structure.md`](../abstractDesign/intent-structure.md) 为准。

## 目标

Wails 是跨项目的整体预览器。它回答“有哪些项目、各项目的意图和假设处于什么状态”，
不承担代码编辑器职责。

| 应做 | 不做 |
|---|---|
| 已注册项目的概览、项目切换和打开项目 | 在代码编辑器中 hover、跳转或补全 |
| intent 树、摘要、状态和 lint 汇总 | 代码锚点的右键操作或实时诊断 |
| 查看 intent、assumption、关系和影响 | intent/assumption 的深度表单编辑 |
| 通过系统默认 IDE 打开某个 intent 源文件 | 自己解析或直接读写 `.kron/` 文件 |

“编辑”入口只需打开对应 Markdown 文件，由用户的 IDE 完成编辑。这样 GUI 不会形成第二套
编辑体验，也不会与 VSCode 扩展争夺同一职责。

## 与 Kron 通讯

每个已打开项目各自启动并持有一个 `kron serve-mcp -CWD <project-root>` 子进程；窗口关闭或
项目从 GUI 中关闭时停止该子进程。GUI 只通过 MCP SDK 调用工具，不能 import Go 的
`internal/` 或调用 `serve-lsp`。

首批界面使用只读工具：`kron_list`、`kron_get`、`kron_tree`、`kron_impact`、
`kron_intent_density`、`kron_stale`、`kron_lint`、`kron_assume_list`、
`kron_assume_get` 和 `kron_assume_check`。MCP 的返回值是协议边界：GUI 不自行扫描
`.kron/intents` 以补齐或缓存数据。

多进程同时访问同一项目是正常情况。GUI 不建立单例服务，也不向 VSCode 或 AI 工具发送消息；
每次刷新重新请求 MCP，Kron 的文件锁与 intent 元数据快照负责观察磁盘变化。

## 验收

- 没有 Kron 项目的项目应显示可理解的空状态，并提供“在终端初始化”或“打开项目”入口。
- 外部修改 intent 后刷新，树和详情必须显示磁盘上的新内容。
- “在 IDE 中打开”应定位到实际 intent Markdown 文件，而不是复制内容到 GUI。
- GUI 源码只位于 `frontend/wails/`，不与 `frontend/vscode/` 互相 import。
