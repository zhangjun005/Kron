# 领域模型实现参考

> 来源：`docs/abstractDesign/architecture.md` §3  
> 本文件是**实施建议**，不是架构真理。结构体字段如有变更，请同步修改此处。

---

## 1 `Intent`

```go
// Intent is one design intent, persisted as .kron/intents/<slug>.md.
type Intent struct {
    Slug        string      // 文件相对路径（相对 .kron/intents/），不含 .md
    Frontmatter Frontmatter
    Body        string      // Markdown 正文（不含 frontmatter）
    SourcePath  string      // 磁盘绝对路径；store 层注入，不参与落盘序列化
}
```

**注意**：`Slug` 由调用方（`kron add` / MCP）保证唯一性，store 层不额外校验唯一约束。

---

## 2 `Frontmatter`

```go
type Frontmatter struct {
	Symbol      []string    `yaml:"symbol,omitempty"`     // 关联代码符号列表
	CreatedBy   string      `yaml:"created_by"`           // "@user" 或 "agent:<model>"
	UpdatedAt   time.Time   `yaml:"updated_at"`           // ISO 8601
	Reviewers   []string    `yaml:"reviewers,omitempty"`  // 可选；多协作项目使用
	Status      Status      `yaml:"status,omitempty"`     // draft / active / superseded
	Assumptions []Assumption `yaml:"assumptions,omitempty"` // 可选；可验证前提
	References  []string    `yaml:"references,omitempty"`  // 可选；软引用关系（v1.2+）
	DependsOn   []string    `yaml:"depends_on,omitempty"` // 可选；硬依赖关系（v1.2+）
}
```

