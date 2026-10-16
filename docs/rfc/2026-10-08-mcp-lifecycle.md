# RFC: MCP 进程生命周期与并发安全 (kron serve-mcp 拍板)

| 字段 | 值 |
|---|---|
| **状态** | 草案 (Proposed) — 待 zhangjun005 拍板 |
| **作者** | AI assistant, 经 zhangjun005 委托 |
| **创建日期** | 2026-10-08 |
| **目标版本** | v1.0 (架构拍板) + v1.1 (并发锁实现) |
| **影响范围** | `docs/abstractDesign/architecture.md` §一 增补 / `docs/implementation/mcp.md` §6 (新) / `internal/store/writer.go` (增 flock) / `docs/process/pending-decisions.md` (关 F1) |

---

## 1. 动机 (2026-10-08 拍板)

### 1.1 用户问题 (口头)

> "一个项目一个 mcp, 提供给人的是 GUI 和 IDE, 怎么保证不打架?"

### 1.2 现实情况

- **MCP 协议本身** = stdio JSON-RPC = **1-to-1 长连接**, **不**支持多 client 共享一个 server 进程
- 一个 AI 工具 (Claude Desktop / Cursor AI) 一次 session = 一次 `spawn "kron serve-mcp"` + 一次 stdio pipe + session 结束 kill
- 一个 VSCode 扩展 = 一次 `spawn "kron serve-mcp"` + VSCode 生命周期内复用
- 一个 Wails GUI = 一次 `spawn "kron serve-mcp"` + Wails 窗口生命周期内复用

**也就是说**：

| 客户端 | spawn 次数 | MCP 进程寿命 |
|---|---|---|
| Claude Desktop | 1 次/session | AI session 寿命 |
| Cursor AI | 1 次/session | AI session 寿命 |
| VSCode 扩展 | 1 次 | VSCode 窗口寿命 |
| Wails GUI | 1 次 | Wails 窗口寿命 |

**一个项目 = 同时可能存在 N 个 MCP 进程** (N = 客户端数)。

### 1.3 关键澄清: "一个项目一个 MCP" 的真实含义

| 误解 | 真相 |
|---|---|
| ❌ 全局只跑一个 MCP daemon | ✅ 每个 client (VSCode / Wails / AI) 各自 spawn 一个 `kron serve-mcp`, 各自管理 |
| ❌ 多次 `kron_init` 工具调用 = 多次 spawn | ✅ 一次 session 一次 spawn, 之后**复用 stdio pipe** |
| ❌ 需要 HTTP daemon / Unix socket / 跨进程协调 | ✅ 不需要 — stdio pipe 由 client SDK 管 |

### 1.4 "打架" 是什么?

**问题** = **多 MCP 进程同时写同一 `.kron/intents/<slug>.md`**:

```
时间线:
  t1: VSCode 扩展的 MCP-A 调 `update("auth/jwt.md", 内容A)`
  t2: Wails 的 MCP-B 同时调 `update("auth/jwt.md", 内容B)`

无锁 = 后写者覆盖 → 内容A 丢失
有锁 = t2 等 t1 完成 → 内容B 落, 内容A 已经在 t1 之前被读取
```

**MCP 协议不防** (它只管 "JSON-RPC 格式 + 长连接"); **OS 进程隔离不防** (多进程都跑同一文件); **防打架靠 `internal/store` 显式加锁**。

---

## 2. 拍板方案

### 2.1 MCP 进程生命周期 = stdio 父进程寿命

```
AI 工具启动
  → child_process.spawn("kron", ["serve-mcp"])   (stdio: pipe)
  → JSON-RPC initialize handshake
  → tools/list, tools/call × N
  → AI 工具退出
  → stdio pipe 关闭 → kron 进程 exit (code 0)
```

**`kron serve-mcp` 是无状态进程**:
- **不**维护全局单例
- **不**监听 TCP / Unix socket
- **不**写 PID 文件
- **不**做 `flock` 自我协调
- **不**需要"启动时检查已有实例"

### 2.2 并发安全 = `internal/store` 按逻辑 intent 加 OS 文件锁

每个写操作都按已验证的 slug 对同一个稳定 sidecar lock file 获取排他 OS 锁。不能直接锁 intent 文件本身：`writeFileAtomic` 用 rename 替换 inode，替换后新进程可能锁到新 inode，从而绕过仍持有旧 inode 锁的进程。

```go
// internal/store/intent_lock.go — stable path, independent from atomic renames
func (w *Writer) lockIntent(slug string) (unlock func(), err error) {
    key := sha256.Sum256([]byte(slug))
    path := filepath.Join(w.Root, ".kron", ".lock", hex.EncodeToString(key[:]) + ".flock")
    if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
        return nil, fmt.Errorf("mkdir lock directory: %w", err)
    }
    f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
    if err != nil {
        return nil, fmt.Errorf("open: %w", err)
    }
    if err := lockOSFile(f); err != nil {
        _ = f.Close()
        return nil, fmt.Errorf("lock: %w", err)
    }
    return func() { _ = unlockOSFile(f); _ = f.Close() }, nil
}
```

**锁粒度** = **每个逻辑 intent 一把稳定 sidecar 锁**（跨进程可见；leaf 与 `README.md` 两种物理形态共用同一 slug 锁）：

```
MCP-A: lock(".kron/.lock/<slug-hash>.flock") → read/modify/rename → unlock
MCP-B: lock(".kron/.lock/<slug-hash>.flock") → 等 → 锁释放 → read/modify/rename → unlock
```

sidecar 文件在进程退出后保留，只释放 OS 锁，不删除文件；删除后并发进程可能分别锁住不同 inode。**不锁整目录**（粒度太粗，会阻塞其他 slug 写）。Windows 用 `LockFileEx`，macOS/Linux 用 `flock`，不增加 Go 依赖。

