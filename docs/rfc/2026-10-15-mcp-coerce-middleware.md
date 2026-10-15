# RFC: MCP 桥 Coerce Middleware（v1.2 补丁）

| 字段 | 值 |
|---|---|
| **状态** | **待 zhangjun005 拍板** |
| **作者** | AI assistant |
| **创建日期** | 2026-10-15 |
| **修订** | r1 2026-10-15 15:20: 初始草案; 基于 2026-10-15 实测 (r3 + r4 + c 轮 raw-probe) |
| **目标版本** | v1.2 补丁 (不涉及 v1.3 范围) |
| **上游** | `docs/snapshot/2026-10-15-McpRealUsage.md` §9.5.2 (B1-B3 Cursor 桥 bug); §9.6.4 (B4 server 端 bug); 本次 c 步 raw-stdio 完整验证 |

---

## 1 动机

### 1.1 Cursor MCP 桥把参数类型编错

2026-10-15 实测发现 Cursor (以及 Claude Code 等 AI 工具的 MCP 桥) 在调用 MCP tool 时,
会把 Go 类型期望的原始值编为 JSON string 或 object:

| Bug | 触发场景 | 传入值 | 期望类型 | 实际收到 | 错误 |
|---|---|---|---|---|---|
| **B1** | `kron_get {include_relations: true}` | `true` | `bool` | `"true"` (string) | `-32602: type "string", want "boolean"` |
| **B2** | `kron_update {reviewers: [...]}` | `["@alice"]` | `[]string` | `{"item":"@alice"}` (object) | `-32602: type "object", want "array"` |
| **B3** | `kron_stale {days_threshold: 0}` | `0` | `int` | `"0"` (string) | `-32602: type "string", want "integer"` |

这三个 bug **全在 Cursor 桥层**, 不是 kron server 错。同一份代码走 raw stdio (Go client 直连) 全 PASS。

根因: AI 模型在 JSON-RPC 里把参数值写成了 string/object, MCP SDK 用 Go struct 的 `json.Unmarshal` 解析失败。

### 1.2 Server 端也有同类问题 (B4)

`kron-drift-check` prompt 的 `days_threshold` 参数在 prompt args 里是 `map[string]string` (SDK 强类型)。
当 AI 传 `{"days_threshold": 30}` (number) 时, SDK 内部 unmarshal 进 `map[string]string` 失败, 报:

```
-32000: json: cannot unmarshal number into Go struct field .arguments.days_threshold of type string
```

**同一字段在 tool (`kron_stale`, `int`) 和 prompt (`kron-drift-check`, `map[string]string`) 用了不同的 schema**, 客户端必须记两套编码。

### 1.3 为什么等 Cursor 不现实

搜索结果确认:
- Cursor 论坛报告 145807 / 151180 / 169687 / 83815 **从 2025-05 起就有人报**, 截至 2026-10-15 **仍 open**, 无 fix timeline
- Cursor 3.12.x 反而引入了新的 regression (参数丢失)
- 164361 揭露了新的大 JSON args 拼接 bug, 仍是 open

**Cursor MCP 桥的 encoding bug 是一个已知的、长期存在的、多发的 bug 类, 靠修 Cursor 来解决不现实**。

### 1.4 为什么不能改 schema

有人建议用 `anyOf: [array, string]` 或 `z.coerce.*` 改 schema:
- **改了 schema 就破坏了 strict spec** — schema 不再是类型的真描述
- **raw stdio 客户端 (Go SDK 直连) 仍然用正确类型**, 被迫走 string 路径
- **Cursor 未来修了 bug**, middleware 不影响 (类型匹配时不 coerce, no-op)

正确做法: **不改变 schema, 只在入口处兜底修复类型错误**。

---

## 2 方案

### 2.1 选型: middleware 而非改 schema

跟 HelpCode-ai/anythingmcp PR #169 的做法一致:
- Go: `z.coerce.boolean()` — schema 仍是 `type: boolean`, coerce 层在 handler 入口兜底
- MCP: 在 `CallToolRequest` handler dispatch 前加 middleware, 把 string/object 修正为正确类型

**Schema 保持 strict (MCP spec 的合同), middleware 只在类型不匹配时活跃**。

### 2.2 接入点

SDK 的 `AddReceivingMiddleware` 允许在 request dispatch 前拦截:

