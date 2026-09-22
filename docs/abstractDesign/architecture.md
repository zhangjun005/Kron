# 架构设计（Architecture）

> 本文档是 Kron v1 的架构真理源。`AGENTS.md` 与 `.cursor/rules/*` 是它的镜像。
> 风格沿用 [`intent-structure.md`](./intent-structure.md)：铁律 + 字段表 + 示例。
> 措辞保持建议性，不锁死实现细节。

---

## 〇、铁律（架构不可妥协）

1. **业务逻辑只在 `internal/`**；`cmd/kron/` 只做 wiring。任何在 `cmd/` 写 `for` 或 `if err != nil` 的冲动都要移走。
2. **底层与访问层解耦**：`internal/store`、`internal/parser`、`internal/model` 是纯函数库，不感知调用方是 CLI / MCP / LSP / IDE / GUI。所有访问层都位于 `cmd/kron/<sub>/`（如 `cmd/kron/cli`、`cmd/kron/serve-mcp`），**不**再使用 `internal/cli/` 或 `internal/gui/` 之类的访问层包名。
3. **访问层之间禁止互相调用**（新增）：CLI / MCP / LSP / IDE / GUI 五种访问层之间**不能**直接 import 或调用彼此。访问层需要跨入口复用的能力（如 IDE 复用 LSP）必须**下沉到 `internal/`**实现，再由各访问层独立调用。任何"访问层调用访问层"的代码都是架构违规。
4. **调用方身份走 `context.Context`**：所有需要区分调用方的函数签名第一个参数必须是 `ctx context.Context`，身份 key 由调用层注入。
5. **零新依赖**：与 `AGENTS.md` 一致；任何新依赖必须带 commit body 论证。
6. **数据真理源是仓库内的 `.md`**：二进制不持有项目数据；不维护状态机、不做双源同步。
7. **CI lint 是唯一可信门禁**：运行时不维护隐式状态；强约束通过 `kron lint` 在 CI 报错实现，不靠运行时默认值兜底。
8. **依赖方向单向**：`cmd → {cli, serve-mcp, ...} → {store, parser, lint} → model`；严禁反向；`model` 零外部依赖。

---

## 〇·五、抽象层次定义（访问层之间关系的论证）

### 〇·五·1 三个抽象层次

| 层 | 中文 | 职责 | 仓库位置 | 谁**不**能访问它 |
|---|---|---|---|---|
| **对象层（Domain）** | 对象 | 表示 `.kron/intents/*.md` 的数据结构；最少行为；零外部 import | `internal/model/` | 任何东西——它只能被 import，不能 import 任何东西 |
| **业务层（Business）** | 业务 | 文件 I/O、YAML 解析、锚点扫描、锚点校验；操作对象层；不知道谁来调用、不知道进程模型 | `internal/store/`, `internal/parser/`, `internal/lint/`, ... | 访问层**不**被业务层反向 import（`internal/` 永远不 import `cmd/kron/`） |
| **访问层（Access）** | 访问 | 协议适配（cobra / JSON-RPC / LSP / HTTP）、参数解析、输出序列化、组装业务层函数 | `cmd/kron/<sub>/`（`cli/`、`serve-mcp/`、`serve-gui/` ...） | 业务层**不**被要求感知访问层；其他访问层**绝不**被 import |

> **不是**"业务逻辑层 vs 访问层"二层——加对象层是因为它解决"什么是数据、什么是规则、什么是协议"这个本质问题。
> 没对象层做参照，业务层和访问层的边界会塌缩成"反正都在 internal"。

### 〇·五·2 访问层之间禁止互相调用：三条理由

**A. 关注点分离（哲学层面）**——每个访问层有自己的协议、生命周期、失败语义：

| 访问层 | 协议 | 生命周期 | 失败语义 |
|---|---|---|---|
| CLI | 命令行 argv / stdout / stderr | 一次 fork-exit（百毫秒级） | POSIX exit code + stderr |
| MCP | stdio JSON-RPC 2.0 | 长连接（响应 keepalive） | JSON-RPC error code |
| LSP | socket + 自定 frame 协议 | 长连接（响应 alive check） | LSP `window/showMessage` |
| IDE 扩展 | VSCode / Cursor 扩展 API | 长连接（编辑器进程寿命） | VSCode `window.showErrorMessage` |
| GUI | HTTP / WebSocket | 长连接（浏览器 tab 寿命） | HTTP status code + JSON |

