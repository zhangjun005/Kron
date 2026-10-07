# RFC: Frontmatter 结构化引用字段 (`references` + `depends_on`)

> **本 RFC 已落地 + 已归档** (2026-10-03, commit `a373828` + `ea186a8`)。作为**历史决策记录**保留。schema / lint / MCP 工具的"v1.2 来源"以 [`docs/abstractDesign/intent-structure.md`](../../abstractDesign/intent-structure.md) §三 为准（事实层真理），本文**不再作为决策源**。

| 字段 | 值 |
|---|---|
| **状态** | **已落地 (Implemented) + 已归档 (2026-10-06 移入 `docs/rfc/archive/`)** — commit `a373828 feat(frontmatter): add references + depends_on per RFC 2026-10-03` (2026-10-03)；后续 `ea186a8 feat(serve-mcp): polish tools_list_get + relations edge cases` 打磨边缘 case。本 RFC 顶栏更新滞后，参考 [`../2026-10-04-mcp-sdk-adoption.md` §10 第 350 行](../2026-10-04-mcp-sdk-adoption.md) 的归档标注。 |
| **作者** | AI assistant, 经 zh-jun 委托 |
| **创建日期** | 2026-10-03 |
| **目标版本** | v1.2 — **已落地** (2026-10-03) |
| **影响范围** | frontmatter schema / lint / MCP 工具 / 三份事实文档 |

---

## 1. 动机 (Motivation)

### 1.1 当前现状

Kron 的 intent 之间目前**没有结构化的引用关系字段**。意图之间的关联仅靠两种隐式机制:

| 机制 | 形态 | 工具是否消费 |
|---|---|---|
| Markdown 相对链接 | `参见 [auth/jwt](auth/jwt.md)` | **不消费**(普通 markdown,自动 spawn 遍历时跳过) |
| `frontmatter.symbol` 字符串交集 | `symbol: ["pkg.Foo"]` | `kron_impact` 推断 (推断式,非显式) |
| Code anchors | `// @kron:intent <slug>` | `kron_impact` / `kron_lint` 消费 (代码→intent,反向) |

### 1.2 问题:三种关系无法表达

| 想表达的关系 | 当前能怎么做 | 缺点 |
|---|---|---|
| **软引用** (see-also / 灵感来源 / 阅读路径) | 正文写 markdown 链接 | 工具看不见、AI 看不见、`kron_impact` 不返回 |
| **硬依赖** (理解本 intent 前必须先读) | 同上 | 同上,且**无 lint 校验**(链接 target 改名 = silent broken) |
| **被反向引用关系** | `kron_impact` 返回 `depends_on_intents`(基于 symbol 推断) | 推断不准(symbol 字符串巧合匹配会被误算) |

### 1.3 现实证据: `references-snapshot.md` 自身的反面教材

[`docs/process/references-snapshot.md`](../process/references-snapshot.md) 是一个**因为 markdown 链接不可靠而被迫存在的文件**:

- 2026-09-26 生成的快照,**§4.1 / §4.2 连续两个小节标记"待重生成"**
- §五 提出"写个 regen 脚本"——5 周后仍未落地,只能手工跑 `rg`
- 文件存在的全部意义 = **追踪 markdown 链接的漂移**

如果 markdown 链接真可靠,这个 281 行的快照文件**不需要存在**。

### 1.4 工具链缺口

| 工具 | 当前能力 | 加字段后 |
|---|---|---|
| `kron_list` | 按 slug 前缀过滤 | 不变 |
| `kron_get` | 返回 frontmatter + body | 返回新增 `references` / `depends_on` |
| `kron_impact` | 返回 `depends_on_intents` (基于 symbol 推断) | 拆为 `references` / `prerequisites` 两组,**显式优先于推断** |
| `kron_delete` | 检查 anchor 反向引用 | 检查 dependents (depends_on 引用者) 并 soft-warn |
| `kron_lint` | 检查 dangling anchor | 新增 2 规则:`dangling-reference` (warn) / `dangling-depends-on` (error) |

---

## 2. 提案 (Proposal)

### 2.1 新增 2 个可选字段

