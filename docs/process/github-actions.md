# GitHub Actions 运行手册

> 本文件是**实施流程**，不是架构真理。
> 规则清单在 [`ci-enforcement.md`](./ci-enforcement.md)；本文件只讲**这套 CI 怎么跑、怎么看、怎么改**。
> 受众：第一次接触 GitHub Actions 的维护者，以及需要操作/解释 CI 的 AI agent。

---

## 1 心智模型

一句话：**你在仓库里放一个 YAML，GitHub 收到某个"事件"后，开一台全新的虚拟机，按 YAML 执行命令，再把结果回报到对应的 commit 上。**

层级（越往下越细）：

```
仓库
└─ .github/workflows/store-platform.yml   ← workflow（一个 YAML = 一个 workflow）
   └─ 一次触发 = 一次 run                 ← 例："7c13fe9 push 到 master"
      └─ run 里 N 个 job                  ← matrix 展开：每个 OS 一个 job
         └─ 每个 job 里若干 step          ← checkout → setup-go → go test
            └─ 每个 step 有日志
   └─ 每个 job 结束产出 1 个 check run    ← commit / PR 旁边那个 ✓ / ✗
```

| 英文 | 中文习惯叫法 | 本仓库实例 |
|---|---|---|
| workflow | 工作流（YAML 文件） | `store-platform.yml`，名 `Store platform tests` |
| event / trigger | 触发事件 | push 到 master、`workflow_dispatch` |
| run | 一次运行记录 | 如 `39753176717` |
| job | 运行内并行的工作单元 | 3 个（每平台一个） |
| step | job 内单个步骤 | checkout / setup-go / go test |
| runner | GitHub 提供的虚拟机 | `ubuntu-latest` / `macos-latest` / `windows-latest` |
| matrix | 一个 job 定义展开成多份 | `os: [3 个值]` → 3 个 job |
| action | 别人写好的可复用步骤 | `actions/checkout@v4`、`actions/setup-go@v5` |
| check run | 汇报到 commit 上的结果 | PR 页面 / commit 旁的 ✓ ✗ |
| cache | 跨运行复用的文件包 | setup-go 缓存的 Go 模块目录 |

---

## 2 当前 workflow

文件：[`.github/workflows/store-platform.yml`](../../.github/workflows/store-platform.yml)（仓库里**唯一**的 workflow）

```yaml
name: Store platform tests          # 显示名，出现在检查列表里

on:                                 # 何时自动跑
  push:
    branches: [master]              #   仅 push 到 master
  workflow_dispatch:                #   或在 Actions 页手动点

concurrency:                        # 同分支连推 → 取消上一轮，避免排队堆积
  group: store-platform-${{ github.ref }}
  cancel-in-progress: true

jobs:
  store:
    name: internal/store (${{ matrix.os }})
    runs-on: ${{ matrix.os }}
    timeout-minutes: 15             # 每个 job 最多 15 分钟（GitHub 默认 360）
    strategy:
      fail-fast: false              # 一个平台失败，另外两个照样跑完
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
    steps:
      - uses: actions/checkout@v4           # 拉代码到那台虚拟机上
      - uses: actions/setup-go@v5           # 装 Go（版本读 go.mod → 1.27）+ 恢复缓存
        with:
          go-version-file: go.mod
          cache: true
      - name: Test store package
        run: go test ./internal/store       # 跑 internal/store 的 100 个测试
```

### 2.1 为什么是这三个平台

`internal/store` 的锁实现按 OS 分文件、由 build tag 互斥：

| 文件 | build tag | 系统调用 |
|---|---|---|
| `intent_lock_unix.go` | `darwin \|\| linux` | `syscall.Flock` |
| `intent_lock_windows.go` | `windows` | `LockFileEx` |

本机只有 Windows，`flock` 那条分支只能在真 Linux / macOS 上验证 → 需要三平台。

### 2.2 现状定位：**非门禁**

- ruleset `protect-master` 里**没有** required status checks → 这个 workflow 的结果**不挡合并**
- 它的作用是「合并后的跨平台体检」：master 前进后自动跑一次，红了只提示、不阻塞
- 想改成真门禁：在 ruleset 里加 required status checks（见 §7）

---

## 3 触发矩阵

| 你做的事 | 会不会跑 | 原因 |
|---|---|---|
| push（含 merge）到 master | ✅ 跑 1 次（3 job） | `on.push.branches: [master]` |
| push 到功能分支 | ❌ 不跑 | branches 只列了 master |
| 开 / 更新 PR | ❌ 不跑 | 没有 `on.pull_request` |
| Actions 页点 **Run workflow** | ✅ 跑 | `workflow_dispatch` |
| 改 workflow 文件本身 | 不因此触发 | 要等合并或手动 |
| 往**仍带旧文件的分支**推 commit | ⚠️ 会跑 | 老分支里的文件还是旧触发条件 |
| 手动 re-run 某次运行 | ✅ 跑 | Actions 页面按钮 |

