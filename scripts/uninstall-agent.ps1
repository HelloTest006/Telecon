# Remove COE logon agent task and optionally data.
param(
  [Parameter(Mandatory = $true)][string]$DeviceId,
  [string]$InstallRoot = "",
  [switch]$RemoveData
)

$ErrorActionPreference = "Stop"
if (-not $InstallRoot) {
  $InstallRoot = Join-Path $env:LOCALAPPDATA "COE"
}
$TaskName = "COE-Agent-$DeviceId"

$task = Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
if ($task) {
  Stop-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
  Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false
  Write-Host "Removed task $TaskName"
} else {
  Write-Host "No task $TaskName"
}

# Stop process if running from this install
Get-CimInstance Win32_Process -Filter "Name = 'coe-node.exe'" -ErrorAction SilentlyContinue |
  Where-Object { $_.CommandLine -match [regex]::Escape($DeviceId) } |
  ForEach-Object {
    Write-Host "Stopping PID $($_.ProcessId)"
    Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue
  }

if ($RemoveData) {
  $dev = Join-Path $InstallRoot "devices\$DeviceId"
  if (Test-Path $dev) {
    Remove-Item -Recurse -Force $dev
    Write-Host "Removed $dev"
  }
}

Write-Host "Uninstall done for $DeviceId"
