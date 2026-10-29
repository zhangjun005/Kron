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

## 2026-10-29 VSIX 增量

- 假设在 Intent 详情/编辑卡片中可见；支持选择已有假设、创建并关联新假设、解除关联。
- 真实工作区增加 assumption `Check` 操作，展示 hard/soft/expired 汇总和最多五条警告。
- VSIX MCP 客户端契约补齐 `kron_assume_get`、`kron_assume_update`、`kron_assume_delete`、`kron_assume_check`。
- 新建入口从隐藏的 kind 字段改为明确的 Intent document / Intent folder；节点右键菜单同样区分创建子文档和子文件夹。
- 创建时只显示 slug、标题和正文，摘要由正文首段派生；选中 folder/workspace 会预填父级，选中 leaf 时禁用新建入口。当前存储层没有 leaf → node 转换机制。
- Intent 详情补充 Created by，并明确其由 `kron_add` 的 MCP identity/git 配置填充；Assumptions 标题行压缩为 `Assumptions + Check + Add`。
- node 返回路径问题按用户要求暂不修改。

### 详情/编辑卡片布局（2026-10-29 第二轮）

- 卡片不再有顶部标题栏（slug + 关闭按钮那一条）；只保留右上角一个浮动关闭按钮，关闭仍可用点击节点 / 空白处 / Escape。
- 固定类型字段：status chip 显示圆点 + 状态文字，与其它 chip 同高；Created by、Reviewers、Symbol、Depends on、References 只显示当前值，空值显示 `…`，点击在下方展开明细或编辑控件；chip 行距收紧。
- chip 每行最多三个：第一行 status / Created by / Reviewers，第二行 Symbol / Depends on / References。
- Assumptions 区压缩为一行标题 + `Check` + `Add`；每条 assumption 默认只显示 id 与 severity，点击展开文本、rationale 与 `Remove`。
- 正文固定为限定高度和宽度的只读预览，不再提供 Show body 折叠；编辑态去掉 Markdown 预览栏与提示句，正文单独一栏限高。
- Intent 标题与简述字号上调（`text-lg` / 15px）。
- 修复 `kron.webviewFontSize` 不生效：Tailwind 的 rem 字号不跟随 body，改为按设置缩放文档根字号，默认 14 与改动前视觉一致。
- leaf 与 node（Intent folder）都显示 Assumptions 区；只有无 README 的纯结构目录不显示，因为那不是 intent、没有 frontmatter。

### 假设 attach 静默失败修复（2026-10-29 第三轮）

- 现象：在 intent 卡片里点击 `Attach assumption` / `Create and attach` 后没有任何反应，或成功/失败都看不到提示。
- 根因（Webview 反馈层，与 MCP 后端无关）：`formError` 只在 Intent 编辑表单里渲染，Assumptions 面板没有错误渲染；`notice`（成功提示）与 `detailError` 只渲染在删除确认弹层里，所以 assumption 校验失败（空 ID、rationale < 10 字符、ID 已存在、无 workspace）和 MCP 报错全部静默。
- 修复：新增 `assumptionFormError` 并在 Assumptions 面板内渲染；`Use existing` 的 rationale 提示补上至少 10 字符要求；成功/失败提示改为全局右下角 toast（notice 5 秒自动消失，`detailError` 常驻到下次选择），不再依赖删除弹层；`Check` 结果标注为 workspace 级并显示 intent slug；`Default severity` 不再顺带修改 `Intent severity`。
- 端到端验证：用真实 Webview 产物 + 真实 `kron serve-mcp` 子进程 + fixture 临时副本回放 UI 操作，覆盖新建并关联、选择已有假设关联、关联到 node（README）intent、清空关联、空 rationale 被拒；错误提示、成功 toast、Assumptions 计数都符合预期。
- 后端 `kron_assume_*` 与 `kron_update(assumptions)` 行为核对无异常：创建/列出/更新/删除/get/check 正常，重复 attach 去重由服务端 rationale 校验拦截，未知 ID 会按 G-5 自动建 stub；node 返回路径问题仍未改。

### 字体设置覆盖小字号（2026-10-29 第四轮）

- 现象：`kron.webviewFontSize` 只影响 rem 字号，而 Webview 里 10px / 11px / 15px 这些小字是硬编码 px，所以调设置时"小字完全不动"。
- 决策：**不新增第二个设置，也不做固定差值**。所有字号统一写成 rem（0.625 / 0.6875 / 0.9375rem），由同一个设置按比例缩放；理由是一个旋钮即可，文字层次在任何档位都不变形，固定差值在低档位会跌破可读性、在高档位会让层次糊掉。
- 画布几何常量（列宽、行高、节点尺寸、内边距）与详情卡片宽度按同一系数缩放，保证大字号时节点框和卡片不会裁字；卡片位置增加视口边界钳制，避免宽卡片跑出可视区域。
- 实测：设置 14 → 根字号 16px、chip 11px；设置 22 → 根字号 25.14px、chip 17.29px，卡片宽度 380→597px 且不再溢出视口。

