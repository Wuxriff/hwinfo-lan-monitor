#Requires -Version 7
#Requires -RunAsAdministrator
# Opens the Dashboard port for devices on private (home) networks only.
param(
    [int]$Port = 8086,
    [switch]$Remove
)
$ErrorActionPreference = 'Stop'
$name = 'HWiNFO LAN Monitor'

Get-NetFirewallRule -DisplayName $name -ErrorAction SilentlyContinue | Remove-NetFirewallRule
if ($Remove) {
    Write-Host "Removed firewall rule '$name'"
    return
}
New-NetFirewallRule -DisplayName $name -Direction Inbound -Protocol TCP -LocalPort $Port -Action Allow -Profile Private | Out-Null
Write-Host "Allowed inbound TCP $Port on Private networks ('$name')"
