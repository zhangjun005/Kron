# RFC: 客户端分发链路 — 1 zip 装一切 + 用户手动装 kron CLI

| 字段 | 值 |
|---|---|
| **状态** | **草案 (2026-10-13, zhangjun005 拍板启动)** |
| **作者** | AI assistant, 经 zhangjun005 委托 |
| **创建日期** | 2026-10-13 |
| **目标版本** | v1.3+ |
| **实施时机** | **最后发布时再做** (v1.3+ release pipeline 阶段, 不阻塞 GUI / 扩展骨架开发) |
| **补充** | [`docs/rfc/2026-10-09-gui-ide-plan.md`](./2026-10-09-gui-ide-plan.md) §3.3 (Wails 仓库位置) + §4.4 (vsix 调能力) 的"binary 从哪来"细节 |
| **取代** | — (新议题, 不取代任何 RFC) |

---

## 1 动机

[`docs/rfc/2026-10-09-gui-ide-plan.md`](./2026-10-09-gui-ide-plan.md) §3.2 / §4.4 拍板了"客户端 spawn `kron serve-mcp` / `serve-lsp` **子进程**"，但**没**拍:

- 客户端**怎么找到** kron binary（PATH? env? embedded?）
- kron binary **怎么到用户机器上**（go install? 装包附带? 单独下载?）
- Wails GUI 装包 / vsix / kron binary 三者的**分发组合**是什么
- 多平台 / 跨发行版的范围（暂搁）

**这层细节是 2026-10-09 RFC 的实现前提**——客户端 spawn 子进程假设 PATH 上有 kron, 但 PATH 上**怎么有**是另一件事, 不拍就**没法**实施 release pipeline.

### 1.1 拍板背景 (2026-10-13, zhangjun005)

讨论: 客户端 (Wails GUI / vsix) **不**嵌 kron binary (避免 VSCE 政策风险 + 跨平台矩阵), 走 "1 个 zip 装一切 + 用户手动装 kron CLI" 模式, 简化 release pipeline, 不动 Kron Go 主仓 `cmd/kron/` / `internal/`.

---

## 2 拍板

### 2.1 客户端定位 kron binary: PATH only

