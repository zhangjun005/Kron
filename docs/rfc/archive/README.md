# rfc/archive/ — 已落地 / 已 SUPERSEDED 的 RFC 归档

> 本目录存放**已落地或被取代的 RFC 提案**，仅作历史决策记录，**不再作为决策源**。
>
> 引用方式：可在解释"v1.X 决策背景"时引用，但**当前实施细节以 `docs/abstractDesign/` (事实层) + `docs/implementation/` (实施建议) 为准**。
>
> 目录入口与活跃 RFC 清单见 [`docs/rfc/README.md`](../README.md)。

---

## 文件状态表

| 文件 | 原始状态 | 当前归属 | 关联 commit |
|---|---|---|---|
| [`2026-10-03-frontmatter-references.md`](./2026-10-03-frontmatter-references.md) | 草案 (Proposed) | **已落地 + 已归档** (2026-10-06) | `a373828` + `ea186a8` |
| [`2026-10-04-mcp-sdk-selection.md`](./2026-10-04-mcp-sdk-selection.md) | 已采纳 (决策记录) | **已落地 + 已归档**（2026-10-09 登记，2026-10-19 实际移入本目录） | `55820ed` |
| [`2026-10-04-mcp-sdk-adoption.md`](./2026-10-04-mcp-sdk-adoption.md) | 已落地 (实施计划) | **已落地 + 已归档**（2026-10-09 登记，2026-10-19 实际移入本目录） | `55820ed` + `36070e1` + `cafb75a/aa560ec/aeb5fc6` |
| [`2026-10-07-gui-stack.md`](./2026-10-07-gui-stack.md) | SUPERSEDED (2026-10-08) | **已被 `2026-10-09-gui-ide-plan` 思路取代 + 已归档** (2026-10-09) | — |
| [`2026-10-07-lsp-sdk.md`](./2026-10-07-lsp-sdk.md) | REPLACED (2026-10-08) | **已落地**（LSP v1.3 实现并合入 master）+ **已归档** (2026-10-19) | `d234e0a` / `7c3c030` / `22348eb` |
| [`2026-10-08-intent-tree-api.md`](./2026-10-08-intent-tree-api.md) | ACCEPTED (2026-10-08) | **PR-A + PR-B 全部实施 + 已归档** (2026-10-09) | `internal/view/` + `internal/store/paths.go` |
| [`2026-10-08-md-anchors.md`](./2026-10-08-md-anchors.md) | 已拍板 (2026-10-08) | **已落地 + 已归档** (2026-10-09) | `model.AnchorKind` + `parser.ScanMarkdownAnchors` |
| [`2026-10-08-assumptions-standalone.md`](./2026-10-08-assumptions-standalone.md) | 草案 (v1.3 目标) | **6 lint 规则全实 + `internal/assumption/` 包实开 + 已归档** (2026-10-09) | `internal/assumption/` + `internal/lint/assumption_rules.go` |
| [`2026-10-08-writer-readme-symmetry.md`](./2026-10-08-writer-readme-symmetry.md) | DRAFT (2026-10-08) | **PR-A/B/C/D 全实 + 已归档** (2026-10-09) | `model.IntentKind` + `store.paths.resolveWritePath` + `kron add --kind` + MCP `kron_add.kind` |
| [`2026-10-08-lsp-client.md`](./2026-10-08-lsp-client.md) | 草案 (2026-10-08, 拍板启动) | **已实施**（客户端层分工已进 `architecture.md` §〇·五·1 + `AGENTS.md` §7；`serve-lsp` 与 `frontend/vscode/` 已合入）+ **已归档** (2026-10-19) | `d234e0a` / `22348eb` |
| [`2026-10-08-mcp-lifecycle.md`](./2026-10-08-mcp-lifecycle.md) | 草案 (Proposed) | **已落地**（stdio 父进程寿命 + per-intent sidecar flock）+ **已归档** (2026-10-19) | `5f8abb2` / `2798717` |
| [`2026-10-09-markdown-anchors.md`](./2026-10-09-markdown-anchors.md) | 草案 (2026-10-09) | **T1-T5 全实 + 已归档** (2026-10-09) | `model.Anchor.Kind` + `ScanAnchors`/`ScanMarkdownAnchors` 回填 + `tools_schema.AnchorRef.Kind` |
| [`2026-10-12-lsp-decisions.md`](./2026-10-12-lsp-decisions.md) | 待组长拍板 | **决策全部落地**（`go.lsp.dev/*` 依赖入 `go.mod`、`parser.AnchorsForFile` 实开）+ **已归档** (2026-10-19)；逐字节重复的 `-decision-points.md` 副本已软删 | `d234e0a` / `4a2781d` / `7c3c030` |
| [`2026-10-14-mcp-prompts.md`](./2026-10-14-mcp-prompts.md) | 已拍板 (2026-10-14) | **已实施**（`kron-implement` / `kron-drift-check` / `kron-review` 三个 prompt）+ **已归档** (2026-10-19) | `9e5e22b` |
| [`2026-10-14-projects-json-schema-extension.md`](./2026-10-14-projects-json-schema-extension.md) | 已拍板 (2026-10-14) | **已实施**（`projects.json` 精简版 schema + `current_project` 接入）+ **已归档** (2026-10-19) | `33de13e` / `3cfc7d8` / `84cf93c` / `c4ba92a` |
| [`2026-10-14-pr-body-projects-json.md`](./2026-10-14-pr-body-projects-json.md) | PR body 存档 | **已归档** (2026-10-19) | `33de13e`..`c4ba92a` |
| ~~`2026-10-04-mcp-protocol-redesign.md`~~ | ~~规划中~~ | ~~SUPERSEDED + 二次归档~~ | (2026-10-08 软删入 [`.deprecated/`](../../../.deprecated/2026-10-08-doc-cleanup/2026-10-04-mcp-protocol-redesign.md)) |
| ~~`2026-10-12-lsp-decision-points.md`~~ | ~~重复副本~~ | ~~与 `-lsp-decisions.md` 逐字节相同~~ | (2026-10-19 软删入 [`.deprecated/`](../../../.deprecated/2026-10-19-rfc-cleanup/)) |
| ~~`2026-10-16-cli-soft-delete.md`~~ | ~~Accepted~~ | ~~实施已随合并回退，决定作废~~ | (2026-10-19 软删入 [`.deprecated/`](../../../.deprecated/2026-10-19-rfc-cleanup/)) |

