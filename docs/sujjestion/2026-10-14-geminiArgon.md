对于 **Kron** 而言，**不需要引入额外的“常驻后台守护进程（Daemon）”**，维持当前的 **单一 Go 二进制 + 子命令分发（`cli` / `mcp` / `lsp` 各自独立进程）** 是最合理的架构；但围绕 `.kron/intents/` 与源码锚点的**进程内缓存同步与跨进程文件并发机制**，有 4 个关键点值得改进。

---

### 一、 为什么 Kron 不需要做“上层常驻 Daemon”？

在 Kron 的架构中（`cmd → cli → {parser, store} → model`），核心数据源是本地文件系统中的 **`.kron/intents/` Markdown 文件树** 和 **源码中的 `// @kron:intent <slug>` 注释锚点**。

如果为了连接 `kron cli`、`kron mcp` 和 `kron lsp` 而额外抽离一个系统级常驻 Daemon（通过 Socket 通信），弊大于利：

1. **Go 进程本身足够轻量**：`kron mcp` 和 `kron lsp` 分别作为编辑器与 AI 客户端的 `stdio` 子进程常驻时，单个 Go 进程内存占用仅 **10MB ~ 25MB**，两个进程并存毫无内存压力。
2. **数据规模处于毫秒级区间**：一个中型代码库的 `.kron/intents/` 通常只有几十到几百个 Markdown 文件，Go 标准库读取并解析 Frontmatter 的耗时在 **1ms ~ 5ms** 以内，远低于大模型自身的网络推理延迟。
3. **避免守护进程运维灾难**：引入独立 Daemon 会带来跨平台 IPC 兼容（Windows Named Pipe vs Unix Socket）、僵尸进程清理、以及二进制升级后 Daemon 版本与 CLI 版本不一致等工程包袱，违背了“以文件系统为单一事实来源（Filesystem as Truth）”的轻量化初衷。

---

### 二、 Kron 进程机制真正需要改进的 4 个地方

虽然不需要改造成多进程主从架构，但由于 **CLI（短命进程）、LSP（常驻进程）、MCP（常驻进程）以及用户/AI 直接改文件** 会同时操作同一份本地文件，以下 4 个机制需要重点加固：

#### 1. 区分对待两类数据源的缓存策略（解决跨进程“脏读”与全量扫描开销）

`kron mcp` 是长驻进程，如果完全不缓存，每次调用 Tool 都全量掃描整个工程源码找 `// @kron:intent` 会随代码库膨胀变慢；如果启动时全量读入内存不再更新，用户用 `kron add` 或编辑器改了文件，MCP 进程就会读到过期状态。

* **针对 `.kron/intents/*.md`（意图定义层）—— `mtime` 轻量校验缓存**：
不需要复杂的后台线程。在 `store` 层维护一个 `map[filepath]ModTime` 内存缓存，每次 MCP Tool / Resource 被调用时，通过 `os.Stat()` 检查文件修改时间，未变更则直接返回内存中的 `model`，有变更才重新解析对应 Markdown。
* **针对源码 `// @kron:intent`（代码锚点层）—— 内存倒排索引 + `fsnotify` 增量刷新**：
全量正则/语法扫描代码库开销较大。建议在 `kron mcp` 和 `kron lsp` 启动 `initialized` 后，后台起一个 goroutine 通过 `fsnotify` 监听工作区源码保存事件（过滤 `.git`、`node_modules`、`target` 等），文件落盘时异步增量更新该文件的锚点映射表，让 MCP 查询始终走纯内存 O(1) 检索。

#### 2. `store` 层的跨进程原子写入（防止 MCP 与 CLI/LSP 读写冲突）

Go 语言里的 `sync.RWMutex` 只能锁住 `kron mcp` 自己进程内部的协程，无法阻止用户在终端运行 `kron lint/add` 或 AI 同时修改文件。如果 `store` 直接用 `os.WriteFile` 覆盖写入，另一个进程恰好在此时读取，可能读到写了一半的残缺 Frontmatter。

* **改进方案（Write-to-Temp + Atomic Rename）**：
在 `store` 写入 `.kron/intents/` 下的文件时，先写入同目录下的临时文件（如 `.kron/intents/.tmp-<slug>.md`），调用 `f.Sync()` 确保落盘后，再执行 `os.Rename(tmpPath, targetPath)`。操作系统内核保证 `rename` 是原子的，任何时刻 `kron lsp` 或 `kron mcp` 读到的要么是旧文件，要么是完整的新文件，绝不会解析报错。

#### 3. 接入 MCP `roots` 协议，解耦进程启动目录（`cwd`）依赖