### 2.5 与现有 `writeFileAtomic` 的整合

`internal/store/writer.go` L167-202 `writeFileAtomic` **已**是 temp+rename 原子写, 但**不**跨进程防并发 (rename 在 OS 层原子, **多进程**同时打开 + rename 仍 race). 拍板:

- `Write` / `Update` / `MoveToTrash` / `RestoreFromTrash` 在读改写或 rename 的完整事务外层按 slug 加锁，**不**改方法签名
- lock file 位于 `.kron/.lock/`，由 slug 的 SHA-256 派生稳定文件名；覆盖同 slug 的 leaf / README 形态
- `writeFileAtomic` **不**重命名 (向后兼容既有测试期望, 如 `writer_test.go` 直接测该 helper)
- 写方法内的 `_ = ctx` **保持不变**（本任务不改变公开 API；锁等待不承诺 context 取消）

### 2.3 影响面

| 改动 | 风险 |
|---|---|
| `internal/store` 四个写方法 (`Write` / `Update` / `MoveToTrash` / `RestoreFromTrash`) 全加 OS 文件锁 | 低 — 使用 Go 标准库系统调用，不增依赖 |
| 写方法签名**不**变 (仍 `ctx context.Context` 在前) | 兼容 |
| `Read` **不**加锁 (rename 原子 + flock 阻塞写 → 读到的是**全**或**没**, 不存在半写) | 中 — 需 lint 强制 "Read 必须发生在 Write 完成" |
| 行为变化: 同 slug 的写请求等待前一个事务释放锁；OS 锁获取失败时返回带上下文的错误 | 低 — 既有访问层错误路径处理该错误 |

### 2.4 不做的事 (架构铁律合规)

| 不做 | 原因 |
|---|---|
| ❌ **不**加 MCP daemon / HTTP server | 违反 `architecture.md` §〇 铁律 #6 "二进制**不**持有项目数据" |
| ❌ **不**加 Unix socket | 违反铁律 #6 "不维护状态机" |
| ❌ **不**加 PID 文件 | 违反铁律 #7 "不靠运行时隐式状态" |
| ❌ **不**加 `--path` flag (MCP daemon 模式) | 违反 iron rule "新增 CLI flag 走 `cli-flag.md` 流程" |
| ❌ **不**做"启动时单例检查" (`pgrep`) | 违反铁律 #6 #7 |
| ✅ **只**加 per-intent sidecar OS 文件锁 (无数据库/daemon/应用层锁状态) | 符合铁律 #6 #7 (锁状态由 OS 管, **不**由 Kron 进程管) |

---

## 3. 实现路径

### 3.1 Phase 1 (v1.0 拍板, 本 RFC 落地后立即)

- [x] 拍板 "MCP 进程寿命 = stdio 父进程寿命, 不做 daemon"
- [x] 拍板 "并发安全 = `internal/store` flock"
- [x] 增补 `architecture.md` §一 (拍板内容)
- [x] 增补 `implementation/mcp.md` §6 (MCP 寿命说明)

### 3.2 Phase 2 (v1.1 实施, 1-2 周)

- [x] `internal/store` 加 stable per-intent sidecar lock (覆盖 Write / Update / MoveToTrash / RestoreFromTrash), 不改公开 API
- [x] 不新增 `ErrConcurrentWrite`：同 slug 写入等待锁；仅锁系统错误返回普通包装错误
- [x] `internal/store/writer_test.go` 增跨 Writer 与跨进程锁测试 (`go test -race`)
- [x] 不增加 `ErrConcurrentWrite`，因此不修改 error catalog
- [x] 锁错误沿用 `serve-mcp` 的一般内部错误映射
- [x] `_ = ctx` 行保持不变（锁等待不承诺 context 取消；本任务不改变公开 API）

### 3.3 Phase 3 (v1.2+ 优化, 可选)

- [ ] `lint` 加规则: 读 .md 后**未**等待写完成 → 警告 (理想情况 read 也要排队)

---

## 4. 与现有 RFC / 文档的关系

| 文档 | 关系 |
|---|---|
| [`docs/abstractDesign/architecture.md`](../abstractDesign/architecture.md) §一 | **本 RFC §2 拍板后**, 增补 "MCP 进程寿命 = stdio 父进程寿命" 一段 |
| [`docs/implementation/mcp.md`](../implementation/mcp.md) §5 (GUI API 边界 — 已 2026-10-08 作废) | §6 (新): "MCP 寿命与并发" — 本 RFC §2 简化版 |
| [`docs/process/cli-flag.md`](../process/cli-flag.md) | **不**触发 — 本 RFC **不**新增 `--path` flag |
| [`docs/process/new-internal-api.md`](../process/new-internal-api.md) | **触发** — `internal/store` flock API 是"2 个访问层 (MCP + CLI) 共享", 必走 RFC |
| [`docs/process/pending-decisions.md`](../process/pending-decisions.md) | 关 F1 ("一个项目一个 MCP 怎么管?") — 拍板**已**定, 转 Phase 2 实施 |

---

## 5. 关键决策 (本 RFC 拍板后**不可**回退的)

1. **MCP 进程寿命** = stdio 父进程寿命 (单实例 = 复用, 多实例 = 文件锁保正确)
2. **并发安全** = `internal/store` `flock` 粒度 = per-file
3. **不**做 daemon / socket / PID / 单例检查
4. **v1.1 必交付** flock 实现 + 并发测试 (`go test -race`)
5. **客户端层** (VSCode 扩展 / Wails / AI 工具) **不**做"单例检查" — `kron serve-mcp` **不**接受 `--path` flag, **不**做启动期 pgrep
