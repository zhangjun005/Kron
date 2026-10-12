# LSP 访问层实现计划（serve-lsp）

> **状态**：**已实现**（v1.3 能力全部落地：initialize / hover / definition / shutdown / exit）
> **负责人**：@HomoFluctus
> **分支**：`feat/serve-lsp`
> **目标版本**：v1.3
> **上游拍板**：[`docs/rfc/2026-10-07-lsp-sdk.md`](../rfc/2026-10-07-lsp-sdk.md)（SDK/协议选型）+ [`docs/rfc/2026-10-08-lsp-client.md`](../rfc/2026-10-08-lsp-client.md)（进程模型/跨语言接入）
> **流程依据**：[`docs/process/new-access-layer.md`](../process/new-access-layer.md)

---

## 1. 这是什么

`kron serve-lsp` 是 Kron 的 **LSP（Language Server Protocol）访问层**，给编辑器（VSCode / Cursor / Neovim）提供：

- **Hover**：鼠标悬停 `// @kron:intent <slug>` 时，弹窗显示该 intent 的标题 + 摘要
- **Definition**：从锚点跳到 `.kron/intents/<slug>.md` 文件

它是四层架构里的「协议访问层」，本质是**把 LSP 协议的 JSON-RPC 翻译成对 `internal/` 的调用**，不承载业务逻辑。

---

## 2. v1.3 范围（做什么 / 不做什么）

### 做（本分支目标）

| LSP method | 数据来源 | 行为 |
|---|---|---|
| `initialize` | 静态 | 报 server name / version / capabilities |
| `textDocument/hover` | `parser` + `store` | hover 锚点 → 显示 intent 标题 + summary |
| `textDocument/definition` | `parser` + `store` | 跳到 `.kron/intents/<slug>.md` 的 file URI |
| `shutdown` / `exit` | 静态 | 正常退出 |

### 不做（v1.4+ 才评估，本分支**不碰**）

- ❌ `textDocument/completion`（slug 补全）
- ❌ `textDocument/publishDiagnostics`（画红线，等价 `kron lint` A-class）
- ❌ HTTP server / socket / daemon
- ❌ 并发（单实例串行）
- ❌ 跨语言 client SDK（VSCode 用 `vscode-languageclient`，各编辑器生态自己实现 client）

---

## 3. 现状盘点（哪些已就绪，不用重做）

| 已就绪 | 位置 | 说明 |
|---|---|---|
| 路由已接 | `cmd/kron/main.go:37-43` | `kron serve-lsp` dispatch 到 `lsp.Run` |
| 协议层已实现 | `cmd/kron/serve-lsp/` | `main.go` / `server.go` / `hover.go` / `definition.go` 全部落地，stub 已替换 |
| 协议 SDK 决策 | RFC 2026-10-07 §3 | `go.lsp.dev/protocol` v1.0.1（实现 LSP 3.18，向后兼容 RFC 拍的 3.17），**已拍板不可回退** |
| 进程模型决策 | RFC 2026-10-08 §4 | stdio + Content-Length 帧，单实例串行，父进程寿命 |
| 业务层 | `internal/parser` + `internal/store` | 100% 完成，要复用的 API 现成 |

---

## 4. 要复用的 `internal/` API（以代码为准，非文档）

```go
// 找单个文件里的锚点（带行号）—— v1.3 新增，解决 hover 行号定位缺口
func parser.AnchorsForFile(file string) ([]model.Anchor, error)
// model.Anchor{ Slug, FilePath, LineNumber, Kind }  ← LineNumber 1 索引

// 按 slug 读 intent
func store.NewReader(root string) (*store.Reader, error)
func (r *store.Reader) Load(ctx context.Context, slug string) (*model.Intent, error)
// model.Intent{ Slug, Frontmatter{...}, Body, SourcePath }
```

> ⚠️ **文档陷阱**（两处，以代码为准）：
> 1. RFC 里写的是 `store.Get`，**实际代码叫 `store.Load`**（`internal/store/reader.go`）。
> 2. RFC 2026-10-08 §4.4 曾判「`parser.SlugsForFile` 够用」，**错**——它返回 `[]string`（丢行号），hover 无法按行匹配。实施时新增了 `AnchorsForFile`（见 §4.4 修订）。

---

## 5. 任务分解（全部完成 ✅）

| 任务 | 内容 | 状态 |
|---|---|---|
| 任务 0 | 行号定位 API 缺口 → 选方案 A（新增 `AnchorsForFile`） | ✅ |
| 任务 1 | 加 LSP SDK 依赖 `go.lsp.dev/protocol` v1.0.1 | ✅ |
| 任务 2 | `internal/parser` 加 `AnchorsForFile` + 单测 | ✅ |
| 任务 3 | 实现协议层，替换 stub | ✅ |
| 任务 4 | 测试：hover Markdown / definition URI / 悬空锚点 null | ✅ |
| 任务 5 | 补协议文档（本文件 + RFC §4.4 修订） | ✅ |

**任务 0 决议**：选方案 A。`parser.SlugsForFile` 返回 `[]string`（丢行号），无法支撑 hover 的行定位；新增 `parser.AnchorsForFile(file) ([]model.Anchor, error)` 返回带 `LineNumber` 的单文件锚点，封装已有的未导出 `scanFile`。已走 `docs/process/new-internal-api.md` 并更新 RFC §4.4。

