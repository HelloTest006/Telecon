# Self-host without Docker (bare metal)

Run COE Key Authority + signaling as normal processes. **No Docker required.**  
Server components are **AGPL-3.0-or-later**. Operator hosts everything.

## What you need

- Linux or Windows server (or two hosts)
- Go 1.22+ **or** prebuilt binaries from [releases](https://github.com/HelloTest006/Telecon/releases)
- Strong admin token (32+ random bytes)
- TLS for production (self-signed OK for lab only)
- Optional: reverse proxy (Caddy/nginx), coturn

## Layout (recommended)

```text
/opt/coe/                 # or C:\coe\
  bin/
    ka
    coe-signal
    coe-admin
    ka-check
  data/
    master.key            # created on first ka start
    registry.json
    tls/
      server.crt
      server.key
  logs/
```

## 1. Get binaries

### From release (Linux amd64)

```bash
mkdir -p /opt/coe/bin /opt/coe/data/tls /opt/coe/logs
cd /tmp
# download from release page, e.g.:
# ka-linux-amd64, coe-signal-linux-amd64, coe-admin-linux-amd64, ka-check-linux-amd64
install -m 755 ka-linux-amd64 /opt/coe/bin/ka
install -m 755 coe-signal-linux-amd64 /opt/coe/bin/coe-signal
install -m 755 coe-admin-linux-amd64 /opt/coe/bin/coe-admin
install -m 755 ka-check-linux-amd64 /opt/coe/bin/ka-check
```

### Build from source

```bash
git clone https://github.com/HelloTest006/Telecon.git
cd Telecon
go build -o /opt/coe/bin/ka ./cmd/ka
go build -o /opt/coe/bin/coe-signal ./cmd/coe-signal
go build -o /opt/coe/bin/coe-admin ./cmd/coe-admin
go build -o /opt/coe/bin/ka-check ./cmd/ka-check
```

Windows (PowerShell, lab):

```powershell
mkdir C:\coe\bin, C:\coe\data\tls, C:\coe\logs -Force
go build -o C:\coe\bin\ka.exe ./cmd/ka
go build -o C:\coe\bin\coe-signal.exe ./cmd/coe-signal
go build -o C:\coe\bin\coe-admin.exe ./cmd/coe-admin
go build -o C:\coe\bin\ka-check.exe ./cmd/ka-check
```

## 2. Admin token and env

```bash
export COE_KA_ADMIN_TOKEN=$(openssl rand -hex 24)
export COE_KA_MASTER_FILE=/opt/coe/data/master.key
export COE_KA_REGISTRY=/opt/coe/data/registry.json
export COE_KA_TLS_CERT=/opt/coe/data/tls/server.crt
export COE_KA_TLS_KEY=/opt/coe/data/tls/server.key
export COE_KA_TLS_HOSTS=ka.example.com,localhost,127.0.0.1
# save token offline — never commit
```

Windows:

```powershell
$env:COE_KA_ADMIN_TOKEN = -join ((1..48) | ForEach-Object { '{0:x}' -f (Get-Random -Max 16) })
$env:COE_KA_MASTER_FILE = 'C:\coe\data\master.key'
$env:COE_KA_REGISTRY = 'C:\coe\data\registry.json'
$env:COE_KA_TLS_CERT = 'C:\coe\data\tls\server.crt'
$env:COE_KA_TLS_KEY = 'C:\coe\data\tls\server.key'
$env:COE_KA_TLS_HOSTS = 'localhost,127.0.0.1'
```

## 3. Start Key Authority

Lab (auto self-signed cert if missing):

```bash
/opt/coe/bin/ka \
  -listen 0.0.0.0:8443 \
  -master "$COE_KA_MASTER_FILE" \
  -registry "$COE_KA_REGISTRY" \
  -tls-cert "$COE_KA_TLS_CERT" \
  -tls-key "$COE_KA_TLS_KEY" \
  -admin-token "$COE_KA_ADMIN_TOKEN" \
  -rate-limit 60
```

Production-ish (refuses default token; still need real certs / reverse proxy):

```bash
/opt/coe/bin/ka \
  -listen 127.0.0.1:8443 \
  -master "$COE_KA_MASTER_FILE" \
  -registry "$COE_KA_REGISTRY" \
  -tls-cert "$COE_KA_TLS_CERT" \
  -tls-key "$COE_KA_TLS_KEY" \
  -admin-token "$COE_KA_ADMIN_TOKEN" \
  -rate-limit 60 \
  -prod
```

First start creates `master.key` if absent. **Back it up immediately.**

### systemd unit example (`/etc/systemd/system/coe-ka.service`)

```ini
[Unit]
Description=COE Key Authority
After=network.target

[Service]
Type=simple
User=coe
Group=coe
Environment=COE_KA_ADMIN_TOKEN=file:/etc/coe/admin.token
# or EnvironmentFile=/etc/coe/ka.env
WorkingDirectory=/opt/coe
ExecStart=/opt/coe/bin/ka -listen 127.0.0.1:8443 -master /opt/coe/data/master.key -registry /opt/coe/data/registry.json -tls-cert /opt/coe/data/tls/server.crt -tls-key /opt/coe/data/tls/server.key -admin-token ${COE_KA_ADMIN_TOKEN} -rate-limit 60 -prod
Restart=on-failure
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
```

Prefer `EnvironmentFile` with `COE_KA_ADMIN_TOKEN=...` rather than embedding secrets in unit file.

## 4. Start signaling

Plain HTTP lab (put TLS proxy in front for prod):

```bash
/opt/coe/bin/coe-signal -listen 0.0.0.0:8450
```

With TLS files:

```bash
/opt/coe/bin/coe-signal \
  -listen 0.0.0.0:8450 \
  -tls-cert /opt/coe/data/tls/server.crt \
  -tls-key /opt/coe/data/tls/server.key
```

### systemd (`coe-signal.service`)

```ini
[Unit]
Description=COE WebRTC signaling (SDP/ICE only)
After=network.target

[Service]
Type=simple
User=coe
ExecStart=/opt/coe/bin/coe-signal -listen 127.0.0.1:8450
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

Signal carries **SDP/ICE only** — not message plaintext.

## 5. Reverse proxy (recommended for public hosts)

Terminate TLS on Caddy/nginx; proxy to local `ka` and `signal`.

Example Caddy snippets (adjust hostnames):

```text
ka.example.com {
  reverse_proxy https://127.0.0.1:8443 {
    transport http {
      tls_insecure_skip_verify
    }
  }
}

