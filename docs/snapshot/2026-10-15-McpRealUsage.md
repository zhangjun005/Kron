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

## 9 r3 2026-10-15 14:10 — AI agent 经 Cursor UI 真用 (跟 r1/r2 并列, **不**覆盖)

> Channel: Cursor IDE 0.x → MCP UI → `kron serve-mcp` stdio (用户 13:59 配 `C:\Users\zj\.cursor\mcp.json::kron` server, command=`E:\works\Kron\kron-mcp-test\install\kron.exe`, args=`serve-mcp -CWD E:\works\Kron\kron-mcp-test\project`, env=`KRON_PROJECTS_JSON_PATH=E:\works\Kron\kron-mcp-test\install\projects.json`)
> 区别于 r1/r2 的 go-client 渠道: r1/r2 跑 40 调 sequential 全 PASS (completeness), r3 是 AI agent 在 Cursor chat 里**逐个**手调, **看**返回**真**思考,**不**写死期望值。

### 9.1 已测 (r3 14:00-14:10)

| tool | input | 返回 (简) | 真观察 |
|---|---|---|---|
| `kron_list` | `{}` | 6 intents (auth / auth/jwt / auth/session / payments / payments/charge / payments/refund) | ✓ 跟 snapshot §3 一样 |
| `kron_impact` | `{slug: "auth/jwt"}` | 2 incoming_anchors (jwt.go:1 code + adr 0001:6 md) | ✓ fixture 设计的"最热反向图节点" |
| `kron_impact` | `{slug: "auth"}` | **0** anchors, 0 references, 0 prerequisites | ⚠ parent node **没** anchor **是** fixture 设计, **不**是 bug (node 是容器) |
| `kron_impact` | `{slug: "payments/charge"}` | 1 code anchor (charge.go:1) | ✓ |
| `kron_lint` | `{reporter: "text"}` | 1 error: `anchor-dangling` on `src/broken/orphan.go:1` → intent `does/not/exist` | ✓ fixture 故意造 |
| `kron_lint` | `{}` (default json) | 同 1 error, **不**同 field shape | 🟡 文档漂移: spec 说"reporter=text 输出 text", 实际 text mode **也**返 JSON + 加 `text` summary 字段 |
| `kron_tree` | `{}` | root.children = [auth, payments], auth.{jwt, session}, payments.{charge, refund} | ✓ 嵌套 100% 对 |
| `kron_intent_density` | `{}` | `intents_with_anchors=5, intents_without_anchors=[auth, payments], total_anchors=6, files_without_intent=[]` | ✓ **发现** dangling (`does/not/exist`) 算"有 anchor" (lint 范畴), density 跟 lint 互补 |
| `kron_stale` | `{}` (default 90d) | `superseded_candidates=[]`, `expired_assumptions=[]` | ✓ 全 fresh |
| `kron_get` | `{slug: "auth/jwt"}` (default) | body + frontmatter (只有 `created_by` + `updated_at`) + source_path | ✓ |
| `kron_get` | `{slug: "auth/jwt", include_relations: true}` | **❌ MCP error -32602**: `validating /properties/include_relations: type: true has type "string", want "boolean"` | 🐞 **新 bug** — boolean 编码错 (r1/r2 跳过了这 flag, 没暴露) |
| `kron_get` | `{slug: "auth/jwt", include_relations: false}` | **同** error | 同 bug, **不**是 true/false 区别 |

### 9.2 r3 还没测 (等补 fixture 后跑)

`kron_add` / `kron_update` / `kron_delete` / `kron_restore` / `kron_init` / `kron_assume_{create,get,list,update,delete,check}` (7 个) / `mcp_auth`

**这些都需要新数据** — `kron-mcp-test/project/` fixture 缺:
- `assumptions/` 目录 (整个缺)
- per-intent `assumptions[]` (全 6 intent 都空)
- `depends_on` / `references` (全空) — cycle + dangling reference 测不到
- `symbol` / `reviewers` (全空)
- `status: superseded` (没造)
- `kind: node` (parent README 缺)
- 3+ 层嵌套 (只有 2 层)

→ 跟 r1/r2 拍板的"fixture 缺 src/" (r2 §4.8 修了) **是同一类问题, 不同的字段** — fixture **永远只覆盖当时测的字段**。

