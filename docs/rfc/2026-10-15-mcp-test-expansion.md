# RFC: MCP 真实使用测试扩展（v1.2 覆盖缺口补齐）

| 字段 | 值 |
|---|---|
| **状态** | **待 zhangjun005 拍板** (2026-10-15 r2 修订: 加 G0 fixture 根因 + §2.4 fixture 模板) |
| **作者** | AI assistant, 经 zhangjun005 委托 |
| **创建日期** | 2026-10-15 |
| **修订** | r1 2026-10-15 12:25: 14 项; r2 2026-10-15 12:50: 加 G0 (fixture 缺 src/) + §2.4 fixture 模板 (5 src/ + 1 md anchor + 1 orphan), 14 → 15 项, 决策点 5 → 9 个 |
| **目标版本** | v1.2 修补 (不涉及 v1.3 范围) |
| **上游** | [`docs/snapshot/2026-10-15-McpRealUsage.md`](../snapshot/2026-10-15-McpRealUsage.md) §4.8 fixture 根因 + §5 覆盖缺口 |
| **目的** | **本文仅 RFC**, **不**含测试代码. 测试代码待拍板后单独 PR |

---

## 1 动机

2026-10-15 跑了一次 MCP 真实使用 smoke test (sequential Go client, 40 调, 40/40 PASS), 详见 [`2026-10-15-McpRealUsage.md`](../snapshot/2026-10-15-McpRealUsage.md). 发现 **15 个覆盖缺口**:

| # | 缺口 | 影响 | 严重性 |
|---|---|---|---|
| **G0** | **fixture 是空仓库, 0 源文件** — `project-full/` 只有 `.kron/`, 没有任何 `src/` code 或 `docs/` markdown 含 `@kron:intent` 注释. 这是**真用根因**, G1, G2, G3, G11, G13 全部被它拖累 | 见 [snapshot §4.8](../snapshot/2026-10-15-McpRealUsage.md) 详细后果 | 🔴 **根因** |
| G1 | `kron_intent_density` 真实 metric (`max_depth` / `intent_count` / `leaves_count` / `orphans`) 没验 | 不知道 density 计算对不对 (跟 G0 强相关: `orphans` 需要 src/ anchor) | 🟡 必补 |
| G2 | `kron_impact` `incoming_anchors_code` / `incoming_anchors_markdown` 拆字段 (C-6) 没验 | 不知道分 tab 是否真分 (跟 G0 强相关: 缺 src/ = 永远空) | 🟡 必补 |
| G3 | `kron_get` 的 `Relations.Dependents` + `SymbolInferred` + `DependsOnDependents` 全字段 | 整个 relations graph 没验 (跟 G0 强相关: SymbolInferred 走 symbol↔code 互查) | 🟡 必补 |
| G4 | `kron_delete` 的 `dependents` 字段 (有 dependents 时拒绝) | "有 dependent 阻止 delete" 契约没验 | 🟠 重要 |
| G5 | `resources/templates/list` (跟 `resources/list` 拆) | 整个 resource 路径没覆盖 | 🟡 必补 |
| G6 | prompt 的 3 个 (`kron-implement` / `kron-drift-check` / `kron-review`) 实跑 (`prompts/get` 0 调) | 3 个 prompt 是 v1.2 新增, 没实跑 = 没 E2E | 🟠 重要 |
| G7 | `kron_update` PATCH semantics: `null` vs absent vs `[]` 三态 | 只测了 "non-empty" 和 "clear", **不**测 null pointer | 🟠 重要 |
| G8 | `kron_assume_update` PATCH 同上 | 同 | 🟢 一致性 |
| G9 | cycle 创建 (a→b→c→a) + lint 应该报 `depends-on-cycle` | cycle 没造 = 规则没验 | 🟠 重要 |
| G10 | dangling reference + dangling depends_on | dangling 没造 = C-class 规则没验 | 🟠 重要 |
| G11 | anchor-dangling (source file `// @kron:intent missing` + lint 报) | A-class 规则没验 (跟 G0 强相关: 没 src/) | 🟠 重要 |
| G12 | 跨 `kind: node` 3+ 层嵌套 | 只造了 2 层 (`auth/jwt`) | 🟢 边缘 |
| G13 | `kron_lint` reporter=text 跟 reporter=json 输出 | 只测了 default | 🟢 边缘 |
| G14 | **跨进程 cache invalidation** (同一 project 两个 serve-mcp 并发) | singleflight 是 in-process, 跨进程**无** invalidation. 不确定是设计还是 bug | 🟠 **架构问题, 不是测试问题** |