复用 LSP 给 IDE 实现"hover 跳转"，比让 IDE 直接调 `internal/parser` 难 10 倍：要协商
Position encoding、文档同步协议、capabilities。下沉到 `internal/` 是"算法共用"；
保留独立访问层是"协议隔离"——两条目标只能各占一边，混搭必塌。

**B. 进程模型（物理层面）**——不同访问层寿命错配：

- CLI：一次性 fork-then-exit，**不**承载长连接逻辑
- MCP / LSP / GUI：长连接，**不**能接受"每条命令重新协商协议"
- 一个寿命 100ms 的 CLI 进程若承载 LSP 逻辑：进程边界模糊、升级兼容性差
  （升级 LSP 协议要重编 CLI）、部署方式打架（一个二进制 vs 多个服务）

**C. 升级和测试的爆炸半径（实操层面）**——以"IDE 调 LSP 实现 hover 跳转"为例：

```
方案 X：IDE 依赖 LSP 进程
  → LSP 协议从 3.17 升 3.18 后，IDE 必须重新发布、重新打包

方案 Y：IDE 只依赖 internal/parser
  → internal/parser 抽象不变（hover = 给 slug → 给 Anchor → 给 Location），
    LSP 升级到 3.18、IDE 升级、两者独立打包
```

"被依赖方的 ABI 稳定性 = 内部包"——这条经验法则决定我们必须先下沉，再分头打包。

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
| `internal/lint` | 业务层：扫描 + 校验组合（两个以上访问层用到时才开） | 当 CLI、MCP、IDE 都需要 lint 时开 |
| `internal/...` | **不**预设更多 | 每个新 `internal/` 子包都要写"为什么开"的 commit 论证 |

---

## 一、访问层全景

Kron 的核心是**数据格式 + 协议**，不是一个二进制。同一份仓库内 `.md` 数据有五种访问入口：

| 入口 | 触达路径 | 用户 | 核心场景 | 共享底层 | v1 状态 |
|---|---|---|---|---|---|
| **CLI** | 终端直接执行 | 人类 / 脚本 / CI | 初始化、批处理、CI 门禁 | `internal/store`、`internal/parser` | ✅ 交付 |
| **MCP server** | stdio JSON-RPC | AI Agent | 起草 / 检索 / 校验意图 | `internal/store`、`internal/parser` | ✅ 交付 |
| **LSP server** | 编辑器子进程 | 人类（在 IDE 中） | Hover、`Ctrl+单击` 跳转 | `internal/store`、`internal/parser` | ❌ Phase 2 |
| **IDE 插件** | VSCode / Cursor 扩展 | 人类（在 IDE 中） | 锚点高亮、承载 LSP / MCP | **直接**调 `internal/parser`、`internal/store`；**不** import LSP server | ❌ Phase 2 |
| **GUI** | Web 应用 | 人类（视觉化） | 意图树、关系图、可视化编辑 | 通过独立 GUI API 边界访问（见 §5.4.2） | ❌ Phase 2 |

> **访问层之间禁止互相调用**（§〇 铁律 #3 + §〇·五·2 三条理由）。
> 表里"复用 LSP"这一栏的含义是"访问相同底层 `internal/`"，**不是**"IDE 调 LSP"。
> 每种访问层独立 import `internal/`，不依赖其他访问层。Phase 2 的入口（IDE / LSP / GUI）
> 仓库结构未开，先把规则定死，避免 Phase 1 代码里出现"为 Phase 2 留耦合"。

### 1.1 CLI 最小集

MCP / IDE / GUI 覆盖查询、浏览、编辑场景后，CLI 只留"终端独有的"命令：

| 命令 | 用途 | 不放给 MCP / GUI 的原因 |
|---|---|---|
| `kron init` | 创建 `.kron/intents/` 目录、初始化 `config.toml` | — |
| `kron add <slug>` | 起草新意图文件（写入 `.kron/intents/<slug>.md`） | — |
| `kron lint` | CI 门禁：扫描锚点悬空、frontmatter 校验 | 必须可被 GitHub Actions 直接调用 |
| `kron serve-mcp` | 启动 MCP stdio server（v1 交付） | — |

> **不在 CLI 中实现**：`list`、`get`、`update`、`delete`、`restore`——由 MCP 工具集（§5.4.1）覆盖。
> CLI 是 MCP 工具集的**最小超集**：只留人在终端直接敲命令时真正用得上的子集。

