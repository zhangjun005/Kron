# Phase 1 Docs-Only 遗留事项

> 生成时间：2026-09-27
> 关联 commit：`ae0d244`(说服力内核) + `88b36c5`(流程沉淀)
> **状态：2026-10-03 重新评估** —— L1 已完成,本文档进入历史归档,不再引导 v1 收尾。

本 phase 只改 docs，没动 `.go` 代码、frontmatter schema、lint 规则、CLI 子命令。
由此产生 4 条遗留事项，按"是否阻塞说服力"排序。

---

## L1. assumptions / expires_at schema 未落地 ✅ **已完成**

### 原现状

`docs/implementation/mcp.md` §2 中 4 个新工具契约里写了这些字段：

- `kron_assume_check.assumption.severity`
- `kron_assume_check.assumption.expires_at`
- `kron_stale.assumption.expires_at`

但 [`internal/model/intent.go`](../internal/model/intent.go) 的 `Frontmatter` 结构体**没有**这些字段。

### 完成情况（2026-10-03）

`internal/model/intent.go` 已包含：

- `Assumption` 结构体（`ID` / `Text` / `Severity` / `ExpiresAt` / `VerifiedAt` / `VerifiedBy`）
- `Severity` 枚举（`SeverityHard` / `SeveritySoft`）
- `Frontmatter.Assumptions []Assumption` 字段

`internal/parser` 与 `internal/store` 已在 `ParseFrontmatter` 链路中接受 `assumptions` 字段;
`docs/abstractDesign/intent-structure.md` §三 `assumptions` 字段策略已经写完。
`internal/model/intent_test.go` 已有表驱动测试覆盖 hard / soft / 缺字段场景。

### 残余事项

- `docs/implementation/domain-model.md` §2 的 `Frontmatter` 示例未把 `Assumptions` 字段列出
  （spec 漂移,不影响代码;v1 收尾 PR 中同步更新即可）
- L1 原本担心的"例子与代码不一致"风险——`docs/how-it-works.md` 与 `intent-structure.md`
  的示例已与代码一致 ✅

### 状态：**done**

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

## L3. 4 个新 MCP 工具的代码实现在 docs 里已 spec，但代码不存在 🟡 **CLI 部分已完成；MCP 部分仍 open**

### 原现状

`cmd/kron/serve-mcp/` 这个目录**在仓库里当前不存在**（只有占位 stub `cmd/kron/cli/cli.go`）。grep 整个仓库确认过：

```
cmd/kron/
├── main.go
└── cli/cli.go        ← 不是 serve-mcp
```

[`docs/how-it-works.md`](../docs/how-it-works.md) §5 写成"kron serve-mcp 启动后能被 Claude Desktop / Cursor 配置接管"——但**当时 main 分支上这个命令根本不存在**。

### 2026-10-03 拆分

L3 拆成两个子项：

- **L3-CLI ✅ done**：`kron init` / `kron add` / `kron lint` 三个子命令已实现并测试。
  - `kron init` 写 `.kron/intents/` 骨架 + 默认 `config.toml`
  - `kron add <slug>` 验证 slug + 写带哨兵 frontmatter 的 .md
  - `kron lint` 实现 A 类（anchor-dangling）+ B 类（frontmatter-invalid）规则，text/json 两种 reporter
  - `kron serve-mcp` 占位（exit 1 + 明确"not yet available"消息），phase 2 落地时替换
  - 退出码按 `docs/implementation/cli.md` §4：0 / 1 / 2
  - CLI 不引入 cobra（AGENTS.md off-limits），用 stdlib `flag` 派发
- **L3-MCP ⏳ open**：12 个 MCP 工具的实现，按 `docs/process/mcp-tool.md` §2 流程走；推 phase 2

### 状态

- L3-CLI：**done**（本次 v1 收尾）
- L3-MCP：**open**（phase 2）

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

> 2026-10-03 状态变更：L1 标 done；L3 仍 P1（CLI 落地是 v1 收尾首要目标）；
> L2 / L4 优先级不变。L3 描述的"12 工具 MCP"暂不列入 v1 收尾范围（cli.md §1 明确
> CLI 不实现 list/get/update/delete/restore；MCP 整体属于 phase 2）。

| 优先级 | L 编号 | 不做的后果 | 推荐顺序 |
|---|---|---|---|
| ~~P0~~ | ~~L1~~ | ~~docs 与代码 schema 不一致；示范例会失效~~ | **已完成** |
| ~~P1~~ | ~~L3-CLI~~ | ~~别人跑 `kron init/add/lint` 直接破防~~ | **已完成** |
| P1 | L3-MCP | 别人跑 `kron serve-mcp` 直接破防；说服力归零 | phase 2 第一周 |
| P2 | L2 | 文档健康度下降，但不影响外部说服力 | phase 2 第二周顺手 |
| P3 | L4 | 孤立文档，但 spec 已稳 | 等 LSP / IDE / GUI phase |

> **v1 收尾目标（2026-10-03 重新定义）**：完成 L3 中"**CLI 部分**"——
> `kron init` / `kron add` / `kron lint` 三个子命令端到端可运行,lint 跑通 A/B 类规则。
> `kron serve-mcp` 推迟到 phase 2（架构铁律 §〇·五·5: `internal/lint` 按需开,
> CLI 独自用 lint 不下沉）。
> **2026-10-03 状态：L1 + L3-CLI 已完成**；L3-MCP / L2 / L4 仍 open。

P0 + P1-CLI 都完成后，"Kron 必留"才算**端到端可演示**——CLI 部分已经在本批 commit 中端到端跑通。
