# RFC: Assumption 独立文件架构 (B 路径完整实施)

| 字段 | 值 |
|---|---|
| **状态** | **草案 (2026-10-08, 用户拍板)** |
| **作者** | AI assistant, 经 zhangjun005 委托 |
| **创建日期** | 2026-10-08 |
| **目标版本** | v1.3 |
| **影响范围** | `internal/model/intent.go` / `internal/parser/frontmatter.go` / `internal/lint/lint.go` / `cmd/kron/serve-mcp/handlers_assume_check.go` / `cmd/kron/serve-mcp/tools_list_get.go` / `internal/assumption/` (新包) / 5 文档 |

---

## 1 动机

### 1.1 背景

仓库当前 assumption 有**两条**数据架构**并存**:

| 路径 | 实施状态 |
|---|---|
| **A — 行内** (frontmatter `assumptions[]` 嵌 text/severity/expires_at 全字段) | ✅ 有类型 + 测试，但无 lint/MCP 链路 |
| **B — 独立** (`.kron/assumptions/<id>.md` 独立文件, intent frontmatter 只引用 id) | ⚠️ **半死**: 类型 (`model.AssumptionFile`) + 1 测试有，没人调用，没 reader/writer/lint 链路 |

**根因**: B 是设计意图，但没实施完成。

### 1.2 目标

把 B 路径**完整实施**: `internal/assumption/` 包 → frontmatter schema 改 B-3 格式 → lint/MCP 链路 → 文档。

> **无存量迁移**: 当前 `.kron/intents/` 下无数据，实施 PR 直接全量改 schema，lint 规则一步到位，无兼容层。

### 1.3 非目标

- ❌ A 路径迁移兼容层（本仓库无存量数据）
- ❌ GUI / IDE 插件 / LSP server 接入
- ❌ `references` / `depends_on` 改成独立文件

---

## 2 决策总览

| 项 | 拍板值 | 日期 |
|---|---|---|
| Assumption 物理存储 | `.kron/assumptions/<id>.md` (独立文件) | 2026-10-08 |
| Intent frontmatter 引用形式 | `[{id, severity, rationale}]` (B-3 结构) | 2026-10-08 |
| 新 internal/ 包 | `internal/assumption/` (Reader/Writer) | 2026-10-08 |
| 跨意图共享假设 | ✅ 1 改多跟随 | 2026-10-08 |
| 假设独立 status | ✅ `draft` / `active` / `superseded` | 2026-10-08 |
| 新 lint 规则 | `assumption-registry-id-mismatch` (E) | 2026-10-08 |
| 新 lint 规则 | `assumption-orphan` (W) | 2026-10-08 |
| 新 lint 规则 | `assumption-file-id-mismatch` (E) | 2026-10-08 |
| 新 lint 规则 | `assumption-rationale-required` (E) | 2026-10-08 |
| 新 lint 规则 | `assumption-rationale-stale` (W) | 2026-10-08 |
| 新 lint 规则 | `assumption-severity-mismatch` (W) | 2026-10-08 |

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
id: single-region                          # kebab-case, MUST equal filename
text: 服务仅部署在单 region, 无跨区时钟漂移问题
default_severity: soft                      # hard | soft
created_by: "@zhangjun005"
updated_at: "2026-09-22T10:00:00Z"
status: active                              # draft | active | superseded
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

### 3.3 Intent frontmatter 引用形式 (B-3)

`.kron/intents/auth/jwt.md`:

```yaml
<!-- kron:frontmatter -->
created_by: "@zhangjun005"
updated_at: "2026-09-22T10:00:00Z"
symbol: "auth.RefreshToken"
assumptions:
  - id: single-region
    severity: hard
    rationale: "跨 region 时 token 失效爆炸, 必须保证单 region 部署"
  - id: redis-availability
    severity: soft
    rationale: "Redis 偶尔挂, 自动重试可接受"
<!-- /kron:frontmatter -->
```

### 3.4 Frontmatter.Assumptions 字段语义

`internal/model/intent.go` — B-3 Assumption 字段集:

```go
type Assumption struct {
    ID         string   `yaml:"id"`                      // 必需: 引用 .kron/assumptions/<id>.md
    Severity   Severity `yaml:"severity"`                 // 必需: 本意图级别 (跨意图可不同)
    Rationale  string   `yaml:"rationale"`               // 必需: ≥ 10 字符
    ExpiresAt  string   `yaml:"expires_at,omitempty"`
    VerifiedAt string   `yaml:"verified_at,omitempty"`
    VerifiedBy string   `yaml:"verified_by,omitempty"`
}
```

