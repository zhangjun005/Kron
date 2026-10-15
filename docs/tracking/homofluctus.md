# HomoFluctus 工作追踪 / BD 模块负责人

> **负责人**：HomoFluctus
> **模块范围**（你自己认领的）：文件存储 + LSP 访问层 + IDE 扩展 + CLI 软删除/恢复对外入口
> **创建日期**：2026-10-13
> **最后更新**：2026-10-15（v1.3 hardening 落地 + ②-B SDK 集成标记完成 + ②-A push 到 origin/feat/serve-lsp + ①-D store benchmark 降级为「放弃」+ 启动 ③-M0 建仓规划）
> **本文件性质**：个人进度追踪，**不**入 PR / **不**影响 CI；只服务 HomoFluctus 自己 + AI assistant。

---

## 0. 自动化机制（"每干完一步自动更新"如何实现）

### 0.1 触发方式

你干完一步后，对 AI assistant 说以下任一指令即可触发自动更新：

| 你的口头指令 | AI 做什么 |
|---|---|
| "**干完了**：X" 或 "**X 完成**" | 把 X 任务的"进度"列从 ⬜ / 🚧 改为 ✅ + 填"完成 commit"列 |
| "**开始**：X" 或 "**进行中 X**" | 改为 🚧 + 填"开始日期"列 |
| "**卡住了 X**：原因" | 改为 ⚠️ + 填"卡点"列 |
| "**放弃 X**" / "**不做 X 了**" | 改为 ❌ + 填"放弃原因"列 |
| "**新增任务 X**" | 在对应 section 末尾追加 |
| "**对齐 master**" / "**新 master commit**" | 刷新 §6"上游动态"表 |
| "**今日汇报**" | 生成按状态分组的当日进度（✅ / 🚧 / ⚠️） |

### 0.2 自动化保证

- **每次你的工作 session 开始**（你打开 Cursor 跟 AI 说话时），AI **必须**先读这份文档
- **每次你触发上述指令**，AI **必须**改这份文档 + 告诉你"已更新 §X.Y 行"
- **每周日 20:00**（本地时间），AI 提醒你做"周回顾"，刷新 §3 "本周目标" 滚动到下周

### 0.3 不在本文件追踪的内容

- zhangjun005 / 泓洲苏 / lanyue 的工作（只在 §6 上游动态表里提）
- `docs/abstractDesign/architecture.md` 等真理文档的改动（那些走 PR review 流程）
- 与个人进度无关的 session 信息

---

## 1. 总览（4 块交付物 × 当前状态）

| # | 交付物 | 当前位置 | 阻塞 | 当前阶段 | 距完成 |
|---|---|---|---|---|---|
| ① | **文件存储包** | `internal/store/` | ❌ 不阻塞 | v1.2 完成 100% | 已 100% |
| ② | **LSP 访问层** | `cmd/kron/serve-lsp/` | ❌ 不阻塞（泓洲苏 + 你已实装） | v1.3 实装完成，**已 push origin/feat/serve-lsp**，等 zhangjun005 review + 合并 | 95%（剩 5% = ②-A PR 合并） |
| ③ | **IDE 扩展** | `frontend/extensions/vscode/` **独立仓未建** | ❌ 完全独立 | **10/16 启动 M0** | 0% |
| ④ | **CLI 软删/恢复入口** | `cmd/kron/cli/` 内部调 MCP | ⚠️ **等 MCP 工具集就绪** | 未启动 | 0% |

> 关键洞察：**③ 独立仓**是当前最值得 push 的（零阻塞 + 产品感强）；**④ CLI 入口** 100% 等 zhangjun005 MCP 拍板，不应主动启动。

---

## 2. 模块 ① 文件存储包（`internal/store/`）

### 2.1 已完成（v1 / v1.2）

