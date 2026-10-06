# Process（实施流程文档）

> 本目录下所有文件是**实施流程**，不是架构真理。

---

## 文档地图

| 文件 | 触发条件 |
|---|---|
| [`ci-enforcement.md`](./ci-enforcement.md) | 想加 CI 检查 / 想了解现有规则的覆盖盲区 |
| [`cli-flag.md`](./cli-flag.md) | 想给现有 CLI 命令加新 flag |
| [`internal-pkg.md`](./internal-pkg.md) | 想新建 `internal/<name>/` 子包 |
| [`lint-rule.md`](./lint-rule.md) | 想给 `kron lint` 增加新检查规则 |
| [`mcp-tool.md`](./mcp-tool.md) | 想给 `kron serve-mcp` 增加新 MCP 工具 |
| [`migrate.md`](./migrate.md) | 想修改 frontmatter schema |
| [`references-snapshot.md`](./references-snapshot.md) | 想看当前 md 之间实际引用关系（快照而非规则） |
| [`phase-archive/`](./phase-archive/) | 历史 phase 遗留事项归档（**不**作为待办清单使用；详见该目录 README） |

---

## 通用原则

所有流程文档共享以下原则：

1. **先论证，后实现** — 论证 commit 先于实现 commit
2. **幂等** — 脚本必须可以多次运行，结果相同
3. **CI 门禁** — 流程结果通过 `kron lint` 验证
4. **文档同步** — 实施完成后，同步更新 `docs/abstractDesign/architecture.md`（真理）和 `docs/implementation/`（建议）
