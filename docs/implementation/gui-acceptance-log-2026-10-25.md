# Wails GUI 验收记录

日期：2026-10-25  
验收提交：`c22c0e6`  
平台：Windows / amd64

## 结果

**通过已覆盖的实时预览路径。** 使用仓库内 `kron-mcp-test/project/` 作为已注册项目启动生产构建的 Wails GUI，确认窗口正常打开并通过 `kron serve-mcp` 加载数据。

## 自动验收

- `frontend/wails/frontend`: `npm run build` 通过（TypeScript 检查和 Vite 生产构建）。
- `frontend/wails`: `go test -tags "desktop production e2e" -run TestAppBridgeE2E -v .` 通过。
- 桥接 E2E 覆盖注册表读取、MCP 初始化与 18 个工具、意图树、意图详情、lint、assumptions、项目切换以及 CLI 路径解析。
- fixture 返回 5 个意图、2 个 assumptions，lint 汇总为 1 error / 1 warning。

## 桌面检查

- 生产 GUI 二进制成功启动，Projects 视图显示注册表中的项目。
- Explorer 显示 auth/payments 意图层级、归档项和 assumptions；意图画布显示节点与关系连线。
- Source Control 概览显示待处理意图计数以及 `1 errors · 1 warnings`。
- 窗口状态和实时 MCP 连接正常。

## 范围说明

- 本次注册表只配置了一个 fixture 项目；未在 UI 中验收两个不同项目之间的切换。E2E 的 `SwitchProject` 使用当前 fixture 路径。
- 界面版本标签仍显示占位值 `0.0.0-skeleton`。
- 此记录覆盖 Wails GUI，不代表 VS Code LSP 的人工 Hover/F12 验收；后者见 [`lsp-editor-acceptance-log-2026-10-22.md`](./lsp-editor-acceptance-log-2026-10-22.md)。
