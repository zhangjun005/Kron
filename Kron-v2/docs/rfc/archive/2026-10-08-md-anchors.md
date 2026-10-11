# RFC: 仓库普通 MD 反向锚点（v1.2+ 扩展）

| 字段 | 值 |
|---|---|
| **状态** | 已拍板 (2026-10-08, zhangjun005 via chat) |
| **作者** | AI assistant, 经 zhangjun005 委托 |
| **创建日期** | 2026-10-08 |
| **目标版本** | v1.2 (frontmatter references 同步期) — 实施可延至 v1.3 |
| **影响范围** | `intent-structure.md` §一 (扩) / `internal/lint` (新规则) / `kron_impact` (扩维度) / `kron_status` (扩报告) / `docs/implementation/error-catalog.md` (新错码) |

---

## 1 动机 (2026-10-08)

`docs/how-it-works.md` 已是事实上"仓库普通 MD 反向引用意图"的实景示例, 但**规范层**尚未拍板. 本 RFC 拍板如下:

### 1.1 用户原话

> ".kron/ 文件夹外的 md 也能使用 `// @kron:intent` 跟 intent 联系起来"

### 1.2 现状空白

| 锚点方向 | 状态 | 来源文档 |
|---|---|---|
| 代码 → 意图 (`// @kron:intent` in `*.go`) | ✅ 已规范 | `intent-structure.md` §一 |
| 意图 → 意图 (frontmatter `references` / `depends_on`) | ✅ 已规范 (v1.2) | `frontmatter-references.md` |
| **仓库普通 MD → 意图** | ❌ **未规范** (仅 `how-it-works.md` 隐式示例) | (本文填补) |

---

## 2 拍板决策

### 2.1 范围

`kron lint` **扫描全仓库所有 .md 文件**, 不仅代码:

| 包含 | 排除 |
|---|---|
| ✅ 根目录 `*.md` (`README.md`, `CONTRIBUTING.md`, ...) | ❌ `.kron/intents/*.md` (单意图铁律, 自身即意图) |
| ✅ `docs/**/*.md` (含 `how-it-works.md`, `architecture.md` 等) | ❌ `.kron/.trash/*.md` (软删状态) |
| ✅ 任何路径下的 `*.md` (含 `frontend/README.md` 若存在) | ❌ `node_modules/`, `vendor/`, `.git/` |
| ✅ 代码文件 (`*.go`, `*.ts`, `*.py`, ...) — **原有**, 不变 | |

### 2.2 语法

**MD 与代码共用同一锚点指令** `// @kron:intent <slug>`. **不**引入 MD 专属语法 (如 HTML 注释).

```markdown
<!-- 任何 .md 文件, 行级锚点 -->
// @kron:intent auth/refresh-token
# Refresh Token 滚动过期策略

> 引用的开发背景说明, ...
```

**理由**:

- **一致性**: 用户 (zhangjun005 2026-10-08 拍板) 选 A — "与代码共用同一种语法"
- **可移植**: `git grep` 一次扫所有, 不分语言
- **MD 文件多是代码块**: 真正"MD 内容里"出现 `//` 视觉上是注释, **不影响** Markdown 渲染 (`// @kron:intent` 不在 ``` ``` 围栏代码块内时, GitHub 渲染为**纯文本行**, 见 §2.4)

### 2.3 lint 行为

```
[error] anchor dangling: README.md:42 -> "auth/login-v1" (file .kron/intents/auth/login-v1.md not found)
[warn]  anchor soft-dangling: docs/rfc/2026-10-07-xxx.md:10 -> "auth/jwt-rotation" (target not yet committed)
```

**规则与代码锚点一致**:

| 错误 | 行为 | 与代码一致? |
|---|---|---|
| 悬空锚点 (目标 .md 不存在) | error | ✅ |
| 自引用 (意图文件锚定自身) | error | ✅ |
| slug 非法 | error | ✅ (走 `parser.ValidateSlug`) |
| 目标存在但 `status: superseded` | warning | ✅ |

### 2.4 视觉与渲染

**重要**: `// @kron:intent slug` 在 `.md` 文件**正文** (不在代码块围栏内) 时:

- **GitHub 渲染**: 显示为普通段落行, **视觉**与代码注释一致
- **VSCode 预览**: 同上
- **人类阅读**: 视觉上是**注释行**, 不影响阅读
- **机器扫描**: `kron lint` `grep` 一次拿到, 不区分

**如何避免误识别 MD 正文里**真的**只是注释的"// xxx"?**

```
任何 .md 文件中满足以下任一条件, 即被识别为锚点:

1. 行首 (去前导空白) 是 `// @kron:intent ` (精确前缀, 带空格分隔)
2. 锚点指令**不在** ``` ``` 围栏代码块内 (lint 解析时维护 in-fence 状态)
```

**为什么** "**带空格分隔**" 防误识别: 普通 `// hello` `// TODO` `// comment` **不**会被 `kron lint` 当锚点 (只精确匹配 `@kron:intent ` 后接 slug).

