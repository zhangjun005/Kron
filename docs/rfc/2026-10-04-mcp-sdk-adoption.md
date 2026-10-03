# MCP 官方 SDK 接入实施计划 (v1.2)

> 状态:待审阅 + 待批准 dep 例外
> 关联:`docs/rfc/2026-10-04-mcp-sdk-selection.md`(决策理由)、
>       `docs/rfc/2026-10-04-mcp-protocol-redesign.md`(被推翻的手写方案)、`docs/process/mcp-protocol.md`(差距清单)
> 实施日期:2026-10-04 起

---

## 0 决策摘要

- ✅ **采用官方 SDK**:`github.com/modelcontextprotocol/go-sdk/mcp` v1.0.x(锁 spec 2025-06-18)
- ⚠ **破例加 top-level dep**:需在 `AGENTS.md` §〇 加一个 "approved deps exceptions" 条目
- 🏗 **架构定位**:`mcp` import **只**出现在 `cmd/kron/serve-mcp/*.go`,`internal/**` 永不 import

---

## 1 架构合规性矩阵

| architecture.md §〇 铁律 | 当前(未改) | 改后 | 备注 |
|---|---|---|---|
| #1 业务逻辑只在 `internal/` | ✅ 业务代码零改动 | ✅ | 12 handler 业务逻辑不动 |
| #2 Core 与 access 解耦 | ✅ | ✅ | `internal/store/parser/model` 仍不知 MCP |
| #3 访问层不互相 import | ✅ | ✅ | mcp 包只在 serve-mcp 用 |
| #4 caller 走 ctx | ✅(本 commit C 已固化为"v1.2+ scaffolding") | ✅ | SDK 自带 ctx;与 `model.WithCaller` 不冲突 |
| #5 零新 dep | ✅ | ⚠ **破例** | 须在 AGENTS.md §〇 加 exception |
| #6 Markdown + YAML 是真理源 | ✅ | ✅ | 不变 |
| #7 CI lint 是唯一 gate | ✅ | ✅ | 不变 |
| #8 One-way deps | ✅ | ✅ | `mcp` 是 access-layer-only |

**§2.2 import 边界检查**(mechanical):
```
cmd/kron/serve-mcp/**.go → import "github.com/modelcontextprotocol/go-sdk/mcp" ✅ (本包 = access layer)
internal/**.go          → 不 import 该包 ✅ (compile-time 强制)
cmd/kron/cli/**.go      → 不 import 该包 ✅ (cli 不是 MCP 访问层)
```

---

## 2 选定的 SDK 版本 + 锁定策略

| 字段 | 值 |
|---|---|
| 模块 | `github.com/modelcontextprotocol/go-sdk` |
| 版本 | **`v1.0.0`**(GA,锁定 MCP spec **2025-06-18**) |
| Go 要求 | 1.21+(Kron 1.27 ✅) |
| License | Apache-2.0 / MIT / CC-BY-4.0 |
| Stars | 5057+ (官方 + Google 背书) |

**为什么不升级 v1.7+/v1.8+(spec 2026-07-28)**:
- 2026-07-28 spec 是大改:移除 initialize handshake,引入 server/discover RPC,
  多轮重 stream,废除 roots/sampling/logging 等。
- 升级带来的协议层破坏 + 客户端(Claude Desktop / Cursor)尚未全部支持新 spec。
- **保守策略**:v1.0.x 锁住,等客户端普及 2026-07-28 再评估升级。
- v1.0.0 → v1.1.0 是 patch 升级(同 spec),自动 follow。

**未来升级路径**(记录在此,不动):
- 升 v1.7+ 时:跑 `mcp.NewServer` + `AddTool` 兼容性测试
- 评估 `server/discover` 是否启用(`ServerOptions.SupportedProtocolVersions` 配置)
- 移除 Claude Desktop 旧版客户端的 `initialize` 握手 backport(若需要)

---

