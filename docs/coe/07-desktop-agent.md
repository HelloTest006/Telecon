# Desktop agent (track A)

`coe-node` = long-lived desktop agent:

1. TLS to KA + **Ed25519-signed** daily key issue  
2. **DPAPI** (Windows) for identity + general keys at rest  
3. P2P listen / dial (TCP and/or WebRTC)  
4. Localhost app API for UIs (`coe-cli`)  
5. **Logon agent** install (Task Scheduler, user context)

## Install (recommended)

```powershell
go build -o bin/coe-node.exe ./cmd/coe-node
go build -o bin/coe-keygen.exe ./cmd/coe-keygen
go build -o bin/coe-cli.exe ./cmd/coe-cli

# Per-user logon task under %LOCALAPPDATA%\COE (DPAPI-safe)
.\scripts\install-agent.ps1 -DeviceId alice `
  -KaUrl https://ka.example.com -KaCa C:\path\ka.crt `
  -Enroll -AdminToken $env:COE_ADMIN -StartNow
```

Uninstall: `.\scripts\uninstall-agent.ps1 -DeviceId alice`

**Do not** use `install-service.ps1` for real user installs — LocalSystem cannot open user DPAPI keys.

Package release layout: `.\scripts\package.ps1` → `dist/coe/`.

## KA (TLS default)

```text
ka -listen 127.0.0.1:8443
ka-check -url https://127.0.0.1:8443 -ca data/ka/tls/server.crt -admin-token ...
```

## Local app API

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/v1/health` | liveness |
| GET | `/v1/status` | device, epoch, sessions |
| POST | `/v1/sessions` | TCP connect |
| POST | `/v1/sessions/webrtc` | WebRTC connect |
| POST | `/v1/send` | send text |
| GET | `/v1/inbox` | messages |

## CLI

```text
coe-cli -api http://127.0.0.1:7701 -token SECRET status
coe-cli ... connect bob 192.168.1.10:9001
coe-cli ... webrtc dial bob http://signal:8450
coe-cli ... send bob hello
```

## Auth model (KA)

- Transport: TLS 1.2+  
- Device issue: Ed25519  
- Admin enroll/revoke: Bearer token  

See also: [08-prod-ka-checklist](08-prod-ka-checklist.md), [09-webrtc](09-webrtc.md).
