# HomoFluctus 工作追踪 / BD 模块负责人

> **负责人**：HomoFluctus
> **模块范围**（你自己认领的）：文件存储 + LSP 访问层 + IDE 扩展 + CLI 软删除/恢复对外入口
> **创建日期**：2026-10-13
> **最后更新**：2026-10-16（store、CLI、VSCode M0–M4 分步提交并推送至 `origin/feat/serve-lsp`；待 Extension Host 联调）
> **本文件性质**：个人进度追踪，**不**入 PR / **不**影响 CI；只服务 HomoFluctus 自己 + AI assistant。

---

## 1. 总览（4 块交付物）

| 交付物 | 任务清单入口 | 当前主任务 |
|---|---|---|
| ① 文件存储包 | [§2](#2-模块-①-文件存储包) | ①-F 跨 OS 实跑；①-E 跨 Writer/进程协调 |
| ② LSP 访问层 | [§3](#3-模块-②-lsp-访问层) | ②-A-6 PR review / merge |
| ③ IDE 扩展 | [§4](#4-模块-③-ide-扩展) | M3 completion 范围外；M4 树；M5 发布 |
| ④ CLI 软删/恢复 | [§5](#5-模块-④-cli-软删除恢复) | 已实现并测试 |

---

## 2. 模块 ① 文件存储包（`internal/store/`）

| 任务 | 原理 |
|---|---|
| **①-A** `Writer.Update` 边界测试（5 个 case：损坏 frontmatter / kind 保持 / 连续 trash→restore 链 / 并发无 panic） | 已完成 `50dfc20` 10/13；给 LSP e2e 提供"损坏 frontmatter"fixture；见 §8 |
| **①-B** MoveToTrash + RestoreFromTrash 双向幂等测试 | 已有 `TestWriter_TrashRestore_IdempotentChain` 覆盖连续 trash→restore→trash→restore 四步及内容完整性；不代表重复 trash/restore 请求为 no-op |
| **①-C** GC（hard-delete 过期软删项）RFC 拍板 | 架构 §7 标"v1.4+ 不做"——拍板不与实施混谈；实施仍被 §7.2 纪律"不写 GC"拦着 |
| **①-D** store 包 benchmark | **放弃**（10/15 查实 `internal/store/` 0 个 `func Benchmark`；同类工作已被 MCP 侧 `143c860` 覆盖一部分）；按"未完成项不滚到下周"纪律**永久放弃** |
| **①-E** 写并发安全审查（`Writer.Update` 多协程同时改同一 slug） | 已对单个 `Writer` 的 Update 读改写加互斥锁；`go test -race ./internal/store` 通过。不同 Writer 实例/进程间仍无协调，仍可能 last-write-wins |
| **①-F** 跨平台路径测试（Windows / macOS / Linux） | Windows `go test ./internal/store` 通过；macOS / Linux 测试二进制交叉编译通过，未在对应 OS 实际运行 |
| **①-G** 多根仓库支持 | RFC `2026-10-13-multi-project-registry.md` 与 schema 扩展 RFC 已提交；registry、CLI register 和测试已落地，本机并发写仍无进程间锁 |

---

## 3. 模块 ② LSP 访问层（`cmd/kron/serve-lsp/`）

| 任务 | 原理 |
|---|---|
| **②-A-1** 解 `docs/implementation/lsp.md` add/add 冲突 | 已在 `6f0cc4c` 整理冲突正文与约束，删除遗留标记 |
| **②-A-2** 完成 merge | master 合并 commit `3ae6e1d` 已在分支历史；冲突文档已修复 |
| **②-A-3** 跑 §4 红线 | `gofmt -l .`、`go vet ./...`、`go test ./...` 最近已通过；本轮提交后需重跑 |
| **②-A-4** push `feat/serve-lsp` | 已推送至 `50c85b3`；后续追踪日志提交也需推送 |
| **②-A-5** 恢复 `.agents/` | 两个技能文件已纳入 `a42a05f`；内容与 stash blob 完全一致，stash 保留作备份 |
| **②-A-6** PR review + 合并 feat/serve-lsp → master | 仍需维护者 review/merge；不可由本地测试替代 |
| **②-B** LSP SDK 集成（`go.lsp.dev/protocol v1.0.1` + `jsonrpc2` + `uri`） | 已完成 `7c3c030` 10/15；RFC `2026-10-07-lsp-sdk.md` §3 拍板 + `2026-10-12-lsp-decisions.md` §1 知悉 LSP 3.18 vs 3.17 偏差；见 §8 |
| **②-C** 扩展 LSP 方法：`textDocument/completion` | RFC `2026-10-07-lsp-sdk.md` §5 + impl §1.1 明确**不在 v1.3 范围**；v1.4+ 与 lint 引擎协同评估；需 `internal/parser.ValidateSlug` 缓存前置 |
| **②-D** 扩展 LSP 方法：`textDocument/publishDiagnostics` | 同上不在 v1.3 范围；依赖 zhangjun005 主线的 `lint` 引擎 |
| **②-E** 扩展 LSP 方法：`workspace/symbol` | 同上不在 v1.3 范围；依赖 symbol 索引前置 |
| **②-F** e2e 测试覆盖率提升 | v1.3 范围已 28 测试覆盖到位；本项接 ①-D 放弃后空出的 0.5 天 buffer——**本周内不启动**（不让 v1.3 之外的测试回填 ②-F） |

---

## 4. 模块 ③ IDE 扩展（`frontend/vscode/`）

| 任务 | 原理 |
|---|---|
| **③-M0** VSCode 扩展建仓 + 跑通 `vscode-languageclient` 最小 demo | 已建 `frontend/vscode/` 扩展骨架并通过 TypeScript 编译；Extension Host 尚未实跑 |
| **③-M1** 加载 `kron serve-lsp` stdIO 子进程 + `initialize` 握手 | 已接入 `kron.serverPath`（默认 `kron`）和 workspace root 启动参数；无 workspace 时提示且不启动；需在 VSCode Extension Host 配置真实二进制完成手工联调 |
| **③-M2** 在源码锚点行展示 hover 并支持 definition 跳转 | LSP 后端 hover/definition 已有实现；扩展已向 LSP 注册本地文件；VSCode Extension Host 端到端验证仍待完成 |
| **③-M3** definition（go-to-definition）+ completion | definition 随 M2 已由 LSP 提供；completion 属 LSP v1.4+ 评估，不在当前范围 |
| **③-M4** 侧边栏（kron intent 树） | 已用 VSCode 原生 Explorer TreeView + 官方 MCP SDK 调用 `kron_tree`，支持刷新；首 workspace only，暂无详情/编辑 UI |
| **③-M5** 发布到 VSCode Marketplace | 需确认 publisher ID 并使用其 Marketplace 凭据发布；当前未发布 |

---

## 5. 模块 ④ CLI 软删除/恢复（`cmd/kron/cli/`）

| 任务 | 原理 |
|---|---|
| **④-A** RFC 拍板 CLI↔MCP 桥接 | 已由 RFC `2026-10-16-cli-soft-delete.md` 决策：不做访问层桥接，CLI 和 MCP 各自直接调用 `internal/store` |
| **④-B** CLI 入口骨架（参数解析 + 实现） | `kron trash <slug>` / `kron restore <slug>` 已实现，包含参数检查和文件状态测试 |
| **④-C** 实装软删/恢复 | 已直连 `internal/store.Writer`；MCP 继续独立使用相同 store API，不经 CLI 调用 |

---

## 6. 完成历史（git log 摘要）

### 2026-10-16 工作区整合记录

- 解除了 `docs/implementation/lsp.md` 中遗留的 add/add 冲突标记，保留已实现的 LSP 规格和验收契约，并合并协议边界约束；删除与现有实现矛盾的旧 stub 状态描述。
- 恢复的 `.agents/skills/go-style/SKILL.md` 与 `ts-style/SKILL.md` 和 stash 中版本逐字节一致；工作区文件已保留。
- `gofmt -l .`、`go vet ./...`、`go test ./...` 均通过。全仓格式化检查原先发现的 Go 文件已格式化。提交 `a42a05f` 已创建；两次推送均因 GitHub HTTPS 连接失败，`origin/feat/serve-lsp` 尚未更新。
- 本次整合含 MCP、CLI、store、lint、parser、项目注册及文档变更。项目注册的 `projects.json` 读改写仅保证原子文件替换，不提供并发写入串行化；多进程并发注册仍可能丢失更新。
- 保留未完成范围：IDE 扩展 demo / LSP 子进程集成 / UI / Marketplace；completion、diagnostics、workspace/symbol（文档明确不属于当前 LSP v1.3 范围）；CLI 软删/恢复入口后续工作。
- ①-B 经代码核对已完成；①-E 对同一 Writer 实例已加锁并通过 race 测试，跨实例/进程并发仍未解决；①-F 在 Windows 实跑并对 macOS/Linux 交叉编译，仍需对应 OS 实跑。
- ③-M0 建立了扩展骨架和本地 stdio LSP demo，依赖安装与 TypeScript 编译通过；VSCode Extension Host 手工启动尚未验证。
- ③-M1 已切换为真实 `kron serve-lsp -CWD <workspace-root>` 子进程，`kron.serverPath` 支持配置；`npm run compile` 通过。Extension Host 联调和多 workspace 支持仍待完成。
- ③-M4 增加只读意图树：独立启动 `serve-mcp` 子进程，通过 SDK 调用 `kron_tree`；`npm run compile` 通过。多 workspace、节点交互及 Extension Host 实测未完成。
- 顺序提交：`5341b03` (Writer.Update 同实例串行化)、`0c8a564` (存储状态与路径验证日志)、`f882ae5` (VSCode M0)、`e6f292f` (M1)、`070fb6b` (M2)、`9894d28` (CLI RFC)、`6f051f5` (CLI trash/restore)、`b532069` (M4)、`50c85b3` (进度日志)。已推送至 `origin/feat/serve-lsp`。

| 日期 | 任务 | commit | 备注 |
|---|---|---|---|
| 2026-10-13 | ①-A `Writer.Update` 边界测试（5 个 case） | `50dfc20` | 损坏 frontmatter / kind 保持 / trash-restored 幂等链 / 并发无 panic |
| 2026-10-13 | `feat(d-test): add LSP driver` | `dc9eb45` | LSP 手工验证驱动器；`.d-test/` ignore |
| 2026-10-13 | `feat(serve-lsp): wire parser.AnchorsForFile` | `4ee9bdb` | 真实 parser 集成 + 3 个 e2e 测试（hover/definition/shutdown） |
| 2026-10-13 | `chore: ignore .d-test/` | `c250845` | 驱动器隔离 |
| 2026-10-13 | `feat(serve-lsp): v1.3 hardening` | `7c3c030` | textDocumentSync (Full/OpenClose) + errors.Is (P0-5) + logf/notify 注入 (P0-3) + stdio 入口冒烟 (P0-1) + Exit 注入 (P2-1) + 长 body UTF-8 截断 (P1-6) + 1000× 重复稳定性 (P2-4) + 真实 parser e2e + 路径规范化回归；**测试总数 8 → 28** |
| 2026-10-15 | `git push origin feat/serve-lsp` | `7c3c030` (push) | 推送到 `origin/feat/serve-lsp`；本地 7c3c030 = 远端 7c3c030 一致 |
| 2026-10-15 | ②-B LSP SDK 集成 | `7c3c030` (依赖入 `go.mod`) | RFC `2026-10-07-lsp-sdk.md` §3 拍板 + `2026-10-12-lsp-decisions.md` §1 explicit approval 知悉 LSP 3.18 vs 3.17 偏差 |
| 2026-10-16 | tracking 文档重构 | `5815ad7` | 只列任务 + 原理；砍掉状态列 / 工作量列 / 上游动态 / 纪律 / 长期愿景 / AI 自动化协议 |

---

> **最后一行**：HomoFluctus，**你是项目里"两个真正接触人类 + 磁盘"的主线之一**。本文档现在**只**列 ①-④ 模块未完成任务 + 各自原理；进展跟踪（你说话 → AI 改文档）的协议**已砍**——如需恢复 AI 自动化触发器，**单独开 RFC**。
