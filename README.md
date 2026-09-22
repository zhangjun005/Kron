# Kron

> 基于 Git 的 AI 辅助开发**意图管理**系统。

Kron 把代码设计意图（Design Intent）存为**纯 Markdown 文件，放在你的仓库里**——无需 API 服务、无需守护进程、无需私有格式。AI 编程助手可以直接读写它们。一切差异就是 `git diff`。

---

## 痛点

AI 编程助手在清楚"为什么这么做、权衡了什么、放弃了什么"时表现最好。但现状是：

- 设计决策只存在于会议纪要、Slack 消息流或某人的脑子里；六个月后没人记得当时为什么拒绝方案 B。
- 代码里的 `// FIXME` / `// TODO` 只能描述"要做什么"，无法承载"为什么这样做"。
- 接手者（包括下一个 AI Agent）只能对着代码反推意图，推错了就埋雷。

长期下来就产生了 **Intent Debt**：意图丢失、假设漂移、权衡失效。

## Kron 是什么

一个把意图沉淀为**仓库内 Markdown 文件**的系统：

```
.kron/
├── intents/                   # 每个意图一个 .md（核心）
│   ├── 2026-09-19-001-storage-format.md
│   ├── 2026-09-19-002-id-scheme.md
│   └── ...
└── config.toml                # Kron 配置（可选）
```

每个意图文件是一份 Markdown 文档，带一个轻量的 YAML frontmatter：

```markdown
---
symbol: "auth.RefreshToken"
created_by: "@zhangjun005"
updated_at: "2026-09-19T22:30:00Z"
---

# 存储格式选型

> 选 Markdown + YAML frontmatter，零依赖、零迁移成本。

## 为什么
存储需要同时被人和 AI 编辑。选 Markdown + YAML frontmatter 的理由：
- 人类可直接 `git diff` 阅读，无需学新工具
- AI 可直接 prompt-context 读取，无需解析二进制

## 权衡
- **放弃了**：原生 SQLite 索引查询能力 —— 换来了零依赖、零迁移成本
- **代价**：大规模条目下需要全文搜索，不适合 >1万条 的仓库

## 边界假设
- 假设仓库规模在个人 / 小团队级别（<1k 条）
- 假设意图条目不会被频繁跨文件交叉引用（重写场景少见）
```

代码侧用极轻量锚点反向引用：

```go
// @kron:intent 2026-09-19-001-storage-format
func ParseFrontmatter(raw []byte) (map[string]any, string, error) { ... }
```

就这样。**AI 读的文件和你读的一样。**

## Kron 不做什么

- 无后台守护进程、无文件监听、无同步引擎。
- 无双源持久化、无 mtime+hash 状态机冲突处理。
- 无私有格式。Markdown 本身就是 API。
- **不是**任务追踪器。任务只是意图落地时的派生钩子，不是独立实体。
- 无 Electron 应用。（未来可能基于 Tauri 做 GUI，但非必须。）

只要贡献者或 AI 能自信地编辑 Markdown 文件，那文件就是真相。

## 状态

🚧 **重做中（Go）**。

## 参与贡献

格式和 CLI 设计决策在 [`docs/abstractDesign/`](./docs/abstractDesign/)。提 PR 前先读这些——格式是 Kron 赖以生存的东西，改动会直接影响到用户，因为他们拥有这些文件。

## License

MIT.
