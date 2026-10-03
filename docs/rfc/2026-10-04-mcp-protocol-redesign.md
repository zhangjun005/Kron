# MCP 协议层重设计:严格 2025-06-18 兼容

> 状态:规划中 (2026-10-04)
> 关联:`docs/process/mcp-protocol.md` (差距清单)、`docs/rfc/2026-10-03-frontmatter-references.md` (业务层 RFC)
> 目标:让 Kron MCP server 能被 Claude Desktop / Cursor / MCP Inspector 标准接入。

---

## 0 背景:当前实现与标准的差距

Kron 当前 `cmd/kron/serve-mcp/` 是 **"JSON-RPC + 自定义方法"** 实现。
12 个工具以 `kron_*` 命名,客户端**直接发** `{"method": "kron_lint"}`。
MCP 2025-06-18 标准要求**所有工具必须经 `tools/call` 包装**,方法名固定,
工具名在 `params.name` 里,且工具结果必须包成 `{content:[...], isError:bool}` 格式。

主要差距:

| 维度 | 当前 | 标准要求 |
|---|---|---|
| `initialize` 手shake | 不支持 | **MUST** (lifecycle §Initialization) |
| `tools/list` 工具发现 | 不支持 | **MUST** (server/tools §Capabilities) |
| `tools/call` 工具调用 | 自定义 `kron_*` 方法名 | **MUST** 用 `tools/call` + `params.name` |
| 工具结果格式 | 直接 JSON 对象 | **MUST** 包成 `content[]` (ContentBlock 数组) |
| `isError` 字段 | 走 JSON-RPC error 通道 | **SHOULD** 走 `result.isError=true`,LLM 才能自我纠错 |
| `structuredContent` | 不支持 | **OPTIONAL** 走 `result.structuredContent={...}`(机器可读) |
| `ping` | 不支持 | optional, 健康检查 |
| `notifications/cancelled` | 不支持 | optional, 取消 in-progress 请求 |
| ID 处理 | 允许 null | **MUST NOT be null** (区别于 base JSON-RPC) |
| 协议版本协商 | 无 | **MUST** 在 initialize 协商 |

完整标准来源:
- 规范索引: https://modelcontextprotocol.io/specification/2025-06-18
- TypeScript schema(真理源): https://github.com/modelcontextprotocol/specification
- 本研究依据的 schema 引用: `agent-tools/14b3475a-8a5f-4c27-a490-f4eacc3f5fef.txt`(2026-10-04 拉取)

---

## 1 设计目标

### 1.1 业务目标

- [P0] 任何标准 MCP Host(Claude Desktop、Cursor、MCP Inspector、Continue.dev)
      能连上 `kron serve-mcp` 而**不**报"method not found"或解析失败
- [P0] 工具调用结果 LLM 能正确解析(走 `content[]` + `isError`)
- [P0] 工具结果在保留结构化的同时满足 MCP 信封(用 `structuredContent` 字段)
- [P1] 工具结果**机器可读**而**人类可读**(LLM 看到 `content` 文本,客户端 UI 看到 `structuredContent` 对象)

### 1.2 非目标(明确不做)

- **不做 Streamable HTTP transport** — v1.1 仍只支持 stdio(简化实施)
- **不做 Resources / Prompts** — v1.1 仅 Tools(Resources 留 phase 3 见 `phase-2-leftovers.md` L3)
- **不做 OAuth 2.1 / Authorization** — stdio 路径 SHOULD NOT 走 MCP auth(从环境变量取凭证)
- **不做 Sampling / Elicitation** — Kron 是 server 端,不需要向 client 发起
- **不做 `notifications/tools/list_changed`** — 工具集静态,声明 `listChanged:false`
- **不做 progress notifications** — 12 工具都<1s 完成,无意义
- **不做 `notifications/cancelled`** — 工具都是同步的,无可取消的 in-progress work

### 1.3 Backward compatibility 策略

