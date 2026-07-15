# COE P2P Protocol

**Protocol v1 FROZEN** (`v0.1.0-beta`). See [PROTOCOL_V1_FREEZE.md](PROTOCOL_V1_FREEZE.md).

Device ↔ device communication. **No Key Authority on this path.**

## Transports

| Profile | Transport | Use |
|---------|-----------|-----|
| **COE-TCP** (primary) | TCP | Reliable ordered records |
| **COE-UDP** (optional) | UDP | Datagram; app handles loss |

v1 implementations **MUST** support COE-TCP. COE-UDP is optional.

## Addressing (v1)

- Peers identified by `device_id`.
- Reachability: configured `host:port` (LAN or known public IP).
- No discovery protocol in v1.
- NAT: out of scope; both sides need a reachable path (or future appendix).

## TCP framing

Every message:

```
+----------------+---------------------------+
| length u32 BE  | payload (length bytes)    |
+----------------+---------------------------+
```

- `length` = size of CBOR payload only (not including the 4-byte header).
- Max payload: **65536** bytes (implementation **MUST** reject larger).
- Length `0` illegal.

## Record types

Payload is CBOR map or tagged struct with `type` field:

| `type` | Name | Encrypted? |
|--------|------|------------|
| 1 | `HELLO` | No |
| 2 | `HELLO_ACK` | No |
| 3 | `SESSION_OK` | No (optional confirm) |
| 4 | `DATA` | Yes (AEAD blob inside) |
| 5 | `CLOSE` | Optional AEAD |
| 6 | `EPOCH_WARN` | No |

## Handshake (COE-Strong, TCP)

Assume A initiates TCP to B.

### 1. A → B : `HELLO`

| Field | Type | Description |
|-------|------|-------------|
| `type` | 1 | |
| `ver` | uint | Protocol version `1` |
| `device_id` | bstr/string | A |
| `epoch_id` | uint | A’s current epoch |
| `identity_pk` | bstr(32) | A’s X25519 public |
| `eph_pk` | bstr(32) | A’s ephemeral X25519 public (recommended) |
| `session_nonce` | bstr(16) | Random |
| `key_serial` | uint | A’s issued serial (binding) |
| `profile` | string | `"strong"` |

### 2. B → A : `HELLO_ACK`

| Field | Type | Description |
|-------|------|-------------|
| `type` | 2 | |
| `ver` | 1 | |
| `device_id` | | B |
| `epoch_id` | uint | Must match A or be within grace rules |
| `identity_pk` | bstr(32) | B |
| `eph_pk` | bstr(32) | B ephemeral |
| `session_nonce` | bstr(16) | Random |
| `key_serial` | uint | B |
| `profile` | `"strong"` |
| `accept` | bool | false → stop |

### 3. Both sides derive keys

Per [02-key-management](02-key-management.md) / [05-crypto](05-crypto.md):

- Reject if `epoch_id` not in `{current, current-1 within grace}`.
- Reject if either side lacks `GeneralKey` for agreed epoch.
- Compute `SessionRoot`, then `K_send` / `K_recv`.
- Optional: exchange empty `DATA` or `SESSION_OK` with AEAD to confirm.

### Epoch agreement

```
agreed_epoch =
  if A.epoch_id == B.epoch_id: that epoch
  else if one is previous and now < not_after_prev + grace: previous
  else: fail handshake
```

Both must possess general keys for `agreed_epoch`.

## Handshake (COE-Simple)

Same frames; omit or ignore identity/eph fields; derive from shared `OrgDayKey` + nonces + sorted device ids only.

## DATA records

Clear CBOR envelope + ciphertext field:

| Field | Type | Description |
|-------|------|-------------|
| `type` | 4 | |
| `epoch_id` | uint | Must equal session agreed epoch |
| `seq` | uint | Per-direction monotonic, start 0 |
| `nonce` | bstr(12) | AEAD nonce (see crypto) |
| `ciphertext` | bstr | ChaCha20-Poly1305 output (includes tag) |

**AAD** (not sent separately; reconstructed):

```
AAD = "COE-v1-data" || I2OSP(ver,1) || I2OSP(epoch_id,8) ||
      sender_device_id || receiver_device_id || I2OSP(seq,8)
```

Sender encrypts with own `K_send`. Receiver decrypts with peer’s `K_send`.

### Replay protection

- Maintain sliding window of size **128** (or larger) per direction.
- Reject `seq` already seen or older than window low water.
- Gap: allow sparse seq within window; drop duplicates.

### Sequencing

- Separate seq spaces A→B and B→A.
- Overflow of `seq` (u64): rehandshake mandatory before wrap (impractical at normal rates).

## CLOSE

| Field | Description |
|-------|-------------|
| `type` | 5 |
| `reason` | uint / string |
| optional AEAD body | empty plaintext prove possession |

After CLOSE, tear down session keys.

## EPOCH_WARN

Either side **MAY** send when wall clock enters last **N** seconds of epoch (default N=600):

```
{ type: 6, epoch_id, not_after, hint: "rekey" }
```

Peers **SHOULD** finish or rehandshake with new keys after rollover.

## UDP profile (optional)

- One COE payload per UDP datagram (no length prefix); max 1200 bytes recommended for path MTU.
- Same CBOR types; DATA must fit one datagram or use app fragmentation (not specified in v1).
- Handshake over UDP needs retransmit; implementers **SHOULD** prefer TCP for v1 reliability.

## Connection lifecycle

```
TCP connect → HELLO → HELLO_ACK → (SESSION_OK) → DATA* → CLOSE → TCP close
```

Idle timeout: implementation-defined; recommend **300s** without DATA → CLOSE.

## Failure handling

| Condition | Action |
|-----------|--------|
| AEAD fail | Drop record; optional CLOSE; do not reveal padding oracles beyond constant-time fail |
| Bad epoch | Abort handshake |
| Unknown type | Ignore or CLOSE |
| Length > max | Close TCP |
| Peer key_serial mismatch vs policy | Optional reject if roster pins serial |

## What is NOT on the wire

- `GeneralKey` bytes
- `KA_MASTER`
- Identity private / ephemeral private keys
- Raw session root

## Application payload

Plaintext inside AEAD is opaque to COE: chat bytes, CBOR app messages, etc. COE does not define chat semantics — only secure transport of byte strings.

Max plaintext per DATA: such that ciphertext + envelope ≤ 65536.
