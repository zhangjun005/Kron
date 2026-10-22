# CI 可检查规则与待办

> 来源：`docs/abstractDesign/docs-map.md` §六  
> 本文件是**实施流程**，不是架构真理。

---

## 1 已有 CI 检查（已实现）

> ⚠ **2026-10-22 核对**：仓库里只有 `.github/workflows/store-platform.yml` 一个 workflow，
> **不存在** `.github/workflows/ci.yml`。下面前三行实际是「本机手动 + PR 模板 checklist」级别，
> 没有 CI 自动执行，`位置` 列已改正为实情。

| 规则 | 检查命令 | 门禁条件 | 位置 |
|---|---|---|---|
| Go vet | `go vet ./...` | 0 errors | 本机手动 + PR 模板 checklist（**无** workflow） |
| Go format | `gofmt -l .` | 0 files | 本机手动 + PR 模板 checklist（**无** workflow） |
| Go tests | `go test ./...` | 0 failures | 本机手动 + PR 模板 checklist（**无** workflow） |
| Import boundary | `grep -r "internal/cli" cmd/` | 0 matches | 手动或 CI script |
| Import boundary | `grep -r "cmd/kron/" internal/` | 0 matches | 手动或 CI script |
| Store platform tests | `go test ./internal/store` | 三平台均通过 | `.github/workflows/store-platform.yml` — **非门禁**（2026-10-22 起）：PR 不跑，仅 `push: master` 或 `workflow_dispatch` 手动跑 |

### 1.1 门禁调整权（2026-10-22）

**规则源 = [`AGENTS.md`](../../AGENTS.md) §3 红线**「改 CI 门禁 … 只 `@zhangjun005` 有资格」。
本节只记录**怎么硬性落实**，不重复红线正文。

覆盖范围：改 `.github/workflows/*` 的触发条件 / matrix / 检查步骤，以及增删「必需检查
（required status checks）」。

这条规则**不能只靠文档自律**，因为 write 协作者在 GitHub Web 上可以直接编辑 workflow 文件。
硬性约束必须落在 GitHub 设置上（`@zhangjun005` 是唯一 repo admin）：

| 目标 | GitHub 设置 |
|---|---|
| 设置本身只有你能改 | 分支保护 / ruleset / 协作者管理都要求 **admin**；`write` 改不了 |
| 别人改 workflow 必须经你批准 | `master` 上开 `Require a pull request` + `Require review from Code Owners`（`CODEOWNERS` 的 `* @zhangjun005` 已覆盖 `.github/workflows/`） |
| 你自己能直接改 | ruleset 的 **bypass list 只放 Repository admin** |
| 禁止直推 / 强推 / 删分支 | ruleset 加 `Block force pushes` + `Restrict deletions` |
| 外部 PR 不自动跑 workflow | Settings → Actions → Fork pull request workflows → Require approval |

> 事实核对（2026-10-22，用 `GET /rulesets` + `GET /rules/branches/master` + `/collaborators` 查）：
> ruleset **`protect-master`**（id 25624728, `active`, `~DEFAULT_BRANCH`）已生效，`master.protected=true`；
> 实际生效的 rules = `deletion` + `non_fast_forward` + `pull_request`
> （`required_approving_review_count=1`, `require_code_owner_review=true`,
> `dismiss_stale_reviews_on_push=true`）；bypass = `RepositoryRole/Repository admin`
> （`bypass_mode=always`）。协作者：`@zhangjun005`(admin) / `@lanyue811`(write) / `@HomoFluctus`(write)。
> **ruleset 未设置 required status checks** —— 所以与本节「Store platform tests 非门禁」不冲突，
> 不会出现 `Expected — waiting for status to be reported`。

---

## 2 已有 CI 检查（可实现，但未落地）

| 规则 | 检查方式 | 难度 | 备注 |
|---|---|---|---|
| `architecture.md` 不含 API 代码块 | `grep -E '```(go\|sh)' docs/abstractDesign/architecture.md` 应 0 匹配 | 低 | 防止真理膨胀回实施层 |
| `architecture.md` 不含流程图 | `grep -E '^```$' docs/abstractDesign/architecture.md` 计数 ≤ 1（允许依赖图一个） | 低 | 防止真理混入流程 |
| `process/` 文件不含实现代码 | `grep -rE '```(go|sh)' docs/process/` 应 0 匹配 | 低 | 流程文件只含决策树，不含代码块 |
| `implementation/` 文件不含 `为什么` | `grep -riE '(因为|原因|决定|理由)' docs/implementation/` 数量少 | 中 | 允许解释性文字，但不允许决策性文字 |

