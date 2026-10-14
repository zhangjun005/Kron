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
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/store"
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

	// C-3: Route stdlib log and slog output through the MCP Logging
	// capability (kronLoggingHandler) when a session is active; fall
	// back to errOut when no session is established yet (e.g. during
	// the initialize handshake) or after disconnect. This replaces
	// the previous log.SetOutput(errOut) approach.
	//
	// The stdlib log package and slog both use their respective
	// default loggers. We set both so handlers that use log.Print*
	// (stdlib) are also captured. In practice, the main producer of
	// structured logs is the SDK itself (via slog.Logger.Info in
	// server.go); the stdlib log fallback covers any legacy log.*
	// calls in the handler layer.
	// C-3: Route slog output through the MCP Logging capability
	// (shimHandler) when a session is active. This replaces the
	// previous log.SetOutput(errOut) approach for slog-based logging.
	//
	// In production, server.Run() creates the ServerSession internally
	// and doesn't expose it to us, so SetActiveSession is not called
	// from Run. The shimHandler drops records when no session is
	// active (acceptable: pre-init/post-disconnect messages are not
	// the ones users care about). Tests that use newTestSession DO
	// call SetActiveSessionOnClose, so MCP logging works in tests.
	//
	// Stdlib log.* calls: keep log.SetOutput(errOut) for now.
	// There is no way to redirect stdlib log to slog in Go, and
	// we don't use log.* in hot paths anyway.
	shim := &shimHandler{}
	log.SetOutput(errOut)
	slog.SetDefault(slog.New(shim))

	// Create the SDK server with explicit ServerOptions so the
	// contract with MCP clients is clear (G-4):
	//   - Tools: auto-inferred from registerTools() below
	//     (SDK sets Tools: {ListChanged: true} when any tool is
	//     registered).
	//   - Resources, Prompts: SDK leaves them nil because we
	//     never call AddResource / AddPrompt. We do NOT set
	//     HasResources / HasPrompts in ServerOptions, so the
	//     wire contract is "I do not advertise these" — clients
	//     won't issue resources/list or prompts/list against us.
	//   - Logging: SDK defaults Logging to &LoggingCapabilities{}
	//     regardless of whether we register a handler. The C-3
	//     shimHandler wires the actual logging dispatch:
	//     activeSession.Log() is called when a session exists;
	//     if the client never calls logging/setLevel, the SDK
	//     silently drops the message (per MCP spec).
	//
	// G-4 (docs/mcp/backlog.md §G-4 + docs/mcp/missing-capabilities.md §G-4).
	//
	// SDK note: go-sdk v1.1.0 does not expose a Capabilities field
	// in ServerOptions; the only control points are HasResources /
	// HasPrompts / HasTools. We keep them all at zero (the default)
	// to preserve "Tools auto-on, Resources / Prompts off" semantics.
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

	// C-1: register the kron://intents/{slug} resource template.
	// One template covers every intent (no per-intent static
	// AddResource calls); the per-read handler does the lookup.
	// docs/mcp/missing-capabilities.md §C-1.
	registerResources(server)

	// RFC docs/rfc/2026-10-14-mcp-prompts.md: register 3 prompt
	// templates (kron-implement / kron-drift-check / kron-review).
	// The SDK auto-infers the prompts capability from AddPrompt
	// calls (same path as AddTool/AddResourceTemplate); the
	// capabilities_test.go suite asserts this stays true.
	registerPrompts(server)

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
	return runWithSignalCtx(server, transport)
}

// runWithSignalCtx is Run's stdio-entry implementation with a
// context that cancels on os.Interrupt. The split from Run() lets
// tests drive the cancellation path via runWithCtx (no real
// signal required) while production keeps the clean
// signal.NotifyContext integration.
//
// We register ONLY os.Interrupt:
//   - Windows doesn't deliver SIGTERM to console processes, so
//     adding it would be dead code on the platform Kron
//     development happens on.
//   - The stdio MCP server's primary exit path is "client closes
//     stdin" (EOF on os.Stdin) — the SDK's transport.Run goroutine
//     returns from ss.Wait() when the stdin reader hits EOF.
//     Signal-based termination is a fallback for the case where
//     the user (or a parent process) wants to kill the server
//     while the client is still attached (or hung).
//   - stop() unregisters the signal handler when the function
//     returns, so multiple Run() invocations from tests don't
//     leak handlers.
func runWithSignalCtx(server *mcp.Server, transport mcp.Transport) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return runWithCtx(ctx, server, transport)
}

