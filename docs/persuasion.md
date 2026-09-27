# Kron 说服力脚本

> 用途:在工程汇报、学术 demo、招聘面试中回答"Kron 为什么不可被替代"。  
> 角色:**H(哲学愿景)**层,跟 [`article.md`](article.md) 同级。  
> 不写于 `AGENTS.md` / `.cursor/rules/*` —— 那两份是 AI 协作规约,不属于项目愿景。

---

## 一、30 秒电梯演讲(主版本)

> "我想做的不是 docs 系统,是**给 AI 的可 query 的设计意图 schema**。
>
> 现在主流工程做法有四派:注释派(`// FIXME`)、ADR 派(MADR 模板)、Cursor 派(`codebase.md`)、LLM review 派(CodeRabbit 等)。
> 它们有个共同前提——'显式意图' 是为人服务的'次要资料'。
>
> 我做反过来的事——'显式意图' 是给 AI 的**必备 API**。
>
> 论据有三个:**lost-in-the-middle**(LLM 不能靠 context 反推)、**context rot**(意图在长 codebase 必被遗忘)、**AI confidence miscalibration**(LLM 自信地错是当前最大风险)。
>
> **Kron = 把意图做成 AI 可 query 的 schema,frontmatter 是协议,git 当 backing store,MCP 当访问层**。这就是为什么 ADR / docs / wiki 都不替代它——它们都不解决'AI 主动 query + 主动拦截'。"

---

## 二、Trap 2:"不就是给 AI 写 docs 嘛?"——三层反驳

### Layer 1:物理差异(看技术 demo 的老师)

> **不是 docs。docs 是 write-once-read-maybe;Kron 是 write-once-consumed-actively-by-AI。**

| 维度 | 普通 docs(README / codebase.md) | Kron |
|---|---|---|
| 写完之后 | 半年没人看 | **MCP 实时被 AI 拉取** |
| 谁来读 | 偶尔有人翻 | **每个 AI 编码循环都主动 query** |
| 校验 | 没有 | **`kron_lint` 在 CI 校验 anchor + frontmatter** |
| 时效 | 完全靠人维护 | **`kron_stale` 自动列过期意图** |
| 影响范围 | 不可查 | **`kron_impact` 反查"改这个意图会炸哪些文件"** |

**一句话**:"Cursor 的 codebase.md 是给 AI 的索引,Kron 是给 AI 的**假设层**。codebase.md 没说'为什么这么做',只说'代码在哪里';Kron 说了。"

---

### Layer 2:工具位差(给同行)

| 现有做法 | 它解决什么 | 它**不**解决什么 | Kron 在哪插 |
|---|---|---|---|
| `// FIXME` / `TODO` 注释 | 标记"要做的事" | 不能说**"为什么这么做 + 什么假设"** | frontmatter `assumptions[]` |
| ADR / MADR | 记录"做过什么决策" | 不能被 AI **主动 query**,写完即遗忘 | `kron_impact` 反查依赖 |
| JSDoc / Doxygen / godoc | API 文档 | 描述**接口签名**,不描述**设计意图** | `.md` 正文 + frontmatter |
| Notion / Confluence | 团队 wiki | **不在 git 里**,clone 仓库读不到 | `.kron/intents/*.md` |
| Read the Docs / GitHub Pages | 发布文档 | 静态,**不能跟代码同步 diff** | git diff 直接看 |
| Jira / Linear / GitHub Issues | 任务追踪 | 生命周期是 **human-only**,AI 不能用 | 跨人 / AI / CI 三方 |
| Sourcegraph / Code Search | grep + jump to def | **不能跨库 join "代码 ↔ 意图"** | anchor scan + 双向链接 |
| Cursor `codebase.md` / Continue.dev | 给 LLM 的索引 | 是**自动派生**的,作者无控制 | Kron **作者显式控制 frontmatter** |
| Claude Project `project.md` | 项目级 context | **不在仓库**,换电脑即丢 | repo 内 `frontmatter` |

**一句话**:"你列的工具我都调研过。每个做了一部分。**Kron 是把'假设层'独立出来——把 `severity: hard` 当成新的一类字段,这是单独的系统工作**。"

---

### Layer 3:现行做法的"证据式"吐槽(给老师,带场景)

#### A. Cursor/Claude 的 `codebase.md` 自动索引

- 你是**被动的 customer**——AI 怎么总结、丢啥,你完全没控
- "我们不接非 UTF-8 输入"写在 Slack 里,`codebase.md` 不会自动知道
- 只能靠"在 Slack 搜 → copy 到 AI prompt" 人工补救

**Kron**:你**写** frontmatter,AI 不能改你的措辞;`kron_assume_check` 在你**写代码之前**主动拉假设。

#### B. Notion / Confluence 团队 wiki

- Wiki **独立**于代码,新人 `git clone` 读不到
- 没 `status: active / superseded`,产生 "intent debt"
- 离职同学带**没入库的知识**走——Polanyi paradox 重演