### 1.2 MCP 工具集（覆盖增删改查 + 校验 + 软删除恢复）

MCP 工具集覆盖意图**完整生命周期**：

| 类别 | 工具 |
|---|---|
| 初始化 | `kron_init` |
| 增 | `kron_add` |
| 查 | `kron_list`、`kron_get` |
| 改 | `kron_update` |
| 删（软） | `kron_delete` |
| 恢复 | `kron_restore` |
| 校验 | `kron_lint` |

完整入参 / 出参定义见 §5.4.1。AI Agent 通过这套工具标准化访问 Kron，
人类用户在终端用 CLI 子集，GUI 用同一套底层函数（§5.4.2）。

---

## 二、包结构与依赖方向

```
cmd/kron/                        ← 入口，只 dispatch 子命令，不含业务逻辑
├── main.go                      ← thin entry, 仅 import 访问层子包
├── cli/                         ← 访问层：cobra 命令（init / add / lint）
└── serve-mcp/                   ← 访问层：MCP stdio server（v1 交付）

internal/                        ← 只放底层包，禁止 import 任何访问层
├── model/                       ← 领域类型，零外部依赖
├── store/                       ← 文件 I/O、frontmatter 读写；仅依赖 model
└── parser/                      ← 输入解析（CLI 参数、锚点扫描、相对链接识别）；仅依赖 model
```

> **访问层全部位于 `cmd/kron/` 之下**，不再有 `internal/cli/`。CLI / MCP / LSP / IDE / GUI 五种访问层
> 都作为 `cmd/kron/<sub>/` 子包存在，每个独立 go module 级 import。

### 2.1 依赖图

```
                      ┌─► cmd/kron/cli/         ───► internal/store/  ─► model
                      │
cmd/kron/main.go ─────┼─► cmd/kron/serve-mcp/   ───► internal/parser/ ─► model
                      │
                      └─► (Phase 2) cmd/kron/serve-gui/ 等
                          ──────────────────────► internal/lint/    (按需新增)
```

- `model`：纯数据结构与方法，零外部 import。
- `store`：文件系统操作 + YAML 序列化。**不**感知 CLI/MCP/LSP。
- `parser`：字符串解析、锚点扫描、相对链接识别。**不**做文件 I/O（输入由调用方传入）。
- `lint`（当开时）：扫描 + 校验组合逻辑；**不**感知任何访问层（CLI / MCP / IDE 都各自 import 它）。
- `cmd/kron/*`：编排 store / parser / lint；**只** import `internal/*`，**不** import 其他 `cmd/kron/*`。

### 2.2 访问层 import 规则（铁律 #3 的落地形式）

访问层之间的 import 关系**只能**是：`访问层 → internal/*`，**不**允许 `访问层 → 访问层`。

```go
// ✅ 正确：访问层（CLI 子命令）只 import internal/
// cmd/kron/serve-mcp/main.go
import (
    "github.com/xxx/kron/internal/store"
    "github.com/xxx/kron/internal/parser"
)

// ❌ 违规：访问层之间互相 import
// cmd/kron/serve-ide/main.go
import (
    "github.com/xxx/kron/cmd/kron/serve-lsp"   // 严禁
)

// ❌ 违规：在 internal/ 里反向 import 访问层
// internal/parser/foo.go
import (
    "github.com/xxx/kron/cmd/kron/serve-mcp"   // 严禁
)
```

判定清单（CI 可机械检查）：
- `cmd/kron/**` 只能 import `cmd/kron/**` 自身 + `internal/**` + 标准库 + 已批准的第三方包
- `internal/**` 只能 import `internal/**` 自身 + 标准库 + 已批准的第三方包；**不** import `cmd/kron/**`
- 任意 `cmd/kron/<sub>/` 只能 import `cmd/kron/<sub>/` 自身 + `internal/**`，**不** import 其他 `cmd/kron/<other>/`

### 2.3 调用方身份透传

所有 store / parser 函数签名的第一个参数都是 `ctx context.Context`：

```go
// ✅ 正确：身份走 context
func (s *Store) WriteIntent(ctx context.Context, slug string, intent *model.Intent) error

// ❌ 错误：身份作为普通参数
func (s *Store) WriteIntent(caller string, slug string, intent *model.Intent) error
```

身份 key 约定：`caller = "cli"` / `"mcp"` / `"lsp"` / `"ide"` / `"gui"`。MCP 透传时可附带子标识，如 `"mcp:claude-3.7"`。