**为什么用 `{id, severity, rationale}` 而非 `[]string`**: `rationale` / `severity` 强制每个 intent 各自给出，跨意图 severity 差异真支持。

**为什么 rationale 必填**: AI review 时一眼看出"这个 intent 为什么这 severity"。

### 3.5 AssumptionFrontmatter 字段

```go
type AssumptionFrontmatter struct {
    ID              string   `yaml:"id"`                // MUST == filename (without .md)
    Text            string   `yaml:"text"`              // 必需, 跨意图共享
    DefaultSeverity Severity `yaml:"default_severity"`  // 建议默认, 意图可覆盖
    Status          Status   `yaml:"status,omitempty"`  // draft | active | superseded
    CreatedBy       string   `yaml:"created_by"`       // 必需
    UpdatedAt       string   `yaml:"updated_at"`        // 必需, ISO 8601
    ExpiresAt       string   `yaml:"expires_at,omitempty"`
    VerifiedAt      string   `yaml:"verified_at,omitempty"`
    VerifiedBy      string   `yaml:"verified_by,omitempty"`
    Reviewers       []string `yaml:"reviewers,omitempty"`
}
```

**`status: superseded` 的 assumption 不参与 lint / MCP**。

---

## 4 实施路径

### 4.1 Step 1 — 新 internal 包 `internal/assumption/`

走 [`docs/process/internal-pkg.md`](../../process/internal-pkg.md) §3 论证模板。

1. **解决什么问题?** — B 路径 assumption 独立文件 reader/writer
2. **为什么不能放现有包?** — `internal/store` 绑定 intent schema; `internal/parser` 只解析不做 I/O; assumption 有自身生命周期
3. **多少个访问层用?** — CLI (v1.4+ `kron assume add`) + MCP (`kron_assume_check` / `kron_stale`) + LSP (hover, 后续 RFC). 满足"两个以上"门槛
4. **只剩 1 个访问层会拆吗?** — 不会
5. **预计导出函数:**
   - `Reader.List(ctx) ([]*model.AssumptionFile, error)` — 列所有 assumption
   - `Reader.Get(ctx, id string) (*model.AssumptionFile, error)` — 按 id 读
   - `Writer.Write(ctx, *model.AssumptionFile) error` — 写
   - `Writer.Delete(ctx, id string) error` — 软删到 `.kron/.trash/`

### 4.2 Step 2 — 改 `internal/parser/frontmatter.go`

**校验策略 (B-3)**:
- `len(Assumptions) >= 0` (允许空数组)
- 每个元素:
  - `ID` 必填 + `ValidateSlug(id)` + 不能含 `/`
  - `Severity` 必填 + 合法 (`hard` / `soft`)
  - `Rationale` 必填 + ≥ 10 字符

### 4.3 Step 3 — 改 `internal/lint/lint.go` S-class

读 `in.Frontmatter.Assumptions` (id 列表) → `assumption.Reader.Get(ctx, id)` → 判 expired。

```go
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
        continue
    }
    if af.Frontmatter.DefaultSeverity != model.SeverityHard || af.Frontmatter.ExpiresAt == "" {
        continue
    }
    // 判 expired 逻辑
}
```

### 4.4 Step 4 — 改 MCP handlers

**`kron_assume_check`**: 读 `assumption.Reader` → 用 intent 的 `a.Severity` 而非 `default_severity` → 输出 warnings。

**`kron_list` / `kron_get`**: 调 `ar.Get` 拿注册表字段，拼 wire format:

```json
{
  "assumptions": [
    {
      "id": "single-region",
      "text": "服务仅部署在单 region",
      "severity": "hard",
      "default_severity": "soft",
      "rationale": "跨 region 时 token 失效爆炸",
      "expires_at": "2026-12-31",
      "verified_at": "...",
      "status": "active"
    }
  ]
}
```

`a.Severity` 永远优先于 `default_severity`。

### 4.5 Step 5 — 新 lint 规则

