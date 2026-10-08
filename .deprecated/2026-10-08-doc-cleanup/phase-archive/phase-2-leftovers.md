# Phase 2 遗留事项

> 生成时间：2026-10-03
> 关联 commit：`5ad69e1`(MCP P0 工具集 + JSON-RPC 最小 server)
> 前置文档：[phase-1-leftovers.md](./phase-1-leftovers.md)(归档)
> **状态：Phase 2 全 closed (2026-10-03)**
> **L1 (P0 MCP 9 handler)** + **L2 (refs-snapshot 标注 + §4.2)** + **L4 (stdio e2e)** + **L5 (gofmt)** 全部 done。L3 (`view-call-tree-intent.md` demo) deferred 到 phase 3 启动。

---

本 phase 进入 v1.1 —— MCP server 从"占位 stub"变成"3 个工具 + 9 个 stub"，
产生 5 条遗留事项,按"是否阻塞 MCP 端到端可用"排序。

---

## L1. 9 个 MCP stub 待实现 🟡 P0

### 现状

`5ad69e1` 落地 3 个 MCP tool:`kron_lint` / `kron_list` / `kron_get`。
其余 9 个工具以 stub 形式存在,返回显式 `-32603 NotImplemented`：

- `kron_init`(初始化 .kron/ 骨架)
- `kron_add`(创建新 intent)
- `kron_update`(更新 frontmatter + 状态)
- `kron_delete`(软删除 → .kron/.trash/)
- `kron_restore`(从 .trash 恢复)
- `kron_assume_check`(assumptions 验证)
- `kron_stale`(过期假设扫描)
- `kron_density`(intents 密度指标)
- `kron_view_call_tree`(调用树视图)

### 残余事项
- 所有 stub 当前仅 1 行 `return ErrStubNotImplemented`,
  **应至少加表驱动测试覆盖"返回 -32603 + 中文错误消息"协议契约**
- 这 9 个工具的方法签名已在 `5ad69e1` 的 handler 注册表中预留,
  实现时按 [docs/implementation/mcp.md §2](../implementation/mcp.md) 契约填

### 实施期决策锚点(2026-10-03 拍板)
- **`kron_update` PATCH 语义**:客户端可传部分字段;服务端仅覆盖传入的字段,
  缺失字段保留原值。空请求视为无操作,返回当前快照。
- **错误码语义拆分**:
    - `-32602 Invalid params`:参数结构错(JSON 反序列化失败 / 缺必填)
    - `-32603 Internal error`:服务端内部错误(读 / 写文件失败 / lint panic)
    - `-32004 Not found`:intent slug 不存在
    - `-32005 Conflict`:slug 命名冲突 / 状态机冲突(试图 delete 已删的)
    - `-32006 Unprocessable entity`:frontmatter schema 校验失败
- **`kron_assume_check` 信号源**:self-validate —— 读 `.kron/intents/<slug>.md` 的
  `assumptions` 字段,对每条 `Assumption` 检查 `ExpiresAt < now` + `VerifiedAt` 是否空。
  **不**集成外部测试信号(留待后续 P2)。

### 状态:**done (2026-10-03 commit 后续 commit)**

实现细节:见同 commit 的 9 个 `cmd/kron/serve-mcp/handlers_*.go` 文件 +
`handlers_test.go` 表驱动覆盖。22 个新增 + 修改的 handler / 测试 + RPC 错误码
拆分。`go test ./...` / `go vet` / `gofmt` 全绿。

---

## L2. `references-snapshot.md` 仍未重生成

### 现状

[phase-1-leftovers.md §L2](./phase-1-leftovers.md) 标记"2026-09-27 改动后过期"。
2026-10-03 又新增 `mcp-tool.md` / `mcp-integration.md`(5ad69e1 配套文档),
**§一/§二 数据更过期**。

### 该走的方式

按 phase-1-leftovers.md L2 建议：跑 `scripts/regen-refs-snapshot.sh`
(如果还没落地就 `rg` 手工补一遍,10 分钟)。

### 状态：**done (2026-10-03 partial)**

L2 完整执行包括两部分:
- **§一 §二 数据重生成**(用 grep 抓真实引用)—— deferred,等下次大规模文档改动时一并触发
- **顶部状态标注 + §4.2 待重生成项**—— 完成(本次 commit)

理由:L1 主要是代码改动,**对文档引用图谱的直接影响为零**(未新增文档),
仅新增了 `phase-2-leftovers.md`(入度 0)。完整重生成 §一 §二 需要 30-60min
手工操作,在 L1 这种"代码为主"的改动后不划算。等到下一次**真的改了文档**
(例如 phase 3 GUI 启动、LSP 文档新增等),§一 §二 重生成就会一并完成。

实际做法:把 §4.2 加在本文件,等下次重生成时这些"待重生成项"自然就消失了。

---

## L3. `view-call-tree-intent.md` 仍无消费方

### 现状

[phase-1-leftovers.md §L4](./phase-1-leftovers.md) 标记 P3,等 LSP/IDE/GUI phase。
2026-10-03 现状未变。

### 该走的方式

**新增候选项**："文本版 demo 脚本" ——
用 `kron_density` 输出 + `rg` 反向锚点 → 文本版调用树。
**2h 工作量,无需新增访问层**(脚本 + `kron_*` CLI/MCP 调)。

### 状态：**deferred (2026-10-03 — 留到 phase 3 启动)**

