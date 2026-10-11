# RFC: Writer README 简写对称化 + 树形创建 API

> 状态：**DRAFT** (2026-10-08)
> 范围：`internal/model` + `internal/store` + `cmd/kron/cli/add.go` + `cmd/kron/serve-mcp/handlers_add.go`
> 触发：发现 `store.Writer` 不识别 README 简写（`store.Reader` 识别）—— 写入 `auth` → `.kron/intents/auth.md`，但读取 `auth` → `.kron/intents/auth/README.md`（不对称）。`kron add` 当前无法在树中创建 node intent（README 形态）。

## 1 背景

`docs/rfc/2026-10-08-intent-tree-api.md` §3.1 拍板 A1：README-as-intent 简写走存储层——`auth/README.md` 的 slug 是 `auth`，`Load("auth")` 通过 `resolveIntentPath` 找不到 `auth.md` 时 fallback 到 `auth/README.md`。

但 **A1 只兑现了 Read 路径**：

- ✅ `Reader.Load` / `Reader.Exists` / `walkIntentSlugs` 全部识别 README 简写
- ❌ `Writer.Write` / `Writer.Exists` / `Writer.MoveToTrash` / `Writer.RestoreFromTrash` 走 `model.IntentPath(slug)` —— 永远写 `<slug>.md`
- ❌ `model.IntentPath` 是纯 string concat，**永远**返回 `.kron/intents/<slug>.md`

副作用：

- `kron add auth` → 写 `auth.md`；但 `kron lint` / anchor 解析 → 找 `auth/README.md`。**store 层语义漂移**。
- 现有 `kron add` 写不出 node intent（README 形态）—— 树形创建是空缺。
- `internal/store/reader_test.go:291` 注释说"resolves it through `model.IntentPath("auth")` = `.kron/intents/auth/README.md`"——**与 `paths.go:34` 实际实现不符**（实际是 `.kron/intents/auth.md`）。注释撒谎；测试仍绿是因为 `resolveIntentPath` 走 fallback 读到了文件。

## 2 目标

1. **Writer 端对称**：让 `Write` / `Exists` / `MoveToTrash` / `RestoreFromTrash` 走与 Reader `resolveIntentPath` **对称**的 `resolveWritePath` 逻辑。
2. **树形创建**：让 `kron add <slug>` 能创建 node intent（README 形态）或 leaf intent（`.md` 形态），且**父 node intent 必须先存在**（无孤儿子树）。
3. **不破契约**：`model.IntentPath` 保持纯 string concat（RFC 2026-10-08 §6 决策）；frontmatter **不加** `parent` 字段（避免 RFC 改动升级）。

## 3 设计拍板

### 3.1 parent 关系：文件系统位置推导（**不加 frontmatter 字段**）

- 意图 `auth/jwt` 的"父" = 它所在目录 `auth/` 下是否存在 `README.md`
- 存在 = 父 slug 是 `auth`（node intent）
- 不存在 = 该意图是**孤儿 leaf**，lint 报 `intent-orphan-under-no-node`（v1.2+）
- frontmatter **不**加 `parent` 字段（保持现有 schema，零迁移成本）

### 3.2 `model.Intent` 加 `Kind` 字段（**frontmatter 不持久化**）

```go
// model.Intent (新增)
type IntentKind string

const (
    IntentKindNode IntentKind = "node" // 物理形态: <slug>/README.md
    IntentKindLeaf IntentKind = "leaf" // 物理形态: <slug>.md（默认值）
)

type Intent struct {
    Slug        string       // 现有
    Kind        IntentKind   // 新增。frontmatter 不写（避免 schema 改动）
    Frontmatter Frontmatter
    Body        string
    SourcePath  string
}
```

- `Kind` 是**写入路由决策**，**不**进 frontmatter（保持 schema 干净）
- `SerializeMarkdown` 跳过 `Kind`（现有行为：未列入 YAML tag 的字段不写出）
- `ParseMarkdown` 不读 `Kind`（从 `model.Intent` 构造时由调用方决定）

### 3.3 `store.Writer` 新增 `resolveWritePath`（与 `Reader.resolveIntentPath` 对称）

```go
// internal/store/paths.go (新文件, 收纳 resolve 逻辑)
//
// resolveWritePath 决策表:
//   Kind=Leaf (default): <root>/.kron/intents/<slug>.md
//   Kind=Node:           <root>/.kron/intents/<slug>/README.md
//   slug 含 "/" + Kind=Node: <root>/.kron/intents/<dir-segments>/<last-seg>/README.md
```

