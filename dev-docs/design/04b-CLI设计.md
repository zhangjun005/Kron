# 04b · CLI 设计（Kron Command Line）

> 状态：草案 v1（2026-09-05）— **骨架版，待逐节细化**
> 适用范围：定义 `kron` 命令行工具的命令清单、参数、输出格式、退出码语义。
> 读者：实现者、AI Agent（消费 CLI）、用户（参考用法）。
> 前置阅读：[00-总览与架构.md](./00-总览与架构.md)、[03 § 5.8 AI 友好约定](./03-双源同步机制.md)、[07-实施路线图.md](./07-实施路线图.md) § 3-6

---

## 1. 设计原则

| # | 原则 | 说明 |
|---|------|------|
| **P1** | **与 Git CLI 对齐** | 命令风格、参数习惯、`--json` / `--porcelain` 命名、退出码语义都参考 Git。AI 学过一次就能用。|
| **P2** | **AI 一等公民** | 所有命令支持 `--json`；输出稳定可解析；退出码机器可读（详见 03 § 5.8）。|
| **P3** | **人也能直接用** | 默认输出人类可读（带颜色/表格）；不强制 `--json` 才能看懂。|
| **P4** | **POSIX-ish 参数风格** | 长选项 `--json`、短选项 `-v`；子命令用空格（`kron task list`）而非连字符。|
| **P5** | **静默失败透明** | 错误必须打 stderr；退出码必须准确；绝不"假装成功"。|
| **P6** | **配置极简** | v1 几乎不用配置；所有可选项都有合理默认。|
| **P7** | **零网络依赖** | CLI 永远不发起网络请求；守护进程也不做 HTTP 端点（Q8 已定）。|

---

## 2. 命令分类总览

```
kron
├── project        项目级操作
├── task           task 增删改查、状态移动
├── vertex         vertex 绑定、关系查询
├── important      重要文件管理
├── conflict       冲突查询与解决
├── daemon         守护进程控制
├── context        AI 易读层生成
└── help           帮助与版本
```

**命名约定**：
- 复数概念用单数（`task` 不是 `tasks`，`vertex` 不是 `vertices`）— 与 Git 一致（`git branch` 不是 `git branches`）
- 操作动词：`list / show / add / create / delete / move / start / done / back / check / attach / resolve`

---

## 3. 命令清单（骨架）

> 标注：**[P1]** = Phase 1 实现；**[P2]** = Phase 2；**[P4]** = Phase 4；**[P5]** = Phase 5
> 完整 syntax 用 `kron <subcmd> --help` 展示；此处只列骨架。

### 3.1 `kron project`

| 子命令 | Phase | 用途 | 关键参数 |
|--------|-------|------|---------|
| `kron init` | P1 | 初始化当前工作区 | `--force`, `--mode symlink\|copy`, `--no-vertex` |
| `kron status` | P1 | 查看项目状态 | `--json`, `--porcelain`, `--watch`（5s 刷新） |
| `kron list` | P1 | 列出所有项目 | `--json` |
| `kron path` | P1 | 输出 kron-internal 路径 | `--kron-root`, `--important` |
| `kron config` | P4 | 配置读写 | `get <key>`, `set <key> <value>`, `list` |

**行为要点**：
- `kron init` 在当前目录执行；要求是 Git 仓库（除非 `--no-git`）；退出码 4 表示已初始化且未传 `--force`
- `kron status --watch` 是简化版 daemon 状态展示（用户手开终端时用），不会自动启动 daemon
- `kron path` 是 AI 用的"路径探测器"——纯输出，不打印其他文字（确保 AI 可解析）

**`--mode` 选择**：
- `symlink`（默认，Windows 上需开发者模式或管理员）
- `copy`（v1 默认 fallback；性能略差但无权限问题）

### 3.2 `kron task`

**描述（description）约定**：task 必有简短 description（人类读）、可选 long description（MD 正文）。
- `description` 字段存 `tasks.json`（短，单行，≤ 200 字符）→ AI 读这个就够
- long description 存 MD 正文（多行 markdown，编辑 task 时用）
- 命令行 `--desc`（创建时一次性写入），`-m`/`--message`（后续追加/更新）
- `add` 时两者都给：`add <v> --title ... -m "..."` → `-m` 写 description

| 子命令 | Phase | 用途 | 关键参数 |
|--------|-------|------|---------|
| `kron task list <vertex>` | P1 | 列出 vertex 下 task | `--state <s>`, `--tag <t>`, `--json` |
| `kron task show <id>` | P1 | 显示 task 详情（含 description + MD 正文） | `--json` |
| `kron task add <vertex>` | P1 | 新增 task | `--title`, `--desc`, `-m`/`--message`, `--tag`, `--json` |
| `kron task describe <id>` | P1 | 修改 description（短描述） | `-m`/`--message <text>`（必填），`--editor`（调 $EDITOR） |
| `kron task move <id>` | P1 | 移动 state | `--to <state>` |
| `kron task start <id>` | P1 | 标记 doing | — |
| `kron task done <id>` | P1 | 标记 done | — |
| `kron task back <id>` | P4 | 退回上一 state | — |
| `kron task check <id>` | P4 | 勾选（仅 done 列用）| — |
| `kron task attach <id>` | P4 | 关联文件/git/commit | `--file`, `--commit` |
| `kron task detach <id>` | P4 | 取消关联 | `--file`, `--commit` |
| `kron task edit <id>` | P4 | 打开外部编辑器改 MD 正文 | （无）|
| `kron task delete <id>` | P4 | 删除 task | `--force` |
| `kron task reopen <id>` | P4 | done → doing（恢复） | — |

**`-m`/`--message` 语义**：
- 在 `task add`：写 description 字段（短）
- 在 `task describe`：覆盖 description 字段（短）
- 不接受 `-m` 在 `task move`/`task start` 等命令上（避免歧义）

**description 长度限制**：单行 ≤ 200 字符（超出截断 + warn，退出码 0）

**多行 description**：走 `--editor`（`$EDITOR` 或 fallback `notepad.exe`）。

### 3.3 `kron vertex`

**描述（description）约定**：vertex 必有 description 解释"这是干什么的"，存 `vertices.json`。
- `description` 字段存 `vertices.json`（短，单行，≤ 200 字符）
- 长说明放 vertex 根目录的 `README.md`（如有）
- 命令行 `-m`/`--message`（创建/更新时写）
- `show` 同时输出 description + 关联 task 数 + Git 分支

