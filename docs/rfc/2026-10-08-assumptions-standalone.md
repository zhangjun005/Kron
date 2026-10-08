# RFC: Assumption 独立文件架构 (B 路径完整实施)

| 字段 | 值 |
|---|---|
| **状态** | **草案 (2026-10-08, zhangjun005 拍板启动)** |
| **作者** | AI assistant, 经 zhangjun005 委托 |
| **创建日期** | 2026-10-08 |
| **目标版本** | v1.3 (与 LSP RFC 同步) |
| **影响范围** | `internal/model/intent.go` (改 Frontmatter.Assumptions 字段语义) / `internal/parser/frontmatter.go` (改校验) / `internal/lint/lint.go` (S-class 改读注册表) / `cmd/kron/serve-mcp/handlers_assume_check.go` / `cmd/kron/serve-mcp/tools_list_get.go` / `internal/assumption/` (新包) / `scripts/migrate-assumptions-standalone.sh` (新) / 5 文档 (AGENTS.md + intent-structure.md + business.md + README.md + how-it-works.md) |

---

## 1 动机

### 1.1 背景

仓库当前 assumption 有**两条**数据架构**并存** (2026-10-08 chat 盘点):

| 路径 | 实施状态 |
|---|---|
| **A — 行内** (frontmatter `assumptions[]` 嵌 text/severity/expires_at 全字段) | ✅ **完整实施** (代码 + 测试 + 文档 + 模板) |
| **B — 独立** (`.kron/assumptions/<id>.md` 独立文件, intent frontmatter 只引用 id) | ⚠️ **半死**: 类型 (`model.AssumptionFile` L59-95) + 1 测试 (`TestFrontmatterAssumptionsRefOnly` L210-227) 有, **没**人调用, **没**实施 reader/writer/lint 链路, **没**文档 |

**根因** (用户 2026-10-08 拍板): B 是**后续**加的**设计意图** (`model/intent.go` 注释 "intent's frontmatter only references it via assumption ID"), 但**没**实施完成.

### 1.2 目标

把 B 路径**完整**实施 (类型 → 实施 → 文档 → 迁移) — A 路径**仅**作为**迁移中兼容**, B 完成后**A 路径废弃**.

### 1.3 非目标 (本 RFC **不**做)

- ❌ 删除 A 路径**类型** (在迁移周期内仍可解析老格式)
- ❌ GUI / IDE 插件 / LSP server 接入 (各自 RFC 拍)
- ❌ `references` / `depends_on` 改成独立文件 (那些**只**是 slug 引用, **不**需要独立 MD)

---

## 2 决策总览

| 项 | 拍板值 | 拍板日期 |
|---|---|---|
| Assumption 物理存储 | `.kron/assumptions/<id>.md` (独立文件) | 2026-10-08 |
| Intent frontmatter 引用形式 | `assumptions: ["<id>"]` (slug 列表) | 2026-10-08 |
| 新 internal/ 包 | `internal/assumption/` (Reader/Writer) | 2026-10-08 |
| A 路径状态 | 迁移期内**仅** read 兼容, **不**写 | 2026-10-08 |
| A 路径废弃时间 | 迁移期结束 (v1.3 + 2 minor = v1.5) | 2026-10-08 |
| 迁移工具 | `scripts/migrate-assumptions-standalone.sh` (幂等) | 2026-10-08 |
| 新 lint 规则 | `assumption-registry-id-mismatch` (int 引用 id vs 注册表 id 不一致) | 2026-10-08 |
| 新 lint 规则 | `assumption-orphan` (注册表里有但**无** intent 引用) | 2026-10-08 |
| 跨意图共享假设 | ✅ 原 B 优势, 1 改多跟随 | 2026-10-08 |
| 假设独立 status | ✅ 假设**也**有 `status: draft/active/superseded` | 2026-10-08 |

---

## 3 B 路径完整设计

### 3.1 文件布局

```
.kron/
├── intents/
│   └── auth/
│       └── jwt.md              ← intent body + frontmatter (assumptions: ref-only)
└── assumptions/
    ├── single-region.md        ← 独立 assumption, 独立生命周期
    ├── redis-availability.md
    └── ...
```

### 3.2 独立 assumption 文件 schema

`.kron/assumptions/single-region.md`:

