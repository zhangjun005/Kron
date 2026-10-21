#!/usr/bin/env bash
# Thin wrapper for git-bash / WSL: the real logic is the tracked repo script.
set -euo pipefail
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$here/.." && pwd)"
# Convert to a Windows path when running under git-bash/WSL; fall back to the
# plain path when cygpath is unavailable.
script="$repo_root/frontend/wails/scripts/seed-release-env.ps1"
if command -v cygpath >/dev/null 2>&1; then
    script="$(cygpath -w "$script")"
fi
exec powershell.exe -NoProfile -File "$script" "$@"
