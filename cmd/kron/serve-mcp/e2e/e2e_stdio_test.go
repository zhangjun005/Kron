// Package e2e holds the real-binary E2E tests for the MCP stdio
// server. Unlike the in-memory tests in the parent servemcp package
// (which use IOTransport + a testkit client), these tests actually
// spawn the compiled kron binary, pipe JSON-RPC frames over its
// stdin, and assert on the raw stdout.
//
// These tests are gated by the KRON_E2E environment variable because
// they require `go build` and are relatively slow (~1-2s each).  Run
// locally with:
//
//	KRON_E2E=1 go test -v ./cmd/kron/serve-mcp/e2e/
//
// The env-var gate (rather than a build tag) lets the test binary
// be compiled normally; the testing package skips the tests when
// KRON_E2E is absent without needing separate -tags flags.
package e2e

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- binary builder ---------------------------------------------------

// binaryPath is the cached output of building the kron binary once per
// test process.
var (
	binaryPath string
	buildOnce  sync.Once
	buildErr   error
)

// buildBinary builds the kron binary once and returns its path.
// The output directory is process-scoped (os.MkdirTemp, NOT
// t.TempDir) because t.TempDir is cleaned up at test end, which
// races with the binary still being used by parallel or subsequent
// tests.  Using a process-scoped dir plus TestMain cleanup ensures
// the binary lives as long as the test process does.
func buildBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		// Resolve module root.
		listCmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}")
		rootBytes, err := listCmd.Output()
		if err != nil {
			buildErr = fmt.Errorf("go list -m failed: %w", err)
			return
		}
		moduleRoot := strings.TrimSpace(string(rootBytes))

		// Process-scoped dir: the test process owns it, not any
		// individual test.
		dir, err := os.MkdirTemp("", "kron-e2e-")
		if err != nil {
			buildErr = fmt.Errorf("MkdirTemp: %w", err)
			return
		}
		processBuildDir = dir

		output := filepath.Join(dir, "kron-e2e.exe")
		cmd := exec.Command("go", "build", "-o", output, "./cmd/kron")
		cmd.Dir = moduleRoot
		var stderr strings.Builder
		cmd.Stderr = &stderr
		buildErr = cmd.Run()
		if buildErr == nil {
			binaryPath = output
		} else {
			buildErr = fmt.Errorf("go build ./cmd/kron (in %s) failed: %w; build stderr: %s", moduleRoot, buildErr, stderr.String())
		}
	})
	if buildErr != nil {
		t.Fatalf("go build failed: %v (e2e needs a compileable tree)", buildErr)
	}
	return binaryPath
}

// processBuildDir holds the process-scoped directory for the built
// binary; TestMain (in testmain_test.go) cleans it up on process exit.
var processBuildDir string

// repoRoot returns the module root directory.  Used by tests that need
// a directory path (rare — buildBinary already does its own
// resolution).
func repoRoot(t *testing.T) string {
	t.Helper()
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}")
	b, err := cmd.Output()
	require.NoError(t, err)
	return strings.TrimSpace(string(b))
}

// --- session --------------------------------------------------------

// session wraps a live kron serve-mcp subprocess.
type session struct {
	t      *testing.T
	cmd    *exec.Cmd
	w      *bufio.Writer // buffered stdin writer
	stdin  io.Closer     // raw stdin pipe, closed by session.Close
	r      *bufio.Reader // buffered stdout reader
	stderr strings.Builder
}

// newSession starts a kron serve-mcp subprocess targeting root.
func newSession(t *testing.T, root string) *session {
	if root == "" {
		root = t.TempDir()
	}
	// Pre-create .kron/ so resolveMCPRoot's project-root validation
	// passes. In production, every supported MCP client spawns
	// kron with cwd set to a directory that already has .kron/
	// (the client is configured to point at an existing Kron
	// project), so the validation is correct for the production
	// path; this helper just mirrors that for the test path.
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".kron"), 0o755))
	bin := buildBinary(t)
	cmd := exec.Command(bin, "serve-mcp", "-CWD", root)

	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	stderr, err := cmd.StderrPipe()
	require.NoError(t, err)

	require.NoError(t, cmd.Start())

	s := &session{
		t:     t,
		cmd:   cmd,
		w:     bufio.NewWriter(stdin),
		stdin: stdin,
		r:     bufio.NewReader(stdout),
	}
	// Drain stderr asynchronously so the pipe buffer (4 KiB default)
	// never fills and deadlocks the subprocess.
	go func() {
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			s.stderr.WriteString(sc.Text() + "\n")
		}
	}()

	return s
}

