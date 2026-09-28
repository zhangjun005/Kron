# Kron

> 基于 Git 的 AI 辅助开发**意图管理**系统。

Kron 把代码设计意图（Design Intent）存为**纯 Markdown 文件，放在你的仓库里**——无需 API 服务、无需守护进程、无需私有格式。AI 编程助手可以直接读写它们。一切差异就是 `git diff`。

---

## 痛点

AI 编程助手在清楚"为什么这么做、权衡了什么、放弃了什么"时表现最好。但现状是：

- 设计决策只存在于会议纪要、Slack 消息流或某人的脑子里；六个月后没人记得当时为什么拒绝方案 B。
- 代码里的 `// FIXME` / `// TODO` 只能描述"要做什么"，无法承载"为什么这样做"。
- 接手者（包括下一个 AI Agent）只能对着代码反推意图，推错了就埋雷。

长期下来就产生了 **Intent Debt**：意图丢失、假设漂移、权衡失效。

## Kron 解决了什么

1. **决策写完即遗忘** — 写在 Notion / Slack / 脑里,半年后没人记得为何拒绝方案 B。  
   → Kron 把决策落为 `.kron/intents/*.md`,跟代码同 PR 同 review,`git diff` 直接看。
2. **假设沉到代码里找不到** — 写代码时的前提(DAU ≤ 10K、Redis 99.9%)只在脑子里。  
   → frontmatter 的 `assumptions[]` 把假设显式化,带 `severity: hard | soft` 与 `expires_at`。
3. **改代码不知道影响范围** — 改一个 API,下游多少文件多少意图跟着炸。  
   → MCP `kron_impact` 反向查"改这个意图牵连哪些代码 + 哪些下游意图"。
4. **过期假设无人察觉** — "Redis 可用性 ≥ 99.9%" 已经过了一年,没人复审。  
   → MCP `kron_stale` 自动列出过期意图 + 已过 `expires_at` 的假设,CI 可拦门。

第 2、3、4 项是 Kron 跟 `// FIXME`、ADR、Notion、Cursor `codebase.md` 的实质差别——这些替代品都没有 `severity` 字段,也没有被 AI 主动消费的查询接口。

## 意图之间的关系

Kron **不**用字段表达关系(`parent` / `depends_on` 都不存在),只用两种**文件系统 + Markdown 原生**手段:

- **目录 = 层级归属**。`auth/jwt-sliding-window.md` 自动属于 `auth/` 模块;`auth/README.md` 是该模块的总览。
- **相对链接 = 横向依赖**。意图正文里写 `[令牌桶](../rate-limit/token-bucket.md)`,语义是"这条决策依赖 / 复用 / 引用另一条意图"。普通编辑器的 `Ctrl+单击` 就能跳转,Kron 的 `kron_impact` 工具也会扫这种链接来算影响范围。

完整规范见 [`docs/abstractDesign/intent-structure.md`](./docs/abstractDesign/intent-structure.md) §二、§四。

## Kron 是什么

一个把意图沉淀为**仓库内 Markdown 文件**的系统：

```
.kron/
├── intents/                   # 每个意图一个 .md（核心）
│   ├── 2026-09-19-001-storage-format.md
│   ├── 2026-09-19-002-id-scheme.md
│   └── ...
└── config.toml                # Kron 配置（可选）
```

每个意图文件是一份 Markdown 文档，带一个轻量的 YAML frontmatter：

```markdown
---
symbol: "auth.RefreshToken"
created_by: "@zhangjun005"
updated_at: "2026-09-19T22:30:00Z"
# 可选：边界假设结构化（见 docs/abstractDesign/intent-structure.md §三）
# assumptions:
#   - id: small-repo
#     text: 仓库规模在个人/小团队级别（<1k 条）
#     severity: hard
---

# 存储格式选型

> 选 Markdown + YAML frontmatter，零依赖、零迁移成本。

## 为什么
存储需要同时被人和 AI 编辑。选 Markdown + YAML frontmatter 的理由：
- 人类可直接 `git diff` 阅读，无需学新工具
- AI 可直接 prompt-context 读取，无需解析二进制

## 权衡
- **放弃了**：原生 SQLite 索引查询能力 —— 换来了零依赖、零迁移成本
- **代价**：大规模条目下需要全文搜索，不适合 >1万条 的仓库

> 边界假设写在 frontmatter 的 `assumptions` 字段（见 [intent-structure.md §三](./docs/abstractDesign/intent-structure.md#三边界假设-assumptions-字段)），不在正文重复。
```