身份仅用于：
- lint 输出里标注调用方
- 未来审计日志（v1 不实现，但接口要留）

身份**不**用于权限控制——`add` 是文件写入，不存在"高安全需求"。

---

## 三、领域模型骨架

### 3.1 `Intent`

```go
// Intent is one design intent, persisted as .kron/intents/<slug>.md.
type Intent struct {
    Slug        string      // 文件相对路径（相对 .kron/intents/），不含 .md，例如 "auth/jwt-sliding-window"
    Frontmatter Frontmatter
    Body        string      // Markdown 正文（不含 frontmatter）
    SourcePath  string      // 磁盘绝对路径，加载/解析时由 store 层注入，**不**参与落盘序列化
}
```

### 3.2 `Frontmatter`

```go
type Frontmatter struct {
    Symbol    []string `yaml:"symbol,omitempty"`     // 关联代码符号列表
    CreatedBy string   `yaml:"created_by"`          // "@user" 或 "agent:<model>"
    UpdatedAt string   `yaml:"updated_at"`          // ISO 8601
    Reviewers []string `yaml:"reviewers,omitempty"` // 可选
    Status    Status   `yaml:"status,omitempty"`    // draft / active / superseded
}
```

### 3.3 `Status` 枚举

```go
type Status string

const (
    StatusDraft      Status = "draft"
    StatusActive     Status = "active"
    StatusSuperseded Status = "superseded"
)
```

**完全可选，不填 = 不参与生命周期管理**（参见 `intent-structure.md` §三）。

### 3.4 `Anchor`

```go
// Anchor is a parsed // @kron:intent <slug> annotation.
type Anchor struct {
    Slug       string  // 意图路径，不含 .md
    FilePath   string  // 锚点所在源文件路径
    LineNumber int     // 锚点所在行（1-indexed）
}
```

### 3.5 `Config`

```go
type Config struct {
    IntentsDir string `toml:"intents_dir"` // 默认 ".kron/intents"
}
```

配置文件位于 `.kron/config.toml`，**缺失即用默认值**，不存在即不报错。

> **v1 不增加任何配置字段**。`default_reviewer` / `lint_rules` / 其他扩展一律推迟——
> 99% 的用户不会改 `intents_dir`，零配置可用（Zero-config）才是高级的极简。

---

## 四、错误模型

### 4.1 Sentinel 错误清单

```go
var (
    ErrIntentNotFound      = errors.New("intent not found")
    ErrIntentExists        = errors.New("intent already exists")
    ErrAnchorDangling      = errors.New("anchor points to non-existent intent")
    ErrFrontmatterInvalid  = errors.New("frontmatter is invalid")
    ErrSlugInvalid         = errors.New("intent slug is invalid")
    ErrConfigInvalid       = errors.New("config file is invalid")
)
```

### 4.2 包装与判别

- 所有错误用 `fmt.Errorf("...: %w", err)` 包装，附带操作上下文
- 调用方用 `errors.Is(err, store.ErrIntentNotFound)` 判别
- 永不 `==` 比较错误

---

## 五、关键流程

### 5.1 `kron init`

```
探测 .kron/ 是否存在
  ├─ 存在 → 提示已初始化，退出
  └─ 不存在 → mkdir .kron/intents/ + 写 config.toml（默认值）
```

实现位于 `cmd/kron/cli/init.go`，调用 `store.EnsureKronDir(ctx, cfg)`。

### 5.2 `kron add <slug>`

```
接收第一个位置参数 args[0] 作为 slug
  ├─ 缺失或多余参数 → 返回 ErrSlugInvalid
  └─ 存在 →
      正则校验 slug：^[a-z0-9]+(-[a-z0-9]+)*(/[a-z0-9]+(-[a-z0-9]+)*)*$
        ├─ 不匹配 → 返回 ErrSlugInvalid
        └─ 匹配 →
            检查 .kron/intents/<slug>.md 是否已存在
              ├─ 存在 → 返回 ErrIntentExists
              └─ 不存在 →
                  构造模板 frontmatter（created_by 来自 git config user.name）
                  写入文件
```

实现位于 `cmd/kron/cli/add.go`，调用 `store.WriteIntent(ctx, slug, &intent)`。

> **v1 不支持 stdin / 外部模板**。`kron add` 的职责是快速脚手架（Scaffold）——
> 写入默认骨架后，开发者或 AI 直接打开 `.md` 编辑。复杂输入拼接留给编辑器或 GUI。