**保留 `kron_*` 直调模式作为 deprecated 兼容层**,v1.2 仍可工作,计划 v1.3 移除。

理由:
- 当前测试套(`cmd/kron/serve-mcp/handlers_test.go` 等)直接发 `kron_*` JSON-RPC,改造前不能一次性迁移
- 二次回滚风险大,分阶段做
- v1.1 → v1.2 的过渡期,客户端可以同时支持两种调用方式

实现:在 `defaultToolRegistry` 里加 12 个 `kron_*` 包装,把 `kron_X` 请求重定向到 `tools/call` 内部 dispatcher。
**第一次 `kron_X` 调用时**打 stderr warning(deprecation 提示)。

---

## 2 架构:协议层与业务层分离

### 2.1 现状

```
[stdin] → rpc.go:handleLine → defaultToolRegistry["kron_X"] → handleXxx(ctx, params) → 返回结构化 JSON
```

问题:`rpc.go` 直接路由到业务 handler,没有"协议层"概念。

### 2.2 改造后

```
[stdin] → rpc.go:handleLine
              ├─ method = "initialize"             → handleInitialize(req.params)
              ├─ method = "notifications/initialized" → (no-op, ack)
              ├─ method = "tools/list"              → handleToolsList(req.params)
              ├─ method = "tools/call"              → handleToolsCall(req.params) → 内部 dispatcher
              ├─ method = "ping"                    → handlePing()
              ├─ method = "kron_*" (deprecated)    → handleLegacyDirect(req.method, req.params) → warning + 重定向到 tools/call
              └─ method = 未知                      → -32601
              ↓
        [业务 handler 内部] → 仍返回结构化 JSON(structuredContent)
              ↓
        [rpc.go envelope] → 包成 CallToolResult{content:[text], structuredContent, isError}
              ↓
        [stdout] → JSON-RPC 2.0 response
```

### 2.3 新增包:`internal/mcpwire` (下沉候选)

按 architecture §〇·五·3,**只有 ≥2 个访问层共用才下沉**。当前只有 serve-mcp 用。
但**协议层逻辑**(initialize / tools/list / tools/call envelope 包装)**和具体业务无关**,
未来如果 Kron 出 `serve-stdio-as-library` 给其他 server 嵌入,可以下沉。

**v1.2 决策:不预先下沉,代码全在 `cmd/kron/serve-mcp/` 内的 `protocol.go` 和 `wire_envelope.go`。
   留待 phase 3(serve-gui)启动时再走 `internal-pkg.md` 流程评估下沉。**

---

## 3 实施步骤(分 5 个 commit)

### Commit 1:协议握手 + ID 处理 + ping (P0 阻塞项)

**目标**:客户端能完成 initialize handshake,知道有哪些能力

**改动文件**:
- `cmd/kron/serve-mcp/protocol.go` (新) — 协议层方法注册
- `cmd/kron/serve-mcp/rpc.go` (改) — 改 ID 验证、增加对协议方法的 dispatch
- `cmd/kron/serve-mcp/protocol_test.go` (新) — handshake 协议契约

**关键代码** (`protocol.go`):