### 2.5 内部表达

`kron_impact.incoming_anchors` 返回值扩展:

```typescript
{
  incoming_anchors: [
    { location: "internal/auth/jwt.go", line: 42, kind: "code" },
    { location: "README.md", line: 10, kind: "markdown" },
    { location: "docs/design/jwt-flow.md", line: 88, kind: "markdown" },
  ]
}
```

`kind` 字段为新增 (API 增量兼容). `kron_status` 健康报告多 1 行:

```
[kron_status]
...
Reverse anchors per intent:
  auth/refresh-token:    5 code + 3 markdown  (good)
  auth/login-flow:        0 code + 0 markdown  (orphan - no incoming references; check if intent is still relevant)
  ...
```

---

## 3 实施路径

### 3.1 文档同步 (v1.2 文档阶段, 立即)

- ✅ `intent-structure.md` §一 扩: "代码与 MD 共用同一锚点语法 `// @kron:intent <slug>`"
- ✅ `intent-structure.md` §一 加 §1.5 "扫描范围"
- ✅ `how-it-works.md` §2 已隐式示例, 升级为"合规用法"
- ✅ `docs-map.md` §五 "所有文件索引" 注释: `2026-10-08-md-anchors.md` (本文)
- ✅ `implementation/mcp.md` §4 (kron_impact / kron_status) 增 kind 维度

### 3.2 代码实施 (v1.3 GUI 期同步)

- [ ] `internal/lint` 新规则 `markdown-anchor-dangling`
- [ ] `internal/lint` 扫描器读 `.kron/config.toml` (或新字段 `lint.anchor_paths`) — **v1.3 起**
- [ ] `internal/parser` 加 `ScanMarkdownAnchors(path string) []AnchorLocation` 工具
- [ ] `kron_impact` 实现 `incoming_anchors[].kind`
- [ ] `kron_status` 报告维度更新
- [ ] 测试: `.md` 内嵌锚点 → lint 抓到; 围栏代码块内 `// @kron:intent` 不被抓 (假阳)

### 3.3 不做的事

- ❌ **不**引入 MD 专属语法 (HTML 注释 / frontmatter 扩展) — 用户拍板**共用**
- ❌ **不**改 frontmatter `references` / `depends_on` (结构化字段仍是意图间关系真相, 本 RFC 只补**外部** MD 这一个缺失入口)
- ❌ **不**为 MD 锚点增新工具 (`kron_anchor_list` 之类)— 复用 `kron_impact` 即可

---

## 4 与现有 RFC / 文档的关系

| 文档 | 关系 |
|---|---|
| [`intent-structure.md` §一](../abstractDesign/intent-structure.md) | 本 RFC §2.1 §2.2 §2.4 后, §一扩写 |
| [`frontmatter-references.md` (已归档)](./archive/2026-10-03-frontmatter-references.md) | 不冲突: 那是**意图间**结构化字段; 本 RFC 是**外部 MD**反向锚点 |
| [`view-call-tree-intent.md`](../abstractDesign/view-call-tree-intent.md) | §2 受本 RFC 影响 (incoming_anchors 增 kind 维度) |
| [`implementation/mcp.md`](../implementation/mcp.md) §4 | §2.5 实施后, 同步扩 |
| [`internal-pkg.md`](../process/internal-pkg.md) | 不触发 — `internal/lint` 已存在, 本 RFC 是**加规则**, 不**新包** |
| [`new-internal-api.md`](../process/new-internal-api.md) | 不触发 — `ScanMarkdownAnchors` 是新**函数**不是新包 |

---

## 5 关键决策 (本 RFC 拍板后**不可**回退的)

1. **范围**: 仓库**所有 .md** 都被 `kron lint` 扫描 (除 `.kron/intents/` 自己 + `.trash/`)
2. **语法**: 与代码共用 `// @kron:intent <slug>` (一行, 带空格分隔)
3. **防误识别**: 围栏代码块内的 `// @kron:intent` **不**算锚点
4. **影响图**: `incoming_anchors` 增 `kind: "code" | "markdown"` 维度
5. **错误码**: 复用 `model.ErrAnchorDangling`, 不新增

---

## 6 反对意见 (AI 已表达 / 已驳回)

| 反对 | 驳回理由 |
|---|---|
| "MD 锚点与代码冲突, 应分两种语法" | 用户拍板**共用**. 分两种=2 倍学习成本 + 2 倍扫描器; 共用=对齐 grep/copilot/many tools |
| "不应扫全仓库 .md, 应走 allowlist" | 用户拍板**全扫**. allowlist=多 1 配置 surface; 全扫**误识别率近零** (精确前缀匹配) |
| "围栏代码块外的 `// @kron:intent` 视觉上奇怪" | 接受. GitHub 渲染为**普通段落**, 视觉上是**轻量注释**. 比 `[anchor-id]: slug` 自定义语法**不显眼更好** |