| API | 实现 commit | 关联 RFC | 测试 |
|---|---|---|---|
| `Reader.List` | v1 mainline | — | `reader_test.go` |
| `Reader.Read` | v1 mainline | — | `reader_test.go` |
| `Reader.Exists` | v1 mainline | — | `reader_test.go` |
| `Writer.Write` | v1 mainline | — | `writer_test.go` |
| `Writer.MoveToTrash` | v1 mainline | — | `writer_test.go` |
| `Writer.RestoreFromTrash` | v1 mainline | — | `writer_test.go` |
| `Writer.Update` + `UpdatePatch` | v1.2 `1c5a091` | RFC 2026-10-08-writer-readme-symmetry | `writer_test.go` |
| `Config` load/save | v1 mainline | — | `config_test.go` |
| README shorthand (`<slug>/README.md`) | v1.2 `1c5a091` | RFC 2026-10-08-writer-readme-symmetry | `paths_test.go` |

### 2.2 可推进项（按 ROI 排序）

| # | 任务 | 状态 | 阻塞 | 预计工作量 | 关联 RFC | 备注 |
|---|---|---|---|---|---|---|
| ①-A | **Update 边界测试覆盖**（并发 / 跨 kind / 损坏 frontmatter） | ✅ **完成** | ❌ | 1 天 | — | `writer_test.go` 新增 5 个 case：`TestWriter_Update_CorruptedFrontmatter` / `TestWriter_Update_KindPreserved_NodeToNode` / `TestWriter_Update_KindPreserved_LeafToLeaf` / `TestWriter_TrashRestore_IdempotentChain` / `TestWriter_Update_Concurrent_NoPanic` | `50dfc20` 10/13 |
| ①-B | **MoveToTrash + RestoreFromTrash 双向幂等测试** | ⬜ 待开始 | ❌ | 0.5 天 | — | 现状：单步测试有，**连续 trash→restore→trash→restore 链路无** |
| ①-C | **GC（hard-delete 过期软删项）RFC 拍板** | ⬜ 待开始 | ⚠️ 架构 §7 标"v1.4+ 不做" | 1 天 RFC + 0.5 天实施 | 待开 | 需开 RFC `2026-XX-XX-store-gc.md` |
| ①-D | **store 包 benchmark**（`go test -bench=. -benchmem`） | ❌ **放弃** | ❌ | 0.5 天 | — | **10/15 查实：`internal/store/` 0 个 `func Benchmark`；同类工作已被 MCP 侧 `143c860 test(mcp): 加 bench_test.go` 覆盖一部分**。**本周纪律 §7.2：未完成项直接放弃或降级，不滚到下周**。改 ②-C e2e 覆盖率提升 替代本项（见 §3.3 ②-F） |
| ①-E | **写并发安全审查**（`Writer.Update` 多协程同时改同一 slug） | ⬜ 待开始 | ❌ | 1 天 | — | 当前 `writeFileAtomic` 单写安全，**多写需加锁？** |
| ①-F | **跨平台路径测试**（Windows / macOS / Linux） | ⬜ 待开始 | ❌ | 0.5 天 | — | `paths_test.go` 当前可能仅 Linux 验证 |
| ①-G | **多根仓库支持**（一个 Kron 仓管多个 git repo） | ⬜ 待开始 | ⚠️ 架构 §0 暗示单 root | RFC 后定 | 2 周 | 需先开 RFC `2026-XX-XX-multi-root.md` 拍板需求 |

### 2.3 主动选择：本周聚焦 ~~①-A + ①-D~~（共 1.5 天，最高 ROI）→ **本周已聚焦 ①-A**（10/13 完成）；①-D 已放弃（见上 ①-D 行）。本周存储包侧**不再启动新项**——按 §7.1 计划让 buffer 给 ② / ③ 侧

---

## 3. 模块 ② LSP 访问层（`cmd/kron/serve-lsp/`）

### 3.1 你已完成的实装（按时间顺序）