---

## 引用约定

**允许**：

- ✅ `2026-10-03-frontmatter-references.md` §1.3 的"反面教材"段落——可在讨论"为什么 `references-snapshot.md` 要存在"时引用 (注: `references-snapshot.md` 已 2026-10-08 软删入 [`.deprecated/`](../../../.deprecated/2026-10-08-doc-cleanup/references-snapshot.md), 该反面教材论述**仍**有效)
- ✅ `2026-10-04-mcp-protocol-redesign.md` (现位于 `.deprecated/`) §0 的"协议层差距清单"——可在讨论"为什么选 SDK 而不是手写"时引用
- ✅ 留下的文件**作为历史决策的论证底稿**——未来若有人质疑"v1.2 字段怎么来的 / SDK 怎么选"，这些是答卷

**禁止**：

- ❌ 把本目录的 RFC 作为"待评审提案"使用
- ❌ 在新 RFC 里说"参见本目录 RFC" 而不说明是历史语境
- ❌ 在新实施的 commit message 里只引本目录 RFC（应同时引事实层 / 实施层当前文件）

---

## 仍留根目录的 RFC（与本目录对照）

活跃 RFC 的状态与待办见 [`docs/rfc/README.md`](../README.md)。当前根目录留下 8 份：

| 文件 | 状态 | 留根理由 |
|---|---|---|
| [`../2026-10-08-path-conventions.md`](../2026-10-08-path-conventions.md) | DRAFT，**待重开** | §2.1 / §5 引用了不存在的 `view.IntentTreeNode.Path`，需重拍后再实施 |
| [`../2026-10-09-gui-ide-plan.md`](../2026-10-09-gui-ide-plan.md) | 草案（拍板启动） | IDE 侧已按本文落地（VSCode）；Wails GUI 未开工，仍作 GUI 决策入口 |
| [`../2026-10-10-frontend-ui-stack.md`](../2026-10-10-frontend-ui-stack.md) | 草稿 | 前端技术栈，等 Wails GUI 启动 |
| [`../2026-10-12-close-b3-softening.md`](../2026-10-12-close-b3-softening.md) | DRAFT | inline `text` 兜底仍在，等拍板后收口 |
| [`../2026-10-13-gui-distribution-manual-install.md`](../2026-10-13-gui-distribution-manual-install.md) | 草案（拍板启动） | 发行方式，等 GUI |
| [`../2026-10-13-multi-project-registry.md`](../2026-10-13-multi-project-registry.md) | 草案（registry 已实施） | 仍描述 `projects.json` 语义；§2.2 schema 已由 `2026-10-14-projects-json-schema-extension` 取代，并发写未处理 |
| [`../2026-10-15-mcp-coerce-middleware.md`](../2026-10-15-mcp-coerce-middleware.md) | **待拍板（实现已入库）** | 拍板未闭环，不能进归档 |
| [`../2026-10-15-mcp-test-expansion.md`](../2026-10-15-mcp-test-expansion.md) | **待拍板** | 15 项 MCP 覆盖缺口，拍板后再写测试 |

---

## 触发本目录变更的操作

| 操作 | 动作 |
|---|---|
| 新 RFC 落地 | 改原文件顶栏 + `git mv` 进本目录；同步更新本 README 文件状态表 + `docs/rfc/README.md` |
| 新 RFC 被取代 | 改原文件顶栏加 SUPERSEDED 横幅 + `git mv` 进本目录；同步更新本 README |
| RFC 被废弃 / 重复 / 决定作废 | **不**进本目录：移入 `.deprecated/<date>-<reason>/` 并 `git rm --cached`（软删，见 `.gitignore`） |
| 目录本身废弃 | 当 archive 内文件因过期失去引用价值时（本 README 与 `docs/rfc/README.md` 同步处置） |
