# 架构设计（Architecture）

> 本文档是 Kron v1 的**架构真理源**。`AGENTS.md` 与 `.cursor/rules/*` 是它的镜像。
>
> **实施建议**（具体 API 签名、工具契约、流程图）移至 [`docs/implementation/`](../implementation/)
> 和 [`docs/process/`](../process/)。如有冲突，以本文档为准。

---

## 〇、铁律（架构不可妥协）

1. **业务逻辑只在 `internal/`**；`cmd/kron/` 只做 wiring。任何在 `cmd/` 写 `for` 或 `if err != nil` 的冲动都要移走。
2. **底层与访问层解耦**：`internal/store`、`internal/parser`、`internal/model` 是纯函数库，不感知调用方是 CLI / MCP / LSP。所有访问层都位于 `cmd/kron/<sub>/`（如 `cmd/kron/cli`、`cmd/kron/serve-mcp`、`cmd/kron/serve-lsp`），**不**再使用 `internal/cli/` 或 `internal/gui/` 之类的访问层包名。
3. **访问层之间禁止互相调用**（新增）：CLI / MCP / LSP 三种协议访问层之间**不能**直接 import 或调用彼此。需要跨入口复用的能力（如 LSP hover 复用 parser 锚点扫描）必须**下沉到 `internal/`** 实现，再由各访问层独立调用。任何"访问层调用访问层"的代码都是架构违规。
4. **调用方身份走 `context.Context`**（**2026-10-08 变更**：caller 注入 API **不再推荐**，详见 §2.3）：所有需要区分调用方的函数签名第一个参数必须是 `ctx context.Context`。`ctx` 仍保留**用于取消 / 超时传播**；身份属性注入（caller key）**不再**作为推荐做法。
5. **零新依赖**：与 `AGENTS.md` 一致；任何新依赖必须带 commit body 论证。
6. **数据真理源是仓库内的 `.md`**：二进制不持有项目数据；不维护状态机、不做双源同步。
7. **CI lint 是唯一可信门禁**：运行时不维护隐式状态；强约束通过 `kron lint` 在 CI 报错实现，不靠运行时默认值兜底。
8. **依赖方向单向**：`cmd → {cli, serve-mcp, serve-lsp} → {store, parser, lint} → model`；严禁反向；`model` 零外部依赖。
9. **(2026-10-08 新增, 2026-10-09 修订) 三层架构**：
   - **对象层** = `internal/model/`
   - **协议访问层** = `cmd/kron/cli` / `cmd/kron/serve-mcp` / `cmd/kron/serve-lsp`（CLI / MCP / LSP 三种协议适配器）
   - **客户端层** = VSCode 扩展 / Wails / Cursor / 其他 IDE / GUI 客户端
   - **关键原则**：客户端层**只**通过**协议访问层之一**调用能力（拼 JSON 走 serve-mcp、调 LSP protocol 走 serve-lsp、命令行 argv 走 cli）。客户端层代码**存**在 Kron 主仓 `frontend/`（2026-10-09 修订：客户端层**进**主仓，不再独立建仓）；客户端之间**不**互相 import。

---

## 〇·五、抽象层次定义（访问层之间关系的论证）

### 〇·五·1 三个抽象层次 + 客户端层

| 层 | 中文 | 职责 | 仓库位置 | 谁**不**能访问它 |
|---|---|---|---|---|
| **对象层（Domain）** | 对象 | 表示 `.kron/intents/*.md` 的数据结构；最少行为；零外部 import | `internal/model/` | 任何东西——它只能被 import，不能 import 任何东西 |
| **业务层（Business）** | 业务 | 文件 I/O、YAML 解析、锚点扫描、锚点校验；操作对象层；不知道谁来调用、不知道进程模型 | `internal/store/`, `internal/parser/`, `internal/lint/`, ... | 访问层**不**被业务层反向 import（`internal/` 永远不 import `cmd/kron/`） |
| **协议访问层（Protocol Access）** | 协议 | 协议适配（cobra / JSON-RPC / LSP）、参数解析、输出序列化、组装业务层函数；3 个：**CLI / MCP / LSP** | `cmd/kron/cli/`, `cmd/kron/serve-mcp/`, `cmd/kron/serve-lsp/` | 业务层**不**被要求感知访问层；其他协议访问层**绝不**被 import；客户端层**不**直接调业务层 |
| **客户端层（Client）** **(2026-10-08 新增, 2026-10-09 修订)** | 客户端 | 给**人**用的 UI（Wails 桌面 / VSCode 扩展 / Cursor / 其他 IDE / Web GUI）；客户端**只**通过**协议访问层之一**（如 VSCode 扩展拼 JSON 走 serve-mcp、LSP 协议走 serve-lsp）调用能力 | Kron 主仓 `frontend/`（2026-10-09 修订：客户端层**进**主仓，不再独立建仓；各客户端代码在 `frontend/` 下各自子目录或 monorepo 模式） | 客户端**不**直接 import `internal/`（走协议访问层）；客户端**不**互相 import |

