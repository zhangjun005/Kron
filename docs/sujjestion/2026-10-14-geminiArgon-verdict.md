# 复审：`docs/sujjestion/2026-10-14-geminiArgon.md`

> **状态**：复审产物 — 记录 Gemini 建议文件的属实/不属实部分，并给出**与 Kron 架构铁律对齐的**正确建议。
> **拍板**：
> - **不**重做 Gemini 描述的 4 件事（已做 / 违反铁律 / 与 RFC 冲突）
> - **只**采纳与其一致方向 + Kron 架构内的剩余可改动作
> - 推动决策**走** `docs/process/` 对应流程（mcp-tool / internal-pkg / cli-flag）— **不**直接动 code

---

## 〇、复审方法

按 Kron 架构源（[architecture.md §〇](../abstractDesign/architecture.md) 5 铁律）+ 已实装的 `internal/store`、`cmd/kron/serve-mcp` 现状，**逐条**对照 Gemini 建议文件：

| 维度 | 来源 |
|---|---|
| 架构铁律 | [`docs/abstractDesign/architecture.md` §〇](../abstractDesign/architecture.md) |
| 已实装缓存 | `cmd/kron/serve-mcp/cache.go`（OPT-1，[`docs/implementation/bench-2026-10-09.md` §4](../implementation/bench-2026-10-09.md)） |
| 已实装原子写入 | `internal/store/writer.go:writeFileAtomic`（line 478-503） |
| 已实装 `-CWD` 解析 | `cmd/kron/serve-mcp/serve_mcp.go:resolveMCPRoot`（line 264-296） |
| `roots/list` 拍板 | [`docs/rfc/2026-10-13-multi-project-registry.md` §2.8](../../rfc/2026-10-13-multi-project-registry.md)（拍板**不**走 MCP `roots/list`，走 `projects.json::current_project`） |
| Prompts 拍板 | [`docs/mcp/plan-2026-10-13-mcp-completeness.md` §3.1](./plan-2026-10-13-mcp-completeness.md)（待 PR） |

---

## 一、Gemini 建议文件逐条核实

### §一·1 "为什么 Kron 不需要 Daemon"

