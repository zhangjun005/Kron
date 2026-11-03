# Kron v0.3.1 发布记录

- 发布日期：2026-11-02
- 发布 tag：`v0.3.1`
- 发布分支：`master`
- 集成分支：`develop`
- 上一版本：`v0.3.0`

## 发布内容

### VS Code Webview

- 支持同时打开多个 intent 窗口；
- 每个窗口独立拖动、锁定；
- 锁定窗口在点击空白/拖动画布/Escape/workspace 节点时保留；
- 点击当前节点：锁定 → 解锁，未锁定 → 关闭；
- 支持多窗口详情 hydration（所有 intent 预取 `kron_get`）；
- 编辑表单保持单实例，其他窗口只读；
- 关闭/切换编辑时提示 `Save / Discard / Cancel`；
- 意图树增加 tree-only 缩放：15%–300%，`Ctrl/Cmd + 滚轮`、`Fit`、`100%`；
- 修复 workspace 节点误开卡、旧详情残留、窗口完全重叠等问题。

### Wails GUI

- 多 intent 卡片同时存在，各自拖动/锁定；
- 锁定卡在外部交互时保留；
- GUI 不再写共享 `projects.json`；
- 使用本地 `guiProjects.json`（首次启动从共享 registry 复制/合并）；
- 恢复自由切换项目与 Add project，CLI 子进程只写本地副本。

## 构建方式

```powershell
powershell -NoProfile -File frontend/wails/scripts/seed-release-env.ps1 `
  -WorkDir E:\works\kron-release-v0.3.1
cd frontend/vscode/webview; npm run build
cd ..; npm run compile; npm run package
```

## 产物

以发布时 GitHub Release 上实际附件为准；构建完成后填入大小与 SHA-256。

## 历史保留

PR 与分支继续保留，不做清理。
