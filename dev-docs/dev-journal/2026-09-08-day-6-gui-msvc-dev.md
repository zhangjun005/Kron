# Day 6 (下午续) — Tauri dev 首次在 MSVC 工具链下跑起来

> 日期：2026-09-08 ~16:00
> 范围：修复 MSVC toolchain 持久化 + 端口占用清理 + 成功启动 Tauri dev

## TL;DR

GUI 骨架代码（Day 6 上午）早已写完，但**没法本地跑通** —— 链接器是 `x86_64-w64-mingw32-gcc`，
而项目用 `windows-sys` 系列依赖指向 MSVC toolchain。今天花了一个 session 调到能弹窗。

## 背景

### 问题链

1. `cargo build` → 报 `linking with x86_64-w64-mingw32-gcc failed`
   - 一堆 `.drectve -exclude-symbols:... unrecognized` 警告
   - `Warning: corrupt .drectve at end of def file`
   - 最后 `ld` 找不到 MSVC-style import lib（如 `windows.0.52.0.lib`）

2. 根本原因：`rustup default` 还是 `stable-x86_64-pc-windows-gnu`
   - `tauri.conf.json` 用了 `windows-subsystem = "windows"`（MSVC 资源嵌入要 MSVC link.exe）
   - `tauri-build` 在 `embed-resource` 阶段需要 `rc.exe`（MSVC SDK）
   - MinGW ld 不认 `.drectve`（MSVC link.exe 的 directive section）

3. 工具链切换后还有一个隐性坑：**新 PowerShell 进程不自动加载 `Initialize-MSVC` profile**
   - 之前的修复脚本（按用户记忆）是写在 `$PROFILE` 里 → shell 重启就丢
   - 用户"重启了，你试试"才暴露这点

### 解决步骤

1. 重新调用 `Initialize-MSVC`（用户的 PowerShell profile hook）—— 临时
2. `rustup default stable-x86_64-pc-windows-msvc` —— 持久化 default
3. `Remove-Item -Recurse -Force src-tauri/target` —— 旧 gnu 编译产物清掉（避免 link target 混用）
4. `cargo build --manifest-path src-tauri/Cargo.toml` —— 成功，1m14s
5. 第一次 `cargo tauri dev` 报 `Port 5173 is already in use`
   - 之前那个后台 shell 还活着，占着 Vite
   - `Stop-Process -Id <pid> -Force` 干掉
6. 第二次 `cargo tauri dev` → Vite ready，Rust 增量编译 23.82s，`kron-gui.exe` 跑起来

### 验证日志

```
[92m    Finished[0m `dev` profile [unoptimized + debuginfo] target(s) in 23.82s
[92m     Running[0m `target\debug\kron-gui.exe`
2026-09-08T07:53:12.734568Z  INFO kron_gui_lib::commands: watcher stub: would spawn notify watcher here
```

Vite:
```
  VITE v5.4.21  ready in 733 ms
  ➜  Local:   http://localhost:5173/
```

## 关键经验

| 问题 | 教训 |
|------|------|
| `.drectve` 警告 + link 失败 | MinGW toolchain 跑 MSVC-style Rust 项目 → 必败。看 `rustup show` 验证 |
| Shell 重启 MSVC env 失效 | `Initialize-MSVC` 必须挂在 `$PROFILE` 自动加载；不能靠每次手动 source |
| `cargo tauri dev` 第二次起不来 | 前一个 shell 还活着；用 `Get-NetTCPConnection -LocalPort 5173` 找进程 |
| `Select-Object -Last 50 \|` 在 dev server 上卡死 | PowerShell `Last N` 会缓冲所有输出直到 EOF，对长跑进程不要用 |

## 当前警告（待清理）

3 个 warning，0 error：

1. `src/core/context/code_structure.rs:173` — `let n = entries.len()` unused
2. `src/core/context/code_structure.rs:177` — `let is_last = ...` unused
3. `kron-gui/lib` — `linker stdout: 正在创建库 ... kron_gui_lib.dll.lib`（无害，rc.exe 中文输出）

## 下一步

继续 Phase 2 backlog（参考 day-6 上午的 `## 下一步`），按依赖顺序：

1. ~~MSVC toolchain 持久化~~ ✅
2. **清理 unused variable warnings**（顺手，5 分钟）
3. 替换 `watcher.rs` 的 stub —— 实际 `notify::recommended_watcher`
4. V0 项目列表 + `kron project add`（弹原生目录选择）
5. ts-rs 评估（暂时手写 mirror 够用，等 >10 类型）

## 给后续 session 的提示

- 用户的 MSVC env 在 `$PROFILE` 里，开新 shell 自动 OK
- 不要在 PowerShell 用 `2>&1 \| Select-Object -Last N` 跑长跑进程，会死锁
- 想看实时输出直接 `2>&1` 或重定向到文件然后 `Get-Content -Wait`