| 论点 | 属实度 | 证据 |
|---|---|---|
| 1.1 Go 进程 10–25MB | ✅ 属实 | 实测 stdio 子进程 < 30MB（[`bench-2026-10-09.md`](../implementation/bench-2026-10-09.md)） |
| 1.2 毫秒级解析 | ✅ 属实 | `parser.ParseFrontmatter` 单文件 < 1ms（[`bench-2026-10-09.md` §3](../implementation/bench-2026-10-09.md)） |
| 1.3 Daemon 引入 IPC / 僵尸进程 / 升级一致性问题 | ✅ 属实 | 与[架构 §〇 铁律 #5](../abstractDesign/architecture.md) 一致："CI lint 是唯一 gate" — 无运行时隐式状态 |

**结论** ✅ **100% 属实**。**不动**。

---

### §二·1 区分两类数据源的缓存策略

| Gemini 描述 | 实情 | 属实度 |
|---|---|---|
| 现状 "MCP 每次全量扫描" | ❌ **不属实**。`cmd/kron/serve-mcp/cache.go::LoadAllCached`（`serve-mcp` 进程内 + `singleflight` + `intentsDirMtime` 校验）**已实装** | 0% 属实 |
| 建议 `store` 层加 `map[filepath]ModTime` | ❌ **违反架构铁律 #5**——`internal/store` **不**得有运行时隐式状态；缓存放 access layer（serve-mcp），不放 store | 反模式 |
| 建议源码锚点用 `fsnotify` 增量 | ⚠️ 方向对，**未实装**。但**拍板不优先**——`fsnotify` 需新增 top-level dep（[AGENTS.md §3 红线](../../AGENTS.md)），且建议文件自己说"分两阶段，阶段 A 零依赖" | 部分属实 |

**结论** ⚠️ **30% 属实**。**不**采纳建议原文——`store` 层缓存**违反铁律**。**MCP 层缓存已做**。

---

### §二·2 store 跨进程原子写入

| Gemini 描述 | 实情 | 属实度 |
|---|---|---|
| 现状 "直接覆盖写入 `os.WriteFile`" | ❌ **不属实**。`internal/store/writer.go:writeFileAtomic`（line 478-503）**早已是** `os.CreateTemp` + `f.Sync()` + `os.Rename` 完整实现 | 0% 属实 |
| 建议 `store` 包内替换 `os.WriteFile` | ✅ 方向对，但**已实装** | 100% 方向对 / 0% 待做 |
| `Write` / `MoveToTrash` / `RestoreFromTrash` / `Update` 全部走原子写 | ✅ 属实 | 100% |

**结论** ✅ **100% 已做**。**不**动手。

---

### §二·3 `--workspace` + MCP `roots/list`

| Gemini 描述 | 实情 | 属实度 |
|---|---|---|
| 加 `--workspace /path/to/repo` flag | ⚠️ **概念属实，命名不准**。Kron 已有 `-CWD <root>`（`serve_mcp.go:289`），`os.Getwd()` fallback | 70% 属实 |
| 接入 MCP `roots/list` 协议 | ❌ **与拍板冲突**。[`RFC 2026-10-13-multi-project-registry.md` §2.8](../../rfc/2026-10-13-multi-project-registry.md) 拍板 `resolveMCPRoot` 3 级 fallback：`-CWD` > `projects.json::current_project` > `os.Getwd()`。**不**走 MCP `roots/list` | 0% |
| `roots/list_changed` 通知 | ❌ 同上，**不**做 | 0% |

**结论** ⚠️ **30% 属实**。`projects.json` 已拍板做 `current_project` 持久化（MCP roots 是**客户端临时状态**，重启即失；`projects.json` 是**持久化当前项目**，GUI 切项目需要）。

**RFC 拍板**：`projects.json::current_project` 字段（RFC §2.7）+ `resolveMCPRoot` 3 级 fallback（RFC §2.8），**不**走 MCP `roots/list`。

---

### §二·4 `stdio` 隔离 + `ctx` 取消

| Gemini 描述 | 实情 | 属实度 |
|---|---|---|
| 禁止底层包 `os.Stdout` | ✅ **属实**。`internal/` **不**允许 `os.Stdout` 写入。**Kron 用 `io.Writer` 注入**（`runInit` / `runAdd` / `runLint` 接 `out, errOut`）—— Gemini 建议的"全局 `log.SetOutput(os.Stderr)`" 是反模式 | 70% 属实 |
| 全局 `log.SetOutput(os.Stderr)` 入口 | ❌ **反模式**。Kron **不**用 `log` 包 + `log.SetOutput`；`io.Writer` 注入是**单元测试可验证的**（每个 handler 接 writer，测试用 `bytes.Buffer` 抓输出断言） | 0% |
| 父 `ctx` 接 `SIGTERM` / `SIGINT` / `EOF` → 取消 | ✅ **属实**。所有 `store/parser` 函数第一个参数 `ctx context.Context`（`writer.go:127, 168, 217, 333`）。MCP SDK 自带 EOF→ctx cancel | 100% |

**结论** ✅ **70% 属实**。**不**采纳"全局 `log.SetOutput`"反模式。

---

### §三 Prompts 三个核心（kron-implement / kron-drift-check / kron-plan）

| Gemini 描述 | Kron plan 现状 | 属实度 |
|---|---|---|
| "强烈建议做 Prompts" | ✅ [`plan-2026-10-13-mcp-completeness.md` §3.1](./plan-2026-10-13-mcp-completeness.md) **已拍板**——`kron-review` + `kron-implement` + `kron-drift-check`（3 个，工作量 ~1 天） | 100% 方向对 |
| Server 端预拼装 intent + 锚点 | ✅ 属实——`LoadAllCached` + `kron_impact` + `kron_assume_get` 已有的拼装逻辑可复用 | 100% |
| 150 行代码 | ✅ 量级属实 | 100% |
| `kron-plan`（从散乱需求拆解意图树） | ⚠️ **不**在 plan §3.1 列表。Kron 倾向**先**让 AI 走 `kron_assume_check` / `kron_impact` / `kron_lint` 几个 tool **自己**拼拆解流程，**Prompts 只固化高频模板**（review / implement / drift-check） | 不一致 |

**结论** ✅ **80% 属实**。`kron-plan` Prompts **不**在 Kron plan 列表（理由：AI 自己会用 `kron_lint` + `kron_tree` 拼拆解流程；硬塞 prompt 反而限制 AI 灵活度）。

---

## 二、净属实度总结

| 部分 | 属实度 | Kron 现状 |
|---|---|---|
| §一·1 "不需要 Daemon" | ✅ 100% | 已符合架构 |
| §二·1 缓存策略 | ⚠️ 30% | **MCP 层已做**；`store` 层故意**不**做（铁律） |
| §二·2 原子写入 | ✅ 100% 方向对 / 0% 待做 | **已实装** |
| §二·3 roots 协议 | ⚠️ 30% | **走 `projects.json::current_project`**（已拍板） |
| §二·4 stdio 隔离 | ⚠️ 70% | `io.Writer` 注入已做；**不**采纳 `log.SetOutput` 反模式 |
| §三 Prompts | ✅ 80% | `kron-review` / `kron-implement` / `kron-drift-check` plan §3.1 拍板待 PR |

---

## 三、**正确的建议**（Kron 架构内可推进的）

> 排序：按"价值 / 成本比"+ 与已拍板 RFC/plan 一致性。**不催动手**——决策权在人。

### 建议 1 ✅ `kron_assume_get` MCP 工具 — `plan §4`

**对应 Gemini 什么**：

- Gemini 建议"做 Prompts"——`kron_assume_get` 是 Prompts 的**数据依赖**（Prompts 拼装时需要按 ID 拿单条 assumption）
- 计划在 [`plan-2026-10-13-mcp-completeness.md` §4](./plan-2026-10-13-mcp-completeness.md)

**做什么**：

- `kron_assume_get(assumption_id) → model.Assumption`（**单条** assumption 读取）
- 与 `kron_assume_check`（批量子图 diff）配对：check = 批量子图诊断 / get = 单条精读
- 文件：`internal/assumption/Writer/Reader` 已支持 `GetByID`——只需 MCP 包装

**为什么对**：

- Prompts `kron-drift-check` 拼装时需要按 ID 拿 assumption 全文
- `kron_assume_get` 是 `kron_assume_check` 的**互补** tool
- 改动极小（~0.5 天），单 PR，走 [`docs/process/mcp-tool.md`](../process/mcp-tool.md)

**预估**：

- ~30 行 MCP handler + ~50 行 test
- 单文件 `cmd/kron/serve-mcp/handlers_assumption.go` 加一个 `kron_assume_get` case
- 不改 schema、不改 store API、不加 dep

**拍板建议**：✅ **采纳**——`plan §4` 已拍。

---

### 建议 2 ✅ 3 个核心 Prompts — `plan §3.1`

**对应 Gemini 什么**：

- Gemini 建议 "3 个核心 prompts"——Kron 拍板的 **3 个** 与 Gemini 建议**略有差异**（见下表）

| Gemini 建议 | Kron plan 拍板 | 差异 |
|---|---|---|
| `kron-implement` | `kron-implement` | ✅ 一致 |
| `kron-drift-check` | `kron-drift-check` | ✅ 一致 |
| `kron-plan`（拆解意图树） | **不**做 `kron-plan` | ⚠️ Kron **不**采纳——理由见下 |
| — | `kron-review`（review 模板） | ✅ Kron 额外加 |

**为什么不采纳 `kron-plan`**：

- Gemini 假设 AI "不会拆解意图树"——但 `kron_tree` + `kron_lint` tool 已有，AI 自己能用 1–2 个 tool call 拼出拆解流程
- Prompts 的价值是**固化高频模板**（review / implement / drift-check）；拆解**不是高频**（每次需求都不一样，模板硬塞反而限制 AI 灵活度）
- 用户已经在 system prompt / CLAUDE.md 写过 plan-first 流程

**做什么**：

3 个 Prompts 注册到 `cmd/kron/serve-mcp/tools_schema.go`：

1. **`kron-implement(slug)`**：读 `intents/<slug>.md` + 该 slug 所有 `// @kron:intent` 锚点 → 拼成"角色 + 内联上下文 + 强约束规则"塞给 AI
2. **`kron-drift-check(slug?)`**：跑 `kron_lint` → 拼"孤儿意图 / 失效锚点 / 状态陈旧"清单 → 让 AI 修复
3. **`kron-review(slug?)`**：跑 `kron_impact` + `kron_assume_check` → 拼"评审 diff 视角" → 让 AI 评审

**预估**：

- ~150 行 handler（3 × ~50 行 / prompt）
- 模板字符串 + 数据 fetch + JSON 返回
- 不动 store / parser

**拍板建议**：✅ **采纳**——`plan §3.1` 已拍。`kron-plan` **不**做。

---

### 建议 3 ⚠️ 源码锚点 mtime 缓存（**不**加 `fsnotify`） — `bench-2026-10-09.md §5`

**对应 Gemini 什么**：

- Gemini 建议"代码锚点用 `fsnotify` 后台监听"——Kron **不**采纳（理由：`fsnotify` 加新 top-level dep 走 explicit approval，[AGENTS.md §3 红线](../../AGENTS.md)）
- 替代：**mtime-only 缓存**（Gemini 阶段 A）

**做什么**：

- `internal/parser` 加 `mtimeIndex map[string]time.Time`（在**访问层**——`serve-mcp` 进程内，**不**进 `parser`）
- `kron_impact` / `kron_lint` 调用时：`os.Stat` 遍历源码树，mtime 没变 → 跳过 `ScanAnchors`；变了 → 重扫
- 与现有 `intentsDirMtime` 同款机制（`cache.go:147`）

**为什么对**：

- 几百文件项目 `os.Stat` 遍历 < 5ms
- 实际省 99% 的 `ScanAnchors` 重扫
- **零新 dep**——纯 stdlib
- **零 `fsnotify`**——避跨平台 watch 行为差异

**预估**：

- ~80 行 + test
- 改动文件：`cmd/kron/serve-mcp/` 1 个新文件 `anchor_cache.go`（与 `cache.go` 对偶）
- **不**动 `internal/parser`（铁律——parser 是无状态解析器）

**拍板建议**：⚠️ **暂缓**——`bench-2026-10-09.md` 没明确拍板；先观察 `cache.go` 的 `LoadAllCached` 实际命中率，再决定 anchor 缓存是否值得。**理由**：`LoadAllCached` 已经把 `.kron/intents/*.md` 全量读入内存；anchor 端每次工具调用**还要**现扫源码——瓶颈在 anchor 端。但**优先级 P3**（plan §8 第 4 项已标 P3）。

---

### 建议 4 ❌ `roots/list` MCP 协议 — **不**做

**对应 Gemini 什么**：

- Gemini 建议"接入 MCP `roots/list`"——Kron 拍板**不**做（[`RFC 2026-10-13-multi-project-registry.md` §2.8](../../rfc/2026-10-13-multi-project-registry.md)）

**为什么不做**：

- MCP `roots` 是**客户端临时状态**——客户端重启 / 重连即失
- GUI 用户**切项目**需要**持久化**当前项目指针
- `projects.json::current_project` 是文件级持久化，符合 GUI 多项目场景
- 双方案并存**徒增复杂度**（一个 `roots/list_changed` handler + 一个 `projects.json` IO）

**正确替代**（已拍板）：

1. `projects.json` 顶级加 `current_project?: string`（绝对路径）
2. `kron register` 注册后自动 `current_project = <path>`
3. `resolveMCPRoot` 3 级 fallback：`-CWD` > `current_project` > `os.Getwd()`
4. GUI 切项目时同时写 `current_project` + `last_opened`

**拍板建议**：✅ **已拍**——RFC 2026-10-13-multi-project-registry.md 实施时落地。

---

### 建议 5 ❌ 全局 `log.SetOutput(os.Stderr)` — **不**做（反模式）

**对应 Gemini 什么**：

- Gemini 建议"在 mcp / lsp 入口加 `log.SetOutput(os.Stderr)`"

**为什么反模式**：

- `log.SetOutput` 是**全局副作用**——单元测试**不能**用 `bytes.Buffer` 验证隔离
- Kron **已用** `io.Writer` 注入（`runInit(out, errOut io.Writer)` 模式）—— handler 接 writer，测试**直接**断言 `bytes.Buffer` 内容
- 全局 `log` 包调用**不可并发测试**（race detector 会报）

**正确做法**（**已做**）：

- `internal/` **不**用 `log` 包；所有日志走 `io.Writer` 注入
- `cmd/kron/serve-mcp/handlers_logging.go` 是 MCP `logging/message` 通知（**不**是 stderr 日志）
- `cmd/kron/cli/run*.go` 的 `errOut io.Writer` 由 `main.go` 注入（`os.Stderr`）

**拍板建议**：✅ **已正确**——**不**采纳 Gemini 反模式。

---

### 建议 6 ⚠️ `parent ctx` + SIGTERM 取消 — 现状核对

**对应 Gemini 什么**：

- Gemini 建议"`os.Stdin` EOF + `SIGTERM` / `SIGINT` 绑到根 `ctx`"

**现状**：

- `store` / `parser` 函数签**全部**接 `ctx context.Context`（`writer.go:127, 168, 217, 333`）
- MCP SDK（`go-sdk` v1.2+）自带 stdin EOF → `ctx.Done()`——**已**是默认行为
- LSP SDK（`go.lsp.dev/protocol`，v1.3+ 待加）同理

**问题**：

- **`fsnotify` watcher 还没做**——所以 Gemini 担心的"watcher 不退出"目前**不**存在
- 未来如果做 `fsnotify` 增量刷新（建议 3），需要 `ctx.Done()` 关 watcher goroutine——但建议 3 暂缓，**当前没需求**

**拍板建议**：✅ **现状已正确**——**不**动手。

---

## 四、与已拍板文件的对齐

| 拍板文件 | 本复审的态度 |
|---|---|
| [`docs/abstractDesign/architecture.md` §〇](../abstractDesign/architecture.md) | **严格遵守**——Gemini 建议里 `store` 层缓存 + `log.SetOutput` 都违反铁律，**不**采纳 |
| [`docs/rfc/2026-10-13-multi-project-registry.md`](../../rfc/2026-10-13-multi-project-registry.md) | `current_project` 拍板**覆盖** Gemini 的 `roots/list` 建议 |
| [`docs/mcp/plan-2026-10-13-mcp-completeness.md`](./plan-2026-10-13-mcp-completeness.md) | `§3.1 Prompts` + `§4 kron_assume_get` 与本复审一致 |
| [`docs/implementation/bench-2026-10-09.md`](../implementation/bench-2026-10-09.md) | `OPT-1 cache.go` 已落地，对应 Gemini §二·1 缓存建议——**已做** |

---

## 五、不在本复审范围

- ❌ 不催动手（决策权在人）
- ❌ 不写新 code（PR 决定后才写）
- ❌ 不重做已实装（`cache.go` / `writeFileAtomic` / `-CWD`）
- ❌ 不重述 RFC 已拍板（`projects.json::current_project`）

---

## 六、给决策者的清单

**采纳**（与已拍板 plan/RFC 一致）：

1. ✅ 建议 1 `kron_assume_get`——`plan §4`
2. ✅ 建议 2 3 个 Prompts（`kron-implement` / `kron-drift-check` / `kron-review`，**不**做 `kron-plan`）——`plan §3.1`
3. ✅ 建议 4 `projects.json::current_project` 替代 MCP `roots/list`——`RFC §2.7/§2.8`
4. ✅ 建议 5/6 现状已正确——`io.Writer` 注入 + `ctx` 取消，**不**改

**暂缓**（成本 / 收益不对等）：

5. ⚠️ 建议 3 源码锚点 mtime 缓存——P3，观察 `LoadAllCached` 命中率后再决定

**不采纳**（违反架构铁律或与拍板冲突）：

6. ❌ Gemini 建议的 `store` 层缓存（违反铁律 #5）
7. ❌ Gemini 建议的全局 `log.SetOutput`（反模式——`io.Writer` 注入已做）
8. ❌ Gemini 建议的 MCP `roots/list` 协议（与 RFC 拍板冲突）
9. ❌ Gemini 建议的 `kron-plan` Prompt（AI 灵活度优先，**不**固化拆解流程）
