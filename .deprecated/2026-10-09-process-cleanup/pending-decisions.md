# Kron 待决策清单 (2026-10-07 封档)

> **本文档是历史快照** — 封档时 (2026-10-07) 所有未拍板信号集中收口.  
> 后续 chat 已**全部执行完毕** (见末尾 §"执行完成 2026-10-07"), 标记为本批完成的项**不**再是"待决策".  
> 本文件**不**删, 作为决策链路的审计日志保留. 后续 chat **不**应再增项于此文件, 应直接打开对应流程文档 (`new-access-layer.md` / `new-internal-api.md` / 2 份 RFC / `github-branch-protection.md`).

---

## 全部未拍板信号 (10 项 chat 信号 + 3 项发现)

### 来自 chat #1 (本次, "关于 GUI/LSP/MCP 的进一步开发")

| # | 类型 | 状态 | 内容 | 卡点 |
|---|---|---|---|---|
| S1 | 决策 | 部分 | GUI = **Go Wails + shadcn (TS 组件库)** | AGENTS.md "TypeScript only when GUI phase begins" 不够 — wails 模式下 TS **前端** + Go **后端** 都是 v1+ |
| S2 | 决策 | ✅ 已定 | LSP = `go.lsp.dev/protocol` (上次评估推荐的) | 写 RFC 锁定 |
| S3 | 流程约束 | 未决 | **MCP 进一步开发**须先做 internal 扩充 — 在 master 做完再放, 避免"不一致版"被其他开发基于 | 文档化流程; 评估哪些 MCP 增强对应哪些 internal API |
| S4 | 流程约束 | 未决 | "给其他开发做规定" — skill / mcpTools / PR 规矩 / 文档纪律 | 见 §"流程文档规划" |
| S5 | GitHub 配置 | 未决 | **禁止非 zhangjun005 合并 master** — branch protection / CODEOWNERS / PR template | 配置在 GitHub 端; 仓库内要有支持文件 |
| S6 | 流程纪律 | 未决 | "**必须写文档再交付**" | **改向**: 落地到 `docs/process/new-access-layer.md` "PR 流程" 段, **不**进 skill |
| S7 | 新 skill 占位 | ❌ **已废** | (S6 改向后, skill 不再必要) | 用户 2026-10-07 判断: AGENTS.md 已够, **不**开新 skill |
| S8 | 隐含改动 | 推导 | AGENTS.md "TypeScript only when GUI phase begins" 要更新 (S1 的连锁) | 文字更新 |
| S9 | 隐含改动 | 推导 | S5 在仓库内需要 `CODEOWNERS` / `MAINTAINERS.md` / PR template | 文件创建 |
| S10 | 多视角拼接 | 推导 | S4 + S6 + S7 实际上是同一份流程的不同章节 | 合并写 |

### 来自 chat 内"我评估时发现" (不在用户信号里, 但绕不过)

| # | 内容 | 必须联动改的文件 |
|---|---|---|
| F1 | AGENTS.md "off-limits" §"Implementing serve-lsp、IDE plugin、GUI、hard-delete / GC — all Phase 2" — 现在要做 GUI/LSP/MCP, 这条**必须改** | AGENTS.md + `.cursor/rules/project-conventions.mdc` |
| F2 | `project-conventions.mdc` "Repo layout" 段没列 `cmd/kron/serve-lsp` / `serve-gui` 入口 — 现在要加 | `.cursor/rules/project-conventions.mdc` |
| F3 | AGENTS.md §8 "AGENTS.md / `.cursor/rules/*` 是本文档的镜像" — 改 architecture.md 必须回写 AGENTS.md 和 rules | 流程纪律 (S6) 直接相关 |

---

## 流程文档规划 (回应 S4 / S6 / S10)

**已写** (2026-10-07 完成):
- [`docs/process/new-access-layer.md`](new-access-layer.md) — 新增访问层流程 (LSP / IDE / GUI / ...)
- [`docs/process/new-internal-api.md`](new-internal-api.md) — 改 `internal/` 公开 API 流程 (S3 纪律承载)
- [`docs/process/github-branch-protection.md`](github-branch-protection.md) — GitHub 网页配置清单

**章节清单** (新-access-layer.md 全部已落地):
1. 触发条件 (新 entry point 客观判定)
2. 5 问清单 (用户 / 协议 / SDK / 接口面 / 边界)
3. 必交付物 7 步 (RFC → 接口面 → internal → 访问层 → 协议 → PR → pending 更新)
4. 边界铁律 5 条 (重申 architecture.md §〇·三)
5. PR 流程 (自检 / 必填 / 合并纪律)
6. 文档纪律 (S6 收口, 流程纪律**只**活此处)
7. 与其他流程文档的关系
8. 待补 (后续 chat 写 mcp.md / lsp.md / gui.md / CI workflow)

