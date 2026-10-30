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
| `handlers_assumption.go` | 200 | `kron_assume_create` / `kron_assume_list` / `kron_assume_update` / `kron_assume_delete` / `kron_assume_get` (2026-10-14) |
| `handlers_prompts.go` | 420 | `kron-implement` / `kron-drift-check` / `kron-review` 3 个 prompt 模板的 handler (2026-10-14) |
| `prompts_register.go` | 65 | `registerPrompts` — `server.AddPrompt × 3` 注册入口 (2026-10-14) |
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
    server := mcp.NewServer(&mcp.Implementation{Name: "kron", Version: "v0.3.0"}, nil)
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
| `mcp.NewServer(impl, opts)` | 创建 server;`impl`（Name/Title/Version）与 `opts.Instructions` 出现在 initialize 响应;capabilities 由后续注册调用推断 |
| `AddReceivingMiddleware` | 注入 root / caller 到 ctx;**铁律 #4 落地** |
| `registerTools` | 18 次 `mcp.AddTool`——SDK 据 In/Out struct 推断 schema；每个 tool 带 `Title` + `ToolAnnotations`（readOnly/destructive/idempotent/openWorld） |
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

#### 4.3.6 `HandleDelete`(`kron_delete`)——[`handlers_delete.go`](handlers_delete.go) (2026-10-29 子树化)

```go
all, _ := LoadAllCached(ctx, root)              // ① 软读
// kind / 子树 slugs / dependents 并集都来自这一份快照
if kind == node && !in.ConfirmSubtree { return nil, ..., 要求 confirm_subtree }
w.MoveToTrash(ctx, in.Slug)                     // node → 整个 <slug>/ 目录 rename 进 trash
return DeleteOutput{TrashedPath: <真实路径>, Dependents: <子树并集>, DeletedSlugs: <全部删除 slug>}
```

① **move 前算** dependents——避免 move 后 LoadAll 漏数据(并发安全)
② `dependents` **不阻塞** delete——v1 软警告(RFC `2026-10-03-frontmatter-references` §5.3)
③ node 删除必须显式 `confirm_subtree: true`，否则返回 IsError 并告知子树意图数；trash 布局与恢复语义见
[`docs/implementation/2026-10-29-trash-subtree.md`](../implementation/2026-10-29-trash-subtree.md)
④ `kron_restore` 按 trash 形状还原（node 目录 → 整棵子树）；旧 flat trash 条目按 leaf 恢复

#### 4.3.7 `HandleAssumptionGet`(`kron_assume_get`)——[`handlers_assumption.go`](handlers_assumption.go) (2026-10-14 追加)

走 `internal/assumption.Reader.Get(ctx, id)`(reader.go:65-100), 投影到 `AssumptionDetail`(含 `body` + `source_path`)。新 struct 而非扩展 `AssumptionSummary`——保持 list 响应最小。错误路径:`ErrAssumptionNotFound` / `ErrAssumptionFileIdMismatch`(均 → IsError=true);**不**静默回退 id-mismatched 文件(与 `List` 行为不同)。

#### 4.3.8 `kronImplementPrompt`(`kron-implement`)——[`handlers_prompts.go`](handlers_prompts.go) (2026-10-14 追加)

AI 接到新需求时: 走 `LoadAllCached` 拿全部 intent, `matchIntentsByKeyword` 按 slug / frontmatter.symbol / 第一行标题做朴素大小写不敏感子串匹配排序, 命中的 intent 渲染 `relations.Prerequisites` + 假设链接（id/severity/rationale/expires_at/verified_at），Action 要求先验证过期/未验证的 hard 假设。**`feature` 必填**, 缺 → 报错。0 匹配**不**报错, message 引导"kron_add a new one"。

#### 4.3.9 `kronDriftCheckPrompt`(`kron-drift-check`)——[`handlers_prompts.go`](handlers_prompts.go) (2026-10-14 追加)

走 `lint.ComputeStaleReport` 拿结构化视图(`SupersededCandidates` + `ExpiredAssumptions`), 过期条目按 `intent_slug: assumption_id (expired N day(s) ago; expires_at=...)` 渲染；Action 要求用 `kron_assume_update` 复核（`verified_at` / `expires_at`）或降级 severity。`days_threshold` 选填, 默认 90; 不可解析 → 默认值 (静默回退)。复用 `LoadAllCached` + `internal/lint`(不调 `kron_stale` tool, 避免 server 内部 RPC 自身)。

#### 4.3.10 `kronReviewPrompt`(`kron-review`)——[`handlers_prompts.go`](handlers_prompts.go) (2026-10-14 追加)

走 `lint.RunWith` 拿 `[]Diag` 全规则集, 按 `Rule` 字段分组 → 渲染"按规则分组（location + detail + severity）+ 每个 in-scope intent 列表"；Action 含假设复核与重跑 lint。`status` 选填, 默认 "active"; 不可解析 → 默认值。

#### 4.3.12 项目 intent 正文模板（AI 提示词注入，2026-10-29 追加）

项目可在 `.kron/templates/intent-body.md` 定义正文 section 形状（例如 `## Why` / `## How`）。
serve-mcp 只把它注入 **AI 可见文本**，不写入 intent、不做校验：

- initialize `Instructions` 追加 "Project intent body template" 段；
- `kron_add` / `kron_update` 的 tool description 追加模板；
- `kron-implement` prompt 末尾追加模板 section。

文件缺失/空 = 不注入（行为与之前一致）；模板在 server 启动时读取一次（tool description/instructions），
prompt 每次调用实时读取。实现见 `intent_template.go`。

#### 4.3.11 `registerPrompts`——[`prompts_register.go`](prompts_register.go) (2026-10-14 追加)

3 个 `mcp.AddPrompt` 注册入口, SDK 自动推断 `prompts: { listChanged: true }` capability (与 Tools 同样路径)。`capabilities_test.go` 增 1 case 锁定 SDK 行为防止 regression。

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

| MCP 协议能力 | Kron 状态 | 备注 |
|---|---|---|
| `initialize` handshake | ✅ | `Implementation{Name: kron, Title: Kron, Version}` + `ServerOptions.Instructions`（AI 工作流指引） |
| `tools/list` / `tools/call` | ✅ | 18 个 tool；typed In/Out → inputSchema/outputSchema/structuredContent；每个 tool 带 Title + Annotations |
| tool annotations | ✅ | `readOnlyHint`（只读工具）、`destructiveHint`（delete/assume_delete）、`idempotentHint`、`openWorldHint=false` |
| `resources/list` / `templates/list` / `read` | ✅ | `kron://intents/{+slug}` 模板；无静态 resource 条目（模板-only，客户端需支持 templates） |
| `prompts/list` / `prompts/get` | ✅ | 3 个模板 + arguments；描述与渲染内容同步（假设/过期上下文/severity） |
| `logging` / `logging/setLevel` | ✅ | SDK 处理 setLevel；`shimHandler` 把 slog 转 session.Log |
| `notifications/tools/list_changed` | ⚠️ | SDK 广告 listChanged=true，但 tool 集合静态、不主动发通知 |
| `notifications/cancelled` | ✅（SDK） | SDK 处理 cancel；handler 使用 ctx |
| `completion/complete` | ❌ | 未注册 `CompletionHandler`（可选） |
| `resources/subscribe` | ❌ | 未注册订阅 handler（可选） |
| `roots` / `sampling` | ❌ | 不适用；Kron 用 `-CWD` |

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