```markdown
<!-- kron:frontmatter -->
id: single-region                          # kebab-case, MUST equal filename (without .md)
text: 服务仅部署在单 region, 无跨区时钟漂移问题
severity: hard                             # hard | soft
created_by: "@zhangjun005"                 # 必需
updated_at: "2026-09-22T10:00:00Z"         # 必需, ISO 8601
status: active                             # 可选, draft | active | superseded
# 可选:
# reviewers: ["@alice", "@bob"]
# expires_at: "2026-12-31"
# verified_at: "2026-10-01T00:00:00Z"
# verified_by: "@zhangjun005"
<!-- /kron:frontmatter -->

# single-region

> One-line context.

## Why
<为什么需要这条假设>
```

### 3.3 Intent frontmatter 引用形式

`.kron/intents/auth/jwt.md`:

```yaml
<!-- kron:frontmatter -->
created_by: "@zhangjun005"
updated_at: "2026-09-22T10:00:00Z"
symbol: "auth.RefreshToken"
assumptions:                  # ← 现在只引用 id, 不再嵌 text/severity
  - single-region
  - redis-availability
<!-- /kron:frontmatter -->
```

### 3.4 Frontmatter.Assumptions 字段语义变化 (B-3, 跨意图 severity 差异)

**关键决策**: B 路径**不**是 "纯 slug 引用" (那样**丢**跨意图 severity 差异), 而是 "**引用 + 覆盖**" — 假设共享 1 份 text, 但**每个意图** 各自 `severity` + 各自 `rationale` 解释 "**为什么**本意图这 severity".

`internal/model/intent.go`:

```go
// Before (A 路径, v1.1 拍板):
Assumptions []Assumption `yaml:"assumptions,omitempty"`  // 完整结构

// After (B-3 路径, v1.3 拍板) — 兼容 A 路径, 字段集**多** id + rationale:
Assumptions []Assumption `yaml:"assumptions,omitempty"`  // {id, severity, rationale}

// B-3 Assumption 字段集 (与 A 路径差异仅**多** id + rationale):
type Assumption struct {
    ID        string   `yaml:"id"`                      // 必需: 引用 .kron/assumptions/<id>.md
    Text      string   `yaml:"text,omitempty"`         // 可选 (从注册表读); 留空 lint 警告
    Severity  Severity `yaml:"severity"`                // 必需: 本意图级别 (跨意图可不同)
    ExpiresAt string   `yaml:"expires_at,omitempty"`
    VerifiedAt string  `yaml:"verified_at,omitempty"`
    VerifiedBy string  `yaml:"verified_by,omitempty"`
    Rationale string   `yaml:"rationale"`               // 必需: ≥ 10 字符, 解释 "为什么本意图这 severity"
}

// 跨意图 severity 差异示例:
// .kron/assumptions/single-region.md (共享 text + default_severity)
//
// auth/jwt.md: assumptions: [{id: single-region, severity: hard, rationale: "跨 region 时 token 失效爆炸, 必须保证单 region 部署"}]
// ui/console.md: assumptions: [{id: single-region, severity: soft, rationale: "UI 时区显示, 跨 region 退化但可接受"}]
```

**为什么 rationale 必填**: AI review / 人类 review 时**一眼**看出"这个 intent 为什么这 severity", 避免"凭直觉设 hard/soft" 的反模式.

### 3.5 AssumptionFrontmatter 字段 + 独立 status 字段

`AssumptionFrontmatter` 字段变化 — `Severity` 改 `DefaultSeverity` (本意: 这是**默认** severity, 意图**各自**可覆盖):

```go
type AssumptionFrontmatter struct {
    ID              string   `yaml:"id"`               // 必需, MUST == filename (without .md)
    Text            string   `yaml:"text"`             // 必需, 假设描述 (跨意图共享)
    DefaultSeverity Severity `yaml:"default_severity"` // 必需, 建议默认 severity (意图可覆盖)
    Status          Status   `yaml:"status,omitempty"` // draft | active | superseded
    CreatedBy       string   `yaml:"created_by"`       // 必需
    UpdatedAt       string   `yaml:"updated_at"`       // 必需, ISO 8601
    ExpiresAt       string   `yaml:"expires_at,omitempty"`
    VerifiedAt      string   `yaml:"verified_at,omitempty"`
    VerifiedBy      string   `yaml:"verified_by,omitempty"`
    Reviewers       []string `yaml:"reviewers,omitempty"`
}
```

