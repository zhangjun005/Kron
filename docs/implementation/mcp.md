# MCP 工具实现参考

> 来源：`docs/abstractDesign/architecture.md` §1.2、§5.4.1、§5.4.2  
> 本文件是**实施建议**，不是架构真理。API 表面如有变更，请同步修改此处。

---

## 1 工具清单（v1）

| 工具 | 类别 | 源码位置 |
|---|---|---|
| `kron_init` | 初始化 | `cmd/kron/serve-mcp/` |
| `kron_add` | 增 | `cmd/kron/serve-mcp/` |
| `kron_list` | 查 | `cmd/kron/serve-mcp/` |
| `kron_get` | 查 | `cmd/kron/serve-mcp/` |
| `kron_update` | 改 | `cmd/kron/serve-mcp/` |
| `kron_delete` | 软删 | `cmd/kron/serve-mcp/` |
| `kron_restore` | 恢复 | `cmd/kron/serve-mcp/` |
| `kron_lint` | 校验 | `cmd/kron/serve-mcp/` |
| `kron_assume_check` | 主动校验 | `cmd/kron/serve-mcp/` |
| `kron_impact` | 影响分析 | `cmd/kron/serve-mcp/` |
| `kron_intent_density` | 量化审计 | `cmd/kron/serve-mcp/` |
| `kron_stale` | 时效审计 | `cmd/kron/serve-mcp/` |

MCP 必须覆盖**增删改查 + 校验**完整意图生命周期。v1 扩展 4 个工具（`kron_assume_check` / `kron_impact` / `kron_intent_density` / `kron_stale`）让 AI Agent **主动**消费意图，而不是被动 grep Markdown。

> **新增 MCP 工具的流程**：见 [`docs/process/mcp-tool.md`](../process/mcp-tool.md)。

---

## 2 工具契约（v1）

### `kron_init`

| 字段 | 值 |
|---|---|
| 入参 | 无 |
| 出参 | `{ ok: bool, intents_dir: string }` |
| 错误码 | `ErrIntentExists`（已初始化）|

### `kron_add`

| 字段 | 值 |
|---|---|
| 入参 | `slug` (string, required)<br>`symbol` (string, optional)<br>`why` (string, optional) |
| 出参 | `{ ok: bool, path: string }` |
| 错误码 | `ErrSlugInvalid`（slug 格式不符）<br>`ErrIntentExists`（已存在）|

### `kron_list`

| 字段 | 值 |
|---|---|
| 入参 | `prefix` (string, optional) |
| 出参 | `{ intents: IntentSummary[] }` |
| `IntentSummary` 结构 | `slug`, `symbol`, `status`, `updated_at`, `assumes` (optional) |

### `kron_get`

| 字段 | 值 |
|---|---|
| 入参 | `slug` (string, required) |
| 出参 | `{ intent: Intent }` |
| `Intent` 结构 | `slug`, `frontmatter` (含 `symbol` / `created_by` / `updated_at` / `reviewers` / `status` / `assumptions` / **`references`** / **`depends_on`**), `body`, `source_path` |
| 错误码 | `ErrIntentNotFound` |

> **v1.2 变更**：`frontmatter` 新增 `references` 和 `depends_on` 两个字段。详见 [RFC 2026-10-03-frontmatter-references](../rfc/2026-10-03-frontmatter-references.md)。

### `kron_update`

| 字段 | 值 |
|---|---|
| 入参 | `slug` (string, required)<br>`symbol` (string, optional)<br>`body` (string, optional)<br>`status` (string, optional)<br>**`references`** (string[], optional, v1.2+)<br>**`depends_on`** (string[], optional, v1.2+) |
| 出参 | `{ ok: bool, path: string }` |
| 错误码 | `ErrIntentNotFound` / `ErrFrontmatterInvalid` (含 self-reference / invalid slug) |

> **PATCH 语义**：`references` / `depends_on` 与 `reviewers` 行为一致——`null`/`absent` = no change；`[]` = clear；`[...]` = replace。

### `kron_delete`

| 字段 | 值 |
|---|---|
| 入参 | `slug` (string, required) |
| 出参 | `{ ok: bool, trashed_path: string, dependents: [slug] }` |
| 错误码 | `ErrIntentNotFound` |

**软删除机制**：
- 不真正删除文件，移动到 `.kron/.trash/<slug>.md`
- `.kron/.trash/` 与 `.kron/intents/` 平级

> **v1.2 变更**：`dependents` 列出删除前**所有**依赖此 intent 的其他 intent slug（来自它们的 `depends_on` 字段）。**soft warn，不强制 block**——参见 RFC §5.3。

### `kron_restore`

| 字段 | 值 |
|---|---|
| 入参 | `slug` (string, required) |
| 出参 | `{ ok: bool, path: string }` |
| 错误码 | `ErrIntentNotFound` |