| 子命令 | Phase | 用途 | 关键参数 |
|--------|-------|------|---------|
| `kron vertex list` | P4 | 列出所有 vertex（含 description） | `--json` |
| `kron vertex show <name>` | P4 | vertex 详情（description + task 数 + git 分支） | `--json` |
| `kron vertex create <name>` | P4 | 新建 vertex | `-m`/`--message`, `--branch <git-branch>`, `--path <dir>` |
| `kron vertex describe <name>` | P4 | 修改 vertex description | `-m`/`--message <text>`（必填），`--editor` |
| `kron vertex delete <name>` | P4 | 删除 vertex | `--force` |
| `kron vertex relate` | P4 | 查/改 vertex 关系 | `--from`, `--to`, `--type` |

**`-m`/`--message` 语义**（与 task 一致）：
- 在 `vertex create`：写 description
- 在 `vertex describe`：覆盖 description
- 长度 ≤ 200 字符（超出截断 + warn）

**Q22**：`vertex delete` 前是否强制 `vertex describe` 长期保留 description 进历史？
- A: 否（与 task 一致，删了就没了）
- B: 是（写 `vertices-deleted.json` 历史档）
- **倾向 A**（保持简单；真要查从 Git log 里 `git log -S` 找）

### 3.4 `kron important`

**核心概念**：`KRON/important/` = 用户**主动添加**的重要文件目录（idea / 设计文档 / 参考资料）。
- 与 `.kron-context/` 的区别（见 § 3.7）：important 是用户原创内容；context 是 Kron 自动生成的中间文档
- 双源：kron-internal 权威源 ↔ 项目内 `KRON/important/` 软链接/复制（默认 symlink，降级 copy）

| 子命令 | Phase | 用途 | 关键参数 |
|--------|-------|------|---------|
| `kron important list` | P4 | 列出所有重要文件 | `--tag <t>`, `--json`, `--porcelain` |
| `kron important add <path>` | P4 | 添加到 important/ | `--copy` / `--symlink`（默认 symlink）, `--tag`, `-m`/`--message` |
| `kron important remove <path>` | P4 | 移除（kron-internal 删除；项目内链接不删） | `--force`, `--also-delete-source` |
| `kron important show <path>` | P4 | 文件详情 + sync 状态 | `--json` |
| `kron important tag <path>` | P4 | 加/改标签 | `--add <t>`, `--remove <t>`, `--set <t1,t2>` |
| `kron important sync` | P4 | 强制双向同步（解决 drift） | `--dry-run`, `--direction project→internal\|internal→project\|both` |

**`--copy` vs `--symlink`**（影响 `add` 行为）：
- `--symlink`（默认）：kron-internal 是真文件，项目内是符号链接 → 改一边两边同步
- `--copy`：两边都是独立副本 → 改一边不影响另一边；后续 sync 会按时间戳合并（last-write-wins）

**`--also-delete-source`**（`remove` 时）：
- 默认：只删 kron-internal 真文件，项目内链接残留（不删，防误删用户文件）
- 传此 flag：同时删源文件（如 `important/notes.md` → 删 `notes.md`）

**`sync --direction`**：双向漂移修复
- `project→internal`：项目内改动覆盖 kron-internal
- `internal→project`：kron-internal 改动覆盖项目内
- `both`：根据两边 mtime 比较，新的赢（last-write-wins）；并列时交互选择

**Q27 待定**：`add` 时如果源文件已在 important/ 内（重复添加）？
- A: 报错退出码 8
- B: 静默幂等（退出码 0）
- **倾向 B**（AI 友好，重复操作无副作用）

### 3.5 `kron conflict`

**核心概念**：conflict = 双源（kron-internal vs project）同一文件被双方修改且未合并。
- id 格式：`c-<8位短哈希>`（与 task id 命名空间分开）
- 文件路径（conflict 涉及）：相对工作区路径
- 状态：`pending` / `resolved` / `ignored`
- 三种源：`kron-internal`（kron 内部改动）、`project`（Git 工作区改动）、`both`（双方都改）

| 子命令 | Phase | 用途 | 关键参数 |
|--------|-------|------|---------|
| `kron conflict list` | P2 | 列出冲突 | `--status pending/resolved/all`, `--since <date>`, `--json` |
| `kron conflict show <id>` | P2 | 冲突详情（含 unified diff 三方对比） | `--json`, `--diff-only` |
| `kron conflict resolve <id>` | P2 | 解决冲突 | `--use project\|internal\|both\|prompt`, `--reason <text>` |
| `kron conflict ignore <id>` | P2 | 标记为可忽略（不再警告） | `--reason <text>` |
| `kron conflict cleanup` | P4 | 清理已解决的备份文件 | `--older-than <days>`, `--dry-run` |

**`--use` 四种策略**：
- `--use project`：保留项目侧内容，丢弃 kron-internal 侧 → 写回 project + 备份 .bak
- `--use internal`：保留 kron-internal 侧内容，覆盖项目侧 → 写回 project + 备份 .bak
- `--use both`：拼接为 `<<<<<<< internal / ======= / >>>>>>> project`，**人工再修**
- `--use prompt`（默认）：交互式菜单列出 4 选项 → 非 TTY 报错退出码 2

**自动备份**：每次 resolve 在 `<file>.kron-resolved-<timestamp>.bak` 留一份，方便 `cleanup` 清理。

**`--reason` 字段**：可选，写入 `conflict.json` 的 `resolution.reason` 字段，方便以后溯源"为什么这么选"。

### 3.6 `kron daemon`

**核心定位**：守护进程是 `.kron-context/` 自动维护 + 文件监听 + 冲突检测的执行体（详见 04 § 1）。
- v1 单工作区守护（每个工作区一个进程）
- 不监听 TCP/HTTP（Q8 已定，零网络）
- 通过文件锁 + 命名管道与 CLI 通信

| 子命令 | Phase | 用途 | 关键参数 |
|--------|-------|------|---------|
| `kron daemon start` | P2 | 启动守护进程（后台） | `--foreground`, `--log-level debug\|info\|warn\|error` |
| `kron daemon stop` | P2 | 优雅停机 | `--timeout <sec>`（默认 10）, `--force` |
| `kron daemon status` | P2 | 查看状态（PID / 运行时间 / 监听数） | `--json` |
| `kron daemon restart` | P2 | stop + start | `--timeout <sec>` |
| `kron daemon install` | P2 | 注册开机自启 | `--user`（当前用户）/ `--system`（需管理员） |
| `kron daemon uninstall` | P2 | 取消开机自启 | — |
| `kron daemon logs` | P4 | 查看日志 | `--follow`（`-f`）, `--tail <n>`（默认 100）, `--since <date>` |

