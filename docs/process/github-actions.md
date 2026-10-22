# GitHub Actions 运行手册

> 本文件是**实施流程**，不是架构真理。
> 规则清单（哪些是门禁、哪些是盲区）在 [`ci-enforcement.md`](./ci-enforcement.md)；
> 本文件只讲**这套 CI 怎么跑、怎么看、怎么改**。
> 受众：第一次接触 GitHub Actions 的维护者，以及需要操作/解释 CI 的 AI agent。

---

## 1 心智模型

一句话：**你在仓库里放一个 YAML，GitHub 收到某个"事件"后，开一台全新的虚拟机，按 YAML 执行命令，再把结果回报到对应的 commit 上。**

层级（越往下越细）：

```
仓库
└─ .github/workflows/ci.yml        ← workflow（一个 YAML = 一个 workflow）
   └─ 一次触发 = 一次 run          ← 例："某 commit 开了 PR"
      └─ run 里 N 个 job           ← 目前只有 1 个：verify
         └─ 每个 job 里若干 step   ← checkout → setup-go → gofmt → build → vet → import → test
            └─ 每个 step 有日志
   └─ 每个 job 结束产出 1 个 check run   ← commit / PR 旁边那个 ✓ / ✗
```

| 英文 | 中文习惯叫法 | 本仓库实例 |
|---|---|---|
| workflow | 工作流（YAML 文件） | `ci.yml`，名 `CI` |
| event / trigger | 触发事件 | PR、push 到 master、`workflow_dispatch` |
| run | 一次运行记录 | 有编号，可在 Actions 页看 |
| job | 运行内的工作单元 | 1 个：`verify (windows-latest)` |
| step | job 内单个步骤 | 7 个（见 §2） |
| runner | GitHub 提供的虚拟机 | 只用 `windows-latest` |
| action | 别人写好的可复用步骤 | `actions/checkout@v4`、`actions/setup-go@v5` |
| check run | 汇报到 commit 上的结果 | PR 页面 / commit 旁的 ✓ ✗ |
| cache | 跨运行复用的文件包 | setup-go 缓存的 Go 模块目录 |

---

## 2 当前 workflow

文件：[`.github/workflows/ci.yml`](../../.github/workflows/ci.yml)（仓库里**唯一**的 workflow）

```yaml
name: CI

on:
  pull_request:                    # PR 上跑（这是门禁的主场）
  push:
    branches: [master]             # 合并进 master 后再跑一次
  workflow_dispatch:               # 也可以手动点

concurrency:                       # 同分支连推 → 取消上一轮
  group: ci-${{ github.ref }}
  cancel-in-progress: true

permissions:
  contents: read                   # 只读仓库，最小权限

jobs:
  verify:
    name: verify (windows-latest)  # ← 这个名字就是 required check 的标识
    runs-on: windows-latest
    timeout-minutes: 15            # 卡住最多 15 分钟（GitHub 默认 360）
    steps:
      # 1 拉代码    2 装 Go（版本读 go.mod）+ 恢复模块缓存
      # 3 gofmt -l cmd internal frontend 必须无输出
      # 4 go build ./...
      # 5 go vet ./...
      # 6 import 边界（内部访问层不互调，架构 §2.2）
      # 7 重点包测试（见 §2.2）
```

### 2.1 平台：只有 Windows（2026-10-22 决定）

维护者只在 Windows 上开发，Linux / macOS 暂不考虑 → matrix 已删除，只保留 `windows-latest`。

**代价（记录在案）**：`internal/store` 里 `//go:build darwin || linux` 的 `flock` 实现
（`intent_lock_unix.go`）**不再被 CI 验证**。如果以后要重新覆盖，恢复一个 matrix 即可：

```yaml
strategy:
  matrix:
    os: [ubuntu-latest, macos-latest]
runs-on: ${{ matrix.os }}
```

### 2.2 测试范围：只跑"重点包"，不是全量

```bash
go test ./internal/store ./internal/parser ./internal/lint ./internal/relations
```

选这四个的理由：**它们承载数据格式与写入的真理**（frontmatter 解析、写入/锁/原子性、
lint 规则、引用关系），并且都在 2 秒内。实测（本机 `-count=1`）：

