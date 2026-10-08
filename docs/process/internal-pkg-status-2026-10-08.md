# Kron 内部包现状与路线图

> 范围：`internal/` 下所有包。CLI / MCP / 未来的 LSP / IDE / GUI 都从这些包拿数据。
> 这是**业务数据层**的现状报告，不重复架构原则（见 `docs/abstractDesign/architecture.md`）。

---

## 1. 当前功能总览

`internal/` 是一条**读取 → 解析 → 关系运算 → 校验**的单向数据流。**不存任何反向索引**，所有反向视图都从已加载的内存三元组现算。

### 1.1 数据形状（在 `internal/model/`）

仓库里**有三种持久化数据**：intent（设计意图）、assumption（前置条件）、anchor（行级引用）。三者关系见 §1.4。

| 类型 | 文件位置 | 必填字段 | 选填字段 |
|---|---|---|---|
| **Intent** | `.kron/intents/<slug>.md` | `slug`、`created_by`、`updated_at` | `symbol`、`reviewers`、`status`、`assumptions[]`、`references[]`、`depends_on[]`、body |
| **Assumption** | `.kron/assumptions/<id>.md` | `id`、`text`、`default_severity`、`created_by`、`updated_at` | `status`、`reviewers`、`expires_at`、`verified_at`、`verified_by`、body |
| **Anchor**（非持久化，行级扫描产物） | 代码 / `.md` 文件里的 `// @kron:intent <slug>` 行 | `slug`、`file_path`、`line_number` | — |

**Status 三态**（两种数据类型共享）：`draft` → `active` → `superseded`。intent 可省略表示"不参与生命周期"；assumption 现在必须显式给（默认 `active`）。

### 1.2 CRUD 完备性

#### Intent（`.kron/intents/`）—— 完整

| 操作 | 函数 | 行为 | 缺省 / 边界 |
|---|---|---|---|
| 读单个 | `store.Reader.Get` | slug → `*Intent`，含 `SourcePath` | 缺文件 → `ErrIntentNotFound`；`created_by` 缺省无值（lint 不查） |
| 读全部 | `store.Reader.LoadAll` | 按 slug 字典序返回 | 目录不存在 → 空 slice 不报错；任一文件解析失败 → 第一个失败整体回滚 |
| 存在性 | `store.Reader.Exists` / `Writer.Exists` | 布尔 | 仅看 intents 目录，不看 trash |
| 路径 | `IntentPath` / `TrashPath` / `KronDir` | 纯计算 | — |
| 写（创建/覆盖） | `store.Writer.Write` | 原子写（temp + rename） | slug 必须先 `ValidateSlug`；`updated_at` **不自动 bump**，调用方写 |
| 软删 | `store.Writer.MoveToTrash` | 移到 `.kron/.trash/` | trash 已有同名 → `ErrIntentExists`（保护） |
| 还原 | `store.Writer.RestoreFromTrash` | trash → intents | intents 已有同名 → `ErrIntentExists`（保护） |
| 配置 | `store.LoadConfig` / `SaveConfig` | TOML | 缺文件用默认 `intents_dir = ".kron/intents"`；未知 key 忽略 |

**待补**：硬删 / GC（**v1 范围外**，见架构 §5.2.1）。

#### Assumption（`.kron/assumptions/`）—— **不完整**

| 操作 | 函数 | 行为 | 缺省 / 边界 |
|---|---|---|---|
| 读单个 | `assumption.Reader.Get` | id → `*AssumptionFile` | 缺 → `ErrAssumptionNotFound`；`fm.id` 与文件名不匹配 → **返回文件 + 错误并存**（lint 用来报 `RuleAssumptionFileIdMismatch`） |
| 读全部 | `assumption.Reader.List` | 按 id 字典序 | id-mismatch 的文件**也列出**（lint 能看到） |
| 存在性 | `Reader.Exists` / `Writer.Exists` | 布尔 | id 不合法 → 返 `false` |
| 创建 | `Writer.Create(ctx, id, text, defaultSeverity, createdBy)` | 写新文件 | 必填：`id` 通过 `ValidateSlug`、`text ≠ ""`、severity ∈ {hard, soft}、`createdBy ≠ ""`；已存在 → `ErrAssumptionExists` |
| 更新 | `Writer.Update(ctx, id, fm, body)` | 原子覆盖 | `fm.ID` 必须等于 id；文件不存在 → `ErrAssumptionNotFound` |
| **软删** | **❌ 未实现** | — | — |
| **还原** | **❌ 未实现** | — | — |
| **ListTrashed** | **❌ 未实现** | — | — |

