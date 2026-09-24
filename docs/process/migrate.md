# Frontmatter Schema 迁移流程

> 来源：`docs/abstractDesign/architecture.md` §七  
> 本文件是**实施流程**，不是架构真理。当 frontmatter schema 需要变更时按此流程执行。

---

## 1 何时需要迁移

| 变更类型 | 示例 | 需要迁移？ |
|---|---|---|
| 新增可选字段 | 增加 `tags` 字段 | ❌ 不需要（向前兼容） |
| 修改可选字段名 | `symbol` → `symbols` | ✅ 需要（字段名是硬编码键） |
| 新增必需字段 | `created_by` 从可选改为必需 | ✅ 需要（现有文件会 break） |
| 删除字段 | 移除 `reviewers` | ✅ 需要（旧文件会残留） |
| 改变字段类型 | `status` 从 string → enum | ✅ 需要（解析行为改变） |

> 简单原则：**任何破坏向前兼容的变更**都需要迁移流程。

---

## 2 迁移流程（6 步）

### Step 1 — 提出变更（RFC 阶段）

在 `docs/rfc/`（不存在则新建）目录下创建 `YYYY-MM-DD-<short-title>.md`，包含：

- 变更描述（字段语义、为什么改）
- 变更前后的 frontmatter 对照
- 对现有 `.kron/intents/*.md` 的影响（手动统计有多少文件会受影响）
- 迁移策略（自动脚本 / 手动脚本 / 废弃警告期）

### Step 2 — 评审

- 至少 1 名协作者 review
- 将 RFC link 贴到对应 Intent 的评论或 git commit body

### Step 3 — 实现迁移脚本

在 `scripts/migrate-<version>.sh`（或 Go）编写一次性迁移脚本：

```bash
#!/bin/bash
# 示例：给所有 intent frontmatter 增加 updated_at（从文件 mtime 推断）
for f in .kron/intents/**/*.md; do
  mtime=$(stat -c %y "$f" | cut -d. -f1)
  # 用 sed/yq 原地插入 updated_at 字段
done
```

> **脚本必须幂等**：多次运行结果相同。

### Step 4 — 试运行（dry-run）

```bash
# dry-run：不写文件，只打印会改什么
MIGRATE_DRYRUN=1 ./scripts/migrate-<version>.sh
```

### Step 5 — 执行 + commit

```bash
./scripts/migrate-<version>.sh
git add -A
git commit -m "chore(migrate): apply frontmatter schema v<old>→v<new>

Migration script: scripts/migrate-<version>.sh
Affected intents: N files
RFC: docs/rfc/<YYYY-MM-DD-title>.md"
```

### Step 6 — 同步文档

- 更新 `docs/abstractDesign/intent-structure.md` 的 frontmatter schema
- 更新 `docs/implementation/domain-model.md` 的 `Frontmatter` 结构
- 更新 `AGENTS.md` 的 storage format reminder
- PR 描述里包含迁移命令

---

## 3 特殊情况处理

### 情况 A — 破坏性变更且无法自动迁移

如果变更无法用脚本修复（如删除了一个字段的语义），分两阶段发布：

1. **废弃警告期**（至少 1 个 minor version）：`lint` 输出 warning，不报错
2. **硬报错期**：下个 minor version 开始 `lint` 拒绝不合规文件

### 情况 B — 跨仓库迁移

如果 Kron 迁移到新仓库：用 `git log --follow -- .kron/intents/` 导出历史 intent 文件，然后执行 `scripts/migrate-<version>.sh`。

---

## 4 与 CI 的关系

迁移脚本的执行时机：

| 场景 | 何时执行迁移脚本 |
|---|---|
| 新克隆仓库 | `kron lint` 自动检测 + 报错（建议先跑迁移） |
| 已有仓库 pull 最新 | pre-commit hook 或 CI step 执行迁移脚本 |
| CI 流水线 | `kron lint` 先于构建步骤执行，迁移脚本作为独立 CI job |

> **CI 门禁是单向的**：`kron lint` 只报告不合规，不自动修复（防止静默覆盖用户内容）。
