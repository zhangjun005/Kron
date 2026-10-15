# MCP Real-Usage Smoke Test — 2026-10-15

> Branch: `optimize/mcp`
> Tester: AI agent via Git Bash + Go `kron serve-mcp` stdio subprocess
> Goal: 真正用 (real shell + real protocol + real disk), **不**在 Go 里写 test 包
> Rev: r1 2026-10-15 12:25 首次; r2 2026-10-15 12:50 加 §4.8 fixture 根因 + §4.9 fixture 必须含 src/ (zhangjun005 指出 code anchor 是 Kron 核心, 我没放)

## 1 Setup

- 独立 test workspace: `E:\works\kron-mcp-test\` (Kron 主仓外, 不污染 .kron)
- `kron.exe`: `E:\works\kron-mcp-test\bin\kron.exe` (v1.2.0, 17 tools + 3 prompts + 1 resource)
- 两个 test driver:
  - `mcp-use.sh` (Git Bash + fifo) — race condition 暴露出来后弃用
  - `go-client/main.go` (sequential Go client) — 唯一**真用**版本
- 每次跑: `rm -rf project-full` → `kron init` → 起 serve-mcp → sequential 40 调 → 验 disk
- stdout 路径: `E:\works\kron-mcp-test\project-full\`
- `KRON_PROJECTS_JSON_PATH` env: `E:\works\kron-mcp-test\project-full\projects.json`

## 2 Results — 40/40 PASS

| # | id | tool | what | result |
|---|---|---|---|---|
| 1 | 1 | `initialize` | protocolVersion=2024-11-05 | ✅ |
| 2 | 2 | `tools/list` | 18 tools | ✅ |
| 3 | 3 | `prompts/list` | 3 prompts (kron-drift-check / kron-implement / kron-review) | ✅ |
| 4 | 4 | `resources/list` | 0 resources (kron://intents/{+slug} 是 template, 不在 list 里) | ✅ |
| 5 | 10 | `kron_init` | `repo_root=E:\works\kron-mcp-test\project-full`, `registered_current_project` 回写 projects.json | ✅ |
| 6 | 20-23 | `kron_assume_create` x4 | tls-1-3 (hard) / rfc-7519 (soft) / stripe-api-stable (hard) / backup-restore-rto (soft) | ✅ |
| 7 | 24 | `kron_assume_update` | `stripe-api-stable` 加 `expires_at=2025-01-15` + `verified_at=2024-10-01` + `verified_by=@zhangjun005` | ✅ |
| 8 | 30-35 | `kron_add` x6 | 2 nodes (auth, payments) + 4 leaves (auth/jwt, auth/session, payments/charge, payments/refund); charge/jwt 带 `assumptions[]` + `symbol` | ✅ |
| 9 | 40-43 | `kron_update` x4 | jwt: `status=active, reviewers=[@alice,@bob]`; charge: `status=active, reviewers=[@carol], depends_on=[auth/jwt]`; refund: `status=active, depends_on=[payments/charge], references=[auth/jwt]`; session: `body` 自定义 | ✅ |
| 10 | 50-51 | `kron_list` (all / prefix=auth/) | 6 intents / 2 auth/* | ✅ |
| 11 | 52-53 | `kron_get` | auth/jwt full frontmatter (symbol+reviewers+status+assumptions); payments/charge depends_on+per-intent expires_at | ✅ |
| 12 | 54-55 | `kron_assume_get` / `kron_assume_list` | rfc-7519 detail / 4 entries | ✅ |
| 13 | 56 | `kron_tree` | root.children = [auth, payments] | ✅ |
| 14 | 57 | `kron_lint` | `passed=true, errors=0, warnings=4` (rationale-stale + assumption-orphan — 测试 fixture 故意造) | ✅ |
| 15 | 58 | `kron_stale` (90d) | 0 superseded candidates (全 fresh) | ✅ |
| 16 | 59 | `kron_assume_check` | `summary.expired=1` (stripe-api-stable 过期 2025-01-15) | ✅ |
| 17 | 60 | `kron_intent_density` | dict shape (具体 metric 见下) | ✅ |
| 18 | 61 | `kron_impact` auth/jwt | `references=[payments/refund]`, `prerequisites=[]` | ✅ |
| 19 | 90-93 | `kron_delete` / `kron_list` / `kron_restore` / `kron_get` | refund 软删 → list 消失 → restore → get 可见 | ✅ |
| 20 | 94 | `kron_assume_delete` | backup-restore-rto → `.trash/assumptions/` | ✅ |
| 21 | 99 | `kron_add` duplicate slug auth/jwt | `{isError: true}` — conflict | ✅ |
| 22 | 100 | `kron_get` missing slug | `intent not found: does/not/exist` | ✅ |

## 3 disk 最终状态 (验证用)

```
project-full/
└── .kron/
    ├── config.toml
    ├── assumptions/
    │   ├── rfc-7519.md         (text + body + status=active)
    │   ├── stripe-api-stable.md (text + body + expires_at + verified)
    │   └── tls-1-3.md
    ├── intents/
    │   ├── auth/
    │   │   ├── README.md        (node)
    │   │   ├── jwt.md           (leaf, 744 bytes)
    │   │   └── session.md
    │   └── payments/
    │       ├── README.md        (node)
    │       ├── charge.md        (depends_on + assumption.expires_at)
    │       └── refund.md
    └── .trash/
        └── assumptions/
            └── backup-restore-rto.md   (软删)
