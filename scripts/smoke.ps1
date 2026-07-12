# COE smoke: KA + two nodes localhost
$ErrorActionPreference = "Continue"
Set-Location $PSScriptRoot\..

go build -o bin/ka.exe ./cmd/ka
go build -o bin/coe-node.exe ./cmd/coe-node
go build -o bin/coe-keygen.exe ./cmd/coe-keygen

New-Item -ItemType Directory -Force -Path data/ka, data/devices/dev-a, data/devices/dev-b | Out-Null
Remove-Item -Force -ErrorAction SilentlyContinue data/ka/registry.json

$ka = Start-Process .\bin\ka.exe -ArgumentList @("-http","-listen","127.0.0.1:8443") -PassThru -WindowStyle Hidden `
  -RedirectStandardOutput data/ka/ka.out.log -RedirectStandardError data/ka/ka.err.log
Start-Sleep 1

if (-not (Test-Path data/devices/dev-a/identity.json)) { .\bin\coe-keygen.exe -device-id dev-a }
if (-not (Test-Path data/devices/dev-b/identity.json)) { .\bin\coe-keygen.exe -device-id dev-b }

.\bin\coe-node.exe -device-id dev-a -enroll | Out-Null
.\bin\coe-node.exe -device-id dev-b -enroll | Out-Null

$b = Start-Process .\bin\coe-node.exe -ArgumentList @("-device-id","dev-b","-listen","127.0.0.1:9002") -PassThru -WindowStyle Hidden `
  -RedirectStandardOutput data/devices/dev-b/node.out.log -RedirectStandardError data/devices/dev-b/node.err.log
Start-Sleep 1

$a = Start-Process .\bin\coe-node.exe -ArgumentList @(
  "-device-id","dev-a","-peer","dev-b@127.0.0.1:9002","-msg","ping-coe-1"
) -PassThru -WindowStyle Hidden -Wait `
  -RedirectStandardOutput data/devices/dev-a/node.out.log -RedirectStandardError data/devices/dev-a/node.err.log

Write-Host "exit=$($a.ExitCode)"
Get-Content data/devices/dev-a/node.out.log
Get-Content data/devices/dev-b/node.out.log

Stop-Process -Id $b.Id,$ka.Id -Force -ErrorAction SilentlyContinue