很多客户端（如 Claude Desktop 或全局配置的 Cursor MCP）拉起 `kron mcp` 子进程时，进程的当前工作目录（`os.Getwd()`）不一定是当前打开的项目根目录，可能导致 `store` 找不到 `.kron/` 文件夹。

* **改进方案**：
1. 优先支持命令行显式传入 `--workspace /path/to/repo`。
2. 在 MCP 握手完成后，主动向 Client 请求 `roots/list`，提取 `file://` URI 作为 `.kron/` 的查找根路径，并监听 `notifications/roots/list_changed` 实现多工作区切换。



#### 4. 严格隔离 `stdio` 通道与 `context.Context` 级联退出

在 `cmd → cli → {parser, store}` 共享底层包的架构下：

* **禁止底层包直接打印标准输出**：`parser` 和 `store` 在被 CLI 调用时习惯输出提示信息，但被 `kron mcp` 或 `kron lsp` 调用时，任何写往 `os.Stdout` 的非 JSON-RPC 字节都会直接导致客户端协议断连。所有内部日志必须注入 `io.Writer` 并统一指向 `os.Stderr` 或 `.kron/kron.log`。
* **父进程崩溃自清理**：确保 `kron mcp` 将 `os.Stdin` 的 `EOF` 信号以及 `SIGTERM`/`SIGINT` 绑定到根 `context.Context` 上；一旦宿主 IDE 强行关闭，后台的 `fsnotify` watcher 和未完成的任务能立即通过 `ctx.Done()` 安全退出，不在系统后台残留孤儿进程。

---

### 三、 架构演进对照表

| 维度 | 当前/基础形态 | 推荐改进形态（适合 Kron 现阶段） | 独立 Daemon 形态（暂不推荐） |
| --- | --- | --- | --- |
| **进程模型** | `mcp` / `lsp` / `cli` 独立进程 | **保持独立进程**（单一二进制多子命令） | 后台常驻 Daemon + 多个薄 Client |
| **Intents 读取** | 每次全量读盘 或 静态内存 | **`os.Stat(mtime)` 校验 + 按需增量重解析** | 集中式内存状态机 |
| **代码锚点检索** | 每次调用扫盘 (`filepath.Walk`) | **启动异步建索引 + `fsnotify` 增量更新** | 后台常驻索引服务 |
| **并发写入安全** | 直接覆盖写入 (`os.WriteFile`) | **临时文件 + `os.Rename` 原子替换** | 进程间 RPC 串行化写入 |
| **定位工作区** | 依赖进程 `os.Getwd()` | **`--workspace` 参数 + MCP `roots/list` 协议** | 客户端注册工作区路径 |


**整体改动代价非常小，完全不需要推翻现有的 `cmd → cli → {parser, store} → model` 分层架构。**

除了第 4 项（如果引入 `fsnotify`）需要加一个标准第三方库外，**前 3 项全部只用 Go 标准库就能完成，核心改动代码量加起来不到 200 行**，最快一两个小时就能全部落地。

下面是按**投入产出比（ROI）从高到低**排列的具体改动拆解与代码量评估：

---

### 1. 原子写入与 `stdio` 隔离（代价：⭐ 极低 | 约 20~30 行代码 | 15 分钟）

完全不动接口签名，只改 `store` 内部写文件的实现和日志输出流：

* **写文件改原子替换**（在 `store` 包内替换原有的 `os.WriteFile`）：
```go
func atomicWriteFile(targetPath string, data []byte) error {
    dir := filepath.Dir(targetPath)
    // 在同目录下创建临时文件，确保和目标文件在同一个挂载分区（保证 Rename 原子性）
    tmpFile, err := os.CreateTemp(dir, ".kron-tmp-*")
    if err != nil {
        return err
    }
    tmpPath := tmpFile.Name()
    defer os.Remove(tmpPath) // 失败时兜底清理

    if _, err := tmpFile.Write(data); err != nil {
        tmpFile.Close()
        return err
    }
    if err := tmpFile.Sync(); err != nil {
        tmpFile.Close()
        return err
    }
    if err := tmpFile.Close(); err != nil {
        return err
    }
    return os.Rename(tmpPath, targetPath)
}

```


* **输出流隔离**：全局搜索 `parser` 和 `store` 里的 `fmt.Print` / `log.Print`，在 `mcp` 和 `lsp` 入口的 `init` 或 `main` 中加一行 `log.SetOutput(os.Stderr)` 即可。

---

### 2. `.kron/intents/` 的 `mtime` 缓存校验（代价：⭐⭐ 低 | 约 50~70 行代码 | 半小时）

不需要引入任何后台协程，也不改变 `store` 对外暴露的方法，只需在 `Store` 结构体里加一个带读写锁的缓存字典：