```go
// protocolVersion is the MCP spec version Kron supports.
// Bump when implementing new spec features (resources, prompts, ...).
const protocolVersion = "2025-06-18"

// serverInfo is sent in InitializeResult. The "name" is stable;
// "title" and "version" can change between releases.
var serverInfo = struct {
    Name    string `json:"name"`
    Title   string `json:"title"`
    Version string `json:"version"`
}{
    Name:    "kron",
    Title:   "Kron — Git-native intent management",
    Version: "v1.2.0",
}

// serverCapabilities describes what Kron can do on the wire.
// Tools is the only required capability for our scope. We declare
// listChanged: false because the 12-tool set is compiled in.
var serverCapabilities = struct {
    Tools struct {
        ListChanged bool `json:"listChanged"`
    } `json:"tools"`
}{}

// handleInitialize responds to the JSON-RPC handshake (MUST per spec).
// It does not validate the client's protocolVersion; we trust the
// negotiated version on the first initialize request and use it
// for the rest of the session.
func handleInitialize(params json.RawMessage) (any, error) {
    return struct {
        ProtocolVersion string `json:"protocolVersion"`
        Capabilities    any    `json:"capabilities"`
        ServerInfo      any    `json:"serverInfo"`
    }{
        ProtocolVersion: protocolVersion,
        Capabilities:    serverCapabilities,
        ServerInfo:      serverInfo,
    }, nil
}

// handleInitialized is the post-handshake ack. It must produce no
// response (notification) but we still need a no-op entry in the
// dispatch table so the dispatcher doesn't return -32601.
func handleInitialized(_ context.Context, _ json.RawMessage) (any, error) {
    return nil, nil
}

// handlePing is the MCP-defined health check.
func handlePing(_ context.Context, _ json.RawMessage) (any, error) {
    return struct{}{}, nil
}
```

**rpc.go 改动**:

```go
// 1. 拒绝 null id (MCP spec: id MUST NOT be null)
if len(req.ID) == 0 {
    s.writeError(enc, nil, codeInvalidRequest, "request id is required and MUST NOT be null")
    return
}

// 2. ID 已用过? (MCP spec: id MUST NOT be reused)
if s.seenIDs[string(req.ID)] {
    s.writeError(enc, req.ID, codeInvalidRequest, "request id already used in this session")
    return
}
s.seenIDs[string(req.ID)] = true
```

**测试** (`protocol_test.go`):

```go
func TestInitialize_ReturnsServerInfo(t *testing.T) {
    out, _, err := runServer(t, `{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}},"id":1}`+"\n")
    require.NoError(t, err)
    resp := decodeSingleResponse(t, out)
    require.NotNil(t, resp.Result)
    var result struct {
        ProtocolVersion string         `json:"protocolVersion"`
        Capabilities    map[string]any `json:"capabilities"`
        ServerInfo      struct{ Name string `json:"name"` } `json:"serverInfo"`
    }
    require.NoError(t, json.Unmarshal(resp.Result, &result))
    assert.Equal(t, "2025-06-18", result.ProtocolVersion)
    assert.Equal(t, "kron", result.ServerInfo.Name)
    assert.Contains(t, result.Capabilities, "tools")
}

func TestInitialized_NoResponse(t *testing.T) {
    // notifications/initialized is a notification: MUST NOT produce a response.
    out, _, err := runServer(t, `{"jsonrpc":"2.0","method":"notifications/initialized"}`+"\n")
    require.NoError(t, err)
    assert.Empty(t, out)
}

func TestPing_ReturnsEmptyResult(t *testing.T) {
    out, _, err := runServer(t, `{"jsonrpc":"2.0","method":"ping","id":1}`+"\n")
    require.NoError(t, err)
    assert.Contains(t, out, `"id":1`)
    assert.Contains(t, out, `"result":{}`)
}

func TestNullID_Rejected(t *testing.T) {
    // MCP spec: id MUST NOT be null.
    out, _, err := runServer(t, `{"jsonrpc":"2.0","method":"ping","id":null}`+"\n")
    require.NoError(t, err)
    assert.Contains(t, out, `"code":-32600`)
    assert.Contains(t, out, "MUST NOT be null")
}
```

**验收**:
- `claude_desktop_config.json` 配上 `kron` 之后启动 Claude Desktop 不再报"handshake failed"
- 全部 4 个测试通过
- `go vet` / `gofmt` 干净
- 现有 12 handler 测试**继续通过**(未改 business 路径)

**工作量估计**:~120 行新代码 + ~80 行新测试

---

### Commit 2:tools/list 工具发现 (P0)

**目标**:客户端能发现 12 工具的名字、描述、输入 schema

**关键设计点**:`tools/list` 返回的工具 schema 是**机器可读**的 JSON Schema。
当前 schema 在 `docs/implementation/mcp.md` 是 Markdown 描述,需要**代码化**。

