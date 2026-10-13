# MCP 已知 GAP 与未来工作清单(2026-10-11 盘点)

> **目的**:把"底层 `internal/` 改了但 MCP 这一层没收口"的 GAP 和其他候选优化方向**记录下来**,不立即动手。
>
> **来源**:2026-10-11 阅读 `cmd/kron/serve-mcp/` 全部 20 个 `.go` 文件 + `internal/` 对应包 + go-sdk 1.x 文档后整理。
>
> **范围**:只列"协议 / 适配层"问题;业务逻辑问题(如 `kron_impact` 的 `explicit wins` 规则是否合理)由 `docs/process/lint-rule.md` 等流程管,不在此列。

---

## GAP 总览

| 编号 | 类别 | 一句话 | 风险等级 | 估时 |
|---|---|---|---|---|
| G-1 | 协议标准 | `server.Run` 用 `context.Background()` 而非 `signal.NotifyContext`,stdio 进程不响应 Ctrl+C | ~~中~~ — **已完成 (2026-10-13)** ✅(`runWithSignalCtx` + `signal.NotifyContext` 在 `serve_mcp.go:188`) | — |
| G-2 | 测试 | 无真 e2e stdio 测试,`TestRun_StdioProtocolSmoke` 双重 skip | ~~中~~ — **已完成 (2026-10-13)** ✅(`cmd/kron/serve-mcp/e2e/e2e_stdio_test.go` + `serve_mcp_test.go` KRON_E2E gate) | — |
| G-3 | 协议标准 | caller client name 提取未实现(SDK 1.0 无 typed accessor) | 低 | 等 SDK |
| G-4 | 协议标准 | capabilities 未显式声明(Resources / Prompts / Logging 都缺) | ~~低~~ — **已完成 (2026-10-13)** ✅ | — |
| G-5 | **适配层漂移** | `internal/assumption` 只读,`kron_add` / `kron_update` 不写 `.kron/assumptions/<id>.md` | ~~高~~ — **已完成 (2026-10-13)** ✅ | — |
| G-6 | **适配层漂移** | `internal/relations.ReferencingIntents` 无人调用(无 `kron_assumption_*` 工具) | ~~高~~ — **已完成 (2026-10-13)** ✅ (随 G-5 一起) | — |
| G-7 | 性能 | `kron_impact` / `kron_intent_density` 重复全量 `LoadAll`,无缓存 | ~~低(规模敏感)~~ — **已完成 (2026-10-13)** ✅ | — |

---

## 详述

### G-1 `server.Run` 用 `context.Background()` 而非 `signal.NotifyContext`

**现象**(`cmd/kron/serve-mcp/serve_mcp.go:117`):
```go
if err := server.Run(context.Background(), transport); err != nil { ... }
```

**问题**:
- stdio 进程跑起来后,**Ctrl+C 不会让进程退出**——只能等客户端关 stdin
- LSP / 长连接语境 MCP 接受这种行为;但 stdio 进程管理上不优雅
- 紧急情况下(开发调试、AI 工具挂死)需要手动 kill 父进程

**修法**(伪代码,不动手):
```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()
return server.Run(ctx, transport)
```

**为什么不在 2026-10-11 这次做**:
- 不在用户当前的"学习 MCP"目标范围
- 需要决定:window 模式 / Ctrl+Break / SIGTERM 哪个;**OS 行为差异**(Windows 不响应 SIGTERM)
- 关联流程:`docs/process/new-internal-api.md`(ctx 改动需要走流程)

**待办触发条件**:用户决定动手;或第一次出现"stdio 进程无法退出"的事故

---

### G-2 无真 e2e stdio 测试(`TestRun_StdioProtocolSmoke` 双重 skip)

**现象**(`cmd/kron/serve-mcp/serve_mcp_test.go:80-93`):
```go
func TestRun_StdioProtocolSmoke(t *testing.T) {
    if os_getenv("KRON_E2E") == "" {
        t.Skip("KRON_E2E not set; ...")
    }
    t.Skip("subprocess plumbing deferred to commit 3 (testkit layer covers in-memory e2e)")
}
```

