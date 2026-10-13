# RFC: 关闭 B-3 软化窗口 (MigrationMode + inline text → 硬约束)

> 状态：**DRAFT (2026-10-12)**
> 范围：`internal/lint` (MigrationMode 字段) + `internal/model` (Assumption.Text) + `internal/parser` (frontmatter knownFields) + `internal/lint` (RuleAssumptionInline)
> 触发：Option B1 ([`docs/process/kill-inline-assumptions.md`](../../process/kill-inline-assumptions.md)) 落地后,评估"v1.3 → v1.5 软化窗口"是否符合"plan-first, 不是 stage-first"纪律([`AGENTS.md`](../../../AGENTS.md) §5)
>
> 关联 RFC: B-3 [`2026-10-08-assumptions-standalone.md`](../archive/2026-10-08-assumptions-standalone.md) + Option B1 [`kill-inline-assumptions.md`](../../process/kill-inline-assumptions.md)
> 关联实施: PR1 (`0649cee`, revert) → PR2 (`67369c6`, Option B1 拍板) → 本 RFC + 实施 = PR3 (本 RFC 拍板后的硬收口)

## 0 摘要

把 B-3 落地的两处"软化"窗口**全部收口**为硬约束:

1. **删除 `lint.RunOptions.MigrationMode` 字段** — 文档([`intent-structure.md` §三](../../abstractDesign/intent-structure.md)) 已拍板"v1.3 迁移期是 Warning, v1.5 改 Error",本 RFC 把"v1.3 迁移期"窗口**关闭**,rationale 必填从**永久 Warning 升 Error**。
2. **删除 `model.Assumption.Text` 字段** + 删 `parser.frontmatterKnownFields` 的 `text` 项 + `RuleAssumptionInline` 升 Error + 删 `tools_list_get.go` 的 inline text 兜底 — Option B1 §2.1 拍板"parser 接受, lint Warning 推作者清",本 RFC 把"Warning 推"窗口**关闭**,inline `text:` 写就**报错**。

**理由汇总** (详见 §2):
- 拍板本意都是"硬约束",落地是"软窗口" — 拍板 vs 实施有偏离
- "本仓库无存量迁移" ([`2026-10-08-assumptions-standalone.md` §1.2](../archive/2026-10-08-assumptions-standalone.md)) — 软化窗口为不存在的存量数据开
- "plan-first, 不是 stage-first" ([`AGENTS.md` §5](../../../AGENTS.md)) — 软化制造"未来必做"的隐性承诺
- "v1.5 改 Error" 没说"什么条件触发" — 是**无截止日期的版本化"以后再说"** — [`AGENTS.md` §5 红线]: 文档实施计划是辅助, 不是任务清单; 人说"以后再考虑" = 删, 不是留 todo

## 1 背景

### 1.1 B-3 拍板本意是"硬约束"

[`2026-10-08-assumptions-standalone.md` §3.4](../archive/2026-10-08-assumptions-standalone.md) 拍板 intent frontmatter `assumptions[]` 元素字段集:
> `id / severity / rationale / expires_at / verified_at / verified_by` (6 个)

**没有 `text`** — 拍板本意是"intent frontmatter 不写 text, text 走 registry single source of truth"。

[`2026-10-08-assumptions-standalone.md` §3.5](../archive/2026-10-08-assumptions-standalone.md) 拍板 registry 字段集:
> `id / text / default_severity / created_by / updated_at / status / reviewers / expires_at / verified_at / verified_by` (10 个)

**`text` 在 registry 必填**。

`rationale` 必填 ([`intent-structure.md` §三](../../abstractDesign/intent-structure.md)):
> **rationale 必填** (≥ 10 字符): 避免"凭直觉设 hard/soft"的反模式

**`rationale` 是"必填"** — 拍板本意是 Error 级别,不是 Warning。

### 1.2 落地时的两处软化

**软化 1 — `MigrationMode` 字段** (PR1 `610a1ec` 落地):

`internal/lint/lint.go:43-49`:
```go
// MigrationMode controls how RuleAssumptionRationaleRequired is
// surfaced. During the A→B migration window (v1.3 → v1.5),
// empty-rationale is a Warning to avoid blocking existing repos.
// v1.5+ flips this to Error. CLI `kron lint` calls RunWith
// with MigrationMode = true; CI in a v1.5+ project passes
// MigrationMode = false.
MigrationMode bool
```

