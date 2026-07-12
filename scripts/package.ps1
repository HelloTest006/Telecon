# Build release layout under dist/coe/
# Usage: .\scripts\package.ps1
$ErrorActionPreference = "Stop"
$Root = Resolve-Path "$PSScriptRoot\.."
Set-Location $Root

$Out = Join-Path $Root "dist\coe"
Remove-Item -Recurse -Force -ErrorAction SilentlyContinue $Out
New-Item -ItemType Directory -Force -Path "$Out\bin", "$Out\scripts", "$Out\docs" | Out-Null

Write-Host "Building binaries..."
go build -ldflags "-s -w" -o "$Out\bin\ka.exe" ./cmd/ka
go build -ldflags "-s -w" -o "$Out\bin\coe-node.exe" ./cmd/coe-node
go build -ldflags "-s -w" -o "$Out\bin\coe-keygen.exe" ./cmd/coe-keygen
go build -ldflags "-s -w" -o "$Out\bin\coe-cli.exe" ./cmd/coe-cli
go build -ldflags "-s -w" -o "$Out\bin\ka-check.exe" ./cmd/ka-check
go build -ldflags "-s -w" -o "$Out\bin\coe-signal.exe" ./cmd/coe-signal
go build -ldflags "-s -w" -o "$Out\bin\coe-admin.exe" ./cmd/coe-admin

Copy-Item "$Root\scripts\install-agent.ps1" "$Out\scripts\"
Copy-Item "$Root\scripts\uninstall-agent.ps1" "$Out\scripts\"
Copy-Item "$Root\scripts\install-service.ps1" "$Out\scripts\"
Copy-Item "$Root\docs\coe\*.md" "$Out\docs\" -ErrorAction SilentlyContinue

@"
COE desktop package
===================

Recommended install (user logon agent — works with DPAPI):
  powershell -ExecutionPolicy Bypass -File scripts\install-agent.ps1 -DeviceId YOUR_ID -KaUrl https://ka.example.com -StartNow

Uninstall:
  powershell -ExecutionPolicy Bypass -File scripts\uninstall-agent.ps1 -DeviceId YOUR_ID

Prod KA check:
  bin\ka-check.exe -url https://ka.example.com -ca path\to\ca.crt -admin-token YOUR_TOKEN

WebRTC signaling (SDP/ICE only — not message relay):
  bin\coe-signal.exe -listen 0.0.0.0:8450
  coe-cli webrtc accept peer-id http://signal:8450
  coe-cli webrtc dial peer-id http://signal:8450

Do NOT use install-service.ps1 for user DPAPI keys (LocalSystem).
"@ | Set-Content "$Out\README.txt" -Encoding UTF8

Write-Host "Packaged: $Out"
Get-ChildItem -Recurse $Out | Select-Object FullName