**问题**:
- 所有协议层行为(stdin/stdout framing、stdio vs IOTransport 判定、real binary spawn)都是 in-memory 测的
- 真实 stdio 二进制 spawn → JSON-RPC → shutdown 这条路径**没跑过**
- `stdioOrIOTransport` 的 `Fd()` 判定**无真实环境覆盖**——Windows / pipe / redirect 都没测

**修法**:
1. 拆掉 `TestRun_StdioProtocolSmoke` 的双重 skip
2. 写一个 helper:`buildBinary(t) → path`(用 `go build -o tmp kron`)
3. 真 spawn:`exec.Command(path, "serve-mcp", "-CWD", tempDir)`
4. 喂 `initialize` + `tools/list` + `kron_lint` 三帧,断言响应
5. 关 stdin,断言进程退出码 = 0
6. 放到 CI 的 `Kron_E2E=1` task 跑

**为什么不在 2026-10-11 这次做**:
- 写 e2e 是独立工作;**学协议期间不需要**
- subprocess plumbing 涉及 build 产物管理 + CI 配合,需要先讨论

**待办触发条件**:用户决定补 e2e;或第一次出现"in-memory 测过但 stdio 跑挂"的事故

---

### G-3 caller client name 提取未实现

**现象**(`cmd/kron/serve-mcp/serve_mcp.go:230-250`):
```go
func clientNameFromReq(req mcp.Request) string {
    if req == nil { return "" }
    // We intentionally do not call req.GetSession() here: the returned
    // Session is consumed by SDK internals and is not safe to inspect
    // on the receiving path. client attribution is parked on the
    // v1.2.1 milestone where the SDK exposes a stable typed accessor.
    return ""
}
```

**问题**:
- caller 永远 = `"mcp"`(never `"mcp:claude-3.7"` / `"mcp:cursor-1.0"` / ...)
- `internal/` 已经**不依赖** caller 做行为分支(架构 §2.3),所以**功能上没坏**
- 但**审计 / 调试**价值缺失:看 stderr 日志区分不出"是哪个 AI 工具调的"

**修法**:
- 升级 go-sdk 到支持 typed `Session.ClientInfo()` 的版本
- 用 typed accessor 替换 `clientNameFromReq`
- 跑现有测试;**不需要**改协议层

**为什么不在 2026-10-11 这次做**:
- **新 Go dep 走 explicit approval**(AGENTS.md §3 红线;`go.lsp.dev/protocol` 也是同类)
- 升级 SDK 涉及 godoc breaking changes 风险
- 价值不紧急——`internal/` 不依赖

**待办触发条件**:
- 用户决定升级 SDK(可能为其他 feature 一起升)
- 或第一次出现"需要按 client 区分诊断"的实际需求

---

### G-4 capabilities 未显式声明 — **✅ 已完成 (2026-10-13)**

**现象**（`cmd/kron/serve-mcp/serve_mcp.go:84-86`，旧版）：
```go
// Create the SDK server. Capabilities are auto-inferred from
// the registered tools (we register 12 below), so we do not
// pass an explicit Capabilities struct.
server := mcp.NewServer(&mcp.Implementation{...}, nil)
```

**当时的问题**：
- SDK 推断 = 只声明 `tools` capability
- 没声明 `Resources` / `Prompts` / `Logging`——协议层 Kron **不支持**这些
- **未来加 Resources**（例：`kron://intents/<slug>`）时容易踩坑：不显式声明 = 客户端期望读但 server 没实现

**2026-10-13 落地**：

调查发现 go-sdk v1.1.0 `ServerOptions` **没有** `Capabilities` 字段（无法直接设置）；`initCaps` 行为：

