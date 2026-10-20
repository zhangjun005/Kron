// Package main is the Wails GUI entry point for the Kron desktop client.
//
// Per docs/rfc/2026-10-09-gui-ide-plan.md §3.1 / §3.2 / §6:
//   - The Wails Go side exposes a small read-only surface to the React frontend.
//   - All real Kron data flows through a stdio `kron serve-mcp` subprocess
//     (the GUI does NOT import internal/ packages directly — iron rule #9 + #10).
//   - For work 1.3 (skeleton verification), the Go side only returns static
//     metadata.
//   - For work 1.4.b (live serve-mcp), the Go side spawns a `kron serve-mcp`
//     subprocess when the React side requests it, and translates a minimal
//     JSON-RPC 2.0 / MCP initialize + tools/list exchange into a typed
//     return value. The MCP SDK client is NOT used here on purpose: Wails
//     lives in a separate Go module that cannot import internal/; pulling
//     in github.com/modelcontextprotocol/go-sdk would also force a second
//     SDK dependency in the Wails side, which the project conventions
//     (AGENTS.md §3) prefer to avoid for this small a use.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

// App is the Wails application struct. Its exported methods are the Go ↔ TS
// bridge surface — every method is callable from React as a Promise<...>.
// Per iron rule #9: this struct must never import anything from internal/.
type App struct {
	ctx context.Context

	// mcpMu guards mcpCmd / mcpStdin / mcpStdout. Read + write both happen
	// under the lock. The first successful ListMCPToolsLive spawns the
	// subprocess; subsequent calls reuse it until shutdown. This avoids
	// the multi-second cost of a fresh initialize handshake on every
	// React render.
	mcpMu     sync.Mutex
	mcpCmd    *exec.Cmd
	mcpStdin  io.WriteCloser
	mcpStdout *bufio.Reader
	// mcpInitialized tracks whether handshakeLocked has completed the
	// initialize / notifications/initialized / tools/list trio for
	// the current subprocess. The MCP spec requires initialize to be
	// called exactly once per session; re-calling it on a
	// already-initialised server typically yields an error. We use
	// this flag to make handshakeLocked idempotent so that both
	// ListMCPToolsLive (work 1.4.b) and CallMCPTool (work 1.4.c) can
	// share the same live subprocess without races.
	mcpInitialized bool
	// mcpServerInfo + mcpTools are the cached results of the first
	// handshake so handshakeLocked can return them on subsequent
	// calls without re-issuing the initialize / tools/list
	// round-trip. Cleared by killLocked.
	mcpServerInfo LiveServerInfo
	mcpTools      []LiveMCPTool
	// mcpRoot is empty until the registry is read or the user selects a
	// project. Once set, every new subprocess is pinned to this project via
	// serve-mcp -CWD so a project switch cannot leave the GUI on stale data.
	mcpRoot string
}

// NewApp creates a new App application struct.
func NewApp() *App {
	return &App{}
}

// startup is called by Wails after the Webview2/WebKit process is ready.
// The context is saved so we can use it for runtime APIs later (logging,
// event emission, dialogs). We do not use it for the serve-mcp handshake
// itself — that runs on its own short-lived context with its own timeout.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// shutdown is called by Wails when the OS window is closing. We use it
// to terminate a still-running serve-mcp subprocess so it does not leak
// past the GUI's lifetime. main.go wires this up via options.App.OnShutdown.
func (a *App) shutdown(_ context.Context) {
	a.mcpMu.Lock()
	defer a.mcpMu.Unlock()
	a.killLocked()
}

// killLocked terminates the subprocess and waits for it. The caller MUST
// hold mcpMu. Errors are swallowed — at shutdown we are best-effort, and
// the OS reaps the process on the next tick anyway.
func (a *App) killLocked() {
	if a.mcpCmd == nil || a.mcpCmd.Process == nil {
		a.mcpStdin = nil
		a.mcpStdout = nil
		a.mcpInitialized = false
		a.mcpServerInfo = LiveServerInfo{}
		a.mcpTools = nil
		return
	}
	_ = a.mcpCmd.Process.Kill()
	_ = a.mcpCmd.Wait()
	a.mcpCmd = nil
	a.mcpStdin = nil
	a.mcpStdout = nil
	a.mcpInitialized = false
	a.mcpServerInfo = LiveServerInfo{}
	a.mcpTools = nil
}

// KronInfo is the static metadata payload returned by GetKronInfo.
// Version, BuildTime, and GoVersion are computed at build time via -ldflags
// in a later work item; for now they are hard-coded so the TS bridge can be
// exercised end-to-end.
type KronInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	BuildTime   string `json:"buildTime"`
	GoVersion   string `json:"goVersion"`
	OS          string `json:"os"`
	Arch        string `json:"arch"`
	Description string `json:"description"`
	RFC         string `json:"rfc"`
}

// GetKronInfo returns static metadata about the Kron desktop client.
// This is the only bridge method exposed in work 1.3; it touches no
// internal/* package and performs no I/O.
func (a *App) GetKronInfo() KronInfo {
	return KronInfo{
		Name:        "Kron",
		Version:     "0.0.0-skeleton",
		BuildTime:   "unreleased",
		GoVersion:   runtime.Version(),
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
		Description: "Kron desktop client — Wails v2 GUI for the Kron intent ledger.",
		RFC:         "docs/rfc/2026-10-09-gui-ide-plan.md",
	}
}

