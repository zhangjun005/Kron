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
- **理解所有文档的角色与关系**：先读 [`abstractDesign/docs-map.md`](../abstractDesign/docs-map.md)

## 引用规则

本目录下的文件**只**做**信息检索引用**（在"如何使用"或脚注里给读者指路）和**真理引用**（引用 `architecture.md` / `intent-structure.md` 作为论据）。

**不**在正文中把 `process/` 文件作为论据（即不能用"按 process/foo.md 第 X 节规定……"这种措辞）。

> **执行触发**：当本目录的某个实施细节需要"什么时候做"的流程决策时（例：什么时候该加 CLI flag、什么时候该改 schema），说明这个细节应该移到 `process/` 下，而不是反过来引用。
