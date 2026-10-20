# LSP 模块（serve-lsp）决策点汇总

| 字段 | 值 |
|---|---|
| **状态** | **待组长拍板 → 决策已全部落地 + 已归档 (2026-10-19)**：§1 `go.lsp.dev/*` 依赖入 `go.mod`（批准依据见 [`pending-decisions.md`](../../process/pending-decisions.md) 的 LSP-SDK-DEP Resolved 条）；§2 `parser.AnchorsForFile` 已实开（`4a2781d`）；LSP v1.3 已合入 master。本文件仅作历史决策记录。 |
| **归档** | 2026-10-19 — 移入 `docs/rfc/archive/`；当前实施看 [`docs/implementation/lsp.md`](../../implementation/lsp.md) |
| **负责人** | @HomoFluctus |
| **分支** | `feat/serve-lsp` |
| **目标版本** | v1.3 |
| **创建日期** | 2026-10-12 |
| **上游** | [`2026-10-07-lsp-sdk.md`](./2026-10-07-lsp-sdk.md)（SDK/协议选型）+ [`2026-10-08-lsp-client.md`](./2026-10-08-lsp-client.md)（进程模型/跨语言接入） |

> 每个决策点按「**拍板** → **为什么必须这样** → **不这样做 / 选别的方案会怎样**」展开，方便组长快速判断。

---

## 一、红线级（需组长显式确认）

### 1. 新增第三方依赖 `go.lsp.dev/protocol`

- **拍板**：引入 `go.lsp.dev/protocol v1.0.1`（连带 `go.lsp.dev/jsonrpc2` + `go.lsp.dev/uri`）。
- **为什么必须这样**：LSP 是**标准协议**（JSON-RPC over stdio，Content-Length 帧）。架构铁律（`new-access-layer.md` §2 问题 2）明确：**标准协议走现成 SDK，不手写 parser**。手写 LSP 编解码要处理帧边界、转义、错误码、取消语义，极易出错，且是本项目第一次接编辑器，协议 bug 会直接表现为「VSCode 连不上 / hover 不弹」这种难排查的问题。
- **不这样做会怎样**：
  - **手写 LSP 协议** → 违反流程铁律 + 工作量数倍 + 编辑器兼容性风险（协议细节写错就是静默失败）。
  - **换其他 Go SDK**（如 `segmentio/gls`、`sourcegraph/go-lsp`）→ 偏离 `2026-10-07-lsp-sdk.md` §3 已拍板的选型结论，需重新走竞品对比流程。
- **一个需知悉的偏差**：SDK 实现的是 **LSP 3.18**，不是 RFC 拍的 **3.17**。两者向后兼容，hover / definition 行为一致；仅当有「必须严格 3.17」的洁癖时，才需重新评估换 SDK（但 go.lsp.dev/protocol 只发 3.18 语义）。

### 2. 新增 internal API `parser.AnchorsForFile`（**重点审阅**）

- **拍板**：在 `internal/parser` 新增 `AnchorsForFile(file) ([]model.Anchor, error)`，返回**带行号**的单文件锚点。
- **为什么必须这样**：LSP hover 请求带 `position: {line, character}`，服务端必须回答「**光标这一行**是哪个 intent」。而 `2026-10-08-lsp-client.md` §4.4 草案曾判断「复用 `parser.SlugsForFile` 即可」——**这个判断是错的**：

  具体反例：`main.go` 第 3 行有 `// @kron:intent auth-jwt`、第 7 行有 `// @kron:intent rate-limit`。
  - `SlugsForFile` 返回 `["auth-jwt", "rate-limit"]`（去重 slug，**丢行号**）。
  - 用户光标停在第 3 行，hover 带 `line=2` 过来，服务端拿到这串 slug **根本不知道第 3 行是哪一个**，无法正确渲染。
  - 也就是说，用 `SlugsForFile` 连「hover 弹哪条」都答不上，v1.3 的 hover 功能**做不出来**。

- **不这样做会怎样**（两条路都不通）：
  - **复用 `SlugsForFile`** → 无法按行定位，hover 功能报废（如上面反例）。
  - **在 serve-lsp 里自己重读文件按行解析** → 重复造 `parser` 已有的未导出 `parseAnchorLine`，违反铁律「业务逻辑只在 `internal/`」。