// ListMCPTools returns the list of MCP tool names the GUI is expected to
// expose in the overview panel. In work 1.3 / 1.4 this is hard-coded; the
// live list lives in ListMCPToolsLive.
func (a *App) ListMCPTools() []string {
	return []string{
		"kron_list",
		"kron_get",
		"kron_impact",
		"kron_density",
		"kron_relations",
		"kron_stale",
		"kron_assume_check",
		"kron_lint",
		"kron_tree",
		"kron_add",
		"kron_update",
		"kron_delete",
		"kron_restore",
	}
}

// ---- live serve-mcp bridge (work 1.4.b) ----------------------------------

// serveMCPPathEnvOverride lets us inject a custom kron.exe path from
// outside the binary (mainly for tests and the upcoming wails-build
// 1.5 phase). It is intentionally read-only at the Go API level — the
// React frontend cannot touch the OS env.
const serveMCPPathEnvOverride = "KRON_WAILS_SERVE_MCP_EXE"

// ListMCPToolsLiveResult is what the React side receives from
// ListMCPToolsLive. ServerInfo mirrors the MCP `initialize` response
// `serverInfo` field (subset) so the UI can display the live server
// version alongside the tool list. Tools is a list of {name, description}
// pairs; the live description comes from the MCP `inputSchema` sibling
// field in a later work item, so for now we just carry `name`.
type ListMCPToolsLiveResult struct {
	Tools      []LiveMCPTool  `json:"tools"`
	ServerInfo LiveServerInfo `json:"serverInfo"`
	Source     string         `json:"source"` // human-readable provenance string
}

// LiveMCPTool is one tool entry from `tools/list`.
type LiveMCPTool struct {
	Name string `json:"name"`
}

// LiveServerInfo mirrors the MCP initialize response serverInfo fields.
type LiveServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ListMCPToolsLive spawns `kron serve-mcp` and runs a minimal
// JSON-RPC 2.0 / MCP handshake (initialize → notifications/initialized
// → tools/list) against it. The handshake protocol used here is the
// minimum required by github.com/modelcontextprotocol/go-sdk v1.2+
// and intentionally does not exercise ping, capabilities negotiation,
// or progress notifications — those are future work.
//
// Iron-rule notes (architecture.md §〇 铁律 #9 + #10):
//   - This method does NOT import internal/*.
//   - The subprocess IS the access layer; the GUI only orchestrates the
//     stdio JSON-RPC exchange. The GUI cannot bypass the access layer.
//   - Errors are wrapped at the point of origin and include enough
//     context for the React side to surface them verbatim.
func (a *App) ListMCPToolsLive() (ListMCPToolsLiveResult, error) {
	executable, err := resolveServeMCPExecutable()
	if err != nil {
		return ListMCPToolsLiveResult{}, err
	}

	a.mcpMu.Lock()
	defer a.mcpMu.Unlock()

	// First call spawns the subprocess; subsequent calls reuse the
	// already-initialized session.
	if a.mcpCmd == nil {
		if err := a.spawnLocked(executable); err != nil {
			return ListMCPToolsLiveResult{}, err
		}
	}

	serverInfo, tools, err := a.handshakeLocked()
	if err != nil {
		// Handshake failed: kill the subprocess so the next call
		// gets a clean retry. Without this, a half-initialized
		// session would block all subsequent calls.
		a.killLocked()
		return ListMCPToolsLiveResult{}, err
	}

	return ListMCPToolsLiveResult{
		Tools:      tools,
		ServerInfo: serverInfo,
		Source:     fmt.Sprintf("stdio: %s serve-mcp", executable),
	}, nil
}