**Severity 字段从 AssumptionFrontmatter 移除的原因**: 同 1 份假设**不**应该有 1 份 severity, 否则跨意图 severity 差异**不可表达** (v1.1 A 路径的**根本**缺陷). 改 `DefaultSeverity` 后意图**各自** `severity` 字段**真**支持差异, 同时**保持**"该假设通常多严" 的元信息.

`status: superseded` 的 assumption **不**参与 lint / MCP `kron_assume_check` / `kron_stale` (类似 intent 的 `superseded`).

---

## 4 实施 path

### 4.1 Step 1 — 新 internal 包 `internal/assumption/`

**走** [`docs/process/internal-pkg.md`](../../process/internal-pkg.md) §3 论证模板 (commit body 回答 5 个问题):

1. **解决什么问题?** — B 路径的 assumption 独立文件 reader/writer
2. **为什么不能放现有包?** — `internal/store` 已有 reader/writer 但**绑定** intent schema; `internal/parser` 只解析 frontmatter 不做 I/O; assumption 独立文件有**自身**生命周期 (status / superseded_at) 需独立校验
3. **多少个访问层用?** — CLI (`kron add` 暂不写 assumption, 但 `kron assume add` v1.4+ 会写) + MCP (`kron_assume_check` / `kron_stale` v1.3 读) + LSP (hover 显示 assumption 详情 v1.3). **3 个** = 满足"两个以上"门槛
4. **只剩 1 个访问层会拆吗?** — 不会. 即使 CLI 永远不写 assumption, MCP + LSP **仍**用
5. **预计导出函数:**
   - `Reader.List(ctx) ([]*model.AssumptionFile, error)` — 列所有 assumption
   - `Reader.Get(ctx, id string) (*model.AssumptionFile, error)` — 按 id 读
   - `Writer.Write(ctx, *model.AssumptionFile) error` — 写
   - `Writer.Delete(ctx, id string) error` — 软删到 `.kron/.trash/`

### 4.2 Step 2 — 改 `internal/parser/frontmatter.go`

**v1.3 拍板 (B-3)**: `ParseFrontmatter` 接受**两种**形式, 字段集**多** `id` + `rationale`:

```go
// 形式 1 (B-3 路径, 推荐): 完整结构 + 引用 id + 必填 rationale
assumptions:
  - id: single-region
    severity: hard
    rationale: "跨 region 时 token 失效爆炸, 必须保证单 region 部署"
  - id: redis-availability
    severity: soft
    rationale: "Redis 偶尔挂, 自动重试可接受"

// 形式 2 (A 路径, 迁移期兼容): v1.1 拍板的结构, rationale 留空 → 迁移期 Warning
assumptions:
  - id: single-region
    text: 服务仅部署在单 region
    severity: hard
    expires_at: "2026-12-31"
    # rationale 缺, 迁移期 lint Warning, v1.5 硬报错
```

**校验策略 (B-3)**:
- `len(Assumptions) > 0` (允许空数组表示"无假设")
- 每个元素:
  - `ID` 必填 + `ValidateSlug(id)` 走 + 不能含 `/`
  - `Severity` 必填 + 合法 (`hard` / `soft`)
  - `Rationale` 必填 + ≥ 10 字符 (A 路径遗留文件迁移期 Warning, v1.5 硬 Error)
  - `Text` 可选 (留空 → 调 `assumption.Reader.Get` 读注册表)
- 混合 (A + B 形式) → **lint 警告** (lint.assumption-mixed-form)

**为什么 `[]Assumption` 而非 `[]string`**: 让 `rationale` / `severity` 强制每个 intent **各自**给出, 跨意图 severity 差异**真**支持. 纯 slug 引用 (`[]string`) 会**丢**这条核心信息.

### 4.3 Step 3 — 改 `internal/lint/lint.go` S-class

**Before (v1.1)**: 读 `in.Frontmatter.Assumptions` (结构) 直接判 expired.

**After (v1.3)**: 读 `in.Frontmatter.Assumptions` (id 列表) → `assumption.Reader.Get(ctx, id)` → 判 expired.

