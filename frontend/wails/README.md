# frontend/wails/ — Wails GUI 占位（2026-10-09 启动）

> 2026-10-09 拍板：客户端层代码**进** Kron 主仓（[`docs/rfc/2026-10-09-gui-ide-plan.md`](../../../docs/rfc/2026-10-09-gui-ide-plan.md) §6）。
>
> 本目录是**占位**——`wails init` 骨架的填充节奏由 user 拍板，**不**预排"v1.x 阶段"。

## 启动时做什么

按 `docs/rfc/2026-10-09-gui-ide-plan.md` §9.1：

1. `wails init -n kron-wails -t react-ts`（Wails v2 + React + TypeScript 模板）
2. `pnpm dlx shadcn@latest init`（shadcn/ui 初始化）
3. 加 Go ↔ TS 桥（拼 JSON 调 `kron serve-mcp`）
4. spike 验证：Wails 起窗口 + 调 `kron serve-mcp` list 工具

## 职责边界

按 [`docs/rfc/2026-10-09-gui-ide-plan.md` §3](../../docs/rfc/2026-10-09-gui-ide-plan.md)：

- ✅ **整体预览**：多项目概览 + 意图树预览 + 状态条
- ❌ **不**做 hover / 跳转 / 实时诊断 / 补全 / 关系图（IDE 职责）

## 调用方式

进程外 stdio 子进程：`kron serve-mcp`（**不**直接 import `internal/`）。

## 不在范围内

- ❌ **不**直接 import `internal/store` / `internal/parser` / `internal/lint`（铁律 #9 + #1）
- ❌ **不**与 `frontend/vscode/` 互相 import
- ❌ **不**预设 shadcn Chart 选型（spike 验证后拍）
