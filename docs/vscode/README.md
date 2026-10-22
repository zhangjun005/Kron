# VSCode / VSIX 专题交接

> 更新时间：2026-10-21
>
> 本目录记录 VSCode/Cursor 扩展和 VSIX 的当前状态。LSP 服务端协议细节仍以
> [`docs/implementation/lsp.md`](../implementation/lsp.md) 为准；LSP 服务端的交接记录在
> [`docs/lsp/README.md`](../lsp/README.md)。

## 已有内容

`frontend/vscode/` 已经是可编译的扩展工程，当前包含：

- 对每个启动时已打开的 workspace folder，`vscode-languageclient` 启动独立的 `kron serve-lsp -CWD <workspace-root>`；
- 对每个 workspace folder，`@modelcontextprotocol/sdk` 启动独立的 `kron serve-mcp -CWD <workspace-root>`；
- Explorer 中的只读 Kron Intents 树；
- `kron.refreshIntents` 刷新命令；
- `kron.serverPath` 可配置 Kron 可执行文件路径；
- `kron.lspLanguages` 用 zod 校验语言范围，默认覆盖 Go、Markdown、TypeScript、Python 等常用语言；
- deactivate 时停止所有 LSP 和 MCP 子进程；多根工作区的意图树按 workspace 分组。

源码入口是 [`frontend/vscode/src/extension.ts`](../../frontend/vscode/src/extension.ts) 和 [`frontend/vscode/src/intent-tree.ts`](../../frontend/vscode/src/intent-tree.ts)。

## 当前产物

- 依赖和 TypeScript 工程已在仓库中，`out/` 是本机生成的编译目录；
- 仓库中有已打包的 [`kron-vscode-0.0.1.vsix`](../../frontend/vscode/kron-vscode-0.0.1.vsix)；源码变更后须重新执行 `npm run compile` 和 `npm run package`；
- 当前版本号仍是 `0.0.1`，没有 Marketplace 发布信息或发布流程。

## 当前范围

已经能作为开发期骨架使用，但还不是完整的编辑器产品：

- LSP 只实现 hover 和 definition；扩展文档里提到的 completion、diagnostics 尚未在服务端实现；
- 意图树是只读的，没有 intent/assumption 编辑表单、详情页、关系图或写操作 UI；
- 多根 workspace 子进程和意图树已实现；当前 workspace folder 列表在扩展激活时读取，运行期间动态增删 folder 尚未支持；
- 没有仓库内的 Extension Host 自动化验收；
- VSIX 的存在只证明打过包，不能证明 VSCode/Cursor 中的实际行为已经验收。

## 交接验收顺序

1. 在 VSCode Extension Development Host 中验证无 workspace、错误 `serverPath`、启动握手、Hover/F12、未保存修改、MCP 多根树刷新和扩展停机；
2. 用当前源码重新打包 VSIX，检查包内 `extension.js`、依赖和 `package.json` 版本；
3. 再决定是否扩展动态 workspace folder 管理、编辑表单、诊断和 completion。