代码侧用极轻量锚点反向引用：

```go
// @kron:intent 2026-09-19-001-storage-format
func ParseFrontmatter(raw []byte) (map[string]any, string, error) { ... }
```

就这样。**AI 读的文件和你读的一样。**

## Kron 不做什么

- 无后台守护进程、无文件监听、无同步引擎。
- 无双源持久化、无 mtime+hash 状态机冲突处理。
- 无私有格式。Markdown 本身就是 API。
- **不是**任务追踪器。任务只是意图落地时的派生钩子，不是独立实体。
- 无 Electron 应用。（未来可能基于 Tauri 做 GUI，但非必须。）

只要贡献者或 AI 能自信地编辑 Markdown 文件，那文件就是真相。

## 三个入口

Kron 暴露**三个互相独立的访问层**，对应三类用户。三层互不依赖：CLI 不知道 MCP 存在、MCP 不知道 IDE 存在、IDE 不知道 CLI 存在（[`docs/abstractDesign/architecture.md`](./docs/abstractDesign/architecture.md) §〇 铁律 #3）。

### CLI —— 给人 / CI

**最小集**，四个子命令，刻意不再扩展：

| 命令 | 用途 |
|---|---|
| `kron init` | 建立 `.kron/intents/` 骨架 |
| `kron add <slug>` | 脚手架一个新意图文件 |
| `kron lint` | 校验锚点 + frontmatter（**唯一 CI gate**） |
| `kron serve-mcp` | 启动 MCP server（给 AI） |

CLI 故意不做 `list` / `get` / `update` / `delete` / `restore`——查询类操作是 MCP 的事。详情见 [`docs/implementation/cli.md`](./docs/implementation/cli.md)。

### MCP —— 给 AI Agent

AI Agent 不走 CLI。它通过 stdio JSON-RPC 调用 MCP 工具。**v1 共 12 个**，分两组：

- **生命周期 8 个**：`kron_init` / `kron_add` / `kron_list` / `kron_get` / `kron_update` / `kron_delete`（软删）/ `kron_restore` / `kron_lint`
- **查询类 4 个**：`kron_assume_check`（列出某文件依赖的 hard 假设）/ `kron_impact`（修改某意图的影响范围）/ `kron_intent_density`（量化意图覆盖率）/ `kron_stale`（列出过期意图 + 已过期的假设）

后四个让 AI Agent 在编码循环里**主动**调用,而非被动 grep Markdown——这是它跟"AI 读 README.md"的差别。详情见 [`docs/implementation/mcp.md`](./docs/implementation/mcp.md) §2。

### IDE —— 给人 / AI 在编辑器内

> v1 不交付 IDE 插件 / LSP server 本体，但数据契约（frontmatter + 锚点格式）已按"未来 LSP 友好"设计。

预期 IDE 入口提供**三路触发**：

| 触发位置 | 元素 | Hover | Ctrl+单击 |
|---|---|---|---|
| 源码 | `// @kron:intent <slug>` | 弹出意图摘要 | 跳转打开 `.kron/intents/<slug>.md` |
| 意图 MD（横向） | `[@token-bucket](../path/to.md)` | 弹出被依赖意图摘要 | 跳转 |
| 意图 MD（纵向） | `[@auth](README.md)` 或自动推导 | 弹出父模块背景 | 跳转 |

**调用树 × 意图视图**——基于 `kron_impact` + `kron_intent_density` 的输出,Kron 暴露数据;画图是 IDE 的工作。详情见 [`docs/abstractDesign/view-call-tree-intent.md`](./docs/abstractDesign/view-call-tree-intent.md) 与 [`docs/implementation/ide-interaction.md`](./docs/implementation/ide-interaction.md)。

## 状态

仓库 `cmd/kron/` 与 `internal/` 现状即 Kron 当前完成度（看目录比看状态声明更准）。

## 参与贡献

格式和 CLI 设计决策在 [`docs/abstractDesign/`](./docs/abstractDesign/)。提 PR 前先读这些——格式是 Kron 赖以生存的东西，改动会直接影响到用户，因为他们拥有这些文件。

## License

MIT.
