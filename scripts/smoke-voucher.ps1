# Voucher enroll smoke
$ErrorActionPreference = "Continue"
Set-Location $PSScriptRoot\..

go build -o bin/ka.exe ./cmd/ka
go build -o bin/coe-node.exe ./cmd/coe-node
go build -o bin/coe-keygen.exe ./cmd/coe-keygen
go build -o bin/coe-admin.exe ./cmd/coe-admin

New-Item -ItemType Directory -Force -Path data/ka, data/devices/dev-v | Out-Null
Remove-Item -Force -ErrorAction SilentlyContinue data/ka/registry.json
Remove-Item -Recurse -Force -ErrorAction SilentlyContinue data/devices/dev-v
New-Item -ItemType Directory -Force -Path data/devices/dev-v | Out-Null
Get-Process ka -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep -Milliseconds 300

$ka = Start-Process .\bin\ka.exe -ArgumentList @("-listen","127.0.0.1:8443","-admin-token","dev-admin-token") -PassThru -WindowStyle Hidden `
  -RedirectStandardOutput data/ka/voucher.out.log -RedirectStandardError data/ka/voucher.err.log
Start-Sleep 2

$out = & .\bin\coe-admin.exe -ka https://127.0.0.1:8443 -ka-ca data/ka/tls/server.crt -admin-token dev-admin-token voucher 2>&1
Write-Host $out
$code = ($out | Select-String -Pattern '^code=(.+)$').Matches.Groups[1].Value
if (-not $code) { throw "no voucher code" }
Write-Host "voucher code: $code"

.\bin\coe-keygen.exe -device-id dev-v | Out-Null
.\bin\coe-node.exe -device-id dev-v -enroll -voucher $code -ka https://127.0.0.1:8443 -ka-ca data/ka/tls/server.crt
if ($LASTEXITCODE -ne 0) { throw "enroll failed" }

# issue key without admin
$p = Start-Process .\bin\coe-node.exe -ArgumentList @(
  "-device-id","dev-v","-listen","127.0.0.1:9010","-api","127.0.0.1:7710","-api-token","tok-v",
  "-ka","https://127.0.0.1:8443","-ka-ca","data/ka/tls/server.crt"
) -PassThru -WindowStyle Hidden -RedirectStandardOutput data/devices/dev-v/node.out.log -RedirectStandardError data/devices/dev-v/node.err.log
Start-Sleep 2
try {
  $h = Invoke-RestMethod -Uri http://127.0.0.1:7710/v1/status -Headers @{ Authorization = "Bearer tok-v" }
  Write-Host "status:" ($h | ConvertTo-Json -Compress)
} finally {
  Stop-Process -Id $p.Id,$ka.Id -Force -ErrorAction SilentlyContinue
}
Write-Host "SMOKE_VOUCHER_DONE"
