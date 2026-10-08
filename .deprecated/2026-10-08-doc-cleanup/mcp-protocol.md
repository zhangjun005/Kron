# MCP 协议层实现状态

> 本文档记录 MCP 访问层的协议层实现状态,不是架构真理。
> 对应 `cmd/kron/serve-mcp/`。

---

## 1 当前状态:工具层完整,协议层不完整

`cmd/kron/serve-mcp/` 实现了 **12 个业务工具函数**(`kron_*`),这是 Kron 的核心价值。

但**协议层(手shake + 能力协商)未实现**,这意味着标准 MCP Host 无法接入。

---

## 2 缺失的协议层功能

### 2.1 `initialize` 手shake ❌ critical

**MCP spec**:所有 MCP 服务器必须处理 `initialize` 请求,返回服务器能力声明。

**现状**:发 `initialize` → `-32601 method not found`。

**后果**:Claude Desktop / Cursor / MCP Inspector 启动时第一条请求就是 `initialize`,
直接导致 session 无法建立。

**最低实现**(MCP spec §Protocol Overview):
```go
// handleInitialize responds to the JSON-RPC handshake.
// Per the MCP spec the server MUST:
//   1. Return protocolVersion, capabilities, serverInfo in the result.
//   2. Remember the client's protocol version for the session.
//   3. Client then sends notifications/initialized — no response needed.
```

**触发**:2026-10-04 核查发现,见本会话记录。

### 2.2 `tools/list` 工具发现 ❌ critical

**MCP spec**:支持工具的服务器必须声明 `tools` 能力并响应 `tools/list`。

**现状**:没有 `tools/list` handler,客户端无法标准发现 12 工具。

**后果**:MCP Inspector / 规范 MCP Host 无法枚举 Kron 工具集。

**最低实现**:返回 12 个工具的 JSON schema(名称 + 描述 + 输入 schema)。

### 2.3 工具注册表非确定性 ⚠ should

**现状**:`defaultToolRegistry` 用 Go map 注册,map 遍历在 Go 1.27 中**非**确定性。

**后果**:连续两次 `kron serve-mcp` 启动,tools/list(如果实现)可能返回不同顺序,
影响 LLM 提示缓存命中率。

**缓解**:MCP 规范只要求"底层工具集不变时排序一致"。用 `sort.Slice` 按工具名排序即可。

### 2.4 `ping` / `shutdown` ❌ optional

MCP spec 建议支持,非必须。`ping` 用于保活;`shutdown` 用于优雅退出。

---

## 3 架构边界说明

以下代码**故意**留在 `cmd/kron/serve-mcp/` 而**不**下沉到 `internal/`:

| 代码 | 理由 |
|---|---|
| `rpc.go` 协议层(手shake / 错误码映射) | MCP 独有,CLI 不需要 |
| `tools.go` 工具注册表 | CLI 用 cobra flag,MCP 用 registry |
| `serve_mcp.go` 入口 | stdio 进程模型,CLI 用 `flag` |
| 12 个 handler | 各自定义 wire struct(JSON schema 在 docs),不在 internal |

**下沉条件**(architecture.md §〇·五·3):≥2 个访问层共用才下沉。
当前只有 MCP 需要协议层 → 不下沉。

---

## 4 实施计划

### Step 1 — `initialize` handler + `tools/list` handler (P0)

优先级 P0,因为是阻塞标准客户端接入的唯一障碍。

改动文件:`cmd/kron/serve-mcp/rpc.go`

工作量估算:~70 行代码。

验收标准:
- MCP Inspector 连接 `kron serve-mcp` 能建立 session
- `tools/list` 返回 12 个工具的 JSON schema
- `initialize` 返回 `capabilities: {tools: {}}`

### Step 2 — 工具注册表排序稳定化 (P1)

改动文件:`cmd/kron/serve-mcp/tools.go`

工作量估算:~5 行代码。

验收标准:连续 10 次 serve-mcp 启动,`tools/list` 输出顺序相同。

### Step 3 — 文档同步 (与 Step 1 同 PR)

按 `docs/process/mcp-tool.md` §2:
- `docs/implementation/mcp.md` §4 协议与部署 加 `initialize` / `tools/list` 说明
- `docs/abstractDesign/architecture.md` §1.2 MCP 工具集表格 注:"含标准 MCP 手shake"

---

## 5 参考

- MCP spec: https://modelcontextprotocol.io/specification
- MCP JSON-RPC 2.0: https://www.jsonrpc.org/specification
- 核查时间:2026-10-04