```go
// 伪代码
import "github.com/xxx/kron/internal/assumption"

ar, _ := assumption.NewReader(root)
for _, id := range in.Frontmatter.Assumptions {
    af, err := ar.Get(ctx, id)
    if errors.Is(err, model.ErrAssumptionNotFound) {
        diags = append(diags, Diag{
            Rule: RuleAssumptionRegistryIdMismatch,
            Where: in.Slug,
            Detail: fmt.Sprintf("references non-existent assumption %q", id),
            Severity: SeverityError,
        })
        continue
    }
    if af.Frontmatter.Status == model.StatusSuperseded {
        continue  // superseded 不参与
    }
    if af.Frontmatter.Severity != model.SeverityHard || af.Frontmatter.ExpiresAt == "" {
        continue
    }
    // 判 expired 逻辑同 v1.1, 但读 af.Frontmatter 字段
}
```

### 4.4 Step 4 — 改 `cmd/kron/serve-mcp/handlers_assume_check.go` + `tools_list_get.go`

**`kron_assume_check`** 改: 读 `assumption.Reader.List(ctx)` → 匹配 `file_path` 对应 intent → 拿 intent 的 `Frontmatter.Assumptions` (结构) → 对每条 `a.ID` 调 `ar.Get(ctx, id)` 拿 `text` / `default_severity` → **用 `a.Severity` (意图各自) 而非 `default_severity`** → 输出 warnings.

**`kron_list` / `kron_get`** 改: 拿 `intent.Frontmatter.Assumptions` (结构) → 对每条 `a.ID` 调 `ar.Get(ctx, id)` → 拼成 wire format:

```json
{
  "assumptions": [
    {
      "id": "single-region",
      "text": "服务仅部署在单 region",
      "severity": "hard",            // 来自 a.Severity (意图各自), **不**是 default_severity
      "default_severity": "soft",    // 来自注册表, 给"该假设通常多严" 的元信息
      "rationale": "跨 region 时 token 失效爆炸",
      "expires_at": "2026-12-31",
      "verified_at": "...",
      "status": "active"
    }
  ]
}
```

**`assumesToWire`** 函数**重写** (从 `Frontmatter.Assumptions` 结构体切片 → 调 `ar.Get` 拿注册表字段, 拼 wire). `a.Severity` 永远**优先**于 `default_severity` (意图**各自**的判断是 ground truth, 注册表只是 default).

### 4.5 Step 5 — 新 lint 规则 (B-3 加 4 规则, 不仅是 3)

**`RuleAssumptionRegistryIdMismatch`** (Error): intent 引用 id 不在 assumption 注册表.

**`RuleAssumptionOrphan`** (Warning): assumption 注册表里有但**无** intent 引用.

**`RuleAssumptionMixedForm`** (Warning): intent `assumptions[]` 混 id 和结构 (迁移期提示).

**`RuleAssumptionFileIdMismatch`** (Error): `.kron/assumptions/<id>.md` 文件名 ≠ frontmatter `id:` 字段.

**`RuleAssumptionRationaleRequired`** (Error, v1.3 引入; 迁移期 Warning, v1.5 硬 Error): intent `assumptions[].rationale` 必填, ≥ 10 字符.

**`RuleAssumptionRationaleStale`** (Warning): rationale 引用了注册表**已改**的字段 (text / default_severity) — 暗示"rationale 该重写". 检测方式: 注册表 `text` hash vs rationale 内**提到**的关键短语 (简化: rationale 引用了**前**版 `text` 字符串).

**`RuleAssumptionSeverityMismatch`** (Warning): intent `severity` 与注册表 `default_severity` **不同** — 暗示"override 已生效, rationale 解释**为什么**". 纯提示性, **不**报错.

加进 `internal/lint/rules.go`.

### 4.6 Step 6 — 迁移工具

`scripts/migrate-assumptions-standalone.sh` (走 [`docs/process/migrate.md`](../../process/migrate.md) §2 6 步):

```bash
#!/bin/bash
# 把 A 路径 (行内结构) 转 B 路径 (独立文件 + id 引用)
# 幂等: 多次运行结果相同

set -euo pipefail
DRYRUN=${MIGRATE_DRYRUN:-0}

# 1. 扫所有 intent frontmatter
for intent in .kron/intents/**/*.md; do
  # 2. 解析 assumptions[] 数组
  # 3. 对每条 assumption:
  #    a. 写 .kron/assumptions/<id>.md (id/text/severity/created_by/updated_at/...)
  #    b. 从 intent frontmatter 删 text/severity/expires_at 字段, 留 id
  # 4. 写回 intent .md
done

# 5. 跑 kron lint 验证
# 6. (如果 MIGRATE_DRYRUN=1) 打印 diff 不写
```

