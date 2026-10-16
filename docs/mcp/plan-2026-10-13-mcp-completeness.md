# MCP 完整性盘点 — 2026-10-13 复审

> **目的**：把"mcp 全了吗"对话的产物单独归档，避免与 `backlog.md` / `missing-capabilities.md` 混。
>
> **来源**：2026-10-13 复审 `cmd/kron/serve-mcp/` 17 个 tool + 9 个 `internal/<pkg>/` 能力对照 + MCP 1.x spec。
>
> **本文件不催人动手**——只记录现状 + 拍板建议。决策权在用户。

---

## 1 结论

| 维度 | 状态 |
|---|---|
| **功能覆盖**（17 tool vs 9 internal 包） | ✅ **全** |
| **协议层**（tools / resources / logging） | ✅ **全** |
| **协议层缺口**（prompts / roots） | ❌ **未做**，各有原因（见 §3） |
| **可加 tool**（`kron_assume_get`） | 🟡 **建议加**，未做（见 §4） |
| **多客户端共享配置**（`projects.json` 复用） | 🟡 **新发现**，值得开 RFC 拍板（见 §5） |

---

## 2 功能覆盖度 — 17 tool vs 9 internal 包

| `internal/<pkg>/` | 暴露的能力 | 对应 MCP tool |
|---|---|---|
| `model` | 类型 / 常量 / Caller | (无 runtime 能力) |
| `store` | Read / Write / MoveToTrash / Restore | `kron_init` (写 config) / `kron_add` / `kron_update` / `kron_delete` / `kron_restore` |
| `parser` | frontmatter / anchors / slug / template | (被引用, 无独立 tool) |
| `relations` | ReverseLinks / Prereqs / ReferencingIntents | `kron_impact` / `kron_get(include_relations=true)` / `kron_delete` (dependents) / `kron_assume_delete` (referencing_intents) |
| `lint` | 19 Rule + Fix suggestion | `kron_lint` |
| `view` | IntentTree | `kron_tree` |
| `assumption` | Read / Write registry | `kron_assume_*` (4 个) + `kron_assume_check` |
| `identity` | created_by 解析 | (内部用, 所有写操作自动填) |

**结论**：100% 覆盖。**没有"internal 改了一手但 MCP 没暴露"的情况**——G-5 / G-6 已合（2026-10-13）。

---

## 3 协议层缺口 — Prompts / Roots

### 3.1 Prompts

**MCP 协议**：server 可声明 `prompts/list` / `prompts/get`，client 可通过 `/prompt <name>` 触发（Claude Code / Cursor / Continue 等支持）。

**Kron 现状**：❌ **未声明**。

**候选模板**（来自 `missing-capabilities.md` §C-2）：

| 名称 | 用途 | arguments |
|---|---|---|
| `kron-review` | 评审所有 active intents | `status?: draft/active` |
| `kron-assumption-audit` | 检查 expired / unverified hard assumptions | — |
| `kron-stale-report` | 生成 stale intents 报告 | `days_threshold?: int` |

**价值**：
- AI 工具的 `/prompt` 路径走 Prompts（不是 Tools）
- 预置 prompt 比 AI 自由发挥更稳定
- v1.3 GUI / IDE 客户端可以快速接入"一键审计"按钮

**拍板建议**（2026-10-13）：
- **优先级**：中（不是高，AI 工具一般直接调 Tools）
- **实施时机**：v1.3 GUI 上线前
- **工作量**：~1 天
- **关联流程**：`docs/process/mcp-tool.md`（Prompts 算 protocol-level 资源，类似 tools/resources 走同一流程）

