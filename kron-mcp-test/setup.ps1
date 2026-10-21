# Thin wrapper: the real logic lives in the tracked repo script so a wiped
# workspace can always be rebuilt from HEAD.
param([switch]$SkipFrontend, [switch]$Verify)
& (Join-Path (Resolve-Path (Join-Path $PSScriptRoot '..')).Path 'frontend\wails\scripts\seed-release-env.ps1') -SkipFrontend:$SkipFrontend -Verify:$Verify