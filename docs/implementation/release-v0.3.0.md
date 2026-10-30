# Kron v0.3.0 发布记录

- 发布日期：2026-10-30
- 发布 tag：`v0.3.0`（annotated，指向 `11f8246`）
- 发布页面：<https://github.com/zhangjun005/Kron/releases/tag/v0.3.0>
- 发布分支：`master`
- 集成分支：`develop`

## 发布内容

v0.3.0 是从 `develop` 合入 `master` 的首个正式发布版本，对应 release PR
[#8](https://github.com/zhangjun005/Kron/pull/8)；版本统一 PR 为
[#7](https://github.com/zhangjun005/Kron/pull/7)。

主要变化：

- VS Code 扩展：意图树/Webview 工作流、锚点右键菜单、intent → code anchor 跳转、
  多锚点 QuickPick、`.kron/anchor-scan.json` 驱动的语言扫描
- MCP：工具元数据 / annotations、prompts 增强、项目 intent 正文模板注入
- `kron init`：幂等初始化 `templates/intent-body.md` + `anchor-scan.json`
- 工程化：MIT LICENSE、workflow playbook、PR intent checklist、CI Windows 门禁
- 版本统一：VSIX / MCP / LSP 均为 `0.3.0`

## 发布产物

| 产物 | 大小 | SHA-256 |
|---|---:|---|
| `Kron-v0.3.0-windows-amd64.zip` | 16,385,103 B | `311A1CDD4668EE4E67395138DB91B62E981824D2AA08589F32F8DAA21F17DF41` |
| `kron-vscode-0.3.0.vsix` | 4,701,001 B | `1015B45F1E1FEF1016F8202ED01C51962DE8DFBDD61A43AB0DA7DC375817EE3B` |

zip 内含：`kron.exe`（CLI + serve-mcp + serve-lsp）、`Kron-GUI.exe`（Wails）、
`kron-vscode-0.3.0.vsix`、`README.txt`、`LICENSE`。

## 构建方式

构建自 `v0.3.0` tag 对应的工作树（`11f8246`）：

```powershell
# 1) release-shaped GUI/CLI 环境（独立目录，避免占用正在运行的 kron.exe）
powershell -NoProfile -File frontend/wails/scripts/seed-release-env.ps1 `
  -WorkDir E:\works\kron-release-v0.3.0

# 2) VS Code 扩展
cd frontend/vscode/webview; npm run build
cd ..; npm run compile; npm run package
```

验证：

- `kron.exe` 的 MCP `initialize` 返回 `serverInfo.version = v0.3.0`
- VSIX `package.json` 版本为 `0.3.0`
- master CI：`verify (windows-latest)` ✅

## 历史保留

发布前后**没有删除**任何 PR 或分支：

- PR #1–#8 全部保留（closed / merged）
- 远端分支保留（含 `chore/release-v0.3.0`）
- 该记录用于课程项目汇报的提交 / PR / Release 链路追溯：

```
commit 11f8246 → PR #8 → tag v0.3.0 → GitHub Release
```
