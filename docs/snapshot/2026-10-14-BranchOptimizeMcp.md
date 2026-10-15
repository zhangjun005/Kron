**关键发现 11**:
- `go vet` clean
- `gofmt` 报 **5 个文件未格式化**:`cli.go`, `cli_test.go`, `init.go`, `register.go`, `serve_mcp.go`(都是我刚 `Write` 的或本 PR 改过的) — **commit 阶段没跑 gofmt**

完成。下面是完整状态报告。

---

# 📋 Kron v1.3+ 完整代码现状(2026-10-14)

> 浏览范围:`cmd/kron/main.go`, `cli/{cli,init,add,lint,register}.go`, `serve-mcp/{serve_mcp,cache,resources,prompts_register,tools_schema,tools,handlers_*}.go`, `serve-lsp/main.go`, `internal/{model,store,parser,lint,relations,view,assumption,identity}/*.go`。
> 测试: 11/11 包全绿, `go vet` clean, **`gofmt` 5 文件脏**。

## 1️⃣ 架构边界(代码验证,非文档)

| 边界 | 检查 | 状态 |
|---|---|---|
| `internal/` → `cmd/` | 0 imports | ✅ 严守 |
| `cli` → `serve-mcp/lsp` | 0 imports | ✅ 严守 |
| `serve-mcp` → `cli/lsp` | 0 imports | ✅ 严守 |
| `serve-lsp` → `cli/serve-mcp` | 0 imports | ✅ 严守(只有 stub 文件) |
| `internal/` 用 `model.CallerFrom/WithCaller/IsMCPCaller` | 0 真实使用(3 处全是 `model/caller.go` 自己) | ✅ 严守 |

**架构守得很好**。`model/caller.go` 自己承认 API 是 "protocol scaffolding for v1.2+, NOT a consumed v1 feature"。

## 2️⃣ 访问层 — 3/3 拍板 + 1 客户端占位

| 访问层 | 状态 | 主要 capability |
|---|---|---|
| `cmd/kron/cli` | 5 子命令 (`init` / `add` / `register` / `lint` / `help`)+ `main.go` 路由 | 全部 stdlib flag,无 cobra |
| `cmd/kron/serve-mcp` | **17 tools** + **1 resource template** + **3 prompts** + `callerInjectMiddleware` + `LoadAllCached` | 官方 `modelcontextprotocol/go-sdk v1.1.0` |
| `cmd/kron/serve-lsp` | stub,返 `ErrNotImplemented`,0 字节 import 内部 | 0 deps,等 RFC 拍板后加 `go.lsp.dev/protocol` |
| `frontend/` | README placeholder(空目录) | 客户端不进主仓 ✅ |

**入口统一**:`main.go` 用 `os.Args[1]` switch → 三个访问层 peer。`exitFromErr` 同时匹配 `cli.ExitCoder` + `servemcp.ErrNotImplemented` + `lsp.ErrNotImplemented`。

## 3️⃣ 17 MCP tools(全部已注册到 `tools_schema.go:500-517`)

**意图 CRUD**:`kron_add` / `kron_get` / `kron_list` / `kron_update` / `kron_delete` / `kron_restore`

**关系**:`kron_impact` / `kron_intent_density`

**视图**:`kron_tree`(返回 `[]any` children 绕开 SDK schema 递归 panic)

**项目**:`kron_init`

**假设(B-3 独立文件)**:`kron_assume_create` / `kron_assume_list` / `kron_assume_get` / `kron_assume_update` / `kron_assume_delete` / `kron_assume_check`

**健康**:`kron_lint` / `kron_stale`

## 4️⃣ 3 MCP prompts(2026-10-14 拍板)

- `kron-implement` (feature 必填)— keyword match + `relations.Prerequisites`
- `kron-drift-check` (days_threshold 选填,默认 90)— `lint.ComputeStaleReport`
- `kron-review` (status 选填,默认 active)— `lint.RunWith` 全部

## 5️⃣ 1 Resource template

`kron://intents/{+slug}` — RFC 6570 reserved expansion 支持 `auth/jwt` 多段 slug。读取走 `parser.SerializeMarkdown` 返回原始 markdown bytes。

## 6️⃣ 内部包分层

| 包 | 行数规模 | 关键 API | 备注 |
|---|---|---|---|
| `model` | 13k | `Intent` / `Frontmatter` / `Anchor`(+Kind) / `IntentKind` / `WithCaller`(scaffolding) | 零外部依赖 |
| `store` | 18k + 9k registry | `Reader` / `Writer` / `ProjectRegistry`(本 PR) / `resolveRead/WritePath` / `writeFileAtomic` | 含 kind-aware slug → path 路由 + README shorthand |
| `parser` | 25k | `SplitMarkdown`(sentinel+legacy fence) / `ParseFrontmatter` / `ScanAnchors` / `ScanMarkdownAnchors`(fence-aware) | fence 状态机,跨行跟踪 |
| `lint` | 19k + 11k rules | `Run` / `RunWith` / `ComputeStaleReport` / 8 rule 函数(5 A/B-class + 3 C/S-class + 5 B-3) | 命名规则 typed,Diag 双轨输出 + StaleReport 结构化 |
| `relations` | 8k | `ReverseLinks` / `Prerequisites` / `Dependents` / `ReferencingIntents` | 纯计算,无 I/O |
| `view` | 8k | `BuildIntentTree` / `IntentTitle`(H1 + slug fallback) | 1 I/O = 0 |
| `assumption` | 18k+5k | `Reader.List/Get` / `Writer.Create/Update/Delete/Restore` | B-3 独立文件体系 |
| `identity` | 3k | `GitUser`(exec git) / `Handle`(回退链) | "agent:unknown" 兜底 |