**注意**：`Status` 完全可选；不填 = 不参与生命周期管理。
`Assumptions` 完全可选；不填 = "该 intent 无显式假设"（见 [intent-structure.md §三](../abstractDesign/intent-structure.md)）。B-3（v1.3+）：`assumptions[]` 是 `{id, severity, rationale, ...}` 引用形式；共享 `text` 存于 `.kron/assumptions/<id>.md` 的 `AssumptionFile.Text`（见 [§5](#5-assumption-与-assumptionfileb-3-v13)）。`expires_at` / `verified_at` / `verified_by` 全部可选，写在 intent 上（**不**写 registry 文件）。

**`References` / `DependsOn`（v1.2+，2026-10-03 落地）**：

| 字段 | 类型 | 语义 | lint 表现 | MCP 表现 |
|---|---|---|---|---|
| `References` | `[]string` (slug) | 软引用 / see-also | `dangling-reference` (warning) | `kron_impact.references` (反向)、`kron_get.references` |
| `DependsOn` | `[]string` (slug) | 硬依赖 | `dangling-depends-on` (error) + `depends-on-cycle` (error) + `self-reference` (error) | `kron_impact.prerequisites` + `kron_delete.dependents` (soft warn) |

字段细节、示例、校验细节：[intent-structure.md §三 "关系字段"](../abstractDesign/intent-structure.md) + [RFC 2026-10-03-frontmatter-references](../rfc/2026-10-03-frontmatter-references.md)。

---

## 3 `Status` 枚举

```go
type Status string

const (
    StatusDraft      Status = "draft"
    StatusActive     Status = "active"
    StatusSuperseded Status = "superseded"
)
```

状态转换链：

```
draft → active → superseded
```

---

## 4 `Anchor`

```go
// Anchor is a parsed // @kron:intent <slug> annotation.
type Anchor struct {
    Slug       string  // 意图路径，不含 .md
    FilePath   string  // 锚点所在源文件路径
    LineNumber int     // 锚点所在行（1-indexed）
}
```

---

## 5 `Assumption` 与 `AssumptionFile`（B-3, v1.3+）

> 2026-10-08 增补：B-3 拍板（RFC `2026-10-08-assumptions-standalone`）把 assumption 拆为两层——intent frontmatter 里的 `Assumption`（引用 + 覆盖），与 `.kron/assumptions/<id>.md` 里的 `AssumptionFile`（共享 text + 默认 severity 的真理源）。

### 5.1 `Assumption`（写在 intent frontmatter `assumptions[]`）

```go
// Assumption is a verifiable precondition that governs the intent's
// validity. Written in frontmatter's assumptions[] field. B-3: each
// intent carries its own severity + rationale; shared text lives in
// .kron/assumptions/<id>.md (see AssumptionFile).
type Assumption struct {
	ID         string   `yaml:"id"`                       // kebab-case; MUST == registry file name
	Text       string   `yaml:"text,omitempty"`          // 可选；留空从 registry 读
	Severity   Severity `yaml:"severity"`                 // hard | soft；本意图级别，跨意图可不同
	Rationale  string   `yaml:"rationale"`                // 必需：≥ 10 字符，解释本意图为何这 severity
	ExpiresAt  string   `yaml:"expires_at,omitempty"`    // ISO date
	VerifiedAt string   `yaml:"verified_at,omitempty"`
	VerifiedBy string   `yaml:"verified_by,omitempty"`   // @handle
}

type Severity string

const (
	SeverityHard Severity = "hard"
	SeveritySoft Severity = "soft"
)
```

**校验**（`internal/parser/frontmatter.go validateFrontmatter`）：

- `id` 必填 + `parser.ValidateSlug(id)` + 不能含 `/`（assumption 注册表是扁平目录）
- `severity` 必填，且取值 `hard` / `soft`
- `rationale` 必填，≥ 10 字符（v1.3 迁移期 lint Warning，v1.5 硬 Error）
- `text` 可选；留空从 registry 读，留非空 = 仍处 A→B 迁移期（lint 报 `RuleAssumptionMixedForm`）

### 5.2 `AssumptionFile`（`.kron/assumptions/<id>.md`）

```go
// AssumptionFile is one assumption stored as .kron/assumptions/<id>.md.
// The intent's frontmatter only references it via assumption ID; this
// file is the source of truth for shared text + default_severity.
type AssumptionFile struct {
	Slug        string                 // 文件名不含 .md；MUST == Frontmatter.ID
	Frontmatter AssumptionFrontmatter
	Body        string                 // markdown 正文（frontmatter 之后）
	SourcePath  string                 // store 层注入，不参与落盘序列化
}

// AssumptionFrontmatter is the YAML metadata of an assumption file.
// Severity is renamed to DefaultSeverity (B-3): the per-intent
// severity is ground truth; this is only a default that intents
// may override (with rationale explaining why).
type AssumptionFrontmatter struct {
	ID              string   `yaml:"id"`                  // MUST == Slug
	Text            string   `yaml:"text"`                // 跨意图共享的描述
	DefaultSeverity Severity `yaml:"default_severity"`    // 建议默认；意图各自覆盖
	Status          Status   `yaml:"status,omitempty"`    // draft | active | superseded
	CreatedBy       string   `yaml:"created_by"`          // @handle 或 agent:<model>
	UpdatedAt       string   `yaml:"updated_at"`          // ISO 8601
	ExpiresAt       string   `yaml:"expires_at,omitempty"`
	VerifiedAt      string   `yaml:"verified_at,omitempty"`
	VerifiedBy      string   `yaml:"verified_by,omitempty"`
	Reviewers       []string `yaml:"reviewers,omitempty"`
}
```

**`status: superseded` 的 assumption 不参与 lint / MCP `kron_assume_check` / `kron_stale`**——类似 intent 的 `superseded` 语义。

**`DefaultSeverity` vs `Assumption.Severity` 关系**：

- registry 里是**默认**（"该假设通常多严"）
- intent frontmatter 里是**该意图实际**（ground truth）
- 不同意图可不同（`single-region` 在 `auth/jwt.md` 是 `hard`，在 `ui/console.md` 是 `soft`）
- intent 的 `rationale` 解释"为什么本意图这 severity"

读写 API 走 `internal/assumption/` 包（`Reader.List` / `Reader.Get` / `Writer.Create` / `Writer.Update`），不在本文件展开。

### 5.3 lint 规则（B-3 新增 6 条）

| 规则 | 级别 | 说明 |
|---|---|---|
| `assumption-registry-id-mismatch` | Error | intent 引用 id 不在 `.kron/assumptions/` |
| `assumption-orphan` | Warning | registry 有但**无** intent 引用（`superseded` 跳过） |
| `assumption-mixed-form` | Warning | intent assumptions 混 id 引用 + inline `text` / `expires_at`（A→B 迁移期） |
| `assumption-file-id-mismatch` | Error | `.kron/assumptions/<id>.md` 文件名 ≠ frontmatter `id:` |
| `assumption-rationale-required` | Error / Warning | v1.3 迁移期 Warning，v1.5 改 Error |
| `assumption-rationale-stale` | Warning | intent inline `text` 与 registry `text` 分歧 |
| `assumption-severity-mismatch` | Warning | intent severity ≠ registry `default_severity`（override 合法） |

`Severity` 语义见 [intent-structure.md §三](../abstractDesign/intent-structure.md) `assumptions` 字段策略。
`expires_at` / `verified_at` / `verified_by` 不驱动任何自动行为——只供 MCP `kron_stale` 与 LSP hover 提示。

---

## 6 `Config`

```go
type Config struct {
    IntentsDir string `toml:"intents_dir"` // 默认 ".kron/intents"
}
```

配置文件位于 `.kron/config.toml`，**缺失即用默认值**，不存在即不报错。

> **v1 不增加任何配置字段**。`default_reviewer` / `lint_rules` / 其他扩展一律推迟。

---

## 7 与 `intent-structure.md` 的关系

| 字段 | intent-structure.md 定义 | domain-model.md 落地 |
|---|---|---|
| `symbol` | 关联代码符号 | `Frontmatter.Symbol` |
| `created_by` | 作者 | `Frontmatter.CreatedBy` |
| `updated_at` | 更新时间 | `Frontmatter.UpdatedAt` |
| `reviewers` | 评审人 | `Frontmatter.Reviewers`（可选） |
| `status` | 状态 | `Frontmatter.Status` |
| `title` | 标题行 | `Body` 第一行 Markdown 语法 |
| `body` | 正文 | `Body` 剩余部分 |