**`--foreground`**（`start` 时）：
- 默认后台（daemonize）
- 传此 flag：前台运行，日志走 stderr（调试用，`Ctrl+C` 退出）
- 与 `daemon logs --follow` 等价但可交互

**`--log-level`**：
- `debug` / `info`（默认）/ `warn` / `error`
- 全局可用（与 04 § 4.1 `-v` 联动：`-v` → info，`-vv` → debug）

**PID/lock 文件路径**（OS 差异）：

| OS | PID 文件 | Lock 文件 | Socket/管道 |
|----|---------|----------|------------|
| **Windows** | `%APPDATA%\Kron\daemon\<workspace-hash>.pid` | 同左 .lock | `\\.\pipe\kron-<workspace-hash>` |
| **Linux/macOS** | `~/.local/share/kron/daemon/<workspace-hash>.pid` | 同左 .lock | `~/.local/share/kron/daemon/<workspace-hash>.sock` |

**`status --json` 输出**：
```json
{
  "running": true,
  "pid": 12345,
  "started_at": "2026-09-05T18:00:00+08:00",
  "uptime_seconds": 3600,
  "watching": { "kron_internal_files": 5, "git_files": 3 },
  "context_files": { "total": 8, "stale": 2 }
}
```

**`install --user` vs `--system`**：
- `--user`（默认）：注册到当前用户登录项（Windows 任务计划程序用户级 / Linux systemd --user / macOS launchd 用户 agent）
- `--system`（v1 可选实现）：注册系统级服务（需管理员权限）

**`stop` 优雅流程**：
1. 发送 SIGTERM（Windows: WM_CLOSE）
2. 等待 `--timeout` 秒
3. 守护进程停止监听、刷新缓存、关闭管道、删除 PID 文件
4. 超时未停 → `--force` 强杀（SIGKILL / TerminateProcess）

### 3.7 `kron context`

**核心定位（重要！避免过度承诺）**：`.kron-context/` 是 Kron 自动生成的 **AI 友好的结构性中间文档缓存**——**只做机器可重现的事实数据**，不做语义理解。
- 让 AI "打开项目就有结构化快照可读"，省去反复 `git log` / `tree` 的开销
- **不试图"让 AI 一键理解项目"**——理解靠 AI 自己 + 读 `KRON/important/`
- 由守护进程后台自动维护（详见 04 § 1），用户零感知
- 可整目录删了重新生成（无状态风险）

**严格原则（Q28 锁定）**：
- ✅ Kron 只生成"事实型"——可由 git / 文件系统 / 现有 JSON 重现的数据
- ❌ Kron 不做"代码语义理解"——不做 import 图分析、不做技术债评估、不做"上次重构是什么"
- ❌ Kron 不做"AI 辅助生成"——不调 LLM、不接 API Key（与哲学一致）
- ⭐ **真正让 AI 理解项目的，是 `KRON/important/` 里人类写的架构/约定文档**——Kron 不替人类思考

**生成清单（v1 先 4 个）**：

| 文件 | 内容 | 生成源 | 过期判定 |
|------|------|-------|---------|
| `.kron-context/README.md` | 中间文档索引（每个文件用途 + 明确声明"Kron 只提供事实，语义理解靠 AI 自己"） | 模板 | 手动维护 |
| `.kron-context/git/recent-commits.md` | 最近 100 次 commit（含 author / date / files changed 计数） | `git log` | 新 commit 即过期 |
| `.kron-context/git/branch-summary.md` | 当前分支 vs 主分支（ahead/behind 数 + 列表） | `git rev-list` | HEAD 变化即过期 |
| `.kron-context/code/structure.md` | 项目目录树（深度 3，跳过 node_modules / target / .git） | `tree` / 自实现 | 文件增删即过期 |

**v1 暂不做**（避免 v1 过度膨胀）：
- ~~`code/dependencies.md`~~（需语言检测 + 多格式解析，v2 再说）
- ~~`code/languages.md`~~（tokei 是 Rust 工具，需自带依赖）
- ~~`tasks/active.md`~~（已在 `tasks.json` 内，AI 直接读 `KRON/VERTEX/<v>/tasks.md` 更准）
- ~~`git/file-history.md`~~（v1 不追踪 important/ 的 git history）

| 子命令 | Phase | 用途 | 关键参数 |
|--------|-------|------|---------|
| `kron context` | P5 | **默认增量更新过期的中间文档**（对齐 requirements § 2.4） | （无；常用命令） |
| `kron context --regenerate` | P5 | 完整重新生成（慎用，耗时） | `--only <pattern>`（只重生指定文件） |
| `kron context --list` | P5 | 列出所有中间文档 + 过期状态 | `--json` |
| `kron context show <file>` | P5 | 输出某文件内容（默认走 pager；stdout 输出无装饰） | `--no-pager` |
| `kron context clean` | P5 | 清空 `.kron-context/`（下次运行自动重建） | `--force` |

**命令名约定**：requirements § 2.4 已定：`kron context` = 增量更新、`--regenerate` = 全量重建、`--list` = 列表。后续实现统一以 requirements 为准。

**触发机制**：
- 后台守护进程（默认 + 唯一主触发者，见 04 § 1.2）
- 用户手动（修复异常时）
- AI 工具主动调用（如果 AI 有 shell 权限）

**给 AI 工具的明示建议**（写进 `.kron-context/README.md`）：
```
.kron-context/ 仅包含结构化事实。
项目语义信息请读：
  1. 根目录 README.md
  2. KRON/important/ 下的文档
  3. 必要时直接阅读源码

不要假设读 .kron-context/ 就"理解"了项目。
```

### 3.8 `kron help` / 全局

| 子命令 | Phase | 用途 | 关键参数 |
|--------|-------|------|---------|
| `kron help` | P1 | 总帮助（所有子命令索引） | `--json` |
| `kron help <subcmd>` | P1 | 子命令详细帮助（含示例） | `--json` |
| `kron <subcmd> --help` | P1 | 同上（P1 起所有子命令支持 `--help`） | — |
| `kron --version` | P1 | 版本号（stdout 单行） | `--short`（仅 semver） |
| `kron completions <shell>` | P4 | 生成 shell 补全脚本 | `<bash\|zsh\|fish\|powershell>` |

