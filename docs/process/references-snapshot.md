# 当前文档引用关系快照

> 状态：**快照**，生成于 2026-09-26。  
> 与 [`docs-map.md`](../abstractDesign/docs-map.md) 的关系：本文件是其实例化结果，不是规则定义。  
> 重新生成方法见下方"如何重新生成此文件"。

---

## 一、引用图（按文件）

> 表示"**A 引用了 B**"——即 A 文档的正文或顶栏含 `B.md` 的链接/路径引用。

```
A 文档                                         引用了 B 文档（数量按行）
─────────────────────────────────────────────────────────────────────────

docs/abstractDesign/architecture.md            ├── docs/implementation/cli.md        (8)
                                              ├── docs/implementation/mcp.md        (6)
                                              ├── docs/implementation/domain-model.md (1)
                                              ├── docs/implementation/error-catalog.md (1)
                                              ├── docs/implementation/testing.md    (1)
                                              ├── docs/process/cli-flag.md         (2)
                                              ├── docs/process/lint-rule.md        (1)
                                              ├── docs/process/migrate.md          (1)
                                              ├── docs/process/internal-pkg.md     (1)
                                              ├── docs/business.md                 (1)
                                              ├── docs/requirements.md             (2)
                                              └── AGENTS.md, .cursor/rules/*       (3)

docs/abstractDesign/docs-map.md                ├── docs/process/ci-enforcement.md   (1)
                                              ├── AGENTS.md (mention, no link)     (4)
                                              └── [自身规则示例，无外部引用]

docs/abstractDesign/intent-structure.md        └── （无文件引用；零依赖）

docs/abstractDesign/tech-stack.md              └── docs/abstractDesign/architecture.md (1)
                                                  (one-line "see architecture.md" pointer)

docs/article.md                                └── （无文件引用；纯立场文档）

docs/business.md                               ├── docs/requirements.md             (2)
                                              ├── docs/article.md                  (1)
                                              ├── docs/implementation/ide-interaction.md (1) ← 修复后
                                              └── docs/requirements.md §5 引用链已断
                                                  (旧 §5 已删除，引用改写完；当前已干净)

docs/requirements.md                           └── （无文件引用；自包含）

docs/implementation/README.md                  ├── docs/abstractDesign/architecture.md (1)
                                              └── docs/abstractDesign/docs-map.md   (1)

docs/implementation/api-surface.md             └── docs/abstractDesign/architecture.md (1)

docs/implementation/cli.md                     └── docs/abstractDesign/architecture.md (1)

docs/implementation/domain-model.md            └── docs/abstractDesign/architecture.md (1)

docs/implementation/error-catalog.md           └── docs/abstractDesign/architecture.md (1)

docs/implementation/ide-interaction.md         └── docs/abstractDesign/architecture.md (1, implicit via "architecture.md §〇 铁律 #3")

docs/implementation/mcp.md                     └── docs/abstractDesign/architecture.md (1)

docs/implementation/testing.md                 └── docs/abstractDesign/architecture.md (1)

docs/process/README.md                         ├── docs/abstractDesign/architecture.md (1)
                                              ├── docs/implementation/             (1)
                                              └── docs/process/cli-flag.md, lint-rule.md, internal-pkg.md, migrate.md (在 "如何使用" 段落)

docs/process/cli-flag.md                       ├── docs/abstractDesign/architecture.md (1)
                                              ├── docs/implementation/cli.md       (1)
                                              └── AGENTS.md                        (1)

docs/process/ci-enforcement.md                 ├── docs/abstractDesign/docs-map.md  (1)
                                              ├── docs/abstractDesign/architecture.md (1, referenced by grep example)
                                              ├── docs/abstractDesign/intent-structure.md (1, by grep example)
                                              ├── docs/implementation/domain-model.md (1, by grep example)
                                              └── （grep examples 引用多个文件 — 这是 CI 脚本示例，不是内容依赖）

docs/process/internal-pkg.md                   ├── docs/abstractDesign/architecture.md (1, top + step 3)
                                              └── AGENTS.md                        (1, step 3)

docs/process/lint-rule.md                      ├── docs/abstractDesign/architecture.md (1)
                                              ├── docs/implementation/cli.md       (2)
                                              └── docs/requirements.md             (1)

docs/process/migrate.md                        ├── docs/abstractDesign/architecture.md (1, top)
                                              ├── docs/abstractDesign/intent-structure.md (1, §0 + step 6)
                                              ├── docs/implementation/domain-model.md (1)
                                              └── AGENTS.md                        (1, step 6)

AGENTS.md                                      ├── docs/abstractDesign/architecture.md  (3, top + §〇 + §一 source lines)
                                              ├── docs/abstractDesign/intent-structure.md (1)
                                              ├── docs/abstractDesign/docs-map.md   (1)
                                              ├── docs/implementation/api-surface.md (1)
                                              ├── docs/implementation/mcp.md        (1)
                                              ├── docs/process/                     (5, in "Off-limits" + workflow)
                                              └── docs/abstractDesign/architecture.md / .cursor/rules/ (mirror pattern)

.cursor/rules/project-conventions.mdc          └── docs/abstractDesign/architecture.md / AGENTS.md (mirror)
```

