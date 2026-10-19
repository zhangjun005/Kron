# LSP Access Layer

> Source of truth (architecture): [`docs/abstractDesign/architecture.md`](../abstractDesign/architecture.md) §〇 铁律 + §一 访问层
> Source of truth (tech selection): [`docs/rfc/2026-10-07-lsp-sdk.md`](../rfc/2026-10-07-lsp-sdk.md)
> Source of truth (client layer boundary): [`docs/rfc/2026-10-08-lsp-client.md`](../rfc/2026-10-08-lsp-client.md)
> Source of truth (caller-injection change): [`docs/tracking/homofluctus.md`](../tracking/homofluctus.md) 10-12 决策点

This document is the LSP **access layer** specification. The Go entry point is `cmd/kron/serve-lsp/` (package `lsp`); the package implements the LSP server-side protocol over stdio.

---

## 1 能力清单 (capabilities v1.3)

The server registers 5 LSP methods. All other methods are explicitly "not implemented" by embedding `protocol.UnimplementedServer`.

| Method | Direction | Capability advertised in `Initialize` |
|---|---|---|
| `initialize` | request | n/a (handshake) |
| `initialized` | notification | (no-op, required by spec) |
| `shutdown` | request | n/a |
| `exit` | notification | n/a |
| `textDocument/didOpen` | notification | `TextDocumentSync.OpenClose = true` |
| `textDocument/didChange` | notification | `TextDocumentSync.Change = Full` |
| `textDocument/didClose` | notification | `TextDocumentSync.OpenClose = true` |
| `textDocument/hover` | request | `HoverProvider = true` |
| `textDocument/definition` | request | `DefinitionProvider = true` |

### 1.1 v1.4+ 评估 (NOT v1.3 scope)

| Method | Status | Why deferred |
|---|---|---|
| `textDocument/completion` | v1.4+ 评估 | Need `internal/parser.ValidateSlug` caching |
| `textDocument/publishDiagnostics` | v1.4+ 评估 | Need `internal/lint` integration |
| `workspace/diagnostic` | v1.4+ 评估 | 依赖 publishDiagnostics |
| `textDocument/inlineCompletion` | v1.4+ 评估 | 依赖 completion |
| `textDocument/signatureHelp` | v1.4+ 评估 | 依赖 symbol 前置 |
| `workspace/symbol` | v1.4+ 评估 | 依赖 symbol 索引 |

---

## 2 工具契约 (request/response shape per method)

### 2.1 `initialize`

**Request** (`InitializeParams`): standard LSP.

**Response** (`InitializeResult`):

```go
&protocol.InitializeResult{
    Capabilities: protocol.ServerCapabilities{
        TextDocumentSync: &protocol.TextDocumentSyncOptions{
            OpenClose: ptr(true),
            Change:    ptr(TextDocumentSyncKindFull),
        },
        HoverProvider:      Boolean(true),
        DefinitionProvider: Boolean(true),
    },
    ServerInfo: protocol.ServerInfo{
        Name:    "kron",
        Version: NewOptional("v1.3.0"),
    },
}
```

### 2.2 `textDocument/hover`

**Request** (`HoverParams`): standard LSP `TextDocumentPositionParams` (URI + 0-indexed line/character).

**Resolution**: server scans `<URI.FsPath>` via `parser.AnchorsForFile`, matches the 0-indexed line + 1 to the parser's 1-indexed `Anchor.LineNumber`, loads the intent via `store.Reader.Load(ctx, slug)`, and renders a Markdown `MarkupContent` with the slug (and status, if set) as heading + the first paragraph of the body (truncated to 200 runes + "…" on a UTF-8 boundary).

**Response**:
- `(*protocol.Hover, nil)` — anchor exists, intent loads, returns `MarkupContent{Kind: Markdown, Value: "**slug** · status\n\nbody…"}`.
- `(nil, nil)` — any of: not on an anchor line, file unreadable, slug dangles (`ErrIntentNotFound`), or any other load failure (logged via `window/logMessage`).

**Dangling anchor contract** (RFC §6): the editor sees no hover and no error. This is the "degrade to no content, never to protocol failure" guarantee.

### 2.3 `textDocument/definition`

**Request** (`DefinitionParams`): same as hover.

**Resolution**: same anchor match + intent load, then return a `LocationSlice` with one `Location` whose URI is the absolute path of the intent's `.md` file (from `intent.SourcePath`) and whose Range is `{Line: 0, Character: 0}` to `{Line: 0, Character: 0}` (top of the file).

**Response**:
- `protocol.LocationSlice{[location]}` — anchor exists, intent loads, jumps to the `.md`.
- `(nil, nil)` — same failure modes as hover.

### 2.4 `textDocument/didOpen` / `didChange` / `didClose`

v1.3 takes no action on these notifications — the server re-reads the file from disk on every hover/definition request (no in-memory cache). The methods are present on the server (vs. left to `UnimplementedServer`) so the editor sees the capabilities the `Initialize` result declared are actually being served. Without this, editors like VSCode and Neovim disconnect after the first timeout when the server declares a capability and silently ignores the corresponding notifications.

