# 文档关系图谱（Docs Map）

> 本文件是**架构真理**的一部分。所有文档的分类、角色、引用方向以本文档为准。
> 架构真理位于 [`architecture.md`](./architecture.md)，本文档是它的地图附件。

---

## 一、文档角色分类（8 类）

| 角色 | 标识 | 说明 | 文件数 |
|---|---|---|---|
| **A 需求事实** | A | 从外部输入的约束；工具本身不能否定它 | 1 |
| **B 架构真理** | B | Kron 的不可妥协边界；其他所有文件都不得与之矛盾 | 2 |
| **C 数据格式真理** | C | `.kron/intents/*.md` 的文件 schema；跨工具一致 | 1 |
| **D 业务边界** | D | Kron 对外提供哪些能力；决定 B 的适用范围 | 1 |
| **F 实施建议** | F | 具体 API 签名、工具契约；B 的展开，不可与之矛盾 | 7 |
| **G 实施流程** | G | 决策树与检查清单；触发时强制遵循 | 7 |
| **H 哲学愿景** | H | 学术论文摘要与设计原则；D 的来源之一 | 2 |
| **K 实景** | K | 真实代码 + 真实场景下的 Kron 行为示例；人类友好的入口 | 1 |

另有协作共识文件：

| 标识 | 文件 | 说明 |
|---|---|---|
| **I 协作共识** | `AGENTS.md` | AI 编程规范；新人 onboarding 入口；受 B/F/G 同步更新 |
| **J Cursor 规则** | `.cursor/rules/*.mdc` | Cursor IDE 行为规范；受 B/F/G 同步更新 |

---

## 二、角色关系图

```
A (需求事实)
  │
  ├──► D (业务边界)          "需求决定业务范围"
  │
  └──► B (架构真理)           "需求约束架构边界"

D (业务边界)
  │
  └──► B (架构真理)            "业务决定架构职责"

E (技术选型)
```
> E 角色文档 (`tech-stack.md`) 2026-10-08 软删, 内容并入 AGENTS.md. 若 E 角色需恢复, 新增 `docs/abstractDesign/tech-stack.md` 时按下列关系挂载即可.
```
  │
  └──► B (架构真理)            "技术支撑架构实现"

H (哲学愿景)
  │
  └──► D (业务边界)            "学术思想影响设计决策"

B (架构真理)  ──────────────►  F (实施建议)
"真理引用展开"                    "展开必须与真理一致"

B (架构真理)  ──────────────►  G (实施流程)
"真理引用流程"                    "流程产出必须与真理一致"

B (架构真理)  ──────────────►  I (协作共识)
"真理同步至协作层"               "AI 规范受真理约束"

C (数据格式真理)
  │
  ├──► B (架构真理)             "数据模型是架构 §3 的事实来源"
  │
  ├──► I (协作共识)             "frontmatter 示例是 AGENTS.md 的 storage reminder"

G (实施流程)  ──────────────►  B (架构真理)
"流程步骤引用真理"               (单向引用；B 不引用 G)

F (实施建议)   ──────────────►  B (架构真理)
"API 签名引用真理"               (单向引用；B 不引用 F)

F (实施建议)   ──────────────►  C (数据格式真理)
"domain-model.md 引用 intent-structure.md"
```

---

## 三、单向引用链（核心原则）

> **所有引用方向必须从左指到右；禁止反向。**

```
A ──► B ──► F
  │    │
  │    └──► G
  │
  └──► D ──► B

C ──► B
  │
  └──► I

E ──► B    *(保留为占位; E 角色 2026-10-08 软删, 若恢复见上面 E 块说明)*

H ──► D ──► B
```

### 3.1 两类引用

| 引用类型 | 含义 | 是否允许反向 |
|---|---|---|
| **内容引用**（必须单向）| 在正文中把另一份文件作为论据（"按 §X 规定……"） | ❌ 禁止 |
| **导航引用**（允许灵活）| 在"如何使用"段落或文档地图中给读者指路 | ✅ 允许 |

**区分方法**：在 `docs/implementation/README.md` 或 `docs/process/README.md` 这类"如何使用"段落中引用别处是导航引用——不是内容依赖。CI 不把这种引用当作反向依赖处理。

| 禁止的引用方向 | 违反示例 | 为什么禁止 |
|---|---|---|
| F → G | `implementation/cli.md` 引用 `process/cli-flag.md` 的决策树作为正文论据 | 实施建议不应引用流程；应该独立存在 |
| G → F | `process/migrate.md` 引用 `implementation/domain-model.md` 的 struct 作为"该流程何时触发"的条件 | 流程应引用真理层，不引用实施层 |
| B → F | `architecture.md` 包含具体 API 签名 | 真理不应展开到实施层；展开是 F 的职责 |
| B → G | `architecture.md` 包含决策树 | 真理不应包含流程；流程是 G 的职责 |
| I → G | `AGENTS.md` 把 process doc 内容抄进来 | 协作共识只引用真理；流程由 G 独立存在 |
| C → F | `intent-structure.md` 引用 `domain-model.md` 的 struct 字段 | 事实层不应引用实施层 |