**不**补的后果:

- **lint 规则 = 测试驱动 = lint 规则是 LLM 写的, 不实测 = 不知道对不对**. v1.2 lint 跑过的"无 error"可能漏报 8 个 rule 里的多个 (跟 G0 强相关: 5 个 rule 0 触发)
- **relations graph 0 验** = `kron_impact` 是 GUI/IDE 的核心查询, 错了用户看不到 (跟 G0 强相关: SymbolInferred 0 验)
- **PATCH semantics 0 验** = 客户端 SDK 拼错 `null` vs `[]` 时, server 行为不知道
- **跨进程 cache** = 多窗口 GUI / IDE + CLI 同 project 时, 修改可能**不**生效
- **G0 fixture 空仓库** = 真用测的**半个产品**, anchor 双向链接完全没跑 — 这是**最大**单一缺口

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

### 2.3 要补的 15 项 (G0-G14) — 拍板**做哪些**

| # | 拍板建议 | 理由 |
|---|---|---|
| **G0** | **做** (前置) | fixture 缺 src/ 是根因, 修了 G0 自动带 5 个 rule + Relations + impact anchor 字段全部可验. **必须先做** |
| G1 | **做** | density 是 v1.2 新增 tool, 不验 = 不知道计算对错 (跟 G0 一起) |
| G2 | **做** | C-6 是规范拍板, 不验 = GUI 拆 tab 时崩 (跟 G0 一起) |
| G3 | **做** | relations graph 是 impact/delete 基础, 不验 = GUI 不可用 (跟 G0 一起) |
| G4 | **做** | "有 dependent 阻止 delete" 是 RFC §1.4 拍板契约 |
| G5 | **做** | resource template 跟 list 拆, 是 SDK 必修, 不验 = GUI 无法读 intent |
| G6 | **做** | 3 prompt 是 v1.2 新增, prompts/get 都没调 = 等于 0 E2E |
| G7 | **做** | PATCH 三态是 spec 拍板, 不验 = 客户端拼错都不知道 |
| G8 | **跟 G7** | 同 handler, 加 3 个 case 边际成本 0 |
| G9 | **做** | depends-on-cycle 是 C-class 规则, 不造 cycle 不验 = 规则是死代码 |
| G10 | **做** | dangling 两条规则同上 |
| G11 | **做** | A-class 规则, 不造 anchor 不验 = 规则是死代码 (跟 G0 一起) |
| G12 | **不做** | 边际信息少, 3+ 层 vs 2 层 几乎不验新东西 |
| G13 | **做** | reporter=text 是 human-readable 路径, 不验 = GUI/IDE 调 lint 的文本面板会拿不到 |
| G14 | **独立 RFC, 这次不做** | 跨进程 cache 是**架构问题**, 不是测试问题. 需要单独 RFC 拍板 "in-process cache 是设计" 还是 "需要 fsnotify" |

**结果**: G0-G11, G13 共 **14 项做**. G12, G14 不做.

**执行序**: G0 先做 (改 fixture 模板), 然后 G1, G2, G3, G11 一起 (因为都依赖 G0 的 src/ + md anchor). G4-G10, G13 各自独立.

### 2.4 fixture 模板 (拍板 — 修 G0)

**拍板**: 新建 `project-full-with-src/` 模板, **不**改 `project-full/` (保留 happy path baseline).

**模板结构** (5 个 src/ file + 2 个 docs/ anchor + 6 个 intent + 4 个 assumption):

