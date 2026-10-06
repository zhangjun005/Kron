# RFC: 反向"源文件"视图 (`SourceFiles`: intent → 锚定代码)

| 字段 | 值 |
|---|---|
| **状态** | 草案 (Proposed) |
| **作者** | AI assistant, 经 zh-jun 委托 |
| **创建日期** | 2026-10-04 |
| **目标版本** | 未定 — 不属于任何已规划 phase。v1.2（[`2026-10-03-frontmatter-references.md`](./2026-10-03-frontmatter-references.md), commit `a373828`, 2026-10-03 已落地）与 phase 2（[`docs/process/phase-2-leftovers.md`](../process/phase-2-leftovers.md), 2026-10-03 全 closed）都**已收口**；phase 3 尚未规划。 |
| **影响范围** | `internal/parser` / `internal/relations` / `internal/lint` / `kron_impact` (未来) |

---

## 1. 动机 (Motivation)

### 1.1 当前现状

`internal/parser/anchors.go` 的扫描方向是**从代码到 intent**：

```
// @kron:intent <slug>     →   检查 slug 是否存在于 .kron/intents/
```

具体证据：

- `parser.ScanAnchors(dir)` 返回全量 `[]model.Anchor`，**没有"按 slug 过滤"入口**。
- `parser.SlugsForFile(filePath)` 只支持"已知文件 → 列出 slug"，不支持"已知 slug → 列出文件"。
- `internal/relations.ReverseLinks / Dependents / Prerequisites` 三个函数全部在 `[]*model.Intent` 切片内找关系，**完全不看源码树**。
- `internal/lint.Run` 顺手扫了所有锚点做 A-class (`anchor-dangling`) 校验，扫完即弃，**不保留任何索引**。

### 1.2 问题：反向视图缺一栏

| 想知道的反问 | 现在的答案 |
|---|---|
| 谁 `references` 我？ | ✅ `relations.ReverseLinks` 返回 |
| 谁 `depends_on` 我？ | ✅ `relations.Dependents` / `ReverseLinks` 返回 |
| 谁跟我 `symbol` 重合？ | ✅ `relations.ReverseLinks` (`symbolInferred`) 返回 |
| **哪些源文件锚定到我？** | ❌ **没有** — `kron_impact` 拿不到这一栏 |

后果（实际场景）：

- 改一条 intent 的 slug 前，想知道"哪些文件要同步改锚点"——**答不上**。
- 删一条 intent 前的安全检查，只看 `depends_on` 反向，不知道有 5 个 `.go` 文件锚定着它。
- IDE hover 想给"本函数由哪条 intent 兜底"的提示——**单边只有 forward**。

### 1.3 跟已有 RFC 的关系

- [`2026-10-03-frontmatter-references.md`](./2026-10-03-frontmatter-references.md) §1.1 表 3 行明确写 "Code anchors: 代码→intent, 反向"——**注意：这个表述有歧义**。该 RFC 描述的是"代码里写了 `@kron:intent X`，X 反向知道这个代码"的能力。但 v1.2（commit `a373828`）实际只解决了"intent → 哪些 intent 引用我"（同质图），**没有解决**"intent → 哪些源文件锚定我"（异质图）。
- 本 RFC 是上述那行"反向"二字的**真正落地**。

### 1.4 与 lint A-class 的重复扫描问题

`RuleAnchorDangling` 已经走过一遍 `ScanAnchors`。如果新增 `SourceFiles` 也独立 `ScanAnchors`，**两条访问路径各扫一次同一棵树**。在仓库较大时会成为隐性 CPU 成本。本 RFC §5.3 给出共用方案。

---

## 2. 提案 (Proposal)

### 2.1 预置：底层能力（v1.2 目标）

新增两个包级函数，**不接入任何 access layer**：

```go
// internal/parser/anchors.go
func AnchorsForSlug(dir, slug string) ([]model.Anchor, error)

// internal/relations/relations.go
func SourceFiles(intents []*model.Intent, root, targetSlug string) ([]model.Anchor, error)
```

约束（写代码前定死，避免日后漂移）：

- `AnchorsForSlug` 复用 `WalkSourceFiles`（共享 skip-dir + binary 过滤）。
- `SourceFiles` 委托给 `AnchorsForSlug`；当 `intents` 仅为满足未来"已知 slug 存在性"前置检查时使用，**主路径是 walk 源码树**。
- `SourceFiles` **不返回 error** 的可能性——见 §5.1 兼容性约束。

### 2.2 不做的事（v1.2 范围内**显式排除**）