**`kron help <subcmd>` 与 `--help` 区别**：
- 完全等价；只是入口不同（一个是独立命令、一个是 flag）
- 输出内容：从代码内嵌 doc comment 提取（单一事实源，不重复维护）

**`kron help --json` 输出**（AI 用于自描述）：
```json
{
  "name": "kron task",
  "description": "Manage tasks within a vertex",
  "subcommands": ["list", "show", "add", "describe", "move", ...],
  "examples": [
    "kron task add backend --title 'OAuth login' -m '...'",
    "kron task list backend --state doing --json"
  ],
  "exit_codes": { "0": "success", "8": "task not found" }
}
```

**`kron completions <shell>` 行为**：
- stdout 输出 shell 补全脚本（无装饰）
- 用户：`kron completions powershell >> $PROFILE` 即可
- 支持的子命令补全：完整子命令名 + 常用 flag（不补全 task id / vertex name 等动态值）

---

## 4. 全局约定

### 4.1 全局 flag

| flag | 短 | 含义 |
|------|-----|------|
| `--json` | — | 输出机器可读 JSON（AI 必需） |
| `--porcelain` | — | 极简输出（每行一项，便于 grep/awk） |
| `--quiet` | `-q` | 静默（只输出错误） |
| `--verbose` | `-v` | 详细日志（叠加：`-vv`, `-vvv`） |
| `--no-color` | — | 禁用 ANSI 颜色（CI 用） |
| `--workspace <path>` | `-C` | 指定工作区（默认当前目录） |
| `--config <path>` | — | 指定 config.json 路径 |

### 4.2 互斥规则（优先级从左到右）

| 组合 | 行为 |
|------|------|
| `--json` + `--porcelain` | 报错退出码 2（`--json` 胜出或直接拒？）**Q29 待定**：倾向**报错退出**（避免歧义） |
| `--quiet` + `--verbose` | 报错退出码 2（必须二选一） |
| `--json` + 默认表格输出 | `--json` 覆盖（隐式，不报错） |
| `--no-color` + TTY 不存在 | 隐式禁用 color（无需传 flag） |

### 4.3 输出模式优先级（叠加场景）

多个 flag 同时传时，**最终输出模式**由下表决定：

| 优先级 | 模式 | 触发条件 | 适用场景 |
|--------|------|---------|---------|
| 1（最高）| `--json` | 用户显式传 | AI 调用、pipeline |
| 2 | `--porcelain` | 用户显式传 | shell 脚本、grep |
| 3 | TTY 检测 | `isatty(stdout)` 为真 | 人类交互 |
| 4 | CI 环境变量 | `CI=true` / `NO_COLOR` 存在 | CI/CD |
| 5（最低）| 默认人类模式 | 否则 | fallback |

**示例**：
- `kron task list --json` → JSON（无论 stdout 是否 TTY）
- `kron task list | grep doing` → TTY 不存在 → 表格退化纯文本（**Q30 待定**：要不要自动退化为 `--porcelain`？倾向**否**——保持显式）
- `CI=true kron task list` → 无 color、无装饰表格

### 4.4 `--verbose` 级别（与 `--log-level` 联动）

| flag | `daemon.log_level` | stderr 输出 |
|------|-------------------|------------|
| （无）| `warn` | warn + error |
| `-v` | `info` | info + warn + error |
| `-vv` | `debug` | debug + info + warn + error |
| `-vvv` | `trace` | 全开（含函数调用） |

**注**：CLI 进程本身的 verbosity 与守护进程日志 verbosity **分开设置**——`kron -v task list` 不会改 daemon 日志级别。

### 4.5 配置加载顺序

```
CLI args > 环境变量 (KRON_*) > kron-internal/config.json > 内置默认
```

详见 § 9。

---

## 5. 输出格式

### 5.1 三种模式

| 模式 | 触发 | 用途 | 示例场景 |
|------|------|------|---------|
| **人类可读（默认）** | 无 flag | 终端用户 | 带颜色、表格、emoji |
| **`--json`** | `--json` | AI / 脚本 | 结构化、稳定 schema |
| **`--porcelain`** | `--porcelain` | grep/awk 流水线 | 每行一项 |

### 5.2 `--json` 约定（与 Git 一致）

- 所有字段命名 snake_case
- 必填字段永远存在（不省略）
- 时间字段统一 RFC3339（`2026-09-05T18:00:00+08:00`）
- 枚举用字符串（如 `"state": "doing"`，不用数字）
- 数字字段不省略零（`"count": 0`，不是省略）
- 数组空时输出 `[]`，不省略字段

### 5.3 人类可读约定

- 默认输出表格（`┌─┬─┐` 风格或 `column -t`）
- 成功 → 绿色；警告 → 黄色；错误 → 红色
- 路径用相对工作区路径
- 时间用本地时区、相对表达（"5 minutes ago"）

### 5.4 `--porcelain` 约定

- 每行一个原子记录
- 字段用 tab 分隔
- 不输出空行、不输出标题
- 字段顺序稳定（与 schema 一致）

### 5.5 各命令的 JSON / porcelain schema（核心子命令）

> **约定**：schema 是**契约**——破坏兼容性视为 BREAKING。下面是 v1 锁定版。

#### 5.5.1 `kron project status`

**`--json` stdout**：
```json
{
  "workspace": "E:\\works\\Kron",
  "kron_root": "E:\\works\\Kron\\KRON",
  "kron_internal": "D:\\Apps\\Kron\\data\\projects\\abc123\\kron-internal",
  "git": { "branch": "main", "remote": "origin/main", "is_repo": true },
  "initialized_at": "2026-09-05T10:00:00+08:00",
  "daemon": { "running": true, "pid": 12345 }
}
```

**`--porcelain` stdout**（每行一个原子事实）：
```
workspace=E:\works\Kron
kron_internal=D:\Apps\Kron\data\projects\abc123\kron-internal
git_branch=main
daemon=running
```

#### 5.5.2 `kron task list`

**`--json` stdout**：
```json
{
  "vertex": "backend",
  "tasks": [
    {
      "id": "t-001",
      "title": "OAuth login",
      "state": "doing",
      "priority": "high",
      "labels": ["auth", "p1"],
      "depends_on": ["t-002"],
      "created_at": "2026-09-04T10:00:00+08:00",
      "updated_at": "2026-09-05T09:30:00+08:00"
    }
  ]
}
```

**`--porcelain` stdout**（一行一 task）：
```
t-001\tdoing\tOAuth login\thigh
t-002\ttodo\tDB migration\tmedium
```