> **(2026-10-08 变更, 2026-10-09 修订)** 原 §〇·五·1 把 IDE / GUI 当 access layer 看待——**新版图**把 IDE / GUI 提到**客户端层**（它们是"给**人**用的 UI"），而 access layer 收紧到**协议层**（CLI / MCP / LSP 三种 wire protocol）。2026-10-09 修订：客户端层代码**存**在 Kron 主仓 `frontend/`，不再独立建仓。理由：v1.3 开发阶段主仓放 `frontend/` 让 clone 一次搞定、PR 一个写完、开发体验更顺；协议隔离（客户端走 serve-mcp / serve-lsp，不直接 import `internal/`）仍保证铁律 #1 的 `internal/` 零外部依赖不被污染。

> **不是**"业务逻辑层 vs 访问层"二层——加对象层是因为它解决"什么是数据、什么是规则、什么是协议"这个本质问题。
> 没对象层做参照，业务层和访问层的边界会塌缩成"反正都在 internal"。

### 〇·五·2 访问层之间禁止互相调用：三条理由

**A. 关注点分离（哲学层面）**——每个访问层有自己的协议、生命周期、失败语义：

| 访问层 | 协议 | 生命周期 | 失败语义 |
|---|---|---|---|
| CLI | 命令行 argv / stdout / stderr | 一次 fork-exit（百毫秒级） | POSIX exit code + stderr |
| MCP | stdio JSON-RPC 2.0 | 长连接（响应 keepalive） | JSON-RPC error code |
| LSP | stdio + 自定 frame 协议 | 长连接（响应 alive check） | LSP `window/showMessage` |

> **(2026-10-08 变更)** 原 §〇·五·2 表把 IDE 扩展 / GUI 当 access layer——**新版图**把 IDE / GUI 移到**客户端层**（见 §〇·五·1）。本节剩余论证（关于"下沉到 internal"的论证）原封不动——只是**枚举**从 5 减到 3（CLI / MCP / LSP）。

复用 LSP 给**客户端层**实现"hover 跳转"，比让客户端直接调 `internal/parser` 难 10 倍：要协商
Position encoding、文档同步协议、capabilities。下沉到 `internal/` 是"算法共用"；
保留独立协议访问层是"协议隔离"——两条目标只能各占一边，混搭必塌。客户端层**只**通过协议访问层**之一**拿能力（VSCode 扩展既可以拼 JSON 走 serve-mcp 拿 list/get，也可以起 serve-lsp 子进程拿 hover/go-def）；客户端层**不**直接 import `internal/`。

**B. 进程模型（物理层面）**——不同协议访问层寿命错配：

- CLI：一次性 fork-then-exit，**不**承载长连接逻辑
- MCP / LSP：长连接，**不**能接受"每条命令重新协商协议"
- 一个寿命 100ms 的 CLI 进程若承载 LSP 逻辑：进程边界模糊、升级兼容性差
  （升级 LSP 协议要重编 CLI）、部署方式打架（一个二进制 vs 多个服务）

> **(2026-10-08 变更)** 客户端层（IDE / Wails / Web）寿命与协议访问层**不**绑定：客户端寿命 = 它宿主（编辑器进程 / Wails 窗口 / 浏览器 tab）；客户端**不**自己实现协议，**只**调协议访问层（spawn serve-mcp / serve-lsp 子进程、或拼 JSON-RPC）。客户端寿命错配**不**属于本节论证范围。