### 5.2.1 软删除与恢复（MCP 独占，CLI 不暴露）

```
kron_delete <slug>：
  校验 .kron/intents/<slug>.md 存在
    ├─ 不存在 → 返回 ErrIntentNotFound
    └─ 存在 →
        mkdir -p .kron/.trash/
        移动文件：.kron/intents/<slug>.md → .kron/.trash/<slug>.md
        返回 { ok: true, trashed_path: ".kron/.trash/<slug>.md" }

kron_restore <slug>：
  校验 .kron/.trash/<slug>.md 存在
    ├─ 不存在 → 返回 ErrIntentNotFound
    └─ 存在 →
        移动文件：.kron/.trash/<slug>.md → .kron/intents/<slug>.md
        返回 { ok: true, path: ".kron/intents/<slug>.md" }
```

实现位于 `internal/store/trash.go`，CLI **不**提供 `kron delete` / `kron restore` 子命令。

### 5.3 `kron lint`

```
默认从当前工作目录（CWD）开始递归扫描所有源码文件
  跳过目录黑名单（return filepath.SkipDir）：
    .git、.kron、node_modules、vendor、dist、bin、target、.idea、.vscode
对每个源码文件 → parser.ScanAnchors → 收集 []Anchor
对每个 Anchor → store.ResolveIntent(slug) → 校验文件存在
  └─ 不存在 → 输出 [error] anchor dangling + 计入 errors
遍历 .kron/intents/**/*.md → store.LoadAll → 校验 frontmatter 必需字段
  └─ 缺失 → 输出 [error] frontmatter invalid + 计入 errors

退出码语义：
  0 = 通过（无错误）
  1 = 有 lint 错误
  2 = 内部失败（IO / 解析异常）
输出格式支持 --reporter=text|json，便于 CI 集成。
```

> **v1 不暴露 `--path` 自定义扫描根**。全仓扫描 + 硬编码黑名单是 v1 的全部策略；
> 真实需求出现时再加 flag。

### 5.4 MCP / LSP 复用边界

| 复用层 | v1 是否交付 | 方式 |
|---|---|---|
| MCP server (`kron serve-mcp`) | ✅ v1 交付 | stdio JSON-RPC；`cmd/kron/serve-mcp/` 子命令做序列化，业务逻辑直接调 `internal/store` 与 `internal/parser` 包 |
| LSP server | ❌ Phase 2 | LSP 协议过于笨重（文件同步、Position 偏移、生命周期、AST 解析），v1 不碰 |
| IDE 插件 | ❌ Phase 2 | 宿主 LSP / MCP 进程；自身**不** import LSP/MCP 代码。锚点扫描/意图加载等业务逻辑**直接**调 `internal/parser` 与 `internal/store` |
| GUI 客户端 | ❌ Phase 2 | 通过独立的 GUI API 边界访问（见 §5.4.2），不依赖 CLI 的 `--json` flag，不依赖任何其他访问层 |

> **访问层之间禁止互相调用**（§〇 铁律 #3）。表中所有"复用"行的含义都是
> "共享同一个 `internal/` 业务逻辑"，不是"这一层调另一层"。例如：IDE 插件**不** import
> LSP server，而是与 LSP 各自独立 import `internal/parser` / `internal/store`。

#### 5.4.1 MCP 工具契约（v1）

| 工具 | 入参 | 出参 | 类别 |
|---|---|---|---|
| `kron_init` | 无 | `{ ok: bool, intents_dir: string }` | 初始化 |
| `kron_add` | `slug` (string, required)<br>`symbol` (string, optional)<br>`why` (string, optional) | `{ ok: bool, path: string }` | **增** |
| `kron_list` | `prefix` (string, optional) | `{ intents: IntentSummary[] }` | **查** |
| `kron_get` | `slug` (string, required) | `{ intent: Intent }` | **查** |
| `kron_update` | `slug` (string, required)<br>`symbol` (string, optional)<br>`body` (string, optional)<br>`status` (string, optional) | `{ ok: bool, path: string }` | **改** |
| `kron_delete` | `slug` (string, required) | `{ ok: bool, trashed_path: string }` | **软删** |
| `kron_restore` | `slug` (string, required) | `{ ok: bool, path: string }` | **恢复** |
| `kron_lint` | 无 | `{ passed: bool, errors: string[] }` | **校验** |