字段顺序：`id \t state \t title \t priority`

#### 5.5.3 `kron task show <id>`

**`--json` stdout**：
```json
{
  "id": "t-001",
  "vertex": "backend",
  "title": "OAuth login",
  "description": "Implement OAuth2 with Google provider...",
  "state": "doing",
  "priority": "high",
  "labels": ["auth", "p1"],
  "depends_on": ["t-002"],
  "blocks": ["t-005"],
  "files": ["src/auth/oauth.rs"],
  "history": [
    { "state": "todo", "at": "2026-09-04T10:00:00+08:00" },
    { "state": "doing", "at": "2026-09-05T09:30:00+08:00" }
  ],
  "created_at": "2026-09-04T10:00:00+08:00",
  "updated_at": "2026-09-05T09:30:00+08:00"
}
```

#### 5.5.4 `kron daemon status`

**`--json` stdout**（见 § 3.6，已锁定）：
```json
{
  "running": true,
  "pid": 12345,
  "started_at": "2026-09-05T18:00:00+08:00",
  "uptime_seconds": 3600,
  "watching": { "kron_internal_files": 5, "git_files": 3 },
  "context_files": { "total": 4, "stale": 1 }
}
```

**`--porcelain` stdout**：
```
running=true
pid=12345
uptime_seconds=3600
context_stale=1
```

#### 5.5.5 `kron context --list`

**`--json` stdout**：
```json
{
  "files": [
    {
      "path": ".kron-context/git/recent-commits.md",
      "exists": true,
      "size_bytes": 4823,
      "generated_at": "2026-09-05T18:00:00+08:00",
      "stale": false,
      "stale_reason": null
    },
    {
      "path": ".kron-context/code/structure.md",
      "exists": true,
      "size_bytes": 1042,
      "generated_at": "2026-09-05T17:30:00+08:00",
      "stale": true,
      "stale_reason": "file added: src/new.rs"
    }
  ]
}
```

#### 5.5.6 `kron conflict list`

**`--json` stdout**：
```json
{
  "conflicts": [
    {
      "id": "c-abc123",
      "path": "KRON/important/notes.md",
      "type": "both_modified",
      "detected_at": "2026-09-05T18:00:00+08:00",
      "internal_mtime": "2026-09-05T17:55:00+08:00",
      "project_mtime": "2026-09-05T17:58:00+08:00"
    }
  ]
}
```

### 5.6 错误输出（与 § 7 联动）

- **人类模式**：stderr 输出 `error: <message>` + `hint: <suggestion>`（同行换行也可）
- **`--json` 模式**：stderr 输出单行 JSON 对象（与 § 7 schema 完全一致）；stdout **必须为空**
- **`--porcelain` 模式**：stderr 与人类模式相同（仅供用户）；stdout 仍可输出成功记录

---

## 6. 退出码语义总表

> 与 03 § 5.8.2 一致；此处是 CLI 层统一表。

