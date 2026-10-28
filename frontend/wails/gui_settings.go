package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	guiSettingsSchemaVersion = 1
	guiThemeSystem           = "system"
	guiThemeLight            = "light"
	guiThemeDark             = "dark"
)

// GUISettings contains persistent preferences for the Wails client.
// Project registration remains in projects.json; this file is only for GUI
// presentation preferences that are safe to carry with a portable install.
type GUISettings struct {
	SchemaVersion int                           `json:"schema_version"`
	Theme         string                        `json:"theme"`
	Projects      map[string]GUIProjectSettings `json:"projects,omitempty"`
}

// GUIProjectSettings contains presentation metadata for one registered project.
type GUIProjectSettings struct {
	DisplayName string `json:"display_name,omitempty"`
	CoverImage  string `json:"cover_image,omitempty"`
}

func guiSettingsPath() (string, error) {
	if override := os.Getenv("KRON_GUI_SETTINGS_PATH"); override != "" {
		path, err := filepath.Abs(override)
		if err != nil {
			return "", fmt.Errorf("wails: resolve KRON_GUI_SETTINGS_PATH: %w", err)
		}
		return path, nil
	}
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("wails: locate GUI executable for guiSettings.json: %w", err)
	}
	return filepath.Join(filepath.Dir(executable), "guiSettings.json"), nil
}

func defaultGUISettings() GUISettings {
	return GUISettings{SchemaVersion: guiSettingsSchemaVersion, Theme: guiThemeSystem, Projects: make(map[string]GUIProjectSettings)}
}

func validateGUITheme(theme string) error {
	switch theme {
	case guiThemeSystem, guiThemeLight, guiThemeDark:
		return nil
	default:
		return fmt.Errorf("wails: unsupported GUI theme %q (expected system, light, or dark)", theme)
	}
}

// GetGUISettings reads guiSettings.json beside the GUI executable.
func (a *App) GetGUISettings() (GUISettings, error) {
	path, err := guiSettingsPath()
	if err != nil {
		return GUISettings{}, err
	}
	settings, _, err := readGUISettings(path)
	return settings, err
}

func readGUISettings(path string) (GUISettings, map[string]json.RawMessage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return defaultGUISettings(), make(map[string]json.RawMessage), nil
		}
		return GUISettings{}, nil, fmt.Errorf("wails: read guiSettings.json %q: %w", path, err)
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return GUISettings{}, nil, fmt.Errorf("wails: parse guiSettings.json %q: %w", path, err)
	}
	if fields == nil {
		fields = make(map[string]json.RawMessage)
	}
	var settings GUISettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return GUISettings{}, nil, fmt.Errorf("wails: parse guiSettings.json %q: %w", path, err)
	}
	if settings.SchemaVersion == 0 {
		settings.SchemaVersion = guiSettingsSchemaVersion
	}
	if settings.Theme == "" {
		settings.Theme = guiThemeSystem
	}
	if settings.Projects == nil {
		settings.Projects = make(map[string]GUIProjectSettings)
	}
	if err := validateGUITheme(settings.Theme); err != nil {
		return GUISettings{}, nil, err
	}
	return settings, fields, nil
}

// SetGUITheme validates and atomically persists the Wails theme preference.
func (a *App) SetGUITheme(theme string) (GUISettings, error) {
	if err := validateGUITheme(theme); err != nil {
		return GUISettings{}, err
	}
	path, err := guiSettingsPath()
	if err != nil {
		return GUISettings{}, err
	}
	settings, fields, err := readGUISettings(path)
	if err != nil {
		return GUISettings{}, err
	}
	settings.SchemaVersion = guiSettingsSchemaVersion
	settings.Theme = theme
	if settings.Projects == nil {
		settings.Projects = make(map[string]GUIProjectSettings)
	}
	schemaVersionJSON, err := json.Marshal(settings.SchemaVersion)
	if err != nil {
		return GUISettings{}, fmt.Errorf("wails: encode guiSettings schema version: %w", err)
	}
	themeJSON, err := json.Marshal(settings.Theme)
	if err != nil {
		return GUISettings{}, fmt.Errorf("wails: encode guiSettings theme: %w", err)
	}
	fields["schema_version"] = schemaVersionJSON
	fields["theme"] = themeJSON
	projectsJSON, err := json.Marshal(settings.Projects)
	if err != nil {
		return GUISettings{}, fmt.Errorf("wails: encode guiSettings projects: %w", err)
	}
	fields["projects"] = projectsJSON
	data, err := json.MarshalIndent(fields, "", "  ")
	if err != nil {
		return GUISettings{}, fmt.Errorf("wails: encode guiSettings.json: %w", err)
	}
	data = append(data, '\n')
	if err := writeGUISettingsAtomic(path, data); err != nil {
		return GUISettings{}, err
	}
	return settings, nil
}

