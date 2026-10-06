# MCP SDK 选型:go-sdk vs 手写(2026-10-04 调研)

> **状态:已采纳** — 官方 SDK (`github.com/modelcontextprotocol/go-sdk/mcp`) 已采用；实施见 [`2026-10-04-mcp-sdk-adoption.md`](./2026-10-04-mcp-sdk-adoption.md)，落地于 commit `55820ed refactor(serve-mcp): adopt official go-sdk` (2026-10-04)。本文作为**决策记录**保留，不再引导新决策。
> 关联: [`2026-10-04-mcp-protocol-redesign.md`](./archive/2026-10-04-mcp-protocol-redesign.md) (5 commit 手写计划,被本文挑战) — **SUPERSEDED，归档**
> 调研日期:2026-10-04

---

## 0 结论先行

| 维度 | 官方 SDK (`modelcontextprotocol/go-sdk/mcp`) | 手写(原 RFC 计划) |
|---|---|---|
| 工作量 | **~2-3 小时** | ~10.5 小时 |
| 协议兼容性 | **自动跟踪最新 spec**(当前 2026-07-28) | 仅 2025-06-18,要手动追 spec 演进 |
| 代码量 | 0 行协议代码,只写 12 工具 handler | ~600 行协议代码 + 12 工具 |
| 长期维护 | SDK 升级即可,免费拿到未来 spec 支持 | 每次 spec 升级都要手动改 |
| 依赖 | **新加 1 个 dep** (官方 SDK + 传递 dep) | 0 新 dep |
| **符合 AGENTS.md** | ❌ **"新加 top-level dep 需显式批准"** | ✅ 0 新 dep |
| LLM 友好 | 自动 `content` + `structuredContent` + `isError` | 需手写 envelope |

**建议**: **用官方 SDK**,但需你**显式批准**破例加 dep,理由见 §5。

---

## 1 Go 生态现状(2026-10-04 调研)

按"成熟度/可获取性"排序:

| 库 | 维护方 | Tier / Stars | 状态 | Kron 适用性 |
|---|---|---|---|---|
| `github.com/modelcontextprotocol/go-sdk/mcp` | **官方 + Google** | **Tier 1**, 5057 ⭐ | **v1.0.0+ GA**, v1.7+ 支持 spec 2026-07-28 | ✅ **首选** |
| `github.com/mark3labs/mcp-go` | 社区 (edzynda) | 9150 ⭐ | 稳定, 影响官方 SDK 设计 | ✅ 备选 |
| `github.com/klarlabs-studio/mcp-go` | 社区 | (生产级框架) | 较新, 中间件多 | ❌ 过重 |
| `github.com/anatolykoptev/go-mcpserver` | 社区 | (薄 boot 库) | 基于官方 SDK | ❌ 不必加层 |
| `BackendStack21/go-mcp` | 社区 | (零依赖) | 较小众 | ❌ 未验证 |

来源:
- 官方 SDK 主页: `https://github.com/modelcontextprotocol/go-sdk`(2025-04-23 创建, Apache-2.0)
- SDK Tiering: `https://modelcontextprotocol.io/docs/2026-07-28/sdk`
- 2026 Go 生态比较: `https://fast.io/resources/mcp-server-golang/`

---

## 2 官方 SDK 关键 API 表面

### 2.1 创建 server + 注册工具(`AddTool`)

来自 `https://go.sdk.modelcontextprotocol.io/server/` §Tools:

```go
import "github.com/modelcontextprotocol/go-sdk/mcp"

// 1. 创建 server
server := mcp.NewServer(&mcp.Implementation{
    Name:    "kron",
    Version: "v1.2.0",
}, nil) // nil = 默认 options

// 2. 注册一个工具(handler 函数签名 = (ctx, req, In) → (*Result, Out, error))
type LintInput struct {
    Reporter string `json:"reporter" jsonschema:"text or json"`
}
type LintOutput struct {
    Passed  bool   `json:"passed"`
    Summary string `json:"summary"`
}

func HandleLint(ctx context.Context, req *mcp.CallToolRequest, in LintInput) (
    *mcp.CallToolResult, LintOutput, error,
) {
    out := LintOutput{Passed: true, Summary: "0 errors, 0 warnings"}
    return nil, out, nil
}

mcp.AddTool(server, &mcp.Tool{
    Name:        "kron_lint",
    Description: "Run lint on .kron/intents/; returns diagnostics",
}, HandleLint)

// 3. 启动 stdio server
if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
    log.Fatal(err)
}
```

### 2.2 SDK **自动做**的事(原 RFC 5 个 commit 的全部内容)

来自官方文档:

> `AddTool` does the following automatically:
> - If `Tool.InputSchema` is unset, the input schema is **inferred from the `In` type** (must be struct or map)
> - If `Tool.OutputSchema` is unset and the `Out` type is not `any`, the output schema is **inferred from the `Out` type**
> - `jsonschema` struct tags provide argument/output descriptions
> - Tool arguments are **validated against the input schema**
> - Tool arguments are **marshaled into the `In` value**
> - Tool output is **marshaled into the result's `StructuredContent`, as well as the unstructured `Content`**
> - Output is **validated against the tool's output schema**
> - If an ordinary error is returned, it is stored in `CallToolResult` and `IsError` is set to `true`

**这覆盖了 RFC 全部 5 个 commit**:
- Commit 1(initialize + ping + ID 处理)→ SDK 自动
- Commit 2(tools/list + JSON Schema)→ SDK 自动从 Go struct 推断
- Commit 3(tools/call + envelope)→ SDK 自动
- Commit 4(mini JSON Schema validator)→ SDK 自动
- Commit 5(文档同步)→ 仍需 ~1h

### 2.3 stdio transport(我们的目标)

```go
// 一行启动
server.Run(ctx, &mcp.StdioTransport{})

// SDK 内部做的:
// - read JSON-RPC messages from stdin
// - write JSON-RPC responses to stdout
// - write logs to stderr
// - handle initialize handshake
// - handle notifications/initialized
// - handle tools/list, tools/call
// - handle ping
// - handle notifications/cancelled (MCP 2025-06-18+)
// - protocol version negotiation
```

**完全替换当前 `cmd/kron/serve-mcp/rpc.go`** (~190 行 + 12 handler 共 ~1500 行业务代码)中的协议层。

---

## 3 对比方案详细

### 方案 A:用官方 SDK (推荐)

**代码量变化**:
- 删:`cmd/kron/serve-mcp/rpc.go` (~190 行,协议层)
- 删:`cmd/kron/serve-mcp/protocol.go` (RFC 计划新建,~120 行)
- 删:`cmd/kron/serve-mcp/tools_schema.go` (RFC 计划新建,~250 行)
- 删:`cmd/kron/serve-mcp/wire_envelope.go` (RFC 计划新建,~150 行)
- 改:12 个 handler 改签名,从 `func(ctx, json.RawMessage) (any, error)` 改为 `func(ctx, *mcp.CallToolRequest, In) (*mcp.CallToolResult, Out, error)` (~600 行变化)
- 改:`cmd/kron/serve-mcp/serve_mcp.go` (~85 行 → ~30 行,只剩 stdio 启动)
- 改:`go.mod` 加 `github.com/modelcontextprotocol/go-sdk`
- 改:测试套件改 expected response(从 `{intents:[]}` → `{content:[{text:"..."}], structuredContent:{intents:[]}, isError:false}`)
- **净增**:约 +200 行(dep 的代码不在仓库内,但**协议代码净删 700+ 行**)

**优势**:
- 工作量从 10.5h → **2-3h**
- 未来 spec 升级(2026-07-28, 2027-...)SDK 自动跟随
- 自动拿到 LLM 友好的 `content` + `structuredContent` + `isError` 信封
- 测试更稳定(协议层由 SDK 维护,我们只测业务)

**风险**:
- AGENTS.md "新加 top-level dep 需显式批准" — 需你拍板
- 现有 12 handler 测试需大改 expected response
- 业务代码改签名有 bug 风险(12 个 handler)

### 方案 B:手写(原 RFC 计划)

保留当前 0 新 dep 状态,实现全部 5 个 commit。

**优势**:
- AGENTS.md 完全合规
- 12 handler 业务代码零改(只改 envelope)

**劣势**:
- 工作量大(10.5h)
- 未来 spec 升级要手动跟随
- 重写 SDK 已做的工作

### 方案 C:不接入 MCP 标准

维持现状,12 工具只在内部用。

**评估**:不是合理选项 —— AGENTS.md 写"MCP server"是 v1 设计目标,12 工具做完却不能被任何客户端调,价值存疑。

---

## 4 关键决策点

### 4.1 选哪个 SDK?

| 选项 | 推荐度 | 理由 |
|---|---|---|
| **官方 `modelcontextprotocol/go-sdk`** | ⭐⭐⭐ | Tier 1, Google 背书, 自动跟 spec |
| 社区 `mark3labs/mcp-go` | ⭐⭐ | 较老, 社区影响大, 但非官方 |
| 其他(3 个) | ⭐ | 不必考虑 |

### 4.2 是否破例批准"加新 dep"?

**AGENTS.md §0 铁律 #5 明确说** "Zero new dependencies" 是铁律。
但同文件也说"Things that are off-limits **without explicit ask**" — **明确批准机制存在**。

**破例理由**:
1. SDK 是 Tier 1 官方库,Apache-2.0 协议,Google 背书
2. 把 10.5h 协议代码量降到 ~700 行业务签名变化
3. 未来 spec 升级免费得到支持
4. Kron 本身就是 MCP server,**与 SDK 是"协作"而非"对抗"关系**