**改动文件**:
- `cmd/kron/serve-mcp/tools_schema.go` (新) — 12 工具的 JSON Schema 集中定义
- `cmd/kron/serve-mcp/handle_tools_list.go` (新) — `tools/list` handler
- `cmd/kron/serve-mcp/tools_schema_test.go` (新) — schema 完整性

**schema 集中设计**:

```go
// toolsSchema returns the JSON Schema for each tool's input arguments.
// These are static — they match the docstring in handlers_*.go and the
// docs/implementation/mcp.md §2 contract.
//
// Why hand-rolled map[string]any instead of generating from structs?
// JSON Schema has no native Go type (no tag syntax); writing a
// reflection-based generator would obscure the contract. We keep
// schemas in code so they live next to the handlers that consume them.
var toolsSchema = map[string]map[string]any{
    "kron_init": {
        "type": "object",
        "properties": map[string]any{},
    },
    "kron_add": {
        "type": "object",
        "properties": map[string]any{
            "slug":  map[string]any{"type": "string", "description": "kebab-case slug, must pass parser.ValidateSlug"},
            "symbol": map[string]any{"type": "string"},
            "why":   map[string]any{"type": "string"},
        },
        "required": []string{"slug"},
    },
    "kron_update": {
        "type": "object",
        "properties": map[string]any{
            "slug":       map[string]any{"type": "string"},
            "symbol":     map[string]any{"type": "string"},
            "status":     map[string]any{"type": "string", "enum": []string{"draft", "active", "superseded"}},
            "reviewers":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
            "body":       map[string]any{"type": "string"},
            "references": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
            "depends_on": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
        },
        "required": []string{"slug"},
    },
    // ... 9 more
}
```

**handler**:

```go
// handleToolsList returns the 12-tool manifest. Supports cursor-based
// pagination (currently all 12 fit in one page, but the plumbing is
// here for the day someone adds 50+ tools).
func handleToolsList(params json.RawMessage) (any, error) {
    var p struct {
        Cursor string `json:"cursor"`
    }
    _ = json.Unmarshal(params, &p) // cursor ignored: small list

    tools := make([]map[string]any, 0, 12)
    // IMPORTANT: sort by tool name for deterministic order
    // (MCP spec SHOULD; LLM prompt cache hit rate).
    names := make([]string, 0, len(toolsSchema))
    for n := range toolsSchema {
        names = append(names, n)
    }
    sort.Strings(names)

    for _, n := range names {
        tools = append(tools, map[string]any{
            "name":        n,
            "title":       toolTitles[n],       // human-readable
            "description": toolDescriptions[n], // from handler docstring
            "inputSchema": toolsSchema[n],
        })
    }
    return map[string]any{"tools": tools}, nil
}
```

**验收**:
- `tools/list` 返回 12 个工具,每个含 `name` / `description` / `inputSchema`
- 工具顺序**稳定**(按 name 排序)
- 任何 12 工具的输入参数变更(添加 `references` 等)时,只改 `tools_schema.go` 一处
- gofmt/vet/test 全绿

**工作量估计**:~250 行新代码(12 工具 × ~15 行 schema)+ ~60 行新测试

---

### Commit 3:tools/call 调度 + 结果信封包装 (P0)

**目标**:客户端能调任一工具,结果包成 `CallToolResult` 格式

**关键设计点**:
- `tools/call` 解 `params.name` + `params.arguments`
- 调对应的业务 handler(从 `defaultToolRegistry` 拿,**不复用 `kron_*` 注册表 key**)
- 业务 handler 仍返回结构化 JSON(零改动)
- **`envelope.go` 把返回包装**:`{content:[{type:"text",text:JSON.stringify(result)}], structuredContent:result, isError:false}`
- 业务 handler 返回 `error` 时:`isError:true` + `content:[{type:"text",text:err.Error()}]`,**不**走 JSON-RPC error 通道(spec 明确)