**2026-10-14 落地**（zhangjun005 拍板改名 + RFC [`2026-10-14-mcp-prompts.md`](../rfc/2026-10-14-mcp-prompts.md)）：
- 3 个真名 = `kron-implement` / `kron-drift-check` / `kron-review`（替代原 `kron-assumption-audit` / `kron-stale-report` / `kron-review`）
- `server.AddPrompt` × 3（go-sdk v1.1 已支持, 无新 dep）
- handler 内部走 `internal/` 直接（不调 `kron_*` tool, 避免 server 内部 RPC 自身）
- `prompts: { listChanged: true }` SDK 自动推断 + capabilities_test 锁定
- 单 PR，~320 行 handler + ~60 行 register + ~190 行 test

### 3.2 Roots

**MCP 协议**：client → server 方向（client 声明"我关心的 root 列表"，server 拉取）。不在 server 侧声明。

**Kron 现状**：❌ **未集成**。

**单 repo 场景的价值**：
- `roots/list` 返回 `~/my-app`，但 Kron 需要 `<repo>/.kron/intents/`
- 强行信任会跑错目录
- **结论**：Kron 作为单 repo 工具**不**需要 Roots 集成

**多项目 GUI 场景的价值**：
- 假设未来 Wails GUI 同时管理多个 project，每个 project 跑一个 `kron serve-mcp` 子进程
- 子进程可以拉 client roots，知道"我在这个 project 上下文中"
- **但**当前 RFC 拍板用 `projects.json`（文件共享）**不**用 Roots 协议——因为 Roots 协议栈里 server **不能**改 root，**只能**拉信息

**拍板建议**：
- **优先级**：低
- **实施时机**：**暂不**——`projects.json` 共享方案覆盖了多项目 GUI 场景
- **关联**：见 §5

---

## 4 待加 tool — `kron_assume_get` ✅ **完成 (2026-10-14)**

**问题**：当前 4 个 `kron_assume_*` tool（create / update / delete / list）+ 1 个 `kron_assume_check`（扫过期），**没有** `kron_assume_get` 按 ID 读单个 assumption。

**影响**：
- AI 想知道 assumption 的完整 body 只能调 `kron_assume_list`（只返回 summary，**没有** body）
- 或读引用它的 intent 的 `assumptions[]` 字段（**只**有 `id` + `text` + `severity`，**没有** body）

**复用基础**：
- `internal/assumption.Reader.Get(ctx, id)` **已存在**（reader.go:65-100）
- 返回 `*model.AssumptionFile{FM, Body, SourcePath}`，**含 body**
- 加一个 MCP handler + In/Out struct + test 即可

**拍板建议**：
- **优先级**：中（不紧急，但跟其他 4 个 assumption 工具对称）
- **实施时机**：本会话**之后**（用户原话"列入计划"）
- **工作量**：~0.5 天
- **关联流程**：`docs/process/mcp-tool.md`
- **新增 wire shape**（待拍板）：
  ```go
  type AssumptionGetInput struct {
      ID string `json:"id" jsonschema:"assumption id (must pass parser.ValidateSlug)"`
  }
  type AssumptionGetOutput struct {
      Assumption AssumptionSummary `json:"assumption" jsonschema:"full assumption (id, text, default_severity, status, body, ...)"`
  }
  ```
  或者直接暴露 `*model.AssumptionFile`（**含** body）。

**疑问**：`AssumptionSummary`（list 返回的）**不**含 body。`kron_assume_get` 要返回**含 body** 的全量。

候选方案：
- (a) 扩展 `AssumptionSummary` 加 `Body string` 字段（同时影响 list 输出，**不**推荐）
- (b) 新增 `AssumptionDetail` 类型（含 body），`kron_assume_get` 用它
- (c) `kron_assume_get` 直接返回 `AssumptionFile`（内部类型→wire 的 projection）

**待 PR 拍板**。

---

## 5 新发现 — `projects.json` 共享 (跨 client)

### 5.1 背景

[`docs/rfc/2026-10-13-multi-project-registry.md`](../../rfc/2026-10-13-multi-project-registry.md) 拍板了：

- `projects.json` 存 `Kron.exe` 同级目录
- 字段：`projects: [{ path, added_at, last_opened }]`
- 写者：`kron register <path>` / `kron init`（自动）/ GUI
- 读者：GUI 启动时