```go
// SDK protocol.go initCaps
caps := &ServerCapabilities{
    Logging: &LoggingCapabilities{},  // 无条件，始终
}
if tools registered: caps.Tools = &ToolCapabilities{ListChanged: true}
if prompts registered: caps.Prompts = &PromptCapabilities{...}
if resources registered: caps.Resources = &ResourceCapabilities{...}
```

因此：
- `Tools`：由 `registerTools()` 自动推断 ✅
- `Resources` / `Prompts`：不注册 → SDK 推断 nil ✅
- `Logging`：SDK 无条件设为 `{}` → **无法关闭**，但这是 SDK 行为，`logging/setLevel` 调用会返回 method-not-found（符合 MCP spec）

**落地内容**（`serve_mcp.go` 更新了注释）：

```go
// SDK note: go-sdk v1.1.0 does not expose a Capabilities field
// in ServerOptions; the only control points are HasResources /
// HasPrompts / HasTools. We keep them all at zero (the default)
// to preserve "Tools auto-on, Resources / Prompts off" semantics.
server := mcp.NewServer(&mcp.Implementation{...}, nil)
```

**测试**（`capabilities_test.go`）验证 initialize 握手返回的 wire JSON：
- `prompts` / `resources` **必须不存在**（我们不注册）
- `tools` **必须存在**，且 `listChanged = true`（SDK 自动推断）
- `logging` **存在**（SDK 无条件 `{}`），`logging/setLevel` 调用会得到 method-not-found（可接受）

**结论**：G-4 在 SDK v1.1.0 下无法通过 API 强制，但**文档化了现状** + 加了测试防止 regression。`docs/mcp/missing-capabilities.md` §G-4 有完整说明。

---

### G-5 ⚠️ `internal/assumption` 只读(MCP 没写路径) — **✅ 已完成 (2026-10-13)**

**当时的现象**(grep 结果,2026-10-11):
- `cmd/kron/serve-mcp/` 内 4 处 `assumption.NewReader`(用)
- `cmd/kron/serve-mcp/` 内 **0 处** `assumption.NewWriter` 或 `Writer.Create/Update`
- `internal/assumption/writer.go` 已有实装,服务 5 个 method

**当时的问题**:
- B-3 拍板意图:`.kron/assumptions/<id>.md` 是 single source of truth
- 现实:没人写它,`kron_add` 写新 intent 时如果 `assumptions[]` 引用 `single-region`,**只**写 intent 文件,**不**写 `single-region.md`
- 下次 `kron_get` 走 `assumesToWire` → `ar.Get(ctx, "single-region")` → `ErrNotFound` → 兜底 inline `text` → **`default_severity` 字段空着**
- 等于 RFC 拍板"single source of truth",实际"全是 inline copy"

**修法**(候选):
1. `kron_add` / `kron_update` 写 intent 时,检查其 `assumptions[]` ID,自动 create/update 对应 `.kron/assumptions/<id>.md`
2. 新增 `kron_assumption_create` / `kron_assumption_update` / `kron_assumption_delete` 三个工具
3. `kron_assumption_delete` 走 G-6 的 `ReferencingIntents` 检查 dependents

**当时依赖**:
- `internal/assumption/writer.go` 已就绪——直接调
- 需要走 `docs/process/mcp-tool.md` 流程(新工具)
- 需要走 `docs/process/new-internal-api.md` 流程(`internal/assumption` 是 v1.3 新包)

**当时风险**:
- 与 RFC [`2026-10-08-assumptions-standalone.md`](../.deprecated/2026-10-09-process-cleanup/) 拍板本意一致——**没违反约束**
- 但落地时**会**有"什么时候写" / "delete 怎么处理引用了它的 intent" 等问题——需要人决策

**当时优先级**:**最高**——这是数据流断点,长期会"假装有 registry"

