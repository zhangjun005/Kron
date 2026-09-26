# 新增 `internal/` 子包流程

> 来源：`docs/abstractDesign/architecture.md` §〇·五·3、§〇·五·5  
> 本文件是**实施流程**，不是架构真理。当业务逻辑需要新 `internal/` 子包时按此流程执行。

---

## 1 什么时候需要新 `internal/` 子包

决策树：

```
要加的逻辑是什么？
├─ 纯数据结构（没有任何行为）
│   └─ → 放 internal/model/ 的现有文件中，或新建 internal/model/<name>.go
├─ 文件 I/O / frontmatter 序列化
│   └─ → 放 internal/store/
├─ 输入解析（CLI 参数 / markdown 文本 / 锚点语法 / 相对链接）
│   └─ → 放 internal/parser/
├─ 组合逻辑，恰好只有一个访问层（CLI 或 MCP）要用
│   └─ → 留在该访问层包内（cmd/kron/cli/ 或 cmd/kron/serve-mcp/）
└─ 组合逻辑，两个以上访问层都要用
    └─ → 新开 internal/<业务名>/（见 §2）
```

---

## 2 新包命名规则

| 原则 | 示例 |
|---|---|
| 用名词，不用动词 | `internal/lint`（名词），不 `internal/linting` |
| 与现有包不重叠 | 不开 `internal/check/`（已有 `internal/parser`） |
| 业务语义优先于技术语义 | `internal/lint`（业务：lint）好过 `internal/validation`（技术：校验） |
| 长度不超过两个词 | `internal/lint` / `internal/scan`，不 `internal/anchorvalidation` |

---

## 3 新包论证模板

每次新建 `internal/<name>/` 子包，必须在 **commit body** 中回答以下 5 个问题：

```markdown
## 新包论证：internal/<name>/

**1. 这个包解决什么问题？**
<一句话描述>

**2. 为什么不能放在现有包？**
<说明为什么不适合 internal/store / internal/parser / 访问层内>

**3. 有多少个访问层要用它？**
<列出 CLI / MCP / LSP / IDE / GUI>

**4. 如果以后只剩一个访问层用它，会拆分吗？**
<是 / 否，说明条件>

**5. 包内预计有哪些导出函数？**
<列出签名>
```

---

## 4 实现步骤

### Step 1 — 论证 commit

在添加任何代码之前，先写一个**纯论证 commit**：

```bash
git commit --allow-empty -m "docs(architecture): propose internal/<name>/

论证：internal/<name>/ 为什么需要

See docs/process/internal-pkg.md"
```

> 如果论证被拒绝（评审者认为逻辑可以塞进现有包），撤销这个空 commit 即可。

### Step 2 — 实现

- 包注释：每个包文件顶部必须有一句话说明包职责
- 导出函数必须有 godoc
- 第一个导出的函数必须有测试（`*_test.go`）
- 不在包内 import 任何 `cmd/kron/` 或其他访问层

### Step 3 — 集成

- 在 `docs/abstractDesign/architecture.md` 的 §〇·五·5 包分工表中加入新包行
- 更新 `AGENTS.md` 的目录结构

---

## 5 v1 已有 `internal/` 包

| 包 | 职责 | 何时开 |
|---|---|---|
| `internal/model` | 对象层：纯数据结构 + 校验方法 | v1 必须 |
| `internal/store` | 业务层：文件 I/O + YAML 序列化 | v1 必须 |
| `internal/parser` | 业务层：输入解析（slug、markdown、锚点、相对链接） | v1 必须 |
| `internal/lint` | 业务层：扫描 + 校验组合（两个以上访问层用到时才开） | 当 CLI、MCP、IDE 都需要 lint 时开 |
| `internal/...` | **不**预设更多 | 每个新包都要 commit 论证 |
