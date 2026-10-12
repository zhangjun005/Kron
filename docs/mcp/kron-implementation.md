# Kron MCP 实现侧:协议层切片

> **目的**:把 `cmd/kron/serve-mcp/` 现状按"协议层"切片讲清——读完能把"代码"和"协议"对上。
>
> **先读** [`protocol.md`](./protocol.md)。本文件假设你已建立"JSON-RPC envelope / stdio / capabilities / tool registration"的心智模型。
>
> **代码源**:`e:/works/Kron/cmd/kron/serve-mcp/`,行号以 2026-10-11 仓库版本为准。

---

## 一、文件清单 + 协议层角色

| 文件 | 行数 | 协议层角色 |
|---|---|---|
| `serve_mcp.go` | 250 | 入口:从 `Run(args, in, out, errOut)` 到 `server.Run` |
| `tools.go` | 31 | `assertCallerMCP` 防御性 guard |
| `tools_schema.go` | 354 | 12 个工具的 In/Out struct + `registerTools` 总入口 |
| `tools_lint.go` | 100 | `HandleLint`(`kron_lint`)实现 + 输出格式转换 |
| `tools_list_get.go` | 195 | `HandleList` / `HandleGet` + `assumesToWire` 共享 helper |
| `handlers_init.go` | 55 | `kron_init` |
| `handlers_add.go` | 125 | `kron_add` |
| `handlers_update.go` | 130 | `kron_update` + `validateFrontmatterShape` |
| `handlers_delete.go` | 75 | `kron_delete` |
| `handlers_restore.go` | 50 | `kron_restore` |
| `handlers_assume_check.go` | 135 | `kron_assume_check` |
| `handlers_impact.go` | 155 | `kron_impact` |
| `handlers_density.go` | 160 | `kron_intent_density` + `scanOrphanFiles` / `countLines` |
| `handlers_tree.go` | 85 | `kron_tree` + `treeNodeFromView` 投影 |
| `handlers_stale.go` | 80 | `kron_stale` |
| `serve_mcp_test.go` | 130 | stdio 协议层 smoke test |
| `handlers_test.go` | 33K | 工具级测试(在内存 client) |
| `handlers_relations_test.go` | 14K | relations 相关测试 |
| `bench_test.go` | 11K | 基准测试 |
| `testkit.go` + `testdata_helpers_test.go` | — | in-memory client 测试工具 |

**协议层切片**:
- **入口层**:`serve_mcp.go`(Run / stdioOrIOTransport / callerInjectMiddleware)
- **Tool 注册层**:`tools_schema.go`(registerTools)
- **Handler 实现层**:`handlers_*.go` + `tools_*.go`(12 个)
- **ctx 工具**:`tools.go`(assertCallerMCP)
- **测试层**:`testkit.go` / `serve_mcp_test.go` / `handlers_test.go` / `handlers_relations_test.go` / `bench_test.go`

---

## 二、入口层(`serve_mcp.go`)

### 2.1 `Run(args, in, out, errOut)`——`main.go` 调它

```go
func Run(args []string, in io.Reader, out, errOut io.Writer) error {
    root, err := resolveMCPRoot(args)        // 解析 -CWD <root>
    log.SetOutput(errOut)                    // 日志走 errOut(避免污染 stdout JSON-RPC)
    server := mcp.NewServer(&mcp.Implementation{Name: "kron", Version: "v1.2.0"}, nil)
    server.AddReceivingMiddleware(callerInjectMiddleware(root))
    registerTools(server)
    transport, _ := stdioOrIOTransport(in, out)
    return server.Run(context.Background(), transport)
}
```

**逐行协议层映射**:

| 行 | 协议层作用 |
|---|---|
| `resolveMCPRoot` | 拿仓库根——协议无关;**handler 后续通过 ctx 拿到**,而不是闭包 |
| `log.SetOutput(errOut)` | **关键**——stdio 模式下 stdout 是 JSON-RPC 帧专用;**任何调试日志必须 stderr**(G-1 相关) |
| `mcp.NewServer(impl, nil)` | 创建 server;`impl` 在 initialize 响应里出现;`nil` = 用 SDK 默认 capabilities |
| `AddReceivingMiddleware` | 注入 root / caller 到 ctx;**铁律 #4 落地** |
| `registerTools` | 12 次 `mcp.AddTool`——SDK 据 In/Out struct 推断 schema |
| `stdioOrIOTransport` | 选真实 stdio 还是测试用 IOTransport |
| `server.Run(ctx, transport)` | 阻塞读循环;**当前 ctx 是 `context.Background()`**——G-1 |

### 2.2 `resolveMCPRoot`——CLI flag 解析

```go
if len(args) == 0 { return os.Getwd() }
if len(args) == 2 && args[0] == "-CWD" { return filepath.Abs(args[1]) }
return "", fmt.Errorf("kron serve-mcp: unexpected arguments %v ...", args)
```

- **唯一 flag**:`-CWD <root>`(MCP 客户端启动 kron 子进程时,需要告诉它仓库在哪)
- **缺省**:当前工作目录
- **任何其他参数** → error(避免和 SDK / Kron 内部 flag 冲突)

### 2.3 `callerInjectMiddleware`——ctx 装饰

```go
return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
    ctx = withRepoRoot(ctx, root)
    ctx = withMCPCaller(ctx, clientNameFromReq(req))
    return next(ctx, method, req)
}
```

- `withRepoRoot`:把 `root` 塞 ctx;handler 调 `repoRootFrom(ctx)` 拿
- `withMCPCaller`:把 caller 塞 ctx;handler 调 `model.CallerFrom(ctx)` 拿
- `clientNameFromReq`:**当前永远返回 `""`**——`mcp.Request` 在 SDK 1.0 没有 typed ClientInfo accessor

**为什么用 ctx 而不是闭包**:
- v1.1 是 `handleXxx(root string)` 闭包;12 个 handler 各自闭包 root
- v1.2 收口:中间件注入,handler 签名干净;**符合铁律 #4**(caller 走 ctx)

### 2.4 `stdioOrIOTransport`——传输选择

```go
if f, ok := in.(*os.File); ok && f.Fd() == 0 {       // fd=0 = stdin
    if of, ok := out.(*os.File); ok && of.Fd() == 1 {  // fd=1 = stdout
        return &mcp.StdioTransport{}, nil
    }
}
return &mcp.IOTransport{Reader: r, Writer: w}, nil
```

- 真实 stdio → `StdioTransport`(SDK 直接读 os.Stdin / 写 os.Stdout,**零拷贝**)
- 否则 → `IOTransport`(用 `nopReadCloser` / `nopWriteCloser` 包;测试场景)

**`Fd()` 的副作用**:
- Windows:stdin/stdout fd 是 0/1;`exec.Command` 子进程时**不保证**继承同一 fd
- 当前 `TestRun_StdioProtocolSmoke` 双重 skip,这个判定**无真实 e2e 覆盖**(G-2)

---

## 三、Tool 注册层(`tools_schema.go`)

### 3.1 12 个工具的 In/Out struct

每个工具的输入输出都用 **Go struct + `jsonschema:"..."` tag** 定义。SDK 读 tag 自动生成 `tools/list` 返回的 schema——**手写 JSON-Schema 不需要**。

例(`InitInput`):
```go
type InitInput struct{}  // 空 = 无参数
type InitOutput struct {
    OK                 bool   `json:"ok"                       jsonschema:"true if .kron/ exists or was created"`
    IntentsDir         string `json:"intents_dir"               jsonschema:"path to the intents directory (e.g. .kron/intents)"`
    AlreadyInitialised bool   `json:"already_initialised"       jsonschema:"true if .kron/ was already present (idempotent re-init)"`
}
```

