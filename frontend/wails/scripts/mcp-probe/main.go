// Command mcp-probe is a tiny MCP stdio client used by the release-shaped
// environment verifier. It spawns `kron serve-mcp` WITHOUT -CWD from an
// unrelated working directory and calls kron_list, so a non-empty result
// proves the server resolved the project from projects.json next to kron.exe
// (store.ProjectsJSONPath) rather than from the process working directory.
//
// It is deliberately hand-rolled (newline-delimited JSON-RPC, no SDK) so the
// probe does not depend on any package beyond the standard library, and it
// writes raw bytes (no BOM) — PowerShell's StreamWriter prefixes stdin with
// U+FEFF, which makes Go's JSON decoder reject the first frame.
//
// Usage:
//
//	go build -o mcp-probe.exe ./scripts/mcp-probe
//	./mcp-probe.exe -kron <app>/kron.exe -dir <non-project-dir>
//
// Exit codes: 0 = list returned (intents printed), 2 = server failed before
// responding (startup failure or EOF), 3 = listed but intent count != want.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	kronPath := flag.String("kron", "", "path to the kron CLI executable")
	workDir := flag.String("dir", "", "working directory for the server process; must not be a Kron project")
	wantIntents := flag.Int("want", 8, "expected number of intents in kron_list")
	flag.Parse()
	if *kronPath == "" || *workDir == "" {
		fmt.Fprintln(os.Stderr, "mcp-probe: -kron and -dir are required")
		os.Exit(2)
	}
	if err := run(*kronPath, *workDir, *wantIntents); err != nil {
		fmt.Fprintln(os.Stderr, "mcp-probe:", err)
		os.Exit(2)
	}
}

func run(kronPath, workDir string, wantIntents int) error {
	absKron, err := filepath.Abs(kronPath)
	if err != nil {
		return fmt.Errorf("resolve kron path: %w", err)
	}
	cmd := exec.Command(absKron, "serve-mcp")
	cmd.Dir = workDir
	// The child's stderr is inherited intentionally: on the negative control
	// the server explains why it refused to start.
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start serve-mcp: %w", err)
	}
	defer func() {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	writer := bufio.NewWriter(stdin)
	defer func() { _ = writer.Flush() }()
	reader := bufio.NewReader(stdout)

	send := func(payload map[string]any) error {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		if _, err := writer.Write(append(data, '\n')); err != nil {
			return err
		}
		return writer.Flush()
	}

	readFrame := func(wantID int) (map[string]any, error) {
		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				return nil, fmt.Errorf("read frame: %w", err)
			}
			var frame map[string]any
			if err := json.Unmarshal(line, &frame); err != nil {
				continue // tolerate stray non-JSON log lines
			}
			if id, ok := frame["id"].(float64); ok && int(id) == wantID {
				return frame, nil
			}
		}
	}

	if err := send(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "kron-mcp-probe", "version": "0.0.0"},
		},
	}); err != nil {
		return err
	}
	if _, err := readFrame(1); err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	if err := send(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"}); err != nil {
		return err
	}
	if err := send(map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params":  map[string]any{"name": "kron_list", "arguments": map[string]any{}},
	}); err != nil {
		return err
	}
	frame, err := readFrame(2)
	if err != nil {
		return fmt.Errorf("kron_list: %w", err)
	}
	result, _ := frame["result"].(map[string]any)
	if result == nil {
		return fmt.Errorf("kron_list returned no result: %v", frame)
	}
	if isError, _ := result["isError"].(bool); isError {
		return fmt.Errorf("kron_list returned isError=true")
	}
	structured, _ := result["structuredContent"].(map[string]any)
	intents, _ := structured["intents"].([]any)
	if len(intents) != wantIntents {
		fmt.Printf("intents=%d\n", len(intents))
		os.Exit(3)
	}
	fmt.Printf("intents=%d\n", len(intents))
	return nil
}
