# 2026-10-28 VSCode / Wails 交接记录

本文记录本轮客户端验收、实现和发布态验证结果，供后续接手者继续工作。

## Wails GUI

- 增加 `guiSettings.json`，保存主题和项目卡片设置；读取时兼容 UTF-8 BOM。
- 支持 `system`、`light`、`dark` 主题。
- 首页改为横向项目卡片列表，支持项目封面、项目添加和项目初始化/注册。
- 使用 `icon/kronLogo.png` 作为启动页和发布包图标。
- 项目详情页 Explorer 标题栏的操作按钮统一靠右。
- 增加在文件管理器中打开当前项目目录的操作。
- 发布脚本改用 Wails builder 生成带 PE 图标资源的 `Kron-GUI.exe`。
- `kron-mcp-test/app/` 已按发布态同步 GUI、CLI、`projects.json`、`guiSettings.json` 和说明文件。

## VSCode P0/P1 实现

- Intent 正文支持本地 Markdown 预览，编辑与预览并列显示。
- 增加 `kron.webviewFontSize` 设置，范围 10–24。
- Webview 基础颜色接入 VSCode 主题变量。
- 多根工作区变化时重建 MCP/LSP 连接并刷新树。
- 补齐 `kron_lint` 的客户端工具契约。
- 节点详情卡片改为跟随节点在下方展开，可相对节点拖拽。
- 点击节点、空白区域、Escape、关闭按钮时，详情卡片和节点高亮同步清理。
- Edit 会让同一张节点卡片切换为编辑状态，不再额外打开全屏编辑窗口。
- node 类型的 Edit 按钮已启用；底层 `kron_update` 会保持 `README.md` 节点形态。
- 侧边栏标题栏的刷新、打开工作区、重启连接命令改为 VSCode 图标。
- 右键菜单支持第一次点击外部关闭。
- 编辑卡片第一行字段改为顶部对齐。

## 新建流程现状

- 顶部 `New intent` 创建根级 slug。
- 节点右键菜单的 `Create child intent` 自动锁定父级并只允许新增一个 slug 段。
- 新建时可选择 leaf 或 node；node 通过 `kron_add(kind=node)` 创建目录形式的 `README.md`。
- 当前仍缺少清晰的 workspace 根节点创建入口；“创建文档/创建文件夹”的信息架构需要后续重构，不应继续把 node/leaf 藏在 Additional fields 中。

## Assumption marker 验证

`<!-- @kron:assumes single-region -->` 是已有设计的可选正文 marker，由
`internal/parser/assumption_anchors.go` 解析，供 orphan lint 将正文引用和 frontmatter 引用合并判断。
它不会修改 frontmatter，也不会自动创建 assumption。示例项目的 `auth/jwt.md` 同时存在 frontmatter 引用和正文 marker，因此属于重复声明。

## 真实验证

- `go test ./internal/parser -run 'AssumptionAnchor|ScanAssumption'`
- `go test ./internal/lint -run 'Assumption'`
- 在示例项目临时副本上用 `kron-mcp-test/app/kron.exe` 调用 `kron_update` 更新 node `auth`：实际写入 `.kron/intents/auth/README.md`，没有创建 `auth.md`。
- 注意：当前 `kron_update` 返回的 `path` 仍可能显示 `.kron/intents/auth.md`，但实际写入路径保持为 node 的 `README.md`。
- VSCode：`npm run compile`、`npm run build:webview`、`npm run package` 均通过。

## 后续讨论项

- 将编辑表单收敛为“创建文档/创建文件夹 + 标题/正文/预览”的最小路径。
- 将 root 创建和 child 创建做成两个明确入口。
- 用图标、上下文和 tooltip 替代大量 Kron 内部术语。
- 修正 `kron_update` 对 node 返回路径的 wire 输出。