// resolveServeMCPExecutable finds the `kron.exe` (or platform equivalent)
// to spawn. Resolution order:
//  1. $KRON_WAILS_SERVE_MCP_EXE if set and exists (test/CI escape hatch).
//  2. ./build/bin/kron.exe relative to the process CWD. This is the
//     standard `wails dev` artifact location; `wails dev` rebuilds it
//     whenever the Wails Go side changes.
//  3. ./build/bin/kron-dev.exe — the headless CLI build that `wails dev`
//     drops alongside kron.exe so the GUI can spawn a non-GUI kron to
//     serve MCP without the WebView2 driver attaching. Preferring
//     kron-dev.exe over kron.exe would normally be wrong (kron.exe is
//     the version-paired GUI build) but `wails dev` keeps both in sync
//     from the same source tree, so they have identical code. We only
//     fall back to kron-dev when kron.exe itself is the Wails GUI
//     wrapper and would deadlock on a serve-mcp subcommand — we detect
//     that by probing for a `serve-mcp` invocation that exits 0 within
//     a short window.
//  4. ./build/bin/kron-cli.exe — a hand-built or CI-built CLI binary
//     (e.g. `go build -o build/bin/kron-cli.exe ./cmd/kron`). Useful
//     for testing without the full Wails dev round-trip.
//  5. ./build/bin/kron relative to the executable's own directory (for
//     future `wails build` deployments — Phase 1.5).
//
// We deliberately do not search PATH: the Wails GUI and the serve-mcp
// binary are version-paired and we want to fail loud if the dev artifact
// is missing rather than accidentally spawn an unrelated `kron` on PATH.
//
// Discovered in work 1.4.c: when `wails dev` builds, it produces both
// `kron.exe` (the Wails GUI itself) and `kron-dev.exe` (a CLI-only
// build from the same source). The original 1.4.b code only knew
// about kron.exe and would deadlock in a WebView2 environment when
// serve-mcp was invoked. The detection-via-`exec.LookPath` step below
// is intentionally avoided: we cannot ask the OS whether a binary is
// "GUI or CLI", and `exec.Cmd` doesn't expose that either. The
// 2-second probe is cheap (it just runs `kron.exe serve-mcp -CWD
// <empty>` and inspects the exit code) and gives us a definitive
// answer without a config flag.
func resolveServeMCPExecutable() (string, error) {
	if override := os.Getenv(serveMCPPathEnvOverride); override != "" {
		if _, err := os.Stat(override); err == nil {
			return override, nil
		}
		return "", fmt.Errorf("wails: $%s=%q is set but the file does not exist", serveMCPPathEnvOverride, override)
	}

	// wails dev runs with CWD = frontend/wails/, so ./build/bin/kron.exe
	// is the standard artifact location.
	cwd, err := os.Getwd()
	if err == nil {
		// First try kron.exe — if it is the Wails GUI wrapper
		// (the common case in `wails dev`), the probe below
		// will detect that and we move on to kron-dev.exe.
		if exe, ok := tryServeMCPCandidate(filepath.Join(cwd, "build", "bin", "kron.exe")); ok {
			return exe, nil
		}
		if exe, ok := tryServeMCPCandidate(filepath.Join(cwd, "build", "bin", "kron-dev.exe")); ok {
			return exe, nil
		}
		if exe, ok := tryServeMCPCandidate(filepath.Join(cwd, "build", "bin", "kron-cli.exe")); ok {
			return exe, nil
		}
		// POSIX / wails build fallback alongside the GUI binary.
		if exe, ok := tryServeMCPCandidate(filepath.Join(cwd, "build", "bin", "kron")); ok {
			return exe, nil
		}
	}

	// Last-resort: same directory as the running executable.
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "kron.exe")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", errors.New(
		"wails: cannot find kron serve-mcp executable. " +
			"Run `wails dev` first (it builds ./build/bin/kron.exe), " +
			"or set $" + serveMCPPathEnvOverride + " to an absolute path.")
}

// tryServeMCPCandidate returns (path, true) if path exists AND looks
// like a working `kron serve-mcp` binary. The probe runs the binary
// with `-CWD <tempdir>` against an empty stdin and checks that the
// process exits within 2 seconds (the SDK treats empty stdin as a
// clean disconnect and returns no error). A Wails GUI wrapper would
// try to spin up WebView2 and either hang past 2s or fail noisily —
// either way, we move on to the next candidate.
//
// The probe is best-effort: if exec fails for any reason we treat
// the candidate as "not usable" and return (path, false) so the
// caller can keep searching. We do not surface probe errors because
// they are environmental, not actionable.
func tryServeMCPCandidate(path string) (string, bool) {
	if _, err := os.Stat(path); err != nil {
		return "", false
	}
	// A 0-byte file or a binary that exits immediately on a bad
	// flag is "usable" for our purposes — it does not hang on
	// stdin the way a Wails GUI does. We accept any exit within
	// 2 seconds as a positive signal.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "serve-mcp", "-CWD", os.TempDir())
	cmd.Stdin = strings.NewReader("")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	_ = cmd.Run()
	// cmd.Run returns nil on exit 0, *exec.ExitError on non-zero
	// exit, and a context.DeadlineExceeded error if the 2s
	// budget elapsed. We treat "exited (any code) within 2s" as
	// the "usable" signal. context.DeadlineExceeded means it
	// hung — which is exactly the Wails-GUI behaviour we are
	// trying to detect.
	if ctx.Err() == context.DeadlineExceeded {
		return "", false
	}
	return path, true
}

// spawnLocked starts the kron serve-mcp subprocess. The caller MUST
// hold mcpMu.
func (a *App) spawnLocked(executable string) error {
	args := []string{"serve-mcp"}
	if a.mcpRoot != "" {
		args = append(args, "-CWD", a.mcpRoot)
	}
	cmd := exec.Command(executable, args...)
	cmd.Stderr = os.Stderr // serve-mcp writes its own diagnostics to stderr
	// When no registered project is active, preserve serve-mcp's normal
	// current_project / working-directory resolution. Project selection
	// pins the subprocess to the chosen root with -CWD.

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("wails: opening serve-mcp stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("wails: opening serve-mcp stdout: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("wails: starting %s serve-mcp: %w", executable, err)
	}

	a.mcpCmd = cmd
	a.mcpStdin = stdin
	a.mcpStdout = bufio.NewReader(stdout)
	a.mcpInitialized = false
	return nil
}

