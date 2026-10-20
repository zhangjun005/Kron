> **已归档 (2026-10-19)** — 这是 `projects.json` 交付的 PR body 存档，不是 RFC；仅作历史记录。

## RFC 拍板

[docs/rfc/2026-10-14-projects-json-schema-extension.md](./2026-10-14-projects-json-schema-extension.md) §2 拍板的最终 schema, 配 [docs/rfc/2026-10-13-multi-project-registry.md §2.8](../2026-10-13-multi-project-registry.md) 的 resolveMCPRoot fallback 优先级.

## 改动 (3 sub-commits)

| Commit | 范围 |
|---|---|
| `33de13e` | `internal/store/project_registry.go` (236 行) + 17 单元测试. ProjectRegistry / ProjectEntry 类型 + ReadProjectsJSON / WriteProjectsJSON / RegisterProject / SetCurrentProject / TouchLastOpened. omitempty + Go 零值兜底, 走现有 `writeFileAtomic` 原子写, KRON_PROJECTS_JSON_PATH 仅作测试旁路 |
| `3cfc7d8` | `kron register` 新子命令 + `kron init` 自动 register. 4 CLI 测试 |
| `84cf93c` | `serve-mcp/resolveMCPRoot` 3 级 fallback (-CWD > current_project > Getwd). 4 测试 |

## 测试

- `go test ./... -count=1 -short` 11/11 包全绿
- `go vet ./...` clean
- `gofmt -l .` clean

## 未做 (按 RFC §4 拍板范围)

- §4 #4 GUI 侧 — 铁律 #9: 客户端不进主仓, 主仓 frontend/ 只有 README 占位. 改 GUI 走独立仓
- §4 #5 整体 e2e 覆盖 — §1-§3 的单元测试已覆盖关键路径, 无独立 e2e

## 待 reviewer

1. `KRON_PROJECTS_JSON_PATH` env 作测试旁路是否可接受 (RFC §3 拍板"❌ 不做 KRON_PROJECTS_PATH" 指生产; 旁路只对 `ProjectsJSONPath()` 一处)
2. `kron register` 触红线"v1 CLI 最小集 init/add/lint/serve-mcp/serve-lsp", 但 RFC 拍板"按 RFC 执行" 优先于 AGENTS.md 红线. 接受?
3. `cli/init.go` 中 register 失败走 warning 而非 fatal — init 主流程 (.kron/ + config.toml) 成功即返回 0

## Plan

[docs/rfc/2026-10-14-projects-json-schema-extension.md](./2026-10-14-projects-json-schema-extension.md)