**envelope 设计**:

```go
// envelope wraps a business-handler result into the MCP CallToolResult
// shape. The "content" field is for the LLM (text); the
// "structuredContent" field is for the client UI / downstream code
// (typed JSON object).
//
// If err is non-nil, the result has isError=true and the error message
// goes in content[0].text. The caller should NOT return both result
// and err.
type callResult struct {
    Content          []contentBlock `json:"content"`
    StructuredContent any            `json:"structuredContent,omitempty"`
    IsError          bool           `json:"isError,omitempty"`
}

type contentBlock struct {
    Type string `json:"type"`
    Text string `json:"text"`
}

func envelopeResult(result any, err error) (any, error) {
    if err != nil {
        // Per MCP spec: tool execution errors SHOULD be in result.isError,
        // not as a JSON-RPC error. This is critical for LLM self-correction.
        return callResult{
            Content: []contentBlock{{Type: "text", Text: err.Error()}},
            IsError: true,
        }, nil // NOTE: nil — we are returning a successful envelope with isError
    }
    // Marshal result to JSON for the text field; keep the original as structuredContent.
    textBytes, marshalErr := json.Marshal(result)
    if marshalErr != nil {
        return nil, fmt.Errorf("envelope: marshal result: %w", marshalErr)
    }
    return callResult{
        Content:          []contentBlock{{Type: "text", Text: string(textBytes)}},
        StructuredContent: result,
    }, nil
}
```

**handleToolsCall**:

```go
// handleToolsCall dispatches a tools/call request to the right
// business handler and wraps the result in the CallToolResult envelope.
func (s *server) handleToolsCall(ctx context.Context, params json.RawMessage) (any, error) {
    var p struct {
        Name      string         `json:"name"`
        Arguments map[string]any `json:"arguments"`
    }
    if err := json.Unmarshal(params, &p); err != nil {
        return nil, fmt.Errorf("tools/call: invalid params: %w", err)
    }
    if p.Name == "" {
        return nil, fmt.Errorf("tools/call: name is required")
    }
    handler, ok := s.tools[p.Name]
    if !ok {
        return nil, fmt.Errorf("tools/call: unknown tool: %q", p.Name)
    }
    // Re-marshal arguments as json.RawMessage for the handler signature.
    argsBytes, err := json.Marshal(p.Arguments)
    if err != nil {
        return nil, fmt.Errorf("tools/call: marshal arguments: %w", err)
    }
    result, err := handler(ctx, argsBytes)
    return envelopeResult(result, err)
}
```

**rpc.go 改动**:

```go
// 把 defaultToolRegistry 拆成两层:
// - protocolHandlers: initialize/ping/tools.list/tools.call/notifications.initialized
// - toolHandlers: 12 个 kron_*
//
// handleLine 优先查 protocolHandlers,再查 toolHandlers。
```

**关键决定 — `kron_*` 仍可工作(deprecated)**:

```go
// handleLegacyDirect maps a `kron_*` JSON-RPC method to a tools/call
// internal call. This is a deprecated path; emit a stderr warning
// the first time per session so tests / agents see it.
func (s *server) handleLegacyDirect(ctx context.Context, method string, params json.RawMessage) (any, error) {
    if !s.legacyWarned {
        fmt.Fprintln(s.errOut, "kron serve-mcp: WARNING — using deprecated direct method", method, "— switch to tools/call with params.name =", strings.TrimPrefix(method, "kron_"))
        s.legacyWarned = true
    }
    argsBytes, _ := json.Marshal(map[string]any{}) // legacy methods had no arguments bag
    // Wrap as if it were a tools/call but skip the envelope (return raw result).
    handler, ok := s.tools[method]
    if !ok {
        return nil, fmt.Errorf("unknown method: %q", method)
    }
    return handler(ctx, argsBytes)
}
```