### 2.5 `shutdown` / `exit`

- `shutdown`: returns `nil`; the SDK keeps the connection open until `exit` arrives.
- `exit`: invokes `s.exitFn(0)` (LSP §3.5.4 mandates exit code 0 after a clean `shutdown` → `exit` sequence). The default `s.exitFn` is `os.Exit` (wired in `Run()`); tests inject a recorder to assert on the call.

---

## 3 底层实现

### 3.1 包边界

`cmd/kron/serve-lsp/` imports only:
- `internal/parser` (only `parser.AnchorsForFile`)
- `internal/store` (only `store.NewReader` and `store.Reader.Load`)
- `internal/model` (only `model.Anchor`, `model.Intent`, `model.ErrIntentNotFound`, `model.AnchorKind`)

It does NOT import any other access layer (`cmd/kron/cli`, `cmd/kron/serve-mcp`) per architecture.md §〇 铁律 #2. It does NOT import LSP-client concerns (those live in `frontend/extensions/vscode/` or `frontend/wails/` per the client-layer split, RFC 2026-10-08-lsp-client).

### 3.2 数据流 (hover)

```
Editor (VSCode / Cursor / Neovim)
  │  Content-Length framed JSON-RPC
  │  textDocument/hover { textDocument: {uri}, position: {line, character} }
  ▼
jsonrpc2 stream
  │
  ▼
protocol.ServerHandler dispatches to server.Hover
  │
  │  anchorSlugAt(ctx, &params.TextDocumentPositionParams):
  │    1. parser.AnchorsForFile(ctx, td.TextDocument.URI.FsPath())
  │       → returns []model.Anchor (each with Slug, FilePath, LineNumber, Kind)
  │    2. for each anchor, if a.LineNumber == td.Position.Line + 1
  │       → return (a.Slug, true)
  │    3. else return ("", false)
  │
  │  if !ok: return (nil, nil)
  │
  │  store.Reader.Load(ctx, slug):
  │    → *model.Intent (with Frontmatter, Body, SourcePath)
  │    → or error (ErrIntentNotFound / ErrFrontmatterInvalid / IO)
  │
  │  if errors.Is(err, model.ErrIntentNotFound): return (nil, nil)  // dangling
  │  if err != nil:
  │     s.logf(ctx, MessageTypeError, "kron serve-lsp: hover: load %s: %v", slug, err)
  │     return (nil, nil)
  │
  │  renderIntentHover(intent) → Markdown string
  │    summary = summarizeBody(intent.Body) (first paragraph, ≤ 200 runes, "…" on overflow)
  │    return &Hover{Contents: &MarkupContent{Kind: Markdown, Value: "**slug** · status\n\nsummary"}}
  │
  ▼
Editor displays the hover popup.
```

### 3.3 caller identity

**This layer does NOT call `model.WithCaller`.** Per [`internal/model/caller.go`](../../internal/model/caller.go) head comment (2026-10-08 change) and the 10-12 decision point in [`docs/tracking/homofluctus.md`](../tracking/homofluctus.md), the caller-injection API is preserved for backwards compatibility but the `internal/` packages no longer branch on ctx caller keys. New access layer code therefore does not inject caller.

### 3.4 ctx propagation

Every function on the server (`Hover`, `Definition`, `anchorSlugAt`, `DefaultAnchorsForFile`) takes `ctx context.Context` first, per architecture.md §〇 铁律 #3 (every store/parser fn takes ctx first). `Run()` passes `context.Background()` today because `jsonrpc2` does not yet expose per-request cancellation hooks to the server side; v1.4+ may forward the request context once the SDK exposes a stable typed accessor.

### 3.5 logf / notify / exitFn injection

The server has three side-channel fields that `Run()` wires in production:

| Field | Production wire (in Run) | Test wire |
|---|---|---|
| `s.logf` | `Client.LogMessage(ctx, &LogMessageParams{Type, Message})` with `Client` from `protocol.ClientFromContext`; falls back to `errOut` if Client is unavailable | a recorder closure; asserts on the message |
| `s.notify` | `Client.ShowMessage(ctx, &ShowMessageParams{Type, Message})` with same Client lookup; falls back to `errOut` | (no v1.3 test) |
| `s.exitFn` | `os.Exit` | a recorder closure; asserts on the code |

This three-field shape mirrors the design rationale in [`cmd/kron/serve-mcp/serve_mcp.go`](../../cmd/kron/serve-mcp/serve_mcp.go) (the MCP layer has the same injection pattern, adapted to `mcp.Middleware`).

---

## 4 测试覆盖

