# RFC: 3 个 MCP Prompts 拍板 (`kron-implement` / `kron-drift-check` / `kron-review`)

| 字段 | 值 |
|---|---|
| **状态** | **已拍板 (2026-10-14) → 已实施 + 已归档 (2026-10-19)**：三个 prompt（`kron-implement` / `kron-drift-check` / `kron-review`）已随 `9e5e22b` 落地。本文件仅作历史决策记录。 |
| **作者** | AI assistant, 经 zhangjun005 委托 |
| **创建日期** | 2026-10-14 |
| **目标版本** | v1.3+ |
| **实施时机** | 与 [`2026-10-14-projects-json-schema-extension.md`](./2026-10-14-projects-json-schema-extension.md) 同步 (最后发布时再做) |
| **补充** | [`docs/mcp/missing-capabilities.md` §C-2](../../mcp/missing-capabilities.md) (候选模板 + 风险评估) + [`docs/mcp/plan-2026-10-13-mcp-completeness.md` §3.1](../../mcp/plan-2026-10-13-mcp-completeness.md) (协议层缺口) |
| **取代** | — (新议题) |

---

## 1 动机

[`docs/mcp/plan-2026-10-13-mcp-completeness.md` §3.1](../../mcp/plan-2026-10-13-mcp-completeness.md) 拍板 MCP 协议层 **Prompts** 能力**未声明**, 候选模板来自 `missing-capabilities.md §C-2`:

| 原候选名 | 拍板 |
|---|---|
| `kron-review` | ✅ 保留 |
| `kron-assumption-audit` | ❌ 改名 `kron-drift-check` (zhangjun005 2026-10-14) |
| `kron-stale-report` | ❌ 改名 `kron-implement` (zhangjun005 2026-10-14, 改用现有 tool 数据源) |
| (新增) | ✅ `kron-implement` / `kron-drift-check` |

**最终 3 个 Prompts** (zhangjun005 拍板): `kron-implement` / `kron-drift-check` / `kron-review`.

**为什么"现在做"** (vs plan §3.1 拍板"v1.3 GUI 上线前"):
- AI Agent 已在用 `kron serve-mcp` 做开发循环, Prompts 比 AI 自由发挥更稳定
- **`kron-implement`** 解决"AI 不知道从哪个 intent 开始实现新需求"问题 — `kron_add` 不会自动选 depends_on, AI 自由发挥**会**漏前置 intent
- **`kron-drift-check`** 解决"AI 改了代码但没更新 intent"问题 — `kron_stale` 数据但**不**告诉 AI 该改哪个
- **`kron-review`** 解决"PR review 时缺全景"问题 — 评审人**没** kron 视图, AI 自由发挥**会**漏 cross-intent 影响

**协议层 SDK 支持** (go-sdk v1.1 已支持, 无新 dep):
- `Server.AddPrompt(p *Prompt, h PromptHandler)` — 拍板
- `mcp.Prompt` / `mcp.PromptArgument` / `mcp.PromptMessage` — 类型已存在
- SDK 自动推断 capabilities (与 Tools 同样路径) — `prompts: { listChanged: true }` 出现在 `initialize` 响应

---

## 2 拍板

### 2.1 3 个 Prompts 真名 + arguments

| Prompt 名 | 用途 | arguments |
|---|---|---|
| `kron-implement` | AI 接到新需求时: 找"最匹配的现有 intent" + 它的 depends_on, 输出"先读哪几个、再写哪几个" | `feature: string` (必填) |
| `kron-drift-check` | AI 改完代码后: 对比 `kron_stale` 数据 + 改动文件, 输出"哪些 intent 该 update / 哪些 anchor 失修" | `days_threshold?: int` (选填, 默认 90) |
| `kron-review` | 评审 active intents 全景, 输出"哪些 status 改 / 哪些 frontmatter 漏字段 / 哪些 cross-link 缺" | `status?: string` (选填, 默认 "active") |

### 2.2 handler 模式 — 复用 `internal/` 数据