```go
type cachedIntent struct {
    mtime  time.Time
    intent *model.Intent
}

type Store struct {
    mu    sync.RWMutex
    cache map[string]cachedIntent
    // ... 原有字段
}

```

* **读取逻辑**：每次调用 `Get` 或 `List` 时，先对文件做一次极轻量的 `os.Stat(path)`（耗时微秒级），对比 `info.ModTime().Equal(cached.mtime)`。如果一致，直接返回内存里的 `*model.Intent`；不一致才读盘解析并更新 `cache`。
* **破坏性**：**0**，上层 `cli`、`mcp`、`lsp` 完全无感。

---

### 3. 接入 `--workspace` 与 MCP `roots`（代价：⭐⭐ 低 | 约 40~60 行代码 | 半小时）

如果目前你的 `store` 和 `parser` 是通过传入 `rootDir string` 初始化的，那改动只集中在 `cmd/mcp` 入口层：

1. **保底方案（5 分钟）**：给 `kron mcp` 子命令加一个 `--workspace` flag，如果没传再退化到 `os.Getwd()`。用户在配置 MCP JSON 时直接传入工作区路径参数即可解决 90% 的路径错位问题。
2. **协议级方案（20 分钟）**：在 MCP Server 的 `OnInitialized` 回调里调一次客户端的 `ListRoots(ctx)`，把拿到的第一个 `file://` URI 用 `url.Parse` 转成本地路径，动态赋给 `store.SetRoot(path)`。

---

### 4. 代码锚点（`// @kron:intent`）的缓存与增量更新（代价：⭐⭐⭐ 中 | 约 100~150 行代码）

这一项是唯一稍微花点精力的地方，但可以**分两步走（渐进式演进）**：

* **阶段 A（零外部依赖，耗时 30 分钟）**：
**不引入 `fsnotify**`！直接复用上面第 2 点的思路：记录工程内源码文件的 `filepath -> mtime` 映射表。
当 MCP 请求查询锚点时，快速遍历一次文件树但**只做 `os.Stat` 不打开文件**，只有 `mtime` 变了的源码文件才送进 `parser` 重新扫描 `// @kron:intent`。对于几百个文件的项目，`os.Stat` 遍历只需几毫秒，直接省掉 99% 的磁盘读取和正则匹配开销。
* **阶段 B（后续项目大了再做，耗时 1~2 小时）**：
等未来管理上万文件的超大型仓库时，再引入 `[github.com/fsnotify/fsnotify](https://github.com/fsnotify/fsnotify)`，在后台挂一个监听协程。因为 Windows/Linux/macOS 对递归目录监听的行为有差异，这部分调试成本最高，**现阶段完全可以先用“阶段 A”平替**。

---

### 总结：建议推进顺序

| 优先级 | 改动项 | 耗时估计 | 是否需要改架构/加依赖 |
| --- | --- | --- | --- |
| **P0（立刻可改）** | `log.SetOutput(os.Stderr)` + `--workspace` 参数支持 | **10 分钟** | 否 |
| **P1（顺手加固）** | `store` 写入改为 `CreateTemp` + `os.Rename` | **15 分钟** | 否 |
| **P2（性能提升）** | `store` 与 `parser` 加入基于 `os.Stat(mtime)` 的按需解析缓存 | **40 分钟** | 否 |
| **P3（按需演进）** | MCP `roots/list` 动态切换 & `fsnotify` 后台监听 | **1~2 小时** | 仅 `fsnotify` 需加依赖 |

按 **P0 → P1 → P2** 改下来，总共也就 **1 小时左右的工作量**，不增加任何第三方依赖，就能把本地多进程并发读写和重复扫盘的隐患彻底解决掉。


**强烈建议做，而且对于 Kron 来说，Prompts 是性价比极高、最能体现“意图驱动（Intent-Driven）”核心价值的模块。**

原因在于：**大模型天然认识 Git 和文件操作，但它天生不认识 Kron 的规范**（比如 `.kron/intents/` 的目录层级、Frontmatter 格式要求、以及代码里的 `// @kron:intent <slug>` 锚点语法）。

如果只暴露 **Tools** 和 **Resources**，你每次在 Cursor 或 Claude Code 里开始干活时，都得手动敲一段话教 AI：“*先去读一下某意图，改代码的时候记得加上 `@kron:intent` 注释，改完后再更新状态……*”。而做了 **Prompts**，这些工作流直接变成编辑器里的斜杠命令（Slash Commands），一键就能把「操作规程 + 意图上下文 + 关联源码位置」打包喂给 AI。

---

### 一、 为什么 Kron 特别需要 Prompts？（解决 3 个核心痛点）