**已知 BUG**（PR 1 修复）：`Create` 把 `updated_at` 写死成字符串字面量 `"TODO"`，**所有**通过 `Create` 创建的 assumption 都有 `updated_at: TODO` 字段（当前 6 个 B-3 规则不查这个字段，所以没炸，但语义上坏）。

#### Anchor（行级扫描产物，不持久化）—— 完整

| 操作 | 函数 | 行为 |
|---|---|---|
| 全仓代码扫 | `parser.ScanAnchors` | 返回 `[]Anchor`，按 `(FilePath, LineNumber)` 排序 |
| 全仓 .md 扫 | `parser.ScanMarkdownAnchors` | 同上，**跳过** `.kron/intents/`、`.kron/.trash/`、`.git/`、`node_modules/` 等；围栏内行不计数 |
| 单文件查 slug | `parser.SlugsForFile` | 去重 + 字典序 |
| 反向查（**待补**，PR 3） | **❌ 未实现** | 见 §3 |

### 1.3 校验（`internal/lint/`）—— 完整

`RunWith(ctx, root, RunOptions)` 是一次性 walk，返回 `[]Diag`。**零外部 IO 副作用**。14 条规则分 3 组：

| 组 | 规则 | 严重度 |
|---|---|---|
| **结构** | `frontmatter-invalid`、`anchor-dangling` | error |
| **关系** | `dangling-reference` (warn)、`dangling-depends-on` (error)、`depends-on-cycle` (error)、`self-reference` (error) | 混合 |
| **陈旧** | `stale-superseded-candidate` (warn, 默认 90 天)、`expired-hard-assumption` (warn) | warn |
| **假设（B-3）** | `assumption-registry-id-mismatch` (error)、`assumption-orphan` (warn)、`assumption-mixed-form` (warn)、`assumption-file-id-mismatch` (error)、`assumption-rationale-required` (warn/error 取决于 MigrationMode)、`assumption-rationale-stale` (warn)、`assumption-severity-mismatch` (warn) | 混合 |

**输出**：`[]Diag`（机器）+ `StaleReport`（结构化陈旧视图，给 `kron_stale` MCP 工具用）。

**副作用控制**：`opts.StaleDaysThreshold < 0` 关掉 S 类规则；`opts.MigrationMode` 控制 `rationale-required` 的严重度。

### 1.4 关系图（`internal/relations/`）—— 完整（**只看 frontmatter 关系**）

接受预加载的 `[]*model.Intent`，**零 IO**。提供 3 个反向 view：

| 函数 | 输入 | 输出 | 用例 |
|---|---|---|---|
| `ReverseLinks(intents, target)` | 单 slug | 3 个 sorted slice：`references`、`dependsOnDependents`、`symbolInferred` | `kron_impact` 反向 view |
| `Dependents(intents, target)` | 单 slug | 排序的 `[]string` | `kron_delete` 提示"删之前哪些 intent 依赖它" |
| `Prerequisites(intents, target)` | 单 slug | 排序的 `[]string`（= `depends_on` ∪ 符号推断，explicit 优先） | `kron_impact` 前置条件 |

**`symbolInferred` 算法**：两个 intent 的 `frontmatter.symbol` 集合有交集 → 软依赖。**`explicitSet`（已在 `depends_on` 里的）赢**，不重复列。

### 1.5 数据流向（"读取 → 解析 → 关系运算 → 校验"）

