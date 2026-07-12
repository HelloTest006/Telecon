# COE Key Management

## Hybrid design principle

| Mechanism | Role |
|-----------|------|
| **CSPRNG + HKDF-SHA-256** | All secret key bytes (`KA_MASTER` expansion → `GeneralKey`, session keys) |
| **Xoroshiro-128++** | Non-secret **on-demand** identifiers: `request_id`, `key_serial` stream, public `epoch_tag` mixers, anti-cache tickets |
| **Long-term identity ECDH** | COE-Strong mutual auth and session binding |
| **Ephemeral ECDH (recommended)** | Per-session forward secrecy |

**Normative rule:** Xoroshiro-128 **MUST NOT** be the sole source of cryptographic key material. Implementations that derive AEAD keys only from Xoroshiro output are **non-compliant**.

## Epoch

```
epoch_id    = floor(unix_utc / 86400)
not_before  = epoch_id * 86400
not_after   = not_before + 86400
grace_after = 300 seconds into next epoch (receive/close only)
```

## Server secrets

### `KA_MASTER`

- 32 bytes (256-bit), generated once with OS CSPRNG.
- Stored in HSM or sealed config; never shipped to devices.
- Rotation of `KA_MASTER` **invalidates** derivation for new epochs unless a documented re-wrap ceremony is performed (out of band).

### Xoroshiro-128++ instance (per process or per issue)

- Seeded at process start:  
  `seed_128 = SHA-256(KA_MASTER || "coe-xoroshiro-seed" || process_nonce)[0:16]`  
  (or two 64-bit words from that digest).
- On each key-issue request, advance PRNG to emit:
  - `request_id` (u64 or 16-byte ticket)
  - `key_serial` (u64, monotonic preference still from DB; PRNG may mix into public tag only)
  - optional public `mix_tag`

These values **MAY** appear in logs and responses. They **MUST NOT** be fed alone into AEAD key slots.

## Per-device daily general key

### Derivation (normative)

```
GeneralKey_d =
  HKDF-SHA-256(
    ikm  = KA_MASTER,
    salt = I2OSP(epoch_id, 8) || "COE-v1-salt",
    info = "COE-v1-general" || device_id || I2OSP(enrollment_counter, 4),
    L    = 32
  )
```

| Field | Description |
|-------|-------------|
| `device_id` | Stable unique id (16+ bytes recommended; UUID or KA-assigned) |
| `enrollment_counter` | Bumps on re-enroll / identity rotation so old devices cannot reuse derivation |
| `L` | 32 bytes |

### Issuance policy

1. Device authenticates to KA (see [03-ka-api](03-ka-api.md)).
2. KA computes `GeneralKey_d` for **current** `epoch_id` (or next if within pre-issue window — optional; v1 = current only).
3. **At most one logical general key** per `(device_id, epoch_id)`.
4. Re-request same epoch: return **same** `GeneralKey_d` (idempotent reconnect). Audit every request.
5. Different epoch: new key; previous epoch key not returned.
6. Revoked device: `403` / `revoked`; no key bytes.
7. KA **MUST NOT** write raw `GeneralKey` to application logs.

### Delivery

- Over TLS only.
- Response includes: `epoch_id`, `not_before`, `not_after`, `key_serial`, `request_id`, `general_key` (32 bytes), `profile_hint`.
- Device stores key in OS secure storage (or encrypted file with device secret) until `not_after + grace`.

### Device duties

- Wipe `GeneralKey` for epochs older than `current - 1` after grace.
- Never send `GeneralKey` to peers in clear (only use inside HKDF).
- Never upload app payloads to KA.

## Profiles

### COE-Simple (optional demo)

- Org shares one root for the day:

```
OrgDayKey =
  HKDF-SHA-256(
    ikm  = KA_MASTER,
    salt = I2OSP(epoch_id, 8) || "COE-v1-salt",
    info = "COE-v1-org-day" || org_id,
    L    = 32
  )
```

