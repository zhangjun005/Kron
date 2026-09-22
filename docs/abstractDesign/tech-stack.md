# 技术选型

> 本文档记录各层的技术建议，供决策参考。架构与模块拆分见 [`architecture.md`](./architecture.md)。

**Kron 的核心是数据格式 + 协议**，不是任何一个二进制。CLI、MCP、编辑器插件、TS 客户端都是这套数据的不同访问入口。

按访问层分四块：

---

## 1. 核心 — Go

**建议 Go 1.27 + cobra。**

- 文件 I/O、字符串处理、并发是 Kron 的全部工作量，Go 标准库直接覆盖，零依赖起步。
- 单文件二进制，发布、复制、跨平台都没成本。
- `go vet` / `gofmt` 是强约定，配合 `internal/` 包边界，AI 生成的代码很难越界。
- 对比 Rust/Tauri 原型：编译慢、依赖重、单人维护成本高。Go 的开发曲线更平，符合"个人开发者工具"定位。

**不建议** Rust 重写、Tauri 内嵌、Electron 后端、WASM。

---

## 2. UI — TypeScript 客户端

**建议 纯 TypeScript + 轻量 UI 框架（待定：React / Svelte / Vue），以 Web 应用形式交付。**

- Kron 的数据是仓库里的 .md 文件，任何能发起 HTTP 请求或调 CLI 子进程的地方都能访问。客户端不是"唯一的入口"，只是人类更舒服的入口。
- TS 是 skill 规范好的，方向确定。
- 客户端通过 Go CLI 的 `--json` 输出获取数据，不直接读写文件系统——Go CLI 才是 Single Source of Truth 的实现层。
- Electron / 纯 Web / PWA 都行，具体形态等 Core CLI（Phase 2）稳定后再决定。

**不建议** Tauri（Rust 重叠）、桌面原生框架。

---

## 3. MCP 服务 — Go 实现

**建议 复用 Go 代码库，加 `serve-mcp` 子命令。**

- MCP server 本质是 stdio JSON-RPC，Go 手写 `encoding/json` 即可，不引入新依赖。
- 工具（Tools）映射到 Kron 的原子操作：`kron_add`、`kron_list`、`kron_get` 等，数据层复用现有 store/parser 包，零重复逻辑。
- Cursor / Claude Code 原生支持 stdio MCP server，配置到 `.cursor/mcp.json` 即可，无需额外 SDK。

**不建议** 独立 MCP 二进制（维护成本）、HTTP/SSE（stdio 已够）、绑定特定 LLM。

---

## 4. 编辑器高亮 — TextMate 语法 + LSP Hover

**建议 两段式：TextMate grammar 做颜色，LSP 做悬停。**

- **高亮**：`// @kron:intent` 这类锚点注释跨语言都长得一样，用 TextMate grammar（`tmLanguage.json`）足以识别 + 上色，成本最低、兼容所有 VSCode 系编辑器。
- **悬停 / 跳转**：封装为轻量 LSP 子进程，复用 Go CLI 的解析能力，实现 `textDocument/hover` 与 `textDocument/definition`。
- LSP 是标准协议，未来 Neovim / JetBrains 也能复用，不锁死 VSCode。

**不建议** VSCode Extension API 独占方案、自研协议、Tree-sitter 全量解析（v1 阶段）。

---

## 依赖原则（贯穿四层）

- 数据全部存仓库，工具自身目录**不存项目数据**。
- 备份 / 行进全靠 git，不做自备份。
- 能用标准库就不引依赖；引依赖必须在 commit body 写清理由。
- 协议优先于绑定（stdio MCP、TextMate、LSP 都是标准协议），不绑死单一客户端。