**验收**:
- 12 工具都能通过 `tools/call` 调
- 业务 handler **零改动**(它们仍返回结构化 JSON)
- 老 `kron_*` 调用仍工作,但 stderr 出 deprecation warning
- 业务错误 → `result.isError: true`,JSON-RPC 通道成功
- 协议错误(找不到工具)→ JSON-RPC `-32602` invalid params

**工作量估计**:~150 行新代码 + ~150 行新测试

---

### Commit 4:12 业务 handler 入参校验改用 JSON Schema (P1)

**目标**:把 12 handler 内的手写 `json.Unmarshal` 入参验证替换成 JSON Schema 校验,
**这样 12 handler 共享 `tools_schema.go` 的 schema,schema 是唯一真理源。**

**改 or 不改?**

| 方案 | 收益 | 代价 |
|---|---|---|
| 不改 — handler 继续手写 unmarshal | 12 handler 零改 | 12 handler 的"必填"逻辑和 schema 可能漂移 |
| 改 — 用 `github.com/xeipuuv/gojsonschema` 校验 | schema 唯一真理 | 引入新依赖(AGENTS.md off-limits) |
| 改 — 自己写 100 行 mini JSON Schema validator | 无新依赖,schema 唯一真理 | 100 行,12 工具受益 |

**v1.2 决策**:**不引入新依赖**;**自己写 mini 校验**。
工具入参都很简单(2-5 个字段),100 行 mini 校验足够。

`internal/mcpenv` (暂不下沉) 提供:

```go
// validateArguments checks args against schema. Returns nil if OK.
// Supports: type=string/number/boolean/integer/array/object,
// required[], enum, items.type. Good enough for v1.2's 12 tools.
func validateArguments(schema, args map[string]any) error { ... }
```

`handleToolsCall` 调 `validateArguments(toolsSchema[p.Name], p.Arguments)`,失败时返回 -32602 invalid params。

**验收**:
- schema 是 12 工具的"必填 / 类型 / 枚举"唯一真理源
- 业务 handler 内部仍 unmarshal(零改);但**重复字段**改在 schema 校验阶段拒绝
- 错误消息与 MCP 风格一致(走 `error.data` 含 field path)

**工作量估计**:~150 行 mini validator + ~80 行测试

---

### Commit 5:文档同步 + MCP Inspector 端到端 (P0,合 v1.2 release)

**目标**:docs 与代码完全对齐,Claude Desktop 实测通过

**改动文件**:
- `docs/implementation/mcp.md` §2 工具契约 — 加"via `tools/call`" 说明 + JSON Schema 引用
- `docs/implementation/mcp.md` §4 协议与部署 — 加 `initialize` / `notifications/initialized` / `ping` 流程图
- `docs/abstractDesign/architecture.md` §1.2 — 工具集表注:"via MCP `tools/call` since v1.2"
- `docs/how-it-works.md` §5 — 更新示例(用 `tools/call` 而非 `kron_*`)
- `README.md` — 加 Claude Desktop 配置示例

**端到端验证**:
- 配置 `claude_desktop_config.json` 指向 `kron serve-mcp`
- 在 Claude Desktop 对话里说"list my intents"
- Claude 调用 `tools/list` → 显示 12 工具
- Claude 调用 `tools/call name=kron_list` → 显示意图列表
- 截图存档到 `docs/screenshots/` (git-ignored? 或者文档仓库?)

**工作量估计**:~80 行文档改动 + 1 小时端到端验证

---

## 4 风险与回退

| 风险 | 严重度 | 回退方案 |
|---|---|---|
| LLM 看到 `content[text]` 是 JSON 字符串而非结构化数据 | 中 | commit 3 引入 `structuredContent` 字段,LLM 工具能识别 |
| 现有 12 handler 测试直接发 `kron_*`,commit 3 后变成 deprecated | 低 | 测试继续通过(`kron_*` 兼容),加 stderr warning |
| `clientInfo` 缺失导致 spec 不合规 | 低 | `initialize` 不强制要求服务器读 clientInfo;可加可选解析 |
| Tools 注册表 map 顺序非确定 | 低 | commit 2 已用 `sort.Strings` 排序 |
| Streamable HTTP 客户端(未来)连不上 | 中 | v1.2 不解决,记入 `mcp-protocol.md` backlog |
| `notifications/cancelled` 缺失导致 client 取消失败 | 低 | 当前 12 工具都同步,无 in-progress work,影响为 0 |

