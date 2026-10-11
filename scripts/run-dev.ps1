#Requires -Version 7
param([string]$Addr = ':8086')
$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
# Optional untracked env.ps1 can point PATH at a local Go toolchain.
$envScript = Join-Path $PSScriptRoot 'env.ps1'
if (Test-Path $envScript) { . $envScript }
Push-Location $root
try {
    go build -o bin\hwinfo-lan-monitor.exe .\cmd\hwinfo-lan-monitor
    if ($LASTEXITCODE) { exit $LASTEXITCODE }
    & .\bin\hwinfo-lan-monitor.exe -headless -addr $Addr
} finally {
    Pop-Location
}
