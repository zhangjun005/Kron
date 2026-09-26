# CI 可检查规则与待办

> 来源：`docs/abstractDesign/docs-map.md` §六  
> 本文件是**实施流程**，不是架构真理。

---

## 1 已有 CI 检查（已实现）

| 规则 | 检查命令 | 门禁条件 | 位置 |
|---|---|---|---|
| Go vet | `go vet ./...` | 0 errors | `.github/workflows/ci.yml` |
| Go format | `gofmt -l .` | 0 files | `.github/workflows/ci.yml` |
| Go tests | `go test ./...` | 0 failures | `.github/workflows/ci.yml` |
| Import boundary | `grep -r "internal/cli" cmd/` | 0 matches | 手动或 CI script |
| Import boundary | `grep -r "cmd/kron/" internal/` | 0 matches | 手动或 CI script |

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
