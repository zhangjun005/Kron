# Kron 是怎么工作的（实景）

> 目的：用一段真实的代码、一个真实的仓库状态，展示 Kron **作为工具是什么**——它提供哪些功能、对谁有用、典型工作流长什么样。  
> **本文件不声明任何功能的"实现状态"**——你读它就信它是设计意图；想看落地情况请看仓库 `cmd/kron/*` 与 `internal/*` 当前代码。  
> 与 [`README.md`](../../README.md) 的关系：README 是 60 秒概览；本文档是 10 分钟实景。  
> 与 [`docs/abstractDesign/architecture.md`](abstractDesign/architecture.md) 的关系：那是**架构真理**，讲"应当如此"；本文档不重复规则，只看运行中的 Kron 长什么样。

---

## 1 一个真实场景:AI 写了一周代码,你接手排查

设你今天刚加入团队分支。**上周 AI 编码助手一口气写完了一个 auth 模块**——几天内提交了十几个新文件、几十处函数、几百行注释。你需要在它**自信满满的输出**里找出:① 哪些决策值得 review;② 哪些假设已经悄悄过时;③ 下游改个 API 会不会炸到这些新写的代码。

```bash
git checkout feature/ai-auth-week
git log --oneline -20            # AI 这周的 commit 密度高得反常
```

`ls` 后你会看见：

```
your-project/
├── .kron/                                ← Kron 目录
│   ├── config.toml                       ← 一行:intents_dir = ".kron/intents"
│   └── intents/                          ← 每个意图一个 .md
│       ├── 2026-09-20-auth-refresh-token.md
│       ├── 2026-09-20-auth-rotation.md
│       ├── 2026-09-21-rate-limit.md
│       ├── 2026-09-22-token-bucket.md
│       ├── 2026-09-23-soft-delete.md
│       └── ...
└── internal/auth/
    ├── refresh.go                        ← 你要 review 的代码
    ├── rotate.go
    ├── ...
```

打开 `internal/auth/refresh.go`：

```go
package auth

import "time"

// @kron:intent auth-refresh-token
func NewRefreshToken(userID string) (string, error) {
    // ... 4 字节随机 ...
}
```

注释 `// @kron:intent auth-refresh-token` 是**锚点**——一个**轻量反向引用**。锚点本身不是 Kron 的核心数据，只是把代码符号钩到意图文件。

---

## 2 跟随锚点:哪一行指向哪个意图

打开 `internal/auth/refresh.go`,你看到几个 `// @kron:intent <slug>` 锚点。注释 `// @kron:intent auth-refresh-token` 是**锚点**——一个**轻量反向引用**。锚点本身不是 Kron 的核心数据,只是把代码符号钩到意图文件。

面对一周 AI 写下的代码量,你不会逐个 `cat` `.md`——你会问 AI:**"这块代码依赖了哪些假设?过期了没?"**

```
人类: 这文件依赖哪些 hard 假设? 有过期的吗?
AI:    我帮你跑了 kron_assume_check + kron_stale:
       auth-refresh-token:
         - hard "single-issuer" 还 active
         - soft "redis-availability" 还 active
         - hard "daus-under-10k" 还 active
         - soft "clock-skew-30s" 还 active(还剩 60 天)
       auth-rotation:
         - hard "rotate-on-password-change" 还 active
       rate-limit:
         ⚠ hard "no-burst-protection" 已过期(expires_at: 2026-09-01)
       建议:先补上 burst protection 的新意图,或把这个 hard 改 soft。
```

这是 §3.2 提到的 **`kron_assume_check` + `kron_stale`** 在实际场景里的用法——AI 主动 query 假设清单 + 主动告警过期,人类不需要挨个翻文件。

```markdown
---
symbol:
  - "auth.NewRefreshToken"
created_by: "@zhangjun005"
updated_at: "2026-09-22T10:00:00Z"
reviewers:
  - "@alice"
status: "active"

# assumptions 写在 frontmatter 里，是正文"边界假设"的结构化表达
# 正文"边界假设"可保留为纯自然语言补充，或删除以避免重复
assumptions:
  - id: single-issuer
    text: 全系统只有单一签发方，不需要 RS256 多密钥支持
    severity: hard
    expires_at: "2026-12-31"
  - id: daus-under-10k
    text: 当前 DAU ≤ 10K，黑名单放 Redis 无压力
    severity: hard
  - id: redis-availability
    text: Redis 可用性 ≥ 99.9%，Redis 挂了则 token 吊销失效（业务可接受）
    severity: soft
    expires_at: "2027-06-01"
  - id: clock-skew-30s
    text: 客户端时钟偏差 ≤ 30 秒，否则 exp 判断误差导致误踢用户
    severity: hard
---

# Refresh Token 实现选型

> 用 4 字节 random + 30 天过期；不上 JWT。

## 为什么
- 服务端需主动吊销——JWT 做不到
- 4 字节足够区分 ~4B 个活跃 token（业务天花板 < 1M）

## Trade-offs
- **放弃**：跨服务 token 共享（违反"吊销立即全局生效"诉求）
- **代价**：每次请求一次 Redis 查询
```

