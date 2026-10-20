# 分支实现状态记录（2026-10-18）

> **2026-10-19 后续**：`feat/serve-lsp` 已合入 `master`（合并提交 `22348eb`），本文所述「合并前仍需处理」已按合并记录处置；
> `feat/wails-skeleton` 仍独立，未合并。当前 LSP 状态以 [`lsp.md`](./lsp.md) §0 为准。

本文记录对两个伙伴分支的只读验收结果。记录基于远程分支当前提交，不代表这些分支已合并到 `master`。

## `feat/serve-lsp`

- 最新提交：`7c3c030 feat(serve-lsp): v1.3 hardening — textDocumentSync, errors.Is, logf/notify, stdio smoke`
- 与当时 `origin/master` 有共同历史；相对主线为 **9 个领先提交、31 个落后提交**（原文写成「31 领先 / 9 落后」，方向写反，2026-10-19 订正）。
- 已实现 LSP 服务端：`initialize`、`shutdown`、`exit`、`textDocument/hover`、`textDocument/definition`，以及 `didOpen`、`didChange`、`didClose` 协议处理。
- 锚点解析通过 `parser.AnchorsForFile` 读取当前文件，按光标所在行匹配 `@kron:intent <slug>`。
- Hover 返回 intent slug、status 和正文首段；正文最多 200 个 rune，并保持 UTF-8 有效。
- Definition 返回对应 `.kron/intents/<slug>.md`（或 README shorthand）的文件位置，跳转到文件开头。
- Intent 根目录不是启发式发现：默认使用 LSP 进程工作目录，也支持 `-CWD <root>`；读取路径固定为 `<root>/.kron/intents/`。当前实现不会逐级向上搜索 `.kron`。
- 当前没有文档内容缓存；`didOpen`、`didChange`、`didClose` 是空操作，hover/definition 每次从磁盘重新读取已保存内容。这符合“Markdown 文件是唯一真相源”的简化设计，但未保存编辑不会影响结果。
- 验证：在该分支工作树运行 `go test ./...` 通过。
- 合并前仍需处理：新增 LSP 顶层依赖的显式批准、`git diff --check` 报出的文档尾随空格，以及 `cmd/kron-d-test` 是否应作为开发工具保留。

### VSCode 适配状态

未实现。`frontend/vscode/` 只有占位 README，没有扩展工程、`vscode-languageclient`、激活代码、侧边栏、Webview、MCP 客户端、VSIX 打包配置或可安装 VSIX。该分支只完成 Kron 侧 LSP server，尚未完成 VSCode 客户端。

## `feat/wails-skeleton`

- 最新提交：`41a9e11 chore(wails): ignore build/ output + Wails frontend md5 cache`
- 与当前 `origin/master` 没有共同祖先；该分支来自另一套 Kron v2 基线，不能直接按普通 merge-base diff 评估。
- 已实现 Wails v2 + React + TypeScript 的 GUI 骨架，以及静态 Go↔TS 桥接：`GetKronInfo`、`ListMCPTools`。
- GUI README 明确标注真实 `kron serve-mcp` plumbing、`wails dev` 验证和 `wails build` 产物属于后续工作。
- Go 模块测试通过：在分支内层 `Kron/` 目录运行 `go test ./...` 通过。
- 前端 `npm run build` 当前失败：缺少 `vite/client` 类型，且 TypeScript 配置使用了已移除的 `baseUrl` 选项。
- 已发现工具名不一致：GUI 列表返回 `kron_density`，实际 MCP 工具名为 `kron_intent_density`。

### VSCode 适配状态

未实现。该分支同样只有 `frontend/vscode/README.md` 占位文件，没有 VSIX、侧边栏、LSP 客户端或 MCP 客户端代码。Wails GUI 骨架不能替代 VSCode 扩展。

## 职责边界记录

- `serve-lsp`：协议服务端，负责锚点解析、Hover、Definition，以及未来可选的诊断/补全。
- VSCode 扩展：负责启动 `kron serve-lsp`、接入 `vscode-languageclient`、注册侧边栏和 Webview、调用 `serve-mcp`，并提供面向人类的 intent 浏览与编辑入口。
- Wails GUI：负责多项目概览和 intent 树预览，不承担 IDE 级 Hover、跳转和实时编辑器集成。

