# kron-mcp-test — 发布态模拟测试环境

**用途**：真实发布态（release-shaped）的集成测试。GUI、CLI/MCP、registry、被测项目全部放在**同一个目录**里，直接双击运行，不模拟 `%LOCALAPPDATA%\Programs\kron` / PATH-only 的用户安装流程。

**约定**：以后所有「真实环境模拟测试」都在这个目录进行；`project/` 是入库的共享 fixture，发布态重建脚本直接从它复制测试项目。

## 布局

```
kron-mcp-test/
├── app/                     ← 发布态运行目录（启动这里）
│   ├── Kron-GUI.exe         ← Wails GUI（raw build，双击）
│   ├── kron.exe             ← CLI + serve-mcp + serve-lsp（同一个二进制）
│   ├── projects.json        ← registry，由 `kron register` 写在 exe 旁
│   ├── guiSettings.json     ← GUI 主题和项目卡片偏好
│   ├── covers/               ← GUI 项目封面（用户选择后生成）
│   ├── README.txt           ← 模拟发布 zip 内的说明
│   └── serve-mcp.log        ← 运行时由 GUI spawn 的 serve-mcp 写入
├── project/                 ← 被测项目（入库 fixture，含 .kron/ + 锚点）
├── setup.ps1                ← 重建入口（薄封装，逻辑在仓库脚本里）
├── setup.sh                 ← 同上，git-bash/WSL 用
├── go-client/ raw-probe*/   ← optimize/mcp 时期的 MCP 客户端/协议探测源码（历史保留）
├── NOTES.md                 ← optimize/mcp 时期的记录（历史）
└── README.md                ← 本文件
```

## 重建 / 验证

```powershell
# 从仓库根目录（重新构建 GUI + CLI + 复制 fixture + 跑 kron register + 验证 MCP）
powershell -NoProfile -File frontend/wails/scripts/seed-release-env.ps1 -Verify

# 已构建过、只验证
powershell -NoProfile -File frontend/wails/scripts/seed-release-env.ps1 -SkipFrontend -Verify
```

脚本（均已入库）：

| 脚本 | 作用 |
|---|---|
| `frontend/wails/scripts/seed-release-env.ps1` | 从 HEAD 构建 `app/`、复制 fixture、用真实 `kron register` 生成 `projects.json` |
| `frontend/wails/scripts/verify-mcp-registry.ps1` | 验证 `serve-mcp` **不带 `-CWD`** 时从 exe 旁的 `projects.json` 解析项目；含负向对照（移走 json 必须失败） |
| `frontend/wails/scripts/mcp-probe/` | 手写 stdio MCP 客户端（Go，无 BOM 问题），供验证脚本使用 |

## 已验证的发布态行为

1. **GUI**：`app\Kron-GUI.exe` 双击（cwd = `app`）→ 项目卡片首页 → 选择项目 → spawn `app\kron.exe serve-mcp -CWD project` → Live 树正常。
2. **MCP 引用 json**：`kron.exe serve-mcp`（无 `-CWD`，cwd 设为无关目录）→ 仍从 `app\projects.json` 解析出 8 条 intent；移走 `projects.json` 后同一调用失败（负向对照）。
3. **CLI**：在 `project/` 内运行 `app\kron.exe lint`，结果与仓库 fixture 一致（1 error + 1 warning）。

## 命名说明

Windows 文件系统大小写不敏感，`Kron.exe` 与 `kron.exe` **不能共存**于同一目录。这里优先保留 CLI 的发布名 `kron.exe`（文档/工具都按 `kron` 引用），GUI 用 `Kron-GUI.exe`。GUI 的 CLI 发现顺序里 exe 同目录优先，所以不需要 PATH。

## 历史

`go-client/`、`raw-probe*/`、`NOTES.md` 来自 `optimize/mcp` 时期的 MCP smoke（v1.2.0，7 个调用），保留作参考；它们**不是**发布态环境的一部分，重建脚本不会动它们。
