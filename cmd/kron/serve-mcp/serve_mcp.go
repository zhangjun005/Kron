// Package servemcp is the MCP stdio server entry point for Kron.
//
// This package is a peer of cmd/kron/cli in the access layer. Like
// every access layer, it owns its own command-line surface and its
// own process model (stdin/stdout JSON-RPC) and delegates all
// business logic to internal/store, internal/parser, and internal/lint.
// It MUST NOT import any sibling access layer package (cmd/kron/cli,
// future cmd/kron/serve-gui).
//
// Wire protocol: JSON-RPC 2.0 over stdio (one JSON object per line).
// Since v1.2 the protocol layer is delegated to the official
// github.com/modelcontextprotocol/go-sdk (Tier 1 SDK maintained with
// Google). See docs/rfc/2026-10-04-mcp-sdk-adoption.md for the
// architectural decision and architecture.md §2.4 for the approved-dep
// exception.
package servemcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/xxx/kron/internal/model"
)

// ErrNotImplemented is the v1 sentinel kept for backwards compatibility
// with cmd/kron/main.go, which checks errors.Is(err, ErrNotImplemented)
// to map it to exit 1. Since v1.1 the MCP server is implemented; this
// sentinel is no longer returned by Run. It is kept exported so external
// tooling (e.g. CI scripts) that grep for "not yet available" continues
// to parse the binary's stderr without changes.
var ErrNotImplemented = errors.New("kron serve-mcp: not yet available (planned for phase 2)")

// serverName / serverVersion / protocolVersion are surfaced to MCP
// clients during the initialize handshake. Bump Version with each Kron
// release. Pin protocolVersion to whatever MCP spec the pinned SDK
// version targets — see go.mod's go-sdk requirement line and
// docs/rfc/2026-10-04-mcp-sdk-adoption.md §2.
const (
	serverName    = "kron"
	serverVersion = "v1.2.0"
)

// Run is the entry point. args[0] is the optional "-CWD <root>" flag;
// absent it defaults to the process's current working directory.
//
// The Run signature is preserved from v1.1 (args, in, out, errOut) so
// cmd/kron/main.go does not need to know about the SDK. The actual
// transport is the SDK's StdioTransport; we hand it os.Stdin /
// os.Stdout regardless of the (in, out, errOut) parameters because
// the stdio MCP transport only operates on the real OS streams.
// Tests that pass bytes.Buffer / strings.Reader as in/out bypass Run
// via a test helper (testTransportFromBuffers) defined in
// serve_mcp_test.go.
//
// Per-tool caller identity is "mcp:<client>" where <client> comes from
// the MCP initialize ClientInfo; bare "mcp" if no client name was
// supplied. The caller is an assertion, not an authz gate; tools MUST
// NOT use it for permission decisions (architecture.md §2.3).
func Run(args []string, in io.Reader, out, errOut io.Writer) error {
	_ = in // stdio transport reads os.Stdin directly; signature preserved for main.go compat

	root, err := resolveMCPRoot(args)
	if err != nil {
		fmt.Fprintln(errOut, "kron serve-mcp:", err)
		return err
	}

	// Route logs to errOut (typically os.Stderr). stdio transport
	// writes only JSON-RPC messages to stdout; server.Run() will
	// ignore anything written to stderr.
	log.SetOutput(errOut)

	// Create the SDK server. Capabilities are auto-inferred from
	// the registered tools (we register 12 below), so we do not
	// pass an explicit Capabilities struct.
	server := mcp.NewServer(&mcp.Implementation{
		Name:    serverName,
		Version: serverVersion,
	}, nil)

	// Inject the Kron caller identity into every handler ctx before
	// dispatch. This satisfies architecture.md §〇 铁律 #4: access
	// layers identify themselves via context.Context. The
	// middleware reads the client name from
	// req.Session().ClientInfo() once we have it; on the very
	// first dispatch (where the session may not be set yet, e.g.
	// for ping / initialize) it falls back to bare "mcp".
	server.AddReceivingMiddleware(callerInjectMiddleware(root))

	// Register the 12 kron_* tools. Done after middleware so
	// handlers see the injected caller on their ctx.
	registerTools(server)

	// Run blocks until the client closes stdin or the transport
	// errors out. Returning a non-nil error here means
	// unrecoverable transport failure (rare).
	//
	// In production (cmd/kron/main.go calls Run with os.Stdin /
	// os.Stdout) we use StdioTransport which reads/writes the real
	// OS streams.
	//
	// In tests (which pass strings.NewReader / bytes.Buffer as in)
	// we use IOTransport so the SDK reads from the supplied Reader
	// instead of os.Stdin. We detect the test path by checking
	// whether in is an *os.File whose fd is 0 (stdin); if it is
	// not, we fall back to IOTransport.
	transport, err := stdioOrIOTransport(in, out)
	if err != nil {
		return err
	}
	if err := server.Run(context.Background(), transport); err != nil {
		return fmt.Errorf("kron serve-mcp: %w", err)
	}
	return nil
}

