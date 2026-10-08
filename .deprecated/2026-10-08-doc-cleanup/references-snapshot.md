# 当前文档引用关系快照

> 状态：**快照**，生成于 2026-09-26。  
> 与 [`docs-map.md`](../abstractDesign/docs-map.md) 的关系：本文件是其实例化结果，不是规则定义。  
> 重新生成方法见下方"如何重新生成此文件"。
>
> **注意**：2026-09-27 本批改动（MCP 工具集 8 → 12、新增 [`view-call-tree-intent.md`](../abstractDesign/view-call-tree-intent.md) 与 [`mcp-tool.md`](./mcp-tool.md)）后，**§一 §二 数据已过期**；见 §四 末"已修复的陈旧引用（待重生成）"。
>
> **2026-10-03 追加**：phase 2 L1（MCP 9 handler 实现 + `phase-2-leftovers.md` 新建）提交 commit `8e04914`。L1 主要影响 internal 代码，**对文档引用关系图谱的直接影响为零**（未新增文档、未删除文档）。新增待重生成项见 §4.2。

---

## 一、引用图（按文件）

> 表示"**A 引用了 B**"——即 A 文档的正文或顶栏出现 B 的链接/路径。  
> 内容由 `grep` 真实抓取（2026-09-26）。未列出的 = 无引用。

### 1.1 真理层（abstractDesign/）

- [`docs/abstractDesign/architecture.md`](../abstractDesign/architecture.md) → 引用（21 处）:
  - [`docs/implementation/cli.md`](../implementation/cli.md) × 8
  - [`docs/implementation/mcp.md`](../implementation/mcp.md) × 6
  - [`docs/implementation/domain-model.md`](../implementation/domain-model.md) × 1
  - [`docs/implementation/error-catalog.md`](../implementation/error-catalog.md) × 1
  - [`docs/implementation/testing.md`](../implementation/testing.md) × 1
  - [`docs/process/cli-flag.md`](../process/cli-flag.md) × 2
  - [`docs/process/lint-rule.md`](../process/lint-rule.md) × 1
  - [`docs/process/migrate.md`](../process/migrate.md) × 1
  - [`docs/process/internal-pkg.md`](../process/internal-pkg.md) × 1
  - [`docs/business.md`](../business.md) × 1
  - [`docs/requirements.md`](../requirements.md) × 2
  - [`AGENTS.md`](../../AGENTS.md)（mirror 段落）× 3
- [`docs/abstractDesign/docs-map.md`](../abstractDesign/docs-map.md) → 引用（2 处）:
  - [`docs/process/ci-enforcement.md`](../process/ci-enforcement.md) × 1
  - `AGENTS.md` 提及 × 1（无链接）
- [`docs/abstractDesign/intent-structure.md`](../abstractDesign/intent-structure.md) → **无引用**
- [`docs/abstractDesign/tech-stack.md`](../abstractDesign/tech-stack.md) → 引用（1 处）:
  - [`architecture.md`](../abstractDesign/architecture.md) × 1（一行"see architecture.md"指针）

### 1.2 业务/需求层（顶层 docs/）

- [`docs/article.md`](../article.md) → **无引用**
- [`docs/business.md`](../business.md) → 引用（3 处）:
  - [`docs/requirements.md`](../requirements.md) × 2
  - [`docs/article.md`](../article.md) × 1
  - [`docs/implementation/ide-interaction.md`](../implementation/ide-interaction.md) × 1（导航引用，已修复 §5 悬空引用）
- [`docs/requirements.md`](../requirements.md) → **无引用**

### 1.3 实施层（implementation/）

- [`docs/implementation/README.md`](../implementation/README.md) → 引用（2 处）:
  - [`architecture.md`](../abstractDesign/architecture.md) × 1
  - [`docs-map.md`](../abstractDesign/docs-map.md) × 1
- [`docs/implementation/api-surface.md`](../implementation/api-surface.md) → 引用（1 处）:
  - [`architecture.md`](../abstractDesign/architecture.md) × 1（顶栏）