**C. 升级和测试的爆炸半径（实操层面）**——以"客户端层（VSCode 扩展）调 LSP 实现 hover 跳转"为例：

```
方案 X：VSCode 扩展 直接依赖 LSP 进程 (拼 LSP wireframe)
→ LSP 协议从 3.17 升 3.18 后，VSCode 扩展 必须重新发布、重新打包

方案 Y：VSCode 扩展 依赖 serve-lsp 子进程 (走 LSP SDK), 业务代码依赖 internal/parser (下沉)
→ internal/parser 抽象不变（hover = 给 slug → 给 Anchor → 给 Location），
  LSP 升级到 3.18、VSCode 扩展 升级、两者独立打包
```

"被依赖方的 ABI 稳定性 = 内部包"——这条经验法则决定我们必须先下沉，再分头打包。

> **(2026-10-08 变更)** VSCode 扩展 / Wails / Cursor 等客户端**只**通过协议访问层（serve-mcp / serve-lsp）调 `internal/`，**不**直接 import `internal/`。一个项目 = 一个 serve-mcp 实例（与 AI 工具**共用**，见 §一·1）；LSP 是**编辑器侧 hover 预览**专用（§一·2）。

### 〇·五·3 业务层下沉判定（什么下沉到 `internal/`、什么不）

| 场景 | 判定 | 例子 |
|---|---|---|
| 操作 `.md` 文件 / 读 frontmatter / 写 `Intent` | ✅ 下沉到 `internal/store` | `WriteIntent(ctx, slug, *Intent)` |
| 解析 CLI 参数串 / markdown 文本 / 锚点语法 | ✅ 下沉到 `internal/parser` | `ParseSlug("auth/jwt")` |
| 编排 store + parser 的"组合逻辑"**只在**一个访问层用 | ❌ **不**下沉，留在该访问层包内 | MCP 的 JSON-RPC envelope 处理留在 `serve-mcp/` |
| 编排 store + parser 的"组合逻辑"**两个以上**访问层都要用 | ✅ 下沉到**新开**的 `internal/<业务名>/` | `kron_lint` 逻辑 CLI、MCP、IDE 都调 → `internal/lint/` |
| 协议消息格式 / 输入输出序列化 | ❌ **不**下沉，永远留在访问层 | MCP 的 request schema 留在 `serve-mcp/` |

> **不**再有"业务函数库"作为单独一层（拒绝 `internal/business/` 或 `internal/usecase/`）。
> 上一轮已经把 `internal/cli/` 删了，理由相同：业务编排要么下沉到访问层自己，
> 要么下沉到具体业务的内部包（`internal/lint/`），**不**留中间层。

### 〇·五·4 跨访问层复用的三级回退

```
情况 A：两个以上访问层都需要的能力（lint / list / get / 校验）
     → 下沉到 internal/<业务名>/（专项包；不是 store，也不是 parser）
     → 例：internal/lint/ 供 CLI、MCP、IDE 三家共享

情况 B：访问层之间本来就不该共享的能力（协议消息格式 / 输入输出序列化）
     → 各自实现，禁止提取
     → 例：MCP JSON-RPC envelope 永远只活于 cmd/kron/serve-mcp/

情况 C：一个访问层独有的子能力，别的访问层想蹭
     → 拒绝；想蹭必须先证明是情况 A
     → 例：serve-mcp 想蹭 cli 的 --json 输出格式 → 拒绝；MCP 应该用 JSON-RPC 字段
```

### 〇·五·5 `internal/` 之下的包分工（v1 草案，可随 v1 推进调整）