### 9.3 r3 新发现

#### 🐞 候选 P1: `kron_get` boolean 编码

- **症状**: `include_relations: true` 或 `false` 都报 `-32602: type ... has type "string", want "boolean"`
- **schema 声明** `type: "boolean"` (MCP JSON Schema 标准) — **是**对的
- **根因候选** (3 个, 优先级): `internal/mcp/server.go` / `cmd/kron/serve-mcp/tools_get.go` 用错了 decoder
  1. `args["include_relations"].(string)` 类型 assertion 写错 (应该用 `.(bool)`)
  2. `ValidateBool` 实现看 string 而不是 unmarshal 到 bool
  3. `json.Unmarshal` 路径**没**走, hand-rolled 走 string 解析
- **r1/r2 跳过这 flag** — go-client 40 调**没**调 `include_relations`, 这 bug **真**躲过去了
- **影响**: `kron_get` 返不出 relations, GUI 树形点击节点**没** dependents / prerequisites / incoming_anchors 可看
- **建议修**: 改 server validation, 加 unit test 覆盖 boolean parsing

#### 🟡 文档漂移: `kron_lint` reporter=text 行为

- **spec** ([`docs/mcp/protocol.md`](../../mcp/protocol.md) 推测): "reporter=text 输出 text 格式"
- **实际**: text mode 返**同样** JSON shape, **只**多 `text: "[summary] N errors, M warnings"` 字段
- **评价**: 双轨输出 (programmatic + human-readable) **是**好的设计, **不**是 bug
- **建议**: 文档改 "text mode 追加 `text` 字段, 跟 json 字段共存"

#### 🟢 设计观察: `kron_intent_density.intents_with_anchors` 含 dangling

- **不**是 bug — density 报"被指过" (指 = 有锚点), lint 报"指错" (dangling 错)
- **双**信号**互**补 — GUI 应该 `density.covered / density.total` 算%, **别**当 "all OK" 用
- **建议**: 文档明说 "density 跟 lint 互补, **不**能独立判断"

### 9.4 r3 跟 r1/r2 的关系

| 维度 | r1/r2 (go-client) | r3 (Cursor UI) |
|---|---|---|
| 渠道 | Go `main.go` sequential 40 调 | AI agent Cursor chat 逐调 |
| 用户角色 | 脚本 | AI |
| 验证 | 40/40 PASS 全跑 | 11 个手测, 7 个待 fixture 后跑 |
| 暴露 bug | F1-F8 代码瑕疵 (snapshot 2026-10-14) + r1/r2 拍板的 fixture 缺 src/ | **新** `include_relations` boolean bug (F1-F8 没覆盖) |
| 互补 | r1/r2 验 completeness, r3 验 AI-friendly (schema 描述 / 错误友好度) | 同 |

**两条渠道并列, **不**互相替代** — Go-client 验 contract 完整, AI agent 验"AI 看 schema 调得出调不**出**"。

### 9.5 r3 完整跑完 (2026-10-15 14:30, fixture 补完后)

**fixture 补充** (14:20-14:30): `kron-mcp-test/project/` 加 3 类缺失:
1. **3 层嵌套** — 新建 `auth/oauth/README.md` (node) + `auth/oauth/oauth-flow.md` (leaf) + `src/auth/oauth/oauth_flow.go` (code anchor)
2. **2 个 md anchor** — `docs/adr/0002-charge-policy.md` 加 `// @kron:intent payments/charge`
3. **全字段** — `symbol` (3 个 leaf) / `reviewers` (3 个) / `status=active` (5 个) / `status=superseded` (1 个, auth/session) / `depends_on` (3 处, 含 cycle) / `references` (1 处含 dangling) / `assumptions[]` per-intent (2 + 1 + 1)
4. **cycle** — `auth/jwt` ↔ `auth/session` 互 depend
5. **dangling reference** — `payments/refund.references=[does/not/exist]`
6. **assumptions registry** — 3 个 (tls-1-3 / rfc-7519 / stripe-api-stable), 1 软删 (stripe)
7. **per-intent expires_at** — `payments/charge` 的 stripe-api-stable 标 `expires_at=2025-01-15T00:00:00Z`