每个 Prompt handler 内部**调** `internal/store` / `internal/relations` / `internal/lint` / `internal/assumption` 拿数据, **再**拼 `mcp.PromptMessage[]` 返回. **不**调 `kron_*` tool (避免 server 内部 RPC 自身).

伪代码 (`handlers_prompts.go` 新建, ~80-120 行 / Prompt):

```go
func HandleKronImplementPrompt(
    ctx context.Context, _ *mcp.GetPromptRequest,
) (*mcp.GetPromptResult, error) {
    args := _  // SDK 不传 args, 走自定义 params
    feature := args["feature"]   // "" → IsError=true

    root := repoRootFrom(ctx)
    all, err := LoadAllCached(ctx, root)
    if err != nil { return nil, err }

    // 1. 找 feature 关键词最匹配的 intent (朴素: 字符串匹配 slug + frontmatter.symbol)
    matches := matchIntentsByKeyword(all, feature)

    // 2. 每个 match 算 prerequisites (走 relations.Prerequisites)
    var bullets []string
    for _, m := range matches {
        prereqs := relations.Prerequisites(all, m.Slug)
        bullets = append(bullets,
            fmt.Sprintf("- intent: %s — read first: %v, then implement at .kron/intents/%s.md",
                m.Slug, prereqs, m.Slug))
    }

    return &mcp.GetPromptResult{
        Description: "Pick the right intent to implement a new feature",
        Messages: []mcp.PromptMessage{
            mcp.NewUserMessage(
                fmt.Sprintf("## User Request\n%s\n\n## Matched Intents\n%s\n\n## Action\n1. Read each matched intent + its prerequisites\n2. Decide if any prerequisite is missing → kron_add it first\n3. Use kron_update (PATCH) to apply changes",
                    feature, strings.Join(bullets, "\n"))),
        },
    }, nil
}
```

**关键设计**:

1. **`feature` 必填** (空 → IsError=true) — 与其他 2 个 prompt 的 `?` 形成对比
2. **复用 `LoadAllCached`** (cache.go 已就位) — 避免重复 I/O
3. **复用 `relations.Prerequisites`** (internal/relations 已就位) — 不下沉
4. **不调 `kron_*` tool** — 内部用 `internal/` 直接调, 避免 server 内部 RPC 自身 (架构清晰)
5. **`PromptMessage.Role` = "user"** (MCP spec: prompt 模板渲染成 user message, 让 client LLM 接着回)

### 2.3 协议层 — capabilities 自动推断

go-sdk v1.1 `ServerOptions` **无** `Capabilities` 字段 (与 [`backlog.md` G-4 §4](../../mcp/backlog.md) 拍板一致). 走 SDK 自动推断:

- 注册 `AddPrompt` → `ServerCapabilities.Prompts = &PromptCapabilities{ListChanged: true}` (SDK 内部)
- `initialize` 响应**自动**包含 `prompts: { listChanged: true }`
- **不需要**显式 capabilities 声明 (与 Tools 同样的推断路径)

**测试** (`capabilities_test.go` 增 1 case):
- `initialize` 响应**必须**有 `prompts: { listChanged: true }` 字段
- **不能**回归 (若将来 SDK 升级改行为, 立刻发现)

### 2.4 wire 协议

`prompts/list` (client 调) 响应:

```json
{
  "jsonrpc": "2.0", "id": 1, "result": {
    "prompts": [
      { "name": "kron-implement", "description": "Pick the right intent to implement a new feature",
        "arguments": [{"name": "feature", "description": "...", "required": true}] },
      { "name": "kron-drift-check", "description": "...", "arguments": [...] },
      { "name": "kron-review", "description": "...", "arguments": [...] }
    ]
  }
}
```

`prompts/get` (client 调) 响应:

```json
{
  "jsonrpc": "2.0", "id": 2, "result": {
    "description": "...",
    "messages": [
      {"role": "user", "content": {"type": "text", "text": "## User Request\n..."}}
    ]
  }
}
```

### 2.5 错误处理

