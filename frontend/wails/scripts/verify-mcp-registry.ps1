# Verifies the release-shaped environment: `kron serve-mcp` WITHOUT -CWD must
# resolve the active project from projects.json next to kron.exe
# (store.ProjectsJSONPath). The probe (scripts/mcp-probe, hand-rolled stdio
# client) runs the server from an unrelated temp directory, so a successful
# kron_list can only come from the adjacent registry.
#
# Negative control: rename projects.json away and require the same invocation
# to fail (fallback is os.Getwd(), which is not a Kron project).
#
# Usage (normally via seed-release-env.ps1 -Verify):
#   powershell -NoProfile -File frontend/wails/scripts/verify-mcp-registry.ps1 `
#       -AppDir <repo>/kron-mcp-test/app -ProjectDir <repo>/kron-mcp-test/project
param(
    [Parameter(Mandatory = $true)][string]$AppDir,
    [Parameter(Mandatory = $true)][string]$ProjectDir
)

$ErrorActionPreference = 'Stop'
$kron = Join-Path $AppDir 'kron.exe'
$registry = Join-Path $AppDir 'projects.json'
if (-not (Test-Path $kron)) { throw "missing CLI binary: $kron" }
if (-not (Test-Path $registry)) { throw "missing registry: $registry" }

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..')).Path
$probeDir = Join-Path $env:TEMP ('kron-probe-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $probeDir | Out-Null
$probe = Join-Path $probeDir 'mcp-probe.exe'
$probeSrc = Join-Path $RepoRoot 'frontend\wails\scripts\mcp-probe'

Push-Location (Join-Path $RepoRoot 'frontend\wails')
try {
    go build -o $probe ./scripts/mcp-probe
} finally {
    Pop-Location
}

$probeCwd = Join-Path $probeDir 'not-a-kron-project'
New-Item -ItemType Directory -Path $probeCwd | Out-Null

function Invoke-Probe {
    # PS 5.1 promotes native-command stderr to a terminating error when
    # ErrorActionPreference is Stop; the negative control deliberately expects
    # stderr from the server, so relax the preference for this call only.
    $previous = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        $out = & $probe -kron $kron -dir $probeCwd 2>&1
        $code = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $previous
    }
    return [pscustomobject]@{ExitCode = $code; Output = ($out | Out-String).Trim()}
}

$failures = @()

# ---- positive: registry next to kron.exe wins over the (invalid) cwd -------
$positive = Invoke-Probe
if ($positive.ExitCode -eq 0 -and $positive.Output -match 'intents=8') {
    Write-Output "PASS: serve-mcp (no -CWD, cwd=$probeCwd) resolved 8 intents from $registry"
} else {
    $failures += "positive probe failed: exit=$($positive.ExitCode) output=$($positive.Output)"
}

# ---- negative: without the registry the same invocation must fail ----------
$backup = "$registry.bak"
Move-Item $registry $backup
try {
    $negative = Invoke-Probe
} finally {
    Move-Item $backup $registry
}
if ($negative.ExitCode -ne 0) {
    Write-Output "PASS: without projects.json the same invocation failed as expected (exit=$($negative.ExitCode))"
} else {
    $failures += "negative control failed: server still returned results without the registry ($($negative.Output))"
}

Remove-Item $probeDir -Recurse -Force -ErrorAction SilentlyContinue

if ($failures.Count -gt 0) {
    $failures | ForEach-Object { Write-Output "FAIL: $_" }
    exit 1
}
Write-Output "PASS: projects.json next to kron.exe is the source of truth for serve-mcp"
