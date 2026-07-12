# COE Cryptography

## Algorithm suite (v1)

| Role | Algorithm |
|------|-----------|
| Hash / KDF | SHA-256 / HKDF-SHA-256 ([RFC 5869](https://www.rfc-editor.org/rfc/rfc5869)) |
| AEAD | ChaCha20-Poly1305 ([RFC 8439](https://www.rfc-editor.org/rfc/rfc8439)) |
| DH | X25519 ([RFC 7748](https://www.rfc-editor.org/rfc/rfc7748)) |
| Non-secret PRNG | xoroshiro128++ |
| Master entropy | OS CSPRNG (`crypto/rand` in Go) |

## Sizes

| Item | Bytes |
|------|-------|
| `KA_MASTER` | 32 |
| `GeneralKey` | 32 |
| `SessionRoot` | 32 |
| `K_send` / AEAD key | 32 |
| AEAD nonce | 12 |
| Poly1305 tag | 16 (appended to ciphertext per RFC 8439) |
| X25519 public/private | 32 |
| `session_nonce` | 16 |
| Issue client `nonce` | 16 |

## HKDF conventions

```
HKDF(ikm, salt, info, L) =
  HKDF-Expand(HKDF-Extract(salt, ikm), info, L)
```

- Empty salt: use `HashLen` zeros only if a section explicitly says `salt=empty`; prefer explicit salts.
- All `info` strings are ASCII without NUL terminator.
- `I2OSP(x, n)` = unsigned big-endian integer `x` in `n` bytes.
- String concatenation `||` is byte concatenation.

### Labels (normative)

| Purpose | `info` / salt material |
|---------|------------------------|
| Xoroshiro seed material | `"coe-xoroshiro-seed"` inside SHA-256 input |
| General key salt suffix | `"COE-v1-salt"` |
| General key info | `"COE-v1-general" \|\| device_id \|\| I2OSP(enrollment_counter, 4)` |
| Org day key info | `"COE-v1-org-day" \|\| org_id` |
| Session root info | `"COE-v1-session" \|\| I2OSP(epoch_id, 8) \|\| id_lo \|\| id_hi` |
| Send key info | `"COE-v1-send" \|\| device_id` |
| Optional msg key | `"COE-v1-msg" \|\| I2OSP(seq, 8)` |

`id_lo \|\| id_hi` = two `device_id`s sorted lexicographically as byte strings (UTF-8 if string form).

## GeneralKey derivation

```
salt = I2OSP(epoch_id, 8) || "COE-v1-salt"
info = "COE-v1-general" || device_id || I2OSP(enrollment_counter, 4)
GeneralKey = HKDF(KA_MASTER, salt, info, 32)
```

## Session derivation (COE-Strong)

```
ss_static = X25519(sk_local, pk_peer)

// if ephemeral used:
ss_eph = X25519(eph_sk_local, eph_pk_peer)

// ordered general keys:
if device_id_local < device_id_peer as bytes:
  // each side still uses global order by id, not "local/peer":
pass

if device_id_A < device_id_B:
  gk = GeneralKey_A || GeneralKey_B
  id_lo, id_hi = device_id_A, device_id_B
else:
  gk = GeneralKey_B || GeneralKey_A
  id_lo, id_hi = device_id_B, device_id_A

ikm = ss_static || ss_eph || gk   // omit ss_eph if no eph
// if no eph: ikm = ss_static || gk

salt = session_nonce_A || session_nonce_B
// order nonces by device_id of creator:
if device_id_A < device_id_B:
  salt = nonce_A || nonce_B
else:
  salt = nonce_B || nonce_A

info = "COE-v1-session" || I2OSP(epoch_id, 8) || id_lo || id_hi
SessionRoot = HKDF(ikm, salt, info, 32)

K_send_A = HKDF(SessionRoot, salt=empty_32zero_or_nil, info="COE-v1-send"||device_id_A, 32)
K_send_B = HKDF(SessionRoot, salt=empty_32zero_or_nil, info="COE-v1-send"||device_id_B, 32)
```

**Salt for send HKDF:** use **Extract salt = 32 zero bytes** for interoperability (normative).

## Session derivation (COE-Simple)

```
ikm = OrgDayKey
salt = ordered(session_nonce_A, session_nonce_B) by device_id
info = "COE-v1-session-simple" || I2OSP(epoch_id, 8) || id_lo || id_hi
SessionRoot = HKDF(ikm, salt, info, 32)
// K_send_* same as Strong
```

## AEAD seal / open

### Nonce construction (normative)

12-byte nonce, unique per `(K_send, nonce)` pair:

```
nonce = I2OSP(0, 4) || I2OSP(seq, 8)   // big-endian
```

- `seq` is the DATA record sequence number for that send direction.
- **MUST NOT** reuse `seq` under same `K_send`.
- Rehandshake → new `SessionRoot` → seq may restart at 0.

### Seal

```
ciphertext || tag = ChaCha20-Poly1305-Seal(
  key  = K_send_sender,
  nonce = nonce,
  aad   = AAD,
  pt    = application_plaintext
)
```

Wire field `ciphertext` = `ciphertext || tag` (RFC 8439 layout).

### AAD

```
AAD = "COE-v1-data" ||
      I2OSP(1, 1) ||                 // ver
      I2OSP(epoch_id, 8) ||
      sender_device_id ||
      receiver_device_id ||
      I2OSP(seq, 8)
```

`device_id` encodings **MUST** match handshake byte encoding exactly.

## Xoroshiro-128++

### Allowed outputs

- `request_id`
- `public_tag`
- Non-cryptographic load-balancing tokens

### Forbidden

- AEAD keys, `GeneralKey`, nonces for AEAD (use CSPRNG or seq construction), ECDH private keys

### Seeding

```
h = SHA-256(KA_MASTER || "coe-xoroshiro-seed" || process_nonce)
s0 || s1 = h[0:8] || h[8:16]   as little-endian u64 words
```

`process_nonce` = 16 bytes from `crypto/rand` at process start.

Algorithm: xoroshiro128++ as published by Blackman & Vigna; next state after each u64 emit.

## Randomness requirements

| Value | Source |
|-------|--------|
| `KA_MASTER` | CSPRNG |
| Identity / ephemeral sk | CSPRNG |
| `session_nonce`, issue `nonce` | CSPRNG |
| AEAD nonce | Deterministic from `seq` (above) |
| Tickets | Xoroshiro (non-secret) |

## Constant-time

- AEAD verify failures: constant-time compare of tags (library default).
- Do not branch on secret key bits in custom code.

## Test vectors (outline)

Future file `test-vectors.json` **SHOULD** include:

1. Fixed `KA_MASTER`, `epoch_id`, `device_id`, `enrollment_counter` → `GeneralKey`
2. Fixed X25519 scalars + general keys + nonces → `SessionRoot`, `K_send_*`
3. Fixed key, seq, plaintext, AAD → ciphertext+tag

Until vectors exist, implementations cross-check with a second language or known HKDF test suite for labels.

## Downgrade resistance

- `ver` and `profile` in handshake **MUST** be authenticated by inclusion in session derivation `info` or by first AEAD confirm.
- Normative: include profile byte in session `info`:

```
info = "COE-v1-session" || I2OSP(epoch_id, 8) || id_lo || id_hi || profile_string
```

(`profile_string` = `"strong"` or `"simple"`).

## Forbidden practices

1. ECB or unauthenticated encryption
2. AES-CBC + HMAC invent-your-own without review (use suite above)
3. Truncating Poly1305 tags
4. Sharing one nonce across directions
5. Logging secrets
6. Deriving keys from passwords without proper KDF (out of scope; enrollment is cert-based)