## 7️⃣ 性能 / 优化(G-7 OPT-1)

- `cache.go` — `LoadAllCached` 走 `singleflight + mtime invalidation`,346x 提速(bench 文档)
- `singleflight` **自实现**(180-200 行),避加 `x/sync/singleflight` dep — 注释承认
- `sortAnchors` 插入排序(`<10k` 假设) — 注释承认
- `parseConfigTOML` 自实现,避加 `BurntSushi/toml` dep — 注释承认

## 8️⃣ 主要发现(实际可改的)

### 🟡 **代码瑕疵(已发现,可问要不要修)**

| # | 位置 | 问题 |
|---|---|---|
| F1 | `cmd/kron/serve-mcp/serve_mcp.go:441-456` `clientNameFromReq` | 永远返 `""` — 注释承认"intentionally do not call req.GetSession()",client attribution 死代码。`assertCallerMCP` 16 处全跑但所有 ctx 落到 bare `"mcp"` |
| F2 | `cmd/kron/serve-mcp/resources.go:158-174` `isIntentNotFound` | 手写 `Unwrap()` 循环,虽然同文件其它处已经 `import "errors"` — 应该用 `errors.Is` |
| F3 | `cmd/kron/serve-mcp/handlers_prompts.go:158-164, 232-238` | `ar, arErr := assumption.NewReader(root); _ = ar` — reader 创建后即丢弃,完全死代码 |
| F4 | `cmd/kron/serve-mcp/cache.go:170-200` `singleflight.Do` 返 `err` | 永远是 `nil`,调用方 `res, _, _ := sfGroup.Do(...)` 收 `cacheResult{err}` 字段(API 形状混乱)|
| F5 | `cmd/kron/serve-mcp/tools_schema.go:444-460` `TreeNode.Children []any` | 注释承认"SDK v1.1 recursive struct panic 绕道",wire shape OK 但失去 Go 类型安全 |
| F6 | `cmd/kron/serve-mcp/handlers_init.go` | `kron_init` **不**写 `projects.json::current_project`,而 `cli/init.go` 自动 register。**两套 init 行为不一致** — 用户在 MCP 客户端调 init 后,GUI 仍读不到 current_project |
| F7 | `cmd/kron/serve-mcp/handlers_add.go:180-220` `resolveAssumptionsForIntent` | rationale 长度校验**不在这里**(注释解释 "deferred to lint"),但 auto-create 用的 `r.Rationale` 字段从未校验 — 写一个 <10 char 的 stub registry 是合法但后续被 lint 拦下,路径不优雅 |
| F8 | `go fmt` 5 文件脏 | `cli/cli.go`, `cli/cli_test.go`, `cli/init.go`, `cli/register.go`, `serve-mcp/serve_mcp.go` — **提交前未跑 `gofmt -l .`** |

### 🟢 **架构拍板正确(已就位)**

- `internal/` 零依赖 `cmd/kron/` ✅
- 3 访问层互不 import ✅
- `internal/model` 零外部 import ✅
- 错误类型 + 包装规约(`%w` + `errors.Is`/`errors.As`) 拍板
- 写文件统一走 `writeFileAtomic`(store/registry 都复用)
- 写后 `InvalidateCache(root)` 拍板(handlers 严格执行)
- `model/CallerFrom/WithCaller` 公开但 internal 零使用,访问层接 ctx 走

### 🟠 **架构警告(待你定要不要改)**

| 位置 | 描述 |
|---|---|
| `model/caller.go:3-31` | 头注释自己承认 caller API "NOT a consumed v1 feature" — 实际是死代码,但保留向后兼容 |
| `assertCallerMCP` × 16 处 | 每次 handler 入口多 1 次 `CallerFrom` + `IsMCPCaller`,纯防御性 dispatcher-bug check |
| `clientNameFromReq` 注释 | 引用的 `docs/process/rfc-2026-10-04-mcp-sdk-adoption.md` 路径**不存在**(真实是 `docs/rfc/2026-10-04-mcp-sdk-adoption.md`) |

### 🔴 **生产语义缺口**

- **`projects.json::current_project` 在 MCP 路径下写不出去**: `kron_init`(MCP) 只建 `.kron/` + `config.toml`, **不** register 不设 current。如果用户从 Claude Code 调 `kron_init` 创建新项目,然后想"切回 GUI 的 current",projects.json 完全没记录。CLI init 走 register 路径是 OK 的,但 MCP init 漏了。**这是 PR 拍板范围外的 gap**。