- **为什么不选旧 RFC 拍的 `AnchorAtPosition(file, line, col)` 而是 `AnchorsForFile`**：
  - `AnchorsForFile` 返回全文件锚点，行匹配留在访问层（`anchorSlugAt` 遍历 `LineNumber`），**职责更清晰**：parser 只做「提取」，访问层做「匹配」。
  - 它直接封装已有的未导出 `scanFile`，新增代码量最小、可复用性好（未来 completion 的 slug 补全也要扫全文件锚点）。
- **影响**：已走 `docs/process/new-internal-api.md` 流程，并已修正 RFC §4.4。

---

## 二、需组长知晓的决策（已按架构铁律执行）

### 3. repo root 来源：可选 `-CWD <root>`，缺省 `os.Getwd()`

- **为什么必须这样**：`store.NewReader(root)` 要靠 root 定位 `.kron/` 目录。编辑器（VSCode 扩展）spawn 子进程时，工作目录**不保证**是 repo root——如果只认 `os.Getwd()`，从别的目录启动会找不到 `.kron`，导致 `Load` 全失败、hover 全 null。
- **不这样做会怎样**：没有 `-CWD` 覆盖，编辑器多窗口 / 非 root 启动场景直接失效；而 serve-mcp 已有此 flag，照抄能保证两个子命令被编辑器以**同一种方式** spawn。

### 4. 能力范围收敛：v1.3 只做 `initialize / hover / definition / shutdown / exit`

- **为什么**：v1.3 目标是「最小可用」，先验证 stdio 握手 + 两个核心跳转能力。
- **不做 completion / diagnostics 会怎样**：
  - 无 `completion` → 用户敲 `@kron:intent ` 时没有 slug 补全（可接受，不影响核心流程）。
  - 无 `publishDiagnostics` → 编辑时不给悬空锚点画红线（等价 `kron lint` A-class 的能力，留到 v1.4+ 与 lint 引擎共用）。

### 5. 进程模型：stdio + Content-Length 帧，单实例串行，父进程寿命

- **为什么必须这样**：
  - **stdio（非 socket）**：LSP 协议强约束 stdio/socket 二选一，stdio 最简单，且与 serve-mcp 同模式。
  - **串行（非并发）**：hover / definition 都是**纯读** + **本地小文件**（< 100 KB），单请求延迟 < 5ms，串行足够。
  - **父进程寿命（非 daemon）**：编辑器退出 = serve-lsp 退出，无共享状态、无锁、无状态同步。
- **不这样做会怎样**：做 daemon/socket 引入多实例共享状态与锁；做并发引入 goroutine 调度 + ctx 取消复杂度——v1.3 用不上，纯属过度设计（违反「不为假想需求设计」）。

### 6. 悬空锚点返回 `null`，不报协议错误

- **为什么必须这样**：悬空锚点（intent 被删 / 改名 / 未建）是**正常情况**，不是异常。LSP 语义里 hover/definition 返回空结果是「无此能力」，编辑器自然不弹窗、不跳转。
- **不这样做会怎样**：若返回协议错误，编辑器会弹出错误提示，且触发客户端的错误回退逻辑；把正常情况当错误处理，体验差且不符合 LSP 惯例。

### 7. 不调用 `model.WithCaller`

- **为什么必须这样**：`model.WithCaller` 是已废弃但保留的 caller 注入 API，`ctx` 注入身份属性不便于开发（调试栈不直观、单测需额外包装、类型安全弱）。
- **不这样做会怎样**：新代码若调用废弃 API，违反架构红线，且将来 `caller.go` 删除时直接 break。

---

## 三、文档修正（纯记录，无代码影响）

| 修正 | 原（错） | 实际（对） | 后果若不改 |
|---|---|---|---|
| API 名 | `store.Get` | `store.Load` | 照文档写代码会调错 API |
| CLI 框架 | `cobra` | stdlib flag 手写分发 | 与 CLAUDE.md 现状冲突（cobra 是过时信息） |
| Go 包命名 | — | 目录 `serve-lsp`（对齐 serve-mcp），包名 `lsp` | import path 单段一致性 |

---

## 待组长回应

- **§1 依赖**：`go.lsp.dev/protocol v1.0.1` 是否正式批准入 `go.mod`？（含 LSP 3.18 vs 3.17 的偏差知悉）
- **§2 新 API**：`parser.AnchorsForFile` 是否接受？（**此点修正了原 RFC §4.4 的实质错误判断，建议重点审阅**）
- 其余为执行层既定铁律，如实汇报即可。
