# LSP 专题交接

> 更新时间：2026-10-21
>
> 本目录用于把 LSP 当前状态交给后续负责的伙伴。实现细节仍以
> [`docs/implementation/lsp.md`](../implementation/lsp.md) 为准；本目录记录现状判断、风险和接手顺序。

## 当前结论

`cmd/kron/serve-lsp/` 已合入主分支，能通过 stdio 提供基础的 `initialize`、`hover`、`definition`、`shutdown` 和 `exit`。服务端通过 `internal/parser` 扫描 `// @kron:intent <slug>`，再通过 `internal/store` 读取 intent Markdown。

VSCode 扩展对每个激活时已打开的 workspace folder 启动独立的 `serve-lsp` 与 `serve-mcp` 子进程；多根意图树按 workspace 分组。运行期间动态增删 workspace folder 尚未实现。

## 当前进展与剩余验收

服务端声明全文文档同步，并按 URI 维护打开文档的快照。Hover 和 definition 对打开文件使用快照中的当前内容；未打开文件从磁盘读取。保存前的编辑也会参与查询。

快照按文档版本拒绝过期变更，并在 `didClose` 时删除。真实 stdio e2e 覆盖 Content-Length 握手、Hover、Definition 与正常退出；`Run()` 已处理 Ctrl+C 取消。Ctrl+S 前后仍需在 Extension Host 验证编辑中的锚点立即影响 Hover/F12，关闭文件后使用磁盘内容。

## 仍需验证

- VSCode Extension Host 中的 hover 与 F12 人工验收；
- Ctrl+S 前后 hover/definition 行为；
- 多根 workspace 的编辑器内行为与树刷新仍需在 Extension Host 手动验收；启动时已为每个 folder 建立独立客户端。

## 交接入口

- [LSP 实现状态与协议契约](../implementation/lsp.md)
- [LSP 合并问题记录](../implementation/merge-issues-2026-10-19.md)
- [VSCode 扩展专题](../vscode/README.md)
- [VSCode 扩展实现指南](../implementation/vscode-extension.md)
- [架构边界](../abstractDesign/architecture.md)

## 文档清理项

- `docs/implementation/lsp.md` 中 `frontend/extensions/vscode/` 已统一为 `frontend/vscode/`。
- `docs/how-it-works.md` 已更新为基础 LSP 与 VSCode 扩展骨架已交付，未保存内容同步及编辑器内验收仍有缺口。
