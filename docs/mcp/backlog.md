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
| G-1 | 协议标准 | `server.Run` 用 `context.Background()` 而非 `signal.NotifyContext`,stdio 进程不响应 Ctrl+C | 中 | 30 min |
| G-2 | 测试 | 无真 e2e stdio 测试,`TestRun_StdioProtocolSmoke` 双重 skip | 中 | 2 h |
| G-3 | 协议标准 | caller client name 提取未实现(SDK 1.0 无 typed accessor) | 低 | 等 SDK |
| G-4 | 协议标准 | capabilities 未显式声明(Resources / Prompts / Logging 都缺) | 低 | 1 h |
| G-5 | **适配层漂移** | `internal/assumption` 只读,`kron_add` / `kron_update` 不写 `.kron/assumptions/<id>.md` | **高** | 1 d |
| G-6 | **适配层漂移** | `internal/relations.ReferencingIntents` 无人调用(无 `kron_assumption_*` 工具) | **高** | 1 d |
| G-7 | 性能 | `kron_impact` / `kron_intent_density` 重复全量 `LoadAll`,无缓存 | 低(规模敏感) | 0.5 d |

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

### G-4 capabilities 未显式声明

**现象**(`cmd/kron/serve-mcp/serve_mcp.go:84-86`):
```go
// Create the SDK server. Capabilities are auto-inferred from
// the registered tools (we register 12 below), so we do not
// pass an explicit Capabilities struct.
server := mcp.NewServer(&mcp.Implementation{...}, nil)
```

**问题**:
- SDK 推断 = 只声明 `tools` capability
- 没声明 `Resources` / `Prompts` / `Logging`——协议层 Kron **不支持**这些
- 当前**没坏**——客户端 initialize 不会要求这些
- **未来加 Resources**(例:`kron://intents/<slug>`)时容易踩坑:不显式声明 = 客户端期望读但 server 没实现

**修法**(伪代码):
```go
server := mcp.NewServer(impl, &mcp.ServerOptions{
    Capabilities: &mcp.ServerCapabilities{
        Tools:   &mcp.ToolCapabilities{ListChanged: false},
        // Resources: nil  // 显式 = 不提供
        // Prompts:   nil  // 显式 = 不提供
    },
})
```

**为什么不在 2026-10-11 这次做**:
- 行为等价(SDK 默认就是这结果)
- 等真有 Resources / Prompts 需求时**一起做**

**待办触发条件**:第一次出现"kron 应该提供 resource / prompt"的需求

---

### G-5 ⚠️ `internal/assumption` 只读(MCP 没写路径)

**现象**(grep 结果):
- `cmd/kron/serve-mcp/` 内 4 处 `assumption.NewReader`(用)
- `cmd/kron/serve-mcp/` 内 **0 处** `assumption.NewWriter` 或 `Writer.Create/Update`
- `internal/assumption/writer.go` 已有实装,服务 5 个 method

**问题**:
- B-3 拍板意图:`.kron/assumptions/<id>.md` 是 single source of truth
- 现实:没人写它,`kron_add` 写新 intent 时如果 `assumptions[]` 引用 `single-region`,**只**写 intent 文件,**不**写 `single-region.md`
- 下次 `kron_get` 走 `assumesToWire` → `ar.Get(ctx, "single-region")` → `ErrNotFound` → 兜底 inline `text` → **`default_severity` 字段空着**
- 等于 RFC 拍板"single source of truth",实际"全是 inline copy"

**修法**(候选):
1. `kron_add` / `kron_update` 写 intent 时,检查其 `assumptions[]` ID,自动 create/update 对应 `.kron/assumptions/<id>.md`
2. 新增 `kron_assumption_create` / `kron_assumption_update` / `kron_assumption_delete` 三个工具
3. `kron_assumption_delete` 走 G-6 的 `ReferencingIntents` 检查 dependents

**依赖**:
- `internal/assumption/writer.go` 已就绪——直接调
- 需要走 `docs/process/mcp-tool.md` 流程(新工具)
- 需要走 `docs/process/new-internal-api.md` 流程(`internal/assumption` 是 v1.3 新包)

**风险**:
- 与 RFC [`2026-10-08-assumptions-standalone.md`](../.deprecated/2026-10-09-process-cleanup/) 拍板本意一致——**没违反约束**
- 但落地时**会**有"什么时候写" / "delete 怎么处理引用了它的 intent" 等问题——需要人决策

**优先级**:**最高**——这是数据流断点,长期会"假装有 registry"

**待办触发条件**:用户决定补;或第一次出现"AI 查 assumption 拿到空 default_severity"的实际事故

