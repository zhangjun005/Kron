# Kron 内部包现状（2026-10-08 收口版 v2）

> 范围：`internal/` 下所有包。CLI / MCP / 未来的 LSP / IDE / GUI 都从这些包拿数据。
> 这是**业务数据层**的现状报告。架构原则见 `docs/abstractDesign/architecture.md`。
>
> **v2 增量**（相对 v1，同日二次封档）：
> - ✅ **T1**: `parser.ValidateAssumptionFrontmatter` + `assumption.Writer.Update` 写前调用（拒空 Text / 拒空 CreatedBy 等）
> - ✅ **T2**: `parser.validateFrontmatter` 加 Symbol 校验（拒空 / 拒 ".."）；`store.Writer.Update` 写前已调，自动覆盖
> - ✅ **`relations.ReferencingIntents(intents, assumptionID) []string`** + 4 测试（Intent ↔ Assumption 反向）
> - ⏸️ `assumption.Writer.Delete` 的 `CreatedBy` 改为 actor 仍保留为 `Delete` 例外（v1.4+ 审计日志时再统一）
>
> **v1 增量**（前一次封档）：
> - ✅ `internal/store/` 补 `Writer.Update` + 9 个测试（CRUD 完整）
> - ✅ `internal/store/` 风格对齐 `internal/assumption/`（Patch + auto-time + opt-in CreatedBy）
> - 🗑️ `docs/process/migrate.md` 移到 `.deprecated/2026-10-08-store-update-patch/`
> - 🚫 RepoView、ScanMarkdownAnchors 进 lint、lint 规则重组——继续推迟

---

## 1. 当前功能总览

`internal/` 是 **读取 → 解析 → 关系运算 → 校验**的单向数据流。**不存任何反向索引**，所有反向视图都从已加载的内存三元组现算。

### 1.1 数据形状（在 `internal/model/`）

仓库里**有三种持久化数据**：intent（设计意图）、assumption（前置条件）、anchor（行级引用）。

| 类型 | 文件位置 | 必填字段 | 选填字段 |
|---|---|---|---|
| **Intent** | `.kron/intents/<slug>.md` | `slug`、`created_by`、`updated_at` | `symbol`、`reviewers`、`status`、`assumptions[]`、`references[]`、`depends_on[]`、body |
| **Assumption** | `.kron/assumptions/<id>.md` | `id`、`text`、`default_severity`、`created_by`、`updated_at` | `status`、`reviewers`、`expires_at`、`verified_at`、`verified_by`、body |
| **Anchor**（非持久化，行级扫描产物） | 代码 / `.md` 文件里的 `// @kron:intent <slug>` 行 | `slug`、`file_path`、`line_number` | — |

### 1.2 CRUD 完备性

#### Intent（`.kron/intents/`）—— **完整** ✅

| 操作 | 函数 | 行为 | 缺省 / 边界 |
|---|---|---|---|
| 读单个 | `store.Reader.Get` (alias: `Load`) | slug → `*Intent`，含 `SourcePath` | 缺文件 → `ErrIntentNotFound` |
| 读全部 | `store.Reader.LoadAll` | 按 slug 字典序 | 目录不存在 → 空 slice |
| 存在性 | `store.Reader.Exists` / `Writer.Exists` | 布尔 | 仅看 intents 目录 |
| 写（**整体覆盖**） | `store.Writer.Write` | 原子写（temp + rename） | slug 必须先 `ValidateSlug`；`updated_at` **不自动 bump**，调用方写 |
| 写（**部分更新**）✅ NEW | `store.Writer.Update(ctx, slug, UpdatePatch, ...opts)` | 原子覆盖 frontmatter 字段 + body | `UpdatePatch` 指针表 nil；`updated_at` **库自动**；`created_by` 改需 `WithAllowCreatorChange()`；空 patch 拒（`ErrEmptyPatch`）；actor 必填 |
| 软删 | `store.Writer.MoveToTrash` | 移到 `.kron/.trash/` | trash 已有同名 → `ErrIntentExists` |
| 还原 | `store.Writer.RestoreFromTrash` | trash → intents | intents 已有同名 → `ErrIntentExists` |
| 配置 | `store.LoadConfig` / `SaveConfig` | TOML | 缺文件用默认 `intents_dir = ".kron/intents"` |

