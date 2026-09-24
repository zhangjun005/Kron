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
    Symbol    []string `yaml:"symbol,omitempty"`     // 关联代码符号列表
    CreatedBy string   `yaml:"created_by"`          // "@user" 或 "agent:<model>"
    UpdatedAt string   `yaml:"updated_at"`          // ISO 8601
    Reviewers []string `yaml:"reviewers,omitempty"` // 可选；多协作项目使用
    Status    Status   `yaml:"status,omitempty"`    // draft / active / superseded
}
```

**注意**：`Status` 完全可选；不填 = 不参与生命周期管理。

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

## 5 `Config`

```go
type Config struct {
    IntentsDir string `toml:"intents_dir"` // 默认 ".kron/intents"
}
```

配置文件位于 `.kron/config.toml`，**缺失即用默认值**，不存在即不报错。

> **v1 不增加任何配置字段**。`default_reviewer` / `lint_rules` / 其他扩展一律推迟。

---

## 6 与 `intent-structure.md` 的关系

| 字段 | intent-structure.md 定义 | domain-model.md 落地 |
|---|---|---|
| `symbol` | 关联代码符号 | `Frontmatter.Symbol` |
| `created_by` | 作者 | `Frontmatter.CreatedBy` |
| `updated_at` | 更新时间 | `Frontmatter.UpdatedAt` |
| `reviewers` | 评审人 | `Frontmatter.Reviewers`（可选） |
| `status` | 状态 | `Frontmatter.Status` |
| `title` | 标题行 | `Body` 第一行 Markdown 语法 |
| `body` | 正文 | `Body` 剩余部分 |