**任务 1 决议**：`go get go.lsp.dev/protocol@v1.0.1`（连带 `go.lsp.dev/jsonrpc2` + `go.lsp.dev/uri`）。SDK 实际实现 LSP 3.18，向后兼容 RFC 拍的 3.17；hover / definition 在两个版本行为一致。

---

## 6. 关键决策点汇总（已全部拍板 ✅）

| # | 决策 | 结论 |
|---|---|---|
| 1 | 行号定位 | 新增 `parser.AnchorsForFile`（带行号），**不**复用 `SlugsForFile`（丢行号） |
| 2 | `serve-lsp` 的 repo root 来源 | 照抄 `serve-mcp`：可选 `-CWD <root>`，缺省 `os.Getwd()` |
| 3 | `go.lsp.dev/protocol` 依赖批准 | 已批准（RFC 2026-10-07 §3 为权威来源） |

---

## 7. 验收清单（PR 可合并的标准）

- [x] `go build ./...` 通过
- [x] `go vet ./...` 通过；`gofmt -l .` 仅剩 `internal/parser/` 的预存 CRLF 噪声（非本次改动）；`go test ./...` 全部通过
- [x] `serve-lsp` 能完成 initialize → hover → definition → shutdown → exit 全握手
- [x] 悬空锚点不崩、返回 null
- [x] 未 import 任何兄弟访问层 / 未调 `model.WithCaller`
- [x] `docs/implementation/lsp.md` 协议细节已补
- [x] RFC §4.4 的 API 缺口叙述已修正（任务 0 选方案 A）

---

## 8. 协议实现细节

### 8.1 文件布局

```
cmd/kron/serve-lsp/
├── main.go          ← Run(args, in, out, errOut)：resolveRoot → store.NewReader → protocol.NewServer → 阻塞到 conn.Done()
├── server.go        ← server struct + Initialize/Shutdown/Exit + anchorSlugAt 辅助 + anchorFinder 注入点
├── hover.go         ← Hover + renderIntentHover + summarizeBody
├── definition.go    ← Definition
├── mock.go          ← mockAnchorsForFile 占位（等 parser.AnchorsForFile 真版落地后删）
├── server_test.go   ← 7 个测试（hover 3 / definition 2 / initialize / shutdown）
└── hover_test.go    ← summarizeBody 单测（含 UTF-8 截断边界）
```

`server` 内嵌 `protocol.UnimplementedServer`，只覆写 v1.3 五个方法；其余请求返回「not implemented」、通知被忽略（`UnimplementedServer` 契约）。无 package 级可变状态；`Run` 签名与 `serve-mcp.Run` 同形，供 `cmd/kron/main.go` 统一 dispatch。

### 8.2 transport 与进程模型

- `jsonrpc2.NewHeaderStream(&stdioRWC{in, out})`：Content-Length 帧，stdio。
- `stdioRWC` 把 `in`（读）/`out`（写）合成一个 `io.ReadWriteCloser`，`Close()` 是 no-op（父进程拥有底层 OS 流）。
- `protocol.NewServer(ctx, s, stream)` 自动 `conn.Go()`，`<-conn.Done()` 阻塞到父进程关 stdin 或 exit 通知；`conn.Err()` 对干净 EOF 返回 nil。
- 日志走 `errOut`（stderr），stdout 保持纯 LSP 帧。

### 8.3 repo root 解析

`resolveRoot(args)`：空参 → `os.Getwd()`；`-CWD <root>` → `filepath.Abs`；其他 → error。与 `serve-mcp` 同 flag 面，编辑器 spawn 两个子命令方式一致。

### 8.4 hover 渲染（Markdown 模板）

锚点行命中 → `store.Load(slug)` → `renderIntentHover`：

```
**<slug>** · <status>          ← status 为空则不渲染「 · 」后缀

<正文首段，截断至 200 字符，超出加 …>
```

- `summarizeBody`：`TrimSpace` → 取到首个 `\n\n`（第一段）→ 再 `TrimSpace` → 超 200 截断加 `…`。
- 未命中锚点 / 文件读不了 / slug 悬空 → 返回 `(nil, nil)`（空 hover），**不**报协议错误。

### 8.5 definition 跳转 URI

锚点行命中 → `store.Load(slug)` → 返回 `uri.File(intent.SourcePath)` 的 `Location`，Range 为 `{0,0}–{0,0}`（文件顶部）。悬空锚点 → `nil`。

### 8.6 行号换算（关键约定）

- LSP `Position.Line` 是 **0 索引**；`model.Anchor.LineNumber` 是 **1 索引**。
- `anchorSlugAt`：`line := int(td.Position.Line) + 1`，遍历注入的 `s.anchors(filePath)` 返回的锚点，`a.LineNumber == line` 命中。

### 8.7 锚点扫描注入（v1.3 临时态）

`anchorSlugAt` 通过 `anchorFinder` 函数字段注入锚点来源，**不**直接调 `parser.AnchorsForFile`。原因：`AnchorsForFile` 的实现归 internal 层（组长在 master 单独实现），本分支先用 `mockAnchorsForFile`（`mock.go`，返回空）占位，测试注入 `fakeAnchors`。真版落地后，`Run()` 把注入从 `mockAnchorsForFile` 换成 `parser.AnchorsForFile`，并删除 `mock.go`。