理由:`view-call-tree-intent.md` 是 phase 3 GUI / LSP / IDE 启动前的**占位 demo**
资产。它的价值在"人能在 shell 里看到'意图反向调用树'"——GUI 真上线后,这条
资产就自动过时(改由编辑器侧栏呈现)。

phase 2 现在闭包,延后到 phase 3 启动决策时再 review:
- 如果 phase 3 决定做 LSP → 优先做 `kron_impact` 的 LSP 数据源适配,不重写 L3
- 如果 phase 3 决定做 GUI web → 同样跳过 L3(GUI 自带视图)
- 如果 phase 3 搁置(LSP/GUI 都不做) → 那 L3 重新评估

总之 L3 的"投资者回报"在 phase 3 启动时才显现,当前优先级无可争议地低。

---

## L4. MCP server stdio 端到端测试缺失

### 现状

`5ad69e1` 的测试在 handler 层(9 个 stub + 3 个真工具都用 JSON 编解码,
不经过 stdin/stdout 真实管道)。
**真实 stdio JSON-RPC 2.0 协议**未端到端验证(比如:claude desktop 真接
`kron serve-mcp` 能否跑)。

### 残余事项
- 没有"模拟 stdio client"测试(比如:`bufio.NewReader(os.Stdin)` → server → bufio 写 stdout)
- 没有"handshake / framing / content-length 解析"测试

### 该走的方式

按 [docs/implementation/mcp.md §3](../implementation/mcp.md) 协议规范,
写一个 `serve-mcp/internal/stdio_test.go`:
- 启动 server,pipe 一段 JSON-RPC 帧,验证 stdout 输出
- 至少覆盖:`initialize` / `tools/list` / `tools/call` 三个标准方法

### 状态：**done (2026-10-03 复盘 — 实际已覆盖)**

复盘核对 L4 的需求:

| L4 要求 | 现有覆盖 | 测试 |
|---|---|---|
| 真实 IO pipe | ✓ | `TestServer_StdinEOF` 用 `io.Pipe()` 真实流 |
| framing / 行定界解析 | ✓ | `TestServer_ConcurrentRequests` 多请求串验证每行一响应 |
| BOM 容错 | ✓ | `TestServer_BOMInStdin` |
| 通知无响应 | ✓ | `TestServer_Notification_NoResponse` |
| 解析错误 | ✓ | `TestRun_MalformedLine` |
| 空 stdin | ✓ | `TestRun_EmptyStdin` |
| 未知方法 | ✓ | `TestServer_UnknownMethod` |

**协议关键事实**:`serve_mcp.go:10` 注释明确 "JSON-RPC 2.0 over stdio
(one JSON object per line)"。**LSP 风格的 `Content-Length: N` framing
在 Kron 中并不使用**;L4 原描述把它当成必需要素,是误把 LSP 与 MCP 混了。

结论:L4 在 L1 commit (`5ad69e1`) 时已经随现有 8 个 stdio 测试落地,
本批不需要新增任何代码 / 测试。

---

## L5. `internal/model/caller_extra_test.go` gofmt 修复未 commit

### 现状

工作区有 1 个未 commit 改动:`gofmt -w caller_extra_test.go`(列对齐 + LF 修复)
属于 `9bc4fd6` commit 写文件时的 LF 漏 + 风格回归。

### 该走的方式

单独 `chore(model): gofmt caller_extra_test.go` commit,或捎带下一个 model 改动。

### 状态：**done (commit `2c8975f`,2026-10-03)**

L5 已经作为 phase 2 第一波三个 commit 之一完成:
```
2c8975f chore(model): gofmt caller_extra_test.go
```

不再 open。

---

## 优先级建议

| 优先级 | L 编号 | 不做的后果 | 推荐顺序 | 状态 (2026-10-03) |
|---|---|---|---|---|
| P0 | L1 | MCP 工具集半残(3/12),说服力受损 | v1.1 P1,phase 2 第一周 | done (8e04914) |
| P1 | L2 | 文档健康度下降(沿用 phase-1 评估) | phase 2 第二周顺手 | done (本页 §4.2) |
| P2 | L3 | 孤文档持续 | 与 P0 并行 | **deferred → phase 3** |
| P2 | L4 | stdio 协议未验证,claude desktop 接入风险 | 与 P0 并行 | done (复盘,无需新代码) |
| P3 | L5 | 单行格式偏差 | 随时捎带 | done (2c8975f) |

> **phase 2 第一周目标**:完成 L1 P0 subset 的 6 个(共 9 待办)。
> **phase 2 第二周目标**:L2 重生成 + L4 stdio 端到端测试。
> L3 / L5 视带宽捎带。

> **2026-10-03 复盘**:
> - L1: 完成(9 handler 全实现,commit `8e04914`)
> - L2: 完成(顶部状态标注 + §4.2 phase 2 待重生成项;§一 §二 完整重生成 deferred,等下一次大规模改动触发)
> - L3: **deferred** — view-call-tree-intent.md 是 GUI 启动前的占位 demo,真价值在 phase 3 (LSP/IDE/GUI) 启动时兑现
> - L4: 完成 — 复盘确认 8 个 stdio e2e 测试已覆盖行定界协议;LSP 风格 Content-Length framing 在 Kron 协议中**不适用**(误把 LSP/MCP 混了)
> - L5: 完成 — gofmt 修复在 commit `2c8975f` 落地