---

## 新 skill 规划 (回应 S6 / S7)

**❌ 已废 (2026-10-07 决定)**: 用户判断 AGENTS.md + `.cursor/rules/project-conventions.mdc` **已经够用**, **不**开 `.cursor/skills/kron-discipline/`.  

**理由 (用户口径)**:  
- AGENTS.md 已有 "Workflow" 段 + "Things that are off-limits without explicit ask" 段 + "Coding standards" 段
- `.cursor/rules/project-conventions.mdc` 是其镜像, **alwaysApply: true** —— Cursor 任何 agent 编辑任何文件都会自动加载
- **不**需要再开一个 skill —— 多一个 skill 等于多一处可能"忘了读"的入口

**改向**: S6 "必须写文档再交付" 纪律**不**进 skill, **改**进 `docs/process/new-access-layer.md` (上文 §"流程文档规划") 的 "PR 流程" 章节. 流程纪律**只**活在一处 = `docs/process/`, **不**散落 skill 里.

---

## GitHub 配置规划 (回应 S5 / S9)

**已建** (2026-10-07):
- `.github/CODEOWNERS` — 所有路径 owner = @zhangjun005
- `.github/pull_request_template.md` — PR 必填字段 (8 段)
- `docs/process/github-branch-protection.md` — **人工**操作清单 (master 分支保护)

**待操作** (用户**手动**去 GitHub 网页):
- Settings → Branches → Add rule (master)
- 必勾项见 `github-branch-protection.md` §2.2
- 必**不**勾项见 §2.3 (force push / deletions / bypass)

**待补** (后续 chat 写):
- `.github/workflows/ci.yml` (必**先**建, 否则 branch protection 找不到 status check)
- `MAINTAINERS.md` 仓库根 (单维护者, 暂以 AGENTS.md 替代)

---

## 联动改动清单 (回应 F1 / F2 / F3)

**注意**: 改以下文件**必须**同时改对应镜像, 否则 architecture.md §9 单向引用链破裂.

| 主源 | 镜像 | 这次必改 | 实际状态 |
|---|---|---|---|
| `docs/abstractDesign/architecture.md` | `AGENTS.md` + `.cursor/rules/project-conventions.mdc` | ❓ 这次是否要改 architecture.md? 待定 | **未改** — 用户未明确拍, 保持架构真理源**不动** |
| `AGENTS.md` "off-limits" 段 | `.cursor/rules/project-conventions.mdc` "Off-limits" 段 | ✅ F1 — GUI/LSP/MCP 不再是 Phase 2 | ✅ **已改** |
| `AGENTS.md` 锁定的"CLI 最小集 init/add/lint/serve-mcp" | `.cursor/rules/project-conventions.mdc` | ✅ F2 — Repo layout 加 serve-lsp / serve-gui | ✅ **已改** |
| AGENTS.md "TypeScript only when GUI phase begins" | `.cursor/rules/project-conventions.mdc` Tech stack 段 | ✅ S8 — wails + shadcn 引入 TS 前端 | ✅ **已改** |
| `docs/abstractDesign/tech-stack.md` (若有) | — | ❓ 等 RFC 拍 LSP SDK / GUI 栈后回写 | ❓ `tech-stack.md` 仓库**未**找到 (可能不存在) |

---

## 不在本清单的事 (本次明确**不**做)

| 事 | 原因 |
|---|---|
| 实际写 `internal/` 代码 (relations / identity / lint 扩) | RFC 已规划, 实施**等**用户拍 RFC 后启动 |
| 改 `docs/abstractDesign/architecture.md` | 架构真理源**不动** (用户未明确拍) |
| GitHub 网页操作 (branch protection) | 需用户**手动**去 GitHub 网页, AI 做**不**了 |
| `MAINTAINERS.md` 仓库根 | 本批**不**建, 暂以 AGENTS.md "作者" 段替代 |
| 任何 commit | 等用户 review |

---

## 下一 chat 启动建议

按这个顺序**一个一个**谈 (用户**自己**排, 不需要 AI 排):

1. **AGENTS.md "off-limits" 段** — 改 (F1) — **最小**改动, **最大**避免后续踩雷
2. **`project-conventions.mdc` 路由** — 改 (F2 + S8) — 跟上 F1
3. ~~**`kron-discipline` skill** — 建 (S7)~~ — **❌ 已废 (2026-10-07)**: 不开新 skill
4. **`new-access-layer.md` + `new-internal-api.md`** — 写 (S4 / S6) — 流程纪律**只**活在这两份文档里
5. **LSP SDK RFC** — 写 (S2) — 已选定 `go.lsp.dev/protocol`, 写 RFC 锁定
6. **GUI 栈 RFC (wails + shadcn)** — 写 (S1) — 选定后写
7. **`CODEOWNERS` / PR template** — 建 (S5 / S9)
8. **GitHub branch protection** — 网页操作 (S5)

