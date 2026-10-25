# VS Code LSP 人工验收记录

日期：2026-10-22  
结果：**未完成**（没有观测到 Extension Development Host 内的 Hover/F12 UI）

## 目标样例

仓库内的 `.d-test` fixture 可用于编辑器验收：

- 文件：`internal/auth/refresh.go`
- 锚点：`// @kron:intent auth/refresh-token`
- 对应 intent：`.kron/intents/auth/refresh-token.md`

## 本次实际检查

- 桌面接口识别到一个 VS Code 窗口，标题为 `kron.exe - Kron - Visual Studio Code`；当时没有识别到 Extension Development Host 窗口。
- 尝试用隔离 profile 启动 Extension Development Host。受限进程启动 VS Code 时被 Electron 安装目录 ACL 拒绝；没有更改系统 ACL。
- 随后尝试恢复已有 VS Code 窗口继续操作，但窗口状态持续报告为最小化，未能取得可操作的编辑器界面。

## 未验收项目

本次没有实际观察到以下编辑器行为，不能记为通过：

- 锚点行 Hover 显示 intent 内容；
- F12 跳转到 intent Markdown；
- 未保存时修改锚点后 Hover/F12 立即反映变化；
- 保存前后及关闭文件后的行为；
- 多根 workspace 中各根目录的 Hover/F12、意图树分组和刷新；
- 扩展停机时 LSP/MCP 子进程退出。

服务端快照、真实 stdio e2e 和 Ctrl+C 的自动化状态以 [`lsp.md`](./lsp.md) §0 记录为准；这些自动化结果不替代上面的编辑器人工验收。

## 后续验收入口

在可见的 VS Code 窗口中打开 `.d-test`，用 `frontend/vscode/` 作为 Extension Development Host 的开发扩展目录，并将 `kron.serverPath` 配置为仓库根目录的 `kron.exe`。随后按 [`vscode-extension.md`](./vscode-extension.md) 验证 Hover/F12、保存前后行为、树刷新和停机；记录实际看到的结果后再把本日志更新为通过或部分通过。
