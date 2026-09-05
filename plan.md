# Kron 开发计划

> 本文件是 Kron 项目开发的行动清单，按优先级排列。
> 对应需求文档：dev-docs/requirements.md
> 对应设计文档：dev-docs/design/00-总览与架构.md

---

## 项目概览

- **一句话**：让个人开发者的项目代码 + 文档 + 任务对 AI 透明可读、对人零学习成本的桌面工具。
- **技术栈**：Tauri 2.x + Rust + React 18 + TypeScript + Tailwind CSS + Zustand
- **进度**：设计阶段，尚未开始编码

---

## 设计文档清单

| 编号 | 文档 | 状态 | 说明 |
|------|------|------|------|
| 00 | 00-总览与架构.md | ✅ 完成 | 路线图 + 系统全景图 |
| 01 | 01-数据模型.md | ✅ 完成 | 所有 struct + JSON/MD Schema |
| 02 | 02-模块划分.md | ✅ 完成 | 7 crate + 依赖图 + Trait 契约 + Git 边界 |
| 03 | 03-双源同步机制.md | ✅ 完成 | 双源状态机 + 冲突检测 + 备份 + 复原 + 边缘情况 |
| 04 | 04-守护进程与文件监听.md | ✅ 完成 | notify 防抖 + 事件路由 + 调度器 + 健康检查 |
| 04b | 04b-CLI设计.md | ✅ 完成（骨架） | 7 类 50+ 命令清单 + 输出模式 + 退出码 + 配置加载顺序 |
| 05 | 05-GUI设计.md | ⬜ 待写 | 组件树 + 交互细节 |
| 06 | 06-数据格式规范.md | ⬜ 待写 | JSON/MD 序列化细节、版本迁移 |
| 07 | 07-实施路线图.md | ✅ 完成 | 7 Phase 任务分解 + 验收标准 + 里程碑 + 风险 |

---

## 开发阶段

### Phase 0：脚手架（2 天）
- [ ] Tauri + React + Vite 项目初始化
- [ ] Rust workspace 结构搭建
- [ ] CI 基础（cargo test + 前端 lint）

### Phase 1：双源 CLI（1 周）
- [ ] `kron init` 创建软链接 + 目录结构
- [ ] `kron status` 读取项目状态
- [ ] `kron storage` 模块完整实现
- [ ] MD 解析器（task 格式）
- [ ] `kron task add/move/start/done/list`
- [ ] **无 GUI、无守护进程——只有 CLI**
- [ ] **里程碑 M1：演示脚本（init → add → move → done）跑通**

### Phase 2：守护进程 + 同步（2 周）
- [ ] `kron-daemon` 启动 + Windows 计划任务自启
- [ ] `notify` 监听器 + 200ms 防抖 + 1s 批窗
- [ ] 单实例锁（PID + flock）
- [ ] 双源同步引擎（5 态状态机）
- [ ] 冲突检测 + 备份 + 解决
- [ ] `kron conflict list/show/resolve`（带 --json + 退出码）
- [ ] `kron daemon start/stop/status`
- [ ] `kron ready --json` 哨兵命令
- [ ] **里程碑 M2：单向自动同步跑通**
- [ ] **里程碑 M3：冲突检测+解决完整闭环 + 24h soak test**

### Phase 3：GUI 最小集（2 周，可与 P4 并行）
- [ ] Tauri 命令注册 + Zustand store
- [ ] 项目列表视图（左侧栏）
- [ ] 文件列表视图（中间栏，仿 Windows 资源管理器）
- [ ] Task 看板视图（4 列 + 拖拽）
- [ ] 双击打开 MD（系统默认应用）
- [ ] 5 分钟自动刷新
- [ ] **里程碑 M4：GUI 三大视图都能用**

### Phase 4：CLI 完善 + Vertex 管理（1 周，可与 P3 并行）
- [ ] `kron vertex create/list/delete/show` + Git 树推导
- [ ] `kron task back/check/attach` 等完整操作
- [ ] `kron important add/list/remove/show`
- [ ] `kron tag add/list/remove`
- [ ] Shell 补全脚本（PowerShell + bash + zsh）
- [ ] 完整错误信息（带 suggestion）