| Method | Test | Layer |
|---|---|---|
| (入口) | `TestRun_EmptyStdin` | stdio 入口 (mirrors `servemcp.TestRun_EmptyStdin`) |
| (入口) | `TestRun_UnknownArgs` | `resolveRoot` rejects unknown flags |
| `initialize` | `TestInitialize_ReportsCapabilities` | base capabilities |
| `initialize` | `TestInitialize_DeclaresTextDocumentSync` | TextDocumentSync opts (P0-2) |
| `textDocument/didOpen` | `TestDidOpen_NoOp` | handler presence |
| `textDocument/didChange` | `TestDidChange_NoOp` | handler presence |
| `textDocument/didClose` | `TestDidClose_NoOp` | handler presence |
| `textDocument/hover` | `TestHover_OnAnchor` | fakeAnchors path |
| `textDocument/hover` | `TestHover_NotOnAnchor` | off-anchor → nil |
| `textDocument/hover` | `TestHover_DanglingAnchor` | dangle → nil, no error |
| `textDocument/hover` | `TestHover_NonNotFoundError_LogsAndReturnsNil` | frontmatter corruption → logf fires, h = nil |
| `textDocument/hover` | `TestHover_RealParserEndToEnd` | real parser + store + render path |
| `textDocument/hover` | `TestHover_RealParser_NotOnAnchor` | real parser off-anchor |
| `textDocument/hover` | `TestHover_LongBodyTruncated` | 300-CJK body → "…" suffix, UTF-8 valid |
| `textDocument/hover` | `TestHover_RepeatedInvocation` | 1000× stability (P2-4) |
| `textDocument/definition` | `TestDefinition_OnAnchor` | fakeAnchors path |
| `textDocument/definition` | `TestDefinition_DanglingAnchor` | dangle → nil |
| `textDocument/definition` | `TestDefinition_NonNotFoundError_LogsAndReturnsNil` | frontmatter corruption |
| `textDocument/definition` | `TestDefinition_RealParserEndToEnd` | real parser + store |
| `shutdown` | `TestShutdown_NoError` | clean shutdown |
| `exit` | `TestExit_NotifiesZero` | `exitFn(0)` invoked |
| `exit` | `TestExit_NoExitFn_NoError` | zero-value `exitFn` does not panic |
| (internal) | `TestSummarizeBody_*` (5 tests in `hover_test.go`) | helper unit tests |

**Total: 28 tests** as of v1.3 (vs. 8 at v1.3 stub). All pass under `go test ./cmd/kron/serve-lsp/...`.

### 4.1 Test fixtures

`server_test.go` uses `writeTestFile(t, root, rel, content)` to build a fresh repo in `t.TempDir()` per test. The two factory functions are:

- `newTestServer(t)` — builds a `&server{reader, anchors: fakeAnchors}` with a `main.go` anchoring `auth-jwt` on line 3 and a `dangling.go` anchoring `no-such-intent` on line 1.
- `newTestServerWithRealParser(t)` — same layout but uses `parser.AnchorsForFile` as the anchor scanner (currently redirects through the `defaultAnchorsForFile` adapter).

Direct struct construction (`&server{...}`) is used for tests that need a custom `anchorFinder` or `logf` recorder.

---

## 5 RFC 链接 + 决策点

- **SDK selection**: [`docs/rfc/2026-10-07-lsp-sdk.md`](../rfc/2026-10-07-lsp-sdk.md) — `go.lsp.dev/protocol v1.0.1` + `go.lsp.dev/jsonrpc2 v1.0.1` + `go.lsp.dev/uri v1.0.1`.
- **Client layer split**: [`docs/rfc/2026-10-08-lsp-client.md`](../rfc/2026-10-08-lsp-client.md) — VSCode extension (lives in `frontend/extensions/vscode/`) consumes `kron serve-lsp` and `kron serve-mcp` as stdio subprocesses. This server does not import any client-layer concern.
- **caller-injection change**: [`docs/tracking/homofluctus.md`](../tracking/homofluctus.md) 10-12 决策点 §7 — new access layer code does not call `model.WithCaller`; `internal/` does not read ctx caller.
- **Dep approval status**: `go.lsp.dev/protocol` and friends are listed in `go.mod` but flagged in [`docs/process/pending-decisions.md`](../process/pending-decisions.md) (P2-3) as awaiting explicit human approval per AGENTS.md §3 red line. Removal is NOT planned — the PR keeps the deps and the marker; reverting costs more than the marker.

## 6 开发边界

本文件说明 `cmd/kron/serve-lsp/` 的职责，不替代新增依赖和 `internal/` API 所需的审批流程。架构边界以 [`architecture.md`](../abstractDesign/architecture.md) 为准，锚点语法以 [`intent-structure.md`](../abstractDesign/intent-structure.md) 为准。

- LSP 只读取 Kron 数据并返回编辑器协议结果；不创建或编辑 intent、assumption 或锚点，不维护项目索引，也不调用 `serve-mcp`。
- 每个编辑器 workspace 启动一个 `kron serve-lsp -CWD <workspace-root>`；stdin/stdout 使用 LSP Content-Length JSON-RPC，编辑器退出即进程退出。
- LSP 与 MCP 是兄弟访问层，不得互相 import、调用或通过内存共享状态连接。
- LSP 只调用所需的 `internal/parser` 和 `internal/store` 能力；业务逻辑留在 `internal/`。
- `context.Context` 只传递取消和 deadline；不得通过 caller 注入行为身份。
- 新增 Go 顶层依赖仍须显式批准；不得手写协议帧来规避依赖审批。
