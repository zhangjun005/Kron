package model

import (
	"context"
	"testing"
)

func TestWithCaller_RoundTrip(t *testing.T) {
	ctx := context.Background()
	ctx = WithCaller(ctx, CallerMCP)

	if got, want := CallerFrom(ctx), CallerMCP; got != want {
		t.Errorf("CallerFrom = %q, want %q", got, want)
	}
}

func TestCallerFrom_EmptyWhenAbsent(t *testing.T) {
	ctx := context.Background()

	if got, want := CallerFrom(ctx), ""; got != want {
		t.Errorf("CallerFrom = %q, want %q", got, want)
	}
}

func TestCallerFrom_EmptyWhenUnrelatedValue(t *testing.T) {
	// A different package's context value must NOT be visible to CallerFrom.
	type otherKey struct{}
	ctx := context.WithValue(context.Background(), otherKey{}, "mcp")

	if got, want := CallerFrom(ctx), ""; got != want {
		t.Errorf("CallerFrom = %q, want %q", got, want)
	}
}

func TestWithCaller_DerivedString(t *testing.T) {
	// MCP can pass "mcp:claude-3.7" as a derivative identifier.
	// CallerFrom must return the full string verbatim.
	ctx := WithCaller(context.Background(), "mcp:claude-3.7")

	if got, want := CallerFrom(ctx), "mcp:claude-3.7"; got != want {
		t.Errorf("CallerFrom = %q, want %q", got, want)
	}
}

func TestWithCaller_DoesNotMutateParent(t *testing.T) {
	parent := context.Background()
	if got, want := CallerFrom(parent), ""; got != want {
		t.Fatalf("parent ctx must not carry caller identity; got %q", got)
	}

	child := WithCaller(parent, CallerCLI)
	if got, want := CallerFrom(child), CallerCLI; got != want {
		t.Errorf("child ctx must carry injected identity; got %q, want %q", got, want)
	}

	// parent must remain untouched (WithValue semantics).
	if got, want := CallerFrom(parent), ""; got != want {
		t.Errorf("WithCaller must not mutate parent ctx; got %q, want %q", got, want)
	}
}

func TestWithCaller_AcceptsEmpty(t *testing.T) {
	// Empty caller is documented as valid (used by direct test invocations).
	ctx := WithCaller(context.Background(), "")

	if got, want := CallerFrom(ctx), ""; got != want {
		t.Errorf("CallerFrom = %q, want %q", got, want)
	}
}

func TestCallerConstants(t *testing.T) {
	// Pin the wire values. Internal packages and access layers depend on these
	// exact strings (architecture.md §2.3).
	wants := map[string]string{
		"CallerCLI": CallerCLI,
		"CallerMCP": CallerMCP,
		"CallerLSP": CallerLSP,
		"CallerIDE": CallerIDE,
		"CallerGUI": CallerGUI,
	}
	expected := map[string]string{
		"CallerCLI": "cli",
		"CallerMCP": "mcp",
		"CallerLSP": "lsp",
		"CallerIDE": "ide",
		"CallerGUI": "gui",
	}
	for name, got := range wants {
		if want := expected[name]; got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}