1. **省去重复的“规则灌输”**：
不需要污染全局的 `.cursorrules` 或 `CLAUDE.md`，只在用户触发特定意图工作流时，按需把 Kron 的格式规范（如 Frontmatter 字段约束、锚点注释语法）注入当前对话。
2. **省去 AI 多次 Tool Call 的往返开销（Pre-fetching Context）**：
如果只靠 Tools，AI 需要先调 `get_intent` -> 等待返回 -> 再调 `find_anchors` -> 等待返回 -> 再去读源码文件，耗时且费 Token。
通过 Prompts，当用户输入 `/implement-intent auth/jwt` 时，`kron mcp` 直接在服务端把**该意图的 Markdown 全文 + 所有包含 `// @kron:intent auth/jwt` 的源码文件路径与行号**一次性拼装进 Prompt 返回，AI 拿到就能直接写代码（Zero-shot Context）。
3. **固化“文档与代码同步”的闭环**：
很多时候代码改完了，`.kron/intents/` 里的状态忘了更新，或者代码里的锚点成了悬空引用。Prompts 可以把 `lint` 结果直接塞进对话，引导 AI 自动修复。

---

### 二、 最适合 Kron 落地的 3 个核心 Prompts

不需要做多，做精以下 3 个，就能覆盖 Kron **90%** 的日常使用场景：

#### 1. `kron-implement`（按意图编码）

* **参数**：`slug`（必填，例如 `parser/anchor-extract`）
* **Server 内部做的事**：
1. 调用 `store.Get(slug)` 读取对应的 `.kron/intents/*.md` 内容（以及它的父级/子级意图摘要）。
2. 调用 `parser` 查找当前代码库中所有打上 `// @kron:intent <slug>` 的文件位置与代码片段。


* **生成的 Prompt 结构**：
* **角色与目标**：要求 AI 严格围绕该 Intent 的设计目标编写或修改代码。
* **内联上下文**：直接附带 Intent Markdown 原文 + 现有锚点所在的文件路径和代码上下文。
* **强约束规则**：要求在新增的核心入口函数/结构体处必须标注 `// @kron:intent <slug>`，完成后调用 Kron Tool 更新意图状态。



#### 2. `kron-drift-check` / `kron-sync`（意图偏移检查与同步）

* **参数**：`slug`（可选，不填则检查最近修改或全部 active 意图）
* **Server 内部做的事**：
运行一次内部 `lint` 逻辑，找出未关联代码的孤儿意图、失效的锚点，或提取某意图关联的所有代码实现。
* **生成的 Prompt 结构**：
让 AI 对比 `.kron/intents/<slug>.md` 中记录的设计方案与当前源码锚点处的真实实现，判断是否存在“设计偏移（Intent Drift）”——如果代码改了但文档没更新，让 AI 自动调用 Tool 更新意图文档；如果产生了新子任务，引导拆分子意图。

#### 3. `kron-plan`（从散乱需求拆解意图树）

* **参数**：`topic`（必填，新功能或重构主题描述）
* **Server 内部做的事**：
读取当前 `.kron/intents/` 的目录树结构（Hierarchy），作为背景上下文。
* **生成的 Prompt 结构**：
内置 Kron 的 Frontmatter 规范和“目录即树”的设计哲学，引导 AI 先和用户讨论方案，确认后按规范调用创建工具生成对应的 `.kron/intents/` 节点，而不是一上来就乱写代码。

---

### 三、 开发代价有多大？

**极低，基本只是对现有 `store` 和 `parser` 的一次字符串拼接封装：**

* **零底层逻辑新增**：你在开发 CLI（`init`, `add`, `lint`）和 MCP Tools 时，已经写好了读取 Intent 和扫描锚点的函数。Prompts handler 只是在内部调用这些现成的 Go 函数，把拿到的数据塞进 `fmt.Sprintf` 模板里返回。
* **代码量评估**：在 `mcp-go`（或官方 Go SDK）中，注册一个 Prompt 并实现 Handler 大约只需 **30 ~ 50 行代码**。3 个核心 Prompts 加起来也就 **150 行左右**，半天内就能写完调通。

---

### 四、 推进优先级建议

虽然强烈建议做，但需要注意**依赖顺序**：

1. **第一步（基石）**：先把 **Tools**（增删改查意图、搜索锚点）和前面讨论的 **`store` 缓存/原子写入** 跑稳。因为 AI 在通过 Prompt 开启任务后，最终闭环（比如修改意图状态、创建新子意图）还是得靠调用 Tools 来落盘。
2. **第二步（体验起飞）**：紧接着加上这 **2~3 个核心 Prompts**。一旦加上，你在宿主 IDE 里使用 Kron 的流畅度会有质的飞跃——从“手动指挥 AI 调工具”变成“一条斜杠命令直接进入编码上下文”。