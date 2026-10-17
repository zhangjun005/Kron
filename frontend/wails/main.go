// Kron Wails GUI entry point. See app.go for the bridge surface and
// docs/rfc/2026-10-09-gui-ide-plan.md §6 for placement.
package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		// Title shown in the OS window chrome.
		Title:     "Kron",
		Width:     1280,
		Height:    800,
		MinWidth:  900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		// Kron brand background — neutral dark surface used while the
		// React app boots. Final theming lives in CSS variables; this is
		// just the pre-paint colour.
		BackgroundColour: &options.RGBA{R: 24, G: 24, B: 27, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Kron failed to start:", err.Error())
	}
}