// stdioOrIOTransport picks the right mcp.Transport for the caller's
// (in, out). If in is os.Stdin (file descriptor 0) and out is
// os.Stdout, use the SDK's StdioTransport which reads/writes the
// real OS streams directly. Otherwise use IOTransport with the
// caller-provided Reader / Writer (typically bytes.Buffer /
// strings.NewReader from tests).
func stdioOrIOTransport(in io.Reader, out io.Writer) (mcp.Transport, error) {
	if f, ok := in.(*os.File); ok && f.Fd() == 0 {
		// Best-effort: also require out to be stdout.
		if of, ok := out.(*os.File); ok && of.Fd() == 1 {
			return &mcp.StdioTransport{}, nil
		}
	}
	// Wrap in/out as ReadCloser / WriteCloser for IOTransport.
	r, ok := in.(io.ReadCloser)
	if !ok {
		r = nopReadCloser{in}
	}
	w, ok := out.(io.WriteCloser)
	if !ok {
		w = nopWriteCloser{out}
	}
	return &mcp.IOTransport{Reader: r, Writer: w}, nil
}

type nopReadCloser struct{ io.Reader }

func (nopReadCloser) Close() error { return nil }

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }

// resolveMCPRoot parses the optional "-CWD <root>" flag pair. Keeping
// it stdlib-only (no cobra) is intentional — this layer is
// independently testable and shouldn't drag in CLI argument conventions.
func resolveMCPRoot(args []string) (string, error) {
	if len(args) == 0 {
		return os.Getwd()
	}
	if len(args) == 2 && args[0] == "-CWD" {
		return filepath.Abs(args[1])
	}
	return "", fmt.Errorf("kron serve-mcp: unexpected arguments %v (only -CWD <root> is supported)", args)
}

// callerInjectMiddleware returns a server.AddReceivingMiddleware
// function that injects model.WithCaller(ctx, "mcp:<client>") into the
// ctx of every incoming request, so handlers downstream can call
// model.CallerFrom(ctx) per architecture.md §〇 铁律 #4.
//
// The SDK calls this middleware once per request, with the request
// available as the *mcp.Request interface (which exposes
// GetSession()). Client name extraction happens after the initialize
// handshake; for the very first request (initialize itself) the
// session is not yet established, so we fall back to bare "mcp".
//
// root is captured per-Run() and threaded to handlers via ctx so that
// handlers can resolve repo-relative paths without a closure. This
// matches the v1.1 behaviour where root was baked into each
// handleXxx(root) closure.
func callerInjectMiddleware(root string) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			ctx = withRepoRoot(ctx, root)
			ctx = withMCPCaller(ctx, clientNameFromReq(req))
			return next(ctx, method, req)
		}
	}
}

// withMCPCaller injects the MCP caller identity into ctx. If client
// is non-empty the caller is "mcp:<client>"; otherwise it falls back
// to the bare "mcp" identity.
func withMCPCaller(ctx context.Context, client string) context.Context {
	if client == "" {
		return model.WithCaller(ctx, model.CallerMCP)
	}
	return model.WithCaller(ctx, model.CallerMCP+":"+client)
}

// repoRootKey is the unexported ctx key under which the resolved
// repository root is stored. Handlers read it via repoRootFrom.
// Keeping the value on the ctx (rather than a per-handler closure)
// preserves the v1.1 "handler signature has no root parameter"
// contract while letting the middleware set it per-Run().
type repoRootKey struct{}

// withRepoRoot stores the repo root on ctx for handlers to read via
// repoRootFrom.
func withRepoRoot(ctx context.Context, root string) context.Context {
	return context.WithValue(ctx, repoRootKey{}, root)
}

// repoRootFrom extracts the repo root set by the middleware. Handlers
// that took a root-closure parameter in v1.1 should now read root via
// this helper to honour the middleware-set value (and to keep the
// access-layer abstraction uniform: the root flows in via ctx, not
// via per-tool closure state).
func repoRootFrom(ctx context.Context) string {
	if s, ok := ctx.Value(repoRootKey{}).(string); ok {
		return s
	}
	return ""
}

// clientNameFromReq extracts the MCP client name from the request
// session. Returns "" when the session is not yet established
// (e.g., during initialize itself) — the caller is then bare "mcp".
//
// In mcp-go-sdk v1.0 the Request interface exposes GetSession() but
// the Session interface itself does not expose a typed ClientInfo
// accessor at this version; we fall back to "" and rely on the
// future SDK to add the typed accessor. See
// docs/process/rfc-2026-10-04-mcp-sdk-adoption.md §C for the
// follow-up that wires ClientInfo-based attribution once the SDK
// stabilises.
func clientNameFromReq(req mcp.Request) string {
	if req == nil {
		return ""
	}
	// We intentionally do not call req.GetSession() here: the returned
	// Session is consumed by SDK internals and is not safe to inspect
	// on the receiving path. client attribution is parked on the
	// v1.2.1 milestone where the SDK exposes a stable typed accessor.
	return ""
}