- 不动 `model.IntentPath`（RFC 2026-10-08 §6 红线）
- `Writer.Write` / `Writer.Exists` / `Writer.MoveToTrash` / `Writer.RestoreFromTrash` 全切到 `resolveWritePath`
- **若** `Kind=Leaf` 且目标 `<slug>.md` 不存在，但 `<slug>/README.md` 存在 → 报 `ErrSlugCollision`（node intent 已占位）
- **若** `Kind=Node` 且目标 `<slug>/README.md` 不存在，但 `<slug>.md` 存在 → 报 `ErrSlugCollision`（leaf 已占位）
- **顶层 `slug="README"` 特殊**：始终写 `.kron/intents/README.md`（不创 `README/README.md` 死循环）

### 3.4 `kron add --kind` + `--parent`（**不破 CLI 最小子集**）

`kron add` 加两个标志：

- `--kind leaf|node`（**默认 leaf**）—— 选择物理形态
- `--parent <slug>`（**可选**）—— 显式指定父 slug（lint 检查：父必须是 node intent）

CLI 校验顺序：

1. `parser.ValidateSlug(slug)`
2. 若 `--parent` 提供：检查父 intent `Exists`（= node intent）—— 不存在报 `kron add: parent <slug> is not a node intent`
3. 检查 `w.Exists(slug)` 目标路径未占位
4. 调 `w.Write` 写文件

### 3.5 MCP `kron_add` 加 `kind` 字段

`kron_add` input schema 加 `kind?: "node" | "leaf"`（默认 `leaf`），**不**加 `parent`（parent 由 slug 字符串里的 `/` 隐式决定：`auth/jwt` → 父 = `auth`，调用方传 `auth/jwt` 时 store 端校验 `auth/README.md` 存在）。

## 4 实施拆解

| PR | 内容 | 估时 | 风险 |
|---|---|---|---|
| **PR-A** | `model.IntentKind` 类型 + `model.Intent.Kind` 字段 + unit test | 30min | 低（新增字段，默认值兼容） |
| **PR-B** | `internal/store/paths.go` 抽 `resolveWritePath` + `Writer.Write/Exists/MoveToTrash/RestoreFromTrash` 全切过去 + 修 `reader_test.go:291` 注释 | 2h | 中（动 store 4 个方法 + 注释） |
| **PR-C** | `kron add --kind` + `--parent` 标志 + `parser.ValidateKind` 校验 | 1h | 低（CLI 扩展） |
| **PR-D** | MCP `kron_add` input schema 加 `kind` 字段 + handler 透传 | 1h | 低（access layer 增量） |

## 5 不在本 RFC 范围

- ❌ frontmatter `parent` 字段（拍板**不**加）
- ❌ `intent-slug-collision` lint 规则（v1.2+ 独立 PR；PR-B 报的 `ErrSlugCollision` 是 store 硬错）
- ❌ `intent-orphan-under-no-node` lint 规则（v1.2+ 独立 PR）
- ❌ view 包改动（`BuildIntentTree` 已处理 README-as-dir 提升，PR-B 完成）
- ❌ anchor 工具的 README 来源（`md-anchors.md` 独立 RFC）

## 6 决策

| 决策 | 拍板 |
|---|---|
| `model.IntentPath` 改不改 | **不改**（RFC 2026-10-08 §6 红线） |
| frontmatter 加不加 `parent` 字段 | **不加**（文件系统位置推导） |
| `model.Intent` 加不加 `Kind` 字段 | **加**（frontmatter 不持久化） |
| `Kind` 默认值 | **leaf**（向后兼容现有 `kron add`） |
| Writer 端 README 简写落地位置 | **store 包新 `paths.go` 抽 `resolveWritePath`** |
| `kron add` CLI 怎么扩 | **加 `--kind` + `--parent` 两个 flag** |
| MCP `kron_add` 怎么扩 | **加 `kind` 字段；不加 `parent`（slug `/` 隐式）** |
| 顶层 `README.md` 怎么写 | **始终写 `.kron/intents/README.md`**（不创子目录） |

## 7 相关 RFC

- [`2026-10-08-intent-tree-api.md`](2026-10-08-intent-tree-api.md) §3.1 A1 拍板（Read 侧已兑现；本 RFC 兑现 Write 侧）
- [`2026-10-08-path-conventions.md`](2026-10-08-path-conventions.md) §2.1 锁定 `model.IntentPath` 是纯 string concat