**明确**写了"❌ MCP 层管项目列表"（理由：stateless stdio server）。

### 5.2 新需求（2026-10-13 用户提）

> 之前我们提到给 GUI 加个 JSON，不如直接让这个 JSON 也让我们用？就放在 kron.exe 的同级路径？

**含义**：让 **CLI / 第三方工具 / AI 工具**也读这个 JSON。

**具体场景**：
- **AI 工具**（Claude Code 终端 / Cursor IDE）：用户问"我有哪些项目"，AI **不**走 MCP（因为 MCP 是单项目 stdio），而是**直接读**这个 JSON
- **第三方脚本**：CI / build 工具想知道"哪些路径是 Kron 项目"
- **CLI 子命令**：`kron list-projects`（RFC §2.6 已拍）

### 5.3 当前 RFC 状态（2026-10-13 合并到 multi-project-registry）

`current_project` 已合并到 [`docs/rfc/2026-10-13-multi-project-registry.md`](../../rfc/2026-10-13-multi-project-registry.md) §2.7 / §2.8：

- `projects.json` 顶级加 `current_project?: string`（绝对路径，可空）
- `kron register` 注册成功后自动 `current_project = <path>`
- `kron serve-mcp` 的 `resolveMCPRoot` 3 级 fallback：
  1. `-CWD <path>` flag（最高）
  2. `projects.json` 的 `current_project`
  3. `os.Getwd()`（最低）
- GUI 用户切换项目时同时写 `current_project` + `last_opened`（一次写回）

§5.4 之前的"开新 RFC 拍板"建议**作废**——已并入现有 RFC。

### 5.4 真正缺什么（2026-10-13 复审）

**没**拍的部分：

1. **第三方工具公共读取 API**（`internal/store/ReadProjects()` / `WriteProject()` 函数）
2. **AI 工具的接入路径**——MCP 不管，那 AI 工具**直接** stdlib `os.ReadFile(filepath.Dir(os.Executable()) + "/projects.json")` 读？还是经由 CLI `kron list-projects --json` 转一手？
3. **格式演进**——v1.3 字段够吗？未来要不要加 `last_lint` / `last_intent_count` / `version`（破坏性变更时 bump）

**拍板建议**（用户拍板前**不**动手）：

| 项 | 建议 |
|---|---|
| 公共读取 API | `internal/store/ReadProjects() ([]ProjectEntry, error)` + `WriteProject(entry)`；CLI 复用同一 API，不重复实现 |
| AI 工具接入 | 走 CLI：`kron list-projects --json` 输出 JSON（不**不**走 MCP，因为 stateless 假设） |
| 第三方脚本 | 走同一 CLI：`kron list-projects --json` / `kron register <path>` |
| 格式版本 | JSON 加 `version: 1` 字段；不兼容变更 bump `version` |
| 写权限 | **只** `kron register` / `kron init` / GUI 写；第三方**只**读 |

**实施时机**：与 multi-project-registry RFC **同周期**（v1.3+ release pipeline 阶段）。

---

## 6 跟现有文档的关系

| 现有文件 | 关系 |
|---|---|
| `backlog.md` | **不**重复——本文件是 backlog 中 G-* / C-* 之外的"复审产物" |
| `missing-capabilities.md` | §3 Prompts / Roots 的论证**同源**，但本文件给"mcp 全了吗"问题一个**直接答案** |
| `kron-implementation.md` | **不**重复——本文件不切片 17 个 tool，只评估覆盖度 |
| `protocol.md` | **不**重复——本文件不重述协议 |
| `../../rfc/2026-10-13-multi-project-registry.md` | §5 是**补充**——RFC 已拍 `current_project` + 3 级 fallback；本文件记录"还没拍的剩余部分" |

---

## 7 不在本文件范围

