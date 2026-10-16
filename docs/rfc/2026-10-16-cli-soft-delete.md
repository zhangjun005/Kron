# RFC: CLI intent trash and restore commands

| Field | Value |
|---|---|
| Status | Accepted for implementation in this task (2026-10-16) |
| Scope | `kron trash <slug>` and `kron restore <slug>` |

## Decision

The CLI calls `internal/store.Writer.MoveToTrash` and
`internal/store.Writer.RestoreFromTrash` directly. It does not spawn
`kron serve-mcp`, speak MCP JSON-RPC, or import another access layer.
MCP remains an independent protocol adapter over the same store package.

This resolves the CLI↔MCP bridge question in favor of the existing
architecture rule that protocol access layers must not call one another.
The shared behavior is already available in the business layer, so a bridge
would add a second process, protocol failure modes, and no reusable logic.

## CLI contract

- `kron trash <slug>` moves an existing intent into `.kron/.trash/`.
- `kron restore <slug>` restores a trashed intent.
- Each command accepts exactly one slug and no extra flags.
- Success prints the action and slug to stdout; failures include command
  context on stderr and return a non-zero exit code.
- Existing store sentinel errors retain their meaning; no new store API or
  data format is introduced.
- The commands do not attempt to lock across processes beyond the existing
  store filesystem behavior.

## Tests

CLI tests cover dispatch, required/extra arguments, successful execution,
missing intents, repeated trash, restore without trash, and the resulting
filesystem state. Store behavior remains covered by `internal/store` tests.

## Boundaries

The CLI package may depend on `internal/store`. It must not import
`cmd/kron/serve-mcp` or call MCP handlers. No new Go dependency is needed.