// SetGUIProject updates the display metadata for a registered project.
func (a *App) SetGUIProject(projectPath string, displayName string) (GUISettings, error) {
	settings, err := a.GetGUISettings()
	if err != nil {
		return GUISettings{}, err
	}
	abs, err := filepath.Abs(projectPath)
	if err != nil {
		return GUISettings{}, fmt.Errorf("wails: resolve GUI project path: %w", err)
	}
	if settings.Projects == nil {
		settings.Projects = make(map[string]GUIProjectSettings)
	}
	metadata := settings.Projects[abs]
	metadata.DisplayName = strings.TrimSpace(displayName)
	settings.Projects[abs] = metadata
	return a.writeGUISettings(settings)
}

// ChooseProjectCover opens a native file picker and stores a portable copy of
// the selected image beside the GUI executable.
func (a *App) ChooseProjectCover(projectPath string) (GUISettings, error) {
	if a.ctx == nil {
		return GUISettings{}, fmt.Errorf("wails: GUI runtime is not ready")
	}
	selected, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{Title: "Choose project cover image"})
	if err != nil {
		return GUISettings{}, fmt.Errorf("wails: choose project cover: %w", err)
	}
	if selected == "" {
		return a.GetGUISettings()
	}
	data, err := os.ReadFile(selected)
	if err != nil {
		return GUISettings{}, fmt.Errorf("wails: read project cover %q: %w", selected, err)
	}
	executable, err := os.Executable()
	if err != nil {
		return GUISettings{}, fmt.Errorf("wails: locate GUI executable for project cover: %w", err)
	}
	coverDir := filepath.Join(filepath.Dir(executable), "covers")
	if err := os.MkdirAll(coverDir, 0o755); err != nil {
		return GUISettings{}, fmt.Errorf("wails: create project covers directory: %w", err)
	}
	abs, err := filepath.Abs(projectPath)
	if err != nil {
		return GUISettings{}, fmt.Errorf("wails: resolve GUI project path: %w", err)
	}
	digest := sha256.Sum256([]byte(abs))
	ext := filepath.Ext(selected)
	target := filepath.Join(coverDir, fmt.Sprintf("%x%s", digest[:8], ext))
	if err := os.WriteFile(target, data, 0o644); err != nil {
		return GUISettings{}, fmt.Errorf("wails: write project cover %q: %w", target, err)
	}
	settings, err := a.GetGUISettings()
	if err != nil {
		return GUISettings{}, err
	}
	if settings.Projects == nil {
		settings.Projects = make(map[string]GUIProjectSettings)
	}
	metadata := settings.Projects[abs]
	metadata.CoverImage = filepath.ToSlash(filepath.Join("covers", filepath.Base(target)))
	settings.Projects[abs] = metadata
	return a.writeGUISettings(settings)
}