**binary-locator = PATH only** ([VSCode `which kron` 标准做法](https://code.visualstudio.com/api/working-with-extensions/publishing-extension#platform-specific-extensions)).

| 客户端 | 探测方式 | 失败行为 |
|---|---|---|
| Wails GUI | Go 端 `exec.LookPath("kron")` | **静默** (用户**不**弹窗, dev console 1 行 warning) |
| VSCode 扩展 | TS 端 `which` / Node `child_process.spawn` PATH 探测 | **静默** (OUTPUT 面板 1 行 info) |

**不**走 `KRON_BINARY_PATH` 环境变量. **不**走 VSCode `kron.binaryPath` setting. 理由:

- 零环境变量概念 (符合 Kron 零配置原则, architecture §3.5)
- `which kron` 跨平台标准, Go / Node 都有内置支持
- 减少概念: 用户**只**需要"把 kron 装到 PATH", 完事

### 2.2 kron binary 怎么到用户机器上: 用户手动装

**用户自己**负责把 kron.exe / kron 放到 PATH 上, GUI / 扩展**不**自动注册.

| 步骤 | 谁做 | 工具 |
|---|---|---|
| 1. 下载 `kron-v1.3.0-<plat>-<arch>.zip` | 用户 | 浏览器 |
| 2. 解压 | 用户 | 系统 unzip / 7-Zip |
| 3. 拷 `kron.exe` 到 `%LOCALAPPDATA%\Programs\kron\` (Windows) / `/usr/local/bin/` (Mac, sudo) | 用户 | 文件管理器 / cp |
| 4. 加 PATH | 用户 | Windows "环境变量" GUI / `~/.zshrc` 加 `export PATH=...` |
| 5. 终端 `which kron` 验证 | 用户 | shell |
| 6. 开 Wails GUI / 装 vsix → 正常工作 | 用户 | 双击 / Marketplace 装 |

**GUI / 扩展**完全不参与这 6 步, **不**写 NSIS postinstall, **不**写 DMG postinstall, **不**写 Wails 启动器 PATH 引导.

**理由**:

- 零 installer 脚本, 简化 Kron 主仓 `build/` 目录
- 零提权弹窗 (System PATH 修改需 UAC / sudo, 用户级 PATH 自管)
- 零版本耦合 (kron binary 升级不影响 client release)
- 零架构变更 (5 铁律 + 8 import 边界**不**动)

### 2.3 release 发包组合: 1 个 Windows zip 装一切 (raw 形态, 0 installer)

每个 release tag (v1.3.0 / v1.3.1 / ...) **只**发 1 个 zip (**仅** Windows 11 amd64):

```
GitHub release v1.3.0
└── kron-v1.3.0-windows-amd64.zip
    ├── Kron/                     (Wails GUI raw 形态, `wails build -nsis=false`)
    │   ├── Kron.exe              (~80MB, 双击跑, 0 installer)
    │   └── <Wails 运行时资源>
    ├── kron.exe                  (~15MB, kron CLI, Go build ./cmd/kron)
    └── README.txt                (2-3 行手工装指引)
```

**用户**解开 zip:
1. 拷 `Kron/` 目录到任何位置 (桌面 / 文档 / `C:\Apps\Kron\`) → 双击 `Kron\Kron.exe` 跑
2. 手动拷 `kron.exe` 到 `%LOCALAPPDATA%\Programs\kron\` + 加 PATH
3. 终端 `which kron` 验证
4. 升级 = 覆盖目录 / 覆盖 `.exe`。卸载 = 直接删

**不**发 Mac (darwin) / **不**发 Linux / **不**发 windows-arm64。Mac 用户走 `go install github.com/xxx/kron/cmd/kron@latest` (Go 生态默认, **不**算我做的"分发"); Mac Kron Go 主仓代码**仍**在 `cmd/kron/serve-mcp/` / `serve-lsp/`, Go 跨平台编译是默认能力, **不**进 release pipeline 即代表"不发 binary, 不承诺 Mac 平台支持".

### 2.4 vsix 分发: 仍直接 Marketplace 装

vsix **不**含 kron binary, 仍走 VSCode Marketplace 装.

- 找到 kron → spawn 子进程 (现有 §4.4)
- 找不到 kron → 静默 + OUTPUT 面板 1 行 info "kron not found, install from https://github.com/.../releases"

---

## 3 不做的事 (明确)

| 项 | 状态 | 理由 |
|---|---|---|
| ❌ NSIS postinstall / DMG postinstall 脚本 | **不**写 | 减少 build 复杂度, 用户自己装 |
| ❌ Wails NSIS installer (Windows) | **不**用 | 走 `wails build -nsis=false` raw build, 0 installer; 升级 = 覆盖目录 |
| ❌ DMG 装包 (Mac) | **不**做 | Mac **不**在 v1.3+ 分发计划内, 走 `go install`; 详见 §2.3 |
| ❌ `kron install` / `kron update` CLI 子命令 | **不**加 | AGENTS.md §3 红线 (CLI 只能 init/add/lint/serve-mcp/serve-lsp); 走用户手动 = 零主仓改动 |
| ❌ `KRON_BINARY_PATH` 环境变量 | **不**引 | 减少概念, PATH only |
| ❌ Wails GUI 装包**内含** kron binary | **不**做 | 体积膨胀 + 跨平台矩阵; 走 zip 并排 |
| ❌ vsix**内含** kron binary | **不**做 | VSCE 政策风险 ([官方建议](https://code.visualstudio.com/api/working-with-extensions/publishing-extension#platform-specific-extensions) **不**推荐嵌入原生 binary) |
| ❌ Wails 第一次启动检测 + 自动注册 PATH | **不**做 | 提权复杂 (System PATH 需 UAC / sudo); 用户自己管 |
| ❌ GUI 弹 "Install kron now?" 引导 | **不**做 | 用户选 silent (本 RFC §2.1) |
| ❌ 多平台适配 (Linux / Windows arm64 / Mac) | **不**做 | v1.3+ **仅** Windows 11 amd64 (详见 §2.3); Linux / arm64 / Mac 走 v1.4+ RFC 重拍 |
| ❌ `kron` Go 主仓 `cmd/kron/` 改动 | **不**动 | 0 改动 = 0 红线 |
| ❌ `internal/` 改动 | **不**动 | 同上 |
| ❌ `go.mod` `require` 块新增 | **不**做 | 0 改动 |
| ❌ `config.toml` 新字段 | **不**加 | 零配置原则 |

---

## 4 实施清单 (最后发布时再做, 估 ~1.5 天)

> ⚠️ **本节是"最后发布时再做"事项清单, 不阻塞 v1.3 GUI / 扩展骨架开发**.

| # | 改动 | 仓库位置 | 工作量 |
|---|---|---|---|
| 1 | `build/release.ps1` (Windows): `wails build -nsis=false` (raw) → `go build -o dist/kron.exe ./cmd/kron` → 拷 `Kron/` 目录 + `kron.exe` + `README.txt` 到 zip → `gh release create` | Kron 主仓 `build/` (新建) | 0.5 天 |
| ~~2~~ | ~~`build/release.sh` (Mac): 同上 darwin 路径~~ | — | **删** (v1.3+ **不**发 Mac) |
| 3 | `frontend/wails/README.md` 新建: "Install" 段 (说明手动装 kron) | Kron 主仓 `frontend/wails/` | 0.5 天 |
| 4 | `frontend/vscode/README.md` 新建: "Prereq" 段 (kron not found → 去 release 页面) | Kron 主仓 `frontend/vscode/` | 0.5 天 |
| 5 | 主仓 `README.md` 新建: "Install kron CLI" 段 (指向 release) | Kron 主仓 | 0.5 天 |
| 6 | 同步 `docs/rfc/2026-10-09-gui-ide-plan.md` §3.3 / §4.4: 标注"binary 分发见 [`2026-10-13-gui-distribution-manual-install.md`](./2026-10-13-gui-distribution-manual-install.md)" | Kron 主仓 `docs/` | 0.5 天 |

**总改动 ~1.5 天. `cmd/kron/` / `internal/` / `go.mod` / `config.toml` 全 0 改动.**

### 4.1 实施顺序

1. Wails GUI 骨架 (v1.3 RFC 拍板后另开 PR)
2. vsix 骨架 (v1.3 RFC 拍板后另开 PR)
3. **最后发布时** (release pipeline 阶段) 写 §4 5 项 release automation (`wails build -nsis=false` raw 形态, Mac 整段**不**实施)
4. release tag v1.3.0 → 跑 release 脚本 → 资产上架 → README / docs 同步

---

## 5 红线确认

按 [`AGENTS.md` §3](../../AGENTS.md) 红线 + [`architecture.md` §〇 铁律](../abstractDesign/architecture.md) 走查:

| 红线 | 拍板 |
|---|---|
| 加新 top-level dep (`go.mod` `require` 块) | ❌ 不加 (0 改动) |
| 改 `go.mod` Go version | ❌ 不改 |
| 改 package 路径 / 改 package 名 | ❌ 不改 |
| `go generate` / reflection | ❌ 不用 |
| 强推 / 改 shared branch 历史 | ❌ 不做 |
| 加新 CLI 子命令 (init/add/lint/serve-mcp/serve-lsp 之外) | ❌ 不加 (`kron install` **不**加, 走用户手动) |
| 加 `config.toml` 字段 | ❌ 不加 (零配置) |
| 改 frontmatter schema | ❌ 不改 |
| 改 intents/assumptions 数据架构 | ❌ 不改 |
| 新 lint 规则 | ❌ 不加 |
| 新 `internal/` 包 | ❌ 不加 |
| 改 5 铁律 + 8 import 边界 | ❌ 不改 |
| access layer 互调 | ❌ 不让 (GUI 走 serve-mcp, vsix 走 serve-mcp + serve-lsp, 各自独立) |
| `internal/` import `cmd/kron/` | ❌ 不让 |
| `cmd/kron/<sub>/` import 另一个 `cmd/kron/<other>/` | ❌ 不让 |

**全部红线避, 0 改动 Kron Go 主仓.**

---

## 6 跟现有 RFC 的关系

| 现有 RFC | 关系 |
|---|---|
| [`2026-10-09-gui-ide-plan.md`](./2026-10-09-gui-ide-plan.md) §3.3 Wails 仓库位置 | **不**改; 本 RFC 是 §3.3 实施前提 |
| [`2026-10-09-gui-ide-plan.md`](./2026-10-09-gui-ide-plan.md) §4.4 vsix 调能力 | **不**改; 本 RFC 是 §4.4 实施前提 |
| [`2026-10-09-gui-ide-plan.md`](./2026-10-09-gui-ide-plan.md) §5 SDK 选型 | **不**改; 0 Go dep 变动 |
| [`2026-10-10-frontend-ui-stack.md`](./2026-10-10-frontend-ui-stack.md) shadcn/ui 选型 | **不**改; 客户端层**不**依赖 kron binary 分发 |

**最后发布时**同步: `2026-10-09-gui-ide-plan.md` §3.3 / §4.4 加 1 行 "binary 分发见 `2026-10-13-gui-distribution-manual-install.md`" 引用.

---

## 7 待办 (v1.3+ release pipeline 阶段, 估 ~1.5 天)

> ⚠️ **本节是"最后发布时再做"事项. 标在这里是防止遗忘, 不构成 v1.3 GUI / 扩展骨架的 blocker.**

- [ ] `build/release.ps1` 写完 (含 `wails build -nsis=false`), 跑通 v1.3.0-rc1 release 流程
- [ ] `frontend/wails/README.md` "Install" 段写完 (Windows only, raw 形态)
- [ ] `frontend/vscode/README.md` "Prereq" 段写完
- [ ] 主仓 `README.md` "Install kron CLI" 段写完 (Windows only, 砍 Mac / Linux 段)
- [ ] 修订 `2026-10-09-gui-ide-plan.md` §3.3 / §4.4 加 1 行引用
- [ ] (v1.4+ 评估) Windows arm64 是否纳入
- [ ] (v1.4+ 评估) Mac (darwin) 分发是否启动 (现在仅 Windows, Mac 走 `go install`)
- [ ] (v1.4+ 评估) Linux 是否纳入
- [ ] (v1.4+ 评估) 是否要 `kron upgrade` CLI 子命令 (二进制自升级, 走 RFC 重拍)
- [ ] (v1.4+ 评估) 是否要给 Wails GUI 加 "kron not found" 引导 (现在 silent, 用户反馈差再开 RFC)
