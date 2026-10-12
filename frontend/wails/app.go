// Package main is the Wails GUI entry point for the Kron desktop client.
//
// Per docs/rfc/2026-10-09-gui-ide-plan.md §3.1 / §3.2 / §6:
//   - The Wails Go side exposes a small read-only surface to the React frontend.
//   - All real Kron data flows through a stdio `kron serve-mcp` subprocess
//     (the GUI does NOT import internal/ packages directly — iron rule #9 + #10).
//   - For work 1.3 (skeleton verification), the Go side only returns static
//     metadata. MCP tool plumbing lands in a later work item.
package main

import (
	"context"
	"fmt"
	"runtime"
)

// App is the Wails application struct. Its exported methods are the Go ↔ TS
// bridge surface — every method is callable from React as a Promise<...>.
// Per iron rule #9: this struct must never import anything from internal/.
type App struct {
	ctx context.Context
}

// NewApp creates a new App application struct.
func NewApp() *App {
	return &App{}
}

// startup is called by Wails after the Webview2/WebKit process is ready.
// The context is saved so we can use it for runtime APIs later (logging,
// event emission, dialogs). We do not use it for anything in work 1.3.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// KronInfo is the static metadata payload returned by GetKronInfo.
// Version, BuildTime, and GoVersion are computed at build time via -ldflags
// in a later work item; for now they are hard-coded so the TS bridge can be
// exercised end-to-end.
type KronInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	BuildTime   string `json:"buildTime"`
	GoVersion   string `json:"goVersion"`
	OS          string `json:"os"`
	Arch        string `json:"arch"`
	Description string `json:"description"`
	RFC         string `json:"rfc"`
}

// GetKronInfo returns static metadata about the Kron desktop client.
// This is the only bridge method exposed in work 1.3; it touches no
// internal/* package and performs no I/O.
func (a *App) GetKronInfo() KronInfo {
	return KronInfo{
		Name:        "Kron",
		Version:     "0.0.0-skeleton",
		BuildTime:   "unreleased",
		GoVersion:   runtime.Version(),
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
		Description: "Kron desktop client — Wails v2 GUI for the Kron intent ledger.",
		RFC:         "docs/rfc/2026-10-09-gui-ide-plan.md",
	}
}

// ListMCPTools returns the list of MCP tool names the GUI is expected to
// expose in the overview panel. In work 1.3 this is hard-coded; the real
// implementation will spawn `kron serve-mcp` and call tools/list.
func (a *App) ListMCPTools() []string {
	return []string{
		"kron_list",
		"kron_get",
		"kron_impact",
		"kron_density",
		"kron_relations",
		"kron_stale",
		"kron_assume_check",
		"kron_lint",
		"kron_tree",
		"kron_add",
		"kron_update",
		"kron_delete",
		"kron_restore",
	}
}

// String returns a human-readable summary, useful for debugging.
func (k KronInfo) String() string {
	return fmt.Sprintf("%s v%s (%s/%s, Go %s)", k.Name, k.Version, k.OS, k.Arch, k.GoVersion)
}
