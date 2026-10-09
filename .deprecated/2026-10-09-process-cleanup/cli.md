# CLI 命令实现参考

> 来源：`docs/abstractDesign/architecture.md` §1.1、§5.1、§5.2、§5.2.1、§5.3、§7  
> 本文件是**实施建议**，不是架构真理。API 表面如有变更，请同步修改此处。

---

## 1 命令清单（v1）

| 命令 | 用途 | 源码位置 |
|---|---|---|
| `kron init` | 创建 `.kron/intents/` + `config.toml` | `cmd/kron/cli/init.go` |
| `kron add <slug>` | 脚手架新意图文件 | `cmd/kron/cli/add.go` |
| `kron lint` | CI 门禁：锚点悬空 + frontmatter 校验 | `cmd/kron/cli/lint.go` |
| `kron serve-mcp` | 启动 MCP stdio server | `cmd/kron/cli/serve_mcp.go` |

**不在 CLI 中实现**：`list` / `get` / `update` / `delete` / `restore`——由 MCP 工具集覆盖。

---

## 2 `kron init`

### 流程

```
探测 .kron/ 是否存在
  ├─ 存在 → 提示已初始化，退出
  └─ 不存在 → mkdir .kron/intents/ + 写 .kron/config.toml（默认值 intents_dir = ".kron/intents"）
```

### 实现要点

- 实现位于 `cmd/kron/cli/init.go`
- 调用 `store.EnsureKronDir(ctx, cfg)` 创建目录
- `config.toml` 仅含 `intents_dir` 字段（v1 不扩展）

---

## 3 `kron add <slug>`

### 流程

```
接收第一个位置参数 args[0] 作为 slug
  ├─ 缺失或多余参数 → 返回 ErrSlugInvalid
  └─ 存在 →
      正则校验 slug：^[a-z0-9]+(-[a-z0-9]+)*(/[a-z0-9]+(-[a-z0-9]+)*)*$
        ├─ 不匹配 → 返回 ErrSlugInvalid
        └─ 匹配 →
            检查 .kron/intents/<slug>.md 是否已存在
              ├─ 存在 → 返回 ErrIntentExists
              └─ 不存在 →
                  构造模板 frontmatter（created_by 来自 git config user.name）
                  写入文件
```

### 实现要点

- 实现位于 `cmd/kron/cli/add.go`
- 调用 `store.WriteIntent(ctx, slug, &intent)`
- `created_by` 从 `git config user.name` 获取，格式 `@username`
- slug 校验正则：`^[a-z0-9]+(-[a-z0-9]+)*(/[a-z0-9]+(-[a-z0-9]+)*)*$`

> **v1 不支持 stdin / 外部模板**。`kron add` 职责是脚手架（Scaffold）——写入默认骨架后，开发者或 AI 直接打开 `.md` 编辑。

---

## 4 `kron lint`

### 流程

```
默认从当前工作目录（CWD）开始递归扫描所有源码文件
  跳过目录黑名单（filepath.SkipDir）：
    .git、.kron、node_modules、vendor、dist、bin、target、.idea、.vscode
对每个源码文件 → parser.ScanAnchors → 收集 []Anchor
对每个 Anchor → store.ResolveIntent(slug) → 校验文件存在
  └─ 不存在 → 输出 [error] anchor dangling + 计入 errors
遍历 .kron/intents/**/*.md → store.LoadAll → 校验 frontmatter 必需字段
  └─ 缺失 → 输出 [error] frontmatter invalid + 计入 errors

退出码：
  0 = 通过（无错误）
  1 = 有 lint 错误
  2 = 内部失败（IO / 解析异常）
输出格式支持 --reporter=text|json，便于 CI 集成。
```

### 实现要点

- 实现位于 `cmd/kron/cli/lint.go`
- 调用 `internal/lint/` 包（CLI / MCP / IDE 共享）
- `--reporter` flag 支持 `text`（默认）和 `json`

> **v1 不暴露 `--path` 自定义扫描根**。全仓扫描 + 硬编码黑名单是 v1 的全部策略。

---

## 5 软删除与恢复（MCP 独占，CLI 不暴露）

### 流程

```
kron_delete <slug>：
  校验 .kron/intents/<slug>.md 存在
    ├─ 不存在 → 返回 ErrIntentNotFound
    └─ 存在 →
        mkdir -p .kron/.trash/
        移动文件：.kron/intents/<slug>.md → .kron/.trash/<slug>.md
        返回 { ok: true, trashed_path: ".kron/.trash/<slug>.md" }

kron_restore <slug>：
  校验 .kron/.trash/<slug>.md 存在
    ├─ 不存在 → 返回 ErrIntentNotFound
    └─ 存在 →
        移动文件：.kron/.trash/<slug>.md → .kron/intents/<slug>.md
        返回 { ok: true, path: ".kron/intents/<slug>.md" }
```

- 实现位于 `internal/store/trash.go`
- CLI **不**提供 `kron delete` / `kron restore` 子命令

---

## 6 v1 范围外（CLI 不做的事）

以下明确不在 v1 CLI 范围内：

| 特性 | 原因 |
|---|---|
| `kron list` / `get` / `update` / `delete` / `restore` | 由 MCP 工具集覆盖；CLI 不重复实现 |
| `kron add` stdin / 外部模板支持 | 脚手架职责，复杂输入留给编辑器或 GUI |
| `kron lint` `--path` 自定义扫描根 | 全仓 + 黑名单足够；真实需求出现时再加 flag |
| 守护进程 / 文件监听 / 状态机 | 不在 v1 范围 |
| `parent` / `depends_on` 拓扑建模 | 目录 + 相对链接已足够 |
| `config.toml` 字段扩展 | v1 仅 `intents_dir` |