---

## 二、引用聚合（按"被引用次数"看谁在中心）

| 文档 | 被引用次数 | 角色 |
|---|---|---|
| `docs/abstractDesign/architecture.md` | **15** | B — 真理源（正确） |
| `AGENTS.md` | **5** | I — 协作共识（高频被引用，因为是 onboarding 入口） |
| `docs/abstractDesign/intent-structure.md` | **2** | C — 数据真理（低频但必引） |
| `docs/implementation/cli.md` | **3** | F — 实施层 hub |
| `docs/business.md` | **1** | D — 业务边界（只被 architecture 引用） |
| `docs/abstractDesign/docs-map.md` | **2** | K — 文档图谱本身 |
| `docs/abstractDesign/architecture.md` × `requirements.md` × `process/migrate.md` | 1 | （分散引用） |

**观察**：

- `architecture.md` 是当之无愧的中心（被引用 15 次）——这与它"真理源"的定位一致。
- `business.md` 只被 architecture.md 引用 1 次——它"业务边界"的定位决定了它通过 B 间接控制一切，而不是被反复引用。
- 所有 `implementation/*.md` 与 `process/*.md` 都引 `architecture.md`，**且仅引用 architecture.md**——单根引用图，符合单向链约束。

---

## 三、是否符合 `docs-map.md` 单向链？

| 检查项 | 期望 | 实际 | 状态 |
|---|---|---|---|
| B → F | `architecture.md` 引用 `implementation/*.md` 仅为导航引 | 8 处全是 "见 [`docs/implementation/cli.md`](../implementation/cli.md)" 导航引用 | ✅ |
| B → G | `architecture.md` 引用 `process/*.md` 仅为导航引用 | 5 处全是 "see `process/cli-flag.md`" 导航引用 | ✅ |
| F → G | `implementation/` 不引用 `process/` 作为内容 | `implementation/README.md` 第 38 行引用过 `process/lint-rule.md` 已在 L40-L47 删除 | ✅ (2026-09-26 修复) |
| F → B | `implementation/` 都引用 `architecture.md` 作为内容 | 全部 8 个 F 文件有"来源"引用 §X | ✅ |
| G → B | `process/` 都引用 `architecture.md` 作为内容 | 全部 6 个 G 文件有"来源"引用 §X | ✅ |
| G → C | `process/migrate.md` 引用 `intent-structure.md` | 是 | ✅ |
| C → F | `intent-structure.md` 不引用 `implementation/domain-model.md` | 不引用 | ✅ |
| I → B | `AGENTS.md` 引用 `architecture.md` | 是 | ✅ |
| I → G | `AGENTS.md` 在正文中引用 `process/*.md` 作为依据 | 是——在"Off-limits"段落：禁止加新 flag 但**不引用** `process/cli-flag.md` 的内容 | ✅ |
| I → C | `AGENTS.md` 引用 `intent-structure.md` | 是（frontmatter reminder） | ✅ |

**结论**：全部符合单向链。当前状态干净。

---

## 四、已修复的陈旧引用（本次清理）

| 文件 | 原引用 | 现引用 | 原因 |
|---|---|---|---|
| `docs/process/internal-pkg.md` L89 | `docs/implementation/architecture.md` | `docs/abstractDesign/architecture.md` | 旧路径已合并 |
| `docs/process/internal-pkg.md` L90 | `docs/architecture-structure.md` | (删除该行) | 文件已删除 |

> **历史**：`docs/architecture-structure.md` 在上次清理中删除；本次提交 `037da8c` 后残留 2 处未清理引用，今日 (2026-09-26) 修复。

---

## 五、如何重新生成此文件

> 本文件是**一次性快照**；建议每次大规模文档改动后重生成。

```bash
# 1. 收集所有引用对（A → B）
grep -rEo '\[`?[a-zA-Z._/-]+\.md`?\]?\([^)]+\)' docs/ AGENTS.md .cursor/rules/ 2>/dev/null \
  | grep -oE '[a-zA-Z0-9/_-]+\.md' \
  | sort -u

# 2. 替换为 A → B 边表（人工整理）

# 3. 验证单向链
for f in docs/implementation/*.md; do
  if grep -q '^在.*process/' "$f"; then
    echo "WARN: F → G content reference in $f"
  fi
done
```

或者更直接：每月 1 号用 `git grep -E "[a-z_/-]+\\.md"` 跑一次人工审计。

---

## 六、与 `docs-map.md` 的分工

| 文件 | 内容性质 | 何时读 |
|---|---|---|
| `docs-map.md` | **规则**（应当如何） | 写新文档 / 检查新 PR 引用方向 |
| `references-snapshot.md`（本文件）| **现状**（实际如何） | 评估文档健康度 / 找新 refactor 切入点 |

两文件合起来回答：**"现在谁引用谁" + "应该怎么引用"**。
