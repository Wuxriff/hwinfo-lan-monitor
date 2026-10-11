#Requires -Version 7
# Builds the release executable into dist\hwinfo-lan-monitor.exe.
param([string]$Version = '0.1.0')
$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
# Optional untracked env.ps1 can point PATH at a local Go toolchain.
$envScript = Join-Path $PSScriptRoot 'env.ps1'
if (Test-Path $envScript) { . $envScript }
Push-Location $root
try {
    go vet ./...
    if ($LASTEXITCODE) { exit $LASTEXITCODE }
    go test ./...
    if ($LASTEXITCODE) { exit $LASTEXITCODE }

    Push-Location cmd\hwinfo-lan-monitor
    go-winres make --arch amd64 --product-version $Version --file-version $Version
    $code = $LASTEXITCODE
    Pop-Location
    if ($code) { exit $code }

    New-Item -ItemType Directory -Force dist | Out-Null
    go build -trimpath -ldflags '-H windowsgui -s -w' -o dist\hwinfo-lan-monitor.exe .\cmd\hwinfo-lan-monitor
    if ($LASTEXITCODE) { exit $LASTEXITCODE }
    $size = [math]::Round((Get-Item dist\hwinfo-lan-monitor.exe).Length / 1MB, 1)
    Write-Host "Built dist\hwinfo-lan-monitor.exe ($size MB), version $Version"
} finally {
    Pop-Location
}