```yaml
<!-- kron:frontmatter -->
symbol: ["auth.RefreshToken"]
created_by: "@zhangjun005"
updated_at: "2026-10-03T10:00:00Z"

# 新增 ↓
references:                                  # 软引用 —— see-also / 灵感来源 / 阅读路径
  - oauth2-best-practices
  - session-timeout-ux
depends_on:                                  # 硬依赖 —— 理解本 intent 前必须先读
  - auth/token-storage
  - auth/csrf-protected
<!-- /kron:frontmatter -->
```

### 2.2 字段语义

| 字段 | 语义 | 对称性 | 类型 | YAML tag |
|---|---|---|---|---|
| `references` | 软引用。指向其他 intent 时表达"see-also / 灵感来源 / 推荐先读" | **对称** (双向记录均可,但语义无方向性) | `[]string` (slug 列表) | `references,omitempty` |
| `depends_on` | 硬依赖。本 intent 的成立**依赖**于目标 intent 的前提;目标失效时本 intent 必须重新审视 | **不对称** (A depends_on B 不蕴含 B depends_on A) | `[]string` (slug 列表) | `depends_on,omitempty` |

**slug 形态**:与现有 `symbol` 字段的语义无关——这里**只**作为"intent 相对路径"`(`Audit/jwt.md` → `auth/jwt`)。复用 `parser.ValidateSlug` 校验。

### 2.3 工具表现差异(关键)

| 场景 | `references` (软) | `depends_on` (硬) |
|---|---|---|
| `kron_get` wire shape | 列表展示 | 列表展示 + ⚠ hard dependency 标签 |
| `kron_impact` 输出 | `related_intents: [slug]` | `prerequisites: [slug]` (与 symbol 推断结果合并时显式优先) |
| `kron_delete` 删 target 后 | (无影响) | 返回 `dependents: [a, b, c]` 列表, soft warn,**不强制 block** |
| `kron_lint` 悬空 | `dangling-reference` (**warning**) | `dangling-depends-on` (**error**) |
| AI 消费语义 | "可读可不读" | "必须先读,否则可能误读本 intent" |

### 2.4 与现有机制的关系

- **不废弃** markdown 相对链接——它们仍然是人类阅读体验的一部分
- **不替代** `symbol` + code anchor——那是代码→intent 方向
- **新增** intent→intent 显式结构化字段——机器可消费

---

## 3. 字段一致性约束 (Validation)

### 3.1 必做校验

- 每个引用必须是合法的 slug (`parser.ValidateSlug`)
- 不允许自引用 (`references` 含自身 slug / `depends_on` 含自身 slug → 错误)
- 不允许循环 `depends_on` (A → B → A 检测,lint 规则而非 schema 校验)

### 3.2 不做校验

- 不要求引用目标必须存在 — **存在性是 lint 职责,不是 schema 职责** (允许"先写引用,目标后建"的工作流)
- 不要求双向对称 — `A references B` 不要求 `B references A`
- 不要求去重 — 重复由 YAML 解析层自动忽略,但 lint 可 warning

---

## 4. 实施计划 (Implementation Plan)

### 4.1 文档同步 (migrate.md §0 三文件原则)

| 层级 | 文件 | 修改 |
|---|---|---|
| **事实层** | `docs/abstractDesign/intent-structure.md` | §三 frontmatter schema 表格加 2 行;新增 §3.1 "字段语义与边界" |
| **事实层** | `docs/abstractDesign/architecture.md` | §七 "不做的事" 表格删除 `parent` / `depends_on` 拓扑建模 一行;改为在 §八 演进方向(或新增章节)说明已落地 |
| **实施层** | `docs/implementation/domain-model.md` | §2 `Frontmatter` struct 加 2 字段 + 注释 |
| **实施层** | `docs/implementation/mcp.md` | §2 `kron_impact` 输出 shape 改;`kron_get` 返回加 2 字段;`kron_delete` 加 `dependents` 字段 |
| **协作层** | `AGENTS.md` | "Storage format reminder" 段加 references / depends_on 示例 |
| **OFF-limits** | `AGENTS.md` | 删除"Adding `parent` / `depends_on` to frontmatter" 一行 (off-limits 不再适用) |

### 4.2 代码变更清单

| 文件 | 改动 | 估时 |
|---|---|---|
| `internal/model/intent.go` | `Frontmatter` struct 加 `References []string` + `DependsOn []string` 字段,带 yaml tag + godoc | 15min |
| `internal/parser/frontmatter.go` | `validateFrontmatter` 改用白名单模式 (见 §5.1);新增 `validateReferences` / `validateDependsOn` (复用 `ValidateSlug` + 自引用检查) | 30min |
| `cmd/kron/serve-mcp/tools_list_get.go` | `intentFrontmatterWire` 加 2 字段;`intentSummaryWire` 暂不变 | 10min |
| `cmd/kron/serve-mcp/handlers_impact.go` | 输出拆为 `references` (symmetric 显式) + `prerequisites` (depends_on + symbol 推断合并,显式优先);从 `frontmatter` 直读,**不**走 store 反查 | 20min |
| `cmd/kron/serve-mcp/handlers_delete.go` | 删除前扫描全 store,返回 `dependents: []string` (反向 `depends_on` 引用者列表) | 20min |
| `internal/lint/` (新规则) | `dangling-reference` (warning) + `dangling-depends-on` (error) + `depends-on-cycle` (var) | 1h |
| 测试 | 表驱动 + round-trip + 工具 wire shape 测试 | 1h |

总估时: **~3.5h**

### 4.3 兼容性与迁移

**用户面**:项目目前无外部用户 (`AGENTS.md` 顶部"Phase 1 closure"语境 + README 显示内部工具),不存在迁移成本。

**代码面**:`internal/parser/frontmatter.go:172` 当前用 `dec.KnownFields(true)`(严格模式,未知字段报错)。本 RFC 提议**保留严格性但改为白名单**(见 §5.1),原因是:

- 严格性有价值 (schema drift 应报错,而非 silent 丢弃)
- 但**当前实现把"strict"和"hardcoded 字段集"耦合** — 新增字段时必须改代码
- 白名单模式把"strict"与"已知字段集"解耦 — 新增字段时改一处即可

### 4.4 步骤

1. **RFC review** (zh-jun) ← 你
2. 改 `internal/model/intent.go` + parser 校验 + 测试
3. 改 `cmd/kron/serve-mcp/` 三个 handler
4. 改 `internal/lint/` 加 3 规则
5. 跑 `go vet ./... && gofmt -l . && go test ./...`
6. 三文件同步 (按 §4.1 表)
7. 单 commit:`feat(frontmatter): add references + depends_on structural fields (RFC 2026-10-03)`

---

## 5. 待决问题 / 设计抉择

### 5.1 `KnownFields(true)` 处理方式

**当前问题**:`frontmatter.go:172` 的 `KnownFields(true)` 让 yaml.v3 在解码时**严格拒绝**未识别字段。加 `references` / `depends_on` 后:
- 老 `.md` 文件无这 2 字段 → 没问题 (omitempty)
- 但若其他工具/手写加进未知字段 (例如注释) → 当前会报错

**3 种方案**:

| 方案 | 优点 | 缺点 |
|---|---|---|
| **A. 改 `KnownFields(false)`** | 改动最小 | 失去 strict 保护 (typo 的字段名 `creede_b` 会被静默吞掉) |
| **B. 硬编码字段列表** | 保留 strict | 每次加字段都要改 parser,容易忘 |
| **C. 维护 known-fields 白名单 (Recommended)** | 保留 strict;新增字段改 1 处;白名单本身在 `internal/model/intent.go` 旁,接近"事实源" | 略增复杂度 |

**建议 C**:在 `internal/parser/frontmatter.go` 新增 `var knownFrontmatterFields = map[string]struct{}{...}`,yaml decoder 用它做 `KnownFields(true)` 的范围。

### 5.2 字段命名

候选:
- `references` / `depends_on` ← **推荐** (贴近语义,贴近现有 `assumptions` 命名风格)
- `related` / `prereq` (更短,但语义模糊)
- `see_also` / `requires` (英文更地道,但偏离 Kron 现有中文主导的命名习惯)

**建议 `references` / `depends_on`**。

### 5.3 `kron_delete` 对 dependents 的响应

**问题**:删 intent B,但 A.depends_on 包含 B。A 没声明 `status: superseded` 时,是否允许删?

**3 种选择**:

| 方案 | 行为 |
|---|---|
| **(a) Soft warn (推荐)** | 列出 dependents 列表 + warning;不强制 block。允许 v1 渐进清理 |
| (b) Hard block | 拒绝删除,提示"先处理 dependents" |
| (c) Auto-supersede dependents | 自动改 A 的 status 为 superseded |

**建议 (a)**:符合 v1 "不强制,只提示"原则 (与 `kron lint` 一致)。

### 5.4 自引用与循环检测的归属

- **自引用** (A.references 含 A) → schema 校验 (解析时就报错)
- **循环** (A.depends_on B,B.depends_on A) → **lint 规则** (非 schema;解析时无法独立决定,需 LoadAll 全图后才知)

---

## 6. 风险与权衡

### 6.1 风险

| 风险 | 缓解 |
|---|---|
| frontmatter schema 漂移 (`references` / `depends_on` 命名被改) | 白名单 + schema 校验把 typo 抓在 schema 阶段 |
| 误用"严肃"的 depends_on (写软引用到 depends_on) | 文档 + §2.3 工具行为差异表;无技术强校验 |
| `KnownFields(true)` 改动引入回归 | 测试覆盖 round-trip + 旧文件兼容 |
| AI 写入时漏字段 | `kron_lint` 不强制 (都是 optional) |

### 6.2 权衡

- **显式 > 隐式**:多写 2 行 yaml,换取工具链完整语义
- **可读 > 严格**:lint error (depends_on) 比 warning 更早 commit,作为"安全网"
- **简单 > 自动**:`kron_delete` 不自动改 dependents status,留给人决策

---

## 7. 替代方案 (Considered Alternatives)

### 7.1 不加字段,只让 markdown 链接被 `kron_impact` 解析

**否决理由**:与 `references-snapshot.md` 反面教材同样的根本问题——markdown 链接**不是结构化**:
- `[auth/jwt](auth/jwt.md)` 与 `[other label](auth/jwt.md)` 工具无法分辨"是否有意图引用"
- 链接 target 改名 / 文件移动 → silent broken,直到 lint 显式跑
- AI 生成时容易忘(正文链接 vs 字段,后者机器可消费)

### 7.2 加 `parent` 单一字段代替 references/depends_on 二分

**否决理由**:
- `parent` 表达**树**,不表达**关系**(A.depends_on B 不蕴含 B 是 A 的 parent)
- 二分 (软/硬) 是 lint 区分 warning/error 的基础
- 树级已通过文件系统目录表达 (intent-structure.md §二)

### 7.3 加 `links` 单字段,内嵌类型

**否决理由**:
- 类型嵌套使 yaml 更冗长
- 不区分软硬 → 失去工具行为差异的根基
- 与现有 `assumptions` (单字段数组 + 结构化) 的风格不一致

---

## 8. 开放问题 (Open Questions)

需要在 review 时拍板:

1. 字段名最终敲定 → §5.2 (建议 `references` / `depends_on`)
2. `KnownFields` 处理方案 → §5.1 (建议 C)
3. `kron_delete` 对 dependents 行为 → §5.3 (建议 a soft warn)
4. `depends_on` lint 严重度 → §2.3 (建议 error)
5. 是否同步加 `references` (soft) 的 lint 规则 → §2.3 (建议 warning)

---

## 9. 参考 (References)

- [`docs/abstractDesign/intent-structure.md`](../abstractDesign/intent-structure.md) §三 (frontmatter schema 真理)
- [`docs/abstractDesign/architecture.md`](../abstractDesign/architecture.md) §〇铁律 #6, §七 (范围外清单)
- [`docs/process/migrate.md`](../process/migrate.md) (本 RFC 必须遵循的流程)
- [`docs/process/references-snapshot.md`](../process/references-snapshot.md) (本 RFC 引用的现实反面教材)
- [`docs/process/lint-rule.md`](../process/lint-rule.md) (新增 lint 规则时的流程)
- 历史对话: agent-transcripts/359581c1...jsonl L111-112 (前次提议 references/depends_on 的实现,后因 off-limits 未落地)