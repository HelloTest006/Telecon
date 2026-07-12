# COE Threat Model

## Assets

| Asset | Sensitivity | Location |
|-------|-------------|----------|
| Application message plaintext | Critical | Device memory only during process |
| `GeneralKey` (daily) | Critical | Device secure storage; KA HSM/secrets |
| `KA_MASTER` | Critical | KA only |
| Long-term device identity private key | Critical | Device only |
| Session keys | Critical | Device RAM for session lifetime |
| Enrollment credentials / device certs | High | Device + KA |
| Peer roster (IPs, device_ids) | Medium (metadata) | Devices |
| Epoch / key_serial / request tickets | Low (public metadata) | Logs, wire |

## Adversaries

### A1 — Passive network eavesdropper (P2P path)

- Sees ciphertext, headers, timing, IPs, ports, sizes.
- **Goal**: recover plaintext.
- **Mitigation**: AEAD with secret session keys; no plaintext on wire.

### A2 — Active MITM on P2P path

- Can drop, replay, inject, reorder packets between devices.
- **Does not** hold current `GeneralKey`s or identity private keys.
- **Mitigation**: AEAD tags; handshake transcript binding; monotonic seq + replay window; identity authentication in COE-Strong.

### A3 — Compromised KA *after* successful issue

- Attacker later steals KA disk/logs but not past `KA_MASTER` uses already erased from devices’ old keys.
- **Mitigation**: epoch rotation; devices wipe expired keys; optional forward secrecy via ephemeral ECDH (recommended in handshake).
- **Residual**: if KA logs issued keys in plaintext, past epochs leak — **KA must not log raw general keys**.

### A4 — Compromised KA *at issue time*

- Attacker controls KA or `KA_MASTER` while keys are minted.
- **Can**: mint valid daily keys; impersonate KA to devices if TLS/cert PKI also broken.
- **Mitigation**: HSM, split control, audit, device pinning of KA cert/SPKI.
- **Residual**: in-scope as residual risk; COE does not protect against fully malicious KA for that epoch.

### A5 — Stolen / lost device (while epoch valid)

- Attacker extracts `GeneralKey` + identity key from device.
- **Can**: decrypt that device’s sessions for remaining epoch; impersonate device until revoke.
- **Mitigation**: secure storage, screen lock, remote revoke on KA, short epoch, wipe on expire.

### A6 — Malicious peer (authorized device)

- Valid enrolled device turns evil.
- **Can**: read sessions it participates in; spam peers it can reach.
- **Cannot** (COE-Strong): forge other devices’ identity without their keys.
- **Mitigation**: enrollment policy, revoke, application-layer ACLs.

### A7 — KA API attacker (unauthenticated Internet)

- Probes key issue endpoint.
- **Mitigation**: TLS, mTLS/device auth, rate limits, one key per device per epoch.

## Out of scope

- Endpoint malware with full process memory access during active session
- Side-channel (power, cache) extraction of keys
- Legal compulsion of KA operator
- Traffic analysis / metadata privacy (who talks to whom)
- Physical bus attacks without secure element assumptions
- Quantum adversary breaking ECDH/ChaCha (future PQ suite)

## Assumptions

1. Device and KA clocks stay within stated skew (or grace handles rollover).
2. Peer addresses are known/configured; attacker may still be on path (A1/A2).
3. TLS 1.3 to KA is correctly implemented; devices pin or validate KA identity.
4. `KA_MASTER` entropy ≥ 256 bits from CSPRNG at generation.
5. Devices delete expired general keys after grace.
6. Application does not exfiltrate keys via side channels of its own.

## Security properties (intended)

| Property | Profile | Held against |
|----------|---------|--------------|
| Confidentiality of P2P payloads | Both | A1, A2 (without keys) |
| Integrity / authenticity of records | Both | A2 |
| Server never sees app plaintext | Both | Architecture |
| Epoch-bounded key lifetime | Both | Policy |
| Mutual device authentication | COE-Strong | A2, A6 (other devices) |
| Forward secrecy within session | If ephemeral ECDH used | A3, delayed device compromise |
| Anonymity | Neither | Not provided |

## Explicit non-claim

COE is **not** “uncrackable.” Breaks if:

- Daily and/or identity secrets leak
- KA is malicious at issue time
- Crypto primitives are broken
- Implementation bugs (nonce reuse, key log, weak RNG for ECDH)

Correct product language: **“Requires current epoch key material (and device identity secrets in Strong profile) to decrypt; keys rotate every 24 hours; messages never transit the key server.”**