MCP 必须覆盖**增删改查 + 校验**完整意图生命周期。每个工具的底层都映射到同一个
`internal/store` 或 `cmd/kron/cli` 函数，签名带 `ctx context.Context`，caller 由
MCP server 注入为 `"mcp:<agent>"`。

##### 软删除机制

- **`kron_delete`** 不真正删除文件，而是把 `.kron/intents/<slug>.md` 移到 `.kron/.trash/<slug>.md`
- `.kron/.trash/` 与 `.kron/intents/` 平级，是软删除的"暂存区"
- **`kron_restore`** 把 `.kron/.trash/<slug>.md` 移回 `.kron/intents/<slug>.md`
- 软删除给"误删"留出悔过窗口；硬删除（彻底删 `.trash/` 内容）留给用户手动 `git rm` 或后续 `kron gc` 子命令（v1 不实现）

> CLI 子集是 MCP 工具集的**最小超集**：`kron init` / `kron add` / `kron lint`；
> 其余 `list` / `get` / `update` / `delete` / `restore` 只在 MCP 暴露，CLI 不重复实现。

#### 5.4.2 GUI API 边界（Phase 2 预留）

GUI 不走 CLI 的 `--json` flag，而是通过**单独的 API 包**消费同一份 `internal/store` /
`internal/parser`。Phase 2 实现时建议形态：

- **HTTP server 子命令**（`kron serve-gui`）启动轻量 HTTP 服务，对外暴露 REST 端点
- 端点对应 §5.4.1 的 MCP 工具集（`POST /intents` / `GET /intents` / `PATCH /intents/:slug` / `DELETE /intents/:slug` / `GET /lint`）
- 共享同一个 `cmd/kron/cli` 函数库（与未来 `cmd/kron/serve-gui`），访问层之间通过 `internal/` 解耦

> v1 不实现 `serve-gui`，但 `cmd/kron/cli` 的函数签名要为它留口（已经是 `ctx` 透传形态）。

---

## 六、测试策略

| 层 | 策略 | 工具 |
|---|---|---|
| `model` | 纯单元 | `testing` + `testify/assert` |
| `store` | tempdir 集成（`t.TempDir()`） | 同上 |
| `parser` | 表驱动 | 同上 |
| `cli` | cobra `cmd.Execute()` 端到端 | 同上 |

- 测试 fixture 放在 `testdata/`（git 跟踪）
- 集成测试覆盖完整 `kron init → add → lint` 闭环
- 不引入 race detector 之外的额外测试框架

---

## 七、不做的事（v1 范围外）

- 守护进程、文件监听、状态机、双源同步
- CLI `list` / `get` / `update` / `delete` / `restore`（由 MCP 工具集覆盖；CLI 不重复实现）
- `parent` / `depends_on` 拓扑建模（目录 + 相对链接已足够）
- 健康度诊断：过期 / 孤儿 / 冲突检测（`requirements.md` 未明确要求）
- 导入迁移、`config.toml` 字段扩展（v1 仅 `intents_dir`）
- `kron add` 的 stdin / 外部模板支持
- `kron lint` 的 `--path` 自定义扫描根（全仓 + 黑名单足够）
- `serve-lsp`、IDE 插件、GUI 客户端（明确推迟到 Phase 2）
- AI 起草 → 入库的完整工作流（v1 后另议；MCP 已为它留接口）

---

## 八、演进方向（v1 不实现，接口要留）

| 方向 | 留口方式 |
|---|---|
| AI 起草工作流 | MCP 工具集已覆盖完整生命周期；store 接口走 ctx，可携带 caller 身份 |
| GUI 客户端 | 独立的 GUI API 边界（见 §5.4.2），不复用 CLI `--json` flag |
| 健康度诊断 | `kron lint` 子命令可扩展 lint 规则集 |
| 多编辑器 LSP 复用 | LSP server 与 CLI 解耦，可独立打包 |

---

## 与其他文档的关系

| 文档 | 关系 |
|---|---|
| [`intent-structure.md`](./intent-structure.md) | 数据格式真理源；本文档引用其 frontmatter schema 与锚点语法 |
| [`tech-stack.md`](./tech-stack.md) | 技术栈依据；本文档的包结构基于其选型 |
| [`business.md`](../business.md) | 业务边界；本文档的"不做的事"对齐其"不做的事" |
| [`requirements.md`](../requirements.md) | 需求事实来源；本文档不引入新需求，只落实其约束 |
| `AGENTS.md` / `.cursor/rules/*` | 本文档的镜像；本文档更新后回写同步 |
