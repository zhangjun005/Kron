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

func TestSeverity(t *testing.T) {
	tests := []struct {
		name string
		give Severity
		want string
	}{
		{name: "hard", give: SeverityHard, want: "hard"},
		{name: "soft", give: SeveritySoft, want: "soft"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.give) != tt.want {
				t.Errorf("Severity = %q, want %q", tt.give, tt.want)
			}
		})
	}
}

func TestAssumption(t *testing.T) {
	a := Assumption{
		ID:         "single-region",
		Text:       "服务仅部署在单 region，无跨区时钟漂移问题",
		Severity:   SeverityHard,
		ExpiresAt:  "2026-12-31",
		VerifiedAt: "2026-09-22",
		VerifiedBy: "@zhangjun005",
	}

	if a.ID != "single-region" {
		t.Errorf("ID = %q, want %q", a.ID, "single-region")
	}
	if a.Severity != SeverityHard {
		t.Errorf("Severity = %v, want %v", a.Severity, SeverityHard)
	}
	if a.ExpiresAt != "2026-12-31" {
		t.Errorf("ExpiresAt = %q, want %q", a.ExpiresAt, "2026-12-31")
	}
}

func TestAssumptionFrontmatter(t *testing.T) {
	fm := AssumptionFrontmatter{
		ID:              "single-region",
		Text:            "服务仅部署在单 region",
		DefaultSeverity: SeverityHard,
		CreatedBy:       "@zhangjun005",
		UpdatedAt:       "2026-09-22T10:00:00Z",
		Reviewers:       []string{"@alice"},
	}

	if fm.ID != "single-region" {
		t.Errorf("ID = %q, want %q", fm.ID, "single-region")
	}
	if fm.CreatedBy != "@zhangjun005" {
		t.Errorf("CreatedBy = %q, want %q", fm.CreatedBy, "@zhangjun005")
	}
	if len(fm.Reviewers) != 1 || fm.Reviewers[0] != "@alice" {
		t.Errorf("Reviewers = %v, want [@alice]", fm.Reviewers)
	}
}

func TestAssumptionFile(t *testing.T) {
	af := AssumptionFile{
		Slug: "single-region",
		Frontmatter: AssumptionFrontmatter{
			ID:              "single-region",
			Text:            "服务仅部署在单 region",
			DefaultSeverity: SeverityHard,
			CreatedBy:       "@zhangjun005",
			UpdatedAt:       "2026-09-22T10:00:00Z",
		},
		Body: "# Single Region\n\n> 服务仅部署在单 region。",
	}

	if af.Slug != af.Frontmatter.ID {
		t.Errorf("Slug (%q) must equal Frontmatter.ID (%q)", af.Slug, af.Frontmatter.ID)
	}
	if af.Body == "" {
		t.Error("Body should not be empty")
	}
}

func TestFrontmatterWithAssumptions(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	fm := Frontmatter{
		Symbol:    []string{"auth.RefreshToken"},
		CreatedBy: "@zhangjun005",
		UpdatedAt: now,
		Reviewers: []string{"@alice"},
		Status:    StatusActive,
		Assumptions: []Assumption{
			{
				ID:       "single-region",
				Text:     "服务仅部署在单 region",
				Severity: SeverityHard,
			},
			{
				ID:       "redis-availability",
				Text:     "Redis 可用性 ≥ 99.9%",
				Severity: SeveritySoft,
			},
		},
	}

	if len(fm.Assumptions) != 2 {
		t.Errorf("Assumptions count = %d, want 2", len(fm.Assumptions))
	}
	if fm.Assumptions[0].ID != "single-region" {
		t.Errorf("Assumptions[0].ID = %q, want %q", fm.Assumptions[0].ID, "single-region")
	}
	if fm.Assumptions[1].Severity != SeveritySoft {
		t.Errorf("Assumptions[1].Severity = %v, want %v", fm.Assumptions[1].Severity, SeveritySoft)
	}
}

func TestFrontmatterAssumptionsRefOnly(t *testing.T) {
	// 方案 B 下，intent 内联的 assumptions[] 只保留 id。
	// 其他字段从 .kron/assumptions/<id>.md 注册表读取。
	fm := Frontmatter{
		CreatedBy: "@zhangjun005",
		UpdatedAt: time.Now().UTC(),
		Assumptions: []Assumption{
			{ID: "single-region"},
			{ID: "csrf-protected"},
		},
	}

	if len(fm.Assumptions) != 2 {
		t.Fatalf("want 2 assumption refs, got %d", len(fm.Assumptions))
	}
	for i, a := range fm.Assumptions {
		if a.ID == "" {
			t.Errorf("Assumptions[%d].ID must not be empty", i)
		}
		// 方案 B 下，Text/Severity 在注册表里，frontmatter 内联可省略
		// (lint 会从注册表解析)
	}
}
