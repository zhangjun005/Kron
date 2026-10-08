# RFC: 反向"源文件"视图 (`SourceFiles`: intent → 锚定代码)

| 字段 | 值 |
|---|---|
| **状态** | 草案 (Proposed) |
| **作者** | AI assistant, 经 zh-jun 委托 |
| **创建日期** | 2026-10-04 |
| **最后修订** | 2026-10-06（§2.2/§2.3/§3.6/§3.8/§9/§10 修订） |
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
- ❌ **不写磁盘缓存**（无 `.kron/.cache/anchors.json`、无外部备份目录）——与 `docs/requirements.md:3` "不做自备份"红线一致。
- ❌ 不扩展锚点语法（不识别 markdown 里的裸 slug，不做 symbol 字符串扫描）。
- ❌ 不读 `.kron/.trash/`（软删除的 intent 不参与 `SourceFiles` 关联——见 §5.4）。
- ⏸ **进程内 cache**——进入待评估（§3.8），不在 RFC 阶段承诺实施。

### 2.3 决策记录（2026-10-04 用户拍板；2026-10-06 复核）

| 决策 | 选项 | 理由 |
|---|---|---|
| 反向来源 | **只看 `@kron:intent` 硬锚点** | 软匹配会被 RFC 文档的非意图引用污染 |
| 性能策略 | **默认**每次调用重扫源码树 | 仓库规模 v1 <10k 文件，单次 walk <0.1s；进程内 cache 复杂度见 §3.8 |
| 暴露面 | **只预置底层，不接 CLI/MCP** | 留出 RFC 阶段打磨 schema 与 UX |
| 磁盘缓存 | **禁止** | 与 `requirements.md:3` "不做自备份"红线一致 |

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

lint `RuleAnchorDangling` 走 `ScanAnchors`，`AnchorsForSlug` 也要走同一棵树。四种解耦方式：

1. **不管**：lint 跑一次（CI 触发），`AnchorsForSlug` 跑一次（按需），重复扫。
2. **磁盘缓存**：lint 跑完写 `.kron/.cache/anchors.json`，`AnchorsForSlug` 读缓存。**已废**——与 `requirements.md:3` "不做自备份"红线冲突。
3. **共用 helper**：parser 加 `Index(dir) (map[string][]Anchor, error)`，lint 和 relations 都调它。**单次 walk，零缓存**。
4. **先量后做**（2026-10-06 新增）：先在 `kron_impact` handler 出口打 `time.Now()` 计时日志，跑 1–2 周量实际调用频率；**根据量测结果**再决定要不要进 §3.8 评估 cache 落地。

**当前倾向方案 4**——把"扫一遍源码树"提不提到 parser 原语层是次要的；**先搞清楚"扫一次的成本 vs 调用频率"的比值**再说。如果每次会话 1 次，cache 永远不做（直接删 §3.8）；如果每次会话 >5 次，再按 §3.8 评估落地。

### 3.7 还没想清楚的（v1.2 当天再定）

- MCP 响应里 `LineNumber` 序列化为 `int` 还是 `string`。
- 单文件锚点数量上限（防 1 万行锚点撑爆响应）。
- 软删除的 intent 在 `SourceFiles` 里的归属（**当前倾向不参与**，见 §5.4）。

### 3.8 进程内 cache 待评估（2026-10-06 用户拍板启动）

> **状态**：本节是**待评估项**，不是实施承诺。RFC 阶段需要回答 4 个真问题才能落地；不预判结论。

#### 3.8.1 适用边界——cache 给谁用？

候选：

- **A. 全进程**——任何 access layer 共享一个 cache。坏处：CLI 跑完填缓存，MCP 后续拿到 CLI 留下的脏数据（虽然跨进程不串，但同进程的 CLI→MCP 切换会有"看不见的写入"）。
- **B. 进程内 + per-access-layer**——每个进程一个 cache，**不跨进程**。CLI 短命不需要缓存；MCP 长会话最受益。
- **C. 进程内 + per-root**——同进程、同一 repo root 共享。CI 在 monorepo 里跑多个 repo 时，缓存不串。

**当前倾向 B**——理由：CLI 本来就是一次性的，cache 在它身上是浪费；MCP 是唯一真正受益者。

