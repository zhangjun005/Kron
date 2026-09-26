# Kron 是怎么工作的（实景）

> 目的：用一段真实的代码、一个真实的仓库状态，展示 Kron **作为工具是什么**——它提供哪些功能、对谁有用、典型工作流长什么样。  
> **本文件不声明任何功能的"实现状态"**——你读它就信它是设计意图；想看落地情况请看仓库 `cmd/kron/*` 与 `internal/*` 当前代码。  
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

## 3 Kron 提供哪些能力

> **本节只列功能，不标完成度。**  
> 一个能力"在 Kron 的设计里"是"被期望提供"，跟它在当前仓库里"已落地可运行"是两回事——后者请看仓库当前 main 分支的 `cmd/kron/cli/` 与 `cmd/kron/serve-mcp/`，或 [`internal/model/intent.go`](../internal/model/intent.go)（这是当前唯一一份真实代码示例，字段名严格来自 `model.Frontmatter`）。

### 3.1 CLI 提供的功能

Kron 设计四个 CLI 子命令（完整契约在 [`docs/implementation/cli.md`](implementation/cli.md)）：

- **`kron init`** —— 在仓库根创建 `.kron/intents/` 目录 + 写 `config.toml`（默认值：`intents_dir = ".kron/intents"`）。
- **`kron add <slug>`** —— 脚手架一个新意图文件（写默认 frontmatter 骨架；之后人或 AI 用编辑器继续编辑）。
- **`kron lint`** —— 扫描仓库所有源码，检查两类错误：① 代码里的 `// @kron:intent <slug>` 锚点是否指向真实存在的 `.md` 文件；② `.kron/intents/**/*.md` 是否含必填 frontmatter 字段。
- **`kron serve-mcp`** —— 启动 MCP stdio server，暴露 JSON-RPC 工具给 AI Agent。

CLI **不**实现 `list` / `get` / `update` / `delete` / `restore`——这五个是 MCP-only。

### 3.2 MCP 给 AI Agent 提供的功能

AI Agent 不走 CLI。Kron 启动一个 stdio JSON-RPC server，暴露 8 个工具：

- 写：`kron_init` / `kron_add` / `kron_update`
- 读：`kron_list` / `kron_get`
- 删除：`kron_delete`（软删除到 `.kron/.trash/`）/ `kron_restore`（从 `.trash/` 移回）
- 校验：`kron_lint`

完整工具契约见 [`docs/implementation/mcp.md`](implementation/mcp.md)。

### 3.3 文件格式提供的功能

由于数据就是 `.kron/intents/*.md`，以下"功能"是 Markdown + Git 自带的，不靠 Kron 提供：

- `git diff` 直观看到意图变更
- `git log` / `git blame` 追溯意图历史
- PR review 时直接读 Markdown
- 多人协作按 Markdown 处理 merge conflict

完整命令清单见 [`docs/implementation/cli.md`](implementation/cli.md)；MCP 工具契约见 [`docs/implementation/mcp.md`](implementation/mcp.md)。

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

设计：如果你是 AI Agent（不是人），你**不会用 CLI**——你用 MCP。预期配置如下（谁配置谁就能调 8 个工具；当前是否真正可用，看 main 分支）：

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

设计上的工具集：`kron_init` / `kron_add` / `kron_list` / `kron_get` / `kron_update` / `kron_delete` / `kron_restore` / `kron_lint`。这部分是**接口契约**的预期—— MCP 服务自身实现进度见仓库代码现状。

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