---

## 5 测试矩阵

| 层级 | 测试 | 状态 |
|---|---|---|
| 单元 | `caller_test.go` + 5 常量测试 | ✅ 已有,commit 1 不动 |
| 单元 | `protocol_test.go` (新增) handshake 4 测试 | 计划 commit 1 |
| 单元 | `tools_schema_test.go` (新增) schema 完整性 | 计划 commit 2 |
| 单元 | `handle_tools_call_test.go` (新增) envelope 行为 | 计划 commit 3 |
| 单元 | `internal/mcpenv/validate_test.go` mini validator | 计划 commit 4 |
| 集成 | 现有 `handlers_test.go` (12 handler × 多种 case) | ✅ 已有,需小幅改 expected response(包 envelope) |
| 集成 | `handlers_relations_test.go` (3+1 测试) | ✅ 已有,同上 |
| 集成 | `serve_mcp_test.go` (10+ stdio e2e 测试) | ✅ 已有,需加 `initialize` 前置 |
| 端到端 | Claude Desktop 接入 | commit 5 |

**现有测试的影响**:
- `serve_mcp_test.go` 每个测试前要加 `initialize` 步骤(可写一个 helper `initializeTest(t)`)
- `handlers_test.go` / `handlers_relations_test.go` 的 expected `resp.Result` 字段会从 `{intents: [...]}` 变成 `{content: [...], structuredContent: {intents: [...]}, isError: false}` ——需要改 expected 字符串或 unmarshal 目标

---

## 6 时间表(估计,非硬截止)

| 阶段 | 估时 | 累计 |
|---|---|---|
| Commit 1 握手 | 1.5 小时 | 1.5h |
| Commit 2 tools/list | 2 小时 | 3.5h |
| Commit 3 tools/call + envelope | 3 小时 | 6.5h |
| Commit 4 schema 校验 | 2 小时 | 8.5h |
| Commit 5 文档 + e2e | 2 小时 | 10.5h |
| **总计** | **~10.5 小时** | |

每个 commit 后跑 `go vet ./... && gofmt -l . && go test ./...` 确保全绿。

---

## 7 决策点(等你拍板)

- [ ] **业务 handler 内部改动幅度**:12 handler 零改(只改 envelope) vs handler 也加 `structuredContent` 包装
  → 当前 plan:12 handler 零改,envelope 统一处理
- [ ] **`kron_*` 兼容层保留多久**:v1.2 / v1.3 / 永久
  → 当前 plan:v1.2 保留(每 session 一次 warning),v1.3 移除
- [ ] **mini JSON Schema validator 是否值得做**
  → 当前 plan:值得,100 行换 schema 唯一真理源
- [ ] **`notifications/cancelled` 是否要支持**
  → 当前 plan:不做,工具都同步
- [ ] **是否要 Resources / Prompts 顺手做**
  → 当前 plan:不做,留 phase 3

---

## 8 相关文档

- `docs/process/mcp-protocol.md` — 当前差距清单(已被本 RFC 取代,commit 5 后归档)
- `docs/implementation/mcp.md` — 工具契约(待 commit 5 更新)
- `docs/abstractDesign/architecture.md` §1.2 — 工具集定义(待 commit 5 更新)
- MCP 2025-06-18 spec — `https://modelcontextprotocol.io/specification/2025-06-18`
- MCP TypeScript schema — `https://github.com/modelcontextprotocol/specification`(本研究的 54KB 引用在 `agent-tools/14b3475a-...txt`)