```
project-full-with-src/
├── .kron/
│   ├── config.toml
│   ├── intents/
│   │   ├── auth/
│   │   │   ├── README.md            (node)
│   │   │   ├── jwt.md               (leaf, symbol=pkg/auth.SignIn)
│   │   │   └── session.md
│   │   └── payments/
│   │       ├── README.md            (node)
│   │       ├── charge.md            (leaf, symbol=pkg/payments.Charge)
│   │       └── refund.md
│   └── assumptions/
│       ├── rfc-7519.md
│       ├── stripe-api-stable.md
│       └── tls-1-3.md
├── src/                             (NEW — fixture G0 fix)
│   ├── auth/
│   │   ├── jwt.go                   // @kron:intent auth/jwt
│   │   │                            // additional lines for line-number test
│   │   └── session.go               // @kron:intent auth/session
│   ├── payments/
│   │   ├── charge.go                // @kron:intent payments/charge
│   │   └── refund.go                // @kron:intent payments/refund
│   └── broken/
│       └── orphan.go                // @kron:intent does/not/exist  (for anchor-dangling)
├── docs/                            (NEW — md anchor)
│   └── adr/
│       └── 0001-jwt-rotation.md     // @kron:intent auth/jwt  (md anchor)
└── go.mod                           (minimal)
```

**`src/*.go` 内容** 至少 5 行, 含 `package` 声明 + `// @kron:intent <slug>` 注释 (line 1) + 至少 1 个 `func` (line 5+). 这样 `kron_impact` 的 `incoming_anchors` 有真实数据, `kron_lint` 能报 `anchor-dangling` (orphan.go).

**`docs/adr/0001-jwt-rotation.md`** 含 md anchor 验 `incoming_anchors_markdown` (C-6 拆字段).

**`kron_impact auth/jwt`** 应该返:
- `incoming_anchors: [{file_path:"src/auth/jwt.go", line:1}, {file_path:"docs/adr/0001-jwt-rotation.md", line:1}]` (2 个)
- `incoming_anchors_code: [{src/auth/jwt.go, 1}]` (1 个)
- `incoming_anchors_markdown: [{docs/adr/0001-jwt-rotation.md, 1}]` (1 个)
- `references: [payments/refund]` (反向)
- `prerequisites: []` (auth/jwt 自己没 depends_on)
- 错配的 `src/broken/orphan.go` 让 `kron_lint` 报 `anchor-dangling`

**fixture 模板生成方式**: 不手工写, Go driver 跑前**自动**生成 (mkdir + WriteFile). 这样 fixture 跟 assertion 在同一 source of truth, 改一处同步.

### 2.5 lint 规则覆盖策略

**拍板**: 8 个 lint 规则 (A-class 2 + B-class 1 + C-class 4 + S-class 2 + B-3 6) **每个**至少造一个 positive case + 一个 negative case (干净时不报).

**理由**:
- 现在测试 fixture 故意造了 4 个 warning (rationale-stale x3 + assumption-orphan x1), 验证 lint **能**报
- 但**没**造: anchor-dangling / frontmatter-invalid / depends-on-cycle / dangling-reference / dangling-depends-on / stale-superseded-candidate / expired-hard-assumption / assumption-registry-id-mismatch / assumption-mixed-form / assumption-file-id-mismatch / assumption-rationale-required
- 等于 lint 规则的 **覆盖矩阵没建**

**`anchor-dangling` 验** (A-class 2 个 rule 之一) 走 G0 fixture 的 `src/broken/orphan.go`. 主仓 `internal/parser/anchors_test.go` 已经有完整 unit 测 4 注释形式 (Go `//` / Python `#` / SQL `--` / Erlang `%`) / .kron/ skip / binary skip / 排序 / FilePath 规则 / parseAnchorLine 18 个 case — 白盒 100%, 黑盒 0%. **L3 (E2E) 不重 L1**, 只验跨"anchor + intent 互查"的部分, 也就是 dangling.

**`depends-on-cycle` 验** (C-class): 6 个 intent 走 `a→b→c→a` 拓扑, lint 应该报 N 个 cycle (overlap 允许, v1 spec).

**`stale-superseded-candidate` + `expired-hard-assumption`** 验: 调 `kron_lint` 时带 `days_threshold` + 用 os.Chtime 改 `auth/jwt.md` mtime 到 100 天前.

实施: 13 个新 case (8 rule positive + 5 重要 rule negative) 在 L3 driver 加 ~200 行. **不**改 lint 代码本身 (那是另一 PR).

### 2.6 fixture 复用 vs 重建