| 码 | 名称 | 含义 | AI 处理建议 |
|----|------|------|------------|
| **0** | `Success` | 成功 | 继续 |
| **1** | `GeneralError` | 未分类错误 | 看 stderr |
| **2** | `UsageError` | 参数错误 | 重试（改参数） |
| **3** | `ConflictPending` | 有冲突待决策 | 调 `conflict list` |
| **4** | `NotInitialized` | 工作区未初始化 | 先 `kron init` |
| **5** | `WorkspaceNotFound` | 工作区路径不存在 | 检查路径 |
| **6** | `IoError` | 文件系统错误 | 检查权限/磁盘 |
| **7** | `Locked` | 文件/资源被锁 | 等待后重试 |
| **8** | `NotFound` | 资源不存在（task/conflict/project） | 检查 ID |
| **9** | `PermissionDenied` | 权限不足 | 提示用户 |
| **10** | `DaemonError` | 守护进程相关错误 | 看 `kron daemon status` |
| **64-78`** | 命令专属码 | 各命令自定义 | 见各命令章节 |

**注**：守护进程作为 `kron daemon start` 子命令的退出码（10-14）见 04 § 2.4；CLI 其他命令不会用 10-14，避免混淆。

**每种退出码的典型触发场景**：

| 码 | 典型触发命令 | stderr 输出形态 |
|----|------------|---------------|
| **0** | 任何命令成功执行 | （无 stderr；成功信息走 stdout） |
| **1** | 文件损坏、JSON 解析失败、未知内部错误 | `error: <message>` + `Run 'kron <subcmd> --help'` |
| **2** | `kron task add` 漏 `--title`、`kron task move` 缺 `--to`、`kron conflict resolve --use prompt` 但 stdin 不是 TTY | `error: missing required argument --title` |
| **3** | `kron task done` 但存在 `pending` 冲突阻止状态推进；`kron sync` 自动跑时遇冲突 | `error: 2 conflicts pending. Run 'kron conflict list'` |
| **4** | `kron status` / `kron task add` 在未初始化目录执行 | `error: kron not initialized. Run 'kron init'` |
| **5** | `kron --workspace /nonexistent path status` | `error: workspace not found: /nonexistent` |
| **6** | 磁盘满、权限拒绝（kron-internal 目录只读）、文件被外部进程占用 | `error: I/O error: <details>` |
| **7** | `kron task add` 时另一个 daemon 正在写 tasks.json（flock 失败） | `error: locked by another process. Retry in 1s` |
| **8** | `kron task show t-nonexist`、`kron task move t-deleted` | `error: task not found: t-nonexist` |
| **9** | Windows 上试图写 `C:\Windows\System32\`、kron-internal 不可写且用户拒绝授权 | `error: permission denied: <path>` |
| **10** | `kron daemon start` 时 socket 端口冲突、PID 文件已存在、依赖 watcher 启动失败 | `error: daemon start failed: <details>` |

---

## 7. 错误信息 schema

```rust
// 结构化错误（--json + stderr 共用）
{
  "code": "ConflictPending",     // 退出码语义名
  "exit_code": 3,                // 对应退出码
  "message": "1 conflict pending",  // 一句话
  "context": {
    "conflict_id": "c-abc123",
    "path": "KRON/important/notes.md"
  },
  "suggestion": "Run 'kron conflict show c-abc123' to inspect",  // 下一步
  "docs_url": "https://kron.dev/errors/conflict-pending"  // 可选
}
```

**约定**：
- `message` ≤ 80 字符
- `suggestion` 给可执行命令（不是泛泛"请联系管理员"）
- `context` 字段按错误类型动态扩展
- 人类可读模式下 `message + suggestion` 合并为一行
- `--json` 模式下：JSON 对象走 stderr（便于 shell pipeline），stdout 为空

**示例 1：`NotFound`（task 不存在）**

人类模式（stderr）：
```
error: task not found: t-abc123
hint: Run 'kron task list backend' to see available tasks
```

`--json` 模式（stderr）：
```json
{
  "code": "NotFound",
  "exit_code": 8,
  "message": "task not found: t-abc123",
  "context": { "task_id": "t-abc123", "vertex": "backend" },
  "suggestion": "Run 'kron task list backend'"
}
```

**示例 2：`ConflictPending`（提交 task 时存在冲突）**

人类模式（stderr）：
```
error: 1 conflict pending. Cannot move task t-001 to done.
hint: Run 'kron conflict list' to inspect, then 'kron conflict show c-xyz789'
```

`--json` 模式（stderr）：
```json
{
  "code": "ConflictPending",
  "exit_code": 3,
  "message": "1 conflict pending. Cannot move task t-001 to done.",
  "context": {
    "task_id": "t-001",
    "conflicts": ["c-xyz789"],
    "blocking_command": "task move"
  },
  "suggestion": "Run 'kron conflict list' to inspect"
}
```

---

## 8. Shell 补全策略

| Shell | 文件 | 安装命令 |
|-------|------|---------|
| **PowerShell** | `_kron.ps1` | `kron completions powershell > $PROFILE` |
| **Bash** | `_kron` | `kron completions bash > ~/.local/share/bash-completion/completions/kron` |
| **Zsh** | `_kron` | `kron completions zsh > "${fpath[1]}/_kron"` |
| **Fish** | `kron.fish` | `kron completions fish > ~/.config/fish/completions/kron.fish` |

**补全内容**：
- 子命令名
- 静态参数（`--json`, `--quiet` 等）
- 动态值：task ID、vertex 名、conflict ID（调内部接口列举）

**生成机制**：用 [`clap_complete`](https://crates.io/crates/clap_complete) 自动生成。

---

## 9. 配置加载顺序

```
优先级从高到低：
1. CLI 参数        例：kron --workspace /path/to/proj
2. 环境变量        例：KRON_WORKSPACE=/path/to/proj
3. kron-internal/config.json
4. 内置默认值
```

**环境变量命名**：`KRON_<KEY>`，全大写，下划线分隔。

**示例**：

| 来源 | 设置 | 生效命令 |
|------|------|---------|
| CLI | `kron --workspace /tmp/test` | 仅本次 |
| 环境变量 | `KRON_WORKSPACE=/tmp/test` | 整个 shell 会话 |
| config | `"workspace": "/tmp/test"` | 整个工作区 |
| 默认 | 当前目录 | — |

**配置覆盖**：`kron config set <key> <value>` 写 config.json；`kron config get <key>` 读。

### 9.1 `kron-internal/config.json` 详细清单（v1 锁定）

**文件位置**：`kron-internal/config.json`（与 `tasks.json` 同级）

**完整 schema**：
```json
{
  "version": 1,
  "project": {
    "name": "kron",
    "main_branch": "main",
    "work_dir": null
  },
  "daemon": {
    "auto_start": true,
    "log_level": "info",
    "pid_file_dir": null,
    "watch": {
      "kron_internal": true,
      "project_md": true,
      "git_head": true,
      "important_dir": true
    }
  },
  "context": {
    "auto_update": true,
    "files": {
      "git/recent-commits.md": { "enabled": true, "max_commits": 100 },
      "git/branch-summary.md": { "enabled": true },
      "code/structure.md": { "enabled": true, "max_depth": 3 }
    }
  },
  "important": {
    "default_link_mode": "symlink",
    "fallback_copy": true,
    "exclude_patterns": ["node_modules/", "target/", ".git/", "*.tmp"]
  },
  "ui": {
    "default_output_mode": "auto",
    "color": "auto",
    "pager": "auto",
    "table_style": "unicode"
  },
  "backup": {
    "enabled": true,
    "max_versions": 50,
    "dir": null
  }
}
```

**每个 key 说明**：

| Key | 类型 | 默认 | 说明 |
|-----|------|------|------|
| `version` | u32 | 1 | config schema 版本；用于 v2 迁移 |
| `project.name` | string | 取自 `package.json` / `Cargo.toml` / 目录名 | 项目显示名 |
| `project.main_branch` | string | `"main"`（回退 `"master"`） | branch-summary.md / 冲突检测的主分支基准 |
| `project.work_dir` | string \| null | null | 守护进程监听的额外目录（多工作区预留） |
| `daemon.auto_start` | bool | `true` | `kron` 命令执行时是否自动拉起 daemon |
| `daemon.log_level` | enum | `"info"` | `error` / `warn` / `info` / `debug` / `trace` |
| `daemon.pid_file_dir` | string \| null | null | 自定义 PID 目录（默认 OS 默认路径） |
| `daemon.watch.*` | bool | 全 `true` | 各类监听开关 |
| `context.auto_update` | bool | `true` | 文件变化时自动增量更新 |
| `context.files.<name>.enabled` | bool | `true` | 单个中间文档开关 |
| `context.files.git/recent-commits.md.max_commits` | u32 | 100 | recent-commits 显示条数 |
| `context.files.code/structure.md.max_depth` | u32 | 3 | tree 深度（0=无限） |
| `important.default_link_mode` | enum | `"symlink"` | `add` 时默认链接模式 |
| `important.fallback_copy` | bool | `true` | symlink 失败时降级为 copy |
| `important.exclude_patterns` | string[] | `["node_modules/", ...]` | add 时跳过的模式 |
| `ui.default_output_mode` | enum | `"auto"` | `auto` / `human` / `json` / `porcelain` |
| `ui.color` | enum | `"auto"` | `auto` / `always` / `never` |
| `ui.pager` | enum | `"auto"` | `auto` / `always` / `never` |
| `ui.table_style` | enum | `"unicode"` | `unicode` / `ascii` / `markdown` |
| `backup.enabled` | bool | `true` | 是否自动备份 |
| `backup.max_versions` | u32 | 50 | 保留的备份版本数 |
| `backup.dir` | string \| null | null | 自定义备份目录（默认全局 backups/） |

**修改方式**：
```bash
kron config get daemon.log_level
kron config set daemon.log_level debug
kron config list                  # 列出所有 key + value + 来源
kron config list --json           # AI 用
kron config reset <key>           # 恢复为内置默认
```

### 9.2 环境变量清单（`KRON_*`）

| 变量 | 对应 config key | 类型 | 说明 |
|------|----------------|------|------|
| `KRON_WORKSPACE` | `--workspace` | path | 等价 `--workspace`（优先级低于 CLI arg） |
| `KRON_CONFIG` | `--config` | path | 自定义 config.json 路径（高级用户） |
| `KRON_OUTPUT` | `ui.default_output_mode` | `json` / `porcelain` / `human` | 强制输出模式 |
| `KRON_NO_COLOR` | `ui.color=never` | 任意非空值 | 同 `NO_COLOR`（标准约定） |
| `KRON_NO_PAGER` | `ui.pager=never` | 任意非空值 | 禁用 pager |
| `KRON_LOG_LEVEL` | `daemon.log_level` | enum | 守护进程日志级别 |
| `KRON_DAEMON_SOCKET` | `daemon.pid_file_dir` 旁 | path | 自定义 socket/管道路径 |
| `KRON_AUTO_DAEMON` | `daemon.auto_start` | `0` / `1` | 是否自动启动 daemon |
| `NO_COLOR` | `ui.color=never` | 任意非空值 | 跨工具标准约定 |
| `CI` | 触发 § 4.3 优先级 4 | `true` | CI 环境识别 |

**优先级**（重申 § 4.5）：`CLI args > KRON_* > config.json > 内置默认 > NO_COLOR/CI（特殊）`

**特殊全局约定**：
- `NO_COLOR`（[no-color.org](https://no-color.org/)）——任意工具的标准约定，Kron 遵守
- `CI`（GitHub Actions / GitLab CI 通用约定）——强制非交互模式

## 10. 待讨论

| # | 问题 | 选项 | 倾向 |
|---|------|------|------|
| Q16 | `kron` 是否支持 `--dry-run`？ | A: 仅 conflict resolve 支持 B: 所有写操作都支持 | **A**（避免误改；只在写操作前给提示） |
| Q17 | `kron task add` 是否默认打开外部编辑器？ | A: 否（必须 --title） B: 是（无 --title 调 $EDITOR） | **A**（CLI 一致性） |
| Q18 | `kron` 是否支持 `git` 子命令别名？ | A: 否 B: 是 | **A**（不重复 Git 功能） |
| Q19 | `kron` 是否需要 `cd` 到工作区才工作？ | A: 必须 cd B: 支持全局 `--workspace` | **B**（更灵活） |
| Q20 | 命令分组是 `kron task list` 还是 `kron list task`？ | A: 动词在前 B: 名词在前 | **A**（与 Git 一致：`git branch list`）|
| **Q22** | 删除 vertex 时是否保留 description 进历史档？ | A: 否（与 task 一致） B: 是（写 `vertices-deleted.json`） | **A**（见 § 3.3）|
| **Q23** | `-m` 与 `--desc` 是否冲突？ | A: `-m` 是 `--message` 别名 B: 两者分别用：`-m` 短、`--desc` 长 | **A**（`--desc` 是 `--message` 的语义化别名，专门用于 task/vertex）|
| **Q24** | `task describe` 默认调编辑器还是必须 `-m`？ | A: 默认调 $EDITOR B: 必须 `-m` | **B**（AI 友好，不假设有交互式编辑器）|
| **Q25** | description 字段在 `--json` 里的命名？ | A: `description` B: `desc` C: `message` | **A**（完整单词，AI 读起来更明确）|
| **Q26** | `.kron-context/` v1 实现哪些中间文档？ | A: 全 8 个 B: 只 3 个（git/recent + tasks/active + README） C: 4 个（+ git/branch-summary） | **C**（见 § 3.7）|
| **Q27** | `important add` 重复文件行为？ | A: 报错 exit 8 B: 静默幂等 | **B**（AI 友好，见 § 3.4）|
| **Q28** | `.kron-context/` 是否包含"语义理解"内容？ | A: 只做事实型 B: 包含 LLM 生成的语义摘要 | **A**（不调 LLM、不接 API Key；语义靠 AI 自己读 `important/`）|
| **Q29** | `--json` + `--porcelain` 同时传？ | A: 报错 exit 2 B: `--json` 胜出（静默覆盖） | **A**（避免歧义，见 § 4.2）|
| **Q30** | pipe 到 `grep` 时是否自动退化为 `--porcelain`？ | A: 是（隐式智能） B: 否（保持显式） | **B**（简单可预测）|
| **Q31** | `task start` 是否作为 `task move --to doing` 的别名？ | A: 是（加 start） B: 否（只用 move + done） | **B**（只 `done` 别名；`start` 保留为 move 的通用能力）|

---

## 11. 相关文档

- [00-总览与架构.md](./00-总览与架构.md) § 6.1 CLI 设计原则
- [03 § 5.8 AI 友好约定](./03-双源同步机制.md) 退出码、错误信息、--json 详细定义
- [04-守护进程与文件监听.md](./04-守护进程与文件监听.md) `kron daemon` 命令实现
- [07-实施路线图.md](./07-实施路线图.md) § 3 P1 命令首批实现

---

## 附录 A：命令命名一致性检查

### A.1 操作动词清单（强约定，不允许漂移）

| 动词 | 含义 | 使用场景 | 示例 |
|------|------|---------|------|
| `list` | 列出集合 | 所有"集合 → 列表"操作 | `task list`, `vertex list`, `conflict list`, `important list` |
| `show` | 显示单个对象详情 | 跟 `list` 对偶，必须接 id/标识 | `task show <id>`, `vertex show <name>` |
| `add` | 新增一个实例到集合 | 子集合实例（task / important file） | `task add`, `important add` |
| `create` | 创建新顶层概念 | 顶层实体（vertex / project） | `vertex create`, `kron init`（隐式 create） |
| `delete` | 删除对象（kron-internal 删真文件） | 所有删除 | `task delete`, `vertex delete` |
| `move` | 改变对象归属/状态 | task 状态切换、important 跨目录 | `task move`, `important move`（v2 候选）|
| `start` | 进入活跃状态 | task 进入 doing 的语义别名 | `task start <id>` ≡ `task move <id> --to doing` |
| `done` | 进入完成状态 | task 进入 done 的语义别名 | `task done <id>` ≡ `task move <id> --to done` |
| `back` | 退回上一状态 | task 状态回退 | `task back <id>` |
| `attach` | 关联外部对象 | task 关联 commit / 文件 | `task attach`, `vertex attach`（v2） |
| `resolve` | 解决冲突 | conflict 专属 | `conflict resolve` |
| `ignore` | 标记跳过 | conflict / 警告忽略 | `conflict ignore` |
| `build` / `clean` | 目录生成 / 清空 | context 专属（**但实际不用，见 § 3.7 修正**） | `context clean` |
| `regenerate` | 全量重建 | context 专属 flag | `context --regenerate` |
| `start` / `stop` / `restart` / `status` | 进程生命周期 | daemon 专属 | `daemon start` 等 |
| `install` / `uninstall` | 服务注册 | daemon 专属 | `daemon install` 等 |
| `logs` | 查看日志 | daemon 专属 | `daemon logs` |
| `path` | 输出路径（探测用） | project 专属（AI 用） | `kron path --kron-root` |
| `config` | 配置读写 | project 专属 | `config get/set/list` |
| `help` | 帮助 | 全局 | `kron help <subcmd>` |
| `completions` | shell 补全脚本 | 全局 | `kron completions powershell` |

### A.2 一致性自检表

| 命名 | 命令 | 一致性 |
|------|------|-------|
| `kron task list` vs `kron vertex list` | 都是 `list` | ✅ |
| `kron task show` vs `kron conflict show` | 都是 `show` | ✅ |
| `kron task add` vs `kron important add` | 都是 `add` | ✅ |
| `kron vertex create` vs `kron task add` | `create` vs `add` | ✅ **Q21 已定**（create = 顶层；add = 实例） |
| `kron daemon start` vs `kron conflict resolve` | 动词不同 | ✅（语义不同） |
| `kron task done` vs `kron task move` | `done` vs `move` | ✅（done 是 move 语义别名） |

**Q21 已定**：
- `create`：用于"创建一个新概念"（vertex）
- `add`：用于"加一个实例"（task、important file）
- 别名：`task start` ≡ `task move --to doing`，`task done` ≡ `task move --to done`（Q31 待定：是否真做别名？倾向**只 `done` 别名，`start` 不用**——保留 move 的通用性）

### A.3 命名反模式（不允许出现）

| 反模式 | 错误示例 | 正确示例 | 原因 |
|--------|---------|---------|------|
| 复数命名 | `kron tasks list`, `kron vertices` | `kron task list`, `kron vertex` | 与 Git 一致 |
| 动词+er | `kron task lister`, `kron task adder` | 用 `list` / `add` | 命令动词化 |
| 嵌套动词 | `kron task add-new` | `kron task add` | 单一职责 |
| 模糊名 | `kron task process`, `kron task handle` | 用具体动词 | 语义模糊 |
| 拼音/中文 | `kron task 添加` | `kron task add` | 全英文（国际化） |

### A.4 命名变更规则

- v1 锁定后任何命名变更视为 **breaking change**
- 改名必须在 CHANGELOG 标 `BREAKING:`
- 保留旧名 1 个版本（`kron task add-new` 兼容旧版 `kron task add` 至少 1 版）
- **倾向保留区分**——语义不同

---

## 附录 B：未实现的命令（v2+ 候选）

- `kron sync` — 手动触发同步（v1 全自动；保留给调试）
- `kron watch` — 前台模式运行 watcher（调试用）
- `kron log` — sync/daemon 事件历史
- `kron reflog` — 误操作恢复（Git 风格）
- `kron stash` — 临时保存
- `kron bisect` — 找出引入问题的 commit
- `kron graph` — vertex 关系图

**v1 不实现**，避免功能膨胀。

---

**文档结束（v1）**

---

## 12. AI 工作流剧本（端到端）

**场景**：AI Agent 进入新项目，从零开始管理 task，全程用 `--json`。

**完整流程**（每步含期望输出）：

```bash
# Step 1: 检查是否初始化
$ cd ~/proj && kron status --json
# → exit 0（已初始化），或 exit 4（未初始化）