| 改动 | commit | 说明 |
|---|---|---|
| LSP v1.3 主实现 | `d234e0a` (merge from master, 泓洲苏) | initialize / hover / definition / shutdown / exit |
| 实现计划 + 决策点 RFC §4.4 修正 | `f75907f` (merge) | `docs/implementation/lsp.md` + `2026-10-12-lsp-decisions.md` |
| `serve-lsp` help 文本修复 | `e8e2d49` (merge) | 去掉"stub"过时描述 |
| ignore `.d-test/` | `c250845` | D end-to-end LSP test fixture |
| wire `parser.AnchorsForFile` + 删除 mock + 3 个 e2e | `4ee9bdb` | 真实 parser 集成（hover/definition/shutdown） |
| LSP driver for stdIO 手工验证 | `dc9eb45` | `.d-test/` 驱动器，**已 ignore** |
| store writer 5 边界测试 | `50dfc20` | 给 LSP e2e 提供"损坏 frontmatter"测试 fixture |
| **v1.3 hardening** | `7c3c030` | textDocumentSync (Full/OpenClose) + errors.Is (P0-5) + logf/notify 注入 (P0-3) + stdio 入口冒烟 (P0-1) + Exit 注入 (P2-1) + 长 body UTF-8 截断 (P1-6) + 1000× 重复稳定性 (P2-4) + 真实 parser e2e + 路径规范化回归 |

### 3.2 状态：已 push，待 zhangjun005 review + 合并

- **本地分支**：`feat/serve-lsp`，HEAD = `7c3c030`（v1.3 hardening 落地）
- **远端**：`origin/feat/serve-lsp = 7c3c030`（**10/15 16:30 push 成功**——前几次 timeout 是网络抖动，443 实际是通的）
- **相对 master**：领先 5 commit（不含 merge commit `3ae6e1d`）
  - 新增 commit: `c250845` (ignore) / `4ee9bdb` (wire) / `dc9eb45` (driver) / `50dfc20` (test) / `7c3c030` (hardening)
- **下游消费测试**（10/15 当前）: `go test ./cmd/kron/serve-lsp/...` → **28/28 通过**, `go vet` 干净
- **合并时机**：BD 任务全部干完后一次性合 master（按 §7.1 计划，**10/19 周日 review + 合并**）
- **合并前必做**：`go vet ./...` + `gofmt -l .` + `go test ./...` 全过 ✅

### 3.3 可推进项

| # | 任务 | 状态 | 阻塞 | 预计工作量 | 关联 RFC | 备注 |
|---|---|---|---|---|---|---|
| ②-A | **PR review + 合并** | 🚧 **进行中** | ⚠️ zhangjun005 review 阻塞 | 0.5 天 | — | gh CLI 未装，走浏览器 PR 流程；分支已 push；等 review + 10/19 一次性合并 |
| ②-B | **LSP SDK 集成**（`go.lsp.dev/protocol v1.0.1` + `jsonrpc2` + `uri`） | ✅ **完成** | ❌ 已落 `go.mod` | — | RFC `2026-10-07-lsp-sdk.md` §3（拍板选 A） + `2026-10-12-lsp-decisions.md` §1（拍板入 `go.mod`，3.18 vs 3.17 偏差知悉接受） | **本项原表记 "切入手搓" 是误读**——RFC 早就拍板 `go.lsp.dev/protocol`；本分支已直接用 SDK 集成，无手搓阶段。**10/15 标记完成** |
| ②-C | **扩展 LSP 方法**：`textDocument/completion` | ⬜ v1.4+ 评估 | ❌（需 `internal/parser.ValidateSlug` 缓存） | 2 天 | RFC `2026-10-07-lsp-sdk.md` §5 + impl §1.1 | **不在 v1.3 范围**；v1.4+ 与 lint 引擎（②-D）协同评估 |
| ②-D | **扩展 LSP 方法**：`textDocument/publishDiagnostics` | ⬜ v1.4+ 评估 | ⚠️ 需 `lint` 引擎（zhangjun005 主线） | 1-2 天 | RFC `2026-10-07-lsp-sdk.md` §5 + impl §1.1 | **不在 v1.3 范围**；v1.4+ 与 `kron lint` A-class 共用 |
| ②-E | **扩展 LSP 方法**：`workspace/symbol` | ⬜ v1.4+ 评估 | ❌（需 symbol 索引） | 1 天 | impl §1.1 | **不在 v1.3 范围**；依赖 symbol 前置 |
| ②-F | **e2e 测试覆盖率提升**（v1.3 已 28/28，**v1.4+ 再说**） | ⬜ v1.4+ 评估 | ⚠️ 依赖 ②-C/D/E | 1 天/方法 | — | 当前 v1.3 范围已 28 测试覆盖到位；本项接 ①-D 放弃后**空出的 0.5 天 buffer**——本周内**不**启动（不让 v1.3 之外的测试回填 ②-F） |