> 这是其中一份意图文件的 raw 内容——AI 通过 MCP 拉到的就是这个形状,字段名严格来自 [`internal/model/intent.go`](../internal/model/intent.go) 的 `Frontmatter` 结构。人和 AI 读的是同一份数据。

---

## 3 Kron 提供哪些能力

> **本节只列功能，不标完成度。**  
> 一个能力"在 Kron 的设计里"是"被期望提供"，跟它在当前仓库里"已落地可运行"是两回事——后者请看仓库当前 main 分支的 `cmd/kron/cli/` 与 `cmd/kron/serve-mcp/`，或 [`internal/model/intent.go`](../internal/model/intent.go)（这是当前唯一一份真实代码示例，字段名严格来自 `model.Frontmatter`）。

Kron 暴露**三个互相独立的访问层**，对应三类用户。**这三层互不依赖**：CLI 不知道 MCP 存在、MCP 不知道 IDE 存在、IDE 不知道 CLI 存在（[`architecture.md`](../abstractDesign/architecture.md) §〇 铁律 #3）。

### 3.1 CLI —— 给人 / CI 用的最小集

CLI 是 v1 唯一已稳定的访问层。**只四个子命令**（最小集，不扩展）：

| 命令 | 用途 | 典型场景 |
|---|---|---|
| `kron init` | 创建 `.kron/intents/` + 默认 `config.toml` | 新仓库第一次用 Kron |
| `kron add <slug>` | 脚手架一个新意图文件（写默认 frontmatter 骨架） | 写新意图前的骨架 |
| `kron lint` | 扫描仓库校验锚点 + frontmatter；**唯一 CI 强制命令** | `kron lint` 在 CI gate 强制跑 |
| `kron serve-mcp` | 启动 MCP stdio server 给 AI 用 | 见 §3.2 |

**CLI 不实现**：`list` / `get` / `update` / `delete` / `restore` / `assume_check` / `impact` / `intent_density` / `stale` —— 查询类操作是 MCP 的事,不是 CLI 的事。这是有意为之——CLI 故意小。

完整契约见 [`docs/implementation/cli.md`](implementation/cli.md)。

### 3.2 MCP —— 给 AI Agent 的可 query schema

AI Agent **不走 CLI**——它通过 stdio JSON-RPC 调用 MCP 工具。v1 工具集**共 12 个**，分三组：

**生命周期 8 个**：增删改查 + 软删/恢复 + 校验

| 工具 | 一句话 |
|---|---|
| `kron_init` | 在仓库建立 `.kron/` 骨架 |
| `kron_add` | 脚手架一个新意图文件 |
| `kron_list` | 列出所有意图（按 `status` / `symbol` 过滤） |
| `kron_get` | 取出指定 slug 的意图全文 + frontmatter |
| `kron_update` | 修改意图正文或 frontmatter（保留 git 历史） |
| `kron_delete` | 软删除：移到 `.kron/.trash/`（可恢复） |
| `kron_restore` | 从 `.trash/` 移回 `.kron/intents/` |
| `kron_lint` | 校验锚点 + frontmatter；与 `kron lint` 等价 |

**AI 主动消费 4 个**（这是 Kron 相对注释 / ADR / `codebase.md` 的差异化）：

| 工具 | 一句话 | 价值 |
|---|---|---|
| `kron_assume_check` | 列出某文件依赖的所有 `hard` 假设 | AI 改代码前**自动**收到假设清单 |
| `kron_impact` | 列出修改某意图会牵连哪些代码 + 下游意图 | 影响范围可视化 |
| `kron_intent_density` | 量化"意图覆盖率"（哪些意图无锚点 / 哪些核心文件无意图） | 意图盲区指标 |
| `kron_stale` | 找出过期意图 + 已过 `expires_at` 的假设 | 自动告警，不再人工巡检 |

完整契约见 [`docs/implementation/mcp.md`](implementation/mcp.md) §2。

### 3.3 IDE —— 给人 / AI 在编辑器内的可调用能力

> **范畴**：v1 不交付 IDE 插件 / LSP server 本体，但 Kron 的数据契约（frontmatter + 锚点格式）已经按"未来 LSP 友好"设计。

预期 IDE 入口提供**三路触发**（来自 [`implementation/ide-interaction.md`](implementation/ide-interaction.md)）：

| 触发位置 | 元素 | Hover | Ctrl+单击 |
|---|---|---|---|
| 源码 | `// @kron:intent jwt-sliding-window` | 弹出意图摘要 | 跳转打开 `.kron/intents/jwt-sliding-window.md` |
| 意图 MD（横向） | `[@token-bucket](../rate-limit/token-bucket.md)` | 弹出被依赖意图摘要 | 跳转打开目标 `.md` |
| 意图 MD（纵向父级） | `[@auth](README.md)` 或自动推导 | 弹出父模块背景与范围边界 | 跳转打开父级 README.md |