# Step 2: 未初始化则初始化（首次）
$ kron init --mode copy --json
# → exit 0

# Step 3: 查看项目根路径（AI 需要知道 kron-internal 在哪）
$ kron path --kron-root
# → /home/user/proj/kron-internal

# Step 4: 查看已有 vertex
$ kron vertex list --json
# → [{"name": "backend", "description": "...", ...}]

# Step 5: 添加 task（--json 返回新 task id）
$ kron task add backend --title "实现 OAuth 登录" -m "集成 GitHub OAuth，含 refresh token" --json
# → exit 0
# → {"id": "t-a1b2c3", "title": "...", "description": "...", "state": "todo", ...}

# Step 6: 列 doing 状态的 task（北极星验证场景）
$ kron task list backend --state doing --json
# → []

# Step 7: 移到 doing
$ kron task move t-a1b2c3 --to doing
# → exit 0

# Step 8: 列 doing 状态（应当含 t-a1b2c3）
$ kron task list backend --state doing --json
# → [{"id": "t-a1b2c3", "title": "实现 OAuth 登录", "state": "doing", ...}]

# Step 9: 关联 commit
$ kron task attach t-a1b2c3 --commit $(git rev-parse HEAD)
# → exit 0

# Step 10: 标记完成
$ kron task done t-a1b2c3
# → exit 0

