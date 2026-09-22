# Kron Project Structure

> 简明说明：Kron 的目录组织、谁能 import 谁、为什么这样划分。
> 完整架构论证见 [architecture.md §〇·五 抽象层次定义](./abstractDesign/architecture.md#〇五抽象层次定义访问层之间关系的论证) 与 [§〇 铁律](./abstractDesign/architecture.md#〇铁律架构不可妥协)。

## 一行话

Go 代码全部位于 [`cmd/kron/`](../cmd/kron/)（入口 + 各种访问层子包）与 [`internal/`](../internal/)（无外部依赖的底层包）两个根目录。

## 三个抽象层次

Kron 用三层划分而不是常见的两层（业务 vs 访问）：

| 层 | 职责 | 仓库位置 |
|---|---|---|
| **对象层（Domain）** | `.md` 文件的数据结构本身；最少行为；零外部 import | `internal/model/` |
| **业务层（Business）** | 文件 I/O、YAML 解析、锚点扫描、校验；操作对象层；不感知调用方 | `internal/store/`, `internal/parser/`, `internal/lint/`（按需） |
| **访问层（Access）** | 协议适配（cobra / JSON-RPC / LSP / HTTP）、参数解析、输出序列化 | `cmd/kron/<sub>/`（`cli/`、`serve-mcp/`、`serve-gui/` ...） |

详细论证见 `architecture.md` §〇·五（访问层之间为什么不能互相调用的三条理由 + 下沉判定 + 三级回退）。

## 完整目录树

```
Kron/
├── cmd/kron/
│   ├── main.go                    ← thin entry；仅 import 访问层子包
│   ├── cli/                       ← 访问层：cobra 命令（kron init / add / lint）
│   └── serve-mcp/                 ← 访问层：MCP stdio server（v1 计划）
│
├── internal/
│   ├── model/                     ← 领域类型（Intent, Frontmatter, Config），零外部依赖
│   ├── store/                     ← 文件 I/O + YAML frontmatter 读写
│   └── parser/                    ← CLI 参数解析、markdown 锚点扫描、相对链接识别
│
├── docs/                          ← 设计与需求文档（保留）
├── .cursor/                       ← Cursor 规则与 skill（保留）
├── go.mod, go.sum
├── README.md, AGENTS.md
└── docs/architecture-structure.md ← 本文件
```

## 三条铁律（详见 `architecture.md` §〇）

1. **`internal/` 只放底层包**：`internal/model`、`internal/store`、`internal/parser`。
   不放任何访问层代码——过去把 CLI 放到 `internal/cli/` 是错误的，现已移至 `cmd/kron/cli/`。

2. **访问层全部在 `cmd/kron/<sub>/`**：CLI / MCP / LSP / IDE / GUI 都作为访问层子包平铺在此目录下。
   每个访问层 = 一个子包 = 一个独立的 cobra 子命令 / 一个独立的可执行入口。

3. **访问层之间禁止互相调用**：
   - `cmd/kron/cli/` **不** import `cmd/kron/serve-mcp/`
   - `cmd/kron/serve-mcp/` **不** import `cmd/kron/cli/`
   - 任何"两个访问层都要用的逻辑"必须先下沉到 `internal/` 的包，再由两边分别 import。
   - `internal/` 永远**不**反向 import `cmd/kron/`。

## 依赖方向（合法）

```
                       ┌──► cmd/kron/cli/         ┐
                       │                           │
cmd/kron/main.go ──────┤                           ├──► internal/store/ ──► model
                       │                           │
                       └──► cmd/kron/serve-mcp/    ├──► internal/parser/ ─► model
                           (Phase 1 实现)          │
                                                   └──► internal/model/
```

## 依赖方向（违规 → 拒绝合并）

```
internal/* ──► cmd/kron/*       ❌  底层反向依赖入口
cmd/kron/cli/ ──► cmd/kron/serve-mcp/    ❌  访问层互相调用
```

## 为什么这样划分

- **GUI 与 MCP 平级**：Phase 2 增加 `cmd/kron/serve-gui/` 跟现在的 CLI、MCP 完全同构，
  三个子包都用同一套 `internal/store` + `internal/parser`，互不依赖。
- **没有第二个 `internal/cli/`**：所有"编排 store + parser 的业务逻辑"都下沉到 `internal/store`
  函数本身（推荐），或在 `cmd/kron/cli/` 这种访问层就地组合。**不**再有"业务函数库 + 访问层"的双层
  分裂——这种分裂在小项目是冗余，在大项目会演化成"业务函数库 = 第二个访问层"的反模式。
- **CI 可机械检查**：见 `architecture.md` §2.2 的 import 规则清单，
  `grep -r "internal/cli" cmd/` 应该 0 命中，`grep -r "cmd/kron/" internal/` 也应 0 命中。

## 演进点（v1 不实现，结构已留口）

| 方向 | 现在已有的形态 | 后续如何加 |
|---|---|---|
| GUI 客户端 | `cmd/kron/serve-gui/` 位置已留（未创建） | 新增 `cmd/kron/serve-gui/main.go`，从 `internal/store` 调用 |
| IDE 插件 | 不在仓库内（Cursor/VSCode 扩展独立打包） | 独立项目，**只** import `internal/parser` + `internal/store` |
| LSP server | 未创建（v1 不交付） | 新增 `cmd/kron/serve-lsp/`，与 CLI / MCP 同构 |
| 业务函数库 | 不存在——业务逻辑下沉到 `internal/store` 函数内 | **不**再开 `internal/` 之下的"业务编排层" |