**协议层映射**:
- `json:"..."` tag → JSON 字段名
- `jsonschema:"..."` tag → 字段描述(LLM 用来决定"调不调、传什么")
- 字段类型 → JSON Schema type(Go `string` → `{"type": "string"}`;`[]string` → `{"type": "array", "items": {"type": "string"}}`)

### 3.2 `registerTools`——一次性注册

```go
mcp.AddTool(server, &mcp.Tool{Name: "kron_init", Description: "..."}, HandleInit)
mcp.AddTool(server, &mcp.Tool{Name: "kron_add",  Description: "..."}, HandleAdd)
// ... 12 calls total
```

**顺序锁定**:`tools_schema.go:331-344` 注释说"tools/list response is byte-identical across process starts"——**但** SDK 内部如果用 map 维护注册,这点保不住。当前**无 test 锁定顺序**。

### 3.3 `TreeNode.Children []any`——SDK 限制

```go
type TreeNode struct {
    // ...
    Children []any `json:"children,omitempty" jsonschema:"child nodes (directories first, then lexicographic name) — []any to break schema cycle on go-sdk v1.1"`
}
```

- **为什么 `[]any` 不是 `[]*TreeNode`**:go-sdk v1.1 的 jsonschema 推断在递归 struct 指针上**panic**("cycle detected")
- **wire shape 不变**:JSON 都是数组 of TreeNode 对象
- **handler 端**:`children := make([]any, 0, len(n.Children)); for _, c := range n.Children { children = append(children, treeNodeFromView(c)) }`——`treeNodeFromView` 返回 `*TreeNode`,直接当 `any` append

**未来**:SDK 修了 cycle detection 后,可以收回 `[]*TreeNode`(backlog 不列;等 SDK 升级)

---

## 四、Handler 实现层(`handlers_*.go` + `tools_*.go`)

### 4.1 通用 handler 模式

```go
func HandleXxx(ctx context.Context, _ *mcp.CallToolRequest, in XxxInput) (
    *mcp.CallToolResult, XxxOutput, error,
) {
    if err := assertCallerMCP(ctx); err != nil {    // 1. 防御
        return nil, XxxOutput{}, err
    }
    // 2. ctx 取 root
    root := repoRootFrom(ctx)
    // 3. 参数校验
    if in.Slug == "" {
        return nil, XxxOutput{}, fmt.Errorf("%w: ...", model.ErrSlugInvalid)
    }
    if err := parser.ValidateSlug(in.Slug); err != nil {
        return nil, XxxOutput{}, err
    }
    // 4. 调 internal/
    r, err := store.NewReader(root)
    if err != nil { return nil, XxxOutput{}, fmt.Errorf("...") }
    intent, err := r.Load(ctx, in.Slug)
    // 5. 投影到 wire shape
    return nil, XxxOutput{...}, nil
}
```

**协议层映射**:
1. `assertCallerMCP`——铁律 #4 防御(如果 middleware 漏了,这里炸)
2. `repoRootFrom(ctx)`——middleware 注入的 root
3. 参数校验——**Kron 不依赖 SDK 的 schema 校验**(双层防御)
4. `internal/store` / `internal/relations` / `internal/assumption` / `internal/view`——业务逻辑
5. 投影到 wire struct——**internal 类型绝不直接返回**(边界清晰)

### 4.2 错误处理

| 错误来源 | Kron 怎么包 | SDK 怎么呈现 |
|---|---|---|
| `model.ErrSlugInvalid` | `fmt.Errorf("%w: ...", model.ErrSlugInvalid)` | `error` 进 result.content[0].text;**IsError=true** |
| `model.ErrIntentNotFound` | 同上 | 同上 |
| `model.ErrFrontmatterInvalid` | `fmt.Errorf("%w: %s: %v", model.ErrFrontmatterInvalid, slug, err)` | 同上 |
| `model.ErrIntentExists` | `fmt.Errorf("%w: %s", ...)` | 同上(返回 -32005 Conflict per mcp.md spec) |
| `os.IsNotExist(err)` 包装成 `model.ErrConfigInvalid` | `fmt.Errorf("%w: .kron/ not found; run kron_init first", model.ErrConfigInvalid)` | 同上 |
| `assertCallerMCP` 失败 | 直接返 `error`,不进 wire shape | SDK 把 error 文本塞 `result.content[0].text`;**`IsError=true`** |

