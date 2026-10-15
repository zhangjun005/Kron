# RFC: MCP 真实使用测试扩展（v1.2 覆盖缺口补齐）

| 字段 | 值 |
|---|---|
| **状态** | **待 zhangjun005 拍板** |
| **作者** | AI assistant, 经 zhangjun005 委托 |
| **创建日期** | 2026-10-15 |
| **目标版本** | v1.2 修补 (不涉及 v1.3 范围) |
| **上游** | [`docs/snapshot/2026-10-15-McpRealUsage.md`](../snapshot/2026-10-15-McpRealUsage.md) §5 覆盖缺口 |
| **目的** | **本文仅 RFC**, **不**含测试代码. 测试代码待拍板后单独 PR |

---

## 1 动机

2026-10-15 跑了一次 MCP 真实使用 smoke test (sequential Go client, 40 调, 40/40 PASS), 详见 [`2026-10-15-McpRealUsage.md` §5](../snapshot/2026-10-15-McpRealUsage.md). 发现 **12 个覆盖缺口**:

| # | 缺口 | 影响 | 严重性 |
|---|---|---|---|
| G1 | `kron_intent_density` 真实 metric (`max_depth` / `intent_count` / `leaves_count` / `orphans`) 没验 | 不知道 density 计算对不对 | 🟡 必补 |
| G2 | `kron_impact` `incoming_anchors_code` / `incoming_anchors_markdown` 拆字段 (C-6) 没验 | 不知道分 tab 是否真分 | 🟡 必补 |
| G3 | `kron_get` 的 `Relations.Dependents` + `SymbolInferred` + `DependsOnDependents` 全字段 | 整个 relations graph 没验 | 🟡 必补 |
| G4 | `kron_delete` 的 `dependents` 字段 (有 dependents 时拒绝) | "有 dependent 阻止 delete" 契约没验 | 🟠 重要 |
| G5 | `resources/templates/list` (跟 `resources/list` 拆) | 整个 resource 路径没覆盖 | 🟡 必补 |
| G6 | prompt 的 3 个 (`kron-implement` / `kron-drift-check` / `kron-review`) 实跑 (`prompts/get` 0 调) | 3 个 prompt 是 v1.2 新增, 没实跑 = 没 E2E | 🟠 重要 |
| G7 | `kron_update` PATCH semantics: `null` vs absent vs `[]` 三态 | 只测了 "non-empty" 和 "clear", **不**测 null pointer | 🟠 重要 |
| G8 | `kron_assume_update` PATCH 同上 | 同 | 🟢 一致性 |
| G9 | cycle 创建 (a→b→c→a) + lint 应该报 `depends-on-cycle` | cycle 没造 = 规则没验 | 🟠 重要 |
| G10 | dangling reference + dangling depends_on | dangling 没造 = C-class 规则没验 | 🟠 重要 |
| G11 | anchor-dangling (source file `// @kron:intent missing` + lint 报) | A-class 规则没验 | 🟠 重要 |
| G12 | 跨 `kind: node` 3+ 层嵌套 | 只造了 2 层 (`auth/jwt`) | 🟢 边缘 |
| G13 | `kron_lint` reporter=text 跟 reporter=json 输出 | 只测了 default | 🟢 边缘 |
| G14 | **跨进程 cache invalidation** (同一 project 两个 serve-mcp 并发) | singleflight 是 in-process, 跨进程**无** invalidation. 不确定是设计还是 bug | 🟠 **架构问题, 不是测试问题** |

**不**补的后果:

- **lint 规则 = 测试驱动 = lint 规则是 LLM 写的, 不实测 = 不知道对不对**. v1.2 lint 跑过的"无 error"可能漏报 8 个 rule 里的多个
- **relations graph 0 验** = `kron_impact` 是 GUI/IDE 的核心查询, 错了用户看不到
- **PATCH semantics 0 验** = 客户端 SDK 拼错 `null` vs `[]` 时, server 行为不知道
- **跨进程 cache** = 多窗口 GUI / IDE + CLI 同 project 时, 修改可能**不**生效

## 2 拍板

### 2.1 测试代码位置 (红线)

**拍板**: 测试驱动**不**进 Kron 主仓.

