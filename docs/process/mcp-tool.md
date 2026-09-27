# 新增 MCP 工具流程

> 来源：[`docs/abstractDesign/architecture.md`](../abstractDesign/architecture.md) §1.2、§〇·五·5  
> 本文件是**实施流程**（G 角色），不是架构真理。给 `kron serve-mcp` 增加新工具时按此流程执行。

---

## 1 决策树：要不要加 MCP 工具

```
要加的工具涉及什么？
├─ 只是新 JSON-RPC endpoint，后台是已有 internal/ 函数组合
│   └─ → §2 简单流程（绝大多数情况走这条）
├─ 需要新的 internal/ 函数
│   └─ → 先走 [`internal-pkg.md`](./internal-pkg.md) 开新包流程，再回到 §2
└─ 跨访问层都要用（CLI / IDE / GUI 也想要同等输出）
    └─ → 先考虑下沉到 internal/lint/；见 [architecture.md §〇·五·5](../abstractDesign/architecture.md)
        不下沉只因"两个访问层共用"是反模式——必须先证明三家用
```

**架构层约束**（来自 [`architecture.md`](../abstractDesign/architecture.md) §〇 铁律 #3）：

- MCP 是 v1 已落地的访问层，新工具**只**能加在 `cmd/kron/serve-mcp/` 下
- 新工具**不**能 import 其他访问层（CLI / 未来的 GUI / IDE 插件）
- 新工具的协议消息格式（JSON schema / 错误码映射）**不**下沉——永远留在 `cmd/kron/serve-mcp/`

---

## 2 新增 MCP 工具流程（4 步）

### Step 1 — 在 `docs/implementation/mcp.md` §2 追加工具契约

按现有格式追加一节 `### <tool_name>`，含入参 / 出参 / 错误码 / 说明 / 触发场景 5 个字段。
**说明**不超过 3 行；超过则表明该工具应在 `cmd/kron/serve-mcp/` 内拆多个函数，而不是写更多文字。

### Step 2 — 同步真理层与协作层（同一 PR）

| 文件 | 改动 |
|---|---|
| [`docs/abstractDesign/architecture.md`](../abstractDesign/architecture.md) §1.2 | MCP 工具集表格扩 1 行 |
| [`docs/business.md`](../business.md) §2.3 | "AI 主动消费意图"段追加该工具名 + 一句话 |
| [`docs/how-it-works.md`](../how-it-works.md) §5 | AI Agent 视角工具集扩 1 个 |
| [`docs/abstractDesign/docs-map.md`](../abstractDesign/docs-map.md) §五 | 索引不需改（除非本流程文档新增了 G 文件） |

> **代码 PR 与 docs PR 可分开**：docs PR 不阻塞代码合并，但**新工具的 MCP 契约必须在代码 PR 合之前进 `mcp.md`**，否则下游消费者无法对齐。

### Step 3 — 在 `docs/process/references-snapshot.md` 标记本批

在 §一 引用图里把新工具名（如果出现在其他文件中）补一行；§二 入度排名会自动变化，无需手工改。
**重生成脚本**：见 [`references-snapshot.md`](./references-snapshot.md) §五。

### Step 4 — CI 验证

```bash
# 1) 文档自洽：grep 新工具名在 docs/ 出现 ≥ 4 处
rg -l "<tool_name>" docs/

# 2) 单向引用链不破：F 层新增 mcp.md 段落不引 G
# （人工 review）

# 3) 若改了 architecture.md / business.md：跑 docs-map 自检
# 见 docs/process/ci-enforcement.md
```

---

## 3 工具命名规则

| 规则 | 示例 |
|---|---|
| 全小写 + 下划线 + `kron_` 前缀 | `kron_assume_check`（不是 `assumeCheck` 或 `KronAssumeCheck`） |
| 动宾结构 | `kron_impact` / `kron_stale` / `kron_intent_density`（不是 `kron_impact_analyzer`） |
| 动词是一般现在时第三人称单数 | `kron_check`（不是 `kron_checking`） |
| 与 MCP 协议惯例一致 | JSON-RPC 工具名 `kron_*`（不是 `kron.*` 或 `Kron`） |

---

## 4 MCP 工具范畴

### 4.1 允许的 MCP 工具类别

| 类别 | 已落地 | 触发场景 |
|---|---|---|
| 增删改查 | `kron_init` / `kron_add` / `kron_list` / `kron_get` / `kron_update` / `kron_delete` / `kron_restore` | 意图生命周期 |
| 校验 | `kron_lint` | CI 门禁 |
| 主动消费（AI Agent） | `kron_assume_check` / `kron_impact` / `kron_intent_density` / `kron_stale` | AI 在编码循环中**主动**消费意图 |

### 4.2 不允许的 MCP 工具类别

| 场景 | 为什么不放进 MCP |
|---|---|
| 通用 Markdown 解析 | 不是 Kron 的职责；用 ripgrep / grep |
| 跨仓库 intent 查询 | 当前 v1 不支持多 workspace |
| 意图间的传递闭包计算 | v1 用相对链接 + 目录层级，不建图（见 [`business.md`](../business.md) §2.2） |
| 自动起草新意图 | 留 GUI / IDE 插件 phase；MCP 只暴露 `kron_add` 脚手架能力 |

---

## 5 与 CLI / IDE / GUI 的关系

| 维度 | MCP | CLI | IDE / GUI |
|---|---|---|---|
| 入口 | `kron serve-mcp`（stdio JSON-RPC）| `kron init / add / lint` | 各自宿主 |
| 查询能力 | `kron_list` / `kron_get` / `kron_impact` / `kron_intent_density` / `kron_stale` | **不**做查询（v1 CLI 最小集） | 直连 `internal/store` + `internal/parser` |
| 校验 | `kron_lint` | `kron lint`（共享 `internal/lint/`，当开时） | 同左 |
| 主动消费 | `kron_assume_check` / `kron_impact` / `kron_intent_density` / `kron_stale` | **不**做 | 通过 MCP 调用或直连 `internal/` |

> **关键原则**：MCP 工具集的**全集**（12 个）由 [`docs/abstractDesign/architecture.md`](../abstractDesign/architecture.md) §1.2 定义；CLI / IDE / GUI **不**重复 MCP 已覆盖的能力。