### 审计后收敛（2026-10-29 第五轮）

- **项目范围改为单项目**：一个窗口只服务第一个含 `.kron/intents` 的 workspace folder，其余 folder 不建连接、不进意图树；激活事件由 `onStartupFinished` 改为 `workspaceContains:.kron/**` + `onView`，避免在非 Kron 工作区起进程。
- **设置即时生效**：`kron.serverPath` / `kron.lspLanguages` 在每次重建连接时重新读取，"Restart Connections" 直接重建连接；新增 `onDidChangeConfiguration`；`kron.webviewFontSize` 通过 `kron_font_size` 消息实时推送到 Webview，根字号、画布几何、卡片宽度一起缩放，无需重开面板。
- **MCP 错误分级**：工具拒绝、输出校验失败、超时只算单次调用失败，不再把连接状态置为 failed；连接死亡只由 transport `onerror` / `onclose` 和启动失败决定。超时文案改为带工具名。
- **假设卡片**：以 intent 自身 `assumptions[]` 为准渲染，registry 只补充共享文本；registry 加载失败不会再把已关联假设藏掉。
- **锚点识别按 parser 语义重写**（新增 `src/anchors.ts`）：任意注释前缀、行内任意位置、允许尾随文字；Markdown 跳过 fenced code block，Markdown 锚点只下划线 + 定义跳转、不弹创建提示；`.kron/` 下的文档不扫描。
- **工具契约收敛**：host `MCP_TOOL_NAMES` 与 Webview 名单统一为实际使用的 8 个工具；移除 `kron_tree`（树走独立消息）、`kron_lint`、`kron_assume_get/update/delete` 的 zod 契约，消除死条目和名单漂移。
- **Webview 持久化**：用 `getState` / `setState` 保存选中 slug 与过滤词，面板隐藏/重建后恢复。
- **文档**：`vscode-extension.md` 锚点章节改为已实现的语义；多根/刷新章节改为单项目策略，并标注"保存/外部变更刷新"未实现。
- 用户决定不做：编辑器保存/外部文件变更刷新、编辑表单丢弃保护、打包工程小项；N+1 `kron_get` 优化暂不做。

### 锚点右键菜单（2026-10-29 第六轮）

- 补上 `editor/context` 菜单，按锚点状态出现 4 / 3 / 2 项：有效锚点（Open Intent Details、Edit Intent、Open Intent Markdown、Show Intent Impact）、悬空锚点（Create Intent for Anchor、Change Anchor Target、Remove Anchor）、无锚点（Create Intent and Attach Anchor、Attach Existing Intent Anchor）；`.kron/` 内文档与非文件文档不出现菜单。
- 有效锚点四个动作：打开工作区面板并选中、直接进入编辑态、打开 `.md` 原文、`kron_impact` 摘要（明细写 Output）；悬空锚点创建时若改了 slug，锚点同步改写；Change Anchor Target 用 QuickPick + `WorkspaceEdit`；Remove Anchor 删除整行。
- 创建+插锚点顺序固定：`kron_add` 成功 → 刷新树 → 在符号行上一行插入注释锚点；创建失败不改文件，插入失败提示"已创建但锚点未插入"并给出 slug。注释前缀按 languageId 选择（`//` / `#` / `--` / `%` / `<!-- -->`），未知语言弹可编辑预览。
- 状态用 `kronAnchorState` 上下文键（none / checking / valid / dangling / unsupported），光标与文档变化 200ms 防抖更新；存在性判断走一次 `kron_get` 并缓存 5 秒，树刷新/写操作后失效。
- 面板联动新增宿主→Webview 的 `kron_select` 消息：新面板要等它首次请求 tree（监听器已挂载）后才下发，避免消息被丢弃；`edit: true` 时等 intent 详情加载完再自动进入编辑态。

### 假设注册表面板（2026-10-29 第七轮）

- 主界面 `Archived` 左侧新增 `Assumptions` 按钮，打开独立的假设注册表弹窗：左侧列表（id / 默认 severity / 被引用次数），右侧详情与编辑。
- 详情可改共享文本、默认 severity、status、expires_at、reviewers；保存走 `kron_assume_update`。`New assumption` 走 `kron_assume_create`；`Delete` 走 `kron_assume_delete`（软删除），确认框显示仍引用它的 intent 数量。
- "Used by" 来自 `kron_assume_check` 的 per-intent 记录（severity / rationale），点击可跳到对应 intent 卡片。
- MCP 契约补回 `kron_assume_get` / `kron_assume_update` / `kron_assume_delete`（Webview 可请求的工具扩到 11 个），host 与 Webview 名单保持同步。
- 独立预览（mock）模式下同一面板用内存数据工作，便于不装 Kron 二进制时验收 UI。
- 修复"面板里看不到日期"：`kron_assume_list` 的 zod 输出契约原先只声明 `id/text/default_severity/status`，zod 默认丢弃未知字段，把 `created_by` / `updated_at` / `expires_at` / `verified_at` / `verified_by` / `reviewers` 全部剥掉了；契约补齐后列表行显示过期日期，详情新增只读元数据区（Created by / Updated at / Expires at (stored) / Verified at / Verified by）。
- `expires_at` 用 date input 编辑时只取前 10 位显示，保存时若用户没改就原样回写，避免把带时间的 RFC3339 精度截断。

