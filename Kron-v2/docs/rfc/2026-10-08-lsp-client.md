# RFC: LSP 跨语言客户端接入 (取代 `2026-10-07-lsp-sdk.md` 的 §1.3 + §6)

| 字段 | 值 |
|---|---|
| **状态** | **草案 (2026-10-08, zhangjun005 拍板启动)** |
| **作者** | AI assistant, 经 zhangjun005 委托 |
| **创建日期** | 2026-10-08 |
| **目标版本** | v1.3 (与 `2026-10-07-lsp-sdk.md` 同步) |
| **取代** | [`docs/rfc/2026-10-07-lsp-sdk.md`](./2026-10-07-lsp-sdk.md) §1.3 + §6 跨语言客户端接入形式 (SDK 决策 §3/§4 **保留**) |
| **影响范围** | `cmd/kron/serve-lsp/` (新, 骨架) / `docs/implementation/lsp.md` (新) / `docs/implementation/ide-interaction.md` (重写为'VSCode 扩展 / Cursor 调 serve-mcp / serve-lsp 子进程') |

---

## 1 动机 (2026-10-08)

[`docs/rfc/2026-10-07-lsp-sdk.md`](./2026-10-07-lsp-sdk.md) 锁定了 LSP 协议 SDK (`go.lsp.dev/protocol` + LSP 3.17 + stdio) 与 4 个能力 (hover/definition/completion/diagnostics), 但 §1.3 "跨语言 client 端 SDK 选" 与 §6 "调用 `internal/` 边界" **未**拍板. **SUPERSEDED 旧 RFC** 把这部分**推迟**到本 RFC.

### 1.1 拍板背景 (2026-10-08, zhangjun005)

`internal/` 4 个包 (`store` / `parser` / `relations` / `lint`) **已**完整实现, LSP 服务**只**是 thin wiring:

- `hover` = `parser.ScanAnchors(file) → 找位置 → store.Get(slug) → 渲染 Markdown`
- `definition` = 同上前两步 → 跳 `.kron/intents/<slug>.md`

**不**需要新 internal 包, **不**需要新算法 — 跨语言客户端接入是**唯一**未决问题.

---

## 2 决策总览