> **不**是 AI 排的优先级 — 是按"**最小动作先做, 后续依赖前面**"的依赖关系排的. 用户可以**重新排**.

---

## 执行完成 (2026-10-07 二次封档)

> 用户指令"全部做完再让我看, 别停", AI 按 §"下一 chat 启动建议" 1→8 顺序**全部**完成, 不分批.

**完成清单**:

| # | 步骤 | 文件 | 状态 |
|---|---|---|---|
| 1 | 改 AGENTS.md "off-limits" 段 (F1) | `AGENTS.md` | ✅ |
| 2 | 改 `project-conventions.mdc` 镜像 (F2 + S8) | `.cursor/rules/project-conventions.mdc` | ✅ |
| 3 | ~~建 `kron-discipline` skill (S7)~~ | — | ❌ 已废 (用户决定) |
| 4 | 写 `new-access-layer.md` + `new-internal-api.md` (S4 / S6 / S3) | `docs/process/new-access-layer.md` + `docs/process/new-internal-api.md` | ✅ |
| 5 | 写 LSP SDK RFC (S2) | `docs/rfc/2026-10-07-lsp-sdk.md` | ✅ |
| 6 | 写 GUI 栈 RFC (S1) | `docs/rfc/2026-10-07-gui-stack.md` | ✅ |
| 7 | 建 `CODEOWNERS` / PR template / 操作清单 (S5 / S9) | `.github/CODEOWNERS` + `.github/pull_request_template.md` + `docs/process/github-branch-protection.md` | ✅ |
| 8 | GitHub branch protection (S5) | — | ⏳ 用户**手动**操作 (AI 做不了) |

**未 commit** — 等用户 review 一次性合.

**本批**未动的:
- `docs/abstractDesign/architecture.md` (架构真理源, 用户未拍, AI **不**擅自动)
- `docs/abstractDesign/tech-stack.md` (仓库**未**找到此文件, 推测不存在)
- 任何 `internal/` 代码 (RFC 已规划接口, 实施**等** RFC 合 master 后)

---

## 第三轮封档 (2026-10-08)

> 用户 2026-10-08 review "基本不存在问题, 可以继续看看还有没有需要决定的".  
> AI 扫 architecture.md / tech-stack.md / mcp.md / docs-map.md / `cmd/kron/serve-mcp/` 后**发现 9 项** D1-D9.

### 用户口径 (2026-10-08)

- **GUI 选型**: "原生的应用 (使用前端写再通过 wails 转) **或者** vscode 侧边栏+窗口"
- **Web 应用**: "这完全偏离了原意" — **显式**否决
- **两套 GUI 路径次序**: Wails 先 (v1.3), VSCode 扩展后 (v1.4+)

### D1-D9 处置

| # | 标题 | 处置 |
|---|---|---|
| **D1** | GUI = Wails + VSCode 双路径 (撤 2026-10-07 错推"Web 应用") | ✅ **已修** `2026-10-07-gui-stack.md` 整篇推倒 (§1/§2/§3/§4/§5/§6/§7/§8/§9/§10/§11/§12) |
| **D2** | AGENTS.md / project-conventions.mdc 措辞 | ✅ **已修** Tech stack 表 4 行 + Repo layout 引用 §1.1 |
| **D3** | architecture.md §七 vs §一 自相矛盾 | ✅ **已修** §七 拆 3 行 (Wails v1.3 / LSP v1.3+ / VSCode 扩展 v1.4+); §一 123 行改 "Wails 原生" + 加 "VSCode 扩展" 副行 |
| **D4** | architecture.md `internal/lint` 描述差 phase | ✅ **已修** §〇·五·5 加 `internal/relations` / `internal/identity` 行 (3 包均 v1 已实开) |
| **D5** | MCP 协议层 `initialize` / `tools/list` 缺失 | ⏳ **未动** (用户**没**勾; 写在 `docs/process/mcp-protocol.md` §4 实施计划, VSCode 扩展 v1.4+ **必须**前置于此) |
| **D6** | `new-access-layer.md` §8 漏引已存在文档 | ✅ **已修** 加 9 行"仓库已存在流程/协议文档"表 |
| **D7** | `docs-map.md` §五 索引漏 3 文件 | ✅ **已修** 加 `mcp-protocol.md` / `github-branch-protection.md` / `new-access-layer.md` / `new-internal-api.md` / `references-snapshot.md` / `pending-decisions.md` / `view-call-tree-intent.md` |
| **D8** | `pending-decisions.md` S1 "Wails + shadcn" 已错 | ✅ **已修** (D1 涵盖) |
| **D9** | AGENTS.md `internal/relations` 措辞 "kron_impact / kron_delete" | ⏳ **未动** (措辞偏差, **不**致命; 待 v1.4 评估 `kron_delete` 是否升级为 hard block) |

