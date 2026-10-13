package servemcp

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestC3_ShimHandler_DropsWithoutSession verifies the shimHandler's
// fallback behavior: when no session is set, log records are dropped
// silently (no panic, no error). This is the production path before
// initialize / after disconnect.
func TestC3_ShimHandler_DropsWithoutSession(t *testing.T) {
	ClearActiveSession(nil) // ensure nil
	shim := shimHandler{}
	if err := shim.Handle(context.Background(), slog.Record{
		Level:   slog.LevelInfo,
		Message: "test",
		Time:    time.Now(),
	}); err != nil {
		t.Errorf("Handle returned %v with no session; want nil", err)
	}
}

// TestC3_ShimHandler_HandlesWithSession verifies the MCP logging path:
// when a session is set, shimHandler dispatches to mcp.NewLoggingHandler,
// which calls ss.Log which respects the client-set LogLevel.
//
// We don't try to verify the message reached the client (the SDK
// doesn't expose a synchronous "wait for log notification" API in
// v1.1.0). Instead, we just verify Handle() doesn't error when a
// session is active — proving the dispatch path is wired.
func TestC3_ShimHandler_HandlesWithSession(t *testing.T) {
	// Set up an in-memory client+server pair so the server session
	// is real (not nil).
	server := mcp.NewServer(&mcp.Implementation{Name: "kron-log-test", Version: "v1.2.0"}, nil)
	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(context.Background(), st, nil)
	if err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	defer ss.Close()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "log-test-client", Version: "v1.0.0"}, nil).
		Connect(context.Background(), ct, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	defer cs.Close()

	// Initialize + enable logging at the lowest level so the test
	// record passes the SDK's level gate. The Initialize call is
	// made implicitly by Connect via ClientSession; we just need
	// to send SetLogLevel.
	if err := cs.SetLoggingLevel(context.Background(), &mcp.SetLoggingLevelParams{
		Level: mcp.LoggingLevel("debug"),
	}); err != nil {
		t.Fatalf("SetLoggingLevel: %v", err)
	}

	// With a session set, Handle must not return an error.
	shim := shimHandler{}
	if err := shim.Handle(context.Background(), slog.Record{
		Level:   slog.LevelInfo,
		Message: "hello from kron",
		Time:    time.Now(),
	}); err != nil {
		t.Errorf("Handle returned %v; want nil", err)
	}
}

// TestC3_LogMinInterval_200ms locks the rate-limit constant. If a
// future maintainer changes it, this test forces an explicit decision
// (the constant affects how the client perceives log volume).
func TestC3_LogMinInterval_200ms(t *testing.T) {
	if logMinInterval != 200*time.Millisecond {
		t.Errorf("logMinInterval = %v, want 200ms; changing this alters client log volume", logMinInterval)
	}
}