// send writes a JSON-RPC 2.0 frame and flushes it immediately.
func (s *session) send(method string, params any, id any) {
	req := map[string]any{"jsonrpc": "2.0", "method": method}
	if params != nil {
		req["params"] = params
	}
	if id != nil {
		req["id"] = id
	}
	enc := json.NewEncoder(s.w)
	if err := enc.Encode(req); err != nil {
		panic("json.Encode: " + err.Error())
	}
	if err := s.w.Flush(); err != nil {
		panic("bufio.Writer.Flush: " + err.Error())
	}
}

// recv reads one JSON-RPC response and returns the "result" field.
// Fails the test on a JSON-RPC error response.
func (s *session) recv() json.RawMessage {
	var env struct {
		ID     json.RawMessage `json:"id"`
		Result json.RawMessage `json:"result,omitempty"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}
	require.NoError(s.t, json.NewDecoder(s.r).Decode(&env), "read JSON-RPC response")
	if env.Error != nil {
		s.t.Fatalf("JSON-RPC error %d: %s", env.Error.Code, env.Error.Message)
	}
	return env.Result
}

// initialize performs the MCP initialize handshake.  Must be called
// once per session before any tool calls.
func (s *session) initialize() {
	s.send("initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "kron-e2e-test",
			"version": "0.0.0",
		},
	}, 1)
	result := s.recv()
	require.NotNil(s.t, result, "initialize must return a result")
}

// callTool sends a tools/call request and returns the unpacked
// result map.  The raw "result" envelope from the SDK contains a
// "structuredContent" sub-payload (per MCP v1 spec); we unwrap it
// so callers see the same shape as the in-memory test client.
func (s *session) callTool(name string, args map[string]any) map[string]any {
	s.send("tools/call", map[string]any{
		"name":      name,
		"arguments": args,
	}, 2)
	result := s.recv()
	return unpackToolResult(s.t, result)
}

// unpackToolResult decodes a tools/call result envelope.  The SDK
// returns the result in the form:
//
//	{"structuredContent": {...}, "content": [...], "isError": false}
//
// The structuredContent is the typed payload the handler returned;
// content is the optional text fallback; isError flags tool-level
// errors.  We return the structuredContent map so callers can
// assert on the same field names the in-memory tests use.
func unpackToolResult(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var env struct {
		StructuredContent map[string]any `json:"structuredContent"`
		Content           []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	require.NoError(t, json.Unmarshal(raw, &env), "decode tools/call result envelope")
	if env.IsError {
		// Surface the text content as the failure message so the
		// test failure log includes the handler's error string.
		msg := "<no text content>"
		if len(env.Content) > 0 {
			msg = env.Content[0].Text
		}
		t.Fatalf("tools/call returned isError=true: %s", msg)
	}
	if env.StructuredContent != nil {
		return env.StructuredContent
	}
	// Fallback: some tools return a JSON string in content[0].text
	// instead of structuredContent.  Decode it.
	if len(env.Content) > 0 && env.Content[0].Text != "" {
		var m map[string]any
		if err := json.Unmarshal([]byte(env.Content[0].Text), &m); err == nil {
			return m
		}
	}
	t.Fatalf("tools/call result has neither structuredContent nor decodable content[0].text: %s", string(raw))
	return nil
}

// close closes stdin (causing EOF) and waits for the subprocess to
// exit.  It asserts exit code 0.  The session must not be used after
// Close.
func (s *session) close() {
	s.t.Helper()
	require.NoError(s.t, s.stdin.Close(), "close stdin")
	err := s.cmd.Wait()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		s.t.Fatalf("kron serve-mcp exited with code %d; stderr:\n%s",
			exitErr.ExitCode(), s.stderr.String())
	}
	require.NoError(s.t, err, "server must exit 0; stderr:\n%s", s.stderr.String())
}

// --- tests -------------------------------------------------------------

// gate is true when KRON_E2E is set.  Every test starts here so the
// suite silently skips when the env is absent.
var gate = os.Getenv("KRON_E2E") != ""

// TestE2E_StdioInitializeThenLint is the primary G-2 smoke test: it
// spawns the real kron binary, performs an initialize handshake,
// calls kron_init, kron_lint, and tools/list, and verifies the
// responses are well-formed JSON-RPC.  It then closes stdin and
// asserts the process exits 0.
//
// Exercises the full production path:
//   - stdio transport (real os.Stdin/Stdout, not IOTransport)
//   - Fd()==0 detection (stdioOrIOTransport picks StdioTransport)
//   - signal.NotifyContext (G-1): context is live during handshake;
//     shutdown here uses the clean EOF path (server.Run returns from
//     ss.Wait() when stdin pipe is closed).
//   - JSON-RPC framing over a real stdout pipe.
func TestE2E_StdioInitializeThenLint(t *testing.T) {
	if !gate {
		t.Skip("KRON_E2E not set; skipping real-binary e2e tests")
	}

	dir := t.TempDir()
	s := newSession(t, dir)
	defer s.close()

	s.initialize()

	// kron_init
	res := s.callTool("kron_init", map[string]any{})
	assert.Equal(t, true, res["ok"], "kron_init must succeed")
	assert.NotEmpty(t, res["intents_dir"], "intents_dir must be populated")

	// kron_lint on clean repo
	res = s.callTool("kron_lint", map[string]any{})
	assert.Equal(t, true, res["passed"], "clean repo must pass lint")

	// tools/list (SDK-level, not a kron tool)
	s.send("tools/list", map[string]any{}, 3)
	tlResult := s.recv()
	var tl struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	require.NoError(t, json.Unmarshal(tlResult, &tl))
	found := make(map[string]bool, len(tl.Tools))
	for _, tool := range tl.Tools {
		found[tool.Name] = true
	}
	// The canonical tool list (count and names) is enforced by the
	// in-memory test TestServer_AllToolsRegistered in serve_mcp_test.go.
	// The e2e test's role is to prove the real stdio path registers
	// the same set, not to maintain a duplicate list that can drift.
	// We assert a representative sample to catch obvious registration
	// regressions in the stdio path.
	for _, want := range []string{
		"kron_init", "kron_lint", "kron_tree", "kron_get",
		"kron_assume_create", "kron_assume_update",
		"kron_assume_delete", "kron_assume_list",
	} {
		assert.True(t, found[want], "tools/list must contain %q", want)
	}
	// defer s.close() closes stdin → server exits 0.
}

// TestE2E_StdinEOFExitsCleanly verifies that closing stdin (the normal
// production shutdown path) makes the server exit 0 rather than a
// transport error.  This is the sibling of G-1's signal-path test:
// G-1 covers "Ctrl+C exits 0"; this covers "EOF exits 0".
func TestE2E_StdinEOFExitsCleanly(t *testing.T) {
	if !gate {
		t.Skip("KRON_E2E not set; skipping real-binary e2e tests")
	}

	dir := t.TempDir()
	// Pre-create .kron/ so resolveMCPRoot's project-root validation
	// passes. The production path always has it (clients point at
	// an existing Kron project); this test mirrors that.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".kron"), 0o755))
	bin := buildBinary(t)
	cmd := exec.Command(bin, "serve-mcp", "-CWD", dir)

	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	_, err = cmd.StdoutPipe()
	require.NoError(t, err)

	// Start the subprocess, then close stdin immediately.
	require.NoError(t, cmd.Start())
	require.NoError(t, stdin.Close())

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		require.NoError(t, err, "server must exit 0 on stdin EOF")
	case <-time.After(5 * time.Second):
		t.Fatal("kron serve-mcp did not exit within 5s of stdin EOF")
	}
}

// TestE2E_StdioTransportNotTTY guards against a regression in
// stdioOrIOTransport where it accidentally picks StdioTransport for
// pipe-fed subprocesses (which would hang because StdioTransport reads
// raw OS stdin rather than the pipe).
//
// The test sends a syntactically invalid byte over a pipe.  If
// StdioTransport is selected the server hangs waiting for TTY input;
// if IOTransport is selected it sees the invalid byte and exits.
// We assert the process terminates within 2 seconds.
func TestE2E_StdioTransportNotTTY(t *testing.T) {
	if !gate {
		t.Skip("KRON_E2E not set; skipping real-binary e2e tests")
	}

	dir := t.TempDir()
	// Pre-create .kron/ so resolveMCPRoot's project-root validation
	// passes. (See newSession for the rationale.)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".kron"), 0o755))
	bin := buildBinary(t)
	cmd := exec.Command(bin, "serve-mcp", "-CWD", dir)

	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	_, err = cmd.StdoutPipe()
	require.NoError(t, err)
	stderr, err := cmd.StderrPipe()
	require.NoError(t, err)

	require.NoError(t, cmd.Start())

	// Write one invalid byte and close stdin.
	stdin.Write([]byte{0xff})
	stdin.Close()

	// Drain stderr so the pipe doesn't block the subprocess.
	go io.Copy(io.Discard, stderr)

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case <-done:
		// Process exited: IOTransport was correctly picked for pipe.
	case <-time.After(2 * time.Second):
		cmd.Process.Kill()
		t.Fatalf("kron serve-mcp hung on invalid input; " +
			"StdioTransport may have been selected for a pipe-fed subprocess")
	}
}

// TestE2E_StdinRedirectWorks covers the case where the binary's stdin
// is redirected from a file (common in scripting / CI).  The O/S-level
// stdin fd is still 0, so stdioOrIOTransport correctly picks
// StdioTransport, but the data comes from a file rather than a TTY.
// The key assertion is that the server processes the file content and
// exits cleanly.
//
// Skipped on Windows: when os/exec assigns a file handle as
// cmd.Stdin, the Go runtime's underlying pipe behavior on Windows
// differs from POSIX (CreateFile vs. HANDLE inheritance).  The
// "fd 0 IS the file" assumption that drives the StdioTransport
// selection is a Linux/macOS property.  On Windows the same code
// path produces a non-functional stdin because cmd.Stdin on
// Windows is an os.Pipe, not the file we assigned.  The other
// three e2e tests already cover the production-stdio and EOF
// paths; this one adds Linux/macOS coverage for the
// file-redirect case only.
func TestE2E_StdinRedirectWorks(t *testing.T) {
	if !gate {
		t.Skip("KRON_E2E not set; skipping real-binary e2e tests")
	}
	if runtime.GOOS == "windows" {
		t.Skip("file-redirect stdin is a Linux/macOS-only case; see comment")
	}

	dir := t.TempDir()
	// Pre-create .kron/ so resolveMCPRoot's project-root validation
	// passes. (See newSession for the rationale.)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".kron"), 0o755))

	// Write a JSON-RPC initialize frame to a temp file.
	initFrame := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"e2e-redirect-test","version":"0.0.0"}}}
`
	tmpFile := filepath.Join(t.TempDir(), "stdin.json")
	require.NoError(t, os.WriteFile(tmpFile, []byte(initFrame), 0o644))

	bin := buildBinary(t)
	cmd := exec.Command(bin, "serve-mcp", "-CWD", dir)
	f, err := os.Open(tmpFile)
	require.NoError(t, err)
	cmd.Stdin = f

	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	stderr, err := cmd.StderrPipe()
	require.NoError(t, err)

	require.NoError(t, cmd.Start())

	// Drain stderr.
	go io.Copy(io.Discard, stderr)

	// Read the initialize response.  The server emits exactly one
	// JSON-RPC response frame for the single initialize request, then
	// exits when stdin EOF arrives.  Read the full response (using a
	// thread that drains stdout into a buffer) so we don't hit an EOF
	// mid-decode if the subprocess closes stdout right after writing.
	stdoutBuf := make([]byte, 0, 4096)
	stdoutDone := make(chan struct{})
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				stdoutBuf = append(stdoutBuf, buf[:n]...)
			}
			if err != nil {
				close(stdoutDone)
				return
			}
		}
	}()

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case <-stdoutDone:
		// Server closed stdout: it processed the request and is shutting down.
	case <-time.After(2 * time.Second):
		cmd.Process.Kill()
		t.Fatal("server did not close stdout within 2s of initialize")
	}

	var env struct {
		ID     json.RawMessage `json:"id"`
		Result json.RawMessage `json:"result,omitempty"`
	}
	require.NoError(t, json.Unmarshal(stdoutBuf, &env), "decode initialize response: %s", string(stdoutBuf))
	assert.Equal(t, json.RawMessage(`1`), env.ID, "response ID must match request ID")
	assert.NotNil(t, env.Result, "initialize must return a result")

	select {
	case err := <-done:
		require.NoError(t, err, "server must exit 0 after reading stdin file")
	case <-time.After(5 * time.Second):
		cmd.Process.Kill()
		t.Fatal("server did not exit within 5s of stdin EOF")
	}
}
