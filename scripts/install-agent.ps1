# Install coe-node as a per-user logon agent (Task Scheduler).
# Runs in the logged-on user context so Windows DPAPI can open identity keys.
# No admin required for the scheduled task itself.
#
# Usage:
#   .\scripts\install-agent.ps1 -DeviceId alice -KaUrl https://ka.example.com -KaCa C:\path\ka.crt
#   .\scripts\install-agent.ps1 -DeviceId alice -Enroll -AdminToken secret
param(
  [Parameter(Mandatory = $true)][string]$DeviceId,
  [string]$InstallRoot = "",
  [string]$SourceBin = "",
  [string]$Listen = "0.0.0.0:9001",
  [string]$Api = "127.0.0.1:7701",
  [string]$ApiToken = "",
  [string]$KaUrl = "https://127.0.0.1:8443",
  [string]$KaCa = "",
  [string]$AdminToken = "",
  [string]$Voucher = "",
  [switch]$Enroll,
  [switch]$StartNow,
  [switch]$SkipKeygen
)

$ErrorActionPreference = "Stop"

if (-not $InstallRoot) {
  $InstallRoot = Join-Path $env:LOCALAPPDATA "COE"
}
if (-not $SourceBin) {
  $cand = Join-Path (Resolve-Path "$PSScriptRoot\..").Path "bin"
  if (Test-Path (Join-Path $cand "coe-node.exe")) {
    $SourceBin = $cand
  } elseif (Test-Path (Join-Path $PSScriptRoot "..\bin\coe-node.exe")) {
    $SourceBin = (Resolve-Path "$PSScriptRoot\..\bin").Path
  } elseif (Test-Path (Join-Path $PSScriptRoot "coe-node.exe")) {
    $SourceBin = $PSScriptRoot
  } else {
    $SourceBin = Join-Path (Resolve-Path "$PSScriptRoot\..").Path "bin"
  }
}

$BinDir = Join-Path $InstallRoot "bin"
$DevDir = Join-Path $InstallRoot "devices\$DeviceId"
$LogDir = Join-Path $InstallRoot "logs"
$CfgPath = Join-Path $DevDir "node.json"
$Identity = Join-Path $DevDir "identity.json"
$Store = Join-Path $DevDir "keys"
$TaskName = "COE-Agent-$DeviceId"

New-Item -ItemType Directory -Force -Path $BinDir, $DevDir, $Store, $LogDir | Out-Null

function Copy-Bin([string]$name) {
  $src = Join-Path $SourceBin $name
  if (-not (Test-Path $src)) { throw "missing binary: $src — run scripts\package.ps1 or go build" }
  Copy-Item $src (Join-Path $BinDir $name) -Force
}

Write-Host "Installing into $InstallRoot"
Copy-Bin "coe-node.exe"
Copy-Bin "coe-keygen.exe"
Copy-Bin "coe-cli.exe"
if (Test-Path (Join-Path $SourceBin "ka-check.exe")) {
  Copy-Bin "ka-check.exe"
}

