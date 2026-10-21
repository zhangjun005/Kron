// raw-probe: drive kron serve-mcp over stdio using only
// bufio.Scanner (line-delimited JSON-RPC). Print every response
// we get, in order. Exit 0 if we see 11 responses, else 1.
//
// Usage: go run ./raw-probe
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

const (
	kronExe     = `E:\works\Kron\kron-mcp-test\install\kron.exe`
	projectRoot = `E:\works\Kron\kron-mcp-test\project`
	projEnv     = `E:\works\Kron\kron-mcp-test\install\projects.json`
)

type rpcReq struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResp struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   any             `json:"error,omitempty"`
}

func raw(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, kronExe, "serve-mcp", "-CWD", projectRoot)
	cmd.Dir = projectRoot
	cmd.Env = append(os.Environ(), "KRON_PROJECTS_JSON_PATH="+projEnv)
	stdin, err := cmd.StdinPipe()
	must(err)
	stdout, err := cmd.StdoutPipe()
	must(err)
	stderr, err := cmd.StderrPipe()
	must(err)
	must(cmd.Start())

	// Drain stderr async
	var errBuf []byte
	var errMu sync.Mutex
	go func() {
		b, _ := io.ReadAll(stderr)
		errMu.Lock()
		errBuf = b
		errMu.Unlock()
	}()

	// Line-delimited writes
	w := bufio.NewWriter(stdin)

	// 11 requests
	requests := []rpcReq{
		{JSONRPC: "2.0", ID: 1, Method: "initialize", Params: raw(map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]string{"name": "go-raw-probe", "version": "0"},
		})},
		{JSONRPC: "2.0", Method: "notifications/initialized"},
		{JSONRPC: "2.0", ID: 2, Method: "prompts/list", Params: raw(map[string]any{})},
		{JSONRPC: "2.0", ID: 3, Method: "resources/list", Params: raw(map[string]any{})},
		{JSONRPC: "2.0", ID: 4, Method: "resources/templates/list", Params: raw(map[string]any{})},
		{JSONRPC: "2.0", ID: 5, Method: "prompts/get", Params: raw(map[string]any{
			"name":      "kron-drift-check",
			"arguments": map[string]any{"days_threshold": 30},
		})},
		{JSONRPC: "2.0", ID: 6, Method: "prompts/get", Params: raw(map[string]any{
			"name":      "kron-implement",
			"arguments": map[string]any{"feature": "add idempotency key to charge"},
		})},
		{JSONRPC: "2.0", ID: 7, Method: "prompts/get", Params: raw(map[string]any{
			"name":      "kron-review",
			"arguments": map[string]any{"status": "active"},
		})},
		{JSONRPC: "2.0", ID: 8, Method: "resources/read", Params: raw(map[string]any{
			"uri": "kron://intents/auth/jwt",
		})},
		{JSONRPC: "2.0", ID: 9, Method: "resources/read", Params: raw(map[string]any{
			"uri": "kron://intents/payments/charge",
		})},
		{JSONRPC: "2.0", ID: 10, Method: "resources/read", Params: raw(map[string]any{
			"uri": "kron://intents/does/not/exist",
		})},
	}
	for _, r := range requests {
		// Use json.Encoder so we get a trailing newline (line-delimited)
		if err := json.NewEncoder(w).Encode(r); err != nil {
			fmt.Fprintln(os.Stderr, "encode:", err)
			os.Exit(1)
		}
	}
	must(w.Flush())

	// Read responses (one per request, in id order)
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	got := 0
	for got < 10 { // 10 of 11 are request/response; notifications/initialized returns nothing
		if !sc.Scan() {
			fmt.Fprintln(os.Stderr, "scan done; err:", sc.Err())
			break
		}
		var r rpcResp
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			fmt.Fprintf(os.Stderr, "decode err on line %q: %v\n", sc.Text(), err)
			continue
		}
		got++
		bb, _ := json.MarshalIndent(r, "", "  ")
		fmt.Printf("=== response id=%d ===\n%s\n\n", r.ID, string(bb))
	}

	// Close stdin to let kron shut down
	_ = stdin.Close()
	_ = cmd.Wait()

	errMu.Lock()
	stderrOut := errBuf
	errMu.Unlock()
	if len(stderrOut) > 0 {
		fmt.Println("=== stderr ===")
		fmt.Println(string(stderrOut))
	}
	fmt.Printf("=== exit: %d, responses: %d/10 ===\n", cmd.ProcessState.ExitCode(), got)
}
