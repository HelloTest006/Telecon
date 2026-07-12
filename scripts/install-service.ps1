# LEGACY / DEMO ONLY — Windows Service as LocalSystem.
# DPAPI user-protected keys will NOT open under LocalSystem.
# Prefer: .\scripts\install-agent.ps1 (logon task in user context).
#
# Usage (Admin):
#   .\scripts\install-service.ps1 -DeviceId dev-a ...
param(
  [Parameter(Mandatory = $true)][string]$DeviceId,
  [string]$BinDir = (Resolve-Path "$PSScriptRoot\..\bin").Path,
  [string]$DataDir = (Resolve-Path "$PSScriptRoot\..\data").Path,
  [string]$Listen = "0.0.0.0:9001",
  [string]$Api = "127.0.0.1:7701",
  [string]$ApiToken = "change-me",
  [string]$KaUrl = "https://127.0.0.1:8443",
  [string]$KaCa = "",
  [string]$ServiceName = "",
  [switch]$IUnderstandDpapiBreaks
)

$ErrorActionPreference = "Stop"
if (-not $IUnderstandDpapiBreaks) {
  throw @"
REFUSED: install-service.ps1 runs as LocalSystem; user DPAPI identity keys will fail.

Use instead (recommended):
  .\scripts\install-agent.ps1 -DeviceId $DeviceId -StartNow

Only if you really want a machine service (e.g. lab without DPAPI), re-run with:
  -IUnderstandDpapiBreaks
"@
}

if (-not $ServiceName) { $ServiceName = "COENode-$DeviceId" }
if (-not $KaCa) { $KaCa = Join-Path $DataDir "ka\tls\server.crt" }

$exe = Join-Path $BinDir "coe-node.exe"
if (-not (Test-Path $exe)) {
  throw "missing $exe — build first"
}

$identity = Join-Path $DataDir "devices\$DeviceId\identity.json"
$store = Join-Path $DataDir "devices\$DeviceId\keys"
if (-not (Test-Path $identity)) {
  throw "missing identity $identity"
}

$binPath = "`"$exe`" -service -device-id $DeviceId -identity `"$identity`" -store `"$store`" -listen $Listen -api $Api -api-token $ApiToken -ka $KaUrl -ka-ca `"$KaCa`""

$existing = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
if ($existing) {
  Stop-Service $ServiceName -Force -ErrorAction SilentlyContinue
  sc.exe delete $ServiceName | Out-Null
  Start-Sleep 1
}

Write-Host "WARNING: LocalSystem service — DPAPI user keys will not work"
sc.exe create $ServiceName binPath= $binPath start= auto DisplayName= "COE Node ($DeviceId)" | Out-Host
sc.exe description $ServiceName "COE agent (NOT DPAPI-safe under LocalSystem)" | Out-Null
Start-Service $ServiceName
Get-Service $ServiceName | Format-List Name, Status, StartType