**关键参考**：`kron_impact` MCP handler 在每次 Agent 提问时调用 1–N 次。**未量测前不锁结论**——见 §9 (7)。

#### 3.8.2 缓存粒度

候选：

| 粒度 | 形式 | 失效难度 | 占用内存 |
|---|---|---|---|
| 粗（整棵扫一次） | `[]model.Anchor` 单值 | 任何文件改动 = 整缓存失效（基于 mtime） | 小（<1MB） |
| 中（按 slug 字典） | `map[string][]model.Anchor` | 哪个 slug 失效 = 反向映射**再扫一次**才能知道 | 中 |
| 细（按 file path） | `map[file][]anchor` | 哪个 file 改了 = 那 file 的 entry 失效 | 大 |

**粗粒度"刷新"就是"看到 root 目录 mtime 变了就整个清掉"**——简单到几乎不用想。但**准确度差**：用户编辑了一个 .go 文件但**没动锚点**——缓存也失效重扫。这是**伪失效**，无害但浪费。

**中/细粒度的本质困难**：`@kron:intent` 注释**没有反向引用**——它**不是** `auth/jwt 引用了 ./refresh.go`，而是 `./refresh.go 锚定到 auth/jwt`。**所以"按 slug 失效"和"按 file 失效"在数据结构上不对等**。粗粒度可以选"任何 IO 变化都重扫"（不细追究）；中/细粒度**做不到**精准（除非引入 watcher，而 watcher 本身就要持久化）。

**当前倾向粗粒度**——中/细粒度要么失效不准（不对称），要么引依赖（fsnotify 与"零新依赖"红线冲突）。

#### 3.8.3 失效机制——什么时候刷新？

候选：

| 触发 | 实现 | 可靠度 |
|---|---|---|
| (a) TTL | cache 写时记时间，过期重扫 | 简单，**但**时间窗内有脏读 |
| (b) mtime 哨兵 | cache 写时记 `root` 的 mtime，调用前 `os.Stat` 比对 | 中等；**不能 100% 检测** root 树的修改（Linux 行为，Windows 类似但**不是**所有 FS 都保证） |
| (c) Watcher | `fsnotify` 监听 root 树 | 准——但 **AGENTS.md off-limits "零新依赖"**，fsnotify 是新顶层依赖 |
| (d) 不失效 | 进程内 cache **不**刷新——直到进程退出 | 等同"per-process cache"——`kron serve-mcp` 进程在的时候，缓存**永远不刷新**——这是**最危险**的方案 |
| (e) 显式 `kron_reload` MCP 工具 | 给 agent 一个"刷新缓存"的工具 | 把"什么时候刷新"的责任**完全**推给调用方 |

**当前倾向 (b) mtime 哨兵 + (a) TTL 兜底**——mtime 变了重扫；TTL（默认 60s）兜 mtime 不可靠；不引新依赖。