```
   ┌──────────────────────────────────────────────────────┐
   │                  持久化层（磁盘）                     │
   │  .kron/intents/<slug>.md     ─┐                      │
   │  .kron/.trash/<slug>.md      ─┤                      │
   │  .kron/assumptions/<id>.md   ─┤                      │
   │  .kron/config.toml            │                      │
   │  代码/.md 里的 // @kron:intent │ 锚点行（无文件级边界）│
   └───────────────────────────────┼──────────────────────┘
                                   │
        ┌──────────────────────────▼──────────────────────────┐
        │  读：store.Reader / assumption.Reader / config       │
        │       parser.ScanAnchors / parser.ScanMarkdownAnchors│
        │       纯磁盘 I/O，无业务逻辑                          │
        └──────────────────────────┬──────────────────────────┘
                                   │
                                   ▼
        ┌──────────────────────────────────────────────────┐
        │  解析：parser.ParseFrontmatter / SplitMarkdown     │
        │       model.{Intent, AssumptionFile, Anchor}      │
        │  → 内存里的结构化数据                                │
        └──────────────────────────┬──────────────────────────┘
                                   │
        ┌──────────────────────────▼──────────────────────────┐
        │  # 【这里应该有】解析后的"三件套"临时内存对象         │
        │  （见 §3 优化项 ❶）                                  │
        └──────────────────────────┬──────────────────────────┘
                                   │
        ┌──────────────────────────▼──────────────────────────┐
        │  关系 / 校验：                                        │
        │   relations.{ReverseLinks, Dependents, Prerequisites}│
        │   lint.RunWith                                       │
        │   全部消费同一个 []*model.Intent + 内存三件套        │
        └──────────────────────────┬──────────────────────────┘
                                   │
                                   ▼
        ┌──────────────────────────────────────────────────┐
        │  出口：                                             │
        │   CLI  `kron lint`        → 文本 + 退出码         │
        │   MCP  `kron_lint`        → JSON-RPC              │
        │   MCP  `kron_impact`      → ReverseLinks + 锚点    │
        │   未来 LSP / IDE / GUI  → 同内存三件套             │
        └──────────────────────────────────────────────────┘
```

**关键点**：所有"反向"和"交叉"查询都**现算**，**不存反向索引**（这是项目铁律，与 markdown 单一数据源同步）。代价是每次 MCP call 重新 walk；好处是磁盘即真相，零同步问题。

### 1.6 读取 / 解析的"三件套"（**现状 = 散在三处，需要在内存里合一**）

每次 `kron_impact` / `kron_lint` / `kron_stale` 调用，**上游其实都同时需要**这三组数据，但**目前每次都各调各的**：

| 数据组 | 来源 | 现在的调用次数 |
|---|---|---|
| **Intent 集合** | `store.Reader.LoadAll(ctx)` | 1 次（每工具内） |
| **Assumption 集合** | `assumption.Reader.List(ctx)` | 1 次（`kron_lint` 内），其他工具 0 次 |
| **Anchor 集合** | `parser.ScanAnchors` + `parser.ScanMarkdownAnchors` | `kron_lint` 1 次（仅 code），其他工具 0 次 |

**问题**：
- 三个数据组**没有任何统一的 in-memory 容器**包在一起
- 每次工具调用都重新走磁盘，没有"已经加载好"的快照
- `kron_impact` 想同时看 references / depends_on / symbol / **incoming_anchors**（RFC 拍板），得**自己分别**调 4 个底层 API
- LSP 客户端要 hover 某行（需要 anchor + 关联 intent），现在没有"在一个包里拿全"的入口

**期望的临时内存对象**（PR 3 阶段产物，**你拍板的语义**，我只是表达）：

```
type RepoView struct {
    Root         string
    LoadedAt     time.Time            // 一次扫描的时间戳，缓存判断用
    Intents      []*model.Intent      // LoadAll 的结果
    Assumptions  map[string]*AssumptionFile  // List + id 索引 O(1)
    CodeAnchors  []model.Anchor       // ScanAnchors
    MdAnchors    []model.Anchor       // ScanMarkdownAnchors
}

// 现算反向视图（无副作用）
func (v *RepoView) ReverseBySlug(slug string) ReverseView  // references + dependents + symbolInferred + incoming_anchors
func (v *RepoView) AnchorsPointingTo(slug string) []AnchorLocation
func (v *RepoView) StaleView(now time.Time, thresholdDays int) StaleReport
```

