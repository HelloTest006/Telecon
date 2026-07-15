# Compatibility

## Protocol v1 freeze

**Wire and crypto surface is frozen** as of **`v0.1.0-beta`**.

Normative freeze document: [docs/coe/PROTOCOL_V1_FREEZE.md](docs/coe/PROTOCOL_V1_FREEZE.md)

| Change type | Allowed without new protocol major? |
|-------------|-------------------------------------|
| Bugfix preserving wire bytes | yes |
| Additive optional JSON/API fields | yes (if old clients ignore) |
| Rename/remove required wire fields | **no** → protocol v2 |
| Change AEAD/HKDF labels/nonce | **no** → protocol v2 |
| New P2P `type` &lt; 8 reused | **no** |
| New P2P `type` ≥ 8 optional | yes if not required for v1 |

## Supported combinations

| Agent | KA | Protocol | Status |
|------|----|----------|--------|
| `v0.1.0-beta` | `v0.1.0-beta` | v1 frozen | supported |
| later `v0.1.x` claiming protocol v1 | same | v1 frozen | supported if freeze honored |

## Product vs protocol

These may change without protocol v2:

- install scripts, Docker, admin HTML UI
- update-manifest URL layout (signing still future)
- STUN host defaults

## Upgrade guidance

1. Upgrade KA and signal first  
2. Run `ka-check`  
3. Roll agents  
4. Only re-mint vouchers if enroll policy changed (not for pure wire patches)

## Security note

Freeze is about **interop**, not a guarantee of audit completeness. Production still needs ops discipline and (later) external crypto review.