- [`docs/implementation/cli.md`](../implementation/cli.md) → 引用（1 处）:
  - [`architecture.md`](../abstractDesign/architecture.md) × 1（顶栏）
- [`docs/implementation/domain-model.md`](../implementation/domain-model.md) → 引用（1 处）:
  - [`architecture.md`](../abstractDesign/architecture.md) × 1（顶栏）
- [`docs/implementation/error-catalog.md`](../implementation/error-catalog.md) → 引用（1 处）:
  - [`architecture.md`](../abstractDesign/architecture.md) × 1（顶栏）
- [`docs/implementation/ide-interaction.md`](../implementation/ide-interaction.md) → 引用（1 处）:
  - [`architecture.md`](../abstractDesign/architecture.md)（通过"§〇 铁律 #3"间接）
- [`docs/implementation/mcp.md`](../implementation/mcp.md) → 引用（1 处）:
  - [`architecture.md`](../abstractDesign/architecture.md) × 1（顶栏）
- [`docs/implementation/testing.md`](../implementation/testing.md) → 引用（1 处）:
  - [`architecture.md`](../abstractDesign/architecture.md) × 1（顶栏）

### 1.4 流程层（process/）

- [`docs/process/README.md`](../process/README.md) → 引用（3 处）:
  - [`architecture.md`](../abstractDesign/architecture.md) × 1
  - `docs/implementation/` × 1（目录级，无具体文件）
  - 各 process doc × 4（"如何使用"段落中导航引用）
- [`docs/process/cli-flag.md`](../process/cli-flag.md) → 引用（3 处）:
  - [`architecture.md`](../abstractDesign/architecture.md) × 1（顶栏）
  - [`implementation/cli.md`](../implementation/cli.md) × 1（Step 4）
  - [`AGENTS.md`](../../AGENTS.md) × 1（Step 5）
- [`docs/process/ci-enforcement.md`](../process/ci-enforcement.md) → 引用（2 处）:
  - [`docs-map.md`](../abstractDesign/docs-map.md) × 1（顶栏）
  - grep 示例中提及多文件 — **不计入内容引用**（脚本示例）
- [`docs/process/internal-pkg.md`](../process/internal-pkg.md) → 引用（2 处）:
  - [`architecture.md`](../abstractDesign/architecture.md) × 2（顶栏 + Step 3）
  - [`AGENTS.md`](../../AGENTS.md) × 1（Step 3）
- [`docs/process/lint-rule.md`](../process/lint-rule.md) → 引用（3 处）:
  - [`architecture.md`](../abstractDesign/architecture.md) × 1（顶栏）
  - [`implementation/cli.md`](../implementation/cli.md) × 2
  - [`requirements.md`](../requirements.md) × 1
- [`docs/process/migrate.md`](../process/migrate.md) → 引用（4 处）:
  - [`architecture.md`](../abstractDesign/architecture.md) × 1（顶栏）
  - [`intent-structure.md`](../abstractDesign/intent-structure.md) × 2（§0 + Step 6）
  - [`domain-model.md`](../implementation/domain-model.md) × 1（§0 + Step 6）
  - [`AGENTS.md`](../../AGENTS.md) × 2（§0 + Step 6）
- [`docs/process/references-snapshot.md`](../process/references-snapshot.md) → **本文件**

### 1.5 协作层

- [`AGENTS.md`](../../AGENTS.md) → 引用（11 处）:
  - [`architecture.md`](../abstractDesign/architecture.md) × 3（顶栏 + §〇 + §一 source 行）
  - [`intent-structure.md`](../abstractDesign/intent-structure.md) × 1
  - [`docs-map.md`](../abstractDesign/docs-map.md) × 1
  - [`implementation/api-surface.md`](../implementation/api-surface.md) × 1
  - [`implementation/mcp.md`](../implementation/mcp.md) × 1
  - `docs/process/` × 5（Off-limits + workflow 段）
- [`.cursor/rules/project-conventions.mdc`](../../.cursor/rules/project-conventions.mdc) → 引用（mirror）:
  - [`architecture.md`](../abstractDesign/architecture.md)（mirror）
  - [`AGENTS.md`](../../AGENTS.md)（mirror）
