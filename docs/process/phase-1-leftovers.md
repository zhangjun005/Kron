# Phase 1 Docs-Only 遗留事项

> 生成时间：2026-09-27  
> 关联 commit：`ae0d244`(说服力内核) + `88b36c5`(流程沉淀)  
> 状态：**未做，留给后续 phase**

本 phase 只改 docs，没动 `.go` 代码、frontmatter schema、lint 规则、CLI 子命令。
由此产生 4 条遗留事项，按"是否阻塞说服力"排序。

---

## L1. assumptions / expires_at schema 未落地（**最关键**）

### 现状

`docs/implementation/mcp.md` §2 中 4 个新工具契约里写了这些字段：

- `kron_assume_check.assumption.severity`
- `kron_assume_check.assumption.expires_at`
- `kron_stale.assumption.expires_at`

但 [`internal/model/intent.go`](../internal/model/intent.go) 的 `Frontmatter` 结构体**没有**这些字段。
[`docs/how-it-works.md`](../how-it-works.md) §2 的示例 frontmatter 里有 `assumptions: [...]` + `severity: hard/soft` + `expires_at`——**示例与代码不一致**。

### 后果

- 4 个新工具的契约是**未来才能落地**的契约，不是"现在就能用"
- docs 描述的"AI 主动消费"承诺，目前在数据层缺原料
- 例子与代码不一致已经发生，违反 "implementation-status = code" 的实景原则

### 该走的流程

[`docs/process/migrate.md`](../docs/process/migrate.md)（frontmatter schema 修改流程）。

### 估算工作量

- `internal/model/intent.go`：`Frontmatter` 加 `Assumptions []Assumption`；新增 `Assumption` 结构体 + `Severity` 枚举
- `internal/store/`: frontmatter 反序列化加字段
- `docs/abstractDesign/intent-structure.md`：补 `assumptions[]` schema 段
- `docs/implementation/domain-model.md`：同步 struct 定义
- `docs/business.md` §2.3.1 表格里"severity" / "expires_at" 字段可以保留（已是 spec）
- 测试：表驱动法覆盖 hard / soft / 缺字段 / expires_at 格式异常

约 1 天工作量，1 个独立 PR。

---

## L2. `references-snapshot.md` §一 §二 数据已过期

### 现状

`docs/process/references-snapshot.md` 是 2026-09-26 的快照。PR-A + PR-B 改了 8 个文档、新增 2 个，但**未重生成**快照。

顶栏与 §4.1 已显式标记过期：

> 2026-09-27 本批改动（MCP 工具集 8 → 12、新增 view-call-tree-intent.md 与 mcp-tool.md）后，§一 §二 数据已过期

### 后果

- §二"入度排名"会少算 view-call-tree-intent.md（被 6 个文件引用）与 mcp-tool.md（被 2 个文件引用）
- §一"引用图"会少算多行

### 该走的方式

**不要手工伪造新数字**（让"快照"失去意义）。正确做法：
1. 跑 `scripts/regen-refs-snapshot.sh`（[references-snapshot.md §五](../docs/process/references-snapshot.md) 列了脚本，但 ci-enforcement.md 标 P3 未落地）
2. 如果脚本还没落地，按 §五.1 的手工 `rg` 命令补一遍

### 估算工作量

- 落地脚本：1 小时（含 CI P3 评估）
- 一次性手工重生成：10 分钟

---

## L3. 4 个新 MCP 工具的代码实现在 docs 里已 spec，但代码不存在

### 现状

`cmd/kron/serve-mcp/` 这个目录**在仓库里当前不存在**（只有占位 stub `cmd/kron/cli/cli.go`）。我 grep 整个仓库确认过：

```
cmd/kron/
├── main.go
└── cli/cli.go        ← 不是 serve-mcp
```

[`docs/how-it-works.md`](../docs/how-it-works.md) §5 写成"kron serve-mcp 启动后能被 Claude Desktop / Cursor 配置接管"——但**当前 main 分支上这个命令根本不存在**。

### 后果

- "产品说服力"在 docs 层已写完
- 但"使用 Kron"还是依赖 docs 设计意图，**没有任何可运行代码佐证**
- 别人读 docs 觉得"很好",然后 `kron serve-mcp` 报错"not implemented",说服力归零

### 该做的范围

按 [`docs/process/mcp-tool.md`](../docs/process/mcp-tool.md) §2 流程开 phase 2：

1. 在 `internal/store/store.go` + `internal/parser/parser.go` 把 stub 变可运行（load / list / scan anchor）
2. 新建 `cmd/kron/serve-mcp/`
3. 实现 12 个工具的占位（8 个原工具按现有 spec + 4 个新工具按本次 docs spec）
4. 端到端跑 `kron serve-mcp` + 一个 MCP 客户端（最小 demo）

### 估算工作量

- 2-3 天（核心是 store + parser 实现 + 12 工具占位）
- 不可与本 phase 合并——docs 已经是稳定 spec，代码 PR 可以单独走

---

## L4. 调用树 × 意图视图的"消费方"尚未表态

### 现状

[`docs/abstractDesign/view-call-tree-intent.md`](../docs/abstractDesign/view-call-tree-intent.md) §3 列出"谁可以消费":

- IDE 插件（VSCode / Cursor）
- LSP server
- GUI 客户端
- 任何想画调用树的客户端

但**目前仓库里这三类客户端都不存在**——v1 明确"不做 LSP / IDE / GUI"。

### 后果

- 视图的数据契约已有，**消费侧没人接** = 视图目前是孤立文档
- 但这本身就是 docs 阶段该有的状态（spec 先于实现）

### 该做的范围

不在 phase 1 / phase 2 范围。等到 v1 后续 phase：

- LSP / IDE / GUI 任一项启动时，本文档直接复用
- 或做一个简单的"第三方脚本 demo"：用 `kron_intent_density` 输出 + grep 反向锚点 → 文本版调用树

### 估算工作量

视消费侧选择：

- 文本版 demo 脚本：2 小时
- IDE 扩展：不在本评估范围（架构 §〇 铁律 #3 决定 IDE 是新访问层）

---

## 优先级建议（按"完成说服力闭环"紧迫度）

| 优先级 | L 编号 | 不做的后果 | 推荐顺序 |
|---|---|---|---|
| P0 | L1 | docs 与代码 schema 不一致；示范例会失效 | **立即做**（1 天） |
| P1 | L3 | 别人跑 `kron serve-mcp` 直接破防；说服力归零 | phase 2 第一周 |
| P2 | L2 | 文档健康度下降，但不影响外部说服力 | phase 2 第二周顺手 |
| P3 | L4 | 孤立文档，但 spec 已稳 | 等 LSP / IDE / GUI phase |

P0 + P1 都完成后，"Kron 必留"才算**端到端可演示**——不是只在 docs 写得漂亮。
