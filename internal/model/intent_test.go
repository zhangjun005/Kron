package model

import (
	"testing"
	"time"
)

func TestStatus(t *testing.T) {
	tests := []struct {
		name  string
		give  Status
		want  string
		isSet bool
	}{
		{name: "draft", give: StatusDraft, want: "draft", isSet: true},
		{name: "active", give: StatusActive, want: "active", isSet: true},
		{name: "superseded", give: StatusSuperseded, want: "superseded", isSet: true},
		{name: "empty is valid, means no lifecycle", give: Status(""), want: "", isSet: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.give) != tt.want {
				t.Errorf("Status = %q, want %q", tt.give, tt.want)
			}
			isSet := tt.give != ""
			if isSet != tt.isSet {
				t.Errorf("Status %q isSet = %v, want %v", tt.give, isSet, tt.isSet)
			}
		})
	}
}

func TestIntent(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	fm := Frontmatter{
		Symbol:    []string{"auth.RefreshToken"},
		CreatedBy: "@zhangjun005",
		UpdatedAt: now,
		Reviewers: []string{"@alice", "@bob"},
		Status:    StatusActive,
	}

	intent := Intent{
		Slug:        "auth/jwt-sliding-window",
		Frontmatter: fm,
		Body:        "# Intent title\n\n> One-line summary.\n\n## Why\n...",
		SourcePath:  "/home/user/project/.kron/intents/auth/jwt-sliding-window.md",
	}

	if intent.Slug != "auth/jwt-sliding-window" {
		t.Errorf("Slug = %q, want %q", intent.Slug, "auth/jwt-sliding-window")
	}
	if intent.Frontmatter.Status != StatusActive {
		t.Errorf("Frontmatter.Status = %v, want %v", intent.Frontmatter.Status, StatusActive)
	}
	if len(intent.Frontmatter.Symbol) != 1 || intent.Frontmatter.Symbol[0] != "auth.RefreshToken" {
		t.Errorf("Frontmatter.Symbol = %v, want [%s]", intent.Frontmatter.Symbol, "auth.RefreshToken")
	}
	if intent.SourcePath == "" {
		t.Error("SourcePath should be set by store layer")
	}
}

func TestConfigDefaults(t *testing.T) {
	// Config itself has no validation logic; defaults are applied by the caller.
	// This test documents the expected default: IntentsDir = ".kron/intents".
	cfg := Config{}
	if cfg.IntentsDir != "" {
		t.Errorf("Config.IntentsDir default = %q, want empty (caller applies default)", cfg.IntentsDir)
	}

	// When set, it should round-trip.
	cfg.IntentsDir = ".kron/intents"
	if cfg.IntentsDir != ".kron/intents" {
		t.Errorf("Config.IntentsDir = %q, want %q", cfg.IntentsDir, ".kron/intents")
	}
}

func TestAnchor(t *testing.T) {
	a := Anchor{
		Slug:       "auth/jwt-sliding-window",
		FilePath:   "/home/user/project/cmd/auth/main.go",
		LineNumber: 42,
	}

	if a.Slug != "auth/jwt-sliding-window" {
		t.Errorf("Slug = %q, want %q", a.Slug, "auth/jwt-sliding-window")
	}
	if a.LineNumber != 42 {
		t.Errorf("LineNumber = %d, want 42", a.LineNumber)
	}
}