---

## 四、三套同步约束（必须同时成立）

### 约束 1 — Frontmatter Schema（数据格式层）

```
C (intent-structure.md)
        │  事实层
        ▼
F (implementation/domain-model.md)
        │  实施层
        ▼
I (AGENTS.md "Storage format reminder")
        │  协作层
```

变更触发：`process/migrate.md` §0。三个文件必须在同一个 PR 内同步修改。

### 约束 2 — 架构边界（架构真理层）

```
B (architecture.md)
        │  真理层
        ▼
F (implementation/*)  +  G (process/*)
        │  实施建议 + 实施流程
```

变更触发：任意影响 B 的 PR。F 和 G 必须在对应 process doc 的 checklist 中引用 B 的相关章节。

### 约束 3 — 业务边界（业务边界层）

```
A (requirements.md)  ──►  D (business.md)  ──►  B (architecture.md)
     需求事实                业务定义                架构实现
```

变更触发：`requirements.md` 出现新约束。D 和 B 必须在下次更新中同步。

---

## 五、所有文件索引

| 角色 | 文件 | 受约束于 |
|---|---|---|
| **A** | `docs/requirements.md` | —（外部输入） |
| **B** | `docs/abstractDesign/architecture.md` | A, D, E |
| **B** | `docs/abstractDesign/view-call-tree-intent.md` | B, C (call-tree intent 视图) |
| **C** | `docs/abstractDesign/intent-structure.md` | —（数据事实） |
| **D** | `docs/business.md` | A, H |
| **F** | `docs/implementation/api-surface.md` | B, C |
| **F** | `docs/implementation/cli.md` | B |
| **F** | `docs/implementation/domain-model.md` | B, C |
| **F** | `docs/implementation/error-catalog.md` | B |
| **F** | `docs/implementation/ide-interaction.md` | B |
| **F** | `docs/implementation/mcp.md` | B |
| **F** | `docs/implementation/testing.md` | B |
| **F** | `docs/abstractDesign/view-call-tree-intent.md` | B, C |
| **G** | `docs/process/cli-flag.md` | B |
| **G** | `docs/process/internal-pkg.md` | B |
| **G** | `docs/process/lint-rule.md` | B |
| **G** | `docs/process/migrate.md` | B, C |
| **G** | `docs/process/ci-enforcement.md` | B |
| **G** | `docs/process/new-access-layer.md` | B (v1.3+ 新增访问层流程) |
| **G** | `docs/process/new-internal-api.md` | B (改 `internal/` 公开 API 流程) |
| **G** | `docs/process/pending-decisions.md` | — (决策链路审计日志, 2026-10-07 封档; 2026-10-08 软删审计见末尾 C1) |
| **G** | `docs/process/github-branch-protection.md` | — (GitHub 网页操作清单, **不**是文档真理) |
| **G** | `docs/process/ci-enforcement.md` | B |
| **H** | `docs/article.md` | — |
| **H** | `docs/persuasion.md` | — |
| **K** | `docs/how-it-works.md` | `internal/model/intent.go` (代码示例的真实类型) |
| **I** | `AGENTS.md` | B, C, F, G |
| **J** | `.cursor/rules/project-conventions.mdc` | B, F |
| **Z** | `.deprecated/` | — (软删归档, 2026-10-08 起; `.gitignore` 显式排除新文件, 已 index 文件保留) |

> **2026-10-08 软删**: 8 文件移入 [`.deprecated/2026-10-08-doc-cleanup/`](../../.deprecated/2026-10-08-doc-cleanup/) — `tech-stack.md` / `references-snapshot.md` / `phase-archive/*` (3) / `mcp-protocol.md` / `2026-10-04-mcp-protocol-redesign.md` / `2026-10-04-source-files-reverse-view.md`. 详见该目录 README + `pending-decisions.md` C1.

---

## 六、"CI 可机械检查" 的当前状态

| 规则 | 能否机械检查 | 检查方式 |
|---|---|---|
| 导入边界（`cmd/kron/` ↔ `internal/`） | ✅ 可以 | `grep -r "internal/cli" cmd/` 应 0 命中；`grep -r "cmd/kron/" internal/` 应 0 命中 |
| Frontmatter schema 三文件同步 | ❌ 不可以 | 靠 `process/migrate.md` 的人工 checklist |
| 文档单向引用方向 | ❌ 不可以 | 靠 code review |
| `architecture.md` 内容不膨胀 | ❌ 不可以 | 靠 code review（检查 §1/§3/§5 是否出现 API 代码块） |

具体 CI 检查建议见 [`docs/process/ci-enforcement.md`](../process/ci-enforcement.md)。
