# LSP 专题交接

> 更新时间：2026-10-21
>
> 本目录用于把 LSP 当前状态交给后续负责的伙伴。实现细节仍以
> [`docs/implementation/lsp.md`](../implementation/lsp.md) 为准；本目录记录现状判断、风险和接手顺序。

## 当前结论

`cmd/kron/serve-lsp/` 已合入主分支，能通过 stdio 提供基础的 `initialize`、`hover`、`definition`、`shutdown` 和 `exit`。服务端通过 `internal/parser` 扫描 `// @kron:intent <slug>`，再通过 `internal/store` 读取 intent Markdown。

VSCode 扩展已能启动两个独立子进程：`serve-lsp` 负责编辑器语义能力，`serve-mcp` 负责只读意图树。当前扩展只连接第一个 workspace folder。

## 当前阻塞点

服务端在 `initialize` 中声明了全文文档同步（`OpenClose=true`、`Change=Full`），但 `didOpen`、`didChange`、`didClose` 仍是空操作。Hover 和 definition 每次从磁盘读取，因此未保存的编辑不会参与解析。

这不会阻止已保存文件上的 hover 和跳转，但会造成编辑器内结果滞后。后续必须二选一：

1. 实现按 URI 保存的文档快照，处理全文变更，并让 hover/definition 优先读取快照；
2. 在实现快照前撤销同步能力声明，明确 LSP 只读取磁盘上的已保存内容。

当前更适合先采用第 2 种，保持 v1 的只读范围清晰；如果要支持未保存内容，第 1 种应作为独立实现任务，并补 parser 从内存文本扫描的接口和并发测试。

## 仍需验证

- 真实 Content-Length stdio 客户端启动、握手、请求和退出流程；
- VSCode Extension Host 中的 hover 与 F12 人工验收；
- `Ctrl+S` 前后 hover/definition 行为；
- Ctrl+C 或父进程退出时 `serve-lsp` 的进程收尾；
- 多根 workspace 行为；当前只使用第一个 workspace folder。

## 交接入口

- [LSP 实现状态与协议契约](../implementation/lsp.md)
- [LSP 合并问题记录](../implementation/merge-issues-2026-10-19.md)
- [VSCode 扩展专题](../vscode/README.md)
- [VSCode 扩展实现指南](../implementation/vscode-extension.md)
- [架构边界](../abstractDesign/architecture.md)

## 文档清理项

- `docs/implementation/lsp.md` 中仍有 `frontend/extensions/vscode/` 的旧路径，应改为 `frontend/vscode/`。
- `docs/how-it-works.md` 仍写着 v1 不交付 LSP server，和当前代码状态不一致，应改成“已交付基础 LSP，仍有同步和验收缺口”。