- ❌ 不暴露给 `kron_impact` MCP 工具（v1.2 内另开 RFC 决定字段名/JSON schema）。
- ❌ 不暴露给 CLI（`kron where-used` 子命令不在 v1 必需范围，AGENTS.md "off-limits"）。
- ❌ 不引入 `.kron/.cache/anchors.json` 缓存（v1 每次重扫）。
- ❌ 不扩展锚点语法（不识别 markdown 里的裸 slug，不做 symbol 字符串扫描）。
- ❌ 不读 `.kron/.trash/`（软删除的 intent 不参与 `SourceFiles` 关联——见 §5.4）。

### 2.3 决策记录（2026-10-04 用户拍板）

| 决策 | 选项 | 理由 |
|---|---|---|
| 反向来源 | **只看 `@kron:intent` 硬锚点** | 软匹配会被 RFC 文档的非意图引用污染 |
| 性能策略 | **每次调用重扫源码树** | 仓库规模 v1 <10k 文件，单次 walk <0.1s；缓存失效复杂度不值得 |
| 暴露面 | **只预置底层，不接 CLI/MCP** | 留出 RFC 阶段打磨 schema 与 UX |

---

## 3. 设计选择（已讨论，待 RFC 阶段确认）

> 本节记录**已讨论过但 RFC 阶段需要再确认**的开放点。**不是 v1.2 的实现承诺**。

### 3.1 软匹配算不算 source_files

`@kron:intent` 是**显式硬锚点**；markdown 正文里写"参见 auth/jwt"是**软引用**。当前 `AnchorsForSlug` 只匹配硬锚点。RFC 阶段决定：v1.2 是不是**只**接受显式声明？答：是。

### 3.2 单文件多锚点的去重策略

- **Parser 层**：原样返回全部行（保真）。
- **Relations 层**（未来暴露时）：按 `(file, line)` 去重，或聚合为 `{file, first_line, count}`。

v1.2 预置阶段**不强制**去重策略——`SourceFiles` 直传 `[]model.Anchor` 即可，调用方各自决定。

### 3.3 跳过的目录要不要在结果里说明

`source_files` 旁边的"覆盖了哪些目录"信息——v1.2 暂不提供。如未来需要，由 lint 顺手暴露（不是 `SourceFiles` 的职责）。

### 3.4 跨平台路径

`model.Anchor.FilePath` 来自 `WalkSourceFiles`，可能是绝对路径。**未来 MCP 响应必须**转成相对仓库根。v1.2 预置阶段**保留**绝对路径（与 lint 的 `Where` 字段当前做法不同，但 lint 自己做了 `relPath`，parser 层不需要预先归一化）。`SourceFiles` 签名收 `root` 即可：root 同时承担"扫描起点"与"未来相对路径基准"。

### 3.5 命名

候选：`source_files` / `references_from_code` / `anchored_in` / `where_used`。**预置阶段不强制**——RFC 决定 schema 时再定。

### 3.6 与 lint A-class 的解耦

lint `RuleAnchorDangling` 走 `ScanAnchors`，`AnchorsForSlug` 也要走同一棵树。三种解耦方式：

1. **不管**：lint 跑一次（CI 触发），`AnchorsForSlug` 跑一次（按需），重复扫。
2. **缓存**：lint 跑完写 `.kron/.cache/anchors.json`，`AnchorsForSlug` 读缓存。
3. **共用 helper**：parser 加 `Index(dir) (map[string][]Anchor, error)`，lint 和 relations 都调它。**单次 walk，零缓存**。

**倾向方案 3**——把"扫一遍源码树"提到 parser 原语层，lint 和 relations 都站在它之上。

### 3.7 还没想清楚的（v1.2 当天再定）

- MCP 响应里 `LineNumber` 序列化为 `int` 还是 `string`。
- 单文件锚点数量上限（防 1 万行锚点撑爆响应）。
- 软删除的 intent 在 `SourceFiles` 里的归属（**当前倾向不参与**，见 §5.4）。

---

## 4. 测试策略

### 4.1 单元测试

| 函数 | fixture 风格 | 断言 |
|---|---|---|
| `parser.AnchorsForSlug` | tmpdir + 若干 .go / .md / .txt 文件 | (a) 只返回目标 slug；(b) 排序确定；(c) `.kron/`、`node_modules/`、二进制文件都被跳过；(d) 无锚点时返回空切片（非 nil） |
| `relations.SourceFiles` | tmpdir + 实际 `AnchorsForSlug` | (a) 传 `intents=nil` 也能跑（不需要 intent 数据）；(b) `targetSlug` 不在 `intents` 也允许 |

### 4.2 回归风险

