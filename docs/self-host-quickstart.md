# Self-host quickstart

Goal: get one operator-run COE stack working for own devices.

## What you need

- one Linux or Windows host for `ka`
- one host for `coe-signal` (can be same box)
- one public domain name
- optional TURN for hard NAT / internet users
- Windows device for `coe-node`

## 1. Prepare environment

Create `.env` from `.env.example` and set:

```text
DOMAIN=ka.example.com
COE_KA_ADMIN_TOKEN=<strong random>
EXTERNAL_IP=<public ip>
COE_TURN_USER=coe
COE_TURN_PASS=<strong random>
```

Rules:

- never keep `dev-admin-token`
- never use `-http` in production
- back up `master.key` and `registry.json`

## 2. Start stack

For small beta, use:

```bash
docker compose -f docker-compose.prod.yml up -d --build
```

This starts:

- `ka`
- `coe-signal`
- `caddy`
- optional `turn`

## 3. Validate deployment

Run:

```bash
ka-check -url https://ka.example.com -ca /path/to/ka.crt -admin-token "$COE_KA_ADMIN_TOKEN"
```

Required result: exit `0`.

If `admin_token_set` fails, rotate token. If TLS fails, fix certs first.

## 4. Mint voucher

```bash
coe-admin -ka https://ka.example.com -ka-ca /path/to/ka.crt -admin-token "$COE_KA_ADMIN_TOKEN" voucher -max-uses 1 -ttl-hours 72 -label alice-laptop
```

Share only printed `code` with device owner.

## 5. Install Windows agent

On device:

```powershell
.\scripts\install-agent.ps1 -DeviceId alice -KaUrl https://ka.example.com -KaCa C:\path\ka.crt -Voucher <code> -StartNow
```

This:

- generates identity
- enrolls with voucher
- creates user logon task
- starts local API

## 6. Test local API

```powershell
.\bin\coe-cli.exe -api http://127.0.0.1:7701 -token <api-token> status
```

Check:

- `protect: true`
- current `epoch_id`
- empty or live `sessions`

## 7. Connect peers

LAN / direct TCP:

```powershell
coe-cli -api http://127.0.0.1:7701 -token <tok> connect bob 192.168.1.22:9001
coe-cli -api http://127.0.0.1:7701 -token <tok> send bob hello
```

WebRTC:

```powershell
coe-cli -api http://127.0.0.1:7701 -token <tok> webrtc dial bob https://signal.example.com
```

## 8. Turn on TURN for internet users

Set on agents:

```text
COE_STUN=stun:stun.example.com:3478
COE_TURN_URLS=turn:turn.example.com:3478
COE_TURN_USER=coe
COE_TURN_PASS=<secret>
```

Without TURN, many NATs fail.

## 9. Backup

At minimum back up:

- `/data/master.key`
- `/data/registry.json`
- TLS materials

Use `scripts/backup-ka.sh` on Linux hosts.

## 10. Beta handoff to testers

Give testers:

- release zip
- KA URL
- KA CA / cert file
- voucher code
- support instructions

## Supported first topology

One KA host, one signal host, one TURN, Windows agents.

Keep beta narrow. Expand later.
