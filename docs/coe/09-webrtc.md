# WebRTC path (off-LAN)

COE app payloads stay end-to-end encrypted. WebRTC carries the **same COE P2P records** on a data channel. Signaling carries **SDP/ICE only**.

## Components

| Process | Role |
|---------|------|
| `coe-signal` | HTTP long-poll exchange for offer/answer/ICE |
| `coe-node` | WebRTC dial/accept via local API |
| STUN | Public ICE (default multi Google STUN; replace in locked-down nets) |
| TURN | Optional for symmetric NAT (operator-hosted coturn or provider) |

## Run

```text
# 1) Signaling (can be separate host; still no plaintext)
coe-signal -listen 0.0.0.0:8450

# 2) Peer B waits
coe-cli -api http://127.0.0.1:7702 -token tok-b webrtc accept dev-a http://SIGNAL:8450

# 3) Peer A dials
coe-cli -api http://127.0.0.1:7701 -token tok-a webrtc dial dev-b http://SIGNAL:8450

# 4) Same send/inbox as TCP
coe-cli ... send dev-b hello
```

Or use **`coe-tray`** UI over the local agent API.

## TURN / ICE policy

```text
# environment on coe-node
COE_STUN=stun:stun.example.com:3478,stun:stun1.l.google.com:19302
COE_TURN_URLS=turn:turn.example.com:3478
COE_TURN_USER=coe
COE_TURN_PASS=secret
# force TURN-only candidates (corporate / hard NAT)
COE_ICE_POLICY=relay
```

Or `node.json`: `stun`, `turn_urls`, `turn_user`, `turn_pass`.

| `COE_ICE_POLICY` | Behavior |
|------------------|----------|
| `all` (default) | host + server-reflexive + relay |
| `relay` | TURN only (requires TURN URLs) |

Default STUN list uses multiple public servers when unset. Without TURN, many cellular / corporate NATs fail ICE.

## Security notes

- Signal server **must not** be trusted with app data (it never sees COE DATA plaintext).
- Still requires both devices have valid daily keys + identity ECDH (Strong).
- Use TLS in front of `coe-signal` in production (reverse proxy) or `-tls-cert`/`-tls-key`.
- Prefer private STUN/TURN for enterprise.

## API

`POST /v1/sessions/webrtc`

```json
{"device_id":"dev-b","signal_url":"http://127.0.0.1:8450","role":"dial","wait_sec":60}
```

`role`: `dial` | `accept`
