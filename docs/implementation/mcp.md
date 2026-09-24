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

MCP 必须覆盖**增删改查 + 校验**完整意图生命周期。

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
| `IntentSummary` 结构 | `slug`, `symbol`, `status`, `updated_at` |

### `kron_get`

| 字段 | 值 |
|---|---|
| 入参 | `slug` (string, required) |
| 出参 | `{ intent: Intent }` |
| 错误码 | `ErrIntentNotFound` |

### `kron_update`

| 字段 | 值 |
|---|---|
| 入参 | `slug` (string, required)<br>`symbol` (string, optional)<br>`body` (string, optional)<br>`status` (string, optional) |
| 出参 | `{ ok: bool, path: string }` |
| 错误码 | `ErrIntentNotFound` |

### `kron_delete`

| 字段 | 值 |
|---|---|
| 入参 | `slug` (string, required) |
| 出参 | `{ ok: bool, trashed_path: string }` |
| 错误码 | `ErrIntentNotFound` |

**软删除机制**：
- 不真正删除文件，移动到 `.kron/.trash/<slug>.md`
- `.kron/.trash/` 与 `.kron/intents/` 平级

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

---

## 3 底层实现

- 每个工具底层映射到 `internal/store` / `internal/parser` / `internal/lint` 函数
- 签名带 `ctx context.Context`，caller 由 MCP server 注入为 `"mcp:<agent>"`（如 `"mcp:claude-3.7"`）
- JSON-RPC 序列化 / 反序列化留在 `cmd/kron/serve-mcp/`，不做下沉

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

## 5 GUI API 边界（Phase 2 预留）

GUI 不走 CLI 的 `--json` flag，而是通过**单独的 API 包**消费同一份 `internal/store` / `internal/parser`。

Phase 2 建议形态：

- **HTTP server 子命令**（`kron serve-gui`）启动轻量 HTTP 服务
- 端点对应 MCP 工具集：`POST /intents` / `GET /intents` / `PATCH /intents/:slug` / `DELETE /intents/:slug` / `GET /lint`
- 共享同一个 `internal/store` / `internal/parser` 函数，访问层之间通过 `internal/` 解耦

> **访问层之间禁止互相调用**（architecture.md §〇 铁律 #3）。GUI API 与 CLI/MCP 是平级访问层，各自独立 import `internal/`。

v1 不实现 `serve-gui`，但 `cmd/kron/cli` 的函数签名要为它留口（已是 `ctx` 透传形态）。