> `pull_request` 事件用的是 **PR 的 merge ref**（base + head 合并后的树），不是 head 分支本身。
> 所以一个改 workflow 的 PR 跑的是**改后的**版本；一个没改 workflow 的 PR 跑的是 **base（master）上的**版本。

---

## 4 一次运行长什么样

以 `39753176717`（PR #5 合并后触发）为例，三个 job 并行，总墙钟 **44s**：

| job | 耗时 | 结果 |
|---|---|---|
| internal/store (ubuntu-latest) | 17s | ✅ |
| internal/store (windows-latest) | 38s | ✅ |
| internal/store (macos-latest) | 40s | ✅ |

ubuntu 那个 job 的 step 时间线：

```
1. Set up job                   1s   ← 分配虚拟机
2. Run actions/checkout@v4      1s   ← 拉代码
3. Run actions/setup-go@v5     10s   ← 装 Go + 恢复缓存（占总耗时大头）
4. Test store package           1s   ← 真正的 go test
7. Post Run actions/setup-go    0s   ← 收尾（保存缓存 / 清理）
8. Post Run actions/checkout    0s
9. Complete job                 0s
```

结论：**大部分时间在"准备环境"，不在测。** macOS 慢是因为自带编译开销（test 17s）。

---

## 5 在哪里看

| 想看什么 | 位置 |
|---|---|
| 全部运行历史 | 仓库顶部 **Actions** → 左侧选 workflow |
| 某次运行日志 | Actions → 点 run → 点 job → 展开每个 step |
| 某个 commit 的结果 | commit 右侧图标（✓ / ✗ / 灰）；点开是每个 job 一个 check |
| PR 的检查 | PR 页面底部 Checks 区（现在新 PR 会是空的） |
| 缓存占用 | Settings → Actions → Caches（当前 17 条 / 约 480 MB） |
| workflow 定义 | `.github/workflows/store-platform.yml` |

### 手动跑一次

```
仓库 → Actions → 左侧 "Store platform tests"
     → 右上 "Run workflow" → 选分支（默认 master）→ Run workflow
```

约 1 分钟出结果。**这是以后需要验证 macOS / Linux 行为时的推荐方式。**

---

## 6 费用与资源

| 项目 | 现状 |
|---|---|
| 单次运行 | 约 44s |
| 历史累计（14 次运行） | 约 63 个计费分钟 |
| 缓存 | 17 条 / 约 480 MB（7 天不访问自动删除；单仓上限 10 GB） |
| artifacts | 0（本 workflow 不产 artifact） |
| **费用** | **$0** —— public 仓库 + 标准 runner，既不计费也不占 2000 分钟配额 |

> 公开仓库用标准 GitHub-hosted runner 的分钟数**永久免费**；只有 **larger runner** 才会
> 「即使 public 也收费」。归档依据见 `ci-enforcement.md`。

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

### 想改成真门禁（required check）时

1. 在 ruleset `protect-master` 里加 **Require status checks to pass**
2. 把要卡的 job 名加进去（名字 = workflow 名 / job 名，如 `Store platform tests / internal/store (ubuntu-latest)`）
3. 注意：加了 required check 后，PR 上**必须**出现该 check 且通过才能合并；
   一旦某个 workflow 被禁用/删除，PR 会永久卡在 `Expected — waiting for status to be reported`

---

## 8 常见疑问

**Q：`cancelled` 和 `failure` 有什么区别？**
`failure` = 命令真的返回非 0；`cancelled` = 被取消（人工 / 排队超时 / `concurrency` 顶掉）。
两者都可能显示成红叉，但含义完全不同。macOS 曾出现「排队 15 分钟后被取消」：
annotation 原文 `The job was not acquired by Runner of type hosted even after multiple attempts`
（arm64 容量紧张），**不是代码问题**，重跑即可。

**Q：本地 `go test` 过了，为什么 CI 还要跑？**
CI 跑的是**另一个操作系统**，编译出的是另一套 build tag 的代码（见 §2.1）。
本地过 = Windows 分支过；只有 CI 能验证 `flock` 那条分支。

**Q：`concurrency` 会不会误取消正常的运行？**
只有**同一分支**短时间内多次触发才会。连推两次 master，第一次会显示 cancelled——因为不是门禁，没有实际影响。

**Q：`timeout-minutes` 和"排队 15 分钟被取消"是一回事吗？**
不是。`timeout-minutes` 管的是 **job 开始执行后**能跑多久；排队阶段由 GitHub 的
runner 调度策略决定，YAML 控制不了。

**Q：以后想加别的检查怎么办？**
先更新 [`ci-enforcement.md`](./ci-enforcement.md)（规则清单是真理），再改 workflow。
public 仓免费，所以成本只有时间。

---

## 9 与其他文档的关系

| 文档 | 职责 |
|---|---|
| [`ci-enforcement.md`](./ci-enforcement.md) | **哪些检查存在 / 哪些还缺**（规则清单 + 盲区） |
| 本文件 | 这套 CI **怎么运行、怎么观察、怎么改** |
| [`AGENTS.md`](../../AGENTS.md) §3 | 门禁调整权的红线 |
