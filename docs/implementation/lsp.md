# LSP 开发边界与交付契约

本文件面向 `cmd/kron/serve-lsp/` 开发者。它说明 LSP 的职责，不替代新增依赖和
`internal/` API 所需的审批流程。架构边界以
[`architecture.md`](../abstractDesign/architecture.md) 为准，锚点语法以
[`intent-structure.md`](../abstractDesign/intent-structure.md) 为准。

## 当前状态

`cmd/kron/serve-lsp/` 目前只是返回“not implemented”的启动骨架，不能由 VSCode 扩展作为
可用语言服务启动。实现前仍须处理已记录的 Go SDK 依赖批准，以及位置感知锚点解析 API 的
决定；这两项不能由客户端代码绕过。

## LSP 只做什么

LSP 是编辑器语义协议适配器。它只读取 Kron 数据并返回编辑器协议结果：

| 能力 | v1 首次交付 | 行为 |
|---|---|---|
| `initialize` / `shutdown` / `exit` | 是 | 标准生命周期 |
| `textDocument/hover` | 是 | 光标在锚点上时展示 intent 标题、摘要和必要状态 |
| `textDocument/definition` | 是 | 跳到对应 intent Markdown |
| completion | 否 | 后续才考虑 slug 补全 |
| diagnostics | 否 | 后续才考虑复用 lint 结果发布诊断 |

LSP 不创建或编辑 intent、assumption 或锚点，不弹出业务表单，不维护项目索引，也不调用
`serve-mcp`。所有写操作由 VSCode 扩展经 MCP 发起。

## 位置语义

一个文件可有多个锚点；仅知道该文件有哪些 slug 不足以服务 hover 或 definition。LSP 必须以
请求的 line/character 定位光标所在的具体锚点，并返回该锚点的 slug。锚点不存在、目标 intent
不存在或其已删除时，hover 与 definition 返回 `null`/空结果，不返回协议错误。

锚点的解析规则不能复制到 `cmd/kron/serve-lsp/`。若现有 parser API 不携带行号，必须先按
`new-internal-api` 流程将“单文件、带位置的锚点提取”下沉到 `internal/parser`，再由 LSP 做
位置匹配。这既避免重复语法解析，也保证 lint、MCP 与 LSP 对同一行代码得出同一结论。

## 进程与依赖边界

- 每个编辑器 workspace 启动一个 `kron serve-lsp -CWD <workspace-root>`，stdin/stdout 是
  LSP 的 Content-Length JSON-RPC；编辑器退出即进程退出。
- LSP 与 MCP 是兄弟访问层：不得 import、调用或通过内存共享状态互相连接。
- LSP 只调用它需要的 `internal/parser` 与 `internal/store` 能力；业务逻辑留在 `internal/`。
- 不使用 caller 注入；`context.Context` 仅传递取消和 deadline。
- 采用已经选定的 LSP SDK 前，新增 Go 顶层依赖仍须显式批准；不得手写协议帧来规避该要求。

## 验收

- 标准 VSCode `LanguageClient` 能完成握手并优雅关闭。
- 同一文件两个不同锚点的 hover/definition 必须各自命中正确 slug。
- 悬空锚点返回空结果，编辑器不显示错误弹窗。
- LSP 的测试覆盖有效锚点、悬空锚点、多个锚点、非锚点位置和定义 URI。