- Every enrolled device in `org_id` receives the same `OrgDayKey` as its “general key.”
- Session: `SessionKey = HKDF(OrgDayKey, "session" || session_nonce || sorted peer ids)`.
- **Blast radius:** any one device leak decrypts all org P2P for that epoch.
- **Not default** for production.

### COE-Strong (default)

Each device has distinct `GeneralKey_d` plus long-term X25519 identity keypair `(sk_d, pk_d)`.

**Static ECDH shared secret:**

```
ss_static = X25519(sk_A, pk_B)   // = X25519(sk_B, pk_A)
```

**Optional ephemeral (recommended):**

```
// A generates eph_A, B generates eph_B; exchange public eph
ss_eph = X25519(eph_sk_A, eph_pk_B)  // and symmetrically
```

**Session root:**

```
SessionRoot =
  HKDF-SHA-256(
    ikm  = ss_static || ss_eph || GeneralKey_A || GeneralKey_B,
    salt = session_nonce_A || session_nonce_B,
    info = "COE-v1-session" || I2OSP(epoch_id, 8) || sorted(device_id_A, device_id_B),
    L    = 32
  )
```

If ephemeral omitted (not recommended):

```
ikm = ss_static || GeneralKey_A || GeneralKey_B
```

**Direction keys:**

```
K_ab = HKDF-SHA-256(SessionRoot, salt=empty, info="COE-v1-send" || device_id_A, L=32)
K_ba = HKDF-SHA-256(SessionRoot, salt=empty, info="COE-v1-send" || device_id_B, L=32)
```

Sender uses own send key; receiver uses peer’s send key for decrypt.

**Ordering of `GeneralKey_A || GeneralKey_B`:** lexicographic by `device_id` so both sides concatenate identically:

```
if device_id_A < device_id_B:
  gk_pair = GeneralKey_A || GeneralKey_B
else:
  gk_pair = GeneralKey_B || GeneralKey_A
```

Same rule for sorting ids in `info`.

## Why both daily keys in COE-Strong?

If only ECDH were used, a stolen long-term identity key could open sessions without daily contact to KA. Requiring **both** peers’ current `GeneralKey`s means:

- Each peer must have successfully issued for this epoch.
- Revocation / non-issue of either side blocks new sessions.
- Network attacker without either daily key cannot complete SessionRoot even if they somehow observe public keys.

## Ratcheting (optional extension)

v1 **MAY** derive per-message keys:

```
K_msg_n = HKDF-SHA-256(K_send, info="COE-v1-msg" || I2OSP(n, 8), L=32)
```

v1 baseline: single `K_send` per direction + unique nonce from seq (see [05-crypto](05-crypto.md)).

## Revocation

| Event | KA action | Device action |
|-------|-----------|---------------|
| Admin revoke | Mark device revoked; refuse issue | Wipe keys on next failed issue / push if available |
| Epoch end | Stop issuing old epoch | Wipe old keys after grace |
| Suspected leak | Revoke + bump `enrollment_counter` on re-enroll | New identity + new derivation |

v1 has **no** real-time push revoke; devices learn on next `/v1/key/issue` or optional `/v1/key/status`.

## Xoroshiro-128++ reference role

Server generates **on demand** when handling issue:

```
ticket = {
  request_id:  xoroshiro.next_u64() or 16 bytes from PRNG stream,
  public_tag:  xoroshiro mix (non-secret),
}
```

Documented algorithm family: **xoroshiro128++** (Blackman & Vigna). Exact splitmix64 seed expand from 128-bit seed is implementation-defined as long as seed comes from hash(CSPRNG material) above.

## Lifecycle diagram

```
Enroll ──► Daily Issue ──► Store GeneralKey ──► P2P Session(s)
              ▲                    │                    │
              │                    │                    ▼
              └──── next epoch ────┴──── wipe expired ──┘
```