实现细节 ([`internal/lint/lint.go:90-95`](../../../internal/lint/lint.go)):
> `Run(...)` defaults `MigrationMode = true` so the v1.3 → v1.5 migration window remains non-blocking for existing repos. v1.5+ callers (CI in upgraded projects) pass `MigrationMode = false` explicitly.

**软化 2 — inline `text` 字段** (PR2 `67369c6` Option B1 落地):

`internal/model/intent.go:75-83` 的 `model.Assumption` struct:
```go
type Assumption struct {
    ID string `yaml:"id"`
    // ...
    // Text is a human-readable description of the precondition.
    // Optional; if empty, callers should read from the assumption registry via ID.
    Text string `yaml:"text,omitempty"`
    // ...
}
```

`parser/frontmatter.go:170-188` 的 `knownFrontmatterFields` 集合: `"text"` 还在,parser 接受 inline `text:` 写入。

`internal/lint/assumption_rules.go` 走 `RuleAssumptionInline` Warning 而非 Error:
- 检测到 inline `text:` → **Warning** (`parser.HasInlineAssumptionText` raw-bytes pre-scan)

`cmd/kron/serve-mcp/tools_list_get.go` 的 `assumesToWire` 兜底:
- 独立文件没写时 → **fallback 用 intent frontmatter 的 inline `text`**,`default_severity` 字段空着

[`kill-inline-assumptions.md` §2.1](../../process/kill-inline-assumptions.md) 拍板的"软化":
> 折中: parser 接受 (含 round-trip), lint 推迁移 (RuleAssumptionInline, Warning, 非阻塞)

### 1.3 Option B1 拍板原文中的"未来窗口"

[`intent-structure.md` §三](../../abstractDesign/intent-structure.md):
> rationale 必填 (≥ 10 字符): ... v1.3 迁移期是 Warning, v1.5 改 Error

[`kill-inline-assumptions.md` §6](../../process/kill-inline-assumptions.md):
> 后续 (G-5 收口, 不在本 PR): kron_add / kron_update 收到 []string 假设 ID 列表时, 对每个 ID 调 assumption.Writer.Create ... 后续 cleanup PR 可单独再做

**两处都是"无截止日期的版本化未来"** — `v1.5` / "后续 cleanup PR" 没拍:
- 谁决定 v1.5 到了?
- 什么条件触发 cleanup PR?
- 如果不到 v1.5 / 不 cleanup 怎么办?

[`AGENTS.md` §5](../../../AGENTS.md) 红线:
> 任何"v1.3+ 拍板 / v1.4+ 评估 / v1.5 收口"等分阶段实施提案**显著拉高**实施成本: (a) 制造跟踪债 (b) 给后续 PR 强加"该实现 X" 的隐性承诺 (c) 与"plan-first, **不**是 stage-first" 流程纪律冲突

**这条红线**直接撞上"v1.3 Warning / v1.5 Error" / "后续 cleanup PR"。

### 1.4 历史: PR1 (commit `0649cee`) 试过硬删 — 回退

[`kill-inline-assumptions.md` §0.1](../../process/kill-inline-assumptions.md):
> PR1 走的是"完全删 inline"路线: 删 model.Assumption struct, 删 Frontmatter.Assumptions 字段, 改 MCP wire 类型 (删 AssumeEntry / AssumeWarning / AssumeCheckSummary)。commit `0649cee` 已落地, 但后续 lint orphan 规则**完全无信号源** — intent body 既不引用 assumption id, frontmatter 也没了, registry 是孤岛。**回退 PR1 改用 Option B1**。

**回退原因** (回看): 不是"硬删"方向错,是**删除面太广** — `model.Assumption` 整体删 + `Frontmatter.Assumptions` 整体删 + 改 MCP wire = 一刀切导致 orphan 无信号。

**正确做法** (本 RFC): 删 `Text` 单字段 (不删 `model.Assumption` struct) + 删 parser whitelist 单项 (`text`) + 升 `RuleAssumptionInline` Error — **保留** `id / severity / rationale / expires / verified` per-intent 元数据,给 orphan 规则保留信号源 (frontmatter 引用 + body marker 双路径仍然在)。

