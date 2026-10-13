# 方案: B-3 假设架构 (Option B1) — 双源收口

> 拍板: B-3 路线, 但**不**砍 inline frontmatter。  
> 日期: 2026-10-12 (PR2 落地)  
> 关键决策: `model.Assumption` 保留 (含 `Text` 字段, 作 round-trip 兼容),
> `Frontmatter.Assumptions` 保留 (per-intent 元数据),
> `text` 字段在 frontmatter 中**保留** (parser 不报错),
> 但**新增 lint 规则** `RuleAssumptionInline` 在 `text:` 出现时**Warning** 提示迁移。  
> 假设**真正**的单一真相源 (text) 是 `.kron/assumptions/<id>.md`。

---

## 0. 背景与历史

### 0.1 PR1 (2026-10-12 早): 尝试过砍掉 inline — 失败, 已回退

PR1 走的是"完全删 inline"路线:
- 删 `model.Assumption` struct
- 删 `Frontmatter.Assumptions` 字段
- 改 MCP wire 类型 (删 `AssumeEntry` / `AssumeWarning` / `AssumeCheckSummary`)

commit `0649cee` 已落地, 但后续 lint orphan 规则**完全无信号源** — intent
body 既不引用 assumption id, frontmatter 也没了, registry 是孤岛。

回退 PR1 改用 Option B1。

### 0.2 Option B1 (本 PR, 拍板): 双源共存, lint 推动迁移

B-3 的本质是"text 单一真相源在 registry",**不**意味着 frontmatter 必须空白。
Frontmatter 可以保留 per-intent 元数据 (id, severity, rationale, expires_at,
verified_at, verified_by) — 这正是 per-intent 覆盖语义。

Text 在 frontmatter 出现是**A 路径遗留**, 但 parse 仍接受, lint 推作者清掉。

### 0.3 目标

- ✅ `model.Assumption` 字段集保留 (含 `Text`, 作 round-trip)
- ✅ `Frontmatter.Assumptions` 保留 (per-intent 元数据)
- ✅ Parser 不报错地接受 `text:` (A 路径兼容)
- ✅ 新增 `RuleAssumptionInline` (Warning) — 推作者从 frontmatter 删 `text:`
- ✅ 新增 `@kron:assumes <id>` body marker (B-3 的 orphan 规则信号源)
- ✅ `RuleAssumptionOrphan` 重写, 走 frontmatter 引用 ∪ body marker 双路径
- ❌ **不**改 `AssumptionFile` / `AssumptionFrontmatter` (registry 类型不变)
- ❌ **不**改 MCP wire 类型 (假设列表照常下发)

---

## 1. 受影响文件清单 (PR2 实际改动)

### 1.1 `internal/parser/` — 新增 marker + 检测函数

| 文件 | 改动 |
|---|---|
| `assumption_anchors.go` (新建) | `ScanAssumptionAnchors` (raw) + `SlugsForAssumption` (id→slugs 倒排) + `isValidAssumptionID` |
| `assumption_anchors_test.go` (新建) | 12 个测试 (empty / body marker / 跨意图 / 错误 id / 非 .kron/intents/ 跳过) |
| `frontmatter.go` | `HasInlineAssumptionText` raw-bytes 检测 (lint 用) |

### 1.2 `internal/lint/` — RuleAssumptionInline + 重写 orphan 规则

| 文件 | 改动 |
|---|---|
| `rules.go` | 新增 `RuleAssumptionInline = "assumption-inline-text"` |
| `assumption_rules.go` | 全部重写 — `RuleAssumptionInline` 检测 + `RuleAssumptionOrphan` 走双路径 (frontmatter ∪ body marker) |
| `lint.go` | `runAssumptionRules` 签名加 `root` 参数 (raw 读 FM + walk body marker 用) |
| `lint_test.go` | `TestRun_AssumptionMixedForm` → `TestRun_AssumptionInlineText_Fires`;新增 `TestRun_AssumptionOrphan_NotOrphanIfBodyMarker` + `TestRun_AssumptionOrphan_NoMarkerNoFM`; 4 个旧 fixture 移除 inline `text:` (与新规则不冲突) |

### 1.3 `internal/model/` — **无改动**

`model.Assumption` 字段集保持原样 (含 `Text`)。B-3 是**行为**约束 (registry
是 text 唯一源), 不是**结构**约束 (struct 删字段)。

### 1.4 `cmd/kron/serve-mcp/` — **无改动**

Wire 类型 (`AssumeEntry` / `AssumeWarning` / `AssumeCheckSummary`) 保留,
MCP 走 frontmatter.Assumptions + registry 合并。

### 1.5 `internal/store/` — **无改动**

`UpdatePatch.Assumptions` 不动 (PR1 的 dead-code 删除本就是低风险清理, 但
本次为了保持 git diff 范围, 顺带回滚)。后续 cleanup PR 可单独再做。