**写时失效的真正难点**：`parser` 包**没有**写时钩入点。`store.Writer` 改 .kron/intents/*.md，**但不**改源文件。**源文件的修改在 kron 之外**——IDE / `git checkout` / 用户手 `cat > refresh.go` 都会改源文件。`kron` **根本看不到这些写**。

退化原则：锚点语法是 `// @kron:intent <slug>`——slug 改了、文件删了，**只是 anchor-dangling 错（lint 会抓到）**，**不会**让有效 anchor 凭空消失。**TTL/mtime 失效下，cache 偶尔脏的代价是"看到陈旧但不误导的数据"——可接受**。

#### 3.8.4 "运行时隐式状态"合规审查

`architecture.md` §〇 铁律 7 原话：**"CI lint is the only enforceable gate — no runtime implicit state; strong constraints are expressed as `kron lint` errors in CI, not runtime defaults."**

**严格读**："no runtime implicit state" 的语境是**"让约束只能在 CI lint 里表达"**——意思是不能有"kron 在运行时偷偷改文件 / 改环境变量 / 改 git 状态"这种隐式副作用。

**进程内 `sync.Map` 不属于**这种——它：

- ✅ 不写磁盘
- ✅ 不改外部进程
- ✅ 不影响 git
- ✅ 不改 .kron/intents/*.md
- ⚠️ 但它**让第二次 `kron_impact` 拿到第一次看不到的中间状态**——这个**算不算**隐式？

**当前判断**：**不算**——理由：`parser` 包本来就**每次重读文件**（无状态），加 cache 之后**对外可观察的行为不变**（同一时刻同一仓库，结果一致）；变的只是**性能**。

**对照**：`store` 包**明确说**"no in-memory caches"（`internal/store/reader.go:27`）——它**选择**保守。`parser` 选**激进**也合理，但**得有 commit message / 注释明说**——留证，让后来人能 revert。

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

**完全无影响**。身份解析与 caller 注入与本 RFC 无关。**(2026-10-08 历史注)**: caller 注入 API **不再推荐**，详见 architecture.md §2.3。本节对当前 caller API 状态仍适用。

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
| RFC 草案 | 本文件（2026-10-04 初稿；2026-10-06 §2.2/§2.3/§3.6/§3.8/§9/§10 修订） | 现状 |
| **频率量测**（前置） | `kron_impact` handler 出口加 `time.Now()` 计时日志，跑 1–2 周 | RFC §9 (7) |
| 决定 cache 适用边界 | 根据量测结果决定是否进 §3.8 子节 | 量测完成后 |
| phase 3 规划 | `parser.AnchorsForSlug` + `relations.SourceFiles` + 单测 | phase 3 启动时把本 RFC 加入 backlog；**不依赖量测结果启动实施** |
| 暴露阶段（未知） | MCP `kron_impact` 加字段 + CLI 子命令（可选） | 待 phase 3 启动后另开 RFC 决定 schema |

---

## 9. 待办

- [ ] RFC 阶段决定：是否引入 `parser.Index`（§3.6 方案 3），还是先在 `AnchorsForSlug` 里跑全量 + 过滤（方案 1）。
- [ ] RFC 阶段决定：`SourceFiles` 命名（`source_files` / `anchored_in` / 其他）。
- [ ] RFC 阶段决定：MCP 响应 schema 与 CLI 子命令边界。
- [ ] **新（2026-10-06）**：phase 3 启动时把本 RFC 加入 backlog；phase 3 启动前本文件**不进入实施窗口**。
- [ ] 跟进 [`2026-10-03-frontmatter-references.md`](./2026-10-03-frontmatter-references.md) §1.1 表 3 行表述的修订（RFC 顶栏已标"已落地"；该处"反向"字样在事实层已被追平）。
- [ ] 评估 [`docs/process/references-snapshot.md`](../process/references-snapshot.md) 是否可在本 RFC 落地后正式退役。
- [ ] **新（2026-10-06）**：在 `cmd/kron/serve-mcp/handlers_impact.go` 出口加 `time.Now()` 计时日志，记录 `ScanAnchors` 耗时 + 本次会话累计调用次数；跑 1–2 周后回填量测数据，**作为 §3.8 进程内 cache 是否落地的唯一决策依据**。
- [ ] **新（2026-10-06）**：在 §3.8.1 / §3.8.2 / §3.8.3 评估落地前，本 RFC 实施工作**不开**；仅允许"频率量测"（上一条）作为前置依赖推进。

---

## 10. 与 `requirements.md:3` "不做自备份" 边界澄清

> **2026-10-06 用户拍板**

`docs/requirements.md:3` 原文："备份/行进全部依赖 git，不要做任何的自备份想法"。`docs/abstractDesign/tech-stack.md:64` 同义重申。

**这条红线管的是 `.kron/intents/*.md` 的内容冗余**——禁止磁盘副本：

- ❌ 无 `.kron/.cache/anchors.json`（磁盘 cache）
- ❌ 无外部备份目录
- ❌ 无 git 之外的 `.md` 副本

**进程内 `sync.Map` 类型的纯运行时结构**不**在该红线覆盖范围**——`sync.Map` 只活在 `kron serve-mcp` 进程内存中，进程退出即消失；**不构成"备份"**。

但进程内 cache **仍受** `architecture.md` §〇 铁律 7 约束（"无运行时隐式状态"）——合规审查详见 §3.8.4。

**未来若有新讨论混淆这两条**（"自备份" vs "运行时 cache"），回看本节。
