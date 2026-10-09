<!--
PR 必填字段说明见 docs/process/new-access-layer.md §5.2
新访问层 PR 必填 §"内部访问层" / §"RFC 链接" / §"文档同步声明"
internal/ API 改动 PR 必填 §"调用方清单" / §"RFC 链接"
-->

## Summary

<!-- 一句话: 这个 PR 解决什么问题 / 加什么能力 -->

## Scope

<!--
新加访问层 / 新加 internal 子包 / 新加 CLI flag / 改 frontmatter / ...
具体类别: 添加 / 修改 / 删除
-->

## Type of change

- [ ] 🆕 New access layer (`cmd/kron/serve-<layer>/`)
- [ ] 🆕 New `internal/` package
- [ ] ✏️  Change to existing `internal/` API (签名 / 字段 / 行为)
- [ ] 📝 Docs only (RFC / process / AGENTS / rule mirror)
- [ ] 🐛 Bug fix
- [ ] 🧪 Test only
- [ ] 🔧 Tooling / CI

## Related RFC (S6 必填)

<!--
如有 RFC, 链接到 docs/rfc/<file>.md
无 RFC → 写 "无 (本次无需 RFC)" 并说明原因
-->

[docs/rfc/<YYYY-MM-DD>-<name>.md](path) **OR** "无 (本次无需 RFC, 因为 <一句话原因>)"

## 受影响的访问层

<!--
列出**所有**被改动的访问层 (即使只是加依赖 / 加配置, 也要列出)
未列出的访问层 = PR 流程违反
-->

- [ ] `cmd/kron/cli/`
- [ ] `cmd/kron/serve-mcp/`
- [ ] `cmd/kron/serve-lsp/` (3-of-3 协议访问层, 当前 stub; 实施按 `docs/rfc/2026-10-07-lsp-sdk.md` + `docs/rfc/2026-10-08-lsp-client.md`)
- [ ] **不适用** (本 PR 不动访问层 — 解释: ____)

## internal/ 接口面 (新加 / 改 API 时必填)

<!--
列出本 PR 改动**后**会暴露给访问层的 internal 符号
新加: 列出签名
改: 旧签名 → 新签名
删: 列出符号 + 替代方案
-->

| 符号 | 变化 | 签名 |
|---|---|---|
| | 新加 / 改 / 删 | |

## 调用方清单 (新加 / 改 internal API 时必填)

<!--
列出**所有**会调本 PR 改动后 internal 符号的访问层
- 同一 PR 内已改: ✅
- 同 PR 内待改: 🚧
- 后续 PR 改: 📅 (写 PR 链接占位)
- 不调: N/A
-->

| 调用方 | 状态 |
|---|---|
| `cmd/kron/cli/` | ✅ / 🚧 / 📅 / N/A |
| `cmd/kron/serve-mcp/` | ✅ / 🚧 / 📅 / N/A |
| `cmd/kron/serve-lsp/` (stub; 实施按 LSP RFC 拍板) | ✅ / 🚧 / 📅 / N/A |

## 文档同步声明 (S6 必填)

<!--
AGENTS.md "Mirror of architecture.md" 纪律
改 architecture.md 必须回写 AGENTS.md + .cursor/rules/
-->

- [ ] `docs/abstractDesign/architecture.md` — 已同步 / 不需要 (原因: ____)
- [ ] `AGENTS.md` — 已镜像 / 不需要 (原因: ____)
- [ ] `.cursor/rules/project-conventions.mdc` — 已镜像 / 不需要 (原因: ____)
- [ ] `docs/implementation/*.md` — 已更新 / 不需要
- [ ] `docs/process/*.md` (新增流程文档) — 已写 / 不需要
- [ ] `docs/rfc/*.md` (新增 RFC) — 已合 master / 同 PR 内

## Test 覆盖

- [ ] 单元测试 (`go test ./...` 通过)
- [ ] 集成测试 (stdIO 协议 / 跨进程, 如果有)
- [ ] CI 静态检查 (`go vet ./...` + `gofmt -l .` 通过)
- [ ] **新加访问层**: 手测 stdIO 启动 / 关停 / 错误路径

## Pre-merge checklist (S5 合并纪律)

- [ ] `go vet ./...` 通过
- [ ] `gofmt -l .` 无输出
- [ ] `go test ./...` 通过
- [ ] §"文档同步声明"全部勾上
- [ ] §"调用方清单"无遗漏
- [ ] 自分支基于最新 master (`git fetch origin && git rebase origin/master`)
- [ ] **未**直接 commit 到 master

## Notes for reviewer

<!--
给 reviewer 看的额外说明:
- 关键设计取舍
- 已知限制 / TODO
- 风险点
-->
