# 新增 / 修改 `internal/` 公开 API 流程

> 来源: `docs/abstractDesign/architecture.md` §〇·三 + §〇·五  
> 配合 `internal-pkg.md` (新子包) + `new-access-layer.md` (新访问层).  
> 本文件是**实施流程**, 不是架构真理. 任何"扩 `internal/` 现有包公开 API"按此流程.

---

## 1 适用场景

- 在现有 `internal/<pkg>/` 里**新加**导出函数 / 类型
- **改签名**已有导出函数 (即使是"加个参数")
- **删**导出函数 / 字段

**不**适用:
- **新开** `internal/<name>/` 子包 → 走 `internal-pkg.md`
- **新开** 访问层 (`cmd/kron/serve-<layer>/`) → 走 `new-access-layer.md`
- 内部实现 (非导出符号) 改动 → **不**走此流程, 正常 PR 即可

---

## 2 触发判定: 公开 API 改动 vs 内部实现改动

```go
// ❌ 公开 API 改动 — 走本流程
func (s *Store) ListByAuthor(ctx context.Context, author string) ([]Intent, error)
type Intent struct {
    Symbol     string  // 改字段名 / 类型 / 标签
    Assumptions []Assumption
}

// ❌ 公开 API 改动 (即使是"加参数")
func ParseAnchor(text string, strict bool) (Anchor, error)
   ↓
func ParseAnchor(text string, strict bool, opts ParseOpts) (Anchor, error)

// ✅ 内部实现 — 不走本流程
func (s *Store) readFile(path string) ([]byte, error)  // 私有
func parseFrontmatterInternal(...)                     // 私有
```

**判定**:
- 函数 / 类型 / 字段**以大写字母开头** → 公开 API → 走本流程
- `// exported:` 注释标注 → 公开 API → 走本流程
- 任何"在 `cmd/kron/<sub>/` 里会 import 这个符号" → 公开 API → 走本流程

---

## 3 必走流程 (S3 + S6 配套)

`internal/` 公开 API 改动**必须**按以下顺序:

1. **先** 评估: 这 API **有**几个访问层会调?
2. **如果 ≥ 2 个** → 必走 RFC (在 `docs/rfc/<YYYY-MM-DD>-internal-<api>.md`)
3. **如果 = 1 个** → PR body 必须**写明**"为什么是 1 个, 而不是下沉 / 合并"
4. **再** 改 `internal/<pkg>/<file>.go` + 测试
5. **再** 改**所有**调用方 (`cmd/kron/cli/`, `cmd/kron/serve-mcp/`, `cmd/kron/serve-lsp/`)——客户端层（VSCode 扩展 / Wails）**不**是 internal package 直接调用方，**不**列入本表
6. **再** 同步**所有**文档 (`docs/implementation/api-surface.md` 等)
7. **最后** 提 PR, PR body 必填"调用方清单"字段

---

## 4 RFC 必填 (≥ 2 个访问层时)

| # | 字段 | 示例 |
|---|---|---|
| 1 | API 签名 | `func (s *Store) MoveToTrash(ctx context.Context, slug string) error` |
| 2 | 调用方清单 | `cmd/kron/serve-mcp` (kron_delete tool) / `cmd/kron/cli` (未来 `kron trash` 子命令, 计划) |
| 3 | 错误模型 | `ErrNotFound` / `ErrAlreadyTrashed` / 包装语义 |
| 4 | 上下文注入 | ~~caller key 取值: `"cli"` / `"mcp"` / ...~~ **(2026-10-08 变更)** ctx **仅**用于取消 / 超时传播；caller 注入 API **不再推荐**（详见 architecture.md §2.3） |
| 5 | 并发语义 | 同一 slug 并发调用: 后写者胜 / 错误 / 锁? |
| 6 | 测试覆盖 | 单测 / 表驱动 / 集成测试范围 |
| 7 | 文档同步声明 | api-surface.md / domain-model.md / error-catalog.md 哪些**必**改 |
| 8 | migration 策略 | 如果是改签名 (不是新加), 老 API 怎么处理 (deprecate / 立即换 / shim) |

**§4 的核心**: 第 5 项 (并发语义) **不**能在 PR 阶段才**第一次**讨论 — 必须**先**写在 RFC.

---

## 5 PR 必填字段

```markdown
## Scope
<新加 / 改签名 / 删 — 哪个, 哪个包, 哪个符号>

## internal/ API 改动
- [ ] 新加: <签名>
- [ ] 改签名: <旧签名> → <新签名>, migration 见 RFC §8
- [ ] 删: <符号名>, 替代方案 <什么>

## 调用方清单 (受 §3 第 5 项约束)
<列出**所有**改动后必须同步的访问层 + 它们的 PR / commit>

| 调用方 | 是否本 PR 内 | 链接 |
|---|---|---|
| `cmd/kron/serve-mcp` | 是 / 否 | <PR 链接> |
| `cmd/kron/serve-lsp` (stub, 3-of-3 协议访问层骨架) | 是 / 否 / 不适用 | — |

## RFC 链接 (≥ 2 个调用方时必填)
[docs/rfc/<YYYY-MM-DD>-internal-<api>.md](path)

## 文档同步声明
- [ ] `docs/implementation/api-surface.md` 已更新
- [ ] `docs/implementation/error-catalog.md` 已更新 (如有新错误)
- [ ] `docs/abstractDesign/intent-structure.md` 已更新 (如改 frontmatter)
```

---

## 6 顺序纪律的**强约束**

`internal/` 公开 API 改动**必须**在**任何**访问层代码之前:

- ❌ **错**: "我先在 `serve-mcp` 里调 `store.MoveToTrash`, 后面 `internal/` 再补这个函数" — 这是**不一致版 master** 的**主要**成因 (S3 警告).
- ✅ **对**: "我先在 `internal/store/store.go` 写 `MoveToTrash` + 测试, **然后** `serve-mcp` 调它"

**理由** (S3 配套):
- `internal/` API 稳定**前**, 任何访问层基于"假想 API"开发 = 一致性回归
- `internal/` API 稳定**后**, 多个访问层可以**并行**开发 (因为接口已锁)
- master 上"基于未锁接口的访问层"会让其他开发者**复制**这个错误

**机械检查** (CI lint):
- 提 PR 时 `internal/` 的导出符号清单 = 访问层调用的清单 (用 `go list` + grep 静态扫)

---

## 7 与其他流程文档的关系

| 场景 | 走哪个 |
|---|---|
| 删 `internal/<pkg>/` 整包 | `internal-pkg.md` (反向) |
| 改 `internal/<pkg>/` 公开 API | **本文件** |
| 加 `cmd/kron/serve-<layer>/` | `new-access-layer.md` |
| 改 frontmatter 字段 (影响 `internal/model`) | `migrate.md` + **本文件** |
| 改 CLI flag 涉及 `internal/parser` | `cli-flag.md` + **本文件** |

---

## 8 待补 (后续 chat 写)

- RFC 模板 (复用现有 RFC 格式, 但**强制** §4 8 项)
- 静态扫描脚本 (`tools/check-internal-api-surface.sh`)