| 项 | 拍板值 | 来源 |
|---|---|---|
| **LSP 协议 SDK** | `go.lsp.dev/protocol v3.17+` | 旧 RFC §3 (本 RFC 沿用, **不**重选) |
| **LSP 协议版本** | 3.17 | 旧 RFC §4.1 |
| **transport** | stdio (与 MCP 同模式) | 旧 RFC §4.2 |
| **能力 v1.3** | `hover` + `definition` (最小可用) | 旧 RFC §5 |
| **能力 v1.4+** | `completion` + `publishDiagnostics` | 旧 RFC §5 |
| **跨语言客户端接入 (本 RFC 核心)** | **VSCode 扩展 / Cursor 调 `serve-mcp` + `serve-lsp` 子进程**, **不**走独立 client SDK 跨语言包装 | 本 RFC §3 |
| **serve-lsp 进程寿命** | stdio 父进程 (编辑器) 寿命 — 退出 = 退出, **不**做 daemon | 本 RFC §4.1 |
| **serve-lsp 并发** | 单实例单请求, 串行处理 (与 MCP 同模式) | 本 RFC §4.2 |
| **serve-lsp 与 serve-mcp 关系** | **互不调用** (架构铁律 #2) — VSCode 扩展**自己**起两个子进程 | 本 RFC §4.3 |

---

## 3 跨语言客户端接入 — 拍板 (取代旧 RFC §1.3)

### 3.1 候选

| 候选 | 描述 | 评估 |
|---|---|---|
| **A. 编辑器分别起 `serve-mcp` + `serve-lsp` 两个子进程** | VSCode 扩展 / Cursor 配置: 两个 child process, 各自独立 stdio | ✅ **推荐** — 复用现有 5 access layer 架构, 0 新 SDK 依赖 |
| B. 写跨语言 SDK (类似 `@kron/lsp-client` npm 包) | 跨语言 client 端 SDK 包装层 | ❌ **拒绝** — 与 serve-mcp 跨 AI 客户端的"独立 spawn" 模式不一致; 重复劳动 |
| C. 起 HTTP server (`serve-lsp --http`) | 多 client 可连同一个 LSP 实例 | ❌ **拒绝** — LSP over TCP 不是主流 client 习惯; 增加进程模型复杂度 |

### 3.2 选 A 的具体形式

**VSCode 扩展 (`extensions/vscode-kron/`, 新建)**:

```typescript
// extensions/vscode-kron/src/extension.ts
import * as vscode from 'vscode';
import { spawn, ChildProcess } from 'child_process';
import { LanguageClient } from 'vscode-languageclient/node';

export function activate(context: vscode.ExtensionContext) {
  // 1. 起 serve-lsp 子进程 (LSP server, hover/definition 用)
  const lspServer: ChildClient = spawnLspServer('kron', ['serve-lsp']);
  const lspClient = new LanguageClient(
    'kron', lspServer,
    { documentSelector: [{ scheme: 'file', pattern: '**/*.{go,ts,py,md}' }] }
  );
  lspClient.start();

  // 2. 起 serve-mcp 子进程 (MCP server, 12 工具给 Cursor AI 用)
  const mcpServer = spawn('kron', ['serve-mcp'], { stdio: ['pipe','pipe','pipe'] });
  registerMcpClient(context, mcpServer);
}
```

**Cursor 配置 (`~/.cursor/mcp.json`)**:

```json
{
  "mcpServers": {
    "kron": {
      "command": "kron",
      "args": ["serve-mcp"]
    }
  }
}
```

**Cursor LSP** 通过 **VSCode extension 兼容层** 自动发现 — VSCode 扩展 `kron.vscode-kron` 安装后, Cursor 复用其 LSP client 注册.

### 3.3 拍板理由 (用户 2026-10-08 接受)

1. **与 serve-mcp 接入模式一致** — AI Agent 走 stdio JSON-RPC, 编辑器走 stdio JSON-RPC (LSP), **对称**
2. **0 新依赖** (除 LSP SDK `go.lsp.dev/protocol`, 旧 RFC §8 已拍)
3. **VSCode 扩展**用 `vscode-languageclient` (TypeScript) 是**事实标准**, 不需要新写
4. **未来跨编辑器** (Neovim / Helix) 各 client 端**自己**实现 LSP client 协议 — **不**下沉 SDK 到 Kron 仓库
5. **客户端 SDK 由各编辑器生态提供**: VSCode 走 `vscode-languageclient` (TS, 事实标准), Cursor 复用 VSCode extension, Neovim 走 `nvim-lspconfig`, Helix 走内置 LSP. Kron 仓库**不**下沉跨语言 client SDK (重复 B 候选)

### 3.4 不在本 RFC 范围

- ❌ VSCode 扩展**本身** (TypeScript 包) — 走 `extensions/vscode-kron/` 独立子项目, **不**在 Kron Go 仓库
- ❌ Cursor / Neovim / Helix 客户端实现细节 — 各编辑器生态**自己**写 LSP client
- ❌ LSP **业务逻辑** (hover 渲染什么 / definition 跳哪) — 走 `docs/implementation/lsp.md` (本 PR 同步新建)

---

## 4 serve-lsp 进程模型 (取代旧 RFC §6)

### 4.1 进程寿命

**stdio 父进程 (编辑器) 寿命** — 编辑器退出 = `serve-lsp` 退出. **不**做 daemon. **不**做 socket.

理由 (与 serve-mcp 一致, 见 `architecture.md` §〇·五·5):

- LSP 协议**强约束** stdio / socket 二选一; stdio 简单
- 多实例 (多编辑器窗口) = 多子进程, **无共享状态**, 避免锁

### 4.2 并发模型

**单实例单请求, 串行处理** — LSP 协议**允许**并发 request, 但 v1.3 **不**实现. 理由:

- hover / definition 都是**纯读** (`parser.ScanAnchors` + `store.Get`), 串行足够
- 文件 I/O 是**本地** + **小文件** (< 100 KB), 串行延迟 < 5ms
- 串行实现**简单** — 无锁, 无 goroutine 调度, 无 ctx 取消复杂度

**v1.4+ 评估**并发 (`go func` + `errgroup`):

- 触发条件: hover 渲染调用 `relations.ReverseLinks` (可能慢)
- 风险: 串行实现**先**上线, 性能**真不够**再优化

### 4.3 serve-lsp 与 serve-mcp 关系

**架构铁律 #2 (来自 `architecture.md` §〇·五·2)**: "5 个访问层 (CLI / MCP / LSP / IDE / GUI) 互相**不**调用".

`serve-lsp` 与 `serve-mcp` **互不调用**. VSCode 扩展**自己**起两个子进程 (见 §3.2):

```
┌──────────────────────────┐
│ VSCode 扩展              │
│ (extensions/vscode-kron) │
├──────────────────────────┤
│ ┌─LanguageClient────────┐│
│ │ spawn('kron',         ││
│ │       ['serve-lsp'])  ││
│ │ ↕ stdio JSON-RPC      ││
│ │  hover / definition   ││
│ └───────────────────────┘│
│ ┌─MCPClient─────────────┐│
│ │ spawn('kron',         ││
│ │       ['serve-mcp'])  ││
│ │ ↕ stdio JSON-RPC      ││
│ │  12 工具              ││
│ └───────────────────────┘│
└──────────────────────────┘
       ↑ ↓ (两个 stdio)
┌──────────────────────────┐
│ Kron 仓库                 │
│   cmd/kron/serve-lsp/   │  ← LSP server (新)
│   cmd/kron/serve-mcp/   │  ← MCP server (已存在)
└──────────────────────────┘
       ↑ ↓
┌──────────────────────────┐
│ internal/               │
│   parser/anchors.go     │  ← ScanAnchors (复用)
│   store/reader.go       │  ← Get (复用)
└──────────────────────────┘
```

### 4.4 调用 internal/ 边界 (取代旧 RFC §6)

`cmd/kron/serve-lsp/` **只**调 `internal/parser` + `internal/store`. **不**调 `internal/relations` (v1.3 hover 不需要反向链接, definition 不需要拓扑), **不**调 `internal/lint` (v1.3 不发 diagnostics).

**新增 internal/parser API** (走 `docs/process/new-internal-api.md`):

```go
// 旧 RFC §6 拍板
func AnchorAtPosition(file string, line, col int) (slug string, ok bool)
```

**实际** (经 2026-10-08 重新评估): `SlugsForFile(file)` **已**存在 (`internal/parser/anchors.go` L62) — 复用即可, **不**需要新加 `AnchorAtPosition`. 简化方案:

```go
// serve-lsp 内部伪代码
func (s *Server) hover(ctx, params) Hover {
    fileSlugs := parser.SlugsForFile(params.TextDocument.URI.Path)
    for _, slug := range fileSlugs {
        intent := store.Get(ctx, slug)
        if intent.SourcePath contains line(params.Position.Line) {
            return renderHover(intent)
        }
    }
}
```

> **修订**: 旧 RFC §6 拍板"新增 `AnchorAtPosition`" 实际**不**需要 — `SlugsForFile` 已够. **不**走 `new-internal-api.md` 流程, **因为** `SlugsForFile` **已存在** (`internal/parser/anchors.go` L70-95), 既**不**是新加 API, 也**不**改签名 — 是**直接复用**, **不**触发流程门槛.

### 4.5 serve-lsp caller 注入 — **不**管 (2026-10-08 拍)

`model.WithCaller` / `CallerFrom` / `CallerLSP` 等 API 在 `internal/model/caller.go` **仍存在** (兼容 `cmd/kron/serve-mcp` 旧 `callerInjectMiddleware` 与既有 import), 但:

- **新**访问层代码 (含本 RFC 拍板的 `serve-lsp`) **不**调 `model.WithCaller` 注入
- `internal/` **不**依赖 ctx 上的 caller key 做行为分支
- 原因: `context.Context` 注入身份属性**不便于开发** (调试栈不直观 / 单元测试需额外包装 / 类型安全弱)
- 详见 [`docs/abstractDesign/architecture.md`](../abstractDesign/architecture.md) §2.3

**实施影响**: `cmd/kron/serve-lsp/main.go` **不**写 `ctx = model.WithCaller(ctx, model.CallerLSP)`, 直接 `cmd.Context()` 传入 `store.Get` / `parser.SlugsForFile`. `ctx` 仍保留**用于取消 / 超时传播** (与 store/parser 函数签名兼容).

**未来 caller 区分场合** (本 RFC **不**列, 等需要时再拍):
- lint 输出归因 (caller-aware diagnostics)
- 审计日志
- 多租户权限

---

## 5 实施路径

### 5.1 文档同步 (v1.3 文档阶段, 立即)

- ✅ **本 RFC** `2026-10-08-lsp-client.md` (新增, 取代旧 RFC §1.3 + §6)
- ✅ `docs/implementation/lsp.md` (新建, 协议实现细节: hover 渲染 Markdown 模板 / definition 跳转 URI 格式)
- ✅ `docs/implementation/ide-interaction.md` (重写为'VSCode 扩展 / Cursor 调 serve-mcp / serve-lsp 子进程')

### 5.2 代码实施 (v1.3 启动后, 用户**不**要求立即开工 — 仅写 RFC + 文档)

**前置条件** (实施前**必**完成):
- [ ] `go.mod` 当前**未**有 `go.lsp.dev/protocol` (grep `go.mod | grep lsp`), 若**已**有则跳过加 dep
- [ ] 若**未**有, 走 "新 top-level dep" explicit approval: 在 commit body 写 "lsp.dev/protocol 依赖理由" 4 段 (竞品对比 / 包大小 / 维护活跃度 / API 稳定性)
- [ ] approval 后 `go get go.lsp.dev/protocol@v3.17.0`
- [ ] `cmd/kron/serve-lsp/main.go` **不**调 `model.WithCaller` (caller 注入 API **不**管, 走 §4.5)

**实施清单**:

- [ ] `cmd/kron/serve-lsp/main.go` (cobra 启动 stdio LSP server)
- [ ] `cmd/kron/serve-lsp/server.go` (注册 `initialize` / `textDocument/hover` / `textDocument/definition` / `shutdown` / `exit`)
- [ ] `cmd/kron/serve-lsp/handlers/hover.go` (调 `parser.SlugsForFile` + `store.Get` + 渲染 Markdown)
- [ ] `cmd/kron/serve-lsp/handlers/definition.go` (调 `parser.SlugsForFile` + 跳 `.kron/intents/<slug>.md`)
- [ ] `go.mod` 加 `go.lsp.dev/protocol v3.17.0+` (走 explicit approval = 本 RFC 旧版 §8 已拍)
- [ ] 测试: hover 拿到正确 Markdown / definition 拿到正确 URI / 悬空锚点 hover 返回 null (不报错)

### 5.3 不做的事

- ❌ **不**实现 `textDocument/completion` (v1.4+ 评估)
- ❌ **不**实现 `textDocument/publishDiagnostics` (v1.4+ 评估, 与 `kron lint` 规则 A-class 共用)
- ❌ **不**起 HTTP server (stdin/stdout only)
- ❌ **不**做 daemon (stdio 父进程寿命)
- ❌ **不**做并发 (v1.3 串行足够)
- ❌ **不**写跨语言 client SDK (VSCode 扩展用 `vscode-languageclient`, Cursor 用内置 LSP, Neovim 各写各的)

---

## 6 关键决策 (本 RFC 拍板后**不可**回退的)

1. **跨语言客户端接入**: VSCode 扩展 / Cursor **自己**起 `serve-mcp` + `serve-lsp` 两个子进程, **不**下沉 SDK
2. **进程模型**: stdio, 单实例单请求, 串行, 跟 serve-mcp 同
3. **复用 internal/**: `parser.SlugsForFile` + `store.Get`, **不**新加 `AnchorAtPosition` (旧 RFC §6 修订)
4. **能力 v1.3**: 仅 `hover` + `definition` (旧 RFC §5 沿用)
5. **v1.4+ 候选**: `completion` (slug 补全) + `publishDiagnostics` (与 lint A-class 共用)

---

## 7 与旧 RFC 的关系

| 旧 RFC `2026-10-07-lsp-sdk.md` 章节 | 状态 | 说明 |
|---|---|---|
| §1.1 背景 | ✅ 沿用 | |
| §1.2 目标 | ✅ 沿用 | |
| §1.3 跨语言 client SDK | ❌ **被本 RFC §3 取代** | |
| §2 候选评估 | ✅ 沿用 (SDK) | |
| §3 决策 (SDK) | ✅ 沿用 (`go.lsp.dev/protocol`) | |
| §4 协议版本 + transport | ✅ 沿用 (3.17 + stdio) | |
| §5 能力范围 | ✅ 沿用 (v1.3 hover/definition, v1.4+ completion/diagnostics) | |
| §6 调用 `internal/` 边界 | ⚠️ **被本 RFC §4.4 取代** (新增 `AnchorAtPosition` **取消**, 复用 `SlugsForFile`) | |
| §7 不兼容风险 | ✅ 沿用 | |
| §8 待办 | ⚠️ **被本 RFC §5 修订** (1 / 3 沿用, 4 取消) | |
| §9 备选方案 | ✅ 沿用 | |

> **状态修订**: 本 RFC 拍板后, 旧 RFC `2026-10-07-lsp-sdk.md` 头部 `状态` 字段由 "**SUPERSEDED**" 改为 "**REPLACED (2026-10-08)**, §1.3 / §6 由 `2026-10-08-lsp-client.md` 取代".

---

## 8 反对意见 (AI 已表达 / 已驳回)

| 反对 | 驳回理由 |
|---|---|
| "跨语言 SDK (候选 B) 应下沉到 Kron 仓库" | 拒绝. 与 serve-mcp 跨 AI 客户端的"独立 spawn" 模式不一致; 重复劳动; 0 价值 (各编辑器生态有自己 LSP client 库) |
| "LSP over HTTP (候选 C) 更易部署" | 拒绝. LSP over TCP 不是主流 client 习惯; 增加进程模型复杂度; stdio 简单且足够 |
| "hover 渲染需要 relations.ReverseLinks" | 拒绝. v1.3 hover **只**显示 intent 标题 + summary, **不**显示反向依赖; v1.4+ 评估 |
| "serve-lsp 应支持并发" | 拒绝. v1.3 hover/definition 都是**纯读** + **小文件** + **本地**, 串行延迟 < 5ms. **真不够**再优化 (v1.4+) |
| "新增 `AnchorAtPosition` 是必要的" | 拒绝. `SlugsForFile` 已**足够**; 实际不需要"按行查" 能力 — hover 是按 file 扫, 然后 client 端 line match (LSP client 给的 position) |
