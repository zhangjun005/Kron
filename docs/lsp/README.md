# LSP 专题交接

> 更新时间：2026-10-21
>
> 本目录用于把 LSP 当前状态交给后续负责的伙伴。实现细节仍以
> [`docs/implementation/lsp.md`](../implementation/lsp.md) 为准；本目录记录现状判断、风险和接手顺序。

## 当前结论

`cmd/kron/serve-lsp/` 已合入主分支，能通过 stdio 提供基础的 `initialize`、`hover`、`definition`、`shutdown` 和 `exit`。服务端通过 `internal/parser` 扫描 `// @kron:intent <slug>`，再通过 `internal/store` 读取 intent Markdown。

VSCode 扩展已能启动两个独立子进程：`serve-lsp` 负责编辑器语义能力，`serve-mcp` 负责只读意图树。当前扩展只连接第一个 workspace folder。

## 当前阻塞点

按本次交接决策，服务端不声明文档同步能力。Hover 和 definition 每次从磁盘读取已保存内容；未保存的编辑不会参与解析。

已保存文件上的 hover 和跳转可用；保存后重新请求会读到磁盘新内容。若未来需要保存前响应未保存编辑，应单独实现按 URI 的文档快照、parser 内存文本扫描接口及并发测试。

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

- `docs/implementation/lsp.md` 中 `frontend/extensions/vscode/` 已统一为 `frontend/vscode/`。
- `docs/how-it-works.md` 已更新为基础 LSP 与 VSCode 扩展骨架已交付，未保存内容同步及编辑器内验收仍有缺口。
