# Kron 是怎么工作的（实景）

> 目的：用一段真实的代码、一个真实的仓库状态，展示 Kron **具体**提供什么、**哪些是 v1 已实现、哪些是 Phase 2**。  
> 与 [`README.md`](../../README.md) 的关系：README 是 60 秒概览；本文档是 10 分钟实景。  
> 与 [`docs/abstractDesign/architecture.md`](abstractDesign/architecture.md) 的关系：那是**架构真理**，讲"应当如此"；本文档不重复规则，只看运行中的 Kron 长什么样。

---

## 1 一个真实场景：接手 `auth.RefreshToken`

设你 6 个月前离开项目，现在回来接手。

```bash
git clone git@github.com:yourorg/kron-auth-demo.git
cd kron-auth-demo
```

`ls` 后你会看见：

```
your-project/
├── .kron/                          ← Kron 目录
│   ├── config.toml                 ← 一行：intents_dir = ".kron/intents"
│   └── intents/
│       └── auth-refresh-token.md   ← 本次示例的意图文件
└── internal/auth/refresh.go        ← 你要接手的代码
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

## 2 跟随锚点：哪一行指向哪个意图

想看 `NewRefreshToken` 的设计意图？两种方式：

**（a）CLI**：暂未提供 `kron get`（v1 CLI 仅有 `init` / `add` / `lint` / `serve-mcp`；取意图走 MCP，详见 §5）。

**（b）手动阅读**：

```bash
# 锚点 → slug
$ sed -n '5p' internal/auth/refresh.go       # 提取锚点行
// @kron:intent auth-refresh-token
$ cat .kron/intents/auth-refresh-token.md
```

`.kron/intents/auth-refresh-token.md` 内容（v1 实际存储格式）：

```markdown
---
symbol:
  - "auth.NewRefreshToken"
created_by: "@zhangjun005"
updated_at: "2026-09-22T10:00:00Z"
reviewers:
  - "@alice"
status: "active"
---

# Refresh Token 实现选型

> 用 4 字节 random + 30 天过期；不上 JWT。

## 为什么
- 服务端需主动吊销——JWT 做不到
- 4 字节足够区分 ~4B 个活跃 token（业务天花板 < 1M）

## Trade-offs
- **放弃**：跨服务 token 共享（违反"吊销立即全局生效"诉求）
- **代价**：每次请求一次 Redis 查询

## Invariants / Assumptions
- 假设 ≤10K DAU
- 假设 Redis 可用性 ≥ 99.9%
```

> 这是 v1 当前实现（按 [`internal/model/intent.go`](../internal/model/intent.go) 的 `Frontmatter` 结构）。字段名严格来自 `model.Frontmatter`，不杜撰。

---

## 3 Kron 现在的实景能力（v1 已落地 vs. 计划中）

| 场景 | 当前可用 | 实现状态 |
|---|---|---|
| 在仓库里创建意图文件 | `kron init` + `kron add <slug>` | ✅ 已落地（脚手架一份默认骨架） |
| 编辑意图文件（你自己） | 直接用编辑器 | ✅ 一直能（Markdown + YAML） |
| 编辑意图文件（AI Agent 通过 MCP） | `serve-mcp` 暴露 `kron_add` / `kron_list` / `kron_get` / `kron_update` / `kron_delete` / `kron_restore` / `kron_lint` 等工具 | ✅ 已落地（stdio JSON-RPC） |
| **CI 拦截锚点悬空** | `kron lint` 扫描代码中 `// @kron:intent <slug>` 检查 slug 对应 `.md` 文件是否存在 | ✅ 已落地 |
| CI 拦截 frontmatter 缺字段 | `kron lint` 检查 `created_by` / `updated_at` 必填 | ✅ 已落地 |
| 上 Git 后用 `git diff` 看意图变更 | 纯 Markdown，天然支持 | ✅ 一直能 |
| 在 IDE 里 Hover 看意图 | Phase 2 | ❌ LSP / IDE 插件未实现 |
| GUI 客户端 | Phase 2 | ❌ 未实现 |
| 硬删除 / GC `.kron/.trash/` | Phase 2 | ❌ 故意不做 |
| 健康度诊断（过期 / 孤儿） | 未要求 | ❌ 暂不做 |

完整命令清单见 [`docs/implementation/cli.md`](implementation/cli.md)；MCP 工具契约见 [`docs/implementation/mcp.md`](implementation/mcp.md)。

---

## 4 短短的一段：`kron lint` 实际跑起来长什么样

```bash
$ kron lint
[error] anchor dangling: internal/legacy/login.go:42 -> "auth.login-v1" (file .kron/intents/auth/login-v1.md not found)
[ok]    0 errors, 0 warnings
exit 1
```

CI 集成一行：

```yaml
# .github/workflows/ci.yml
- name: kron lint
  run: |
    go install ./cmd/kron
    kron lint
```

`kron lint` 是目前**唯一一个在 CI 强制运行**的命令（架构真理 §五·CI 门禁）。其他命令（`init` / `add` / `lint` / `serve-mcp`）只在本地用。

---

## 5 AI Agent 视角：MCP 是入口

如果你是 AI Agent（不是人），你**不会用 CLI**——你用 MCP。Kron 起一个 stdio JSON-RPC 服务：

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

之后你（Agent）可以调 8 个工具：`kron_init` / `kron_add` / `kron_list` / `kron_get` / `kron_update` / `kron_delete` / `kron_restore` / `kron_lint`。人不需要任何额外工作。

这意味着：作者写完意图文件，**人和 AI 用同一份数据**——零双向同步、零事件总线、零状态机。

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
