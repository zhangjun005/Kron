# API 表面与函数签名参考

> 来源：`docs/abstractDesign/architecture.md` §3、§4  
> 本文件是**实施建议**，不是架构真理。函数签名如有变更，请同步修改此处。

---

## 1 核心函数签名（`internal/` 包）

### 1.1 `internal/model/`（对象层）

```go
// Intent is one design intent, persisted as .kron/intents/<slug>.md.
type Intent struct {
    Slug        string
    Frontmatter Frontmatter
    Body        string
    SourcePath  string
}

type Frontmatter struct {
    Symbol    []string
    CreatedBy string
    UpdatedAt string
    Reviewers []string
    Status    Status
}

type Status string

const (
    StatusDraft      Status = "draft"
    StatusActive     Status = "active"
    StatusSuperseded Status = "superseded"
)

type Anchor struct {
    Slug       string
    FilePath   string
    LineNumber int
}

type Config struct {
    IntentsDir string
}
```

### 1.2 `internal/store/`（业务层）

```go
// EnsureKronDir creates .kron/intents/ and writes .kron/config.toml if absent.
func EnsureKronDir(ctx context.Context, cfg *model.Config) error

// WriteIntent persists a new intent file at .kron/intents/<slug>.md.
func WriteIntent(ctx context.Context, slug string, intent *model.Intent) error

// LoadIntent reads a single intent file.
func LoadIntent(ctx context.Context, slug string) (*model.Intent, error)

// LoadAll reads all intent files under cfg.IntentsDir.
func LoadAll(ctx context.Context, cfg *model.Config) ([]*model.Intent, error)

// ResolveIntent checks if .kron/intents/<slug>.md exists.
func ResolveIntent(ctx context.Context, slug string) error

// MoveToTrash soft-deletes an intent: intents/ → .trash/.
func MoveToTrash(ctx context.Context, slug string) (string, error)

// RestoreFromTrash restores an intent: .trash/ → intents/.
func RestoreFromTrash(ctx context.Context, slug string) (string, error)

// ValidateFrontmatter checks required fields and status enum.
func ValidateFrontmatter(fm *model.Frontmatter) error
```

> 所有 store 函数第一个参数 `ctx context.Context`。caller 由访问层注入（`"cli"` / `"mcp"` / ...）。

### 1.3 `internal/parser/`（业务层）

```go
// ParseSlug validates and normalizes a slug string.
func ParseSlug(raw string) (string, error)

// ScanAnchors scans a source file for // @kron:intent <slug> lines.
func ScanAnchors(filePath string, src io.Reader) ([]model.Anchor, error)

// ParseFrontmatter extracts YAML frontmatter from markdown content.
func ParseFrontmatter(content string) (*model.Frontmatter, string, error)

// RenderIntentTemplate returns the default .md skeleton for a new intent.
func RenderIntentTemplate(slug, createdBy string) string
```

### 1.4 `internal/lint/`（业务层，按需）

```go
// LintResult holds the outcome of a full lint run.
type LintResult struct {
    Passed bool
    Errors []LintError
}

// LintError describes one lint violation with location context.
type LintError struct {
    Kind    string // "anchor-dangling" | "frontmatter-required" | ...
    Message string
    File    string
    Line    int
}

// Run executes all lint checks over the given config and source glob.
func Run(ctx context.Context, cfg *model.Config, sourceRoots []string) (*LintResult, error)
```

---

## 2 访问层函数签名（`cmd/kron/`）

### 2.1 CLI 入口

```go
// cmd/kron/main.go
func Execute() error

// cmd/kron/cli/init.go
var cmdInit = &cobra.Command{
    Use:   "init",
    Short: "Initialize .kron/intents/ and config.toml",
    RunE:  runInit,
}

// cmd/kron/cli/add.go
var cmdAdd = &cobra.Command{
    Use:   "add [slug]",
    Args:  cobra.ExactArgs(1),
    Short: "Scaffold a new intent file",
    RunE:  runAdd,
}

// cmd/kron/cli/lint.go
var cmdLint = &cobra.Command{
    Use:   "lint",
    Short: "Scan anchors and validate frontmatter (CI gate)",
    RunE:  runLint,
}
```

### 2.2 MCP Server 入口

```go
// cmd/kron/serve-mcp/main.go
func main() {
    srv := mcpserver.New()
    srv.RegisterTool("kron_init",    handleInit)
    srv.RegisterTool("kron_add",     handleAdd)
    srv.RegisterTool("kron_list",    handleList)
    srv.RegisterTool("kron_get",     handleGet)
    srv.RegisterTool("kron_update",  handleUpdate)
    srv.RegisterTool("kron_delete",  handleDelete)
    srv.RegisterTool("kron_restore", handleRestore)
    srv.RegisterTool("kron_lint",    handleLint)
    srv.Serve() // stdio JSON-RPC loop
}

// ctx always passed; caller = "mcp:<agent-id>" injected by server
```

---

## 3 错误 sentinel 清单

```go
var (
    ErrIntentNotFound     = errors.New("intent not found")
    ErrIntentExists       = errors.New("intent already exists")
    ErrAnchorDangling     = errors.New("anchor points to non-existent intent")
    ErrFrontmatterInvalid = errors.New("frontmatter is invalid")
    ErrSlugInvalid        = errors.New("intent slug is invalid")
    ErrConfigInvalid      = errors.New("config file is invalid")
)
```

所有错误用 `fmt.Errorf("...: %w", err)` 包装，调用方用 `errors.Is` / `errors.As` 判别。

---

## 4 Caller 身份约定

| 访问层 | caller 值 | 示例 |
|---|---|---|
| CLI | `"cli"` | — |
| MCP | `"mcp:<agent>"` | `"mcp:claude-3.7"` |
| LSP | `"lsp"` | — |
| IDE 插件 | `"ide"` | — |
| GUI | `"gui"` | — |

在 `context.Context` 中以 `context.WithValue(ctx, callerKey, "cli")` 注入，store/parser/lint 函数通过 `ctx.Value(callerKey)` 读取。