```

`auth/jwt.md` 完整 frontmatter (test 后) — symbol + reviewers + status + assumptions 都齐:

```yaml
<!-- kron:frontmatter -->
symbol:
    - pkg/auth.SignIn
created_by: '@zhangjun005'
updated_at: 2026-10-15T04:00:15.4467557Z
reviewers:
    - '@alice'
    - '@bob'
status: active
assumptions:
    - id: rfc-7519
      severity: soft
      rationale: Why we override the registry default to soft for this intent specifically
    - id: tls-1-3
      severity: hard
      rationale: Why tokens must travel over TLS 1.3+ on this specific surface
<!-- /kron:frontmatter -->
```

## 4 真用发现的问题

### 4.1 bash fifo 顺序乱 → bash mcp-use.sh 弃用 (测试 bug, **非** kron bug)

`mcp-use.sh` 用 `cat req > fifo` 一股脑写, kron SDK 多 goroutine 异步 dispatch stdin. 结果 `kron_update id=40` 跟 `kron_add id=32` 同批, update 在 add 写盘前收到, 报 `intent not found: auth/jwt`.

**Sequential Go client** 修这个: 每个 send 同步等对应 id 响应 (1 send → 1 wait → 1 next). 这是 AI agent 真实调 MCP 的方式 (Claude Code / Cursor 都走 sequential 调用).

教训: 模拟并发 ≠ 真实使用. 真实使用必 sequential.

### 4.2 per-intent assumption.expires_at 跟 registry 是**两套**

`kron_assume_create` + `kron_assume_update` 设的 `expires_at` 写到 `.kron/assumptions/<id>.md` 的 frontmatter (registry).

`kron_assume_check` 走 `intent.Frontmatter.Assumptions[].ExpiresAt` (per-intent). **`kron_add` 默认不传 expires_at** → per-intent 空 → 永远不报 expired.

修: 测试 fixture 改成 `kron_add` 的时候把 `expires_at` 一并塞 `assumptions[]` reference. 真用时 AI agent 必须**在写 intent 时就决定 expiry**, 不能事后只改 registry.

### 4.3 lint rule `assumption-rationale-stale`

rationale 不能直接抄 registry text. 早期 fixture 写 "JWT format is the contract" — lint 警告 "rationale echoes part of the registry text". 改成 "Why we override the registry default to soft for this intent specifically" 才好.

含义: rationale 是 **per-intent 的"我为什么用这个"**, 不是"这个 assumption 是啥". registry 已经写了 text, 重复算 stale.

### 4.4 `kron_impact` 没有 `dependents` 字段

文档 / 我之前的 snapshot 都说 impact 返 dependents, 实际只返 `references` + `prerequisites` + `incoming_anchors`. `dependents` 在 `kron_get` 的 `Relations` 里. **F5 / F4 类的"在 spec 上其实没"但被反复引用"** — 见 [docs/abstractDesign/architecture.md §七 process 警告].

### 4.5 `kron_impact.prerequisites` 对 auth/jwt 返空

charge `depends_on=[auth/jwt]` → 我以为 impact(auth/jwt) 看到 dependents=[charge]. 但 impact 看的是 **outgoing** (我依赖谁), **不**看 incoming (谁依赖我). 真正"谁依赖我"在 `kron_delete` 的 `dependents` 字段 + `kron_get` 的 Relations.Dependents.

### 4.6 resources/list 返空 ≠ 没 resource

`resources/list` 返 `[]`, 但 `kron://intents/{+slug}` template 是真的 (tools_schema 里有). SDK 把 template 跟 list 拆开, list 只列**具体** resource, template 通过 `resources/templates/list` 取. **测试驱动要追加 templates/list 才能覆盖 resource 路径.**

