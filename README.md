# COE — Communication over Encryption

Devices pull a **daily key** from a Key Authority; **messages stay device↔device** (TCP or WebRTC). Protocol is an open specification.

## Licenses

| Component | License |
|-----------|---------|
| Protocol spec (`docs/coe/`) | **Apache-2.0** |
| Client / agent (`coe-node`, `coe-cli`, crypto, P2P) | **Apache-2.0** |
| Servers (`ka`, `coe-signal`, `coe-admin`, Docker images) | **AGPL-3.0-or-later** |

See [LICENSE](LICENSE) and [LICENSES/](LICENSES/).

## Quick start (dev)

```powershell
go test ./...
go build -o bin/ka.exe ./cmd/ka
go build -o bin/coe-node.exe ./cmd/coe-node
go build -o bin/coe-keygen.exe ./cmd/coe-keygen
go build -o bin/coe-cli.exe ./cmd/coe-cli
go build -o bin/coe-admin.exe ./cmd/coe-admin
go build -o bin/coe-signal.exe ./cmd/coe-signal

.\bin\ka.exe -listen 127.0.0.1:8443
# other terminal
.\bin\coe-admin.exe voucher
.\bin\coe-keygen.exe -device-id alice
.\bin\coe-node.exe -device-id alice -enroll -voucher <code> -ka https://127.0.0.1:8443 -ka-ca data/ka/tls/server.crt
```

## Self-host servers (Docker)

```bash
export COE_KA_ADMIN_TOKEN=$(openssl rand -hex 24)
docker compose up -d --build
docker compose exec ka /coe-admin -ka https://127.0.0.1:8443 -ka-insecure \
  -admin-token "$COE_KA_ADMIN_TOKEN" voucher
# copy /data/tls/server.crt to clients as -ka-ca
```

Optional TURN for hard NAT: set `COE_TURN_URLS`, `COE_TURN_USER`, `COE_TURN_PASS` on agents; see `docker-compose.yml` coturn example.

## Desktop agent (Windows)

```powershell
.\scripts\install-agent.ps1 -DeviceId alice -KaUrl https://ka.example.com -KaCa C:\path\ka.crt -Voucher $code -StartNow
```

Uses **user logon** Task Scheduler (DPAPI-safe). Do not run as LocalSystem.

## Docs

- Spec index: [docs/coe/README.md](docs/coe/README.md)
- Desktop agent: [docs/coe/07-desktop-agent.md](docs/coe/07-desktop-agent.md)
- Prod KA: [docs/coe/08-prod-ka-checklist.md](docs/coe/08-prod-ka-checklist.md)
- WebRTC: [docs/coe/09-webrtc.md](docs/coe/09-webrtc.md)
- Self-host: [docs/coe/10-selfhost.md](docs/coe/10-selfhost.md)

## Security claims

Confidentiality requires secrecy of daily keys and device identity material. Keys rotate every 24h. The key server is not on the message path. **Not** “uncrackable.”
