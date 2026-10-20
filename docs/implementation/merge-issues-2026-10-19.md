# 合并问题记录：serve-lsp 分支（2026-10-19）

## 合并结果

- 来源：`origin/feat/serve-lsp`
- 合并目标：`master`
- 合并方式：`--no-ff`
- 合并提交：见当前 `HEAD`
- Git 预演：无冲突

分支已合入，包含 `serve-lsp`、VSCode 客户端骨架、store 跨进程 intent 锁，以及 CLI 的 `kron trash` / `kron restore`。

## 已识别问题与处理

`cmd/kron/cli/intent_trash.go` 新增了 `trash` 和 `restore` 两个 CLI 子命令。当前架构真理源 [`docs/abstractDesign/architecture.md`](../abstractDesign/architecture.md) 仍把 CLI 限定为最小命令集，并明确将 `restore` 归给 MCP。因此这部分实现与现有 CLI 边界文档不一致。

这不是协议层互调问题：CLI 直接调用 `internal/store`，没有调用 MCP；MCP 仍通过相同的 store 能力提供 `kron_restore`。问题是公开 CLI 契约和架构命令清单没有同步。

## 影响范围

- CLI 帮助、命令分发和脚本允许列表需要纳入新命令，或恢复原有最小 CLI 边界。
- MCP、LSP、VSCode 客户端和 `.kron` 数据格式没有直接的协议兼容变化。
- `internal/store` 的写入锁会同时影响已有的 `Write` / `Update`，需要继续做跨平台验证。

### CLI `trash` / `restore`

已按原架构边界废除 CLI 入口：删除命令分发、帮助文本、实现文件和对应 CLI 测试。`internal/store` 的软删除能力及 MCP 的 `kron_delete` / `kron_restore` 保留，数据仍移动到 `.kron/.trash/`，不会直接物理删除。

### VSCode workspace 范围

当前 VSCode 扩展只启动第一个 workspace folder 的 MCP/LSP 子进程。该限制与单项目当前指针模型一致；多根 workspace 的目标方案是每个 folder 独立启动一组子进程，暂不扩大实现范围。

### LSP 文档同步与能力边界

当前 `serve-lsp` 的 `initialize` 声明了 `TextDocumentSync`（`OpenClose=true`、`Change=Full`），但
`didOpen`、`didChange`、`didClose` 目前只返回成功，没有保存客户端内存中的未保存文本。这样可以维持
请求链路，但不符合“语言功能基于同步后文档内容计算”的完整语义。后续只能二选一：

1. 增加按 URI 保存文档内容的内存层，`didOpen` 写入全文、`didChange` 应用全文或增量变更、`didClose` 删除；
   Hover、Definition、Completion、Diagnostics 等统一读取这份内容。
2. 在真正支持同步前撤销 `OpenClose` / `Change` 能力声明，明确服务端只读取磁盘，因此未保存内容不参与解析。

标准能力的实现方向也已记录：Completion 返回候选项和必要的替换编辑；Diagnostics 发布带 URI、范围、严重级别
和消息的诊断并在内容更新后清理旧结果；References 返回同一 slug 的引用位置；Rename 返回由客户端应用的
`WorkspaceEdit`；Document Symbols 返回当前文件的 anchor 树或列表；Workspace Symbols 才负责全项目查询。
未声明的能力不属于协议缺陷，但一旦在 `initialize` 中声明，就必须提供相应语义。

### LSP 对 MCP 的影响

当前 LSP 和 MCP 是两个独立的 stdio 子进程，均通过 `internal/parser`、`internal/store` 读取同一工作区；
LSP 不调用 MCP，MCP 也不调用 LSP。现有 LSP 仅增加读路径，不改变 MCP 工具、MCP wire schema、store 写入锁或
`.kron` 数据格式。即使将来 LSP 增加文档内存层，也只影响 LSP 看到的未保存文本，不会把未保存内容写入 MCP 或磁盘。
只有实现 Rename、Code Action 等会产生写操作的 LSP 能力时，才需要明确通过 `WorkspaceEdit` 交给客户端，或复用
`internal/store` 的写入锁；不应直接调用 `serve-mcp`。

### 主流语言服务器的通用做法

以 gopls、rust-analyzer、Pyright 这类语言服务器为参照，通常采用以下结构：

- 维护每个打开文档的内存快照，按全文或增量变更更新；语言功能不直接把磁盘文件当作打开文档的唯一事实。
- 使用持久的分析服务和工作区索引，避免每次 Hover、Completion、References 都从头扫描整个项目。
- Completion 根据光标上下文返回候选项和替换编辑；Diagnostics 在编辑后异步、短暂延迟地重新计算，并发布最新结果、清理旧结果。
- References、Rename、Workspace Symbols 依赖跨文件索引；Rename 返回 `WorkspaceEdit`，由客户端应用而不是服务端自行改文件。
- Document Symbols 返回当前文件的层级符号，Workspace Symbols 使用单独的工作区查询；所有能力都通过 `initialize` 能力协商，未实现的能力不声明。
- 对长时间分析支持取消、过期结果丢弃和配置变更后的重新分析，避免旧结果覆盖新文档状态。

### LSP 后续改动成本记录

以下估算以当前 Kron 的 anchor/slug 业务范围为基准，不包含完整编程语言类型系统的复杂度：

| 改动 | 主要工作 | 粗略成本 |
|---|---|---|
| 修正同步声明 | 不再声明 `OpenClose` / `Change` | 半天以内 |
| 实现全文同步 | 文档内存表、三个同步通知、解析器从文本读取、并发测试 | 1～2 天 |
| 简单 Completion | 根据 slug 列表补全 `@kron:intent` | 1～2 天 |
| 基础 Diagnostics | 未知 slug、格式错误和 frontmatter 错误映射为诊断 | 1～3 天 |
| References | 扫描或索引全项目 anchor | 1～3 天 |
| Document Symbols | 将当前文件 anchor 返回为 Outline 符号 | 半天～1 天 |
| Workspace Symbols | 建立项目级 slug/文件索引并支持查询 | 1～3 天 |
| Rename | `WorkspaceEdit`、文件名变化、冲突和写入协调 | 2～4 天 |

其中全文同步是其他能力的基础；如果把这些能力都做成可长期使用的实现，主要成本在文档状态、跨文件索引、
诊断调度和 Rename 的工作区编辑，不在 LSP 方法注册本身。

### `TestCacheEffect` 超时

该测试是性能测量，不应随普通 `go test ./...` 执行。它现在只有在设置 `KRON_RUN_CACHE_EFFECT=1` 时才运行；专用运行方式为：

```text
KRON_RUN_CACHE_EFFECT=1 go test ./cmd/kron/serve-mcp -run TestCacheEffect -v -timeout 10m
```

## 后续任务

1. 继续验证 store 跨平台锁在 Linux、macOS、Windows 上的行为。
2. 删除合并提交遗留的临时违规标记已经完成。
3. 运行 `go vet ./...`、`gofmt -l .`、`go test ./...`。
4. 在决定支持未保存文本前，先统一 `TextDocumentSync` 声明与实际处理方式。