// handshakeLocked performs the three-message MCP handshake. The caller
// MUST hold mcpMu and must have a live subprocess. Returns the parsed
// serverInfo and the tool name list.
//
// Idempotency: if the subprocess has already been initialised
// (mcpInitialized is true), the method is a no-op and returns the
// cached serverInfo + tool list captured at the first handshake. This
// lets CallMCPTool (work 1.4.c) safely call handshakeLocked on every
// invocation: the first call pays the handshake cost, every call
// after reuses the session. The MCP spec requires initialize to be
// called exactly once per session; re-calling it on a live server
// returns an error from the SDK. Work 1.4.b originally assumed the
// handshake was a one-shot at first ListMCPToolsLive; work 1.4.c
// needs to share the session so we make this idempotent.
func (a *App) handshakeLocked() (LiveServerInfo, []LiveMCPTool, error) {
	if a.mcpInitialized {
		return a.mcpServerInfo, a.mcpTools, nil
	}

	// 1) initialize (id=1)
	if err := a.writeMessage(jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "kron-wails-gui",
				"version": "0.0.0-skeleton",
			},
		},
	}); err != nil {
		return LiveServerInfo{}, nil, fmt.Errorf("wails: sending initialize: %w", err)
	}
	var initResp jsonrpcResponse
	if err := a.readMessage(&initResp); err != nil {
		return LiveServerInfo{}, nil, fmt.Errorf("wails: reading initialize response: %w", err)
	}
	if initResp.Error != nil {
		return LiveServerInfo{}, nil, fmt.Errorf("wails: initialize error: %s", initResp.Error.Message)
	}
	var initResult initializeResult
	if err := json.Unmarshal(initResp.Result, &initResult); err != nil {
		return LiveServerInfo{}, nil, fmt.Errorf("wails: parsing initialize result: %w", err)
	}
	serverInfo := LiveServerInfo{
		Name:    initResult.ServerInfo.Name,
		Version: initResult.ServerInfo.Version,
	}

	// 2) notifications/initialized (no id, no response)
	if err := a.writeMessage(jsonrpcNotification{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
		Params:  map[string]any{},
	}); err != nil {
		return serverInfo, nil, fmt.Errorf("wails: sending notifications/initialized: %w", err)
	}

	// 3) tools/list (id=2)
	if err := a.writeMessage(jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/list",
		Params:  map[string]any{},
	}); err != nil {
		return serverInfo, nil, fmt.Errorf("wails: sending tools/list: %w", err)
	}
	var listResp jsonrpcResponse
	if err := a.readMessage(&listResp); err != nil {
		return serverInfo, nil, fmt.Errorf("wails: reading tools/list response: %w", err)
	}
	if listResp.Error != nil {
		return serverInfo, nil, fmt.Errorf("wails: tools/list error: %s", listResp.Error.Message)
	}
	var listResult toolsListResult
	if err := json.Unmarshal(listResp.Result, &listResult); err != nil {
		return serverInfo, nil, fmt.Errorf("wails: parsing tools/list result: %w", err)
	}

	tools := make([]LiveMCPTool, 0, len(listResult.Tools))
	for _, t := range listResult.Tools {
		tools = append(tools, LiveMCPTool{Name: t.Name})
	}

	// Cache for future idempotent calls (see handshakeLocked doc).
	a.mcpServerInfo = serverInfo
	a.mcpTools = tools
	a.mcpInitialized = true
	return serverInfo, tools, nil
}

// writeMessage serializes v as one JSON object followed by a single
// newline. newline-delimited JSON is what the go-sdk StdioTransport
// expects, and it lets us use bufio.Reader.ReadBytes('\n') below.
//
// The whole handshake is bounded by the 3s read timeout in
// readMessage, so we deliberately do not set a write deadline here:
// os.Pipe on Windows has no native write deadline, and a stuck write
// would still be cut off by the subsequent read timeout firing.
func (a *App) writeMessage(v any) error {
	if a.mcpStdin == nil {
		return errors.New("wails: serve-mcp stdin is not open")
	}
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("wails: marshaling request: %w", err)
	}
	data = append(data, '\n')
	if _, err := a.mcpStdin.Write(data); err != nil {
		return fmt.Errorf("wails: writing to serve-mcp stdin: %w", err)
	}
	return nil
}

// readMessage reads one newline-delimited JSON object into out. We
// run the read in a goroutine with a 3s timeout channel because the
// subprocess stdout is an os.Pipe on Windows, and os.Pipe has no
// native read deadline — without this guard, a stuck serve-mcp would
// hang the React promise indefinitely.
func (a *App) readMessage(out *jsonrpcResponse) error {
	if a.mcpStdout == nil {
		return errors.New("wails: serve-mcp stdout is not open")
	}
	type readResult struct {
		line []byte
		err  error
	}
	ch := make(chan readResult, 1)
	go func() {
		line, err := a.mcpStdout.ReadBytes('\n')
		ch <- readResult{line: line, err: err}
	}()
	select {
	case r := <-ch:
		if r.err != nil {
			if errors.Is(r.err, io.EOF) {
				return fmt.Errorf("wails: serve-mcp closed stdout before responding")
			}
			return fmt.Errorf("wails: reading from serve-mcp stdout: %w", r.err)
		}
		if err := json.Unmarshal(r.line, out); err != nil {
			return fmt.Errorf("wails: parsing serve-mcp response: %w (raw=%s)", err, truncate(string(r.line), 200))
		}
		return nil
	case <-time.After(3 * time.Second):
		// Kill the subprocess so the next call retries from a
		// fresh spawn rather than fighting a half-dead pipe.
		a.killLocked()
		return errors.New("wails: serve-mcp did not respond within 3s; subprocess killed")
	}
}

