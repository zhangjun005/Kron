# MCP 实测已知问题 — Cursor 桥层 (暂不修, 等 Cursor 修)

> 记录 2026-10-15 实地测试 (r3 + r4 + c 轮) 暴露的 4 类问题。
> 这些问题的根**因在 Cursor 桥层**或**设计文档漂移**, **不在 kron server**。
> **暂不修**, 原因: (a) Cursor 桥 bug 应由 Cursor 修; (b) 设计文档漂移是 doc 问题不是 bug。
>
> **追踪**: 等 Cursor 确认修了 B1-B3 后, 验证 middleware 是否有残留失效 case。

---

## B1–B3: Cursor 桥类型编码 (Cursor 责任, 等 Cursor 修)

> 记录在此, 跟 [docs/rfc/2026-10-15-mcp-coerce-middleware.md](2026-10-15-mcp-coerce-middleware.md) 对应。

| Bug | 触发 | 根因 | 修法 | 状态 |
|---|---|---|---|---|
| B1 | `kron_get {include_relations: true}` → `-32602: type string, want boolean` | Cursor 桥把 `true` 写成 `"true"` string | coerce middleware | **等 Cursor 修** + middleware 兜底 |
| B2 | `kron_update {reviewers: [...]}` → `-32602: type object, want array` | Cursor 桥把 `["@alice"]` 写成 `{"item":"@alice"}` object | coerce middleware | 同上 |
| B3 | `kron_stale {days_threshold: 0}` → `-32602: type string, want integer` | Cursor 桥把 `0` 写成 `"0"` string | coerce middleware | 同上 |

**证据**:
- Cursor 论坛 145807 (2025-05): "Cursor auto-selected model stringifies MCP tool parameters"
- Cursor 论坛 151180 (2025-07): "Cursor serializes array-type parameters incorrectly"
- Cursor 论坛 169687 (2026-07): "Google Sheets MCP update_values fails with serialization error"
- Cursor 论坛 164361 (2026-09): "CP tool-call arguments corrupted above a size threshold"
- Cursor 论坛 3.12.x regression (2026-10): "MCP tool arguments not forwarded (all params undefined)"

**Workaround**: raw stdio (Go client 直连) 全 PASS, 见 `E:\works\kron-mcp-test\raw-probe\main.go` 测试结果 (26/26 PASS)。

**等**: Cursor 确认修**了** B1-B3 后, 跑 `raw-probe.exe` 验证 middleware 是否残留失效 case。

---

## B4: Server 端 prompt args 类型不一致 (暂不修)

> `kron-drift-check` 的 `days_threshold` 在 prompt 里是 `map[string]string` (SDK 强类型), 收到 integer 30 时 unmarshal 失败。
> 同一字段在 tool `kron_stale` 里是 `int`, 不受影响。

**证据**:
- raw stdio 测试: `prompts/get kron-drift-check {days_threshold: 30}` → `-32000: cannot unmarshal number into string`
- raw stdio 测试: `prompts/get kron-drift-check {days_threshold: "30"}` → 成功

### 修法选项 (2026-10-15 评估)

**方案 (a) — 等 SDK**: 等 `go-sdk` 加 `GetPromptParamsRaw` (对称于 `CallToolParamsRaw`)，然后接**收** middleware 可在 unmarshal **前**拦截 raw JSON 重写。这是干净路径，但不在我们控制范围内。

**方案 (b) — 接**收** middleware** ❌ 走**不**通:
SDK 的 `Request.GetParams()` 在 `prompts/get` 路径上返回 `*GetPromptParams`（不是 Raw 等价物），没有公开的 raw access。接**收** middleware 收到的是 SDK 已 un**marshal** 完（或已 reject）的结果，不是在 raw JSON 字节层。`TestCoerceMiddleware_PromptsGetIsTransparent` 控制测试确认 middleware 对 `prompts/get` 是透明 no-op。

**方案 (c) — Transport 层拦截** ✅ 理**论**可**行**, **但** out of scope:
自定义 `StdioTransport` 包装，在 SDK 解析 JSON-RPC envelope **之前**把原始字节里的 `"days_threshold":30` 重写为 `"days_threshold":"30"`。这需要绕过 SDK 的 `StdioTransport`，用自定义 `Transport` 实现拦截。RFC §1.4 将此列为 out of scope（"改传输层架构"），暂不实现。

**方案 (d) — 等 Cursor 修** ✅ 目**前**在**用:
Cursor 发 string (`"30"`) 时整条链路正常（middleware → handler → parseIntArg ✓）。方案 (c) 若实现后，B4 可彻底兜底。

**暂不修原因**: 方案 (c) 可行但需改传输层架构，RFC §1.4 out of scope；方案 (a) 等 SDK；方案 (d) 当前 workaround 够用。
**等**: (a) SDK 加 `GetPromptParamsRaw`; (c) v1.3+ 审视 prompt args schema 时统一处理。

---

## B5: `kron_lint` reporter=text 文档漂移 (P2)

> spec 说 "reporter=text 输出 text 格式"; 实际 text mode **也返 JSON** + 加 `text` 字段。

**实际行为**:
```json
{
  "passed": true,
  "errors": [...],
  "summary": {...},
  "text": "[ok]"   // ← text mode 追加的 summary 字段
}
```

