# Implementation Reference（已归档）

> ⚠️ **本文档目录已归档**（2026-10-09）。8 份实施参考文件因描述与 `internal/` 当前实现
> 严重错位（签名错、字段漏、规则名错），全部移到 `.deprecated/2026-10-09-process-cleanup/`。
> 重新生成 `internal/` 实施参考是后续 PR 的范畴（按 `docs/process/new-internal-api.md` 流程）。
>
> 架构真理位于 [`docs/abstractDesign/architecture.md`](../abstractDesign/architecture.md)。  
> 实施参考落地页（待重写）：

| 原文件 | 归档位置 |
|---|---|
| `api-surface.md` | `.deprecated/2026-10-09-process-cleanup/api-surface.md` |
| `cli.md` | `.deprecated/2026-10-09-process-cleanup/cli.md` |
| `domain-model.md` | `.deprecated/2026-10-09-process-cleanup/domain-model.md` |
| `error-catalog.md` | `.deprecated/2026-10-09-process-cleanup/error-catalog.md` |
| `ide-interaction.md` | `.deprecated/2026-10-09-process-cleanup/ide-interaction.md` |
| `lsp.md` | `.deprecated/2026-10-09-process-cleanup/lsp.md` |
| `mcp.md` | `.deprecated/2026-10-09-process-cleanup/mcp.md` |
| `testing.md` | `.deprecated/2026-10-09-process-cleanup/testing.md` |

## 为什么归档

具体错位清单（对账日期 2026-10-09）：

- `api-surface.md` — 列了 `WriteIntent(ctx, slug, intent)` 但实际是 `Writer.Write(ctx, *Intent)`；
  列了 `ParseSlug(raw string) (string, error)` 但实际是 `ValidateSlug(slug string) error`；
  列了 `ScanAnchors(filePath, src io.Reader)` 但实际是 `ScanAnchors(dir string) ([]Anchor, error)`；
  列了 `LintResult` / `LintError` 但实际是 `Diag` / `Severity` / `Rule`。
- `domain-model.md` — 缺 `model.AnchorKind` 类型 + 2 const + 3 方法；缺 `model.Anchor.Kind` 字段；
  `Frontmatter.UpdatedAt` 写 `time.Time` 但 `AssumptionFrontmatter.UpdatedAt` 是 `string`（双标注不一致）；
  缺 `model.Severity` + `model.IntentKind` 类型；`Status` 校验位置写错（实际在 parser 层）。
- `error-catalog.md` — 缺 `ErrConcurrentWrite` / `ErrEmptyPatch` / `ErrCreatorChangeNotAllowed` /
  `ErrSlugInvalid`（实为 `model.ErrSlugInvalid`）/
  `ErrAssumptionNotFound` / `ErrAssumptionExists` / `ErrAssumptionAlreadyDeleted` /
  `ErrAssumptionNotInTrash` / `ErrConfigInvalid`（实为 store 包而非 model）。
- `cli.md` — `kron lint` 流程图说"调 `store.ResolveIntent(slug)`"但实际走
  `lint.Run(ctx, root)` → `parser.ScanAnchors` + `store.Reader.Exists` 链；
  `kron add` 流程图说"调 `store.WriteIntent`"但实际是 `Writer.Write`。
- `mcp.md` — `kron_get` 出参表说 `references` 是 `v1.2+` 但 frontmatter schema 自 v1 已含；
  工具列表里 `kron_stale` 默认 90 天的描述写在 `lint.StaleDaysDefault` 之外无引用。
- `ide-interaction.md` — `textDocument/publishDiagnostics` 标 `⏳ v1.4+` 但 `kron_lint` A-class
  已可发（`lint.Run` 输出 + 客户端渲染），缺位更准。
- `lsp.md` — 全文件是 v1.3 拍板下的实施假设；`serve-lsp` 实际**未**实施（`cmd/kron/serve-lsp/`
  目录不存在）；本文件描述的 hover/definition handler 是设计稿，不是落地代码。
- `testing.md` — `os.Chdir(tmp)` 写法在 Go 1.27 上不阻塞但与 testify best practice 冲突；
  CI 门禁表里漏 `go vet ./...`（architecture.md 铁律 #4）。

## 何时重写

任何新 `internal/` API、新 MCP 工具、新 lint 规则落地时，**先**改 `internal/*` 代码 + 测，
**然后**按需重写 `docs/implementation/<topic>.md`（不走本 README——本 README 只起"已归档"提示作用）。

## 客户端层与 LSP 协议访问层（v1.3+ 待重写）

| 文档 | 状态 | 触发条件 |
|---|---|---|
| `lsp.md` (LSP 协议实现细节) | 🔜 v1.3 待写 | `cmd/kron/serve-lsp/` 实施时（按 `docs/process/new-access-layer.md` §3 交付物 2 + 5）|
| `vscode-extension.md` (VSCode 扩展) | 📦 内部 (`frontend/vscode/`) | 2026-10-09 修订：客户端层进主仓 `frontend/` |
| `gui.md` / `wails.md` (Wails GUI) | 📦 内部 (`frontend/wails/`) | 2026-10-09 修订：`frontend/` 不再是纯占位，实施代码在 `frontend/wails/` |
| `mcp.md` (MCP 协议实现细节) | 🔜 待重写 | 与 `lsp.md` 同批——MCP 工具加到 13+ 时需要重写（见 v1.3+ `kron_assumption_*` 工具候选）|