| 包 | 职责 | 何时开 |
|---|---|---|
| `internal/model` | 对象层：纯数据结构 + 校验方法 | v1 必须 |
| `internal/store` | 业务层：文件 I/O + YAML 序列化 | v1 必须 |
| `internal/parser` | 业务层：输入解析（slug、markdown 文本、锚点语法、相对链接） | v1 必须 |
| `internal/lint` | 业务层：扫描 + 校验组合（CLI 的 `kron lint` 与 MCP 的 `kron_lint` 共享引擎） | **v1 已实开**（含 `internal/lint/lint_test.go`）；CLI 与 MCP 共享引擎 |
| `internal/relations` | 业务层：反向链接图（v1.2+ `references` / `depends_on` 的反向视图） | **v1.2 已实开**（含 `internal/relations/relations_test.go`） |
| `internal/identity` | 业务层：`created_by` 解析（GitUser / Handle） | **v1 已实开**（含 `internal/identity/identity_test.go`） |
| `internal/assumption` | 业务层：`.kron/assumptions/<id>.md` 独立文件 reader/writer（B-3, RFC `2026-10-08-assumptions-standalone.md`） | **v1.3 已实开**（含 `internal/assumption/{reader,writer}.go` + 测试）；MCP (`kron_assume_check` / `kron_list` / `kron_get` / `kron_impact`) 共享 |
| `internal/...` | **不**预设更多 | 每个新 `internal/` 子包都要写"为什么开"的 commit 论证 |

---

## 一、访问层全景

> **(2026-10-08 变更)** 原 §一 把 IDE / GUI 当作与 CLI / MCP / LSP **同级的访问层**——**新版图**把它们移到**客户端层**（见 §〇·五·1 + 铁律 #9）。访问层**枚举** = CLI / MCP / LSP 三种 wire protocol；客户端层（IDE / Wails / Cursor / VSCode 扩展 / Web GUI）**只**通过这 3 种协议访问层**之一**调 `internal/`，**不**直接 import `internal/`。

Kron 的核心是**数据格式 + 协议**，不是一个二进制。同一份仓库内 `.md` 数据通过**三层**触达：

### 1.0 三层触达图

| 层 | 类型 | 入口 | 用户 | 触达路径 | 共享底层 |
|---|---|---|---|---|---|
| **协议访问层** | wire protocol | **CLI** | 人类 / 脚本 / CI | 终端直接执行 | `internal/store`、`internal/parser` |
| **协议访问层** | wire protocol | **MCP server** | AI Agent | stdio JSON-RPC | `internal/store`、`internal/parser` |
| **协议访问层** | wire protocol | **LSP server** | 人类（在编辑器中） | stdio + JSON-RPC over LSP | `internal/store`、`internal/parser` |
| **客户端层** | GUI 客户端 | **VSCode 扩展** | 人类（在 VSCode 中） | 拼 JSON-RPC 走 `serve-mcp` 子进程 + spawn `serve-lsp` 子进程 | 客户端**不**直接 import `internal/`；通过 serve-mcp / serve-lsp **间接**触达 |
| **客户端层** | GUI 客户端 | **Wails（多项目 GUI）** | 人类（视觉化） | Wails 进程内 Go ↔ TS 桥 → 调**协议访问层**之一（Wails **不**直接 import `internal/`；与 VSCode 扩展共用 serve-mcp 实例，承载**简单整体项目管理** + 意图树预览） | 同上 |
| **客户端层** | GUI 客户端 | **Cursor / 其他 IDE 扩展** | 人类（在 Cursor / 其他 IDE 中） | 复用 VSCode 扩展 LSP / MCP 集成（VSCode 扩展协议兼容 Cursor） | 同上 |

> **关键原则**：
> 1. **协议访问层 = 3 个**（CLI / MCP / LSP）。Kron 主仓只 commit 这 3 个。
> 2. **客户端层 ≠ Kron 主仓**。VSCode 扩展源码 / Wails 前端代码**不**入主仓。VSCode 扩展**独立**仓库；Wails 前端在 `frontend/` 已被 `.gitignore`。
> 3. **(2026-10-08 拍板)** **MCP 进程寿命 = stdio 父进程寿命**。每个 client (VSCode 扩展 / Wails GUI / AI 工具) 各自 spawn 一个 `kron serve-mcp`, 各自复用, 各自 kill。**一个项目可能同时存在 N 个 MCP 实例** (N = client 数), 并发安全靠 `internal/store` 文件锁 (`flock` per-file), **不**靠 MCP 协议层 / OS 进程隔离 / 单例 daemon。**不**开独立 GUI 接口层——GUI 是**用户**用的, 不是**协议**。详见 [`docs/rfc/2026-10-08-mcp-lifecycle.md`](../rfc/2026-10-08-mcp-lifecycle.md)。
> 4. **LSP 是编辑器侧 hover 预览专用**（`textDocument/hover` + `textDocument/definition`）。VSCode 扩展既走 serve-mcp（业务能力）也起 serve-lsp 子进程（编辑器 UI 集成）。
> 5. **客户端层互相不依赖**。Wails **不** import VSCode 扩展；VSCode 扩展**不** import Wails。它们**独立**通过协议访问层拿能力。