### Phase 5：AI 易读层（1 周）
- [ ] `.kron-context/` 生成器（git/code/api 三种）
- [ ] README 文档（AI 启动读取指南）
- [ ] `kron context build/update/show`
- [ ] **里程碑 M5：AI 启动测试 100% 准确**

### Phase 6：打磨 + 打包（1 周）
- [ ] 双主题切换
- [ ] 系统托盘菜单 + 通知
- [ ] MSI 安装包（Windows）
- [ ] 用户手册（中英）
- [ ] 演示视频
- [ ] GitHub Release v1.0.0
- [ ] **里程碑 M6：v1.0 发布**

---

## 关键设计决策记录

| 日期 | 决策 | 位置 |
|------|------|------|
| 2026-09-05 | 00-总览与架构.md 完成 | dev-docs/design/00-总览与架构.md |
| 2026-09-05 | `vertices.json` 为汇总索引，权威源为各 vertex `_meta.json` | 同上 |
| 2026-09-05 | Task 时间字段仅运行时计算，mtime = updated_at | 同上 |
| 2026-09-05 | 守护进程事件 debounce：单文件 200ms，批量 1s | 同上 |
| 2026-09-05 | v1 不做加密/权限检查/远程接口 | 同上 |
| 2026-09-05 | 软链接 fallback：失败时回退到复制模式 | 同上 |
| 2026-09-05 | ShellExecute fallback：`cmd /c start "" "<path>"` | 同上 |
| 2026-09-05 | 01-数据模型.md 完成 | dev-docs/design/01-数据模型.md |
| 2026-09-05 | Task MD 中不存 `created_at` / `updated_at` 元数据（只存 `<!-- kron:task_meta -->` 注释可选） | 同上 |
| 2026-09-05 | Task `VertexTaskStates` 存完整状态历史（用于 back 命令） | 同上 |
| 2026-09-05 | Important 文件用 MD5 hash 校验和做增量同步 | 同上 |
| 2026-09-05 | ConflictRecord 存 `kron-internal/conflicts/<id>.json` | 同上 |
| 2026-09-05 | 7 个 crate：core/storage/git/sync/context/cli/gui/daemon | dev-docs/design/02-模块划分.md |
| 2026-09-05 | `kron-core` 零外部依赖（仅 std）| 同上 |
| 2026-09-05 | `kron-git` 降级为工具模块，无监听接口 | 同上 |
| 2026-09-05 | 守护进程不监听 `.git/` 任何文件 | 同上 |
| 2026-09-05 | MD 解析器作为 `kron-storage` 子模块（避免新 crate）| 同上 |
| 2026-09-05 | 前端用 Vite + React + Zustand + Tailwind | 同上 |
| 2026-09-05 | 双源状态机 5 态：Synced/InternalOnly/ProjectOnly/Conflict/Syncing | dev-docs/design/03-双源同步机制.md |
| 2026-09-05 | 重要文件决策算法：hash 终极仲裁 + mtime 方向辅助 | 同上 |
| 2026-09-05 | 项目内被外部删除时不自动恢复（防误覆盖用户意图）| 同上 |
| 2026-09-05 | Kron 内部被外部删除时自动从项目内恢复 | 同上 |
| 2026-09-05 | 冲突备份存 `kron-internal/conflicts/<id>/` 含双方字节副本 | 同上 |
| 2026-09-05 | 冲突备份永久保留，用户手动清理 | 同上 |
| 2026-09-05 | auto_restore_on_external_delete 默认 false | 同上 |
| 2026-09-05 | 守护进程监听防抖窗口 500ms | 同上 |
| 2026-09-05 | **冲突检测与解决解耦：守护进程不阻塞等待决策** | dev-docs/design/03-双源同步机制.md §5.1 |
| 2026-09-05 | **AI 友好约定：所有命令支持 `--json`、语义化退出码、结构化错误** | 同上 §5.8 |
| 2026-09-05 | **`kron conflict show --json` 输出含结构化 diff（unified 格式）** | 同上 §5.8.4 |
| 2026-09-05 | **`kron ready` 哨兵命令：AI 工作流起点** | 同上 §5.8.6 |
| 2026-09-05 | **AI 是冲突决策一等公民：通过 CLI 主动查询/解决，无需人类介入** | 同上 §5.9 |
| 2026-09-05 | **借鉴 Git 设计原则：按需调用、轮询查询、零 HTTP 端点、零主动通知** | 同上 §5A |
| 2026-09-05 | **Q8 修订：v1 仅 CLI，不做守护进程 HTTP 端点** | 同上 §14 Q8 |
| 2026-09-05 | **守护进程单例约束：每工作区仅一个实例，PID 文件 + flock 双重防护** | dev-docs/design/04-守护进程与文件监听.md §2.1, §10 |
| 2026-09-05 | **守护进程 8 个退出码（0/10/11/12/13/14/1），13 = 监听器启动失败** | 同上 §2.4 |
| 2026-09-05 | **Handler 隔离：单文件串行，跨文件并行，panic 不影响其他** | 同上 §5.3, §8.1 |
| 2026-09-05 | **看门狗策略：1h 内 ≥5 次 panic 触发主动退出 14** | 同上 §8.2 |
| 2026-09-05 | **守护进程不监听自家 `kron-internal/**`，避免死循环** | 同上 §3.2 |
| 2026-09-05 | **守护进程资源预算：< 50 MB 内存，< 1% 空闲 CPU** | 同上 §11.1 |
| 2026-09-05 | **Q11-Q15：v1 守护进程仅 Windows，macOS/Linux v2；外部监控重启** | 同上 §14 |
| 2026-09-05 | **7 Phase 实施路线图：MVP = P0-P2（CLI + 守护进程），GUI 可后置** | dev-docs/design/07-实施路线图.md §1.4 |
| 2026-09-05 | **7 个里程碑：M1 CLI 演示 / M2 单向同步 / M3 冲突闭环 / M4 GUI / M5 AI 测试 / M6 v1.0 发布 / M7 用户反馈** | 同上 §9.1 |
| 2026-09-05 | **风险预案：W4 末未到 M3 则砍 GUI，W6 末未到 M4 则砍 P4 一半** | 同上 §10.1 |
| 2026-09-05 | **北极星指标：AI 100% 准确返回"doing 状态的 task" 即 v1.0 成功** | 同上 附录 B |
| 2026-09-05 | **CLI `-m`/`--message`/`--desc` 同义：task add/vertex create/describe 通用短描述 flag；limit 200 字符** | dev-docs/design/04b-CLI设计.md § 3.2/3.3 + Q23-Q25 |
| 2026-09-05 | **task 用两层描述：description（短，存 tasks.json，AI 主读）+ MD 正文（长，存 .md 文件）** | 同上 § 3.2 |

---

## 待讨论问题（需求文档中未解决）

| # | 问题 | 我的建议 | 状态 |
|---|------|---------|------|
| 1 | Kron 内部数据是否加密？ | 不做（v1 个人本地工具） | ⬜ 待确认 |
| 2 | 云盘自动备份 UI？ | 不做 UI，用户用云盘客户端同步 data 目录 | ⬜ 待确认 |
| 3 | Vertex 删除是否保留 task？ | 不做（避免孤儿 task） | ⬜ 待确认 |
| 4 | 守护进程扫描频率？ | 默认 5 分钟，可配置 | ⬜ 待确认 |
| 5 | 首次启动引导设置默认 MD 应用？ | 不做 | ⬜ 待确认 |
| 6 | ShellExecute 失败的 fallback？ | `cmd /c start "" "<path>"` | ⬜ 待确认 |
| 7 | Vertex 默认值是否预设？ | 不预设，用户自行创建 | ⬜ 待确认 |

---

**最后更新**：2026-09-05
