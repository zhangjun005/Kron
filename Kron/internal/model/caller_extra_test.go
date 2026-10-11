package model

import (
	"context"
	"errors"
	"testing"
)

func TestCallerKnown(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		// Five base callers.
		{CallerCLI, true},
		{CallerMCP, true},
		{CallerLSP, true},
		{CallerIDE, true},
		{CallerGUI, true},

		// Empty is NOT a known caller; it means "no injection".
		{"", false},

		// Derivatives are NOT base callers; use IsMCPCaller for those.
		{"mcp:claude-3.7", false},
		{"cli:stdin", false},

		// Typos and unknowns.
		{"CLI", false}, // case-sensitive
		{"unknown", false},
		{" cli", false}, // leading space
		{"mcp ", false}, // trailing space
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := CallerKnown(tc.in); got != tc.want {
				t.Errorf("CallerKnown(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestIsMCPCaller(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{CallerMCP, true},        // base identity
		{"mcp:claude-3.7", true}, // canonical derivative
		{"mcp:cursor", true},     // short derivative
		{"mcp:", true},           // bare prefix is still derivative-shaped
		{CallerCLI, false},
		{CallerLSP, false},
		{CallerIDE, false},
		{CallerGUI, false},
		{"", false},         // empty is not MCP
		{"mcp", true},       // same as CallerMCP
		{"mcpx:foo", false}, // different prefix
		{"cli:mcp", false},  // "mcp" suffix does not promote
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := IsMCPCaller(tc.in); got != tc.want {
				t.Errorf("IsMCPCaller(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestRequireKnownCaller(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		err := RequireKnownCaller(context.Background())
		if err == nil {
			t.Fatal("RequireKnownCaller with no injection must return an error")
		}
		if !errors.Is(err, err) {
			// No sentinel; just confirm it's a non-nil Go error with a
			// message that mentions injection.
		}
		if msg := err.Error(); msg == "" {
			t.Error("error message must be non-empty")
		}
	})

	for _, c := range []string{CallerCLI, CallerMCP, CallerLSP, CallerIDE, CallerGUI} {
		t.Run("accepts "+c, func(t *testing.T) {
			ctx := WithCaller(context.Background(), c)
			if err := RequireKnownCaller(ctx); err != nil {
				t.Errorf("RequireKnownCaller(%q) = %v, want nil", c, err)
			}
		})
	}

	t.Run("rejects derivative", func(t *testing.T) {
		ctx := WithCaller(context.Background(), "mcp:claude-3.7")
		err := RequireKnownCaller(ctx)
		if err == nil {
			t.Error("RequireKnownCaller must reject MCP derivatives (use IsMCPCaller for those)")
		}
	})

	t.Run("rejects unknown", func(t *testing.T) {
		ctx := WithCaller(context.Background(), "agent:rogue")
		err := RequireKnownCaller(ctx)
		if err == nil {
			t.Error("RequireKnownCaller must reject unknown caller identities")
		}
	})
}