### 3.4 主动选择

- **本周（10/13 ~ 10/19）已聚焦**：②-A（PR push + 等 review + 10/19 合并）
- **②-B 现状修正**：原表 "②-B LSP SDK 切入手搓" 任务描述与 RFC 拍板不符——RFC 早就选了 SDK，本分支已落地。**重命名为 ②-B "LSP SDK 集成" 并标 ✅**（10/15）
- **②-C/D/E 现状修正**：原表把"completion / diagnostics / symbol"列为"本周可推进"——错了；RFC `2026-10-07-lsp-sdk.md` §5 + impl §1.1 已明确 v1.4+ 评估。**改 v1.4+ 评估状态**（10/15）

---

## 4. 模块 ③ IDE 扩展（`frontend/extensions/vscode/` 独立仓）

### 4.1 现状

- **未建仓**（10/13 状态）
- **位置**：`frontend/extensions/vscode/`（Kron 主仓占位）
- **关联文档**：`docs/rfc/2026-10-08-lsp-client.md` + `docs/rfc/2026-10-10-frontend-ui-stack.md`
- **关联 UI 库**：shadcn/ui（zhangjun005 §7.3 拍板）

### 4.2 里程碑

| 里程碑 | 状态 | 阻塞 | 工作量 | 说明 |
|---|---|---|---|---|
| **M0**：建仓 + `npm create vscode-extension` | ⬜ | ❌ 完全独立 | 0.5 天 | 起空仓；**不** clone Kron Go 仓 |
| **M1**：加载 `kron serve-lsp` stdIO 子进程 + `initialize` 握手 | ⬜ | ⚠️ 需 master 合了 LSP 实现 | 1-2 天 | 用 `vscode-languageclient`；本地 `go build` 出 `kron` 二进制 |
| **M2**：打开 `.kron/intents/*.md` → hover 出 `[[anchor]]` 解析 | ⬜ | ⚠️ 需 M1 + LSP 实现 | 1 天 | 端到端首次 demo |
| **M3**：definition（go-to-definition）+ completion | ⬜ | ⚠️ 需 ②-C 完成 | 2 天 | 跳转 + 补全 |
| **M4**：侧边栏（kron intent 树） | ⬜ | ❌ | 2-3 天 | 用 shadcn/ui + VSCode webview |
| **M5**：发布到 VSCode Marketplace | ⬜ | ⚠️ 需 zhangjun005 批 publisher 账号 | 1 天 | 走 `vsce publish` |

### 4.3 主动选择

- **本周（10/13 ~ 10/19）已聚焦**：
  - ~~本周聚焦 **M0**（起仓），只投入 0.5 天~~ → **改 10/16 启动 M0**（v1.3 LSP hardening 10/15 落地后空出 ② 侧工作日）
- **10/16 计划**：③-M0 起仓 + 跑通 vscode-languageclient 最小 demo（0.5 天）
- **依赖确认**：M1 需 LSP master 合入 —— 按 §7.1 计划 10/19 一次性合并，M1 实际启动 = 下周（10/20+）
- **风险**：VSCode extension 走 `vsce publish` 需 zhangjun005 批 publisher 账号（M5 阻塞）