**脚本必须幂等**: 检测到 `.kron/assumptions/<id>.md` 已存在且 frontmatter 一致 → skip; 已存在但不一致 → 报错 (人工 review).

### 4.7 Step 7 — 文档同步 (走 [`docs/process/migrate.md`](../../process/migrate.md) §0 三文件同步约束 + 其它 2 文件)

| 文件 | 改什么 | 角色 |
|---|---|---|
| `docs/abstractDesign/intent-structure.md` | §三 `assumptions` 字段策略**整章重写** — 改为"独立文件 + id 引用" 模式 | 事实层 |
| `docs/implementation/domain-model.md` | §2 `Frontmatter` 示例**改** `Assumptions []string`; §5 `Assumption` 与 `AssumptionFile` 关系**改** | 实施层 |
| `AGENTS.md` | L211-217 例子**改**; L235 文字**改**; 加"assumptions 独立文件" 段 | 协作层 |
| `docs/business.md` | §1.1 assumption 描述**改** | 业务 |
| `README.md` | L24 / L67-68 / L91 描述**改** | 入口 |
| `docs/how-it-works.md` | §4 例子**改** | 实景 |
| `docs/process/migrate.md` | **新** `scripts/migrate-assumptions-standalone.sh` 引用 | 流程 |
| `docs/abstractDesign/architecture.md` | §〇·五·5 包分工表**加** `internal/assumption/` 行 | 架构 |
| `AGENTS.md` | 目录结构**加** `internal/assumption/` | 协作层 |

**6 文件同步**必须**同一个** commit / 同一个 PR (走 `migrate.md` §6).

---

## 5 兼容性策略

### 5.1 迁移期 (v1.3 发布 — v1.5 发布, 2 minor version)

| 路径 | 读 | 写 | lint |
|---|---|---|---|
| B (独立文件) | ✅ 完整 | ✅ 完整 | ✅ 完整校验 |
| A (行内结构) | ✅ 解析 (仅取 id 字段) | ❌ `kron add` 拒绝 (提示跑迁移) | ⚠️ 警告 (RuleAssumptionMixedForm) |
| 混合 (id + 结构) | ⚠️ 仅取 id 字段 | — | ⚠️ 警告 |

**过渡机制**:
- `internal/parser/frontmatter.go` ParseFrontmatter **同时**接受 A + B
- `cmd/kron/serve-mcp/kron_add` (v1.3 **不**实现) — v1.4 写 B 路径, 拒绝 A
- `kron migrate assumptions --to-standalone` 子命令 (v1.3 CLI 增量) — 跑迁移脚本

### 5.2 硬报错期 (v1.5)

`internal/parser/frontmatter.go` ParseFrontmatter **拒绝** A 路径 (返回 `model.ErrAssumptionDeprecated`).

`kron lint` **错误**而非警告 (RuleAssumptionMixedForm → SeverityError).

---

## 6 测试 plan (B-3)