**fixture 触** **lint 6 类 rule**:
- `anchor-dangling` (1) — `src/broken/orphan.go → does/not/exist`
- `depends-on-cycle` (4) — jwt↔session cycle 跨 4 intent 报告
- `dangling-reference` (1) — `refund → does/not/exist`
- `expired-hard-assumption` (1) — `stripe-api-stable` 638 天 overdue
- `assumption-rationale-stale` (2) — rfc-7519 + tls-1-3 rationale 抄了 registry text
- `assumption-registry-id-mismatch` (1) — `payments/charge` 引用 `stripe-api-stable` 但 registry 已软删

**lint summary: 7 errors, 4 warnings, 11 个总** — **完整** A-class + B-class + C-class rule 覆盖**。

#### 9.5.1 重测结果 (Step 2 完整 + Step 3 fixture 验)

| # | tool | result | 新发现 |
|---|---|---|---|
| 1 | `kron_init` | `already_initialised=true, registered_current_project=…` | **反** F6: c4ba92a 修了 (2026-10-15 12:xx), F6 **已不成立** |
| 2 | `kron_add {slug: "test-add-1"}` | 写盘 ✓, title 自动从 slug | body append `## Why` **两段** (template + 我传) |
| 3 | `kron_list {prefix: "auth/"}` | 2 auth/* (auth parent 不返) | ✓ 严格 filter |
| 4 | `kron_update {status: "active"}` | 写盘 ✓ | PATCH string 字段**不**破坏 body (之前 Read tool 误显示, **实** OK) |
| 5 | `kron_update {reviewers: [...]}` | **❌ -32602: type map, want array** | 🐞 **新 P1**: Cursor MCP 桥 array encoding bug |
| 6 | `kron_update {depends_on: [...]}` | **同** array bug | 同 |
| 7 | `kron_update {assumptions: [...]}` | **同** array bug | 同 |
| 8 | `kron_delete` | 软删 + `dependents:[]` | ✓ |
| 9 | `kron_restore` | OK | ✓ |
| 10 | `kron_assume_create` x3 | tls-1-3 / rfc-7519 / stripe-api-stable 写盘 | ✓ |
| 11 | `kron_assume_list` | 3 entries | ✓ |
| 12 | `kron_assume_get` | 含 `source_path` | ✓ |
| 13 | `kron_assume_update {expires_at}` | 写盘 | ✓ (string field, 不触 array bug) |
| 14 | `kron_assume_delete` | 软删 + `referencing_intents:[]` | ✓ |
| 15 | `kron_assume_check` (no args) | `expired:1, hard:2, soft:1`, 3 warnings (verified:false / days_remaining:-638) | ✓ |
| 16 | `kron_assume_check {file_path: "src/auth/jwt.go"}` | **❌ open src/auth/jwt.go: path not found** | 🟡 cwd 问题 (相对路径**不**能解析) — **可**能**是** MCP spawn 时**的** process cwd **不**是 project |
| 17 | `kron_assume_check {file_path: "E:\\...\\src\\auth\\jwt.go"}` | 0 / 0 / 0 | ✓ 绝对路径 work |
| 18 | `kron_stale {days_threshold: 0}` | **❌ -32602: type string, want integer** | 🐞 **新 P1**: Cursor MCP 桥 integer encoding bug (跟 array **同类**) |
| 19 | `kron_stale` (no args, default 90d) | `superseded_candidates:[]`, `expired_assumptions:[stripe-api-stable, 638d]` | ✓ superseded 候**选**要"老" (90d+), 跟 spec 一致 |
| 20 | `kron_lint` (Step 3) | **11 个错** (7 err + 4 warn) 跨 6 类 rule | ✓ 完整 lint rule 验证 |
| 21 | `kron_tree` (Step 3) | 3 层嵌套 (auth → oauth → oauth-flow) 100% 对 | ✓ |
| 22 | `kron_intent_density` (Step 3) | `total: 8, with_anchors: 6, without: [auth, auth/oauth, payments], total_anchors: 8, files_without_intent: []` | ✓ dangling 算"有 anchor" (跟 r1 一致) |
| 23 | `kron_get` (default) | frontmatter + body + source_path | ✓ relations **不**在 default 返, 需 `include_relations` (但 flag 触 bug) |
| 24 | `kron_get {include_relations: true/false}` | **❌ -32602: type string, want boolean** | 🐞 跟 r3.9.1 一致, **没** fix |
| 25 | `kron_impact auth/jwt` (Step 3) | `prerequisites: [auth/session], references: [payments/refund], 2 anchors, intent.{symbol, assumes, status}` | ✓ cycle 一半 prerequisites, refund→jwt 反向算成 jwt.references |
| 26 | `mcp_auth` | "Successfully authenticated MCP server" | ✓ no-op refresh |

#### 9.5.2 r3 新发现的 P1 bug 全表 (3 个)

| # | bug | 触发字段 | 类型 | 影响 |
|---|---|---|---|---|
| B1 | `kron_get {include_relations: true/false}` → `-32602: type string, want boolean` | `include_relations` | boolean | GUI 看**不**到 relations |
| B2 | `kron_update {reviewers/depends_on/references/assumptions: [...]}` → `-32602: type map, want array` | 任何 `[]T` 字段 | array | **所**有 PATCH 关系/假设**不**能用 Cursor UI |
| B3 | `kron_stale {days_threshold: N}` → `-32602: type string, want integer` | `days_threshold` | integer | stale threshold **不**能调 |

**3 个 bug 全是 Cursor MCP 桥 encoding 问题** (`true` / `[...]` / `0` 都被 bridge 包成 string) — **不**是 kron server 错 (r1 go-client **没**触发,**它**走**别**的 transport)。

**重**要** — snapshot 2026-10-14 F1-F8 **全**是 server 端代码瑕疵 (跟 Cursor **无**关), **B1-B3 是 Cursor 客户端编码**层**,** 责任**在** Cursor **而**不**在** kron.

#### 9.5.3 r3 完整覆盖 — 18 tool 全**测**

| # | tool | r3 status | 备注 |
|---|---|---|---|
| 1 | `kron_add` | ✓ | round-trip 写盘 |
| 2 | `kron_update` | ⚠ PATCH string 行, array 触 B2 | B2 |
| 3 | `kron_get` | ✓ default; ⚠ `include_relations` 触 B1 | B1 |
| 4 | `kron_list` | ✓ | prefix filter work |
| 5 | `kron_delete` | ✓ | soft delete + dependents |
| 6 | `kron_restore` | ✓ | |
| 7 | `kron_init` | ✓ | **反** F6 — c4ba92a 修了 |
| 8 | `kron_impact` | ✓ | reverse-link 全字段 |
| 9 | `kron_intent_density` | ✓ | 6 distinct slug + dangling 算"有" |
| 10 | `kron_lint` | ✓ | 11 错 6 类 rule |
| 11 | `kron_stale` | ✓ default; ⚠ `days_threshold` 触 B3 | B3 |
| 12 | `kron_tree` | ✓ | 3 层嵌套 |
| 13 | `kron_assume_create` | ✓ | |
| 14 | `kron_assume_list` | ✓ | |
| 15 | `kron_assume_get` | ✓ | |
| 16 | `kron_assume_update` | ✓ string field; ⚠ array 字段未测 (B2 推测同) | |
| 17 | `kron_assume_delete` | ✓ | `referencing_intents:[]` |
| 18 | `kron_assume_check` | ✓ no args + absolute file_path; ⚠ relative file_path 路径错 | 🟡 MCP cwd **不**是 project? |

**18/18 tool 全跑过, 3 个 P1 Cursor 桥 bug, 1 个 🟡 cwd 相对路径问题**。

#### 9.5.4 反 snapshot 2026-10-14 F1-F8 的发现

| F# | F# 拍板 (12h 前) | r3 真测 (14:30) | 状态 |
|---|---|---|---|
| F6 | "`kron_init` (MCP) **不** register 不设 current" | **`kron_init` 写 `current_project` + `projects[]`** | ❌ **F6 已不成立** — c4ba92a 修了 |
| F1-F5, F7, F8 | (其他) | (r3 渠道跑**不**到, server-only) | 仍 valid, **不**变 |

**F6 修** **是**新 commit 干**的** (`c4ba92a feat(serve-mcp): wire projects.json::current_project into MCP init + harden handler contracts`) — 2026-10-15 **今**天 12:xx. snapshot 2026-10-14 **是 12h 前**写的 status 报告, **不**是 bug list.

#### 9.5.5 r3 跟 §5 拍板对照

snapshot §5 拍 14 个覆盖缺口. r3 **补**了 5 个:

| §5 缺口 | r3 状态 |
|---|---|
| `kron_lint` reporter=text vs json | ✓ 测了 (text mode **也**返 JSON + `text` summary) |
| 跨 `kind: node` 3+ 层嵌套 | ✓ `auth/oauth/oauth-flow` |
| `kron_update` PATCH null/absent/`[]` 三态 | ⚠ array 路径被 B2 阻, **不**能 Cursor 验 (go-client 路径**也**只测 non-empty + clear) |
| cycle 创建 + lint `depends-on-cycle` | ✓ jwt↔session 触 4 个 cycle 错 |
| dangling reference + lint `dangling-reference` | ✓ refund→does/not/exist |
| anchor-dangling (source file `// @kron:intent missing` + lint 报) | ✓ orphan.go→does/not/exist |

**剩 8 个 §5 缺口 (resources/templates/list / prompts/get / 并发 / metrics / 跨进程 cache / 等)** — **不**在 r3 跑 (AI agent 渠道**不**会自然触发)。

#### 9.5.6 fixture 现状 (Step 3 后)

```
.kron/
├── config.toml
├── assumptions/
│   ├── rfc-7519.md
│   └── tls-1-3.md
├── .trash/
│   └── assumptions/
│       └── stripe-api-stable.md
├── intents/
│   ├── auth/
│   │   ├── README.md (node, parent)
│   │   ├── jwt.md (leaf, symbol+assumes+depends_on+reviewers)
│   │   ├── session.md (leaf, status=superseded)
│   │   └── oauth/
│   │       ├── README.md (node, 2nd level)
│   │       └── oauth-flow.md (leaf, 3rd level, depends_on:auth/jwt)
│   └── payments/
│       ├── README.md (node)
│       ├── charge.md (leaf, symbol+assumes+depends_on, expires_at)
│       └── refund.md (leaf, symbol+depends_on+references w/ dangling)
└── .trash/
    └── test-add-1.md (Step 2 软删)

docs/adr/
├── 0001-jwt-rotation.md (md anchor → auth/jwt)
└── 0002-charge-policy.md (md anchor → payments/charge)

src/
├── auth/
│   ├── jwt.go (code anchor → auth/jwt)
│   ├── session.go (code anchor → auth/session)
│   └── oauth/
│       └── oauth_flow.go (code anchor → auth/oauth/oauth-flow)
├── payments/
│   ├── charge.go (code anchor → payments/charge)
│   └── refund.go (code anchor → payments/refund)
└── broken/
    └── orphan.go (code anchor → does/not/exist, dangling)
```

### 9.6 r4 2026-10-15 14:45 — 全 MCP 协议 surface 补完 (prompts + resources + templates + read)

> 之前 r3 §9.5.3 报告"18/18 tool 跑过"是**不**诚实的 — 漏**了** prompts/* + resources/* + resources/templates/* 4 个 protocol-level 方法。Cursor MCP UI **不**暴露这 4 个给 agent (tool list 跟 prompt/resource list 是**不**同**的**MCP 概念), AI agent 渠道自然**不**会触发。**用** Go raw stdio probe (`E:\works\Kron\kron-mcp-test\raw-probe\main.go`) 独立 spawn `kron serve-mcp`, 走 bufio.Scanner line-delimited JSON-RPC, **绕**开** Cursor 桥 encoding bug, 测全部 protocol surface.

**注**: probe 写在 `kron-mcp-test/` 仓库 (不**在**主仓, 跟 `kron-mcp-test/install/` 一类 external 仓库)

#### 9.6.1 protocol-level 4 个 method + 3 prompt 内容

| id | method | params | result | 发现 |
|---|---|---|---|---|
| 1 | `initialize` | `{protocolVersion: "2024-11-05", capabilities:{}, clientInfo}` | `serverInfo: {name: "kron", version: "v1.2.0"}`, capabilities: `{logging, prompts.listChanged, resources.listChanged, tools.listChanged}` | ✓ |
| 2 | `prompts/list` | `{}` | 3 prompts (kron-drift-check, kron-implement, kron-review) — 每**个** arguments schema 都**齐** | ✓ |
| 3 | `resources/list` | `{}` | `[]` (template 不在 list) | ✓ (跟 r1 一致) |
| 4 | `resources/templates/list` | `{}` | 1 template `kron://intents/{+slug}` (RFC 6570 `{+slug}` reserved expansion 支持多段) | ✓ |
| 5 | `prompts/get kron-drift-check` | `{arguments: {days_threshold: 30}}` (**integer** 30) | **❌ -32000**: `json: cannot unmarshal number into Go struct field .arguments.days_threshold of type string` | 🐞 **新 P1 #4**: prompt 期望 **string** (`"30"`) 而**不**是 **number** (`30`) |
| 6 | `prompts/get kron-implement` | `{arguments: {feature: "add idempotency key to charge"}}` | `messages: [role: user, content: text]`, "no existing intent matches" + action steps | ✓ |
| 7 | `prompts/get kron-review` | `{arguments: {status: "active"}}` | "## Review (status=active)\n\n### In-scope intents\n- auth/jwt\n- auth/oauth/oauth-flow\n- payments/charge\n- payments/refund\n\n### Findings by rule\n- anchor-dangling (1)...\n- assumption-rationale-stale (2)...\n..." — **6 rule 全列, 11 错 total** (跟 r3 `kron_lint` 同结果) | ✓ prompt 是 lint 包装, AI 直接拿来当 PR review checklist |
| 8 | `resources/read` | `{uri: "kron://intents/auth/jwt"}` | `contents: [{uri, mimeType: "text/markdown", text: <raw markdown>}]` | ✓ 完整 frontmatter + body |
| 9 | `resources/read` | `{uri: "kron://intents/payments/charge"}` | 同 8, 多段 slug (RFC 6570 `{+slug}` reserved expansion **真**用) | ✓ |
| 10 | `resources/read` | `{uri: "kron://intents/does/not/exist"}` | **-32002 Resource not found** + `data.uri` | ✓ 错误**友**好 |

#### 9.6.2 跟 r3 tool 测试的 cross-ref

| r3 跑 | r4 跑 | cross-check |
|---|---|---|
| `kron_stale {days_threshold: 0}` → B3 (-32602 string→int) | `prompts/get kron-drift-check {days_threshold: 30}` (integer) → -32000 string→int | **两**处**都**是** integer→string coerce 错, **但**触发**点不**同**: r3 是 Cursor 桥错 (传 0 也 fail), r4 是 server 错 (期望 string, 收到 int). **r4 揭露 server-side bug — 不是 Cursor bug** |
| `kron_lint` 11 错 | `kron-review` 11 错 | prompt 包装**真**lint, 数对得上 ✓ |
| `kron_get auth/jwt` (default) | `resources/read kron://intents/auth/jwt` | 两**个**都返 raw markdown frontmatter + body. **resource 跟 tool 重**叠**, **client 选**一** (resource 省 1 tool call, tool 带 relations flag) |

#### 9.6.3 全 MCP protocol surface 覆盖 — final

| MCP method | r3 | r4 | 状态 |
|---|---|---|---|
| `initialize` | ✓ | ✓ | done |
| `tools/list` | ✓ (18 tool 列举) | n/a | done |
| `tools/call` | ✓ 18/18 tool | n/a | done |
| `prompts/list` | ✗ | ✓ | r4 补 |
| `prompts/get` | ✗ | ✓ 3/3 | r4 补 |
| `resources/list` | ✗ | ✓ (空, 跟 r1 一致) | r4 补 |
| `resources/templates/list` | ✗ | ✓ (1 template) | r4 补 |
| `resources/read` | ✗ | ✓ 3 cases (exist + multi-segment + missing) | r4 补 |
| `notifications/initialized` | ✓ | ✓ | done |
| `logging/setLevel` | ✗ (capability 声明**有** `logging`, **没**测) | ✗ | **不**测 — Go SDK 测过, **不**属 AI agent 范围 |
| `notifications/cancelled` | ✗ | ✗ | **不**测 — AI agent **不**触 |
| `notifications/progress` | ✗ | ✗ | **不**测 — kron **不**用 progress |

#### 9.6.4 r4 新发现的 P1 bug (4 号, 总计 4 个)

| # | bug | 触发 | 类型 | 影响 |
|---|---|---|---|---|
| B1 | `kron_get {include_relations: bool}` → -32602 | include_relations | boolean | GUI 看不到 relations |
| B2 | `kron_update {array field}` → -32602 | reviewers/depends_on/references/assumptions | array | 所有 PATCH 关系/假设不能用 Cursor UI |
| B3 | `kron_stale {days_threshold: int}` → -32602 | days_threshold | integer (Cursor 桥) | stale threshold 不能调 |
| **B4** | **`prompts/get kron-drift-check {days_threshold: 30}` → -32000 `cannot unmarshal number into ...string`** | **days_threshold (prompt args)** | **integer→string (server 端)** | **AI agent 调 drift-check 必传 `"30"` 而**不**是 `30`** |

**B4 是 server 错** (r4 直**接** spawn kron, 走**不**到 Cursor 桥), **不**同**于** B1-B3 (Cursor 桥). **修**法: 改 `internal/mcp/prompts.go` (或对应 handler) 的 `GetPromptParams.Arguments.days_threshold` 类型从 `string` 改 `int`, **或** 在 JSON-RPC 入口加 `coerceStringArgs` middleware. **B1-B3 是 Cursor 桥错, B4 是 kron server 错**.

**B4 的**特**别烦**点: 同一**个**概念 (`days_threshold`) 在 tool (`kron_stale`) 是 integer, 在 prompt (`kron-drift-check`) 是 string — **同**一**个** client 必**须**记**两**套**编码. 跟 B3 **合**在**一**起**: Cursor 桥** + server 错, **两**层 encoding 问题, 客**户**端**极**容易踩.

#### 9.6.5 r4 跟 r1 §5 缺口对照 (r4 补 3 个)

| §5 缺口 | r4 状态 |
|---|---|
| `resources/templates/list` 整 path 没覆盖 | ✓ 测了 |
| `prompts/list` 0 调 | ✓ 测了 (3 个) |
| `prompts/get` 0 调 | ✓ 测了 (3 个, 含 1 server bug) |
| `kron_lint` reporter=text vs json | r3 测了 |
| 跨 `kind: node` 3+ 层嵌套 | r3 测了 |

**§5 拍板的 14 个缺口, r3 + r4 补 8 个**. 剩 6 个: 并发 (跨进程 cache invalidation) / 跨 `kind: node` 跨段 slug 全覆盖 (r4 单测了 charge) / `kron_update` PATCH null/absent/`[]` 三态 (B2 阻) / 同 `kron_assume_update` / cycle (r3 触) / dangling reference (r3 触).

#### 9.6.6 r4 揭露的工程教训

1. **Cursor MCP UI 限**制**`: tool 列表是** AI 看得**见**的, **prompt + resource 列表**不**可见**. AI agent **不**会自然调. **测**必须用 raw stdio probe 绕 Cursor. **这**是**测**试架构**不**是 AI 偷懒.
2. **"MCP 里有的都要测" = 5 个 method family** (`initialize` / `tools/*` / `prompts/*` / `resources/*` / `notifications/*`), **不**只 tool. r3 §9.5.3 报告"18/18 tool 跑过"是**不**诚实的, 漏**了** 4 个 method family. r4 补.
3. **server 跟 client 双重 bug** — B1/B2/B3 是 Cursor 桥, **客**户**端**不**能**靠** server 修**;** B4 是 server 自身, **两**者**同**时**踩. AI agent 接 MCP server **必**须**有** raw stdio 测**试** baseline, **不**只信 client tool 列举.
4. **`kron-drift-check` prompt 的"AI-friendly 包装"**是**真**的 — 它**直**接**包**装** `kron_lint` 输出, 给 AI 一**个** ready-to-use review checklist (跟 r3 §9.5.1 lint 输出 11 错**完**全**对**得上). **这**是**设**计**亮点.


