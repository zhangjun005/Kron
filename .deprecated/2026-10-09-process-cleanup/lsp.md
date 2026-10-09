# LSP Server 实施细节 (`serve-lsp`)

> 拍板自 [`docs/rfc/2026-10-08-lsp-client.md`](../rfc/2026-10-08-lsp-client.md). SDK 锁定 [`docs/rfc/2026-10-07-lsp-sdk.md`](../rfc/2026-10-07-lsp-sdk.md) (`go.lsp.dev/protocol` + LSP 3.17 + stdio).

---

## 1 文件树

```
cmd/kron/serve-lsp/
├── main.go              ← cobra 启动 stdio LSP server
├── server.go            ← 注册 initialize / hover / definition / shutdown / exit
├── handlers/
│   ├── hover.go         ← 调 parser.SlugsForFile + store.Get + 渲染 Markdown
│   └── definition.go    ← 调 parser.SlugsForFile + 跳 .kron/intents/<slug>.md
└── render/
    └── hover.go         ← Markdown 模板 (title / frontmatter summary / body first 5 lines)
```

**依赖** (`go.mod`):
- `go.lsp.dev/protocol v3.17.0+` (LSP SDK, 走 explicit approval = 旧 RFC §8 + 本 RFC §6.3)
- `internal/parser` (复用 `SlugsForFile`)
- `internal/store` (复用 `Get`)
- `internal/model` (类型)

**不**依赖: `internal/relations` (v1.3 hover 不需要反向链接), `internal/lint` (v1.3 不发 diagnostics).

---

## 2 进程模型

见 [`2026-10-08-lsp-client.md` §4](../rfc/2026-10-08-lsp-client.md):

- **stdio 父进程寿命** — 编辑器退出 = `serve-lsp` 退出. **不**做 daemon.
- **单实例单请求, 串行处理** — hover / definition 都是纯读, 串行延迟 < 5ms.
- **互不调用 serve-mcp** — VSCode 扩展**自己**起两个子进程.

---

## 3 hover 实现

### 3.1 触发条件

LSP client 在 `textDocument/hover` 请求里给 `Position` (line + col). 服务端:

1. 拿 `params.TextDocument.URI` → file path
2. 调 `parser.SlugsForFile(file)` 拿**此文件所有**锚点 (slug list)
3. 调 `store.Get(ctx, slug)` 拿 intent 全文 (每个 slug)
4. 遍历**每个** intent, **匹配**:
   - **位置精确匹配**: anchor 行号 = `params.Position.Line + 1` (LSP 0-indexed, parser 1-indexed)
   - 或**位置近似匹配**: anchor 行在 `params.Position.Line ± 2` 范围内 (允许 hover 时鼠标在 anchor 注释**内**任意位置)
5. 调 `render.Hover(intent)` 渲染 Markdown
6. 返回 `protocol.Hover{Contents: protocol.MarkupContent{Kind: protocol.Markdown, Value: ...}}`

### 3.2 hover 渲染模板

```markdown
# <Intent Title>                          ← 从 body 第一行 `# ` 提取

> <One-line summary>                      ← 从 body 第二段 `> ` 提取

---

**Slug**: `auth/jwt-sliding-window`
**Status**: active
**Created by**: @zhangjun005
**Updated at**: 2026-09-22T10:00:00Z

## Assumptions
- hard "single-region" — 服务仅部署在单 region... (expires 2026-12-31)
- soft "redis-availability" — Redis 99.9% 可用...

## Why (前 5 行)
<Body §Why 前 5 行>

[kron://open?slug=auth/jwt-sliding-window]                    ← 跳 definition
```

### 3.3 悬空锚点处理

`parser.SlugsForFile` 拿到的 slug **不**校验存在性 (走 parser 即可, store.Get 会返回 `ErrIntentNotFound`). v1.3 hover 行为:

- 锚点**指向不存在** intent → hover 返回 `null` (LSP 规范: null = no hover, 编辑器**不**显示弹窗)
- **不**在 hover 内报错 (kron lint 错误由 `kron_lint` MCP 工具 / `kron lint` CLI 报, **不**是 hover 职责)
- v1.4+ 评估: 用 `textDocument/publishDiagnostics` 报红线 (与 CLI lint A-class 共用)

---

## 4 definition 实现

### 4.1 触发条件

LSP client 在 `textDocument/definition` 请求里给 `Position`. 服务端:

1. 同 hover 前 4 步
2. 调 `store.Get(ctx, slug)` (已有)
3. 返回 `protocol.Location{
   URI: protocol.URI("file:///<root>/" + intent.SourcePath),
   Range: protocol.Range{Start: protocol.Position{Line: 0, Character: 0}, End: protocol.Position{Line: 0, Character: 0}},
}`

### 4.2 跳目标

- 默认跳 `.kron/intents/<slug>.md` 顶部 (line 0, col 0)
- 编辑器打开后**不**滚动到具体章节 (LSP 协议**不**支持 markdown section 级跳转; v1.4+ 评估 `textDocument/documentLink`)

---

## 5 与 v1 锚点扫描的对齐

v1 `internal/parser.ScanAnchors` (在 `lint.Run` 里调) 返回**全仓库**所有 `[]model.Anchor`, 含 file + line + slug. LSP hover / definition 复用 `SlugsForFile` (单文件), **不**复用 `ScanAnchors` (全仓库) — 性能/范围都**不**对.

---

## 6 测试

| 测试 | 输入 | 期望 |
|---|---|---|
| `TestHover_ExactAnchorLine` | 文件 `auth.go` line 50 = `// @kron:intent jwt`, request position (50, 5) | 返回 hover 内容含 intent title |
| `TestHover_AnchorLineApprox` | request position (51, 0) (下一行) | 仍返回 hover (允许 ±2 行) |
| `TestHover_DanglingAnchor` | 文件含 `// @kron:intent nonexistent`, request on this line | 返回 null (no hover) |
| `TestHover_NoAnchor` | 文件无锚点 | 返回 null |
| `TestDefinition_ValidSlug` | 文件含锚点, request on anchor line | 返回 Location URI = .kron/intents/<slug>.md |
| `TestDefinition_Dangling` | 同上但 slug 不存在 | 返回 null |
| `TestInitialize_Capabilities` | request = `initialize` | 返回 server capabilities (HoverProvider: true, DefinitionProvider: true) |
| `TestShutdown_Exit` | request = `shutdown` then `exit` | server 退出 (exit code 0) |

---

## 7 不在本文件范围

- ❌ LSP **协议** SDK 选型 → [`2026-10-07-lsp-sdk.md`](../rfc/2026-10-07-lsp-sdk.md)
- ❌ **跨语言客户端**接入 → [`2026-10-08-lsp-client.md`](../rfc/2026-10-08-lsp-client.md)
- ❌ VSCode 扩展 (`extensions/vscode-kron/`) 实现 — 独立子项目, **不**在 Kron Go 仓库
- ❌ GUI (`serve-gui`) HTTP API — 走独立 RFC
- ❌ completion / diagnostics v1.4+ 候选 — 暂不实现