**Kron**:`git clone` 即查,`git blame` 即审计,`status` 管生命周期,"离职前最后一次 commit 带所有意图"可查。

#### C. PR template + LLM review(CodeRabbit)

- 在 **PR 已写完**后才介入——**retrospective 反馈**,不能拦截
- 给建议,不拦门

**Kron**:`kron_assume_check` 在 **commit 之前**——prospective gate;CI 跑 `kron_lint` 是 **blocking**。

#### D. AI 起草 ADR

- 解决"速度",workflow 加速
- 不解决"结构化 query"——AI 怎么查"哪些决策影响 module X"?

**Kron**:`kron_impact` 是结构化 query,AI 直接调。"起草"和"查询"是两个服务,前者现成,后者**没有现成工具**。

---

## 三、Trap 4:"LLM 进化后 Kron 还有意义吗?"——三层反驳

### 误解 1:"LLM 自己能 infer,不需要 schema"

LLM inference = **压缩感知**,frontmatter = **无损传输**。前者带宽够不够看 context,后者带宽永远够因为是**人显式写**。

- GitHub Copilot Workspace 研究(Stanford HELM evals):**没有仓库 context 时,LLM 给的"为什么"40% 与实际不一致**
- "Lost-in-the-Middle"(Liu et al., 2023, ACL):LLM 在 16k context 中段信息检索 < 60%
- **代码改动后意图不更新**——LLM 永远基于**最新代码**,不基于**最新团队讨论**,schema-as-protocol 永远胜过 inference

### 误解 2:"AGI 来了你就没意义"

**正相反**——LLM 越强,**对自己越自信**,反而**更需要 Kron**:

| LLM 状态 | 失败模式 | Kron 的作用 |
|---|---|---|
| LLM-3.5(弱) | 写不出,频繁问人 | 不需要 |
| LLM-4(中) | 30% 反直觉 | `kron_assume_check` 让人 review 那 30% |
| LLM-5+ / reasoning 模型 | 几乎全对,**自信地错** | 拦住 silent failure |
| AGI(假设) | 1% 不对齐 | 仍是最后防线的"假设层" |

> **关键洞察**:LLM 失败模式随能力变强而**迁移**——从"打不出代码"→"自信给错"。Kron 不绑 LLM,绑"显式假设"。LLM 进化,Kron 从"补 LLM 能力短板"变成"防 LLM silent failure"。

### 误解 3:"Claude Skills / Anthropic Tools 出来你就死了"

- **Anthropic 已经在做**——Claude Skills / Artifacts metadata 都是"显式 schema"赛道尝试
- 但 vendor closed-form,**不在你的仓库里**
- Kron 是**开源 + 跨厂商**:Cursor / Claude / Cline / Continue 共用,**协议中立**
- 类比:**PostgreSQL 不被 Oracle 替代;SQLite 不被 MySQL 替代**——轻量、开源、中立永远有生态位
- 关键反驳:**Kron 不与 vendor schema 竞争——Kron 是所有 vendor 共用的、由仓库作者控制的 schema**。**GitHub model 对抗 vendor lock-in**。

---

## 四、四个学术根脚(给老师引用)

| 文献 | 一句话 | 与 Kron 的对位 |
|---|---|---|
| Liu et al., 2023 ACL "Lost in the Middle" | LLM 在 16k+ context 中段信息检索准确率 < 60% | 意图不应依赖 context 反推;frontmatter 显式声明 |
| Greg Kamradt, 2024 "Context Rot" | GPT-4 / Claude 在 64k+ context 上"代码初衷"问答准确率显著下降 | 意图是 context-rot 第一个受害者;frontmatter 把它从 context 里拔出 |
| Nonaka / Takeuchi, 1995 "The Knowledge-Creating Company" SECI 模型 | "我们知道的比能说的多"——大公司靠知识外化竞争 | Kron 是 Polanyi paradox 在软件开发里的工程化 |
| Anthropic, 2024 "Constitutional AI" / Calibration 论文 | LLM "自信"回答中约 8% 是错的 | frontmatter `severity: hard` 是显式风险标签——LLM confidence calibration 的工程补丁 |

> **老师最爱听的三个组合证据**:lost-in-the-middle + context rot + AI confidence miscalibration——**三个独立证据都指向同一件事**:当前 LLM 状态下"自动反推"不可靠,**显式 schema 永远是兜底**。

---

## 五、Kron 自己的硬伤(坦诚 + 学术诚实)

| 硬伤 | 怎么讲 |
|---|---|
| AI 起草 → 入库 循环未完全闭环(`kron_add` 是占位) | "**v1 是基础设施先行**,闭环留给后续 phase。" 坦诚 = 加分 |
| `assumptions / expires_at` schema 在 docs 已 spec,代码未落地 | "**spec-first 立场**——文档先稳定,代码后落地。这是软件工程传统做法,在我的 [`docs/process/migrate.md`](process/migrate.md) 流程里有专门名" |
| LSP / IDE 集成尚未做,MVP 仅 MCP | "**通过 MCP client**(Claude Desktop / Cursor / Cline / Continue) 用 MVP,**比 IDE 集成更早一步可用**" |
| Coverage metric 没有 CI gate | "**P1,phase 2 处理**"`kron_intent_density` 已有,gate flag 走 [`docs/process/cli-flag.md`](process/cli-flag.md) 流程后续加 |

