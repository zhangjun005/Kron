# IDE / LSP 交互实施参考（Phase 2 占位）

> 状态：**未实现**。v1 不交付 IDE 插件 / LSP server / GUI 客户端。
> 本文件提前记录 Phase 2 实施时的"代码 → MD"交互模式，避免 v1 期间锚点格式与未来 LSP 能力错位。

---

## 1 三路触发位置

| 触发位置 | 元素形式 | Hover | Ctrl+单击 |
|---|---|---|---|
| 源码 | `// @kron:intent jwt-sliding-window` | 弹出意图摘要 | 跳转打开对应 `.md` |
| 意图 MD（横向） | `[@token-bucket](../rate-limit/token-bucket.md)` | 弹出被依赖意图摘要 | 跳转打开目标 `.md` |
| 意图 MD（纵向父级） | `[@auth](README.md)` 或自动推导 | 弹出父模块背景与范围边界 | 跳转打开父级 README.md |

---

## 2 与 v1 锚点格式的关系

v1 `internal/parser.ScanAnchors` 已支持 `// @kron:intent <slug>` 行级扫描。

Phase 2 实施时：

- **LSP 路径**：复用 `internal/parser` + `internal/store`，独立打包 `cmd/kron/serve-lsp/`
- **IDE 插件路径**：独立项目（如 VSCode / Cursor 扩展），**只** import `internal/parser` 与 `internal/store`，不依赖 LSP server
- **GUI 路径**：通过独立 HTTP API（见 [`mcp.md`](./mcp.md) §5）

> **访问层之间禁止互相调用**（architecture.md §〇 铁律 #3）。三路实施路径各自独立 import `internal/`，不互相依赖。

---

## 3 为什么不放 v1

| 原因 | 说明 |
|---|---|
| 范围控制 | v1 只交付 CLI + MCP；GUI / LSP / IDE 是 Phase 2 |
| LSP 协议成本 | 文件同步、Position 偏移、生命周期管理——v1 不碰 |
| IDE 插件开发链 | VSCode / Cursor 扩展独立打包，与 Go 仓库解耦 |
| GUI HTTP API | 需要独立 API 包，与 MCP 是平级访问层 |

锚点格式（`// @kron:intent <slug>`）在 v1 已固化，未来 Phase 2 实施时**不**应改变此格式——避免破坏现有用户。
