# frontend/vscode/ — VSCode 扩展占位（2026-10-09 启动）

> 2026-10-09 拍板：客户端层代码**进** Kron 主仓（[`docs/rfc/2026-10-09-gui-ide-plan.md`](../../../docs/rfc/2026-10-09-gui-ide-plan.md) §6）。
>
> 本目录是**占位**——`yo code` 骨架的填充节奏由 user 拍板，**不**预排"v1.x 阶段"。

## 启动时做什么

按 `docs/rfc/2026-10-09-gui-ide-plan.md` §9.2：

1. `npx --package yo --package generator-code -- yo code`（yo code 生成器）
2. 选 TypeScript + Webpack 模板
3. 加 `vscode-languageclient` 处理 LSP 客户端
4. 加 `@modelcontextprotocol/sdk`（或 VSCode 1.85+ 内置 MCP 客户端）处理 MCP
5. `pnpm dlx shadcn@latest init`（shadcn/ui 初始化）
6. spike 验证：最小 `.vsix` 装在 VSCode F5 host，侧边栏能调 `kron serve-mcp` list 工具

## 职责边界

按 [`docs/rfc/2026-10-09-gui-ide-plan.md` §3](../../docs/rfc/2026-10-09-gui-ide-plan.md)：

- ✅ **功能最全**：hover / definition / 侧边栏 / 详情面板 / 弹窗 / 跳转 / 关系图
- ❌ **不**做项目管理（GUI 职责）

## 调用方式

进程外 stdio 子进程：
- `kron serve-mcp`（业务能力：list / get / read）
- `kron serve-lsp`（编辑器集成：hover / definition）

**不**直接 import `internal/`。

## 不在范围内

- ❌ **不**直接 import `internal/store` / `internal/parser` / `internal/lint`（铁律 #9 + #1）
- ❌ **不**与 `frontend/wails/` 互相 import
- ❌ **不**预设图表库（shadcn 原生 Chart 优先，spike 验证后拍）
- ❌ **不**在 v1 实施 `textDocument/completion` + `publishDiagnostics`（待 user 拍）
