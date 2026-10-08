# RFC: Intent 树 API + README-as-intent 语义映射

> 状态：**DRAFT** (2026-10-08)
> 范围：内部包 `internal/view/` + 存储层 README 识别 + 未来 MCP/IDE 工具
> 触发：本 PR 实现 `internal/view/BuildIntentTree` 时发现存储层有 2 个 RFC 语义 gap

## 1 背景

`intent-structure.md` §一 + §二 + `ide-interaction.md` 描述的目录树模型：

```
.kron/intents/
├── README.md                ← 根 "节点意图"
├── auth/
│   ├── README.md          ← 模块 "节点意图"
│   ├── jwt.md
│   └── password.md
```

锚点语法 `// @kron:intent <slug>` 走"目录简写"：`auth` 简写 `auth/README.md`。
意图路径中的 `/` 对应目录分隔符；`README.md` 可用目录名简写。

## 2 当前实现（v1, PR `pending` 之后）

### ✅ 已实现

- `internal/view/BuildIntentTree([]*model.Intent) *IntentTreeNode`
  - 从扁平 intent 列表建树
  - dir 节点（合成）+ leaf 节点（携带 Intent）
  - dir 节点和 Intent 节点可共存（`auth` 有 leaf 时，`auth/jwt` 同时存在）
  - `Flatten()` pre-order 输出
  - `Sort()` 递归排序（dirs 优先，Name 字典序）
  - `IntentTitle(*model.Intent) string` Body 第一行 H1 / fallback slug TitleCase

### ❌ 已知 gap

#### Gap A: README.md 不被识别为目录节点 intent

- `walkIntentSlugs` 把 `auth/README.md` slugify 成 `auth/README`
- `LoadAll` 返回 slug `"auth/README"`，`Get("auth")` 找不到
- `// @kron:intent auth` 在 lint 中**应该**对应 `auth/README.md`，但**实际**找不到（anchor-dangling 误报）

#### Gap B: view 树里的 README 是 leaf `auth/README`，不是 dir `auth`

- 即使 Gap A 修了，view 树应该把 `auth/README.md` 提升为 dir `auth` 节点（带 Intent）
- 当前 `BuildIntentTree` 把 `auth/README` 当 `auth/README` 的 leaf

#### Gap C: anchor 的 `kind: code | markdown` 维度未落地

- RFC `2026-10-08-md-anchors.md` §2 拍板 `incoming_anchors: { ..., kind: "markdown" }`
- `model.Anchor` **无** `Kind` 字段
- `parser.ScanMarkdownAnchors` 实现存在但**跳过** `.kron/`（含 README.md）
- `kron_impact` 响应 schema **无** `kind` 字段

## 3 候选方案

### 3.1 README-as-intent slug 映射（Gap A）

| 方案 | 描述 | 评价 |
|---|---|---|
| **A1: walkIntentSlugs 特殊处理** | 碰到 `*/README.md` 时 slug 简写为去掉 `README` 的目录路径 | 简单；动存储层；要 RFC 拍"README 存在则目录可寻址" |
| **A2: LoadAll 后处理** | store 仍返回 `auth/README`，view 包在 BuildIntentTree 前把 `auth/README` 提升为 `auth` dir | 存储层无改动；语义在 view 包集中；view 包复杂化 |
| **A3: 双 slug 索引** | `auth` 和 `auth/README` 同时存在（同一 Intent 引用，Get("auth") = Get("auth/README")） | 模糊性大；哪个是规范名？ |

**倾向 A1**——存储层 1 处改动，view 包无需特殊处理；与 `intent-structure.md` §一"目录简写"语义对齐。

### 3.2 view 树 README 提升（Gap B）

| 方案 | 描述 | 评价 |
|---|---|---|
| **B1: BuildIntentTree 检测 leaf slug 以 `/README` 结尾** | 改成 dir + Intent 节点 | view 包增加特殊处理 |
| **B2: 调用方预处理** | 调用 view 前把 `auth/README` 的 Intent 改 slug 为 `auth` | 改 model.Intent 字段，破坏 store 契约 |
| **B3: 拒绝处理** | 维持现状，README 在树里就是 leaf | 文档 vs 代码继续不一致 |

**倾向 B1**——本地处理不破坏 model；view 包多 5 行；README 自动成节点意图。

### 3.3 Anchor.Kind 维度（Gap C）

跨包改动（model + parser + lint + MCP schema），不在 view 包范围。
**作为独立 RFC `2026-10-08-md-anchors.md` 的实施路径 PR 处理**。

## 4 建议 PR 拆解

| PR | 内容 | 估时 | 风险 |
|---|---|---|---|
| **PR-A** | RFC 拍板后，walkIntentSlugs 特殊处理 README + LoadAll 后处理 | 1.5h | 中（动存储层） |
| **PR-B** | view.BuildIntentTree 检测 `*/README` slug 并提升 | 30min | 低（view 包局部） |
| **PR-C** | model.Anchor 加 Kind 字段 + ScanMarkdownAnchors 不再跳过 .kron/ + kron_impact schema | 4h | 高（跨 4 包 + MCP schema） |

## 5 不在本 RFC 范围

- ❌ MCP `kron_list` / `kron_get` / 新 `kron_tree` 工具——access-layer 改动
- ❌ Wails GUI / VSCode 扩展 / LSP 树视图渲染——客户端层
- ❌ intent-density 树形展示——`kron_intent_density` 是 MCP 工具，看 `internal/parser` 现状

## 6 等待决策

| 决策 | 选项 |
|---|---|
| 走 PR-A 还是 PR-B | PR-A (改存储) / PR-B (view 包兜) / 两个都做 |
| README-as-intent 边界 | 只 `*/README.md` 触发 / `*/README.md` + `*/index.md` 都触发 |
| 双 slug 冲突时的优先名 | 目录名（`auth`）/ 文件名（`auth/README`） |