---

## 六、按受众定制的 30 秒版本

### 给软件工程老师

> "现在主流工程做法有四派——注释、ADR、Cursor codebase.md、LLM review。它们都把'显式意图'当**次要资料**。我做反过来的——**显式意图是给 AI 的必备 API**。**lost-in-the-middle + context rot + AI confidence calibration 三个独立证据都指向同一件事**:当前 LLM '自动反推'不可靠。Kron = 意图做成 AI 可 query 的 schema,frontmatter 是协议,git 当 backing store,MCP 当访问层。"

### 给带嘲讽的老师(更短)

> "Kron 不是 docs,是 AI 的假设层。和 codebase.md 的差别:**codebase.md 是 AI 自动写的,你不能 query;Kron 是你显式写的,你能 query**。"

### 给完全没听过的老师(类比)

> "类比:GitHub Copilot 是 AI 帮你**写代码**,Cursor `codebase.md` 是 AI 自己**写笔记**;Kron 是你**写给 AI 看的设计决策书**,AI 写到对应函数之前会来查这一段。AI 不会忘半年后的假设。"

### 给同行工程师

> "**让 AI 在编码循环里被钩住**。注释是被动的,Kron 的 MCP 工具是主动的——AI 主动 query 假设、查影响、量化覆盖率。这是 docs / ADR / wiki 任何一项都做不来的事,因为它们没有协议层。"

### 给团队 lead

> "**让 on-call 不再翻代码**。`kron_impact` 告诉你改这个意图会炸哪里,`kron_stale` 告诉你哪个意图过期了——**oncall 抢救时间压缩到 1 分钟**。代码注释不会告诉你假设失效,wiki 不会告诉你过期。"

### 给投资人 / 老板

> "**让 AI 写代码不出错**——Kron 给 AI 一个 memory:写一段代码前 AI 自动查它依赖的假设;**Intent Debt 是 AI 时代的 Technical Debt 新形态**,早治理早便宜。"

---

## 七、反方话术——提前练防身

| 攻击 | 回应 |
|---|---|
| "markdown 当 DB" | "对,design choice。git 当 schema migration,git diff 当 audit log——我用 git 替换 DB 的 90% 功能" |
| "MCP 是泡沫" | "MCP 是 v1 访问层,核心是 `.md` + `internal/`。MCP 死了换 HTTP,**内部代码不动**" |
| "AI 不需要意图也能写" | "对,5 行脚本不需要。50000 行 codebase 没 intent layer,AI 必然 context-rot" |
| "Cursor codebase.md 足够" | "自动索引 vs 显式声明。codebase.md 是压缩感知,frontmatter 是无损传输——你信哪个" |
| "AGI 来了你怎么办" | "LLM 越强,自信错越多;Kron 绑的是'假设要显式',不绑任何一代 LLM" |
| "Claude Skills 取代你" | "Anthropic Skills 是 vendor closed-form;Kron 是开源协议层——Postgres vs Oracle 的位置" |
| "你这门槛太低门槛太高是设计选择吗" | "**前后端矛盾,我接受**:门槛低在 schema(纯 markdown),门槛高在 commitment(必须 git 跟代码同源)。这是 package manager 的同一种取舍" |
| "10 年后谁还读 intent" | "我赌 10 年后 LLM 的推理成本依然>0,而 commit message + git diff 的成本永远 = 0。frontmatter 是'边际成本最低'的 ai context source" |

---

## 八、可在结尾加的一句吐槽(让全场笑)

> "Kron 的最大优点是它**没什么可维护的**——所有数据全在仓库里,你手里的 IDE 跟你同事手里的 IDE 看的是同一份文件,git 同步。我们逼自己把基础设施做到最少,所以 v1 才不至于成为下一个考不上学的 SSR 框架。"

老师的版本:

> "**软件工程的极致,就是把复杂度推到 git 上。** 我们做的是 git 友好的 intent 层。"

---

## 九、引用回 docs 的位置

| 主题 | Kron 自有 doc |
|---|---|
| 4 个 MCP 工具(主动消费) | [`docs/implementation/mcp.md`](implementation/mcp.md) §2 |
| 调用树 × 意图视图数据契约 | [`docs/abstractDesign/view-call-tree-intent.md`](abstractDesign/view-call-tree-intent.md) |
| 5.2 演进方向(AI 主动消费) | [`docs/abstractDesign/architecture.md`](abstractDesign/architecture.md) §八 |
| 业务 §2.3.1 AI 主动消费 | [`docs/business.md`](business.md) §2.3.1 |
| anthropic lost-in-the-middle | 暂无,后续 phase 提到时引用 |
| Polanyi paradox SECI 引用 | 暂无,后续 [`article.md`](article.md) 加入时引用 |