// runWithCtx is the innermost Run implementation: it owns no
// process-level state and can be driven with any context.Context
// the caller supplies. Tests pass a context they control (and
// cancel); the production path supplies a signal.NotifyContext.
// The two layers (signal vs. plain context) are intentionally
// separate so the "production vs. test" choice happens at the
// boundary, not deep inside Run.
//
// context.Canceled is the expected outcome of a signal-triggered
// shutdown. We swallow it so main.go's exitFromErr maps the run
// to exit 0 (clean shutdown) rather than 1 (transport failure).
// Other ctx-derived errors (DeadlineExceeded) are surfaced the
// same way: a server that times out is shutting down on the
// operator's behalf, not failing.
func runWithCtx(ctx context.Context, server *mcp.Server, transport mcp.Transport) error {
	// C-3: The shimHandler routes log records to the MCP Logging
	// capability when a session is active. In production, server.Run()
	// creates the session internally and doesn't expose it to us, so
	// we cannot call SetActiveSession. The shimHandler's fallback
	// (which uses a no-op logger) handles the "no session" case.
	// Tests that use newTestSession (testkit.go) DO call
	// SetActiveSessionOnClose after server.Connect, so MCP logging
	// works for tests.
	if err := server.Run(ctx, transport); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil
		}
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
//
// Fallback priority (RFC docs/rfc/2026-10-13-multi-project-registry.md
// §2.8 + docs/rfc/2026-10-14-projects-json-schema-extension.md §2.1):
//
//  1. -CWD <root> flag   — explicit override, wins
//  2. projects.json::current_project — GUI-maintained active project
//  3. os.Getwd()         — stdio default
//
// After resolving the path, we validate that <root>/.kron/ exists.
// MCP clients (Claude Code, Cursor MCP runtime, VSCode extensions)
// typically spawn the kron subprocess with cwd set to the project
// root, so os.Getwd() is correct in the common case. But there is
// no protocol-level guarantee: a misconfigured client can spawn
// kron in $HOME or some other directory, in which case the SDK
// hands us a perfectly valid path that just isn't a Kron project.
// Validating up front lets us fail with a clear, actionable error
// ("pass -CWD <repo-root>") instead of waiting for the first
// tool call to surface ".kron/ not found; run kron_init first"
// — which is misleading because the project isn't missing, the
// cwd is.
func resolveMCPRoot(args []string) (string, error) {
	switch {
	case len(args) == 2 && args[0] == "-CWD":
		abs, err := filepath.Abs(args[1])
		if err != nil {
			return "", fmt.Errorf("kron serve-mcp: cannot resolve -CWD path %q: %w", args[1], err)
		}
		return validateIsKronProject(abs)

	case len(args) == 0:
		// Try current_project from projects.json before falling
		// back to os.Getwd(). Per RFC §2.8 priority 2. A missing
		// or unparseable registry is NOT an error here — we
		// silently fall through to os.Getwd() because the stdio
		// spawn may legitimately happen before any kron
		// register/init has ever run (e.g. first-time setup).
		if cp, err := readCurrentProjectFromRegistry(); err == nil && cp != "" {
			if abs, err := filepath.Abs(cp); err == nil {
				if _, statErr := os.Stat(filepath.Join(abs, ".kron")); statErr == nil {
					return abs, nil
				}
				// current_project points at a non-Kron path
				// (e.g. user removed .kron/ since registering).
				// Fall through to os.Getwd() rather than
				// failing — the user may have multiple
				// checkouts and this one simply isn't
				// current anymore.
			}
		}
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("kron serve-mcp: cannot determine working directory: %w", err)
		}
		return validateIsKronProject(wd)

	default:
		return "", fmt.Errorf("kron serve-mcp: unexpected arguments %v (only -CWD <root> is supported)", args)
	}
}

// readCurrentProjectFromRegistry reads the multi-project
// registry and returns its current_project field, or "" +
// non-nil error if anything goes wrong (missing file, parse
// error, empty current_project). The non-nil error lets the
// caller distinguish "no registry" from "registry without
// current_project" — both are valid fallback states.
func readCurrentProjectFromRegistry() (string, error) {
	reg, err := store.ReadProjectsJSON()
	if err != nil {
		return "", err
	}
	if reg == nil || reg.CurrentProject == "" {
		return "", fmt.Errorf("no current_project set")
	}
	return reg.CurrentProject, nil
}

// validateIsKronProject checks that path contains a .kron/
// directory and returns a clear error if not. Used by
// resolveMCPRoot after picking a root from -CWD or os.Getwd().
func validateIsKronProject(path string) (string, error) {
	kronDir := filepath.Join(path, ".kron")
	info, err := os.Stat(kronDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf(
				"kron serve-mcp: %s is not a Kron project (.kron/ not found). "+
					"Pass -CWD <repo-root> to point at the directory that contains .kron/",
				path,
			)
		}
		return "", fmt.Errorf("kron serve-mcp: cannot stat %s: %w", kronDir, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf(
			"kron serve-mcp: %s exists but is not a directory (.kron/ must be a directory)",
			kronDir,
		)
	}
	return path, nil
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
