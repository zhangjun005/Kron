# RFC: FilePath 字段约定

> 状态：**DRAFT** (2026-10-08)
> 范围：`internal/model`、`internal/parser`、`internal/store` 中所有"标识文件"的路径字段
> 触发：实现 RFC `2026-10-08-intent-tree-api.md` 时发现 `Intent.SourcePath` / `Anchor.FilePath` 走的是**两套**约定（绝对 vs 跟随调用方），没有规范

## 1 背景

Kron 内部有几个"标识文件"的路径字段，跨包用：

| 字段 | 类型定义 | 当前实际值 |
|---|---|---|
| `model.Intent.SourcePath` | `string` | 绝对路径（store 强制 `filepath.Abs`） |
| `model.AssumptionFile.FilePath` | `string` | 绝对路径（假设，待核对） |
| `model.Anchor.FilePath` | `string` | **跟随调用方**：调用 `parser.ScanAnchors(abs)` 给出绝对，调用 `parser.ScanAnchors(".")` 给出相对 |

**没有 RFC 写清楚**"FilePath 字段应该是绝对还是相对"——代码里三层各做各的，跨包消费时出现两个问题：

1. **跨平台不一致**：Windows 下 `filepath.Rel` 出来是 `auth\jwt.md`，已经 `ToSlash`；但如果调用方传相对 `dir`，walk 出来是 `./auth/jwt.md`，序列化到 JSON-RPC 时人类读起来怪。
2. **语义不清晰**：store 的 `SourcePath` 是"磁盘上这文件在哪"（绝对是必要的，因为要 `os.Open`）；parser 的 `Anchor.FilePath` 是"这条 anchor 在哪个文件"（**标识用**，不需要绝对）。

## 2 拍板

### 2.1 按"是否要拿这个 path 去 `os.Open`"分两类

| 用途 | 字段 | 拍成什么 |
|---|---|---|
| **IO 用**（要 `os.Open`） | `model.Intent.SourcePath` | **绝对路径**（不动） |
| **IO 用** | `model.AssumptionFile.FilePath` | **绝对路径**（不动；现状已如此） |
| **标识用**（不 IO） | `model.Anchor.FilePath` | **相对 root，正斜杠分隔** |
| **标识用**（不 IO） | `view.IntentTreeNode.Path` | **段名 `[]string`**（不动） |

### 2.2 "相对 root" 的精确定义

- 基准：`r.Root` = 仓库根（store.NewReader 拿到的绝对路径）
- 形式：`filepath.ToSlash(filepath.Rel(root, abs))`
- 例：`r.Root = "/repo"`，anchor 在 `/repo/internal/auth/jwt.go` → `FilePath = "internal/auth/jwt.go"`
- 顶层文件：`FilePath = "README.md"`
- 跨盘（Windows）：`Rel` 返回 `..\..\other-repo\file.go`——**保持原样**（不报错；lint 层决定如何处理）

### 2.3 改动范围

| 改动 | 文件 | 行数估算 |
|---|---|---|
| `parser.ScanAnchors`：`WalkSourceFiles` 拿到 `abs` 后转 `Rel+ToSlash` 再 emit | `internal/parser/anchors.go` | 5 行 |
| `parser.ScanMarkdownAnchors`：同上 | `internal/parser/markdown_anchors.go` | 5 行 |
| `parser.SlugsForFile(filePath)`：保持现状（输入是 filePath、输出是 slug，**不**走 Rel——因为它**是**单文件 API，不属于 walk 体系） | — | 0 |
| 测试 | `internal/parser/anchors_test.go` | 30 行 |
| godoc 更新（`model.Anchor.FilePath`） | `internal/model/intent.go` 或新文件 | 5 行 |

### 2.4 不在范围

- `model.AssumptionFile.FilePath`（假设现状已绝对，待 PR 实施时核对；如果现状不是绝对，**也**改）
- `kron_impact` MCP 响应 schema（access-layer，单独 RFC）
- `view.IntentTreeNode.Path`（已是段名，不涉及文件路径）

## 3 候选方案

### 3.1 Anchor.FilePath：相对 root（推荐）vs 绝对 vs 跟着 walk

| 方案 | 描述 | 评价 |
|---|---|---|
| **P1: 相对 root** | walk 后处理时 `Rel + ToSlash` | 跨平台友好；人读清晰；序列化稳定；和 `view.IntentTreeNode.Path` 风格相近（都是"标识"，不 IO） |
| **P2: 绝对** | walk 内部 `Abs` 调用方传入的 dir | IO 友好；但 anchor 字段不是 IO 用，没必要 |
| **P3: 跟 walk** | 现状 | 不一致；调用方传啥就啥；测试用绝对，CLI 也用绝对，但理论上有人传相对就翻车 |

**倾向 P1**——`Anchor.FilePath` 是 anchor 标识符的一部分，跨平台 + 跨进程稳定比"绝对"更值。

### 3.2 错误信息里的路径

Lint 报错、kron_assume_check 报错里的路径：**优先相对 root，fallback 绝对**。但这是 access-layer 决策，不在 internal RFC 范围——记一句"access-layer 走相对 root"。

## 4 建议 PR 拆解

| PR | 内容 | 估时 | 风险 |
|---|---|---|---|
| **PR-D** | `parser.ScanAnchors` + `ScanMarkdownAnchors` 改 emit `Rel+ToSlash` 路径 | 30 min | 中（改 `Anchor.FilePath` 语义；任何消费方都受影响） |
| **PR-D-test** | anchors_test 加 `TestScanAnchors_FilePath_Relative` | 20 min | 低 |

## 5 决策

| 决策 | 拍板 |
|---|---|
| `Intent.SourcePath` | **绝对**（不动） |
| `AssumptionFile.FilePath` | **绝对**（不动；现状已如此） |
| `Anchor.FilePath` | **相对 root，正斜杠**（改 parser） |
| `view.IntentTreeNode.Path` | **段名**（不动） |
| 错误信息里的路径 | access-layer 自己决定，本 RFC 不拍 |

## 6 不在本 RFC 范围

- MCP 响应 schema 路径字段（access-layer）
- GUI 树视图路径展示（access-layer）
- `kron init` / `kron add` 输出路径格式（access-layer）