**关键**:Kron 永远用 `model.Err*` sentinel 包装,客户端可以用 `errors.Is` 在 content 文本里搜前缀判定错误类型(go-sdk 当前**没**把 sentinel error 的 `Is()` 关系序列化出去——这是 G-3 相关的能力缺失)

### 4.3 关键 handler 详解

#### 4.3.1 `HandleList`(`kron_list`)——[tools_list_get.go:28-63](tools_list_get.go)

```go
all, err := r.LoadAll(ctx)
var ar *assumption.Reader
if ard, ardErr := assumption.NewReader(root); ardErr == nil { ar = ard }  // ① 软读 assumption registry
out := ListOutput{Intents: make([]IntentSummary, 0, len(all))}
for _, intent := range all {
    if in.Prefix != "" && !strings.HasPrefix(intent.Slug, in.Prefix) { continue }
    out.Intents = append(out.Intents, intentSummaryFromIntent(intent, ar))  // ② 投影
}
```

① **"软读"** = `assumption.NewReader` 失败不致命(没 registry 的 repo 是 pre-migration);只在成功时挂上
② `intentSummaryFromIntent` 投影到 wire shape + 调 `assumesToWire` 把 inline + registry 假设合并

#### 4.3.2 `HandleImpact`(`kron_impact`)——[handlers_impact.go:47-156](handlers_impact.go)

```go
target, err := r.Load(ctx, in.Slug)
all, err := r.LoadAll(ctx)
references, _, _ := relations.ReverseLinks(all, in.Slug)         // ① reverse-link 图
prerequisites := relations.Prerequisites(all, in.Slug)           // ② explicit ∪ symbol-inferred

var ar *assumption.Reader
if ard, ardErr := assumption.NewReader(root); ardErr == nil { ar = ard }

// ③ 双流锚点
anchors, _ := parser.ScanAnchors(root)
markdownAnchors, _ := parser.ScanMarkdownAnchors(root)
allAnchors := append(anchors, markdownAnchors...)

for _, a := range allAnchors {
    if a.Slug != in.Slug { continue }
    kind := string(model.AnchorKindCode)
    if a.Kind.IsMarkdown() { kind = string(model.AnchorKindMarkdown) }  // ④ Kind 透传
    incoming = append(incoming, AnchorRef{...})
}
```

① **不取** depends-on / symbol-inferred(`_` 占位)——delete 关心,impact 不关心
② `Prerequisites` = 显式 `depends_on` ∪ 符号推断;"explicit wins" 规则
③ RFC `2026-10-08-md-anchors.md` §2.5:code 锚 + markdown 锚合并,`Kind` 字段透传
④ `model.AnchorKind.IsMarkdown()` 处理"零值默认为 code"的向后兼容

#### 4.3.3 `HandleIntentDensity`(`kron_intent_density`)——[handlers_density.go:33-104](handlers_density.go)

```go
codeAnchors, _ := parser.ScanAnchors(root)
markdownAnchors, _ := parser.ScanMarkdownAnchors(root)
withAnchor := make(map[string]struct{})
anchoredFiles := make(map[string]struct{})
for _, a := range codeAnchors { withAnchor[a.Slug] = struct{}{}; anchoredFiles[a.FilePath] = struct{}{} }
for _, a := range markdownAnchors { withAnchor[a.Slug] = struct{}{}; anchoredFiles[a.FilePath] = struct{}{} }
// orphans = walk + 过滤 anchored + 行数 > 50
orphans, _ := scanOrphanFiles(root, anchoredFiles)
```