| 包 | 耗时 | 是否在门禁里 |
|---|---|---|
| `internal/store` | 1.7s | ✅ |
| `internal/parser` | 0.9s | ✅ |
| `internal/lint` | 0.9s | ✅ |
| `internal/relations` | 0.7s | ✅ |
| `internal/model` / `identity` / `view` / `assumption` | 0.7–1.9s | ❌ 便宜，可加 |
| `cmd/kron/cli` | 1.7s | ❌ 可加 |
| `cmd/kron/serve-lsp` | 1.0s | ❌ 可加 |
| **`cmd/kron/serve-mcp`** | **31.9s** | ❌ 这就是"全量"太慢的主因 |
| 两个 `e2e` 套件 | 7s / 7s | ❌ |

要加减包，只改 `ci.yml` 里 `重点包测试` 那一行的列表。

### 2.3 为什么 gofmt 在 Windows runner 上也可靠

`.gitattributes` 强制 `* text=auto eol=lf`，显式 `eol` 优先于 `core.autocrlf=true`，
所以即使跑在 Windows 上，检出的 Go 源码也是 LF → `gofmt -l` 只看"真的没格式化"，
不会因为行尾误报。（历史上本机曾因 CRLF 误报 74 个文件，就是 `.gitattributes` 之前是坏行
`noeol=lf` 导致的。）

扫描范围是 `gofmt -l cmd internal frontend`，**不含** `kron-mcp-test/`：那里是 MCP 测试
fixture（`project/`、`go-client/`、`raw-probe/` 各有自己的 `go.mod`，内容是故意不规范的
样例源码），要求它们"格式化"没有意义。

---

## 3 触发矩阵

| 你做的事 | 会不会跑 | 原因 |
|---|---|---|
| 开 PR / 往 PR 推 commit | ✅ | `on.pull_request` |
| push（含 merge）到 master | ✅ | `on.push.branches: [master]` |
| push 到功能分支（非 PR） | ❌ | push 只匹配 master |
| Actions 页点 **Run workflow** | ✅ | `workflow_dispatch` |
| 改 workflow 文件本身 | ✅ 会（PR 用改后的版本） | `pull_request` 取 merge ref 的文件 |
| 手动 re-run 某次运行 | ✅ | Actions 页面按钮 |

> `pull_request` 事件用的是 **PR 的 merge ref**（base + head 合并后的树）：改 workflow 的 PR
> 跑的是**改后**的版本；没改 workflow 的 PR 跑的是 **master 上**的版本。

---

## 4 一次运行长什么样

单 job、7 个 step、串行。典型耗时（GitHub windows-latest，含装 Go）：

| step | 典型耗时 | 说明 |
|---|---|---|
| checkout | 2–5s | 拉代码 |
| setup-go | 15–25s | 装 Go 1.27 + 恢复模块缓存 |
| gofmt | <1s | 只扫描 |
| go build | 20–40s | 首次编译全部包（无 build cache） |
| go vet | 10–20s | 复用 build 结果 |
| import 边界 | 3–5s | `go list -deps` |
| 重点包测试 | 10–20s | 编译 + 跑 4 个包的测试 |
| **合计** | **约 1.5–2 分钟** | 远低于 15 分钟上限 |

**大部分时间在"准备环境 + 编译"，不在测。** 这也是为什么不把 32 秒的 `serve-mcp` 塞进来。

---

## 5 在哪里看

| 想看什么 | 位置 |
|---|---|
| 全部运行历史 | 仓库顶部 **Actions** → 左侧 `CI` |
| 某次运行日志 | Actions → 点 run → 点 `verify (windows-latest)` → 展开每个 step |
| 某个 commit 的结果 | commit 右侧图标（✓ / ✗ / 灰）；点开是 1 个 check |
| PR 的检查 | PR 页面底部 Checks 区（有 `verify (windows-latest)` 这一条） |
| 缓存占用 | Settings → Actions → Caches |
| workflow 定义 | `.github/workflows/ci.yml` |

### 手动跑一次

```
仓库 → Actions → 左侧 "CI"
     → 右上 "Run workflow" → 选分支（默认 master）→ Run workflow
```

约 2 分钟出结果。

---

## 6 费用与资源