```go
// cmd/kron/serve-mcp/serve_mcp.go
server.AddReceivingMiddleware(coerceMiddleware())
```

`coerceMiddleware` 读 `CallToolParamsRaw.Arguments json.RawMessage` (还没 unmarshal 的原始 JSON), 在 JSON 字节层做 coerce, 然后塞回去。

**注意**: `GetPromptRequest` 的 `Arguments` 是 `map[string]string` (不是 RawMessage), 不能在 middleware 层修 B4。
B4 的修复在 handler 层 (见 §2.5)。

### 2.3 Coerce 规则

```go
// cmd/kron/serve-mcp/coerce.go

// CoerceParams 从 raw Arguments JSON 里检测并修正类型错误.
// 它在 SDK unmarshal 之前运行, 所以类型不匹配不会导致 -32602.
func CoerceParams(args json.RawMessage, schema InputSchema) (json.RawMessage, bool) {
    // 读取原始 JSON map
    var raw map[string]any
    if err := json.Unmarshal(args, &raw); err != nil {
        return args, false // 无法解析, 不 coerce
    }
    changed := false

    for name, prop := range schema.Properties {
        expected := prop.Type // "boolean" | "integer" | "number" | "array" | "string" | "object"
        val, ok := raw[name]
        if !ok {
            continue
        }
        coerced, did := coerce(val, expected)
        if did {
            raw[name] = coerced
            changed = true
        }
    }

    if !changed {
        return args, false
    }
    out, err := json.Marshal(raw)
    if err != nil {
        return args, false
    }
    return out, true
}
```

具体 coerce 逻辑:

```
期望 bool, 收到 string ("true"/"false"/"1"/"0")  → bool
期望 bool, 收到其他非 bool                         → 不 coerce (让 unmarshal 报字段错)
期望 int/number, 收到 string ("30"/"-5")          → int/float64
期望 array, 收到 object {"item": x}                → []any{x}  (单元素数组)
期望 array, 收到 string "[...]" 且 HasPrefix "[" → []any (JSON decode)
期望 array, 收到其他非 array                       → 不 coerce
期望 int, 收到 float64 (30.0)                     → int(30)
```

### 2.4 实现: middleware chain

```go
// cmd/kron/serve-mcp/coerce.go

// coerceMiddleware 返回一个 MCP middleware, 它在每个 request dispatch 前
// 修正被 Cursor 桥编错的参数.
func coerceMiddleware() mcp.Middleware {
    return func(next mcp.MethodHandler) mcp.MethodHandler {
        return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
            // 只处理 tools/call
            if method != "tools/call" {
                return next(ctx, method, req)
            }

            // 拿 raw Arguments
            rawParams, ok := req.GetParams().(*mcp.CallToolParamsRaw)
            if !ok {
                return next(ctx, method, req)
            }
            if len(rawParams.Arguments) == 0 {
                return next(ctx, method, req)
            }

            // 拿 schema (从 req 的 Extra 或从已知 tool schema 查)
            schema := getSchemaForRequest(req)
            if schema == nil {
                return next(ctx, method, req)
            }

            // Coerce
            coerced, did := CoerceParams(rawParams.Arguments, schema)
            if did {
                rawParams.Arguments = coerced
            }

            return next(ctx, method, req)
        }
    }
}
```

**Schema 从哪来**: 每个 tool 的 InputSchema 是 `tools_schema.go` 里定义的 Go struct。
在 middleware 层, 我们用已知的 18 个 tool 的 schema (hardcode 一个 `map[toolName]PropertySchema`)。
另一种方案: 在 `CallToolParamsRaw` 里面读 `name` 字段, 再查 schema map。

### 2.5 B4 状态：无法在服务端修复 (需要 SDK 改动)

B4 (`prompts/get kron-drift-check` 的 `days_threshold` 收到 number `30` 而非 string `"30"`) **不是** tool middleware 的范畴，因为 `GetPromptRequest.Params.Arguments` 在 SDK 里类型是 `map[string]string`。更根本的问题是：

- Client 发 `{"days_threshold": 30}` (number) → **整个** `arguments` JSON unmarshal 进 `map[string]string` 失**败** → SDK 返 `-32603` → **handler 根本跑不到**

