package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGUISettingsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "guiSettings.json")
	t.Setenv("KRON_GUI_SETTINGS_PATH", path)

	app := NewApp()
	settings, err := app.GetGUISettings()
	if err != nil {
		t.Fatalf("GetGUISettings missing file: %v", err)
	}
	if settings.Theme != guiThemeSystem {
		t.Fatalf("default theme = %q, want %q", settings.Theme, guiThemeSystem)
	}

	settings, err = app.SetGUITheme(guiThemeLight)
	if err != nil {
		t.Fatalf("SetGUITheme: %v", err)
	}
	if settings.Theme != guiThemeLight {
		t.Fatalf("saved theme = %q, want %q", settings.Theme, guiThemeLight)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read settings file: %v", err)
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatalf("parse settings file: %v", err)
	}
	var theme string
	if err := json.Unmarshal(wire["theme"], &theme); err != nil {
		t.Fatalf("decode wire theme: %v", err)
	}
	if theme != guiThemeLight {
		t.Fatalf("wire theme = %q, want %q", theme, guiThemeLight)
	}
}

func TestSetGUIThemeRejectsUnknownTheme(t *testing.T) {
	app := NewApp()
	if _, err := app.SetGUITheme("neon"); err == nil {
		t.Fatal("SetGUITheme accepted unsupported theme")
	}
}

func TestGUISettingsAcceptsUTF8BOM(t *testing.T) {
	path := filepath.Join(t.TempDir(), "guiSettings.json")
	if err := os.WriteFile(path, append([]byte{0xEF, 0xBB, 0xBF}, []byte(`{"schema_version":1,"theme":"dark"}`)...), 0o644); err != nil {
		t.Fatalf("write BOM settings: %v", err)
	}
	t.Setenv("KRON_GUI_SETTINGS_PATH", path)
	settings, err := NewApp().GetGUISettings()
	if err != nil {
		t.Fatalf("GetGUISettings with BOM: %v", err)
	}
	if settings.Theme != guiThemeDark {
		t.Fatalf("theme = %q, want %q", settings.Theme, guiThemeDark)
	}
}
