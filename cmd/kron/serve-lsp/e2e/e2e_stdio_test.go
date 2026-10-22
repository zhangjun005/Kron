package e2e

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestServeLSPStdio(t *testing.T) {
	root, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}").Output()
	require.NoError(t, err)
	moduleRoot := strings.TrimSpace(string(root))
	bin := filepath.Join(t.TempDir(), "kron-e2e")
	if os.PathSeparator == '\\' {
		bin += ".exe"
	}
	build := exec.Command("go", "build", "-o", bin, "./cmd/kron")
	build.Dir = moduleRoot
	output, err := build.CombinedOutput()
	require.NoError(t, err, string(output))

	project := t.TempDir()
	intent := "<!-- kron:frontmatter -->\ncreated_by: '@test'\nupdated_at: 2026-10-03T10:00:00Z\nstatus: active\n<!-- /kron:frontmatter -->\n\n# Auth\n\nAuthentication intent.\n"
	require.NoError(t, os.MkdirAll(filepath.Join(project, ".kron", "intents"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(project, ".kron", "intents", "auth.md"), []byte(intent), 0o644))
	source := filepath.Join(project, "main.go")
	require.NoError(t, os.WriteFile(source, []byte("package main\n// @kron:intent auth\nfunc main() {}\n"), 0o644))

	cmd := exec.Command(bin, "serve-lsp", "-CWD", project)
	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	cmd.Stderr = io.Discard
	require.NoError(t, cmd.Start())
	defer func() {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()

	r := bufio.NewReader(stdout)
	w := bufio.NewWriter(stdin)
	send := func(v any) {
		t.Helper()
		body, marshalErr := json.Marshal(v)
		require.NoError(t, marshalErr)
		_, writeErr := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n%s", len(body), body)
		require.NoError(t, writeErr)
		require.NoError(t, w.Flush())
	}
	recv := func() map[string]json.RawMessage {
		t.Helper()
		_ = r // avoid a test hang without bounding the process below
		var length int
		for {
			line, readErr := r.ReadString('\n')
			require.NoError(t, readErr)
			if line == "\r\n" {
				break
			}
			if _, scanErr := fmt.Sscanf(line, "Content-Length: %d", &length); scanErr != nil {
				t.Fatalf("unexpected LSP header %q", line)
			}
		}
		body := make([]byte, length)
		_, readErr := io.ReadFull(r, body)
		require.NoError(t, readErr)
		var response map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(body, &response))
		return response
	}

	send(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"capabilities": map[string]any{}}})
	resp := recv()
	require.JSONEq(t, "1", string(resp["id"]))
	require.Contains(t, string(resp["result"]), "hoverProvider")
	send(map[string]any{"jsonrpc": "2.0", "method": "initialized", "params": map[string]any{}})
	uri := "file:///" + filepath.ToSlash(strings.TrimPrefix(source, string(filepath.VolumeName(source))))
	if filepath.VolumeName(source) != "" {
		uri = "file:///" + filepath.ToSlash(source)
	}
	send(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "textDocument/hover", "params": map[string]any{"textDocument": map[string]string{"uri": uri}, "position": map[string]int{"line": 1, "character": 0}}})
	hover := recv()
	require.Contains(t, string(hover["result"]), "auth")
	send(map[string]any{"jsonrpc": "2.0", "id": 3, "method": "textDocument/definition", "params": map[string]any{"textDocument": map[string]string{"uri": uri}, "position": map[string]int{"line": 1, "character": 0}}})
	definition := recv()
	require.Contains(t, string(definition["result"]), "auth.md")
	send(map[string]any{"jsonrpc": "2.0", "id": 4, "method": "shutdown"})
	require.JSONEq(t, "4", string(recv()["id"]))
	send(map[string]any{"jsonrpc": "2.0", "method": "exit"})
	require.NoError(t, stdin.Close())
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("serve-lsp did not exit after exit notification")
	}
}