这不是"参数类型错误"，是"整块 JSON 无效"。没有服务端 middleware 能补救，因为 SDK 在派发之前就 reject 了。

RFC 早期草案曾提出"handler 层改 `parseIntArg` 接受 `any`"——但那只对"已进入 handler 的 string 参数"有效，对"SDK 已在 unmarshal 阶段 reject 掉"的 numeric 值**无效**。所以 handler 层修复**不是** B4 的解法。

#### 实际可工作的路径

如果 client 端修复（发 string 而非 number），整条链路是通的：

```
Client 端:  {"days_threshold": "30"}   ← string（绕开 B4）
           ↓
SDK:        map[string]string 接收  ✓ (成功)
           ↓
Middleware: "prompts/get" → next()   (透明，不动)
           ↓
Handler:   parseIntArg("30") = 30   ✓ (已工作)
```

raw-probe 端到端测试已覆盖这个路径（见 §4.3）。

#### B4 现状总结

| 情况 | Client 发送 | 结果 |
|---|---|---|
| ✅ 已工作 | `"30"` (string) | middleware → handler ✓ |
| ❌ 不工作 | `30` (number) | SDK unmarshal 失败，handler 跑不到 |
| ❌ 无服务端修法 | — | 需等 SDK 加 `GetPromptParamsRaw` 或客户端修 |

**RFC §2.5 结论**: B4 是 SDK 层限制，`docs/snapshot/2026-10-15-McpKnownIssues-CursorBridge.md` 已记录。服务端 coerce middleware 对 B4 无能为力。

### 2.6 覆盖矩阵

| Bug | 触发场景 | 修法 | 覆盖 |
|---|---|---|---|
| B1 | `kron_get {include_relations: "true"}` | middleware coerce | ✅ |
| B2 | `kron_update {reviewers: {item:"@alice"}}` | middleware coerce | ✅ |
| B3 | `kron_stale {days_threshold: "30"}` | middleware coerce | ✅ |
| B4 | `prompts/get kron-drift-check {days_threshold: 30}` | 无服务端修法 | ❌ (SDK 层限制) |

B4 行标注"无服务端修法"：当 client 发送 number 时，SDK unmarshal 阶段已 reject，整条请求不进入 handler 层。`parseIntArg("30")` 只在 string 路径下工作（✅ 已通）。

### 2.7 Cursor 修了 bug 之后会怎样

当 Cursor 修了编码 bug, 正确类型的值会原封不动地通过 middleware:
- `"true"` (string) 不会 coerce 为 bool (因为已经是 string 了, coerce bool 只会找 `"true"/"false"`)
- `["@alice"]` (array) 不会 coerce (因为类型已经对了)

**不会 break**: middleware 的 coerce 逻辑有严格的目标类型匹配, 类型对了就不动。

---

## 3 不做

| 事项 | 原因 |
|---|---|
| 改 tool schema 加 `anyOf: [target, string]` | 破坏 strict spec, raw stdio 客户端被逼走 string 路径 |
| 改 prompt args schema `map[string]any` | 改 SDK 强类型, 需等 SDK 本身支持 |
| 自定义 transport 层拦截 prompts/get raw | 绕过派发是协议层 hack, 不适合长期方案; SDK 支持是唯一干净路径 |
| 改 `internal/` 底层 | middleware 在 MCP 层, 不进 `internal/` |
| 加新 go.mod dep | 纯 stdlib (`encoding/json`) 实现, 无需 external dep |

---

## 4 PR 范围

### 4.1 新增文件

```
cmd/kron/serve-mcp/coerce.go          — coerce middleware (新)
cmd/kron/serve-mcp/coerce_test.go     — 单元测试 (8 fixture, 见 §4.2)
```

### 4.2 单元测试 (coerce_test.go)

