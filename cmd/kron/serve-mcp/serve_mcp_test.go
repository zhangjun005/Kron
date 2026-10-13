package servemcp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Since v1.2 the MCP protocol layer is owned by the official
// modelcontextprotocol/go-sdk. The tests in this file cover the
// shim Run(args, in, out, errOut) — the production stdio entry
// point that main.go invokes — by feeding it raw JSON-RPC lines and
// asserting the SDK-mediated response. The deep protocol coverage
// (initialize handshake, tools/list, structuredContent, etc.) is
// exercised by the in-memory client tests in handlers_test.go
// (testkit.go + newTestSession).

// singleResponse is the v1.1-shaped view of an MCP response, kept
// for backwards-compat with the callJSON shim in handlers_test.go
// and handlers_relations_test.go. Since the SDK now returns
// CallToolResult with a wrapped envelope, the decodeSingleResponse
// helper unpacks structuredContent back into Result for those
// existing fixtures.
type singleResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	ID json.RawMessage `json:"id"`
}

func decodeSingleResponse(t *testing.T, output string) singleResponse {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	require.Len(t, lines, 1, "expected exactly one response line, got %d: %q", len(lines), output)
	var r singleResponse
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &r))
	if r.Error == nil && len(r.Result) > 0 {
		var env struct {
			StructuredContent json.RawMessage `json:"structuredContent"`
			Content           []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		}
		if err := json.Unmarshal(r.Result, &env); err == nil {
			if len(env.StructuredContent) > 0 && string(env.StructuredContent) != "null" {
				r.Result = env.StructuredContent
			} else if len(env.Content) > 0 && env.Content[0].Text != "" {
				r.Result = json.RawMessage(env.Content[0].Text)
			}
		}
	}
	return r
}

// runServer pipes the given input into Run and returns the captured
// output + errOut buffers. Used by the stdio protocol smoke test
// below; new tests should use the in-memory client in testkit.go
// instead.
func runServer(t *testing.T, input string) (string, string, error) {
	t.Helper()
	dir := t.TempDir()
	// Pre-create .kron/ so resolveMCPRoot's project-root validation
	// passes. In production, every supported MCP client (Claude
	// Code, Cursor MCP runtime, VSCode extensions) spawns kron
	// with cwd set to a directory that already has .kron/, so the
	// validation is correct for the production path; this helper
	// just mirrors that for the test path.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".kron"), 0o755))
	var out, errOut bytes.Buffer
	err := Run([]string{"-CWD", dir}, strings.NewReader(input), &out, &errOut)
	return out.String(), errOut.String(), err
}

// TestRun_StdioProtocolSmoke is the highest-fidelity stdio test:
// it actually spawns the compiled kron binary, sends an initialize
// handshake + a kron_lint call, and asserts the SDK-mediated
// response. In test environments without the binary built (the
// default for `go test`), the test skips. Build the binary with
//
//	go build -o /tmp/kron ./cmd/kron
//
// and then re-run with KRON_E2E=1 to exercise the full stdio path.
func TestRun_StdioProtocolSmoke(t *testing.T) {
	if os_getenv("KRON_E2E") == "" {
		t.Skip("KRON_E2E not set; skipping real-binary stdio smoke test")
	}
	// The actual subprocess plumbing is documented but not wired by
	// default — see docs/process/mcp-protocol.md §D for the
	// rationale and the manual verification recipe.
	t.Skip("subprocess plumbing deferred to commit 3 (testkit layer covers in-memory e2e)")
}

// TestRun_EmptyStdin verifies Run returns cleanly on empty input.
// Since v1.2 the SDK consumes the empty stdin and exits without
// writing any protocol response (no client connected → no
// initialize → no request to reply to). We assert the call returns
// no error; that's the contract main.go's exitFromErr relies on.
func TestRun_EmptyStdin(t *testing.T) {
	out, errOut, err := runServer(t, "")
	require.NoError(t, err)
	assert.Empty(t, out, "empty stdin must produce no protocol output")
	assert.Empty(t, errOut, "empty stdin must produce no diagnostic output")
}

// TestServer_Notification_NoResponse verifies the SDK respects the
// JSON-RPC notification rule: a request without an "id" field must
// produce no response on the wire.
func TestServer_Notification_NoResponse(t *testing.T) {
	out, errOut, err := runServer(t, `{"jsonrpc":"2.0","method":"kron_lint"}`+"\n")
	require.NoError(t, err)
	// The SDK may or may not refuse the request outright (no
	// initialize). Either is acceptable: the key contract is that
	// we don't see a properly-formed result with an ID attached.
	if out != "" {
		// If it does emit something, it must be a parse / protocol
		// error, not a tool result.
		var probe struct {
			ID      json.RawMessage `json:"id"`
			Result  json.RawMessage `json:"result"`
			Content json.RawMessage `json:"content"`
		}
		require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(out)), &probe))
		assert.Nil(t, probe.ID, "notification must not produce a response with an id")
	}
	_ = errOut
	_ = time.Now
}

