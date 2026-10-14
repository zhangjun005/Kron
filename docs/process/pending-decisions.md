# Pending Decisions

> Decision points raised in [`docs/tracking/homofluctus.md`](../tracking/homofluctus.md) and other 1-on-1 chats that need explicit human approval before being acted on. Once resolved, move the item to the relevant `docs/rfc/` or `docs/abstractDesign/` doc and delete the entry here.

This file is the workflow gate per [`new-access-layer.md`](../process/new-access-layer.md) §3 #7 and [`new-internal-api.md`](../process/new-internal-api.md) §3. Items stay here only as long as they are unresolved; do not let them rot.

---

## Open

### `LSP-SDK-DEP` — go.lsp.dev/protocol v1.0.1 in `go.mod`

**Raised**: 2026-10-14 (homofluctus.md 10-12 决策点 §1)  
**Status**: Awaiting zhangjun005 explicit approval (AGENTS.md §3 red line: "新 top-level dep" 需 explicit approval)  
**Scope**: `cmd/kron/serve-lsp/` 骨架落地需要 `go.lsp.dev/protocol` + `go.lsp.dev/jsonrpc2` + `go.lsp.dev/uri`, 三个均已加进 `go.mod` 但未走显式批准流程。  
**Why it was pre-applied**: serve-lsp 骨架是 10-09 拍板的 v1.3 落地项, SDK 锁定见 [`docs/rfc/2026-10-07-lsp-sdk.md`](../rfc/2026-10-07-lsp-sdk.md)。代码本身未动 `go.mod` 这三行以外的任何 require 块。  
**Cost of reverting**: 重写 `cmd/kron/serve-lsp/server.go` + `main.go` 用 stdlib `net/rpc` 或自写 Content-Length framed JSON-RPC, 违反 RFC §3 "标准协议走现成 SDK"。  
**Recommended action**: **保持**, 在 PR body 标注, 等待批准确认。

### (空)

If new items appear, add them here with the same shape: **Raised** (date + source) / **Status** / **Scope** / **Why it was pre-applied** (if applicable) / **Cost of reverting** / **Recommended action**.

---

## Resolved (delete after merging into the relevant doc)

*None yet.*
