# COE — Communication over Encryption

**Public beta. Self-hosted only.** Project does **not** run shared auth, key, signaling, or relay infrastructure for users.

COE gives operators software to run their **own** Key Authority, signaling, and optional TURN infrastructure for **their own** devices. Devices pull a **daily key** from that operator-run Key Authority; messages stay **device to device** over TCP or WebRTC.

## Who hosts what

| Thing | Hosted by |
|------|-----------|
| `ka` Key Authority / daily keys | operator |
| `coe-signal` SDP/ICE signaling | operator |
| TURN relay for hard NAT | operator |
| `coe-node` desktop agent | device owner / operator |
| Protocol spec | open |

If user cannot or will not run their own backend, COE is not right fit today.

## Security claims (honest)

Confidentiality requires secrecy of daily keys and device identity material. Keys rotate every 24 hours. Key server is not on message path. Project does **not** claim “uncrackable.”

## Licenses

| Component | License |
|-----------|---------|
| Protocol spec (`docs/coe/`) | **Apache-2.0** |
| Client / agent (`coe-node`, `coe-cli`, crypto, P2P) | **Apache-2.0** |
| Servers (`ka`, `coe-signal`, `coe-admin`, Docker images) | **AGPL-3.0-or-later** |

See [LICENSE](LICENSE), [LICENSES/](LICENSES/), [NOTICE](NOTICE).

## Fast path

1. Operator deploys self-host stack: [docs/coe/10-selfhost.md](docs/coe/10-selfhost.md)
2. Operator validates deployment: [docs/coe/08-prod-ka-checklist.md](docs/coe/08-prod-ka-checklist.md)
3. Operator mints voucher: `coe-admin voucher`
4. User installs Windows agent: `scripts/install-agent.ps1`
5. User enrolls with voucher and connects peers

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
```

Need real TLS + TURN for wider internet beta. See [docs/coe/10-selfhost.md](docs/coe/10-selfhost.md).

## Desktop agent (Windows)

```powershell
.\scripts\install-agent.ps1 -DeviceId alice -KaUrl https://ka.example.com -KaCa C:\path\ka.crt -Voucher $code -StartNow
```

Uses **user logon** Task Scheduler so DPAPI-protected keys open correctly. Do not run as LocalSystem for real installs.

## Release assets

- build release zip: `scripts/build-release.ps1 -Version 0.1.0-beta`
- public beta release: GitHub release `v0.1.0-beta`
- self-host stack: `docker-compose.yml`, `docker-compose.prod.yml`, `Dockerfile`, `Caddyfile`

## Docs

- spec index: [docs/coe/README.md](docs/coe/README.md)
- self-host quickstart: [docs/self-host-quickstart.md](docs/self-host-quickstart.md)
- beta scope: [BETA_SCOPE.md](BETA_SCOPE.md)
- self-host security: [SECURITY_SELFHOST.md](SECURITY_SELFHOST.md)
- production KA checks: [docs/coe/08-prod-ka-checklist.md](docs/coe/08-prod-ka-checklist.md)
- WebRTC and TURN: [docs/coe/09-webrtc.md](docs/coe/09-webrtc.md)
- vouchers: [docs/coe/11-vouchers.md](docs/coe/11-vouchers.md)
- public beta checklist: [docs/coe/12-public-beta-checklist.md](docs/coe/12-public-beta-checklist.md)
- compatibility: [COMPATIBILITY.md](COMPATIBILITY.md)
- changelog: [CHANGELOG.md](CHANGELOG.md)