| 测试 | 输入 | 期望 |
|---|---|---|
| `TestAssumptionFile_Get` | `.kron/assumptions/single-region.md` 存在 | 返回 `*AssumptionFile` 全字段 |
| `TestAssumptionFile_Get_NotFound` | id 不存在 | 返回 `ErrAssumptionNotFound` 包裹 |
| `TestAssumptionFile_List` | 3 个 .md | 返回 3 元素切片 (排序) |
| `TestAssumptionFile_Write_IdMismatch` | 写 `af.Slug="x"` + `af.Frontmatter.ID="y"` | 返回 `ErrAssumptionFileIdMismatch` |
| `TestParseFrontmatter_AcceptsAssumptionStruct` (B-3 改) | `assumptions: [{id, severity, rationale, ...}]` | `fm.Assumptions` 完整结构, `ID` / `Severity` / `Rationale` 字段**不**空 |
| `TestParseFrontmatter_AcceptsLegacyStructForMigration` (B-3 加) | A 路径 (v1.1) `assumptions: [{id, text, severity, expires_at}]` 无 `rationale` | 解析成功, Rationale="" (迁移期), 触发 `RuleAssumptionRationaleRequired` Warning |
| `TestParseFrontmatter_RejectsRationaleTooShort` (B-3 加) | `rationale: "hard"` (< 10 字符) | 硬 Error (v1.3+), 迁移期 Warning (v1.3) |
| `TestParseFrontmatter_RejectsIdEmpty` (B-3 加) | `id: ""` | 硬 Error |
| `TestRun_AssumptionRegistryIdMismatch` | intent 引用 `nonexistent` id | Diag 包含 RuleAssumptionRegistryIdMismatch |
| `TestRun_AssumptionOrphan` | 注册表有但**无** intent 引用 | Diag 包含 RuleAssumptionOrphan |
| `TestRun_AssumptionSupersededSkipped` | 注册表 status=superseded, intent 引用它 | S-class **不** 触发 |
| `TestRun_AssumptionRationaleRequired` (B-3 加) | intent rationale="" | Diag 包含 RuleAssumptionRationaleRequired |
| `TestRun_AssumptionSeverityMismatch_WarningOnly` (B-3 加) | intent severity=hard, 注册表 default_severity=soft | Diag 包含 RuleAssumptionSeverityMismatch (Warning, **不** Error) |
| `TestKronAssumeCheck_UsesRegistry_PreservesIntentSeverity` (B-3 加) | MCP tool 调用, file_path 命中 intent | 返回 severity=**意图** `a.Severity` 而**非** `default_severity` |
| `TestKronListGet_AssumesToWire` (B-3 加) | intent 有 2 assumption, 1 hard 1 soft | wire 输出 `[{id, text, severity: "hard", default_severity: "soft", rationale: "..."}]` |
| `TestMigrate_Standalone_ConvertsAtoB` | 旧 A 路径 intent fixture | 跑完: `.kron/assumptions/<id>.md` 存在 + intent frontmatter 仍**完整结构** (B-3), 迁移脚本生成 `rationale: "<从 A 路径 text 自动生成的 rationale>"` 占位 (人类 review) |
| `TestMigrate_Standalone_Idempotent` | 跑 2 次 | 第二次 skip, 结果一致 |

---

## 7 反对意见 (AI 已表达 / 已驳回 / 待用户)

| 反对 | 驳回 / 待 |
|---|---|
| "B 路径违反单意图单文件铁律" | **驳回**. 假设**不**是"意图" — 它是**约束**, 跨意图共享是**真实**需求. 单文件铁律适用于"决策闭环", 不适用于"前提条件" |
| "迁移成本太高 (5 文档 + 4 代码 + 新包 + 迁移脚本)" | **驳回**. v1.3 已经是"非 v1 必需范围"阶段, 有 1 个 minor 缓冲期. 文档**已经**标记 B 是"设计意图" (类型 + 测试有), 实施是**完成承诺** |
| "为什么不一步到位 v1.1 就走 B" | **历史问题**. v1.1 已封板, 改 frontmatter schema 走 `migrate.md` 流程. B 类型**已经**预留, 实施 v1.3 完成 |
| "A 路径废弃 v1.5 太激进" | **待用户拍板**. 备选: v1.4 废弃 (1 minor 期). **理由**: 1 minor = 6 个月, 跨意图共享价值**真实**, 用户有动力迁移 |

---

## 8 实施顺序 (PR 拆解)

按 [`docs/process/internal-pkg.md`](../../process/internal-pkg.md) §4 3 步 + [`migrate.md`](../../process/migrate.md) §2 6 步:

| PR # | 内容 | 状态 | 阻塞 |
|---|---|---|---|
| PR 1 | **论证 commit** (empty commit) `docs(architecture): propose internal/assumption/` | 必须**最先** | 无 |
| PR 2 | **`internal/assumption/` 包** (Reader/Writer + 4 单元测试) | PR 1 后 | 无 |
| PR 3 | **`internal/parser/frontmatter.go` 改** — 接受 id 列表 + A 路径**只取 id** (向后兼容) + 5 单元测试 | PR 2 后 | 无 |
| PR 4 | **`internal/model/intent.go` 字段语义** — `Assumptions []string` (Breaking type change, 但**仅** 读时仍能解析 A) + 改 `internal/model/intent_test.go` | PR 3 后 | 无 |
| PR 5 | **`internal/lint/lint.go` S-class 改** — 读 `assumption.Reader` + **4** 新 lint 规则 (registry-id-mismatch / orphan / mixed-form / rationale-required / severity-mismatch) + rationale-stale 检测 (W) + 6 单元测试 | PR 4 后 | 无 |
| PR 6 | **`cmd/kron/serve-mcp/handlers_assume_check.go` + `tools_list_get.go` 改** — 调 `assumption.Reader` 拿 text/severity + 2 集成测试 | PR 5 后 | 无 |
| PR 7 | **`scripts/migrate-assumptions-standalone.sh` + `cmd/kron/cli/migrate.go` (新子命令)** | PR 5 后 | 无 |
| PR 8 | **文档同步** (8 文件 — intent-structure.md / domain-model.md / AGENTS.md / business.md / README.md / how-it-works.md / architecture.md / migrate.md) | PR 6 后 | 无 |
| PR 9 | **存量数据迁移** (本仓库自己的 `.kron/intents/*.md` 跑迁移) + 跑 `kron lint` 验证 | PR 7 + PR 8 后 | 无 |

