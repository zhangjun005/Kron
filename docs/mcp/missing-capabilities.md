# MCP 缺失能力（2026-10-13）

> 按 MCP 协议标准（[specification](https://modelcontextprotocol.io/specification)）对照 Kron `serve-mcp` 现状，
> 记录缺失的 capability 和实现建议。
> 对应 backlog 条目见各节。

---

## 能力对照总表

| MCP Capability | Kron 状态 | 优先级 | 关联 backlog |
|---|---|---|---|
| `tools/*` | ✅ 17 个工具 | — | — |
| `tools/list` | ✅ | — | — |
| `tools/call` | ✅ | — | — |
| `resources/*` | ❌ 未声明 | 中 | C-1 |
| `prompts/*` | ❌ 未声明 | 低 | C-2 |
| `logging/*` | ❌ 未声明 | 中 | C-3 |
| `roots/*` | ❌ 未声明 | 低 | 新发现 |
| `server/info` | ✅ | — | — |
| `server/ping` | ✅ | — | — |

---

## G-4 — Capabilities 显式声明（高优先级）

**问题**：`cmd/kron/serve-mcp/serve_mcp.go` 里 `NewServer(impl, nil)` 没有显式传 `ServerCapabilities`。
G-4 说得很清楚："Capabilities 缺省 = 全部默认 true"，导致 client 可能误以为有 Resources/Prompts/Logging。

**修法**：把 `nil` 替换成显式 `ServerCapabilities`，只声明 Tools，其他显式为 `nil`。

```go
// serve_mcp.go
server := mcp.NewServer(impl, &mcp.ServerOptions{
    Capabilities: &mcp.ServerCapabilities{
        Tools: &mcp.ToolCapabilities{
            ListChanged: true, // 支持 tools/list 变更通知
        },
        // 其余全部 nil = 明确"不支持"
        Resources:        nil,
        Prompts:         nil,
        Logging:         nil,
    },
})
```

**风险**：低。显式声明比隐式更安全。

---

## C-1 — Resources（kron://intents/\<slug\> URI）（中优先级）

**状态** — ✅ **已实装 (2026-10-13)**

实现位置:`cmd/kron/serve-mcp/resources.go`,`testkit.go` + `serve_mcp.go`
中 `registerResources(server)` 调用链。Wire-level 测试
(`resources_lint_suggestion_test.go`) 覆盖 `resources/list`、
`resources/read` 正常路径、not-found、malformed URI 四种 case。

### 背景

MCP Resources 是 server → client 的只读数据暴露。跟 Tools 的区别：

- **Tools**：client 调 server 执行动作（mutating）
- **Resources**：server 暴露 URI，client 主动拉取（read-only）

典型交互：

```
client                              server
  │                                    │
  │  ← resources/list                  │  ← 注册 kron://intents/*
  │  → { resources: [                 │
  │       { uri: "kron://intents/ux", │
  │         name: "UX intent",        │
  │         mimeType: "text/markdown"  │
  │       }] }                        │
  │                                    │
  │  ← resources/read { uri: "..." }   │  ← AI 工具主动读
  │  → { contents: [{...}] }          │
```

### 价值

AI 对话里直接引用 intent 内容更自然：

```
# 不需要 Resources
调用 kron_get(slug="ux-design")

# 有 Resources 后
直接读 kron://intents/ux-design
```

### 实现方案

**方案 A（简单）：Intent URI**

```
kron://intents/<slug>
```

- URI template: `kron://intents/{slug}`
- 每个 intent 是一个 resource
- `resources/list` 动态枚举（走 `LoadAllCached`）

**方案 B（丰富）：分层 URI**

```
kron://intents/              → 目录索引（list intent slugs）
kron://intents/<slug>        → 单个 intent
kron://assumptions/          → assumption registry 索引
kron://assumptions/<id>      → 单个 assumption
```

**推荐方案 A**，理由：

1. 单 intent URI 最直接，跟 `kron_get` 功能不重复
2. `resources/list` 可以加 `PaginatedList` 支持大量 intents
3. 方案 B 增加复杂度，assumptions URI 用途不明确

### 修法（方案 A）

```go
// serve_mcp.go: 注册 Resources capability
Resources: &mcp.ResourceCapabilities{
    Subscribe:   boolPtr(false), // 不需要订阅（intents 变化不推）
    ListChanged: boolPtr(true),  // 支持 resources/list 变更通知
},

// handlers_resources.go（新建）
func HandleResourcesList(ctx context.Context, root string) ([]mcp.Resource, error) {
    all, err := LoadAllCached(ctx, root)
    if err != nil {
        return nil, err
    }
    resources := make([]mcp.Resource, 0, len(all))
    for _, intent := range all {
        resources = append(resources, mcp.Resource{
            URI:         "kron://intents/" + intent.Slug,
            Name:        intent.Slug,
            MimeType:    "text/markdown",
            Description: intent.Frontmatter.Symbol[0], // 第一个 symbol 作为描述
        })
    }
    return resources, nil
}

func HandleResourcesRead(ctx context.Context, root, uri string) (*mcp.ReadResourceResult, error) {
    // 解析 kron://intents/<slug>
    slug := strings.TrimPrefix(uri, "kron://intents/")
    r, _ := store.NewReader(root)
    intent, err := r.Load(ctx, slug)
    if err != nil {
        return nil, err
    }
    return &mcp.ReadResourceResult{
        Contents: []mcp.ResourceContents{{
            URI:       uri,
            MimeType:  "text/markdown",
            Text:      string(parser.MustMarshal(intent)), // 重组 frontmatter + body
        }},
    }, nil
}
```

**风险**：中等。

- 需要改 `NewServer` 握手阶段
- `resources/list` 在 500 intents 时返回 500 个 resource，可能触发 client 截断
- AI 工具（Claude Code / Cursor MCP runtime）不一定主动调用 `resources/list` — 目前没有 client 主动请求 Resources 的已知场景

**待定**：需要先验证 target MCP client（Claude Code / Cursor）是否支持 Resources。

---

### 实装记录 (2026-10-13)

实现位置：`cmd/kron/serve-mcp/resources.go`，在 `serve_mcp.go`
的 `registerTools(server)` 之后调 `registerResources(server)`。
testkit 同步注册了 resources，所以 wire-level 测试可以走 in-memory
client。

URI 模板改成 `kron://intents/{+slug}`（**不是** `{slug}`）：
最初按 RFC 6570 普通 expansion 写 `{slug}`，但**不**工作。
go-sdk v1.1.0 的 `lookupResourceHandler` 把模板编译成 regex
（`tmpl.Regexp().MatchString`），普通 `{slug}` 不跨 `/`，所以
`kron://intents/auth/jwt` 永远匹配不到。SDK 自己的
`TestTemplateMatch` 表明 `{+path}`（reserved expansion）才是允许
保留 `/` 的形式。Slug 自身的语法（`parser.ValidateSlug`）**不变**；
这纯粹是 wire 模板的语法选择。

最终 handler 形态（resources.go）：

```go
const kronIntentsResourceTemplate = "kron://intents/{+slug}"

func registerResources(server *mcp.Server) {
    template := &mcp.ResourceTemplate{
        Name:        "kron-intent",
        Title:       "Kron intent",
        Description: "...",
        MIMEType:    "text/markdown",
        URITemplate: kronIntentsResourceTemplate,
    }
    server.AddResourceTemplate(template, handleIntentResourceRead)
}
```

handler 行为：

- 正常路径：`store.NewReader(root).Load(slug)` + `parser.SerializeMarkdown(intent)` → markdown 文本返回
- Not-found：`model.ErrIntentNotFound` → `mcp.ResourceNotFoundError(uri)`（SDK 推荐做法，避免泄露 server 内部）
- malformed URI（非 `kron://intents/` 前缀）：→ not-found
- 解析错（frontmatter invalid 等）：→ wrapped error，`isError` 设为 true

**风险评估（实装后）**：

- ✅ go-sdk v1.1.0 已支持
- ⚠️ Client 端是否主动用 Resources 仍**待验证**。Claude Desktop
  / Cursor MCP runtime 默认**不**主动调 `resources/list`；AI 工具
  通常直接调 `kron_get` tool。如果 client 不主动用，Resources 是
  "协议层齐了但消费侧空闲"的状态
- ✅ Resources 不与 Tools 互斥：client 可以**同时**用
  `kron_get`（结构化 JSON）和 `resources/read`（原始 markdown）

---

## C-2 — Prompts（kron-review / kron-audit 模板）（低优先级）

### 背景

MCP Prompts 是预定义好的 LLM 调用模板，client 可以通过 `prompts/list` 获取，`prompts/get` 获取具体渲染后的 prompt。

**典型用途**：

```
# client 侧
> /prompt kron-review   ← 从 MCP server 获取预置的 review prompt

# server 侧实现
prompts/list → [
  { name: "kron-review", description: "Review all active intents", arguments: [...] },
  { name: "kron-audit", description: "Audit assumption freshness", arguments: [...] },
]
prompts/get(name="kron-review", args) → { messages: [...] }
```

### 候选 Prompt 模板

| 名称 | 用途 | arguments |
|---|---|---|
| `kron-review` | 评审所有 active intents | `status?: draft/active` |
| `kron-assumption-audit` | 检查 expired/unverified hard assumptions | — |
| `kron-stale-report` | 生成 stale intents 报告 | `days_threshold?: int` |

### 实现方案

```go
// handlers_prompts.go（新建）
func HandlePromptsList(ctx context.Context) []mcp.Prompt {
    return []mcp.Prompt{
        {
            Name:        "kron-review",
            Description: "Review all intents with a given status",
            Arguments: []mcp.PromptArgument{{
                Name:        "status",
                Description: "filter by status: draft or active",
                Required:    false,
            }},
        },
    }
}

func HandlePromptsGet(ctx context.Context, name string, args map[string]string) (*mcp.GetPromptResult, error) {
    switch name {
    case "kron-review":
        status := args["status"]
        return renderKronReviewPrompt(ctx, status)
    }
    return nil, fmt.Errorf("unknown prompt: %s", name)
}
```

**风险**：低（Prompts 不影响工具调用），但价值也低（AI 工具一般直接调 Tools）。

---

## C-3 — Logging Capability（替代 stderr hack）（中优先级）

### 现状

`serve-mcp` 当前用 `fmt.Fprintf(os.Stderr, ...)` 打印诊断信息。这不是 MCP 协议里的 logging。

### 目标

用 MCP 的 `logging/message` notification 替代 stderr hack：

```
← logging/setLevel { level: "info" }    ← client 配置 log 级别
← logging/message { level: "warn", logger: "kron", data: "..." } ← server 发日志
```

### 实现方案

```go
// serve_mcp.go
Logging: &mcp.LoggingCapabilities{
    Level: []string{"debug", "info", "warn", "error"},
},

// handlers_logging.go（新建）
func HandleLoggingSetLevel(ctx context.Context, level string) error {
    // 存储当前 level，后续 handler 用
    setLogLevel(level)
    return nil
}

// server.go 里加一个 logger 适配
type kronLogger struct{ server *mcp.Server }
func (l *kronLogger) Info(msg string) {
    l.server.NotifyLoggingMessage(ctx, mcp.LoggingMessageParams{
        Level:  "info",
        Logger: "kron",
        Data:   msg,
    })
}
```

**风险**：低。但需要考虑：
- 如果 client 没有配置 `logging/setLevel`，所有日志都被丢弃（MCP spec 要求 client 主动请求）
- `kron_init` / `kron_add` 这种写操作的成功日志可能过度

---

## C-5 — kron_lint 加 fix suggestion hint（DX 改善）

**状态** — ✅ **已实装 (2026-10-13)**

见 `docs/mcp/backlog.md` C-5。

**修法**：在 `LintDiag` 输出里加 `suggestion?: string` 字段，对应 rule 的自动修复建议（如果有）。

示例：

```json
{
  "rule": "anchor-dangling",
  "where": "main.go:1",
  "severity": "error",
  "detail": "anchor points to non-existent intent \"auth/never-existed\"",
  "suggestion": "Create .kron/intents/<slug>.md or fix the slug in the anchor"
}
```

### 实装记录 (2026-10-13)

实现位置：

- `internal/lint/diag.go` — `Diag` 加 `Suggestion string` 字段
- `internal/lint/rules.go` — 新增 `SuggestionFor(Rule) string` 目录，覆盖全部 19 条 Rule
- `internal/lint/lint.go` — `RunWith` 入口用 `defer` 回填
  `Suggestion`（defer 的关键作用：覆盖 `LoadAll` 失败的早返路径，
  普通的"末尾回填循环"会被那条 `return` 绕过）
- `cmd/kron/serve-mcp/tools_schema.go` — `LintDiag` 加
  `Suggestion string`（`json:"suggestion,omitempty"`）
- `cmd/kron/serve-mcp/tools_lint.go` — 透传 `d.Suggestion`
- `cmd/kron/cli/lint.go` — `kron lint` text 输出额外印一行
  `       hint: <suggestion>`；JSON 输出加 `"suggestion"` 字段

测试：

- `TestSuggestionForCatalogExhaustiveness`：每条 Rule 常量都
  必须有非空 SuggestionFor 条目（防止"加新 Rule 忘记填"）
- `TestRun_BackfillsSuggestion`：三个 sub-path（A-class / T-class /
  C-class）触发不同 rule 的 diag 都被回填
- `TestLint_SuggestionPropagates`：wire-level 通过 `kron_lint` tool
  拿到 `LintOutput.Errors[].Suggestion` 非空

注意：当前 `Suggestion` 是**静态目录**，**不**做模板插值
（不把 `<slug>` 替换成实际 slug）。模板插值需要"在 Diag 上
记录 slug / 路径再渲染"两步，v1 暂不做；提了具体 value 时
AI client 也能从 `Where` 字段自己拿到。

---

## C-6 — kron_impact incoming_anchors 拆分 code/markdown（DX 改善）

见 `docs/mcp/backlog.md` C-6。

**现状**：`incoming_anchors` 是一个数组，`kind` 字段标注 code/markdown，但 GUI/IDE 侧消费时要自己过滤。

**修法**：拆成两个字段：

```json
{
  "incoming_anchors_code":      [...],
  "incoming_anchors_markdown":  [...],
}
```

这样 GUI/IDE 可以直接渲染两个 tab，不需要前端再过滤。

---

## C-7 — kron_get relations 加 dependents 反向（一致性）

见 `docs/mcp/backlog.md` C-7。

**现状**：`kron_get(include_relations=true)` 返回的 `relations` 里没有 `dependents`（哪些 intent 依赖这个）。`kron_delete` 里已经有 `dependents` 字段，逻辑对称。

**修法**：在 `GetRelations` 里加 `Dependents []string`。

---

## Roots Capability（新发现，低优先级 — 方向在 RFC 拍板前**不**动手）

### MCP spec 的方向

MCP Roots 的方向是反的（[spec](https://modelcontextprotocol.io/specification/2025-06-18/client/roots)）：

- **Client 暴露 Roots**（在 `ClientCapabilities.Roots` 声明 + 调 `AddRoots` 注册）
- **Server 拉取 Roots**（调 `roots/list`，或响应 `notifications/roots/list_changed`）
- 我们的 missing-capabilities.md 旧版"在 server 端 `ServerCapabilities.Roots` 声明"是**错的** —— Roots 字段属于 `ClientCapabilities`，不在 server 侧声明。

### Kron 当前的 root 来源

Kron 是单 repo 工具。`resolveMCPRoot` 按以下优先级决定 root：

1. `-CWD` flag（CLI 参数）
2. process 的 `os.Getwd()`（stdio 模式下默认）
3. `config.toml` 的 `intents_dir`（**未实现**，架构 §3.5 零配置原则）

Roots 在这里**不参与** root 解析 —— client 声明的 `roots` 是 client 的工作目录，不一定是 `.kron/intents/` 所在。两者**解耦**。

### 单 repo 场景下的判断

Kron 当前的 stdio 进程 = 一个 session = 一个 root。`ss.ListRoots` 在这里**没价值**：

- client（Claude Desktop / Cursor）声明的 root 可能是 `~/my-app`，但 Kron 需要的是 `<repo>/.kron/intents/`
- 强行信任 `roots/list` 会让 Kron 跑错目录

**结论**：Kron 作为单 repo 工具**不**需要 Roots 集成。Wails GUI 多项目是另一个层级的问题（架构 §〇 铁律 #9：GUI 是 client 层，不在 Kron 主仓），由 GUI 端用多 session 调多个 `kron serve-mcp` 子进程解决，每个子进程自己 `-CWD` 到对应 project。

### 未来如果要做（仅作记录）

假设未来 Wails GUI / IDE 真的需要 server 侧感知 client 的 project 根，做法是：

```go
// serve_mcp.go
server := mcp.NewServer(impl, &mcp.ServerOptions{
    RootsListChangedHandler: func(ctx context.Context, req *mcp.RootsListChangedRequest) {
        // 收到 client 通知：roots 变了，可以重新读
        ss := req.Session
        roots, err := ss.ListRoots(ctx, nil)
        if err != nil { return }
        // ... 用 roots 做点什么
    },
})
```

注意：

- **不能**用 roots 覆盖 `-CWD` 行为（必须保持显式优先）
- LSP 层**不**改 —— Roots 是 MCP 概念；LSP 用 `workspaceFolders` 是另一套
- 改动只发生在 `cmd/kron/serve-mcp/` 内，不进 `internal/`

### 触发条件

第一次出现"Wails GUI / IDE 同时管理多项目且需要 server 感知 project 根"的实际需求。届时先开 RFC 拍板范围（是否多 root、root 来源优先级、缓存策略）。

---

## 实现优先级总结

| 优先级 | 条目 | 预估工时 | 理由 |
|---|---|---|---|
| **高** | G-4 Capabilities 显式声明 | 1h | 防御性，修复协议契约 |
| **高** | C-7 kron_get 加 dependents | 0.5h | 与 kron_delete 对称，无争议 |
| **中** | C-3 Logging capability | 1h | 替代 stderr hack |
| **中** | C-1 Resources | 2h | 价值待验证（client 不一定用） |
| **中** | C-6 incoming_anchors 拆分 | 0.5h | DX 改善 |
| **低** | C-5 lint fix suggestion | 1h | DX 改善 |
| **低** | C-2 Prompts | 2h | MCP client 不一定用 |
| **低** | Roots capability | 1h | 未来多项目场景 |
