# `docs/mcp/` — Kron MCP 协议学习与未来工作

> **目的**:MCP 协议入门 + Kron 实现侧协议层切片 + 已知 GAP 与未来工作清单。
>
> **读者画像**(本目录按此画像组织):有 web 后端开发基础,首次接触 MCP 协议——重点是"JSON-RPC envelope、capabilities、transport、tool registration"为何设计成现在这样,以及 Kron 现状对应到协议的哪一层。

---

## 阅读顺序

| 序 | 文件 | 内容 | 何时读 |
|---|---|---|---|
| 1 | [`protocol.md`](./protocol.md) | MCP 标准协议入门(从 JSON-RPC envelope / stdio / capabilities 讲起);读完能回答"MCP 协议在做什么" | **先读**——建立"协议长什么样"的心智模型 |
| 2 | [`kron-implementation.md`](./kron-implementation.md) | 把 `cmd/kron/serve-mcp/` 现状按"协议层"切片讲清——`Run` / middleware / handler / tool registration 在协议里对应什么 | 读 1 后**立刻**读——把代码和协议对上 |
| 3 | [`backlog.md`](./backlog.md) | 已知 GAP 盘点(底层 `internal/` 改了 MCP 没跟上)+ 候选优化方向 | 读 1+2 后**按需**——决定下次 PR 范围 |

---

## 与现有文档的关系

| 既有文档 | 关系 |
|---|---|
| `docs/abstractDesign/architecture.md` §一 | 本目录**不**重复架构铁律;`kron-implementation.md` 只解释"MCP 这一层做什么" |
| `docs/how-it-works.md` §5.2 | 本目录**不**重写"AI 怎么用 MCP";只补"协议层细节" |
| `docs/implementation/`(2026-10-09 归档) | 原 `implementation/mcp.md` 严重过时,本目录作为**新参考**;**不**从 `.deprecated/` 复活(避免双份维护) |
| `AGENTS.md` / `.cursor/rules/project-conventions.mdc` | **不**改——本目录是"学习 + 计划",非"流程 / 铁律" |

---

## 本目录**不**做的事(避免误导)

- **不**写"Kron 已实装 X / 未实装 Y"的状态表(状态走代码顶部注释,`go-style` skill §"模块完成状态")
- **不**给协议下**新定义**——只整理 SDK 文档里的标准,标差异点
- **不**动 `internal/` / `cmd/kron/serve-mcp/` 代码——本目录**只**是 markdown
- **不**复制 RFC archive 内容——`.deprecated/2026-10-09-process-cleanup/` 留作历史

---

## 创建动机(2026-10-11)

- 仓库有 `cmd/kron/serve-mcp/` 实装,但**没有**"先理解协议再读代码"的入口文档
- `internal/assumption` / `internal/relations` / `internal/view` 三个 v1.2+ 包在 MCP 这一层**只用了部分 API**——`ReferencingIntents` / `assumption.Writer` **完全没被 MCP 调用**;新接手的人难以从代码反推"GAP 在哪"
- 协议本身(SDK 1.x + protocol version `2026-07-28`)有"能力协商" / "通知" / "订阅"等机制——`kron serve-mcp` **完全没有用到**——需要文档说明"为什么当前不用、什么时候该用"