**关键约束**（跟你过去定的原则对齐）：
- **不存**任何反向索引 → 所有 view 是**现算**的
- `RepoView` 是**单次调用内的临时对象**，不缓存，不持久化
- 调用方拿 `*RepoView` 跟现在拿 `[]*model.Intent` 一样自由

---

## 2. 现状评分

| 维度 | 评分 | 备注 |
|---|---|---|
| Intent CRUD 完整度 | ✅ 100% | 包括软删、还原、配置 |
| Assumption CRUD 完整度 | ⚠️ 70% | 缺软删 / 还原 / ListTrashed；1 个 BUG（TODO 字面量） |
| 锚点扫描 | ⚠️ 50% | 两个扫描器在但**没汇聚**；MD 扫描器是孤儿函数；无反向查 API |
| 关系图 | ✅ 100% | 3 个函数覆盖所有反向 view（基于 frontmatter） |
| 校验 | ✅ 100% | 14 条规则覆盖结构 / 关系 / 陈旧 / 假设 |
| 三方数据汇聚 | ❌ 0% | 每次调用各扫各的，没有 `RepoView` |
| 数据层 → 出口 | ✅ 100% | CLI / MCP 都接得对 |
| 测试覆盖 | ✅ 高 | 所有 internal 包都有 _test.go |

---

## 3. 需要补全 / 优化的事项

按"业务数据层"视角分两类：**新功能** 和 **修缮**。每条都附业务动机，不是为了"代码好"。

### 3.1 P0 — 必修（数据/语义坏）

#### ❶ 修 `Writer.Create` 的 `"TODO"` 字面量 BUG
**业务动机**：现在 `migrate.go` 调 `Create` 出来的所有 assumption 文件，`updated_at` 字段是字面量 `"TODO"`。任何后续按 `updated_at` 排序 / 筛选 / 展示陈旧度的功能（`kron_stale` MCP 工具、未来的 assumption 时间线）都会被这个值污染。**6 个 B-3 规则暂时不查它所以没炸**——是定时炸弹。

**修复**：`Create` 自动写 `now()`；`Update` 自动写 `now()` 且**不动** `created_by`；`Status` 默认 `active`。

#### ❷ 补 `assumption` 的软删 / 还原 / ListTrashed
**业务动机**：
- 当前 assumption 只能 Create / Update，**没有删除路径** —— 一旦写错 id 就得手动 `rm`
- 跟 intent 行为不一致（intent 有 `MoveToTrash` + `RestoreFromTrash`），破坏统一性
- 未来 lint / MCP 工具要问"哪些 assumption 已废弃但还在用"，没有"软删状态"字段可用

**实现**（已拍板）：共享 `.kron/.trash/`，跟 intent 同目录；`MoveToTrash` 用 slug 改名 + 目录保护（已有同名 → `ErrAssumptionExists`）。

### 3.2 P1 — 强烈建议（数据层缺关键能力）

#### ❸ `RepoView` 内存三件套 + 反向锚点查询
**业务动机**（你点出来的）：
- `kron_impact` 想报"哪些文件/哪行锚了我"——**现在没有任何 API 能给**
- `kron_status` / 未来的 IDE status bar 想"一个调用拿全"——**现在要 4 次 IO**
- LSP hover 想"光标这行 → 这个 intent 的完整画像"——**现在没法在一个事务里拿**

**API 形态**（**不存**反向，只在内存里现算）：
```
func LoadRepoView(ctx, root) (*RepoView, error)
func (v *RepoView) AnchorsPointingTo(slug string) []AnchorLocation
func (v *RepoView) ReverseBySlug(slug string) ReverseView   // 含 references + dependents + symbolInferred + incoming_anchors
```

**跟现有 `relations` 的关系**：`RepoView` 内部调 `relations.ReverseLinks` / `Dependents` / `Prerequisites`，**不重复实现**。`RepoView` = "加载 + 内存化"，`relations` = "纯关系计算"。