**恢复机制**：
- 从 `.kron/.trash/<slug>.md` 移回 `.kron/intents/<slug>.md`
- 硬删除（彻底清 `.trash/`）留给用户手动 `git rm` 或后续 `kron gc`（v1 不实现）

### `kron_lint`

| 字段 | 值 |
|---|---|
| 入参 | 无 |
| 出参 | `{ passed: bool, errors: string[] }` |

### `kron_assume_check`

| 字段 | 值 |
|---|---|
| 入参 | `file_path` (string, optional) |
| 出参 | `{ warnings: [{intent_slug, assumption_id, severity, text}] }` |
| 错误码 | 无 |

**说明**：读 `.kron/intents/*.md` 的 `assumptions[]`，对锚点指定的意图列出"该 `file_path` 依赖的 hard 假设清单"。
v1 简化：仅列清单，不做 diff 对比（留 TODO）。
**触发场景**：AI Agent 改完代码、准备 commit 前自动调——把"AI 主动校验假设"从口头承诺变成可执行能力。

### `kron_impact`

| 字段 | 值 |
|---|---|
| 入参 | `slug` (string, required) |
| 出参 | `{ intent: IntentSummary, incoming_anchors: [Anchor], references: [string], prerequisites: [string] }` |
| 错误码 | `ErrIntentNotFound` |

**说明**：
- `incoming_anchors` 是反向 anchor 扫描结果（哪些源文件依赖这个意图）；
- **`references`（v1.2+）**：反向软链接视图——列出**所有**在 `references` 中包含本 intent slug 的其他 intent；
- **`prerequisites`（v1.2+）**：本 intent 的前置依赖——`depends_on` 显式优先 ∪ symbol 推断（被显式覆盖的 symbol 推断不再重复出现）。

**触发场景**：AI Agent 准备修改某个意图前自动调——把"意图 × 调用树"视图的数据基础变成可执行 API。

### `kron_intent_density`

| 字段 | 值 |
|---|---|
| 入参 | 无 |
| 出参 | `{ total_intents: int, total_anchors: int, coverage: { intents_with_anchors: int, intents_without_anchors: [slug] }, files_without_intent: [path] }` |
| 错误码 | 无 |

**说明**：`files_without_intent` 是行数 > 50 且 0 anchor 的源文件——意图盲区。
**触发场景**：CI / IDE 侧栏——把"意图覆盖率"从感觉变成可量化指标。

### `kron_stale`

| 字段 | 值 |
|---|---|
| 入参 | `days_threshold` (int, optional, 默认 90) |
| 出参 | `{ superseded_candidates: [slug], expired_assumptions: [{slug, assumption_id, expires_at}] }` |
| 错误码 | 无 |

**说明**：`superseded_candidates` 是 `status=active` 但 `updated_at > days_threshold` 的意图；
`expired_assumptions` 是已过 `expires_at` 但尚未 superseded 的硬假设。
**触发场景**：CI / 定时任务——把"假设失效"从人工巡检变成自动告警。

---

## 3 底层实现

- 每个工具底层映射到 `internal/store` / `internal/parser` / `internal/lint` 函数
- 签名带 `ctx context.Context`，caller 由 MCP server 注入为 `"mcp:<agent>"`（如 `"mcp:claude-3.7"`）
- JSON-RPC 序列化 / 反序列化留在 `cmd/kron/serve-mcp/`，不做下沉
- v1 扩展的 4 个工具（`kron_assume_check` / `kron_impact` / `kron_intent_density` / `kron_stale`）
  是 `internal/store` + `internal/parser` 现成能力的**组合调用**，**不**下沉到新包；
  当 CLI / IDE 也需要同等 lint 输出时再考虑下沉 `internal/lint/`（见 [architecture.md §〇·五·5](../abstractDesign/architecture.md)）

---

## 4 协议与部署

| 项目 | 选型 |
|---|---|
| 协议 | stdio JSON-RPC 2.0 |
| 启动方式 | `kron serve-mcp` 子命令（cobra） |
| 长连接 | 是（响应 keepalive） |
| 失败语义 | JSON-RPC error code |
| 生命周期 | MCP server 进程寿命 |

---

## 5 GUI API 边界（设计性预留接口）

GUI 不走 CLI 的 `--json` flag，而是通过**单独的 API 包**消费同一份 `internal/store` / `internal/parser`。

建议形态：

- **HTTP server 子命令**（`kron serve-gui`）启动轻量 HTTP 服务
- 端点对应 MCP 工具集：`POST /intents` / `GET /intents` / `PATCH /intents/:slug` / `DELETE /intents/:slug` / `GET /lint`
- 共享同一个 `internal/store` / `internal/parser` 函数，访问层之间通过 `internal/` 解耦

> **访问层之间禁止互相调用**（architecture.md §〇 铁律 #3）。GUI API 与 CLI/MCP 是平级访问层，各自独立 import `internal/`。

v1 不实现 `serve-gui`，但 `cmd/kron/cli` 的函数签名要为它留口（已是 `ctx` 透传形态）。