signal.example.com {
  reverse_proxy 127.0.0.1:8450
}
```

If KA already has public certs and listens on `:8443`, proxy optional.

## 6. Validate

```bash
/opt/coe/bin/ka-check \
  -url https://127.0.0.1:8443 \
  -ca /opt/coe/data/tls/server.crt \
  -admin-token "$COE_KA_ADMIN_TOKEN"
```

Want exit code **0**. Fix TLS / token / clock before enrolling devices.

Health probe:

```bash
curl -sk https://127.0.0.1:8443/v1/health
```

## 7. Mint voucher + enroll device

```bash
/opt/coe/bin/coe-admin \
  -ka https://127.0.0.1:8443 \
  -ka-ca /opt/coe/data/tls/server.crt \
  -admin-token "$COE_KA_ADMIN_TOKEN" \
  voucher -max-uses 1 -ttl-hours 72 -label alice-laptop
```

Copy printed `code` once. On Windows device (agent binaries from release):

```powershell
.\coe-keygen.exe -device-id alice
.\coe-node.exe -device-id alice -enroll -voucher <code> `
  -ka https://ka.example.com -ka-ca C:\path\ka.crt
.\scripts\install-agent.ps1 -DeviceId alice `
  -KaUrl https://ka.example.com -KaCa C:\path\ka.crt -StartNow
```

Or enroll inside install script with `-Voucher` (see [07-desktop-agent.md](coe/07-desktop-agent.md)).

## 8. TURN (internet / hard NAT)

Install [coturn](https://github.com/coturn/coturn) on host or use a provider. Example flags:

```text
--lt-cred-mech
--user=coe:STRONG_PASS
--realm=example.com
--listening-port=3478
--external-ip=YOUR_PUBLIC_IP
--min-port=49152
--max-port=65535
```

Open UDP/TCP 3478 (and relay range). On each agent:

```text
COE_STUN=stun:turn.example.com:3478
COE_TURN_URLS=turn:turn.example.com:3478
COE_TURN_USER=coe
COE_TURN_PASS=STRONG_PASS
```

## 9. Backup

Minimum:

| Path | Why |
|------|-----|
| `master.key` | all daily key derivation |
| `registry.json` | devices, vouchers, serials |
| TLS cert/key | client trust |

Linux helper: `scripts/backup-ka.sh /opt/coe/data /var/backups/coe`

Test restore on spare host before you need it.

## 10. Firewall sketch

| Port | Service |
|------|---------|
| 443 or 8443 | KA (HTTPS) |
| 8450 or 443 (proxy) | signal |
| 3478 / 5349 | TURN |
| 49152–65535/udp | TURN relays (if coturn defaults) |

Do **not** expose agent local API (`127.0.0.1:7701`) to the internet.

## Windows server notes

- Same binaries with `.exe`
- Prefer NSSM or Task Scheduler for long-running `ka` / `coe-signal`
- Store admin token in machine secret store / ACL-locked env file
- Agent on end-user PCs still uses **user logon** task for DPAPI — not LocalSystem

## Lab-only shortcuts (never prod)

```bash
# plain HTTP KA — insecure
ka -http -listen 127.0.0.1:8443 -admin-token dev-admin-token
```

Devices then need `-ka-insecure` or HTTP URL. Do not ship this to real users.

## See also

- Docker path: [coe/10-selfhost.md](coe/10-selfhost.md)
- Operator quickstart: [self-host-quickstart.md](self-host-quickstart.md)
- Prod checks: [coe/08-prod-ka-checklist.md](coe/08-prod-ka-checklist.md)
- Security: [../SECURITY_SELFHOST.md](../SECURITY_SELFHOST.md)
- Vouchers: [coe/11-vouchers.md](coe/11-vouchers.md)
- WebRTC: [coe/09-webrtc.md](coe/09-webrtc.md)
