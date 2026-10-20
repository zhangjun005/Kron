# RFC 索引

> **RFC 的生命周期只有三步**：提案（留本目录根）→ 拍板并实施 → 落地后移入 [`archive/`](./archive/README.md)。
> 被废弃、重复、或决定作废的 RFC **不**进归档，软删到 [`.deprecated/`](../../.deprecated/)（`.gitignore` 排除）。
>
> **状态真相只有一个地方**：每份 RFC 顶栏的 `状态` 字段 + 本索引的"现在等什么"。
> 其他文档（`docs/implementation/`、`docs/process/`、`docs/abstractDesign/`）只引用 RFC，**不重复声明 RFC 状态**。

---

## 活跃 RFC（仍在根目录）

| RFC | 状态 | 现在等什么 |
|---|---|---|
| [`2026-10-08-path-conventions.md`](./2026-10-08-path-conventions.md) | DRAFT (2026-10-08) + 待重开 | §2.1 / §5 引用了不存在的 `view.IntentTreeNode.Path`；重拍前**不**做 §2.1 改动 |
| [`2026-10-09-gui-ide-plan.md`](./2026-10-09-gui-ide-plan.md) | 草案（2026-10-09 拍板启动） | IDE 侧已按本文落地；Wails GUI 未开工 |
| [`2026-10-10-frontend-ui-stack.md`](./2026-10-10-frontend-ui-stack.md) | 草稿 | 前端技术栈细节，等 Wails GUI 启动 |
| [`2026-10-12-close-b3-softening.md`](./2026-10-12-close-b3-softening.md) | DRAFT (2026-10-12) | inline `text` 兜底仍在；拍板后做硬收口 |
| [`2026-10-13-gui-distribution-manual-install.md`](./2026-10-13-gui-distribution-manual-install.md) | 草案（2026-10-13 拍板启动） | GUI 发行方式，等 GUI |
| [`2026-10-13-multi-project-registry.md`](./2026-10-13-multi-project-registry.md) | 草案（2026-10-13 拍板启动；registry 已实施） | §2.2 schema 已由 `2026-10-14-projects-json-schema-extension` 拍板取代；`projects.json` 并发写仍未处理 |
| [`2026-10-15-mcp-coerce-middleware.md`](./2026-10-15-mcp-coerce-middleware.md) | **待拍板（实现已入库）** | 中间件已随 `91ade43` 落地，但拍板未闭环 |
| [`2026-10-15-mcp-test-expansion.md`](./2026-10-15-mcp-test-expansion.md) | **待拍板** | 15 项 MCP 覆盖缺口（G0 fixture 空仓库是根因），拍板后另开 PR 写测试 |

---

## 已归档（历史决策记录，不再作决策源）

全部清单见 [`archive/README.md`](./archive/README.md)。最近一次归档（2026-10-19）移入 9 份：
`2026-10-04-mcp-sdk-{selection,adoption}` / `2026-10-07-lsp-sdk` / `2026-10-08-{lsp-client,mcp-lifecycle}` /
`2026-10-12-lsp-decisions` / `2026-10-14-{mcp-prompts,projects-json-schema-extension,pr-body-projects-json}`。

## 已软删（`.deprecated/`）

| 文件 | 原因 | 位置 |
|---|---|---|
| `2026-10-12-lsp-decision-points.md` | 与 `2026-10-12-lsp-decisions.md` 逐字节相同的重复副本 | [`.deprecated/2026-10-19-rfc-cleanup/`](../../.deprecated/2026-10-19-rfc-cleanup/) |
| `2026-10-16-cli-soft-delete.md` | 决定被作废：CLI `trash` / `restore` 违反架构 §1.1 最小命令集，实施已回退 | 同上 |

---

## 处置流程

| 情形 | 动作 |
|---|---|
| 新提案 | 在根目录新建 `YYYY-MM-DD-<slug>.md`，顶栏写状态；在本文"活跃 RFC"表登记 |
| 拍板 + 实施完成 | 改 RFC 顶栏为已落地 → `git mv` 到 [`archive/`](./archive/README.md) → 同步 `archive/README.md` 与本文件 |
| 被新 RFC 取代 | 顶栏加 SUPERSEDED 横幅并写明取代者 → 移入 `archive/` |
| 作废 / 重复 / 决定被推翻 | 移入 `.deprecated/<date>-<reason>/` 并 `git rm --cached`；在本文"已软删"表登记 |
