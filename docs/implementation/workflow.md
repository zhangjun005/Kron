# Kron 工作流 Playbook

> 面向人、AI Agent、IDE/GUI 的端到端使用流程。命令/工具名以实际实现为准（见
> `docs/mcp/kron-implementation.md` 与 `docs/implementation/cli.md`）。

## 0. 角色与分工

| 角色 | 职责 | 主要入口 |
|---|---|---|
| 人类作者 / Reviewer | 写 intent、定假设、审 PR | IDE（VSIX）、Git |
| AI Agent | 读意图/假设、实现、更新 intent、跑检查 | MCP（`kron-implement` / `kron-review` / `kron-drift-check`） |
| CI | 硬门禁：lint 错误必须为 0 | CLI `kron lint` |
| GUI | 多项目总览、状态巡检、封面对比（只读） | Wails GUI |

## 1. 初始化一个新项目

```bash
kron init            # 建 .kron/、注册项目、设为 current_project
```

- `init` 幂等：已有 `.kron/` 时保留骨架，但仍会注册 + 选中（与 MCP `kron_init` 一致）。
- 组织方式：模块级 intent 用 `<slug>/README.md`（node），具体能力用 leaf；层级靠目录表达。
- 可选：在 `.kron/templates/intent-body.md` 定义正文 section 模板（如 `## Why` / `## How`），
  MCP server 会把它注入 `initialize.Instructions`、`kron_add`/`kron_update` 描述和
  `kron-implement` prompt，约束 AI 通过 MCP 写 intent 的正文结构。
- 共享前提（region、幂等、限流、精度…）写 `.kron/assumptions/<id>.md`，填
  `text` / `default_severity` / `created_by`；重要的设置 `expires_at`。

## 2. 日常开发循环（人 + IDE）

1. **需求进入**：先写/改 intent（为什么、边界、不做什么），用 IDE 表单或 `kron_add`。
   - 关联 assumption 时逐条填 `severity`、`rationale`、必要时 `expires_at`。
   - 关系：`depends_on` = 不懂它就无法理解本意图的硬前置；`references` = see-also。
2. **写代码**：在归属符号上方加 `@kron:intent <slug>`；IDE 里 Ctrl+click 跳回意图。
3. **本地收口**：`kron lint`；修 dangling anchor/reference、depends_on 环、过期 hard assumption。
4. **提交/PR**：intent Markdown 与代码一起进 PR；按 PR 模板的 "Kron intent checklist" 填。
   - 新功能用 `kron-implement` 拿"匹配意图 + prerequisites + assumptions"。
   - 审 PR 用 `kron-review` 拿按规则分组的 lint 明细（含 detail/severity）。
5. **合并后**：`status: draft → active`；reviewers 落 `@handle`；假设复核后更新
   `verified_at` / `verified_by` / `expires_at`。

## 3. AI Agent 循环（MCP 客户端）

1. `kron-implement(feature=...)` → 读返回的 intent / prerequisites / assumptions。
2. `kron_get` 读正文；`kron_assume_get` 读共享假设文本。
3. **过期/未验证的 hard assumption 是风险**：先 `kron_assume_check`，必要时
   `kron_assume_update`（`verified_at` / `expires_at`）或向人确认，禁止自行假设。
4. 实现代码 + 保持锚点同步；用 `kron_update` 改 intent（PATCH，只发改动字段）。
5. 结束前 `kron_lint`（必要时 `kron_stale`），修完再交付。
6. 删除/恢复走 `kron_delete` / `kron_restore`（node 删除需 `confirm_subtree: true`）。

## 4. 周期巡检（团队治理）

- **每迭代 / 每周**
  - `kron_stale(days_threshold=…)` 找长期未更新的 active intent；
  - `kron_assume_check` 找过期/未验证的 hard assumption；
  - 处理：`kron_update`（refresh `updated_at` 或 `status=superseded`）、
    `kron_assume_update`（复核/改 severity/延后或清空 `expires_at`）。
- **季度**：清理 superseded / 重复 intent；软删不再成立的 assumption（引用它的 intent 同步 detach）。
- **度量**：`kron_intent_density` 看哪些源文件没有锚点覆盖。

## 5. CI 门禁建议

```yaml
- name: Kron lint
  run: ./kron lint            # 退出码 1 = 有 error，PR 阻断
- name: Kron stale report
  run: ./kron stale --days 90 # 可选：报告不阻断
```

- 不要在 CI 里跑 `kron init`（CI 只读检查）。
- 如果仓库使用 `kron-mcp-test/app/kron.exe` 这类发布目录，CI 用自己构建的 `kron` 二进制。

## 6. 工具边界（不要越界）

- IDE 只通过 MCP/LSP 访问 Kron，不直接 import `internal/`。
- GUI 只读（项目总览/巡检），不做编辑；编辑在 IDE 或 Agent。
- Agent 不直接改 `.kron/*.md` 文件，统一走 MCP 工具，保证校验/软删/注册表一致。
- 手工改 Markdown 可以，但改完必须 `kron lint`；`.kron/` 位置固定在项目根。

## 7. 当前已知缺口（工作流会感知到的）

- VSIX 没有保存/外部变更自动刷新（需手动 Refresh；客户端事件驱动方案待做）。
- 没有 trash 列举工具 → IDE 侧看不到"真正被软删"的清单（只能看 superseded）。
- GUI 的树 enrichment 仍是逐 leaf `kron_get`（大项目慢）。
- LSP 只有 hover/definition，无 diagnostics/completion。
- CI workflow 与 LICENSE 已补齐，但尚无发布流水线。