**反方意见**(如果选择 B):
1. Kron 设计原则"零隐式状态" — 加 SDK 引入了隐式 dep,可能影响 binary 自举
2. `go.mod` 已有 testify / yaml.v3,加 SDK 是第 3 个 dep,**进入"运行时框架"层**
3. 12 工具规模小,手写可控

### 4.3 引入方式

| 方式 | 工作量 | 兼容性 |
|---|---|---|
| `go get github.com/modelcontextprotocol/go-sdk/mcp` 然后改签名 | 最小 | spec 2026-07-28 默认 |
| `pin v1.0.0` 不升级,稳定 spec 2025-06-18 | 中等 | 与 RFC 计划的 spec 一致 |
| 引入 `mark3labs/mcp-go` 而非官方 | 略小 | 较老,可能落后 spec |

**建议**:`pin v1.0.0` 或 `v1.1.0`,锁定 spec 2025-06-18(与当前 RFC 一致)。
SDK 升级到 v1.7+ 时再评估(spec 2026-07-28 是大改)。

---

## 5 建议路线(等你拍板)

### 路线 1:批准官方 SDK, 2-3h 完成

1. **commit 1**:`go.mod` 加 SDK, `cmd/kron/serve-mcp/serve_mcp.go` 改用 `mcp.NewServer` + `mcp.StdioTransport`,删 `rpc.go` 协议层
2. **commit 2**:12 handler 改签名 + 加 `mcp.AddTool` 调用,删旧 `defaultToolRegistry` 风格
3. **commit 3**:测试套件改 expected response
4. **commit 4**:文档同步(`docs/implementation/mcp.md` §4 协议与部署 简化,加 SDK 引用)

### 路线 2:不批准, 走原 RFC 10.5h

5 个 commit 全部按 `docs/rfc/2026-10-04-mcp-protocol-redesign.md` §3 执行。

### 路线 3:折中

引入 SDK 但**只用于 initialize/ping/protocol 层**,**12 业务 handler 仍走 `toolHandler` 旧风格**,
通过 SDK 的"low-level `AddTool(*mcp.Tool, mcp.ToolHandler)`" API 接入。
**好处**:12 handler 业务代码零改,只改 envelope。
**坏处**:失去 `In/Out` 类型推断,需要手写 JSON Schema(回到 RFC Commit 2 的工作)。
**评估**:不如路线 1 干净。

---

## 6 风险表(路线 1 视角)

| 风险 | 严重度 | 缓解 |
|---|---|---|
| 12 handler 改签名引入 bug | 中 | 测试套完整覆盖,改完跑全测 |
| SDK 与现有 `caller` API 冲突 | 低 | SDK 自己有 ctx,但我们的 `model.WithCaller` 不冲突 |
| SDK 未来大改 API,我们跟不上 | 中 | pin v1.x,自己控制升级节奏 |
| 依赖膨胀 | 中 | SDK 自己也用 `jsonschema` 包(官方子项目) |
| 第三方安全审计 | 中 | Apache-2.0 + Google 背书,社区审计 |
| 退出困难 | 高 | 业务代码(`internal/store` / `internal/parser`)与 SDK 零耦合,只 12 handler 受影响;若必须退出,12 handler 改回原签名即可 |

---

## 7 决策项(等你拍板)

- [ ] **A1**: 批准 `github.com/modelcontextprotocol/go-sdk` 作为新 dep(破例 AGENTS.md §0 铁律 #5)
- [ ] **A2**: 用 `mark3labs/mcp-go` 替代(社区维护,非官方)
- [ ] **A3**: 用其他 SDK
- [ ] **B**: 走原 RFC 手写 5 commit 计划,0 新 dep
- [ ] **C**: 维持现状,12 工具仅内部用(放弃 MCP 兼容)
- [ ] **Pin 策略**: 用 `v1.0.0` / `v1.1.0` 锁 2025-06-18 spec / 还是用 latest 跟 2026-07-28

---

## 8 文档关联

- 旧 RFC (待废弃): `docs/rfc/2026-10-04-mcp-protocol-redesign.md`
- 当前差距清单: `docs/process/mcp-protocol.md`
- AGENTS.md 铁律 #5: `AGENTS.md` line 101
- off-limits 列表: `AGENTS.md` line 160
- 现有 go.mod 状态: `go.mod` (3 deps: testify, yaml.v3, go.yaml.in/yaml/v3 indirect)

---

## 9 后续动作(决策后)

如果选 A1:
1. 写一份 `docs/rfc/2026-10-04-mcp-go-sdk-adoption.md` 详细计划(覆盖 4 个 commit, ~2-3h 工时)
2. 在 `docs/abstractDesign/architecture.md` §〇 加一个"approved deps exception"段落
3. 实施 4 个 commit,每个 commit 跑 `go vet / gofmt / go test / 端到端`
4. 文档同步 + Claude Desktop 端到端验证