| 错误来源 | handler 怎么包 | SDK 怎么呈现 |
|---|---|---|
| `feature` 为空 (`kron-implement`) | `fmt.Errorf("kron-implement: feature argument is required")` | error → IsError=true |
| `repoRootFrom(ctx)` 失败 | `fmt.Errorf("kron-implement: %w", err)` | 同上 |
| `LoadAllCached` 失败 | `fmt.Errorf("kron-implement: load: %w", err)` | 同上 |
| `matchIntentsByKeyword` 0 匹配 | **不**报错, 消息体说"no matched intent; consider kron_add a new one" | 仍返回 messages (mcp spec: 0 match 是合法结果) |
| `days_threshold` 解析失败 (`kron-drift-check`) | 走 default (90) | 不报错 |
| `status` 解析失败 (`kron-review`) | 走 default ("active") | 不报错 |

**关键**: `kron-implement` 的 `feature` 是**唯一必填**参数 (其他 2 个都有 default), 缺失 → 报错 (避免 AI 拿到空模板).

### 2.6 命名冲突检查

`mcp.AddPrompt` 注册的 `name` 必须**不**与 `mcp.AddTool` 的 `name` 冲突. 当前 17 个 tool 全部 `kron_*`, Prompts 也用 `kron_*` 命名. **MCP spec**: tools 和 prompts 是**独立**命名空间 (协议层), 不会冲突.

但**用户视角**可能混淆 ("`/prompt kron_review`" vs "`kron_review`" tool 提示). 选**不带下划线**形式 (`kron-implement`) — 比 `kron_implement` 更像"快捷命令名"而非"工具名".

---

## 3 不做的事 (明确)

| 项 | 状态 | 理由 |
|---|---|---|
| ❌ `kron-assumption-audit` prompt | **不**做 | 改名 `kron-drift-check` (覆盖更广, 不止 assumption) |
| ❌ `kron-stale-report` prompt | **不**做 | 改名 `kron-implement` (覆盖更广, 不止 stale) |
| ❌ Prompts 调 `kron_*` tool (server 内部 RPC) | **不**做 | 调 `internal/` 直接, 避免 server 内部 RPC 自身 |
| ❌ `logging/message` 集成到 prompts 渲染 | **不**做 | logging 是独立 capability, 与 Prompts 解耦 |
| ❌ 复杂 LLM 链 (multi-turn) | **不**做 | Prompts 是**单轮**模板, 不维护对话历史 |
| ❌ `sampling` (server 反向让 LLM 生成) | **不**做 | MCP spec 能力, Kron 不参与 (YAGNI) |
| ❌ Resources template (`kron://prompts/...`) | **不**做 | Prompts 走 `prompts/get`, 不走 resources/read |
| ❌ Prompts 国际化 (i18n) | **不**做 | 输出英文, 跟现有 tool description 一致 |
| ❌ 新 Go dep (升级 SDK / 加 prompts lib) | **不**做 | go-sdk v1.1 已支持, 不加 dep |
| ❌ 改 `kron-implement` 必填为 `?` | **不**做 | 必填, 避免 AI 拿到空模板; 与其他 2 个 prompt 的 optional 形成对比 |
| ❌ Prompts 写操作 | **不**做 | 渲染 message 不写盘, 永远是 read-only |

---

## 4 实施清单 (与 PR 1 `kron_assume_get` 同周期)

