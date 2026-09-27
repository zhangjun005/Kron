# Kron 说服力点列

> 用途:面对同行 / 软件工程老师提问时,翻这份查"如何答"。  
> 角色:H(哲学愿景)层,跟 [`article.md`](article.md) 同级;不入 AGENTS / `.cursor/rules/*`。

---

## 一、Trap 2 反驳:"Kron 不就是给 AI 写 docs 嘛?"

### 物理差异(把"主动消费"和"被动文档"区分开)

| 维度 | 普通 docs(README / `codebase.md`) | Kron |
|---|---|---|
| 写完之后 | 半年没人看 | MCP 实时被 AI 拉取 |
| 谁来读 | 偶尔有人翻 | 每个 AI 编码循环都主动 query |
| 校验 | 没有 | `kron_lint` 在 CI 校验 anchor + frontmatter |
| 时效 | 完全靠人维护 | `kron_stale` 自动列过期意图 |
| 影响范围 | 不可查 | `kron_impact` 反查"改这个意图会炸哪些文件" |

**点**:Cursor 的 `codebase.md` 是给 AI 的索引,Kron 是给 AI 的**假设层**——`codebase.md` 没说"为什么这么做",只说"代码在哪里";Kron 说了。  
**如何答**:举"半年没人看的 README"对比"AI 每个 PR 自动 query 假设清单"的物理差异,把"文档=被动参考 / schema=主动契约"掰开。

---

### 工具位差(11 类同行熟悉工具逐一对照"Kron 在哪插")

| 现有做法 | 它解决什么 | 它**不**解决什么 | Kron 在哪插 |
|---|---|---|---|
| `// FIXME` / `TODO` 注释 | 标记"要做的事" | 不能说"为什么 + 什么假设" | frontmatter `assumptions[]` |
| ADR / MADR 模板 | 记录"做过什么决策" | 不能被 AI 主动 query,写完即遗忘 | `kron_impact` 反查依赖 |
| JSDoc / Doxygen / godoc | API 文档 | 描述接口签名,不描述设计意图 | `.md` 正文 + frontmatter |
| Notion / Confluence | 团队 wiki | 不在 git 里,clone 仓库读不到 | `.kron/intents/*.md` |
| Read the Docs / GitHub Pages | 发布文档 | 静态,不能跟代码同步 diff | git diff 直接看 |
| Jira / Linear / GitHub Issues | 任务追踪 | 生命周期 human-only,AI 不能用 | 跨人 / AI / CI 三方 |
| Sourcegraph / Code Search | grep + jump to def | 不能跨库 join "代码 ↔ 意图" | anchor scan + 双向链接 |
| Cursor `codebase.md` / Continue.dev | 给 LLM 的索引 | 自动派生,作者无控制 | 作者显式控制 frontmatter |
| Claude Project `project.md` | 项目级 context | 不在仓库,换电脑即丢 | repo 内 `frontmatter` |
| Eng blog / Postmortem | 知识沉淀 | 不可机读 | 是 schema + Markdown |

**点**:现有 11 类工具**每个**只做了 Kron 工作的一小部分;Kron 把"假设层"独立出来——`severity: hard` 这种字段,所有现行工具都没有。  
**如何答**:挑注释行锤一句,然后说"剩下 10 行都类似"。

---

### 现行做法的证据式吐槽(四类 demo 场景)

**A. Cursor / Claude 自动索引**

**点**:你是被动的 customer——AI 怎么总结、丢啥,你完全没控。  
**如何答**:举"假设你有个'我们不接非 UTF-8 输入'写在 Slack 里,`codebase.md` 不会自动知道;Kron 把假设写在 frontmatter,AI 必须主动 query"。

**B. Notion / Confluence 团队 wiki**

**点**:Wiki 独立于代码,半年后没人翻;没 `status` 管生命周期;离职同学带没入库的知识走。  
**如何答**:Polanyi paradox 重演——直接讲"git clone 即查 + status 管生命周期"。

**C. PR template + LLM review(CodeRabbit)**

**点**:在 PR 已写完之后才介入,只能给建议不能拦门。  
**如何答**:"retrospective 反馈 vs prospective gate"。

**D. AI 起草 ADR**

**点**:起草是速度,query 是结构化——AI 怎么查"哪些决策影响 module X"?  
**如何答**:两个独立服务,前者现成,后者没现成工具。