## 3 实施 — 5 个 commit

### Commit 1:`go.mod` + 引入 SDK + 替换 stdio 主循环

**改动**:
- `go.mod`:加 `github.com/modelcontextprotocol/go-sdk v1.0.0`
- `go.sum`:自动更新
- `cmd/kron/serve-mcp/serve_mcp.go`:用 `mcp.NewServer` + `server.Run(ctx, &mcp.StdioTransport{})`
  替换当前 `kv + goroutine + 12-handler-direct-dispatch` 主循环
- 暂**不改** 12 handler 签名(下一步)

**关键代码**(`serve_mcp.go`):

```go
package serve_mcp

import (
    "context"
    "log"

    "github.com/modelcontextprotocol/go-sdk/mcp"
)

// serverName / serverVersion are surfaced to MCP clients during initialize.
// Update Version with each Kron release.
const (
    serverName    = "kron"
    serverVersion = "v1.2.0"
    protocolVersion = "2025-06-18" // matched to go-sdk v1.0.x
)

// Run starts the MCP stdio server. It blocks until stdin closes or an
// unrecoverable transport error occurs.
func Run(ctx context.Context) error {
    server := mcp.NewServer(&mcp.Implementation{
        Name:    serverName,
        Version: serverVersion,
    }, &mcp.ServerOptions{
        // Pin to MCP spec 2025-06-18 (SDK v1.0.x default).
        // Future versions: extend this slice.
    })

    // 12 tool registrations go here in commit 2.
    registerTools(server)

    log.SetOutput(stderrOrNull()) // MCP requires stdout to be JSON-only
    if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
        return fmt.Errorf("serve-mcp: %w", err)
    }
    return nil
}

func stderrOrNull() io.Writer {
    // stdio transport: stdout is for JSON-RPC, stderr for logs.
    // Returning os.Stderr unconditionally is the canonical pattern.
    return os.Stderr
}
```

**架构验证**:
- `serve_mcp.go` 在 `cmd/kron/serve-mcp/` ✅(access layer 可 import approved deps)
- `mcp.NewServer` 创建的 server instance **不** 传给 `internal/` ✅

**测试**:
- 不改任何 handler 测试 — 协议层握手由 SDK 保证
- 现有 `serve_mcp_test.go` 的 stdio e2e 测试**需改 expected response**(从 `{"result":{...}}` 到 `{"result":{"content":[...],"structuredContent":{...}}}`)— 这是 commit 3 的范围

**工作量**:~60 行 + `go get` + `go mod tidy`

---

### Commit 2:12 handler 改签名 + `mcp.AddTool` 注册

**目标**:12 业务 handler 改为 SDK 风格的 `(ctx, *mcp.CallToolRequest, In) → (*mcp.CallToolResult, Out, error)` 签名

**改动**:
- 12 个 `handlers_*.go`:改签名,**业务逻辑几乎不动**
- 删:`tools.go` 里 `defaultToolRegistry` map(SDK 自己管理)
- 删:`handlers_*.go` 里的 `toolHandler` 类型 + `assertCallerMCP` guard(SDK 自己处理 ctx 注入)
- 新增:`tools_schema.go`(新)— 12 handler 的 `In`/`Out` Go struct + `mcp.AddTool(server, &mcp.Tool{...}, HandlerFunc)`

**示例**(`handlers_lint.go` 改后):