```go
func TestCoerceBool(t *testing.T) {
    // B1: "true" → true
    args := `{"include_relations":"true"}`
    schema := PropertySchema{"include_relations": {Type: "boolean"}}
    got, did := CoerceParams(json.RawMessage(args), schema)
    assert.True(t, did)
    var v struct{ IncludeRelations bool }
    json.Unmarshal(got, &v)
    assert.True(t, v.IncludeRelations)
}

func TestCoerceIntFromString(t *testing.T) {
    // B3: "30" → 30
    args := `{"days_threshold":"30"}`
    schema := PropertySchema{"days_threshold": {Type: "integer"}}
    got, did := CoerceParams(json.RawMessage(args), schema)
    assert.True(t, did)
    var v struct{ DaysThreshold int }
    json.Unmarshal(got, &v)
    assert.Equal(t, 30, v.DaysThreshold)
}

func TestCoerceArrayFromObject(t *testing.T) {
    // B2: {"item":"@alice"} → ["@alice"]
    args := `{"reviewers":{"item":"@alice"}}`
    schema := PropertySchema{"reviewers": {Type: "array"}}
    got, did := CoerceParams(json.RawMessage(args), schema)
    assert.True(t, did)
    var v struct{ Reviewers []string }
    json.Unmarshal(got, &v)
    assert.Equal(t, []string{"@alice"}, v.Reviewers)
}

// 不该 coerce 的 (type 已经是正确的)
func TestNoOpWhenTypeMatches(t *testing.T) {
    args := `{"include_relations":true}`
    schema := PropertySchema{"include_relations": {Type: "boolean"}}
    _, did := CoerceParams(json.RawMessage(args), schema)
    assert.False(t, did) // 不需要 coerce
}

// 不该 coerce 的 (非标准字符串值)
func TestNoOpForInvalidBoolString(t *testing.T) {
    args := `{"include_relations":"yes"}` // "yes" 不是标准布尔字符串
    schema := PropertySchema{"include_relations": {Type: "boolean"}}
    _, did := CoerceParams(json.RawMessage(args), schema)
    assert.False(t, did) // "yes" 不是 "true"/"false"/"1"/"0", 不 coerce
}
```

8 个 test case:
1. B1: `{"include_relations":"true"}` → `true`
2. B2: `{"reviewers":{"item":"@alice"}}` → `["@alice"]`
3. B3: `{"days_threshold":"30"}` → `30`
4. B4: `parseIntArg(30)` → `30` (handler 层, 不走 middleware)
5. Raw stdio 回归: `{"include_relations":true}` → 不 coerce
6. Raw stdio 回归: `{"reviewers":["@alice"]}` → 不 coerce
7. Raw stdio 回归: `{"days_threshold":30}` → 不 coerce
8. 不该 coerce: `{"include_relations":"yes"}` (非标准) → 不 coerce

### 4.3 修改文件

```
cmd/kron/serve-mcp/serve_mcp.go         — 加 coerceMiddleware() 到 AddReceivingMiddleware
cmd/kron/serve-mcp/handlers_prompts.go  — parseIntArg 改为接受 any
cmd/kron/serve-mcp/prompts_register.go  — (无改动, schema 声明不变)
```

### 4.4 提交前必跑

```
go vet ./cmd/kron/serve-mcp/...
gofmt -l .
go test ./cmd/kron/serve-mcp/... -run Coerce
```

---

## 5 决策点

| 决策 | 选项 | 默认 |
|---|---|---|
| 5.1 B4 的修法 | a) parseIntArg 改 any / b) 等 SDK 加 GetPromptParamsRaw | (a) handler 层改 |
| 5.2 middleware schema 来源 | a) hardcode 18 个 tool schema / b) 运行时从 registerTools 拿 / c) 每个 tool handler 自行 coerce | (a) hardcode schema map |
| 5.3 coerce 失败时 | a) 原样返回, 让 SDK 报错 / b) 返回明确的 -32602 | (a) 原样返回 |
| 5.4 B2 object→array 的检测方式 | a) 检测 `{"item": ...}` / b) 检测 keys=1 且值不是 array / c) 只在 tool 是 kron_update/kron_add 时激活 | (a) 检测 `{"item": ...}` |

**需要 zhangjun005 拍板: 5.1, 5.2, 5.3, 5.4**。

---

## 6 跟其他 RFC 的关系

| RFC | 关系 |
|---|---|
| `docs/rfc/2026-10-15-mcp-test-expansion.md` | 测试 fixture 模板 (G0) 已完成, 本 RFC 是修复 B1-B4 的工具代码 |
| `docs/rfc/2026-10-04-mcp-sdk-adoption.md` | SDK 的 `AddReceivingMiddleware` 是 v1.2 选型的核心能力 |
| `docs/rfc/2026-10-14-mcp-prompts.md` | B4 修的是 `kron-drift-check` 的 args 解析, 不改 prompt template |