### 1.1 CLI 最小集

| 命令 | 职责 |
|---|---|
| `kron init` | 初始化 Kron 工作区 |
| `kron add <slug>` | 脚手架新意图文件 |
| `kron lint` | CI 门禁：锚点悬空 + frontmatter 校验 |
| `kron serve-mcp` | 启动 MCP stdio server |

完整实现参考：见 [`docs/implementation/cli.md`](../implementation/cli.md)。  
不在 CLI 中实现：`list` / `get` / `update` / `delete` / `restore`——由 MCP 工具集覆盖。

### 1.2 MCP 工具集

| 类别 | 工具 | 职责 |
|---|---|---|
| 初始化 | `kron_init` | 提供一次性工作区建立能力 |
| 增 | `kron_add` | 提供意图文件脚手架能力 |
| 查 | `kron_list`、`kron_get` | 提供意图列表与详情查询能力 |
| 改 | `kron_update` | 提供意图内容更新能力 |
| 删（软） | `kron_delete` | 提供可恢复的软删除能力 |
| 恢复 | `kron_restore` | 提供从软删除状态还原的能力 |
| 校验 | `kron_lint` | 提供锚点与 frontmatter 校验能力 |
| 主动消费 | `kron_assume_check`、`kron_impact`、`kron_intent_density`、`kron_stale` | 提供 AI Agent 编码循环中**主动**消费意图的能力——把"AI 主动校验假设 / 量化覆盖率 / 影响范围"从口头承诺变成可执行 API |

完整工具契约（入参/出参/错误码）：见 [`docs/implementation/mcp.md`](../implementation/mcp.md)。

> 新增 MCP 工具的流程：见 [`docs/process/mcp-tool.md`](../process/mcp-tool.md)。

---

## 二、包结构与依赖方向

```
cmd/kron/                        ← 入口，只 dispatch 子命令，不含业务逻辑
├── main.go                      ← thin entry, 仅 import 协议访问层子包
├── cli/                         ← 协议访问层：cobra 命令（init / add / lint）
├── serve-mcp/                   ← 协议访问层：MCP stdio server（v1 交付）
└── serve-lsp/                   ← 协议访问层：LSP stdio server（v1.3+）

internal/                        ← 只放底层包，禁止 import 任何协议访问层
├── model/                       ← 领域类型，零外部依赖
├── store/                       ← 文件 I/O、frontmatter 读写；仅依赖 model
└── parser/                      ← 输入解析（CLI 参数、锚点扫描、相对链接识别）；仅依赖 model
```

> **(2026-10-08 变更, 2026-10-09 修订)** 原"CLI / MCP / LSP / IDE / GUI 五访问层"改为"CLI / MCP / LSP 三协议访问层"。**不**再保留 `cmd/kron/serve-gui/`（GUI 是**客户端层**，不是访问层——独立）。2026-10-09 修订：客户端层代码进 Kron 主仓 `frontend/`（`frontend/wails/` + `frontend/vscode/`），**不**再独立建仓。

### 2.1 依赖图

```
                      ┌─► cmd/kron/cli/         ───► internal/store/  ─► model
                      │
cmd/kron/main.go ─────┼─► cmd/kron/serve-mcp/   ───► internal/parser/ ─► model
                      │
                      └─► cmd/kron/serve-lsp/   ───► internal/lint/    (按需新增)
                          (v1.3+)             ──────────────────────────► model
```

