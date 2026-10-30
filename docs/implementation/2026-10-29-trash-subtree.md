# Node 删除改为整棵子树（2026-10-29 决策记录）

> 本文记录一次数据布局/行为变更的决策与落地细节。变更前 `.kron/.trash` 只存叶子文件，
> node 意图删除只把 `README.md` 移走、restore 后变成 leaf；本变更让 trash 镜像树形结构。

## 1. 背景

原实现（`internal/store/writer.go`）：

- `MoveToTrash(node)` 通过 `resolveReadPath` 找到 `<slug>/README.md`，但目标固定为
  `.kron/.trash/<slug>.md` —— trash 里 node 被降级为叶子文件；
- `RestoreFromTrash` 固定恢复到 `<slug>.md` —— node 恢复后变成 leaf，且可能与已存在的
  `<slug>/` 目录形成 leaf+dir 并存（写入路径有 collision guard，restore 没有）；
- `kron_update` 对 node 返回 `.kron/intents/<slug>.md`，与实际写入的
  `.kron/intents/<slug>/README.md` 不一致。

用户 2026-10-29 拍板：**删除 node 时应直接移除整棵子树**；不采用"trash 里保留叶子文件再
restore 时提级"这种猜测式恢复（"leaf 直接提级也不妥"）。

## 2. 决策（全部已确认）

| # | 决策 |
|---|---|
| D1 | `kron_delete` 对 node 增加安全阀：必须显式传 `confirm_subtree: true`，否则报错并告知子树意图数量 |
| D2 | `kron_delete` 输出新增 `deleted_slugs []string`（节点 + 子树全部 slug） |
| D2b | `kron_restore` 输出新增 `restored_slugs []string`（同理由，恢复 node 会带回整棵子树） |
| D3 | `dependents` 改为子树并集（排除被删的 slug 自身），按字典序排序 |
| D4 | VSIX 同步：启用 node 的 Delete 按钮，确认框明示"整棵子树 N 个意图一起进 trash" |
| D5 | 旧 `.trash/<slug>.md`（旧逻辑把 node 存成叶子）按 leaf 恢复，不自动提级；文档写清 |

## 3. 新布局与行为

```text
.kron/intents/<slug>.md            leaf   ──delete──▶ .kron/.trash/<slug>.md
.kron/intents/<slug>/              node   ──delete──▶ .kron/.trash/<slug>/
  ├─ README.md                                            ├─ README.md
  └─ child.md                                             └─ child.md
```

- **删除 node**：整个 `<slug>/` 目录 rename 到 trash，子树一起离开活树；无逐文件删除。
- **恢复**：先探测 `.kron/.trash/<slug>/`（node）再探测 `<slug>.md`（leaf），按 trash 形状还原；
  目标任一形态（`<slug>.md` 或 `<slug>/`）已存在 → `ErrIntentExists`，拒绝合并。
- **旧 trash 兼容**：`<slug>.md` 一律按 leaf 恢复（D5）。
- **wire**：
  - `kron_delete`：`trashed_path` 返回真实路径（leaf 文件 / node 目录），新增 `deleted_slugs`；
  - `kron_restore`：`path` 返回真实恢复路径（node 为 `<slug>/README.md` 等），新增 `restored_slugs`；
  - `kron_update`：`path` 返回真实写入路径（node 为 `<slug>/README.md`）。
- **崩溃一致性**：delete/restore 仍是单次 `os.Rename`；trash 冲突检查在 rename 前完成。

## 4. 涉及文件

| 层 | 文件 |
|---|---|
| model | `internal/model/paths.go`：新增 `TrashDirPath`，`TrashPath` 注明仅 leaf |
| store | `internal/store/writer.go`：`MoveToTrash` / `RestoreFromTrash` 形状保留；新增 `RelativeIntentPath` / `RelativeTrashPath` |
| MCP | `cmd/kron/serve-mcp/handlers_delete.go`（子树、confirm、dependents 并集、deleted_slugs）、`handlers_restore.go`、`handlers_update.go`、`tools_schema.go` |
| VSIX | `package.json` 无变化；`webview/src/IntentCanvas.tsx` 启用 node 删除；`webview/src/App.tsx` 传 `confirm_subtree`、确认文案与提示；`src/mcp/client.ts` 契约补 `confirm_subtree` / `deleted_slugs` / `restored_slugs` |
| 测试 | `internal/store/paths_test.go` 更新 node 删除断言，新增子树删除/恢复、旧 flat trash、目录冲突用例 |

## 5. 迁移与兼容

- **无存量数据迁移**：目前仓库与用户项目没有使用中的 trash 条目（用户确认"这玩意目前没有人用"）。
- 旧 trash 条目（若有）按 D5 恢复为 leaf，不自动猜测 node 形状。
- `.trash` 布局变化不影响 lint / LSP / 读取路径（活树扫描不包含 `.trash`）。

## 6. 验收

- `go build ./...`、`go vet ./...`、`go test ./internal/store ./cmd/kron/serve-mcp` 通过。
- 手工：删除带子意图的 node → 子树从树中消失、`.kron/.trash/<slug>/` 出现；restore → 子树与
  README 形状恢复；不带 `confirm_subtree` 的 node 删除返回 IsError。
