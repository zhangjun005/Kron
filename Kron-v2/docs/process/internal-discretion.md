# 内部包纪律 — 何时下沉到 `internal/`、何时不

> 2026-10-09 写。3 人协作背景（含 GUI / IDE 开发）下，避免"任何和底层相关的东西就往 `internal/` 塞"的常见误判。
>
> 架构真理：[`docs/abstractDesign/architecture.md` §〇 铁律 #1](../../docs/abstractDesign/architecture.md)。本文件是**判定准则**（什么下沉、什么不），**不**是流程文档——具体流程走 [`internal-pkg.md`](./internal-pkg.md) + [`new-internal-api.md`](./new-internal-api.md)。

---

## 1. TL;DR — 黄金问题

> **这个行为是否被 ≥2 个不同的"用户"用到，且这些用户**互不知道**彼此的存在？**
>
> - ✅ **是** → 下沉到 `internal/`
> - ❌ **否** → 留在调用方（CLI / MCP / LSP / GUI / IDE）

展开前，先把"用户"定义清楚。

---

## 2. "用户"在 Kron 的语境

`internal/` 的存在意义：**消除重复** + **保持调用方简洁**。"用户"不是看 API 文档的人，而是**另一个 Go 包**。

| 真正的"用户" | 判定方式 |
|---|---|
| **同一访问层不同子命令**（MCP 的 `kron_add` 与 CLI 的 `kron add`）| 两个不同 Go 函数共享同一段逻辑 → 下沉 |
| **不同访问层**（MCP + LSP + CLI 共用一段解析）| 三处复用 → 下沉 |
| **客户端层**（GUI / IDE 调 MCP，间接调 internal）| **不**算用户——它们走协议访问层，**不**直接 import `internal/`（铁律 #9）|
| **测试代码**（`xxx_test.go`）| **不**算用户——测试可以 import 任何东西 |

**反例**（**不**是用户）：

- "未来 GUI 可能会调"——**不**算"现在的用户"。GUI 调 MCP，**不**直接 import `internal/`
- "这个函数挺通用的"——"通用"是形容词，**不**是依据。必须有**至少一处** Go 代码真正复用
- "内部包应该集中所有逻辑"——集中是结果，**不**是原因。集中了没人调，就成 dead code

---

## 3. 决策流程（先问这 4 个问题）

按顺序问。**任一**答"否" → **不**下沉；**全部**答"是" → 下沉。

### Q1. 是否被 ≥2 个 Go 调用方复用？

- **答"是"** → 进 Q2
- **答"否"**（只有一个调用方）→ 留在调用方，**不**下沉

**WHY**：单调用方的"通用"函数就是该调用方的私有函数。把它挪到 `internal/` 不会"复用"任何东西，只会让调用方多写一行 import。

### Q2. 这些调用方是否在**不同**的访问层 / 包？

- **答"是"**（如 MCP + CLI，或 MCP + 未来的 LSP）→ 进 Q3
- **答"否"**（同包内两个函数共用）→ 留在该包内（可能是子包 / 私有 helper），**不**下沉到 `internal/`

**WHY**：`internal/` 的边界意义是"访问层可以 import，业务层不能反过来"。同包内抽公共函数**不**需要 `internal/` 这层——直接放子包或包内 helper 即可。

### Q3. 这个行为是否**纯函数** + **无 GUI 副作用**？

- **答"是"**（纯数据变换 / 解析 / 扫描）→ 进 Q4
- **答"否"**（涉及 UI 渲染 / 弹窗 / 通知 / 协议帧）→ 留在调用方

**WHY**：`internal/` 零外部 import 铁律（architecture §〇 铁律 #8）。任何"和 GUI 库相关"或"和协议帧相关"的代码下沉都会**污染**业务层。

### Q4. 这个行为是否**无 access-layer 特定语义**（不区分 CLI / MCP / LSP 行为）？

- **答"是"** → **下沉**
- **答"否"**（如"CLI 走 stderr，MCP 走 JSON-RPC 错误码"）→ 留在调用方

**WHY**：`internal/store` / `internal/parser` / `internal/lint` 不该知道"调用方是哪个 wire protocol"。若函数有"if CLI, do X; if MCP, do Y"分支，**下沉**会逼业务层感知协议——违反铁律 #1 + #2。

---

## 4. 决策矩阵

| Q1 ≥2 调用方 | Q2 跨包 | Q3 纯函数 | Q4 协议无关 | → 动作 |
|---|---|---|---|---|
| ❌ | — | — | — | **不**下沉（留在调用方）|
| ✅ | ❌ | — | — | **不**下沉（子包/包内 helper）|
| ✅ | ✅ | ❌ | — | **不**下沉（涉及 UI/协议帧）|
| ✅ | ✅ | ✅ | ❌ | **不**下沉（有协议分支）|
| ✅ | ✅ | ✅ | ✅ | ✅ **下沉**到 `internal/<existing-or-new>/` |

**优先级**：Q1 > Q2 > Q3 > Q4。最先答"否"即停止。

---

## 5. 常见误判（**不**下沉）

### 5.1 "GUI 可能会用到" ≠ 现在下沉

```
"这个函数挺通用的，以后 GUI 也要调"
→ 错。GUI 调 MCP（或 LSP），**不**直接 import `internal/`。
   若 GUI 真要某个能力，**先**加 MCP / LSP 工具，再让 GUI 调它。
```

### 5.2 "和文件 I/O 有关" ≠ 必须下沉

```
"它读 .md 文件了，所以放 internal/store"
→ 错。`internal/store` 是**业务规则**层（reader / writer / MoveToTrash）。
   若函数只是"读 .md 文件并返回 []byte"，放 `cmd/kron/<sub>/` 内的 helper
   就够，不该进 `internal/store`。
```

### 5.3 "会涉及假设 registry" ≠ 必须下沉

```
"kron_assume_check 用了 internal/assumption，所以假设相关代码都该下沉"
→ 错。`kron_assume_check` handler 用 `internal/assumption.Reader` 是**调用**，
   不是"所有和假设有关的代码都该下沉"。Handler 的输出投影（`AssumeWarning`）
   是 MCP 协议层，**不**该进 `internal/assumption`。
```

### 5.4 "测试需要" ≠ 必须下沉

```
"我得在测试里 mock 它，所以下沉到 internal"
→ 错。若只是为了测试，定义 interface 在调用方包内即可。
   `internal/` 的目的是"复用"，不是"测试便利"。
```

### 5.5 "LSP 可能会 hover 用到" ≠ 现在下沉

```
"intent 标题 + summary 渲染是 hover 通用逻辑，下沉到 internal/view"
→ 对，但前提是 ≥2 个调用方**现在**就在用。
   若只有 `kron_get` 在用，留在 `cmd/kron/serve-mcp/` 包内 helper。
   等 LSP hover 实装后再下沉——若届时 LSP hover 也要它。
```

---

## 6. 反向问题：什么**该**下沉

✅ **该**下沉（典型例子）：

| 场景 | 理由 |
|---|---|
| `parser.ScanAnchors(root)` | CLI `kron lint` + MCP `kron_lint` + MCP `kron_intent_density` 都用 |
| `lint.Run(ctx, root)` | CLI `kron lint` + MCP `kron_lint` 都用 |
| `store.Reader.LoadAll(ctx)` | MCP `kron_list` + `kron_assume_check` + `kron_impact` + `kron_intent_density` 都用 |
| `relations.ReverseLinks(intents, slug)` | MCP `kron_impact` + `kron_delete`（dependents 警告）都用 |
| `identity.Handle(...)` | CLI `kron add` + MCP `kron_add` 都用 |
| `assumption.Reader` | MCP `kron_assume_check` + `kron_list` + `kron_get` 都用 |

❌ **不**该下沉（典型反例）：

| 场景 | 理由 |
|---|---|
| MCP 工具的输出投影（`IntentSummary` / `AssumeWarning` 等）| **只** MCP 用，wire shape 是协议层责任 |
| CLI 命令行解析（`switch os.Args[1]`）| **只** CLI 用，访问层私有 |
| LSP hover 渲染格式 | **只** LSP 用（实装后）|
| `handlers_density.go` 里的 `scanOrphanFiles` / `countLines` | 暂时**只** `kron_intent_density` 用；**不**下沉 |
| GUI "整体预览"的组件树构建 | **只** GUI 用（实装后）|

---

## 7. 灰色地带（按 4 问仍答"是"，但要再想想）

### 7.1 "似乎会被 LSP 用"

```
LSP hover/definition 计划里要"返回 intent 标题 + 摘要"，
而 MCP `kron_get` 已经返回 `IntentBody{Frontmatter, Body, SourcePath}`。

→ Q1: 现有调用方 = MCP `kron_get`。LSP 还没实装，**不算** ≥2 调用方。
→ 答: **不**下沉。LSP 实装**前**，`kron_get` 解析逻辑留在 `cmd/kron/serve-mcp/`。
   LSP 实装**后**复用 MCP（spawn serve-mcp 子进程拿 intent 数据），**不**再下沉。
```

### 7.2 "似乎会被 GUI 用"

```
GUI 计划要"多项目概览 + 意图树预览"。
意图树构建（`view.IntentTree`）现在被 MCP `kron_list` 用 + 计划 GUI 用。

→ Q1: 现在 = 1 调用方（MCP `kron_list`）。
→ Q2: GUI **不**直接 import `internal/view`（铁律 #9），走 MCP。
   也就是说 GUI 不会"成为 internal/view 的第二个 Go 调用方"。
→ 答: **不**下沉，留在 `internal/view/`。
   （其实它**已经**在 internal，但写这段是给"未来要往里塞"的人提个醒。）
```

### 7.3 "测试需要它在下层"

```
"我在 internal/lint_test.go 里要 mock parser 行为"
→ 错。Lint **不**该 mock parser。Lint **应该**走真 parser。
   若某个 lint 规则需要特殊输入准备，在 test 内部构造测试 fixture 即可，
   **不**该改 production API 形状。
```

---

## 8. 速查清单（提交前自问）

往 `internal/` 加新文件 / 改公开 API 前，**逐条**自问：

- [ ] **Q1**: 现在有 ≥2 个 Go 调用方在用吗？（不是"以后会有"）
- [ ] **Q2**: 这 ≥2 个调用方在不同包吗？
- [ ] **Q3**: 是纯函数 + 无 UI / 协议帧副作用吗？
- [ ] **Q4**: 没有"if CLI / if MCP / if LSP"分支吗？
- [ ] **不是**"GUI 以后要用"作为依据
- [ ] **不是**"和文件 I/O 有关"作为依据
- [ ] **不是**"测试 mock 方便"作为依据

**任一**不勾 → **不**下沉。

---

## 9. 与现有流程的关系

- **新 `internal/<name>/` 包** → [`internal-pkg.md`](./internal-pkg.md)
- **改 `internal/` 公开 API** → [`new-internal-api.md`](./new-internal-api.md)
- **新 MCP 工具**（含 handler） → [`mcp-tool.md`](./mcp-tool.md)
- **新 CLI flag / subcommand** → [`cli-flag.md`](./cli-flag.md)
- **新 lint 规则** → [`lint-rule.md`](./lint-rule.md)

本文件**不**替代这些流程；它是**判定准则**——在走流程**前**，先确认"该不该下沉"。

---

## 10. WHY 写这条纪律（背景）

按 user 2026-10-09 原话：

> "为了方便协作，包括前端后面共 3 人开发，需要一个纪律……对于一些**仅在一个模块有意义**的行为，**不能因为和底层有关**就往 internal 沉淀。"

翻译成判定规则：

- "仅在一个模块有意义" → Q1 答"否"
- "不能因为和底层有关" → Q2-Q3 提醒：别让"看起来像底层"误导下沉决策

这条纪律对**AI 协作者**尤其重要——AI 倾向把"通用"代码下沉（出于"分而治之"的本能），但人写的 `internal/` 应该是**复用驱动**（Q1），不是**分类驱动**（"看起来属于底层"）。
