# Self-host KA + signal

Run your own Key Authority for your devices. **Server code is AGPL-3.0-or-later.**  
Project does **not** host shared auth/key servers — each operator runs their own.

| Path | Doc |
|------|-----|
| Docker | this file |
| Bare metal (no Docker) | [../self-host-bare-metal.md](../self-host-bare-metal.md) |
| Operator quickstart | [../self-host-quickstart.md](../self-host-quickstart.md) |

## Docker Compose

```bash
git clone <repo> && cd Telecon
export COE_KA_ADMIN_TOKEN=$(openssl rand -hex 24)
docker compose up -d --build
```

Services:

| Service | Port | Role |
|---------|------|------|
| `ka` | 8443 | Key Authority (TLS, daily keys, vouchers) |
| `signal` | 8450 | WebRTC SDP/ICE only (put TLS proxy in prod) |

Data volume: `ka-data` → `/data/master.key`, `/data/registry.json`, `/data/tls/`.

### Create enroll voucher

```bash
docker compose exec ka /coe-admin \
  -ka https://127.0.0.1:8443 -ka-insecure \
  -admin-token "$COE_KA_ADMIN_TOKEN" \
  voucher -max-uses 1 -ttl-hours 72 -label "alice-laptop"
```

Share **only** the printed `code` with the device owner (once).

### Device enroll (no admin token on device)

```text
coe-node -device-id alice -enroll -voucher <code> \
  -ka https://ka.example.com -ka-ca ka.crt
```

### Production flags

```text
ka -prod -admin-token $STRONG -rate-limit 60 -listen 0.0.0.0:8443
```

`-prod` refuses default admin token and plain HTTP.

### TURN (hard NAT)

1. Run coturn (see compose comment) or a cloud TURN provider.
2. On each agent:

```text
set COE_TURN_URLS=turn:turn.example.com:3478
set COE_TURN_USER=coe
set COE_TURN_PASS=secret
```

Or in `node.json`: `turn_urls`, `turn_user`, `turn_pass`.

### Checklist

```text
ka-check -url https://ka.example.com -ca ka.crt -admin-token $STRONG
```

See [08-prod-ka-checklist.md](08-prod-ka-checklist.md).