#### ❹ `parser.ScanMarkdownAnchors` 接进 `lint.Run`
**业务动机**：README.md 里写 `// @kron:intent auth/jwt`，按 RFC §2.1 应该报 `anchor-dangling`，**实际不报**——`RunWith` 只调了 `ScanAnchors`，没调 MD 扫描器。这是 lint 的**事实性漏报**。

**修复**：`RunWith` 在 `ScanAnchors` 之后追加 `ScanMarkdownAnchors`，合并 `anchors` 列表。Diag.Where 不变（`path:line` 已经够区分）。

#### ❺ `relations.ReferencingIntents(intents, assumptionID)` 反向 API
**业务动机**：
- `kron_delete` 风格的"删之前问 dependents"现在**只对 intent 软链**有效（`relations.Dependents`）
- 对 **assumption** 的反向引用没有现成 API，每个调用方得自己 `for _, in := range intents { for _, a := range in.Assumptions { ... } }`
- 未来 `kron_assumption_delete` MCP 工具直接消费这个函数

**实现**：复用 `relations` 包的 in-memory 风格，2 小时。

### 3.3 P2 — 修缮（代码组织 / 健壮性）

#### ❻ `lint` 规则文件重组
**现状**：6 个 B-3 规则中 1 个（`RuleAssumptionFileIdMismatch`）散在 `lint.go` 里，5 个在 `assumption_rules.go`。**RFC §6 拍板的"每条规则自包含"违反**。

**业务动机**：未来加 `RuleAssumptionReviewersEmpty` / `RuleAssumptionVerifiedStale` 等规则时，**没有统一签名**会导致走读混乱。3 小时纯重组，零行为变化。

#### ❷ 几个**已发现**的边角问题（顺手修）

| 问题 | 影响 | 工作量 |
|---|---|---|
| `internal/parser/anchors.go` 末尾的 `var _ = filepath.Join` 占位（防止 goimports 抱怨 unused import） | 丑 | 5 分钟 |
| `internal/parser/markdown_anchors.go` 同样的占位 | 丑 | 5 分钟 |
| `identity.Handle` 的 `"agent:"` 前缀判断里没排除 `"agent:"` 单独成字符串的情况 | 边界 | 10 分钟 |
| `model.Anchor` 缺 `kind` 字段，没法区分 code 锚点 / md 锚点 | ❸ 修完自动解决 | 顺手 |

### 3.4 P3 — 范围外（按 RFC 拍板 v1 推迟）

- ❌ MCP `kron_assumption_*` 写工具（v1.4 之前不暴露）
- ❌ CLI `kron assumption` 子命令（v1.4 之前不暴露）
- ❌ LSP / IDE / GUI 接入（不在 v1 必需范围）
- ❌ assumption `text` 改 → rationale 自动 stale 检测（边界模糊，留 RFC）
- ❌ `ScanMarkdownAnchors` 的 symbol 推断（RFC 没拍）
- ❌ assumption 硬删 / GC（架构 §5.2.1 明确不做）

---

## 4. 推荐执行顺序

```
本周
  ❶ (15 分钟)  ─┐
                ├─ 一个 PR，1.75 小时
  ❷ (1.5 小时) ─┘

下周
  ❸ (4-6 小时)  ─┐
                 ├─ 一个 PR，5-7 小时；含 model.Anchor 加 Kind 字段
  ❹ (1 小时)    ─┘
  ❺ (2 小时)    ── 独立 PR

月底
  ❻ (3 小时)    ── 纯重组
  ❷' 边角 (30 分钟) ── 跟 ❻ 一个 PR
```

**总投入**：~14 小时（约 2 个工作日），6 个 PR。

---

## 5. 跨 PR 待你拍板的 3 个决策点

1. **`RepoView` 落哪个包**——`internal/parser`（跟扫描器一起）？还是新建 `internal/view`（独立）？
2. **`model.Anchor` 加 `Kind` 字段的兼容性**——加完字段后所有 7 处调用点跟着改？还是用"两个结构体共存"的过渡方案？
3. **`Diag.Where` 是否给 MD 锚点加 kind 后缀**（`README.md:42` → `README.md:42 [md]`）？还是维持现状（用户看 `path` 后缀就懂）？
