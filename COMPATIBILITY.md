# Compatibility

## Current beta assumptions

- client / agent version should match same beta family as KA
- WebRTC signaling format should match same beta family
- voucher format should match same beta family

## Supported combinations

| Agent | KA | Status |
|------|----|--------|
| `v0.1.0-beta` | `v0.1.0-beta` | supported |

## Upgrade guidance

1. upgrade KA and signal first
2. run `ka-check`
3. mint new voucher only if enroll flow changed
4. roll agents after KA healthy

## Beta warning

No stability promise across beta lines yet.
