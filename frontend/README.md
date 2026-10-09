# frontend/ — Kron 客户端层占位（Wails v1.3+ 启动时启用）

> 本目录是 v1.3+ Wails GUI 客户端的**占位**。当前 v1 / v1.2 阶段**不**启用，
> 按 [`docs/abstractDesign/architecture.md`](../docs/abstractDesign/architecture.md) §〇 铁律 #9 + §〇·五·1，
> 客户端层**不**进 Kron Go 主仓——本目录是**最小**占位，Wails 项目**启动时**再填实。

## 为什么是占位（不是 v1 必需）

按 architecture §〇·五·1 表格：

| 层 | 仓库位置 |
|---|---|
| 对象层 | `internal/model/` |
| 业务层 | `internal/{store,parser,lint,...}` |
| **协议访问层** | `cmd/kron/{cli,serve-mcp,serve-lsp}/` |
| **客户端层** | **外部**（VSCode 扩展 / Wails / Cursor / Web GUI）|

Kron **主仓**只 commit 3 个协议访问层 + 业务层 + 对象层。客户端层 (Wails、VSCode 扩展、Cursor 客户端) **不**进主仓——这是**架构铁律**。

**所以**：

- 本目录**不**是"将来要开发的 Wails 代码"——`frontend/` 的实际内容（Go Wails 后端 + TS shadcn 前端）会**单独**放在一个**独立**仓（按 `docs/process/new-access-layer.md` §3 + 客户端层定义），与 Kron Go 主仓解耦。
- 本目录**仅**作为**视觉占位** + 提醒"v1.3 启动时启用 Wails"。

## 占位内容（本目录现状）

- `README.md`（本文件）—— 唯一文件
- 未来**不**会**自动**填代码

## v1.3 启动时该做什么

按 architecture §〇·五·2 + `docs/process/new-access-layer.md`：

1. **开 RFC** [`docs/rfc/`](../docs/rfc/)：Wails SDK 选型 / 多项目 GUI 形态 / 意图树预览组件 / 与 serve-mcp 实例复用模式。
2. RFC 拍板后**单独建仓** `github.com/xxx/kron-wails`（或类似），不在主仓开发。
3. **Wails 进程**通过 stdio 子进程调 `kron serve-mcp`（与 AI 客户端**共用**同一个 serve-mcp 实例——见 `docs/rfc/2026-10-08-mcp-lifecycle.md` §6.1 拍板的"一个项目一个 MCP 实例"）。
4. Wails **不**直接 import `internal/`——只走 serve-mcp JSON-RPC，符合铁律 #9。
5. Wails 轻量职责：多项目概览 + 意图树预览（按 architecture §七 "范围外" 描述：Wails = 轻量，不重复造 HTTP 端）。

## 与 `.gitignore` 的关系

`.gitignore` 第 28-30 行：

```
# Frontend (will be reintroduced when GUI phase starts)
frontend/node_modules/
frontend/dist/
frontend/.vite/
```

Wails 项目**实际**的 node_modules / dist / .vite 输出**会**进 `frontend/`，**但**与本 README 的"占位"模式**不冲突**——本目录 v1.3+ 启用 Wails 后**整体迁移**到独立仓。

## 与 Wails SDK 选型的关系

`docs/abstractDesign/architecture.md` 提到"v1.3+ GUI 选 Wails"，但**具体 SDK 锁定 RFC 还未开**（`docs/rfc/2026-10-07-gui-stack.md` 已归档 SUPERSEDED，状态待重写）。本目录**不**预设任何技术选型。

## 不在本占位范围

- ❌ **不**预设 Wails 版本
- ❌ **不**预设前端框架（shadcn/ui 是 architecture.md 提到的候选，但**未**拍板）
- ❌ **不**写任何 Go Wails 后端代码
- ❌ **不**写任何 TypeScript / React / Vite 代码
- ❌ **不**改 `.gitignore` 把 `frontend/` 整体 ignore——本目录是占位，README 应**进** git

## 与 LSP 占位的关系

- LSP 协议访问层 = `cmd/kron/serve-lsp/` (3-of-3 协议访问层，**在主仓**)
- LSP 客户端 = VSCode 扩展 / Cursor / Neovim 客户端，**不在主仓**——按 §〇 铁律 #9 客户端层不存主仓
- Wails 客户端 = 同上，**不在主仓**——本目录是占位，**不**是实施起点
