# Prints the change in kron-mcp-test/project since the previous call and
# refreshes the baseline. Used while exercising the kron MCP tools one by one.
#
#   powershell -NoProfile -File kron-mcp-test/check-project-diff.ps1 -Init      # baseline
#   powershell -NoProfile -File kron-mcp-test/check-project-diff.ps1             # diff vs baseline
param([switch]$Init)

$root = 'E:\works\Kron\kron-mcp-test\project'
$baseline = Join-Path $env:TEMP 'kron-project-manifest.json'

function Get-Manifest {
    $map = @{}
    Get-ChildItem $root -Recurse -File -Force | ForEach-Object {
        $rel = $_.FullName.Substring($root.Length + 1)
        $map[$rel] = (Get-FileHash $_.FullName -Algorithm SHA256).Hash
    }
    return $map
}

$current = Get-Manifest
if ($Init -or -not (Test-Path $baseline)) {
    $current | ConvertTo-Json -Depth 3 | Set-Content $baseline -Encoding UTF8
    Write-Output ("baseline: {0} files" -f $current.Count)
    exit 0
}

# Store as a JSON object (path -> hash). PS 5.1's ConvertFrom-Json does NOT
# unroll a top-level array when fed through the pipeline, so an array baseline
# would compare as one opaque object.
$previous = @{}
$previousJson = Get-Content $baseline -Raw | ConvertFrom-Json
foreach ($prop in $previousJson.PSObject.Properties) {
    $previous[$prop.Name] = $prop.Value
}

$added = @($current.Keys | Where-Object { -not $previous.ContainsKey($_) } | Sort-Object)
$removed = @($previous.Keys | Where-Object { -not $current.ContainsKey($_) } | Sort-Object)
$modified = @($current.Keys | Where-Object { $previous.ContainsKey($_) -and $previous[$_] -ne $current[$_] } | Sort-Object)

if ($added.Count -eq 0 -and $removed.Count -eq 0 -and $modified.Count -eq 0) {
    Write-Output 'project diff: (no changes)'
} else {
    Write-Output 'project diff:'
    foreach ($p in $added) { Write-Output "  + $p" }
    foreach ($p in $modified) { Write-Output "  ~ $p" }
    foreach ($p in $removed) { Write-Output "  - $p" }
}

$current | ConvertTo-Json -Depth 3 | Set-Content $baseline -Encoding UTF8