| # | 改动 | 仓库位置 | 工作量 |
|---|---|---|---|
| 1 | `cmd/kron/serve-mcp/handlers_prompts.go` (新) 3 个 handler + `matchIntentsByKeyword` helper (~300 行) | `cmd/kron/serve-mcp/` (新建) | 1 天 |
| 2 | `cmd/kron/serve-mcp/serve_mcp.go` 注册: `server.AddPrompt(p, h) × 3` (在 `registerTools` 后) | `cmd/kron/serve-mcp/serve_mcp.go` (修改) | 0.2 天 |
| 3 | `cmd/kron/serve-mcp/prompts_test.go` (新) handler 级测试: 必填校验 / 0 匹配 / 渲染输出快照 (golden file) | `cmd/kron/serve-mcp/` (新建) | 0.5 天 |
| 4 | `cmd/kron/serve-mcp/capabilities_test.go` 增 1 case: `initialize` 响应含 `prompts: { listChanged: true }` | `cmd/kron/serve-mcp/` (修改) | 0.1 天 |
| 5 | `docs/mcp/kron-implementation.md` §一 文件清单 + `handlers_prompts.go` / `prompts_test.go` 行; §四 handler 列表追加 §4.3.8/9/10 | `docs/mcp/kron-implementation.md` (修改) | 0.2 天 |
| 6 | `docs/mcp/plan-2026-10-13-mcp-completeness.md` §3.1 标 ✅ 完成 (2026-10-14) | `docs/mcp/` (修改) | 0.05 天 |
| 7 | `docs/mcp/missing-capabilities.md` §C-2 标 ✅ 完成 (2026-10-14) | `docs/mcp/` (修改) | 0.05 天 |

**总改动 ~2.1 天**. 单 PR.

---

## 5 红线确认

| 红线 | 拍板 |
|---|---|
| 加新 top-level dep (`go.mod` `require` 块) | ❌ 不加 (go-sdk v1.1 已支持) |
| 加新 CLI 子命令 (init/add/lint/serve-mcp/serve-lsp 之外) | ❌ 不加 (Prompts 走 MCP) |
| `internal/` import `cmd/kron/` | ❌ 不让 |
| `cmd/kron/<sub>/` 互调 | ❌ 不让 |
| 改 frontmatter / config / 8 import 边界 | ❌ 不改 |
| Prompts handler 调 `kron_*` tool (server 内部 RPC) | ❌ 不做 (调 `internal/` 直接) |

---

## 6 跟现有文档的关系

| 现有文件 | 关系 |
|---|---|
| [`docs/mcp/plan-2026-10-13-mcp-completeness.md` §3.1](../../mcp/plan-2026-10-13-mcp-completeness.md) | **本 RFC 是其落地**: §3.1 候选模板 → §2.1 真名 + arguments |
| [`docs/mcp/missing-capabilities.md` §C-2](../../mcp/missing-capabilities.md) | **不**重复论证; 本 RFC 引用其风险评估 + 实现伪码, §C-2 标完成 |
| [`docs/mcp/kron-implementation.md`](../../mcp/kron-implementation.md) | **本 RFC 实施时**追加 §4.3.8/9/10 + §一 文件清单 |
| [`docs/mcp/backlog.md`](../../mcp/backlog.md) | **不**重复 (本 RFC 走流程拍板, **不**走 backlog 跟踪) |
| [`docs/process/mcp-tool.md`](../../process/mcp-tool.md) | **流程 doc §2 引用 `docs/implementation/mcp.md` 不存在**; 本 RFC **不**走 mcp-tool.md 流程 (Prompts 走 RFC 拍板, 流程 doc **之后**改) |

---

## 7 待办 (v1.3+ release pipeline 阶段)

> ⚠️ **本节是"最后发布时再做"事项.**

- 实施清单 §4 全部 7 项.
- 同步修订 `docs/process/mcp-tool.md` §2: 补充 Prompts 走 RFC 拍板 + 创建 `docs/implementation/mcp.md` 或删除该引用 (留待用户拍板).
- 同步修订 `docs/mcp/plan-2026-10-13-mcp-completeness.md` §3.1 / `missing-capabilities.md` §C-2: 标完成.

---

## 8 用户拍板确认 (2026-10-14, zhangjun005)

- **3 个 Prompts 真名** = `kron-implement` / `kron-drift-check` / `kron-review` (替换原候选 `kron-assumption-audit` / `kron-stale-report` / `kron-review`)
- **实施时机** = "**先开 mini RFC 拍板, 再实施 PR 2**" (用户原话)
- **能力要求** = "**看上层是否需要**" (用户原话) → 拍板: 不加 `option=include_body` 类额外开关, 走**最小可用**
