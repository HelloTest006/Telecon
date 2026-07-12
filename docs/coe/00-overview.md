# COE Overview

**COE** = Communication over Encryption.

## Goals

1. **Daily key gate** — Without the current epoch general key material (issued by the Key Authority), a passive or active network adversary cannot decrypt COE session traffic for that epoch.
2. **Server off the message path** — Key Authority (KA) issues keys only. Application payloads flow **device ↔ device**.
3. **Bounded exposure** — Keys expire every **24 hours** (UTC epoch). Compromise of one epoch does not automatically yield the next.
4. **Practical LAN / known-address P2P** — Peers reach each other by configured IP:port (TCP primary).
5. **Honest security claims** — Spec states what is protected and what is not. No absolute “uncrackable” guarantee.

## Non-goals (v1)

- NAT traversal / WebRTC (appendix candidate only)
- Message relay or store-and-forward on KA
- Anonymous / metadata-hiding networking
- Post-quantum KEMs (may be added later)
- Multi-hop mesh routing
- Claiming computational impossibility against all adversaries forever

## High-level architecture

```
┌─────────────┐   TLS 1.3 (key issue only)   ┌──────────────────┐
│  Device A   │ ────────────────────────────► │  Key Authority   │
│  (coe-node) │ ◄── GeneralKey + epoch meta   │  (central)       │
└──────┬──────┘                               └────────▲─────────┘
       │                                               │
       │  COE P2P: TCP (AEAD records)                  │ same daily
       │  optional UDP datagram profile                │ key path
       ▼                                               │
┌─────────────┐                                        │
│  Device B   │ ───────────────────────────────────────┘
│  (coe-node) │
└─────────────┘
```

| Component | Role |
|-----------|------|
| **Key Authority (KA)** | Enroll devices, authenticate, issue **one active general key per device per epoch**, revoke, audit |
| **coe-node (device)** | Pull daily key, hold long-term identity keypair, open P2P sessions, encrypt app data |
| **Roster / config** | Local peer list: `device_id → host:port` (out of band; not provided by KA in v1) |

## Communication modes

| Link | Purpose | Data on wire |
|------|---------|--------------|
| Device → KA | Daily key issue / revoke check | Auth, epoch, key material (confidential under TLS) |
| Device ↔ Device | Application communication | COE handshake + AEAD ciphertext only |

KA **must not** receive application plaintext or P2P session keys.

## Epoch model

- **Epoch id**: `epoch_id = floor(unix_utc_seconds / 86400)` (UTC calendar day boundary).
- **Validity**: `not_before = epoch_id * 86400`, `not_after = not_before + 86400`.
- **Grace**: implementations **should** accept sessions still using `epoch_id - 1` for up to **300 seconds** after rollover for in-flight close/rehandshake.
- **Clock**: devices **should** sync time (NTP). Allowed skew for issue: ±120 seconds vs KA.

## Security stance (summary)

| Claim language | Spec meaning |
|----------------|--------------|
| “Highly safe” | AEAD + short-lived keys + server off message path + strong binder profile |
| “Uncrackable without general key” | **Marketing shorthand only.** Correct form: *confidentiality of P2P records requires secrecy of session key material; that material depends on daily general keys (and identity secrets in COE-Strong).* |
| Xoroshiro-128 | **PRNG for non-secret identifiers and tickets only** — never sole source of cryptographic key bits |

Full threat model: [01-threat-model](01-threat-model.md).

## Design defaults (v1)

| Decision | Choice |
|----------|--------|
| Stack (future impl) | Go |
| KA transport | HTTPS / TLS 1.3 |
| P2P transport | TCP length-prefixed records; UDP optional |
| Wire encoding | CBOR |
| AEAD | ChaCha20-Poly1305 |
| KDF | HKDF-SHA-256 |
| Default profile | **COE-Strong** (identity ECDH + both daily keys) |
| PRNG for tickets/IDs | Xoroshiro-128++ (server), hybrid with real CSPRNG seed |

## Document map

See [README](README.md).
