package servemcp

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// testSession bundles an mcp.ClientSession and the mcp.Server it
// connects to (via InMemoryTransport) so individual tests can call
// tool methods directly. Cleanup is the test's t.Cleanup callback.
//
// Why InMemoryTransport (and not stdio + bytes.Buffer):
//
//	The official go-sdk v1.0/v1.1 StdioTransport reads/writes the
//	real os.Stdin / os.Stdout streams only. IOTransport (added in
//	v1.1) accepts arbitrary io.ReadCloser / io.WriteCloser but in
//	practice its batched-write pipeline (see go-sdk v1.1 transport.go
//	line 600-625) makes single-message round-trips unreliable in
//	test setups where the input ends immediately. NewInMemoryTransports
//	uses an io.Pipe under the hood and works as expected.
//
//	Test code in this file is therefore 100% in-memory: it bypasses
//	Run() (which is the production stdio entry point) and instead
//	drives the server through a ClientSession. Run() itself is still
//	the production entry point; we exercise it once in
//	TestRun_StdioProtocolSmoke (see serve_mcp_test.go) by spawning
//	the compiled binary as a subprocess.
type testSession struct {
	Server  *mcp.Server
	Session *mcp.ClientSession
	Cleanup func()
}

// newTestSession constructs a test kron server (registered 12 tools
// against the supplied root directory) and a client session talking
// to it via InMemoryTransport. root is typically t.TempDir().
//
// The returned cleanup closes the client session and the server.
// The test should defer cleanup() right after the call.
//
// KRON_PROJECTS_JSON_PATH isolation: kron_init's RFC 2026-10-14-§2.6
// side effect (register project + set current_project) writes to
// the multi-project registry at <kron-bin>/projects.json. Without
// isolation every test would race on the same on-disk file. We
// honour the project's documented test escape hatch (see
// internal/store/project_registry.go: ProjectsJSONPath reads
// KRON_PROJECTS_JSON_PATH first) and set it to a per-test temp path.
// t.Setenv auto-cleans on test end.
func newTestSession(t *testing.T, root string) *testSession {
	t.Helper()
	registryPath := filepath.Join(t.TempDir(), "projects.json")
	t.Setenv("KRON_PROJECTS_JSON_PATH", registryPath)

	server := mcp.NewServer(&mcp.Implementation{Name: "kron-test", Version: "v1.2.0"}, nil)

	// Inject the repo root on the ctx so handlers read .kron/intents/
	// from the test's temp dir, not from os.Getwd().
	server.AddReceivingMiddleware(callerInjectMiddleware(root))
	server.AddReceivingMiddleware(coerceMiddleware())

	registerTools(server)
	// C-1: register the kron://intents/{slug} resource template
	// so test clients can exercise resources/list + resources/read
	// against the same handler the production server uses.
	registerResources(server)
	// RFC 2026-10-14-mcp-prompts.md: register the 3 prompt
	// templates so test clients can exercise prompts/list +
	// prompts/get against the same handlers the production server
	// uses.
	registerPrompts(server)

	st, ct := mcp.NewInMemoryTransports()

	// Connect server first (the client requires the server session
	// to be live to negotiate the initialize handshake).
	ss, err := server.Connect(context.Background(), st, nil)
	if err != nil {
		t.Fatalf("server.Connect: %v", err)
	}

	// Now connect the client. cs holds a single bidirectional session.
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v1.0.0"}, nil).
		Connect(context.Background(), ct, nil)
	if err != nil {
		_ = ss.Close()
		t.Fatalf("client.Connect: %v", err)
	}

	cleanup := func() {
		_ = cs.Close()
		_ = ss.Close()
		ClearActiveSession(ss)
	}
	// C-3: wire active session for MCP logging so tests can also
	// exercise the logging path (even though tests don't verify it yet).
	clearSession := SetActiveSessionOnClose(ss)
	t.Cleanup(clearSession)
	return &testSession{Server: server, Session: cs, Cleanup: cleanup}
}
