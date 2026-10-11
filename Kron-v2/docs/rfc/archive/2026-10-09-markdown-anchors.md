# RFC: 2026-10-09-markdown-anchors

## 状态
- **作者**: @zhangjun005
- **创建**: 2026-10-09
- **阶段**: 草案 → 实施路径
- **替换**: 无

## 动机

`kron_impact` 返回的 `incoming_anchors` 目前用字符串 `"anchor"` 或空字符串区分来源：

```go
// 当前 wire 形状（tools_schema.go）
type AnchorRef struct {
    FilePath string `json:"file_path"`
    Line     int    `json:"line"`
    Kind     string `json:"kind"` // "" = 代码锚点
}
```

问题是：

1. **语义不透明**：`""` 必须查文档才知道是"代码锚点"，没有类型安全。
2. **lint 不感知**：扫描 `.md` 文件的 `parser.ScanMarkdownAnchors` 与扫描代码的 `parser.ScanAnchors` 共用同一 `model.Anchor` 结构，lint 层无法区分两者走不同规则。
3. **`kron_assume_check` 无法按来源过滤**：AI Agent 问"这个 `.md` 文档依赖哪些 intent"，目前只能返回全部锚点。
4. **未来扩展方向丢失**：如果有 `// ## @kron:intent section-name`（章节级锚点）等新形态，当前结构没有扩展位。

## 方案

### 1. `model.Anchor` 加 `Kind` 字段（**已规划**，见 architecture.md §〇·五·5 T7 backlog）

```go
// 在 internal/model/intent.go 的 Anchor 结构体中加：

// Kind classifies the physical origin of the anchor.
// Values:
//   - AnchorKindCode: a "// @kron:intent <slug>" comment in a source file
//   - AnchorKindMarkdown: a "@kron:intent <slug>" line in a .md file
//                          (outside .kron/intents/ and .trash/)
type AnchorKind string

const (
    AnchorKindCode      AnchorKind = "code"
    AnchorKindMarkdown  AnchorKind = "markdown"
)

// Anchor is ...
type Anchor struct {
    Slug      string
    FilePath  string
    LineNumber int
    Kind      AnchorKind  // ← 新增，零值为 AnchorKindCode（向后兼容）
}
```

### 2. `parser.ScanMarkdownAnchors` 回填 `Kind`

`ScanMarkdownAnchors` 扫描时显式设 `Kind: model.AnchorKindMarkdown`；`ScanAnchors` 保持零值（`AnchorKindCode`）。

向后兼容：旧数据或未迁移的 anchor 对象 `Kind == ""` 等于 `"code"`，lint / `kron_impact` 读到 `""` 时 fallback 到 `AnchorKindCode`。

### 3. `internal/lint` 新增一条 T-class 规则（可选，v1.4+）

```
intent-markdown-anchor-under-intent-dir: Warning
  → "@kron:intent" 出现在 .kron/intents/*.md 内（lint walk 默认已跳过，这里加一条明确的警告）
```

### 4. `tools_schema.go` AnchorRef.Kind 类型化

```go
type AnchorRef struct {
    FilePath string `json:"file_path"`
    Line     int    `json:"line"`
    Kind     string `json:"kind"` // "code" | "markdown"; empty = "code" (backward compat)
}
```

Wire 格式不变（仍是 string），只是值集合从 `{ "", "code" }` 扩展到 `{ "code", "markdown" }`。

## 实施路径

| 步骤 | 文件 | 改动 | PR |
|---|---|---|---|
| T1 | `internal/model/intent.go` | 加 `AnchorKind` 类型 + 常量 + `Anchor.Kind` 字段 + `Valid()` | PR-A |
| T2 | `internal/parser/anchors.go` | `ScanAnchors` 回填 `Kind: ""`（零值，向后兼容） | PR-A |
| T3 | `internal/parser/markdown_anchors.go` | `ScanMarkdownAnchors` 设 `Kind: AnchorKindMarkdown` | PR-A |
| T4 | `internal/lint/lint.go` | A-class 扫描结果中 Kind 信息透传到 Diag（future use）；当前 Run 返回的 Diag 仍不感知 Kind——TBD 是否需要 | PR-B |
| T5 | `cmd/kron/serve-mcp/tools_schema.go` + `tools_lint.go` | `AnchorRef.Kind` 文档注明 `"code"` / `"markdown"` / `""` | PR-B |
| T6 | `docs/rfc/2026-10-08-md-anchors.md` | 正式归档本文档，替代草案 | PR-B |

## 不做的事

- ❌ `Anchor.LineOffset`（LSP 需要的列偏移）：留给 LSP 接入时再议，Anchor 是行级足够
- ❌ `Anchor.SectionName`（章节级锚点）：未来 RFC
- ❌ lint 跨 Kind 走不同规则（v1 scope 外）
- ❌ `model.Anchor` 持久化（Anchor 是扫描产物，从不落盘）

## 向后兼容

- 零值 `AnchorKind{}` 向后等于 `AnchorKindCode`
- `store.Writer` / `store.Reader` 不序列化 `Anchor`（Anchor 不落盘），无 frontmatter migration
- `AnchorRef.Kind` 空字符串向后等于 `"code"`

## 依赖

- 无新依赖
- 无 schema 变更（Anchor 不落盘）

## RFC 草案：README-as-intent 语义映射（独立 issue，另开 RFC）

当前 `auth/README.md` slugify 成 `auth/README`，与 `// @kron:intent auth` 的语义期望不匹配：

```
// @kron:intent auth     ← 意图是"auth 模块的设计意图"
// auth/README.md        ← 物理上是 "auth/README" slug

// 当前行为：
//    anchor "auth" → Get("auth") → 404

// 期望行为（RFC 待定）：
//    anchor "auth" → Get("auth") 或 Get("auth/README") 之一存在则命中
```

此 RFC 草案已记录于 `docs/rfc/2026-10-08-intent-tree-api.md`，**不**在本文 scope 内。

## 决策点（待你拍板）

1. **Kind 字段加在哪里**：`Anchor` 结构体上（当前方案）还是包装成 `AnchorRef`（MCP wire 类型，model 不感知）？
2. **lint 是否需要感知 Kind**：T4 是先做还是先跳过（等 LSP 需要时再加）？
3. **README-as-intent 语义映射**：是同 PR 解决还是另开 RFC？
