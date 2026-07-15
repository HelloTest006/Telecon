# COE Protocol v1 — Wire freeze

**Status:** **FROZEN** as of release **`v0.1.0-beta`**.

This document is the compatibility contract for implementers.  
Changes that break any **normative** item below require a new major protocol version (`v2`), not a silent beta bump.

---

## Scope of the freeze

| In freeze | Out of freeze (may change without v2) |
|-----------|----------------------------------------|
| P2P record types, framing, AAD, AEAD suite | Admin UI HTML/CSS |
| Strong/Simple session derivation labels | Docker compose layouts |
| KA HTTP JSON endpoints listed below | Install scripts paths |
| KEY_OFFER wrap + seal | STUN server hostnames |
| Epoch math (UTC day, grace) | Local agent API extras |
| Voucher enroll fields | Update-manifest format signing |

Product/UX and operator tooling can evolve. **Wire and crypto labels cannot** without `v2`.

---

## Normative references (v1)

1. [04-p2p-protocol.md](04-p2p-protocol.md) — framing, handshake, DATA, KEY_OFFER  
2. [05-crypto.md](05-crypto.md) — algorithms, HKDF labels, nonces, AAD  
3. [02-key-management.md](02-key-management.md) — general key, epoch, profiles  
4. [03-ka-api.md](03-ka-api.md) — issue/enroll/voucher API shapes  

If this freeze note conflicts with an older draft sentence, **this freeze + crypto/p2p docs win**.

---

## Frozen suite (v1)

| Role | Algorithm / rule |
|------|------------------|
| Hash / KDF | SHA-256 / HKDF-SHA-256 |
| AEAD | ChaCha20-Poly1305 (RFC 8439) |
| Identity / eph DH | X25519 |
| Issue auth | Ed25519 over `COE-v1-issue` payload |
| GeneralKey | 32 bytes via HKDF from `KA_MASTER` |
| AEAD nonce | 12 bytes = `I2OSP(0,4) \|\| I2OSP(seq,8)` |
| TCP frame | `u32 BE length` + payload; max 65536; length 0 illegal |
| Protocol `ver` | `1` |

Xoroshiro-128++ remains **non-secret tickets only** (never AEAD/GeneralKey bits).

---

## Frozen P2P record types

| `type` | Name | Notes |
|--------|------|--------|
| 1 | HELLO | clear |
| 2 | HELLO_ACK | clear |
| 3 | SESSION_OK | optional |
| 4 | DATA | AEAD |
| 5 | CLOSE | |
| 6 | EPOCH_WARN | |
| 7 | KEY_OFFER | sealed GeneralKey under wrap key |

New types **MUST** use unused numbers ≥ 8 and be negotiated only after a version bump or optional capability (not required for v1 interop).

---

## Frozen KA HTTP surface (v1)

| Method | Path | Auth |
|--------|------|------|
| GET | `/v1/health` | none |
| POST | `/v1/enroll` | admin **or** `voucher_code` |
| POST | `/v1/key/issue` | device signature |
| GET | `/v1/key/status` | device header / admin |
| POST | `/v1/revoke` | admin |
| POST | `/v1/vouchers` | admin |
| GET | `/v1/vouchers` | admin |
| POST | `/v1/vouchers/revoke` | admin |

JSON field names used by the above remain stable.  
Additive optional fields are allowed if ignored by old clients.  
**Removing or renaming required fields is a breaking change.**

Endpoints outside this table (e.g. `/admin`, `/v1/devices`, `/v1/update/check`) are **operator/product extensions** — not required for protocol v1 peer interop.

---

## Frozen profiles

| Profile | Session binder |
|---------|----------------|
| `strong` (default) | ECDH static (+ eph) ∥ ordered GeneralKeys ∥ epoch ∥ profile string |
| `simple` | Org day key only (demo / closed LAN) |

---

## Compatibility promise (v1)

1. An agent speaking **protocol ver=1** Strong profile **MUST** interoperate with another ver=1 Strong peer that follows [05-crypto.md](05-crypto.md) and [04-p2p-protocol.md](04-p2p-protocol.md).  
2. KA and agent from the **same release line** (`v0.1.0-beta` and later patches that claim “protocol v1”) remain wire-compatible for the frozen surface.  
3. **Patches** (`v0.1.0-beta.1`, security fixes) **MUST NOT** break frozen items.  
4. **Breaking** wire/crypto changes **MUST** ship as **protocol v2** with a new `ver` and migration notes.

---

## Explicit non-promises

- Absolute security (“uncrackable”)  
- Metadata privacy  
- Hosted KA by the project  
- Cross-version beta experimental APIs (`/admin` layout, tray UI, update auto-download)

---

## Release binding

| Field | Value |
|-------|--------|
| Protocol | COE v1 |
| First freeze release | `v0.1.0-beta` |
| Date (repo) | 2026-07-15 |

Implementations **SHOULD** advertise `ver: 1` in HELLO and refuse unknown major versions.
