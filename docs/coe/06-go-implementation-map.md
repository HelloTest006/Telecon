# COE Go Implementation Map

Spec-only guidance for a future Go codebase. **No code in this deliverable.**

## Suggested module

```
module github.com/example/coe   // replace with real path
go 1.22+
```

## Directory layout

```
cmd/
  ka/                 # Key Authority HTTP server
  coe-node/           # Device agent: pull key + P2P
  coe-keygen/         # Optional: generate KA_MASTER, device ids (offline)

internal/
  crypto/
    hkdf.go           # HKDF wrappers, labels
    aead.go           # ChaCha20-Poly1305 seal/open + nonce from seq
    x25519.go         # identity + ephemeral helpers
    general_key.go    # GeneralKey / OrgDayKey derivation
    session.go        # SessionRoot, K_send
    xoroshiro.go      # xoroshiro128++ for tickets only
  kaapi/
    types.go          # request/response structs (CBOR + JSON tags)
    client.go         # device → KA client
    server.go         # chi/echo/stdlib handlers
    auth.go           # mTLS / device binding
    audit.go          # structured audit without secrets
  p2p/
    frame.go          # TCP length-prefix read/write
    handshake.go      # HELLO / HELLO_ACK
    session.go        # state machine, replay window
    data.go           # DATA seal/open
    udp.go            # optional
  device/
    store.go          # secure-ish key storage interface
    enroll.go
    clock.go          # epoch helpers, skew checks
  config/
    config.go         # roster YAML/JSON: peers host:port
  cborx/
    codec.go          # fxamacker/cbor or similar

docs/coe/             # this specification tree
testdata/
  vectors/            # future test-vectors.json
```

## Package responsibilities

| Package | Responsibility | Must not |
|---------|----------------|----------|
| `internal/crypto` | Pure crypto, no network | Log secrets; use Xoroshiro for keys |
| `internal/kaapi` | HTTP issue/enroll/revoke | Relay P2P messages |
| `internal/p2p` | Device sessions | Call KA on message path |
| `cmd/ka` | Wire KA process | Store app chat history |
| `cmd/coe-node` | Wire device process | Send plaintext to KA |

## Dependencies (illustrative)

| Need | Module |
|------|--------|
| CBOR | `github.com/fxamacker/cbor/v2` |
| HTTP router | stdlib or `chi` |
| TLS | `crypto/tls` |
| X25519 / HKDF / ChaCha | `golang.org/x/crypto` |
| Config | `gopkg.in/yaml.v3` or JSON |

Prefer stdlib where enough.

## Process: Key Authority

```
main:
  load KA_MASTER from env/file/HSM stub
  open audit sink
  load device registry (DB or file)
  init xoroshiro from SHA-256(master||…)
  listen HTTPS :8443
  routes: /v1/health, /v1/enroll, /v1/key/issue, /v1/key/status, /v1/revoke
```

**Env (example):**

- `COE_KA_MASTER_FILE`
- `COE_KA_TLS_CERT` / `COE_KA_TLS_KEY`
- `COE_KA_CLIENT_CA` (for mTLS)

## Process: coe-node

```
main:
  load identity sk, device_id, KA URL, peer roster
  POST /v1/key/issue → store GeneralKey + epoch meta
  for each peer or on-demand:
    dial TCP host:port
    handshake → session
    read/write DATA from stdin or local API
  background: refresh key near epoch boundary
```

**Local API (optional future):** localhost UNIX socket or HTTP for apps to send/receive plaintext to the node (node does crypto). Keeps KA off message path.

## Interfaces (sketch)

```go
// storage
type KeyStore interface {
    PutGeneral(epoch uint64, key []byte, meta Meta) error
    GetGeneral(epoch uint64) ([]byte, Meta, error)
    WipeBefore(epoch uint64) error
}

// p2p
type Session interface {
    Send(plaintext []byte) error
    Recv() ([]byte, error)
    Close(reason string) error
    Epoch() uint64
}
```

## Testing plan (when implementing)

1. **Unit:** HKDF vectors, nonce construction, replay window, epoch math.
2. **Interop:** two `coe-node` on localhost TCP, COE-Strong.
3. **KA:** issue idempotency same epoch; revoke blocks issue.
4. **Negative:** wrong epoch, AEAD tamper, seq replay.
5. **Never:** commit `KA_MASTER` or sample `general_key` from live systems into git.

## Build tags / profiles

- Default build: COE-Strong.
- `simple` build tag or config flag: COE-Simple for demos only; log loud warning at startup.

## Implementation order (recommended)

1. `internal/crypto` + unit tests  
2. `kaapi` issue + file registry  
3. `p2p` TCP handshake + DATA  
4. `cmd/ka` + `cmd/coe-node`  
5. Grace/epoch rollover tests  
6. Optional UDP  

## Compliance checklist

- [ ] Xoroshiro not used for AEAD/GeneralKey bits  
- [ ] No app payload to KA  
- [ ] `Cache-Control: no-store` on key responses  
- [ ] Audit without raw keys  
- [ ] Replay window enforced  
- [ ] Claims in UI match [01-threat-model](01-threat-model.md) language  