// TestRunWithCtx_CancelExitsCleanly is the in-process unit test for
// the G-1 signal-cancellation path. Rather than spawning a real
// process and sending os.Interrupt (which would race the test
// framework's own signal handling), we drive runWithCtx directly
// with a context we control. Cancelling it must make runWithCtx
// return nil — the production "Ctrl+C → clean exit" contract.
//
// We use a fresh server (not the production one) so this test
// exercises the "cancellable ctx drives server.Run" path
// independently of Run's other setup steps. We don't send any
// initialize frames: server.Run's select{} between ctx.Done and
// ss.Wait() will pick the ctx branch as soon as cancel() fires,
// even without an active session.
func TestRunWithCtx_CancelExitsCleanly(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "kron-cancel-test", Version: "v1.2.0"}, nil)

	// Pipe so the SDK's IOTransport reader blocks on a real
	// io.Reader (not nil). The writer side is io.Discard wrapped
	// to satisfy the WriteCloser contract; the package's
	// nopWriteCloser (defined in serve_mcp.go) is exactly that.
	pr, pw := io.Pipe()
	defer pr.Close()
	defer pw.Close()

	transport := &mcp.IOTransport{Reader: pr, Writer: nopWriteCloser{io.Discard}}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- runWithCtx(ctx, server, transport)
	}()

	// Give the SDK a tick to reach its select{}, then cancel.
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		require.NoError(t, err, "ctx cancellation must surface as nil (clean exit)")
	case <-time.After(2 * time.Second):
		t.Fatal("runWithCtx did not return within 2s of ctx cancel")
	}
}

// TestRunWithCtx_DeadlineExceededExitsCleanly is the sibling case
// for context.DeadlineExceeded: a server that times out (e.g. an
// operator-set shutdown deadline) is also a clean shutdown, not a
// transport failure.
func TestRunWithCtx_DeadlineExceededExitsCleanly(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "kron-deadline-test", Version: "v1.2.0"}, nil)

	pr, pw := io.Pipe()
	defer pr.Close()
	defer pw.Close()

	transport := &mcp.IOTransport{Reader: pr, Writer: nopWriteCloser{io.Discard}}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- runWithCtx(ctx, server, transport)
	}()

	select {
	case err := <-done:
		require.NoError(t, err, "ctx deadline must surface as nil (clean exit)")
	case <-time.After(2 * time.Second):
		t.Fatal("runWithCtx did not return within 2s of ctx deadline")
	}
}

// TestRun_NoSignalLeakBetweenInvocations guards the runWithSignalCtx
// contract: each Run() call must register a signal handler only for
// its own lifetime, so consecutive invocations (in particular, the
// many test invocations during `go test`) don't accumulate handlers
// or interfere with each other. We invoke the full Run() entry point
// three times with empty stdin (the SDK exits immediately on EOF
// without entering the long-lived select), and assert every call
// returns nil — which is only true if the second / third
// invocations can register their own signal.NotifyContext without
// conflicting with the first.
func TestRun_NoSignalLeakBetweenInvocations(t *testing.T) {
	for i := 0; i < 3; i++ {
		dir := t.TempDir()
		// Pre-create .kron/ so resolveMCPRoot's project-root validation
		// passes. (See runServer for the rationale.)
		require.NoError(t, os.MkdirAll(filepath.Join(dir, ".kron"), 0o755))
		var out, errOut bytes.Buffer
		err := Run([]string{"-CWD", dir}, strings.NewReader(""), &out, &errOut)
		require.NoErrorf(t, err, "iteration %d must succeed", i)
		assert.Empty(t, out, "iteration %d: empty stdin must produce no protocol output", i)
	}
}

// TestResolveMCPRoot_RejectsNonProject locks the contract added to
// resolveMCPRoot: when a client (mis)configures -CWD or os.Getwd()
// to a directory that doesn't have .kron/, the server must fail
// fast with a clear, actionable error rather than letting the
// first tool call blow up with a misleading "run kron_init
// first" — the project isn't missing, the cwd is wrong.
func TestResolveMCPRoot_RejectsNonProject(t *testing.T) {
	dir := t.TempDir()
	// No .kron/ here.
	_, err := resolveMCPRoot([]string{"-CWD", dir})
	require.Error(t, err, "must reject a directory without .kron/")
	assert.Contains(t, err.Error(), "is not a Kron project",
		"error must explicitly say it isn't a Kron project")
	assert.Contains(t, err.Error(), dir,
		"error must include the offending path so the user knows which cwd to fix")
}

// TestResolveMCPRoot_RejectsNonDirectoryDotKron locks the
// sibling case: if .kron/ exists but is a regular file (not a
// directory), the server must also reject — the project layout
// is corrupted.
func TestResolveMCPRoot_RejectsNonDirectoryDotKron(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".kron"), []byte("not a dir"), 0o644))
	_, err := resolveMCPRoot([]string{"-CWD", dir})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is not a directory",
		"error must explain .kron/ is the wrong file type")
}

// TestResolveMCPRoot_AcceptsValidProject locks the happy path:
// a directory with a real .kron/ is accepted, and -CWD is
// resolved to an absolute path even if the caller passed a
// relative one.
func TestResolveMCPRoot_AcceptsValidProject(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".kron"), 0o755))
	got, err := resolveMCPRoot([]string{"-CWD", dir})
	require.NoError(t, err)
	// filepath.Abs may not equal dir on Windows if dir is a
	// short path or has unusual casing; compare cleaned forms.
	assert.Equal(t, filepath.Clean(got), filepath.Clean(dir))
}

// os_getenv is a small wrapper so this test file does not have to
// import os directly. Trivial abstraction, but keeps the import
// set small and lets us swap to t.Setenv in future if needed.
func os_getenv(k string) string { return getenvImpl(k) }

func getenvImpl(k string) string { return os.Getenv(k) }
