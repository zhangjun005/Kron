# 新增访问层流程

> 来源: `docs/abstractDesign/architecture.md` §〇·三 (核心铁律) + §一 (五个访问层)  
> 本文件是**实施流程**, 不是架构真理. 当业务需要第 6+ 访问层 (LSP / IDE / GUI / ...), 或现有访问层需要**重大**协议改动, 按此流程执行.

---

## 1 什么时候需要新访问层

决策树:

```
要加的是访问层 (新 entry point) 还是 internal 子包?
├─ 只在某个现有访问层内组合的逻辑
│   └─ → 留在该访问层 (cmd/kron/<sub>/) 或下沉到 internal/ (见 internal-pkg.md)
├─ 现有访问层加新子命令 / 新 MCP tool / 新 CLI flag
│   └─ → 走对应流程 (cli-flag.md / mcp 协议扩展流程 — 待补)
└─ **新 entry point** (新进程 / 新协议 / 新 transport / 全新用户)
    └─ → 新访问层 (见 §2)
```

**"新 entry point" 的客观判定 (满足任一即算)**:
- 启动方式是**独立进程** (e.g. `kron serve-lsp` 跑 stdio-jsonrpc socket)
- 协议**不**与现有访问层共用 (LSP vs JSON-RPC vs HTTP)
- 服务的用户群**不**重叠 (e.g. IDE 编辑器用户 vs CLI 用户)

---

## 2 5 问清单

新增访问层前, 必须在 **RFC 文档** (§3 列出) **先**回答这 5 个问题:

1. **这层服务的用户是谁?**
   - 人类 (开发 / 评审 / 维护) / AI agent / CI / 编辑器插件 / GUI 用户?
   - 用户**不会**直接打字用 CLI 的话, 大概率是新访问层.

2. **协议是否标准?**
   - 标准协议 (JSON-RPC / LSP / HTTP / WebSocket) → 用现成 SDK, **不**手写 parser
   - 自定协议 → 几乎**一定**是错的选择, 重新考虑

3. **是否有现成 SDK 替代手写?**
   - 走 §1 的判定后, 评估 SDK 候选: 维护活跃度 / Go 原生 / 协议版本兼容 / 协议覆盖度
   - SDK 锁定决策**必须**写进 RFC (不可在 RFC 之后再换)

4. **与已有 `internal/*` 的接口面**
   - 新访问层**只**调 `internal/` (受 architecture.md §〇·三 约束)
   - 列出它**会**调哪些包 (store / parser / relations / lint / ...), **不会**调哪些
   - 任何 `internal/` 没现成 API 的能力, **先**扩 `internal/` **再**写访问层 (S3 纪律)

5. **与已有访问层的边界**
   - **谁**会调它? (启动方式: 子进程 / 进程内 / 网络)
   - 它**会**调谁? (必须是 `internal/`, **不**是其他访问层)
   - 共享逻辑**必须**下沉 `internal/`, **不**能在两个访问层之间互相 import (违反 architecture.md §〇·三)

---

## 3 必交付物 (按顺序, 顺序错误 = 流程违反)

新增访问层**必须**按以下顺序交付. **任一缺失** = PR 被拒.

| # | 交付物 | 位置 | 谁写 | 时机 |
|---|---|---|---|---|
| 1 | **设计 RFC** | `docs/rfc/<YYYY-MM-DD>-<layer>.md` | 设计者 | 写代码前 |
| 2 | **接口面文档** | `docs/implementation/<layer>.md` (新文件) | 设计者 | RFC 拍板后, 写代码前 |
| 3 | **`internal/` API 扩** | `internal/<pkg>/<file>.go` (+ 测试) | 实施者 | 访问层代码前 |
| 4 | **访问层骨架** | `cmd/kron/serve-<layer>/` (新包) | 实施者 | 接口稳定后 |
| 5 | **访问层协议文档** | `docs/implementation/<layer>-protocol.md` (新文件) | 实施者 | 与代码同步 |
| 6 | **PR 描述** | GitHub PR body | 实施者 | 提 PR 时 |
| 7 | **`pending-decisions.md` 更新** | `docs/process/pending-decisions.md` | 实施者 | PR 合并前 |

**§3 顺序的核心纪律** (S6 流程纪律):
- **1 拍板**才动 2; **2 拍板**才动 3; **3 稳定**才动 4.
- 任何"先写访问层代码, 后面再补 RFC"的 PR = **流程违反**, review 时**直接拒**.

---

## 4 边界铁律 (architecture.md §〇·三 重申)

新增访问层**必须**满足:

1. **不** import 其他访问层 (`cmd/kron/serve-<other>/...`)
2. **不**被 `internal/` import (单向: `cmd → cli → internal`)
3. ~~**只**通过 `context.Context` 注入 caller key (`"cli"` / `"mcp"` / `"lsp"` / `"ide"` / `"gui"` / ...)~~ 
   > **变更 (2026-10-08)**: caller 注入 API **不再推荐**。新访问层代码**不**再调 `model.WithCaller` 注入身份属性；`internal/` **不**读取 ctx 上的 caller key 做行为分支。`model.WithCaller` / `CallerFrom` 等 API **保留**以兼容既有 import。原因：`context.Context` 注入身份属性**不便于开发**。详见 [`internal/model/caller.go`](../../internal/model/caller.go) 头注释与 architecture.md §2.3 变更记录。