| 项目 | 现状 |
|---|---|
| 单次运行 | 约 1.5–2 分钟 |
| 缓存 | setup-go 的 Go 模块缓存（每平台一份，约 28–30 MB；7 天不访问自动删除；单仓上限 10 GB） |
| artifacts | 0（本 workflow 不产 artifact） |
| **费用** | **$0** —— public 仓库 + 标准 runner，既不计费也不占 2000 分钟配额 |

> 公开仓库用标准 GitHub-hosted runner 的分钟数**永久免费**；只有 **larger runner** 才会
> 「即使 public 也收费」。

---

## 7 修改 workflow 的权限与流程

**只有 `@zhangjun005`（repo admin）有资格调整门禁**——红线写在 [`AGENTS.md`](../../AGENTS.md) §3，
硬性落实在 GitHub ruleset `protect-master`：

| 保障 | 机制 |
|---|---|
| 设置本身只有 admin 能改 | ruleset / 分支保护 / 协作者管理都要求 admin |
| 别人改 workflow 必须你 review | `Require a pull request` + `Require review from Code Owners`（`CODEOWNERS` 的 `* @zhangjun005` 覆盖 `.github/workflows/`） |
| 禁止强推 / 删分支 | ruleset 的 `non_fast_forward` + `deletion` |
| 自己能绕过 | bypass list = `Repository admin`，`bypass_mode = always`（**有意保留**：允许 owner 直推 master） |

### 想让 check 真正"挡合并"

ruleset 目前**没有** required status checks → 检查红了只提示、不阻塞。要变成真门禁：

1. Settings → Rules → `protect-master` → 勾 **Require status checks to pass**
2. 搜索并加入 **`verify (windows-latest)`**
3. 之后该 check 未通过（或未出现）就不能合并

⚠️ 注意两个坑：

- 一旦设为 required，**workflow 被禁用/删除** 会让 PR 永久卡在 `Expected — waiting for status to be reported`
- 手动 `Re-run` 只影响那一次运行的结论，不会"补"到已合并的 PR 上

---

## 8 常见疑问

**Q：`cancelled` 和 `failure` 有什么区别？**
`failure` = 命令真的返回非 0；`cancelled` = 被取消（人工 / `concurrency` 顶掉 / 排队超时）。
两者都可能显示成红叉，但含义完全不同。历史上 macOS arm64 曾出现「排队 15 分钟后被取消」，
annotation 原文是 `The job was not acquired by Runner of type hosted even after multiple attempts`，
**不是代码问题**——现在只用 Windows runner，这类问题不会再出现。

**Q：本地过了，CI 为什么还要跑？**
本地跑的是你的环境 + 你的工作区状态（可能 CRLF、可能没提交）；CI 跑的是**干净检出**，
能抓到「忘了提交新文件」「依赖没写进 go.mod」「行尾/权限问题」这类本地看不见的问题。

**Q：`concurrency` 会不会误取消正常的运行？**
只有**同一分支**短时间内多次触发才会。连推两次同一分支，第一次会显示 cancelled。

**Q：`timeout-minutes` 和"排队被取消"是一回事吗？**
不是。`timeout-minutes` 管 **job 开始执行后**能跑多久；排队阶段由 GitHub 调度决定，YAML 控制不了。

**Q：为什么 `gofmt` 在 CI 里能过，本机却报一堆？**
本机工作区是 CRLF（历史遗留 + `core.autocrlf=true`）。修法二选一：
`git config core.autocrlf false` 后重新检出，或直接 `git checkout-index -a -f` 用仓库里
的 LF 内容覆盖工作区。`.gitattributes` 的 `eol=lf` 已保证**新检出**是 LF。

**Q：以后想加别的检查怎么办？**
先更新 [`ci-enforcement.md`](./ci-enforcement.md)（规则清单是真理），再改 `ci.yml`。
public 仓免费，成本只有时间。

---

## 9 与其他文档的关系

| 文档 | 职责 |
|---|---|
| [`ci-enforcement.md`](./ci-enforcement.md) | **哪些检查存在 / 哪些还缺 / 哪些是盲区** |
| 本文件 | 这套 CI **怎么运行、怎么观察、怎么改** |
| [`AGENTS.md`](../../AGENTS.md) §3 | 门禁调整权的红线 |