## 2 拍板

### 2.1 删除 `RunOptions.MigrationMode` — rationale 必填从永久 Warning 升 Error

| 维度 | 现状 | 本 RFC 拍板 |
|---|---|---|
| `RunOptions.MigrationMode` 字段 | 存在,`Run()` 默认 `true` | **删** |
| `assumption_rules.go:133-135` `if !opts.MigrationMode` 分支 | 存在,默认 Warning | **删** (无条件 Error) |
| `RuleAssumptionRationaleRequired` severity | `MigrationMode: true` → Warning; `false` → Error | **永久 Error** |
| `lint.Run` 默认值 | `RunOptions{MigrationMode: true}` | `RunOptions{}` (无字段) |
| CI 行为 | 默认 `Run` = Warning (非阻塞) | **Error** (阻塞) |
| CLI 行为 | 默认 `Run` = Warning (非阻塞) | **Error** (阻塞) |
| 文档拍板 | "v1.3 迁移期 Warning, v1.5 Error" | **永久 Error, 取消版本化未来** |

**理由**:
1. 拍板本意是 Error ([`intent-structure.md` §三](../../abstractDesign/intent-structure.md) rationale 必填)
2. "迁移期"为不存在的存量数据开 — RFC §1.2 拍板"无存量迁移"
3. CLI 默认 Warning = **本地 lint 和 CI lint 行为不一致** — 违反 [`architecture.md` §〇 铁律 #5](../../abstractDesign/architecture.md) "CI lint 是唯一门控"
4. `MigrationMode` = **隐式状态** — 违反 [`architecture.md` §〇 铁律 #6](../../abstractDesign/architecture.md)
5. "v1.5 改 Error" 是无截止日期的版本化未来 — 违反 [`AGENTS.md` §5](../../../AGENTS.md) "plan-first, 不是 stage-first"

### 2.2 删除 `model.Assumption.Text` — inline text 写就报错

| 维度 | 现状 | 本 RFC 拍板 |
|---|---|---|
| `model.Assumption.Text` 字段 | 存在,`yaml:"text,omitempty"` | **删** |
| `parser.frontmatterKnownFields` 中 `"text"` 项 | 存在 (parser 接受) | **删** (parser 报错: 未知字段) |
| `RuleAssumptionInline` severity | Warning (推作者清) | **Error** (阻塞) |
| `RuleAssumptionInline` 检测方式 | `parser.HasInlineAssumptionText` raw-bytes pre-scan | **保留** (parser 报错后, raw-bytes 检测可作冗余, 但 Error 来自 parser 而非 lint) |
| `assumesToWire` inline text 兜底 | 独立文件没写 → fallback inline `text`, `default_severity` 空 | **删兜底** (无独立文件 → 报错 `ErrAssumptionNotFound`, 不 fallback) |
| MCP wire 类型 `AssumeEntry.Text` | 从 `model.Assumption.Text` 来 | **删** (从 registry 来) |
| 所有 intent fixture 中的 inline `text:` | 存在 (4 处?) | **清掉, 补独立文件** |
| 文档拍板 | "parser 接受, lint Warning 推" | **parser 拒绝, lint Error 锁** |

**理由**:
1. B-3 RFC §3.4 拍板字段集**没有** `text` — 落地多出字段,直接违反
2. Option B1 §2.1 "A 路径遗留" — 既然 RFC §1.2 拍板"无存量迁移",**没有"遗留"概念**
3. inline `text` 兜底 = registry 不是 single source of truth ([`mcp/backlog.md` G-5](../../../mcp/backlog.md) 已识别)
4. "软化窗口"= "推作者"靠自觉 — 工程纪律失效
5. Option B1 "软推" 已经实施 (`67369c6` 落地),本 RFC 是"软→硬"的升级拍板

### 2.3 不在本 RFC 范围

- `model.AssumptionFrontmatter` (registry 文件字段集) — **不动** (10 字段已对齐 RFC §3.5)
- `Frontmatter.Assumptions` 字段 — **不动** (per-intent 元数据, orphan 规则需要)
- `model.Assumption` 其它字段 (id / severity / rationale / expires / verified) — **不动**
- `@kron:assumes <id>` body marker — **不动** (Option B1 拍板, orphan 规则信号源)
- `RuleAssumptionOrphan` 双路径 (frontmatter ∪ body marker) — **不动**
- MCP 假设写路径 ([`mcp/backlog.md` G-5](../../../mcp/backlog.md)) — **不在本 RFC** (G-5 单独 RFC + PR)
- `RuleAssumptionFileIdMismatch` / `RuleAssumptionSeverityMismatch` / `RuleAssumptionRationaleStale` — **不动**

## 3 实施 (PR3, 沿用 `optimize/mcp` 分支)

### 3.1 改动清单

| 改动 | 文件 | 行数估算 |
|---|---|---|
| 删 `MigrationMode` 字段 | `internal/lint/lint.go` (struct + 注释 + 默认值) | -20 |
| 删 `if !opts.MigrationMode` 分支 | `internal/lint/assumption_rules.go:133-135` | -3 |
| 删 `MigrationMode: true` 5 处测试调用 | `internal/lint/lint_test.go` (527, 557, 580, 615, 793) | -5 |
| 删 `TestRun_AssumptionRationale_MigrationModeFlag` 双值测试 | `internal/lint/lint_test.go:779-800` | -25 |
| 改 rationale-required 测试: 单一 `SeverityError` 断言 | `internal/lint/lint_test.go` | 净 -10 |
| 删 `model.Assumption.Text` 字段 | `internal/model/intent.go:81-83` | -5 |
| 删 `parser.frontmatterKnownFields` 的 `"text"` 项 | `internal/parser/frontmatter.go:178-179` | -1 |
| 改 `RuleAssumptionInline` severity: Warning → Error | `internal/lint/assumption_rules.go` | -1 + 1 |
| 改 `assumesToWire` 兜底: 删 inline text fallback | `cmd/kron/serve-mcp/tools_list_get.go` | -10 |
| 改 MCP wire: 删 `AssumeEntry.Text` (或改从 registry 来) | `cmd/kron/serve-mcp/tools_schema.go` | -5 |
| 改现有 intent fixture: 删 inline `text:`, 补独立文件 | `internal/lint/lint_test.go` (4 处) | +20 (独立文件) -8 (inline) |
| 新增测试: 删 `text` 后整 frontmatter 加载**报错** | `internal/parser/frontmatter_test.go` | +30 |
| 新增测试: 删 `text` 后 `RuleAssumptionInline` Error | `internal/lint/lint_test.go` | +20 |

**预估**: 净 -10 行 (改 > 加), 1 个 PR, 半天工作量。

### 3.2 实施前必跑

```bash
go vet ./...
gofmt -l .
go test ./...
```

### 3.3 实施步骤

1. `optimize/mcp` 分支已开, 1 commit ahead (`67369c6` Option B1), 本 PR 接着加 commit
2. **本 RFC 单独 commit** (docs 先行, 与 PR2 的 `b725b8c` docs 先行一致)
3. 代码改动**一个 commit** (删除面相关, 拆分会丢一致性)
4. 测试 fix **同一个 commit** (改 model 字段 + 改测试必须同步)
5. 跑 §3.2 三件套

### 3.4 关联流程

- 删 `MigrationMode` 字段 = 改 `internal/lint` API → [`docs/process/new-internal-api.md`](../../process/new-internal-api.md) 流程
- 删 `model.Assumption.Text` = 改 `internal/model` API + 改 frontmatter schema → [`docs/process/migrate.md`](../../process/migrate.md) 流程
- 但 §1.4 提到的"plan-first, 不是 stage-first"纪律, 软→硬升级**不**算新增 migration — 是 RFC 拍板的"窗口关闭"

## 4 不需要迁移脚本

跟 B-3 落地同样理由 (RFC §1.2): 本仓库无存量数据, 实施时一次性改完所有 fixture。

CI 强制: 删 `MigrationMode` 后, 任何**含 inline text 的 intent** 都报 Error, 仓库内 0 个这样的文件 (Option B1 落地时 `b725b8c` 之后 4 个 fixture 已清)。

## 5 影响范围摘要

| 维度 | 变化 |
|---|---|
| `internal/lint.RunOptions` | 字段 -1 (`MigrationMode`) |
| `internal/lint.assumption_rules.RationaleRequired` | Warning → Error (永久) |
| `internal/model.Assumption` | 字段 -1 (`Text`) |
| `internal/parser.frontmatterKnownFields` | 集合 -1 (`text`) |
| `RuleAssumptionInline` | Warning → Error |
| `assumesToWire` (MCP) | 删 inline text 兜底 |
| MCP wire `AssumeEntry.Text` | 字段语义: 从 inline 来 → 从 registry 来 (接口不变, 数据源变) |
| CLI `kron lint` 行为 | 默认 Warning → Error (本地 lint 现在阻塞) |
| CI `kron lint` 行为 | 默认 Error (不变) |
| 文档拍板 | "v1.3 Warning / v1.5 Error" → "永久 Error, 取消版本化未来" |
| 测试 | 5 处 `MigrationMode: true/false` 调用删, 4 处 fixture inline `text:` 删, 新增 2 个 Error 锁定测试 |

## 6 与现有 RFC 的关系

### 6.1 与 B-3 RFC 的关系

[`2026-10-08-assumptions-standalone.md`](../archive/2026-10-08-assumptions-standalone.md) 拍板的**字段集** (intent: 6 个, registry: 10 个) **仍是真理**。本 RFC 是**"实施时的软化窗口"**关闭 — 拍板本身不动。

### 6.2 与 Option B1 的关系

[`kill-inline-assumptions.md`](../../process/kill-inline-assumptions.md) §2.1 拍板的"parser 接受 + lint Warning 推"是**软化窗口**的实施方案。本 RFC 升级该拍板为"parser 拒绝 + lint Error 锁"。

**Option B1 拍板原文保留** — 它记录了"为什么软化 + 软化窗口期做了什么" — 实施历史不该被覆盖。本 RFC 是**新拍板**叠加, 不是覆盖。

### 6.3 与 [`mcp/backlog.md` G-5](../../../mcp/backlog.md) 的关系

G-5 = "internal/assumption 只读, MCP 写路径缺失" — 与本 RFC **部分重叠但不重合**:

| 维度 | 本 RFC §2.2 | G-5 |
|---|---|---|
| 删 inline `text` 兜底 | ✅ | ❌ |
| 删 `model.Assumption.Text` 字段 | ✅ | ❌ |
| 升 `RuleAssumptionInline` Error | ✅ | ❌ |
| 加 `kron_assumption_create/update/delete` 写路径 | ❌ | ✅ |

**建议**: G-5 仍走**单独 RFC** (本 RFC 范围已经够大, 不要再叠 G-5 的"写路径设计")。

## 7 替代方案 (供拍板回退)

### 7.1 维持现状 (Option B1 软化)

**不改**, 接受"v1.3 Warning / v1.5 Error" + "parser 接受 + lint Warning 推"。

- 优点: 0 实施成本, 不动任何代码
- 缺点: 拍板与实施偏离持续存在, 隐性承诺持续生效
- 与 [`AGENTS.md` §5](../../../AGENTS.md) 纪律冲突: "文档实施计划是辅助, 不是任务清单"

### 7.2 只删 `MigrationMode`, 保留 inline `text` 软化

- 优点: 一半成本, 修最硬的"隐式状态"问题
- 缺点: 拍板偏离仍存在 (B-3 RFC §3.4 没 text), 软化窗口未关闭

### 7.3 (本 RFC) 两个一起删, 一次性收口

- 优点: 一次性关窗口, 拍板与实施对齐
- 缺点: PR 范围比 7.2 大, 测试改动多

**本 RFC 推荐 7.3** — "plan-first, 不是 stage-first" 纪律要求一次性拍板, 不分阶段软化。

## 8 拍板请求

请确认:
1. RFC §2.1 删 `MigrationMode` 拍板接受? (Y/N)
2. RFC §2.2 删 `model.Assumption.Text` + 升 `RuleAssumptionInline` Error 拍板接受? (Y/N)
3. §3.3 实施步骤 1-5 接受? (Y/N)
4. §6.2 与 Option B1 关系 (叠加, 不覆盖) 接受? (Y/N)

拍板后 commit RFC, 接着 PR3 实施。