理由:
- Kron 主仓 `internal/`, `cmd/`, `docs/` 已严格分层 (架构 §〇 铁律 1-9). test driver 是**外部** AI agent 工具, **不**是 Kron 自身测试
- 现有 Go 单元测试 (`internal/parser/..._test.go` 等) 是**白盒**测 `internal/`, 跟 E2E **不**是同层
- E2E 在主仓 = `kron test` 子命令 (架构 §1.1 红线: **不**加 CLI 子命令)

实施: 沿用 `E:\works\kron-mcp-test\` (仓外 workspace). 跑在 dev 机, **不**入 git.

### 2.2 测试分层 (不重叠)

| 层 | 位置 | 范围 |
|---|---|---|
| L1: `internal/*_test.go` | Kron 主仓 | 单包白盒, mock 一切外部 |
| L2: `cmd/kron/*/*_test.go` (现有 `serve_mcp_test.go` 等) | Kron 主仓 | 同进程多 handler 集成 (in-memory) |
| L3: `go-client/main.go` (E2E) | `E:\works\kron-mcp-test\go-client\` | 真子进程 + 真 stdio + 真 disk |
| L4: GUI 客户端测试 (未拍板, v1.3+) | GUI 仓 | GUI 自身 (E2E by Wails) |

**L3 不**重 L1/L2 任何 assertion. L3 只验 **L1/L2 不能验的**:
- 真协议 (JSON-RPC over stdio)
- 真进程生命周期
- 真 disk
- 真 cache (in-process 跨 request)
- 真 lint (跨多 file)
- 真 LLM 视角"我想这么用"是否 work

### 2.3 要补的 14 项 (G1-G14) — 拍板**做哪些**

| # | 拍板建议 | 理由 |
|---|---|---|
| G1 | **做** | density 是 v1.2 新增 tool, 不验 = 不知道计算对错 |
| G2 | **做** | C-6 是规范拍板, 不验 = GUI 拆 tab 时崩 |
| G3 | **做** | relations graph 是 impact/delete 基础, 不验 = GUI 不可用 |
| G4 | **做** | "有 dependent 阻止 delete" 是 RFC §1.4 拍板契约 |
| G5 | **做** | resource template 跟 list 拆, 是 SDK 必修, 不验 = GUI 无法读 intent |
| G6 | **做** | 3 prompt 是 v1.2 新增, prompts/get 都没调 = 等于 0 E2E |
| G7 | **做** | PATCH 三态是 spec 拍板, 不验 = 客户端拼错都不知道 |
| G8 | **跟 G7** | 同 handler, 加 3 个 case 边际成本 0 |
| G9 | **做** | depends-on-cycle 是 C-class 规则, 不造 cycle 不验 = 规则是死代码 |
| G10 | **做** | dangling 两条规则同上 |
| G11 | **做** | A-class 规则, 不造 anchor 不验 = 规则是死代码 |
| G12 | **不做** | 边际信息少, 3+ 层 vs 2 层 几乎不验新东西 |
| G13 | **做** | reporter=text 是 human-readable 路径, 不验 = GUI/IDE 调 lint 的文本面板会拿不到 |
| G14 | **独立 RFC, 这次不做** | 跨进程 cache 是**架构问题**, 不是测试问题. 需要单独 RFC 拍板 "in-process cache 是设计" 还是 "需要 fsnotify" |

**结果**: G1-G11, G13 共 **13 项做**. G12, G14 不做.

### 2.4 lint 规则覆盖策略

**拍板**: 8 个 lint 规则 (A-class 2 + B-class 1 + C-class 4 + S-class 2 + B-3 6) **每个**至少造一个 positive case + 一个 negative case (干净时不报).

**理由**:
- 现在测试 fixture 故意造了 4 个 warning (rationale-stale x3 + assumption-orphan x1), 验证 lint **能**报
- 但**没**造: anchor-dangling / frontmatter-invalid / depends-on-cycle / dangling-reference / dangling-depends-on / stale-superseded-candidate / expired-hard-assumption / assumption-registry-id-mismatch / assumption-mixed-form / assumption-file-id-mismatch / assumption-rationale-required
- 等于 lint 规则的 **覆盖矩阵没建**

实施: 13 个新 case (8 rule positive + 5 重要 rule negative) 在 L3 driver 加 ~150 行. **不**改 lint 代码本身 (那是另一 PR).

### 2.5 fixture 复用 vs 重建

**拍板**: 13 个新 case 走**新建独立 fixture** (新 project dir, 新 batch_id), **不**复用 10-15 fixture.

**理由**:
- 10-15 fixture 是 "happy path" (4 假设 + 6 intent, 完整 graph)
- 13 个 lint case 需要 **故意错** (orphan, dangling, cycle), **不**能跟 happy path 混
- 13 case 各自独立, 一个 case 跑前 wipe (rm -rf + kron init)
- 13 case 跑完, **不**影响 10-15 fixture 的可重复性 (隔离)

实施: 13 个新 Go file (`go-client/lint_<rule>.go`) 各跑独立 project, **不**共享.

### 2.6 driver 形态

**拍板**: 沿用 `go-client/main.go` 的 **sequential Go client 模式**. **不**写 bash, **不**写 PowerShell.

理由 (见 [snapshot §4.1](../snapshot/2026-10-15-McpRealUsage.md)):
- bash fifo 顺序乱, 弃用
- PowerShell `Get-Content` / `Out-File` 在 Windows + Git Bash 路径混用, 复杂
- Sequential Go 是真实使用方式, 跟 Claude Code / Cursor / VSCode 调 MCP 一致

### 2.7 输出 + CI (未来)

**拍板**: L3 driver 输出 PASS/FAIL 到 stdout (现有 main.go 模式). **不**生成 JUnit XML / 不接 CI.

理由:
- L3 在 dev 机手跑, 不进 CI
- CI 跑 L1 (白盒) + L2 (handler 集成) 已足够
- L3 是 **"AI agent 自己用之前先验"** 的工具, 不是回归测试

未来如果 GUI 仓要 L3 → 等 v1.3+ GUI 拍板后, GUI 仓自己起 L3, **不**在这 RFC 范围.

## 3 不做 (给后续 RFC 留口子)

| 缺口 | 为什么不做 | 何时做 |
|---|---|---|
| G12 (3+ 层 nesting) | 边际信息少 | v1.3+ GUI 测时一起 |
| G14 (跨进程 cache invalidation) | 架构问题, 需独立 RFC | 单独 RFC 拍板: fsnotify 还是 "单进程 is 设计" |
| 客户端 SDK 行为 (Claude Code / Cursor 怎么拼 PATCH) | 跨公司 SDK, 测不到 | 真实部署时 E2E |
| 性能 (latency, throughput) | `cache.go` 自带 bench (346x 提速) | 现有 bench 够 |
| `kron_impact` `incoming_anchors` 旧字段 vs 新拆字段 一致性 | 旧字段是 deprecated, 验证 deprecated==code+markdown 合并即可, 边际信息少 | 跟 G2 一起验 |

## 4 PR 范围 (拍板后)

1. `E:\works\kron-mcp-test\go-client\main.go` — 扩现有 driver, 加 13 个新 case (估计 +300 行)
2. **不**改 Kron 主仓任何文件
3. **不**写新 Go file (case 全 inline, 跟现有 40 调一起)
4. 跑完更新 [`docs/snapshot/2026-10-15-McpRealUsage.md`](../snapshot/2026-10-15-McpRealUsage.md) §5, 把 13 缺口标 "已验"

**等拍板**.

## 5 决策点

| 决策 | 选项 | 默认 |
|---|---|---|
| 5.1 L3 driver 位置 | a) 仓外 `E:\works\kron-mcp-test\` (现) / b) Kron 仓 `e2e/` 子目录 / c) 单独 kron-e2e 仓 | (a) 仓外 — 架构铁律 §1.1 |
| 5.2 13 case 全 inline 跟 40 调一起, 还是独立 file | a) inline (现 main.go 模式) / b) 13 个 subdirectory / c) 13 个 Go file | (a) inline — 顺序读, 一遍跑完 |
| 5.3 fixture 复用 | a) 13 个新独立 project / b) 复用 10-15 project, 走 kron_update patch | (a) — 见 §2.5 |
| 5.4 lint 规则覆盖策略 | a) 8 rule 全 positive / b) 13 重要 rule positive | (a) — 全 |
| 5.5 reporter=text | a) 加进 main.go / b) 单独 case file | (a) — inline |

**需要 zhangjun005 拍板: 5.1, 5.2, 5.3, 5.4, 5.5 + §2.3 表格 (13 项)**.
