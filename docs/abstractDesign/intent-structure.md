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

> **本文档保留的 frontmatter 仅用于元数据，不表达层级或依赖关系。**
> 层级靠目录，依赖靠相对链接。

**建议字段：**
- `symbol`：单字符串或字符串列表；关联的代码符号（函数 / 结构体 / 接口名）
- `created_by`：人类用户直接写 `@<github_handle>`（如 `@zhangjun005`），AI 生成写 `agent:<model/tool>`（如 `agent:claude-3.7-sonnet`）
- `updated_at`：ISO 8601 时间戳
- `reviewers`：（可选，多人协作时使用）签名列表，`@<github_handle>` 数组，声明对该意图决策进行过 Review 的人员
- `status`：（可选）`draft` / `active` / `superseded`，详见下方"status 字段策略"

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

### 正文骨架

```markdown
# 意图名称

> 一句话简述该决策的核心目的。

## 为什么（Why）
记录"当初为何如此决策"。

## 权衡（Trade-offs）
选择与放弃的考量。

## 边界假设（Invariants / Assumptions）
必须遵守的前提与限制。
```

### 字段 / 章节的选用

- 一级标题下的引述块作为**摘要段**，IDE 悬浮预览默认抓取此处
- 二级标题非强制，但 AI 生成时建议遵循，便于结构化解析

### 示例

```markdown
---
symbol:
  - "auth.RefreshToken"
  - "auth.TokenClaims"
created_by: "@zhangjun005"
reviewers:
  - "@alice"
updated_at: "2026-09-22T10:00:00Z"
---

# JWT 滑动窗口续期

> 访问令牌有效期 15 分钟，刷新令牌有效期 7 天；续期时采用滑动窗口策略，用户每次活跃操作都将 token 有效期顺延。

## 为什么（Why）
OAuth2 标准推荐短期令牌 + 刷新策略；15 分钟窗口兼顾安全与用户体验。

## 权衡（Trade-offs）
- ✅ 令牌泄露窗口小
- ❌ 用户每次操作均需写 DB 更新过期时间，高并发下有压力
- 选择：接受写压力，换取安全性

## 边界假设（Invariants / Assumptions）
- 刷新令牌存储在 HttpOnly Cookie 中，不经过 JS
- 续期接口需 CSRF 保护
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
---
status: active
symbol: "auth.RefreshToken"
created_by: "@alice"
reviewers:
  - "@bob"
updated_at: "2026-09-22T10:00:00Z"
---
```

- `created_by`：决策发起人
- `reviewers`：对该决策进行过 Review 背书的人员列表
- `status`：`draft`（草稿/待 Review）→ `active`（已达成共识）→ `superseded`（被新方案替代）

---