// GetProjectCoverData returns a selected project cover as a data URL for the
// WebView. The image itself remains outside guiSettings.json.
func (a *App) GetProjectCoverData(projectPath string) (string, error) {
	settings, err := a.GetGUISettings()
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(projectPath)
	if err != nil {
		return "", fmt.Errorf("wails: resolve GUI project path: %w", err)
	}
	metadata, ok := settings.Projects[abs]
	if !ok || metadata.CoverImage == "" {
		return "", nil
	}
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("wails: locate GUI executable for project cover: %w", err)
	}
	coverPath := filepath.Join(filepath.Dir(executable), filepath.FromSlash(metadata.CoverImage))
	data, err := os.ReadFile(coverPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("wails: read project cover %q: %w", coverPath, err)
	}
	contentType := mime.TypeByExtension(filepath.Ext(coverPath))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

// AddProject opens the native folder picker and adds the selected directory to
// the registry. Uninitialised folders receive the existing idempotent `kron
// init` skeleton; existing Kron projects are registered directly.
func (a *App) AddProject() (ListProjectsLiveResult, error) {
	if a.ctx == nil {
		return ListProjectsLiveResult{}, fmt.Errorf("wails: GUI runtime is not ready")
	}
	selected, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{Title: "Add Kron project"})
	if err != nil {
		return ListProjectsLiveResult{}, fmt.Errorf("wails: choose project folder: %w", err)
	}
	if selected == "" {
		return a.ListProjectsLive()
	}
	root, err := filepath.Abs(selected)
	if err != nil {
		return ListProjectsLiveResult{}, fmt.Errorf("wails: resolve selected project folder: %w", err)
	}
	command, err := a.resolveCLI()
	if err != nil {
		return ListProjectsLiveResult{}, err
	}
	action := "register"
	if !isKronProject(root) {
		action = "init"
	}
	cmd := exec.Command(command, action, "-CWD", root)
	cmd.SysProcAttr = hiddenProcAttr()
	if output, runErr := cmd.CombinedOutput(); runErr != nil {
		return ListProjectsLiveResult{}, fmt.Errorf("wails: kron %s %q failed: %w (%s)", action, root, runErr, strings.TrimSpace(string(output)))
	}
	return a.ListProjectsLive()
}

// OpenProjectFolder opens the current project in the platform file manager.
func (a *App) OpenProjectFolder(projectPath string) error {
	root, err := filepath.Abs(projectPath)
	if err != nil {
		return fmt.Errorf("wails: resolve project folder: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return fmt.Errorf("wails: inspect project folder %q: %w", root, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("wails: project path %q is not a directory", root)
	}
	var command string
	var args []string
	var procAttr *syscall.SysProcAttr
	switch runtime.GOOS {
	case "windows":
		command = "explorer.exe"
		args = []string{"/root,", root}
		// explorer.exe is a shell process, not a console child. Applying
		// CREATE_NO_WINDOW can make it return without opening a window.
		procAttr = nil
	case "darwin":
		command = "open"
		args = []string{root}
		procAttr = hiddenProcAttr()
	default:
		command = "xdg-open"
		args = []string{root}
		procAttr = hiddenProcAttr()
	}
	process := exec.Command(command, args...)
	process.SysProcAttr = procAttr
	if err := process.Start(); err != nil {
		return fmt.Errorf("wails: open project folder %q with %s: %w", root, command, err)
	}
	return nil
}

func (a *App) writeGUISettings(settings GUISettings) (GUISettings, error) {
	path, err := guiSettingsPath()
	if err != nil {
		return GUISettings{}, err
	}
	settings.SchemaVersion = guiSettingsSchemaVersion
	if settings.Projects == nil {
		settings.Projects = make(map[string]GUIProjectSettings)
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return GUISettings{}, fmt.Errorf("wails: encode guiSettings.json: %w", err)
	}
	data = append(data, '\n')
	if err := writeGUISettingsAtomic(path, data); err != nil {
		return GUISettings{}, err
	}
	return settings, nil
}

func writeGUISettingsAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".guiSettings-*.tmp")
	if err != nil {
		return fmt.Errorf("wails: create guiSettings.json temporary file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("wails: write guiSettings.json temporary file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("wails: sync guiSettings.json temporary file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("wails: close guiSettings.json temporary file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("wails: replace guiSettings.json: %w", err)
	}
	return nil
}
