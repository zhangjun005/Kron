//go:build e2e

// Bridge end-to-end test for the Wails GUI.
//
// It drives the same App methods the React frontend calls, against a real
// `kron serve-mcp` subprocess and a real registry file. Run it from
// frontend/wails/ (the bridge resolves the CLI relative to the cwd):
//
//	$env:KRON_PROJECTS_JSON_PATH = "E:\works\Kron\frontend\wails\build\bin\projects.json"
//	go test -tags "desktop production e2e" -run TestAppBridgeE2E -v .
//
// The registry must point at a Kron project (kron-mcp-test/project is the
// fixture used during merge preparation).
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppBridgeE2E(t *testing.T) {
	if os.Getenv("KRON_PROJECTS_JSON_PATH") == "" {
		t.Skip("set KRON_PROJECTS_JSON_PATH to the GUI-adjacent projects.json")
	}
	app := NewApp()
	t.Cleanup(func() {
		app.mcpMu.Lock()
		defer app.mcpMu.Unlock()
		app.killLocked()
	})

	var current string
	t.Run("ListProjectsLive", func(t *testing.T) {
		res, err := app.ListProjectsLive()
		if err != nil {
			t.Fatalf("ListProjectsLive: %v", err)
		}
		if len(res.Projects) == 0 {
			t.Fatal("registry has no projects")
		}
		if res.Current == "" {
			t.Fatal("registry has no current_project")
		}
		current = res.Current
		t.Logf("registry=%s current=%s projects=%d", res.ProjectsPath, res.Current, len(res.Projects))
	})

	t.Run("ListMCPToolsLive", func(t *testing.T) {
		res, err := app.ListMCPToolsLive()
		if err != nil {
			t.Fatalf("ListMCPToolsLive: %v", err)
		}
		if res.ServerInfo.Name != "kron" {
			t.Errorf("server name = %q, want kron", res.ServerInfo.Name)
		}
		names := map[string]bool{}
		for _, tool := range res.Tools {
			names[tool.Name] = true
		}
		for _, want := range []string{
			"kron_tree", "kron_get", "kron_lint", "kron_impact",
			"kron_intent_density", "kron_stale", "kron_assume_check",
		} {
			if !names[want] {
				t.Errorf("missing tool %s (got %d tools)", want, len(res.Tools))
			}
		}
		if len(res.Tools) != 18 {
			t.Errorf("tool count = %d, want 18", len(res.Tools))
		}
		t.Logf("server=%s %s tools=%d", res.ServerInfo.Name, res.ServerInfo.Version, len(res.Tools))
	})

	t.Run("KronTree", func(t *testing.T) {
		raw, err := app.CallMCPTool("kron_tree", "{}")
		if err != nil {
			t.Fatalf("kron_tree: %v", err)
		}
		t.Logf("raw_tree=%.800s", raw)
		var out struct {
			Root struct {
				Name     string `json:"name"`
				Children []struct {
					Name     string `json:"name"`
					Children []struct {
						Slug string `json:"slug"`
					} `json:"children"`
				} `json:"children"`
			} `json:"root"`
		}
		if err := json.Unmarshal([]byte(raw), &out); err != nil {
			t.Fatalf("parse kron_tree: %v\nraw: %s", err, raw)
		}
		var groups []string
		for _, child := range out.Root.Children {
			groups = append(groups, child.Name)
			t.Logf("group %q children=%d", child.Name, len(child.Children))
		}
		for _, want := range []string{"auth", "payments"} {
			if !strings.Contains(strings.Join(groups, ","), want) {
				t.Errorf("tree groups %v missing %q", groups, want)
			}
		}
	})

	t.Run("KronGet", func(t *testing.T) {
		raw, err := app.CallMCPTool("kron_get", `{"slug":"auth/jwt"}`)
		if err != nil {
			t.Fatalf("kron_get: %v", err)
		}
		// The fixture intentionally carries the full frontmatter schema; the
		// GUI info card projects these fields, so assert they survive the wire.
		var out struct {
			Intent struct {
				Slug        string `json:"slug"`
				Frontmatter struct {
					Symbol      []string `json:"symbol"`
					CreatedBy   string   `json:"created_by"`
					UpdatedAt   string   `json:"updated_at"`
					Status      string   `json:"status"`
					Reviewers   []string `json:"reviewers"`
					References  []string `json:"references"`
					DependsOn   []string `json:"depends_on"`
					Assumptions []struct {
						ID        string `json:"id"`
						Severity  string `json:"severity"`
						Rationale string `json:"rationale"`
						ExpiresAt string `json:"expires_at"`
					} `json:"assumptions"`
				} `json:"frontmatter"`
			} `json:"intent"`
		}
		if err := json.Unmarshal([]byte(raw), &out); err != nil {
			t.Fatalf("parse kron_get: %v\nraw: %.400s", err, raw)
		}
		fm := out.Intent.Frontmatter
		if out.Intent.Slug != "auth/jwt" {
			t.Errorf("slug = %q, want auth/jwt", out.Intent.Slug)
		}
		for name, ok := range map[string]bool{
			"symbol":     len(fm.Symbol) > 0,
			"created_by": fm.CreatedBy != "",
			"updated_at": fm.UpdatedAt != "",
			"status":     fm.Status == "active",
			"reviewers":  len(fm.Reviewers) > 0,
			"references": len(fm.References) > 0,
		} {
			if !ok {
				t.Errorf("frontmatter field %q missing/incorrect: %+v", name, fm)
			}
		}
		if len(fm.Assumptions) == 0 || fm.Assumptions[0].ID != "single-region" {
			t.Errorf("assumptions = %+v, want [single-region ...]", fm.Assumptions)
		} else if fm.Assumptions[0].Severity == "" || fm.Assumptions[0].Rationale == "" {
			t.Errorf("assumption missing severity/rationale: %+v", fm.Assumptions[0])
		}
		t.Logf("frontmatter: symbols=%v reviewers=%v assumptions=%d references=%v",
			fm.Symbol, fm.Reviewers, len(fm.Assumptions), fm.References)
	})

	t.Run("KronLint", func(t *testing.T) {
		raw, err := app.CallMCPTool("kron_lint", "{}")
		if err != nil {
			t.Fatalf("kron_lint: %v", err)
		}
		var out struct {
			Summary struct {
				Errors   int `json:"errors"`
				Warnings int `json:"warnings"`
			} `json:"summary"`
		}
		if err := json.Unmarshal([]byte(raw), &out); err != nil {
			t.Fatalf("parse kron_lint: %v\nraw: %.300s", err, raw)
		}
		if out.Summary.Errors != 1 {
			t.Errorf("lint errors = %d, want 1 (src/broken/orphan.go dangling anchor)", out.Summary.Errors)
		}
		t.Logf("lint errors=%d warnings=%d", out.Summary.Errors, out.Summary.Warnings)
	})

	t.Run("SwitchProject", func(t *testing.T) {
		if current == "" {
			t.Skip("no current project from ListProjectsLive")
		}
		res, err := app.SwitchProject(current)
		if err != nil {
			t.Fatalf("SwitchProject: %v", err)
		}
		if res.Current == "" {
			t.Error("SwitchProject did not set current_project")
		}
		if len(res.Projects) == 0 || res.Projects[0].LastOpened == "" {
			t.Error("SwitchProject did not update last_opened")
		}
		t.Logf("switched to %s last_opened=%s", res.Current, res.Projects[0].LastOpened)
	})

	t.Run("OpenInIDENonExistentPath", func(t *testing.T) {
		// Negative path only: a positive call would launch the user's
		// editor (code/cursor), which is not something a test should do.
		res, err := app.OpenInIDE(filepath.Join(os.TempDir(), "kron-definitely-missing.md"))
		if err != nil {
			t.Fatalf("OpenInIDE returned a Go error instead of a result: %v", err)
		}
		if res.OK {
			t.Errorf("OpenInIDE reported OK for a missing file (ide=%q)", res.IDE)
		}
		if res.Err == "" {
			t.Error("OpenInIDE did not explain the failure")
		}
		t.Logf("expected failure: %s", res.Err)
	})

	// A double-clicked GUI starts with cwd = exe dir, so the CLI must be
	// discoverable from the executable's own directory too. Copy the CLI next
	// to the test binary, chdir somewhere unrelated, and resolve again.
	t.Run("ResolveFromExecutableDir", func(t *testing.T) {
		source, statErr := os.Stat("build/bin/kron-cli.exe")
		if statErr != nil {
			t.Skip("build/bin/kron-cli.exe not present; build it first")
		}
		exePath, exeErr := os.Executable()
		if exeErr != nil {
			t.Fatalf("os.Executable: %v", exeErr)
		}
		exeDir := filepath.Dir(exePath)
		dest := filepath.Join(exeDir, "kron-cli.exe")
		if _, err := os.Stat(dest); err == nil {
			t.Skipf("%s already exists; refusing to overwrite", dest)
		}
		content, readErr := os.ReadFile("build/bin/kron-cli.exe")
		if readErr != nil {
			t.Fatalf("read CLI: %v", readErr)
		}
		if writeErr := os.WriteFile(dest, content, source.Mode()); writeErr != nil {
			t.Skipf("cannot place CLI next to test binary: %v", writeErr)
		}
		t.Cleanup(func() { _ = os.Remove(dest) })

		previous, wdErr := os.Getwd()
		if wdErr != nil {
			t.Fatalf("getwd: %v", wdErr)
		}
		if chdirErr := os.Chdir(t.TempDir()); chdirErr != nil {
			t.Fatalf("chdir: %v", chdirErr)
		}
		t.Cleanup(func() { _ = os.Chdir(previous) })

		found, findErr := resolveServeMCPExecutable()
		if findErr != nil {
			t.Fatalf("resolve from unrelated cwd: %v", findErr)
		}
		if filepath.Clean(found) != filepath.Clean(dest) {
			t.Errorf("resolved %q, want %q", found, dest)
		}
	})
}
