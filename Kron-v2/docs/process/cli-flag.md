# 新增 CLI Flag 流程

> 来源：`docs/abstractDesign/architecture.md` §1.1、§7  
> 本文件是**实施流程**，不是架构真理。给现有 CLI 命令增加新 flag 时按此流程执行。

---

## 1 决策树：要不要加 flag

```
要加的 flag 是什么类型的？
├─ 新命令（如 kron status）
│   └─ ❌ 不允许——v1 CLI 最小集已锁定：init / add / lint / serve-mcp
│       → 需要评审，问人类：真的需要这个命令吗？MCP 够不够？
├─ 现有命令的全局 flag（如 --config PATH）
│   └─ → 走 §2
├─ 现有命令的行为 flag（如 kron add --template PATH）
│   └─ → 走 §2
└─ 现有命令的输出 flag（如 kron lint --reporter json）
    └─ → 走 §2（最安全的 flag 类型，因为它只改变输出格式，不改变数据）
```

> **架构层约束**：CLI 只做"终端独有"的事。查询类 flag（`--format` / `--json` / `--table`）
> 原则上属于 MCP/GUI 的职责；加到 CLI 里会模糊访问层边界。
> 唯一例外：`kron lint --reporter` 是因为 CI 必须支持 `text` 和 `json` 两种格式。

---

## 2 新增 flag 流程（6 步）

### Step 1 — 语义审查

回答以下问题，写入 commit body：

| 问题 | 答案示例 |
|---|---|
| 这个 flag 在 MCP/GUI 里有没有等价能力？ | MCP `kron_lint` 已经支持 `--reporter` |
| 如果 MCP 没有等价能力，为什么要放 CLI 而不是 MCP？ | 必须能被 GitHub Actions 直接调用 |
| 加这个 flag 会让 CLI 和 MCP 行为不一致吗？ | 不会——flag 只影响输出格式 |
| flag 默认值是什么？| `text` |
| flag 是 breaking change 吗？| 否——新 flag 加默认值，向后兼容 |

### Step 2 — 在 `cmd/kron/cli/` 中实现

```go
// cmd/kron/cli/lint.go
var reporter string

var cmdLint = &cobra.Command{
    Use:   "lint",
    Short: "Scan anchors and validate frontmatter (CI gate)",
    RunE:  runLint,
}

func init() {
    cmdLint.Flags().StringVar(&reporter, "reporter", "text",
        "Output format: text or json (for CI integration)")
}
```

### Step 3 — 写测试

- 端到端测试：用 `cobra.Command.ExecuteContext` + `bytes.Buffer` 捕获输出
- 验证 `--reporter=json` 输出是合法 JSON

### Step 4 — 更新 `docs/implementation/cli.md`

在对应命令节下加一行 flag 说明：

```markdown
### kron lint

...（现有流程）...

#### Flags

| Flag | 默认值 | 用途 |
|---|---|---|
| `--reporter` | `text` | 输出格式：`text` 或 `json` |
```

### Step 5 — 更新 `AGENTS.md`

在 "Off-limits without explicit ask" 中确认不在禁止列表（`--reporter` 属于已允许的输出类 flag）。

### Step 6 — CI 验证

```bash
go vet ./...
gofmt -l .
go test ./...
```

---

## 3 Flag 命名规则

| 规则 | 示例 |
|---|---|
| 用全小写 + 连字符 | `--reporter`（不 `--R` 或 `--out-format`） |
| flag 名必须是名词或形容词，不能是动词 | `--reporter`（noun），不 `--output-json`（verb） |
| bool flag 命名 | `--dry-run` / `--verbose`（adj），不用 `--show-verbose` |
| 与 cobra 惯例一致 | `--flag-name`（cobra 自动转 `--flag-name`） |

---

## 4 CLI Flags 范畴

> 实景：当前仓库里 `cmd/kron/cli/` 是 stub（返回 "not yet implemented"），没有任何 flag 落地。  
> 本节描述设计**范畴**——CLI flag 应当承担什么、不应当承担什么。实景以仓库代码为准。

### 4.1 设计上允许的 flag 类别

- `--reporter text|json`（在 `kron lint` 上）：给人 vs 给 CI 用的输出格式。
- 其它 flag 都属于本表的 ❌ 范围。

### 4.2 不允许的 flag 类别

| 场景 | CLI flag | MCP 参数 |
|---|---|---|
| 初始化 | `kron init`（无 flag） | `kron_init`（无参数） |
| 输出格式 | `kron lint --reporter json` | `kron_lint`（出参 `{ errors: string[] }` 是 JSON 本身） |
| 文件过滤 | 不允许 | 不允许 |

> **v1 范围外**：`--path`（自定义扫描根）、`--config`（自定义配置路径）、stdin 支持——见 architecture.md §7。

---

## 5 与 MCP 工具集的关系

| 场景 | CLI flag | MCP 参数 |
|---|---|---|
| 初始化 | `kron init`（无 flag） | `kron_init`（无参数） |
| 输出格式 | `kron lint --reporter json` | `kron_lint`（出参 `{ errors: string[] }` 是 JSON 本身） |
| 文件过滤 | ❌ 不在 v1 范围 | ❌ 不在 v1 范围 |

> 关键原则：**协议格式（JSON）是 MCP 的事**；CLI 的 `--reporter json` 只是给人类看的格式化糖衣。
> 如果 AI Agent 需要程序化访问，用 MCP；人类在终端需要 CI 兼容输出，用 `kron lint --reporter json`。
