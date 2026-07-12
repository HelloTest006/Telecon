# COE — Communication over Encryption

Protocol and architecture specification (v1 design).

## Reading order

1. [00-overview](00-overview.md) — goals, non-goals, architecture
2. [01-threat-model](01-threat-model.md) — adversaries, assumptions, residual risk
3. [02-key-management](02-key-management.md) — hybrid keys, Xoroshiro boundary, issuance
4. [03-ka-api](03-ka-api.md) — Key Authority HTTPS API
5. [04-p2p-protocol](04-p2p-protocol.md) — device↔device TCP/UDP framing
6. [05-crypto](05-crypto.md) — algorithms, labels, sizes
7. [06-go-implementation-map](06-go-implementation-map.md) — Go package layout
8. [07-desktop-agent](07-desktop-agent.md) — agent install + local API
9. [08-prod-ka-checklist](08-prod-ka-checklist.md) — production KA
10. [09-webrtc](09-webrtc.md) — off-LAN WebRTC path
11. [10-selfhost](10-selfhost.md) — Docker KA + signal
12. [11-vouchers](11-vouchers.md) — enroll without admin token
13. [12-public-beta-checklist](12-public-beta-checklist.md) — upload readiness

## One-line summary

Devices pull a **daily general key** from a central Key Authority once per 24h epoch. **Message content never touches the server** — peers talk direct TCP/UDP with AEAD under session keys derived from daily material + long-term identity ECDH.

## Status

| Item | Status |
|------|--------|
| Protocol + architecture spec | This tree |
| Runnable server/clients | Not in scope of this deliverable |
| Stack (future) | Go |
| P2P (v1) | Direct TCP (primary), UDP datagram profile optional |

## Profiles

| Profile | Session key binder | Use |
|---------|-------------------|-----|
| **COE-Strong** (default) | ECDH(A,B) ∥ GeneralKey_A ∥ GeneralKey_B ∥ epoch | Production intent |
| **COE-Simple** | Single org-wide daily root | Demos / closed LAN only |
