# Build release assets + optional Windows signtool sign
# Usage (PowerShell):
#   .\scripts\build-release.ps1 -Version 0.1.0-beta -Sign $false

param(
  [string]$Version = "0.1.0-beta",
  [switch]$Sign,
  [string]$CertThumb = "",
  [string]$Timestamp = "http://timestamp.digicert.com"
)

$ErrorActionPreference = "Stop"
$root = Resolve-Path "$PSScriptRoot\.."
Set-Location $root

$dist = Join-Path $root "dist\release-$Version"
Remove-Item -Recurse -Force -ErrorAction SilentlyContinue $dist
New-Item -ItemType Directory -Force -Path $dist | Out-Null
New-Item -ItemType Directory -Force -Path "$dist\scripts" | Out-Null

Write-Host "Building binaries..."
go build -ldflags "-s -w -X main.version=$Version" -o "$dist\ka.exe" ./cmd/ka
go build -ldflags "-s -w -X main.version=$Version" -o "$dist\coe-signal.exe" ./cmd/coe-signal
go build -ldflags "-s -w -X main.version=$Version" -o "$dist\coe-admin.exe" ./cmd/coe-admin
go build -ldflags "-s -w -X main.version=$Version" -o "$dist\ka-check.exe" ./cmd/ka-check

go build -ldflags "-s -w -X main.version=$Version" -o "$dist\coe-node.exe" ./cmd/coe-node
go build -ldflags "-s -w -X main.version=$Version" -o "$dist\coe-cli.exe" ./cmd/coe-cli
go build -ldflags "-s -w -X main.version=$Version" -o "$dist\coe-keygen.exe" ./cmd/coe-keygen

if ($Sign) {
  if (-not $CertThumb) { throw "Provide -CertThumb for signtool" }
  Write-Host "Signing Windows binaries..."
  $files = @("$dist\coe-node.exe","$dist\coe-cli.exe","$dist\coe-keygen.exe","$dist\ka-check.exe")
  foreach ($f in $files) {
    & signtool sign /sha1 $CertThumb /tr $Timestamp /td sha256 /fd sha256 /a $f
  }
}

Copy-Item docker-compose.yml $dist\
Copy-Item docker-compose.prod.yml $dist\
Copy-Item Caddyfile $dist\
Copy-Item Dockerfile $dist\
Copy-Item scripts\install-agent.ps1 $dist\scripts\
Copy-Item scripts\uninstall-agent.ps1 $dist\scripts\
Copy-Item scripts\package.ps1 $dist\scripts\

"COE $Version" | Set-Content "$dist\VERSION.txt" -Encoding ASCII

Compress-Archive -Path "$dist\*" -DestinationPath "$dist\coe-$Version.zip" -Force

Write-Host "Release ready: $dist"
Get-ChildItem $dist