- `scanOrphanFiles` 复用 `parser.WalkSourceFiles` 的 skip-list + binary sniff(避免重复实现)
- 50 行阈值与 CLI `kron lint` intent-coverage 规则一致(`docs/process/lint-rule.md`)

#### 4.3.4 `HandleUpdate`(`kron_update`)——[handlers_update.go:42-106](handlers_update.go)

```go
intent, err := r.Load(ctx, in.Slug)
// Apply patch (每个非 nil 字段 = replace)
if in.Symbol != nil { intent.Frontmatter.Symbol = in.Symbol }
if in.Status != nil { intent.Frontmatter.Status = model.Status(*in.Status) }
if in.References != nil { intent.Frontmatter.References = parser.DedupStrings(in.References) }
// ...
intent.Frontmatter.UpdatedAt = time.Now().UTC()
if err := validateFrontmatterShape(in.Slug, &intent.Frontmatter); err != nil {  // ① 写前重验
    return nil, UpdateOutput{}, fmt.Errorf("%w: %v", model.ErrFrontmatterInvalid, err)
}
w, err := store.NewWriter(root)
w.Write(ctx, intent)
```

① **PATCH 语义**:用 `*string` / `[]string` 区分"没传"(`nil`=不动)vs"传了空数组"(`[]string{}`=清空);v1.1 的 `patchString` / `patchStringList` 自定义反序列化已删(2026-10-03 决策)
② **写前重验**:`validateFrontmatterShape` 走 `parser.ValidateFrontmatter`(round-trip)+ self-reference 校验

#### 4.3.5 `HandleTree`(`kron_tree`)——[handlers_tree.go:34-86](handlers_tree.go)

```go
all, err := r.LoadAll(ctx)
internalTree := view.BuildIntentTree(all)
return nil, TreeOutput{Root: treeNodeFromView(internalTree)}, nil
```

- `view.BuildIntentTree` 把 `[]*model.Intent` 拍成 `*view.IntentTreeNode` 树(目录结构 = slug 分隔)
- `treeNodeFromView` 投影到 wire `*TreeNode`;**`Children []any`** 避开 SDK 递归 struct panic

#### 4.3.6 `HandleDelete`(`kron_delete`)——[handlers_delete.go:34-71](handlers_delete.go)

```go
dependents := []string{}
if r, rerr := store.NewReader(root); rerr == nil {
    if all, lerr := r.LoadAll(ctx); lerr == nil {
        dependents = relations.Dependents(all, in.Slug)  // ① 软读
    }
}
w, err := store.NewWriter(root)
if err := w.MoveToTrash(ctx, in.Slug); err != nil { return ... }
return nil, DeleteOutput{OK: true, TrashedPath: ..., Dependents: dependents}, nil
```

① **move 前算** dependents——避免 move 后 LoadAll 漏数据(并发安全)
② `dependents` **不阻塞** delete——v1 软警告(RFC `2026-10-03-frontmatter-references` §5.3)

---

## 五、ctx 工具(`tools.go`)

### 5.1 `assertCallerMCP`——防御

```go
func assertCallerMCP(ctx context.Context) error {
    c := model.CallerFrom(ctx)
    if !model.IsMCPCaller(c) {
        return fmt.Errorf("internal: tool invoked with non-MCP caller %q (dispatcher bug)", c)
    }
    return nil
}
```

- 每个 handler **第一个**调用——middleware 漏了立刻炸
- 注释说"since v1.2 it's still here"——`internal/` 已不依赖 caller 行为分支,但**防御性 guard 保留**

---

## 六、测试层

### 6.1 in-memory client(`testkit.go`)——工具级