---

## 二、Trap 4 反驳:"LLM 进化后 Kron 还有意义吗?"

**误解 1:LLM 自己能 infer,不需要 schema**

**点**:LLM inference = 压缩感知,frontmatter = 无损传输;带宽永远够因为是人显式写。  
**如何答**:举 Lost-in-the-Middle(Liu et al., 2023 ACL)——LLM 在 16k context 中段信息检索 < 60%,意图不应靠 context 反推。

**误解 2:AGI 来了你就没意义**

**点**:LLM 越强,对自己越自信,反而越需要显式假设层;失败模式随能力变强而迁移——从"打不出代码" → "自信给错"。  
**如何答**:用三档失败模式表("3.5 写不出 / 4 写错 30% / 5+ 自信错")说明 Kron 拦的是失败模式的迁移。

**误解 3:Claude Skills / Anthropic Tools 出来你怎么办**

**点**:Anthropic Skills 是 vendor closed-form,不在仓库;Kron 是开源协议层,Cursor / Claude / Cline / Continue 共用。  
**如何答**:类比 PostgreSQL 不被 Oracle 替代——**轻量、开源、协议中立永远有生态位**;关键反驳:**Kron 是所有 vendor 共用的、由仓库作者控制的 schema**。

---

## 三、跨 trap 都用得上的"硬钉"数据点

每个数据点都指向同一件事——**当前 LLM 状态下"自动反推"不可靠**。提哪个都行,组合提尤其稳。

| 文献 | 一句话 | 对位 |
|---|---|---|
| Liu et al., 2023 ACL "Lost in the Middle" | LLM 16k+ context 中段信息检索 < 60% | 意图不应靠 context 反推 |
| Greg Kamradt, 2024 "Context Rot" | GPT-4 / Claude 64k+ context "代码初衷"问答准确率显著下降 | 把意图从 context 里拔出 |
| Nonaka / Takeuchi, 1995 SECI 模型 | "我们知道的比能说的多"——靠知识外化竞争 | Kron 是 Polanyi paradox 的工程化 |
| Anthropic, 2024 Constitutional AI / Calibration 论文 | LLM "自信"回答中约 8% 错 | `severity: hard` 显式风险标签 |

---

## 四、Kron 自身硬伤(坦诚 = 加分)

| 硬伤 | 如何答 |
|---|---|
| `kron_add` 占位,AI 起草 → 入库 闭环未完成 | v1 基础设施先行,闭环后续 phase |
| `assumptions / expires_at` schema 在 docs 已 spec,代码未落地 | spec-first 立场,走 [`migrate.md`](process/migrate.md) 流程 |
| LSP / IDE 集成未做,MVP 仅 MCP | 通过 MCP client 用 MVP,比 IDE 集成早一步可用 |
| Coverage metric 没有 CI gate | P1,phase 2;`kron_intent_density` 已 spec,gate flag 走 [`cli-flag.md`](process/cli-flag.md) 流程 |

---

## 五、Kron 自身护城河(被问"凭什么不被替代"用)

| 维度 | 护城河 |
|---|---|
| 数据格式 | 单数据源 `.md`,git diff 自带 audit |
| 协议层 | MCP 是 v1 访问层选择,核心 `internal/` + `.md` schema 不动 |
| 跨 LLM 厂商 | 开源 + 协议中立,Cursor / Claude / Cline / Continue 共用 |
| 跨语言 | 行级正则 + anchor scan,不绑 AST |
| 跨项目规模 | frontmatter 是 ad-hoc YAML,故意走最低公分母 schema |
| 集成 | MCP 12 工具 + CI 门禁,与现有 ADR / wiki / Jira **正交不替代** |

---

## 六、引用回 docs 的位置

| 主题 | Kron 自有 doc |
|---|---|
| 4 个 MCP 工具(主动消费) | [`docs/implementation/mcp.md`](implementation/mcp.md) §2 |
| 调用树 × 意图视图数据契约 | [`docs/abstractDesign/view-call-tree-intent.md`](abstractDesign/view-call-tree-intent.md) |
| §八 演进方向 | [`docs/abstractDesign/architecture.md`](abstractDesign/architecture.md) §八 |
| §2.3.1 AI 主动消费 | [`docs/business.md`](business.md) §2.3.1 |
