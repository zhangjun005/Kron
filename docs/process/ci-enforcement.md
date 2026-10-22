# CI 可检查规则与待办

> 来源：`docs/abstractDesign/docs-map.md` §六  
> 本文件是**实施流程**，不是架构真理。

---

## 1 已有 CI 检查（已实现）

> 运行手册（怎么跑、怎么看、怎么手动触发）见 [`github-actions.md`](./github-actions.md)。
>
> **2026-10-22 重建**：仓库现在只有一个 workflow —— [`.github/workflows/ci.yml`](../../.github/workflows/ci.yml)
> （`name: CI`，单 job `verify (windows-latest)`）。旧的 `store-platform.yml`（三平台）已删除：
> 维护者只用 Windows，Linux / macOS 暂不考虑。
>
> 范围是**刻意收窄**的：只覆盖"代码"，不覆盖"文档"；测试只跑重点包，不跑全量。

| 规则 | 检查命令 | 门禁条件 | 位置 |
|---|---|---|---|
| Go format | `gofmt -l cmd internal frontend`（不含 `kron-mcp-test/` fixture） | 无输出 | `ci.yml` step `gofmt（要求无输出）` |
| Go build | `go build ./...` | exit 0 | `ci.yml` step `go build` |
| Go vet | `go vet ./...` | 0 errors | `ci.yml` step `go vet` |
| Import 边界 (a) | `go list -deps ./internal/...` 中无 `kron/cmd/` | 0 matches | `ci.yml` step `import 边界` |
| Import 边界 (b) | `cmd/kron/<sub>` 不依赖另一个 `cmd/kron/<other>` | 0 matches | 同上 |
| 重点包测试 | `go test ./internal/store ./internal/parser ./internal/lint ./internal/relations` | 全绿 | `ci.yml` step `重点包测试` |

**刻意不覆盖（决策记录，不是遗漏）**

| 不覆盖的东西 | 原因 |
|---|---|
| `cmd/kron/**` 的测试、两个 `e2e` 套件 | 属于"全量"档：`serve-mcp` 单包 32s，e2e 各 7s，太重 |
| `internal/model` / `identity` / `view` / `assumption` | 便宜（<2s）但暂不算"重点"，需要时改一行即可加 |
| Linux / macOS 平台 | 维护者只用 Windows；代价是 `internal/store` 的 `flock` 分支不再被 CI 验证 |
| **文档类规则**（内链完整性、architecture 无代码块、process 无实现代码…） | 2026-10-22 决定：文档不设门禁 —— 文档改动频率低、检查存量破口多（实测 74 处断链），设门禁容易在临门一脚卡住，收益不抵成本 |

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
> **ruleset 未设置 required status checks** —— `ci.yml` 已存在但尚未成为强制门禁。
> 要变成真门禁：ruleset `protect-master` → Require status checks → 加入 `verify (windows-latest)`。

---

## 2 文档类检查（2026-10-22 决定：**不设门禁**，仅留档）

> 决定理由：文档改动频率低、存量破口多（实测 74 处断链）、门禁时机少，
> 设成门禁容易在临门一脚卡住，收益不抵成本。以下规则**只作人工 review 参考**。

| 规则 | 检查方式 | 难度 | 备注 |
|---|---|---|---|
| `architecture.md` 不含 API 代码块 | `grep -E '```(go\|sh)' docs/abstractDesign/architecture.md` 应 0 匹配 | 低 | 防止真理膨胀回实施层 |
| `architecture.md` 不含流程图 | `grep -E '^```$' docs/abstractDesign/architecture.md` 计数 ≤ 1（允许依赖图一个） | 低 | 防止真理混入流程 |
| `process/` 文件不含实现代码 | `grep -rE '```(go|sh)' docs/process/` 应 0 匹配 | 低 | 流程文件只含决策树，不含代码块 |
| `implementation/` 文件不含 `为什么` | `grep -riE '(因为|原因|决定|理由)' docs/implementation/` 数量少 | 中 | 允许解释性文字，但不允许决策性文字 |

---

## 3 文档一致性检查（留档，**不计划**自动化）

> 与 §2 同理：文档类不设门禁。以下思路仅在未来某次文档重构时作参考。

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

## 5 落地状态（2026-10-22）

| 项 | 状态 |
|---|---|
| Import boundary | ✅ **已落地**：`ci.yml` 的 `import 边界` step 用 `go list -deps` 检查（不用 grep，避免注释误报） |
| Go build / vet / gofmt | ✅ **已落地**：`ci.yml` |
| 重点包测试（store/parser/lint/relations） | ✅ **已落地**：`ci.yml` |
| `architecture.md` 无代码块 / 无流程图 | ⏸ 不设门禁（§2） |
| Frontmatter 三文件同步 | ⏸ 不设门禁（§2/§3），需要时人工 review |
| 单向引用方向 | ⏸ 不设门禁（§2/§3） |
| 文档内链完整性 | ❌ **存量 74 处断链**，连"可检查"都还不到；先清理再谈，且按 §2 决定不设门禁 |
| 全量 `go test ./...`（含 `cmd/` 与 e2e） | ⏸ 刻意不跑（`serve-mcp` 单包 32s）——只在本机按需跑 |

P0 和 P1 建议在 **v1 交付前落地**。