```go
package serve_mcp

import (
    "context"

    "github.com/modelcontextprotocol/go-sdk/mcp"

    "github.com/xxx/kron/internal/lint"
)

type LintInput struct {
    Reporter string `json:"reporter" jsonschema:"text or json (default text)"`
}

type LintOutput struct {
    Passed  bool   `json:"passed"           jsonschema:"whether all rules passed"`
    Errors  int    `json:"errors"           jsonschema:"number of error-severity diagnostics"`
    Warnings int   `json:"warnings"         jsonschema:"number of warning-severity diagnostics"`
    Diags   []lint.Diag `json:"diags"     jsonschema:"diagnostic list"`
}

func HandleLint(ctx context.Context, _ *mcp.CallToolRequest, in LintInput) (
    *mcp.CallToolResult, LintOutput, error,
) {
    diags, err := lint.Run(ctx, ".") // root resolution TBD
    if err != nil {
        return nil, LintOutput{}, fmt.Errorf("lint: %w", err)
    }
    return nil, LintOutput{
        Passed:   !lint.HasErrors(diags),
        Errors:   countBySeverity(diags, lint.SeverityError),
        Warnings: countBySeverity(diags, lint.SeverityWarning),
        Diags:    diags,
    }, nil
}

func registerLint(server *mcp.Server) {
    mcp.AddTool(server, &mcp.Tool{
        Name:        "kron_lint",
        Description: "Scan .kron/intents/ for stale references, dangling depends_on, and frontmatter anomalies. Exit semantics: passed=true means no error-severity diagnostics.",
    }, HandleLint)
}
```

**SDK 自动做的**(替代了 RFC 5 commits 全部):
- 从 `LintInput` struct 推断 `inputSchema`(type=object + properties + required)
- 从 `LintOutput` struct 推断 `outputSchema`
- 验证入参 vs schema,失败时返回 `CallToolResult{IsError:true}`
- 业务错误 → 自动包成 `result.isError=true`
- 业务成功 → 自动包成 `result.content=[{type:"text",text:JSON}] + result.structuredContent=Out`

