# 意图系统结构设计

> 本文档描述意图系统的具体结构。
> 措辞保持建议性，留出实现灵活性；层级用目录表达，依赖用相对链接表达。

---

## 一、代码锚点（Code Anchors）

**建议语法：** `// @kron:intent <意图路径>`

### 意图路径定义

- 意图路径 = 意图文件相对于 `.kron/intents/` 的路径（不含 `.md` 后缀）
- 例：`.kron/intents/auth/jwt-sliding-window.md` → 意图路径 `auth/jwt-sliding-window`
- 例：`.kron/intents/storage-format.md` → 意图路径 `storage-format`

### 目录简写

- 目录下的 `README.md` 可简写为目录名本身
- 例：`auth/README.md` 既可写 `auth/README`，也可简写为 `auth`
- 实现时建议两种写法都接受

### 跨语言兼容

- 行级注释前缀（`//`, `#`, `--` 等）均可
- 意图路径放在锚点指令之后，注释行内唯一标识

### 贴合位置（建议）

- 注释建议贴在目标符号（函数、结构体、接口、类）紧邻的上一行
- 避免在锚点与目标符号之间插入空行
- 这是 IDE 跳转定位的最佳实践，扫描器可据此降低误匹配率

### 适用范围（2026-10-08 拍板, [`docs/rfc/2026-10-08-md-anchors.md`](../../rfc/2026-10-08-md-anchors.md)）

`// @kron:intent <slug>` **不仅代码可用, 仓库所有 .md 文件也可用**. 适用范围:

| 文件类型 | 扫描? | 备注 |
|---|---|---|
| 源代码 (`*.go`, `*.ts`, `*.py`, ...) | ✅ | 原行为 |
| 根目录 + 子目录 `*.md` (`README.md`, `docs/**/*.md`, ...) | ✅ | 新增 (仓库内**任何** .md) |
| `.kron/intents/*.md` | ❌ | 自身即意图, 不需锚定自身 |
| `.kron/.trash/*.md` | ❌ | 软删状态, 不扫描 |
| 围栏代码块 (\`\`\`\` ... \`\`\`\`) 内的 `// @kron:intent` | ❌ | 维持 in-fence 状态机, 避免假阳 |

**语法与代码完全一致**, 不引入 MD 专属语法. 例 (`README.md`):

```markdown
// @kron:intent auth/refresh-token
# Refresh Token 滚动过期策略

> 引用的开发背景说明 ...
```

### 解析路径

- 文件查找规则（按序探测）：
  1. 探测 `.kron/intents/<意图路径>.md`（单文件）
  2. 若为目录简写，探测 `.kron/intents/<意图路径>/README.md`（目录总览）
- 意图路径中的 `/` 对应文件系统分隔符（Windows 上需统一归一化为正斜杠 `/`）

---

## 二、目录结构（Filesystem as Hierarchy）

### 核心原则：单意图单文件（One Intent, One Markdown）

> **铁律**：一个 Intent 对应一个 `.md` 文件，禁止多意图聚合在同一文件内。

- **原子性**：每个 `.md` 文件有且仅记录一个具体的意图/决策闭环。
- **禁止堆砌**：禁止在单个 `.md` 文件内混合记录多项平级架构决策；宁可拆成同级目录下的多个文件，也不堆砌在单个大文件中。
- **协同收益**：细粒度的文件级隔离，保证多人/多 Agent 并行提交时 Git 冲突概率降至极低，且便于精准向 LLM 投喂上下文。

> *"一个 Intent 对应单一决策闭环；一个 Intent 独占一个 `.md` 文件。细粒度的物理隔离，是抵御 Git 冲突与上下文膨胀的第一道防线。"*

### 基础约定

- 根目录：`.kron/intents/`
- 文件夹 = 模块分组
- `.md` 文件 = 单条意图
- 意图目录与源代码目录是独立的两套，各自按需组织；具体映射机制留待后续讨论
- 命名建议：kebab-case，全英文，避免空格和特殊字符

### 示例结构

```
.kron/intents/
├── README.md
├── storage/
│   ├── README.md
│   └── storage-format.md
└── auth/
    ├── README.md
    ├── jwt-sliding-window.md
    └── password-hasher.md
```

---

## 三、意图文件内容模板

### Frontmatter

> **本文档保留的 frontmatter 仅用于元数据，层级靠目录表达。**
> 意图间的**显式依赖**和**软引用**靠结构化字段（见下方"关系字段"小节），不是 markdown 相对链接。

**建议字段：**
- `symbol`：单字符串或字符串列表；关联的代码符号（函数 / 结构体 / 接口名）
- `created_by`：人类用户直接写 `@<github_handle>`（如 `@zhangjun005`），AI 生成写 `agent:<model/tool>`（如 `agent:claude-3.7-sonnet`）
- `updated_at`：ISO 8601 时间戳
- `reviewers`：（可选，多人协作时使用）签名列表，`@<github_handle>` 数组，声明对该意图决策进行过 Review 的人员
- `status`：（可选）`draft` / `active` / `superseded`，详见下方"status 字段策略"
- `assumptions`：（可选）可验证前提列表，详见下方"Assumptions 字段策略"
- `references`：（可选，v1.2+）软引用关系列表，详见下方"关系字段"小节
- `depends_on`：（可选，v1.2+）硬依赖关系列表，详见下方"关系字段"小节

### `status` 字段策略

**完全可选，不设默认值。**

不填 `status` = 未参与生命周期管理，等价于"该文件存在即有效"。

理由：
- **静默默认值制造歧义**：默认 `active` 会让草稿误标为已达成共识；默认 `draft` 又强迫单人用户每次手动"激活"。
- **契合 Kron 的人格**：`README.md` 明确声明"无双源持久化、无 mtime+hash 状态机冲突处理"。可选字段正好保持这一基调。
- **强约束交给 CI**：如果团队要求所有新意图必须显式声明 `active`，让 `kron lint` 在 CI 里**报错**而非默认值兜底——既不静默，也不会让所有 `.md` 都多出一行无意义 yaml。

`status` 三态语义：
- `draft`（草稿/待 Review）
- `active`（已达成共识）
- `superseded`（被新方案替代，文件保留不删）

### `assumptions` 字段策略

**完全可选，默认为空数组，等价于"该 intent 无显式假设"。**

语义：每条假设表达"这段代码在什么前提下成立"。假设显式化后，AI Agent 和 LSP 可主动消费。

字段结构（**B-3**，RFC [`2026-10-08-assumptions-standalone.md`](../../rfc/2026-10-08-assumptions-standalone.md)）：
```yaml
assumptions:
  - id: <短机器名>           # 唯一标识，对应 .kron/assumptions/<id>.md（**不**含 "/"）
    text: <自然语言描述>      # （可选）从 registry 读，inline 留空以完成 A→B 迁移
    severity: hard | soft     # 必需：本意图级别（跨意图可不同）
    rationale: "<≥10 字符>"   # 必需：解释"为什么本意图这 severity"
    expires_at: "YYYY-MM-DD" # （可选）人工估量的重新审视截止日期
    verified_at: "YYYY-MM-DD" # （可选）最近一次人工确认该假设仍成立的日期
    verified_by: "@handle"   # （可选）确认人
```

**B-3 关键变化**（v1.3, 落地无迁移）：

- **共享 `text` 存于 registry**：`.kron/assumptions/<id>.md` 里的 `text:` 是单一真理源；intent frontmatter 的 `text:` 是迁移期兼容的 inline copy，B-3 起**不再需要**——新写的 intent **直接**只引用 id 即可
- **`default_severity` 取代 `severity`**（在 registry 文件里）：表示"该假设通常多严"；intent 各自的 `severity` 字段是 ground truth
- **`rationale` 必填**（≥ 10 字符）：避免"凭直觉设 hard/soft"的反模式；v1.3 迁移期是 Warning，v1.5 改 Error
- **跨意图 severity 可不同**：同一条假设（`single-region`）在 `auth/jwt.md` 可能是 `hard`、在 `ui/console.md` 可能是 `soft`，各自的 `rationale` 解释"为什么"
- **本仓库无存量数据**：不需要假设迁移脚本——见 [RFC §1.2](../../rfc/2026-10-08-assumptions-standalone.md) "无存量迁移"

`severity` 语义：
- `hard`：假设破裂时，相关代码逻辑必须修改。例如"Redis 可用性 ≥ 99.9%"破了意味着 token 吊销完全失效，必须改。
- `soft`：假设破裂时，代码仍可工作，但存在性能或功能降级。例如"DAU ≤ 10K"破了意味着 Redis 内存压力上升，需要评估。

`expires_at` 用途：
- 不驱动任何自动行为
- 仅供 LSP hover 提示变色（MCP `kron_assume_check` 也可读）
- 到期不报错，只提醒"该重新审这条假设了"

`id` 命名建议：kebab-case，如 `single-region`、`blacklist-fits-ram`、`clock-skew-30s`。

### 关系字段（`references` / `depends_on`，v1.2+）

> **动机**：markdown 相对链接只对人类阅读有意义，工具链（lint / MCP / IDE）看不见。结构化字段把"这条 intent 与哪些其他 intent 有关系"变成机器可消费的事实。详见 [`docs/rfc/2026-10-03-frontmatter-references.md`](../../rfc/2026-10-03-frontmatter-references.md)（RFC 提案）+ [`architecture.md`](../abstractDesign/architecture.md)（架构依据）。

**两个字段，语义不同**：

| 字段 | 类型 | 语义 | 对称性 | 工具表现 |
|---|---|---|---|---|
| `references` | `[]string`（slug） | 软引用 / see-also / 推荐阅读 | 对称（双向记录均可） | `kron_lint`: 悬空 → warning；`kron_impact`: 列入 `references` |
| `depends_on` | `[]string`（slug） | 硬依赖 / 理解本 intent 前必须先读 | 反对称（A→B 不蕴含 B→A） | `kron_lint`: 悬空 → **error**；`kron_delete`: 列出 dependents（soft warn，不阻塞）；`kron_impact`: 列入 `prerequisites` |

**示例**：

```yaml
<!-- kron:frontmatter -->
symbol: ["auth.RefreshToken"]
created_by: "@zhangjun005"
updated_at: "2026-10-03T10:00:00Z"

references:                          # 软：灵感来源 / 推荐阅读
  - oauth2-best-practices
  - session-timeout-ux
depends_on:                          # 硬：必须先读才能理解本 intent
  - auth/token-storage
  - auth/csrf-protected
<!-- /kron:frontmatter -->
```

**校验规则**：

- 每个条目必须是合法 slug（见 [`parser.ValidateSlug`](../../implementation/domain-model.md)）——kebab-case 段、可含 `/` 分组、不可含 `..` / 不可 `.md` 后缀。
- 自引用（A.references 含 A） → 错误。`kron_update` 写时校验；`kron_lint` 扫时校验。
- `depends_on` 形成环（A→B→A） → `kron_lint` 报 `depends-on-cycle` 错误。
- 引用目标不存在 → soft = warning；hard = error（见上表）。
- **不需要双向记录**：A.depends_on B 不要求 B.depends_on A。
- **不替代 markdown 相对链接**：链接保留作人类阅读；字段是给工具的。

### 正文骨架

```markdown
# 意图名称

> 一句话简述该决策的核心目的。

## 为什么（Why）
记录"当初为何如此决策"。

## 权衡（Trade-offs）
选择与放弃的考量。

<!-- 边界假设写在 frontmatter 的 assumptions 字段里，不在正文重复 -->
```

### 字段 / 章节的选用

- 一级标题下的引述块作为**摘要段**，IDE 悬浮预览默认抓取此处
- 二级标题非强制，但 AI 生成时建议遵循，便于结构化解析

### 示例

```markdown
<!-- kron:frontmatter -->
symbol:
  - "auth.RefreshToken"
  - "auth.TokenClaims"
created_by: "@zhangjun005"
reviewers:
  - "@alice"
updated_at: "2026-09-22T10:00:00Z"
status: "active"

assumptions:
  - id: single-region
    severity: hard
    rationale: "跨 region 时 token 失效爆炸, 必须保证单 region 部署"
  - id: csrf-protected
    severity: hard
    rationale: "续期接口已加 CSRF token 防护, 跨意图共享"
<!-- /kron:frontmatter -->

# JWT 滑动窗口续期

> 访问令牌有效期 15 分钟，刷新令牌有效期 7 天；续期时采用滑动窗口策略，用户每次活跃操作都将 token 有效期顺延。

## 为什么（Why）
OAuth2 标准推荐短期令牌 + 刷新策略；15 分钟窗口兼顾安全与用户体验。

## 权衡（Trade-offs）
- ✅ 令牌泄露窗口小
- ❌ 用户每次操作均需写 DB 更新过期时间，高并发下有压力
- 选择：接受写压力，换取安全性
```

---

## 四、相对链接约定（Cross-References）

**建议语法：** `[意图名称](相对文件路径)`

### 识别规则

- 路径落在 `.kron/intents/` 内部的 `.md` 相对链接，IDE 与 Kron 工具均自动识别为意图引用
- 识别依据是**路径**，不是方括号内文本；方括号里写任何人类可读的标签都行

### 三段语义

- `意图名称`：人看的标签，就是显示文本，不需要包含路径
- `(相对文件路径)`：标准 Markdown 相对路径，负责真正的文件寻址；普通编辑器原生支持 `Ctrl+单击` 跳转

### 链接分类

- 内部链接：`.kron/intents/` 范围内的跨条目引用
- 外部链接：指向仓库其他文档（如 `docs/`）——保持标准 Markdown 语法即可，不做意图交互

### 示例

```markdown
### 权衡
- 受 [令牌桶限流算法](../rate-limit/token-bucket.md) 的频次约束
- 复用 [密码哈希方案](password-hasher.md) 的加密规范
```

方括号里写人看的标签，寻址完全交给括号里的相对路径。这避免了方括号重复书写路径，也保持了链接文本的自然可读性。

---

## 五、多人协同机制（Collaboration）

> 本节描述多人/多 Agent 并行协作时的轻量机制，不影响单人使用的极简体验。

### Intent-First Code Review

分支开发时，新增代码与新增/修改的 `.md` 意图文件应同 commit 同 PR。

Review 范式：
1. Reviewer 先看 PR 中的意图文件变更——权衡合理吗？边界条件周全吗？
2. 意图达成共识后，再审查代码实现是否符合意图。

### CI 门禁（Linter）

`kron lint` 可作为 GitHub Actions / CI 门禁，检测以下场景：

- **悬空锚点**：代码里有 `// @kron:intent foo/bar`，但对应的 `foo/bar.md` 不存在 → CI 报错。
- **缺失意图**：修改了核心代码但未附带或更新对应的 intent 文件 → CI 警告。

### 多人协同 Frontmatter 扩展

```yaml
<!-- kron:frontmatter -->
status: active
symbol: "auth.RefreshToken"
created_by: "@alice"
reviewers:
  - "@bob"
updated_at: "2026-09-22T10:00:00Z"
<!-- /kron:frontmatter -->
```

- `created_by`：决策发起人
- `reviewers`：对该决策进行过 Review 背书的人员列表
- `status`：`draft`（草稿/待 Review）→ `active`（已达成共识）→ `superseded`（被新方案替代）

---