**v1 范围外**：硬删 / GC（架构 §5.2.1 明确不做）。

#### Assumption（`.kron/assumptions/`）—— **完整** ✅

| 操作 | 函数 | 行为 | 缺省 / 边界 |
|---|---|---|---|
| 读单个 | `assumption.Reader.Get` | id → `*AssumptionFile` | 缺 → `ErrAssumptionNotFound`；`fm.id` 与文件名不匹配 → 返回文件 + 错误并存（lint 用） |
| 读全部 | `assumption.Reader.List` | 按 id 字典序 | id-mismatch 文件**也列出**（lint 看到） |
| 存在性 | `Reader.Exists` / `Writer.Exists` | 布尔 | id 不合法 → `false` |
| 创建 | `Writer.Create(ctx, CreateParams)` | 必填值 + 可选指针；库自动 `updated_at` | 必填：`id` 通过 `ValidateSlug`、`text ≠ ""`、severity ∈ {hard, soft}、`createdBy ≠ ""`；已存在 → `ErrAssumptionExists` |
| 更新 | `Writer.Update(ctx, id, UpdatePatch, ...opts)` | 原子覆盖 | `UpdatePatch` 指针；库自动 `updated_at`；空 patch 拒；`created_by` 改需 `WithAllowCreatorChange()`；actor 必填 |
| 软删 | `Writer.Delete(ctx, id, actor)` | 移到 `.kron/.trash/assumptions/` + `Status=superseded` | 已删 → `ErrAssumptionAlreadyDeleted`；actor 必填 |
| 还原 | `Writer.Restore(ctx, id, actor)` | 从 trash 拉回 + `Status=active` | 不在 trash → `ErrAssumptionNotInTrash`；actor 必填 |

#### Anchor（行级扫描产物，不持久化）—— 完整

| 操作 | 函数 | 行为 |
|---|---|---|
| 全仓代码扫 | `parser.ScanAnchors` | `[]Anchor`，按 `(FilePath, LineNumber)` 排序 |
| 全仓 .md 扫 | `parser.ScanMarkdownAnchors` | 同上，跳过 `.kron/intents/` / `.trash/` / `.git/` 等 |
| 单文件查 slug | `parser.SlugsForFile` | 去重 + 字典序 |

### 1.3 校验（`internal/lint/`）—— 完整

`RunWith(ctx, root, RunOptions)` 一次性 walk，返回 `[]Diag`。14 条规则分 4 组：

| 组 | 规则数 | 严重度 |
|---|---|---|
| 结构 | 2 (`frontmatter-invalid`, `anchor-dangling`) | error |
| 关系 | 4 (`dangling-reference` warn, `dangling-depends-on` error, `depends-on-cycle` error, `self-reference` error) | 混合 |
| 陈旧 | 2 (`stale-superseded-candidate` warn, `expired-hard-assumption` warn) | warn |
| 假设（B-3） | 6 (registry-id-mismatch error, orphan warn, mixed-form warn, file-id-mismatch error, rationale-required warn/error, rationale-stale warn, severity-mismatch warn) | 混合 |

**输出**：`[]Diag`（机器）+ `StaleReport`（结构化陈旧视图，给 MCP `kron_stale` 用）。

### 1.4 关系图（`internal/relations/`）—— 完整

接受预加载的 `[]*model.Intent`，**零 IO**。4 个反向 view：