> **客户端层（VSCode 扩展 / Wails / Cursor / 其他 IDE / Web GUI）不**在 Go 依赖图里（Go 代码不 import TS/React 客户端）；但**代码存在** Kron 主仓 `frontend/` 下（2026-10-09 修订）。客户端层**只**通过协议访问层（CLI 子进程 / serve-mcp stdio JSON-RPC / serve-lsp stdio LSP 协议）调能力。
>
> 一个项目 = 一个 `serve-mcp` 实例（人类用 AI 工具与 GUI 客户端**共用**）；LSP 是**编辑器侧 hover 预览**专用（VSCode 扩展 spawn serve-lsp 子进程拿 `textDocument/hover` / `textDocument/definition`）。

- `model`：纯数据结构与方法，零外部 import。
- `store`：文件系统操作 + YAML 序列化。**不**感知 CLI/MCP/LSP。
- `parser`：字符串解析、锚点扫描、相对链接识别。**不**做文件 I/O（输入由调用方传入）。
- `lint`（当开时）：扫描 + 校验组合逻辑；**不**感知任何访问层（CLI / MCP / IDE 都各自 import 它）。
- `cmd/kron/*`：编排 store / parser / lint；**只** import `internal/*`，**不** import 其他 `cmd/kron/*`。

### 2.2 协议访问层 import 规则（铁律 #3 的落地形式）

协议访问层之间的 import 关系**只能**是：`协议访问层 → internal/*`，**不**允许 `协议访问层 → 协议访问层`。

```go
// ✅ 正确：协议访问层（CLI 子命令）只 import internal/
// cmd/kron/serve-mcp/main.go
import (
    "github.com/xxx/kron/internal/store"
    "github.com/xxx/kron/internal/parser"
)

// ❌ 违规：协议访问层之间互相 import
// cmd/kron/serve-lsp/main.go
import (
    "github.com/xxx/kron/cmd/kron/serve-mcp"   // 严禁
)

// ❌ 违规：在 internal/ 里反向 import 协议访问层
// internal/parser/foo.go
import (
    "github.com/xxx/kron/cmd/kron/serve-mcp"   // 严禁
)
```

判定清单（CI 可机械检查）：
- `cmd/kron/**` 只能 import `cmd/kron/**` 自身 + `internal/**` + 标准库 + 已批准的第三方包
- `internal/**` 只能 import `internal/**` 自身 + 标准库 + 已批准的第三方包；**不** import `cmd/kron/**`
- 任意 `cmd/kron/<sub>/` 只能 import `cmd/kron/<sub>/` 自身 + `internal/**`，**不** import 其他 `cmd/kron/<other>/`
- **客户端层（VSCode 扩展 / Wails / Cursor / Web GUI）不**在 Kron 主仓 CI 范围——它们是外部项目，独立 lint / 验收；但**客户端层 import 约束**与协议访问层一致：**不**直接 import `internal/`、**不** import 协议访问层（VSCode 扩展**不** import `serve-mcp` Go 代码，只通过 stdio JSON-RPC 通信）

判定清单（CI 可机械检查）：
- `cmd/kron/**` 只能 import `cmd/kron/**` 自身 + `internal/**` + 标准库 + 已批准的第三方包
- `internal/**` 只能 import `internal/**` 自身 + 标准库 + 已批准的第三方包；**不** import `cmd/kron/**`
- 任意 `cmd/kron/<sub>/` 只能 import `cmd/kron/<sub>/` 自身 + `internal/**`，**不** import 其他 `cmd/kron/<other>/`
- **客户端层（VSCode 扩展 / Wails / Cursor / Web GUI）不**在 Kron 主仓 CI 范围——它们是外部项目，独立 lint / 验收；但**客户端层 import 约束**与协议访问层一致：**不**直接 import `internal/`、**不** import 协议访问层（VSCode 扩展**不** import `serve-mcp` Go 代码，只通过 stdio JSON-RPC 通信）

### 2.3 调用方身份透传

> **变更 (2026-10-08)**: caller 注入 API **不再推荐**用于新访问层代码。`model.WithCaller` / `CallerFrom` / `IsMCPCaller` / `RequireKnownCaller` **保留**（兼容旧 import 与 `cmd/kron/serve-mcp` 的 `callerInjectMiddleware`），但**新**访问层代码**不**再调 `model.WithCaller` 注入；`internal/` **不**依赖 ctx 上的 caller key 做行为分支（属性归因信息**仍**可读，**仅**作为 lint / 审计的提示）。原因：`context.Context` 注入身份属性**不便于开发**（调试栈不直观 / 单元测试需额外包装 / 类型安全弱）。详见 [`internal/model/caller.go`](../../internal/model/caller.go) 头注释。

