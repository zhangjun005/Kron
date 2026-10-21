# Seeds the release-shaped integration test environment at <repo>/kron-mcp-test.
#
# Everything the release bundle would ship sits in ONE folder so the GUI, the
# CLI/MCP binary and projects.json all agree on the same paths:
#
#   kron-mcp-test/
#   ├── app/                     "release-like" runtime, launch from here
#   │   ├── Kron-GUI.exe         Wails GUI (raw build, double-click)
#   │   ├── kron.exe             CLI + serve-mcp + serve-lsp (one binary)
#   │   ├── projects.json        registry written by `kron register` (exe dir)
#   │   └── README.txt
#   ├── project/                 project under test (committed fixture)
#   └── ...                      (scripts / clients live beside these)
#
# Why the GUI is not named Kron.exe next to kron.exe: Windows' filesystem is
# case-insensitive, so "Kron.exe" and "kron.exe" cannot coexist in one folder.
# The CLI keeps the release name (docs/tools refer to `kron`); the GUI is
# Kron-GUI.exe. If you need the literal RFC layout (Kron/Kron.exe + kron.exe at
# the zip root) the registry would end up in two different directories, which
# is exactly what this workspace avoids.
#
# projects.json lives next to BOTH executables on purpose:
#   - the GUI bridge reads it from its own exe directory;
#   - `kron serve-mcp` without -CWD resolves the project from the registry next
#     to the CLI executable (store.ProjectsJSONPath).
#
# Usage:
#   powershell -NoProfile -File frontend/wails/scripts/seed-release-env.ps1
#   powershell -NoProfile -File ... -SkipFrontend      # reuse existing dist
#   powershell -NoProfile -File ... -Verify            # also run the MCP json probe
param(
    [string]$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..')).Path,
    [string]$WorkDir = '',
    [switch]$SkipFrontend,
    [switch]$Verify
)

$ErrorActionPreference = 'Stop'
if ($WorkDir -eq '') {
    $WorkDir = Join-Path $RepoRoot 'kron-mcp-test'
}
$App = Join-Path $WorkDir 'app'
$Project = Join-Path $WorkDir 'project'
$Fixture = Join-Path $RepoRoot 'kron-mcp-test\project'

if (-not (Test-Path $Fixture)) {
    throw "fixture not found: $Fixture (expected the committed kron-mcp-test/project)"
}
# Never let a stale dev override leak into the release-like env: the whole
# point is that the registry next to the executables is the one being used.
Remove-Item Env:\KRON_PROJECTS_JSON_PATH -ErrorAction SilentlyContinue

# ---- 1. project under test -------------------------------------------------
# The default release environment keeps the committed fixture at
# kron-mcp-test/project. A custom -WorkDir gets an isolated copy instead.
$sameProject = [IO.Path]::GetFullPath($Project).TrimEnd('\') -eq
    [IO.Path]::GetFullPath($Fixture).TrimEnd('\')
if (-not $sameProject) {
    if (Test-Path $Project) {
        Remove-Item $Project -Recurse -Force
    }
    Copy-Item $Fixture $Project -Recurse
}
foreach ($stale in @('app', 'install', 'bin', 'logs')) {
    $full = Join-Path $WorkDir $stale
    if (Test-Path $full) {
        Remove-Item $full -Recurse -Force
    }
}
New-Item -ItemType Directory -Path $App | Out-Null

# ---- 2. frontend bundle + GUI (raw Wails build) ----------------------------
if (-not $SkipFrontend) {
    Push-Location (Join-Path $RepoRoot 'frontend\wails\frontend')
    try {
        npm run build | Out-Null
    } finally {
        Pop-Location
    }
}
Push-Location (Join-Path $RepoRoot 'frontend\wails')
try {
    go build -tags "desktop production" -ldflags "-w -s -H windowsgui" `
        -o (Join-Path $App 'Kron-GUI.exe') .
} finally {
    Pop-Location
}

# ---- 3. CLI / MCP / LSP binary --------------------------------------------
Push-Location $RepoRoot
try {
    go build -o (Join-Path $App 'kron.exe') ./cmd/kron
} finally {
    Pop-Location
}

# ---- 4. registry via the real user workflow (kron register) ----------------
Push-Location $Project
try {
    & (Join-Path $App 'kron.exe') register | Out-Null
} finally {
    Pop-Location
}
$registryPath = Join-Path $App 'projects.json'
if (-not (Test-Path $registryPath)) {
    throw "kron register did not create $registryPath"
}
$registry = Get-Content $registryPath -Raw | ConvertFrom-Json
if ([string]::IsNullOrWhiteSpace($registry.current_project)) {
    throw "registry has no current_project: $registryPath"
}
if ((Resolve-Path $registry.current_project).Path -ne (Resolve-Path $Project).Path) {
    throw "registry current_project points at '$($registry.current_project)', expected '$Project'"
}

# ---- 5. release-like README.txt -------------------------------------------
$readme = @"
Kron test bundle (release-shaped)

1) Double-click Kron-GUI.exe (it finds kron.exe next to it).
2) kron.exe is the CLI + MCP/LSP server (kron serve-mcp / kron serve-lsp).
3) projects.json (next to both executables) selects the active project.
"@
Set-Content -Path (Join-Path $App 'README.txt') -Value $readme -Encoding ASCII

Write-Output "seeded release-like env: $App"
Write-Output "  Kron-GUI.exe  = Wails GUI"
Write-Output "  kron.exe      = CLI + serve-mcp + serve-lsp"
Write-Output "  projects.json = $registryPath"
Write-Output "  project       = $Project"

if ($Verify) {
    & (Join-Path $PSScriptRoot 'verify-mcp-registry.ps1') -AppDir $App -ProjectDir $Project
}