| 规则 | 级别 | 说明 |
|---|---|---|
| `assumption-registry-id-mismatch` | Error | intent 引用 id 不在 assumption 注册表 |
| `assumption-orphan` | Warning | assumption 在注册表但无 intent 引用 |
| `assumption-file-id-mismatch` | Error | 文件名 ≠ frontmatter `id:` 字段 |
| `assumption-rationale-required` | Error | `assumptions[].rationale` 缺或 < 10 字符 |
| `assumption-rationale-stale` | Warning | rationale 引用了注册表已改的 `text` 字符串 |
| `assumption-severity-mismatch` | Warning | intent severity 与注册表 default_severity 不同 (纯提示) |

加进 `internal/lint/rules.go`。

### 4.6 Step 6 — 文档同步

| 文件 | 改什么 |
|---|---|
| `docs/abstractDesign/intent-structure.md` | §三 `assumptions` 字段重写 — B-3 格式 |
| `docs/implementation/domain-model.md` | `Frontmatter` 示例 + `AssumptionFile` 关系 |
| `AGENTS.md` | storage format reminder + 目录结构 |
| `docs/business.md` | §1.1 assumption 描述 |
| `README.md` | 相关描述 |

---

## 5 测试 plan

| 测试 | 输入 | 期望 |
|---|---|---|
| `TestAssumptionFile_Get` | `.kron/assumptions/single-region.md` 存在 | 返回全字段 |
| `TestAssumptionFile_Get_NotFound` | id 不存在 | `ErrAssumptionNotFound` |
| `TestAssumptionFile_List` | 3 个 .md | 返回 3 元素切片 (排序) |
| `TestAssumptionFile_Write_IdMismatch` | slug ≠ frontmatter id | `ErrAssumptionFileIdMismatch` |
| `TestParseFrontmatter_AcceptsAssumptionStruct` | `assumptions: [{id, severity, rationale}]` | 解析成功，字段不空 |
| `TestParseFrontmatter_RejectsRationaleTooShort` | `rationale: "hard"` (< 10 字符) | Error |
| `TestParseFrontmatter_RejectsIdEmpty` | `id: ""` | Error |
| `TestRun_AssumptionRegistryIdMismatch` | intent 引用 `nonexistent` id | Diag 含 RuleAssumptionRegistryIdMismatch |
| `TestRun_AssumptionOrphan` | 注册表有但无 intent 引用 | Diag 含 RuleAssumptionOrphan |
| `TestRun_AssumptionSupersededSkipped` | status=superseded 被引用 | S-class 不触发 |
| `TestRun_AssumptionRationaleRequired` | rationale="" | Diag 含 RuleAssumptionRationaleRequired |
| `TestRun_AssumptionSeverityMismatch_WarningOnly` | severity=hard, default=soft | Diag 含 Warning 但非 Error |
| `TestKronAssumeCheck_UsesRegistry` | MCP 调用 | 返回意图 `a.Severity` 而非 `default_severity` |
| `TestKronListGet_AssumesToWire` | 2 个 assumption | wire 含 `id/text/severity/default_severity/rationale` |

---

## 6 PR 拆解

| PR | 内容 | 阻塞 |
|---|---|---|
| PR 1 | **论证 commit** `docs(architecture): propose internal/assumption/` | 无 |
| PR 2 | **`internal/assumption/` 包** (Reader/Writer + 4 单元测试) | PR 1 |
| PR 3 | **`internal/parser/frontmatter.go` 改** + 3 单元测试 | PR 2 |
| PR 4 | **`internal/lint/lint.go` 改** + 6 新 lint 规则 + 4 单元测试 | PR 3 |
| PR 5 | **MCP handlers 改** (`handlers_assume_check.go` + `tools_list_get.go`) + 3 集成测试 | PR 4 |
| PR 6 | **文档同步** (intent-structure.md / domain-model.md / AGENTS.md / business.md / README.md) | PR 5 |

**预计 6 个 PR**。

---

## 7 不在本 RFC 范围

- ❌ GUI assumption 编辑器 UI
- ❌ LSP hover
- ❌ `kron assume add` CLI 子命令 (v1.4 评估)
- ❌ A 路径迁移兼容层（本仓库无存量）

---

## 8 关键决策 (不可回退)

1. Assumption 物理存储 = `.kron/assumptions/<id>.md`
2. Intent frontmatter = `[{id, severity, rationale}]` (B-3)
3. 新包 = `internal/assumption/` (Reader/Writer)
4. `AssumptionFrontmatter.Severity` → `DefaultSeverity`
5. `rationale` 必填 + ≥ 10 字符
6. 6 个新 lint 规则 (见 §4.5)
