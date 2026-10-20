# Pending Decisions

> Decision points raised in [`docs/tracking/homofluctus.md`](../tracking/homofluctus.md) and other 1-on-1 chats that need explicit human approval before being acted on. Once resolved, move the item to the relevant `docs/rfc/` or `docs/abstractDesign/` doc and delete the entry here.

This file is the workflow gate per [`new-access-layer.md`](../process/new-access-layer.md) §3 #7 and [`new-internal-api.md`](../process/new-internal-api.md) §3. Items stay here only as long as they are unresolved; do not let them rot.

---

## Open

### (空)

If new items appear, add them here with the same shape: **Raised** (date + source) / **Status** / **Scope** / **Why it was pre-applied** (if applicable) / **Cost of reverting** / **Recommended action**.

---

## Resolved

### `LSP-SDK-DEP` — go.lsp.dev dependencies in `go.mod`

Resolved by [`2026-10-07-lsp-sdk.md`](../rfc/archive/2026-10-07-lsp-sdk.md) §3, which explicitly states that the RFC is the approval for the selected LSP SDK dependencies. The pending entry incorrectly said approval was still outstanding.