### 1.6 `internal/relations/` — **无改动**

`ReferencingIntents` helper 不动 (PR1 同理, 后续 cleanup PR 再做)。

---

## 2. 设计权衡

### 2.1 为什么不让 parser 拒绝 `text:`?

A→B 迁移期间, 仓库内大量既有 intent frontmatter 含 inline `text:`。
如果 parser 拒绝, **整个 frontmatter 加载失败**, lint / MCP 全都报
`RuleFrontmatterInvalid`, 用户看不到真正的问题 ("text 不该在 FM 里")。

折中: parser 接受 (含 round-trip), lint 推迁移 (`RuleAssumptionInline`,
Warning, 非阻塞)。

### 2.2 为什么不删 `model.Assumption.Text`?

`Text` 字段是 struct 层的 round-trip 容器, 删了之后:
- `SerializeMarkdown` 不能完整复刻 (无 text 可写)
- MCP `kron_get` 返回的 frontmatter 没 `text` 字段 (wire 不稳定)
- 既有的 inline 测试 fixture 全要改

保留字段 + lint 推动, 是**最不扰民**的路径。

### 2.3 `@kron:assumes` marker 走什么语法?

跟 `@kron:intent` 同设计:
- 行级注释前缀任意 (`//` / `#` / `--` / `<!-- -->`)
- 必须 token-isolated (左右空白边界)
- `<id>` 跟 `parser.ValidateSlug` 同款规则 (kebab-case, 无 `/`)

不带 `text:` 等其他字段 — marker 只做**引用声明**, per-intent 元数据还在
frontmatter。

### 2.4 orphan 规则双路径 (frontmatter ∪ body marker)

- 一个假设被引用 iff:
  - (a) 至少一个 intent 的 `frontmatter.assumptions[].id == <id>`, **或**
  - (b) 至少一个 intent body 有 `@kron:assumes <id>` 行
- 既不 (a) 也不 (b) → orphan
- `status: superseded` 例外 (跟旧规则一致)

这条双路径给了作者**两种引用方式**:
- 想"代码 + frontmatter 都明确"的 → 用 frontmatter (走 (a))
- 想"body 内联, lint 看到清晰"的 → 用 marker (走 (b))
- 两条同时用 → 不会重复算 (set dedup)

### 2.5 `RuleAssumptionMixedForm` 还在 rules.go, 但不发射

按"frozen 标识符, 老 JSON 仍能 round-trip"原则, `RuleAssumptionMixedForm` 常量
保留在 `internal/lint/rules.go` 但**没有任何代码路径**发射它。

如果未来需要诊断 v1.3 之前的 A-path 迁移遗留, 可以重新激活 (基于
`a.Text != ""` 之类的检查) — 但本 PR 不做。

---

## 3. 不需要迁移脚本

跟 PR1 同样的理由: 无用户, 无生产数据。

---

## 4. 提交前必跑

```bash
go vet ./...
gofmt -l .
go test ./...
```

实际跑过: 10 packages ok, vet clean, gofmt clean。

---

## 5. 影响范围摘要

| 维度 | 变化 |
|---|---|
| 假设数据存储 | frontmatter (per-intent) ∪ registry (text 唯一源) |
| MCP wire 类型 | 不变 |
| lint 规则 | 新增 `assumption-inline-text` (Warning) |
| body marker | 新增 `@kron:assumes <id>` (orphan 规则的信号源) |
| CLI | 无变化 |
| LSP | 无变化 |
| registry writer | 无变化 |

---

## 6. 后续 (G-5 收口, 不在本 PR)

`kron_add` / `kron_update` 收到 `[]string` 假设 ID 列表时, 对每个 ID 调
`assumption.Writer.Create` (如不存在) / `Update` (如存在) → registry
成为"假设被引用时自动创建"的惰性注册表。这是 v1.4+ 的 PR, 不阻塞本 PR。

---

## 7. PR2 vs PR1 的对比

| 维度 | PR1 (回退) | PR2 (本 PR, 拍板) |
|---|---|---|
| `model.Assumption` | 删 | 保留 |
| `Frontmatter.Assumptions` | 删 | 保留 |
| parser 遇 `text:` | 报错 (fallback lenient) | 接受 (round-trip) |
| `RuleAssumptionInline` | 不存在 | 新增, Warning |
| orphan 规则信号源 | 无 | frontmatter + body marker 双路径 |
| body marker | 不存在 | `@kron:assumes <id>` |
| MCP wire | 改 (删 AssumeEntry 等) | 不变 |
| store.Writer.UpdatePatch.Assumptions | 删 | 保留 |
| relations.ReferencingIntents | 删 | 保留 |
| 文件净增减 | -401 行 | +540 行 (新 parser 文件 + lint 重写) |