**2026-10-13 落地**(本次会话,G-5 + G-6 一次 PR):
- 新增 4 个 MCP 工具:`kron_assumption_create` / `kron_assumption_update` / `kron_assumption_delete` / `kron_assumption_get`
- 走 `internal/assumption.NewWriter` 5 个 method,补全写路径
- `kron_delete` 路径上同时扫 `.kron/assumptions/<id>.md` 软警告
- `assumesToWire` 的 inline `text` 兜底**保留**(RFC 2026-10-12 §2.2 拍板后才删,与本 backlog 解耦)
- 4 个旧 fixture 移除 inline `text:`(同步 lint 升 Error 后的清理)

**关联**:
- G-6(`ReferencingIntents`)随 G-5 一起接入,见下方 G-6 节
- RFC [`2026-10-12-close-b3-softening.md`](../rfc/2026-10-12-close-b3-softening.md) 拍板的"inline text → 硬 Error"是**下一份** PR(本 PR 不动 parser/lint 严度,只补写路径)

---

### G-6 ⚠️ `internal/relations.ReferencingIntents` 无人调用 — **✅ 已完成 (2026-10-13,随 G-5)**

**当时的现象**:
- `internal/relations/relations.go:135-167` 显式存在,doc 说"will be needed by `kron_assumption_delete`"
- `cmd/kron/serve-mcp/` 0 处 `ReferencingIntents` 调用
- 0 个 `kron_assumption_*` 工具

**当时的问题**:
- 删除一个 `single-region.md` 时,引用了 `single-region` 的 intent **不感知**——`kron_lint` 报 `dangling-depends-on` 错误,但**删除工具不预警告**
- `kron_delete` 处理 intent 删除时**有** `dependents` 软警告——但 assumption 删除**没有**对应路径

**当时的修法候选**:
- 新增 `kron_assumption_delete`(走 G-5 流程)
- 删除前调 `ReferencingIntents(all, id)` 算 dependents → 软警告(类似 `kron_delete`)
- 配合 G-5 一起做(同一份 process doc 走)

**当时优先级**:**最高**——与 G-5 同源

**2026-10-13 落地**:
- `kron_assumption_delete` handler 调 `relations.ReferencingIntents` 算引用该 ID 的 intent 列表
- 走 `kron_delete` 同款"软警告"路径(返回 dependants 计数,force=true 才真删)
- 同一份 PR,共用 process 流程

**后续**:无独立 follow-up。ReferencingIntents 现在被 `kron_assumption_delete` + `kron_delete` 两路共用,作为"删除前预检"基础设施稳态存在。

---

### G-7 `kron_impact` / `kron_intent_density` 重复全量 `LoadAll` — **✅ 已完成 (2026-10-13)**

**当时的现象**:
- `handlers_impact.go:68`: `all, err := r.LoadAll(ctx)`
- `handlers_density.go:48`: `all, err := r.LoadAll(ctx)`
- `handlers_tree.go:48`: `all, err := r.LoadAll(ctx)`
- `handlers_assume_check.go:70`: `all, err := r.LoadAll(ctx)`
- `handlers_stale.go:52`: `all, err := r.LoadAll(ctx)`
- `handlers_delete.go:54`: `all, err := r.LoadAll(ctx)` (move 前算 dependents)

**当时的问题**:
- 单次 MCP 调用 `LoadAll` 1 次——OK
- AI 工具一次发 5 个 `kron_*` 工具调用 → 5 次 `LoadAll`
- O(repo size) 的 I/O + YAML 解析 × 5 = 大仓库下明显慢

**2026-10-13 落地**:

新增 `cmd/kron/serve-mcp/cache.go`：

- `LoadAllCached(ctx, root)` —— singleflight + mtime invalidation
- 9 个 handler 全部改用 `LoadAllCached` 替换手写 `store.NewReader → LoadAll`
- 写操作后（kron_add / kron_update / kron_delete / kron_restore）显式 `InvalidateCache`
- 不下沉到 `internal/store`（铁律 #5）

**实测效果**（`TestCacheEffect`，500 intents）：