- ❌ 不催人动手（决策权在用户）
- ❌ 不写新 code（PR 决定后才写）
- ❌ 不重述 backlog 已有的 G-1 / G-2 / G-3 / G-7
- ❌ 不动多项目 GUI 拍板（见 RFC）

---

## 9 CLI 边界确认（2026-10-13 复审）

**问题**：CLI 子命令范围该多大？`init` / `add` / `lint` 之外还要不要加？

**原则**（[架构 §〇 铁律 #6](../abstractDesign/architecture.md)）：CLI 只暴露"CI gate / 批处理 / 简单操作"；查询类（list / get / update / delete / restore）**走 MCP**。

### 9.1 当前 CLI 现状

| 命令 | 状态 | 用途 |
|---|---|---|
| `kron init` | ✅ 已有 | 创建 `.kron/` 骨架（idempotent） |
| `kron add <slug>` | ✅ 已有 | scaffold 新 intent（写 frontmatter） |
| `kron lint` | ✅ 已有 | 跑 lint，退出码 = CI gate（0 clean / 1 error / 2 internal） |
| `kron help` | ✅ 已有 | 内置帮助 |
| `kron register` | 📋 RFC 拍板未实装 | 注册当前目录到 `projects.json` |
| `kron list-projects` | 📋 RFC §2.6 拍板未实装 | 读 `projects.json` 显示项目列表 |

**5 个命令 = CLI 全部**。

### 9.2 不在 CLI 的命令（架构铁律 #6）

| 命令 | 走哪 | 理由 |
|---|---|---|
| `kron list` | MCP-only | 查询类，AI 工具主用 |
| `kron get` | MCP-only | 查询类，AI 工具主用 |
| `kron update` | MCP-only | 写类，但**PATCH** 语义交互性弱，AI 主用 |
| `kron delete` | MCP-only | 危险操作，**不**给 shell 走；走 MCP 有 `dependents` 软警告 |
| `kron restore` | MCP-only | 同上 |

### 9.3 评估过的候选（用户拍板**不**做）

| 候选 | 想法 | 不做的理由 |
|---|---|---|
| `kron lint --fix` | 自动修某些 lint error（frontmatter 格式类） | lint 的价值是**诊断**，不是自动修；auto-fix 风险高 |
| `kron add` 交互模式 | 无参数跑交互 prompt（slug / kind / symbol） | 跑 CLI 说明**不想**交互；MCP `kron_add` 已覆盖 |
| `kron tree` 纯文本 | 把 MCP `kron_tree` 逻辑放到 CLI | 价值低——`kron lint` + 文本输出已够；GUI 树视图走 MCP |
| `kron list --format tree` | `kron lint` 树输出移出来 | 同上 |

### 9.4 结论

**CLI 范围到此为止**——`init` / `add` / `lint` / `register` / `list-projects` 5 个够用。

**理由**：
- `init` / `add` = **基本写操作**（批处理 + 脚本友好）
- `lint` = **CI gate**（退出码语义）
- `register` / `list-projects` = **多项目 GUI 支撑**（projects.json 共享）
- 查询 / PATCH / 危险操作 = **MCP 专属**（AI 工具主用 + 安全 + 软警告）

**不催**——`register` / `list-projects` 等多项目 RFC 实施时一起做。

---

## 8 触发 PR 的建议顺序

如果决定**动手**（按"价值 / 成本比"）：

1. **§4 `kron_assume_get`**（~0.5 天，单 PR，走 `mcp-tool.md`）
2. **§3.1 Prompts**（~1 天，单 PR，走 `mcp-tool.md`，含 `kron-review` 模板）
3. **§5.4 projects.json 剩余部分**（~1 天，**在** multi-project-registry RFC 实施时一起做：公共读取 API + `kron list-projects --json`）
4. **§3.2 Roots**（**不**做，`projects.json` 覆盖）
5. **§9 CLI 边界**（**不**做——5 个命令够）

不催——本文件**只**是记录。