### D1 RFC §10 顺修 (RFC 拍板后必做, 当前 PR 范围内)

`2026-10-07-gui-stack.md` §10 列**必须**顺修的 9 行 (architecture.md / tech-stack.md / mcp.md):
- 6/9 已在**本批**完成 (D3 修 §一 + §七 + §〇·五·5 3 行; D2 镜像同步 AGENTS.md + project-conventions.mdc)
- **未**修 3/9: `architecture.md` §五 287 行 / §八 335 行 / §5.4.2 整段 + `tech-stack.md` §2 整段 + `mcp.md` §5 整段

**这 3 处**是 D1 RFC §10 列的"同一 PR 内必做" — AI **没**擅自动, 等用户在**同一 PR** 拍板**时**一起改 (与 D5 "MCP 协议层" 升级一并处理).

### 本批 Git 状态

```
M  AGENTS.md
M  .cursor/rules/project-conventions.mdc
M  docs/abstractDesign/architecture.md
M  docs/abstractDesign/docs-map.md
M  docs/process/new-access-layer.md
M  docs/rfc/2026-10-07-gui-stack.md
M  docs/process/pending-decisions.md  (本文件)
```

(其他**未改**文件: LSP RFC / `new-internal-api.md` / `github-branch-protection.md` / `mcp.md` / `tech-stack.md`)

### 建议 commit 粒度 (3 个)

1. `docs(architecture): align GUI selection (Wails v1.3 + VSCode ext v1.4+)` — architecture.md / docs-map.md / AGENTS.md / project-conventions.mdc / 2026-10-07-gui-stack.md
2. `docs(process): index already-existing flows in new-access-layer.md` — new-access-layer.md §8
3. `docs(pending-decisions): archive D1-D9 findings (D5/D9 deferred)` — pending-decisions.md (本文件)

### D5 (MCP 协议层) 后续路径

D5 是**事实** 风险 (MCP 协议层没完整 → 任何**新**接入 Claude Desktop / Cursor / MCP Inspector 的用户**直接**失败) — 优先级 P0 但**用户**没勾. 等用户**回来**说"做 D5"再处理. 实施计划**已**在 `docs/process/mcp-protocol.md` §4 写明 3 步.

---

## 与本次 RFC 的关系

[RFC 2026-10-04-source-files-reverse-view](../../rfc/2026-10-04-source-files-reverse-view.md) 已合 master (commit `f4a84cb`). 本清单**不影响**该 RFC — RFC §9 待办 (7) (8) (频率量测 + cache 评估) 仍**在执行窗口外**.

---

## 清理审计日志 (2026-10-08)

> **注意**: 本节是**清理动作的审计追溯**, **不**是新增决策. 本文档于 2026-10-07 封档, 此后**只**记录"对已封档条目 / 已删除文件的二次操作", **不**再增新的 F / S 决策.

### C1 软删一批过期/重复文档 (2026-10-08)

8 个文件移入 `.deprecated/2026-10-08-doc-cleanup/`:

| 原路径 | 软删原因 |
|---|---|
| `docs/abstractDesign/tech-stack.md` | 内容已并入 AGENTS.md "Tech stack (locked in)" 表 |
| `docs/process/references-snapshot.md` | 自报"§一 §二 数据已过期"+ 自动化脚本未落地 |
| `docs/process/phase-archive/` (3 文件) | 全部 2026-10-03 closed, 历史归档 |
| `docs/process/mcp-protocol.md` | 协议层状态已并入 `implementation/mcp.md` §6 |
| `docs/rfc/2026-10-04-mcp-protocol-redesign.md` | 已 SUPERSEDED (被 mcp-sdk-selection 取代) |
| `docs/rfc/2026-10-04-source-files-reverse-view.md` | 草案 + phase 3+ 未规划 |

**违规修正**: 此前 chat (2026-10-08) 误向本文件 F 表加 F4 / F5 (MCP 寿命拍板). 已被本次清理撤回. 实际拍板文档为 [`docs/rfc/2026-10-08-mcp-lifecycle.md`](../../rfc/2026-10-08-mcp-lifecycle.md), 实施待办**不**进 F 表, 应**新开** RFC 或直接进 `phase-3-leftovers.md` (待 phase 3 启动).