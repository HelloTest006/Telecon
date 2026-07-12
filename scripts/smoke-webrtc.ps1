# WebRTC path smoke (localhost STUN + coe-signal)
$ErrorActionPreference = "Continue"
Set-Location $PSScriptRoot\..

go build -o bin/ka.exe ./cmd/ka
go build -o bin/coe-node.exe ./cmd/coe-node
go build -o bin/coe-keygen.exe ./cmd/coe-keygen
go build -o bin/coe-cli.exe ./cmd/coe-cli
go build -o bin/coe-signal.exe ./cmd/coe-signal

New-Item -ItemType Directory -Force -Path data/ka, data/devices/dev-a, data/devices/dev-b | Out-Null
Remove-Item -Force -ErrorAction SilentlyContinue data/ka/registry.json
Remove-Item -Recurse -Force -ErrorAction SilentlyContinue data/devices/dev-a, data/devices/dev-b
New-Item -ItemType Directory -Force -Path data/devices/dev-a, data/devices/dev-b | Out-Null
Get-Process ka,coe-node,coe-signal -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep -Milliseconds 400

$ka = Start-Process .\bin\ka.exe -ArgumentList @("-listen","127.0.0.1:8443") -PassThru -WindowStyle Hidden `
  -RedirectStandardOutput data/ka/ka.out.log -RedirectStandardError data/ka/ka.err.log
$sig = Start-Process .\bin\coe-signal.exe -ArgumentList @("-listen","127.0.0.1:8450") -PassThru -WindowStyle Hidden `
  -RedirectStandardOutput data/ka/signal.out.log -RedirectStandardError data/ka/signal.err.log
Start-Sleep 2

.\bin\coe-keygen.exe -device-id dev-a | Out-Null
.\bin\coe-keygen.exe -device-id dev-b | Out-Null
$kaArgs = @("-ka","https://127.0.0.1:8443","-ka-ca","data/ka/tls/server.crt","-admin-token","dev-admin-token")
.\bin\coe-node.exe -device-id dev-a -enroll @kaArgs | Out-Null
.\bin\coe-node.exe -device-id dev-b -enroll @kaArgs | Out-Null

$b = Start-Process .\bin\coe-node.exe -ArgumentList (@(
  "-device-id","dev-b","-listen","127.0.0.1:9002","-api","127.0.0.1:7702","-api-token","tok-b"
) + $kaArgs) -PassThru -WindowStyle Hidden `
  -RedirectStandardOutput data/devices/dev-b/node.out.log -RedirectStandardError data/devices/dev-b/node.err.log
$a = Start-Process .\bin\coe-node.exe -ArgumentList (@(
  "-device-id","dev-a","-listen","127.0.0.1:9001","-api","127.0.0.1:7701","-api-token","tok-a"
) + $kaArgs) -PassThru -WindowStyle Hidden `
  -RedirectStandardOutput data/devices/dev-a/node.out.log -RedirectStandardError data/devices/dev-a/node.err.log
Start-Sleep 2

# B accepts in background job
$acceptJob = Start-Job -ScriptBlock {
  Set-Location $using:PWD
  & .\bin\coe-cli.exe -api http://127.0.0.1:7702 -token tok-b webrtc accept dev-a http://127.0.0.1:8450 2>&1
}
Start-Sleep 1
Write-Host "A dialing WebRTC..."
& .\bin\coe-cli.exe -api http://127.0.0.1:7701 -token tok-a webrtc dial dev-b http://127.0.0.1:8450
$acceptOut = Receive-Job $acceptJob -Wait -AutoRemoveJob
Write-Host "B accept:" $acceptOut

& .\bin\coe-cli.exe -api http://127.0.0.1:7701 -token tok-a send dev-b webrtc-ping
Start-Sleep 1
Write-Host "B inbox:"
& .\bin\coe-cli.exe -api http://127.0.0.1:7702 -token tok-b inbox

Stop-Process -Id $a.Id,$b.Id,$ka.Id,$sig.Id -Force -ErrorAction SilentlyContinue
Write-Host "SMOKE_WEBRTC_DONE"
