# rfc/archive/ — 已落地 / 已 SUPERSEDED 的 RFC 归档

> 本目录存放**已落地或被取代的 RFC 提案**，仅作历史决策记录，**不再作为决策源**。
>
> 引用方式：可在解释"v1.X 决策背景"时引用，但**当前实施细节以 `docs/abstractDesign/` (事实层) + `docs/implementation/` (实施建议) 为准**。

---

## 文件状态表

| 文件 | 原始状态 | 当前归属 | 关联 commit |
|---|---|---|---|
| [`2026-10-03-frontmatter-references.md`](./2026-10-03-frontmatter-references.md) | 草案 (Proposed) | **已落地 + 已归档** | `a373828` + `ea186a8` |
| [`2026-10-04-mcp-sdk-selection.md`](./2026-10-04-mcp-sdk-selection.md) | 已采纳 (决策记录) | **已落地 + 已归档** (2026-10-09) | `55820ed` |
| [`2026-10-04-mcp-sdk-adoption.md`](./2026-10-04-mcp-sdk-adoption.md) | 已落地 (实施计划) | **已落地 + 已归档** (2026-10-09) | `55820ed` + `36070e1` + `cafb75a/aa560ec/aeb5fc6` |
| [`2026-10-07-gui-stack.md`](./2026-10-07-gui-stack.md) | SUPERSEDED (2026-10-08) | **已被 `2026-10-08-gui-layer` 思路取代 + 已归档** (2026-10-09) | — |
| [`2026-10-08-intent-tree-api.md`](./2026-10-08-intent-tree-api.md) | ACCEPTED (2026-10-08) | **PR-A + PR-B 全部实施 + 已归档** (2026-10-09) | `internal/view/` + `internal/store/paths.go` |
| [`2026-10-08-md-anchors.md`](./2026-10-08-md-anchors.md) | 已拍板 (2026-10-08) | **已落地 + 已归档** (2026-10-09) | `model.AnchorKind` + `parser.ScanMarkdownAnchors` |
| [`2026-10-08-assumptions-standalone.md`](./2026-10-08-assumptions-standalone.md) | 草案 (v1.3 目标) | **6 lint 规则全实 + `internal/assumption/` 包实开 + 已归档** (2026-10-09) | `internal/assumption/` + `internal/lint/assumption_rules.go` |
| [`2026-10-08-writer-readme-symmetry.md`](./2026-10-08-writer-readme-symmetry.md) | DRAFT (2026-10-08) | **PR-A/B/C/D 全实 + 已归档** (2026-10-09) | `model.IntentKind` + `store.paths.resolveWritePath` + `kron add --kind` + MCP `kron_add.kind` |
| [`2026-10-09-markdown-anchors.md`](./2026-10-09-markdown-anchors.md) | 草案 (2026-10-09) | **T1-T5 全实 + 已归档** (2026-10-09) | `model.Anchor.Kind` + `ScanAnchors`/`ScanMarkdownAnchors` 回填 + `tools_schema.AnchorRef.Kind` |
| ~~`2026-10-04-mcp-protocol-redesign.md`~~ | ~~规划中~~ | ~~SUPERSEDED + 二次归档~~ | (2026-10-08 软删入 [`.deprecated/`](../../.deprecated/2026-10-08-doc-cleanup/2026-10-04-mcp-protocol-redesign.md)) |

---

## 引用约定

**允许**：

- ✅ `2026-10-03-frontmatter-references.md` §1.3 的"反面教材"段落——可在讨论"为什么 `references-snapshot.md` 要存在"时引用 (注: `references-snapshot.md` 已 2026-10-08 软删入 [`.deprecated/`](../../.deprecated/2026-10-08-doc-cleanup/references-snapshot.md), 该反面教材论述**仍**有效)
- ✅ `2026-10-04-mcp-protocol-redesign.md` (现位于 `.deprecated/`) §0 的"协议层差距清单"——可在讨论"为什么选 SDK 而不是手写"时引用
- ✅ 留下的文件**作为历史决策的论证底稿**——未来若有人质疑"v1.2 字段怎么来的 / SDK 怎么选"，这两份是答卷

**禁止**：

- ❌ 把本目录的 RFC 作为"待评审提案"使用
- ❌ 在新 RFC 里说"参见本目录 RFC" 而不说明是历史语境
- ❌ 在新实施的 commit message 里只引本目录 RFC（应同时引事实层 / 实施层当前文件）

---

## 仍留根目录的 RFC（与本目录对照）

| 文件 | 状态 | 留根理由 |
|---|---|---|
| [`../2026-10-07-lsp-sdk.md`](../2026-10-07-lsp-sdk.md) | SUPERSEDED（[2026-10-08-lsp-client.md](../2026-10-08-lsp-client.md) 取代, 跨语言 client 端 SDK 选型变更） | §3 / §4 SDK 选型有效; §1.3 + §6 跨语言客户端接入形式**作废** (改走 serve-mcp JSON-RPC + spawn serve-lsp) |
| [`../2026-10-08-lsp-client.md`](../2026-10-08-lsp-client.md) | 草案 (v1.3+ 实施) | v1.3+ 入口 RFC, phase 3 启动时按本 RFC 实施 |
| [`../2026-10-08-mcp-lifecycle.md`](../2026-10-08-mcp-lifecycle.md) | 草案 (v1.0 §2.1 拍板已落 architecture.md, v1.1 §2.2 flock 待实施) | v1.0 拍板 + v1.1 实施入口 |
| [`../2026-10-08-path-conventions.md`](../2026-10-08-path-conventions.md) | DRAFT (2026-10-08, **未实施**) | **RFC §2.1 / §5 引用了不存在的 `view.IntentTreeNode.Path` 字段**（实施时 view 包未加 Path 字段）—— 留根待重开 RFC 重拍, 期间**不**做 §2.1 改动 |

---

## 触发本目录变更的操作

| 操作 | 动作 |
|---|---|
| 新 RFC 落地 | 改原文件顶栏 + `git mv` 进本目录；同步更新本 README 文件状态表 |
| 新 RFC 被取代 | 改原文件顶栏加 SUPERSEDED 横幅 + `git mv` 进本目录；同步更新本 README |
| 目录本身废弃 | 当 archive 内文件因过期失去引用价值时（本 README 与 `docs/rfc/README.md` 同步处置） |