---

## 3 待落地 CI 检查（当前靠 code review）

### 3.1 Frontmatter 三文件同步（高价值，建议 v1 完成前落地）

**目的**：防止 `intent-structure.md` 改了字段，`domain-model.md` 和 `AGENTS.md` 没跟上的静默漂移。

**检查脚本思路**：

```bash
#!/bin/bash
# 检查 intent-structure.md 的 YAML 字段列表
FIELDS=$(grep -E '^- \`[a-z_]+\`:' docs/abstractDesign/intent-structure.md | \
          sed 's/.*`\([a-z_]*\)`:.*/\1/' | sort -u)

# 检查 domain-model.md 的 Go struct yaml tag
GOTAGS=$(grep -E 'yaml:"[a-z_' data = $(grep -E 'yaml:"[a-z_]+' docs/implementation/domain-model.md | \
          sed 's/.*yaml:"\([a-z_]*\)".*/\1/' | sort -u)

# diff 应为空
diff <(echo "$FIELDS") <(echo "$GOTAGS") && echo "OK" || echo "MISMATCH"
```

> **局限性**：这个脚本只能检查字段名，不能检查类型（`[]string` vs `string`）。完整类型检查需要正则解析 Go struct——目前没有方案。

### 3.2 单向引用方向（中等价值）

**目的**：防止 F 引用 G、G 引用 F 等循环引用。

**检查脚本思路**（基于已知引用模式）：

```bash
#!/bin/bash
# 检查 implementation/ 是否引用了 process/
IMPL_REFS_PROCESS=$(grep -rh 'process/' docs/implementation/*.md | \
                    grep -v "Architecture TRUTH" | \
                    grep -v "see.*process" || true)
if [ -n "$IMPL_REFS_PROCESS" ]; then
    echo "FAIL: implementation/ should not reference process/"
    echo "$IMPL_REFS_PROCESS"
    exit 1
fi
```

> **当前状态**：手动验证通过（`implementation/README.md` 的"如何使用"段落引用了 `process/` 下的文件——这是文档地图，不是流程引用，应该保留）。需要细化判断逻辑。

### 3.3 文档角色标签一致性（低价值，长期维护）

**目的**：确保每份文档顶部的角色标签（`> 来源：`）与实际内容一致。

**检查方式**：人工 code review（自动检查的价值不够高，暂不投入）。

---

## 4 已知检查盲区（CI 无法覆盖，必须靠人和流程）

| 盲区 | 为什么 CI 管不了 | 缓解措施 |
|---|---|---|
| `architecture.md` §〇·五的判定表措辞是否"过强" | 表格文字是否算"建议性"还是"强制性"是语义问题 | code review；评审者需读 §〇 铁律 |
| `business.md` 的"建议这样设计"是否实际上已经违反 | 文字约束无法量化 | `kron lint` 负责 lint 层约束，业务层约束靠架构 review |
| `ide-interaction.md` 等不在 v1 交付范围的文件，内容是否真的不执行 | 当前阶段无代码落地，无法验证 | v1 收尾时做一次全面 cross-check |
| `AGENTS.md` 与 `architecture.md` 的措辞一致性 | 两份文件措辞不同但含义相同 | code review；两份文件在同一次 PR 修改 |

---

## 5 落地优先级建议

| 优先级 | 检查项 | 理由 |
|---|---|---|
| **P0** | Import boundary（`grep` 两行） | 已在 architecture.md §2.2 写出，立即可加 |
| **P1** | `architecture.md` 无代码块 | 防止真理膨胀回滚；本次拆分的目的之一 |
| **P2** | Frontmatter 三文件同步 | 高价值 + 常见错误 |
| **P3** | 单向引用方向 | 中价值，需要先定义清楚什么算"引用" |

P0 和 P1 建议在 **v1 交付前落地**。