---

## 5. 模块 ④ CLI 软删除/恢复（`cmd/kron/cli/`）

### 5.1 现状

- **完全未开工**
- **预期位置**：`cmd/kron/cli/intent_soft_delete.go` + `cmd/kron/cli/intent_restore.go`
- **预期接口**：`kron intent soft-delete <id>` + `kron intent restore <id>`
- **预期实现**：薄壳 CLI，**只**调 MCP（架构 §〇 铁律 #2 —— 访问层不互调，但 CLI 调 MCP 走 IPC 跨进程是允许的）

### 5.2 阻塞矩阵

| 阻塞项 | 来源 | 解锁时间 | 备注 |
|---|---|---|---|
| MCP `intent_soft_delete` 工具 | zhangjun005 | 等 A1 落地（10/13 状态未在 master） | 见 `docs/implementation/` |
| MCP `intent_restore` 工具 | zhangjun005 | 等 A5 落地（10/13 状态未在 master） | 同上 |
| CLI 如何调 MCP 的架构拍板 | **你自己** | RFC `2026-XX-XX-cli-soft-delete-mcp-bridge.md` | 候选：子进程 stdIO / unix socket / 库内 import |

### 5.3 可推进项

| # | 任务 | 状态 | 阻塞 | 预计工作量 | 关联 RFC | 备注 |
|---|---|---|---|---|---|---|
| ④-A | **开 RFC 拍板 CLI↔MCP 桥接** | ⬜ 待开始 | ❌ | 1 天 | 待开 `2026-XX-XX-cli-soft-delete-mcp-bridge.md` | 列出 3 候选方案 + 权衡 |
| ④-B | **CLI 入口骨架**（flag 解析 + stub 实现） | ⬜ 待开始 | ❌ | 0.5 天 | — | 等 MCP 就绪前可先写，go test 通过 |
| ④-C | **实装：调 MCP 真实工具** | ⬜ 待开始 | ⚠️ ④-A 拍板 + zhangjun005 MCP 就绪 | 1-2 天 | ④-A | 等依赖就绪再写 |

### 5.4 主动选择：本周聚焦 **④-A**（开 RFC），不写代码

---

## 6. 上游动态（zhangjun005 / 泓洲苏 / lanyue 进展）

> 本节**只**记录对 BD 模块有影响的他人 commit。**不**记录无关 commit。

| 日期 | 作者 | commit | 对 BD 模块影响 |
|---|---|---|---|
| 2026-10-13 | zhangjun005 | `81103b2 merge: bring optimize/mcp` | MCP 工作合入 master，**④ CLI 入口部分解锁**（A1/A5 待 A9 后落地） |
| 2026-10-13 | zhangjun005 | (待) B-3 softening `375c425` | assumptions 数据架构调整，**① 存储包** 可能有微调 |
| 2026-10-12 | zhangjun005 | (RFC) `2026-10-12-close-b3-softening.md` | 写完拍板 |
| 2026-10-12 | 泓洲苏 | `e8e2d49 fix(cli): serve-lsp help` | **② LSP** 已实装 |
| 2026-10-12 | 泓洲苏 | `f75907f docs(lsp): 决策点汇总` | **② LSP** 实施计划 |
| 2026-10-12 | 泓洲苏 | `d234e0a feat(serve-lsp): v1.3` | **② LSP** 主实现 |
| 2026-10-13 | HomoFluctus（你） | `dc9eb45 feat(d-test): LSP driver` | **② LSP** e2e 验证 |
| 2026-10-13 | HomoFluctus（你） | `50dfc20 test(store): 5 boundary cases` | **① 存储包** 边界测试 |
| 2026-10-13 | HomoFluctus（你） | `4ee9bdb wire parser.AnchorsForFile` | **② LSP** 真实 parser 集成 |
| 2026-10-13 | HomoFluctus（你） | `c250845 ignore .d-test/` | **② LSP** 驱动器隔离 |
| 2026-10-13 | HomoFluctus（你） | `7c3c030 v1.3 hardening` | **② LSP** textDocumentSync/errors.Is/logf/notify/stdio smoke |
| 2026-10-15 | HomoFluctus（你） | `git push origin feat/serve-lsp` | **② LSP** push 成功（7c3c030）

