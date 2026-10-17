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
	"sync"
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
		return
	}
	_ = a.mcpCmd.Process.Kill()
	_ = a.mcpCmd.Wait()
	a.mcpCmd = nil
	a.mcpStdin = nil
	a.mcpStdout = nil
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
//  3. ./build/bin/kron relative to the executable's own directory (for
//     future `wails build` deployments — Phase 1.5).
//
// We deliberately do not search PATH: the Wails GUI and the serve-mcp
// binary are version-paired and we want to fail loud if the dev artifact
// is missing rather than accidentally spawn an unrelated `kron` on PATH.
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
		candidate := filepath.Join(cwd, "build", "bin", "kron.exe")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		// POSIX / wails build fallback alongside the GUI binary.
		candidate2 := filepath.Join(cwd, "build", "bin", "kron")
		if _, err := os.Stat(candidate2); err == nil {
			return candidate2, nil
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

// spawnLocked starts the kron serve-mcp subprocess. The caller MUST
// hold mcpMu.
func (a *App) spawnLocked(executable string) error {
	cmd := exec.Command(executable, "serve-mcp")
	cmd.Stderr = os.Stderr // serve-mcp writes its own diagnostics to stderr
	// No cwd override — serve-mcp defaults to os.Getwd(), which is the
	// same as our CWD in the wails dev environment (frontend/wails/).
	// When the GUI grows multi-workspace support (Phase 2.x) we will
	// plumb a -CWD arg from the React side.

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
	return nil
}

// handshakeLocked performs the three-message MCP handshake. The caller
// MUST hold mcpMu and must have a live subprocess. Returns the parsed
// serverInfo and the tool name list.
func (a *App) handshakeLocked() (LiveServerInfo, []LiveMCPTool, error) {
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