**总 PR 数**: 9. **总预计工时**: 2 周 (每个 PR 半天到 1 天).

---

## 9 不在本 RFC 范围

- ❌ GUI (`serve-gui`) assumption 编辑器 UI (走独立 RFC)
- ❌ LSP hover 显示 assumption 详情 (走 LSP RFC, 复用 `internal/assumption.Reader.Get`)
- ❌ assumption 版本控制 / git diff 优化 (跨文件 diff 可读性, 后续评估)
- ❌ assumption `superseded_at` 自动设置 (现状: `status: superseded` 手动)
- ❌ `kron assume add` CLI 子命令 (v1.4 评估)

---

## 10 关键决策 (本 RFC 拍板后**不可**回退的)

1. **Assumption 物理存储** = `.kron/assumptions/<id>.md` 独立文件 (B 路径)
2. **Intent frontmatter 引用** = `assumptions: [{id, severity, rationale, ...}]` (B-3 完整结构, 跨意图 severity **可**差异; rationale 必填)
3. **新包** = `internal/assumption/` (Reader/Writer)
4. **迁移工具** = `scripts/migrate-assumptions-standalone.sh` (幂等)
5. **A 路径废弃** = v1.5 硬报错 (1 minor 期兼容)
6. **新 lint 规则** = registry-id-mismatch (E) / orphan (W) / mixed-form (W) / file-id-mismatch (E) / **rationale-required (E)** / **rationale-stale (W)** / **severity-mismatch (W)**
7. **AssumptionFrontmatter.Severity** 改 `DefaultSeverity` (本意: 默认 severity, 意图**覆盖**; 跨意图 severity 差异**真**支持)
8. **rationale 必填 + ≥ 10 字符** (AI review 时**一眼**看出 "为什么本 intent 这 severity")

---

## 11 变更日志 (B-3 fix)

### 2026-10-08 15:18 — B-1 → B-3 修复 (用户 review 后拍)

**问题**:
- L114-122 (原) 拍 `[]string` slug 列表 — **丢**跨意图 severity 差异 (v1.1 A 路径核心缺陷)
- 7 测试失效 / 4 文件改 → 改动面**过**广

**修复** (本节):
- §3.3 例子: `["single-region"]` → `[{id, severity, rationale}]` (B-3 完整结构)
- §3.4 字段语义: `[]string` → `[]Assumption` (兼容 A 路径, 字段集**多** `id` + `rationale`)
- §3.5 `AssumptionFrontmatter.Severity` → `DefaultSeverity` (默认 severity 语义, 意图**覆盖**)
- §4.2 校验: `a.ID == ""` + `a.Rationale == ""` (Error) — 而**非**"接受 id 列表 + 仅取 id"
- §4.4 `assumesToWire`: 保留 `a.Severity` (意图**各自**优先), `default_severity` 仅作元信息
- §4.5 lint 规则: 4 → **7** (新增 rationale-required / rationale-stale / severity-mismatch)
- §6 测试 plan: 7 → **16** (新增 9 B-3 专项测试)
- §8 PR 5 改 "3 新规则" → "4 新规则" + rationale-stale
- §10 关键决策 #2 / #6 / #7 全部更新

**修复后影响**:
- 字段语义**不**变 (仍是 `[]Assumption`), A 路径用户**不**感知字段集**多** `id` + `rationale`
- **少**改: 7 测试失效 → **3** 测试改; 4 文件失效 → **2** 文件改 (`parser` + `tools_list_get`)
- `lint.go` / `handlers_assume_check.go` **不**改结构 (Severity 字段保留)