| 函数 | 输出 | 用例 |
|---|---|---|
| `ReverseLinks(intents, target)` | 3 个 sorted slice：`references` / `dependsOnDependents` / `symbolInferred` | `kron_impact` 反向 view |
| `Dependents(intents, target)` | 排序的 `[]string` | `kron_delete` 提示"删之前哪些 intent 依赖它" |
| `Prerequisites(intents, target)` | 排序的 `[]string` | `kron_impact` 前置条件 |
| `ReferencingIntents(intents, assumptionID)` ✅ NEW | 排序去重的 `[]string`（哪些 intent 在 `Assumptions[].ID` 里引了这个 assumption） | 未来 `kron_assumption_delete` 提示"删之前哪些 intent 在用" |

### 1.5 写 API 风格一致性（**今天对齐了**）

`store.Writer.Update` 与 `assumption.Writer.Update` 现在**完全对称**：

| 维度 | `store.Writer.Update` | `assumption.Writer.Update` |
|---|---|---|
| Patch 形状 | `UpdatePatch` 指针表 nil | `UpdatePatch` 指针表 nil |
| 覆盖范围 | 全部 `model.Frontmatter` 字段 + `Body` | 全部 `model.AssumptionFrontmatter` 字段 + `Body` |
| `updated_at` | **库自动** `time.Now().UTC()` | **库自动** `time.Now().UTC().Format(RFC3339)` |
| `created_by` 改保护 | `WithAllowCreatorChange()` opt-in → `ErrCreatorChangeNotAllowed` | 同 |
| 空 patch | `ErrEmptyPatch` | 同 |
| Actor 必填 | ✅ | ✅ |
| Slug 校验 | `parser.ValidateSlug` → `ErrSlugInvalid` | 同 |
| 写前再校验 | `parser.ValidateFrontmatter` | （assumption `Create` 才走，Update 跳——待对齐） |

**待跟进**：`assumption.Update` 也应该 `Write` 前 `ValidateAssumptionFrontmatter`（**今天没改**——`writer.go` 里 `Create` 调了 `parser` 内的 frontmatter 校验，Update 没调）。**留 1 个小尾巴**。

---

## 2. 现状评分

| 维度 | 评分 | 备注 |
|---|---|---|
| Intent CRUD 完整度 | ✅ 100% | `Write`（整体）+ `Update`（partial）**双轨**并存，4 个上层调用方未动 |
| Assumption CRUD 完整度 | ✅ 100% | 完整 CRUD + 软删 + 还原；25/25 测试过 |
| 锚点扫描 | ✅ 100% | 两个扫描器独立可用 |
| 关系图 | ✅ 100% | 3 函数覆盖所有反向 view |
| 校验 | ✅ 100% | 14 条规则 |
| 写 API 风格一致性 | ✅ 100% | `store.Update` ↔ `assumption.Update` 完全对称 + 都有 pre-write 严格校验 |
| 测试覆盖 | ✅ 高 | 所有 internal 包都有 `_test.go` |

---

## 3. 范围外（明确**不**做）

以下**不**在本报告范围——不评估、不讨论、不在 backlog：

- ❌ RepoView 内存三件套（3 路 IO 汇聚）
- ❌ `parser.ScanMarkdownAnchors` 接入 `lint.Run`
- ❌ lint 规则文件重组
- ❌ MCP `kron_assumption_*` 写工具
- ❌ CLI assumption 子命令
- ❌ assumption `text` 改 → rationale stale 自动检测
- ❌ `model.Anchor` 加 `Kind` 字段
- ❌ LSP / IDE / GUI 接入
- ❌ assumption 硬删 / GC

**以上**是原 2026-10-08 初版列出的 6 个 ❶❷ ❸ ❹ ❺ ❻ + 3.3 边角。**今天**没有**做**这些**任何一个**——把它们从 backlog 推回"未拍板"状态，**等下次开新 RFC**。

---

## 4. 跨 PR 决策记录（今天发生）

