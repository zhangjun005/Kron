# Implementation Reference（实施参考）

> 架构真理位于 [`docs/abstractDesign/architecture.md`](../abstractDesign/architecture.md)。  
> 本目录下所有文件是**实施建议**，不是架构真理。如有冲突，以 `architecture.md` 为准。

---

## 文档地图

| 文件 | 内容 |
|---|---|
| [`api-surface.md`](./api-surface.md) | 所有导出函数签名（`internal/` + `cmd/kron/`） |
| [`cli.md`](./cli.md) | CLI 命令实现参考（init / add / lint / soft-delete） |
| [`domain-model.md`](./domain-model.md) | 领域模型结构体（Intent / Frontmatter / Status / Anchor / Config） |
| [`error-catalog.md`](./error-catalog.md) | 错误 sentinel 清单 + 包装规则 + 退出码对照 |
| [`ide-interaction.md`](./ide-interaction.md) | IDE / LSP 三路交互（Phase 2 占位） |
| [`mcp.md`](./mcp.md) | MCP 工具契约（8 工具入参/出参）+ GUI API 边界（Phase 2） |
| [`testing.md`](./testing.md) | 分层测试策略 + fixture 管理 + CI 门禁 |

---

## 关系图

```
architecture.md（真理）
  ├─ §1.1 / §1.2  → cli.md + mcp.md
  ├─ §3           → domain-model.md
  ├─ §4           → error-catalog.md
  ├─ §5           → cli.md（流程）+ mcp.md（契约）+ process/ 下的流程文档
  └─ §6           → testing.md
```

---

## 如何使用

- **写代码前**：先查本目录，找对应函数的签名示例
- **新增 lint 规则**：先读 [`process/lint-rule.md`](../process/lint-rule.md)
- **新增 CLI flag**：先读 [`process/cli-flag.md`](../process/cli-flag.md)
- **改 frontmatter schema**：先读 [`process/migrate.md`](../process/migrate.md)