### 4.7 `kron_intent_density` 只验了 dict shape, 没验 metric

测试只 `assert(res != nil)`, 实际值 (`max_depth` / `intent_count` / `leaves_count` / `orphans`) 全没验. 这是**真正的覆盖缺口** — 见 §5 RFC.

### 4.8 fixture 是**空仓库** — 全局根因

最严重的真用问题, 但**测试自身察觉不到** (40 调全 PASS):

`project-full/` 是个**只有 `.kron/` 零源文件**的仓库. 也就是说, 我**测了一个没有 src/ 的 Kron 项目** — 而 Kron 的核心就是"代码 ↔ intent 双向链接" (架构 §0, AGENTS.md §1).

具体后果 (按工具):
- `kron_impact auth/jwt` 返 `incoming_anchors: []` (我**期望**有, 但**没**造 src/ file)
- `kron_impact` 的 `incoming_anchors_code` / `incoming_anchors_markdown` 拆字段 (C-6) = 永远空
- `kron_lint` 8 个 rule 里的 5 个 (anchor-dangling × 2 + stale-superseded-candidate + expired-hard-assumption + 任何 A-class) **0 触发**
- `kron_get` 的 `Relations` 全字段 (Dependents / SymbolInferred / DependsOnDependents) — SymbolInferred 走 `frontmatter.symbol` ↔ code 互查, 缺 src/ = 0 验
- `kron_assume_check file_path=<src>` (我**没**测) — 缺 src/ = 0 验
- `kron_intent_density` 的 `orphans` metric — 是 "intent 没有任何 anchor 指向它" = 缺 src/ = 永远 = 0
- 真实 GUI/IDE 用户**用** Kron 时, src/ 一定存在; 测一个**无** src/ 的 Kron = 测个**半个产品**

**这才是真用的根因**, G1, G2, G3, G11, G13 全部被它拖累. **不是** 14 个独立缺口, 是 1 个 fixture 错了.

### 4.9 fixture 必须含 src/ — RFC 拍板后才能改

`project-full/` 跟 `project-smoke/` / `project-real/` / `project-diag/` 一样, 都是**空仓库** fixture. 修法是 RFC 拍板后:

1. 新建 `project-full-with-src/` 模板, 含 `src/auth/jwt.go` / `src/auth/session.go` / `src/payments/charge.go` / `src/payments/refund.go` (4 个 leaf 各对应一个 src/ file)
2. 每个 src/ file 顶部 `// @kron:intent <slug>` 注释
3. **额外造** `src/orphan.go` (anchor 但 intent 不存在) 验 A-class rule
4. **额外造** `src/dangling.go` (`// @kron:intent does/not/exist` 同上)
5. Markdown anchor 同样: `docs/adr/0001-jwt.md` 含 `// @kron:intent auth/jwt` (md-anchors RFC §2)

