# 调用树 × 意图视图

> 来源：[`docs/abstractDesign/architecture.md`](./architecture.md) §一、§八；[`docs/requirements.md`](../requirements.md) §四。  
> 本文件是**实施建议**（F 角色），不是架构真理。如有冲突，以 `architecture.md` 为准。
>
> **不画调用树**：本文档**只规定 Kron 暴露什么数据**，不规定 IDE / LSP / GUI 怎么画调用树本身——那是消费方的工作。

---

## 1 视图形态

调用树节点上**可选**贴一个意图标签 `[intent_slug]`，作为"代码符号 ↔ 设计意图"的视觉桥梁：

```
[auth/refresh-token]                 ← 意图节点（目录里的 .md）
└─ NewRefreshToken                   ← 函数节点（源码）
   ├─ Login      [auth/login-flow]   ← 锚点 → auth/login-flow.md
   └─ Refresh    [auth/refresh-flow]
      └─ RefreshToken  (无意图)      ← "意图盲区"
```

**关键语义**：

- 调用树节点 = **代码符号**（函数 / 类型 / 方法），由消费方（IDE / LSP / GUI）按自己的语言机制生成
- 意图标签 = **反向锚点** 的可视化（`// @kron:intent <slug>`）
- 没有意图标签的节点 = **意图盲区**

---

## 2 Kron 暴露的数据

视图的**全部数据基础**由 MCP 工具集（[`architecture.md`](../abstractDesign/architecture.md) §1.2）中两个工具提供：

| 数据需求 | 工具类别 | 输出字段 |
|---|---|---|
| 哪些源文件依赖某个意图（反向锚点） | 影响分析类 | `incoming_anchors` |
| 哪些意图没有任何锚点（意图盲区） | 量化审计类 | `coverage.intents_without_anchors` |
| 哪些源文件 > 50 行且 0 锚点（代码盲区） | 量化审计类 | `files_without_intent` |
| 意图间的横向关联（共享 `symbol`） | 影响分析类 | `depends_on_intents` |

工具的入参 / 出参 / 错误码完整契约位于 [`architecture.md`](../abstractDesign/architecture.md) §1.2；F 层不在本文档展开。

**为什么由 MCP 工具提供而不是导出新的 `internal/` 函数**：

- 消费方包括 IDE / LSP / GUI 三种访问层，**没有新增访问层**时不必下沉（见 [`architecture.md`](../abstractDesign/architecture.md) §〇·五·5）
- 这两个工具已能覆盖视图所需的所有数据；视图本身**不引入新字段**

---

## 3 消费方约定

**谁**可以消费：

- IDE 插件（VSCode / Cursor 扩展）
- LSP server（hover + definition + call hierarchy）
- GUI 客户端
- 任何想画调用树的客户端（包括第三方脚本）

**怎么**消费：

- **MCP-aware 客户端**：直接调 MCP 工具
- **非 MCP-aware 客户端**：通过 [`architecture.md`](../abstractDesign/architecture.md) §一 表中"IDE 插件"行的方式——**直接** import `internal/parser` 与 `internal/store`，**不** import `cmd/kron/serve-mcp/`
- 视图状态本身**不**入 `internal/`：调用树是消费方的内存对象，Kron 不持有

**关键约束**（受 [`architecture.md`](../abstractDesign/architecture.md) §〇 铁律 #3 约束）：

- 消费方之间**不**互相调用（IDE 不调 LSP；GUI 不调 CLI）
- 消费方**不**调其他访问层（IDE 不 import `cmd/kron/serve-mcp/`）

---

## 4 不做什么

| 不做 | 原因 |
|---|---|
| Kron 自己画调用树 | 无 AST 解析能力（见 [`requirements.md`](../requirements.md) §二"基础方案"）；行级正则扫描只产出锚点 |
| Kron 与具体 IDE 的 Call Hierarchy API 集成 | 跨访问层边界（见 [`architecture.md`](../abstractDesign/architecture.md) §〇 铁律 #3） |
| Kron 规定意图节点的视觉样式 | 颜色 / icon / 折叠规则留消费方——同一份数据，多种呈现 |
| 新增 `internal/` 子包承载视图逻辑 | 视图逻辑在消费方侧；`internal/` 只暴露数据 |

---

## 5 演进方向

| 方向 | 触发条件 |
|---|---|
| 新增 `kron_call_graph` 工具（完整调用树 + 意图标签） | 消费方持续反馈 "用 `kron_impact` 拼调用树太麻烦"；当前 `kron_impact` + `kron_intent_density` 组合已够用 |
| 视图进入 LSP server | v1 不在范围（见 [`architecture.md`](../abstractDesign/architecture.md) §一） |
| 视图进入 `cmd/kron/serve-gui/` | GUI phase 启动后；当前不在 v1 |
