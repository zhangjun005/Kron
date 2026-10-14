package servemcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestCapabilities_ExplicitContract verifies the ServerCapabilities
// advertised in the initialize response match Kron's intent (G-4).
//
// G-4 motivation:
//
//   - The MCP spec requires clients to consult the server's
//     `capabilities` field after `initialize` to learn what
//     methods they may call. Misleading capabilities (e.g. logging:
//     {}  when no logging handler is registered) lead to client
//     errors at runtime.
//
//   - go-sdk v1.1.0 behaviour (see protocol.go initCaps):
//     caps := &ServerCapabilities{ Logging: &LoggingCapabilities{} }
//     if tools registered: caps.Tools = &ToolCapabilities{...}
//     if prompts registered: caps.Prompts = ...
//     if resources registered: caps.Resources = ...
//     So the SDK ALWAYS advertises Logging: {} regardless of
//     whether the server registered a handler. We document this
//     in the test contract: clients may safely call
//     logging/setLevel against us and receive a method-not-found
//     per MCP spec — we never register the handler.
//
//   - We do NOT register any prompts or resources, so those keys
//     must be absent from the wire response.
//
//   - Tools MUST be present and listChanged MUST be true (SDK
//     auto-sets it when any tool is registered via AddTool).
//
// We exercise the actual initialize handshake end-to-end via the
// InMemoryTransport pipe: client → server `initialize` → server
// returns its `capabilities` field. We decode that and assert the
// expected shape.
func TestCapabilities_ExplicitContract(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "kron-cap-test", Version: "v1.2.0"}, nil)
	server.AddReceivingMiddleware(callerInjectMiddleware(t.TempDir()))
	registerTools(server)
	// 2026-10-14: register the 3 prompts so the capability check
	// below covers the new auto-inferred prompts capability too.
	registerPrompts(server)

	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(context.Background(), st, nil)
	if err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	defer ss.Close()

	cs, err := mcp.NewClient(&mcp.Implementation{Name: "cap-test-client", Version: "v1.0.0"}, nil).
		Connect(context.Background(), ct, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	defer cs.Close()

	// The Connect call above has already run the initialize handshake
	// internally; the result is exposed via ClientSession.InitializeResult().
	res := cs.InitializeResult()
	if res == nil {
		t.Fatal("InitializeResult is nil; initialize handshake did not complete")
	}

	// Re-marshal + unmarshal to get a wire-level view of capabilities.
	// (mcp.InitializeResult embeds the SDK's struct; raw JSON is
	// what MCP clients actually consume.)
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var wire struct {
		Capabilities json.RawMessage `json:"capabilities"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(wire.Capabilities) == 0 {
		t.Fatal("capabilities is empty in initialize response")
	}

	// Decode the capabilities as a map so we can do field-by-field
	// assertions regardless of struct shape.
	var caps map[string]json.RawMessage
	if err := json.Unmarshal(wire.Capabilities, &caps); err != nil {
		t.Fatalf("unmarshal caps: %v", err)
	}

	// Resources MUST be absent (we never register any).
	if _, ok := caps["resources"]; ok {
		t.Errorf("expected resources to be absent, got %s", string(caps["resources"]))
	}

	// Tools must be present (we register 17 tools in registerTools).
	tools, ok := caps["tools"]
	if !ok {
		t.Fatal("expected tools capability to be present (we register 17 tools)")
	}
	var toolsCap struct {
		ListChanged bool `json:"listChanged"`
	}
	if err := json.Unmarshal(tools, &toolsCap); err != nil {
		t.Fatalf("unmarshal tools cap: %v", err)
	}
	if !toolsCap.ListChanged {
		t.Error("expected tools.listChanged = true (SDK auto-infer from registered tools)")
	}

	// 2026-10-14: Prompts must be present (we register 3 prompts
	// via registerPrompts). SDK auto-infers the prompts
	// capability from AddPrompt calls, just like AddTool for
	// tools. listChanged should be true to signal that the set
	// is dynamic (so future prompts don't break clients that
	// cache the initial list).
	prompts, ok := caps["prompts"]
	if !ok {
		t.Fatal("expected prompts capability to be present (we register 3 prompts via registerPrompts)")
	}
	var promptsCap struct {
		ListChanged bool `json:"listChanged"`
	}
	if err := json.Unmarshal(prompts, &promptsCap); err != nil {
		t.Fatalf("unmarshal prompts cap: %v", err)
	}
	if !promptsCap.ListChanged {
		t.Error("expected prompts.listChanged = true (SDK auto-infer from registered prompts)")
	}

	// Logging: SDK unconditionally advertises Logging: {} (we
	// cannot disable this in v1.1.0). Document the behaviour:
	// clients may call logging/setLevel, and will receive a
	// method-not-found error because we never register a
	// logging handler. This is acceptable per MCP spec.
	if _, ok := caps["logging"]; !ok {
		t.Error("expected logging capability to be present (SDK default: empty struct; clients calling logging/setLevel will get method-not-found)")
	}
}