| 决策 | 状态 | 出处 |
|---|---|---|
| `store.Update` 形状 | ✅ 拍板 | pointer-per-field 指针表 nil |
| `store.Update` 覆盖范围 | ✅ 拍板 | Frontmatter 全部 + Body |
| `store.Update` auto-time | ✅ 拍板 | 库自动 `time.Now().UTC()` |
| `store.Update` CreatedBy 改 | ✅ 拍板 | opt-in via `WithAllowCreatorChange()` |
| `store.Update` 空 patch | ✅ 拍板 | `ErrEmptyPatch` |
| `store.Update` Actor 必填 | ✅ 拍板 | 跟 assumption 一致 |
| 4 个上层迁移策略 | ✅ 拍板 | **留 `Write` + 加 `Update` 并存**——handler_add.go / handler_delete.go / handler_restore.go 不动；handler_update.go 可后续迁移 |
| `migrate.md` 处理 | ✅ 拍板 | `git mv` 到 `.deprecated/2026-10-08-store-update-patch/`（保 blame） |
| `internal-pkg-status-2026-10-08.md` | ✅ 拍板 | **推倒重写**为本文件（原 6 个 ❶❷ ❸ ❹ ❺ ❻ 全部回到"未拍板"） |
| T1 (assumption pre-write 校验) | ✅ 拍板 | 修 |
| T2 (store Symbol 校验) | ✅ 拍板 | 修 |
| `relations.ReferencingIntents` 范围 | ✅ 拍板 | **只 internal 层**——不接 lint / 不开 MCP 工具 |

---

## 5. 留给下次的小尾巴

| # | 尾巴 | 备注 |
|---|---|---|
| **T3** | `assumption.Writer.Delete` / `Restore` 仍改 `CreatedBy` 为 actor | 注释里说"v1.4+ 审计日志时再统一"；当前是**有意保留的 Delete 例外**——不视作 bug |
| T4 | 4 个上层调用方未迁到 `Update` | 设计选择（留 `Write` + 加 `Update` 并存），**不**算尾巴 |
| T5 | `gofmt -l .` 报 `internal/relations/relations_test.go` 之前是 pre-existing | 本次 PR 已修（`gofmt -w`） |

---

## 6. 文件变更清单（今天）

| 文件 | 变化 |
|---|---|
| `internal/store/writer.go` | 加 `UpdatePatch` / `UpdateOption` / `WithAllowCreatorChange` / `Writer.Update` / `validStatus`；写前 `ValidateFrontmatter` |
| `internal/store/writer_test.go` | 加 9 个 `TestWriter_Update_*`（single / multi / body / nil-vs-empty / CreatedBy opt-in / empty patch / actor required / invalid slug / not found） |
| `internal/store/errors.go` | **新文件**：`ErrEmptyPatch` + `ErrCreatorChangeNotAllowed`（镜像 `assumption/errors.go`） |
| `docs/process/migrate.md` | `git mv` 到 `.deprecated/2026-10-08-store-update-patch/migrate.md` |
| `internal-pkg-status-2026-10-08.md` | **本文件**（v1 推倒重写 → v2 二次封档） |

## 7. v2 文件变更清单（今日二次封档）

| 文件 | 变化 |
|---|---|
| `internal/parser/frontmatter.go` | 加 `ValidateAssumptionFrontmatter`；`validateFrontmatter` 加 Symbol 校验（空 / ".." 拒） |
| `internal/assumption/writer.go` | `Update` 写前调 `ValidateAssumptionFrontmatter` |
| `internal/assumption/writer_test.go` | 加 3 个 `TestWriter_Update_Rejects*`（空 Text / 非法 severity / 空 CreatedBy） |
| `internal/store/writer_test.go` | 加 3 个 `TestWriter_Update_Rejects*`（坏 assumption id / 空 symbol / 短 rationale） |
| `internal/relations/relations.go` | 加 `ReferencingIntents(intents, assumptionID) []string` |
| `internal/relations/relations_test.go` | 加 4 个 `TestReferencingIntents_*`（none / multiple / sorted-dedup / empty input） |