# Step 11: 验证完成状态
$ kron task list backend --state done --json
# → [{"id": "t-a1b2c3", ..., "done_at": "2026-09-05T..."}]
```

**错误处理剧本**：

```bash
# 场景: 在未初始化目录跑命令
$ cd /tmp && kron task add backend --title "test"
# stderr: error: kron not initialized. Run 'kron init'
# exit: 4

# AI 重试（加 init）
$ kron init && kron task add backend --title "test" --json
```

**并发剧本**（多 AI 同时跑）：

```bash
# AI-1 启动
$ kron task add backend --title "task A" --json
# → {"id": "t-001", ...}

# AI-2 启动（短时间后）
$ kron task add backend --title "task B" --json
# → {"id": "t-002", ...}
# 不冲突：tasks.json 用文件锁串行化；退出码都 0

# 极端：两进程同时 move 同一 task
# AI-1: $ kron task move t-001 --to doing
# AI-2: $ kron task move t-001 --to done
# 后到的赢（last-write-wins）；前者可能返回 exit 0 但 state 实际被覆盖
# 解决：AI 自己加 retry + re-read 模式
```

**AI Agent SDK 模式（v2+ 候选）**：

为避免 AI 反复 fork 进程，v2 可提供 `kron mcp` 子命令，通过 MCP server 暴露给 AI（详见 Q9）。