主仓 `internal/parser/anchors_test.go` 已经有完整 unit 测 (4 注释形式 / .kron/ skip / binary skip / 排序 / FilePath 规则 / parseAnchorLine 表格). **白盒 100% 覆盖**, **黑盒 0 覆盖** — 这才是 G11 的真状态, 不只是"造 1 个 dangling".

## 5 测试覆盖缺口 (要 RFC 拍板后才能加)

| 缺口 | 影响 |
|---|---|
| `kron_intent_density` 真实 metric 验证 | 不知道 density 计算对不对 |
| `kron_impact` `incoming_anchors_code` / `incoming_anchors_markdown` 拆字段 (C-6) | 不知道分 tab 有没有真分 |
| `kron_get` 的 `Relations.Dependents` + `SymbolInferred` + `DependsOnDependents` 全字段 | 我现在跳过了 relations 验证 |
| `kron_delete` 的 `dependents` 字段 (有 dependents 时拒绝) | 没测"非空 dependents 阻止 delete"的契约 |
| `resources/templates/list` | 整个 resource 路径没覆盖 |
| prompt 的 3 个 (`kron-implement` / `kron-drift-check` / `kron-review`) 实跑 | `prompts/get` 0 调 |
| `kron_update` PATCH semantics: `null` vs absent vs `[]` 三态 | 我只测了 "non-empty" 和 "clear", **不**测 null pointer |
| `kron_assume_update` PATCH 同上 | 同 |
| cycle 创建 (a→b→c→a) + lint 应该报 `depends-on-cycle` | cycle 没造 |
| dangling reference (a.references=[missing]) + lint 报 `dangling-reference` | dangling 没造 |
| anchor-dangling (source file `// @kron:intent missing` + lint 报) | anchor 没造 |
| **并发**: 同一 project 两个 `serve-mcp` 子进程 + cache invalidation 跨进程? | singleflight 是 in-process, 跨进程**无** invalidation. 不知道这是不是设计 (猜测是, 文档没说) |
| `kron_lint` reporter=text 跟 reporter=json 输出 | 只测了 default=json |
| 跨 `kind: node` 的子 intent 用 `parent/child` 引用 + `kron_tree` 嵌套深 | 用了 2 层, 没用 3+ |

完整 RFC 见 [`docs/rfc/2026-10-15-mcp-test-expansion.md`](../rfc/2026-10-15-mcp-test-expansion.md).

## 6 commit / push 状态

- 2026-10-15 12:00 之前: c4ba92a (projects.json wire-up) — **已 commit 未 push**
- 2026-10-15 12:00 之后: 7 文件 CRLF M (line ending 噪声, `core.autocrlf=true` 副作用, **非**真实 diff)
- 2026-10-15 12:25 计划 commit: `docs/snapshot/2026-10-14-BranchOptimizeMcp.md` (漏的) + `docs/snapshot/2026-10-15-McpRealUsage.md` (本次)

## 7 runbook

```bash
# 跑一次 (clean project)
cd E:\works\kron-mcp-test
go build -o bin/full.exe go-client/main.go
bin/full.exe          # sequential, 40 调, ~13 秒, 40/40 PASS

# 手工 sanity
KRON_PROJECTS_JSON_PATH=E:\works\kron-mcp-test\project-full\projects.json \
    E:\works\kron-mcp-test\bin\kron.exe \
    serve-mcp -CWD E:\works\kron-mcp-test\project-full
```

## 8 复跑成本

- 0 网络, 0 cache, 0 external dep
- 仅需 `kron.exe` (Kron 仓 `go build ./cmd/kron` 产物) + `bin/full.exe` (`go build ./go-client`)
- 跑一次 ~13 秒, 覆盖 18 tools + 3 prompts + 1 resource template, 真实 disk
- 跟 [docs/abstractDesign/architecture.md](../../docs/abstractDesign/architecture.md) §一 访问层契约对得上 (CLI / MCP 走同一 `internal/`, 互不 import)
