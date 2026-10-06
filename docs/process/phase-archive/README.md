# phase-archive/ — 历史阶段遗留事项归档

> 本目录存放**已 closed phase 的遗留事项文档**，仅作历史档案，**不再引导开发**。
>
> 引用方式：仅在讲"某 phase 当时解决了什么 / 没解决什么"的历史语境下引用，不作为"待办清单"使用。

---

## 目录约定

- 文件命名沿用原 `phase-N-leftovers.md`，便于 `git log --follow` 追溯。
- **移动而非删除**：本目录与根 `process/` 平级，保留原文件主体，仅加状态横幅。
- 新 phase 启动时：旧 phase 文档**必须**迁入本目录，不得留在根 `process/`。

---

## 文件状态表

| 文件 | phase | closed 时间 | 关联 commit | 当前定位 |
|---|---|---|---|---|
| [`phase-1-leftovers.md`](./phase-1-leftovers.md) | Phase 1（2026-09-27 docs-only） | 2026-10-03 复核 | `ae0d244` + `88b36c5` + `7a7db14` + `487ca22` | **历史归档**。L1 + L3-CLI done，L3-MCP 移交 phase 2，L2 / L4 按当时判断处置 |
| [`phase-2-leftovers.md`](./phase-2-leftovers.md) | Phase 2（v1.1 MCP server 落地） | 2026-10-03 全 closed | `5ad69e1` + `8e04914` + `1033890` + `2c8975f` | **历史归档**。L1 / L2 / L4 / L5 done，L3（`view-call-tree-intent.md` demo）deferred 到 phase 3 启动时再 review |

---

## 引用约定

**禁止**：

- ❌ 把 `phase-archive/` 下的文件作为"待办清单"使用
- ❌ 在新 RFC 里说"参见 phase-N-leftovers.md" 而不说明是历史语境
- ❌ 在 `process/README.md` 主表里把本目录文件列为"流程文档"

**允许**：

- ✅ `phase-1-leftovers.md` §1.3 的"反面教材"段落——可在讨论"为什么 schema 要严格"时引用
- ✅ `phase-2-leftovers.md` §L4 的"stdio vs LSP-style framing 混淆"复盘——可在 MCP 协议讨论时引用
- ✅ `phase-2-leftovers.md` §L3 的 "phase 3 启动条件"段落——可在 phase 3 规划时引用

---

## 触发本目录变更的操作

| 操作 | 动作 |
|---|---|
| 新 phase 启动 | 旧 phase 文档**必须**迁入本目录（见 `process/README.md` 主表引用更新） |
| phase 状态变化（任何一项 done/deferred/blocked） | 修改本目录文件**主体**（状态横幅与原文件）；不在本 README 里同步状态横幅（README 是目录级，不重复 phase 内部） |
| 目录本身废弃 | 当 phase 1 + phase 2 文档因过期失去引用价值时（本目录 README 与 process 主 README 同步处置） |