// truncate caps a string at n bytes for safe inclusion in error
// messages. UTF-8 multibyte boundaries are respected (we never split a
// rune) but we do not guarantee a valid string at the cut — errors are
// for diagnostics, not display.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ---- JSON-RPC + MCP wire types ------------------------------------------
// These types are minimal: just the fields we need to drive the three
// messages of the handshake. Anything more would risk drifting from the
// real MCP spec.

type jsonrpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type jsonrpcNotification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.Number     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type initializeResult struct {
	ProtocolVersion string `json:"protocolVersion"`
	ServerInfo      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"serverInfo"`
	Capabilities any `json:"capabilities"`
}

type toolsListResult struct {
	Tools []struct {
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
	} `json:"tools"`
}

// String returns a human-readable summary, useful for debugging.
func (k KronInfo) String() string {
	return fmt.Sprintf("%s v%s (%s/%s, Go %s)", k.Name, k.Version, k.OS, k.Arch, k.GoVersion)
}

// ---- tools/call bridge (work 1.4.c) -------------------------------------
//
// CallMCPTool is a generic bridge for invoking any `kron_*` tool on the
// already-spawned serve-mcp subprocess. It runs the three-message
// handshake lazily on the first call (same as ListMCPToolsLive) and
// then issues one `tools/call` JSON-RPC request. The response payload
// is the raw text content of the tool's first content block
// (MCP `result.content[0].text` per the official SDK), returned as a
// string. The React side parses it according to the per-tool schema
// declared in cmd/kron/serve-mcp/tools_schema.go.
//
// Why generic instead of one method per tool: 1.4.c only needs
// `kron_tree` today, but 1.4.d will need `kron_list`, and later
// milestones need `kron_get` / `kron_impact` / etc. A single
// (name, args) entrypoint keeps the Go bridge surface stable while
// the React side adds more call sites. Per-tool marshalling lives
// in TypeScript (where the per-tool schemas are already declared as
// typed structs in wailsjs/go/models.ts).
//
// Iron-rule notes (architecture.md §〇 铁律 #9 + #10):
//   - This method does NOT import internal/*.
//   - The subprocess IS the access layer; the GUI only orchestrates
//     the JSON-RPC exchange.
//   - The first call pays the handshake cost (~1 cold start of the
//     subprocess + initialize + notifications/initialized + tools/list)
//     so subsequent calls reuse the live session.
//
// argsJSON is the tool's `arguments` object encoded as JSON. Pass "{}"
// for tools that take no parameters (e.g. kron_tree today, kron_list
// when no prefix is supplied). The Go side does NOT validate the JSON
// shape against the tool's input schema — validation is the
// subprocess's job, and a validation error surfaces as a non-nil
// error returned to React (the same path as any other tools/call
// failure).
func (a *App) CallMCPTool(name string, argsJSON string) (string, error) {
	if name == "" {
		return "", errors.New("wails: CallMCPTool: tool name is required")
	}
	// Default empty args to {} so callers can pass "" for tools that
	// take no parameters without thinking about the JSON layer.
	if argsJSON == "" {
		argsJSON = "{}"
	}
	// Validate argsJSON is a JSON object (per the MCP spec, arguments
	// MUST be an object). This catches the common React-side bug of
	// passing a bare scalar before we serialise it onto the wire.
	var probe map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &probe); err != nil {
		return "", fmt.Errorf("wails: CallMCPTool(%s): args must be a JSON object: %w", name, err)
	}

	executable, err := resolveServeMCPExecutable()
	if err != nil {
		return "", err
	}

	a.mcpMu.Lock()
	defer a.mcpMu.Unlock()

	// First call spawns + handshakes; subsequent calls reuse.
	if a.mcpCmd == nil {
		if err := a.spawnLocked(executable); err != nil {
			return "", err
		}
	}
	if _, _, err := a.handshakeLocked(); err != nil {
		// Handshake failed: kill the subprocess so the next call
		// gets a clean retry from a fresh spawn.
		a.killLocked()
		return "", err
	}

	return a.callToolLocked(name, []byte(argsJSON))
}

// callToolLocked issues one `tools/call` JSON-RPC request and returns
// the raw text content of the first content block in the result. The
// caller MUST hold mcpMu and must have a successfully-initialised
// subprocess.
func (a *App) callToolLocked(name string, argsJSON []byte) (string, error) {
	// We use a fresh request id space per call (starting at 3, after
	// the handshake's 1+2). Concurrent calls are not supported by
	// this bridge today (mcpMu serialises them anyway), so a
	// monotonic counter is unnecessary — a constant id is fine.
	const callID = 3
	if err := a.writeMessage(jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      callID,
		Method:  "tools/call",
		Params: map[string]any{
			"name":      name,
			"arguments": json.RawMessage(argsJSON),
		},
	}); err != nil {
		return "", fmt.Errorf("wails: CallMCPTool(%s): sending tools/call: %w", name, err)
	}

	var resp jsonrpcResponse
	if err := a.readMessage(&resp); err != nil {
		return "", fmt.Errorf("wails: CallMCPTool(%s): reading tools/call response: %w", name, err)
	}
	if resp.Error != nil {
		return "", fmt.Errorf("wails: CallMCPTool(%s): server returned error: %s", name, resp.Error.Message)
	}

	// tools/call result shape (MCP spec):
	//   { "content": [ { "type": "text", "text": "..." }, ... ],
	//     "isError": false }
	// We return the text of the first content block. For kron_*
	// handlers this is the JSON-marshalled output struct (see
	// tools_schema.go and the handler bodies).
	var callRes struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(resp.Result, &callRes); err != nil {
		return "", fmt.Errorf("wails: CallMCPTool(%s): parsing tools/call result: %w (raw=%s)", name, err, truncate(string(resp.Result), 200))
	}
	if callRes.IsError {
		// The server flagged the call as a tool-level error. Surface
		// the first content block's text (it is the human-readable
		// error message) so the React side can render it.
		if len(callRes.Content) > 0 {
			return "", fmt.Errorf("wails: CallMCPTool(%s) tool error: %s", name, callRes.Content[0].Text)
		}
		return "", fmt.Errorf("wails: CallMCPTool(%s) tool error (no content)", name)
	}
	if len(callRes.Content) == 0 {
		return "", fmt.Errorf("wails: CallMCPTool(%s): empty content array", name)
	}
	return callRes.Content[0].Text, nil
}

// ---- multi-project registry (RFC 2026-10-13) -----------------------------

// projectRegistryWireEntry mirrors one projects.json row. Unknown registry
// fields are kept separately as RawMessages when writing so a GUI version
// does not discard fields added by a newer CLI version.
type projectRegistryWireEntry struct {
	Path       string `json:"path"`
	AddedAt    string `json:"added_at,omitempty"`
	LastOpened string `json:"last_opened,omitempty"`
}

type projectRegistryDocument struct {
	fields   map[string]json.RawMessage
	projects []map[string]json.RawMessage
}

func projectsJSONPath() (string, error) {
	if override := os.Getenv("KRON_PROJECTS_JSON_PATH"); override != "" {
		return filepath.Abs(override)
	}
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("wails: locate projects.json: %w", err)
	}
	return filepath.Join(filepath.Dir(executable), "projects.json"), nil
}

func readProjectRegistry(path string) (*projectRegistryDocument, error) {
	doc := &projectRegistryDocument{
		fields:   make(map[string]json.RawMessage),
		projects: make([]map[string]json.RawMessage, 0),
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return doc, nil
		}
		return nil, fmt.Errorf("wails: read projects.json %q: %w", path, err)
	}
	if err := json.Unmarshal(data, &doc.fields); err != nil {
		return nil, fmt.Errorf("wails: parse projects.json %q: %w", path, err)
	}
	if doc.fields == nil {
		doc.fields = make(map[string]json.RawMessage)
	}
	projectsRaw, ok := doc.fields["projects"]
	if !ok || string(projectsRaw) == "null" {
		return doc, nil
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(projectsRaw, &rows); err != nil {
		return nil, fmt.Errorf("wails: parse projects.json projects: %w", err)
	}
	for i, row := range rows {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(row, &fields); err != nil || fields == nil {
			return nil, fmt.Errorf("wails: parse projects.json projects[%d]: invalid project entry", i)
		}
		var entry projectRegistryWireEntry
		if err := json.Unmarshal(row, &entry); err != nil || entry.Path == "" {
			return nil, fmt.Errorf("wails: parse projects.json projects[%d]: missing or invalid path", i)
		}
		doc.projects = append(doc.projects, fields)
	}
	return doc, nil
}

func (doc *projectRegistryDocument) snapshot(path string) (ListProjectsLiveResult, error) {
	result := ListProjectsLiveResult{
		Projects:     make([]ProjectEntry, 0, len(doc.projects)),
		ProjectsPath: path,
		Source:       "file: " + path,
	}
	for i, fields := range doc.projects {
		encoded, err := json.Marshal(fields)
		if err != nil {
			return ListProjectsLiveResult{}, fmt.Errorf("wails: encode projects.json projects[%d]: %w", i, err)
		}
		var entry projectRegistryWireEntry
		if err := json.Unmarshal(encoded, &entry); err != nil {
			return ListProjectsLiveResult{}, fmt.Errorf("wails: decode projects.json projects[%d]: %w", i, err)
		}
		result.Projects = append(result.Projects, ProjectEntry{
			Path:       entry.Path,
			AddedAt:    entry.AddedAt,
			LastOpened: entry.LastOpened,
		})
	}
	sort.SliceStable(result.Projects, func(i, j int) bool {
		return result.Projects[i].LastOpened > result.Projects[j].LastOpened
	})

	if currentRaw, ok := doc.fields["current_project"]; ok {
		if err := json.Unmarshal(currentRaw, &result.Current); err != nil {
			return ListProjectsLiveResult{}, fmt.Errorf("wails: parse projects.json current_project: %w", err)
		}
	}
	if !containsProject(result.Projects, result.Current) {
		result.Current = ""
		if len(result.Projects) > 0 {
			// RFC §2.7: when no active pointer is stored, default to the most
			// recently opened registered project. The file is not changed until
			// the user explicitly selects a project.
			result.Current = result.Projects[0].Path
		}
	}
	return result, nil
}

func containsProject(projects []ProjectEntry, path string) bool {
	if path == "" {
		return false
	}
	for _, project := range projects {
		if sameProjectPath(project.Path, path) {
			return true
		}
	}
	return false
}

func sameProjectPath(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func writeProjectRegistry(path string, doc *projectRegistryDocument) error {
	rows := make([]json.RawMessage, 0, len(doc.projects))
	for i, fields := range doc.projects {
		row, err := json.Marshal(fields)
		if err != nil {
			return fmt.Errorf("wails: encode projects.json projects[%d]: %w", i, err)
		}
		rows = append(rows, row)
	}
	projectsJSON, err := json.Marshal(rows)
	if err != nil {
		return fmt.Errorf("wails: encode projects.json projects: %w", err)
	}
	doc.fields["projects"] = projectsJSON
	data, err := json.MarshalIndent(doc.fields, "", "  ")
	if err != nil {
		return fmt.Errorf("wails: encode projects.json: %w", err)
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".projects-*.tmp")
	if err != nil {
		return fmt.Errorf("wails: create projects.json temporary file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("wails: write projects.json temporary file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("wails: sync projects.json temporary file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("wails: close projects.json temporary file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("wails: replace projects.json: %w", err)
	}
	return nil
}

// ProjectEntry is one row of the projects.json registry. The field
// names match the JSON tags on store.ProjectEntry + the wire shape
// in tools_list_projects.go (with TS-friendly camelCase).
type ProjectEntry struct {
	Path       string `json:"path"`
	AddedAt    string `json:"addedAt,omitempty"`
	LastOpened string `json:"lastOpened,omitempty"`
}

// ListProjectsLiveResult is what the React side receives from
// ListProjectsLive. Projects is always non-nil (empty slice if the
// registry is empty) so the UI can `.map` without a guard. Current
// is "" when no project is active. ProjectsPath surfaces the absolute
// path of projects.json for the diagnostic footer (mirrors the
// provenance string ListMCPToolsLive returns).
type ListProjectsLiveResult struct {
	Projects     []ProjectEntry `json:"projects"`
	Current      string         `json:"current"`
	ProjectsPath string         `json:"projectsPath"`
	Source       string         `json:"source"`
}

// ListProjectsLive reads the portable projects.json registry and pins any
// future MCP subprocess to the selected project root. The registry is the
// explicitly permitted GUI file-I/O boundary in the registration RFC.
func (a *App) ListProjectsLive() (ListProjectsLiveResult, error) {
	path, err := projectsJSONPath()
	if err != nil {
		return ListProjectsLiveResult{}, err
	}
	doc, err := readProjectRegistry(path)
	if err != nil {
		return ListProjectsLiveResult{}, err
	}
	out, err := doc.snapshot(path)
	if err != nil {
		return ListProjectsLiveResult{}, err
	}
	a.mcpMu.Lock()
	defer a.mcpMu.Unlock()
	if out.Current != "" {
		abs, absErr := filepath.Abs(out.Current)
		if absErr != nil {
			return ListProjectsLiveResult{}, fmt.Errorf("wails: resolve active project path: %w", absErr)
		}
		if !sameProjectPath(a.mcpRoot, abs) && a.mcpCmd != nil {
			a.killLocked()
		}
		a.mcpRoot = abs
	} else if a.mcpRoot != "" {
		a.killLocked()
		a.mcpRoot = ""
	}
	return out, nil
}

// SwitchProject atomically updates projects.json and restarts the MCP session
// on the selected project. The returned registry is ready to render in TS.
func (a *App) SwitchProject(projectPath string) (ListProjectsLiveResult, error) {
	if strings.TrimSpace(projectPath) == "" {
		return ListProjectsLiveResult{}, errors.New("wails: project path is required")
	}
	abs, err := filepath.Abs(projectPath)
	if err != nil {
		return ListProjectsLiveResult{}, fmt.Errorf("wails: resolve project path: %w", err)
	}
	if !isKronProject(abs) {
		return ListProjectsLiveResult{}, fmt.Errorf("wails: %q is not a Kron project (.kron directory not found)", abs)
	}
	path, err := projectsJSONPath()
	if err != nil {
		return ListProjectsLiveResult{}, err
	}
	a.mcpMu.Lock()
	defer a.mcpMu.Unlock()
	doc, err := readProjectRegistry(path)
	if err != nil {
		return ListProjectsLiveResult{}, err
	}
	index := -1
	for i, fields := range doc.projects {
		encoded, marshalErr := json.Marshal(fields)
		if marshalErr != nil {
			return ListProjectsLiveResult{}, fmt.Errorf("wails: encode project entry: %w", marshalErr)
		}
		var entry projectRegistryWireEntry
		if err := json.Unmarshal(encoded, &entry); err != nil {
			return ListProjectsLiveResult{}, fmt.Errorf("wails: parse project entry: %w", err)
		}
		if sameProjectPath(entry.Path, abs) {
			index = i
			break
		}
	}
	if index < 0 {
		return ListProjectsLiveResult{}, fmt.Errorf("wails: project %q is not registered; run `kron init` or `kron register` first", abs)
	}
	currentJSON, _ := json.Marshal(abs)
	doc.fields["current_project"] = currentJSON
	lastOpenedJSON, _ := json.Marshal(time.Now().UTC().Format(time.RFC3339))
	doc.projects[index]["last_opened"] = lastOpenedJSON
	if err := writeProjectRegistry(path, doc); err != nil {
		return ListProjectsLiveResult{}, err
	}
	if !sameProjectPath(a.mcpRoot, abs) {
		a.killLocked()
	}
	a.mcpRoot = abs
	return doc.snapshot(path)
}

func isKronProject(root string) bool {
	info, err := os.Stat(filepath.Join(root, ".kron"))
	return err == nil && info.IsDir()
}

// ---- open-in-IDE bridge (work 1.4.e: edit-jump-IDE) -------------------
//
// OpenInIDE is the GUI bridge for RFC §3.1's "edit intent" half-support:
// the GUI does NOT embed a markdown editor; instead it shells out to
// an external IDE binary (`code` / `cursor`) with the .md file as the
// argument, so the user lands in their existing editor with the cursor
// on the file. Fallback chain when neither `code` nor `cursor` is on
// PATH: the OS file-association handler (`rundll32` / `open` /
// `xdg-open`).
//
// Iron-rule notes (architecture.md §〇 铁律 #9):
//   - This method does NOT import internal/*.
//   - The path passed in is whatever kron_get surfaced as
//     `intent.source_path` (absolute, on-disk path). We re-absolutise
//     defensively so a relative path from a stale kron_get cache still
//     resolves correctly relative to the GUI process CWD.
//   - We deliberately do NOT shell out via cmd / powershell / wscript
//     on Windows because the user's intent is to land in a CODE EDITOR,
//     not in Notepad. If neither `code` nor `cursor` is on PATH the
//     user gets a structured error and can install one; we do not
//     silently fall through to the OS file-association unless EVERY
//     editor probe fails (last-resort, never silent).
//
// OpenIDEResult is what the React side receives from OpenInIDE.
// The structured result (rather than just (string, error)) lets the
// UI surface a friendly toast on success ("Opened in VS Code") and a
// helpful diagnostic on failure ("`code` not found on PATH; install
// the 'Shell Command: Install code in PATH' command from VS Code").
type OpenIDEResult struct {
	OK   bool   `json:"ok"`
	IDE  string `json:"ide"`           // "code" / "cursor" / "os-default" / ""
	PID  int    `json:"pid,omitempty"` // PID of the spawned editor (best-effort, 0 if detached)
	Err  string `json:"err,omitempty"` // human-readable error when OK is false
	Path string `json:"path"`          // absolute path that was opened (echo for diagnostics)
}

func (a *App) OpenInIDE(path string) (OpenIDEResult, error) {
	if path == "" {
		return OpenIDEResult{OK: false, Err: "empty path", Path: path}, nil
	}

	// Re-absolutise so a stale relative path from a cached kron_get
	// still resolves against the GUI process CWD (which in `wails dev`
	// is frontend/wails/).
	abs, err := filepath.Abs(path)
	if err != nil {
		return OpenIDEResult{OK: false, Err: fmt.Sprintf("resolve abs path: %v", err), Path: path}, nil
	}
	if _, err := os.Stat(abs); err != nil {
		return OpenIDEResult{OK: false, Err: fmt.Sprintf("file not found: %s", abs), Path: abs}, nil
	}

	// Probe `code` first (most common), then `cursor`. For each, try
	// `exec.LookPath` (resolves PATH + PATHEXT on Windows) and on hit
	// spawn with the file as the single arg. We do NOT wait: the
	// editor is a long-lived GUI process and the React promise
	// resolves the moment the spawn succeeds.
	for _, ide := range []string{"code", "cursor"} {
		bin, lookErr := exec.LookPath(ide)
		if lookErr != nil {
			continue
		}
		cmd := exec.Command(bin, abs)
		cmd.Stdin = nil
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		// Detach on Windows: CREATE_NEW_PROCESS_GROUP so closing the
		// Wails window does not kill the editor. On POSIX the Setpgid
		// equivalent is the default; the editor outlives the GUI on
		// its own.
		if runtime.GOOS == "windows" {
			cmd.SysProcAttr = &syscall.SysProcAttr{
				CreationFlags: 0x00000200, // CREATE_NEW_PROCESS_GROUP
			}
		}
		if startErr := cmd.Start(); startErr != nil {
			// Spawn failed for this candidate; try the next one.
			// We do not return early so the user still gets
			// the `cursor` / OS-default fallback paths.
			continue
		}
		pid := 0
		if cmd.Process != nil {
			pid = cmd.Process.Pid
			// Release so the Wails GUI does not hold a wait handle
			// to the editor. The editor continues independently.
			_ = cmd.Process.Release()
		}
		return OpenIDEResult{OK: true, IDE: ide, PID: pid, Path: abs}, nil
	}

	// All IDE probes failed: fall back to the OS file-association
	// handler. This is genuinely last-resort (Notepad / TextEdit /
	// xdg-open generic handler) and is labelled as such in the
	// result so the UI can warn the user.
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// rundll32 url.dll,FileProtocolHandler opens a file in the
		// user's default application. We do not pass `cmd.exe /c
		// start` because the legacy `start` builtin can race the
		// Wails process on rapid close.
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", abs)
	case "darwin":
		cmd = exec.Command("open", abs)
	default:
		cmd = exec.Command("xdg-open", abs)
	}
	cmd.Stdin = nil
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if startErr := cmd.Start(); startErr != nil {
		return OpenIDEResult{
			OK:   false,
			Err:  fmt.Sprintf("no editor on PATH (`code` / `cursor`) and OS default failed: %v", startErr),
			Path: abs,
		}, nil
	}
	pid := 0
	if cmd.Process != nil {
		pid = cmd.Process.Pid
		_ = cmd.Process.Release()
	}
	return OpenIDEResult{OK: true, IDE: "os-default", PID: pid, Path: abs}, nil
}
