// raw-probe-b1b2b3: drive kron serve-mcp over stdio using only
// bufio.Scanner (line-delimited JSON-RPC). Mimic what Cursor would send
// for kron_list / kron_get / kron_stale, with B1/B2/B3 type-encoding
// bugs in place. The receiving-side coerce middleware should turn the
// bad encodings into legal ones, so all calls return success.
//
// Usage: go run ./raw-probe-b1b2b3
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

	// Sequence:
	//   1) initialize handshake
	//   2) notifications/initialized
	//   3) tools/list (discover)
	//   4) kron_list  (control: well-typed, no args)
	//   5) kron_get slug=auth/jwt include_relations=true  (B1: bool as "true")
	//   6) kron_stale days_threshold=0                   (B3: int 0 as string "0")
	//   7) kron_stale days_threshold=30                  (control: still pass after coerce?)
	requests := []rpcReq{
		{JSONRPC: "2.0", ID: 1, Method: "initialize", Params: raw(map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]string{"name": "go-raw-probe-b1b2b3", "version": "0"},
		})},
		{JSONRPC: "2.0", Method: "notifications/initialized"},
		{JSONRPC: "2.0", ID: 2, Method: "tools/list", Params: raw(map[string]any{})},
		// kron_list: no arguments allowed by schema
		{JSONRPC: "2.0", ID: 3, Method: "tools/call", Params: raw(map[string]any{
			"name":      "kron_list",
			"arguments": map[string]any{},
		})},
		// kron_get with B1 (bool as string "true")
		{JSONRPC: "2.0", ID: 4, Method: "tools/call", Params: raw(map[string]any{
			"name":      "kron_get",
			"arguments": map[string]any{"slug": "auth/jwt", "include_relations": "true"},
		})},
		// kron_stale with B3 (int 0 as string "0")
		{JSONRPC: "2.0", ID: 5, Method: "tools/call", Params: raw(map[string]any{
			"name":      "kron_stale",
			"arguments": map[string]any{"days_threshold": "0"},
		})},
		// kron_stale with B3 (int 30 as string "30")
		{JSONRPC: "2.0", ID: 6, Method: "tools/call", Params: raw(map[string]any{
			"name":      "kron_stale",
			"arguments": map[string]any{"days_threshold": "30"},
		})},
	}
	for _, r := range requests {
		if err := json.NewEncoder(w).Encode(r); err != nil {
			fmt.Fprintln(os.Stderr, "encode:", err)
			os.Exit(1)
		}
	}
	must(w.Flush())

	// Read responses (notifications yield nothing; only request/response pairs appear)
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	got := 0
	want := 6
	for got < want {
		if !sc.Scan() {
			fmt.Fprintln(os.Stderr, "scan done; err:", sc.Err())
			break
		}
		lineBytes := make([]byte, len(sc.Bytes()))
		copy(lineBytes, sc.Bytes())
		var r rpcResp
		if err := json.Unmarshal(lineBytes, &r); err != nil {
			fmt.Fprintf(os.Stderr, "decode err on line %q: %v\n", string(lineBytes), err)
			continue
		}
		got++

		errLabel := "OK"
		if r.Error != nil || len(r.Result) == 0 {
			errLabel = "FAIL"
		}

		// Print result (truncated for large payloads)
		short := r.Result
		suffix := ""
		if len(short) > 300 {
			short = short[:300]
			suffix = " ...(truncated)"
		}
		fmt.Printf("=== id=%d %s method=%s ===\n  %s%s\n\n",
			r.ID, errLabel, methodFor(r.ID), string(short), suffix)

		// For kron_list: parse text content and count intents.
		// The server sends content[0].text as a JSON string value,
		// double-encoded: `"{\"intents\":[...]}"`.
		// Step 1: unmarshal r.Result → wrapper with raw text bytes.
		// Step 2: unmarshal raw bytes as string → gives `{"intents":[...]}`.
		// Step 3: unmarshal that string → Intents slice.
		if r.ID == 3 && r.Error == nil && len(r.Result) > 0 {
			var wrapper struct {
				Content []struct {
					Text json.RawMessage `json:"text"` // raw `"{\"intents\":[...]}"` bytes
				} `json:"content"`
			}
			if err2 := json.Unmarshal(r.Result, &wrapper); err2 == nil && len(wrapper.Content) > 0 {
				var textStr string
				if err3 := json.Unmarshal(wrapper.Content[0].Text, &textStr); err3 == nil {
					var body struct {
						Intents []map[string]any `json:"intents"`
					}
					if err4 := json.Unmarshal([]byte(textStr), &body); err4 == nil {
						fmt.Printf("    >>> kron_list returned %d intents\n\n", len(body.Intents))
					}
				}
			}
		}
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
	fmt.Printf("=== exit: %d, responses: %d/%d ===\n", cmd.ProcessState.ExitCode(), got, want)
}

func methodFor(id int) string {
	switch id {
	case 1:
		return "initialize"
	case 2:
		return "tools/list"
	case 3:
		return "kron_list (control)"
	case 4:
		return "kron_get (B1: include_relations=\"true\")"
	case 5:
		return "kron_stale (B3: days_threshold=\"0\")"
	case 6:
		return "kron_stale (B3: days_threshold=\"30\")"
	}
	return "?"
}