- [`.cursor/skills/go-style/SKILL.md`](../../.cursor/skills/go-style/SKILL.md) → 引用（1 处）:
  - 通用建议：`docs/abstractDesign/XX-name.md`（模板说明）

### 1.6 人类入口层

- [`docs/how-it-works.md`](../how-it-works.md) → 引用（9 处）:
  - [`README.md`](../../README.md) × 1
  - [`architecture.md`](../abstractDesign/architecture.md) × 2
  - [`intent-structure.md`](../abstractDesign/intent-structure.md)（间接，通过 model 引用）× 1
  - [`model/intent.go`](../../internal/model/intent.go) × 1
  - [`implementation/cli.md`](../implementation/cli.md) × 1
  - [`implementation/mcp.md`](../implementation/mcp.md) × 1
  - [`docs-map.md`](../abstractDesign/docs-map.md) × 1
  - [`AGENTS.md`](../../AGENTS.md) × 1
  - [`references-snapshot.md`](../process/references-snapshot.md) × 1

---

## 二、被引用次数排名（按"被引"看谁在中心）

> 数据基于 `rg -l <target> docs AGENTS.md .cursor`（排除本文件自身）。  
> 入度 = "多少个其他文件提到了它"，而非"出现多少行"。  
> 重新生成：`bash scripts/regen-refs-snapshot.sh`（见 §五）。

| 排名 | 文档 | 入度 | 角色 | 评价 |
|---|---|---|---|---|
| 1 | [`architecture.md`](../abstractDesign/architecture.md) | **18** | B 真理源 | 中心节点（正确） |
| 2 | [`AGENTS.md`](../../AGENTS.md) | **8** | I 协作共识 | 高频是因为 onboarding 入口 |
| 3 | [`intent-structure.md`](../abstractDesign/intent-structure.md) | **7** | C 数据真理 | frontmatter 三文件同步链路必引 |
| 4 | [`requirements.md`](../requirements.md) | **6** | A 需求事实 | architecture + business + lint-rule 都引 |
| 5 | [`implementation/cli.md`](../implementation/cli.md) | **4** | F 实施层 | 实施层 hub |
| 5 | [`docs-map.md`](../abstractDesign/docs-map.md) | **4** | K 文档图谱 | AGENTS + ci-enforcement + 多处导航 |
| 5 | [`business.md`](../business.md) | **4** | D 业务边界 | architecture §九 + business §1.x + 多处 |
| 8 | `implementation/mcp.md`, `implementation/domain-model.md`, `implementation/error-catalog.md`, `implementation/testing.md`, `implementation/ide-interaction.md`, `implementation/api-surface.md`, `implementation/README.md` | 2-3 | F | 各 process doc 顶栏 + architecture §九 |
| 8 | `process/cli-flag.md`, `process/lint-rule.md`, `process/internal-pkg.md`, `process/migrate.md`, `process/ci-enforcement.md` | 1-2 | G | architecture §九 表格引用 |
| 8 | `article.md`, `tech-stack.md` | 1-2 | H / E | 边缘节点（独立文档） |

**观察**：

- `architecture.md` 入度 18——名副其实的中心（真理源定位正确）。
- `AGENTS.md` 入度 8——协作层被引合理（onboarding 入口必须显眼）。
- 所有 `implementation/*.md` 和 `process/*.md` 至少被 `architecture.md` §九引用——构成"真理源 → 所有实施/流程层"的全图谱覆盖。

---

## 三、是否符合 [`docs-map.md`](../abstractDesign/docs-map.md) 单向链？

> 表中 ✅ / ❌ 表示**链路规则是否通过**（与"功能是否已实现"无关）。