- `newTestSession` 创建 client + server,`InMemoryTransport` 走同进程 RPC
- 所有 `handlers_test.go` / `handlers_relations_test.go` 用这个——覆盖 12 个 tool 的 In/Out 转换、错误路径、registry 兜底

### 6.2 stdio smoke(`serve_mcp_test.go`)——协议级

**三个 test**:
1. `TestRun_StdioProtocolSmoke`——**双重 skip**(`KRON_E2E` 没设 + 注释说"subprocess plumbing deferred to commit 3")
2. `TestRun_EmptyStdin`——空 stdin 不挂、零输出(SDK 默认行为)
3. `TestServer_Notification_NoResponse`——无 `id` 字段的请求不应产生响应(JSON-RPC notification 规则)

**G-2 真 e2e stdio 测试缺位**——这是 backlog 第一项

### 6.3 基准(`bench_test.go`)

- 11K 字节,具体规则不展开
- 关注点:`LoadAll` 在大仓库下的开销(直接驱动 G-7 缓存需求)

---

## 七、和"协议标准"对照(2026-10-11 现状)

| MCP 协议能力 | Kron 是否用到 | 备注 |
|---|---|---|
| `initialize` handshake | ✅(SDK 默认) | 没显式接 `InitializeHandler` |
| `tools/list` | ✅ | 12 个 tool 来自 `mcp.AddTool` |
| `tools/call` | ✅ | handler 签名见 §四 |
| `notifications/tools/list_changed` | ❌ | 当前 tool 集合是静态的;未来动态加 tool 才需要 |
| `notifications/cancelled` | ❌ | 没用;**未来加 ctx 中间件超时**需要处理 |
| `Resources`(URI 资源) | ❌ | 未来 `kron://...` URI 可能用 |
| `Prompts`(prompt 模板) | ❌ | 当前不适用 |
| `Logging` | ❌ | 当前走 `log.SetOutput(errOut)` 写 stderr,没用 `logging/setLevel` |
| `Subscriptions/Listen` | ❌ | MCP 2.0 SEP-2575 新能力;**协议层面很新,Kron 不追** |
| `completion/complete`(参数自动补全) | ❌ | LLM 客户端能力,Kron 不参与 |
| `roots`(客户端声明 root) | ❌ | Kron 用 `-CWD <root>` 直接收 |
| `sampling`(让 server 反向让 LLM 生成) | ❌ | 不适用 |

**评估**:Kron 当前用的是 MCP 协议的**核心子集**(`initialize` + `tools/list` + `tools/call`)。够用,符合"YAGNI"——加其他能力**等真有需求**。

---

## 八、ctx 透传:从入口到 `internal/` 的链路

```
[stdio JSON-RPC 帧]
    ↓
[server.Run(transport)]                        ← SDK 解析
    ↓
[AddReceivingMiddleware(callerInjectMiddleware)] ← 注入 root + caller
    ↓
[mcp.AddTool 注册的 handler]                   ← assertCallerMCP guard
    ↓
[handler 第一行:repoRootFrom(ctx)]
    ↓
[handler 调 store.NewReader(root) / parser.ScanAnchors(root)]
    ↓
[internal/ 函数:ctx context.Context 参数]      ← 取消 / 超时传播
    ↓
[handler 投影到 wire struct]                   ← 边界:internal 类型不外漏
    ↓
[SDK 包成 {content, structuredContent}]
    ↓
[写 stdout JSON-RPC 帧]
```

**关键点**:
- `ctx` **不**用来做 caller-based 行为分支(架构 §2.3 变更)
- `ctx` **用**来做取消 / 超时传播(铁律 #4 保留部分)
- 根路径走 ctx,不走闭包(v1.1 → v1.2 收口)

---

## 九、与 [`backlog.md`](./backlog.md) 的关系

`backlog.md` 列 7 项已知 GAP(编号 G-1 ~ G-7),每项都对应本文件描述的某段代码。**改任何一段前先读 backlog 那一项**——避免重复解决。