**自动刷新规则**：每次你说"**对齐 master**" / "**新 master commit**"，AI 跑 `git fetch origin && git log --oneline origin/master..HEAD` 扫 BD 模块相关 commit 写入此表。

---

## 7. 本周目标（2026-10-13 ~ 2026-10-19）硬性计划

> **非弹性目标**：本周日（10/19）之前全部完成并 push，未完成的项**直接放弃或降级**，不滚到下周。

### 7.1 每日计划（10/15 更新版）

| 日期 | 任务 | 交付物 | 状态 / 备注 |
|---|---|---|---|
| 10/13（周二） | ✅ **①-A** `Writer.Update` 边界测试 | ✅ `50dfc20` | ✅ 已完成 |
| 10/14（周三） | ~~**①-D** `store` benchmark~~ | ~~`store_test.go` `-bench` + 性能报告~~ | ❌ **放弃**（10/15 查实 0 benchmark；同类工作已被 MCP 侧 `143c860` 覆盖一部分；按 §7.2 纪律不滚到下周） |
| 10/15（周四） | ✅ **②-A** `feat/serve-lsp` push 成功 + **②-B** SDK 集成标记完成 | ✅ `origin/feat/serve-lsp = 7c3c030`（v1.3 hardening 落地，28/28 测试过）+ tracking 文档 RFC 对齐 | ✅ 已完成 |
| 10/16（周五） | **③-M0** VSCode 扩展建仓 | `frontend/extensions/vscode/` 独立仓 + 跑通 `vscode-languageclient` 最小 demo | ⬜ 待开始（**新聚焦**） |
| 10/17（周六） | 缓冲日 / ③-M0 续 / ④-A RFC 草案 | 看 M0 进度；如 M0 < 0.5 天，10/17 起 ④-A RFC | ⬜ 待开始 |
| 10/18（周日） | 缓冲日 / 对齐 master + 解决 ②-A 冲突 | `feat/serve-lsp` 与 master 无冲突 | ⬜ 待开始 |
| 10/19（周一） | **一次性合并**：PR 合 master + 周回顾 | **全部任务一次性合入 master** | ⬜ 待开始 |

> **总工作量（已调整）**：① 侧 0.5 天（①-A）+ ② 侧 0.5 天（②-A push 完）+ ③ 侧 0.5 天（M0）= 1.5 天 | 缓冲：3.5 天
> **10/19 计划不变**：feat/serve-lsp 一次性合 master，③-M0 仓在 10/19 前跑通

### 7.2 本周纪律（红线）

- ❌ 每天只做当天计划的事，不超前做明天的任务（防止 buffer 浪费）
- ❌ 不写 GC（hard-delete）实施——等 RFC 拍板
- ❌ 不写 CLI 软删/恢复实装——等 MCP 工具 + RFC ④-A
- ❌ **不切 LSP SDK**（`go.lsp.dev/protocol`）~~——等 RFC ②-B 拍板~~ → **已落地**（RFC `2026-10-07-lsp-sdk.md` §3 早就拍板，10/15 已在 `go.mod` + 集成完成）
- ❌ 不动 frontmatter schema——红线 §3
- ❌ 不加新 CLI 子命令——红线 §3
- ❌ 不让 AI 强推 master / 改 shared branch 历史
- ✅ 每个任务完成后**立刻**说"干完了：X"，触发自动更新
- ✅ **未完成项不滚到下周**——直接放弃或降级（10/14 ①-D 已示范）

---

## 8. 已完成历史（每完成一项从 §2-5 移入此处）