| 检查项 | 期望 | 实际 | 通过？ |
|---|---|---|---|
|---|---|---|---|
| B → F | [`architecture.md`](../abstractDesign/architecture.md) 引用 `implementation/*.md` 仅为导航引用 | 17 处全是 "见 [`implementation/cli.md`](../implementation/cli.md)" 导航引用 | ✅ |
| B → G | [`architecture.md`](../abstractDesign/architecture.md) 引用 `process/*.md` 仅为导航引用 | 5 处全是 "see [`process/cli-flag.md`](../process/cli-flag.md)" 导航引用 | ✅ |
| F → G | [`implementation/`](../implementation/) 不引用 [`process/`](../process/) 作为内容 | 全部仅引用 [`architecture.md`](../abstractDesign/architecture.md) | ✅ |
| F → B | [`implementation/`](../implementation/) 都引用 [`architecture.md`](../abstractDesign/architecture.md) 作为内容 | 8 个 F 文件全部有"来源"引用 §X | ✅ |
| G → B | [`process/`](../process/) 都引用 [`architecture.md`](../abstractDesign/architecture.md) 作为内容 | 6 个 G 文件全部有"来源"引用 §X | ✅ |
| G → C | [`process/migrate.md`](../process/migrate.md) 引用 [`intent-structure.md`](../abstractDesign/intent-structure.md) | 是 | ✅ |
| C → F | [`intent-structure.md`](../abstractDesign/intent-structure.md) 不引用 [`domain-model.md`](../implementation/domain-model.md) | 不引用 | ✅ |
| I → B | [`AGENTS.md`](../../AGENTS.md) 引用 [`architecture.md`](../abstractDesign/architecture.md) | 是 | ✅ |
| I → G | [`AGENTS.md`](../../AGENTS.md) 在正文中引用 `process/*.md` 作为依据 | 在 Off-limits 段禁止加新 flag 但**不引用** [`process/cli-flag.md`](../process/cli-flag.md) 的内容 | ✅ |
| I → C | [`AGENTS.md`](../../AGENTS.md) 引用 [`intent-structure.md`](../abstractDesign/intent-structure.md) | 是（frontmatter reminder） | ✅ |

**结论**：全部符合单向链。当前状态干净。

---

## 四、已修复的陈旧引用（本次清理）

| 文件 | 原引用 | 现引用 | 原因 |
|---|---|---|---|
| `docs/process/internal-pkg.md` L89 | `docs/implementation/architecture.md` | `docs/abstractDesign/architecture.md` | 旧路径已合并 |
| `docs/process/internal-pkg.md` L90 | `docs/architecture-structure.md` | (删除该行) | 文件已删除 |

> **历史**：`docs/architecture-structure.md` 在上次清理中删除；本次提交 `037da8c` 后残留 2 处未清理引用，今日 (2026-09-26) 修复。

### 4.1 已过期引用（2026-09-27 待重生成）

本批改动后 §一 §二 数据需重新生成才能反映。**未重生成前的差异**：

| 文件 | 改动 | 重生成后 §一 期望新增行 |
|---|---|---|
| [`docs/implementation/mcp.md`](../implementation/mcp.md) | §1 表格 8 行 → 12 行；§2 追加 4 个工具契约；§3 加 1 行 "v1 扩展的 4 个工具..." | `architecture.md` 引用 `mcp.md` 计数从 6 → 7 |
| [`docs/abstractDesign/architecture.md`](../abstractDesign/architecture.md) | §1.2 表格扩 1 行 + 末段 1 行；§八 加 1 行 | (无新增引用;但 §1.2 与 §八 文本被多家引用) |
| [`docs/business.md`](../business.md) | §一目标加 1 句;§2.3 末加 2.3.1 段(1 个新工具表格 + 3 行文字) | (无新增引用) |
| [`docs/how-it-works.md`](../how-it-works.md) | §5 工具集列表扩到 12 + 加 2 句 | `implementation/mcp.md` 引用计数从 1 → 2 |
| **新增** [`docs/abstractDesign/view-call-tree-intent.md`](../abstractDesign/view-call-tree-intent.md) | (整篇新文件) | 6 个文件已引用:architecture.md / docs-map.md / how-it-works.md / implementation/mcp.md / implementation/README.md / process/README.md |
| **新增** [`docs/process/mcp-tool.md`](./mcp-tool.md) | (整篇新文件) | 2 个文件已引用:docs-map.md / process/README.md |
| [`docs/abstractDesign/docs-map.md`](../abstractDesign/docs-map.md) | §五 索引加 2 行 | (无新增引用;但本身入度从 4 → 6) |
| [`docs/implementation/README.md`](../implementation/README.md) | 文档地图补 view-call-tree-intent.md;mcp.md 描述 8 → 12 | (无新增引用) |
| [`docs/process/README.md`](./README.md) | 文档地图补 mcp-tool.md | (无新增引用) |