- 现有 `parser.ScanAnchors` 的测试（`anchors_test.go`）**不变**——`AnchorsForSlug` 是它之上的薄包装。
- 现有 `parser.SlugsForFile` **不变**。
- 现有 `internal/relations` 的三个函数**不变**——`SourceFiles` 是第四个，签名风格参照它们。
- 现有 lint 测试**不变**——A-class 没动。

### 4.3 跨平台

Windows 下 `filepath.WalkDir` 用 `\`。测试需覆盖：

- `AnchorsForSlug` 的 `FilePath` 输出形式（与 `ScanAnchors` 一致：当前是绝对路径，**没**做 `/` 归一化；这与 lint 行为一致）。
- slug 中 `/` 与磁盘 `\` 的关系（`parser.ValidateSlug` 已禁止 `\`）。

---

## 5. 影响面（"未来改这里时改什么"）

> 本节是本 RFC 的**主要价值**——记录"现有代码里哪些地方会因为本提案变化而需要联动修改"。

### 5.1 `internal/parser/anchors.go`

**新增**：

- `func AnchorsForSlug(dir, slug string) ([]model.Anchor, error)`
- 内部辅助：`func filterAnchorsBySlug(anchors []model.Anchor, slug string) []model.Anchor`（去重 + 排序 + 过滤）

**不变**：

- `ScanAnchors` / `SlugsForFile` / `parseAnchorLine` / `findAnchorMarker` / `scanFile`
- `sortAnchors` / `anchorLess`

**风险点**：

- 排序去重逻辑如果和 `ScanAnchors` 不一致，`AnchorsForSlug` 的输出序就跟"全量扫一遍"得到的同名子集不同——这会让测试难写。**约束**：`AnchorsForSlug` 等价于 "全量扫一遍 + filter by slug"，**逐元素**比较，不重排序。

### 5.2 `internal/parser/walk.go`

**新增（§3.6 方案 3 落地时）**：

- `func Index(dir string) (map[string][]model.Anchor, error)` —— 一次 walk，构建 `(slug → anchors)` 字典。

**不变**：

- `DefaultSkipDirs` / `IsBinary` / `WalkSourceFiles`

**约束**：

- `Index` 必须是 `WalkSourceFiles` 之上的薄壳，**不能**自己再写 walk。否则 skip-dir 列表会漂移。
- 内存预算：v1 假设锚点总数 <10k，map 值是 slice of pointers（共享 `model.Anchor` 实例），整体 <1MB。

### 5.3 `internal/lint/lint.go`

**未来变化（v1.2+ 接 MCP 时）**：

- A-class 的循环里，把当前"对每个 anchor 调 `r.Exists`"改为"对每个 anchor 调 `Index` 的反向字典查 `r.Exists`"——但**单次 walk 变成 O(1) 查表**。性能提升，但**逻辑不变**（仍然每个 dangling anchor 一条 Diag）。

**v1.2 预置阶段不变**。lint 不动。

### 5.4 `internal/store/reader.go`

**不直接相关**，但有约束：

- `LoadAll` 当前**不**过滤 `.kron/.trash/`。如果 `intents []*model.Intent` 切片里混进了 `.trash` 里的 intent（理论上 LoadAll 只看 `intents/` 不看 `trash/`，所以**不会**），`SourceFiles` 的"targetSlug 不在 intents"分支要正确处理空切片。
- 验证：`store.LoadAll` 路径已经在 `walkIntentSlugs` 里硬编码 `model.KronDir/model.IntentDir`，**不会**踩 `.trash/`。**安全**。

### 5.5 `internal/relations/relations.go`

**新增**：

- `func SourceFiles(intents []*model.Intent, root, targetSlug string) ([]model.Anchor, error)`

**约束**：

- **不**返回 error 的可能性：当前 `relations` 包里所有函数**都**返回 `[]string` 不带 error，因为不读 IO。新加的 `SourceFiles` 要读 IO——**会破坏签名一致性**。
- 折中：把 `SourceFiles` 放在 `relations` 包里（语义上属于"跨 intent 关系"），但**接受 error**。调用方需要用 `errors.Is` 处理"扫描失败但 lint 路径不感知"的边界。详见 §6。

### 5.6 `cmd/kron/serve-mcp/`（MCP access layer）

**v1.2 预置阶段不变**。

**v1.2 暴露阶段（另开 RFC）**：

- `kron_impact` 响应新增一栏（如 `source_files` / `anchored_in`）。
- 响应 schema 用 `jsonschema:` tag 自动派生（与 `ExpiredAssumption` 同模式）。
- 影响 `docs/implementation/mcp.md` 的工具清单。

### 5.7 `cmd/kron/cli/`（CLI access layer）

**v1.2 预置阶段不变**。

**未来可能性**：`kron where-used <slug>` 子命令——但 AGENTS.md "off-limits" 明确说"Adding CLI subcommands beyond init / add / lint / serve-mcp" 需人工批准。本 RFC 不动这条边界。

### 5.8 `docs/`

| 文件 | 变化 |
|---|---|
| `docs/abstractDesign/intent-structure.md` | §二 slug grammar 段落加一句：源文件中 `@kron:intent <slug>` 形式的注释参与反向视图 |
| `docs/abstractDesign/architecture.md` | §〇"无"列里加"反向源文件视图（v1.2+）" |
| `docs/implementation/mcp.md` | `kron_impact` 工具表新增一行 |
| `docs/process/references-snapshot.md` | 如果 `references` / `depends_on` + `source_files` 都落地，这个 281 行的快照文件**可以退役**——但这是另一份 RFC 的事 |

### 5.9 `internal/identity/` / `internal/model/`

**完全无影响**。身份解析与 caller 注入与本 RFC 无关。

---

## 6. 错误模型（v1.2 当天再敲定）

`SourceFiles` 是 `relations` 包里**第一个会返回 error 的函数**。三种处理：

| 方案 | 优点 | 缺点 |
|---|---|---|
| A. `SourceFiles` 返回 `([]model.Anchor, error)` | 直白 | 破坏包内签名一致性 |
| B. 拆成 `relations.SourceFilesOrError(...)` 显式命名 | 自描述 | 包名信息冗余 |
| C. 把 `SourceFiles` 放到 `parser` 包（与 `AnchorsForSlug` 同处） | 签名一致 | 语义上偏"扫描"，不像"关系查询" |

**预置阶段倾向 A**——保持 `SourceFiles` 在 `relations` 包（语义对），但显式声明"此函数是包内第一个带 error 的；这与 `ReverseLinks` 家族的纯函数语义不同，是新引入 IO 的折中"。

---

## 7. 验证清单（动手前自查）

- [x] 不违反架构铁律（iron rules）——`internal/parser` / `internal/relations` 本就是 bottom layer，加函数不引入新依赖。
- [x] 不引入新顶层依赖。
- [x] 不修改 `go.mod` 的 Go 版本。
- [x] 不改 frontmatter schema。
- [x] 不增加 CLI 子命令。
- [x] 不增加 `config.toml` 字段。
- [x] `relations` 包没有"go generate"或反射。
- [x] 调用方写测试时不需要 mock（`AnchorsForSlug` 在 tmpdir 上跑）。
- [x] 与 [`2026-10-03-frontmatter-references.md`](./2026-10-03-frontmatter-references.md) §1.1 表 3 行 "反向" 字样**对账**——本 RFC 显式声明：那行说的"反向"在 v1.2（已落地）是**未实现**的异质图方向，本 RFC 才落地。

---

## 8. 时间线（暂定）

> ⚠ **本节于 2026-10-06 修订**：v1.2（commit `a373828`）与 phase 2（`docs/process/phase-2-leftovers.md`）**均已收口**。原时间线 v1.2 预置 / v1.2 暴露两阶段**作废**——v1.2 不再是本 RFC 的归属。phase 3 未规划，暂无候选 phase。

| 阶段 | 内容 | 触发条件 |
|---|---|---|
| RFC 草案 | 本文件 | 2026-10-04（**现状**） |
| 待 phase 3 规划 | `parser.AnchorsForSlug` + `relations.SourceFiles` + 单测 | phase 3 启动时把本 RFC 加入 backlog |
| 暴露阶段（未知） | MCP `kron_impact` 加字段 + CLI 子命令（可选） | 待 phase 3 启动后另开 RFC 决定 schema |

---

## 9. 待办

- [ ] RFC 阶段决定：是否引入 `parser.Index`（§3.6 方案 3），还是先在 `AnchorsForSlug` 里跑全量 + 过滤（方案 1）。
- [ ] RFC 阶段决定：`SourceFiles` 命名（`source_files` / `anchored_in` / 其他）。
- [ ] RFC 阶段决定：MCP 响应 schema 与 CLI 子命令边界。
- [ ] **新（2026-10-06）**：phase 3 启动时把本 RFC 加入 backlog；phase 3 启动前本文件**不进入实施窗口**。
- [ ] 跟进 [`2026-10-03-frontmatter-references.md`](./2026-10-03-frontmatter-references.md) §1.1 表 3 行表述的修订（RFC 顶栏已标"已落地"；该处"反向"字样在事实层已被追平）。
- [ ] 评估 [`docs/process/references-snapshot.md`](../process/references-snapshot.md) 是否可在本 RFC 落地后正式退役。