**Caller 注入**(架构 §〇 #4 不破):
- `mcp.AddTool` 注册的 handler 第一参数仍是 `ctx context.Context`
- serve-mcp 的 `Run` 启动时**不**需要主动 `model.WithCaller` — 因为 SDK 自己在 ctx 里塞了它自己的 session 信息
- **关键决定**:`HandleXxx` 内部**仍**调 `model.WithCaller(ctx, "mcp:"+client)` 让 `internal/` 看到 caller(架构约定 #4 不破)
  - 怎么拿 `client`?SDK 的 `req.Session` 字段应该有 clientInfo
  - **具体接入点**:在 `Run` 启动时挂 `server.AddReceivingMiddleware(middlewareInjectCaller)`,统一改写 ctx

**Test 改动**:
- `serve_mcp_test.go` 每个测试前不再发 `initialize`/`notifications/initialized`(SDK 自动)
- expected response 从 `{"result":{passed:true,...}}` 改为 `{"result":{"content":[...],"structuredContent":{passed:true,...},"isError":false}}`
- 或者更简洁:测试 helper `callTool(t, server, "kron_lint", args) (Output, error)`,不再手解 JSON-RPC

**工作量**:
- 12 handler × ~30 行签名变化 = ~360 行
- tools_schema.go:1 个文件,~150 行(registerTools 调用 12 个)
- 测试改 expected:~12 测试文件 / ~30 测试 case / ~100 行修改
- **总:~600 行变化,~0 行新增**

---

### Commit 3:测试套 + 端到端 + Claude Desktop 接入

**目标**:MCP Inspector 能列 12 工具,Claude Desktop 能连

**改动**:
- `serve_mcp_test.go` + 12 handler 测试文件:统一改用 SDK 风格 helper
- 新增:`e2e_test.go` 用 `mcp.NewClient` + `mcp.CommandTransport` 在测试里跑真实 stdio 流程
  - 创建 temp repo, 写 .kron/, 启动 server (subprocess), client.Connect → callTool → 验证
  - 这是 RFC §1.4 提到的 "MCP Inspector 端到端"
- 新增:`docs/how-it-works.md` §5 实操示例(用 `claude_desktop_config.json`)

**架构验证**(再次):
- `e2e_test.go` 在 `cmd/kron/serve-mcp/` ✅
- 不 import `internal/` 的测试逻辑 — 验证 internal 边界

**工作量**:~250 行新代码 + ~100 行测试改写

---

### Commit 4:`AGENTS.md` 破例条目 + `architecture.md` 同步

**目标**:架构文档显式记录"mcp 是 approved exception",供未来 agent 参考

**改动**:
- `AGENTS.md` §0 铁律 #5 改 "Zero new dependencies (except: see architecture.md §2.4 'Approved dependencies')"
- `AGENTS.md` 同步 §"Things that are off-limits without explicit ask" 第 1 条:`"Adding new top-level dependencies — except: re-adding currently-approved deps from architecture.md §2.4"`
- `docs/abstractDesign/architecture.md` §2.4(新增)"Approved top-level dependencies":
  - 列表:`mcp-go-sdk v1.0.x`(破理由 2026-10-04)
  - 列表:`stretchr/testify`(测试)
  - 列表:`yaml.v3`/`go.yaml.in/yaml/v3`(数据格式)
- `docs/abstractDesign/architecture.md` §1.2 工具集表注:"via MCP `tools/call` since v1.2 (uses modelcontextprotocol/go-sdk)"

**动机**:让 AGENTS.md "off-limits" 列表**自描述**例外审批流程,避免下次有人要加 dep 时迷惑。

**工作量**:~30 行文档改动

---

### Commit 5:文档同步 + RFC 归档

**目标**:docs 与代码完全对齐

**改动**:
- `docs/implementation/mcp.md` §2 工具契约:加"via `tools/call` since v1.2" + 工具表精简(去掉手工 JSON Schema,引用代码 struct)
- `docs/implementation/mcp.md` §4 协议与部署:重写为 "uses modelcontextprotocol/go-sdk; 12 工具通过 `mcp.AddTool` 注册"
- `docs/how-it-works.md` §5:更新示例
- `README.md`:加 Claude Desktop / Cursor 接入片段
- 归档:`docs/rfc/2026-10-04-mcp-protocol-redesign.md` 加 header "SUPERSEDED by RFC 2026-10-04-mcp-sdk-adoption.md"
- 保留:`docs/rfc/2026-10-04-mcp-sdk-selection.md`(决策理由)
- 新增:`docs/rfc/2026-10-04-mcp-sdk-adoption.md`(本文)

**工作量**:~120 行文档

---

## 4 总工作量估算

| Commit | 工作量 | 累计 |
|---|---|---|
| 1. 引入 SDK + 替换 stdio 主循环 | 0.5h | 0.5h |
| 2. 12 handler 改签名 | 2h | 2.5h |
| 3. 测试套 + e2e | 1.5h | 4h |
| 4. AGENTS.md 破例条目 + architecture 同步 | 0.5h | 4.5h |
| 5. 文档同步 + RFC 归档 | 1h | 5.5h |
| **总计** | **~5.5 小时** | |

vs 原 RFC 5 commits 手写计划:~10.5h → **节省 ~5h**

---

## 5 风险与缓解

| 风险 | 严重度 | 缓解 |
|---|---|---|
| 12 handler 改签名引入 bug | 中 | 测试完整覆盖;改完跑全测 |
| SDK 升级未来破坏 API | 中 | pin v1.0.x;升级时跑兼容测试 |
| `e2e_test.go` subprocess 启动慢 | 低 | 仅 CI 跑;本地默认 skip |
| 业务 ctx 注入 caller 没接好,破架构 #4 | 中 | commit 2 中间件 `InjectCallerMiddleware` 强制注入 |
| mcp 包的传递 dep 把 binary 变大 | 低 | go-sdk 本身 ~150 KB;oauth 等传递 dep **不** 进 final binary(Go linker 静态分析 unused) |
| 退出困难(SDK 不再维护) | 高 | 业务代码与 SDK 仅在 `serve_mcp.go` 边界接触;回退只需重写协议层(~700 LOC) |

---

## 6 不在范围(明确)

- ❌ Streamable HTTP transport — v1.2 仅 stdio
- ❌ Resources / Prompts 暴露 — 留 phase 3
- ❌ OAuth 2.1 / Authorization — stdio 不需要
- ❌ Sampling / Elicitation — Kron 是 server 端
- ❌ `notifications/tools/list_changed` — 工具集静态
- ❌ `notifications/cancelled` — 工具都同步
- ❌ 输出 progress notifications — 12 工具都<1s
- ❌ `cmd/kron/serve-mcp/` 重命名为 `cmd/kron/serve-stdio/` — 不动(向后兼容)
- ❌ 删 `assertCallerMCP` + caller 系列 — **保留**(架构约定 #4 防御层)

---

## 7 验收清单

每个 commit 后:
- [ ] `go vet ./...` 干净
- [ ] `gofmt -l .` 干净
- [ ] `go test ./...` 全绿
- [ ] `go build ./...` 无 unused import 警告
- [ ] 当前 commit 触动的文件 ≤ commit 范围

最终 commit 5 后:
- [ ] MCP Inspector 连上 kron serve-mcp 能 list 12 工具
- [ ] Claude Desktop 配置 `claude_desktop_config.json` 能成功添加 kron
- [ ] LLM 在 Claude Desktop 中说"列我的 intents"能成功调用 `kron_lint` 或 `kron_list`
- [ ] `internal/**` 用 grep `modelcontextprotocol` 验证零 import
- [ ] `cmd/kron/cli/**` 用 grep `modelcontextprotocol` 验证零 import
- [ ] `go.mod` 仅一个 `mcp` 相关 require
- [ ] `docs/rfc/` 有本文 + sdk-selection.md;protocol-redesign.md 标 SUPERSEDED

---

## 8 决策点(等你拍板)

- [ ] **D1**:批准引入 `github.com/modelcontextprotocol/go-sdk v1.0.0` 破例 dep
- [ ] **D2**:`internal/` 是否真的零业务 inst 化(只是 `cmd/kron/serve-mcp/` 引 SDK)— 默认是
- [ ] **D3**:`assertCallerMCP` 是否仍保留(架构 #4 防御) — 默认保留
- [ ] **D4**:e2e_test 跑真实 subprocess 是否可接受(本地略慢 ~3s) — 默认是
- [ ] **D5**:`notifications/tools/list_changed` 静态工具集声明 — 默认 false)

---

## 10 关联文档(实施完后的状态)

| 文件 | 状态 |
|---|---|
| `docs/rfc/2026-10-03-frontmatter-references.md` | 归档(commit a373828 已落地) |
| `docs/rfc/2026-10-04-mcp-protocol-redesign.md` | SUPERSEDED(本文取代) |
| `docs/rfc/2026-10-04-mcp-sdk-selection.md` | 决策记录(保留) |
| `docs/rfc/2026-10-04-mcp-sdk-adoption.md` | **本文**(实施计划) |
| `docs/process/mcp-protocol.md` | 待归档到 `docs/process/_archive/`(commit 5) |
| `AGENTS.md` | §0 铁律 #5 改"except approved deps";off-limits 列表改"(except approved)" |
| `docs/abstractDesign/architecture.md` | §2.4 新增 "Approved dependencies";§1.2 工具集注 |
| `go.mod` | +1 require `modelcontextprotocol/go-sdk v1.0.0` |
| `cmd/kron/serve-mcp/serve_mcp.go` | 重写为 SDK 风格启动 |
| `cmd/kron/serve-mcp/handlers_*.go` (12) | 签名改 SDK 风格 |
| `cmd/kron/serve-mcp/tools_schema.go`(新) | 12 handler register 调用 |
| `cmd/kron/serve-mcp/rpc.go` | 删 |
| `cmd/kron/serve-mcp/tools.go` | 删 defaultToolRegistry,留 SDK-style 工具函数 |