| 日期 | 任务 | commit | 备注 |
|---|---|---|---|
| 2026-10-13 | **①-A** `Writer.Update` 边界测试（5 个 case） | `50dfc20` | 损坏 frontmatter / kind 保持 / trash-restored 幂等链 / 并发无 panic |
| 2026-10-13 | `feat(d-test): add LSP driver` | `dc9eb45` | LSP 手工验证驱动器；`.d-test/` ignore |
| 2026-10-13 | `feat(serve-lsp): wire parser.AnchorsForFile` | `4ee9bdb` | 真实 parser 集成 + 3 个 e2e 测试（hover/definition/shutdown） |
| 2026-10-13 | `chore: ignore .d-test/` | `c250845` | 驱动器隔离 |
| 2026-10-13 | **`feat(serve-lsp): v1.3 hardening`** | `7c3c030` | textDocumentSync (Full/OpenClose) + errors.Is (P0-5) + logf/notify 注入 (P0-3) + stdio 入口冒烟 (P0-1) + Exit 注入 (P2-1) + 长 body UTF-8 截断 (P1-6) + 1000× 重复稳定性 (P2-4) + 真实 parser e2e + 路径规范化回归；**测试总数 8 → 28** |
| 2026-10-15 | `git push origin feat/serve-lsp` | `7c3c030` (push) | 推送到 `origin/feat/serve-lsp`；本地 7c3c030 = 远端 7c3c030 一致 |
| 2026-10-15 | **②-B** LSP SDK 集成（`go.lsp.dev/protocol v1.0.1` + `jsonrpc2` + `uri`） | `7c3c030` (依赖入 `go.mod`) | RFC `2026-10-07-lsp-sdk.md` §3 拍板 + `2026-10-12-lsp-decisions.md` §1 explicit approval 知悉 LSP 3.18 vs 3.17 偏差 |

---

## 9. 长期愿景（≥3 个月）

> **慎给阶段性实施提案**（AGENTS.md §5 纪律）。本节**只**列"BD 模块视角下，Kron 项目值得 push 的方向"，**不**写"v1.3 做 X / v1.5 做 Y"。

- **多根仓库**：BD 模块最有机会 hold 跨仓库场景
- **VSCode Marketplace + Cursor 复用**：一次发布双端受益
- **LSP SDK 化**：让 serve-lsp 真正"产品级"，而不是 demo
- **AI 协作意图账本**：把"为什么这么改"写进 `.kron/intents/<slug>.md` body，让下一个 AI / 人类能秒懂

---

## 10. 附录：AI 自动化协议

### 10.1 启动协议

每次 session 开始，AI 必须：
1. **读取** `docs/tracking/homofluctus.md`（本文件）
2. **扫描** `git status` + `git log --oneline -5` 看是否你刚 commit 过
3. **报告**："📋 读到 HomoFluctus 进度文件。本周 4 任务，当前在 [X] 状态。需要我现在做什么？"

### 10.2 关闭协议

每次 session 结束，AI 必须：
1. **问**："本次 session 完成/开始/卡了哪些任务？"
2. **改**本文件对应行
3. **报告**："已更新 §X.Y 行 [N] 处"

### 10.3 周回顾协议

每周日 20:00（本地），AI 提醒你：
- "📅 本周日：做周回顾。回 §7 本周目标，标记完成/未完成，未完成项滚动到下周或放弃。"
- 你确认后，AI 刷新 §7 + 把已完成项移入 §8

### 10.4 冲突处理

如果本文件与 `docs/abstractDesign/architecture.md` 冲突：
- **架构真理**胜（`architecture.md` §〇 铁律）
- 本文件**不**改架构，只改个人进度

---

> **最后一行**：HomoFluctus，**你是项目里"两个真正接触人类 + 磁盘"的主线之一**。别让 4 块吓到——先 pick 1 块（② LSP PR 合并），跑通"完成 → 移动到 §8"这个循环，**节奏感**比"全栈铺开"更重要。