**拍板**: 14 个新 case 走**新建独立 fixture** (新 project dir, 新 batch_id), **不**复用 10-15 fixture.

**理由**:
- 10-15 fixture 是 "happy path" (4 假设 + 6 intent, 完整 graph, **无** src/)
- 14 个 lint + 关系 case 需要 **故意错** (orphan, dangling, cycle, stale, expired), **不**能跟 happy path 混
- 14 case 各自独立, 一个 case 跑前 wipe (rm -rf + kron init)
- 14 case 跑完, **不**影响 10-15 fixture 的可重复性 (隔离)

实施: 14 个新 Go file (`go-client/lint_<rule>.go`) 各跑独立 project, **不**共享.

### 2.7 driver 形态

**拍板**: 沿用 `go-client/main.go` 的 **sequential Go client 模式**. **不**写 bash, **不**写 PowerShell.

理由 (见 [snapshot §4.1](../snapshot/2026-10-15-McpRealUsage.md)):
- bash fifo 顺序乱, 弃用
- PowerShell `Get-Content` / `Out-File` 在 Windows + Git Bash 路径混用, 复杂
- Sequential Go 是真实使用方式, 跟 Claude Code / Cursor / VSCode 调 MCP 一致

### 2.8 输出 + CI (未来)

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
| **G0 的 fixture 模板是否进 git** | fixture 是 .go file 跑前自动生成, **不**入 git; 真要存档, 放 `E:\works\kron-mcp-test\fixture-templates\` 仓外目录 | 拍板时定 |

## 4 PR 范围 (拍板后)

1. `E:\works\kron-mcp-test\go-client\main.go` — 扩现有 driver, 加 14 个新 case (估计 +400 行, 含 G0 fixture 生成 + 13 个 lint/relations case)
2. **不**改 Kron 主仓任何文件
3. **不**写新 Go file (case 全 inline, 跟现有 40 调一起)
4. 跑完更新 [`docs/snapshot/2026-10-15-McpRealUsage.md`](../snapshot/2026-10-15-McpRealUsage.md) §5, 把 14 缺口标 "已验"

**等拍板**.

## 5 决策点

| 决策 | 选项 | 默认 |
|---|---|---|
| 5.1 L3 driver 位置 | a) 仓外 `E:\works\kron-mcp-test\` (现) / b) Kron 仓 `e2e/` 子目录 / c) 单独 kron-e2e 仓 | (a) 仓外 — 架构铁律 §1.1 |
| 5.2 14 case 全 inline 跟 40 调一起, 还是独立 file | a) inline (现 main.go 模式) / b) 14 个 subdirectory / c) 14 个 Go file | (a) inline — 顺序读, 一遍跑完 |
| 5.3 fixture 复用 | a) 14 个新独立 project / b) 复用 10-15 project, 走 kron_update patch | (a) — 见 §2.6 |
| 5.4 lint 规则覆盖策略 | a) 8 rule 全 positive / b) 13 重要 rule positive | (a) — 全 |
| 5.5 reporter=text | a) 加进 main.go / b) 单独 case file | (a) — inline |
| 5.6 **G0 fixture 模板** | a) 跑前 Go driver 自动生成 / b) 手写模板文件存档 / c) 模板 + driver 混合 | (a) — fixture 跟 assertion 同一 source of truth, 改一处同步 |
| 5.7 **G0 fixture 模板是否进 git** | a) 不进 (driver 内 string) / b) 仓外 fixture-templates/ / c) Kron 仓 testdata/ | (a) 不进 — 仓外独立 workspace 本来就不入 git |
| 5.8 **md anchor 验** | a) 包含 `docs/adr/0001-jwt-rotation.md` (1 个 md anchor) / b) 多 md anchor (3 个分散) | (a) — 1 个够验 C-6 拆分 |
| 5.9 **orphan anchor 数量** | a) 1 个 (`src/broken/orphan.go` 指向 `does/not/exist`) / b) 多个 (覆盖 batch error) | (a) — 1 个够 |

**需要 zhangjun005 拍板: 5.1, 5.2, 5.3, 5.4, 5.5, 5.6, 5.7, 5.8, 5.9 + §2.3 表格 (15 项) + §2.4 fixture 模板**.
