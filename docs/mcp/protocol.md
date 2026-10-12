# MCP 协议入门(给前后端背景、零 MCP 经验的读者)

> **目标**:读完能回答"MCP 协议在做什么、为什么这样设计、Kron 用了哪一部分"。
>
> **不**重复官方 spec——只整理"够用 + Kron 相关"。详细字段表查 [`modelcontextprotocol/go-sdk`](https://github.com/modelcontextprotocol/go-sdk) 源码 `mcp/protocol.go`。
>
> **协议版本**:本仓库 `cmd/kron/serve-mcp/` 当前锁定 `2026-07-28` 协议版本(go-sdk 1.x 默认)。

---

## 一、用 30 秒建立直觉:MCP 是什么

**一句话**:MCP = 让 AI 客户端(Claude / Cursor / 其他 LLM 应用)通过 **JSON-RPC 2.0 over stdio**(或 HTTP)调用**外部工具**的协议。

类比(web 后端视角):
- **MCP server** = 一个 HTTP API 服务,但跑在**进程内 stdio** 上(每个客户端 spawn 一个 server 子进程)
- **tool** = 一个 API endpoint;**MCP client** 就是 AI 客户端
- **JSON-RPC envelope** = 你熟悉的 JSON request/response,但 envelope 是固定的 `{"jsonrpc":"2.0","id":N,"method":"...","params":{...}}`
- **capabilities** = 服务的"我能提供什么"清单;**initialize** 阶段双方交换

**关键设计点**(为什么这么搞):
1. **stdio 进程** = 一个 AI 工具实例 = 一个 `kron serve-mcp` 子进程(架构 §1.0 拍板)
2. **JSON-RPC** = LLM 友好(纯文本,易解析)
3. **capabilities 协商** = 客户端不必假设所有 server 都支持所有 feature(类似 HTTP 头里的 `Accept`)
4. **tool 描述(Description + jsonschema)** = 客户端(LLM)能**自动决策**"该调哪个 tool、传什么参数"

---

## 二、JSON-RPC 2.0 envelope(30 秒复习)

MCP 协议层 = JSON-RPC 2.0。**一帧一行 JSON**(stdio 模式下用 `\n` 分隔):

### 请求(Request)
```json
{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"kron_lint","arguments":{}}}
```

### 响应(Response)
```json
{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"..."}]}}
```

### 错误(Error)
```json
{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"Method not found"}}
```

### 通知(Notification,无 `id`)
```json
{"jsonrpc":"2.0","method":"notifications/tools/list_changed"}
```

### 你需要记住的字段

| 字段 | 必有 | 含义 |
|---|---|---|
| `jsonrpc` | ✅ | 字面 `"2.0"` |
| `id` | 请求/响应需要;通知不要 | 客户端生成;**数字 / 字符串皆可**——MCP go-sdk 用 `int64` |
| `method` | ✅ | 类似 HTTP method;MCP 命名空间 `tools/list` / `tools/call` / `initialize` / `notifications/...` |
| `params` | 视方法 | 工具调用时为 `{"name": "kron_lint", "arguments": {<JSON>}}` |
| `result` / `error` | 二选一 | 类似 HTTP 200 vs 4xx;MCP tool 错误**特殊**——见 §四 |
| `error.code` | 错误必有 | JSON-RPC 标准码 + MCP 自定义码(如 `-32005` 业务错误) |

> **后端类比**:你写过的 HTTP API 把 body 塞在 `request body`;MCP 把方法名 + 参数拆成 `method` + `params`。`id` 等同"trace ID"。

---

## 三、生命周期:从 spawn 到 shutdown

> 这是 stdio 模式下的"客户端连接 server 子进程 → 双向 JSON-RPC 通信 → 客户端关闭 → server 退出"的过程。

```
┌──────────────────┐                          ┌────────────────────┐
│ MCP client       │   spawn 子进程 + stdio   │ kron serve-mcp     │
│ (Claude/Cursor/  │ ───────────────────────► │ (Go process)       │
│  其他 LLM 工具)  │                          │                    │
│                  │   ←── initialize ────►   │   mcp.NewServer()  │
│                  │   ←── tools/list  ──►    │   registerTools()  │
│                  │   ←── tools/call  ──►    │   server.Run()     │
│                  │   ──── close stdin ───►  │   ctx.Done()       │
│                  │                          │   os.Exit(0)       │
└──────────────────┘                          └────────────────────┘
```

### 3.1 三个阶段

| 阶段 | 关键方法 | Kron 现状 |
|---|---|---|
| **连接** | `initialize` 交换 capabilities / protocol version / client-server info | go-sdk 1.1+ 的 `2026-07-28` 协议**没有专门的 initialize/initialized 握手**——`session` 在客户端发第一个请求时**隐式**建立。`TestRun_EmptyStdin` 验证:无 initialize → SDK 直接退出 |
| **通信** | `tools/list` 列工具 / `tools/call` 调工具 / `notifications/...` 服务端推送 | `Run` 启动后阻塞在 `server.Run(ctx, transport)`,接 JSON-RPC 帧 |
| **关闭** | 客户端关 stdin → server `ctx.Done()` → 退出 | **当前没用 `signal.NotifyContext`**(G-1 backlog) |

### 3.2 stdio 的关键约束

- **stdin 只读一次,stdout 只写一次**——协议层 = 单工管道
- **stderr 可被 server 用作日志**——`serve_mcp.go:78-80` 把 `log.SetOutput(errOut)` 正是这个用途
- **每次 client 连接 = 新进程**——**没有守护进程**,没有"reuse connection"
  - 原因(架构 §1.0):MCP 寿命 = stdio 父进程寿命;AI 工具 = 客户端,启 `kron serve-mcp` 子进程
  - 副作用:**协议层不能做"持久状态"**——每次 spawn 都是新 session;Kron 把状态全部放 `.kron/` 文件(架构 §〇 铁律 #6)

### 3.3 `kron serve-mcp` 入口到 SDK 的对应

```go
// cmd/kron/serve-mcp/serve_mcp.go
server := mcp.NewServer(&mcp.Implementation{Name: "kron", Version: "v1.2.0"}, nil)
server.AddReceivingMiddleware(callerInjectMiddleware(root))  // 1
registerTools(server)                                         // 2
server.Run(context.Background(), &mcp.StdioTransport{})      // 3
```

| 行 | 协议层做了什么 |
|---|---|
| `mcp.NewServer` | 创建 server 实例;`Implementation` 出现在 initialize 响应里 |
| `AddReceivingMiddleware` | 注入 ctx 装饰(铁律 #4 caller 透传)——`ctx context.Context` 进 handler 之前 SDK 跑这层 |
| `registerTools` | 12 个 `mcp.AddTool` 调用;SDK 据此推断 `tools/list` 返回的 schema(从 `In/Out` struct + `jsonschema:"..."` tag) |
| `server.Run(..., &mcp.StdioTransport{})` | 启动 stdio 读循环;读一行 JSON → 解析 method → dispatch → 写一行响应 |

---

## 四、Tool 协议(实际写 handler 时碰的字段)

### 4.1 Tool 的"声明"vs"实现"

| 概念 | 在 Kron 哪里 | 在 MCP 协议层 |
|---|---|---|
| 工具**名字** | `&mcp.Tool{Name: "kron_lint", ...}` | `tools/list` 返回的 `name` 字段 |
| 工具**描述** | `Description: "Run lint over ..."` | `tools/list` 返回的 `description` 字段;LLM 用来决策"该不该调" |
| 输入 **schema** | Go struct + `jsonschema:"..."` tag | `tools/list` 返回的 `inputSchema` 字段(JSON Schema 格式) |
| 输出 **schema** | Go struct(Out 位置) | `tools/list` 返回的 `outputSchema` 字段 |
| **handler** | `func(ctx, req, in) (result, out, err)` | `tools/call` 的 `params.name` 命中后被调用 |

### 4.2 handler 签名(go-sdk 1.x)

```go
// kron 风格(为了兼容 main.go 入口 + v1.1 闭包)
func HandleLint(ctx context.Context, _ *mcp.CallToolRequest, in LintInput) (
    *mcp.CallToolResult, LintOutput, error,
)
```

| 位置 | 类型 | 用途 |
|---|---|---|
| 1st `ctx` | `context.Context` | 取消 / 超时(铁律 #4);**caller 透传已废弃**——`internal/` 不再依赖 ctx 上的 caller key |
| 2nd `_` | `*mcp.CallToolRequest` | 几乎用不到(SDK 在 v1.0 没暴露 typed ClientInfo);`clientNameFromReq` 当前**永远返回 ""** |
| 3rd `in` | `LintInput` | SDK 据 `arguments` 字段反序列化;**结构体必须 exported 字段** |
| 返回 1 | `*mcp.CallToolResult` | 通常 `nil`;SDK 自动包成 `{content: [...], structuredContent: {...}}` |
| 返回 2 | `LintOutput` | 用户**业务数据**;SDK 序列化为 `result.structuredContent` |
| 返回 3 | `error` | 非 nil = `IsError: true`;error 文本进 `result.content[0].text` |

### 4.3 `result` vs `error`:MCP 的特殊性

**关键差异**(后端开发者最容易踩的坑):
- **HTTP**:5xx 是 server 责任;4xx 是 client 错
- **MCP**:`result.IsError = true` 表示**"tool 自己跑挂了"**,但 HTTP 状态码**仍然是 200**——`error` 字段**不存在**
- **真正协议层错误**(JSON-RPC 错):填 `error` 字段,如方法不存在、参数解析失败

Kron 的 `kron_lint` 跑出来有 lint error 时:**仍返回 200 + `result.passed: false`**——不是 HTTP 5xx。

### 4.4 `structuredContent` vs `content`

Kron 工具返回的 `Out` struct 被 SDK 自动包成:
```json
{
  "result": {
    "content": [{"type":"text","text":"<JSON 字符串>"}],   // 旧 / 通用通道
    "structuredContent": { ...Out struct... }                // 新 / 类型安全通道
  }
}
```

`decodeSingleResponse`(`serve_mcp_test.go:39-65`)优先读 `structuredContent`,fallback 到 `content[0].text`——这是 SDK 版本兼容的**两边都要看**模式。

---

## 五、Capabilities 与协议版本

### 5.1 Server-side capabilities(server **声明**它有什么)

```go
server := mcp.NewServer(impl, &mcp.ServerOptions{
    Capabilities: &mcp.ServerCapabilities{
        Tools:   &mcp.ToolCapabilities{ListChanged: false},
        Logging: &mcp.LoggingCapabilities{},
    },
})
```

| 字段 | 含义 |
|---|---|
| `Tools` | server 注册了 tool;`ListChanged` 表示"运行中会动态增删" |
| `Prompts` | server 提供 prompt 模板 |
| `Resources` | server 提供可订阅的 URI 资源(Kron **没有**用到) |
| `Logging` | server 支持接收 `logging/setLevel` 调整日志级别 |

**Kron 现状**:`serve_mcp.go:84-86` 注释说"capabilities are auto-inferred from the registered tools"——SDK 默认行为。**Resources / Prompts / Logging 完全没声明**——对应 MCP 协议层 Kron 只暴露 `tools`。

### 5.2 Protocol version 协商

go-sdk 1.x 默认 `2026-07-28` 协议版本。`ServerOptions.SupportedProtocolVersions` 可以**收窄**支持的范围(例如"只支持 2025-11-25"用于"我们还没升级")——**只能收窄,不能放宽**。Kron 没显式设,用 SDK 默认。

### 5.3 capabilities 协商的实际流程

```
client                          server
  │                               │
  │  initialize (含 client caps)  │
  │ ───────────────────────────► │
  │                               │  ← server 据自己 capabilities + client 期望
  │  initialize response          │     计算出"最终能力集"
  │ ◄─────────────────────────── │
  │                               │
  │  ... (后续请求只能用到       │
  │       "最终能力集"里的       │
  │       feature) ...            │
```

**Kron 的简化**:因为 v1.2+ Kron 不实现 `Resources` / `Prompts` / `Logging`,协商是"告知 server 我**只**要 tools"→ server 直接用 `tools` 能力响应。

---

## 六、Transport

| Transport | 用途 | Kron 用法 |
|---|---|---|
| `StdioTransport` | 真实 stdio(JSON-RPC 帧通过 os.Stdin / os.Stdout) | 生产:`serve_mcp.go:131-135` 选这个 |
| `IOTransport` | 测试用(把 `bytes.Buffer` / `strings.NewReader` 当 stdin) | 测试:`stdioOrIOTransport` L140-148 走这个 |
| `InMemoryTransport` | 同进程 client-server 通信,不走 stdio | **未用**——`TestRun_StdioProtocolSmoke` 注释提到"deferred to commit 3" |

`stdioOrIOTransport`(`serve_mcp.go:130-149`)的检测逻辑:
- 如果 `in` 是 `*os.File` 且 fd=0(stdin)、`out` fd=1(stdout)→ `StdioTransport`
- 否则包成 `IOTransport` + nop Closer(测试场景)

**坑点**:Go 的 `*os.File.Fd()` 在跨平台/重定向场景下行为不一致;这个判定的**副作用**是:用 `cmd := exec.Command("kron", "serve-mcp")` 时,如果子进程 stdin 被显式 redirect,`StdioTransport` 不一定选上。当前没有 test 覆盖。

---

## 七、Middleware 与 Handler ctx

`server.AddReceivingMiddleware(func)`——SDK 在 dispatch 任何方法前跑这层,可以包装 `ctx`、改 req、拦截结果。

Kron 的 `callerInjectMiddleware`(`serve_mcp.go:180-195`):
```go
return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
    ctx = withRepoRoot(ctx, root)            // 注入 root 给 handler
    ctx = withMCPCaller(ctx, clientNameFromReq(req))  // 注入 caller(目前永远 "mcp")
    return next(ctx, method, req)
}
```

**设计意图**:
- `root` = 仓库根——handler 通过 `repoRootFrom(ctx)` 拿到,避免每个 handler 闭包捕获
- `caller` = 调用方标识——铁律 #4 要求;`internal/` 已经**不再**依赖它做行为分支(架构 §2.3)

**未来扩展面**:
- 加 timeout 中间件:handler 默认无超时,可加 `context.WithTimeout` 中间件
- 加 metrics 中间件:每个 method 调用计数 / 时长
- 加 audit log 中间件:写调用记录到 stderr

---

## 八、关键约束(Kron 视角)

1. **不要把状态放内存**——stdio 进程寿命不可控;**所有状态在 `.kron/`**(架构 §〇 铁律 #6)
2. **不要在 handler 里做长操作**——`server.Run` 是阻塞的;长操作要支持 ctx cancel
3. **不要在 stdout 写非 JSON-RPC 帧**——SDK 期望一帧一行;混进日志就破坏协议;**Kron 用 stderr**(`log.SetOutput(errOut)` L78-80)
4. **不要假设 client 提供 `ClientInfo`**——go-sdk 1.0 的 `mcp.Request` 没暴露 typed accessor;`clientNameFromReq` 永远返回 `""`(G-3 backlog)
5. **不要写"半套" capabilities**——例如声明 `Resources` 但不实现 → 客户端期望读 resources,实际 panic

---

## 九、和 web 后端开发的对应表

| 概念 | HTTP API | MCP 协议 |
|---|---|---|
| 入口 URL | `POST /api/v1/lint` | `tools/call` 的 `params.name = "kron_lint"` |
| 端点声明 | `router.POST("/lint", handler)` | `mcp.AddTool(server, &Tool{Name: "kron_lint"}, handler)` |
| 请求体 | `request.body` | `params.arguments`(JSON) |
| 响应体 | `response.body` | `result.structuredContent` + `result.content[]` |
| 错误响应 | HTTP 4xx/5xx | `error.code` / `result.IsError: true` |
| OpenAPI schema | `components.schemas` | `tools/list` 返回的 `inputSchema` / `outputSchema` |
| 鉴权头 | `Authorization: Bearer ...` | **MCP 不内置**——Kron 通过 ctx caller key 做软标记(非 authz) |
| 中间件 | Express middleware / gin handler | `server.AddReceivingMiddleware` |
| 进程模型 | 常驻 HTTP server | 每客户端一个 stdio 子进程 |
| Schema 推断 | 手写 OpenAPI / 用 zod | Go struct + `jsonschema:"..."` tag SDK 自动生成 |

---

## 十、动手前要读的几样东西

1. **[go-sdk `design/design.md`](https://github.com/modelcontextprotocol/go-sdk/blob/main/design/design.md)** —— 协议 + SDK 设计的官方源
2. **[go-sdk `docs/server.md`](https://github.com/modelcontextprotocol/go-sdk/blob/main/docs/server.md)** —— server API 详情
3. **[go-sdk `docs/protocol.md`](https://github.com/modelcontextprotocol/go-sdk/blob/main/docs/protocol.md)** —— 协议版本演进
4. **Kron 自己的 [`kron-implementation.md`](./kron-implementation.md)** —— 把代码和协议对上

---

## 十一、未在本文件展开的议题(查 `backlog.md` / `kron-implementation.md`)

- G-1: `server.Run` 用 `context.Background()` 而非 `signal.NotifyContext`
- G-2: 无真 e2e stdio 测试(`TestRun_StdioProtocolSmoke` 双重 skip)
- G-3: caller client name 提取(SDK 限制)
- G-4: capabilities 未显式声明
- G-5: 假设 registry 写路径未实装(`internal/assumption` 只读)
- G-6: `relations.ReferencingIntents` 未被任何 MCP 工具调用
- G-7: `kron_impact` / `kron_intent_density` 重复全量 `LoadAll`(无缓存)
