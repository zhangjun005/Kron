# HomoFluctus 工作追踪 / BD 模块负责人

> **负责人**：HomoFluctus
> **模块范围**（你自己认领的）：文件存储 + LSP 访问层 + IDE 扩展 + CLI 软删除/恢复对外入口
> **创建日期**：2026-10-13
> **最后更新**：2026-10-16（完成工作区整合、格式检查与测试；推送受网络阻断）
> **本文件性质**：个人进度追踪，**不**入 PR / **不**影响 CI；只服务 HomoFluctus 自己 + AI assistant。

---

## 1. 总览（4 块交付物）

| 交付物 | 任务清单入口 | 当前主任务 |
|---|---|---|
| ① 文件存储包 | [§2](#2-模块-①-文件存储包) | ①-B / ①-E / ①-F（未启动） |
| ② LSP 访问层 | [§3](#3-模块-②-lsp-访问层) | ②-A（merge 冲突解 + push） |
| ③ IDE 扩展 | [§4](#4-模块-③-ide-扩展) | ③-M0（建仓 + demo） |
| ④ CLI 软删/恢复 | [§5](#5-模块-④-cli-软删除恢复) | ④-A（开 RFC 草案） |

---

## 2. 模块 ① 文件存储包（`internal/store/`）

| 任务 | 原理 |
|---|---|
| **①-A** `Writer.Update` 边界测试（5 个 case：损坏 frontmatter / kind 保持 / 连续 trash→restore 链 / 并发无 panic） | 已完成 `50dfc20` 10/13；给 LSP e2e 提供"损坏 frontmatter"fixture；见 §8 |
| **①-B** MoveToTrash + RestoreFromTrash 双向幂等测试 | 已有 `TestWriter_TrashRestore_IdempotentChain` 覆盖连续 trash→restore→trash→restore 四步和内容完整性（先前追踪状态已过期） |
| **①-C** GC（hard-delete 过期软删项）RFC 拍板 | 架构 §7 标"v1.4+ 不做"——拍板不与实施混谈；实施仍被 §7.2 纪律"不写 GC"拦着 |
| **①-D** store 包 benchmark | **放弃**（10/15 查实 `internal/store/` 0 个 `func Benchmark`；同类工作已被 MCP 侧 `143c860` 覆盖一部分）；按"未完成项不滚到下周"纪律**永久放弃** |
| **①-E** 写并发安全审查（`Writer.Update` 多协程同时改同一 slug） | 已对单个 `Writer` 的 Update 读改写加互斥锁；`go test -race ./internal/store` 通过。不同 Writer 实例/进程间仍无协调，仍可能 last-write-wins |
| **①-F** 跨平台路径测试（Windows / macOS / Linux） | `paths_test.go` 当前可能仅 Linux 验证；你正在 Windows 上工作，是真风险点 |
| **①-G** 多根仓库支持 | 架构 §0 暗示单 root——必须先开 RFC 拍板需求；纯实施工作 2 周，**RFC 拍板优先** |

---

## 3. 模块 ② LSP 访问层（`cmd/kron/serve-lsp/`）

| 任务 | 原理 |
|---|---|
| **②-A-1** 解 `docs/implementation/lsp.md` add/add 冲突（feat 217 行 vs master 53 行） | 昨天 10/15 21:30 触发的 `git merge origin/master` 在该文件卡住——不解决 → merge commit 写不出来 → 后续 push / 测试都跑不起来 |
| **②-A-2** 完成 merge（`git add` + `git commit`） | merge 冲突解了不算完成，必须 commit 才把 merge 结果落到 `feat/serve-lsp` HEAD |
| **②-A-3** 跑 §4 红线（`go vet ./...` + `gofmt -l .` + `go test ./...`） | AGENTS.md §4 "提交前必跑"——把 master 合进来后必须重跑，避免引入上游回退 |
| **②-A-4** push `feat/serve-lsp` | HEAD 包含 5815ad7（你的 tracking 文档）+ merge commit，**未 push**；不推会让 origin/feat/serve-lsp 落后本地 |
| **②-A-5** 恢复 `.agents/`（`git stash pop`，`stash@{0}`） | merge 前 stash 装的 `.agents/` 2 个 untracked（go-style + ts-style skills），不 pop 会丢 |
| **②-A-6** PR review + 合并 feat/serve-lsp → master | 合并时机 = 10/19 一次性合并（昨天 plan §7.1）；gh CLI 未装走浏览器 PR 流程；阻塞 = zhangjun005 review |
| **②-B** LSP SDK 集成（`go.lsp.dev/protocol v1.0.1` + `jsonrpc2` + `uri`） | 已完成 `7c3c030` 10/15；RFC `2026-10-07-lsp-sdk.md` §3 拍板 + `2026-10-12-lsp-decisions.md` §1 知悉 LSP 3.18 vs 3.17 偏差；见 §8 |
| **②-C** 扩展 LSP 方法：`textDocument/completion` | RFC `2026-10-07-lsp-sdk.md` §5 + impl §1.1 明确**不在 v1.3 范围**；v1.4+ 与 lint 引擎协同评估；需 `internal/parser.ValidateSlug` 缓存前置 |
| **②-D** 扩展 LSP 方法：`textDocument/publishDiagnostics` | 同上不在 v1.3 范围；依赖 zhangjun005 主线的 `lint` 引擎 |
| **②-E** 扩展 LSP 方法：`workspace/symbol` | 同上不在 v1.3 范围；依赖 symbol 索引前置 |
| **②-F** e2e 测试覆盖率提升 | v1.3 范围已 28 测试覆盖到位；本项接 ①-D 放弃后空出的 0.5 天 buffer——**本周内不启动**（不让 v1.3 之外的测试回填 ②-F） |

---

## 4. 模块 ③ IDE 扩展（`frontend/extensions/vscode/` 独立仓）

| 任务 | 原理 |
|---|---|
| **③-M0** VSCode 扩展建仓 + 跑通 `vscode-languageclient` 最小 demo | 10/16 当日交付物（昨天 plan §7.1）；起空仓**不** clone Kron Go 仓（0.5 天工作量） |
| **③-M1** 加载 `kron serve-lsp` stdIO 子进程 + `initialize` 握手 | 用 `vscode-languageclient`；本地 `go build` 出 `kron` 二进制；**阻塞** = master 合了 LSP 实现 → 实际启动 = 下周（10/20+） |
| **③-M2** 打开 `.kron/intents/*.md` → hover 出 `[[anchor]]` 解析 | 端到端首次 demo；**阻塞** = M1 + LSP 实现 |
| **③-M3** definition（go-to-definition）+ completion | **阻塞** = ②-C 完成（v1.4+ 不在 v1.3 范围） |
| **③-M4** 侧边栏（kron intent 树） | 用 shadcn/ui（zhangjun005 §7.3 拍板）+ VSCode webview |
| **③-M5** 发布到 VSCode Marketplace | 走 `vsce publish`；**阻塞** = zhangjun005 批 publisher 账号 |

---

## 5. 模块 ④ CLI 软删除/恢复（`cmd/kron/cli/`）

| 任务 | 原理 |
|---|---|
| **④-A** 开 RFC 拍板 CLI↔MCP 桥接 | 本周聚焦；候选 3 方案：子进程 stdIO / unix socket / 库内 import；列权衡后拍板；工作量 1 天 |
| **④-B** CLI 入口骨架（flag 解析 + stub 实现） | **可提前写**——MCP 工具未就绪前可先写 + go test 通过；预期位置 `cmd/kron/cli/intent_soft_delete.go` + `intent_restore.go` |
| **④-C** 实装：调 MCP 真实工具 | **阻塞** = ④-A 拍板 + zhangjun005 MCP 工具集（A1/A5）就绪 |

---

## 6. 完成历史（git log 摘要）

### 2026-10-16 工作区整合记录

- 解除了 `docs/implementation/lsp.md` 中遗留的 add/add 冲突标记，保留已实现的 LSP 规格和验收契约，并合并协议边界约束；删除与现有实现矛盾的旧 stub 状态描述。
- 恢复的 `.agents/skills/go-style/SKILL.md` 与 `ts-style/SKILL.md` 和 stash 中版本逐字节一致；工作区文件已保留。
- `gofmt -l .`、`go vet ./...`、`go test ./...` 均通过。全仓格式化检查原先发现的 Go 文件已格式化。提交 `a42a05f` 已创建；两次推送均因 GitHub HTTPS 连接失败，`origin/feat/serve-lsp` 尚未更新。
- 本次整合含 MCP、CLI、store、lint、parser、项目注册及文档变更。项目注册的 `projects.json` 读改写仅保证原子文件替换，不提供并发写入串行化；多进程并发注册仍可能丢失更新。
- 保留未完成范围：IDE 扩展 demo / LSP 子进程集成 / UI / Marketplace；completion、diagnostics、workspace/symbol（文档明确不属于当前 LSP v1.3 范围）；CLI 软删/恢复入口后续工作。
- ①-B 经代码核对已完成；①-E 对同一 Writer 实例已加锁并通过 race 测试，跨实例/进程并发仍未解决；①-F 仍需跨平台验证。

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
