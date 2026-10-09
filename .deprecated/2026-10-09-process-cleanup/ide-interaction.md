# IDE / LSP 交互实施参考

> 范畴: 本文件是 v1.3 实施 LSP server (`serve-lsp`) 与 VSCode 扩展 / Cursor 接入的实施细节契约. 拍板自 [`docs/rfc/2026-10-08-lsp-client.md`](../rfc/2026-10-08-lsp-client.md). **不**描述 GUI / IDE 内部实现.

---

## 1 三路触发位置

| 触发位置 | 元素形式 | Hover | Ctrl+单击 |
|---|---|---|---|
| 源码 | `// @kron:intent jwt-sliding-window` | 弹出意图摘要 | 跳转打开对应 `.md` |
| 意图 MD（横向） | `[@token-bucket](../rate-limit/token-bucket.md)` | 弹出被依赖意图摘要 | 跳转打开目标 `.md` |
| 意图 MD（纵向父级） | `[@auth](README.md)` 或自动推导 | 弹出父模块背景与范围边界 | 跳转打开父级 README.md |

---

## 2 与 v1 锚点格式的关系

v1 `internal/parser.ScanAnchors` 与 `internal/parser.SlugsForFile` 已支持 `// @kron:intent <slug>` 行级扫描. LSP server 复用这 2 个 API, **不**新加 `AnchorAtPosition` (见 [`docs/rfc/2026-10-08-lsp-client.md` §4.4](../rfc/2026-10-08-lsp-client.md)).

**v1.3 实施路径**:

- **LSP 路径**: 复用 `internal/parser` + `internal/store`, 独立打包 `cmd/kron/serve-lsp/`
- **VSCode 扩展 / Cursor 路径**: 独立项目 (`extensions/vscode-kron/`, **不**在 Kron Go 仓库), 起 2 个子进程:
  - `kron serve-lsp` (LSP server, hover/definition) — stdio JSON-RPC
  - `kron serve-mcp` (MCP server, 12 工具) — stdio JSON-RPC
- **GUI 路径**: 独立 HTTP API (`serve-gui`, 见 [`gui-stack.md`](../rfc/2026-10-07-gui-stack.md))

> **访问层之间禁止互相调用** (architecture.md §〇 铁律 #2). VSCode 扩展**自己**起 `serve-mcp` + `serve-lsp` 两个子进程, 两进程**互不**通讯. 见 [`2026-10-08-lsp-client.md` §4.3](../rfc/2026-10-08-lsp-client.md) 进程模型图.

---

## 3 LSP 能力 v1.3

| LSP method | 实现? | 数据流 | 备注 |
|---|---|---|---|
| `initialize` | ✅ | 静态 | 报 server name / version / capabilities |
| `textDocument/hover` | ✅ | `parser.SlugsForFile` + `store.Get` | hover `// @kron:intent <slug>` → 显示 intent 标题 + summary (Markdown 块) |
| `textDocument/definition` | ✅ | `parser.SlugsForFile` + 跳 `.kron/intents/<slug>.md` | ctrl+左键 跳到 intent MD 文件 |
| `textDocument/completion` | ⏳ v1.4+ | `store.List` | 补全已有 slug |
| `textDocument/publishDiagnostics` | ⏳ v1.4+ | `internal/lint.Run` (A-class) | 与 CLI lint A-class 共用 |
| `shutdown` / `exit` | ✅ | 静态 | 正常退出 |

**为什么不放 v1**:

| 原因 | 说明 |
|---|---|
| 范围控制 | v1 只交付 CLI + MCP; GUI / LSP / IDE 不属于 v1 交付 |
| LSP 协议成本 | 文件同步、Position 偏移、生命周期管理——v1 不碰 |
| IDE 插件开发链 | VSCode / Cursor 扩展独立打包, 与 Go 仓库解耦 (`extensions/vscode-kron/` 子项目) |
| GUI HTTP API | 需要独立 API 包, 与 MCP 是平级访问层 |

锚点格式 (`// @kron:intent <slug>`) 在 v1 已固化, 未来实施时**不**应改变此格式——避免破坏现有用户. **v1.3+** 新增 MD 锚点 (`.kron/` 外任何 .md 也能用同一语法, 见 [`2026-10-08-md-anchors.md`](../rfc/2026-10-08-md-anchors.md)).
