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

**拍板建议**：
- **优先级**：中（不是高，AI 工具一般直接调 Tools）
- **实施时机**：v1.3 GUI 上线前
- **工作量**：~1 天
- **关联流程**：`docs/process/mcp-tool.md`（Prompts 算 protocol-level 资源，类似 tools/resources 走同一流程）

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

## 4 待加 tool — `kron_assume_get`

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

### 5.3 当前 RFC 缺什么

[`2026-10-10-multi-project-registry.md`](../../rfc/2026-10-13-multi-project-registry.md) **没**拍：

1. **第三方工具怎么读**（缺 `internal/store/ProjectRegistry` 的公共 API）
2. **AI 工具怎么读**（MCP 不管，那走什么？）
3. **格式演进**（v1.2 字段够吗？未来要不要加 `last_lint` / `last_intent_count` 等）

### 5.4 拍板建议

**开新 RFC** `docs/rfc/2026-10-13-projects-json-consumer.md`，拍板：

| 项 | 建议 |
|---|---|
| 公共读取 API | `internal/store.ReadProjects() ([]ProjectEntry, error)` + `WriteProject(path)` |
| 路径解析 | 沿用 RFC 拍板的 `filepath.Dir(os.Executable())` |
| AI 工具接入 | **不**走 MCP（stateless 假设）；**走** stdlib `os.ReadFile` + `encoding/json` 读 |
| 第三方脚本 | 提供 `kron list-projects --json` 输出（RFC §2.6 已拍但**没**说 JSON 格式） |
| 格式版本 | JSON 加 `version: 1` 字段，未来不兼容变更 bump version |
| 写权限 | **只** `kron register` / `kron init` 写；GUI 写 `last_opened`；第三方**只**读 |

**实施时机**：与 multi-project-registry RFC **同周期**（v1.3+ release pipeline 阶段）。

---

## 6 跟现有文档的关系

| 现有文件 | 关系 |
|---|---|
| `backlog.md` | **不**重复——本文件是 backlog 中 G-* / C-* 之外的"复审产物" |
| `missing-capabilities.md` | §3 Prompts / Roots 的论证**同源**，但本文件给"mcp 全了吗"问题一个**直接答案** |
| `kron-implementation.md` | **不**重复——本文件不切片 17 个 tool，只评估覆盖度 |
| `protocol.md` | **不**重复——本文件不重述协议 |
| `../../rfc/2026-10-13-multi-project-registry.md` | §5 是**补充**——"projects.json 共享"是该 RFC 没拍的部分 |

---

## 7 不在本文件范围

- ❌ 不催人动手（决策权在用户）
- ❌ 不写新 code（PR 决定后才写）
- ❌ 不重述 backlog 已有的 G-1 / G-2 / G-3 / G-7
- ❌ 不动多项目 GUI 拍板（见 RFC）

---

## 8 触发 PR 的建议顺序

如果决定**动手**（按"价值 / 成本比"）：

1. **§4 `kron_assume_get`**（~0.5 天，单 PR，走 `mcp-tool.md`）
2. **§3.1 Prompts**（~1 天，单 PR，走 `mcp-tool.md`，含 `kron-review` 模板）
3. **§5 `projects.json` 共享 RFC**（开新 RFC 拍板，~0.5 天文档 + 后续 ~1 天 `internal/store` API）
4. **§3.2 Roots**（**不**做，`projects.json` 覆盖）

不催——本文件**只**是记录。