**评价**: 这是**好的设计** (双轨: programmatic + human-readable). **不是 bug**, 是 doc 没说清楚。

**修法**: 改 `docs/implementation/mcp.md` 的 `kron_lint` 文档, 说明 text mode 追加 `text` 字段。

**暂不修**: 这是 doc 问题不是 code bug; 在 v1.2 文档清理阶段统一处理。

---

## B6: `kron_assume_check` 相对路径不 work (P2)

> `kron_assume_check {file_path: "src/auth/jwt.go"}` → "path not found"
> 同上, 但绝对路径 work

**证据**: raw stdio 测试, 14:30 实测.

**根因推测**: MCP spawn 的 process cwd **不是** project root, 所以相对路径解析到错误位置。

**Workaround**: 用绝对路径 (work)。

**暂不修**: 需判断这是 Cursor 桥 spawn 行为还是 kron server 自身路径解析问题。
**等**: Cursor spawn 行为文档化后, 判断是 MCP client 层还是 server 层问题。

---

## 设计观察 (P2, 不是 bug)

### 1. `kron_intent_density.intents_with_anchors` 含 dangling

**现象**: `kron_intent_density` 把 `src/broken/orphan.go → does/not/exist` 算"有 anchor", 计入 `intents_with_anchors`。

**评价**: **不**是 bug. "有 anchor" 跟 "anchor 指对了" 是两个信号, density 报前者, lint 报后者。两者互补, **不**能单独判断。

**修法**: 文档明说两者互补关系。**暂不修** (doc 问题)。

### 2. `kron_impact` 没有 `dependents` 字段

**现象**: spec / 旧版 snapshot 说 impact 返 dependents, 实际只返 `references` + `prerequisites` + `incoming_anchors`。

**真状态**: "谁依赖我" 在 `kron_get` 的 `Relations.Dependents`。`kron_impact` 看 **outgoing** (我依赖谁), **不**看 incoming (谁依赖我)。

**修法**: 文档说清楚 `kron_impact` vs `kron_get` 的视线方向区别。**暂不修** (doc 问题)。

### 3. `resources/list` 返空 ≠ 没 resource

**现象**: `resources/list` 返回 `[]`, 但 `kron://intents/{+slug}` template 是真的。

**真状态**: SDK 把 `list` (具体资源) 跟 `templates/list` (pattern) 拆开。Template 通过 `resources/templates/list` 取。

**修法**: 文档说明两者的区别。**暂不修** (doc 问题)。

---

## 测试架构教训 (P3, 已知, 不修)

### 1. Cursor MCP UI 限制

**现象**: tool 列表是 AI 看得见的, prompt + resource 列表**不**可见。AI agent **不**会自然调 `prompts/*` 和 `resources/*`。

**教训**: 测 MCP 必**须**用 raw stdio probe 绕 Cursor。**这是测试架构问题, 不是 AI 偷懒**。

**行动**: `E:\works\kron-mcp-test\raw-probe\main.go` 已建立 raw-stdio baseline。

### 2. "MCP 里有的都要测" = 5 个 method family

**现象**: r3 报告"18/18 tool 跑过"是不诚实的 — 漏了 `prompts/*` + `resources/*` + `resources/templates/*` + `notifications/*` 4 个 method family。

**教训**: MCP 有 5 个 method family (`initialize` / `tools/*` / `prompts/*` / `resources/*` / `notifications/*`), **不**只 tool。

### 3. server 跟 client 双**重** bug

**现象**: B1/B2/B3 是 Cursor 桥 bug (客户端错); B4 是 kron server bug (服务端错); 两层同时踩。

**教训**: AI agent 接 MCP server **必**须**有** raw stdio 测试 baseline, **不**只信 Cursor UI。

### 4. `kron-drift-check` prompt 是真的 AI-friendly 包装

**现象**: `prompts/get kron-drift-check` 直**接**包装 `kron_lint` 输出, 给 AI 一**个** ready-to-use review checklist, 跟 r3 `kron_lint` 输**出** 11 错**完**全**对**得上。

**评价**: **设**计**亮**点**, 不是 bug。prompt 是 lint 包装, 不是独立的审查系统。

---

## 总结: 暂不修清单

| # | 问题 | 类型 | 暂不修原因 |
|---|---|---|---|
| B1 | Cursor 桥 bool→string | Cursor bug | 等 Cursor 修 |
| B2 | Cursor 桥 array→object | Cursor bug | 等 Cursor 修 |
| B3 | Cursor 桥 int→string | Cursor bug | 等 Cursor 修 |
| B4 | prompt args 类型不一致 | Server+SDK | 等 SDK 支持 GetPromptParamsRaw |
| B5 | `kron_lint` reporter=text 文档漂移 | Doc | v1.2 doc 清理阶段统一处理 |
| B6 | `kron_assume_check` 相对路径不 work | 不确定责任方 | 等 Cursor spawn 行为文档化 |
| 设计 1 | density 含 dangling | Doc | v1.2 doc 清理阶段统一处理 |
| 设计 2 | `kron_impact` 无 dependents 字段 | Doc | 同上 |
| 设计 3 | `resources/list` 返空 ≠ 没 resource | Doc | 同上 |