---

### G-6 ⚠️ `internal/relations.ReferencingIntents` 无人调用

**现象**:
- `internal/relations/relations.go:135-167` 显式存在,doc 说"will be needed by `kron_assumption_delete`"
- `cmd/kron/serve-mcp/` 0 处 `ReferencingIntents` 调用
- 0 个 `kron_assumption_*` 工具

**问题**:
- 删除一个 `single-region.md` 时,引用了 `single-region` 的 intent **不感知**——`kron_lint` 报 `dangling-depends-on` 错误,但**删除工具不预警告**
- `kron_delete` 处理 intent 删除时**有** `dependents` 软警告——但 assumption 删除**没有**对应路径

**修法**:
- 新增 `kron_assumption_delete`(走 G-5 流程)
- 删除前调 `ReferencingIntents(all, id)` 算 dependents → 软警告(类似 `kron_delete`)
- 配合 G-5 一起做(同一份 process doc 走)

**优先级**:**最高**——与 G-5 同源

**待办触发条件**:同 G-5

---

### G-7 `kron_impact` / `kron_intent_density` 重复全量 `LoadAll`

**现象**:
- `handlers_impact.go:68`: `all, err := r.LoadAll(ctx)`
- `handlers_density.go:48`: `all, err := r.LoadAll(ctx)`
- `handlers_tree.go:48`: `all, err := r.LoadAll(ctx)`
- `handlers_assume_check.go:70`: `all, err := r.LoadAll(ctx)`
- `handlers_stale.go:52`: `all, err := r.LoadAll(ctx)`
- `handlers_delete.go:54`: `all, err := r.LoadAll(ctx)` (move 前算 dependents)

**问题**:
- 单次 MCP 调用 `LoadAll` 1 次——OK
- AI 工具一次发 5 个 `kron_*` 工具调用 → 5 次 `LoadAll`
- O(repo size) 的 I/O + YAML 解析 × 5 = 大仓库下明显慢
- 现行实现**每次都重新读盘**——没有缓存层

**修法**(候选):
1. **短期**:`store.Reader` 加 sync.Map 缓存,key = `filepath.Abs(root) + mtime`;命中 → 直接返
2. **中期**:per-Run 的 in-memory cache(`Run` 入口 `sync.Once` 初始化)——stdio 进程寿命内复用
3. **长期**:监听 `.kron/intents/` 变更失效(架构 §〇 铁律 #6 拒绝 daemon,需要"按需"——client 显式调 invalidate)

**风险**:
- 缓存 = 隐式状态——架构 §〇 铁律 #6 反对
- 但进程内 sync.Map **不**是"状态机",只是性能优化——可以**论证**不算违规
- 真正违规的会是"文件监听自动失效"——目前不需要

**为什么不立即做**:
- 仓库规模不大时(架构示例的 O(10) intents)性能问题不明显
- `bench_test.go` 已经测了——**先看 benchmark 再决定**

**待办触发条件**:
- 第一次出现"AI 工具调 5 个 tool 慢"的用户反馈
- 或仓库 `kron lint` 在 >100 intents 仓库下慢的反馈

---

## 候选优化方向(未列入 GAP 的)

| 编号 | 描述 | 来源 |
|---|---|---|
| C-1 | 加 `Resources`(URI 资源)支持:`kron://intents/<slug>` 可读 | 协议层 §一.0 |
| C-2 | 加 `Prompts` 模板:`kron-review` / `kron-audit` 给 LLM 直接调 | 协议层 §一.0 |
| C-3 | 加 `Logging` capability + `logging/setLevel` handler(替代 stderr hack) | 协议层 |
| C-4 | 加 per-Run cache(见 G-7) | 性能 |
| C-5 | 在 `kron_lint` 错误输出里加"fix suggestion" hint | DX |
| C-6 | 加 `kron_impact` 的 `incoming_anchors` 维度拆分:code / markdown 分开列(现在是混在一个数组) | DX |
| C-7 | 在 `kron_get` 返回的 `relations` 里加 `dependents` 反向(`kron_delete` 已经有这个能力) | 一致性 |

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

1. **G-5 + G-6 一起做**(同流程,1 PR ≈ 1.5 d)— 修数据流断点
2. **G-1**(0.5 d)— 进程管理基本功
3. **G-2**(1 d)— 协议层 coverage
4. **G-4**(0.5 d)— 防御性显式
5. **G-3** 与 SDK 升级一起做(单独 PR)
6. **G-7** 看 benchmark 再定(单独 PR)

不催——按需做,本目录**只**是记录。