> 重生成命令：见 §五 `scripts/regen-refs-snapshot.sh`（当前未落地，按 §五 手工流程执行）。

### 4.2 2026-10-03 phase 2 L1 后续待重生成项

phase 2 L1（commit `8e04914`，9 个 MCP handler 实现）**对文档引用图谱的直接影响**：

| 文件 | 改动 | 重生成后 §一 期望新增行 |
|---|---|---|
| **新增** [`docs/process/phase-2-leftovers.md`](./phase-2-leftovers.md) | (整篇新文件) | 当前入度 0；如 phase-1-leftovers 在主仓被引则同源 |
| [`docs/process/README.md`](./README.md) | 若需要将 phase-2-leftovers.md 加入"流程文档索引"则 +1 行 | (待 review) |
| [`AGENTS.md`](../../AGENTS.md) | 未变 — `phase-2-leftovers.md` 不在 AGENTS.md 的 Off-limits / Workflow 引用中 | (无)

---

## 五、如何重新生成此文件

> 本文件是**快照**；建议每次大规模文档改动后重生成，或每月 1 号做一次例行审计。

### 5.1 一次性手工：复现 §一和 §二

```bash
# 列出所有"被引"的文档
rg -l 'architecture\.md|docs-map\.md|intent-structure\.md|tech-stack\.md|business\.md|article\.md|requirements\.md|implementation/[a-z-]+\.md|process/[a-z-]+\.md|AGENTS\.md' docs AGENTS.md .cursor \
  | rg -v 'references-snapshot\.md' \
  | sort -u

# 算入度（按文件数）
for target in architecture docs-map intent-structure tech-stack business article requirements AGENTS \
              implementation/cli implementation/mcp implementation/domain-model implementation/error-catalog \
              implementation/testing implementation/ide-interaction implementation/api-surface \
              process/cli-flag process/lint-rule process/internal-pkg process/migrate process/ci-enforcement; do
  count=$(rg -l "${target}\.md" docs AGENTS.md .cursor 2>/dev/null | rg -v 'references-snapshot\.md' | wc -l)
  printf "%-40s %s\n" "$target.md" "$count"
done
```

### 5.2 自动化（推荐）

新增 [`scripts/regen-refs-snapshot.sh`](../../scripts/regen-refs-snapshot.sh)：

```bash
#!/bin/bash
# 重生成 docs/process/references-snapshot.md
# 由 docs/process/ci-enforcement.md P3 建议落地
set -e

SNAPSHOT="docs/process/references-snapshot.md"
TODAY=$(date +%Y-%m-%d)

# 计算入度表
TARGETS=(
  "docs/abstractDesign/architecture.md"
  "docs/abstractDesign/docs-map.md"
  "docs/abstractDesign/intent-structure.md"
  "docs/abstractDesign/tech-stack.md"
  "docs/business.md"
  "docs/article.md"
  "docs/requirements.md"
  "AGENTS.md"
  "docs/implementation/cli.md"
  "docs/implementation/mcp.md"
  # ... 其余目标
)

# ...（写入 §二 表格逻辑）

echo "Updated $SNAPSHOT"
```

> 当前未落地（ci-enforcement.md 列为 P3）；手工流程已能产出 §一 §二 数据。

---

## 六、与 [`docs-map.md`](../abstractDesign/docs-map.md) 的分工

| 文件 | 内容性质 | 何时读 |
|---|---|---|
| [`docs-map.md`](../abstractDesign/docs-map.md) | **规则**（应当如何） | 写新文档 / 检查新 PR 引用方向 |
| [`references-snapshot.md`](../process/references-snapshot.md)（本文件）| **现状**（实际如何） | 评估文档健康度 / 找新 refactor 切入点 |

两文件合起来回答：**"现在谁引用谁" + "应该怎么引用"**。