### 假设日期格式放宽（2026-10-29 第八轮，Go 侧）

- 根因：数据格式真理源 [`intent-structure.md`](../../abstractDesign/intent-structure.md) 写的是 `YYYY-MM-DD`，但读取端（`kron_assume_check` + `kron lint` 的 S2）严格按 RFC3339 解析，解析失败静默跳过——文档形态的数据反而不参与过期判断。
- 新增 `internal/parser/dates.go`：`ParseExpiryDate` / `ParseVerificationDate` / `ValidateAssumptionDateFields`。接受 `YYYY-MM-DD` 与完整 RFC3339；纯日期过期按 UTC 当天结束，`verified_at` 纯日期按当天 00:00。
- 三处解析点改用共享 helper：`cmd/kron/serve-mcp/handlers_assume_check.go`、`internal/lint/lint.go`（`ComputeStaleReport` + `runStalenessRules`）。`kron lint` 遇到无法解析的 `expires_at` 会出 warning（复用 S2 规则名），不再静默。
- 写入端校验：`ValidateAssumptionFrontmatter`、`assumption.Writer.Create`、`resolveAssumptionsForIntent`（`kron_add` / `kron_update` 的 assumptions）拒绝无法解析的日期；VSIX 面板发送的 `YYYY-MM-DD` 保持合法。
- 文档同步：`intent-structure.md` 补充两种接受形态与语义；`tools_schema.go` / `lint/diag.go` 的 schema 文案改为 `YYYY-MM-DD or RFC3339`。

### 待讨论：node 删除改为整棵子树（未实施）

- 用户拍板：删除 node 意图时整棵目录（含子意图）进 trash，而不是当前"只移 README + trash 里存成叶子文件 + restore 后变 leaf"。
- 该改动会改 `.kron/.trash` 的 on-disk 布局和删除语义，属于 AGENTS §3 的"改 intents/assumptions 数据架构"红线 → 需先开 RFC + 走 `docs/process/migrate.md`，尚未动代码。

### node 删除子树化（2026-10-29 第九轮，已实施）

- 决策与落地细节见 [`2026-10-29-trash-subtree.md`](2026-10-29-trash-subtree.md)（用户指定记录在 `docs/implementation/`，不再单独开 RFC）。
- trash 镜像树形：leaf → `.trash/<slug>.md`；node → `.trash/<slug>/` 整目录。restore 按 trash 形状还原整棵子树；旧 flat trash 按 leaf 恢复。
- node 删除必须 `confirm_subtree: true`；`kron_delete` 输出 `deleted_slugs` + 子树并集 `dependents`；`kron_restore` 输出 `restored_slugs`；`kron_update` / `kron_restore` 返回真实磁盘路径（node 为 `<slug>/README.md`）。
- VSIX：node 的 Delete 按钮启用，确认框写明子树数量并自动带 confirm；删除/恢复提示涉及 N 个意图。
- 测试：store 新增 node 子树删除/恢复、旧 flat trash、目录冲突用例；`go test ./...` 仅剩既有 fixture 脏数据（`test1`/`test2`）导致的 e2e 计数失败，与本次改动无关。


### VSIX 品牌化（2026-10-29 第十轮）

- 图标：VSIX 市场图标 `media/kronLogo.png`；Webview 左上角由 "K" 占位块换成同一 logo（方形、不圆角）。
- `kron.exe` 通过 `cmd/kron/kron.ico` + `icon.rc` + `rsrc_windows_amd64.syso`（windres 生成）嵌入 Windows PE 图标；Wails 的 `build/appicon.png` 同步为同一 logo（GUI 需重跑 `seed-release-env.ps1` 才生效）。
- 署名：`author` = `zhangjun005, HomoFluctus, lanyue811`；`homepage` / `bugs` 指向 `https://github.com/zhangjun005/Kron`。
- `publisher` 由 `homofluctus` 改为 `zhangjun005`：Marketplace 发布前必须先用发布账号在 manage 页面创建同名 publisher ID；若已有 `homofluctus` 的在线列表，换 publisher 等于换扩展身份，旧列表不能被新 ID 覆盖。
- `README.md` 重写为用户向介绍（Features / Requirements / Settings / Commands / Development）。