所有 store / parser 函数签名的第一个参数都是 `ctx context.Context`（**保留**——用于取消 / 超时传播，**不**用于 caller 透传）：

```go
// ✅ 正确：身份走 context（caller API 已废弃，见上）
func (s *Store) WriteIntent(ctx context.Context, slug string, intent *model.Intent) error

// ❌ 错误：身份作为普通参数
func (s *Store) WriteIntent(caller string, slug string, intent *model.Intent) error
```

身份 key 约定（**保留作历史**，新代码**不**再使用）：`caller = "cli"` / `"mcp"` / `"lsp"` / `"ide"` / `"gui"`。MCP 透传时可附带子标识，如 `"mcp:claude-3.7"`。

身份仅用于：
- lint 输出里标注调用方（**保留**）
- 未来审计日志（v1 不实现，但接口要留）

身份**不**用于权限控制——`add` 是文件写入，不存在"高安全需求"。

---

## 三、领域模型

见 [`docs/implementation/domain-model.md`](../implementation/domain-model.md)。

包含：`Intent` / `Frontmatter` / `Status` / `Anchor` / `Config` 结构体定义与字段语义。

---

## 四、错误模型

见 [`docs/implementation/error-catalog.md`](../implementation/error-catalog.md)。

包含：sentinel 错误清单（`ErrIntentNotFound` 等）、包装规则、CLI 退出码与 MCP JSON-RPC 错误码对照。

---

## 五、关键流程

### 5.1 `kron init`

见 [`docs/implementation/cli.md`](../implementation/cli.md)。

### 5.2 `kron add <slug>`

见 [`docs/implementation/cli.md`](../implementation/cli.md)。

### 5.2.1 软删除与恢复

见 [`docs/implementation/cli.md`](../implementation/cli.md) §5。

### 5.3 `kron lint`

见 [`docs/implementation/cli.md`](../implementation/cli.md) §4。

### 5.4 MCP / LSP 复用边界

| 复用层 | 方式 |
|---|---|
| MCP server | stdio JSON-RPC；`cmd/kron/serve-mcp/` 做序列化，直接调 `internal/store` 与 `internal/parser` |
| LSP server | LSP 协议笨重（文件同步、Position 偏移、生命周期），不做 |
| IDE 插件 | 宿主 LSP / MCP 进程；业务逻辑**直接**调 `internal/parser` 与 `internal/store` |
| GUI 客户端 | 通过独立 GUI API 边界访问（见 [`docs/implementation/mcp.md`](../implementation/mcp.md)），不依赖 CLI `--json` flag |

> **访问层之间禁止互相调用**（§〇 铁律 #3）。表中所有"复用"行的含义都是"共享同一个 `internal/` 业务逻辑"，不是"这一层调另一层"。
> 实际已落地的访问层入口看 `cmd/kron/` 当前目录结构。

#### 5.4.1 MCP 工具契约

见 [`docs/implementation/mcp.md`](../implementation/mcp.md)。

#### 5.4.2 GUI API 边界（设计性预留接口）

见 [`docs/implementation/mcp.md`](../implementation/mcp.md) §5。

---

## 六、测试策略

见 [`docs/implementation/testing.md`](../implementation/testing.md)。

包含：分层测试策略、fixture 管理、表驱动模板、集成测试闭环、CI 门禁命令。

---

## 七、不做的事（v1 范围外）