if (-not $ApiToken) {
  $bytes = New-Object byte[] 16
  [System.Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
  $ApiToken = ([BitConverter]::ToString($bytes) -replace "-", "").ToLowerInvariant()
  Write-Host "Generated API token (save it): $ApiToken"
}

if (-not $KaCa) {
  # Prefer packaged CA next to install, else repo dev cert
  $guess = @(
    (Join-Path $InstallRoot "ka-ca.crt"),
    (Join-Path $env:LOCALAPPDATA "COE\ka-ca.crt"),
    (Join-Path (Resolve-Path "$PSScriptRoot\..").Path "data\ka\tls\server.crt")
  ) | Where-Object { Test-Path $_ } | Select-Object -First 1
  if ($guess) { $KaCa = $guess }
}

if (-not $SkipKeygen -and -not (Test-Path $Identity)) {
  Write-Host "Generating identity (DPAPI-protected)..."
  & (Join-Path $BinDir "coe-keygen.exe") -device-id $DeviceId -out $Identity
  if ($LASTEXITCODE -ne 0) { throw "keygen failed" }
}

$cfg = [ordered]@{
  device_id     = $DeviceId
  identity_path = $Identity
  store_dir     = $Store
  ka_url        = $KaUrl
  ka_ca_file    = $KaCa
  ka_insecure   = $false
  listen_addr   = $Listen
  api_addr      = $Api
  api_token     = $ApiToken
  peers         = @()
  profile       = "strong"
  admin_token   = $AdminToken
}
$cfg | ConvertTo-Json -Depth 5 | Set-Content -Path $CfgPath -Encoding UTF8
Write-Host "Wrote config $CfgPath"

if ($Enroll) {
  if (-not $Voucher -and -not $AdminToken) {
    throw "Enroll requires -Voucher <code> (preferred) or -AdminToken"
  }
  Write-Host "Enrolling with KA $KaUrl ..."
  $enArgs = @("-config", $CfgPath, "-enroll")
  if ($Voucher) {
    $enArgs += @("-voucher", $Voucher)
  }
  if ($AdminToken -and -not $Voucher) {
    $enArgs += @("-admin-token", $AdminToken)
  }
  & (Join-Path $BinDir "coe-node.exe") @enArgs
  if ($LASTEXITCODE -ne 0) { throw "enroll failed" }
}

# Wrapper so Task Scheduler captures logs
$wrapper = Join-Path $DevDir "run-agent.cmd"
$nodeExe = Join-Path $BinDir "coe-node.exe"
$stdout = Join-Path $LogDir "$DeviceId.out.log"
$stderr = Join-Path $LogDir "$DeviceId.err.log"
@"
@echo off
cd /d "$InstallRoot"
"$nodeExe" -config "$CfgPath" >> "$stdout" 2>> "$stderr"
"@ | Set-Content -Path $wrapper -Encoding ASCII

# Remove old task if present
$existing = Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
if ($existing) {
  Write-Host "Removing existing task $TaskName"
  Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false
}

$action = New-ScheduledTaskAction -Execute $wrapper
$trigger = New-ScheduledTaskTrigger -AtLogOn -User $env:USERNAME
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries `
  -StartWhenAvailable -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1) `
  -ExecutionTimeLimit ([TimeSpan]::Zero)
$principal = New-ScheduledTaskPrincipal -UserId $env:USERNAME -LogonType Interactive -RunLevel Limited

Register-ScheduledTask -TaskName $TaskName -Action $action -Trigger $trigger `
  -Settings $settings -Principal $principal -Description "COE desktop agent ($DeviceId) — user logon, DPAPI-safe" | Out-Null

Write-Host "Registered scheduled task: $TaskName (At logon as $env:USERNAME)"

if ($StartNow) {
  Write-Host "Starting agent now..."
  Start-ScheduledTask -TaskName $TaskName
  Start-Sleep 2
  try {
    $h = Invoke-RestMethod -Uri "http://$Api/v1/health" -Headers @{ Authorization = "Bearer $ApiToken" } -TimeoutSec 5
    Write-Host "API health:" ($h | ConvertTo-Json -Compress)
  } catch {
    Write-Host "API not up yet — check $stderr"
  }
}

Write-Host ""
Write-Host "=== COE agent installed ==="
Write-Host "Device:   $DeviceId"
Write-Host "Config:   $CfgPath"
Write-Host "API:      http://$Api"
Write-Host "Token:    $ApiToken"
Write-Host "Task:     $TaskName"
Write-Host "CLI:      & '$BinDir\coe-cli.exe' -api http://$Api -token $ApiToken status"
Write-Host "Uninstall: .\scripts\uninstall-agent.ps1 -DeviceId $DeviceId"
Write-Host ""
Write-Host "NOTE: Prefer this logon agent over Windows Service — DPAPI keys are user-scoped."