**额外能力**：调用树 × 意图视图——见 [`abstractDesign/view-call-tree-intent.md`](../abstractDesign/view-call-tree-intent.md)。它把"调用树节点 ↔ 意图 slug"二维展示,基于 `kron_impact` 与 `kron_intent_density` 的输出。**Kron 不画调用树本身**——画图是 IDE / LSP / GUI 的工作,Kron 只暴露数据。

### 3.4 文件格式自带的能力

由于数据就是 `.kron/intents/*.md`,以下"功能"是 Markdown + Git 自带的,不靠 Kron 提供：

- `git diff` 直观看到意图变更
- `git log` / `git blame` 追溯意图历史
- PR review 时直接读 Markdown
- 多人协作按 Markdown 处理 merge conflict

完整命令清单见 [`docs/implementation/cli.md`](implementation/cli.md)；MCP 工具契约见 [`docs/implementation/mcp.md`](implementation/mcp.md)；IDE 交互契约见 [`docs/implementation/ide-interaction.md`](implementation/ide-interaction.md)。

---

## 4 短短的一段：`kron lint` 的输出形状

`kron lint` 设计上的输出形状（CI 集成时一个 yaml block）：

```yaml
# .github/workflows/ci.yml
- name: kron lint
  run: |
    go install ./cmd/kron
    kron lint
```

预期 `kron lint` 输出形如：

```
[error] anchor dangling: internal/legacy/login.go:42 -> "auth.login-v1" (file .kron/intents/auth/login-v1.md not found)
[error] frontmatter invalid: .kron/intents/foo.md missing 'created_by'
[ok]    0 errors, 0 warnings
exit 1
```

`kron lint` 设计为**唯一一个在 CI 强制运行**的命令。其他三个（`init` / `add` / `serve-mcp`）只在本地用。完整规则见 [`docs/process/lint-rule.md`](process/lint-rule.md)。

---

## 5 AI Agent 视角：MCP 是入口

设计：如果你是 AI Agent（不是人），你**不会用 CLI**——你用 MCP。预期配置如下（谁配置谁就能调 12 个工具；当前是否真正可用，看 main 分支）：

```jsonc
// .cursor/mcp.json (or whatever client config)
{
  "mcpServers": {
    "kron": {
      "command": "kron",
      "args": ["serve-mcp"]
    }
  }
}
```

设计上的工具集（v1 总数 12 个）：

- 生命周期：`kron_init` / `kron_add` / `kron_list` / `kron_get` / `kron_update` / `kron_delete` / `kron_restore`
- 校验：`kron_lint`
- **AI 主动消费**：`kron_assume_check` / `kron_impact` / `kron_intent_density` / `kron_stale`

这 4 个"主动消费"工具让 AI 在编码循环里**主动**消费意图，而不是被动 grep Markdown：

- `kron_assume_check` 在改代码前自动列出"你依赖的假设"
- `kron_impact` 在改意图前自动列出"哪些代码 / 下游意图受影响"
- `kron_intent_density` 让 CI 量化"意图覆盖率"
- `kron_stale` 让 CI 找出"过期意图"

完整契约见 [`docs/implementation/mcp.md`](implementation/mcp.md) §2。

作者写完意图文件，**人和 AI 用同一份数据**——零双向同步、零事件总线、零状态机。这是 Kron 的核心承诺，与是否已实现无关。

---

## 6 你不需要知道的（但很多人会问）

**Q：意图文件是数据库吗？**  
A：不是。是 Git 跟踪的 Markdown。版本控制、diff、merge conflict 都用 Git 原生机制。

**Q：意图会被备份吗？**  
A：Git 本身是备份。Kron 不会在工具目录里偷偷存任何东西。

**Q：意图条目能链接到代码符号吗（自动）？**  
A：不能自动。用 `// @kron:intent <slug>` 锚点手动标。锚点是发现机制，不是事实记录。

**Q：意图过期了怎么办？**  
A：把 `status: "active"` 改为 `status: "superseded"`，再写一个 superseder 意图。`kron_lint` 不会报错。

---

## 7 你接下来读什么

| 如果你是… | 先读 | 然后 |
|---|---|---|
| **新使用者**（想用 Kron）| 本文档（已完成）| [`README.md`](../../README.md) §参与贡献 |
| **新贡献者**（想改 Kron 代码）| [`AGENTS.md`](../../AGENTS.md) | [`docs/abstractDesign/architecture.md`](abstractDesign/architecture.md) |
| **AI Agent 操作员**（想配置 MCP）| §5（已完成）| [`docs/implementation/mcp.md`](implementation/mcp.md) |
| **CLI 用户**（想在 CI 里跑）| §4（已完成）| [`docs/implementation/cli.md`](implementation/cli.md) |
| **文档维护者** | [`docs/abstractDesign/docs-map.md`](abstractDesign/docs-map.md) | [`docs/process/references-snapshot.md`](process/references-snapshot.md) |