| 特性 | 原因 | 流程文档 |
|---|---|---|
| 守护进程、文件监听、状态机、双源同步 | v1 范围外 | — |
| CLI `list` / `get` / `update` / `delete` / `restore` | 由 MCP 工具集覆盖 | — |
| 健康度诊断（过期 / 孤儿 / 冲突检测） | `requirements.md` 未明确要求 | — |
| 导入迁移、`config.toml` 字段扩展 | v1 仅 `intents_dir` | — |
| `kron add` stdin / 外部模板支持 | 脚手架职责，复杂输入留给编辑器或 GUI | — |
| `kron lint` `--path` 自定义扫描根 | 全仓 + 黑名单足够 | — |
| **Wails GUI** (客户端层) | **(2026-10-08 变更) 移到客户端层；多项目概览 + 意图树预览 (轻)，不进 serve-gui 子进程；详细见 [`docs/rfc/2026-10-07-gui-stack.md`](../../docs/rfc/2026-10-07-gui-stack.md) §1.1 (SUPERSEDED) + 待重写 [`docs/rfc/2026-10-08-gui-layer.md`](../../docs/rfc/2026-10-08-gui-layer.md)** | — |
| **LSP server** (`kron serve-lsp`) | **v1.3+ pending — 见 [`docs/rfc/2026-10-07-lsp-sdk.md`](../../docs/rfc/2026-10-07-lsp-sdk.md) §3 (SUPERSEDED) + 待重写 [`docs/rfc/2026-10-08-lsp-client.md`](../../docs/rfc/2026-10-08-lsp-client.md)** | — |
| **VSCode 扩展** (客户端层) | **客户端层；在 Kron 主仓 `frontend/vscode/`；拼 JSON 走 serve-mcp (与 AI 共用) + spawn serve-lsp 拿 hover/go-def** | — |
| **Cursor / 其他 VSCode 兼容 IDE** | **(2026-10-08 新增) 复用 VSCode 扩展集成；客户端层独立项目** | — |
| **hard-delete / GC** | 软删除 + `.kron/.trash/` 足够 | — |
| 新增 CLI flag | — | [`docs/process/cli-flag.md`](../process/cli-flag.md) |
| 新增 lint 规则 | — | [`docs/process/lint-rule.md`](../process/lint-rule.md) |
| 修改 frontmatter schema | — | [`docs/process/migrate.md`](../process/migrate.md) |
| 新增 `internal/` 子包 | — | [`docs/process/internal-pkg.md`](../process/internal-pkg.md) |

---

## 八、演进方向（v1 不实现，接口要留）

| 方向 | 留口方式 | 流程文档 |
|---|---|---|
| AI 起草工作流 | MCP 工具集已覆盖完整生命周期；store 接口走 ctx（caller 注入 API 已废弃，见 §2.3；ctx 仍用于取消 / 超时）| — |
| AI 主动消费意图 | MCP 主动消费类工具（`kron_assume_check` / `kron_impact` / `kron_intent_density` / `kron_stale`）已在 §1.2；详细契约见 [`docs/implementation/mcp.md`](../implementation/mcp.md)；视图形态见 [`view-call-tree-intent.md`](./view-call-tree-intent.md) | [`docs/process/mcp-tool.md`](../process/mcp-tool.md) |
| GUI 客户端 | 独立的 GUI API 边界（见 [`docs/implementation/mcp.md`](../implementation/mcp.md)） | — |
| 健康度诊断 | `kron lint` 子命令可扩展 lint 规则集 | [`docs/process/lint-rule.md`](../process/lint-rule.md) |
| 多编辑器 LSP 复用 | LSP server 与 CLI 解耦，可独立打包 | — |

---

## 九、与其他文档的关系

| 文档 | 关系 |
|---|---|
| [`intent-structure.md`](./intent-structure.md) | 数据格式真理源；本文档引用其 frontmatter schema 与锚点语法 |
| [`docs-map.md`](./docs-map.md) | 所有文档的角色分类与单向引用链；本文档是关系图的被引用者 |
| [`how-it-works.md`](../how-it-works.md) | 实景示例（真实代码 + Kron 行为）；人类入口；本文档是引用者 |
| [`business.md`](../business.md) | 业务边界；本文档的"不做的事"对齐其"不做的事" |
| [`requirements.md`](../requirements.md) | 需求事实来源；本文档不引入新需求，只落实其约束 |
| [`docs/implementation/`](../implementation/) | 实施建议；所有具体 API 签名、工具契约、流程图在此 |
| [`docs/process/`](../process/) | 实施流程；新增 flag / lint 规则 / internal 包 / frontmatter 迁移的决策树 |
| `AGENTS.md` / `.cursor/rules/*` | 本文档的镜像；本文档更新后回写同步 |
