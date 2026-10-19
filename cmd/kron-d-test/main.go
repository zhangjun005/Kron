// D test driver: launches `kron serve-lsp -CWD <root>` as a child
// process, sends a sequence of LSP frames over stdio (initialize →
// initialized → hover → shutdown → exit), and prints every server
// response. Lets a human eyeball what serve-lsp actually returns on
// real Kron repos without installing a VSCode extension.
//
// Run from the Kron repo root: `go run ./cmd/kron-d-test .d-test`.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: kron-d-test <repo-root>")
		os.Exit(2)
	}
	root, err := filepath.Abs(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "abs:", err)
		os.Exit(2)
	}
	if _, err := os.Stat(root); err != nil {
		fmt.Fprintln(os.Stderr, "stat:", err)
		os.Exit(2)
	}

	// Resolve the `kron` binary: prefer $KRON_BIN if set, then a
	// sibling `kron(.exe)` next to our own executable (where `go run`
	// normally lands it), then fall back to whatever $PATH offers.
	kronBin := resolveKronBin()
	fmt.Fprintf(os.Stderr, "[driver] using kron binary: %s\n", kronBin)

	cmd := exec.Command(kronBin, "serve-lsp", "-CWD", root)
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, "stdin pipe:", err)
		os.Exit(2)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, "stdout pipe:", err)
		os.Exit(2)
	}
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "start:", err)
		os.Exit(2)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	// refresh.go: line 3 (0-indexed 2) is the @kron:intent line.
	refreshPath := filepath.Join(root, "internal", "auth", "refresh.go")

	frames := []string{
		// 1. initialize
		mustJSON(map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params": map[string]any{
				"processId": os.Getpid(),
				"rootUri":   toFileURI(root),
				"capabilities": map[string]any{
					"textDocument": map[string]any{
						"hover": map[string]any{
							"contentFormat": []string{"markdown", "plaintext"},
						},
					},
				},
			},
		}),
		// 2. initialized (notification)
		mustJSON(map[string]any{
			"jsonrpc": "2.0",
			"method":  "initialized",
			"params":  map[string]any{},
		}),
		// 3. didOpen (so the server knows the doc)
		mustJSON(map[string]any{
			"jsonrpc": "2.0",
			"method":  "textDocument/didOpen",
			"params": map[string]any{
				"textDocument": map[string]any{
					"uri":        toFileURI(refreshPath),
					"languageId": "go",
					"version":    1,
					"text":       readFile(refreshPath),
				},
			},
		}),
		// 4. hover on the @kron:intent line (line 2 = 0-indexed)
		mustJSON(map[string]any{
			"jsonrpc": "2.0",
			"id":      2,
			"method":  "textDocument/hover",
			"params": map[string]any{
				"textDocument": map[string]any{
					"uri": toFileURI(refreshPath),
				},
				"position": map[string]any{
					"line":      2,
					"character": 0,
				},
			},
		}),
		// 5. hover on a non-anchor line (line 0 = "package auth")
		mustJSON(map[string]any{
			"jsonrpc": "2.0",
			"id":      3,
			"method":  "textDocument/hover",
			"params": map[string]any{
				"textDocument": map[string]any{
					"uri": toFileURI(refreshPath),
				},
				"position": map[string]any{
					"line":      0,
					"character": 0,
				},
			},
		}),
		// 6. definition on the @kron:intent line
		mustJSON(map[string]any{
			"jsonrpc": "2.0",
			"id":      4,
			"method":  "textDocument/definition",
			"params": map[string]any{
				"textDocument": map[string]any{
					"uri": toFileURI(refreshPath),
				},
				"position": map[string]any{
					"line":      2,
					"character": 0,
				},
			},
		}),
		// 7. shutdown
		mustJSON(map[string]any{
			"jsonrpc": "2.0",
			"id":      5,
			"method":  "shutdown",
			"params":  nil,
		}),
		// (No "exit" frame on purpose: server calls os.Exit(0) on
		// the exit notification, which can race with stdout flush
		// and drop in-flight hover/definition responses. Closing
		// stdin instead lets the server notice the parent is gone
		// and shut down cleanly.)
	}

	// Feed frames in a goroutine so the reader can run in parallel.
	go func() {
		for _, f := range frames {
			header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(f))
			_, _ = io.WriteString(stdin, header)
			_, _ = io.WriteString(stdin, f)
		}
		// Hold stdin open briefly so the server can flush pending
		// responses before we close the pipe. Without this, the
		// server's stdout buffer can be torn down before we read
		// hover/definition replies and we see only initialize +
		// shutdown. We do NOT send the LSP "exit" notification;
		// closing stdin on the parent side is enough to let the
		// server's jsonrpc2 read loop notice EOF and return.
		time.Sleep(500 * time.Millisecond)
		_ = stdin.Close()
	}()

	// Give the server a moment to spin up its jsonrpc2 read loop
	// before we start hitting it with frames; otherwise the first
	// request can race against stdio pipe setup.
	time.Sleep(200 * time.Millisecond)

	// LSP frames are Content-Length: N\r\n\r\n<json bytes>. Parse the
	// framing manually so we can print each response.
	reader := bufio.NewReader(stdout)
	for {
		length, err := readContentLength(reader)
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Fprintln(os.Stderr, "read header:", err)
			break
		}
		body := make([]byte, length)
		if _, err := io.ReadFull(reader, body); err != nil {
			fmt.Fprintln(os.Stderr, "read body:", err)
			break
		}
		fmt.Printf("--- LSP response (id=%s, %d bytes) ---\n",
			extractID(string(body)), length)
		prettyPrintJSON(string(body))
	}
}

func readContentLength(r *bufio.Reader) (int, error) {
	var length int
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return 0, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			// End of headers.
			return length, nil
		}
		if strings.HasPrefix(line, "Content-Length:") {
			fmt.Sscanf(line, "Content-Length: %d", &length)
		}
	}
}

func extractID(body string) string {
	var v struct {
		ID json.RawMessage `json:"id"`
	}
	_ = json.Unmarshal([]byte(body), &v)
	return string(v.ID)
}

func prettyPrintJSON(body string) {
	var v any
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		fmt.Println(body)
		return
	}
	out, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(out))
}

func mustJSON(v map[string]any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func readFile(path string) string {
	// Tiny wait so the child process has its stdin/stdout pipes set up
	// before we block on the file system; this is cosmetic.
	time.Sleep(50 * time.Millisecond)
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

// resolveKronBin finds the kron binary to spawn. It checks, in order:
//  1. $KRON_BIN env var (explicit override for CI / unusual layouts)
//  2. ./kron or ./kron.exe next to the current executable — this is
//     where `go build` of cmd/kron lands by default, and where the
//     D driver script usually has it staged.
//  3. Plain "kron" / "kron.exe" on $PATH.
func resolveKronBin() string {
	if v := os.Getenv("KRON_BIN"); v != "" {
		return v
	}
	self, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(self)
		for _, name := range []string{"kron.exe", "kron"} {
			candidate := filepath.Join(dir, name)
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	}
	if runtime.GOOS == "windows" {
		return "kron.exe"
	}
	return "kron"
}

// toFileURI formats an absolute filesystem path as a `file://` URI.
// Every `file://` URI has THREE slashes: `file://` + a leading `/` +
// the absolute path. On Windows the path starts with a drive letter,
// so the URI looks like `file:///C:/path/...` (slash, slash, slash,
// drive). uri.File / uri.New() parse this back to the same absolute
// path that round-trips through FsPath on the LSP server side.
func toFileURI(absPath string) string {
	return "file:///" + filepath.ToSlash(absPath)
}