4. **不** 持有全局可变状态 (architecture.md §〇·五)
5. **不** 在协议层引入新依赖 (依赖都在 RFC §3 拍板)

**机械检查 (CI lint, 见 ci-enforcement.md)**:
- `cmd/kron/serve-<layer>/` 静态扫描**不**含 `cmd/kron/serve-<other>/` 引用
- `internal/**` 静态扫描**不**含 `cmd/kron/**` 引用
- `go vet ./...` + `gofmt -l .` 必须通过

---

## 5 PR 流程 (S5 配套)

### 5.1 提 PR 前自检清单

- [ ] §3 全部 7 个交付物**齐**
- [ ] RFC**已**合 master (或在同一 PR 内)
- [ ] `internal/` 新 API **有**测试 (见 `docs/implementation/testing.md`)
- [ ] `internal/` 新 API **不**破坏现有 `internal/` 行为
- [ ] `go vet ./...` / `gofmt -l .` / `go test ./...` 全部通过
- [ ] `CODEOWNERS` (如果新建) 标注了路径 owner

### 5.2 PR 必填字段 (与 `.github/pull_request_template.md` 一致)

```markdown
## Scope
<访问层名 (e.g. serve-lsp)>

## RFC
[docs/rfc/<YYYY-MM-DD>-<layer>.md](path)

## 受影响的访问层
<列出**所有**会被改动的访问层: 即使只是加依赖 / 加配置, 也要列出>

## internal/ 接口面
<列出新调用的 internal 包: store / relations / lint / ...>

## 文档同步声明
- [ ] `docs/abstractDesign/architecture.md` 是否需要改? (答: 是/否, 为什么)
- [ ] `AGENTS.md` 是否需要镜像同步? (答: 是/否, 为什么)
- [ ] `.cursor/rules/project-conventions.mdc` 是否需要镜像同步? (答: 是/否, 为什么)

## Test 覆盖
- [ ] 单元测试: <列出新增/修改的测试>
- [ ] 集成测试 (如果新访问层有 stdio 协议): <说明>
```

### 5.3 合并纪律 (S5)

- **master 分支**:
  - 必走 PR, **不**允许 direct push (除 hotfix 文档 typo)
  - 必审: `@zhangjun005` (单维护者模型, 见 `CODEOWNERS`)
  - 必检: `go vet` / `go test` / `gofmt` (CI)
  - **管理员也守规则** (GitHub branch protection: Include administrators ✅)
- **其他分支**:
  - 自由 push, 但**不**直接合 master

---

## 6 文档纪律 (S6 收口)

**S6 纪律的最终落点**: 本文件 + `internal-pkg.md` + `lint-rule.md` + `migrate.md` + `cli-flag.md` 五份流程文档**是**Kron 项目流程纪律的**全部**载体. 流程纪律**不**在 skill 里重复 (见 `pending-decisions.md` §S7 已废).

**任何"新增 / 修改 / 删除"的流程动作**:
1. **先**写 RFC (流程修改流程本身, 见 `migrate.md` 类比)
2. **再**更新对应 `docs/process/<name>.md`
3. **再**回写镜像 (AGENTS.md / `.cursor/rules/project-conventions.mdc`)
4. **最后**才动代码

**违反此顺序** = 流程违反, review 时拒.

---

## 7 与 `internal-pkg.md` 的关系

| 场景 | 走哪个流程 |
|---|---|
| 加 `internal/<name>/` 子包 | `internal-pkg.md` |
| 加 `cmd/kron/serve-<layer>/` 访问层 | **本文件** |
| 既有访问层加新子命令 | `cli-flag.md` 或 `mcp.md` (待补) |
| 既有访问层换协议 | **本文件** (按"新 entry point"判定) |

---

## 8 待补 (后续 chat 写)

- `docs/implementation/mcp.md` (现有 MCP 文档, **§1 工具清单 / §2 工具契约 / §3 底层实现** 已写, 12 工具 v1 已实开; 新加 tool 走 [`docs/process/mcp-tool.md`](mcp-tool.md); 协议层 (`initialize` / `tools/list` / `ping`) 已通过 MCP 官方 SDK (`mcp-go`) 自动实现, **不**需要手动维护, 见 [`docs/rfc/2026-10-04-mcp-sdk-selection.md`](../rfc/2026-10-04-mcp-sdk-selection.md))
- `docs/implementation/lsp.md` (新文件, v1.3+ — 见 [`docs/rfc/2026-10-07-lsp-sdk.md`](../rfc/2026-10-07-lsp-sdk.md))
- `docs/implementation/gui.md` (新文件, v1.3+ Wails 主 / v1.4+ VSCode 扩展副 — 见 [`docs/rfc/2026-10-07-gui-stack.md`](../rfc/2026-10-07-gui-stack.md))
- `docs/implementation/vscode-extension.md` (新文件, v1.4+)
- `.github/pull_request_template.md` (本文件 §5.2 引用, 仓库**已**建)
- `CODEOWNERS` (本文件 §5.3 引用, 仓库**已**建)