| 工具 | 冷 | 热 | 加速比 |
|---|---|---|---|
| `kron_stale` | 59ms | 0.17ms | **346x** |
| `kron_impact` | 58ms | 0.35ms | **166x** |
| `kron_intent_density` | 57ms | 1ms | **56x** |
| `kron_list` | 65ms | 4ms | **15x** |
| `kron_lint` | 89ms | 81ms | **1.1x**（scan 主导，无缓存收益）|

**关联**：OPT-2（anchor scan 共享）待评估，见 [`docs/implementation/bench-2026-10-09.md`](../../docs/implementation/bench-2026-10-09.md) §5。

---

## 候选优化方向(未列入 GAP 的)

| 编号 | 描述 | 来源 |
|---|---|---|
| C-1 | 加 `Resources`(URI 资源)支持:`kron://intents/<slug>` 可读 | 协议层 §一.0 |
| C-2 | 加 `Prompts` 模板:`kron-review` / `kron-audit` 给 LLM 直接调 | 协议层 §一.0 |
| C-3 | ~~加 `Logging` capability + `logging/setLevel` handler(替代 stderr hack)~~ — **✅ 已完成 (2026-10-13)** | 协议层 |
| C-4 | ~~加 per-Run cache~~ — 见 G-7（✅ 已完成） | 性能 |
| C-5 | 在 `kron_lint` 错误输出里加"fix suggestion" hint | DX |
| C-6 | ~~加 `kron_impact` 的 `incoming_anchors` 维度拆分:code / markdown 分开列~~ — **✅ 已完成 (2026-10-13)** | DX |
| C-7 | ~~在 `kron_get` 返回的 `relations` 里加 `dependents`~~ — **✅ 已完成 (2026-10-13)** | 一致性 |

---

## 关联流程文档

做任何 GAP 之前,先读对应流程:

| GAP | 流程 |
|---|---|
| G-1(改 Run 入口) | [`docs/process/new-internal-api.md`](../process/new-internal-api.md) |
| G-2(加 e2e) | [`docs/process/ci-enforcement.md`](../process/ci-enforcement.md) |
| G-3(升级 SDK) | [`AGENTS.md` §3](../AGENTS.md) "加新 top-level dep" 红线 |
| G-4(改 ServerOptions) | [`docs/process/new-internal-api.md`](../process/new-internal-api.md) |
| G-5(assumption 写) | [`docs/process/mcp-tool.md`](../process/mcp-tool.md) + [`docs/process/new-internal-api.md`](../process/new-internal-api.md) |
| G-6(assumption delete) | 同 G-5 |
| G-7(加 cache) | [`docs/process/new-internal-api.md`](../process/new-internal-api.md) |

---

## 触发 PR 的建议顺序

如果决定**动手**做一部分(顺序按"价值 / 成本比"):

1. ~~**G-5 + G-6 一起做**~~(同流程,1 PR ≈ 1.5 d)— **✅ 2026-10-13 完成**
2. ~~**G-7** 看 benchmark 再定(单独 PR)~~ — **✅ 2026-10-13 完成（OPT-1）**
3. ~~**G-4**~~ — **✅ 2026-10-13 完成（SDK 行为文档化 + 测试）**
4. ~~**C-7**~~ — **✅ 2026-10-13 完成（kron_get relations.Dependents）**
5. ~~**C-6**~~ — **✅ 2026-10-13 完成（kron_impact 拆 code/markdown）**
6. ~~**C-3**~~ — **✅ 2026-10-13 完成（shimHandler + mcp.NewLoggingHandler）**
7. ~~**G-1**~~ — **✅ 已实装**(`runWithSignalCtx` 在 `serve_mcp.go:188`,2026-10-13 补登)
8. ~~**G-2**~~ — **✅ 已实装**(`cmd/kron/serve-mcp/e2e/e2e_stdio_test.go`,2026-10-13 补登)
9. **G-3** 与 SDK 升级一起做(单独 PR)
10. **C-1** Resources（先验证 client 支持情况）
11. **Roots** （新发现；先开 RFC 拍板范围）
12. **C-5** kron_lint fix suggestion hint

不催——按需做,本目录**只**是记录。
