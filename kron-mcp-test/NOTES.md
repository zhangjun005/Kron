# Kron MCP Smoke Test — 2026-10-15

**Status**: ✅ **All 7 MCP calls succeeded against kron v1.2.0 on Windows.**

This directory is a self-contained MCP smoke test environment for the
optimize/mcp branch. It contains:

- `bin/kron.exe` — Kron binary built from `optimize/mcp @ c4ba92a` (rebuild with `go build -o bin\kron.exe E:\works\Kron\cmd\kron`)
- `bin/go-smoke.exe` — Go MCP client that drives a real newline-delimited JSON-RPC session
- `go-client/` — Source for the Go client (so you can audit it; rebuild with `go build -o ..\bin\go-smoke.exe .`)
- `project/` — A throwaway Kron project (`.kron/intents/`, `config.toml`, `projects.json`) that the smoke client creates and tears down on every run
- `logs/` — captured stderr, request/response bytes for debugging

## Why PowerShell + cmd alone failed

I first tried driving MCP from PowerShell:

1. **PowerShell `[Process]::Start` + `BaseStream.Write`** prepends a UTF-8 BOM
   (`0xEF 0xBB 0xBF`) to the child's stdin. Kron's Go json decoder sees
   `\ufeff` as the first byte and bails with `invalid character '\ufeff'
   looking for beginning of value`.

2. **PowerShell `cmd /c ... < req.bin`** works for byte fidelity (Python
   peek confirmed kron's first stdin byte is `{` 0x7B) but the session
   exits before producing any stdout: `<` closes stdin immediately, the
   SDK's read goroutine sees EOF before it can write back the initialize
   response, and `server.Run` returns cleanly with nothing emitted. The
   "0 bytes stdout" was a **stdin lifecycle** problem, not a Kron bug.

3. **Kron's own e2e tests skip on Windows** for file-redirect stdin
   (`TestE2E_StdinRedirectWorks` is unconditionally skipped when
   `runtime.GOOS == "windows"` — see the comment around `e2e_stdio_test.go:447-451`).
   The "go test ./... 11/11 green" from the prior session was the
   in-memory tests only; KRON_E2E was not set, so `cmd/kron/serve-mcp/e2e`
   was entirely skipped.

## What works

The Go client in `go-client/main.go` follows the same pattern as
Kron's own `TestE2E_StdioInitializeThenLint`:

- `exec.Command` + `cmd.StdinPipe()` + `cmd.StdoutPipe()` + `cmd.StderrPipe()`
- `bufio.Scanner` draining stdout in a goroutine
- JSON-RPC requests serialised as `json.Marshal(req) + "\n"`
- No BOM, no `<` redirect, no `cmd.Stdin = *os.File` (which is the
  Windows-broken case)

The smoke test exercises the **core path** you asked for:

| # | Call                          | Result                                              |
|---|-------------------------------|-----------------------------------------------------|
| 1 | `initialize`                  | serverInfo = `kron v1.2.0`, full capability table   |
| 2 | `notifications/initialized`   | (no response, by spec)                              |
| 3 | `tools/list`                  | 27841 bytes — 17 tools registered                   |
| 4 | `tools/call` `kron_add`       | `auth/jwt` → `.kron/intents/auth/jwt.md`            |
| 5 | `tools/call` `kron_add`       | `payments/charge` → `.kron/intents/payments/charge.md` |
| 6 | `tools/call` `kron_list`      | 2 intents, with `updated_at`                        |
| 7 | `tools/call` `kron_get`       | `auth/jwt` — full body + frontmatter + source_path  |
| 8 | `tools/call` `kron_lint`      | 0 errors, 2 warnings (`intent-orphan-under-no-node`)|

(Note: id numbering changed in the raw script — id=1 is initialize,
id=2 is tools/list, then id=3..7 cover the five tool calls. The Go
script's "ALL 7 CALLS" comment counts the tool-call responses only.)

## How to reproduce

```
cd E:\works\kron-mcp-test\go-client
go build -o ..\bin\go-smoke.exe .
..\bin\go-smoke.exe
```

Each run wipes and re-initialises `E:\works\kron-mcp-test\project/`.

## Side notes (not bugs, just observations)

- `kron_add` rejects `title`, `summary`, `status` — only `slug`,
  `symbol`, `why`, `kind`, `assumptions` are valid. The smoke client
  uses `why` as the user-facing intent body and the `kind` is the
  on-disk form selector (`"leaf"` vs `"node"`).
- After `kron_add`, `kron_lint` reports `intent-orphan-under-no-node`
  as **warnings** (not errors) because `auth/jwt` and `payments/charge`
  have no parent `README.md` node intent. Lint exit code is still 0.
  This is expected behaviour for the "add a deep leaf without first
  creating a parent node" sequence.
- The `kron_add` write goes through `kron.Add` → `store.Writer.Create`
  → `writeFileAtomic`, and the MCP handler returns the canonical
  `.kron/intents/<slug>.md` (or `<slug>/README.md` for nodes) path.

## F5 (TreeNode.Children []any) follow-up

The `[]any` workaround at `tools_schema.go:444-460` does not affect
this smoke test — `kron_tree` was not exercised. The fix is still
blocked behind the SDK-bump gate per `docs/snapshot/2026-10-14-BranchOptimizeMcp.md`.
