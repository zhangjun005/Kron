# Lint 规则扩展流程

> 来源：`docs/abstractDesign/architecture.md` §5.3  
> 本文件是**实施流程**，不是架构真理。给 `kron lint` 增加新规则时按此流程执行。

---

## 1 Lint 规则分类

| 类别 | 含义 | 示例 | 增加方式 |
|---|---|---|---|
| **A 类：Anchor 悬空** | 锚点指向不存在的意图文件 | `@kron:intent auth/jwt` 无对应 `.md` | `internal/parser` + `internal/store` |
| **B 类：Frontmatter 格式** | frontmatter 字段缺失或格式错误 | 缺少 `created_by`、无效 `status` 值 | `internal/store` frontmatter 解析 |
| **C 类：Body 语义** | Markdown body 内部结构不符规范 | 缺少 `## Why` / `## Trade-offs` 节 | `internal/parser` Markdown 解析 |
| **D 类：锚点密度** | 源码文件中锚点过多/过少（可选） | 某文件 100 行无任何锚点 | `internal/parser` 扫描统计 |

A 类和 B 类是 v1 已覆盖的核心规则；C / D 类是 v1 之后的扩展方向。

---

## 2 新增 A/B 类规则（最常见）

### 决策树

```
新规则检查的是什么？
├─ Anchor 悬空（源码中锚点指向的 .md 不存在）
│   └─ → 修改 internal/parser 或 internal/store 的 Anchor 解析逻辑
├─ Frontmatter 缺失/格式
│   └─ → 修改 internal/store/frontmatter.go 的校验函数
└─ 两者都不是
    └─ → 走 §3（扩展方向）
```

### 实现步骤

1. **写测试**：在 `internal/<pkg>/<pkg>_test.go` 中用表驱动法写用例
2. **实现**：在 `internal/<pkg>/` 中实现检查函数
3. **集成**：`cmd/kron/cli/lint.go` 调用新增检查
4. **文档**：更新 `docs/implementation/cli.md` 的 lint 流程 + 本文件

---

## 3 新增 C/D 类规则（扩展方向）

C / D 类规则影响 `kron lint` 的退出码语义（`1 = lint 错误`），按以下流程：

1. **RFC**：在 `docs/rfc/` 提出新规则，说明"这个规则为什么是强约束而不是建议"
2. **评审**：确认是"违反会导致系统不可信"还是"违反只是代码风格"
3. **实现**：在 `internal/lint/rules/` 下新增子包（如 `internal/lint/rules/body.go`）
4. **CI 集成**：在 `kron lint` 命令行 flag 中加上 `--enable-rules` 或硬编码
5. **文档**：更新本文件 + `docs/implementation/cli.md`

> **不是所有检查都要进 `kron lint`**。如果只是"建议"而非"约束"，考虑作为 LSP/IDE 插件的诊断建议输出，不走 CI 门禁。

---

## 4 已有规则清单

| 规则 ID | 类别 | 描述 | 源码位置 |
|---|---|---|---|
| `anchor-dangling` | A | 锚点指向不存在的 intent | `internal/parser.ScanAnchors` + `internal/store.ResolveIntent` |
| `frontmatter-required` | B | `created_by` / `updated_at` 缺失 | `internal/store/ValidateFrontmatter` |
| `frontmatter-status` | B | `status` 值不是 `draft` / `active` / `superseded` | `internal/store/ValidateFrontmatter` |
| `frontmatter-date` | B | `updated_at` 不是有效 ISO 8601 | `internal/store/ValidateFrontmatter` |

---

## 5 与 v1 "不做的事" 的关系

以下明确不在 v1 lint 范围内：

| 特性 | 原因 |
|---|---|
| 健康度诊断（过期 / 孤儿 / 冲突检测） | `requirements.md` 未明确要求 |
| 锚点密度分析 | v1 是最小集；真实需求出现时再加 |
| `--path` 自定义扫描根 | 全仓 + 黑名单足够 |
